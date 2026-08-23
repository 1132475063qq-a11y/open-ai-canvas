package service

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

const filmVideoWorkerID = "film-video-worker-v1"

type filmVideoGeneratedPayload struct {
	Mode  string                       `json:"mode"`
	Video filmProductionGeneratedImage `json:"video"`
}

func (s *Service) saveFilmVideoTaskCompletionWithinStorageQuota(task *model.Task, resultJSON []byte) error {
	if task == nil || task.Type != model.FilmProductionTaskTypeVideo {
		return errors.New("Film video completion requires a Film video task")
	}
	policy, err := s.RuntimePolicy()
	if err != nil {
		return err
	}
	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	usage, err := s.repo.UserStorageUsage(task.UserID)
	if err != nil {
		return err
	}
	attempt, err := s.repo.FilmVideoAttemptByTaskID(task.ID)
	if err != nil {
		return err
	}
	sequenceDetail, err := s.repo.FilmVideoSequenceForUser(task.UserID, task.ProjectID, attempt.SequenceID)
	if err != nil {
		return err
	}
	var slot *model.FilmVideoSlot
	for index := range sequenceDetail.Slots {
		if sequenceDetail.Slots[index].Slot.ID == attempt.SlotID {
			copy := sequenceDetail.Slots[index].Slot
			slot = &copy
			break
		}
	}
	if slot == nil {
		return repository.ErrFilmProductionStateConflict
	}
	now := time.Now().UTC()
	result, command, err := buildFilmVideoCompletion(*task, *attempt, sequenceDetail.Sequence, *slot, resultJSON, now)
	if err != nil {
		return err
	}
	publicInputJSON := publicTaskInputJSON(task.InputJSON)
	taskDelta := int64(len(resultJSON) + len(publicInputJSON) - len(task.ResultJSON) - len(task.InputJSON))
	taskDelta += int64(len(result.URL) + len(result.Payload))
	var session *model.Session
	var message *model.Message
	structuredDelta := int64(0)
	if task.SessionID != "" {
		session, err = s.repo.SessionForUser(task.UserID, task.SessionID)
		if err != nil {
			return err
		}
		session.Status = model.SessionStatusCompleted
		message = &model.Message{
			ID: newID(), UserID: task.UserID, SessionID: task.SessionID, Role: "assistant",
			Content: "短剧镜头视频已生成，等待人工验收。", Payload: string(resultJSON), CreatedAt: now,
		}
		structuredDelta = int64(len(message.Content) + len(message.Payload))
	}
	if err := validateTaskDataGrowthQuotaWithPolicy(usage, taskDelta, policy.Resource); err != nil {
		return err
	}
	if err := validateStructuredStorageQuotaWithPolicy(usage, "session", false, structuredDelta, policy.Resource); err != nil {
		return err
	}
	expectedStatus := task.Status
	completed := *task
	completed.Status = model.TaskStatusSucceeded
	completed.Stage = "等待人工验收"
	completed.Progress = 100
	completed.ResultJSON = string(resultJSON)
	completed.InputJSON = publicInputJSON
	completed.Error = ""
	completed.LeaseOwner = ""
	completed.LeaseExpiresAt = nil
	completed.CompletedAt = &now
	command.Result = &result
	if err := s.repo.SaveFilmVideoTaskCompletion(&completed, expectedStatus, session, message, []model.Result{result}, command); err != nil {
		return err
	}
	*task = completed
	return nil
}

