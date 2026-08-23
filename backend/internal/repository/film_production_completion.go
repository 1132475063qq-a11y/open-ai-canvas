package repository

import (
	"errors"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) SaveFilmProductionTaskCompletion(task *model.Task, expected model.TaskStatus, session *model.Session, message *model.Message, results []model.Result, command FilmProductionCompletionCommand) error {
	if err := validateFilmProductionCompletionCommand(task, results, command); err != nil {
		return err
	}
	now := runtimeCommandTime(command.At)
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := saveTaskCompletionTx(tx, task, expected, session, message, results); err != nil {
			return err
		}
		var attempt model.FilmProductionAttempt
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&attempt, "id = ? AND task_id = ?", command.AttemptID, task.ID).Error; err != nil {
			return err
		}
		if attempt.Status != model.FilmProductionAttemptStatusRunning && attempt.Status != model.FilmProductionAttemptStatusQueued {
			return ErrFilmProductionStateConflict
		}
		if err := validateFilmProductionCompletionScope(task, attempt, command); err != nil {
			return err
		}
		if command.Result.AttemptID != attempt.ID || command.Result.TaskID != task.ID || command.Result.DomainProjectID != attempt.ProjectID ||
			command.Result.ArtifactID != command.ResultArtifact.ID || command.Result.ArtifactRevisionID != command.ResultRevision.ID {
			return ErrFilmProductionStateConflict
		}
		if !filmProductionAccessibleMediaURL(command.Result.URL) || command.Result.Availability != model.ResultAvailabilityReady {
			return ErrFilmProductionMediaMissing
		}
		if command.QCReport.AttemptID != attempt.ID || command.QCReport.ResultID != command.Result.ID ||
			command.QCReport.Decision != model.FilmProductionQCDecisionNotAssessable || command.QCReport.Action != model.FilmProductionQCActionHold ||
			command.QCReport.ArtifactID != command.QCArtifact.ID || command.QCReport.RevisionID != command.QCRevision.ID {
			return ErrFilmProductionStateConflict
		}
		if _, _, err := createProductionArtifactRevisionTx(tx, ProductionArtifactRevisionCreate{
			UserID: attempt.UserID, Artifact: command.ResultArtifact, Revision: command.ResultRevision, ExpectedSequence: 0, At: now,
		}); err != nil {
			return err
		}
		if _, _, err := createProductionArtifactRevisionTx(tx, ProductionArtifactRevisionCreate{
			UserID: attempt.UserID, Artifact: command.QCArtifact, Revision: command.QCRevision, ExpectedSequence: 0, At: now,
		}); err != nil {
			return err
		}
		if err := tx.Create(command.QCReport).Error; err != nil {
			return err
		}
		updated := tx.Model(&model.FilmProductionAttempt{}).
			Where("id = ? AND task_id = ? AND status IN ?", attempt.ID, task.ID, []model.FilmProductionAttemptStatus{
				model.FilmProductionAttemptStatusQueued, model.FilmProductionAttemptStatusRunning,
			}).Updates(map[string]any{
			"status": model.FilmProductionAttemptStatusSucceeded, "result_id": command.Result.ID,
			"result_artifact_id": command.ResultArtifact.ID, "result_revision_id": command.ResultRevision.ID,
			"error": "", "completed_at": now, "updated_at": now,
		})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrFilmProductionStateConflict
		}
		var root model.AgentRuntimeRun
		if err := lockAgentRuntimeRun(tx, attempt.UserID, attempt.RootRunID, &root); err != nil {
			return err
		}
		return appendFilmProductionEventTx(tx, root, command.Event, attempt.ID, now)
	})
}

