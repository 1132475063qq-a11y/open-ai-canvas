package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

const (
	maxFilmVideoSequenceSlots = 12
	maxFilmVideoPromptBytes   = 256 << 10
)

type filmVideoPromptCandidate struct {
	ShotID string
	Prompt string
}

type filmVideoAcceptedImage struct {
	Attempt  model.FilmProductionAttempt
	Result   model.Result
	Resource model.Resource
	Artifact model.ProductionArtifact
	Revision model.ProductionArtifactRevision
}

func extractFilmVideoPrompts(raw string) (map[string]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, BadAuthRequest("视频 Prompt Artifact 没有结构化内容")
	}
	var value any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, BadAuthRequest("视频 Prompt Artifact 内容不是有效 JSON")
	}
	if wrapper, ok := value.(map[string]any); ok {
		if content, exists := wrapper["content"]; exists {
			value = content
		}
	}
	candidates := make([]filmVideoPromptCandidate, 0)
	collectFilmVideoPromptCandidates(value, "", &candidates)
	result := make(map[string]string)
	for _, candidate := range candidates {
		shotID := strings.TrimSpace(candidate.ShotID)
		prompt, err := validateFilmVideoPrompt(candidate.Prompt)
		if err != nil {
			return nil, err
		}
		if shotID == "" {
			continue
		}
		if existing, exists := result[shotID]; exists && existing != prompt {
			return nil, BadAuthRequest("同一镜头存在多个不同的 video_prompt，必须先在 Prompt Artifact 中消除歧义")
		}
		result[shotID] = prompt
	}
	if len(result) == 0 {
		return nil, BadAuthRequest("视频 Prompt Artifact 中没有带 shot_id 的 video_prompt")
	}
	return result, nil
}

func collectFilmVideoPromptCandidates(value any, inheritedShotID string, candidates *[]filmVideoPromptCandidate) {
	switch item := value.(type) {
	case map[string]any:
		shotID := inheritedShotID
		for _, key := range []string{"shotId", "shot_id"} {
			if text, ok := item[key].(string); ok && strings.TrimSpace(text) != "" {
				shotID = strings.TrimSpace(text)
				break
			}
		}
		for _, key := range []string{"video_prompt", "videoPrompt", "animation_prompt", "animationPrompt"} {
			if text, ok := item[key].(string); ok && strings.TrimSpace(text) != "" {
				*candidates = append(*candidates, filmVideoPromptCandidate{ShotID: shotID, Prompt: strings.TrimSpace(text)})
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
			if childShotID == "" && strings.TrimSpace(key) != "" {
				childShotID = strings.TrimSpace(key)
			}
			collectFilmVideoPromptCandidates(item[key], childShotID, candidates)
		}
	case []any:
		for _, child := range item {
			collectFilmVideoPromptCandidates(child, inheritedShotID, candidates)
		}
	}
}

func validateFilmVideoPrompt(prompt string) (string, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" || !utf8.ValidString(prompt) {
		return "", BadAuthRequest("视频 Prompt 为空或编码无效")
	}
	if len([]byte(prompt)) > maxFilmVideoPromptBytes {
		return "", BadAuthRequest("视频 Prompt 超过 256KB")
	}
	return prompt, nil
}

