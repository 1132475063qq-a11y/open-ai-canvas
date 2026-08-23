package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/gorm"
)

const filmVisualQCPromptLimit = 1 << 20

var filmVisualQCDimensions = []string{
	"identity", "anatomy", "contact", "acting", "props", "scene", "composition", "visual_continuity",
}

var filmVisualQCSkillIDs = []string{
	"continuity-check", "ai-production-feasibility-review", "final-film-quality-review",
}

type CreateFilmVisualQCQuoteRequest struct {
	SourceAttemptID      string   `json:"sourceAttemptId"`
	LogicalModelID       string   `json:"logicalModelId"`
	ReferenceResourceIDs []string `json:"referenceResourceIds"`
}

type SubmitFilmVisualQCQuoteRequest struct {
	QuoteFingerprint string `json:"quoteFingerprint"`
}

type FilmVisualQCQuoteView struct {
	ID                   string                          `json:"id"`
	ProjectID            string                          `json:"projectId"`
	RootRunID            string                          `json:"rootRunId"`
	ShotID               string                          `json:"shotId"`
	SourceAttemptID      string                          `json:"sourceAttemptId"`
	SourceResultID       string                          `json:"sourceResultId"`
	LogicalModelID       string                          `json:"logicalModelId"`
	Model                string                          `json:"model"`
	ReferenceResourceIDs []string                        `json:"referenceResourceIds"`
	Dimensions           []string                        `json:"dimensions"`
	Cost                 FilmProductionCostView          `json:"cost"`
	QuoteFingerprint     string                          `json:"quoteFingerprint"`
	RequestFingerprint   string                          `json:"requestFingerprint"`
	Status               model.FilmProductionQuoteStatus `json:"status"`
	ExpiresAt            time.Time                       `json:"expiresAt"`
	CreatedAt            time.Time                       `json:"createdAt"`
	Idempotent           bool                            `json:"idempotent"`
}

type FilmVisualQCAttemptView struct {
	Attempt model.FilmVisualQCAttempt `json:"attempt"`
	Task    TaskSummary               `json:"task"`
	Report  *FilmProductionQCView     `json:"report,omitempty"`
	Cost    FilmProductionCostView    `json:"cost"`
}

type SubmitFilmVisualQCQuoteResult struct {
	Attempt    FilmVisualQCAttemptView `json:"attempt"`
	Idempotent bool                    `json:"idempotent"`
}

type filmVisualQCSource struct {
	Detail              repository.FilmProductionAttemptDetail
	Resource            model.Resource
	ResultArtifact      model.ProductionArtifact
	ResultRevision      model.ProductionArtifactRevision
	StoryboardRevision  model.ProductionArtifactRevision
	PromptRevision      model.ProductionArtifactRevision
	FeasibilityRevision model.ProductionArtifactRevision
}