func buildFilmVideoCompletion(task model.Task, attempt model.FilmVideoAttempt, sequence model.FilmVideoSequence, slot model.FilmVideoSlot, raw []byte, at time.Time) (model.Result, repository.FilmVideoCompletionCommand, error) {
	media, err := filmVideoMediaFactFromResult(raw)
	if err != nil {
		return model.Result{}, repository.FilmVideoCompletionCommand{}, err
	}
	if attempt.TaskID != task.ID || attempt.UserID != task.UserID || attempt.ProjectID != task.ProjectID ||
		attempt.SequenceID != sequence.ID || attempt.SlotID != slot.ID || slot.SequenceID != sequence.ID {
		return model.Result{}, repository.FilmVideoCompletionCommand{}, repository.ErrFilmProductionStateConflict
	}
	resultID, resultArtifactID, resultRevisionID := newID(), newID(), newID()
	qcID, qcArtifactID, qcRevisionID := newID(), newID(), newID()
	result := model.Result{
		ID: resultID, UserID: attempt.UserID, TaskID: task.ID, SessionID: task.SessionID,
		AttemptID: attempt.ID, DomainProjectID: attempt.ProjectID, ArtifactID: resultArtifactID,
		ArtifactRevisionID: resultRevisionID, Kind: model.ResultKindFilmVideoGeneration,
		Availability: model.ResultAvailabilityReady, URL: media.URL, Payload: string(raw), CreatedAt: at,
	}
	mediaRef := map[string]any{
		"url": media.URL, "resourceId": filmProductionEmptyStringAsNil(media.ResourceID), "storageKey": filmProductionEmptyStringAsNil(media.StorageKey),
		"mimeType": filmProductionEmptyStringAsNil(media.MimeType), "width": media.Width, "height": media.Height, "durationMs": media.DurationMs,
	}
	resultContent, err := json.Marshal(map[string]any{
		"schemaVersion": 2, "artifactType": "generation-result", "mediaType": "video", "resultId": result.ID,
		"sequenceId": attempt.SequenceID, "slotId": attempt.SlotID, "attemptId": attempt.ID, "taskId": task.ID,
		"resultState": model.ResultAvailabilityReady, "mediaRef": mediaRef,
		"providerRequestId": filmProductionEmptyStringAsNil(task.ProviderRequestID), "requestFingerprint": attempt.RequestFingerprint,
	})
	if err != nil {
		return model.Result{}, repository.FilmVideoCompletionCommand{}, err
	}
	authorityRefs := []map[string]any{{"kind": "runtime", "id": filmVideoWorkerID}}
	resultArtifact := &model.ProductionArtifact{
		ID: resultArtifactID, UserID: attempt.UserID, ProjectID: attempt.ProjectID, Domain: "film",
		ArtifactType: "generation-result", LogicalKey: "film-video:result:" + attempt.ID,
	}
	resultRevision := &model.ProductionArtifactRevision{
		ID: resultRevisionID, Status: model.ProductionArtifactStatusLocked, ContentJSON: string(resultContent), ContentDigest: digestBytesHex(resultContent),
		SourceRunID: attempt.RootRunID, SourceAttemptID: attempt.ID,
		SourceArtifactRefsJSON: mustFilmJSON([]map[string]any{
			{"artifactId": attempt.AttemptArtifactID, "revisionId": attempt.AttemptRevisionID, "type": "generation-attempt"},
			{"artifactId": attempt.PromptArtifactID, "revisionId": attempt.PromptRevisionID, "type": "ai-video-prompts", "digest": attempt.PromptDigest},
		}),
		AuthorityRefsJSON: mustFilmJSON(authorityRefs), CreatedByType: "runtime", CreatedByID: filmVideoWorkerID, CreatedAt: at,
	}
	qcDecision, issueCodes, qcEvidence, qcNote, hardFailure := evaluateFilmVideoTechnicalQC(media, sequence, slot)
	qcReport := &model.FilmVideoQCReport{
		ID: qcID, UserID: attempt.UserID, IdempotencyKey: "film-video:system-qc:" + attempt.ID,
		ProjectID: attempt.ProjectID, RootRunID: attempt.RootRunID, SequenceID: attempt.SequenceID, SlotID: attempt.SlotID,
		AttemptID: attempt.ID, ResultID: result.ID, Decision: qcDecision, Action: model.FilmProductionQCActionHold,
		IssueCodesJSON: mustFilmJSON(issueCodes), EvidenceJSON: mustFilmJSON(qcEvidence), Note: qcNote, Source: "system",
		AssessmentKind: "technical_media_qc", MediaState: model.FilmContinuityMediaStateAvailable,
		ArtifactID: qcArtifactID, RevisionID: qcRevisionID, CreatedAt: at,
	}
	var rework *model.FilmReworkEvent
	var reworkArtifact *model.ProductionArtifact
	var reworkRevision *model.ProductionArtifactRevision
	if hardFailure != "" {
		rework, reworkArtifact, reworkRevision, err = buildFilmVideoTechnicalRework(attempt, slot, result, qcReport, hardFailure, at)
		if err != nil {
			return model.Result{}, repository.FilmVideoCompletionCommand{}, err
		}
		qcReport.ReworkEventID = rework.ID
	}
	qcContent, err := json.Marshal(map[string]any{
		"schemaVersion": 3, "artifactType": "media-qc-report", "mediaType": "video", "reportId": qcReport.ID,
		"sequenceId": attempt.SequenceID, "slotId": attempt.SlotID, "attemptId": attempt.ID, "resultId": result.ID,
		"decision": qcReport.Decision, "action": qcReport.Action, "issueCodes": issueCodes, "evidence": qcEvidence,
		"note": qcNote, "source": "system", "assessmentKind": qcReport.AssessmentKind, "mediaState": qcReport.MediaState,
		"reworkEventId": filmProductionEmptyStringAsNil(qcReport.ReworkEventID),
	})
	if err != nil {
		return model.Result{}, repository.FilmVideoCompletionCommand{}, err
	}
	qcArtifact := &model.ProductionArtifact{
		ID: qcArtifactID, UserID: attempt.UserID, ProjectID: attempt.ProjectID, Domain: "film",
		ArtifactType: "media-qc-report", LogicalKey: "film-video:system-qc:" + attempt.ID,
	}
	qcRevision := &model.ProductionArtifactRevision{
		ID: qcRevisionID, Status: model.ProductionArtifactStatusLocked, ContentJSON: string(qcContent), ContentDigest: digestBytesHex(qcContent),
		SourceRunID: attempt.RootRunID, SourceAttemptID: attempt.ID,
		SourceArtifactRefsJSON: mustFilmJSON([]map[string]any{
			{"artifactId": resultArtifact.ID, "revisionId": resultRevision.ID, "type": "generation-result"},
			{"artifactId": attempt.AttemptArtifactID, "revisionId": attempt.AttemptRevisionID, "type": "generation-attempt"},
		}),
		AuthorityRefsJSON: mustFilmJSON(authorityRefs), CreatedByType: "runtime", CreatedByID: filmVideoWorkerID, CreatedAt: at,
	}
	command := repository.FilmVideoCompletionCommand{
		AttemptID: attempt.ID, Result: &result, ResultArtifact: resultArtifact, ResultRevision: resultRevision,
		QCReport: qcReport, QCArtifact: qcArtifact, QCRevision: qcRevision,
		ReworkEvent: rework, ReworkArtifact: reworkArtifact, ReworkRevision: reworkRevision, At: at,
		Event: repository.AgentRuntimeEventInput{
			ID: newID(), EventType: "film.production.video.generated", ActorType: "runtime", ActorID: filmVideoWorkerID,
			PayloadJSON: mustFilmJSON(map[string]any{
				"attemptId": attempt.ID, "resultId": result.ID, "sequenceId": attempt.SequenceID, "slotId": attempt.SlotID,
				"resultArtifactId": resultArtifact.ID, "qcDecision": qcReport.Decision,
			}),
		},
	}
	return result, command, nil
}