func (s *Service) resolveFilmVideoAcceptedImage(userID string, projectID string, shotID string, attemptID string) (filmVideoAcceptedImage, error) {
	detail, err := s.repo.FilmProductionAttemptForUser(userID, projectID, strings.TrimSpace(attemptID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return filmVideoAcceptedImage{}, NotFound("来源图片 Attempt 不存在")
	}
	if err != nil {
		return filmVideoAcceptedImage{}, err
	}
	if detail.Attempt.ShotID != strings.TrimSpace(shotID) || detail.Attempt.Status != model.FilmProductionAttemptStatusSucceeded || detail.Result == nil {
		return filmVideoAcceptedImage{}, conflictError("视频槽位必须引用同一镜头已成功的图片 Attempt")
	}
	var latestHuman *model.FilmProductionQCReport
	for index := range detail.QCReports {
		if detail.QCReports[index].Source == "human" {
			latestHuman = &detail.QCReports[index]
		}
	}
	if latestHuman == nil || latestHuman.Decision != model.FilmProductionQCDecisionPass || latestHuman.Action != model.FilmProductionQCActionAccept ||
		latestHuman.ResultID != detail.Result.ID {
		return filmVideoAcceptedImage{}, conflictError("只有人工 PASS 并接受的图片才能进入视频阶段")
	}
	media, err := filmProductionMediaFactFromResult([]byte(detail.Result.Payload))
	if err != nil || strings.TrimSpace(media.ResourceID) == "" {
		return filmVideoAcceptedImage{}, conflictError("已接受图片缺少可持久引用的资源，请重新生成或修复资源")
	}
	resource, err := s.repo.ResourceForUser(userID, media.ResourceID)
	if err != nil {
		return filmVideoAcceptedImage{}, err
	}
	if resource.Status != model.ResourceStatusReady || resource.Kind != "image" || !strings.HasPrefix(resource.MimeType, "image/") {
		return filmVideoAcceptedImage{}, conflictError("已接受图片资源当前不可用于图生视频")
	}
	artifact, revision, err := s.repo.ProductionArtifactRevisionForUser(userID, detail.Attempt.ResultRevisionID)
	if err != nil {
		return filmVideoAcceptedImage{}, err
	}
	if artifact.ID != detail.Attempt.ResultArtifactID || artifact.ProjectID != projectID || artifact.Domain != "film" ||
		artifact.ArtifactType != "generation-result" || revision.Status != model.ProductionArtifactStatusLocked {
		return filmVideoAcceptedImage{}, conflictError("来源图片 Result Artifact 已变化")
	}
	return filmVideoAcceptedImage{
		Attempt: detail.Attempt, Result: *detail.Result, Resource: *resource, Artifact: *artifact, Revision: *revision,
	}, nil
}

func filmVideoReferenceMedia(resource model.Resource) providerMedia {
	return providerMedia{
		ID: resource.ID, Name: resource.ID, Type: resource.MimeType, StorageKey: "resource:" + resource.ID,
		MimeType: resource.MimeType, Bytes: resource.Size, Width: resource.Width, Height: resource.Height,
	}
}

func normalizeFilmVideoAspectRatio(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "9:16", nil
	}
	switch value {
	case "9:16", "16:9", "1:1", "4:3", "3:4":
		return value, nil
	default:
		return "", BadAuthRequest("视频序列比例只支持 9:16、16:9、1:1、4:3 或 3:4")
	}
}

func normalizeFilmVideoDuration(value int64, shotDuration int64) (int64, error) {
	if value == 0 {
		value = shotDuration
	}
	if value == 0 {
		value = 3_000
	}
	if value < 1_000 || value > 30_000 {
		return 0, BadAuthRequest("单个视频镜头时长必须在 1-30 秒之间")
	}
	return value, nil
}

func filmVideoSequenceStatusAfterPlanUpdate(slots []model.FilmVideoSlot) model.FilmVideoSequenceStatus {
	if len(slots) == 0 {
		return model.FilmVideoSequenceStatusReady
	}
	allAccepted := true
	hasGenerating := false
	hasReview := false
	for _, slot := range slots {
		if slot.Status != model.FilmVideoSlotStatusAccepted {
			allAccepted = false
		}
		switch slot.Status {
		case model.FilmVideoSlotStatusQueued, model.FilmVideoSlotStatusRunning:
			hasGenerating = true
		case model.FilmVideoSlotStatusNeedsReview, model.FilmVideoSlotStatusFailed, model.FilmVideoSlotStatusCancelled, model.FilmVideoSlotStatusUncertain:
			hasReview = true
		}
	}
	if allAccepted {
		return model.FilmVideoSequenceStatusNeedsReview
	}
	if hasGenerating {
		return model.FilmVideoSequenceStatusGenerating
	}
	if hasReview {
		return model.FilmVideoSequenceStatusNeedsReview
	}
	return model.FilmVideoSequenceStatusReady
}

