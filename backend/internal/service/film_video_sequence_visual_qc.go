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

var filmVideoSequenceVisualQCDimensions = []string{
	"identity_continuity",
	"costume_continuity",
	"prop_continuity",
	"scene_continuity",
	"screen_direction_axis",
	"action_continuity",
	"lighting_color_continuity",
	"motion_transition",
	"lip_sync_audio_continuity",
}

type FilmVideoSequenceVisualQCSlotInput struct {
	SlotID       string                         `json:"slotId"`
	SampleFrames []FilmVideoVisualQCSampleInput `json:"sampleFrames"`
}

type CreateFilmVideoSequenceVisualQCQuoteRequest struct {
	SequenceID     string                               `json:"sequenceId"`
	LogicalModelID string                               `json:"logicalModelId"`
	Slots          []FilmVideoSequenceVisualQCSlotInput `json:"slots"`
}

type SubmitFilmVideoSequenceVisualQCQuoteRequest struct {
	QuoteFingerprint string `json:"quoteFingerprint"`
}

type FilmVideoSequenceVisualQCQuoteView struct {
	ID                 string                                        `json:"id"`
	ProjectID          string                                        `json:"projectId"`
	RootRunID          string                                        `json:"rootRunId"`
	SequenceID         string                                        `json:"sequenceId"`
	LedgerID           string                                        `json:"ledgerId"`
	LogicalModelID     string                                        `json:"logicalModelId"`
	Model              string                                        `json:"model"`
	SlotEvidence       []model.FilmVideoSequenceVisualQCSlotSnapshot `json:"slotEvidence"`
	Dimensions         []string                                      `json:"dimensions"`
	EvidenceLimitation string                                        `json:"evidenceLimitation"`
	Cost               FilmProductionCostView                        `json:"cost"`
	QuoteFingerprint   string                                        `json:"quoteFingerprint"`
	RequestFingerprint string                                        `json:"requestFingerprint"`
	Status             model.FilmProductionQuoteStatus               `json:"status"`
	ExpiresAt          time.Time                                     `json:"expiresAt"`
	CreatedAt          time.Time                                     `json:"createdAt"`
	Idempotent         bool                                          `json:"idempotent"`
}

type FilmVideoSequenceVisualQCAttemptView struct {
	Attempt      model.FilmVideoSequenceVisualQCAttempt        `json:"attempt"`
	Task         TaskSummary                                   `json:"task"`
	Report       *FilmVideoSequenceReviewView                  `json:"report,omitempty"`
	SlotEvidence []model.FilmVideoSequenceVisualQCSlotSnapshot `json:"slotEvidence"`
	Cost         FilmProductionCostView                        `json:"cost"`
	Valid        bool                                          `json:"valid"`
}

type SubmitFilmVideoSequenceVisualQCQuoteResult struct {
	Attempt    FilmVideoSequenceVisualQCAttemptView `json:"attempt"`
	Idempotent bool                                 `json:"idempotent"`
}

type filmVideoSequenceVisualQCSlotSource struct {
	Detail              repository.FilmVideoSlotDetail
	Attempt             repository.FilmVideoAttemptDetail
	VideoResource       model.Resource
	SourceImageResource model.Resource
	ResultArtifact      model.ProductionArtifact
	ResultRevision      model.ProductionArtifactRevision
	SourceImageRevision model.ProductionArtifactRevision
	DurationMs          int64
}

type filmVideoSequenceVisualQCSource struct {
	Detail         repository.FilmVideoSequenceDetail
	PromptRevision model.ProductionArtifactRevision
	LedgerRevision model.ProductionArtifactRevision
	Slots          []filmVideoSequenceVisualQCSlotSource
}

