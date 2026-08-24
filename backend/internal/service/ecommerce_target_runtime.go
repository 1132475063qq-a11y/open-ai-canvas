package service

// Ecommerce target runtime is intentionally separate from the Film AgentTeam.
// It compiles the Ecommerce Skill contract into the generic Agent Runtime
// tables, but it never invokes a Provider and it never manufactures media.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"infinite-canvas/backend/internal/agentruntime"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/gorm"
)

const (
	EcommerceAgentRegistryID      = "ecommerce-agent-team"
	EcommerceAgentRegistryVersion = "1.0.0"
	EcommerceTargetDomain         = "ecommerce"

	EcommerceTargetArtifactType = "ecommerce-target"
	EcommerceTargetRequestType  = "ecommerce-target-request"
	EcommerceTargetPlanType     = "ecommerce-target-plan"
)

// EcommerceSkillContract is the server-side projection of the independent
// Ecommerce Skills. It deliberately contains no Film Agent or Film Artifact.
type EcommerceSkillContract struct {
	ID          string   `json:"id"`
	Version     int      `json:"version"`
	Family      string   `json:"family"`
	Mode        string   `json:"mode"`
	OwnerAgent  string   `json:"ownerAgent"`
	Inputs      []string `json:"inputs"`
	Outputs     []string `json:"outputs"`
	SourceAsset bool     `json:"sourceAsset"`
}

type EcommerceTargetRegistry struct {
	ID            string                                  `json:"id"`
	Version       string                                  `json:"version"`
	Domain        string                                  `json:"domain"`
	SourceDigest  string                                  `json:"sourceDigest"`
	Skills        []EcommerceSkillContract                `json:"skills"`
	IntentRoutes  []agentruntime.IntentRouteDefinition    `json:"intentRoutes"`
	HandoffRoutes []agentruntime.HandoffRouteDefinition   `json:"handoffRoutes"`
}

type EcommerceTargetArtifactRef struct {
	ArtifactID string `json:"artifactId"`
	RevisionID string `json:"revisionId"`
	Type       string `json:"type"`
	Version    int    `json:"version"`
	Digest     string `json:"digest"`
	Status     string `json:"status"`
}

type CreateEcommerceTargetRunRequest struct {
	IdempotencyKey         string         `json:"idempotencyKey"`
	Objective              string         `json:"objective"`
	IntentRouteID          string         `json:"intentRouteId,omitempty"`
	CanvasID               string         `json:"canvasId,omitempty"`
	Input                  map[string]any `json:"input,omitempty"`
	InputArtifactRevisionIDs []string     `json:"inputArtifactRevisionIds,omitempty"`
	ReviewBeforeExecution  bool           `json:"reviewBeforeExecution"`
}

type EcommerceTargetRunCreateResult struct {
	Detail     repository.AgentRuntimeDetail `json:"detail"`
	Idempotent bool                          `json:"idempotent"`
	Registry   EcommerceTargetRegistry      `json:"registry"`
}

var ecommerceTargetRegistry = buildEcommerceTargetRegistry()

