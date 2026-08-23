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

var filmVideoVisualQCDimensions = []string{
	"identity", "anatomy", "contact", "acting", "props", "scene", "composition", "motion", "temporal_continuity", "lip_sync_audio",
}

type FilmVideoVisualQCSampleInput struct {
	TimeMs     int64  `json:"timeMs"`
	ResourceID string `json:"resourceId"`
}

type FilmVideoVisualQCSample struct {
	TimeMs     int64  `json:"timeMs"`
	ResourceID string `json:"resourceId"`
	MimeType   string `json:"mimeType"`
	Size       int64  `json:"size"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	ETag       string `json:"etag"`
}

type CreateFilmVideoVisualQCQuoteRequest struct {
	SourceAttemptID string                         `json:"sourceAttemptId"`
	LogicalModelID  string                         `json:"logicalModelId"`
	SampleFrames    []FilmVideoVisualQCSampleInput `json:"sampleFrames"`
}

type SubmitFilmVideoVisualQCQuoteRequest struct {
	QuoteFingerprint string `json:"quoteFingerprint"`
}

type FilmVideoVisualQCQuoteView struct {
	ID                 string                          `json:"id"`
	ProjectID          string                          `json:"projectId"`
	RootRunID          string                          `json:"rootRunId"`
	SequenceID         string                          `json:"sequenceId"`
	SlotID             string                          `json:"slotId"`
	ShotID             string                          `json:"shotId"`
	SourceAttemptID    string                          `json:"sourceAttemptId"`
	SourceResultID     string                          `json:"sourceResultId"`
	LogicalModelID     string                          `json:"logicalModelId"`
	Model              string                          `json:"model"`
	SampleFrames       []FilmVideoVisualQCSample       `json:"sampleFrames"`
	Dimensions         []string                        `json:"dimensions"`
	EvidenceLimitation string                          `json:"evidenceLimitation"`
	Cost               FilmProductionCostView          `json:"cost"`
	QuoteFingerprint   string                          `json:"quoteFingerprint"`
	RequestFingerprint string                          `json:"requestFingerprint"`
	Status             model.FilmProductionQuoteStatus `json:"status"`
	ExpiresAt          time.Time                       `json:"expiresAt"`
	CreatedAt          time.Time                       `json:"createdAt"`
	Idempotent         bool                            `json:"idempotent"`
}

type FilmVideoVisualQCAttemptView struct {
	Attempt      model.FilmVideoVisualQCAttempt `json:"attempt"`
	Task         TaskSummary                    `json:"task"`
	Report       *FilmVideoQCView               `json:"report,omitempty"`
	SampleFrames []FilmVideoVisualQCSample      `json:"sampleFrames"`
	Cost         FilmProductionCostView         `json:"cost"`
	Valid        bool                           `json:"valid"`
}

type SubmitFilmVideoVisualQCQuoteResult struct {
	Attempt    FilmVideoVisualQCAttemptView `json:"attempt"`
	Idempotent bool                         `json:"idempotent"`
}

type filmVideoVisualQCSource struct {
	Detail              repository.FilmVideoAttemptDetail
	Sequence            repository.FilmVideoSequenceDetail
	Slot                repository.FilmVideoSlotDetail
	VideoResource       model.Resource
	SourceImageResource model.Resource
	ResultArtifact      model.ProductionArtifact
	ResultRevision      model.ProductionArtifactRevision
	PromptRevision      model.ProductionArtifactRevision
	SourceImageRevision model.ProductionArtifactRevision
	DurationMs          int64
}

func (s *Service) CreateFilmVideoVisualQCQuote(userID string, projectID string, idempotencyKey string, request CreateFilmVideoVisualQCQuoteRequest) (FilmVideoVisualQCQuoteView, error) {
	if err := s.ValidateRuntime(); err != nil {
		return FilmVideoVisualQCQuoteView{}, err
	}
	if _, err := s.requireMutableFilmProject(userID, projectID); err != nil {
		return FilmVideoVisualQCQuoteView{}, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if !filmAgentIdempotencyKeyPattern.MatchString(idempotencyKey) {
		return FilmVideoVisualQCQuoteView{}, BadAuthRequest("X-Idempotency-Key 必须为 8-128 位字母、数字或 ._:-")
	}
	source, err := s.resolveFilmVideoVisualQCSource(userID, projectID, strings.TrimSpace(request.SourceAttemptID))
	if err != nil {
		return FilmVideoVisualQCQuoteView{}, err
	}
	references, referenceIDs, samples, err := s.resolveFilmVideoVisualQCSamples(userID, source, request.SampleFrames)
	if err != nil {
		return FilmVideoVisualQCQuoteView{}, err
	}
	systemPrompt, prompt, err := s.buildFilmVideoVisualQCPrompt(source, samples, referenceIDs)
	if err != nil {
		return FilmVideoVisualQCQuoteView{}, err
	}
	logicalModelID := strings.TrimSpace(request.LogicalModelID)
	requestFingerprint, err := filmVideoVisualQCRequestFingerprint(projectID, source, logicalModelID, samples, referenceIDs, systemPrompt, prompt)
	if err != nil {
		return FilmVideoVisualQCQuoteView{}, err
	}
	if existing, lookupErr := s.repo.FilmVideoVisualQCQuoteByIdempotency(userID, idempotencyKey); lookupErr == nil {
		if existing.ProjectID != projectID || existing.RequestFingerprint != requestFingerprint {
			return FilmVideoVisualQCQuoteView{}, conflictError("该幂等键已用于另一份 Film 视频视觉 QC 报价")
		}
		return filmVideoVisualQCQuoteView(*existing, true)
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return FilmVideoVisualQCQuoteView{}, lookupErr
	}
	if logicalModelID == "" {
		return FilmVideoVisualQCQuoteView{}, BadAuthRequest("请选择支持图片输入的文本逻辑模型")
	}
	input := map[string]any{
		"mode": "text", "prompt": prompt, "referenceImages": references,
		"config": map[string]any{"systemPrompt": systemPrompt},
		"metadata": map[string]any{
			"domain": "film", "filmRootRunId": source.Detail.Attempt.RootRunID, "filmVideoSequenceId": source.Detail.Attempt.SequenceID,
			"filmVideoSlotId": source.Detail.Attempt.SlotID, "filmVideoVisualQCSourceAttemptId": source.Detail.Attempt.ID,
			"filmVideoVisualQCSourceResultId": source.Detail.Result.ID,
		},
	}
	intent := ModelRequestIntentFromTaskInput(input, model.FilmVisualQCTaskTypeVideo, "film_video_visual_qc")
	routed, err := s.ResolveLogicalModel(logicalModelID, intent)
	if err != nil {
		return FilmVideoVisualQCQuoteView{}, err
	}
	if routed.LogicalModel.Capability != "text" || routed.ChannelModel.Capability != "text" {
		return FilmVideoVisualQCQuoteView{}, BadAuthRequest("视频视觉 QC 必须使用文本多模态模型")
	}
	if routed.ChannelModel.Protocol != model.ChannelInterfaceChatCompletion && routed.ChannelModel.Protocol != model.ChannelInterfaceOpenAIResponse {
		return FilmVideoVisualQCQuoteView{}, BadAuthRequest("视频视觉 QC 当前只支持 Chat Completions 或 Responses 多模态协议")
	}
	input = applyRoutedProviderSelection(input, routed)
	if err := s.ValidateTaskCapability(input); err != nil {
		return FilmVideoVisualQCQuoteView{}, err
	}
	if containsInlineMediaDataURL(input) || containsAgentRuntimeSecret(input) {
		return FilmVideoVisualQCQuoteView{}, BadAuthRequest("视频视觉 QC 只能引用已上传采样帧，且不能包含密钥")
	}
	if err := s.protectTaskSecrets(input); err != nil {
		return FilmVideoVisualQCQuoteView{}, err
	}
	requestJSON, err := json.Marshal(input)
	if err != nil {
		return FilmVideoVisualQCQuoteView{}, err
	}
	now := time.Now().UTC()
	quoteID, taskID := newID(), newID()
	task := model.Task{
		ID: taskID, UserID: userID, ProjectID: projectID, Type: model.FilmVisualQCTaskTypeVideo,
		Status: model.TaskStatusQueued, Stage: "等待用户确认视频视觉 QC 费用", Progress: 0, Prompt: prompt,
		Operation: "film_video_visual_qc", Provider: "managed", Model: routed.LogicalModel.Code,
		LogicalModelID: routed.LogicalModel.ID, LogicalModelRevisionID: routed.Revision.ID,
		RouteID: routed.Route.ID, ChannelModelID: routed.ChannelModel.ID, RouteRun: 1, InputJSON: string(requestJSON),
	}
	billing, err := s.taskBillingOrder(userID, &task, input)
	if err != nil {
		return FilmVideoVisualQCQuoteView{}, err
	}
	if billing != nil {
		billing.IdempotencyKey = "film-video-visual-qc:" + quoteID
	}
	billingJSON := ""
	if billing != nil {
		encoded, marshalErr := json.Marshal(billing)
		if marshalErr != nil {
			return FilmVideoVisualQCQuoteView{}, marshalErr
		}
		billingJSON = string(encoded)
	}
	sampleFramesJSON, _ := json.Marshal(samples)
	referenceIDsJSON, _ := json.Marshal(referenceIDs)
	quote := model.FilmVideoVisualQCQuote{
		ID: quoteID, UserID: userID, IdempotencyKey: idempotencyKey, ProjectID: projectID, RootRunID: source.Detail.Attempt.RootRunID,
		SequenceID: source.Detail.Attempt.SequenceID, SequenceRevision: source.Sequence.Sequence.Revision, SlotID: source.Detail.Attempt.SlotID,
		SlotRevision: source.Slot.Slot.Revision, ShotID: source.Slot.Slot.ShotID, SourceAttemptID: source.Detail.Attempt.ID,
		SourceResultID: source.Detail.Result.ID, SourceResourceID: source.VideoResource.ID, SourceResultArtifactID: source.ResultArtifact.ID,
		SourceResultRevisionID: source.ResultRevision.ID, SourceResultDigest: source.ResultRevision.ContentDigest,
		SourceImageArtifactID: source.Slot.Slot.SourceImageArtifactID, SourceImageRevisionID: source.SourceImageRevision.ID,
		PromptArtifactID: source.Sequence.Sequence.PromptArtifactID, PromptRevisionID: source.PromptRevision.ID, PromptDigest: source.PromptRevision.ContentDigest,
		RegistryID: source.Sequence.Sequence.RegistryID, RegistryVersion: source.Sequence.Sequence.RegistryVersion, RegistryDigest: source.Sequence.Sequence.RegistryDigest,
		LogicalModelID: routed.LogicalModel.ID, LogicalModelRevisionID: routed.Revision.ID, RouteID: routed.Route.ID,
		ChannelID: routed.ChannelModel.ChannelID, ChannelModelID: routed.ChannelModel.ID, Model: routed.LogicalModel.Code,
		ProviderModel: routed.ChannelModel.ModelKey, Protocol: routed.ChannelModel.Protocol, CapabilityVersion: routed.ChannelModel.CapabilityVersion,
		ChannelPriceVersion: routed.ChannelModel.PriceVersion, SampleFramesJSON: string(sampleFramesJSON), ReferenceResourceIDsJSON: string(referenceIDsJSON),
		TaskID: taskID, RequestJSON: string(requestJSON), RequestFingerprint: requestFingerprint, BillingJSON: billingJSON,
		Status: model.FilmProductionQuoteStatusPending, ExpiresAt: now.Add(filmProductionQuoteTTL), CreatedAt: now, UpdatedAt: now,
	}
	quote.QuoteFingerprint, err = filmVideoVisualQCQuoteFingerprint(quote)
	if err != nil {
		return FilmVideoVisualQCQuoteView{}, err
	}
	if err := s.repo.CreateFilmVideoVisualQCQuote(repository.FilmVideoVisualQCQuoteCreateCommand{Quote: &quote, At: now}); err != nil {
		return FilmVideoVisualQCQuoteView{}, mapFilmVideoVisualQCError(err)
	}
	return filmVideoVisualQCQuoteView(quote, false)
}

func (s *Service) SubmitFilmVideoVisualQCQuote(userID string, projectID string, quoteID string, idempotencyKey string, request SubmitFilmVideoVisualQCQuoteRequest) (SubmitFilmVideoVisualQCQuoteResult, error) {
	if _, err := s.requireMutableFilmProject(userID, projectID); err != nil {
		return SubmitFilmVideoVisualQCQuoteResult{}, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if !filmAgentIdempotencyKeyPattern.MatchString(idempotencyKey) {
		return SubmitFilmVideoVisualQCQuoteResult{}, BadAuthRequest("X-Idempotency-Key 必须为 8-128 位字母、数字或 ._:-")
	}
	quote, err := s.repo.FilmVideoVisualQCQuoteForUser(userID, projectID, strings.TrimSpace(quoteID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SubmitFilmVideoVisualQCQuoteResult{}, NotFound("Film 视频视觉 QC 报价不存在")
	}
	if err != nil {
		return SubmitFilmVideoVisualQCQuoteResult{}, err
	}
	promptInput, prompt, err := filmVideoVisualQCSubmissionInput(*quote, newID())
	if err != nil {
		return SubmitFilmVideoVisualQCQuoteResult{}, err
	}
	// Attempt ID participates in the immutable task metadata and must be reused.
	attemptID := promptInput["metadata"].(map[string]any)["filmVideoVisualQCAttemptId"].(string)
	if err := s.protectTaskSecrets(promptInput); err != nil {
		return SubmitFilmVideoVisualQCQuoteResult{}, err
	}
	inputJSON, err := json.Marshal(promptInput)
	if err != nil {
		return SubmitFilmVideoVisualQCQuoteResult{}, err
	}
	policy, err := s.RuntimePolicy()
	if err != nil {
		return SubmitFilmVideoVisualQCQuoteResult{}, err
	}
	now := time.Now().UTC()
	task := &model.Task{
		ID: quote.TaskID, UserID: userID, ProjectID: projectID, Type: model.FilmVisualQCTaskTypeVideo,
		Status: model.TaskStatusQueued, Stage: "等待队列调度", Progress: 5, Prompt: prompt,
		Operation: "film_video_visual_qc", Provider: "managed", Model: quote.Model,
		LogicalModelID: quote.LogicalModelID, LogicalModelRevisionID: quote.LogicalModelRevisionID,
		RouteID: quote.RouteID, ChannelModelID: quote.ChannelModelID, RouteRun: 1, InputJSON: string(inputJSON),
	}
	command := repository.FilmVideoVisualQCSubmitCommand{
		UserID: userID, ProjectID: projectID, QuoteID: quote.ID, QuoteFingerprint: strings.TrimSpace(request.QuoteFingerprint),
		IdempotencyKey: idempotencyKey, AttemptID: attemptID, Task: task, ActiveTaskLimit: policy.Task.ActiveTaskLimit, At: now,
		Event: repository.AgentRuntimeEventInput{
			ID: newID(), EventType: "film.production.video.visual_qc.submitted", ActorType: "human", ActorID: userID,
			PayloadJSON: mustFilmJSON(map[string]any{"quoteId": quote.ID, "taskId": quote.TaskID, "sourceAttemptId": quote.SourceAttemptID, "sourceResultId": quote.SourceResultID}),
		},
	}
	stored, err := s.submitFilmVideoVisualQCWithinStorageQuota(command, policy)
	if err != nil {
		return SubmitFilmVideoVisualQCQuoteResult{}, mapFilmVideoVisualQCError(err)
	}
	view, err := s.filmVideoVisualQCAttemptView(repository.FilmVideoVisualQCAttemptDetail{Attempt: stored.Attempt, Task: stored.Task, Billing: stored.Billing}, true)
	if err != nil {
		return SubmitFilmVideoVisualQCQuoteResult{}, err
	}
	return SubmitFilmVideoVisualQCQuoteResult{Attempt: view, Idempotent: stored.Idempotent}, nil
}

func (s *Service) resolveFilmVideoVisualQCSource(userID string, projectID string, attemptID string) (filmVideoVisualQCSource, error) {
	detail, err := s.repo.FilmVideoAttemptForUser(userID, projectID, attemptID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return filmVideoVisualQCSource{}, NotFound("Film 视频 Attempt 不存在")
	}
	if err != nil {
		return filmVideoVisualQCSource{}, err
	}
	if detail.Attempt.Status != model.FilmProductionAttemptStatusSucceeded || detail.Result == nil {
		return filmVideoVisualQCSource{}, conflictError("只有已生成真实视频的 Attempt 才能运行视频视觉 QC")
	}
	sequence, err := s.repo.FilmVideoSequenceForUser(userID, projectID, detail.Attempt.SequenceID)
	if err != nil {
		return filmVideoVisualQCSource{}, err
	}
	var slot *repository.FilmVideoSlotDetail
	for index := range sequence.Slots {
		if sequence.Slots[index].Slot.ID == detail.Attempt.SlotID {
			slot = &sequence.Slots[index]
			break
		}
	}
	if slot == nil || slot.Slot.CurrentAttemptID != detail.Attempt.ID || slot.Slot.ResultID != detail.Result.ID {
		return filmVideoVisualQCSource{}, conflictError("当前视频槽位已切换版本，请选择现行视频后重试")
	}
	media, err := filmVideoMediaFactFromResult([]byte(detail.Result.Payload))
	if err != nil || strings.TrimSpace(media.ResourceID) == "" {
		return filmVideoVisualQCSource{}, conflictError("生成视频缺少可持久引用的资源，不能运行视频视觉 QC")
	}
	videoResource, err := s.repo.ResourceForUser(userID, media.ResourceID)
	if err != nil {
		return filmVideoVisualQCSource{}, err
	}
	if videoResource.Status != model.ResourceStatusReady || videoResource.Kind != "video" || !strings.HasPrefix(strings.ToLower(videoResource.MimeType), "video/") {
		return filmVideoVisualQCSource{}, conflictError("生成视频资源当前不可用于视频视觉 QC")
	}
	imageResource, err := s.repo.ResourceForUser(userID, slot.Slot.SourceImageResourceID)
	if err != nil {
		return filmVideoVisualQCSource{}, err
	}
	if imageResource.Status != model.ResourceStatusReady || imageResource.Kind != "image" {
		return filmVideoVisualQCSource{}, conflictError("视频来源图片当前不可用于身份与场景对照")
	}
	resultArtifact, resultRevision, err := s.repo.ProductionArtifactRevisionForUser(userID, detail.Attempt.ResultRevisionID)
	if err != nil || resultArtifact.ID != detail.Attempt.ResultArtifactID || resultArtifact.ProjectID != projectID || resultArtifact.Domain != "film" ||
		resultArtifact.ArtifactType != "generation-result" || resultArtifact.CurrentRevisionID != resultRevision.ID || resultRevision.Status != model.ProductionArtifactStatusLocked {
		return filmVideoVisualQCSource{}, conflictError("生成视频 Result revision 已变化，请刷新后重试")
	}
	promptArtifact, promptRevision, err := s.repo.ProductionArtifactRevisionForUser(userID, sequence.Sequence.PromptArtifactRevisionID)
	if err != nil || promptArtifact.ID != sequence.Sequence.PromptArtifactID || promptRevision.ContentDigest != sequence.Sequence.PromptArtifactDigest || promptRevision.Status != model.ProductionArtifactStatusLocked {
		return filmVideoVisualQCSource{}, conflictError("视频视觉 QC 的 Prompt 证据已变化")
	}
	imageArtifact, imageRevision, err := s.repo.ProductionArtifactRevisionForUser(userID, slot.Slot.SourceImageRevisionID)
	if err != nil || imageArtifact.ID != slot.Slot.SourceImageArtifactID || imageArtifact.CurrentRevisionID != imageRevision.ID || imageRevision.Status != model.ProductionArtifactStatusLocked {
		return filmVideoVisualQCSource{}, conflictError("视频视觉 QC 的来源图片证据已变化")
	}
	durationMs := firstPositiveInt64(media.DurationMs, videoResource.DurationMs, slot.Slot.DurationMs)
	return filmVideoVisualQCSource{
		Detail: *detail, Sequence: *sequence, Slot: *slot, VideoResource: *videoResource, SourceImageResource: *imageResource,
		ResultArtifact: *resultArtifact, ResultRevision: *resultRevision, PromptRevision: *promptRevision,
		SourceImageRevision: *imageRevision, DurationMs: durationMs,
	}, nil
}

func (s *Service) resolveFilmVideoVisualQCSamples(userID string, source filmVideoVisualQCSource, input []FilmVideoVisualQCSampleInput) ([]providerMedia, []string, []FilmVideoVisualQCSample, error) {
	if len(input) < 3 || len(input) > 8 {
		return nil, nil, nil, BadAuthRequest("视频视觉 QC 必须提供 3-8 张固定时间点采样帧")
	}
	durationMs := source.DurationMs
	if durationMs <= 0 {
		return nil, nil, nil, conflictError("无法确定来源视频时长")
	}
	edgeTolerance := durationMs / 20
	if edgeTolerance < 250 {
		edgeTolerance = 250
	}
	samples := make([]FilmVideoVisualQCSample, 0, len(input))
	ids := make([]string, 0, len(input)+1)
	seen := make(map[string]bool, len(input))
	previousTime := int64(-1)
	for _, item := range input {
		resourceID := strings.TrimSpace(item.ResourceID)
		if resourceID == "" || seen[resourceID] || item.TimeMs < 0 || item.TimeMs > durationMs || item.TimeMs <= previousTime {
			return nil, nil, nil, BadAuthRequest("采样帧必须按时间递增、资源不重复，并位于视频时长范围内")
		}
		resource, err := s.repo.ResourceForUser(userID, resourceID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, NotFound("视频采样帧不存在或不属于当前用户")
		}
		if err != nil {
			return nil, nil, nil, err
		}
		if resource.Status != model.ResourceStatusReady || resource.Kind != "image" || !strings.HasPrefix(strings.ToLower(resource.MimeType), "image/") {
			return nil, nil, nil, BadAuthRequest("视频采样证据必须是已上传完成的图片")
		}
		seen[resourceID] = true
		previousTime = item.TimeMs
		ids = append(ids, resourceID)
		samples = append(samples, FilmVideoVisualQCSample{
			TimeMs: item.TimeMs, ResourceID: resource.ID, MimeType: resource.MimeType, Size: resource.Size,
			Width: resource.Width, Height: resource.Height, ETag: resource.ETag,
		})
	}
	if samples[0].TimeMs > edgeTolerance || samples[len(samples)-1].TimeMs < durationMs-edgeTolerance {
		return nil, nil, nil, BadAuthRequest("采样帧必须覆盖视频开头与结尾")
	}
	ids = append(ids, source.SourceImageResource.ID)
	references, normalizedIDs, err := s.resolveFilmProductionImageReferences(userID, ids)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(normalizedIDs) != len(ids) {
		return nil, nil, nil, conflictError("视频采样帧或来源图片存在重复资源")
	}
	return references, normalizedIDs, samples, nil
}

func (s *Service) buildFilmVideoVisualQCPrompt(source filmVideoVisualQCSource, samples []FilmVideoVisualQCSample, referenceIDs []string) (string, string, error) {
	agent, ok := s.filmAgentRegistry.Agent("quality_control_editor")
	if !ok {
		return "", "", errors.New("Film AgentTeam 缺少 quality_control_editor")
	}
	var system strings.Builder
	system.WriteString("You are executing a paid, evidence-bound visual semantic QC review for a Film production video using ordered sampled frames. Never infer continuous motion or audio facts that the samples cannot prove. The sampled frames are client-browser captures bound to an immutable video result revision, but they are not server-native decode proof. The last attached image is the accepted source-image reference. A model verdict is advisory and must never authorize final acceptance.\n\n")
	system.WriteString("[AGENT quality_control_editor]\n" + agent.DeveloperInstructions + "\n")
	for _, skillID := range filmVisualQCSkillIDs {
		skill, ok := s.filmAgentRegistry.Skill(skillID)
		if !ok || !filmContainsString(skill.OwnerAgentIDs, agent.ID) {
			return "", "", fmt.Errorf("quality_control_editor cannot execute Skill %s", skillID)
		}
		system.WriteString("\n[SKILL " + skill.ID + " v" + skill.Version + "]\n" + skill.Instructions + "\n")
	}
	system.WriteString("\n[VIDEO VISUAL QC OUTPUT CONTRACT]\nReturn exactly one JSON object with no markdown fence and no prose outside it. Allowed keys are schemaVersion, overallDecision, dimensions, and summary. schemaVersion must be 1. overallDecision must be PASS, UNCERTAIN, or FAIL. dimensions must contain exactly once and in this order: identity, anatomy, contact, acting, props, scene, composition, motion, temporal_continuity, lip_sync_audio. Every dimension object must contain only dimension, decision, issueCodes, observations, and rationale. decision uses PASS, UNCERTAIN, or FAIL. Every non-PASS dimension needs at least one uppercase issue code. observations is a non-empty array of concise visible facts. For motion and temporal_continuity, judge only transitions supported by adjacent samples. For lip_sync_audio, use UNCERTAIN when actual audio or sufficiently dense mouth-motion evidence is absent unless the locked prompt proves no speech/audio synchronization is required. overallDecision is FAIL if any dimension FAILs, otherwise UNCERTAIN if any dimension is UNCERTAIN, otherwise PASS. Do not emit an acceptance action.\n")
	roles := make([]map[string]any, 0, len(referenceIDs))
	for index, sample := range samples {
		roles = append(roles, map[string]any{"imageIndex": index + 1, "role": "video_sample", "timeMs": sample.TimeMs, "resourceId": sample.ResourceID})
	}
	roles = append(roles, map[string]any{"imageIndex": len(samples) + 1, "role": "source_image_reference", "resourceId": source.SourceImageResource.ID})
	payload := map[string]any{
		"schemaVersion": 1,
		"target": map[string]any{
			"sourceAttemptId": source.Detail.Attempt.ID, "sourceResultId": source.Detail.Result.ID, "sequenceId": source.Detail.Attempt.SequenceID,
			"slotId": source.Detail.Attempt.SlotID, "shotId": source.Slot.Slot.ShotID, "durationMs": source.DurationMs,
		},
		"imageRoles": roles,
		"samplingEvidence": map[string]any{
			"method": "client_browser_capture", "sourceResultRevisionId": source.ResultRevision.ID,
			"sourceResultDigest": source.ResultRevision.ContentDigest, "provenanceVerifiedByServerDecoder": false,
			"limitation": "Frame resources and timestamps are frozen, but the backend did not decode the source video to independently prove each frame origin.",
		},
		"lockedEvidence": map[string]any{
			"videoPrompt": map[string]any{"revisionId": source.PromptRevision.ID, "digest": source.PromptRevision.ContentDigest, "content": json.RawMessage(source.PromptRevision.ContentJSON)},
			"sourceImage": map[string]any{"revisionId": source.SourceImageRevision.ID, "contentDigest": source.SourceImageRevision.ContentDigest},
		},
		"requiredDimensions": filmVideoVisualQCDimensions,
	}
	promptJSON, err := json.Marshal(payload)
	if err != nil {
		return "", "", err
	}
	if len(promptJSON)+system.Len() > filmVisualQCPromptLimit {
		return "", "", BadAuthRequest("视频视觉 QC 的锁定证据超过 1MiB，请缩小制作产物后重试")
	}
	return system.String(), string(promptJSON), nil
}

func filmVideoVisualQCSubmissionInput(quote model.FilmVideoVisualQCQuote, attemptID string) (map[string]any, string, error) {
	var input map[string]any
	if json.Unmarshal([]byte(quote.RequestJSON), &input) != nil {
		return nil, "", conflictError("Film 视频视觉 QC 报价请求快照已损坏")
	}
	prompt, _ := input["prompt"].(string)
	if strings.TrimSpace(prompt) == "" {
		return nil, "", conflictError("Film 视频视觉 QC 报价缺少 Prompt")
	}
	metadata, _ := input["metadata"].(map[string]any)
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["filmVideoVisualQCQuoteId"] = quote.ID
	metadata["filmVideoVisualQCAttemptId"] = attemptID
	metadata["filmVideoVisualQCSourceAttemptId"] = quote.SourceAttemptID
	metadata["filmVideoVisualQCSourceResultId"] = quote.SourceResultID
	input["metadata"] = metadata
	return input, prompt, nil
}

func filmVideoVisualQCRequestFingerprint(projectID string, source filmVideoVisualQCSource, logicalModelID string, samples []FilmVideoVisualQCSample, referenceIDs []string, systemPrompt string, prompt string) (string, error) {
	encoded, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "projectId": projectID, "rootRunId": source.Detail.Attempt.RootRunID, "sequenceId": source.Detail.Attempt.SequenceID,
		"sequenceRevision": source.Sequence.Sequence.Revision, "slotId": source.Detail.Attempt.SlotID, "slotRevision": source.Slot.Slot.Revision,
		"sourceAttemptId": source.Detail.Attempt.ID, "sourceResultId": source.Detail.Result.ID, "sourceResultRevisionId": source.ResultRevision.ID,
		"sourceResultDigest": source.ResultRevision.ContentDigest, "promptRevisionId": source.PromptRevision.ID, "promptDigest": source.PromptRevision.ContentDigest,
		"sourceImageRevisionId": source.SourceImageRevision.ID, "registryDigest": source.Sequence.Sequence.RegistryDigest, "logicalModelId": logicalModelID,
		"sampleFrames": samples, "referenceResourceIds": referenceIDs, "systemPromptDigest": digestString(systemPrompt), "promptDigestV2": digestString(prompt),
	})
	if err != nil {
		return "", err
	}
	return digestBytesHex(encoded), nil
}

func filmVideoVisualQCQuoteFingerprint(quote model.FilmVideoVisualQCQuote) (string, error) {
	encoded, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "quoteId": quote.ID, "taskId": quote.TaskID, "projectId": quote.ProjectID, "rootRunId": quote.RootRunID,
		"sequenceId": quote.SequenceID, "sequenceRevision": quote.SequenceRevision, "slotId": quote.SlotID, "slotRevision": quote.SlotRevision,
		"sourceAttemptId": quote.SourceAttemptID, "sourceResultId": quote.SourceResultID, "sourceResultRevisionId": quote.SourceResultRevisionID,
		"sourceResultDigest": quote.SourceResultDigest, "sampleFrames": json.RawMessage(quote.SampleFramesJSON), "registryDigest": quote.RegistryDigest,
		"logicalModelId": quote.LogicalModelID, "logicalModelRevisionId": quote.LogicalModelRevisionID, "routeId": quote.RouteID,
		"channelModelId": quote.ChannelModelID, "capabilityVersion": quote.CapabilityVersion, "channelPriceVersion": quote.ChannelPriceVersion,
		"request": json.RawMessage(quote.RequestJSON), "requestFingerprint": quote.RequestFingerprint,
		"billing": json.RawMessage(firstNonEmpty(quote.BillingJSON, "null")), "expiresAt": quote.ExpiresAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return "", err
	}
	return digestBytesHex(encoded), nil
}

func filmVideoVisualQCQuoteView(quote model.FilmVideoVisualQCQuote, idempotent bool) (FilmVideoVisualQCQuoteView, error) {
	var samples []FilmVideoVisualQCSample
	if json.Unmarshal([]byte(quote.SampleFramesJSON), &samples) != nil {
		return FilmVideoVisualQCQuoteView{}, errors.New("Film video visual QC sample evidence is invalid")
	}
	billing, err := filmVideoVisualQCQuoteBilling(quote)
	if err != nil {
		return FilmVideoVisualQCQuoteView{}, err
	}
	status := quote.Status
	if status == model.FilmProductionQuoteStatusPending && !quote.ExpiresAt.After(time.Now()) {
		status = model.FilmProductionQuoteStatusExpired
	}
	return FilmVideoVisualQCQuoteView{
		ID: quote.ID, ProjectID: quote.ProjectID, RootRunID: quote.RootRunID, SequenceID: quote.SequenceID, SlotID: quote.SlotID, ShotID: quote.ShotID,
		SourceAttemptID: quote.SourceAttemptID, SourceResultID: quote.SourceResultID, LogicalModelID: quote.LogicalModelID, Model: quote.Model,
		SampleFrames: samples, Dimensions: append([]string(nil), filmVideoVisualQCDimensions...),
		EvidenceLimitation: "采样帧来自浏览器并已冻结，但后端未独立解码视频证明每帧来源。",
		Cost:               filmProductionCostView(billing), QuoteFingerprint: quote.QuoteFingerprint, RequestFingerprint: quote.RequestFingerprint,
		Status: status, ExpiresAt: quote.ExpiresAt, CreatedAt: quote.CreatedAt, Idempotent: idempotent,
	}, nil
}

func filmVideoVisualQCQuoteBilling(quote model.FilmVideoVisualQCQuote) (*model.BillingOrder, error) {
	if strings.TrimSpace(quote.BillingJSON) == "" || strings.TrimSpace(quote.BillingJSON) == "null" {
		return nil, nil
	}
	var order model.BillingOrder
	if err := json.Unmarshal([]byte(quote.BillingJSON), &order); err != nil {
		return nil, err
	}
	return &order, nil
}

func (s *Service) filmVideoVisualQCAttemptView(detail repository.FilmVideoVisualQCAttemptDetail, valid bool) (FilmVideoVisualQCAttemptView, error) {
	task := detail.Task
	s.hydrateTaskProviderRequestID(&task)
	var report *FilmVideoQCView
	if detail.Report != nil {
		view := filmVideoQCView(*detail.Report)
		report = &view
	}
	var samples []FilmVideoVisualQCSample
	if json.Unmarshal([]byte(firstNonEmpty(detail.Attempt.SampleFramesJSON, "[]")), &samples) != nil {
		return FilmVideoVisualQCAttemptView{}, errors.New("Film 视频视觉 QC 采样证据已损坏")
	}
	return FilmVideoVisualQCAttemptView{
		Attempt: detail.Attempt, Task: taskSummaryForOutput(task), Report: report, SampleFrames: samples,
		Cost: filmProductionCostView(detail.Billing), Valid: valid,
	}, nil
}

func (s *Service) submitFilmVideoVisualQCWithinStorageQuota(command repository.FilmVideoVisualQCSubmitCommand, policy RuntimePolicySetting) (*repository.FilmVideoVisualQCSubmitResult, error) {
	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	if existing, err := s.repo.FilmVideoVisualQCAttemptByIdempotency(command.UserID, command.IdempotencyKey); err == nil {
		if existing.ProjectID != command.ProjectID || existing.QuoteID != command.QuoteID {
			return nil, repository.ErrFilmProductionStateConflict
		}
		return s.repo.SubmitFilmVideoVisualQCQuote(command)
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
	return s.repo.SubmitFilmVideoVisualQCQuote(command)
}

func mapFilmVideoVisualQCError(err error) error {
	switch {
	case errors.Is(err, repository.ErrFilmProductionQuoteExpired):
		return conflictError("Film 视频视觉 QC 报价已过期，请重新采样并报价")
	case errors.Is(err, repository.ErrFilmProductionQuoteConsumed):
		return conflictError("Film 视频视觉 QC 报价已被其他提交使用")
	case errors.Is(err, repository.ErrFilmProductionQuoteConflict):
		return conflictError("Film 视频视觉 QC 报价指纹不匹配，请刷新后重试")
	case errors.Is(err, repository.ErrFilmProductionQuoteDrift):
		return conflictError("视频结果、采样帧、锁定产物、模型或价格已变化，请重新报价")
	case errors.Is(err, repository.ErrFilmProductionActiveAttempt):
		return conflictError("该视频已有正在执行或费用待核对的视觉 QC Attempt")
	case errors.Is(err, repository.ErrFilmProductionStateConflict):
		return conflictError("Film 视频视觉 QC 状态已变化，请刷新后重试")
	case errors.Is(err, repository.ErrFilmProductionMediaMissing):
		return conflictError("生成视频或采样帧不可访问，不能运行视频视觉 QC")
	case errors.Is(err, repository.ErrInsufficientCredits):
		return BadAuthRequest("积分不足，请先使用兑换码充值")
	case errors.Is(err, repository.ErrActiveTaskLimit):
		return BadAuthRequest("同时排队或运行的任务已达到上限，请等待已有任务完成")
	case errors.Is(err, gorm.ErrRecordNotFound):
		return NotFound("Film 视频视觉 QC 关联记录不存在")
	default:
		return err
	}
}

func firstPositiveInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