func (s *Service) CreateFilmVisualQCQuote(userID string, projectID string, idempotencyKey string, request CreateFilmVisualQCQuoteRequest) (FilmVisualQCQuoteView, error) {
	if err := s.ValidateRuntime(); err != nil {
		return FilmVisualQCQuoteView{}, err
	}
	if _, err := s.requireMutableFilmProject(userID, projectID); err != nil {
		return FilmVisualQCQuoteView{}, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if !filmAgentIdempotencyKeyPattern.MatchString(idempotencyKey) {
		return FilmVisualQCQuoteView{}, BadAuthRequest("X-Idempotency-Key 必须为 8-128 位字母、数字或 ._:-")
	}
	source, err := s.resolveFilmVisualQCSource(userID, projectID, strings.TrimSpace(request.SourceAttemptID))
	if err != nil {
		return FilmVisualQCQuoteView{}, err
	}
	references, referenceIDs, err := s.resolveFilmVisualQCReferences(userID, source, request.ReferenceResourceIDs)
	if err != nil {
		return FilmVisualQCQuoteView{}, err
	}
	systemPrompt, prompt, err := s.buildFilmVisualQCPrompt(source, referenceIDs)
	if err != nil {
		return FilmVisualQCQuoteView{}, err
	}
	requestFingerprint, err := filmVisualQCRequestFingerprint(projectID, source, strings.TrimSpace(request.LogicalModelID), referenceIDs, systemPrompt, prompt)
	if err != nil {
		return FilmVisualQCQuoteView{}, err
	}
	if existing, lookupErr := s.repo.FilmVisualQCQuoteByIdempotency(userID, idempotencyKey); lookupErr == nil {
		if existing.ProjectID != projectID || existing.RequestFingerprint != requestFingerprint {
			return FilmVisualQCQuoteView{}, conflictError("该幂等键已用于另一份 Film 视觉 QC 报价")
		}
		return filmVisualQCQuoteView(*existing, true)
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return FilmVisualQCQuoteView{}, lookupErr
	}

	logicalModelID := strings.TrimSpace(request.LogicalModelID)
	if logicalModelID == "" {
		return FilmVisualQCQuoteView{}, BadAuthRequest("请选择支持图片输入的文本逻辑模型")
	}
	input := map[string]any{
		"mode": "text", "prompt": prompt, "referenceImages": references,
		"config": map[string]any{"systemPrompt": systemPrompt},
		"metadata": map[string]any{
			"domain": "film", "filmRootRunId": source.Detail.Attempt.RootRunID, "filmShotId": source.Detail.Attempt.ShotID,
			"filmVisualQCSourceAttemptId": source.Detail.Attempt.ID, "filmVisualQCSourceResultId": source.Detail.Result.ID,
		},
	}
	intent := ModelRequestIntentFromTaskInput(input, model.FilmVisualQCTaskTypeImage, "film_visual_qc")
	routed, err := s.ResolveLogicalModel(logicalModelID, intent)
	if err != nil {
		return FilmVisualQCQuoteView{}, err
	}
	if routed.LogicalModel.Capability != "text" || routed.ChannelModel.Capability != "text" {
		return FilmVisualQCQuoteView{}, BadAuthRequest("视觉 QC 必须使用文本多模态模型")
	}
	if routed.ChannelModel.Protocol != model.ChannelInterfaceChatCompletion && routed.ChannelModel.Protocol != model.ChannelInterfaceOpenAIResponse {
		return FilmVisualQCQuoteView{}, BadAuthRequest("视觉 QC 当前只支持 Chat Completions 或 Responses 多模态协议")
	}
	input = applyRoutedProviderSelection(input, routed)
	if err := s.ValidateTaskCapability(input); err != nil {
		return FilmVisualQCQuoteView{}, err
	}
	if containsInlineMediaDataURL(input) || containsAgentRuntimeSecret(input) {
		return FilmVisualQCQuoteView{}, BadAuthRequest("视觉 QC 只能引用已上传资源，且不能包含密钥")
	}
	if err := s.protectTaskSecrets(input); err != nil {
		return FilmVisualQCQuoteView{}, err
	}
	requestJSON, err := json.Marshal(input)
	if err != nil {
		return FilmVisualQCQuoteView{}, err
	}
	now := time.Now().UTC()
	quoteID, taskID := newID(), newID()
	task := model.Task{
		ID: taskID, UserID: userID, ProjectID: projectID, Type: model.FilmVisualQCTaskTypeImage,
		Status: model.TaskStatusQueued, Stage: "等待用户确认视觉 QC 费用", Progress: 0, Prompt: prompt,
		Operation: "film_visual_qc", Provider: "managed", Model: routed.LogicalModel.Code,
		LogicalModelID: routed.LogicalModel.ID, LogicalModelRevisionID: routed.Revision.ID,
		RouteID: routed.Route.ID, ChannelModelID: routed.ChannelModel.ID, RouteRun: 1, InputJSON: string(requestJSON),
	}
	billing, err := s.taskBillingOrder(userID, &task, input)
	if err != nil {
		return FilmVisualQCQuoteView{}, err
	}
	if billing != nil {
		billing.IdempotencyKey = "film-visual-qc:" + quoteID
	}
	billingJSON := ""
	if billing != nil {
		encoded, marshalErr := json.Marshal(billing)
		if marshalErr != nil {
			return FilmVisualQCQuoteView{}, marshalErr
		}
		billingJSON = string(encoded)
	}
	referenceIDsJSON, _ := json.Marshal(referenceIDs)
	quote := model.FilmVisualQCQuote{
		ID: quoteID, UserID: userID, IdempotencyKey: idempotencyKey, ProjectID: projectID,
		RootRunID: source.Detail.Attempt.RootRunID, ShotID: source.Detail.Attempt.ShotID,
		SourceAttemptID: source.Detail.Attempt.ID, SourceResultID: source.Detail.Result.ID, SourceResourceID: source.Resource.ID,
		SourceResultArtifactID: source.ResultArtifact.ID, SourceResultRevisionID: source.ResultRevision.ID, SourceResultDigest: source.ResultRevision.ContentDigest,
		StoryboardArtifactID: source.Detail.Attempt.StoryboardArtifactID, StoryboardRevisionID: source.StoryboardRevision.ID, StoryboardDigest: source.StoryboardRevision.ContentDigest,
		PromptArtifactID: source.Detail.Attempt.PromptArtifactID, PromptRevisionID: source.PromptRevision.ID, PromptDigest: source.PromptRevision.ContentDigest,
		FeasibilityArtifactID: source.Detail.Attempt.FeasibilityArtifactID, FeasibilityRevisionID: source.FeasibilityRevision.ID, FeasibilityDigest: source.FeasibilityRevision.ContentDigest,
		RegistryID: source.Detail.Attempt.RegistryID, RegistryVersion: source.Detail.Attempt.RegistryVersion, RegistryDigest: source.Detail.Attempt.RegistryDigest,
		LogicalModelID: routed.LogicalModel.ID, LogicalModelRevisionID: routed.Revision.ID,
		RouteID: routed.Route.ID, ChannelID: routed.ChannelModel.ChannelID, ChannelModelID: routed.ChannelModel.ID,
		Model: routed.LogicalModel.Code, ProviderModel: routed.ChannelModel.ModelKey, Protocol: routed.ChannelModel.Protocol,
		CapabilityVersion: routed.ChannelModel.CapabilityVersion, ChannelPriceVersion: routed.ChannelModel.PriceVersion,
		ReferenceResourceIDsJSON: string(referenceIDsJSON), TaskID: taskID, RequestJSON: string(requestJSON), RequestFingerprint: requestFingerprint, BillingJSON: billingJSON,
		Status: model.FilmProductionQuoteStatusPending, ExpiresAt: now.Add(filmProductionQuoteTTL), CreatedAt: now, UpdatedAt: now,
	}
	quote.QuoteFingerprint, err = filmVisualQCQuoteFingerprint(quote)
	if err != nil {
		return FilmVisualQCQuoteView{}, err
	}
	if err := s.repo.CreateFilmVisualQCQuote(repository.FilmVisualQCQuoteCreateCommand{Quote: &quote, At: now}); err != nil {
		return FilmVisualQCQuoteView{}, mapFilmVisualQCError(err)
	}
	return filmVisualQCQuoteView(quote, false)
}

func (s *Service) SubmitFilmVisualQCQuote(userID string, projectID string, quoteID string, idempotencyKey string, request SubmitFilmVisualQCQuoteRequest) (SubmitFilmVisualQCQuoteResult, error) {
	if _, err := s.requireMutableFilmProject(userID, projectID); err != nil {
		return SubmitFilmVisualQCQuoteResult{}, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if !filmAgentIdempotencyKeyPattern.MatchString(idempotencyKey) {
		return SubmitFilmVisualQCQuoteResult{}, BadAuthRequest("X-Idempotency-Key 必须为 8-128 位字母、数字或 ._:-")
	}
	quote, err := s.repo.FilmVisualQCQuoteForUser(userID, projectID, strings.TrimSpace(quoteID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SubmitFilmVisualQCQuoteResult{}, NotFound("Film 视觉 QC 报价不存在")
	}
	if err != nil {
		return SubmitFilmVisualQCQuoteResult{}, err
	}
	attemptID := newID()
	input, prompt, err := filmVisualQCSubmissionInput(*quote, attemptID)
	if err != nil {
		return SubmitFilmVisualQCQuoteResult{}, err
	}
	if err := s.protectTaskSecrets(input); err != nil {
		return SubmitFilmVisualQCQuoteResult{}, err
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return SubmitFilmVisualQCQuoteResult{}, err
	}
	policy, err := s.RuntimePolicy()
	if err != nil {
		return SubmitFilmVisualQCQuoteResult{}, err
	}
	now := time.Now().UTC()
	task := &model.Task{
		ID: quote.TaskID, UserID: userID, ProjectID: projectID, Type: model.FilmVisualQCTaskTypeImage,
		Status: model.TaskStatusQueued, Stage: "等待队列调度", Progress: 5, Prompt: prompt,
		Operation: "film_visual_qc", Provider: "managed", Model: quote.Model,
		LogicalModelID: quote.LogicalModelID, LogicalModelRevisionID: quote.LogicalModelRevisionID,
		RouteID: quote.RouteID, ChannelModelID: quote.ChannelModelID, RouteRun: 1, InputJSON: string(inputJSON),
	}
	command := repository.FilmVisualQCSubmitCommand{
		UserID: userID, ProjectID: projectID, QuoteID: quote.ID, QuoteFingerprint: strings.TrimSpace(request.QuoteFingerprint),
		IdempotencyKey: idempotencyKey, AttemptID: attemptID, Task: task, ActiveTaskLimit: policy.Task.ActiveTaskLimit, At: now,
		Event: repository.AgentRuntimeEventInput{
			ID: newID(), EventType: "film.production.image.visual_qc.submitted", ActorType: "human", ActorID: userID,
			PayloadJSON: mustFilmJSON(map[string]any{"quoteId": quote.ID, "taskId": quote.TaskID, "sourceAttemptId": quote.SourceAttemptID, "sourceResultId": quote.SourceResultID}),
		},
	}
	stored, err := s.submitFilmVisualQCWithinStorageQuota(command, policy)
	if err != nil {
		return SubmitFilmVisualQCQuoteResult{}, mapFilmVisualQCError(err)
	}
	view, err := s.filmVisualQCAttemptView(repository.FilmVisualQCAttemptDetail{Attempt: stored.Attempt, Task: stored.Task, Billing: stored.Billing})
	if err != nil {
		return SubmitFilmVisualQCQuoteResult{}, err
	}
	return SubmitFilmVisualQCQuoteResult{Attempt: view, Idempotent: stored.Idempotent}, nil
}

func (s *Service) resolveFilmVisualQCSource(userID string, projectID string, attemptID string) (filmVisualQCSource, error) {
	detail, err := s.repo.FilmProductionAttemptForUser(userID, projectID, attemptID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return filmVisualQCSource{}, NotFound("Film 生图 Attempt 不存在")
	}
	if err != nil {
		return filmVisualQCSource{}, err
	}
	if detail.Attempt.Status != model.FilmProductionAttemptStatusSucceeded || detail.Result == nil {
		return filmVisualQCSource{}, conflictError("只有已生成真实图片的 Attempt 才能运行视觉 QC")
	}
	media, err := filmProductionMediaFactFromResult([]byte(detail.Result.Payload))
	if err != nil || strings.TrimSpace(media.ResourceID) == "" {
		return filmVisualQCSource{}, conflictError("生成图片缺少可持久引用的资源，不能运行视觉 QC")
	}
	resource, err := s.repo.ResourceForUser(userID, media.ResourceID)
	if err != nil {
		return filmVisualQCSource{}, err
	}
	if resource.Status != model.ResourceStatusReady || resource.Kind != "image" || !strings.HasPrefix(resource.MimeType, "image/") {
		return filmVisualQCSource{}, conflictError("生成图片资源当前不可用于视觉 QC")
	}
	resultArtifact, resultRevision, err := s.repo.ProductionArtifactRevisionForUser(userID, detail.Attempt.ResultRevisionID)
	if err != nil {
		return filmVisualQCSource{}, err
	}
	if resultArtifact.ID != detail.Attempt.ResultArtifactID || resultArtifact.ProjectID != projectID || resultArtifact.Domain != "film" || resultArtifact.ArtifactType != "generation-result" ||
		resultArtifact.CurrentRevisionID != resultRevision.ID || resultRevision.Status != model.ProductionArtifactStatusLocked {
		return filmVisualQCSource{}, conflictError("生成结果 Artifact 已变化，请刷新后重试")
	}
	storyboard, storyboardRevision, err := s.repo.ProductionArtifactRevisionForUser(userID, detail.Attempt.StoryboardRevisionID)
	if err != nil || storyboard.ID != detail.Attempt.StoryboardArtifactID || storyboardRevision.ContentDigest != detail.Attempt.StoryboardDigest || storyboardRevision.Status != model.ProductionArtifactStatusLocked {
		return filmVisualQCSource{}, conflictError("视觉 QC 的分镜证据已变化")
	}
	promptArtifact, promptRevision, err := s.repo.ProductionArtifactRevisionForUser(userID, detail.Attempt.PromptRevisionID)
	if err != nil || promptArtifact.ID != detail.Attempt.PromptArtifactID || promptRevision.ContentDigest != detail.Attempt.PromptDigest || promptRevision.Status != model.ProductionArtifactStatusLocked {
		return filmVisualQCSource{}, conflictError("视觉 QC 的 Prompt 证据已变化")
	}
	feasibility, feasibilityRevision, err := s.repo.ProductionArtifactRevisionForUser(userID, detail.Attempt.FeasibilityRevisionID)
	if err != nil || feasibility.ID != detail.Attempt.FeasibilityArtifactID || feasibilityRevision.ContentDigest != detail.Attempt.FeasibilityDigest || feasibilityRevision.Status != model.ProductionArtifactStatusLocked {
		return filmVisualQCSource{}, conflictError("视觉 QC 的制作可行性证据已变化")
	}
	return filmVisualQCSource{
		Detail: *detail, Resource: *resource, ResultArtifact: *resultArtifact, ResultRevision: *resultRevision,
		StoryboardRevision: *storyboardRevision, PromptRevision: *promptRevision, FeasibilityRevision: *feasibilityRevision,
	}, nil
}

func (s *Service) resolveFilmVisualQCReferences(userID string, source filmVisualQCSource, extraIDs []string) ([]providerMedia, []string, error) {
	ids := []string{source.Resource.ID}
	decrypted, err := s.decryptTaskInputJSON(source.Detail.Task.InputJSON)
	if err != nil {
		return nil, nil, err
	}
	var original canvasGenerationInput
	if json.Unmarshal([]byte(decrypted), &original) != nil {
		return nil, nil, conflictError("来源生图任务输入已损坏")
	}
	for _, reference := range original.ReferenceImages {
		if strings.HasPrefix(reference.StorageKey, "resource:") {
			ids = append(ids, strings.TrimPrefix(reference.StorageKey, "resource:"))
		}
	}
	ids = append(ids, extraIDs...)
	ids = uniqueNonEmpty(ids)
	if len(ids) > 16 {
		return nil, nil, BadAuthRequest("单次视觉 QC 最多使用 16 张图片证据")
	}
	media, normalizedIDs, err := s.resolveFilmProductionImageReferences(userID, ids)
	if err != nil {
		return nil, nil, err
	}
	if len(normalizedIDs) == 0 || normalizedIDs[0] != source.Resource.ID {
		return nil, nil, conflictError("视觉 QC 第一张证据必须是本次生成结果")
	}
	return media, normalizedIDs, nil
}

func (s *Service) buildFilmVisualQCPrompt(source filmVisualQCSource, referenceIDs []string) (string, string, error) {
	agent, ok := s.filmAgentRegistry.Agent("quality_control_editor")
	if !ok {
		return "", "", errors.New("Film AgentTeam 缺少 quality_control_editor")
	}
	var system strings.Builder
	system.WriteString("You are executing a paid, evidence-bound visual semantic QC review for a Film production image. Never infer facts that are not visible. The first attached image is the generated result; later images are references only. A model verdict is advisory and must never authorize final acceptance.\n\n")
	system.WriteString("[AGENT quality_control_editor]\n" + agent.DeveloperInstructions + "\n")
	for _, skillID := range filmVisualQCSkillIDs {
		skill, ok := s.filmAgentRegistry.Skill(skillID)
		if !ok || !filmContainsString(skill.OwnerAgentIDs, agent.ID) {
			return "", "", fmt.Errorf("quality_control_editor cannot execute Skill %s", skillID)
		}
		system.WriteString("\n[SKILL " + skill.ID + " v" + skill.Version + "]\n" + skill.Instructions + "\n")
	}
	system.WriteString("\n[VISUAL QC OUTPUT CONTRACT]\nReturn exactly one JSON object with no markdown fence and no prose outside it. Allowed keys are schemaVersion, overallDecision, dimensions, and summary. schemaVersion must be 1. overallDecision must be PASS, UNCERTAIN, or FAIL. dimensions must contain exactly once and in this order: identity, anatomy, contact, acting, props, scene, composition, visual_continuity. Every dimension object must contain only dimension, decision, issueCodes, observations, and rationale. decision uses PASS, UNCERTAIN, or FAIL. issueCodes use uppercase letters, digits, dot, underscore, colon or hyphen; every non-PASS dimension needs at least one code. observations is a non-empty array of concise visible facts. Use UNCERTAIN whenever the image or references cannot prove the dimension. overallDecision is FAIL if any dimension FAILs, otherwise UNCERTAIN if any dimension is UNCERTAIN, otherwise PASS. Do not emit an acceptance action.\n")
	roles := make([]map[string]any, 0, len(referenceIDs))
	for index, resourceID := range referenceIDs {
		role := "reference"
		if index == 0 {
			role = "generated_result"
		}
		roles = append(roles, map[string]any{"imageIndex": index + 1, "role": role, "resourceId": resourceID})
	}
	payload := map[string]any{
		"schemaVersion": 1,
		"target":        map[string]any{"sourceAttemptId": source.Detail.Attempt.ID, "sourceResultId": source.Detail.Result.ID, "shotId": source.Detail.Attempt.ShotID},
		"imageRoles":    roles,
		"lockedEvidence": map[string]any{
			"storyboard":            map[string]any{"revisionId": source.StoryboardRevision.ID, "digest": source.StoryboardRevision.ContentDigest, "content": json.RawMessage(source.StoryboardRevision.ContentJSON)},
			"imagePrompt":           map[string]any{"revisionId": source.PromptRevision.ID, "digest": source.PromptRevision.ContentDigest, "content": json.RawMessage(source.PromptRevision.ContentJSON)},
			"productionFeasibility": map[string]any{"revisionId": source.FeasibilityRevision.ID, "digest": source.FeasibilityRevision.ContentDigest, "content": json.RawMessage(source.FeasibilityRevision.ContentJSON)},
		},
		"requiredDimensions": filmVisualQCDimensions,
	}
	promptJSON, err := json.Marshal(payload)
	if err != nil {
		return "", "", err
	}
	if len(promptJSON)+system.Len() > filmVisualQCPromptLimit {
		return "", "", BadAuthRequest("视觉 QC 的锁定证据超过 1MiB，请缩小制作产物后重试")
	}
	return system.String(), string(promptJSON), nil
}

func filmVisualQCSubmissionInput(quote model.FilmVisualQCQuote, attemptID string) (map[string]any, string, error) {
	var input map[string]any
	if json.Unmarshal([]byte(quote.RequestJSON), &input) != nil {
		return nil, "", conflictError("Film 视觉 QC 报价请求快照已损坏")
	}
	prompt, _ := input["prompt"].(string)
	if strings.TrimSpace(prompt) == "" {
		return nil, "", conflictError("Film 视觉 QC 报价缺少 Prompt")
	}
	metadata, _ := input["metadata"].(map[string]any)
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["filmVisualQCQuoteId"] = quote.ID
	metadata["filmVisualQCAttemptId"] = attemptID
	metadata["filmVisualQCSourceAttemptId"] = quote.SourceAttemptID
	metadata["filmVisualQCSourceResultId"] = quote.SourceResultID
	input["metadata"] = metadata
	return input, prompt, nil
}

func filmVisualQCRequestFingerprint(projectID string, source filmVisualQCSource, logicalModelID string, referenceIDs []string, systemPrompt string, prompt string) (string, error) {
	encoded, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "projectId": projectID, "rootRunId": source.Detail.Attempt.RootRunID, "shotId": source.Detail.Attempt.ShotID,
		"sourceAttemptId": source.Detail.Attempt.ID, "sourceResultId": source.Detail.Result.ID,
		"sourceResultRevisionId": source.ResultRevision.ID, "sourceResultDigest": source.ResultRevision.ContentDigest,
		"storyboardRevisionId": source.StoryboardRevision.ID, "storyboardDigest": source.StoryboardRevision.ContentDigest,
		"promptRevisionId": source.PromptRevision.ID, "promptDigest": source.PromptRevision.ContentDigest,
		"feasibilityRevisionId": source.FeasibilityRevision.ID, "feasibilityDigest": source.FeasibilityRevision.ContentDigest,
		"registryDigest": source.Detail.Attempt.RegistryDigest, "logicalModelId": logicalModelID, "referenceResourceIds": referenceIDs,
		"systemPromptDigest": digestString(systemPrompt), "promptDigestV2": digestString(prompt),
	})
	if err != nil {
		return "", err
	}
	return digestBytesHex(encoded), nil
}

func filmVisualQCQuoteFingerprint(quote model.FilmVisualQCQuote) (string, error) {
	encoded, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "quoteId": quote.ID, "taskId": quote.TaskID, "projectId": quote.ProjectID,
		"rootRunId": quote.RootRunID, "shotId": quote.ShotID, "sourceAttemptId": quote.SourceAttemptID, "sourceResultId": quote.SourceResultID,
		"sourceResultRevisionId": quote.SourceResultRevisionID, "sourceResultDigest": quote.SourceResultDigest,
		"registryDigest": quote.RegistryDigest, "logicalModelId": quote.LogicalModelID, "logicalModelRevisionId": quote.LogicalModelRevisionID,
		"routeId": quote.RouteID, "channelModelId": quote.ChannelModelID, "capabilityVersion": quote.CapabilityVersion,
		"channelPriceVersion": quote.ChannelPriceVersion, "request": json.RawMessage(quote.RequestJSON), "requestFingerprint": quote.RequestFingerprint,
		"billing": json.RawMessage(firstNonEmpty(quote.BillingJSON, "null")), "expiresAt": quote.ExpiresAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return "", err
	}
	return digestBytesHex(encoded), nil
}