func (r *Repository) CreateFilmProductionHumanQC(command FilmProductionHumanQCCommand) (*model.FilmProductionQCReport, bool, error) {
	if err := validateFilmProductionHumanQCCommand(command); err != nil {
		return nil, false, err
	}
	now := runtimeCommandTime(command.At)
	var result model.FilmProductionQCReport
	idempotent := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var existing model.FilmProductionQCReport
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&existing, "user_id = ? AND idempotency_key = ?", command.UserID, command.Report.IdempotencyKey).Error
		if err == nil {
			if existing.ProjectID != command.ProjectID || existing.AttemptID != command.AttemptID ||
				existing.ResultID != command.Report.ResultID || existing.Decision != command.Report.Decision ||
				existing.Action != command.Report.Action || existing.IssueCodesJSON != command.Report.IssueCodesJSON ||
				existing.EvidenceJSON != command.Report.EvidenceJSON || existing.Note != command.Report.Note {
				return ErrFilmProductionStateConflict
			}
			result = existing
			idempotent = true
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var project model.Project
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&project, "id = ? AND user_id = ?", command.ProjectID, command.UserID).Error; err != nil {
			return err
		}
		if project.Type != "short-drama" || project.Status != model.ProjectStatusActive {
			return ErrFilmProductionStateConflict
		}
		var attempt model.FilmProductionAttempt
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&attempt, "id = ? AND user_id = ? AND project_id = ?", command.AttemptID, command.UserID, command.ProjectID).Error; err != nil {
			return err
		}
		if attempt.Status != model.FilmProductionAttemptStatusSucceeded || attempt.ResultID == "" {
			return ErrFilmProductionStateConflict
		}
		var generationResult model.Result
		if err := tx.First(&generationResult, "id = ? AND attempt_id = ? AND availability = ?", attempt.ResultID, attempt.ID, model.ResultAvailabilityReady).Error; err != nil {
			return ErrFilmProductionMediaMissing
		}
		report := *command.Report
		if report.AttemptID != attempt.ID || report.ResultID != generationResult.ID || report.RootRunID != attempt.RootRunID || report.ShotID != attempt.ShotID ||
			report.ArtifactID != command.Artifact.ID || report.RevisionID != command.Revision.ID {
			return ErrFilmProductionStateConflict
		}
		if _, _, err := createProductionArtifactRevisionTx(tx, ProductionArtifactRevisionCreate{
			UserID: command.UserID, Artifact: command.Artifact, Revision: command.Revision, ExpectedSequence: 0, At: now,
		}); err != nil {
			return err
		}
		report.CreatedAt = now
		if err := tx.Create(&report).Error; err != nil {
			return err
		}
		var root model.AgentRuntimeRun
		if err := lockAgentRuntimeRun(tx, command.UserID, attempt.RootRunID, &root); err != nil {
			return err
		}
		if err := appendFilmProductionEventTx(tx, root, command.Event, attempt.ID, now); err != nil {
			return err
		}
		result = report
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return &result, idempotent, nil
}

func markFilmProductionAttemptRunningTx(tx *gorm.DB, taskID string, at time.Time) error {
	var attempt model.FilmProductionAttempt
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&attempt, "task_id = ?", taskID).Error
	if err != nil {
		return err
	}
	if attempt.Status == model.FilmProductionAttemptStatusRunning {
		return nil
	}
	if attempt.Status != model.FilmProductionAttemptStatusQueued {
		return ErrFilmProductionStateConflict
	}
	updated := tx.Model(&model.FilmProductionAttempt{}).
		Where("id = ? AND status = ?", attempt.ID, model.FilmProductionAttemptStatusQueued).
		Updates(map[string]any{
			"status":     model.FilmProductionAttemptStatusRunning,
			"started_at": gorm.Expr("COALESCE(started_at, ?)", at), "updated_at": at,
		})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return ErrFilmProductionStateConflict
	}
	return nil
}

func markFilmProductionAttemptTerminalTx(tx *gorm.DB, taskID string, status model.FilmProductionAttemptStatus, errorText string, at time.Time) error {
	if status != model.FilmProductionAttemptStatusFailed && status != model.FilmProductionAttemptStatusCancelled && status != model.FilmProductionAttemptStatusUncertain {
		return errors.New("unsupported Film Production terminal status")
	}
	updated := tx.Model(&model.FilmProductionAttempt{}).
		Where("task_id = ? AND status IN ?", taskID, []model.FilmProductionAttemptStatus{
			model.FilmProductionAttemptStatusQueued, model.FilmProductionAttemptStatusRunning,
		}).Updates(map[string]any{
		"status": status, "error": strings.TrimSpace(errorText), "completed_at": at, "updated_at": at,
	})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return ErrFilmProductionStateConflict
	}
	return nil
}

func saveTaskCompletionTx(tx *gorm.DB, task *model.Task, expected model.TaskStatus, session *model.Session, message *model.Message, results []model.Result) error {
	updated := tx.Model(&model.Task{}).
		Where("id = ? AND status = ?", task.ID, expected).
		Select("*").Omit("id", "created_at").Updates(task)
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return ErrTaskStateConflict
	}
	if session != nil {
		if err := tx.Save(session).Error; err != nil {
			return err
		}
	}
	if message != nil {
		if err := tx.Create(message).Error; err != nil {
			return err
		}
	}
	for index := range results {
		if err := tx.Create(&results[index]).Error; err != nil {
			return err
		}
	}
	return nil
}