func buildEcommerceTargetRegistry() EcommerceTargetRegistry {
	skills := []EcommerceSkillContract{
		{ID: "product-dna", Version: 1, Family: "PRODUCT_INTELLIGENCE", Mode: "STILL_LIFE", OwnerAgent: "ecommerce_product_intelligence", Inputs: []string{"product_upload"}, Outputs: []string{EcommerceArtifactTypeProductDNA}, SourceAsset: true},
		{ID: "creative-direction", Version: 1, Family: "CREATIVE_DIRECTION", Mode: "STILL_LIFE", OwnerAgent: "ecommerce_creative_director", Inputs: []string{EcommerceArtifactTypeProductDNA}, Outputs: []string{EcommerceArtifactTypeCreativeDirection}},
		{ID: "scene-planning", Version: 1, Family: "SCENE_PLANNING", Mode: "STILL_LIFE", OwnerAgent: "ecommerce_scene_director", Inputs: []string{EcommerceArtifactTypeProductDNA, EcommerceArtifactTypeCreativeDirection}, Outputs: []string{EcommerceArtifactTypeScenePlan}},
		{ID: "shot-planning", Version: 1, Family: "SHOT_PLANNING", Mode: "STILL_LIFE", OwnerAgent: "ecommerce_creative_director", Inputs: []string{EcommerceArtifactTypeCreativeDirection, EcommerceArtifactTypeScenePlan}, Outputs: []string{EcommerceArtifactTypeCreativeShotPlan}},
		{ID: "commercial-qa", Version: 1, Family: "COMMERCIAL_QA", Mode: "STILL_LIFE", OwnerAgent: "ecommerce_quality_controller", Inputs: []string{EcommerceArtifactTypeGenerationJob}, Outputs: []string{EcommerceArtifactTypeQAReport}},
	}
	intentRoutes := []agentruntime.IntentRouteDefinition{
		{ID: "IR-01", Name: "Product DNA", TriggerPhrases: []string{"product facts", "商品事实", "商品 DNA"}, PrimaryAgentID: "ecommerce_product_intelligence", SkillIDs: []string{"product-dna"}, RequiredInputArtifactTypes: []string{"product_upload"}, OutputArtifactTypes: []string{EcommerceArtifactTypeProductDNA}},
		{ID: "IR-02", Name: "Creative Direction", TriggerPhrases: []string{"creative direction", "创意方向", "商业创意"}, PrimaryAgentID: "ecommerce_creative_director", SkillIDs: []string{"creative-direction"}, RequiredInputArtifactTypes: []string{EcommerceArtifactTypeProductDNA}, OutputArtifactTypes: []string{EcommerceArtifactTypeCreativeDirection}},
		{ID: "IR-03", Name: "Scene Plan", TriggerPhrases: []string{"scene plan", "场景规划", "生活方式场景"}, PrimaryAgentID: "ecommerce_scene_director", SkillIDs: []string{"scene-planning"}, RequiredInputArtifactTypes: []string{EcommerceArtifactTypeProductDNA, EcommerceArtifactTypeCreativeDirection}, OutputArtifactTypes: []string{EcommerceArtifactTypeScenePlan}},
		{ID: "IR-04", Name: "Shot Plan", TriggerPhrases: []string{"shot plan", "镜头计划", "套图镜头"}, PrimaryAgentID: "ecommerce_creative_director", SkillIDs: []string{"shot-planning"}, RequiredInputArtifactTypes: []string{EcommerceArtifactTypeCreativeDirection, EcommerceArtifactTypeScenePlan}, OutputArtifactTypes: []string{EcommerceArtifactTypeCreativeShotPlan}},
		{ID: "IR-05", Name: "Commercial QA", TriggerPhrases: []string{"commercial QA", "商业质检", "电商质量"}, PrimaryAgentID: "ecommerce_quality_controller", SkillIDs: []string{"commercial-qa"}, RequiredInputArtifactTypes: []string{EcommerceArtifactTypeGenerationJob}, OutputArtifactTypes: []string{EcommerceArtifactTypeQAReport}},
	}
	handoffRoutes := []agentruntime.HandoffRouteDefinition{
		{ID: "HR-01", Name: "Product to Creative", FromAgentIDs: []string{"ecommerce_product_intelligence"}, ToAgentIDs: []string{"ecommerce_creative_director"}, SkillIDs: []string{"creative-direction"}, InputArtifactTypes: []string{EcommerceArtifactTypeProductDNA}, RequiredInputArtifactGroups: [][]string{{EcommerceArtifactTypeProductDNA}}, InputResolutionMode: "static", OutputArtifactTypes: []string{EcommerceArtifactTypeCreativeDirection}, MessageType: "completion_notification", ExecutionMode: "agent", RequiresLockedInput: true, Fanout: "single"},
		{ID: "HR-02", Name: "Creative to Scene", FromAgentIDs: []string{"ecommerce_creative_director"}, ToAgentIDs: []string{"ecommerce_scene_director"}, SkillIDs: []string{"scene-planning"}, InputArtifactTypes: []string{EcommerceArtifactTypeProductDNA, EcommerceArtifactTypeCreativeDirection}, RequiredInputArtifactGroups: [][]string{{EcommerceArtifactTypeProductDNA}, {EcommerceArtifactTypeCreativeDirection}}, InputResolutionMode: "static", OutputArtifactTypes: []string{EcommerceArtifactTypeScenePlan}, MessageType: "request", ExecutionMode: "agent", RequiresLockedInput: true, Fanout: "single"},
		{ID: "HR-03", Name: "Scene to Shot Plan", FromAgentIDs: []string{"ecommerce_scene_director"}, ToAgentIDs: []string{"ecommerce_creative_director"}, SkillIDs: []string{"shot-planning"}, InputArtifactTypes: []string{EcommerceArtifactTypeCreativeDirection, EcommerceArtifactTypeScenePlan}, RequiredInputArtifactGroups: [][]string{{EcommerceArtifactTypeCreativeDirection}, {EcommerceArtifactTypeScenePlan}}, InputResolutionMode: "static", OutputArtifactTypes: []string{EcommerceArtifactTypeCreativeShotPlan}, MessageType: "request", ExecutionMode: "agent", RequiresLockedInput: true, Fanout: "single"},
		{ID: "HR-04", Name: "Shot Plan to Generation", FromAgentIDs: []string{"ecommerce_creative_director"}, ToAgentIDs: []string{"ecommerce_generation_orchestrator"}, SkillIDs: []string{"shot-planning"}, InputArtifactTypes: []string{EcommerceArtifactTypeCreativeShotPlan}, RequiredInputArtifactGroups: [][]string{{EcommerceArtifactTypeCreativeShotPlan}}, InputResolutionMode: "static", OutputArtifactTypes: []string{EcommerceArtifactTypeGenerationJob}, MessageType: "request", ExecutionMode: "agent", RequiresLockedInput: true, Fanout: "single"},
		{ID: "HR-05", Name: "Generation to Commercial QA", FromAgentIDs: []string{"ecommerce_generation_orchestrator"}, ToAgentIDs: []string{"ecommerce_quality_controller"}, SkillIDs: []string{"commercial-qa"}, InputArtifactTypes: []string{EcommerceArtifactTypeGenerationJob}, RequiredInputArtifactGroups: [][]string{{EcommerceArtifactTypeGenerationJob}}, InputResolutionMode: "static", OutputArtifactTypes: []string{EcommerceArtifactTypeQAReport}, MessageType: "completion_notification", ExecutionMode: "agent", RequiresLockedInput: true, Fanout: "single"},
	}
	canonical := struct {
		ID string `json:"id"`; Version string `json:"version"`; Domain string `json:"domain"`
		Skills []EcommerceSkillContract `json:"skills"`; Intents []agentruntime.IntentRouteDefinition `json:"intents"`; Handoffs []agentruntime.HandoffRouteDefinition `json:"handoffs"`
	}{EcommerceAgentRegistryID, EcommerceAgentRegistryVersion, EcommerceTargetDomain, skills, intentRoutes, handoffRoutes}
	encoded, _ := json.Marshal(canonical)
	sum := sha256.Sum256(encoded)
	return EcommerceTargetRegistry{ID: EcommerceAgentRegistryID, Version: EcommerceAgentRegistryVersion, Domain: EcommerceTargetDomain, SourceDigest: hex.EncodeToString(sum[:]), Skills: skills, IntentRoutes: intentRoutes, HandoffRoutes: handoffRoutes}
}