func filmVisualQCQuoteView(quote model.FilmVisualQCQuote, idempotent bool) (FilmVisualQCQuoteView, error) {
	var referenceIDs []string
	if json.Unmarshal([]byte(firstNonEmpty(quote.ReferenceResourceIDsJSON, "[]")), &referenceIDs) != nil {
		return FilmVisualQCQuoteView{}, errors.New("Film visual QC quote references are invalid")
	}
	billing, err := filmVisualQCQuoteBilling(quote)
	if err != nil {
		return FilmVisualQCQuoteView{}, err
	}
	status := quote.Status
	if status == model.FilmProductionQuoteStatusPending && !quote.ExpiresAt.After(time.Now()) {
		status = model.FilmProductionQuoteStatusExpired
	}
	return FilmVisualQCQuoteView{
		ID: quote.ID, ProjectID: quote.ProjectID, RootRunID: quote.RootRunID, ShotID: quote.ShotID,
		SourceAttemptID: quote.SourceAttemptID, SourceResultID: quote.SourceResultID, LogicalModelID: quote.LogicalModelID, Model: quote.Model,
		ReferenceResourceIDs: referenceIDs, Dimensions: append([]string(nil), filmVisualQCDimensions...), Cost: filmProductionCostView(billing),
		QuoteFingerprint: quote.QuoteFingerprint, RequestFingerprint: quote.RequestFingerprint, Status: status,
		ExpiresAt: quote.ExpiresAt, CreatedAt: quote.CreatedAt, Idempotent: idempotent,
	}, nil
}