func validateFilmProductionCompletionCommand(task *model.Task, results []model.Result, command FilmProductionCompletionCommand) error {
	if task == nil || strings.TrimSpace(command.AttemptID) == "" || command.Result == nil || command.ResultArtifact == nil || command.ResultRevision == nil ||
		command.QCReport == nil || command.QCArtifact == nil || command.QCRevision == nil || len(results) == 0 {
		return errors.New("Film Production completion identity is incomplete")
	}
	if err := validateAgentRuntimeEventInput(command.Event); err != nil {
		return err
	}
	found := false
	for index := range results {
		if results[index].ID == command.Result.ID {
			found = true
			break
		}
	}
	if !found {
		return errors.New("Film Production completion Result is missing from Task completion")
	}
	return nil
}

func validateFilmProductionCompletionScope(task *model.Task, attempt model.FilmProductionAttempt, command FilmProductionCompletionCommand) error {
	result := command.Result
	resultArtifact := command.ResultArtifact
	resultRevision := command.ResultRevision
	qc := command.QCReport
	qcArtifact := command.QCArtifact
	qcRevision := command.QCRevision
	if task.Type != model.FilmProductionTaskTypeImage || task.UserID != attempt.UserID || task.ProjectID != attempt.ProjectID ||
		result.UserID != attempt.UserID || result.Kind != model.ResultKindFilmGeneration || result.Availability != model.ResultAvailabilityReady ||
		strings.TrimSpace(result.Payload) == "" || result.ArtifactID != resultArtifact.ID || result.ArtifactRevisionID != resultRevision.ID ||
		resultArtifact.UserID != attempt.UserID || resultArtifact.ProjectID != attempt.ProjectID || resultArtifact.Domain != "film" ||
		resultArtifact.ArtifactType != "generation-result" || resultArtifact.LogicalKey != "film-production:result:"+attempt.ID ||
		resultRevision.Status != model.ProductionArtifactStatusLocked || resultRevision.SourceRunID != attempt.RootRunID ||
		resultRevision.SourceAttemptID != attempt.ID || strings.TrimSpace(resultRevision.ContentDigest) == "" ||
		qc.UserID != attempt.UserID || qc.ProjectID != attempt.ProjectID || qc.RootRunID != attempt.RootRunID || qc.ShotID != attempt.ShotID ||
		qc.Source != "system" || strings.TrimSpace(qc.IdempotencyKey) == "" || qc.ArtifactID != qcArtifact.ID || qc.RevisionID != qcRevision.ID ||
		qcArtifact.UserID != attempt.UserID || qcArtifact.ProjectID != attempt.ProjectID || qcArtifact.Domain != "film" ||
		qcArtifact.ArtifactType != "generation-qc-report" || qcArtifact.LogicalKey != "film-production:system-qc:"+attempt.ID ||
		qcRevision.Status != model.ProductionArtifactStatusLocked || qcRevision.SourceRunID != attempt.RootRunID ||
		qcRevision.SourceAttemptID != attempt.ID || strings.TrimSpace(qcRevision.ContentDigest) == "" {
		return ErrFilmProductionStateConflict
	}
	return nil
}

func validateFilmProductionHumanQCCommand(command FilmProductionHumanQCCommand) error {
	if strings.TrimSpace(command.UserID) == "" || strings.TrimSpace(command.ProjectID) == "" || strings.TrimSpace(command.AttemptID) == "" ||
		command.Report == nil || command.Artifact == nil || command.Revision == nil || strings.TrimSpace(command.Report.IdempotencyKey) == "" ||
		command.Report.Source != "human" || command.Report.UserID != command.UserID || command.Report.ProjectID != command.ProjectID ||
		command.Report.ReviewerUserID != command.UserID {
		return errors.New("Film Production human QC identity is incomplete")
	}
	if err := validateAgentRuntimeEventInput(command.Event); err != nil {
		return err
	}
	switch command.Report.Decision {
	case model.FilmProductionQCDecisionPass:
		if command.Report.Action != model.FilmProductionQCActionAccept {
			return ErrFilmProductionStateConflict
		}
	case model.FilmProductionQCDecisionFail:
		if command.Report.Action != model.FilmProductionQCActionRetry {
			return ErrFilmProductionStateConflict
		}
	case model.FilmProductionQCDecisionUncertain:
		if command.Report.Action != model.FilmProductionQCActionHold {
			return ErrFilmProductionStateConflict
		}
	default:
		return ErrFilmProductionStateConflict
	}
	return nil
}