func (s *Service) resolveFilmVideoMusicResource(userID string, resourceID string) (string, int64, error) {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return "", 0, nil
	}
	resource, err := s.repo.ResourceForUser(userID, resourceID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", 0, NotFound("基础音乐资源不存在")
	}
	if err != nil {
		return "", 0, err
	}
	if resource.Status != model.ResourceStatusReady || resource.Kind != "audio" || !strings.HasPrefix(strings.ToLower(resource.MimeType), "audio/") {
		return "", 0, conflictError("基础音乐资源当前不可用于视频序列")
	}
	return resource.ID, resource.DurationMs, nil
}

func filmVideoSeconds(durationMs int64) string {
	seconds := int64(math.Ceil(float64(durationMs) / 1000))
	if seconds < 1 {
		seconds = 1
	}
	return strconv.FormatInt(seconds, 10)
}

func buildFilmVideoSequenceArtifact(sequence model.FilmVideoSequence, slots []model.FilmVideoSlot, at time.Time) (*model.ProductionArtifact, *model.ProductionArtifactRevision, error) {
	slotContent := make([]map[string]any, 0, len(slots))
	sourceRefs := []map[string]any{{
		"artifactId": sequence.PromptArtifactID, "revisionId": sequence.PromptArtifactRevisionID, "type": "ai-video-prompts", "digest": sequence.PromptArtifactDigest,
	}}
	for _, slot := range slots {
		slotContent = append(slotContent, map[string]any{
			"slotId": slot.ID, "position": slot.Position, "shotId": slot.ShotID, "durationMs": slot.DurationMs,
			"prompt": slot.Prompt, "sourceImageAttemptId": slot.SourceImageAttemptID,
			"sourceImageResultId": slot.SourceImageResultID, "sourceImageResourceId": slot.SourceImageResourceID,
		})
		sourceRefs = append(sourceRefs, map[string]any{
			"artifactId": slot.SourceImageArtifactID, "revisionId": slot.SourceImageRevisionID, "type": "generation-result",
		})
	}
	content, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "artifactType": "video-sequence", "sequenceId": sequence.ID, "title": sequence.Title,
		"aspectRatio": sequence.AspectRatio, "targetDurationMs": sequence.TargetDurationMs,
		"musicResourceId": filmProductionEmptyStringAsNil(sequence.MusicResourceID), "musicDurationMs": sequence.MusicDurationMs,
		"slots": slotContent,
	})
	if err != nil {
		return nil, nil, err
	}
	artifact := &model.ProductionArtifact{
		ID: sequence.ArtifactID, UserID: sequence.UserID, ProjectID: sequence.ProjectID, Domain: "film",
		ArtifactType: "video-sequence", LogicalKey: "film-video:sequence:" + sequence.ID,
	}
	revision := &model.ProductionArtifactRevision{
		ID: sequence.ArtifactRevisionID, Status: model.ProductionArtifactStatusLocked, ContentJSON: string(content), ContentDigest: digestBytesHex(content),
		SourceRunID: sequence.RootRunID, SourceArtifactRefsJSON: mustFilmJSON(sourceRefs),
		AuthorityRefsJSON: mustFilmJSON([]map[string]any{
			{"kind": "registry", "id": sequence.RegistryID, "version": sequence.RegistryVersion, "digest": sequence.RegistryDigest},
			{"kind": "human", "id": sequence.UserID},
		}),
		CreatedByType: "human", CreatedByID: sequence.UserID, CreatedAt: at,
	}
	return artifact, revision, nil
}