func (s *Service) CreateFilmVideoSequenceVisualQCQuote(userID string, projectID string, idempotencyKey string, request CreateFilmVideoSequenceVisualQCQuoteRequest) (FilmVideoSequenceVisualQCQuoteView, error) {
	if err := s.ValidateRuntime(); err != nil {
		return FilmVideoSequenceVisualQCQuoteView{}, err
	}
	if _, err := s.requireMutableFilmProject(userID, projectID); err != nil {
		return FilmVideoSequenceVisualQCQuoteView{}, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if !filmAgentIdempotencyKeyPattern.MatchString(idempotencyKey) {
		return FilmVideoSequenceVisualQCQuoteView{}, BadAuthRequest("X-Idempotency-Key 必须为 8-128 位字母、数字或 ._:-")
	}
	source, err := s.resolveFilmVideoSequenceVisualQCSource(userID, projectID, strings.TrimSpace(request.SequenceID))
	if err != nil {
		return FilmVideoSequenceVisualQCQuoteView{}, err
	}
	references, referenceIDs, evidence, err := s.resolveFilmVideoSequenceVisualQCSamples(userID, source, request.Slots)
	if err != nil {
		return FilmVideoSequenceVisualQCQuoteView{}, err
	}
	systemPrompt, prompt, err := s.buildFilmVideoSequenceVisualQCPrompt(source, evidence)
	if err != nil {
		return FilmVideoSequenceVisualQCQuoteView{}, err
	}
	logicalModelID := strings.TrimSpace(request.LogicalModelID)
	scopeFingerprint, err := filmVideoSequenceScopeFingerprint(source.Detail)
	if err != nil {
		return FilmVideoSequenceVisualQCQuoteView{}, err
	}
	requestFingerprint, err := filmVideoSequenceVisualQCRequestFingerprint(projectID, source, logicalModelID, scopeFingerprint, evidence, referenceIDs, systemPrompt, prompt)
	if err != nil {
		return FilmVideoSequenceVisualQCQuoteView{}, err
	}
	if existing, lookupErr := s.repo.FilmVideoSequenceVisualQCQuoteByIdempotency(userID, idempotencyKey); lookupErr == nil {
		if existing.ProjectID != projectID || existing.RequestFingerprint != requestFingerprint {
			return FilmVideoSequenceVisualQCQuoteView{}, conflictError("该幂等键已用于另一份整组视频连续性 QC 报价")
		}
		return filmVideoSequenceVisualQCQuoteView(*existing, true)
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return FilmVideoSequenceVisualQCQuoteView{}, lookupErr
	}
	if logicalModelID == "" {
		return FilmVideoSequenceVisualQCQuoteView{}, BadAuthRequest("请选择支持足够图片输入的文本逻辑模型")
	}
	input := map[string]any{
		"mode": "text", "prompt": prompt, "referenceImages": references,
		"config": map[string]any{"systemPrompt": systemPrompt},
		"metadata": map[string]any{
			"domain": "film", "filmRootRunId": source.Detail.Sequence.RootRunID,
			"filmVideoSequenceId":                       source.Detail.Sequence.ID,
			"filmVideoSequenceVisualQCScopeFingerprint": scopeFingerprint,
		},
	}
	intent := ModelRequestIntentFromTaskInput(input, model.FilmVisualQCTaskTypeSequence, "film_video_sequence_visual_qc")
	routed, err := s.ResolveLogicalModel(logicalModelID, intent)
	if err != nil {
		return FilmVideoSequenceVisualQCQuoteView{}, err
	}
	if routed.LogicalModel.Capability != "text" || routed.ChannelModel.Capability != "text" {
		return FilmVideoSequenceVisualQCQuoteView{}, BadAuthRequest("整组视频连续性 QC 必须使用文本多模态模型")
	}
	if routed.ChannelModel.Protocol != model.ChannelInterfaceChatCompletion && routed.ChannelModel.Protocol != model.ChannelInterfaceOpenAIResponse {
		return FilmVideoSequenceVisualQCQuoteView{}, BadAuthRequest("整组视频连续性 QC 当前只支持 Chat Completions 或 Responses 多模态协议")
	}
	input = applyRoutedProviderSelection(input, routed)
	if err := s.ValidateTaskCapability(input); err != nil {
		return FilmVideoSequenceVisualQCQuoteView{}, err
	}
	if containsInlineMediaDataURL(input) || containsAgentRuntimeSecret(input) {
		return FilmVideoSequenceVisualQCQuoteView{}, BadAuthRequest("整组视频连续性 QC 只能引用已上传采样帧，且不能包含密钥")
	}
	if err := s.protectTaskSecrets(input); err != nil {
		return FilmVideoSequenceVisualQCQuoteView{}, err
	}
	requestJSON, err := json.Marshal(input)
	if err != nil {
		return FilmVideoSequenceVisualQCQuoteView{}, err
	}
	now := time.Now().UTC()
	quoteID, taskID := newID(), newID()
	task := model.Task{
		ID: taskID, UserID: userID, ProjectID: projectID, Type: model.FilmVisualQCTaskTypeSequence,
		Status: model.TaskStatusQueued, Stage: "等待用户确认整组连续性 QC 费用", Progress: 0, Prompt: prompt,
		Operation: "film_video_sequence_visual_qc", Provider: "managed", Model: routed.LogicalModel.Code,
		LogicalModelID: routed.LogicalModel.ID, LogicalModelRevisionID: routed.Revision.ID,
		RouteID: routed.Route.ID, ChannelModelID: routed.ChannelModel.ID, RouteRun: 1, InputJSON: string(requestJSON),
	}
	billing, err := s.taskBillingOrder(userID, &task, input)
	if err != nil {
		return FilmVideoSequenceVisualQCQuoteView{}, err
	}
	if billing != nil {
		billing.IdempotencyKey = "film-video-sequence-visual-qc:" + quoteID
	}
	billingJSON := ""
	if billing != nil {
		encoded, marshalErr := json.Marshal(billing)
		if marshalErr != nil {
			return FilmVideoSequenceVisualQCQuoteView{}, marshalErr
		}
		billingJSON = string(encoded)
	}
	slotEvidenceJSON, _ := json.Marshal(evidence)
	referenceIDsJSON, _ := json.Marshal(referenceIDs)
	ledger := source.Detail.Continuity.Ledger
	quote := model.FilmVideoSequenceVisualQCQuote{
		ID: quoteID, UserID: userID, IdempotencyKey: idempotencyKey, ProjectID: projectID, RootRunID: source.Detail.Sequence.RootRunID,
		SequenceID: source.Detail.Sequence.ID, SequenceRevision: source.Detail.Sequence.Revision, LedgerID: ledger.ID,
		LedgerArtifactID: ledger.ArtifactID, LedgerRevisionID: source.LedgerRevision.ID, LedgerDigest: source.LedgerRevision.ContentDigest,
		ScopeFingerprint: scopeFingerprint, PromptArtifactID: source.Detail.Sequence.PromptArtifactID, PromptRevisionID: source.PromptRevision.ID,
		PromptDigest: source.PromptRevision.ContentDigest, RegistryID: source.Detail.Sequence.RegistryID, RegistryVersion: source.Detail.Sequence.RegistryVersion,
		RegistryDigest: source.Detail.Sequence.RegistryDigest, LogicalModelID: routed.LogicalModel.ID, LogicalModelRevisionID: routed.Revision.ID,
		RouteID: routed.Route.ID, ChannelID: routed.ChannelModel.ChannelID, ChannelModelID: routed.ChannelModel.ID, Model: routed.LogicalModel.Code,
		ProviderModel: routed.ChannelModel.ModelKey, Protocol: routed.ChannelModel.Protocol, CapabilityVersion: routed.ChannelModel.CapabilityVersion,
		ChannelPriceVersion: routed.ChannelModel.PriceVersion, SlotEvidenceJSON: string(slotEvidenceJSON), ReferenceResourceIDsJSON: string(referenceIDsJSON),
		TaskID: taskID, RequestJSON: string(requestJSON), RequestFingerprint: requestFingerprint, BillingJSON: billingJSON,
		Status: model.FilmProductionQuoteStatusPending, ExpiresAt: now.Add(filmProductionQuoteTTL), CreatedAt: now, UpdatedAt: now,
	}
	quote.QuoteFingerprint, err = filmVideoSequenceVisualQCQuoteFingerprint(quote)
	if err != nil {
		return FilmVideoSequenceVisualQCQuoteView{}, err
	}
	if err := s.repo.CreateFilmVideoSequenceVisualQCQuote(repository.FilmVideoSequenceVisualQCQuoteCreateCommand{Quote: &quote, At: now}); err != nil {
		return FilmVideoSequenceVisualQCQuoteView{}, mapFilmVideoSequenceVisualQCError(err)
	}
	return filmVideoSequenceVisualQCQuoteView(quote, false)
}

func (s *Service) SubmitFilmVideoSequenceVisualQCQuote(userID string, projectID string, quoteID string, idempotencyKey string, request SubmitFilmVideoSequenceVisualQCQuoteRequest) (SubmitFilmVideoSequenceVisualQCQuoteResult, error) {
	if _, err := s.requireMutableFilmProject(userID, projectID); err != nil {
		return SubmitFilmVideoSequenceVisualQCQuoteResult{}, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if !filmAgentIdempotencyKeyPattern.MatchString(idempotencyKey) {
		return SubmitFilmVideoSequenceVisualQCQuoteResult{}, BadAuthRequest("X-Idempotency-Key 必须为 8-128 位字母、数字或 ._:-")
	}
	quote, err := s.repo.FilmVideoSequenceVisualQCQuoteForUser(userID, projectID, strings.TrimSpace(quoteID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SubmitFilmVideoSequenceVisualQCQuoteResult{}, NotFound("整组视频连续性 QC 报价不存在")
	}
	if err != nil {
		return SubmitFilmVideoSequenceVisualQCQuoteResult{}, err
	}
	promptInput, prompt, err := filmVideoSequenceVisualQCSubmissionInput(*quote, newID())
	if err != nil {
		return SubmitFilmVideoSequenceVisualQCQuoteResult{}, err
	}
	attemptID := promptInput["metadata"].(map[string]any)["filmVideoSequenceVisualQCAttemptId"].(string)
	if err := s.protectTaskSecrets(promptInput); err != nil {
		return SubmitFilmVideoSequenceVisualQCQuoteResult{}, err
	}
	inputJSON, err := json.Marshal(promptInput)
	if err != nil {
		return SubmitFilmVideoSequenceVisualQCQuoteResult{}, err
	}
	policy, err := s.RuntimePolicy()
	if err != nil {
		return SubmitFilmVideoSequenceVisualQCQuoteResult{}, err
	}
	now := time.Now().UTC()
	task := &model.Task{
		ID: quote.TaskID, UserID: userID, ProjectID: projectID, Type: model.FilmVisualQCTaskTypeSequence,
		Status: model.TaskStatusQueued, Stage: "等待队列调度", Progress: 5, Prompt: prompt,
		Operation: "film_video_sequence_visual_qc", Provider: "managed", Model: quote.Model,
		LogicalModelID: quote.LogicalModelID, LogicalModelRevisionID: quote.LogicalModelRevisionID,
		RouteID: quote.RouteID, ChannelModelID: quote.ChannelModelID, RouteRun: 1, InputJSON: string(inputJSON),
	}
	command := repository.FilmVideoSequenceVisualQCSubmitCommand{
		UserID: userID, ProjectID: projectID, QuoteID: quote.ID, QuoteFingerprint: strings.TrimSpace(request.QuoteFingerprint),
		IdempotencyKey: idempotencyKey, AttemptID: attemptID, Task: task, ActiveTaskLimit: policy.Task.ActiveTaskLimit, At: now,
		Event: repository.AgentRuntimeEventInput{
			ID: newID(), EventType: "film.production.video.sequence.visual_qc.submitted", ActorType: "human", ActorID: userID,
			PayloadJSON: mustFilmJSON(map[string]any{"quoteId": quote.ID, "taskId": quote.TaskID, "sequenceId": quote.SequenceID, "scopeFingerprint": quote.ScopeFingerprint}),
		},
	}
	stored, err := s.submitFilmVideoSequenceVisualQCWithinStorageQuota(command, policy)
	if err != nil {
		return SubmitFilmVideoSequenceVisualQCQuoteResult{}, mapFilmVideoSequenceVisualQCError(err)
	}
	sequence, err := s.repo.FilmVideoSequenceForUser(userID, projectID, quote.SequenceID)
	if err != nil {
		return SubmitFilmVideoSequenceVisualQCQuoteResult{}, err
	}
	view, err := s.filmVideoSequenceVisualQCAttemptView(repository.FilmVideoSequenceVisualQCAttemptDetail{Attempt: stored.Attempt, Task: stored.Task, Billing: stored.Billing}, *sequence)
	if err != nil {
		return SubmitFilmVideoSequenceVisualQCQuoteResult{}, err
	}
	return SubmitFilmVideoSequenceVisualQCQuoteResult{Attempt: view, Idempotent: stored.Idempotent}, nil
}

func (s *Service) resolveFilmVideoSequenceVisualQCSource(userID string, projectID string, sequenceID string) (filmVideoSequenceVisualQCSource, error) {
	detail, err := s.repo.FilmVideoSequenceForUser(userID, projectID, sequenceID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return filmVideoSequenceVisualQCSource{}, NotFound("视频序列不存在")
	}
	if err != nil {
		return filmVideoSequenceVisualQCSource{}, err
	}
	if !filmVideoSequenceHasAcceptedSlots(*detail) {
		return filmVideoSequenceVisualQCSource{}, conflictError("所有视频槽位通过人工验收后，才能运行整组模型连续性 QC")
	}
	if detail.Continuity == nil {
		return filmVideoSequenceVisualQCSource{}, conflictError("视频序列缺少 Continuity Ledger")
	}
	promptArtifact, promptRevision, err := s.repo.ProductionArtifactRevisionForUser(userID, detail.Sequence.PromptArtifactRevisionID)
	if err != nil || promptArtifact.ID != detail.Sequence.PromptArtifactID || promptArtifact.CurrentRevisionID != promptRevision.ID ||
		promptRevision.ContentDigest != detail.Sequence.PromptArtifactDigest || promptRevision.Status != model.ProductionArtifactStatusLocked {
		return filmVideoSequenceVisualQCSource{}, conflictError("整组连续性 QC 的视频 Prompt 证据已变化")
	}
	ledger := detail.Continuity.Ledger
	ledgerArtifact, ledgerRevision, err := s.repo.ProductionArtifactRevisionForUser(userID, ledger.ArtifactRevisionID)
	if err != nil || ledgerArtifact.ID != ledger.ArtifactID || ledgerArtifact.CurrentRevisionID != ledgerRevision.ID ||
		ledgerArtifact.ArtifactType != "continuity-ledger" || ledgerRevision.Status != model.ProductionArtifactStatusLocked {
		return filmVideoSequenceVisualQCSource{}, conflictError("整组连续性 QC 的 Ledger 证据已变化")
	}
	source := filmVideoSequenceVisualQCSource{Detail: *detail, PromptRevision: *promptRevision, LedgerRevision: *ledgerRevision}
	for _, slot := range detail.Slots {
		var current *repository.FilmVideoAttemptDetail
		for index := range slot.Attempts {
			if slot.Attempts[index].Attempt.ID == slot.Slot.CurrentAttemptID {
				current = &slot.Attempts[index]
				break
			}
		}
		if current == nil || current.Result == nil || current.Attempt.Status != model.FilmProductionAttemptStatusSucceeded || current.Result.ID != slot.Slot.ResultID {
			return filmVideoSequenceVisualQCSource{}, conflictError("视频序列包含不可恢复的已接受槽位")
		}
		media, err := filmVideoMediaFactFromResult([]byte(current.Result.Payload))
		if err != nil || strings.TrimSpace(media.ResourceID) == "" {
			return filmVideoSequenceVisualQCSource{}, conflictError("已接受视频缺少可持久引用的资源")
		}
		videoResource, err := s.repo.ResourceForUser(userID, media.ResourceID)
		if err != nil || videoResource.Status != model.ResourceStatusReady || videoResource.Kind != "video" || !strings.HasPrefix(strings.ToLower(videoResource.MimeType), "video/") {
			return filmVideoSequenceVisualQCSource{}, conflictError("已接受视频资源当前不可用于整组连续性 QC")
		}
		sourceImageResource, err := s.repo.ResourceForUser(userID, slot.Slot.SourceImageResourceID)
		if err != nil || sourceImageResource.Status != model.ResourceStatusReady || sourceImageResource.Kind != "image" {
			return filmVideoSequenceVisualQCSource{}, conflictError("视频来源图片当前不可用于整组连续性 QC")
		}
		resultArtifact, resultRevision, err := s.repo.ProductionArtifactRevisionForUser(userID, current.Attempt.ResultRevisionID)
		if err != nil || resultArtifact.ID != current.Attempt.ResultArtifactID || resultArtifact.CurrentRevisionID != resultRevision.ID ||
			resultArtifact.ArtifactType != "generation-result" || resultRevision.Status != model.ProductionArtifactStatusLocked {
			return filmVideoSequenceVisualQCSource{}, conflictError("已接受视频 Result revision 已变化")
		}
		imageArtifact, imageRevision, err := s.repo.ProductionArtifactRevisionForUser(userID, slot.Slot.SourceImageRevisionID)
		if err != nil || imageArtifact.ID != slot.Slot.SourceImageArtifactID || imageArtifact.CurrentRevisionID != imageRevision.ID ||
			imageArtifact.ArtifactType != "generation-result" || imageRevision.Status != model.ProductionArtifactStatusLocked {
			return filmVideoSequenceVisualQCSource{}, conflictError("视频来源图片 revision 已变化")
		}
		durationMs := firstPositiveInt64(media.DurationMs, videoResource.DurationMs, slot.Slot.DurationMs)
		if durationMs <= 0 {
			return filmVideoSequenceVisualQCSource{}, conflictError("无法确定已接受视频时长")
		}
		source.Slots = append(source.Slots, filmVideoSequenceVisualQCSlotSource{
			Detail: slot, Attempt: *current, VideoResource: *videoResource, SourceImageResource: *sourceImageResource,
			ResultArtifact: *resultArtifact, ResultRevision: *resultRevision, SourceImageRevision: *imageRevision, DurationMs: durationMs,
		})
	}
	return source, nil
}

func (s *Service) resolveFilmVideoSequenceVisualQCSamples(userID string, source filmVideoSequenceVisualQCSource, input []FilmVideoSequenceVisualQCSlotInput) ([]providerMedia, []string, []model.FilmVideoSequenceVisualQCSlotSnapshot, error) {
	if len(input) != len(source.Slots) {
		return nil, nil, nil, BadAuthRequest("必须为视频序列的每个槽位提供采样帧")
	}
	inputBySlot := make(map[string][]FilmVideoVisualQCSampleInput, len(input))
	for _, slot := range input {
		slotID := strings.TrimSpace(slot.SlotID)
		if slotID == "" || inputBySlot[slotID] != nil {
			return nil, nil, nil, BadAuthRequest("整组视频采样槽位不能为空或重复")
		}
		inputBySlot[slotID] = slot.SampleFrames
	}
	allIDs := make([]string, 0, len(input)*2)
	seenResources := make(map[string]bool)
	evidence := make([]model.FilmVideoSequenceVisualQCSlotSnapshot, 0, len(source.Slots))
	for _, slot := range source.Slots {
		frames, ok := inputBySlot[slot.Detail.Slot.ID]
		if !ok || len(frames) < 2 || len(frames) > 4 {
			return nil, nil, nil, BadAuthRequest("每个视频槽位必须提供 2-4 张递增采样帧")
		}
		edgeTolerance := slot.DurationMs / 20
		if edgeTolerance < 250 {
			edgeTolerance = 250
		}
		previousTime := int64(-1)
		samples := make([]model.FilmVideoSequenceVisualQCSampleSnapshot, 0, len(frames))
		for _, frame := range frames {
			resourceID := strings.TrimSpace(frame.ResourceID)
			if resourceID == "" || seenResources[resourceID] || frame.TimeMs < 0 || frame.TimeMs > slot.DurationMs || frame.TimeMs <= previousTime {
				return nil, nil, nil, BadAuthRequest("整组采样帧必须按槽位和时间有序、资源不重复，并位于对应视频时长内")
			}
			resource, err := s.repo.ResourceForUser(userID, resourceID)
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil, nil, NotFound("整组视频采样帧不存在或不属于当前用户")
			}
			if err != nil {
				return nil, nil, nil, err
			}
			if resource.Status != model.ResourceStatusReady || resource.Kind != "image" || !strings.HasPrefix(strings.ToLower(resource.MimeType), "image/") {
				return nil, nil, nil, BadAuthRequest("整组视频采样证据必须是已上传完成的图片")
			}
			seenResources[resourceID] = true
			previousTime = frame.TimeMs
			allIDs = append(allIDs, resourceID)
			samples = append(samples, model.FilmVideoSequenceVisualQCSampleSnapshot{
				TimeMs: frame.TimeMs, ResourceID: resource.ID, MimeType: resource.MimeType, Size: resource.Size,
				Width: resource.Width, Height: resource.Height, ETag: resource.ETag,
			})
		}
		if samples[0].TimeMs > edgeTolerance || samples[len(samples)-1].TimeMs < slot.DurationMs-edgeTolerance {
			return nil, nil, nil, BadAuthRequest("每个视频槽位的采样帧必须覆盖开头与结尾")
		}
		evidence = append(evidence, model.FilmVideoSequenceVisualQCSlotSnapshot{
			Position: slot.Detail.Slot.Position, SlotID: slot.Detail.Slot.ID, SlotRevision: slot.Detail.Slot.Revision,
			ShotID: slot.Detail.Slot.ShotID, DurationMs: slot.DurationMs, SourceAttemptID: slot.Attempt.Attempt.ID,
			SourceResultID: slot.Attempt.Result.ID, SourceResourceID: slot.VideoResource.ID, SourceResourceMimeType: slot.VideoResource.MimeType,
			SourceResourceSize: slot.VideoResource.Size, SourceResourceDurationMs: slot.VideoResource.DurationMs, SourceResourceETag: slot.VideoResource.ETag,
			SourceResultArtifactID: slot.ResultArtifact.ID, SourceResultRevisionID: slot.ResultRevision.ID, SourceResultDigest: slot.ResultRevision.ContentDigest,
			SourceImageResourceID: slot.SourceImageResource.ID, SourceImageArtifactID: slot.Detail.Slot.SourceImageArtifactID,
			SourceImageRevisionID: slot.SourceImageRevision.ID, SourceImageRevisionDigest: slot.SourceImageRevision.ContentDigest, Samples: samples,
		})
	}
	if len(allIDs) > 120 {
		return nil, nil, nil, BadAuthRequest("整组视频采样帧不能超过 120 张，请按场景拆分序列")
	}
	references, normalizedIDs, err := s.resolveFilmProductionImageReferences(userID, allIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(normalizedIDs) != len(allIDs) {
		return nil, nil, nil, conflictError("整组视频采样帧存在重复资源")
	}
	return references, normalizedIDs, evidence, nil
}

func (s *Service) buildFilmVideoSequenceVisualQCPrompt(source filmVideoSequenceVisualQCSource, evidence []model.FilmVideoSequenceVisualQCSlotSnapshot) (string, string, error) {
	agent, ok := s.filmAgentRegistry.Agent("quality_control_editor")
	if !ok {
		return "", "", errors.New("Film AgentTeam 缺少 quality_control_editor")
	}
	var system strings.Builder
	system.WriteString("You are executing a paid, evidence-bound cross-shot continuity review over one ordered Film video sequence. Each shot is represented by browser-captured start/end frames. Never infer continuous motion, dialogue, audio, or frame provenance that the evidence cannot prove. Browser samples are frozen against immutable video revisions, but are not server-native decode proof. Your verdict is advisory and must never authorize final sequence acceptance.\n\n")
	system.WriteString("[AGENT quality_control_editor]\n" + agent.DeveloperInstructions + "\n")
	for _, skillID := range filmVisualQCSkillIDs {
		skill, ok := s.filmAgentRegistry.Skill(skillID)
		if !ok || !filmContainsString(skill.OwnerAgentIDs, agent.ID) {
			return "", "", fmt.Errorf("quality_control_editor cannot execute Skill %s", skillID)
		}
		system.WriteString("\n[SKILL " + skill.ID + " v" + skill.Version + "]\n" + skill.Instructions + "\n")
	}
	system.WriteString("\n[SEQUENCE VISUAL QC OUTPUT CONTRACT]\nReturn exactly one JSON object with no markdown fence and no prose outside it. Allowed top-level keys are schemaVersion, overallDecision, dimensions, transitions, and summary. schemaVersion must be 1. overallDecision must be PASS, UNCERTAIN, or FAIL. dimensions must contain exactly once and in the declared order: identity_continuity, costume_continuity, prop_continuity, scene_continuity, screen_direction_axis, action_continuity, lighting_color_continuity, motion_transition, lip_sync_audio_continuity. Every dimension object contains only dimension, decision, issueCodes, observations, and rationale. transitions must contain exactly one entry for every adjacent shot pair in order, and every entry contains only fromShotId, toShotId, decision, issueCodes, observations, and rationale. Decisions use PASS, UNCERTAIN, or FAIL. Every non-PASS dimension or transition needs at least one uppercase issue code. observations must be a non-empty array of concise visible facts. lip_sync_audio_continuity must be UNCERTAIN because no audio evidence is attached. overallDecision is FAIL if any dimension or transition FAILs, otherwise UNCERTAIN if any is UNCERTAIN, otherwise PASS. Do not emit an acceptance action.\n")
	roles := make([]map[string]any, 0)
	imageIndex := 1
	for _, slot := range evidence {
		for sampleIndex, sample := range slot.Samples {
			role := "middle"
			if sampleIndex == 0 {
				role = "start"
			} else if sampleIndex == len(slot.Samples)-1 {
				role = "end"
			}
			roles = append(roles, map[string]any{
				"imageIndex": imageIndex, "role": "video_sample_" + role, "position": slot.Position,
				"slotId": slot.SlotID, "shotId": slot.ShotID, "timeMs": sample.TimeMs, "resourceId": sample.ResourceID,
			})
			imageIndex++
		}
	}
	transitions := make([]map[string]any, 0, len(evidence)-1)
	for index := 1; index < len(evidence); index++ {
		transitions = append(transitions, map[string]any{"fromShotId": evidence[index-1].ShotID, "toShotId": evidence[index].ShotID})
	}
	payload := map[string]any{
		"schemaVersion": 1,
		"target":        map[string]any{"sequenceId": source.Detail.Sequence.ID, "ledgerId": source.Detail.Continuity.Ledger.ID, "slotCount": len(evidence)},
		"imageRoles":    roles,
		"samplingEvidence": map[string]any{
			"method": "client_browser_capture", "provenanceVerifiedByServerDecoder": false,
			"limitation": "Frame resources and timestamps are frozen, but the backend did not decode each source video to independently prove frame origin. No audio evidence is attached.",
		},
		"lockedEvidence": map[string]any{
			"videoPrompt":      map[string]any{"revisionId": source.PromptRevision.ID, "digest": source.PromptRevision.ContentDigest, "content": json.RawMessage(source.PromptRevision.ContentJSON)},
			"continuityLedger": map[string]any{"revisionId": source.LedgerRevision.ID, "digest": source.LedgerRevision.ContentDigest, "content": json.RawMessage(source.LedgerRevision.ContentJSON)},
			"slots":            evidence,
		},
		"requiredDimensions":  filmVideoSequenceVisualQCDimensions,
		"requiredTransitions": transitions,
	}
	promptJSON, err := json.Marshal(payload)
	if err != nil {
		return "", "", err
	}
	if len(promptJSON)+system.Len() > filmVisualQCPromptLimit {
		return "", "", BadAuthRequest("整组视频连续性 QC 的锁定证据超过 1MiB，请按场景拆分序列")
	}
	return system.String(), string(promptJSON), nil
}

func filmVideoSequenceVisualQCSubmissionInput(quote model.FilmVideoSequenceVisualQCQuote, attemptID string) (map[string]any, string, error) {
	var input map[string]any
	if json.Unmarshal([]byte(quote.RequestJSON), &input) != nil {
		return nil, "", conflictError("整组视频连续性 QC 报价请求快照已损坏")
	}
	prompt, _ := input["prompt"].(string)
	if strings.TrimSpace(prompt) == "" {
		return nil, "", conflictError("整组视频连续性 QC 报价缺少 Prompt")
	}
	metadata, _ := input["metadata"].(map[string]any)
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["filmVideoSequenceVisualQCQuoteId"] = quote.ID
	metadata["filmVideoSequenceVisualQCAttemptId"] = attemptID
	metadata["filmVideoSequenceId"] = quote.SequenceID
	input["metadata"] = metadata
	return input, prompt, nil
}

func filmVideoSequenceVisualQCRequestFingerprint(projectID string, source filmVideoSequenceVisualQCSource, logicalModelID string, scopeFingerprint string, evidence []model.FilmVideoSequenceVisualQCSlotSnapshot, referenceIDs []string, systemPrompt string, prompt string) (string, error) {
	encoded, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "projectId": projectID, "rootRunId": source.Detail.Sequence.RootRunID, "sequenceId": source.Detail.Sequence.ID,
		"sequenceRevision": source.Detail.Sequence.Revision, "ledgerRevisionId": source.LedgerRevision.ID, "ledgerDigest": source.LedgerRevision.ContentDigest,
		"promptRevisionId": source.PromptRevision.ID, "promptDigest": source.PromptRevision.ContentDigest, "registryDigest": source.Detail.Sequence.RegistryDigest,
		"logicalModelId": logicalModelID, "scopeFingerprint": scopeFingerprint, "slotEvidence": evidence, "referenceResourceIds": referenceIDs,
		"systemPromptDigest": digestString(systemPrompt), "promptDigestV2": digestString(prompt),
	})
	if err != nil {
		return "", err
	}
	return digestBytesHex(encoded), nil
}

func filmVideoSequenceVisualQCQuoteFingerprint(quote model.FilmVideoSequenceVisualQCQuote) (string, error) {
	encoded, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "quoteId": quote.ID, "taskId": quote.TaskID, "projectId": quote.ProjectID, "rootRunId": quote.RootRunID,
		"sequenceId": quote.SequenceID, "sequenceRevision": quote.SequenceRevision, "ledgerRevisionId": quote.LedgerRevisionID,
		"scopeFingerprint": quote.ScopeFingerprint, "slotEvidence": json.RawMessage(quote.SlotEvidenceJSON), "registryDigest": quote.RegistryDigest,
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

func filmVideoSequenceVisualQCQuoteView(quote model.FilmVideoSequenceVisualQCQuote, idempotent bool) (FilmVideoSequenceVisualQCQuoteView, error) {
	var evidence []model.FilmVideoSequenceVisualQCSlotSnapshot
	if json.Unmarshal([]byte(quote.SlotEvidenceJSON), &evidence) != nil {
		return FilmVideoSequenceVisualQCQuoteView{}, errors.New("整组视频连续性 QC 证据已损坏")
	}
	billing, err := filmVideoSequenceVisualQCQuoteBilling(quote)
	if err != nil {
		return FilmVideoSequenceVisualQCQuoteView{}, err
	}
	status := quote.Status
	if status == model.FilmProductionQuoteStatusPending && !quote.ExpiresAt.After(time.Now()) {
		status = model.FilmProductionQuoteStatusExpired
	}
	return FilmVideoSequenceVisualQCQuoteView{
		ID: quote.ID, ProjectID: quote.ProjectID, RootRunID: quote.RootRunID, SequenceID: quote.SequenceID, LedgerID: quote.LedgerID,
		LogicalModelID: quote.LogicalModelID, Model: quote.Model, SlotEvidence: evidence,
		Dimensions:         append([]string(nil), filmVideoSequenceVisualQCDimensions...),
		EvidenceLimitation: "采样帧来自浏览器并已冻结，但后端未独立解码视频证明每帧来源；未提供音频证据。",
		Cost:               filmProductionCostView(billing), QuoteFingerprint: quote.QuoteFingerprint, RequestFingerprint: quote.RequestFingerprint,
		Status: status, ExpiresAt: quote.ExpiresAt, CreatedAt: quote.CreatedAt, Idempotent: idempotent,
	}, nil
}

func filmVideoSequenceVisualQCQuoteBilling(quote model.FilmVideoSequenceVisualQCQuote) (*model.BillingOrder, error) {
	if strings.TrimSpace(quote.BillingJSON) == "" || strings.TrimSpace(quote.BillingJSON) == "null" {
		return nil, nil
	}
	var order model.BillingOrder
	if err := json.Unmarshal([]byte(quote.BillingJSON), &order); err != nil {
		return nil, err
	}
	return &order, nil
}

func (s *Service) filmVideoSequenceVisualQCAttemptView(detail repository.FilmVideoSequenceVisualQCAttemptDetail, sequence repository.FilmVideoSequenceDetail) (FilmVideoSequenceVisualQCAttemptView, error) {
	task := detail.Task
	s.hydrateTaskProviderRequestID(&task)
	var report *FilmVideoSequenceReviewView
	if detail.Report != nil {
		view, err := filmVideoSequenceReviewView(*detail.Report, sequence)
		if err != nil {
			return FilmVideoSequenceVisualQCAttemptView{}, err
		}
		report = &view
	}
	var evidence []model.FilmVideoSequenceVisualQCSlotSnapshot
	if json.Unmarshal([]byte(firstNonEmpty(detail.Attempt.SlotEvidenceJSON, "[]")), &evidence) != nil {
		return FilmVideoSequenceVisualQCAttemptView{}, errors.New("整组视频连续性 QC 采样证据已损坏")
	}
	currentScope, err := filmVideoSequenceScopeFingerprint(sequence)
	if err != nil {
		return FilmVideoSequenceVisualQCAttemptView{}, err
	}
	return FilmVideoSequenceVisualQCAttemptView{
		Attempt: detail.Attempt, Task: taskSummaryForOutput(task), Report: report, SlotEvidence: evidence,
		Cost: filmProductionCostView(detail.Billing), Valid: detail.Attempt.ScopeFingerprint == currentScope,
	}, nil
}

func (s *Service) submitFilmVideoSequenceVisualQCWithinStorageQuota(command repository.FilmVideoSequenceVisualQCSubmitCommand, policy RuntimePolicySetting) (*repository.FilmVideoSequenceVisualQCSubmitResult, error) {
	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	if existing, err := s.repo.FilmVideoSequenceVisualQCAttemptByIdempotency(command.UserID, command.IdempotencyKey); err == nil {
		if existing.ProjectID != command.ProjectID || existing.QuoteID != command.QuoteID {
			return nil, repository.ErrFilmProductionStateConflict
		}
		return s.repo.SubmitFilmVideoSequenceVisualQCQuote(command)
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
	return s.repo.SubmitFilmVideoSequenceVisualQCQuote(command)
}

func mapFilmVideoSequenceVisualQCError(err error) error {
	switch {
	case errors.Is(err, repository.ErrFilmProductionQuoteExpired):
		return conflictError("整组视频连续性 QC 报价已过期，请重新采样并报价")
	case errors.Is(err, repository.ErrFilmProductionQuoteConsumed):
		return conflictError("整组视频连续性 QC 报价已被其他提交使用")
	case errors.Is(err, repository.ErrFilmProductionQuoteConflict):
		return conflictError("整组视频连续性 QC 报价指纹不匹配，请刷新后重试")
	case errors.Is(err, repository.ErrFilmProductionQuoteDrift):
		return conflictError("Sequence、Ledger、视频结果、采样帧、模型或价格已变化，请重新报价")
	case errors.Is(err, repository.ErrFilmProductionActiveAttempt):
		return conflictError("该视频序列已有正在执行或费用待核对的模型连续性 QC Attempt")
	case errors.Is(err, repository.ErrFilmProductionStateConflict):
		return conflictError("整组视频连续性 QC 状态已变化，请刷新后重试")
	case errors.Is(err, repository.ErrFilmProductionMediaMissing):
		return conflictError("已接受视频或采样帧不可访问，不能运行整组连续性 QC")
	case errors.Is(err, repository.ErrInsufficientCredits):
		return BadAuthRequest("积分不足，请先使用兑换码充值")
	case errors.Is(err, repository.ErrActiveTaskLimit):
		return BadAuthRequest("同时排队或运行的任务已达到上限，请等待已有任务完成")
	case errors.Is(err, gorm.ErrRecordNotFound):
		return NotFound("整组视频连续性 QC 关联记录不存在")
	default:
		return err
	}
}