func EcommerceTargetRuntimeRegistry() EcommerceTargetRegistry {
	result := ecommerceTargetRegistry
	result.Skills = append([]EcommerceSkillContract(nil), ecommerceTargetRegistry.Skills...)
	result.IntentRoutes = append([]agentruntime.IntentRouteDefinition(nil), ecommerceTargetRegistry.IntentRoutes...)
	result.HandoffRoutes = append([]agentruntime.HandoffRouteDefinition(nil), ecommerceTargetRegistry.HandoffRoutes...)
	return result
}

func (s *Service) EcommerceTargetRuntimeRegistry() (EcommerceTargetRegistry, error) {
	if err := validateEcommerceTargetRegistry(); err != nil {
		return EcommerceTargetRegistry{}, err
	}
	return EcommerceTargetRuntimeRegistry(), nil
}

func validateEcommerceTargetRegistry() error {
	if ecommerceTargetRegistry.ID == "" || ecommerceTargetRegistry.Version == "" || ecommerceTargetRegistry.Domain != EcommerceTargetDomain || len(ecommerceTargetRegistry.IntentRoutes) != 5 || len(ecommerceTargetRegistry.HandoffRoutes) != 5 || len(ecommerceTargetRegistry.Skills) != 5 || len(ecommerceTargetRegistry.SourceDigest) != 64 {
		return errors.New("Ecommerce target registry contract is incomplete")
	}
	seen := map[string]bool{}
	for _, route := range ecommerceTargetRegistry.IntentRoutes {
		if seen[route.ID] || len(route.SkillIDs) == 0 || len(route.OutputArtifactTypes) == 0 {
			return fmt.Errorf("Ecommerce Intent contract %s is invalid", route.ID)
		}
		seen[route.ID] = true
	}
	seen = map[string]bool{}
	for _, route := range ecommerceTargetRegistry.HandoffRoutes {
		if seen[route.ID] || !route.RequiresLockedInput || len(route.RequiredInputArtifactGroups) == 0 {
			return fmt.Errorf("Ecommerce Handoff contract %s is invalid", route.ID)
		}
		seen[route.ID] = true
	}
	return nil
}

