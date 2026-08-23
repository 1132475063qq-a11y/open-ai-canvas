package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/gorm"
)

type CreateFilmVideoSequenceRequest struct {
	RootRunID                string                               `json:"rootRunId"`
	PromptArtifactRevisionID string                               `json:"promptArtifactRevisionId"`
	Title                    string                               `json:"title"`
	AspectRatio              string                               `json:"aspectRatio"`
	TargetDurationMs         int64                                `json:"targetDurationMs"`
	Slots                    []CreateFilmVideoSequenceSlotRequest `json:"slots"`
}

type CreateFilmVideoSequenceSlotRequest struct {
	ShotID               string `json:"shotId"`
	SourceImageAttemptID string `json:"sourceImageAttemptId"`
	DurationMs           int64  `json:"durationMs"`
}

type FilmVideoOptions struct {
	Resolution    string `json:"resolution,omitempty"`
	GenerateAudio *bool  `json:"generateAudio,omitempty"`
	Watermark     *bool  `json:"watermark,omitempty"`
}

type CreateFilmVideoQuoteRequest struct {
	SequenceID       string           `json:"sequenceId"`
	SlotID           string           `json:"slotId"`
	LogicalModelID   string           `json:"logicalModelId"`
	RetryOfAttemptID string           `json:"retryOfAttemptId"`
	Options          FilmVideoOptions `json:"options"`
}

type SubmitFilmVideoQuoteRequest struct {
	QuoteFingerprint string `json:"quoteFingerprint"`
}

type FilmVideoQuoteView struct {
	ID                    string                          `json:"id"`
	ProjectID             string                          `json:"projectId"`
	RootRunID             string                          `json:"rootRunId"`
	SequenceID            string                          `json:"sequenceId"`
	SlotID                string                          `json:"slotId"`
	RetryOfAttemptID      string                          `json:"retryOfAttemptId,omitempty"`
	LogicalModelID        string                          `json:"logicalModelId"`
	Model                 string                          `json:"model"`
	Prompt                string                          `json:"prompt"`
	SourceImageResourceID string                          `json:"sourceImageResourceId"`
	DurationMs            int64                           `json:"durationMs"`
	AspectRatio           string                          `json:"aspectRatio"`
	Options               FilmVideoOptions                `json:"options"`
	Cost                  FilmProductionCostView          `json:"cost"`
	QuoteFingerprint      string                          `json:"quoteFingerprint"`
	RequestFingerprint    string                          `json:"requestFingerprint"`
	Status                model.FilmProductionQuoteStatus `json:"status"`
	ExpiresAt             time.Time                       `json:"expiresAt"`
	CreatedAt             time.Time                       `json:"createdAt"`
	Idempotent            bool                            `json:"idempotent"`
}

type FilmVideoQCView struct {
	ID             string                         `json:"id"`
	AttemptID      string                         `json:"attemptId"`
	ResultID       string                         `json:"resultId"`
	Decision       model.FilmProductionQCDecision `json:"decision"`
	Action         model.FilmProductionQCAction   `json:"action"`
	IssueCodes     []string                       `json:"issueCodes"`
	Evidence       map[string]any                 `json:"evidence"`
	Note           string                         `json:"note"`
	Source         string                         `json:"source"`
	AssessmentKind string                         `json:"assessmentKind"`
	MediaState     model.FilmContinuityMediaState `json:"mediaState"`
	ReworkEventID  string                         `json:"reworkEventId,omitempty"`
	ModelAttemptID string                         `json:"modelAttemptId,omitempty"`
	ReviewerUserID string                         `json:"reviewerUserId,omitempty"`
	ArtifactID     string                         `json:"artifactId"`
	RevisionID     string                         `json:"revisionId"`
	CreatedAt      time.Time                      `json:"createdAt"`
}

type FilmVideoAttemptView struct {
	Attempt          model.FilmVideoAttempt         `json:"attempt"`
	Task             TaskSummary                    `json:"task"`
	Result           *model.Result                  `json:"result,omitempty"`
	QCReports        []FilmVideoQCView              `json:"qcReports"`
	VisualQCAttempts []FilmVideoVisualQCAttemptView `json:"visualQcAttempts"`
	CurrentQC        *FilmVideoQCView               `json:"currentQc,omitempty"`
	Cost             FilmProductionCostView         `json:"cost"`
	Accepted         bool                           `json:"accepted"`
	RetryAllowed     bool                           `json:"retryAllowed"`
}

type FilmVideoSlotView struct {
	Slot     model.FilmVideoSlot    `json:"slot"`
	Attempts []FilmVideoAttemptView `json:"attempts"`
}

type FilmVideoSequenceView struct {
	Sequence         model.FilmVideoSequence                `json:"sequence"`
	Slots            []FilmVideoSlotView                    `json:"slots"`
	Continuity       *FilmContinuityLedgerView              `json:"continuity,omitempty"`
	SequenceReview   *FilmVideoSequenceReviewView           `json:"sequenceReview,omitempty"`
	SequenceVisualQC []FilmVideoSequenceVisualQCAttemptView `json:"sequenceVisualQcAttempts"`
	ReworkEvents     []FilmReworkEventView                  `json:"reworkEvents"`
	Idempotent       bool                                   `json:"idempotent,omitempty"`
}

type FilmContinuityShotStateView struct {
	ID            string                         `json:"id"`
	ShotID        string                         `json:"shotId"`
	Position      int                            `json:"position"`
	ReadIn        map[string]any                 `json:"readIn"`
	WriteOut      map[string]any                 `json:"writeOut"`
	Dimensions    map[string]string              `json:"dimensions"`
	ReferenceLock map[string]any                 `json:"referenceLock"`
	Status        model.FilmContinuityShotStatus `json:"status"`
}

