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

const filmProductionQuoteTTL = 10 * time.Minute

type CreateFilmProductionImageQuoteRequest struct {
	RootRunID                    string                     `json:"rootRunId"`
	ShotID                       string                     `json:"shotId"`
	StoryboardArtifactRevisionID string                     `json:"storyboardArtifactRevisionId"`
	PromptArtifactRevisionID     string                     `json:"promptArtifactRevisionId"`
	FeasibilityRevisionID        string                     `json:"feasibilityArtifactRevisionId"`
	LogicalModelID               string                     `json:"logicalModelId"`
	RetryOfAttemptID             string                     `json:"retryOfAttemptId"`
	ReferenceResourceIDs         []string                   `json:"referenceResourceIds"`
	Options                      FilmProductionImageOptions `json:"options"`
}

type FilmProductionImageOptions struct {
	Size                  string `json:"size,omitempty"`
	Quality               string `json:"quality,omitempty"`
	TransparentBackground *bool  `json:"transparentBackground,omitempty"`
}

type SubmitFilmProductionImageQuoteRequest struct {
	QuoteFingerprint string `json:"quoteFingerprint"`
}

type CreateFilmProductionQCRequest struct {
	Decision   model.FilmProductionQCDecision `json:"decision"`
	Action     model.FilmProductionQCAction   `json:"action"`
	IssueCodes []string                       `json:"issueCodes"`
	Evidence   map[string]any                 `json:"evidence"`
	Note       string                         `json:"note"`
}

type FilmProductionCostView struct {
	Required                   bool                `json:"required"`
	BillingOrderID             string              `json:"billingOrderId,omitempty"`
	BillingMode                string              `json:"billingMode,omitempty"`
	PriceVersion               int64               `json:"priceVersion,omitempty"`
	Quantity                   int64               `json:"quantity,omitempty"`
	AmountMicrocredits         int64               `json:"amountMicrocredits"`
	ReservedAmountMicrocredits int64               `json:"reservedAmountMicrocredits,omitempty"`
	ActualAmountMicrocredits   int64               `json:"actualAmountMicrocredits,omitempty"`
	RefundedAmountMicrocredits int64               `json:"refundedAmountMicrocredits,omitempty"`
	Status                     model.BillingStatus `json:"status,omitempty"`
}

type FilmProductionImageQuoteView struct {
	ID                   string                          `json:"id"`
	ProjectID            string                          `json:"projectId"`
	RootRunID            string                          `json:"rootRunId"`
	ShotID               string                          `json:"shotId"`
	RetryOfAttemptID     string                          `json:"retryOfAttemptId,omitempty"`
	Artifacts            []FilmProductionArtifactRef     `json:"artifacts"`
	LogicalModelID       string                          `json:"logicalModelId"`
	Model                string                          `json:"model"`
	Prompt               string                          `json:"prompt"`
	ReferenceResourceIDs []string                        `json:"referenceResourceIds"`
	Options              FilmProductionImageOptions      `json:"options"`
	Count                int                             `json:"count"`
	Cost                 FilmProductionCostView          `json:"cost"`
	QuoteFingerprint     string                          `json:"quoteFingerprint"`
	RequestFingerprint   string                          `json:"requestFingerprint"`
	Status               model.FilmProductionQuoteStatus `json:"status"`
	ExpiresAt            time.Time                       `json:"expiresAt"`
	CreatedAt            time.Time                       `json:"createdAt"`
	Idempotent           bool                            `json:"idempotent"`
}

type FilmProductionQCView struct {
	ID             string                         `json:"id"`
	AttemptID      string                         `json:"attemptId"`
	ResultID       string                         `json:"resultId"`
	Decision       model.FilmProductionQCDecision `json:"decision"`
	Action         model.FilmProductionQCAction   `json:"action"`
	IssueCodes     []string                       `json:"issueCodes"`
	Evidence       map[string]any                 `json:"evidence"`
	Note           string                         `json:"note"`
	Source         string                         `json:"source"`
	AssessmentKind string                         `json:"assessmentKind,omitempty"`
	ModelAttemptID string                         `json:"modelAttemptId,omitempty"`
	ReviewerUserID string                         `json:"reviewerUserId,omitempty"`
	ArtifactID     string                         `json:"artifactId"`
	RevisionID     string                         `json:"revisionId"`
	CreatedAt      time.Time                      `json:"createdAt"`
}

