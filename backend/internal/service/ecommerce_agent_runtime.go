package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"infinite-canvas/backend/internal/agentruntime"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/gorm"
)

// Ecommerce Agent Runtime IR-01 intentionally has no logical-model or
// provider dependency. It is the first durable, deterministic slice used to
// prove that an Ecommerce Run can be created, claimed, executed and read back
// through the generic Agent Runtime persistence contract.
const (
	ecommerceAgentRuntimeRouteID       = "IR-01"
	ecommerceAgentRuntimeAgentID       = "product_intelligence_agent"
	ecommerceAgentRuntimeSkillID       = "product-intelligence"
	ecommerceAgentRuntimeOutputType    = "product_dna"
	ecommerceAgentRuntimeExecutor      = "ecommerce-product-intelligence-deterministic-v1"
	ecommerceAgentRuntimeModelRef      = "deterministic:none"
	ecommerceAgentRuntimeOwnerSuffix   = ":ecommerce"
	ecommerceAgentRuntimeLeaseDuration = 90 * time.Second
	maxEcommerceAgentRunInputBytes     = 256 << 10
)

var ecommerceAgentRuntimeIdempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,128}$`)

// CreateEcommerceAgentRunRequest is deliberately provider-neutral. Product
// facts are user input, not claims from a model; the deterministic executor
// preserves unknown values instead of inventing commercial facts.
type CreateEcommerceAgentRunRequest struct {
	Objective                string         `json:"objective"`
	Input                    map[string]any `json:"input"`
	ProductAssetIDs          []string       `json:"productAssetIds"`
	ProductFacts             map[string]any `json:"productFacts"`
	InputArtifactRevisionIDs []string       `json:"inputArtifactRevisionIds"`
	IdempotencyKey           string         `json:"idempotencyKey,omitempty"`
}

// EcommerceAgentRunCreateResult mirrors the Film runtime create envelope and
// makes idempotent replay explicit to callers.
type EcommerceAgentRunCreateResult struct {
	Detail     repository.AgentRuntimeDetail `json:"detail"`
	Idempotent bool                          `json:"idempotent"`
}

// CreateEcommerceProductIntelligenceRun is a descriptive alias retained for
// callers that want to make the fixed IR-01 route explicit.
func (s *Service) CreateEcommerceProductIntelligenceRun(userID string, projectID string, idempotencyKey string, request CreateEcommerceAgentRunRequest) (EcommerceAgentRunCreateResult, error) {
	return s.CreateEcommerceAgentRun(userID, projectID, idempotencyKey, request)
}

// CreateEcommerceAgentRun creates the Ecommerce IR-01 durable bundle. It does
// not enqueue a paid task and does not resolve a logical model. Input artifact
// revisions, when supplied, are checked against the Ecommerce project/domain
// and copied as immutable references into the Step contract.
func (s *Service) CreateEcommerceAgentRun(userID string, projectID string, idempotencyKey string, request CreateEcommerceAgentRunRequest) (EcommerceAgentRunCreateResult, error) {
	if err := s.validateEcommerceAgentRegistry(); err != nil {
		return EcommerceAgentRunCreateResult{}, err
	}
	project, err := s.requireEcommerceProject(userID, projectID)
	if err != nil {
		return EcommerceAgentRunCreateResult{}, err
	}

	idempotencyKey = strings.TrimSpace(firstNonEmpty(idempotencyKey, request.IdempotencyKey))
	if !ecommerceAgentRuntimeIdempotencyKeyPattern.MatchString(idempotencyKey) {
		return EcommerceAgentRunCreateResult{}, BadAuthRequest("Ecommerce Agent Run 幂等键必须为 8-128 位字母、数字或 ._:-")
	}
	objective := strings.TrimSpace(request.Objective)
	if objective == "" {
		return EcommerceAgentRunCreateResult{}, BadAuthRequest("请填写商品事实分析目标")
	}
	if len([]rune(objective)) > 4000 {
		return EcommerceAgentRunCreateResult{}, BadAuthRequest("商品事实分析目标不能超过 4000 字")
	}

	normalizedInput, err := normalizeEcommerceAgentRunInput(request)
	if err != nil {
		return EcommerceAgentRunCreateResult{}, err
	}
	inputRefs, err := s.resolveEcommerceAgentInputRefs(userID, project.ID, request.InputArtifactRevisionIDs)
	if err != nil {
		return EcommerceAgentRunCreateResult{}, err
	}
	requestDigest := ecommerceAgentRunRequestDigest(project.ID, objective, normalizedInput, inputRefs)
	if existing, lookupErr := s.repo.AgentRuntimeRunByIdempotency(userID, idempotencyKey); lookupErr == nil {
		if existing.ProjectID != project.ID || existing.Domain != "ecommerce" || ecommerceAgentRunStoredDigest(existing.InputJSON) != requestDigest {
			return EcommerceAgentRunCreateResult{}, conflictError("该幂等键已用于另一项 Ecommerce Agent 请求")
		}
		detail, detailErr := s.repo.AgentRuntimeDetailForUser(userID, existing.ID)
		return EcommerceAgentRunCreateResult{Detail: detail, Idempotent: true}, detailErr
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return EcommerceAgentRunCreateResult{}, lookupErr
	}

	now := time.Now().UTC()
	runID := newID()
	stepID := newID()
	inputRefsJSON := mustEcommerceAgentRuntimeJSON(inputRefs)
	step := model.AgentRuntimeStep{
		ID: stepID, RunID: runID, StepKey: "intent:" + ecommerceAgentRuntimeRouteID,
		Position: 1, RouteKind: "intent", RouteID: ecommerceAgentRuntimeRouteID,
		AgentID: ecommerceAgentRuntimeAgentID, SkillIDsJSON: mustEcommerceAgentRuntimeJSON([]string{ecommerceAgentRuntimeSkillID}),
		Status: model.AgentStepStatusReady, DependsOnStepIDsJSON: "[]", InputArtifactRefsJSON: inputRefsJSON,
		ExpectedOutputArtifactTypesJSON: mustEcommerceAgentRuntimeJSON([]string{ecommerceAgentRuntimeOutputType}),
		OutputArtifactRefsJSON:          "[]", AttemptSequence: 0, Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	runInput, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "requestDigest": requestDigest, "routeId": ecommerceAgentRuntimeRouteID,
		"input": normalizedInput, "inputArtifactRefs": inputRefs,
	})
	if err != nil {
		return EcommerceAgentRunCreateResult{}, err
	}
	run := &model.AgentRuntimeRun{
		ID: runID, UserID: userID, ProjectID: project.ID, Domain: "ecommerce",
		RegistryID: s.ecommerceAgentRegistry.ID, RegistryVersion: s.ecommerceAgentRegistry.Version,
		RegistryDigest: s.ecommerceAgentRegistry.SourceDigest, RouteKind: "intent", IntentRouteID: ecommerceAgentRuntimeRouteID,
		RootRunID: runID, Status: model.AgentRunStatusReady, Objective: objective, InputJSON: string(runInput),
		CurrentStepID: stepID, IdempotencyKey: idempotencyKey, Revision: 1, EventSequence: 2,
		CreatedAt: now, UpdatedAt: now,
	}
	routing := &model.AgentRoutingDecision{
		ID: newID(), RunID: runID, RouteKind: "intent", RouteID: ecommerceAgentRuntimeRouteID,
		IntentRouteID: ecommerceAgentRuntimeRouteID, SelectedAgentID: ecommerceAgentRuntimeAgentID,
		SelectedSkillIDsJSON:  mustEcommerceAgentRuntimeJSON([]string{ecommerceAgentRuntimeSkillID}),
		InputArtifactRefsJSON: inputRefsJSON, AlternativesJSON: "[]", Reason: "IR-01 deterministic product intelligence route",
		Confidence: "confirmed", DecidedByType: "runtime", DecidedByID: ecommerceAgentRuntimeExecutor, CreatedAt: now,
	}
	events := []model.AgentRuntimeEvent{
		{ID: newID(), UserID: userID, RunID: runID, Sequence: 1, EventType: "run.created", ActorType: "user", ActorID: userID,
			ToStatus: string(model.AgentRunStatusPlanning), PayloadJSON: mustEcommerceAgentRuntimeJSON(map[string]any{"registryDigest": run.RegistryDigest}), CreatedAt: now},
		{ID: newID(), UserID: userID, RunID: runID, Sequence: 2, StepID: stepID, EventType: "run.planned", ActorType: "runtime", ActorID: ecommerceAgentRuntimeExecutor,
			FromStatus: string(model.AgentRunStatusPlanning), ToStatus: string(model.AgentRunStatusReady),
			PayloadJSON: mustEcommerceAgentRuntimeJSON(map[string]any{"routeId": ecommerceAgentRuntimeRouteID, "agentId": ecommerceAgentRuntimeAgentID, "skillIds": []string{ecommerceAgentRuntimeSkillID}, "provider": "none", "billing": "none"}), CreatedAt: now},
	}
	if err := s.repo.CreateAgentRuntimeBundle(repository.AgentRuntimeCreateBundle{Run: run, RoutingDecision: routing, Steps: []model.AgentRuntimeStep{step}, Events: events}); err != nil {
		if existing, lookupErr := s.repo.AgentRuntimeRunByIdempotency(userID, idempotencyKey); lookupErr == nil && existing.ProjectID == project.ID && existing.Domain == "ecommerce" && ecommerceAgentRunStoredDigest(existing.InputJSON) == requestDigest {
			detail, detailErr := s.repo.AgentRuntimeDetailForUser(userID, existing.ID)
			return EcommerceAgentRunCreateResult{Detail: detail, Idempotent: true}, detailErr
		}
		return EcommerceAgentRunCreateResult{}, err
	}
	detail, err := s.repo.AgentRuntimeDetailForUser(userID, runID)
	return EcommerceAgentRunCreateResult{Detail: detail}, err
}

// EcommerceAgentRunDetail returns only the requested user's Ecommerce Run;
// generic repository detail is used so attempts, events and artifact
// revisions remain one durable projection.
func (s *Service) EcommerceAgentRunDetail(userID string, projectID string, runID string) (repository.AgentRuntimeDetail, error) {
	if _, err := s.ecommerceProjectForRead(userID, projectID); err != nil {
		return repository.AgentRuntimeDetail{}, err
	}
	detail, err := s.repo.AgentRuntimeDetailForUser(userID, strings.TrimSpace(runID))
	if err != nil {
		return repository.AgentRuntimeDetail{}, err
	}
	if detail.Run.ProjectID != projectID || detail.Run.Domain != "ecommerce" {
		return repository.AgentRuntimeDetail{}, NotFound("Ecommerce Agent Run 不存在")
	}
	return detail, nil
}

// ReadEcommerceAgentRun is a read-oriented alias for API/service callers.
func (s *Service) ReadEcommerceAgentRun(userID string, projectID string, runID string) (repository.AgentRuntimeDetail, error) {
	return s.EcommerceAgentRunDetail(userID, projectID, runID)
}

// ProcessNextEcommerceProductIntelligenceStep is a descriptive alias for the
// fixed IR-01 worker entry point.
func (s *Service) ProcessNextEcommerceProductIntelligenceStep() (bool, error) {
	return s.ProcessNextEcommerceAgentStep()
}

// ProcessNextEcommerceAgentStep claims and executes one Ecommerce Step. The
// executor is intentionally synchronous and deterministic: it produces a
// ProductDNA contract from persisted input, never creates a provider Task and
// never touches billing.
func (s *Service) ProcessNextEcommerceAgentStep() (bool, error) {
	if err := s.validateEcommerceAgentRegistry(); err != nil {
		return false, err
	}
	claim, err := s.repo.ClaimNextAgentRuntimeExecution(repository.AgentRuntimeExecutionClaimCommand{
		Domain: "ecommerce", Owner: s.workerID + ecommerceAgentRuntimeOwnerSuffix,
		LeaseDuration: ecommerceAgentRuntimeLeaseDuration, AttemptID: newID(), TaskID: "deterministic:" + newID(),
		EventID: newID(), Executor: ecommerceAgentRuntimeExecutor, At: time.Now().UTC(),
	})
	if err != nil || claim == nil {
		return false, err
	}
	return true, s.executeEcommerceAgentClaim(*claim)
}

func (s *Service) executeEcommerceAgentClaim(claim repository.AgentRuntimeExecutionClaim) error {
	owner := s.workerID + ecommerceAgentRuntimeOwnerSuffix
	attemptRevision := claim.Attempt.Revision
	inputDigest, promptDigest := ecommerceAgentClaimDigests(claim)
	requestJSON := mustEcommerceAgentRuntimeJSON(map[string]any{
		"schemaVersion": 1, "routeId": ecommerceAgentRuntimeRouteID, "agentId": ecommerceAgentRuntimeAgentID,
		"skillIds": []string{ecommerceAgentRuntimeSkillID}, "expectedOutputArtifactTypes": []string{ecommerceAgentRuntimeOutputType},
		"inputDigest": inputDigest, "promptDigest": promptDigest, "provider": "none", "billing": "none", "deterministic": true,
	})
	prepared, err := s.repo.PrepareClaimedAgentRuntimeAttempt(repository.AgentRuntimeAttemptMetadata{
		Owner: owner, RunID: claim.Run.ID, StepID: claim.Step.ID, AttemptID: claim.Attempt.ID,
		ExpectedAttemptRevision: claim.Attempt.Revision, Executor: ecommerceAgentRuntimeExecutor,
		ModelRef: ecommerceAgentRuntimeModelRef, InputDigest: inputDigest, PromptDigest: promptDigest,
		RequestJSON: requestJSON, At: time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	attemptRevision = prepared.Revision

	rawOutput, err := s.deterministicEcommerceProductDNAOutput(claim, s.ecommerceAgentRegistry)
	if err != nil {
		return s.failEcommerceAgentClaim(claim, attemptRevision, err)
	}
	output, err := agentruntime.ParseExecutionOutput(rawOutput, []string{ecommerceAgentRuntimeOutputType})
	if err != nil {
		return s.failEcommerceAgentClaim(claim, attemptRevision, fmt.Errorf("Ecommerce ProductDNA structured output invalid: %w", err))
	}
	responseJSON, err := agentruntime.CanonicalExecutionOutputJSON(output)
	if err != nil {
		return s.failEcommerceAgentClaim(claim, attemptRevision, err)
	}
	writes, err := s.buildEcommerceProductDNAArtifactWrites(claim, output)
	if err != nil {
		return s.failEcommerceAgentClaim(claim, attemptRevision, err)
	}
	_, err = s.repo.CompleteAgentRuntimeExecution(repository.AgentRuntimeExecutionCompleteCommand{
		Owner: owner, UserID: claim.Run.UserID, RunID: claim.Run.ID, StepID: claim.Step.ID, AttemptID: claim.Attempt.ID,
		ExpectedRunRevision: claim.Run.Revision, ExpectedStepRevision: claim.Step.Revision, ExpectedAttemptRevision: attemptRevision,
		ResponseJSON: responseJSON, ArtifactWrites: writes,
		Event: repository.AgentRuntimeEventInput{
			ID: newID(), EventType: "attempt.succeeded", ActorType: "executor", ActorID: ecommerceAgentRuntimeExecutor,
			PayloadJSON: mustEcommerceAgentRuntimeJSON(map[string]any{"artifactTypes": []string{ecommerceAgentRuntimeOutputType}, "provider": "none", "billing": "none"}),
		},
		At: time.Now().UTC(),
	})
	return err
}

func (s *Service) failEcommerceAgentClaim(claim repository.AgentRuntimeExecutionClaim, attemptRevision int64, executionErr error) error {
	if errors.Is(executionErr, repository.ErrAgentRuntimeLeaseLost) || errors.Is(executionErr, repository.ErrAgentRuntimeStateConflict) {
		return executionErr
	}
	code := "ecommerce_product_intelligence_failed"
	message := truncateRunes(executionErr.Error(), 2000)
	_, err := s.repo.FailAgentRuntimeExecution(repository.AgentRuntimeExecutionFailCommand{
		Owner: s.workerID + ecommerceAgentRuntimeOwnerSuffix, UserID: claim.Run.UserID, RunID: claim.Run.ID,
		StepID: claim.Step.ID, AttemptID: claim.Attempt.ID, ExpectedRunRevision: claim.Run.Revision,
		ExpectedStepRevision: claim.Step.Revision, ExpectedAttemptRevision: attemptRevision, FailureCode: code, Failure: message,
		Event: repository.AgentRuntimeEventInput{ID: newID(), EventType: "attempt.failed", ActorType: "executor", ActorID: ecommerceAgentRuntimeExecutor,
			PayloadJSON: mustEcommerceAgentRuntimeJSON(map[string]any{"failureCode": code, "failure": message, "provider": "none", "billing": "none"})},
		At: time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	return executionErr
}

type ecommerceAgentInputArtifactRef struct {
	ArtifactID string `json:"artifactId"`
	RevisionID string `json:"revisionId"`
	Type       string `json:"type"`
	Version    int    `json:"version"`
	Digest     string `json:"digest"`
	Status     string `json:"status"`
}

func (s *Service) resolveEcommerceAgentInputRefs(userID string, projectID string, revisionIDs []string) ([]ecommerceAgentInputArtifactRef, error) {
	if len(revisionIDs) > 50 {
		return nil, BadAuthRequest("单个 Ecommerce Agent Run 最多引用 50 个 Artifact revision")
	}
	seen := make(map[string]struct{}, len(revisionIDs))
	refs := make([]ecommerceAgentInputArtifactRef, 0, len(revisionIDs))
	for _, revisionID := range revisionIDs {
		revisionID = strings.TrimSpace(revisionID)
		if revisionID == "" {
			continue
		}
		if _, duplicate := seen[revisionID]; duplicate {
			return nil, BadAuthRequest("Ecommerce Agent 输入存在重复 Artifact revision")
		}
		seen[revisionID] = struct{}{}
		artifact, revision, err := s.repo.ProductionArtifactRevisionForUser(userID, revisionID)
		if err != nil {
			return nil, err
		}
		if artifact.ProjectID != projectID || artifact.Domain != "ecommerce" || artifact.ArtifactType != "product_upload" {
			return nil, BadAuthRequest("IR-01 只接受当前 Ecommerce 项目的 product_upload Artifact")
		}
		refs = append(refs, ecommerceAgentInputArtifactRef{ArtifactID: artifact.ID, RevisionID: revision.ID, Type: revisionArtifactType(*artifact), Version: revision.Version, Digest: revision.ContentDigest, Status: string(revision.Status)})
	}
	return refs, nil
}

func revisionArtifactType(artifact model.ProductionArtifact) string { return artifact.ArtifactType }

func normalizeEcommerceAgentRunInput(request CreateEcommerceAgentRunRequest) (map[string]any, error) {
	input := make(map[string]any, len(request.Input)+2)
	for key, value := range request.Input {
		key = strings.TrimSpace(key)
		if key != "" {
			input[key] = value
		}
	}
	if len(request.ProductAssetIDs) > 0 {
		input["productAssetIds"] = uniqueNonEmpty(request.ProductAssetIDs)
	}
	if len(request.ProductFacts) > 0 {
		input["productFacts"] = request.ProductFacts
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return nil, BadAuthRequest("商品事实分析输入格式无效")
	}
	if len(encoded) > maxEcommerceAgentRunInputBytes {
		return nil, BadAuthRequest("商品事实分析输入不能超过 256 KiB")
	}
	var normalized map[string]any
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		return nil, BadAuthRequest("商品事实分析输入无法标准化")
	}
	return normalized, nil
}

func ecommerceAgentRunRequestDigest(projectID string, objective string, input map[string]any, refs []ecommerceAgentInputArtifactRef) string {
	return digestEcommerceAgentRuntimeValue(map[string]any{"projectId": projectID, "objective": objective, "input": input, "inputArtifactRefs": refs})
}

func ecommerceAgentRunStoredDigest(inputJSON string) string {
	var envelope struct {
		RequestDigest string `json:"requestDigest"`
	}
	if json.Unmarshal([]byte(inputJSON), &envelope) == nil {
		return strings.TrimSpace(envelope.RequestDigest)
	}
	return ""
}

func ecommerceAgentClaimDigests(claim repository.AgentRuntimeExecutionClaim) (string, string) {
	inputDigest := digestEcommerceAgentRuntimeValue(map[string]any{"objective": claim.Run.Objective, "input": claim.Run.InputJSON, "refs": claim.Step.InputArtifactRefsJSON})
	promptDigest := digestEcommerceAgentRuntimeValue(map[string]any{"registry": claim.Run.RegistryDigest, "agent": ecommerceAgentRuntimeAgentID, "skill": ecommerceAgentRuntimeSkillID, "output": ecommerceAgentRuntimeOutputType})
	return inputDigest, promptDigest
}

func (s *Service) deterministicEcommerceProductDNAOutput(claim repository.AgentRuntimeExecutionClaim, registry *agentruntime.Registry) (string, error) {
	var envelope struct {
		Input map[string]any `json:"input"`
	}
	if err := json.Unmarshal([]byte(claim.Run.InputJSON), &envelope); err != nil {
		return "", fmt.Errorf("decode Ecommerce Agent input: %w", err)
	}
	var refs []ecommerceAgentInputArtifactRef
	if err := json.Unmarshal([]byte(claim.Step.InputArtifactRefsJSON), &refs); err != nil {
		return "", fmt.Errorf("decode Ecommerce input Artifact refs: %w", err)
	}
	assetIDs := ecommerceInputStringSlice(envelope.Input, "productAssetIds")
	facts := ecommerceInputMap(envelope.Input, "productFacts")
	unknowns := []string{"dimensions", "ingredients", "certifications", "unverified efficacy claims"}
	sources := make([]string, 0, len(refs)+len(assetIDs))
	sourceArtifacts := make([]map[string]any, 0, len(refs))
	for _, ref := range refs {
		sources = append(sources, ref.RevisionID)
		artifact, revision, err := s.repo.ProductionArtifactRevisionForUser(claim.Run.UserID, ref.RevisionID)
		if err != nil {
			return "", fmt.Errorf("load Ecommerce input Artifact %s: %w", ref.RevisionID, err)
		}
		if artifact.ProjectID != claim.Run.ProjectID || artifact.Domain != "ecommerce" || artifact.ArtifactType != "product_upload" || revision.ContentDigest != ref.Digest {
			return "", fmt.Errorf("Ecommerce input Artifact %s no longer matches its immutable reference", ref.RevisionID)
		}
		item := map[string]any{"artifactId": artifact.ID, "revisionId": revision.ID, "type": artifact.ArtifactType, "digest": revision.ContentDigest}
		if strings.TrimSpace(revision.ContentJSON) != "" {
			var content any
			if err := json.Unmarshal([]byte(revision.ContentJSON), &content); err != nil {
				return "", fmt.Errorf("decode Ecommerce input Artifact %s: %w", revision.ID, err)
			}
			item["content"] = content
		}
		if strings.TrimSpace(revision.ContentText) != "" {
			item["contentText"] = revision.ContentText
		}
		sourceArtifacts = append(sourceArtifacts, item)
	}
	for _, id := range assetIDs {
		sources = append(sources, id)
	}
	sort.Strings(sources)
	evidence := "unknown"
	if len(facts) > 0 || len(refs) > 0 || len(assetIDs) > 0 {
		evidence = "recorded"
	}
	content := map[string]any{
		"schemaVersion": 1, "agentId": ecommerceAgentRuntimeAgentID, "skillId": ecommerceAgentRuntimeSkillID,
		"evidence": evidence, "recordedFacts": facts, "sourceRefs": sources, "sourceArtifacts": sourceArtifacts,
		"productAssetIds": assetIDs,
		"mustPreserve":    []string{"overall silhouette and proportions", "material and surface finish", "recorded color", "package structure", "logo placement and readable text", "interfaces, fasteners and accessories"},
		"unknowns":        unknowns,
		"unknownPolicy":   "Unknown product claims, dimensions, ingredients and certifications must remain UNKNOWN; never invent them.",
	}
	if registry != nil {
		content["registry"] = map[string]any{"id": registry.ID, "version": registry.Version, "digest": registry.SourceDigest}
	}
	output := agentruntime.ExecutionOutput{SchemaVersion: 1, Summary: "已根据授权商品输入生成可追溯商品 DNA；无法确认的属性保留 UNKNOWN。", Artifacts: []agentruntime.ExecutionArtifactDraft{{Type: ecommerceAgentRuntimeOutputType, Title: "Product DNA", Content: content, ContentText: "Product DNA is deterministic and evidence-bound. Unknown commercial facts remain UNKNOWN."}}}
	return agentruntime.CanonicalExecutionOutputJSON(output)
}

func (s *Service) buildEcommerceProductDNAArtifactWrites(claim repository.AgentRuntimeExecutionClaim, output agentruntime.ExecutionOutput) ([]repository.ProductionArtifactWrite, error) {
	if len(output.Artifacts) != 1 || output.Artifacts[0].Type != ecommerceAgentRuntimeOutputType {
		return nil, errors.New("Ecommerce ProductDNA output contract is incomplete")
	}
	draft := output.Artifacts[0]
	logicalKey := "step:" + claim.Step.ID + ":" + ecommerceAgentRuntimeOutputType
	artifact := model.ProductionArtifact{ID: newID(), UserID: claim.Run.UserID, ProjectID: claim.Run.ProjectID, Domain: "ecommerce", ArtifactType: ecommerceAgentRuntimeOutputType, LogicalKey: logicalKey}
	expectedSequence := 0
	if existing, err := s.repo.ProductionArtifactByLogicalKey(claim.Run.UserID, claim.Run.ProjectID, "ecommerce", logicalKey); err == nil {
		artifact = *existing
		expectedSequence = existing.RevisionSequence
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	contentJSON, err := json.Marshal(map[string]any{"schemaVersion": 1, "artifactType": draft.Type, "title": strings.TrimSpace(draft.Title), "summary": output.Summary, "content": draft.Content})
	if err != nil {
		return nil, err
	}
	authorityRefs := []map[string]any{{"kind": "registry", "id": claim.Run.RegistryID, "version": claim.Run.RegistryVersion, "digest": claim.Run.RegistryDigest}, {"kind": "agent", "id": claim.Step.AgentID}, {"kind": "skill", "id": ecommerceAgentRuntimeSkillID, "version": "1.0.0"}}
	revision := model.ProductionArtifactRevision{ID: newID(), Status: model.ProductionArtifactStatusReview, ContentJSON: string(contentJSON), ContentText: draft.ContentText, ContentDigest: digestString(string(contentJSON) + "\n" + draft.ContentText), SourceRunID: claim.Run.ID, SourceStepID: claim.Step.ID, SourceAttemptID: claim.Attempt.ID, SourceArtifactRefsJSON: claim.Step.InputArtifactRefsJSON, AuthorityRefsJSON: mustEcommerceAgentRuntimeJSON(authorityRefs), CreatedByType: "agent", CreatedByID: claim.Step.AgentID}
	return []repository.ProductionArtifactWrite{{Artifact: artifact, Revision: revision, ExpectedSequence: expectedSequence}}, nil
}

func ecommerceInputStringSlice(input map[string]any, key string) []string {
	values := make([]string, 0)
	if raw, ok := input[key].([]any); ok {
		for _, item := range raw {
			if value := strings.TrimSpace(fmt.Sprint(item)); value != "" {
				values = append(values, value)
			}
		}
	} else if raw, ok := input[key].([]string); ok {
		values = append(values, raw...)
	} else if raw, ok := input[key].(string); ok && strings.TrimSpace(raw) != "" {
		values = append(values, raw)
	}
	return uniqueNonEmpty(values)
}

func ecommerceInputMap(input map[string]any, key string) map[string]any {
	if value, ok := input[key].(map[string]any); ok {
		return value
	}
	return map[string]any{}
}

func digestEcommerceAgentRuntimeValue(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func mustEcommerceAgentRuntimeJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}