func filmVideoSequenceFingerprint(projectID string, request CreateFilmVideoSequenceRequest, promptRevision model.ProductionArtifactRevision, slots []model.FilmVideoSlot, aspectRatio string, targetDurationMs int64, musicResourceID string, musicDurationMs int64) (string, error) {
	items := make([]map[string]any, 0, len(slots))
	for _, slot := range slots {
		items = append(items, map[string]any{
			"position": slot.Position, "shotId": slot.ShotID, "prompt": slot.Prompt, "durationMs": slot.DurationMs,
			"sourceImageAttemptId": slot.SourceImageAttemptID, "sourceImageResultId": slot.SourceImageResultID,
			"sourceImageResourceId": slot.SourceImageResourceID, "sourceImageRevisionId": slot.SourceImageRevisionID,
		})
	}
	encoded, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "projectId": projectID, "rootRunId": strings.TrimSpace(request.RootRunID),
		"promptRevisionId": promptRevision.ID, "promptDigest": promptRevision.ContentDigest, "title": strings.TrimSpace(request.Title),
		"aspectRatio": aspectRatio, "targetDurationMs": targetDurationMs, "musicResourceId": musicResourceID,
		"musicDurationMs": musicDurationMs, "slots": items,
	})
	if err != nil {
		return "", err
	}
	return digestBytesHex(encoded), nil
}

func filmVideoQuoteFingerprint(quote model.FilmVideoQuote) (string, error) {
	encoded, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "quoteId": quote.ID, "taskId": quote.TaskID, "projectId": quote.ProjectID,
		"rootRunId": quote.RootRunID, "sequenceId": quote.SequenceID, "sequenceRevision": quote.SequenceRevision,
		"slotId": quote.SlotID, "slotRevision": quote.SlotRevision, "retryOfAttemptId": quote.RetryOfAttemptID,
		"sourceImageAttemptId": quote.SourceImageAttemptID, "sourceImageResultId": quote.SourceImageResultID,
		"sourceImageResourceId": quote.SourceImageResourceID, "sourceImageRevisionId": quote.SourceImageRevisionID,
		"promptRevisionId": quote.PromptArtifactRevisionID, "promptDigest": quote.PromptArtifactDigest, "registryDigest": quote.RegistryDigest,
		"logicalModelId": quote.LogicalModelID, "logicalModelRevisionId": quote.LogicalModelRevisionID,
		"routeId": quote.RouteID, "channelModelId": quote.ChannelModelID, "capabilityVersion": quote.CapabilityVersion,
		"channelPriceVersion": quote.ChannelPriceVersion, "request": json.RawMessage(quote.RequestJSON),
		"requestFingerprint": quote.RequestFingerprint, "billing": json.RawMessage(firstNonEmpty(quote.BillingJSON, "null")),
		"expiresAt": quote.ExpiresAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return "", fmt.Errorf("encode Film video quote fingerprint: %w", err)
	}
	return digestBytesHex(encoded), nil
}

func filmVideoQuoteRequestFingerprint(projectID string, sequence model.FilmVideoSequence, slot model.FilmVideoSlot, logicalModelID string, retryOf string, options FilmVideoOptions) (string, error) {
	encoded, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "projectId": projectID, "rootRunId": sequence.RootRunID, "sequenceId": sequence.ID,
		"sequenceRevision": sequence.Revision, "slotId": slot.ID, "slotRevision": slot.Revision, "prompt": slot.Prompt,
		"sourceImageAttemptId": slot.SourceImageAttemptID, "sourceImageResultId": slot.SourceImageResultID,
		"sourceImageResourceId": slot.SourceImageResourceID, "sourceImageRevisionId": slot.SourceImageRevisionID,
		"promptRevisionId": sequence.PromptArtifactRevisionID, "promptDigest": sequence.PromptArtifactDigest,
		"logicalModelId": strings.TrimSpace(logicalModelID), "retryOfAttemptId": strings.TrimSpace(retryOf), "options": options,
	})
	if err != nil {
		return "", err
	}
	return digestBytesHex(encoded), nil
}