func ecommerceTargetIntent(routeID string) (agentruntime.IntentRouteDefinition, bool) {
	for _, route := range ecommerceTargetRegistry.IntentRoutes {
		if route.ID == strings.ToUpper(strings.TrimSpace(routeID)) {
			return route, true
		}
	}
	return agentruntime.IntentRouteDefinition{}, false
}

func ecommerceTargetHandoff(routeID string) (agentruntime.HandoffRouteDefinition, bool) {
	for _, route := range ecommerceTargetRegistry.HandoffRoutes {
		if route.ID == strings.ToUpper(strings.TrimSpace(routeID)) {
			return route, true
		}
	}
	return agentruntime.HandoffRouteDefinition{}, false
}

// CompileEcommerceTargetDAG compiles the complete five-Intent/five-Handoff
// chain into one root Run. Handoff steps are explicit so the persisted graph
// remains auditable even when no Provider is configured.
func CompileEcommerceTargetDAG(runID string, inputRefs []EcommerceTargetArtifactRef, review bool, at time.Time) ([]model.AgentRuntimeStep, error) {
	if strings.TrimSpace(runID) == "" {
		return nil, errors.New("Ecommerce target Run ID is required")
	}
	if err := validateEcommerceTargetRegistry(); err != nil {
		return nil, err
	}
	refsJSON, err := json.Marshal(inputRefs)
	if err != nil {
		return nil, err
	}
	steps := make([]model.AgentRuntimeStep, 0, 10)
	var previous string
	now := at.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	for index := 0; index < 5; index++ {
		intent := ecommerceTargetRegistry.IntentRoutes[index]
		intentID := newID()
		status := model.AgentStepStatusPlanned
		if index == 0 {
			status = model.AgentStepStatusReady
			if review {
				status = model.AgentStepStatusAwaitingHuman
			}
		}
		deps := []string{}
		if previous != "" {
			deps = []string{previous}
		}
		steps = append(steps, model.AgentRuntimeStep{ID: intentID, RunID: runID, StepKey: "intent:" + intent.ID, Position: len(steps), RouteKind: "intent", RouteID: intent.ID, AgentID: intent.PrimaryAgentID, SkillIDsJSON: mustEcommerceJSON(intent.SkillIDs), Status: status, DependsOnStepIDsJSON: mustEcommerceJSON(deps), InputArtifactRefsJSON: string(refsJSON), ExpectedOutputArtifactTypesJSON: mustEcommerceJSON(intent.OutputArtifactTypes), OutputArtifactRefsJSON: "[]", Revision: 1, CreatedAt: now, UpdatedAt: now})
		previous = intentID
		handoff := ecommerceTargetRegistry.HandoffRoutes[index]
		handoffID := newID()
		deps = []string{previous}
		steps = append(steps, model.AgentRuntimeStep{ID: handoffID, RunID: runID, StepKey: "handoff:" + handoff.ID, Position: len(steps), RouteKind: "handoff", RouteID: handoff.ID, AgentID: handoff.ToAgentIDs[0], SkillIDsJSON: mustEcommerceJSON(handoff.SkillIDs), Status: model.AgentStepStatusPlanned, DependsOnStepIDsJSON: mustEcommerceJSON(deps), InputArtifactRefsJSON: string(refsJSON), ExpectedOutputArtifactTypesJSON: mustEcommerceJSON(handoff.OutputArtifactTypes), OutputArtifactRefsJSON: "[]", Revision: 1, CreatedAt: now, UpdatedAt: now})
		previous = handoffID
	}
	return steps, nil
}

