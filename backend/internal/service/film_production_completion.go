package service

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

const filmProductionWorkerID = "film-production-worker-v1"

type filmProductionGeneratedImage struct {
	URL        string `json:"url"`
	DataURL    string `json:"dataUrl"`
	ResourceID string `json:"resourceId"`
	StorageKey string `json:"storageKey"`
	MimeType   string `json:"mimeType"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	DurationMs int64  `json:"durationMs"`
}

type filmProductionGeneratedPayload struct {
	Mode   string                         `json:"mode"`
	Images []filmProductionGeneratedImage `json:"images"`
}

type filmProductionMediaFact struct {
	URL        string
	ResourceID string
	StorageKey string
	MimeType   string
	Width      int
	Height     int
	DurationMs int64
}

func (s *Service) saveFilmProductionTaskCompletionWithinStorageQuota(task *model.Task, resultJSON []byte) error {
	if task == nil || !model.IsFilmProductionTaskType(task.Type) {
		return errors.New("Film Production completion requires a Film production task")
	}
	if task.Type == model.FilmProductionTaskTypeVideo {
		return s.saveFilmVideoTaskCompletionWithinStorageQuota(task, resultJSON)
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
	attempt, err := s.repo.FilmProductionAttemptByTaskID(task.ID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	result, command, err := buildFilmProductionCompletion(*task, *attempt, resultJSON, now)
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
			Content: "短剧镜头图片已生成，等待人工验收。", Payload: string(resultJSON), CreatedAt: now,
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
	if err := s.repo.SaveFilmProductionTaskCompletion(&completed, expectedStatus, session, message, []model.Result{result}, command); err != nil {
		return err
	}
	*task = completed
	return nil
}

func buildFilmProductionCompletion(task model.Task, attempt model.FilmProductionAttempt, raw []byte, at time.Time) (model.Result, repository.FilmProductionCompletionCommand, error) {
	media, err := filmProductionMediaFactFromResult(raw)
	if err != nil {
		return model.Result{}, repository.FilmProductionCompletionCommand{}, err
	}
	if attempt.TaskID != task.ID || attempt.UserID != task.UserID || attempt.ProjectID != task.ProjectID {
		return model.Result{}, repository.FilmProductionCompletionCommand{}, repository.ErrFilmProductionStateConflict
	}

	resultID := newID()
	resultArtifactID := newID()
	resultRevisionID := newID()
	qcID := newID()
	qcArtifactID := newID()
	qcRevisionID := newID()

	result := model.Result{
		ID: resultID, UserID: attempt.UserID, TaskID: task.ID, SessionID: task.SessionID,
		AttemptID: attempt.ID, DomainProjectID: attempt.ProjectID, ArtifactID: resultArtifactID,
		ArtifactRevisionID: resultRevisionID, Kind: model.ResultKindFilmGeneration,
		Availability: model.ResultAvailabilityReady, URL: media.URL, Payload: string(raw), CreatedAt: at,
	}
	mediaRef := map[string]any{
		"url": media.URL, "resourceId": filmProductionEmptyStringAsNil(media.ResourceID), "storageKey": filmProductionEmptyStringAsNil(media.StorageKey),
		"mimeType": filmProductionEmptyStringAsNil(media.MimeType), "width": media.Width, "height": media.Height,
	}
	resultContent, err := json.Marshal(map[string]any{
		"schemaVersion": 2, "artifactType": "generation-result", "resultId": result.ID,
		"attemptId": attempt.ID, "taskId": task.ID, "shotId": attempt.ShotID,
		"resultState": model.ResultAvailabilityReady, "mediaRef": mediaRef,
		"providerRequestId": filmProductionEmptyStringAsNil(task.ProviderRequestID), "requestFingerprint": attempt.RequestFingerprint,
	})
	if err != nil {
		return model.Result{}, repository.FilmProductionCompletionCommand{}, err
	}
	authorityRefs := []map[string]any{
		{"kind": "registry", "id": attempt.RegistryID, "version": attempt.RegistryVersion, "digest": attempt.RegistryDigest},
		{"kind": "runtime", "id": filmProductionWorkerID},
	}
	resultArtifact := &model.ProductionArtifact{
		ID: resultArtifactID, UserID: attempt.UserID, ProjectID: attempt.ProjectID, Domain: "film",
		ArtifactType: "generation-result", LogicalKey: "film-production:result:" + attempt.ID,
	}
	resultRevision := &model.ProductionArtifactRevision{
		ID: resultRevisionID, Status: model.ProductionArtifactStatusLocked,
		ContentJSON: string(resultContent), ContentDigest: digestBytesHex(resultContent),
		SourceRunID: attempt.RootRunID, SourceAttemptID: attempt.ID,
		SourceArtifactRefsJSON: mustFilmJSON([]map[string]any{
			{"artifactId": attempt.AttemptArtifactID, "revisionId": attempt.AttemptRevisionID, "type": "generation-attempt"},
			{"artifactId": attempt.PromptArtifactID, "revisionId": attempt.PromptRevisionID, "type": "prompt", "digest": attempt.PromptDigest},
		}),
		AuthorityRefsJSON: mustFilmJSON(authorityRefs), CreatedByType: "runtime", CreatedByID: filmProductionWorkerID, CreatedAt: at,
	}

	imageIssueCodes := []string{"VISUAL_SEMANTIC_NOT_ASSESSED"}
	imageTechnicalChecks := []map[string]any{
		{"check": "media_access", "status": "PASS"},
		{"check": "mime_type", "status": "PASS", "actual": media.MimeType},
	}
	if media.Width > 0 && media.Height > 0 {
		imageTechnicalChecks = append(imageTechnicalChecks, map[string]any{"check": "dimensions", "status": "PASS", "width": media.Width, "height": media.Height})
	} else {
		imageIssueCodes = append(imageIssueCodes, "IMAGE_DIMENSIONS_UNKNOWN")
		imageTechnicalChecks = append(imageTechnicalChecks, map[string]any{"check": "dimensions", "status": "NOT_ASSESSABLE"})
	}
	imageQCEvidence := map[string]any{
		"mediaAvailable": true, "mediaState": model.FilmContinuityMediaStateAvailable, "technicalChecks": imageTechnicalChecks,
		"unassessedDimensions": []string{"identity", "visual_continuity", "anatomy", "props", "scene", "composition"},
	}
	issueCodesJSON := mustFilmJSON(imageIssueCodes)
	evidenceJSON := mustFilmJSON(imageQCEvidence)
	qcNote := "真实图片媒体已通过可访问性与可判定技术规格检查；人物身份、人体、道具、场景和视觉连续性仍需人工验收。"
	qcReport := &model.FilmProductionQCReport{
		ID: qcID, UserID: attempt.UserID, ProjectID: attempt.ProjectID, RootRunID: attempt.RootRunID, ShotID: attempt.ShotID,
		AttemptID: attempt.ID, ResultID: result.ID, Decision: model.FilmProductionQCDecisionNotAssessable,
		Action: model.FilmProductionQCActionHold, IssueCodesJSON: issueCodesJSON, EvidenceJSON: evidenceJSON,
		Note: qcNote, Source: "system", AssessmentKind: "technical_media_qc", IdempotencyKey: "film-production:system-qc:" + attempt.ID,
		ArtifactID: qcArtifactID, RevisionID: qcRevisionID, CreatedAt: at,
	}
	qcContent, err := json.Marshal(map[string]any{
		"schemaVersion": 3, "artifactType": "generation-qc-report", "reportId": qcReport.ID,
		"attemptId": attempt.ID, "resultId": result.ID, "shotId": attempt.ShotID,
		"decision": qcReport.Decision, "action": qcReport.Action, "issueCodes": imageIssueCodes,
		"evidence": imageQCEvidence,
		"note":     qcNote, "source": "system", "assessmentKind": qcReport.AssessmentKind,
	})
	if err != nil {
		return model.Result{}, repository.FilmProductionCompletionCommand{}, err
	}
	qcArtifact := &model.ProductionArtifact{
		ID: qcArtifactID, UserID: attempt.UserID, ProjectID: attempt.ProjectID, Domain: "film",
		ArtifactType: "generation-qc-report", LogicalKey: "film-production:system-qc:" + attempt.ID,
	}
	qcRevision := &model.ProductionArtifactRevision{
		ID: qcRevisionID, Status: model.ProductionArtifactStatusLocked,
		ContentJSON: string(qcContent), ContentDigest: digestBytesHex(qcContent),
		SourceRunID: attempt.RootRunID, SourceAttemptID: attempt.ID,
		SourceArtifactRefsJSON: mustFilmJSON([]map[string]any{
			{"artifactId": resultArtifact.ID, "revisionId": resultRevision.ID, "type": "generation-result"},
			{"artifactId": attempt.AttemptArtifactID, "revisionId": attempt.AttemptRevisionID, "type": "generation-attempt"},
		}),
		AuthorityRefsJSON: mustFilmJSON(authorityRefs), CreatedByType: "runtime", CreatedByID: filmProductionWorkerID, CreatedAt: at,
	}
	command := repository.FilmProductionCompletionCommand{
		AttemptID: attempt.ID, Result: &result, ResultArtifact: resultArtifact, ResultRevision: resultRevision,
		QCReport: qcReport, QCArtifact: qcArtifact, QCRevision: qcRevision, At: at,
		Event: repository.AgentRuntimeEventInput{
			ID: newID(), EventType: "film.production.image.generated", ActorType: "runtime", ActorID: filmProductionWorkerID,
			PayloadJSON: mustFilmJSON(map[string]any{
				"attemptId": attempt.ID, "resultId": result.ID, "shotId": attempt.ShotID,
				"resultArtifactId": resultArtifact.ID, "qcDecision": qcReport.Decision,
			}),
		},
	}
	return result, command, nil
}

func filmProductionMediaFactFromResult(raw []byte) (filmProductionMediaFact, error) {
	var payload filmProductionGeneratedPayload
	if len(raw) == 0 || json.Unmarshal(raw, &payload) != nil || payload.Mode != "image" || len(payload.Images) != 1 {
		return filmProductionMediaFact{}, repository.ErrFilmProductionMediaMissing
	}
	image := payload.Images[0]
	mediaURL := strings.TrimSpace(firstNonEmpty(image.URL, image.DataURL))
	if !filmProductionAccessibleResultURL(mediaURL) {
		return filmProductionMediaFact{}, repository.ErrFilmProductionMediaMissing
	}
	resourceID := strings.TrimSpace(image.ResourceID)
	storageKey := strings.TrimSpace(image.StorageKey)
	if strings.HasPrefix(storageKey, "resource:") {
		storageResourceID := strings.TrimPrefix(storageKey, "resource:")
		if resourceID != "" && resourceID != storageResourceID {
			return filmProductionMediaFact{}, repository.ErrFilmProductionMediaMissing
		}
		resourceID = storageResourceID
	}
	if strings.TrimSpace(image.MimeType) != "" && !strings.HasPrefix(strings.ToLower(strings.TrimSpace(image.MimeType)), "image/") {
		return filmProductionMediaFact{}, repository.ErrFilmProductionMediaMissing
	}
	if image.Width < 0 || image.Height < 0 {
		return filmProductionMediaFact{}, repository.ErrFilmProductionMediaMissing
	}
	return filmProductionMediaFact{
		URL: mediaURL, ResourceID: resourceID, StorageKey: storageKey,
		MimeType: strings.TrimSpace(image.MimeType), Width: image.Width, Height: image.Height,
	}, nil
}

func filmProductionAccessibleResultURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	if parsed.IsAbs() {
		return (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	return len(parts) == 4 && parts[0] == "api" && parts[1] == "resources" && parts[2] != "" && parts[3] == "file"
}

func filmProductionEmptyStringAsNil(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func (s *Service) updateTaskTerminalState(task *model.Task, expected model.TaskStatus, filmStatus model.FilmProductionAttemptStatus) (bool, error) {
	if model.IsFilmManagedTaskType(task.Type) {
		return s.repo.UpdateTaskTerminalStateWithFilmAttempt(
			task.ID, task.Type, expected, task.Status, filmStatus, task.Stage, task.Error, *task.CompletedAt,
		)
	}
	return s.repo.UpdateTaskTerminalState(task.ID, expected, task.Status, task.Stage, task.Error, *task.CompletedAt)
}
