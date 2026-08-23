package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/gorm"
)

const (
	maxFilmProductionPromptBytes = 256 << 10
	maxFilmProductionQCBytes     = 64 << 10
)

var filmProductionIssueCodePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_.:-]{0,79}$`)

type filmProductionQuoteSources struct {
	RootRun             model.AgentRuntimeRun
	Shot                model.Shot
	Storyboard          model.ProductionArtifact
	StoryboardRevision  model.ProductionArtifactRevision
	Prompt              model.ProductionArtifact
	PromptRevision      model.ProductionArtifactRevision
	Feasibility         model.ProductionArtifact
	FeasibilityRevision model.ProductionArtifactRevision
}

type filmProductionPromptCandidate struct {
	ShotID string
	Prompt string
}

func (s *Service) resolveFilmProductionQuoteSources(userID string, projectID string, request CreateFilmProductionImageQuoteRequest) (filmProductionQuoteSources, string, error) {
	rootRunID := strings.TrimSpace(request.RootRunID)
	shotID := strings.TrimSpace(request.ShotID)
	if rootRunID == "" || shotID == "" {
		return filmProductionQuoteSources{}, "", BadAuthRequest("Film 生图必须指定 RootRun 和镜头")
	}
	root, err := s.requireFilmAgentRootRun(userID, projectID, rootRunID)
	if err != nil {
		return filmProductionQuoteSources{}, "", err
	}
	if root.RootRunID != root.ID || s.filmAgentRegistry == nil || root.RegistryID != s.filmAgentRegistry.ID ||
		root.RegistryVersion != s.filmAgentRegistry.Version || root.RegistryDigest != s.filmAgentRegistry.SourceDigest {
		return filmProductionQuoteSources{}, "", conflictError("Film AgentTeam Registry 已变化，请重新生成并锁定制作产物")
	}
	shot, err := s.repo.ShotForProject(projectID, shotID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return filmProductionQuoteSources{}, "", NotFound("短剧镜头不存在")
	}
	if err != nil {
		return filmProductionQuoteSources{}, "", err
	}

	storyboard, storyboardRevision, err := s.lockedFilmProductionArtifact(userID, projectID, request.StoryboardArtifactRevisionID, []string{"storyboard"})
	if err != nil {
		return filmProductionQuoteSources{}, "", fmt.Errorf("分镜产物不可用于生图：%w", err)
	}
	promptArtifact, promptRevision, err := s.lockedFilmProductionArtifact(userID, projectID, request.PromptArtifactRevisionID, []string{"image-prompt-pack", "prompt-manifest"})
	if err != nil {
		return filmProductionQuoteSources{}, "", fmt.Errorf("Prompt 产物不可用于生图：%w", err)
	}
	feasibility, feasibilityRevision, err := s.lockedFilmProductionArtifact(userID, projectID, request.FeasibilityRevisionID, []string{"production-feasibility-report"})
	if err != nil {
		return filmProductionQuoteSources{}, "", fmt.Errorf("制作可行性产物不可用于生图：%w", err)
	}
	prompt, err := extractFilmProductionImagePrompt(promptRevision.ContentJSON, shotID)
	if err != nil {
		return filmProductionQuoteSources{}, "", err
	}
	return filmProductionQuoteSources{
		RootRun: *root, Shot: *shot, Storyboard: *storyboard, StoryboardRevision: *storyboardRevision,
		Prompt: *promptArtifact, PromptRevision: *promptRevision, Feasibility: *feasibility, FeasibilityRevision: *feasibilityRevision,
	}, prompt, nil
}

func (s *Service) lockedFilmProductionArtifact(userID string, projectID string, revisionID string, allowedTypes []string) (*model.ProductionArtifact, *model.ProductionArtifactRevision, error) {
	revisionID = strings.TrimSpace(revisionID)
	if revisionID == "" {
		return nil, nil, BadAuthRequest("缺少锁定 Artifact revision")
	}
	artifact, revision, err := s.repo.ProductionArtifactRevisionForUser(userID, revisionID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, NotFound("锁定 Artifact revision 不存在")
	}
	if err != nil {
		return nil, nil, err
	}
	allowed := false
	for _, artifactType := range allowedTypes {
		if artifact.ArtifactType == artifactType {
			allowed = true
			break
		}
	}
	if artifact.ProjectID != projectID || artifact.Domain != "film" || !allowed {
		return nil, nil, BadAuthRequest("Artifact 类型或项目归属不符合 Film 生图要求")
	}
	if revision.Status != model.ProductionArtifactStatusLocked || artifact.CurrentRevisionID != revision.ID || strings.TrimSpace(revision.ContentDigest) == "" {
		return nil, nil, conflictError("Artifact 必须使用当前已锁定 revision")
	}
	return artifact, revision, nil
}

func extractFilmProductionImagePrompt(raw string, shotID string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", BadAuthRequest("Prompt Artifact 没有结构化内容")
	}
	var value any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return "", BadAuthRequest("Prompt Artifact 内容不是有效 JSON")
	}
	if wrapper, ok := value.(map[string]any); ok {
		if content, exists := wrapper["content"]; exists {
			value = content
		}
	}
	candidates := make([]filmProductionPromptCandidate, 0)
	collectFilmProductionPromptCandidates(value, "", strings.TrimSpace(shotID), &candidates)
	candidates = uniqueFilmProductionPromptCandidates(candidates)
	for _, candidate := range candidates {
		if candidate.ShotID == shotID {
			return validateFilmProductionPrompt(candidate.Prompt)
		}
	}
	withoutShot := make([]filmProductionPromptCandidate, 0)
	for _, candidate := range candidates {
		if candidate.ShotID == "" {
			withoutShot = append(withoutShot, candidate)
		}
	}
	if len(withoutShot) == 1 {
		return validateFilmProductionPrompt(withoutShot[0].Prompt)
	}
	if len(candidates) == 1 {
		return validateFilmProductionPrompt(candidates[0].Prompt)
	}
	if len(candidates) == 0 {
		return "", BadAuthRequest("Prompt Artifact 中没有可执行的图片 prompt_text")
	}
	return "", BadAuthRequest("Prompt Artifact 含多个镜头提示词，但没有与当前 shotId 精确匹配")
}

func collectFilmProductionPromptCandidates(value any, inheritedShotID string, targetShotID string, candidates *[]filmProductionPromptCandidate) {
	switch item := value.(type) {
	case map[string]any:
		shotID := inheritedShotID
		for _, key := range []string{"shotId", "shot_id"} {
			if text, ok := item[key].(string); ok && strings.TrimSpace(text) != "" {
				shotID = strings.TrimSpace(text)
				break
			}
		}
		for _, key := range []string{"prompt_text", "promptText", "image_prompt", "imagePrompt", "model_prompt", "modelPrompt", "final_prompt", "finalPrompt", "positive_prompt", "positivePrompt", "prompt"} {
			if text, ok := item[key].(string); ok && strings.TrimSpace(text) != "" {
				*candidates = append(*candidates, filmProductionPromptCandidate{ShotID: shotID, Prompt: strings.TrimSpace(text)})
				break
			}
		}
		keys := make([]string, 0, len(item))
		for key := range item {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			childShotID := shotID
			if key == targetShotID {
				childShotID = targetShotID
			}
			collectFilmProductionPromptCandidates(item[key], childShotID, targetShotID, candidates)
		}
	case []any:
		for _, child := range item {
			collectFilmProductionPromptCandidates(child, inheritedShotID, targetShotID, candidates)
		}
	}
}

func uniqueFilmProductionPromptCandidates(input []filmProductionPromptCandidate) []filmProductionPromptCandidate {
	result := make([]filmProductionPromptCandidate, 0, len(input))
	seen := make(map[string]bool, len(input))
	for _, candidate := range input {
		key := candidate.ShotID + "\x00" + candidate.Prompt
		if !seen[key] {
			seen[key] = true
			result = append(result, candidate)
		}
	}
	return result
}

func validateFilmProductionPrompt(prompt string) (string, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" || !utf8.ValidString(prompt) {
		return "", BadAuthRequest("图片 Prompt 为空或编码无效")
	}
	if len([]byte(prompt)) > maxFilmProductionPromptBytes {
		return "", BadAuthRequest("图片 Prompt 超过 256KB")
	}
	return prompt, nil
}

func (s *Service) resolveFilmProductionImageReferences(userID string, rawIDs []string) ([]providerMedia, []string, error) {
	if len(rawIDs) > 32 {
		return nil, nil, BadAuthRequest("Film 单镜头最多引用 32 张图片")
	}
	media := make([]providerMedia, 0, len(rawIDs))
	ids := make([]string, 0, len(rawIDs))
	seen := make(map[string]bool, len(rawIDs))
	for _, rawID := range rawIDs {
		resourceID := strings.TrimSpace(rawID)
		if resourceID == "" || len(resourceID) > 80 {
			return nil, nil, BadAuthRequest("参考图片资源 ID 无效")
		}
		if seen[resourceID] {
			continue
		}
		seen[resourceID] = true
		resource, err := s.repo.ResourceForUser(userID, resourceID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, NotFound("参考图片资源不存在或不属于当前用户")
		}
		if err != nil {
			return nil, nil, err
		}
		if resource.Status != model.ResourceStatusReady || resource.Kind != "image" || !strings.HasPrefix(resource.MimeType, "image/") {
			return nil, nil, BadAuthRequest("参考资源必须是已上传完成的图片")
		}
		ids = append(ids, resourceID)
		media = append(media, providerMedia{
			ID: resource.ID, Name: resource.ID, Type: resource.MimeType, StorageKey: "resource:" + resource.ID,
			MimeType: resource.MimeType, Bytes: resource.Size, Width: resource.Width, Height: resource.Height,
		})
	}
	return media, ids, nil
}

func normalizeFilmProductionImageOptions(input FilmProductionImageOptions) FilmProductionImageOptions {
	input.Size = strings.TrimSpace(input.Size)
	input.Quality = strings.TrimSpace(input.Quality)
	return input
}

func filmProductionCapabilityOptions(options FilmProductionImageOptions) map[string]any {
	result := map[string]any{}
	if options.Size != "" {
		result["size"] = options.Size
	}
	if options.Quality != "" {
		result["quality"] = options.Quality
	}
	if options.TransparentBackground != nil {
		result["transparentBackground"] = *options.TransparentBackground
	}
	return result
}

func (s *Service) validateFilmProductionRetryRequest(userID string, projectID string, rootRunID string, shotID string, retryOf string, promptRevisionID string) error {
	details, err := s.repo.FilmProductionAttemptsForProject(userID, projectID, rootRunID, shotID, 100)
	if err != nil {
		return err
	}
	if retryOf == "" {
		if len(details) > 0 {
			return conflictError("该镜头已有 Attempt，后续生成必须明确引用最近一次 Attempt")
		}
		return nil
	}
	if len(details) == 0 || details[0].Attempt.ID != retryOf {
		return conflictError("付费重试只能引用该镜头最近一次 Attempt")
	}
	source := details[0]
	switch source.Attempt.Status {
	case model.FilmProductionAttemptStatusFailed, model.FilmProductionAttemptStatusCancelled:
		return nil
	case model.FilmProductionAttemptStatusSucceeded:
		var latestHuman *model.FilmProductionQCReport
		for index := range source.QCReports {
			if source.QCReports[index].Source == "human" {
				latestHuman = &source.QCReports[index]
			}
		}
		if latestHuman == nil || latestHuman.Decision != model.FilmProductionQCDecisionFail || latestHuman.Action != model.FilmProductionQCActionRetry {
			return conflictError("已有图片必须先由人工标记 FAIL 并请求重试")
		}
		if source.Attempt.PromptRevisionID == promptRevisionID {
			return conflictError("图片失败后的付费重试必须先锁定新的 Prompt revision")
		}
		return nil
	default:
		return conflictError("当前 Attempt 尚未形成可安全重试的费用终态")
	}
}

func filmProductionSubmissionInput(quote model.FilmProductionQuote, attemptID string) (map[string]any, string, error) {
	var input map[string]any
	if err := json.Unmarshal([]byte(quote.RequestJSON), &input); err != nil {
		return nil, "", conflictError("Film 生图报价请求快照已损坏")
	}
	prompt, _ := input["prompt"].(string)
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil, "", conflictError("Film 生图报价缺少 Prompt")
	}
	metadata, _ := input["metadata"].(map[string]any)
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["filmRootRunId"] = quote.RootRunID
	metadata["filmShotId"] = quote.ShotID
	metadata["filmQuoteId"] = quote.ID
	metadata["filmAttemptId"] = attemptID
	input["metadata"] = metadata
	return input, prompt, nil
}

func filmProductionQuoteView(quote model.FilmProductionQuote, idempotent bool) (FilmProductionImageQuoteView, error) {
	var input canvasGenerationInput
	if err := json.Unmarshal([]byte(quote.RequestJSON), &input); err != nil {
		return FilmProductionImageQuoteView{}, err
	}
	referenceIDs := make([]string, 0, len(input.ReferenceImages))
	for _, reference := range input.ReferenceImages {
		if strings.HasPrefix(reference.StorageKey, "resource:") {
			referenceIDs = append(referenceIDs, strings.TrimPrefix(reference.StorageKey, "resource:"))
		}
	}
	options := FilmProductionImageOptions{Size: input.Config.Size, Quality: input.Config.Quality}
	if value := strings.TrimSpace(input.Config.TransparentBackground); value != "" {
		parsed := strings.EqualFold(value, "true")
		options.TransparentBackground = &parsed
	}
	billing, err := filmProductionQuoteBilling(quote)
	if err != nil {
		return FilmProductionImageQuoteView{}, err
	}
	status := quote.Status
	if status == model.FilmProductionQuoteStatusPending && !quote.ExpiresAt.After(time.Now()) {
		status = model.FilmProductionQuoteStatusExpired
	}
	artifacts := []FilmProductionArtifactRef{
		{ArtifactID: quote.StoryboardArtifactID, RevisionID: quote.StoryboardRevisionID, Type: "storyboard", Digest: quote.StoryboardDigest, Status: string(model.ProductionArtifactStatusLocked)},
		{ArtifactID: quote.PromptArtifactID, RevisionID: quote.PromptRevisionID, Type: "prompt", Digest: quote.PromptDigest, Status: string(model.ProductionArtifactStatusLocked)},
		{ArtifactID: quote.FeasibilityArtifactID, RevisionID: quote.FeasibilityRevisionID, Type: "production-feasibility-report", Digest: quote.FeasibilityDigest, Status: string(model.ProductionArtifactStatusLocked)},
	}
	return FilmProductionImageQuoteView{
		ID: quote.ID, ProjectID: quote.ProjectID, RootRunID: quote.RootRunID, ShotID: quote.ShotID,
		RetryOfAttemptID: quote.RetryOfAttemptID, Artifacts: artifacts, LogicalModelID: quote.LogicalModelID, Model: quote.Model,
		Prompt: input.Prompt, ReferenceResourceIDs: referenceIDs, Options: options, Count: 1, Cost: filmProductionCostView(billing),
		QuoteFingerprint: quote.QuoteFingerprint, RequestFingerprint: quote.RequestFingerprint, Status: status,
		ExpiresAt: quote.ExpiresAt, CreatedAt: quote.CreatedAt, Idempotent: idempotent,
	}, nil
}

func filmProductionQuoteBilling(quote model.FilmProductionQuote) (*model.BillingOrder, error) {
	if strings.TrimSpace(quote.BillingJSON) == "" || strings.TrimSpace(quote.BillingJSON) == "null" {
		return nil, nil
	}
	var order model.BillingOrder
	if err := json.Unmarshal([]byte(quote.BillingJSON), &order); err != nil {
		return nil, err
	}
	return &order, nil
}

func filmProductionCostView(order *model.BillingOrder) FilmProductionCostView {
	if order == nil {
		return FilmProductionCostView{Required: false}
	}
	return FilmProductionCostView{
		Required: true, BillingOrderID: order.ID, BillingMode: order.BillingMode, PriceVersion: order.PriceVersion,
		Quantity: order.Quantity, AmountMicrocredits: order.AmountMicrocredits, ReservedAmountMicrocredits: order.ReservedAmountMicrocredits,
		ActualAmountMicrocredits: order.ActualAmountMicrocredits, RefundedAmountMicrocredits: order.RefundedAmountMicrocredits, Status: order.Status,
	}
}

func (s *Service) filmProductionAttemptView(detail repository.FilmProductionAttemptDetail) (FilmProductionAttemptView, error) {
	task := detail.Task
	s.hydrateTaskProviderRequestID(&task)
	qcViews := make([]FilmProductionQCView, 0, len(detail.QCReports))
	var current *FilmProductionQCView
	var currentHuman *FilmProductionQCView
	for _, report := range detail.QCReports {
		view := filmProductionQCView(report)
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
	visualQCViews := make([]FilmVisualQCAttemptView, 0, len(detail.VisualQCAttempts))
	for _, visualQC := range detail.VisualQCAttempts {
		view, err := s.filmVisualQCAttemptView(visualQC)
		if err != nil {
			return FilmProductionAttemptView{}, err
		}
		visualQCViews = append(visualQCViews, view)
	}
	return FilmProductionAttemptView{
		Attempt: detail.Attempt, Task: taskSummaryForOutput(task), Result: detail.Result, QCReports: qcViews,
		CurrentQC: current, VisualQCAttempts: visualQCViews, Cost: filmProductionCostView(detail.Billing), Accepted: accepted, RetryAllowed: retryAllowed,
	}, nil
}

func filmProductionQCView(report model.FilmProductionQCReport) FilmProductionQCView {
	issueCodes := []string{}
	evidence := map[string]any{}
	_ = json.Unmarshal([]byte(firstNonEmpty(report.IssueCodesJSON, "[]")), &issueCodes)
	_ = json.Unmarshal([]byte(firstNonEmpty(report.EvidenceJSON, "{}")), &evidence)
	return FilmProductionQCView{
		ID: report.ID, AttemptID: report.AttemptID, ResultID: report.ResultID, Decision: report.Decision, Action: report.Action,
		IssueCodes: issueCodes, Evidence: evidence, Note: report.Note, Source: report.Source,
		AssessmentKind: report.AssessmentKind, ModelAttemptID: report.ModelAttemptID, ReviewerUserID: report.ReviewerUserID,
		ArtifactID: report.ArtifactID, RevisionID: report.RevisionID, CreatedAt: report.CreatedAt,
	}
}

func normalizeFilmProductionQCInput(request CreateFilmProductionQCRequest) ([]string, map[string]any, string, error) {
	switch request.Decision {
	case model.FilmProductionQCDecisionPass:
		if request.Action != model.FilmProductionQCActionAccept {
			return nil, nil, "", BadAuthRequest("PASS 必须对应 accept")
		}
	case model.FilmProductionQCDecisionFail:
		if request.Action != model.FilmProductionQCActionRetry {
			return nil, nil, "", BadAuthRequest("FAIL 必须对应 retry")
		}
	case model.FilmProductionQCDecisionUncertain:
		if request.Action != model.FilmProductionQCActionHold {
			return nil, nil, "", BadAuthRequest("UNCERTAIN 必须对应 hold")
		}
	default:
		return nil, nil, "", BadAuthRequest("人工 QC 只接受 PASS、UNCERTAIN 或 FAIL")
	}
	issueCodes := make([]string, 0, len(request.IssueCodes))
	seen := map[string]bool{}
	for _, raw := range request.IssueCodes {
		code := strings.ToUpper(strings.TrimSpace(raw))
		if !filmProductionIssueCodePattern.MatchString(code) {
			return nil, nil, "", BadAuthRequest("QC issueCode 必须为 1-80 位大写字母、数字或 ._:-")
		}
		if !seen[code] {
			seen[code] = true
			issueCodes = append(issueCodes, code)
		}
	}
	if len(issueCodes) > 50 {
		return nil, nil, "", BadAuthRequest("单次 QC 最多记录 50 个问题代码")
	}
	evidence := request.Evidence
	if evidence == nil {
		evidence = map[string]any{}
	}
	if containsInlineMediaDataURL(evidence) || containsAgentRuntimeSecret(evidence) {
		return nil, nil, "", BadAuthRequest("QC 证据不能包含内嵌媒体或密钥")
	}
	encoded, err := json.Marshal(evidence)
	if err != nil || len(encoded) > maxFilmProductionQCBytes {
		return nil, nil, "", BadAuthRequest("QC 证据格式无效或超过 64KB")
	}
	note := strings.TrimSpace(request.Note)
	if utf8.RuneCountInString(note) > 2000 {
		return nil, nil, "", BadAuthRequest("QC 备注不能超过 2000 个字符")
	}
	if request.Decision != model.FilmProductionQCDecisionPass && len(issueCodes) == 0 && note == "" {
		return nil, nil, "", BadAuthRequest("UNCERTAIN 或 FAIL 必须填写问题代码或备注")
	}
	return issueCodes, evidence, note, nil
}

func buildFilmProductionHumanQCArtifact(attempt model.FilmProductionAttempt, result model.Result, report model.FilmProductionQCReport, at time.Time) (*model.ProductionArtifact, *model.ProductionArtifactRevision, error) {
	var issueCodes []string
	var evidence map[string]any
	if err := json.Unmarshal([]byte(report.IssueCodesJSON), &issueCodes); err != nil {
		return nil, nil, err
	}
	if err := json.Unmarshal([]byte(report.EvidenceJSON), &evidence); err != nil {
		return nil, nil, err
	}
	content, err := json.Marshal(map[string]any{
		"schemaVersion": 2, "artifactType": "generation-qc-report", "reportId": report.ID,
		"attemptId": attempt.ID, "resultId": result.ID, "shotId": attempt.ShotID,
		"decision": report.Decision, "action": report.Action, "issueCodes": issueCodes, "evidence": evidence,
		"note": report.Note, "source": "human", "reviewerUserId": report.ReviewerUserID,
	})
	if err != nil {
		return nil, nil, err
	}
	artifact := &model.ProductionArtifact{
		ID: report.ArtifactID, UserID: attempt.UserID, ProjectID: attempt.ProjectID, Domain: "film",
		ArtifactType: "generation-qc-report", LogicalKey: "film-production:human-qc:" + report.ID,
	}
	revision := &model.ProductionArtifactRevision{
		ID: report.RevisionID, Status: model.ProductionArtifactStatusLocked, ContentJSON: string(content), ContentDigest: digestBytesHex(content),
		SourceRunID: attempt.RootRunID, SourceAttemptID: attempt.ID,
		SourceArtifactRefsJSON: mustFilmJSON([]map[string]any{
			{"artifactId": attempt.ResultArtifactID, "revisionId": attempt.ResultRevisionID, "type": "generation-result"},
			{"artifactId": attempt.AttemptArtifactID, "revisionId": attempt.AttemptRevisionID, "type": "generation-attempt"},
		}),
		AuthorityRefsJSON: mustFilmJSON([]map[string]any{
			{"kind": "registry", "id": attempt.RegistryID, "version": attempt.RegistryVersion, "digest": attempt.RegistryDigest},
			{"kind": "human", "id": report.ReviewerUserID},
		}),
		CreatedByType: "human", CreatedByID: report.ReviewerUserID, CreatedAt: at,
	}
	return artifact, revision, nil
}