func filmVisualQCQuoteBilling(quote model.FilmVisualQCQuote) (*model.BillingOrder, error) {
	if strings.TrimSpace(quote.BillingJSON) == "" || strings.TrimSpace(quote.BillingJSON) == "null" {
		return nil, nil
	}
	var order model.BillingOrder
	if err := json.Unmarshal([]byte(quote.BillingJSON), &order); err != nil {
		return nil, err
	}
	return &order, nil
}

func (s *Service) filmVisualQCAttemptView(detail repository.FilmVisualQCAttemptDetail) (FilmVisualQCAttemptView, error) {
	task := detail.Task
	s.hydrateTaskProviderRequestID(&task)
	var report *FilmProductionQCView
	if detail.Report != nil {
		view := filmProductionQCView(*detail.Report)
		report = &view
	}
	return FilmVisualQCAttemptView{Attempt: detail.Attempt, Task: taskSummaryForOutput(task), Report: report, Cost: filmProductionCostView(detail.Billing)}, nil
}

func (s *Service) submitFilmVisualQCWithinStorageQuota(command repository.FilmVisualQCSubmitCommand, policy RuntimePolicySetting) (*repository.FilmVisualQCSubmitResult, error) {
	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	if existing, err := s.repo.FilmVisualQCAttemptByIdempotency(command.UserID, command.IdempotencyKey); err == nil {
		if existing.ProjectID != command.ProjectID || existing.QuoteID != command.QuoteID {
			return nil, repository.ErrFilmProductionStateConflict
		}
		return s.repo.SubmitFilmVisualQCQuote(command)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	usage, err := s.repo.UserStorageUsage(command.UserID)
	if err != nil {
		return nil, err
	}
	incomingBytes := int64(len(command.Task.Prompt) + len(command.Task.InputJSON))
	if err := validateTaskStorageQuotaWithPolicy(usage, incomingBytes, policy.Resource); err != nil {
		return nil, err
	}
	return s.repo.SubmitFilmVisualQCQuote(command)
}

func mapFilmVisualQCError(err error) error {
	switch {
	case errors.Is(err, repository.ErrFilmProductionQuoteExpired):
		return conflictError("Film 视觉 QC 报价已过期，请重新报价")
	case errors.Is(err, repository.ErrFilmProductionQuoteConsumed):
		return conflictError("Film 视觉 QC 报价已被其他提交使用")
	case errors.Is(err, repository.ErrFilmProductionQuoteConflict):
		return conflictError("Film 视觉 QC 报价指纹不匹配，请刷新后重试")
	case errors.Is(err, repository.ErrFilmProductionQuoteDrift):
		return conflictError("生成结果、锁定产物、模型或价格已变化，请重新报价")
	case errors.Is(err, repository.ErrFilmProductionActiveAttempt):
		return conflictError("该图片已有正在执行或费用待核对的视觉 QC Attempt")
	case errors.Is(err, repository.ErrFilmProductionStateConflict):
		return conflictError("Film 视觉 QC 状态已变化，请刷新后重试")
	case errors.Is(err, repository.ErrFilmProductionMediaMissing):
		return conflictError("生成图片或参考媒体不可访问，不能运行视觉 QC")
	case errors.Is(err, repository.ErrInsufficientCredits):
		return BadAuthRequest("积分不足，请先使用兑换码充值")
	case errors.Is(err, repository.ErrActiveTaskLimit):
		return BadAuthRequest("同时排队或运行的任务已达到上限，请等待已有任务完成")
	case errors.Is(err, gorm.ErrRecordNotFound):
		return NotFound("Film 视觉 QC 关联记录不存在")
	default:
		return err
	}
}