func evaluateFilmVideoTechnicalQC(media filmProductionMediaFact, sequence model.FilmVideoSequence, slot model.FilmVideoSlot) (model.FilmProductionQCDecision, []string, map[string]any, string, string) {
	issueCodes := []string{"VISUAL_SEMANTIC_NOT_ASSESSED"}
	checks := []map[string]any{
		{"check": "media_access", "status": "PASS"},
		{"check": "mime_type", "status": "PASS", "actual": media.MimeType},
	}
	hardFailure := ""
	expectedRatio, ratioOK := parseFilmAspectRatio(sequence.AspectRatio)
	if media.Width > 0 && media.Height > 0 && ratioOK {
		actualRatio := float64(media.Width) / float64(media.Height)
		delta := math.Abs(actualRatio-expectedRatio) / expectedRatio
		status := "PASS"
		if delta > 0.03 {
			status = "FAIL"
			hardFailure = "VIDEO_ASPECT_RATIO_MISMATCH"
			issueCodes = append(issueCodes, hardFailure)
		}
		checks = append(checks, map[string]any{
			"check": "aspect_ratio", "status": status, "expected": sequence.AspectRatio,
			"actualWidth": media.Width, "actualHeight": media.Height, "relativeDelta": delta,
		})
	} else {
		issueCodes = append(issueCodes, "VIDEO_DIMENSIONS_UNKNOWN")
		checks = append(checks, map[string]any{"check": "aspect_ratio", "status": "NOT_ASSESSABLE", "expected": sequence.AspectRatio})
	}
	if media.DurationMs > 0 {
		delta := media.DurationMs - slot.DurationMs
		if delta < 0 {
			delta = -delta
		}
		tolerance := int64(500)
		if relative := slot.DurationMs / 5; relative > tolerance {
			tolerance = relative
		}
		status := "PASS"
		if delta > tolerance {
			status = "FAIL"
			if hardFailure == "" {
				hardFailure = "VIDEO_DURATION_MISMATCH"
			}
			issueCodes = append(issueCodes, "VIDEO_DURATION_MISMATCH")
		}
		checks = append(checks, map[string]any{
			"check": "duration", "status": status, "expectedMs": slot.DurationMs,
			"actualMs": media.DurationMs, "toleranceMs": tolerance,
		})
	} else {
		issueCodes = append(issueCodes, "VIDEO_DURATION_UNKNOWN")
		checks = append(checks, map[string]any{"check": "duration", "status": "NOT_ASSESSABLE", "expectedMs": slot.DurationMs})
	}
	evidence := map[string]any{
		"mediaAvailable": true, "mediaState": model.FilmContinuityMediaStateAvailable,
		"technicalChecks":      checks,
		"unassessedDimensions": []string{"narrative", "performance", "identity", "visual_continuity", "physics", "lip_sync", "audio_quality"},
		"contract":             map[string]any{"sequenceId": sequence.ID, "slotId": slot.ID, "shotId": slot.ShotID},
	}
	if hardFailure != "" {
		return model.FilmProductionQCDecisionFail, uniqueFilmIssueCodes(issueCodes), evidence,
			"真实视频媒体已完成技术检查并发现阻断项；已创建最小槽位返工事件。语义与视觉质量仍需真实媒体复核。", hardFailure
	}
	return model.FilmProductionQCDecisionNotAssessable, uniqueFilmIssueCodes(issueCodes), evidence,
		"真实视频媒体已通过可访问性与可判定技术规格检查；人物、动作、场景连续性、口型和音画质量仍需人工验收。", ""
}