type FilmContinuityIssueView struct {
	ID           string           `json:"id"`
	ShotID       string           `json:"shotId,omitempty"`
	Dimension    string           `json:"dimension"`
	Severity     string           `json:"severity"`
	Code         string           `json:"code"`
	Message      string           `json:"message"`
	Authority    string           `json:"authority"`
	Owner        string           `json:"owner"`
	RepairStatus string           `json:"repairStatus"`
	SourceRefs   []map[string]any `json:"sourceRefs"`
	CreatedAt    time.Time        `json:"createdAt"`
}

type FilmContinuityLedgerView struct {
	Ledger model.FilmContinuityLedger    `json:"ledger"`
	Shots  []FilmContinuityShotStateView `json:"shots"`
	Issues []FilmContinuityIssueView     `json:"issues"`
}

type FilmReworkEventView struct {
	model.FilmReworkEvent
	RepairScope map[string]any `json:"repairScope"`
	RecheckGate map[string]any `json:"recheckGate"`
}

type SubmitFilmVideoQuoteResult struct {
	Attempt    FilmVideoAttemptView `json:"attempt"`
	Idempotent bool                 `json:"idempotent"`
}

type CreateFilmVideoQCResult struct {
	Report     FilmVideoQCView      `json:"report"`
	Attempt    FilmVideoAttemptView `json:"attempt"`
	Idempotent bool                 `json:"idempotent"`
}