type FilmProductionAttemptView struct {
	Attempt          model.FilmProductionAttempt `json:"attempt"`
	Task             TaskSummary                 `json:"task"`
	Result           *model.Result               `json:"result,omitempty"`
	QCReports        []FilmProductionQCView      `json:"qcReports"`
	CurrentQC        *FilmProductionQCView       `json:"currentQc,omitempty"`
	VisualQCAttempts []FilmVisualQCAttemptView   `json:"visualQcAttempts"`
	Cost             FilmProductionCostView      `json:"cost"`
	Accepted         bool                        `json:"accepted"`
	RetryAllowed     bool                        `json:"retryAllowed"`
}

type SubmitFilmProductionImageQuoteResult struct {
	Attempt    FilmProductionAttemptView `json:"attempt"`
	Idempotent bool                      `json:"idempotent"`
}

type CreateFilmProductionQCResult struct {
	Report     FilmProductionQCView      `json:"report"`
	Attempt    FilmProductionAttemptView `json:"attempt"`
	Idempotent bool                      `json:"idempotent"`
}

func (s *Service) CreateFilmProductionImageQuote(userID string, projectID string, idempotencyKey string, request CreateFilmProductionImageQuoteRequest) (FilmProductionImageQuoteView, error) {
	if err := s.ValidateRuntime(); err != nil {
		return FilmProductionImageQuoteView{}, err
	}
	if _, err := s.requireMutableFilmProject(userID, projectID); err != nil {
		return FilmProductionImageQuoteView{}, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if !filmAgentIdempotencyKeyPattern.MatchString(idempotencyKey) {
		return FilmProductionImageQuoteView{}, BadAuthRequest("X-Idempotency-Key 必须为 8-128 位字母、数字或 ._:-")
	}

	sources, prompt, err := s.resolveFilmProductionQuoteSources(userID, projectID, request)
	if err != nil {
		return FilmProductionImageQuoteView{}, err
	}
	references, referenceIDs, err := s.resolveFilmProductionImageReferences(userID, request.ReferenceResourceIDs)
	if err != nil {
		return FilmProductionImageQuoteView{}, err
	}
	options := normalizeFilmProductionImageOptions(request.Options)
	requestFingerprint, err := filmProductionSourceRequestFingerprint(projectID, request, sources, prompt, referenceIDs, options)
	if err != nil {
		return FilmProductionImageQuoteView{}, err
	}
	if existing, lookupErr := s.repo.FilmProductionQuoteByIdempotency(userID, idempotencyKey); lookupErr == nil {
		if existing.ProjectID != projectID || existing.RequestFingerprint != requestFingerprint {
			return FilmProductionImageQuoteView{}, conflictError("该幂等键已用于另一份 Film 生图报价")
		}
		return filmProductionQuoteView(*existing, true)
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return FilmProductionImageQuoteView{}, lookupErr
	}
	if err := s.validateFilmProductionRetryRequest(userID, projectID, sources.RootRun.ID, sources.Shot.ID, strings.TrimSpace(request.RetryOfAttemptID), sources.PromptRevision.ID); err != nil {
		return FilmProductionImageQuoteView{}, err
	}

	capabilityOptions := filmProductionCapabilityOptions(options)
	capabilityOptions["count"] = 1
	input := map[string]any{
		"mode":              "image",
		"prompt":            prompt,
		"referenceImages":   references,
		"capabilityOptions": capabilityOptions,
		"config":            map[string]any{"count": "1"},
		"metadata": map[string]any{
			"filmRootRunId": sources.RootRun.ID,
			"filmShotId":    sources.Shot.ID,
		},
	}
	logicalModelID := strings.TrimSpace(request.LogicalModelID)
	if logicalModelID == "" {
		return FilmProductionImageQuoteView{}, BadAuthRequest("请选择图片逻辑模型")
	}
	intent := ModelRequestIntentFromTaskInput(input, model.FilmProductionTaskTypeImage, "image")
	routed, err := s.ResolveLogicalModel(logicalModelID, intent)
	if err != nil {
		return FilmProductionImageQuoteView{}, err
	}
	if routed.LogicalModel.Capability != "image" || routed.ChannelModel.Capability != "image" {
		return FilmProductionImageQuoteView{}, BadAuthRequest("所选逻辑模型不是图片模型")
	}
	input = applyRoutedProviderSelection(input, routed)
	if err := s.ValidateTaskCapability(input); err != nil {
		return FilmProductionImageQuoteView{}, err
	}
	if containsInlineMediaDataURL(input) || containsAgentRuntimeSecret(input) {
		return FilmProductionImageQuoteView{}, BadAuthRequest("Film 生图请求只能引用已上传资源，且不能包含密钥")
	}
	if err := s.protectTaskSecrets(input); err != nil {
		return FilmProductionImageQuoteView{}, err
	}
	requestJSON, err := json.Marshal(input)
	if err != nil {
		return FilmProductionImageQuoteView{}, err
	}

	now := time.Now().UTC()
	quoteID := newID()
	taskID := newID()
	task := model.Task{
		ID: taskID, UserID: userID, ProjectID: projectID, Type: model.FilmProductionTaskTypeImage,
		Status: model.TaskStatusQueued, Stage: "等待用户确认费用", Progress: 0, Prompt: prompt,
		Operation: "image", Provider: "managed", Model: routed.LogicalModel.Code,
		LogicalModelID: routed.LogicalModel.ID, LogicalModelRevisionID: routed.Revision.ID,
		RouteID: routed.Route.ID, ChannelModelID: routed.ChannelModel.ID, RouteRun: 1, InputJSON: string(requestJSON),
	}
	billing, err := s.taskBillingOrder(userID, &task, input)
	if err != nil {
		return FilmProductionImageQuoteView{}, err
	}
	if billing != nil {
		billing.IdempotencyKey = "film-production:" + quoteID
	}
	billingJSON := ""
	if billing != nil {
		encoded, marshalErr := json.Marshal(billing)
		if marshalErr != nil {
			return FilmProductionImageQuoteView{}, marshalErr
		}
		billingJSON = string(encoded)
	}
	quote := model.FilmProductionQuote{
		ID: quoteID, UserID: userID, IdempotencyKey: idempotencyKey, ProjectID: projectID,
		RootRunID: sources.RootRun.ID, ShotID: sources.Shot.ID, RetryOfAttemptID: strings.TrimSpace(request.RetryOfAttemptID), TaskID: taskID,
		StoryboardArtifactID: sources.Storyboard.ID, StoryboardRevisionID: sources.StoryboardRevision.ID, StoryboardDigest: sources.StoryboardRevision.ContentDigest,
		PromptArtifactID: sources.Prompt.ID, PromptRevisionID: sources.PromptRevision.ID, PromptDigest: sources.PromptRevision.ContentDigest,
		FeasibilityArtifactID: sources.Feasibility.ID, FeasibilityRevisionID: sources.FeasibilityRevision.ID, FeasibilityDigest: sources.FeasibilityRevision.ContentDigest,
		RegistryID: sources.RootRun.RegistryID, RegistryVersion: sources.RootRun.RegistryVersion, RegistryDigest: sources.RootRun.RegistryDigest,
		LogicalModelID: routed.LogicalModel.ID, LogicalModelRevisionID: routed.Revision.ID,
		RouteID: routed.Route.ID, ChannelID: routed.ChannelModel.ChannelID, ChannelModelID: routed.ChannelModel.ID,
		Model: routed.LogicalModel.Code, ProviderModel: routed.ChannelModel.ModelKey, Capability: "image", Protocol: routed.ChannelModel.Protocol,
		CapabilityVersion: routed.ChannelModel.CapabilityVersion, ChannelPriceVersion: routed.ChannelModel.PriceVersion,
		RequestJSON: string(requestJSON), RequestFingerprint: requestFingerprint, BillingJSON: billingJSON,
		Status: model.FilmProductionQuoteStatusPending, ExpiresAt: now.Add(filmProductionQuoteTTL), CreatedAt: now, UpdatedAt: now,
	}
	quote.QuoteFingerprint, err = filmProductionQuoteFingerprint(quote)
	if err != nil {
		return FilmProductionImageQuoteView{}, err
	}
	if err := s.repo.CreateFilmProductionQuote(repository.FilmProductionQuoteCreateCommand{Quote: &quote, At: now}); err != nil {
		return FilmProductionImageQuoteView{}, mapFilmProductionError(err)
	}
	return filmProductionQuoteView(quote, false)
}

func (s *Service) SubmitFilmProductionImageQuote(userID string, projectID string, quoteID string, idempotencyKey string, request SubmitFilmProductionImageQuoteRequest) (SubmitFilmProductionImageQuoteResult, error) {
	if _, err := s.requireMutableFilmProject(userID, projectID); err != nil {
		return SubmitFilmProductionImageQuoteResult{}, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if !filmAgentIdempotencyKeyPattern.MatchString(idempotencyKey) {
		return SubmitFilmProductionImageQuoteResult{}, BadAuthRequest("X-Idempotency-Key 必须为 8-128 位字母、数字或 ._:-")
	}
	quote, err := s.repo.FilmProductionQuoteForUser(userID, projectID, strings.TrimSpace(quoteID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SubmitFilmProductionImageQuoteResult{}, NotFound("Film 生图报价不存在")
	}
	if err != nil {
		return SubmitFilmProductionImageQuoteResult{}, err
	}
	attemptID := newID()
	input, prompt, err := filmProductionSubmissionInput(*quote, attemptID)
	if err != nil {
		return SubmitFilmProductionImageQuoteResult{}, err
	}
	if err := s.protectTaskSecrets(input); err != nil {
		return SubmitFilmProductionImageQuoteResult{}, err
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return SubmitFilmProductionImageQuoteResult{}, err
	}
	policy, err := s.RuntimePolicy()
	if err != nil {
		return SubmitFilmProductionImageQuoteResult{}, err
	}
	now := time.Now().UTC()
	task := &model.Task{
		ID: quote.TaskID, UserID: userID, ProjectID: projectID, Type: model.FilmProductionTaskTypeImage,
		Status: model.TaskStatusQueued, Stage: "等待队列调度", Progress: 5, Prompt: prompt,
		Operation: "image", Provider: "managed", Model: quote.Model,
		LogicalModelID: quote.LogicalModelID, LogicalModelRevisionID: quote.LogicalModelRevisionID,
		RouteID: quote.RouteID, ChannelModelID: quote.ChannelModelID, RouteRun: 1, InputJSON: string(inputJSON),
	}
	command := repository.FilmProductionSubmitCommand{
		UserID: userID, ProjectID: projectID, QuoteID: quote.ID, QuoteFingerprint: strings.TrimSpace(request.QuoteFingerprint),
		IdempotencyKey: idempotencyKey, AttemptID: attemptID, AttemptArtifactID: newID(), AttemptRevisionID: newID(),
		Task: task, ActiveTaskLimit: policy.Task.ActiveTaskLimit, At: now,
		Event: repository.AgentRuntimeEventInput{
			ID: newID(), EventType: "film.production.image.submitted", ActorType: "human", ActorID: userID,
			PayloadJSON: mustFilmJSON(map[string]any{"quoteId": quote.ID, "taskId": quote.TaskID, "shotId": quote.ShotID}),
		},
	}
	stored, err := s.submitFilmProductionWithinStorageQuota(command, policy)
	if err != nil {
		return SubmitFilmProductionImageQuoteResult{}, mapFilmProductionError(err)
	}
	detail, err := s.repo.FilmProductionAttemptForUser(userID, projectID, stored.Attempt.ID)
	if err != nil {
		return SubmitFilmProductionImageQuoteResult{}, err
	}
	view, err := s.filmProductionAttemptView(*detail)
	if err != nil {
		return SubmitFilmProductionImageQuoteResult{}, err
	}
	return SubmitFilmProductionImageQuoteResult{Attempt: view, Idempotent: stored.Idempotent}, nil
}

func (s *Service) ListFilmProductionAttempts(userID string, projectID string, rootRunID string, shotID string, limit int) ([]FilmProductionAttemptView, error) {
	if _, err := s.filmProjectForUser(userID, projectID); err != nil {
		return nil, err
	}
	details, err := s.repo.FilmProductionAttemptsForProject(userID, projectID, strings.TrimSpace(rootRunID), strings.TrimSpace(shotID), limit)
	if err != nil {
		return nil, err
	}
	views := make([]FilmProductionAttemptView, 0, len(details))
	for _, detail := range details {
		view, viewErr := s.filmProductionAttemptView(detail)
		if viewErr != nil {
			return nil, viewErr
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *Service) CreateFilmProductionHumanQC(userID string, projectID string, attemptID string, idempotencyKey string, request CreateFilmProductionQCRequest) (CreateFilmProductionQCResult, error) {
	if _, err := s.requireMutableFilmProject(userID, projectID); err != nil {
		return CreateFilmProductionQCResult{}, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if !filmAgentIdempotencyKeyPattern.MatchString(idempotencyKey) {
		return CreateFilmProductionQCResult{}, BadAuthRequest("X-Idempotency-Key 必须为 8-128 位字母、数字或 ._:-")
	}
	detail, err := s.repo.FilmProductionAttemptForUser(userID, projectID, strings.TrimSpace(attemptID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return CreateFilmProductionQCResult{}, NotFound("Film 生图 Attempt 不存在")
	}
	if err != nil {
		return CreateFilmProductionQCResult{}, err
	}
	if detail.Result == nil || detail.Attempt.Status != model.FilmProductionAttemptStatusSucceeded {
		return CreateFilmProductionQCResult{}, conflictError("只有已取得真实图片结果的 Attempt 才能人工验收")
	}
	issueCodes, evidence, note, err := normalizeFilmProductionQCInput(request)
	if err != nil {
		return CreateFilmProductionQCResult{}, err
	}
	now := time.Now().UTC()
	reportID, artifactID, revisionID := newID(), newID(), newID()
	issueJSON, _ := json.Marshal(issueCodes)
	evidenceJSON, _ := json.Marshal(evidence)
	report := &model.FilmProductionQCReport{
		ID: reportID, UserID: userID, ProjectID: projectID, RootRunID: detail.Attempt.RootRunID, ShotID: detail.Attempt.ShotID,
		AttemptID: detail.Attempt.ID, ResultID: detail.Result.ID, Decision: request.Decision, Action: request.Action,
		IssueCodesJSON: string(issueJSON), EvidenceJSON: string(evidenceJSON), Note: note, Source: "human",
		AssessmentKind: "human_media_review", ReviewerUserID: userID,
		IdempotencyKey: idempotencyKey, ArtifactID: artifactID, RevisionID: revisionID, CreatedAt: now,
	}
	artifact, revision, err := buildFilmProductionHumanQCArtifact(detail.Attempt, *detail.Result, *report, now)
	if err != nil {
		return CreateFilmProductionQCResult{}, err
	}
	eventType := "film.production.qc.held"
	if request.Action == model.FilmProductionQCActionAccept {
		eventType = "film.production.qc.accepted"
	} else if request.Action == model.FilmProductionQCActionRetry {
		eventType = "film.production.qc.retry_requested"
	}
	created, idempotent, err := s.repo.CreateFilmProductionHumanQC(repository.FilmProductionHumanQCCommand{
		UserID: userID, ProjectID: projectID, AttemptID: detail.Attempt.ID, Report: report, Artifact: artifact, Revision: revision, At: now,
		Event: repository.AgentRuntimeEventInput{
			ID: newID(), EventType: eventType, ActorType: "human", ActorID: userID,
			PayloadJSON: mustFilmJSON(map[string]any{"attemptId": detail.Attempt.ID, "resultId": detail.Result.ID, "decision": request.Decision, "action": request.Action}),
		},
	})
	if err != nil {
		return CreateFilmProductionQCResult{}, mapFilmProductionError(err)
	}
	updated, err := s.repo.FilmProductionAttemptForUser(userID, projectID, detail.Attempt.ID)
	if err != nil {
		return CreateFilmProductionQCResult{}, err
	}
	view, err := s.filmProductionAttemptView(*updated)
	if err != nil {
		return CreateFilmProductionQCResult{}, err
	}
	return CreateFilmProductionQCResult{Report: filmProductionQCView(*created), Attempt: view, Idempotent: idempotent}, nil
}

func (s *Service) submitFilmProductionWithinStorageQuota(command repository.FilmProductionSubmitCommand, policy RuntimePolicySetting) (*repository.FilmProductionSubmitResult, error) {
	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	if existing, err := s.repo.FilmProductionAttemptByIdempotency(command.UserID, command.IdempotencyKey); err == nil {
		if existing.ProjectID != command.ProjectID || existing.QuoteID != command.QuoteID {
			return nil, repository.ErrFilmProductionStateConflict
		}
		return s.repo.SubmitFilmProductionQuote(command)
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
	return s.repo.SubmitFilmProductionQuote(command)
}

func mapFilmProductionError(err error) error {
	switch {
	case errors.Is(err, repository.ErrFilmProductionQuoteExpired):
		return conflictError("Film 生图报价已过期，请重新报价")
	case errors.Is(err, repository.ErrFilmProductionQuoteConsumed):
		return conflictError("Film 生图报价已被其他提交使用")
	case errors.Is(err, repository.ErrFilmProductionQuoteConflict):
		return conflictError("Film 生图报价指纹不匹配，请刷新后重试")
	case errors.Is(err, repository.ErrFilmProductionQuoteDrift):
		return conflictError("模型、价格或锁定产物已变化，请重新报价")
	case errors.Is(err, repository.ErrFilmProductionActiveAttempt):
		return conflictError("该镜头已有正在执行或费用待核对的 Attempt")
	case errors.Is(err, repository.ErrFilmProductionRetryNotAllowed):
		return conflictError("当前 Attempt 状态或 QC 结论不允许重试")
	case errors.Is(err, repository.ErrFilmProductionStateConflict):
		return conflictError("Film 生图状态已变化，请刷新后重试")
	case errors.Is(err, repository.ErrFilmProductionMediaMissing):
		return conflictError("生成结果没有可访问的媒体文件，不能验收")
	case errors.Is(err, repository.ErrInsufficientCredits):
		return BadAuthRequest("积分不足，请先使用兑换码充值")
	case errors.Is(err, repository.ErrActiveTaskLimit):
		return BadAuthRequest("同时排队或运行的任务已达到上限，请等待已有任务完成")
	case errors.Is(err, gorm.ErrRecordNotFound):
		return NotFound("Film 生图关联记录不存在")
	default:
		return err
	}
}

func filmProductionSourceRequestFingerprint(projectID string, request CreateFilmProductionImageQuoteRequest, sources filmProductionQuoteSources, prompt string, referenceIDs []string, options FilmProductionImageOptions) (string, error) {
	encoded, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "projectId": projectID, "rootRunId": sources.RootRun.ID, "shotId": sources.Shot.ID,
		"storyboardRevisionId": sources.StoryboardRevision.ID, "storyboardDigest": sources.StoryboardRevision.ContentDigest,
		"promptRevisionId": sources.PromptRevision.ID, "promptDigest": sources.PromptRevision.ContentDigest,
		"feasibilityRevisionId": sources.FeasibilityRevision.ID, "feasibilityDigest": sources.FeasibilityRevision.ContentDigest,
		"logicalModelId": strings.TrimSpace(request.LogicalModelID), "retryOfAttemptId": strings.TrimSpace(request.RetryOfAttemptID),
		"prompt": prompt, "referenceResourceIds": referenceIDs, "options": options, "count": 1,
	})
	if err != nil {
		return "", err
	}
	return digestBytesHex(encoded), nil
}

func filmProductionQuoteFingerprint(quote model.FilmProductionQuote) (string, error) {
	encoded, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "quoteId": quote.ID, "taskId": quote.TaskID, "projectId": quote.ProjectID,
		"rootRunId": quote.RootRunID, "shotId": quote.ShotID, "retryOfAttemptId": quote.RetryOfAttemptID,
		"storyboardRevisionId": quote.StoryboardRevisionID, "promptRevisionId": quote.PromptRevisionID,
		"feasibilityRevisionId": quote.FeasibilityRevisionID, "registryDigest": quote.RegistryDigest,
		"logicalModelId": quote.LogicalModelID, "logicalModelRevisionId": quote.LogicalModelRevisionID,
		"routeId": quote.RouteID, "channelModelId": quote.ChannelModelID, "capabilityVersion": quote.CapabilityVersion,
		"channelPriceVersion": quote.ChannelPriceVersion, "request": json.RawMessage(quote.RequestJSON),
		"requestFingerprint": quote.RequestFingerprint, "billing": json.RawMessage(firstNonEmpty(quote.BillingJSON, "null")),
		"expiresAt": quote.ExpiresAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return "", fmt.Errorf("encode Film Production quote fingerprint: %w", err)
	}
	return digestBytesHex(encoded), nil
}