func (s *Service) CreateProjectEcommerceTargetRun(userID string, projectID string, request CreateEcommerceTargetRunRequest) (EcommerceTargetRunCreateResult, error) {
	if err := validateEcommerceTargetRegistry(); err != nil {
		return EcommerceTargetRunCreateResult{}, err
	}
	project, err := s.requireEcommerceProject(userID, projectID)
	if err != nil {
		return EcommerceTargetRunCreateResult{}, err
	}
	key := strings.TrimSpace(request.IdempotencyKey)
	if len(key) < 8 || len(key) > 128 {
		return EcommerceTargetRunCreateResult{}, BadAuthRequest("Ecommerce target Run 幂等键必须为 8-128 位")
	}
	objective := strings.TrimSpace(request.Objective)
	if objective == "" || len([]rune(objective)) > 4000 {
		return EcommerceTargetRunCreateResult{}, BadAuthRequest("Ecommerce target 目标不能为空且不能超过 4000 字")
	}
	input, err := normalizeEcommerceTargetInput(request.Input)
	if err != nil {
		return EcommerceTargetRunCreateResult{}, err
	}
	refs, err := s.resolveEcommerceTargetArtifactRefs(userID, project.ID, request.InputArtifactRevisionIDs)
	if err != nil {
		return EcommerceTargetRunCreateResult{}, err
	}
	if !ecommerceTargetHasProductInput(refs) {
		return EcommerceTargetRunCreateResult{}, BadAuthRequest("Ecommerce target Run 必须引用至少一个已锁定的 product_upload Artifact")
	}
	requestDigest := ecommerceTargetRequestDigest(project.ID, objective, request.IntentRouteID, input, refs, request.ReviewBeforeExecution)
	if existing, lookupErr := s.repo.AgentRuntimeRunByIdempotency(userID, key); lookupErr == nil {
		if existing.ProjectID != project.ID || existing.Domain != EcommerceTargetDomain || ecommerceTargetRunRequestDigest(existing.InputJSON) != requestDigest {
			return EcommerceTargetRunCreateResult{}, Conflict("该幂等键已用于另一项 Ecommerce target 请求")
		}
		detail, detailErr := s.repo.AgentRuntimeDetailForUser(userID, existing.ID)
		return EcommerceTargetRunCreateResult{Detail: detail, Idempotent: true, Registry: EcommerceTargetRuntimeRegistry()}, detailErr
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return EcommerceTargetRunCreateResult{}, lookupErr
	}

	now := time.Now().UTC()
	runID := newID()
	steps, err := CompileEcommerceTargetDAG(runID, refs, request.ReviewBeforeExecution, now)
	if err != nil {
		return EcommerceTargetRunCreateResult{}, err
	}
	inputRefsJSON := mustEcommerceJSON(refs)
	runInput, _ := json.Marshal(map[string]any{"schemaVersion": 1, "requestDigest": requestDigest, "input": input, "inputArtifactRefs": refs, "reviewBeforeExecution": request.ReviewBeforeExecution, "registry": map[string]string{"id": ecommerceTargetRegistry.ID, "version": ecommerceTargetRegistry.Version, "digest": ecommerceTargetRegistry.SourceDigest}})
	status := model.AgentRunStatusReady
	if request.ReviewBeforeExecution {
		status = model.AgentRunStatusAwaitingHuman
	}
	run := &model.AgentRuntimeRun{ID: runID, UserID: userID, ProjectID: project.ID, CanvasID: strings.TrimSpace(request.CanvasID), Domain: EcommerceTargetDomain, RegistryID: ecommerceTargetRegistry.ID, RegistryVersion: ecommerceTargetRegistry.Version, RegistryDigest: ecommerceTargetRegistry.SourceDigest, RouteKind: "intent", IntentRouteID: "IR-01", RootRunID: runID, Status: status, Objective: objective, InputJSON: string(runInput), CurrentStepID: steps[0].ID, IdempotencyKey: key, Revision: 1, EventSequence: 2, CreatedAt: now, UpdatedAt: now}
	routing := &model.AgentRoutingDecision{ID: newID(), RunID: runID, RouteKind: "intent", RouteID: "IR-01", IntentRouteID: "IR-01", SelectedAgentID: steps[0].AgentID, SelectedSkillIDsJSON: steps[0].SkillIDsJSON, InputArtifactRefsJSON: inputRefsJSON, AlternativesJSON: "[]", Reason: "Ecommerce Full Production Chain v1 deterministic target contract", Confidence: "deterministic_match", DecidedByType: "runtime", DecidedByID: ecommerceTargetRegistry.ID, CreatedAt: now}
	events := []model.AgentRuntimeEvent{{ID: newID(), UserID: userID, RunID: runID, Sequence: 1, EventType: "run.created", ActorType: "user", ActorID: userID, ToStatus: string(model.AgentRunStatusPlanning), PayloadJSON: mustEcommerceJSON(map[string]any{"domain": EcommerceTargetDomain, "registryDigest": ecommerceTargetRegistry.SourceDigest}), CreatedAt: now}, {ID: newID(), UserID: userID, RunID: runID, Sequence: 2, StepID: steps[0].ID, EventType: "run.planned", ActorType: "runtime", ActorID: ecommerceTargetRegistry.ID, FromStatus: string(model.AgentRunStatusPlanning), ToStatus: string(status), PayloadJSON: mustEcommerceJSON(map[string]any{"intentRoutes": 5, "handoffRoutes": 5, "inputArtifactRefs": refs}), CreatedAt: now}}
	var decision *model.AgentHumanDecision
	if request.ReviewBeforeExecution {
		decision = &model.AgentHumanDecision{ID: newID(), RunID: runID, StepID: steps[0].ID, Status: model.AgentHumanDecisionStatusPending, Question: "确认按 Ecommerce Full Production Chain v1 编译并执行？", OptionsJSON: mustEcommerceJSON([]map[string]string{{"id": "approve", "label": "确认执行"}, {"id": "cancel", "label": "取消任务"}}), Recommendation: "approve", ImpactRefsJSON: inputRefsJSON, Revision: 1, CreatedAt: now, UpdatedAt: now}
	}
	if err := s.repo.CreateAgentRuntimeBundle(repository.AgentRuntimeCreateBundle{Run: run, RoutingDecision: routing, Steps: steps, HumanDecision: decision, Events: events}); err != nil {
		if existing, lookupErr := s.repo.AgentRuntimeRunByIdempotency(userID, key); lookupErr == nil && existing.ProjectID == project.ID && ecommerceTargetRunRequestDigest(existing.InputJSON) == requestDigest {
			detail, detailErr := s.repo.AgentRuntimeDetailForUser(userID, existing.ID)
			return EcommerceTargetRunCreateResult{Detail: detail, Idempotent: true, Registry: EcommerceTargetRuntimeRegistry()}, detailErr
		}
		return EcommerceTargetRunCreateResult{}, err
	}
	detail, err := s.repo.AgentRuntimeDetailForUser(userID, runID)
	return EcommerceTargetRunCreateResult{Detail: detail, Registry: EcommerceTargetRuntimeRegistry()}, err
}