func (s *Service) CreateFilmVideoSequence(userID string, projectID string, idempotencyKey string, request CreateFilmVideoSequenceRequest) (FilmVideoSequenceView, error) {
	if err := s.ValidateRuntime(); err != nil {
		return FilmVideoSequenceView{}, err
	}
	if _, err := s.requireMutableFilmProject(userID, projectID); err != nil {
		return FilmVideoSequenceView{}, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if !filmAgentIdempotencyKeyPattern.MatchString(idempotencyKey) {
		return FilmVideoSequenceView{}, BadAuthRequest("X-Idempotency-Key 必须为 8-128 位字母、数字或 ._:-")
	}
	if len(request.Slots) == 0 || len(request.Slots) > maxFilmVideoSequenceSlots {
		return FilmVideoSequenceView{}, BadAuthRequest("视频序列必须包含 1-12 个镜头槽位")
	}
	root, err := s.requireFilmAgentRootRun(userID, projectID, strings.TrimSpace(request.RootRunID))
	if err != nil {
		return FilmVideoSequenceView{}, err
	}
	if root.RootRunID != root.ID || s.filmAgentRegistry == nil || root.RegistryID != s.filmAgentRegistry.ID ||
		root.RegistryVersion != s.filmAgentRegistry.Version || root.RegistryDigest != s.filmAgentRegistry.SourceDigest {
		return FilmVideoSequenceView{}, conflictError("Film AgentTeam Registry 已变化，请重新生成并锁定视频 Prompt")
	}
	promptArtifact, promptRevision, err := s.lockedFilmProductionArtifact(userID, projectID, request.PromptArtifactRevisionID, []string{"ai-video-prompts", "prompt-manifest"})
	if err != nil {
		return FilmVideoSequenceView{}, fmt.Errorf("视频 Prompt 产物不可用于生成：%w", err)
	}
	prompts, err := extractFilmVideoPrompts(promptRevision.ContentJSON)
	if err != nil {
		return FilmVideoSequenceView{}, err
	}
	aspectRatio, err := normalizeFilmVideoAspectRatio(request.AspectRatio)
	if err != nil {
		return FilmVideoSequenceView{}, err
	}
	sequenceID := newID()
	now := time.Now().UTC()
	slots := make([]model.FilmVideoSlot, 0, len(request.Slots))
	sequenceShots := make([]model.Shot, 0, len(request.Slots))
	seenShots := make(map[string]bool, len(request.Slots))
	totalDuration := int64(0)
	for position, input := range request.Slots {
		shotID := strings.TrimSpace(input.ShotID)
		if shotID == "" || seenShots[shotID] {
			return FilmVideoSequenceView{}, BadAuthRequest("视频序列中的 shotId 不能为空或重复")
		}
		seenShots[shotID] = true
		shot, err := s.repo.ShotForProject(projectID, shotID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return FilmVideoSequenceView{}, NotFound("视频序列引用的短剧镜头不存在")
		}
		if err != nil {
			return FilmVideoSequenceView{}, err
		}
		prompt, exists := prompts[shotID]
		if !exists {
			return FilmVideoSequenceView{}, BadAuthRequest("视频 Prompt Artifact 缺少镜头 " + shotID + " 的 video_prompt")
		}
		accepted, err := s.resolveFilmVideoAcceptedImage(userID, projectID, shotID, input.SourceImageAttemptID)
		if err != nil {
			return FilmVideoSequenceView{}, err
		}
		duration, err := normalizeFilmVideoDuration(input.DurationMs, shot.DurationMs)
		if err != nil {
			return FilmVideoSequenceView{}, err
		}
		totalDuration += duration
		slots = append(slots, model.FilmVideoSlot{
			ID: newID(), UserID: userID, ProjectID: projectID, RootRunID: root.ID, SequenceID: sequenceID,
			Position: position, ShotID: shotID, Prompt: prompt, DurationMs: duration,
			SourceImageAttemptID: accepted.Attempt.ID, SourceImageResultID: accepted.Result.ID, SourceImageResourceID: accepted.Resource.ID,
			SourceImageArtifactID: accepted.Artifact.ID, SourceImageRevisionID: accepted.Revision.ID,
			Status: model.FilmVideoSlotStatusReady, Revision: 1, CreatedAt: now, UpdatedAt: now,
		})
		sequenceShots = append(sequenceShots, *shot)
	}
	targetDuration := request.TargetDurationMs
	if targetDuration == 0 {
		targetDuration = totalDuration
	}
	if targetDuration < 1_000 || targetDuration > 120_000 {
		return FilmVideoSequenceView{}, BadAuthRequest("视频序列目标时长必须在 1-120 秒之间")
	}
	title := strings.TrimSpace(request.Title)
	if title == "" {
		title = "短剧视频序列"
	}
	if utf8.RuneCountInString(title) > 160 {
		return FilmVideoSequenceView{}, BadAuthRequest("视频序列标题不能超过 160 个字符")
	}
	fingerprint, err := filmVideoSequenceFingerprint(projectID, request, *promptRevision, slots, aspectRatio, targetDuration)
	if err != nil {
		return FilmVideoSequenceView{}, err
	}
	if existing, lookupErr := s.repo.FilmVideoSequenceByIdempotency(userID, idempotencyKey); lookupErr == nil {
		if existing.ProjectID != projectID || existing.RequestFingerprint != fingerprint {
			return FilmVideoSequenceView{}, conflictError("该幂等键已用于另一份视频序列")
		}
		detail, err := s.repo.FilmVideoSequenceForUser(userID, projectID, existing.ID)
		if err != nil {
			return FilmVideoSequenceView{}, err
		}
		view, err := s.filmVideoSequenceView(*detail)
		view.Idempotent = true
		return view, err
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return FilmVideoSequenceView{}, lookupErr
	}
	sequence := model.FilmVideoSequence{
		ID: sequenceID, UserID: userID, IdempotencyKey: idempotencyKey, ProjectID: projectID, RootRunID: root.ID,
		Title: title, AspectRatio: aspectRatio, TargetDurationMs: targetDuration,
		PromptArtifactID: promptArtifact.ID, PromptArtifactRevisionID: promptRevision.ID, PromptArtifactDigest: promptRevision.ContentDigest,
		RegistryID: root.RegistryID, RegistryVersion: root.RegistryVersion, RegistryDigest: root.RegistryDigest,
		RequestFingerprint: fingerprint, ArtifactID: newID(), ArtifactRevisionID: newID(),
		Status: model.FilmVideoSequenceStatusReady, Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	artifact, revision, err := buildFilmVideoSequenceArtifact(sequence, slots, now)
	if err != nil {
		return FilmVideoSequenceView{}, err
	}
	continuity, continuityShots, continuityIssues, continuityArtifact, continuityRevision, err := buildFilmContinuityLedger(sequence, slots, sequenceShots, now)
	if err != nil {
		return FilmVideoSequenceView{}, err
	}
	detail, idempotent, err := s.repo.CreateFilmVideoSequence(repository.FilmVideoSequenceCreateCommand{
		Sequence: &sequence, Slots: slots, Artifact: artifact, Revision: revision,
		Continuity: continuity, ContinuityShots: continuityShots, ContinuityIssues: continuityIssues,
		ContinuityArtifact: continuityArtifact, ContinuityRevision: continuityRevision, At: now,
		Event: repository.AgentRuntimeEventInput{
			ID: newID(), EventType: "film.production.video.sequence_created", ActorType: "human", ActorID: userID,
			PayloadJSON: mustFilmJSON(map[string]any{"sequenceId": sequence.ID, "slotCount": len(slots), "targetDurationMs": targetDuration}),
		},
	})
	if err != nil {
		return FilmVideoSequenceView{}, mapFilmVideoError(err)
	}
	view, err := s.filmVideoSequenceView(*detail)
	view.Idempotent = idempotent
	return view, err
}

func (s *Service) ListFilmVideoSequences(userID string, projectID string, rootRunID string, limit int) ([]FilmVideoSequenceView, error) {
	if _, err := s.filmProjectForUser(userID, projectID); err != nil {
		return nil, err
	}
	details, err := s.repo.FilmVideoSequencesForProject(userID, projectID, strings.TrimSpace(rootRunID), limit)
	if err != nil {
		return nil, err
	}
	views := make([]FilmVideoSequenceView, 0, len(details))
	for _, detail := range details {
		view, err := s.filmVideoSequenceView(detail)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *Service) CreateFilmVideoQuote(userID string, projectID string, idempotencyKey string, request CreateFilmVideoQuoteRequest) (FilmVideoQuoteView, error) {
	if err := s.ValidateRuntime(); err != nil {
		return FilmVideoQuoteView{}, err
	}
	if _, err := s.requireMutableFilmProject(userID, projectID); err != nil {
		return FilmVideoQuoteView{}, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if !filmAgentIdempotencyKeyPattern.MatchString(idempotencyKey) {
		return FilmVideoQuoteView{}, BadAuthRequest("X-Idempotency-Key 必须为 8-128 位字母、数字或 ._:-")
	}
	detail, err := s.repo.FilmVideoSequenceForUser(userID, projectID, strings.TrimSpace(request.SequenceID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return FilmVideoQuoteView{}, NotFound("视频序列不存在")
	}
	if err != nil {
		return FilmVideoQuoteView{}, err
	}
	var slotDetail *repository.FilmVideoSlotDetail
	for index := range detail.Slots {
		if detail.Slots[index].Slot.ID == strings.TrimSpace(request.SlotID) {
			slotDetail = &detail.Slots[index]
			break
		}
	}
	if slotDetail == nil {
		return FilmVideoQuoteView{}, NotFound("视频序列槽位不存在")
	}
	options := normalizeFilmVideoOptions(request.Options)
	requestFingerprint, err := filmVideoQuoteRequestFingerprint(projectID, detail.Sequence, slotDetail.Slot, request.LogicalModelID, request.RetryOfAttemptID, options)
	if err != nil {
		return FilmVideoQuoteView{}, err
	}
	if existing, lookupErr := s.repo.FilmVideoQuoteByIdempotency(userID, idempotencyKey); lookupErr == nil {
		if existing.ProjectID != projectID || existing.RequestFingerprint != requestFingerprint {
			return FilmVideoQuoteView{}, conflictError("该幂等键已用于另一份 Film 视频报价")
		}
		return filmVideoQuoteView(*existing, detail.Sequence, slotDetail.Slot, true)
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return FilmVideoQuoteView{}, lookupErr
	}
	sequenceRetryAllowed := filmVideoSequenceReviewAllowsRetry(*detail, *slotDetail, strings.TrimSpace(request.RetryOfAttemptID))
	if err := validateFilmVideoRetryRequest(*slotDetail, strings.TrimSpace(request.RetryOfAttemptID), sequenceRetryAllowed); err != nil {
		return FilmVideoQuoteView{}, err
	}
	resource, err := s.repo.ResourceForUser(userID, slotDetail.Slot.SourceImageResourceID)
	if err != nil {
		return FilmVideoQuoteView{}, err
	}
	capabilityOptions := map[string]any{
		"size": detail.Sequence.AspectRatio, "videoSeconds": filmVideoSeconds(slotDetail.Slot.DurationMs),
	}
	if options.Resolution != "" {
		capabilityOptions["vquality"] = options.Resolution
	}
	if options.GenerateAudio != nil {
		capabilityOptions["videoGenerateAudio"] = *options.GenerateAudio
	}
	if options.Watermark != nil {
		capabilityOptions["videoWatermark"] = *options.Watermark
	}
	input := map[string]any{
		"mode": "video", "prompt": slotDetail.Slot.Prompt,
		"referenceImages": []any{filmVideoReferenceMedia(*resource)}, "capabilityOptions": capabilityOptions,
		"config": map[string]any{},
		"metadata": map[string]any{
			"filmRootRunId": detail.Sequence.RootRunID, "filmVideoSequenceId": detail.Sequence.ID,
			"filmVideoSlotId": slotDetail.Slot.ID, "videoEditOperation": "image_to_video",
			"videoStartFrameNodeId": resource.ID,
		},
	}
	logicalModelID := strings.TrimSpace(request.LogicalModelID)
	if logicalModelID == "" {
		return FilmVideoQuoteView{}, BadAuthRequest("请选择视频逻辑模型")
	}
	intent := ModelRequestIntentFromTaskInput(input, model.FilmProductionTaskTypeVideo, "image_to_video")
	routed, err := s.ResolveLogicalModel(logicalModelID, intent)
	if err != nil {
		return FilmVideoQuoteView{}, err
	}
	if routed.LogicalModel.Capability != "video" || routed.ChannelModel.Capability != "video" {
		return FilmVideoQuoteView{}, BadAuthRequest("所选逻辑模型不是视频模型")
	}
	input = applyRoutedProviderSelection(input, routed)
	if err := s.ValidateTaskCapability(input); err != nil {
		return FilmVideoQuoteView{}, err
	}
	if containsInlineMediaDataURL(input) || containsAgentRuntimeSecret(input) {
		return FilmVideoQuoteView{}, BadAuthRequest("Film 视频请求只能引用已上传资源，且不能包含密钥")
	}
	if err := s.protectTaskSecrets(input); err != nil {
		return FilmVideoQuoteView{}, err
	}
	requestJSON, err := json.Marshal(input)
	if err != nil {
		return FilmVideoQuoteView{}, err
	}
	now := time.Now().UTC()
	quoteID, taskID := newID(), newID()
	task := model.Task{
		ID: taskID, UserID: userID, ProjectID: projectID, Type: model.FilmProductionTaskTypeVideo,
		Status: model.TaskStatusQueued, Stage: "等待用户确认费用", Progress: 0, Prompt: slotDetail.Slot.Prompt,
		Operation: "image_to_video", Provider: "managed", Model: routed.LogicalModel.Code,
		LogicalModelID: routed.LogicalModel.ID, LogicalModelRevisionID: routed.Revision.ID,
		RouteID: routed.Route.ID, ChannelModelID: routed.ChannelModel.ID, RouteRun: 1, InputJSON: string(requestJSON),
	}
	billing, err := s.taskBillingOrder(userID, &task, input)
	if err != nil {
		return FilmVideoQuoteView{}, err
	}
	if billing != nil {
		billing.IdempotencyKey = "film-video:" + quoteID
	}
	billingJSON := ""
	if billing != nil {
		encoded, err := json.Marshal(billing)
		if err != nil {
			return FilmVideoQuoteView{}, err
		}
		billingJSON = string(encoded)
	}
	quote := model.FilmVideoQuote{
		ID: quoteID, UserID: userID, IdempotencyKey: idempotencyKey, ProjectID: projectID, RootRunID: detail.Sequence.RootRunID,
		SequenceID: detail.Sequence.ID, SequenceRevision: detail.Sequence.Revision, SlotID: slotDetail.Slot.ID, SlotRevision: slotDetail.Slot.Revision,
		RetryOfAttemptID: strings.TrimSpace(request.RetryOfAttemptID), TaskID: taskID,
		SourceImageAttemptID: slotDetail.Slot.SourceImageAttemptID, SourceImageResultID: slotDetail.Slot.SourceImageResultID,
		SourceImageResourceID: slotDetail.Slot.SourceImageResourceID, SourceImageArtifactID: slotDetail.Slot.SourceImageArtifactID,
		SourceImageRevisionID: slotDetail.Slot.SourceImageRevisionID,
		PromptArtifactID:      detail.Sequence.PromptArtifactID, PromptArtifactRevisionID: detail.Sequence.PromptArtifactRevisionID,
		PromptArtifactDigest: detail.Sequence.PromptArtifactDigest, RegistryDigest: detail.Sequence.RegistryDigest,
		LogicalModelID: routed.LogicalModel.ID, LogicalModelRevisionID: routed.Revision.ID,
		RouteID: routed.Route.ID, ChannelID: routed.ChannelModel.ChannelID, ChannelModelID: routed.ChannelModel.ID,
		Model: routed.LogicalModel.Code, ProviderModel: routed.ChannelModel.ModelKey, Protocol: routed.ChannelModel.Protocol,
		CapabilityVersion: routed.ChannelModel.CapabilityVersion, ChannelPriceVersion: routed.ChannelModel.PriceVersion,
		RequestJSON: string(requestJSON), RequestFingerprint: requestFingerprint, BillingJSON: billingJSON,
		Status: model.FilmProductionQuoteStatusPending, ExpiresAt: now.Add(filmProductionQuoteTTL), CreatedAt: now, UpdatedAt: now,
	}
	quote.QuoteFingerprint, err = filmVideoQuoteFingerprint(quote)
	if err != nil {
		return FilmVideoQuoteView{}, err
	}
	if err := s.repo.CreateFilmVideoQuote(repository.FilmVideoQuoteCreateCommand{Quote: &quote, At: now}); err != nil {
		return FilmVideoQuoteView{}, mapFilmVideoError(err)
	}
	return filmVideoQuoteView(quote, detail.Sequence, slotDetail.Slot, false)
}

func (s *Service) SubmitFilmVideoQuote(userID string, projectID string, quoteID string, idempotencyKey string, request SubmitFilmVideoQuoteRequest) (SubmitFilmVideoQuoteResult, error) {
	if _, err := s.requireMutableFilmProject(userID, projectID); err != nil {
		return SubmitFilmVideoQuoteResult{}, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if !filmAgentIdempotencyKeyPattern.MatchString(idempotencyKey) {
		return SubmitFilmVideoQuoteResult{}, BadAuthRequest("X-Idempotency-Key 必须为 8-128 位字母、数字或 ._:-")
	}
	quote, err := s.repo.FilmVideoQuoteForUser(userID, projectID, strings.TrimSpace(quoteID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SubmitFilmVideoQuoteResult{}, NotFound("Film 视频报价不存在")
	}
	if err != nil {
		return SubmitFilmVideoQuoteResult{}, err
	}
	attemptID := newID()
	input, prompt, err := filmVideoSubmissionInput(*quote, attemptID)
	if err != nil {
		return SubmitFilmVideoQuoteResult{}, err
	}
	if err := s.protectTaskSecrets(input); err != nil {
		return SubmitFilmVideoQuoteResult{}, err
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return SubmitFilmVideoQuoteResult{}, err
	}
	policy, err := s.RuntimePolicy()
	if err != nil {
		return SubmitFilmVideoQuoteResult{}, err
	}
	now := time.Now().UTC()
	task := &model.Task{
		ID: quote.TaskID, UserID: userID, ProjectID: projectID, Type: model.FilmProductionTaskTypeVideo,
		Status: model.TaskStatusQueued, Stage: "等待队列调度", Progress: 5, Prompt: prompt,
		Operation: "image_to_video", Provider: "managed", Model: quote.Model,
		LogicalModelID: quote.LogicalModelID, LogicalModelRevisionID: quote.LogicalModelRevisionID,
		RouteID: quote.RouteID, ChannelModelID: quote.ChannelModelID, RouteRun: 1, InputJSON: string(inputJSON),
	}
	command := repository.FilmVideoSubmitCommand{
		UserID: userID, ProjectID: projectID, QuoteID: quote.ID, QuoteFingerprint: strings.TrimSpace(request.QuoteFingerprint),
		IdempotencyKey: idempotencyKey, AttemptID: attemptID, AttemptArtifactID: newID(), AttemptRevisionID: newID(),
		Task: task, ActiveTaskLimit: policy.Task.ActiveTaskLimit, At: now,
		Event: repository.AgentRuntimeEventInput{
			ID: newID(), EventType: "film.production.video.submitted", ActorType: "human", ActorID: userID,
			PayloadJSON: mustFilmJSON(map[string]any{"quoteId": quote.ID, "taskId": quote.TaskID, "sequenceId": quote.SequenceID, "slotId": quote.SlotID}),
		},
	}
	stored, err := s.submitFilmVideoWithinStorageQuota(command, policy)
	if err != nil {
		return SubmitFilmVideoQuoteResult{}, mapFilmVideoError(err)
	}
	detail, err := s.repo.FilmVideoAttemptForUser(userID, projectID, stored.Attempt.ID)
	if err != nil {
		return SubmitFilmVideoQuoteResult{}, err
	}
	view, err := s.filmVideoAttemptView(*detail, true)
	if err != nil {
		return SubmitFilmVideoQuoteResult{}, err
	}
	return SubmitFilmVideoQuoteResult{Attempt: view, Idempotent: stored.Idempotent}, nil
}

func (s *Service) CreateFilmVideoHumanQC(userID string, projectID string, attemptID string, idempotencyKey string, request CreateFilmProductionQCRequest) (CreateFilmVideoQCResult, error) {
	if _, err := s.requireMutableFilmProject(userID, projectID); err != nil {
		return CreateFilmVideoQCResult{}, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if !filmAgentIdempotencyKeyPattern.MatchString(idempotencyKey) {
		return CreateFilmVideoQCResult{}, BadAuthRequest("X-Idempotency-Key 必须为 8-128 位字母、数字或 ._:-")
	}
	detail, err := s.repo.FilmVideoAttemptForUser(userID, projectID, strings.TrimSpace(attemptID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return CreateFilmVideoQCResult{}, NotFound("Film 视频 Attempt 不存在")
	}
	if err != nil {
		return CreateFilmVideoQCResult{}, err
	}
	if detail.Result == nil || detail.Attempt.Status != model.FilmProductionAttemptStatusSucceeded {
		return CreateFilmVideoQCResult{}, conflictError("只有已取得真实视频结果的 Attempt 才能人工验收")
	}
	issueCodes, evidence, note, err := normalizeFilmProductionQCInput(request)
	if err != nil {
		return CreateFilmVideoQCResult{}, err
	}
	if request.Action == model.FilmProductionQCActionAccept && filmVideoHasBlockingTechnicalQC(detail.QCReports) {
		return CreateFilmVideoQCResult{}, conflictError("视频存在系统技术 QC 阻断项，必须按最小返工范围重试后重新验收")
	}
	now := time.Now().UTC()
	report := &model.FilmVideoQCReport{
		ID: newID(), UserID: userID, IdempotencyKey: idempotencyKey, ProjectID: projectID, RootRunID: detail.Attempt.RootRunID,
		SequenceID: detail.Attempt.SequenceID, SlotID: detail.Attempt.SlotID, AttemptID: detail.Attempt.ID, ResultID: detail.Result.ID,
		Decision: request.Decision, Action: request.Action, IssueCodesJSON: mustFilmJSON(issueCodes), EvidenceJSON: mustFilmJSON(evidence),
		Note: note, Source: "human", AssessmentKind: "human_media_review", MediaState: model.FilmContinuityMediaStateAvailable,
		ReviewerUserID: userID, ArtifactID: newID(), RevisionID: newID(), CreatedAt: now,
	}
	artifact, revision, err := buildFilmVideoHumanQCArtifact(detail.Attempt, *detail.Result, *report, now)
	if err != nil {
		return CreateFilmVideoQCResult{}, err
	}
	eventType := "film.production.video.qc.held"
	if request.Action == model.FilmProductionQCActionAccept {
		eventType = "film.production.video.qc.accepted"
	} else if request.Action == model.FilmProductionQCActionRetry {
		eventType = "film.production.video.qc.retry_requested"
	}
	created, idempotent, err := s.repo.CreateFilmVideoHumanQC(repository.FilmVideoHumanQCCommand{
		UserID: userID, ProjectID: projectID, AttemptID: detail.Attempt.ID, Report: report, Artifact: artifact, Revision: revision, At: now,
		Event: repository.AgentRuntimeEventInput{
			ID: newID(), EventType: eventType, ActorType: "human", ActorID: userID,
			PayloadJSON: mustFilmJSON(map[string]any{"attemptId": detail.Attempt.ID, "resultId": detail.Result.ID, "decision": request.Decision, "action": request.Action}),
		},
	})
	if err != nil {
		return CreateFilmVideoQCResult{}, mapFilmVideoError(err)
	}
	updated, err := s.repo.FilmVideoAttemptForUser(userID, projectID, detail.Attempt.ID)
	if err != nil {
		return CreateFilmVideoQCResult{}, err
	}
	view, err := s.filmVideoAttemptView(*updated, true)
	if err != nil {
		return CreateFilmVideoQCResult{}, err
	}
	return CreateFilmVideoQCResult{Report: filmVideoQCView(*created), Attempt: view, Idempotent: idempotent}, nil
}

func filmVideoHasBlockingTechnicalQC(reports []model.FilmVideoQCReport) bool {
	for _, report := range reports {
		if report.Source == "system" && report.AssessmentKind == "technical_media_qc" && report.Decision == model.FilmProductionQCDecisionFail {
			return true
		}
	}
	return false
}

func normalizeFilmVideoOptions(input FilmVideoOptions) FilmVideoOptions {
	input.Resolution = strings.TrimSpace(input.Resolution)
	return input
}

func validateFilmVideoRetryRequest(slot repository.FilmVideoSlotDetail, retryOf string, sequenceRetryAllowed bool) error {
	if len(slot.Attempts) == 0 {
		if retryOf != "" {
			return conflictError("该视频槽位还没有可重试的 Attempt")
		}
		return nil
	}
	latest := slot.Attempts[0]
	if retryOf == "" || latest.Attempt.ID != retryOf {
		return conflictError("已有视频 Attempt，后续生成必须明确引用最近一次 Attempt")
	}
	switch latest.Attempt.Status {
	case model.FilmProductionAttemptStatusFailed, model.FilmProductionAttemptStatusCancelled:
		return nil
	case model.FilmProductionAttemptStatusSucceeded:
		var currentHuman *model.FilmVideoQCReport
		for index := range latest.QCReports {
			if latest.QCReports[index].Source == "human" {
				currentHuman = &latest.QCReports[index]
			}
		}
		if currentHuman != nil && currentHuman.Decision == model.FilmProductionQCDecisionPass && currentHuman.Action == model.FilmProductionQCActionAccept && sequenceRetryAllowed {
			return nil
		}
		if currentHuman == nil || currentHuman.Decision != model.FilmProductionQCDecisionFail || currentHuman.Action != model.FilmProductionQCActionRetry {
			return conflictError("已有视频必须先由人工标记 FAIL 并请求重试")
		}
		return nil
	default:
		return conflictError("当前视频 Attempt 尚未形成可安全重试的费用终态")
	}
}

func filmVideoSubmissionInput(quote model.FilmVideoQuote, attemptID string) (map[string]any, string, error) {
	var input map[string]any
	if json.Unmarshal([]byte(quote.RequestJSON), &input) != nil {
		return nil, "", conflictError("Film 视频报价请求快照已损坏")
	}
	prompt, _ := input["prompt"].(string)
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil, "", conflictError("Film 视频报价缺少 Prompt")
	}
	metadata, _ := input["metadata"].(map[string]any)
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["filmVideoQuoteId"] = quote.ID
	metadata["filmVideoAttemptId"] = attemptID
	input["metadata"] = metadata
	return input, prompt, nil
}

func (s *Service) submitFilmVideoWithinStorageQuota(command repository.FilmVideoSubmitCommand, policy RuntimePolicySetting) (*repository.FilmVideoSubmitResult, error) {
	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	if existing, err := s.repo.FilmVideoAttemptByIdempotency(command.UserID, command.IdempotencyKey); err == nil {
		if existing.ProjectID != command.ProjectID || existing.QuoteID != command.QuoteID {
			return nil, repository.ErrFilmProductionStateConflict
		}
		return s.repo.SubmitFilmVideoQuote(command)
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
	return s.repo.SubmitFilmVideoQuote(command)
}

func filmVideoQuoteView(quote model.FilmVideoQuote, sequence model.FilmVideoSequence, slot model.FilmVideoSlot, idempotent bool) (FilmVideoQuoteView, error) {
	var input canvasGenerationInput
	if err := json.Unmarshal([]byte(quote.RequestJSON), &input); err != nil {
		return FilmVideoQuoteView{}, err
	}
	billing, err := filmVideoQuoteBilling(quote)
	if err != nil {
		return FilmVideoQuoteView{}, err
	}
	status := quote.Status
	if status == model.FilmProductionQuoteStatusPending && !quote.ExpiresAt.After(time.Now()) {
		status = model.FilmProductionQuoteStatusExpired
	}
	generateAudio := parseOptionalBool(input.Config.VideoGenerateAudio)
	watermark := parseOptionalBool(input.Config.VideoWatermark)
	return FilmVideoQuoteView{
		ID: quote.ID, ProjectID: quote.ProjectID, RootRunID: quote.RootRunID, SequenceID: quote.SequenceID, SlotID: quote.SlotID,
		RetryOfAttemptID: quote.RetryOfAttemptID, LogicalModelID: quote.LogicalModelID, Model: quote.Model, Prompt: input.Prompt,
		SourceImageResourceID: quote.SourceImageResourceID, DurationMs: slot.DurationMs, AspectRatio: sequence.AspectRatio,
		Options: FilmVideoOptions{Resolution: input.Config.VQuality, GenerateAudio: generateAudio, Watermark: watermark},
		Cost:    filmProductionCostView(billing), QuoteFingerprint: quote.QuoteFingerprint, RequestFingerprint: quote.RequestFingerprint,
		Status: status, ExpiresAt: quote.ExpiresAt, CreatedAt: quote.CreatedAt, Idempotent: idempotent,
	}, nil
}

func parseOptionalBool(value string) *bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed := strings.EqualFold(value, "true")
	return &parsed
}

func filmVideoQuoteBilling(quote model.FilmVideoQuote) (*model.BillingOrder, error) {
	if strings.TrimSpace(quote.BillingJSON) == "" || strings.TrimSpace(quote.BillingJSON) == "null" {
		return nil, nil
	}
	var order model.BillingOrder
	if err := json.Unmarshal([]byte(quote.BillingJSON), &order); err != nil {
		return nil, err
	}
	return &order, nil
}

func (s *Service) filmVideoSequenceView(detail repository.FilmVideoSequenceDetail) (FilmVideoSequenceView, error) {
	slots := make([]FilmVideoSlotView, 0, len(detail.Slots))
	for _, slot := range detail.Slots {
		attempts := make([]FilmVideoAttemptView, 0, len(slot.Attempts))
		for _, attempt := range slot.Attempts {
			view, err := s.filmVideoAttemptView(attempt, slot.Slot.CurrentAttemptID == attempt.Attempt.ID && slot.Slot.ResultRevisionID == attempt.Attempt.ResultRevisionID)
			if err != nil {
				return FilmVideoSequenceView{}, err
			}
			attempts = append(attempts, view)
		}
		slots = append(slots, FilmVideoSlotView{Slot: slot.Slot, Attempts: attempts})
	}
	var continuity *FilmContinuityLedgerView
	if detail.Continuity != nil {
		view, err := filmContinuityLedgerView(*detail.Continuity)
		if err != nil {
			return FilmVideoSequenceView{}, err
		}
		continuity = &view
	}
	reworkEvents := make([]FilmReworkEventView, 0, len(detail.ReworkEvents))
	for _, event := range detail.ReworkEvents {
		view, err := filmReworkEventView(event)
		if err != nil {
			return FilmVideoSequenceView{}, err
		}
		reworkEvents = append(reworkEvents, view)
	}
	var sequenceReview *FilmVideoSequenceReviewView
	if detail.SequenceReview != nil {
		view, err := filmVideoSequenceReviewView(*detail.SequenceReview, detail)
		if err != nil {
			return FilmVideoSequenceView{}, err
		}
		sequenceReview = &view
	}
	sequenceVisualQC := make([]FilmVideoSequenceVisualQCAttemptView, 0, len(detail.SequenceVisualQC))
	for _, attempt := range detail.SequenceVisualQC {
		view, err := s.filmVideoSequenceVisualQCAttemptView(attempt, detail)
		if err != nil {
			return FilmVideoSequenceView{}, err
		}
		sequenceVisualQC = append(sequenceVisualQC, view)
	}
	return FilmVideoSequenceView{
		Sequence: detail.Sequence, Slots: slots, Continuity: continuity, SequenceReview: sequenceReview,
		SequenceVisualQC: sequenceVisualQC, ReworkEvents: reworkEvents,
	}, nil
}

func (s *Service) filmVideoAttemptView(detail repository.FilmVideoAttemptDetail, currentVersion bool) (FilmVideoAttemptView, error) {
	task := detail.Task
	s.hydrateTaskProviderRequestID(&task)
	qcViews := make([]FilmVideoQCView, 0, len(detail.QCReports))
	var current *FilmVideoQCView
	var currentHuman *FilmVideoQCView
	for _, report := range detail.QCReports {
		view := filmVideoQCView(report)
		qcViews = append(qcViews, view)
		copy := view
		current = &copy
		if report.Source == "human" {
			currentHuman = &copy
		}
	}
	if currentHuman != nil {
		current = currentHuman
	}
	accepted := currentHuman != nil && currentHuman.Decision == model.FilmProductionQCDecisionPass && currentHuman.Action == model.FilmProductionQCActionAccept
	retryAllowed := detail.Attempt.Status == model.FilmProductionAttemptStatusFailed || detail.Attempt.Status == model.FilmProductionAttemptStatusCancelled ||
		(detail.Attempt.Status == model.FilmProductionAttemptStatusSucceeded && currentHuman != nil && currentHuman.Decision == model.FilmProductionQCDecisionFail && currentHuman.Action == model.FilmProductionQCActionRetry)
	visualQCAttempts := make([]FilmVideoVisualQCAttemptView, 0, len(detail.VisualQCAttempts))
	for _, visualAttempt := range detail.VisualQCAttempts {
		view, err := s.filmVideoVisualQCAttemptView(visualAttempt, currentVersion && visualAttempt.Attempt.SourceResultRevisionID == detail.Attempt.ResultRevisionID)
		if err != nil {
			return FilmVideoAttemptView{}, err
		}
		visualQCAttempts = append(visualQCAttempts, view)
	}
	return FilmVideoAttemptView{
		Attempt: detail.Attempt, Task: taskSummaryForOutput(task), Result: detail.Result, QCReports: qcViews, VisualQCAttempts: visualQCAttempts,
		CurrentQC: current, Cost: filmProductionCostView(detail.Billing), Accepted: accepted, RetryAllowed: retryAllowed,
	}, nil
}

func filmVideoQCView(report model.FilmVideoQCReport) FilmVideoQCView {
	issueCodes := []string{}
	evidence := map[string]any{}
	_ = json.Unmarshal([]byte(firstNonEmpty(report.IssueCodesJSON, "[]")), &issueCodes)
	_ = json.Unmarshal([]byte(firstNonEmpty(report.EvidenceJSON, "{}")), &evidence)
	return FilmVideoQCView{
		ID: report.ID, AttemptID: report.AttemptID, ResultID: report.ResultID, Decision: report.Decision, Action: report.Action,
		IssueCodes: issueCodes, Evidence: evidence, Note: report.Note, Source: report.Source, ReviewerUserID: report.ReviewerUserID,
		AssessmentKind: report.AssessmentKind, MediaState: report.MediaState, ReworkEventID: report.ReworkEventID,
		ModelAttemptID: report.ModelAttemptID,
		ArtifactID:     report.ArtifactID, RevisionID: report.RevisionID, CreatedAt: report.CreatedAt,
	}
}

func buildFilmVideoHumanQCArtifact(attempt model.FilmVideoAttempt, result model.Result, report model.FilmVideoQCReport, at time.Time) (*model.ProductionArtifact, *model.ProductionArtifactRevision, error) {
	var issueCodes []string
	var evidence map[string]any
	if err := json.Unmarshal([]byte(report.IssueCodesJSON), &issueCodes); err != nil {
		return nil, nil, err
	}
	if err := json.Unmarshal([]byte(report.EvidenceJSON), &evidence); err != nil {
		return nil, nil, err
	}
	content, err := json.Marshal(map[string]any{
		"schemaVersion": 2, "artifactType": "generation-qc-report", "mediaType": "video", "reportId": report.ID,
		"sequenceId": attempt.SequenceID, "slotId": attempt.SlotID, "attemptId": attempt.ID, "resultId": result.ID,
		"decision": report.Decision, "action": report.Action, "issueCodes": issueCodes, "evidence": evidence,
		"note": report.Note, "source": "human", "assessmentKind": report.AssessmentKind, "mediaState": report.MediaState,
		"reviewerUserId": report.ReviewerUserID,
	})
	if err != nil {
		return nil, nil, err
	}
	artifact := &model.ProductionArtifact{
		ID: report.ArtifactID, UserID: attempt.UserID, ProjectID: attempt.ProjectID, Domain: "film",
		ArtifactType: "generation-qc-report", LogicalKey: "film-video:human-qc:" + report.ID,
	}
	revision := &model.ProductionArtifactRevision{
		ID: report.RevisionID, Status: model.ProductionArtifactStatusLocked, ContentJSON: string(content), ContentDigest: digestBytesHex(content),
		SourceRunID: attempt.RootRunID, SourceAttemptID: attempt.ID,
		SourceArtifactRefsJSON: mustFilmJSON([]map[string]any{
			{"artifactId": attempt.ResultArtifactID, "revisionId": attempt.ResultRevisionID, "type": "generation-result"},
			{"artifactId": attempt.AttemptArtifactID, "revisionId": attempt.AttemptRevisionID, "type": "generation-attempt"},
		}),
		AuthorityRefsJSON: mustFilmJSON([]map[string]any{{"kind": "human", "id": report.ReviewerUserID}}),
		CreatedByType:     "human", CreatedByID: report.ReviewerUserID, CreatedAt: at,
	}
	return artifact, revision, nil
}

func mapFilmVideoError(err error) error {
	switch {
	case errors.Is(err, repository.ErrFilmProductionQuoteExpired):
		return conflictError("Film 视频报价已过期，请重新报价")
	case errors.Is(err, repository.ErrFilmProductionQuoteConsumed):
		return conflictError("Film 视频报价已被其他提交使用")
	case errors.Is(err, repository.ErrFilmProductionQuoteConflict):
		return conflictError("Film 视频报价指纹不匹配，请刷新后重试")
	case errors.Is(err, repository.ErrFilmProductionQuoteDrift):
		return conflictError("视频序列、来源图片、模型或价格已变化，请重新报价")
	case errors.Is(err, repository.ErrFilmProductionActiveAttempt):
		return conflictError("该视频槽位已有正在执行或费用待核对的 Attempt")
	case errors.Is(err, repository.ErrFilmProductionRetryNotAllowed):
		return conflictError("当前视频 Attempt 状态或 QC 结论不允许重试")
	case errors.Is(err, repository.ErrFilmProductionStateConflict):
		return conflictError("Film 视频状态已变化，请刷新后重试")
	case errors.Is(err, repository.ErrFilmProductionMediaMissing):
		return conflictError("视频或来源图片媒体不可访问，不能继续")
	case errors.Is(err, repository.ErrInsufficientCredits):
		return BadAuthRequest("积分不足，请先使用兑换码充值")
	case errors.Is(err, repository.ErrActiveTaskLimit):
		return BadAuthRequest("同时排队或运行的任务已达到上限，请等待已有任务完成")
	case errors.Is(err, gorm.ErrRecordNotFound):
		return NotFound("Film 视频关联记录不存在")
	default:
		return err
	}
}
