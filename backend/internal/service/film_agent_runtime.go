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

const maxFilmAgentRunInputBytes = 256 << 10

var filmAgentIdempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,128}$`)

type CreateFilmAgentRunRequest struct {
	CanvasID                 string         `json:"canvasId"`
	Objective                string         `json:"objective"`
	IntentRouteID            string         `json:"intentRouteId"`
	AgentID                  string         `json:"agentId"`
	Input                    map[string]any `json:"input"`
	InputArtifactRevisionIDs []string       `json:"inputArtifactRevisionIds"`
	ReviewBeforeExecution    bool           `json:"reviewBeforeExecution"`
}

type ResolveFilmAgentDecisionRequest struct {
	ExpectedRunRevision      int64          `json:"expectedRunRevision"`
	ExpectedStepRevision     int64          `json:"expectedStepRevision"`
	ExpectedDecisionRevision int64          `json:"expectedDecisionRevision"`
	Action                   string         `json:"action"`
	Response                 map[string]any `json:"response"`
}

type RetryFilmAgentStepRequest struct {
	ExpectedRunRevision  int64  `json:"expectedRunRevision"`
	ExpectedStepRevision int64  `json:"expectedStepRevision"`
	Reason               string `json:"reason"`
}

type FilmProductionArtifactRef struct {
	ArtifactID string `json:"artifactId"`
	RevisionID string `json:"revisionId"`
	Type       string `json:"type"`
	Version    int    `json:"version"`
	Digest     string `json:"digest"`
	Status     string `json:"status"`
}

type FilmAgentRunCreateResult struct {
	Detail     repository.AgentRuntimeDetail `json:"detail"`
	Idempotent bool                          `json:"idempotent"`
}

func (s *Service) CreateFilmAgentRun(userID string, projectID string, idempotencyKey string, request CreateFilmAgentRunRequest) (FilmAgentRunCreateResult, error) {
	if err := s.ValidateRuntime(); err != nil {
		return FilmAgentRunCreateResult{}, err
	}
	project, err := s.requireMutableFilmProject(userID, projectID)
	if err != nil {
		return FilmAgentRunCreateResult{}, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if !filmAgentIdempotencyKeyPattern.MatchString(idempotencyKey) {
		return FilmAgentRunCreateResult{}, BadAuthRequest("X-Idempotency-Key 必须为 8-128 位字母、数字或 ._:-")
	}
	objective := strings.TrimSpace(request.Objective)
	if objective == "" {
		return FilmAgentRunCreateResult{}, BadAuthRequest("请填写本次 Film Agent 任务目标")
	}
	if len([]rune(objective)) > 4000 {
		return FilmAgentRunCreateResult{}, BadAuthRequest("Film Agent 任务目标不能超过 4000 字")
	}
	if err := s.validateFilmCanvas(userID, project.ID, request.CanvasID); err != nil {
		return FilmAgentRunCreateResult{}, err
	}
	normalizedInput, err := normalizeFilmAgentInput(request.Input)
	if err != nil {
		return FilmAgentRunCreateResult{}, err
	}
	route, selectedAgentID, routeReason, routeConfidence, err := s.selectFilmIntentRoute(objective, request.IntentRouteID, request.AgentID)
	if err != nil {
		return FilmAgentRunCreateResult{}, err
	}
	requestDigest, err := digestFilmAgentRequest(project.ID, objective, route.ID, selectedAgentID, normalizedInput, request.InputArtifactRevisionIDs, request.ReviewBeforeExecution)
	if err != nil {
		return FilmAgentRunCreateResult{}, err
	}
	if existing, lookupErr := s.repo.AgentRuntimeRunByIdempotency(userID, idempotencyKey); lookupErr == nil {
		if existing.ProjectID != project.ID || existing.Domain != "film" || filmRunRequestDigest(existing.InputJSON) != requestDigest {
			return FilmAgentRunCreateResult{}, conflictError("该幂等键已用于另一项 Film Agent 请求")
		}
		detail, detailErr := s.repo.AgentRuntimeDetailForUser(userID, existing.ID)
		return FilmAgentRunCreateResult{Detail: detail, Idempotent: true}, detailErr
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return FilmAgentRunCreateResult{}, lookupErr
	}

	runID := newID()
	createdAt := time.Now().UTC()
	steps, err := s.compileFilmIntentSteps(runID, route, selectedAgentID, nil, request.ReviewBeforeExecution, createdAt)
	if err != nil {
		return FilmAgentRunCreateResult{}, err
	}
	projectRequirementArtifact, projectRequirementRevision, projectRequirementRef, err := buildFilmProjectRequirementsArtifact(userID, *project, runID, steps[0].ID, objective, normalizedInput, createdAt)
	if err != nil {
		return FilmAgentRunCreateResult{}, err
	}
	inputRefs, err := s.resolveFilmInputArtifactRefs(userID, project.ID, request.InputArtifactRevisionIDs)
	if err != nil {
		return FilmAgentRunCreateResult{}, err
	}
	inputRefs = append([]FilmProductionArtifactRef{projectRequirementRef}, inputRefs...)
	if err := validateFilmRouteInputs(route, inputRefs); err != nil {
		return FilmAgentRunCreateResult{}, err
	}
	inputRefsJSON, _ := json.Marshal(inputRefs)
	for index := range steps {
		steps[index].InputArtifactRefsJSON = string(inputRefsJSON)
	}
	runStatus := model.AgentRunStatusReady
	if request.ReviewBeforeExecution {
		runStatus = model.AgentRunStatusAwaitingHuman
	}
	runInputJSON, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "requestDigest": requestDigest, "input": normalizedInput,
		"inputArtifactRefs": inputRefs, "reviewBeforeExecution": request.ReviewBeforeExecution,
	})
	if err != nil {
		return FilmAgentRunCreateResult{}, err
	}
	run := &model.AgentRuntimeRun{
		ID: runID, UserID: userID, ProjectID: project.ID, CanvasID: strings.TrimSpace(request.CanvasID), Domain: "film",
		RegistryID: s.filmAgentRegistry.ID, RegistryVersion: s.filmAgentRegistry.Version, RegistryDigest: s.filmAgentRegistry.SourceDigest,
		IntentRouteID: route.ID, Status: runStatus, Objective: objective, InputJSON: string(runInputJSON),
		CurrentStepID: steps[0].ID, IdempotencyKey: idempotencyKey, Revision: 1, EventSequence: 2,
		CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	routing := &model.AgentRoutingDecision{
		ID: newID(), RunID: run.ID, IntentRouteID: route.ID, SelectedAgentID: selectedAgentID,
		SelectedSkillIDsJSON: mustFilmJSON(route.SkillIDs), InputArtifactRefsJSON: string(inputRefsJSON), AlternativesJSON: "[]",
		Reason: routeReason, Confidence: routeConfidence, DecidedByType: "runtime", DecidedByID: "film-router-v1", CreatedAt: createdAt,
	}
	events := []model.AgentRuntimeEvent{
		{ID: newID(), UserID: userID, RunID: run.ID, Sequence: 1, EventType: "run.created", ActorType: "user", ActorID: userID, ToStatus: string(model.AgentRunStatusPlanning), PayloadJSON: mustFilmJSON(map[string]any{"registryDigest": run.RegistryDigest}), CreatedAt: createdAt},
		{ID: newID(), UserID: userID, RunID: run.ID, Sequence: 2, StepID: steps[0].ID, EventType: "run.planned", ActorType: "runtime", ActorID: "film-router-v1", FromStatus: string(model.AgentRunStatusPlanning), ToStatus: string(runStatus), PayloadJSON: mustFilmJSON(map[string]any{"intentRouteId": route.ID, "agentId": selectedAgentID, "skillIds": route.SkillIDs}), CreatedAt: createdAt},
	}
	var decision *model.AgentHumanDecision
	if request.ReviewBeforeExecution {
		decision = &model.AgentHumanDecision{
			ID: newID(), RunID: run.ID, StepID: steps[0].ID, Status: model.AgentHumanDecisionStatusPending,
			Question:       fmt.Sprintf("确认按 %s 路由执行本次任务？", route.Name),
			OptionsJSON:    mustFilmJSON([]map[string]string{{"id": "approve", "label": "确认执行"}, {"id": "cancel", "label": "取消任务"}}),
			Recommendation: "approve", ResponseJSON: "", ImpactRefsJSON: string(inputRefsJSON), Revision: 1,
			CreatedAt: createdAt, UpdatedAt: createdAt,
		}
		events[1].EventType = "human_decision.requested"
		events[1].PayloadJSON = mustFilmJSON(map[string]any{"decisionId": decision.ID, "intentRouteId": route.ID})
	}
	bundle := repository.AgentRuntimeCreateBundle{
		Run: run, RoutingDecision: routing, Steps: steps, HumanDecision: decision, Events: events,
		Artifacts:         []model.ProductionArtifact{projectRequirementArtifact},
		ArtifactRevisions: []model.ProductionArtifactRevision{projectRequirementRevision},
	}
	if err := s.repo.CreateAgentRuntimeBundle(bundle); err != nil {
		if existing, lookupErr := s.repo.AgentRuntimeRunByIdempotency(userID, idempotencyKey); lookupErr == nil && existing.ProjectID == project.ID && filmRunRequestDigest(existing.InputJSON) == requestDigest {
			detail, detailErr := s.repo.AgentRuntimeDetailForUser(userID, existing.ID)
			return FilmAgentRunCreateResult{Detail: detail, Idempotent: true}, detailErr
		}
		return FilmAgentRunCreateResult{}, err
	}
	detail, err := s.repo.AgentRuntimeDetailForUser(userID, run.ID)
	return FilmAgentRunCreateResult{Detail: detail}, err
}

func (s *Service) ListFilmAgentRuns(userID string, projectID string, limit int) ([]model.AgentRuntimeRun, error) {
	if _, err := s.filmProjectForUser(userID, projectID); err != nil {
		return nil, err
	}
	return s.repo.ProjectAgentRuntimeRunsForDomain(userID, projectID, "film", limit)
}

func (s *Service) FilmAgentRunDetail(userID string, projectID string, runID string) (repository.AgentRuntimeDetail, error) {
	if _, err := s.filmProjectForUser(userID, projectID); err != nil {
		return repository.AgentRuntimeDetail{}, err
	}
	detail, err := s.repo.AgentRuntimeDetailForUser(userID, strings.TrimSpace(runID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return repository.AgentRuntimeDetail{}, NotFound("Film Agent Run 不存在")
	}
	if err != nil {
		return repository.AgentRuntimeDetail{}, err
	}
	if detail.Run.ProjectID != projectID || detail.Run.Domain != "film" {
		return repository.AgentRuntimeDetail{}, NotFound("Film Agent Run 不存在")
	}
	return detail, nil
}

func (s *Service) ResolveFilmAgentDecision(userID string, projectID string, runID string, decisionID string, request ResolveFilmAgentDecisionRequest) (repository.AgentRuntimeDetail, error) {
	if _, err := s.requireMutableFilmProject(userID, projectID); err != nil {
		return repository.AgentRuntimeDetail{}, err
	}
	detail, err := s.FilmAgentRunDetail(userID, projectID, runID)
	if err != nil {
		return repository.AgentRuntimeDetail{}, err
	}
	decision, ok := findFilmHumanDecision(detail.HumanDecisions, decisionID)
	if !ok {
		return repository.AgentRuntimeDetail{}, NotFound("待处理的人工作业决定不存在")
	}
	action := strings.ToLower(strings.TrimSpace(request.Action))
	if action != "approve" && action != "cancel" {
		return repository.AgentRuntimeDetail{}, BadAuthRequest("人工作业决定仅支持 approve 或 cancel")
	}
	response := request.Response
	if response == nil {
		response = map[string]any{}
	}
	response["action"] = action
	responseJSON, err := json.Marshal(response)
	if err != nil || len(responseJSON) > 64<<10 {
		return repository.AgentRuntimeDetail{}, BadAuthRequest("人工决定响应格式无效或超过 64KB")
	}
	runStatus := model.AgentRunStatusReady
	stepStatus := model.AgentStepStatusReady
	if action == "cancel" {
		runStatus = model.AgentRunStatusCancelled
		stepStatus = model.AgentStepStatusCancelled
	}
	_, _, _, err = s.repo.ResolveAgentRuntimeHumanDecision(repository.AgentRuntimeHumanResolve{
		UserID: userID, RunID: detail.Run.ID, DecisionID: decision.ID,
		ExpectedRunRevision: request.ExpectedRunRevision, ExpectedStepRevision: request.ExpectedStepRevision,
		ExpectedDecisionRevision: request.ExpectedDecisionRevision, RunStatus: runStatus, StepStatus: stepStatus,
		ResponseJSON: string(responseJSON),
		Event:        repository.AgentRuntimeEventInput{ID: newID(), EventType: "human_decision.resolved", ActorType: "user", ActorID: userID, PayloadJSON: mustFilmJSON(map[string]any{"decisionId": decision.ID, "action": action})},
		At:           time.Now().UTC(),
	})
	if err != nil {
		return repository.AgentRuntimeDetail{}, mapAgentRuntimeRepositoryError(err)
	}
	return s.FilmAgentRunDetail(userID, projectID, runID)
}

func (s *Service) RetryFilmAgentStep(userID string, projectID string, runID string, stepID string, request RetryFilmAgentStepRequest) (repository.AgentRuntimeDetail, error) {
	if _, err := s.requireMutableFilmProject(userID, projectID); err != nil {
		return repository.AgentRuntimeDetail{}, err
	}
	detail, err := s.FilmAgentRunDetail(userID, projectID, runID)
	if err != nil {
		return repository.AgentRuntimeDetail{}, err
	}
	step, ok := findFilmAgentStep(detail.Steps, stepID)
	if !ok {
		return repository.AgentRuntimeDetail{}, NotFound("Film Agent Step 不存在")
	}
	if step.Status != model.AgentStepStatusFailed {
		return repository.AgentRuntimeDetail{}, conflictError("只有失败的 Film Agent Step 才能重试")
	}
	reason := strings.TrimSpace(request.Reason)
	if len([]rune(reason)) > 1000 {
		return repository.AgentRuntimeDetail{}, BadAuthRequest("重试原因不能超过 1000 字")
	}
	requestJSON := mustFilmJSON(map[string]any{"reason": reason, "source": "manual_retry"})
	_, _, _, err = s.repo.CreateAgentRuntimeAttempt(repository.AgentRuntimeAttemptCreate{
		UserID: userID, RunID: detail.Run.ID, StepID: step.ID,
		ExpectedRunRevision: request.ExpectedRunRevision, ExpectedStepRevision: request.ExpectedStepRevision,
		Attempt: &model.AgentRuntimeAttempt{
			ID: newID(), Executor: "agent-runtime", InputDigest: digestString(step.InputArtifactRefsJSON),
			PromptDigest: digestString(step.AgentID + "\n" + step.SkillIDsJSON), RequestJSON: requestJSON,
		},
		Event: repository.AgentRuntimeEventInput{ID: newID(), EventType: "attempt.created", ActorType: "user", ActorID: userID, PayloadJSON: requestJSON},
		At:    time.Now().UTC(),
	})
	if err != nil {
		return repository.AgentRuntimeDetail{}, mapAgentRuntimeRepositoryError(err)
	}
	return s.FilmAgentRunDetail(userID, projectID, runID)
}

func (s *Service) filmProjectForUser(userID string, projectID string) (*model.Project, error) {
	project, err := s.repo.ProjectForUser(userID, strings.TrimSpace(projectID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NotFound("短剧项目不存在")
	}
	if err != nil {
		return nil, err
	}
	if project.Type != "short-drama" {
		return nil, NotFound("该项目不是短剧项目")
	}
	return project, nil
}

func (s *Service) requireMutableFilmProject(userID string, projectID string) (*model.Project, error) {
	project, err := s.filmProjectForUser(userID, projectID)
	if err != nil {
		return nil, err
	}
	if project.Status == model.ProjectStatusArchived {
		return nil, conflictError("项目已归档，不能创建或修改 Film Agent Run")
	}
	return project, nil
}

func (s *Service) validateFilmCanvas(userID string, projectID string, canvasID string) error {
	canvasID = strings.TrimSpace(canvasID)
	if canvasID == "" {
		return nil
	}
	canvas, err := s.repo.CanvasProjectForUser(userID, canvasID)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && canvas.ProjectID != projectID) {
		return NotFound("短剧画布不存在或不属于该项目")
	}
	return err
}

func normalizeFilmAgentInput(input map[string]any) (map[string]any, error) {
	if input == nil {
		return map[string]any{}, nil
	}
	encoded, err := json.Marshal(input)
	if err != nil || len(encoded) > maxFilmAgentRunInputBytes {
		return nil, BadAuthRequest("Film Agent 输入格式无效或超过 256KB")
	}
	var normalized map[string]any
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		return nil, BadAuthRequest("Film Agent 输入格式无效")
	}
	if containsInlineMediaDataURL(normalized) {
		return nil, BadAuthRequest("Film Agent 输入不能包含内嵌媒体，请引用已上传资源")
	}
	if containsAgentRuntimeSecret(normalized) {
		return nil, BadAuthRequest("Film Agent 输入不能包含 API Key、Token 或鉴权头")
	}
	return normalized, nil
}

func containsAgentRuntimeSecret(value any) bool {
	switch item := value.(type) {
	case map[string]any:
		for key, child := range item {
			normalizedKey := strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(key))
			switch normalizedKey {
			case "apikey", "secretkey", "accesskey", "authorization", "accesstoken", "refreshtoken", "bearertoken":
				return true
			}
			if containsAgentRuntimeSecret(child) {
				return true
			}
		}
	case []any:
		for _, child := range item {
			if containsAgentRuntimeSecret(child) {
				return true
			}
		}
	}
	return false
}

func (s *Service) selectFilmIntentRoute(objective string, routeID string, agentID string) (agentruntime.IntentRouteDefinition, string, string, string, error) {
	routeID = strings.ToUpper(strings.TrimSpace(routeID))
	var route agentruntime.IntentRouteDefinition
	var reason string
	confidence := "confirmed"
	if routeID != "" {
		var ok bool
		route, ok = s.filmAgentRegistry.IntentRoute(routeID)
		if !ok {
			return route, "", "", "", BadAuthRequest("未知的 Film Intent Route：" + routeID)
		}
		reason = "用户明确选择 " + route.ID
	} else {
		matches := make([]struct {
			route agentruntime.IntentRouteDefinition
			score int
		}, 0)
		normalizedObjective := strings.ToLower(strings.Join(strings.Fields(objective), ""))
		for _, candidate := range s.filmAgentRegistry.IntentRoutes {
			score := 0
			for _, phrase := range candidate.TriggerPhrases {
				normalizedPhrase := strings.ToLower(strings.Join(strings.Fields(phrase), ""))
				if strings.Contains(normalizedObjective, normalizedPhrase) {
					score += len([]rune(normalizedPhrase))
				}
			}
			if score > 0 {
				matches = append(matches, struct {
					route agentruntime.IntentRouteDefinition
					score int
				}{route: candidate, score: score})
			}
		}
		sort.SliceStable(matches, func(i, j int) bool { return matches[i].score > matches[j].score })
		if len(matches) == 0 || (len(matches) > 1 && matches[0].score == matches[1].score) {
			return route, "", "", "", BadAuthRequest("无法可靠判断 Film Intent，请明确选择 IR-01 至 IR-15")
		}
		route = matches[0].route
		reason = "命中注册表触发语句，自动选择 " + route.ID
		confidence = "deterministic_match"
	}
	selectedAgentID := strings.TrimSpace(agentID)
	if selectedAgentID == "" {
		if route.RequiresDisambiguation {
			return route, "", "", "", BadAuthRequest(fmt.Sprintf("%s 需要在候选 Agent 中明确选择：%s", route.ID, strings.Join(route.CandidateAgentIDs, "、")))
		}
		selectedAgentID = route.PrimaryAgentID
	} else if selectedAgentID != route.PrimaryAgentID && !filmContainsString(route.CandidateAgentIDs, selectedAgentID) {
		return route, "", "", "", BadAuthRequest("所选 Agent 不属于 " + route.ID + " 的候选范围")
	}
	return route, selectedAgentID, reason, confidence, nil
}

func (s *Service) compileFilmIntentSteps(runID string, route agentruntime.IntentRouteDefinition, selectedAgentID string, inputRefs []FilmProductionArtifactRef, review bool, at time.Time) ([]model.AgentRuntimeStep, error) {
	inputRefsJSON := mustFilmJSON(inputRefs)
	steps := make([]model.AgentRuntimeStep, 0, len(route.SkillIDs))
	for index, skillID := range route.SkillIDs {
		agentID, err := s.filmSkillOwner(route, skillID, selectedAgentID)
		if err != nil {
			return nil, err
		}
		status := model.AgentStepStatusPlanned
		if index == 0 {
			status = model.AgentStepStatusReady
			if review {
				status = model.AgentStepStatusAwaitingHuman
			}
		}
		dependencies := []string{}
		if index > 0 {
			dependencies = []string{steps[index-1].ID}
		}
		expectedOutputs := []string{}
		if index == len(route.SkillIDs)-1 {
			expectedOutputs = route.OutputArtifactTypes
		}
		steps = append(steps, model.AgentRuntimeStep{
			ID: newID(), RunID: runID, StepKey: fmt.Sprintf("intent:%s:%s", route.ID, skillID), Position: index,
			RouteKind: "intent", RouteID: route.ID, AgentID: agentID, SkillIDsJSON: mustFilmJSON([]string{skillID}), Status: status,
			DependsOnStepIDsJSON: mustFilmJSON(dependencies), InputArtifactRefsJSON: inputRefsJSON,
			ExpectedOutputArtifactTypesJSON: mustFilmJSON(expectedOutputs), OutputArtifactRefsJSON: "[]",
			Revision: 1, CreatedAt: at, UpdatedAt: at,
		})
	}
	if len(steps) == 0 {
		return nil, errors.New("Film Intent Route 没有可执行 Skill")
	}
	return steps, nil
}

func (s *Service) filmSkillOwner(route agentruntime.IntentRouteDefinition, skillID string, selectedAgentID string) (string, error) {
	skill, ok := s.filmAgentRegistry.Skill(skillID)
	if !ok {
		return "", errors.New("Film Skill 注册表引用失效：" + skillID)
	}
	priorities := append([]string{selectedAgentID, route.PrimaryAgentID}, route.CandidateAgentIDs...)
	for _, candidate := range priorities {
		if filmContainsString(skill.OwnerAgentIDs, candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("Film Intent %s 没有 Agent 可执行 Skill %s", route.ID, skillID)
}

func (s *Service) resolveFilmInputArtifactRefs(userID string, projectID string, revisionIDs []string) ([]FilmProductionArtifactRef, error) {
	if len(revisionIDs) > 50 {
		return nil, BadAuthRequest("单个 Film Agent Run 最多引用 50 个 Artifact revision")
	}
	seen := make(map[string]struct{}, len(revisionIDs))
	refs := make([]FilmProductionArtifactRef, 0, len(revisionIDs))
	for _, revisionID := range revisionIDs {
		revisionID = strings.TrimSpace(revisionID)
		if revisionID == "" {
			continue
		}
		if _, duplicate := seen[revisionID]; duplicate {
			return nil, BadAuthRequest("Film Agent 输入存在重复 Artifact revision")
		}
		seen[revisionID] = struct{}{}
		artifact, revision, err := s.repo.ProductionArtifactRevisionForUser(userID, revisionID)
		if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && (artifact.ProjectID != projectID || artifact.Domain != "film")) {
			return nil, NotFound("Film 输入 Artifact revision 不存在或不属于该项目")
		}
		if err != nil {
			return nil, err
		}
		canonicalType, ok := s.filmAgentRegistry.CanonicalArtifactType(artifact.ArtifactType)
		if !ok {
			return nil, BadAuthRequest("Artifact 类型不属于 Film AgentTeam：" + artifact.ArtifactType)
		}
		refs = append(refs, FilmProductionArtifactRef{
			ArtifactID: artifact.ID, RevisionID: revision.ID, Type: canonicalType, Version: revision.Version,
			Digest: revision.ContentDigest, Status: string(revision.Status),
		})
	}
	return refs, nil
}

func validateFilmRouteInputs(route agentruntime.IntentRouteDefinition, refs []FilmProductionArtifactRef) error {
	present := make(map[string][]FilmProductionArtifactRef)
	for _, ref := range refs {
		present[ref.Type] = append(present[ref.Type], ref)
	}
	for _, required := range route.RequiredInputArtifactTypes {
		candidates := present[required]
		if len(candidates) == 0 {
			return BadAuthRequest(fmt.Sprintf("%s 缺少必需的 %s Artifact", route.ID, required))
		}
		locked := false
		for _, candidate := range candidates {
			if candidate.Status == string(model.ProductionArtifactStatusLocked) {
				locked = true
				break
			}
		}
		if !locked {
			return conflictError(fmt.Sprintf("%s 的必需输入 %s 尚未锁定", route.ID, required))
		}
	}
	return nil
}

func buildFilmProjectRequirementsArtifact(userID string, project model.Project, runID string, stepID string, objective string, input map[string]any, at time.Time) (model.ProductionArtifact, model.ProductionArtifactRevision, FilmProductionArtifactRef, error) {
	content, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "objective": objective, "input": input,
		"project": map[string]any{"id": project.ID, "name": project.Name, "type": project.Type, "aspectRatio": project.AspectRatio, "sourceType": project.SourceType, "description": project.Description, "stylePresetId": project.StylePresetID, "styleProfileJson": project.StyleProfileJSON, "revision": project.Revision},
	})
	if err != nil {
		return model.ProductionArtifact{}, model.ProductionArtifactRevision{}, FilmProductionArtifactRef{}, err
	}
	artifactID := newID()
	revisionID := newID()
	digest := digestBytesHex(content)
	artifact := model.ProductionArtifact{
		ID: artifactID, UserID: userID, ProjectID: project.ID, Domain: "film", ArtifactType: "project-requirements",
		LogicalKey: "run:" + runID + ":project-requirements", CurrentRevisionID: revisionID, RevisionSequence: 1,
		CreatedAt: at, UpdatedAt: at,
	}
	revision := model.ProductionArtifactRevision{
		ID: revisionID, ArtifactID: artifactID, Version: 1, Status: model.ProductionArtifactStatusLocked,
		ContentJSON: string(content), ContentDigest: digest, SourceRunID: runID, SourceStepID: stepID,
		SourceArtifactRefsJSON: "[]", AuthorityRefsJSON: "[]", CreatedByType: "user", CreatedByID: userID, CreatedAt: at,
	}
	ref := FilmProductionArtifactRef{ArtifactID: artifactID, RevisionID: revisionID, Type: artifact.ArtifactType, Version: 1, Digest: digest, Status: string(revision.Status)}
	return artifact, revision, ref, nil
}

func digestFilmAgentRequest(projectID string, objective string, routeID string, agentID string, input map[string]any, revisionIDs []string, review bool) (string, error) {
	encoded, err := json.Marshal(map[string]any{
		"projectId": projectID, "objective": objective, "intentRouteId": routeID, "agentId": agentID,
		"input": input, "inputArtifactRevisionIds": revisionIDs, "reviewBeforeExecution": review,
	})
	if err != nil {
		return "", err
	}
	return digestBytesHex(encoded), nil
}

func filmRunRequestDigest(inputJSON string) string {
	var envelope struct {
		RequestDigest string `json:"requestDigest"`
	}
	_ = json.Unmarshal([]byte(inputJSON), &envelope)
	return envelope.RequestDigest
}

func digestString(value string) string { return digestBytesHex([]byte(value)) }

func digestBytesHex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func mustFilmJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func findFilmHumanDecision(items []model.AgentHumanDecision, id string) (model.AgentHumanDecision, bool) {
	for _, item := range items {
		if item.ID == strings.TrimSpace(id) {
			return item, true
		}
	}
	return model.AgentHumanDecision{}, false
}

func findFilmAgentStep(items []model.AgentRuntimeStep, id string) (model.AgentRuntimeStep, bool) {
	for _, item := range items {
		if item.ID == strings.TrimSpace(id) {
			return item, true
		}
	}
	return model.AgentRuntimeStep{}, false
}

func filmContainsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func conflictError(message string) *AuthError {
	return &AuthError{Status: 409, Message: message}
}

func mapAgentRuntimeRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrAgentRuntimeStateConflict), errors.Is(err, repository.ErrAgentRuntimeActiveDecision), errors.Is(err, repository.ErrProductionArtifactConflict):
		return conflictError("Film Agent Run 已发生变化，请刷新后重试")
	case errors.Is(err, repository.ErrAgentRuntimeInvalidTransition):
		return conflictError("当前 Film Agent 状态不允许该操作")
	case errors.Is(err, gorm.ErrRecordNotFound):
		return NotFound("Film Agent Run、Step 或 Decision 不存在")
	default:
		return err
	}
}