func ecommerceTargetHasProductInput(refs []EcommerceTargetArtifactRef) bool {
	for _, ref := range refs {
		if ref.Status == string(model.ProductionArtifactStatusLocked) && (ref.Type == EcommerceArtifactTypeProductUpload || ref.Type == "product-upload" || strings.HasPrefix(ref.Type, "ecommerce-target")) {
			return true
		}
	}
	return false
}

func (s *Service) EcommerceTargetRunDetail(userID string, projectID string, runID string) (repository.AgentRuntimeDetail, error) {
	if _, err := s.ecommerceProjectForRead(userID, projectID); err != nil {
		return repository.AgentRuntimeDetail{}, err
	}
	detail, err := s.repo.AgentRuntimeDetailForUser(userID, strings.TrimSpace(runID))
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && (detail.Run.ProjectID != projectID || detail.Run.Domain != EcommerceTargetDomain)) {
		return repository.AgentRuntimeDetail{}, NotFound("Ecommerce target Run 不存在")
	}
	return detail, err
}

func (s *Service) ListEcommerceTargetRuns(userID string, projectID string, limit int) ([]model.AgentRuntimeRun, error) {
	if _, err := s.ecommerceProjectForRead(userID, projectID); err != nil {
		return nil, err
	}
	return s.repo.ProjectAgentRuntimeRunsForDomain(userID, projectID, EcommerceTargetDomain, limit)
}