func parseFilmAspectRatio(value string) (float64, bool) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, false
	}
	width, widthErr := strconv.ParseFloat(parts[0], 64)
	height, heightErr := strconv.ParseFloat(parts[1], 64)
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return 0, false
	}
	return width / height, true
}

func uniqueFilmIssueCodes(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func buildFilmVideoTechnicalRework(attempt model.FilmVideoAttempt, slot model.FilmVideoSlot, result model.Result, report *model.FilmVideoQCReport, reasonCode string, at time.Time) (*model.FilmReworkEvent, *model.ProductionArtifact, *model.ProductionArtifactRevision, error) {
	if report == nil || reasonCode == "" {
		return nil, nil, nil, repository.ErrFilmProductionStateConflict
	}
	eventID, artifactID, revisionID := newID(), newID(), newID()
	repairScope := map[string]any{
		"kind": "video_slot", "sequenceId": attempt.SequenceID, "slotId": attempt.SlotID, "shotId": slot.ShotID,
		"retryOfAttemptId": attempt.ID, "preserveAcceptedSlots": true,
	}
	recheckGate := map[string]any{
		"required":   []string{"new_paid_attempt", "technical_media_qc", "human_media_review"},
		"reasonCode": reasonCode,
	}
	event := &model.FilmReworkEvent{
		ID: eventID, UserID: attempt.UserID, ProjectID: attempt.ProjectID, RootRunID: attempt.RootRunID,
		SequenceID: attempt.SequenceID, SlotID: attempt.SlotID, ShotID: slot.ShotID, AttemptID: attempt.ID,
		ResultID: result.ID, QCReportID: report.ID, MediaType: "video", Source: "system", Severity: "blocking",
		ReasonCode: reasonCode, Owner: "film_project_lead", RepairScopeJSON: mustFilmJSON(repairScope),
		RecheckGateJSON: mustFilmJSON(recheckGate), Status: model.FilmReworkStatusOpen,
		ArtifactID: artifactID, ArtifactRevisionID: revisionID, CreatedAt: at,
	}
	content, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "artifactType": "rework-event", "reworkEventId": event.ID,
		"mediaType": event.MediaType, "source": event.Source, "severity": event.Severity,
		"reasonCode": event.ReasonCode, "owner": event.Owner, "repairScope": repairScope,
		"recheckGate": recheckGate, "status": event.Status, "qcReportId": report.ID,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	artifact := &model.ProductionArtifact{
		ID: artifactID, UserID: attempt.UserID, ProjectID: attempt.ProjectID, Domain: "film",
		ArtifactType: "rework-event", LogicalKey: "film-video:rework:" + event.ID,
	}
	revision := &model.ProductionArtifactRevision{
		ID: revisionID, Status: model.ProductionArtifactStatusLocked, ContentJSON: string(content), ContentDigest: digestBytesHex(content),
		SourceRunID: attempt.RootRunID, SourceAttemptID: attempt.ID,
		SourceArtifactRefsJSON: mustFilmJSON([]map[string]any{
			{"artifactId": result.ArtifactID, "revisionId": result.ArtifactRevisionID, "type": "generation-result"},
			{"artifactId": report.ArtifactID, "revisionId": report.RevisionID, "type": "media-qc-report"},
		}),
		AuthorityRefsJSON: mustFilmJSON([]map[string]any{{"kind": "skill", "id": "final-film-quality-review", "version": "3.0.0"}}),
		CreatedByType:     "runtime", CreatedByID: filmVideoWorkerID, CreatedAt: at,
	}
	return event, artifact, revision, nil
}

func filmVideoMediaFactFromResult(raw []byte) (filmProductionMediaFact, error) {
	var payload filmVideoGeneratedPayload
	if len(raw) == 0 || json.Unmarshal(raw, &payload) != nil || payload.Mode != "video" {
		return filmProductionMediaFact{}, repository.ErrFilmProductionMediaMissing
	}
	video := payload.Video
	mediaURL := strings.TrimSpace(firstNonEmpty(video.URL, video.DataURL))
	if !filmProductionAccessibleResultURL(mediaURL) {
		return filmProductionMediaFact{}, repository.ErrFilmProductionMediaMissing
	}
	resourceID := strings.TrimSpace(video.ResourceID)
	storageKey := strings.TrimSpace(video.StorageKey)
	if strings.HasPrefix(storageKey, "resource:") {
		storageResourceID := strings.TrimPrefix(storageKey, "resource:")
		if resourceID != "" && resourceID != storageResourceID {
			return filmProductionMediaFact{}, repository.ErrFilmProductionMediaMissing
		}
		resourceID = storageResourceID
	}
	if resourceID == "" || strings.TrimSpace(video.MimeType) == "" || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(video.MimeType)), "video/") {
		return filmProductionMediaFact{}, repository.ErrFilmProductionMediaMissing
	}
	if video.Width < 0 || video.Height < 0 || video.DurationMs < 0 {
		return filmProductionMediaFact{}, repository.ErrFilmProductionMediaMissing
	}
	return filmProductionMediaFact{
		URL: mediaURL, ResourceID: resourceID, StorageKey: storageKey, MimeType: strings.TrimSpace(video.MimeType),
		Width: video.Width, Height: video.Height, DurationMs: video.DurationMs,
	}, nil
}