func (s *Service) LockEcommerceTargetArtifact(userID string, projectID string, runID string, artifactID string, expectedRunRevision int64, expectedArtifactSequence int, expectedRevisionID string) (*repository.ProductionArtifactLockResult, error) {
	if _, err := s.requireEcommerceProject(userID, projectID); err != nil {
		return nil, err
	}
	detail, err := s.EcommerceTargetRunDetail(userID, projectID, runID)
	if err != nil {
		return nil, err
	}
	artifact, err := s.repo.ProductionArtifactForUser(userID, strings.TrimSpace(artifactID))
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && (artifact.ProjectID != projectID || artifact.Domain != EcommerceTargetDomain)) {
		return nil, NotFound("Ecommerce target Artifact 不存在")
	}
	if err != nil {
		return nil, err
	}
	return s.repo.LockProductionArtifactForHandoff(repository.ProductionArtifactLockCommand{UserID: userID, ProjectID: projectID, Domain: EcommerceTargetDomain, RunID: detail.Run.ID, ArtifactID: artifact.ID, ExpectedRunRevision: expectedRunRevision, ExpectedArtifactSequence: expectedArtifactSequence, ExpectedRevisionID: strings.TrimSpace(expectedRevisionID), LockedRevisionID: newID(), TriggerID: newID(), Event: repository.AgentRuntimeEventInput{ID: newID(), EventType: "artifact.locked", ActorType: "user", ActorID: userID}, At: time.Now().UTC()})
}

func (s *Service) resolveEcommerceTargetArtifactRefs(userID string, projectID string, revisionIDs []string) ([]EcommerceTargetArtifactRef, error) {
	if len(revisionIDs) > 50 {
		return nil, BadAuthRequest("单个 Ecommerce target Run 最多引用 50 个 Artifact revision")
	}
	refs := make([]EcommerceTargetArtifactRef, 0, len(revisionIDs))
	seen := map[string]bool{}
	for _, id := range revisionIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if seen[id] {
			return nil, BadAuthRequest("Ecommerce target 输入存在重复 Artifact revision")
		}
		seen[id] = true
		artifact, revision, err := s.repo.ProductionArtifactRevisionForUser(userID, id)
		if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && (artifact.ProjectID != projectID || artifact.Domain != EcommerceTargetDomain)) {
			return nil, NotFound("Ecommerce target 输入 Artifact revision 不存在或不属于该项目")
		}
		if err != nil {
			return nil, err
		}
		if revision.Status != model.ProductionArtifactStatusLocked {
			return nil, Conflict("Ecommerce target 输入 Artifact 必须先锁定")
		}
		refs = append(refs, EcommerceTargetArtifactRef{ArtifactID: artifact.ID, RevisionID: revision.ID, Type: artifact.ArtifactType, Version: revision.Version, Digest: revision.ContentDigest, Status: string(revision.Status)})
	}
	sort.SliceStable(refs, func(i, j int) bool { return refs[i].RevisionID < refs[j].RevisionID })
	return refs, nil
}

func normalizeEcommerceTargetInput(input map[string]any) (map[string]any, error) {
	if input == nil {
		return map[string]any{}, nil
	}
	encoded, err := json.Marshal(input)
	if err != nil || len(encoded) > 256<<10 {
		return nil, BadAuthRequest("Ecommerce target 输入格式无效或超过 256KB")
	}
	var normalized map[string]any
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		return nil, BadAuthRequest("Ecommerce target 输入格式无效")
	}
	if containsInlineMediaDataURL(normalized) || containsAgentRuntimeSecret(normalized) {
		return nil, BadAuthRequest("Ecommerce target 输入不能包含内嵌媒体或 Provider 凭据")
	}
	return normalized, nil
}

func ecommerceTargetRequestDigest(projectID, objective, routeID string, input map[string]any, refs []EcommerceTargetArtifactRef, review bool) string {
	encoded, _ := json.Marshal(map[string]any{"projectId": projectID, "objective": objective, "intentRouteId": strings.ToUpper(strings.TrimSpace(routeID)), "input": input, "inputArtifactRefs": refs, "reviewBeforeExecution": review, "registry": ecommerceTargetRegistry.SourceDigest})
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func ecommerceTargetRunRequestDigest(inputJSON string) string {
	var envelope struct{ RequestDigest string `json:"requestDigest"` }
	_ = json.Unmarshal([]byte(inputJSON), &envelope)
	return envelope.RequestDigest
}

func mustEcommerceJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}
