package repository

import (
	"errors"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FilmVideoCompletionCommand struct {
	AttemptID      string
	Result         *model.Result
	ResultArtifact *model.ProductionArtifact
	ResultRevision *model.ProductionArtifactRevision
	QCReport       *model.FilmVideoQCReport
	QCArtifact     *model.ProductionArtifact
	QCRevision     *model.ProductionArtifactRevision
	ReworkEvent    *model.FilmReworkEvent
	ReworkArtifact *model.ProductionArtifact
	ReworkRevision *model.ProductionArtifactRevision
	Event          AgentRuntimeEventInput
	At             time.Time
}

type FilmVideoHumanQCCommand struct {
	UserID    string
	ProjectID string
	AttemptID string
	Report    *model.FilmVideoQCReport
	Artifact  *model.ProductionArtifact
	Revision  *model.ProductionArtifactRevision
	Event     AgentRuntimeEventInput
	At        time.Time
}

func (r *Repository) SaveFilmVideoTaskCompletion(task *model.Task, expected model.TaskStatus, session *model.Session, message *model.Message, results []model.Result, command FilmVideoCompletionCommand) error {
	if err := validateFilmVideoCompletionCommand(task, results, command); err != nil {
		return err
	}
	now := runtimeCommandTime(command.At)
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := saveTaskCompletionTx(tx, task, expected, session, message, results); err != nil {
			return err
		}
		var attempt model.FilmVideoAttempt
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&attempt, "id = ? AND task_id = ?", command.AttemptID, task.ID).Error; err != nil {
			return err
		}
		if attempt.Status != model.FilmProductionAttemptStatusRunning && attempt.Status != model.FilmProductionAttemptStatusQueued {
			return ErrFilmProductionStateConflict
		}
		var slot model.FilmVideoSlot
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&slot, "id = ? AND sequence_id = ?", attempt.SlotID, attempt.SequenceID).Error; err != nil {
			return err
		}
		if slot.CurrentAttemptID != attempt.ID || (slot.Status != model.FilmVideoSlotStatusRunning && slot.Status != model.FilmVideoSlotStatusQueued) {
			return ErrFilmProductionStateConflict
		}
		if err := validateFilmVideoCompletionScope(task, attempt, slot, command); err != nil {
			return err
		}
		if !filmProductionAccessibleMediaURL(command.Result.URL) || command.Result.Availability != model.ResultAvailabilityReady {
			return ErrFilmProductionMediaMissing
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
		if command.ReworkEvent != nil {
			if _, _, err := createProductionArtifactRevisionTx(tx, ProductionArtifactRevisionCreate{
				UserID: attempt.UserID, Artifact: command.ReworkArtifact, Revision: command.ReworkRevision, ExpectedSequence: 0, At: now,
			}); err != nil {
				return err
			}
			if err := tx.Create(command.ReworkEvent).Error; err != nil {
				return err
			}
		}
		updatedAttempt := tx.Model(&model.FilmVideoAttempt{}).
			Where("id = ? AND task_id = ? AND status IN ?", attempt.ID, task.ID, []model.FilmProductionAttemptStatus{
				model.FilmProductionAttemptStatusQueued, model.FilmProductionAttemptStatusRunning,
			}).Updates(map[string]any{
			"status": model.FilmProductionAttemptStatusSucceeded, "result_id": command.Result.ID,
			"result_artifact_id": command.ResultArtifact.ID, "result_revision_id": command.ResultRevision.ID,
			"error": "", "completed_at": now, "updated_at": now,
		})
		if updatedAttempt.Error != nil {
			return updatedAttempt.Error
		}
		if updatedAttempt.RowsAffected != 1 {
			return ErrFilmProductionStateConflict
		}
		updatedSlot := tx.Model(&model.FilmVideoSlot{}).
			Where("id = ? AND current_attempt_id = ? AND status IN ?", slot.ID, attempt.ID, []model.FilmVideoSlotStatus{
				model.FilmVideoSlotStatusQueued, model.FilmVideoSlotStatusRunning,
			}).Updates(map[string]any{
			"status": model.FilmVideoSlotStatusNeedsReview, "result_id": command.Result.ID,
			"result_artifact_id": command.ResultArtifact.ID, "result_revision_id": command.ResultRevision.ID, "updated_at": now,
		})
		if updatedSlot.Error != nil {
			return updatedSlot.Error
		}
		if updatedSlot.RowsAffected != 1 {
			return ErrFilmProductionStateConflict
		}
		if err := aggregateFilmVideoSequenceStatusTx(tx, attempt.SequenceID, now); err != nil {
			return err
		}
		var root model.AgentRuntimeRun
		if err := lockAgentRuntimeRun(tx, attempt.UserID, attempt.RootRunID, &root); err != nil {
			return err
		}
		return appendFilmProductionEventTx(tx, root, command.Event, attempt.ID, now)
	})
}

func (r *Repository) CreateFilmVideoHumanQC(command FilmVideoHumanQCCommand) (*model.FilmVideoQCReport, bool, error) {
	if err := validateFilmVideoHumanQCCommand(command); err != nil {
		return nil, false, err
	}
	now := runtimeCommandTime(command.At)
	var result model.FilmVideoQCReport
	idempotent := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var existing model.FilmVideoQCReport
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
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&project,
			"id = ? AND user_id = ?", command.ProjectID, command.UserID).Error; err != nil {
			return err
		}
		if project.Type != "short-drama" || project.Status != model.ProjectStatusActive {
			return ErrFilmProductionStateConflict
		}
		var attempt model.FilmVideoAttempt
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&attempt,
			"id = ? AND user_id = ? AND project_id = ?", command.AttemptID, command.UserID, command.ProjectID).Error; err != nil {
			return err
		}
		if attempt.Status != model.FilmProductionAttemptStatusSucceeded || attempt.ResultID == "" {
			return ErrFilmProductionStateConflict
		}
		var slot model.FilmVideoSlot
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&slot,
			"id = ? AND sequence_id = ?", attempt.SlotID, attempt.SequenceID).Error; err != nil {
			return err
		}
		if slot.CurrentAttemptID != attempt.ID || slot.ResultID != attempt.ResultID {
			return ErrFilmProductionStateConflict
		}
		var generationResult model.Result
		if err := tx.First(&generationResult, "id = ? AND attempt_id = ? AND kind = ? AND availability = ?",
			attempt.ResultID, attempt.ID, model.ResultKindFilmVideoGeneration, model.ResultAvailabilityReady).Error; err != nil {
			return ErrFilmProductionMediaMissing
		}
		report := *command.Report
		if report.AttemptID != attempt.ID || report.ResultID != generationResult.ID || report.RootRunID != attempt.RootRunID ||
			report.SequenceID != attempt.SequenceID || report.SlotID != attempt.SlotID ||
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
		slotStatus := model.FilmVideoSlotStatusUncertain
		switch report.Action {
		case model.FilmProductionQCActionAccept:
			slotStatus = model.FilmVideoSlotStatusAccepted
		case model.FilmProductionQCActionRetry:
			slotStatus = model.FilmVideoSlotStatusFailed
		}
		updated := tx.Model(&model.FilmVideoSlot{}).Where("id = ? AND current_attempt_id = ?", slot.ID, attempt.ID).
			Updates(map[string]any{"status": slotStatus, "updated_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrFilmProductionStateConflict
		}
		if err := aggregateFilmVideoSequenceStatusTx(tx, attempt.SequenceID, now); err != nil {
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

func markFilmProductionTaskAttemptRunningTx(tx *gorm.DB, taskType string, taskID string, at time.Time) error {
	switch taskType {
	case model.FilmProductionTaskTypeImage:
		return markFilmProductionAttemptRunningTx(tx, taskID, at)
	case model.FilmProductionTaskTypeVideo:
		return markFilmVideoAttemptRunningTx(tx, taskID, at)
	case model.FilmVisualQCTaskTypeImage:
		return markFilmVisualQCTaskAttemptRunningTx(tx, taskID, at)
	case model.FilmVisualQCTaskTypeVideo:
		return markFilmVideoVisualQCTaskAttemptRunningTx(tx, taskID, at)
	case model.FilmVisualQCTaskTypeSequence:
		return markFilmVideoSequenceVisualQCTaskAttemptRunningTx(tx, taskID, at)
	default:
		return nil
	}
}

func markFilmVideoVisualQCTaskAttemptRunningTx(tx *gorm.DB, taskID string, at time.Time) error {
	result := tx.Model(&model.FilmVideoVisualQCAttempt{}).
		Where("task_id = ? AND status = ?", taskID, model.FilmProductionAttemptStatusQueued).
		Updates(map[string]any{"status": model.FilmProductionAttemptStatusRunning, "started_at": at, "updated_at": at})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrFilmProductionStateConflict
	}
	return nil
}

func markFilmProductionTaskAttemptTerminalTx(tx *gorm.DB, taskType string, taskID string, status model.FilmProductionAttemptStatus, errorText string, at time.Time) error {
	switch taskType {
	case model.FilmProductionTaskTypeImage:
		return markFilmProductionAttemptTerminalTx(tx, taskID, status, errorText, at)
	case model.FilmProductionTaskTypeVideo:
		return markFilmVideoAttemptTerminalTx(tx, taskID, status, errorText, at)
	default:
		return nil
	}
}

func markFilmVideoAttemptRunningTx(tx *gorm.DB, taskID string, at time.Time) error {
	var attempt model.FilmVideoAttempt
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&attempt, "task_id = ?", taskID).Error; err != nil {
		return err
	}
	if attempt.Status == model.FilmProductionAttemptStatusRunning {
		return nil
	}
	if attempt.Status != model.FilmProductionAttemptStatusQueued {
		return ErrFilmProductionStateConflict
	}
	updatedAttempt := tx.Model(&model.FilmVideoAttempt{}).
		Where("id = ? AND status = ?", attempt.ID, model.FilmProductionAttemptStatusQueued).
		Updates(map[string]any{"status": model.FilmProductionAttemptStatusRunning, "started_at": gorm.Expr("COALESCE(started_at, ?)", at), "updated_at": at})
	if updatedAttempt.Error != nil {
		return updatedAttempt.Error
	}
	if updatedAttempt.RowsAffected != 1 {
		return ErrFilmProductionStateConflict
	}
	updatedSlot := tx.Model(&model.FilmVideoSlot{}).
		Where("id = ? AND current_attempt_id = ? AND status = ?", attempt.SlotID, attempt.ID, model.FilmVideoSlotStatusQueued).
		Updates(map[string]any{"status": model.FilmVideoSlotStatusRunning, "updated_at": at})
	if updatedSlot.Error != nil {
		return updatedSlot.Error
	}
	if updatedSlot.RowsAffected != 1 {
		return ErrFilmProductionStateConflict
	}
	return aggregateFilmVideoSequenceStatusTx(tx, attempt.SequenceID, at)
}

func markFilmVideoAttemptTerminalTx(tx *gorm.DB, taskID string, status model.FilmProductionAttemptStatus, errorText string, at time.Time) error {
	if status != model.FilmProductionAttemptStatusFailed && status != model.FilmProductionAttemptStatusCancelled && status != model.FilmProductionAttemptStatusUncertain {
		return errors.New("unsupported Film video terminal status")
	}
	var attempt model.FilmVideoAttempt
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&attempt, "task_id = ?", taskID).Error; err != nil {
		return err
	}
	updatedAttempt := tx.Model(&model.FilmVideoAttempt{}).
		Where("id = ? AND status IN ?", attempt.ID, []model.FilmProductionAttemptStatus{
			model.FilmProductionAttemptStatusQueued, model.FilmProductionAttemptStatusRunning,
		}).Updates(map[string]any{"status": status, "error": strings.TrimSpace(errorText), "completed_at": at, "updated_at": at})
	if updatedAttempt.Error != nil {
		return updatedAttempt.Error
	}
	if updatedAttempt.RowsAffected != 1 {
		return ErrFilmProductionStateConflict
	}
	slotStatus := model.FilmVideoSlotStatusFailed
	if status == model.FilmProductionAttemptStatusCancelled {
		slotStatus = model.FilmVideoSlotStatusCancelled
	} else if status == model.FilmProductionAttemptStatusUncertain {
		slotStatus = model.FilmVideoSlotStatusUncertain
	}
	updatedSlot := tx.Model(&model.FilmVideoSlot{}).
		Where("id = ? AND current_attempt_id = ?", attempt.SlotID, attempt.ID).
		Updates(map[string]any{"status": slotStatus, "updated_at": at})
	if updatedSlot.Error != nil {
		return updatedSlot.Error
	}
	if updatedSlot.RowsAffected != 1 {
		return ErrFilmProductionStateConflict
	}
	return aggregateFilmVideoSequenceStatusTx(tx, attempt.SequenceID, at)
}

func validateFilmVideoCompletionCommand(task *model.Task, results []model.Result, command FilmVideoCompletionCommand) error {
	if task == nil || strings.TrimSpace(command.AttemptID) == "" || command.Result == nil || command.ResultArtifact == nil || command.ResultRevision == nil ||
		command.QCReport == nil || command.QCArtifact == nil || command.QCRevision == nil || len(results) == 0 {
		return errors.New("Film video completion identity is incomplete")
	}
	if command.QCReport.Decision == model.FilmProductionQCDecisionFail {
		if command.ReworkEvent == nil || command.ReworkArtifact == nil || command.ReworkRevision == nil {
			return errors.New("Film video technical QC failure requires a Rework Event")
		}
	} else if command.ReworkEvent != nil || command.ReworkArtifact != nil || command.ReworkRevision != nil {
		return errors.New("Film video Rework Event requires a failed technical QC")
	}
	if err := validateAgentRuntimeEventInput(command.Event); err != nil {
		return err
	}
	for index := range results {
		if results[index].ID == command.Result.ID {
			return nil
		}
	}
	return errors.New("Film video completion Result is missing from Task completion")
}

func validateFilmVideoCompletionScope(task *model.Task, attempt model.FilmVideoAttempt, slot model.FilmVideoSlot, command FilmVideoCompletionCommand) error {
	result := command.Result
	qc := command.QCReport
	if task.Type != model.FilmProductionTaskTypeVideo || task.UserID != attempt.UserID || task.ProjectID != attempt.ProjectID ||
		attempt.SequenceID != slot.SequenceID || attempt.SlotID != slot.ID ||
		result.UserID != attempt.UserID || result.TaskID != task.ID || result.AttemptID != attempt.ID || result.DomainProjectID != attempt.ProjectID ||
		result.Kind != model.ResultKindFilmVideoGeneration || result.Availability != model.ResultAvailabilityReady || strings.TrimSpace(result.Payload) == "" ||
		result.ArtifactID != command.ResultArtifact.ID || result.ArtifactRevisionID != command.ResultRevision.ID ||
		command.ResultArtifact.UserID != attempt.UserID || command.ResultArtifact.ProjectID != attempt.ProjectID || command.ResultArtifact.Domain != "film" ||
		command.ResultArtifact.ArtifactType != "generation-result" || command.ResultArtifact.LogicalKey != "film-video:result:"+attempt.ID ||
		command.ResultRevision.Status != model.ProductionArtifactStatusLocked || command.ResultRevision.SourceRunID != attempt.RootRunID ||
		command.ResultRevision.SourceAttemptID != attempt.ID || strings.TrimSpace(command.ResultRevision.ContentDigest) == "" ||
		qc.UserID != attempt.UserID || qc.ProjectID != attempt.ProjectID || qc.RootRunID != attempt.RootRunID || qc.SequenceID != attempt.SequenceID ||
		qc.SlotID != attempt.SlotID || qc.AttemptID != attempt.ID || qc.ResultID != result.ID || qc.Source != "system" ||
		(qc.Decision != model.FilmProductionQCDecisionNotAssessable && qc.Decision != model.FilmProductionQCDecisionFail) ||
		qc.Action != model.FilmProductionQCActionHold || qc.AssessmentKind != "technical_media_qc" ||
		qc.MediaState != model.FilmContinuityMediaStateAvailable ||
		qc.ArtifactID != command.QCArtifact.ID || qc.RevisionID != command.QCRevision.ID ||
		command.QCArtifact.UserID != attempt.UserID || command.QCArtifact.ProjectID != attempt.ProjectID || command.QCArtifact.Domain != "film" ||
		command.QCArtifact.ArtifactType != "media-qc-report" || command.QCArtifact.LogicalKey != "film-video:system-qc:"+attempt.ID ||
		command.QCRevision.Status != model.ProductionArtifactStatusLocked || command.QCRevision.SourceRunID != attempt.RootRunID ||
		command.QCRevision.SourceAttemptID != attempt.ID || strings.TrimSpace(command.QCRevision.ContentDigest) == "" {
		return ErrFilmProductionStateConflict
	}
	if qc.Decision == model.FilmProductionQCDecisionFail {
		rework := command.ReworkEvent
		if qc.ReworkEventID != rework.ID || rework.UserID != attempt.UserID || rework.ProjectID != attempt.ProjectID ||
			rework.RootRunID != attempt.RootRunID || rework.SequenceID != attempt.SequenceID || rework.SlotID != attempt.SlotID ||
			rework.ShotID != slot.ShotID || rework.AttemptID != attempt.ID || rework.ResultID != result.ID || rework.QCReportID != qc.ID ||
			rework.MediaType != "video" || rework.Source != "system" || rework.Status != model.FilmReworkStatusOpen ||
			rework.ArtifactID != command.ReworkArtifact.ID || rework.ArtifactRevisionID != command.ReworkRevision.ID ||
			command.ReworkArtifact.UserID != attempt.UserID || command.ReworkArtifact.ProjectID != attempt.ProjectID ||
			command.ReworkArtifact.Domain != "film" || command.ReworkArtifact.ArtifactType != "rework-event" ||
			command.ReworkRevision.Status != model.ProductionArtifactStatusLocked || command.ReworkRevision.SourceRunID != attempt.RootRunID ||
			command.ReworkRevision.SourceAttemptID != attempt.ID || strings.TrimSpace(command.ReworkRevision.ContentDigest) == "" {
			return ErrFilmProductionStateConflict
		}
	}
	return nil
}

func validateFilmVideoHumanQCCommand(command FilmVideoHumanQCCommand) error {
	if strings.TrimSpace(command.UserID) == "" || strings.TrimSpace(command.ProjectID) == "" || strings.TrimSpace(command.AttemptID) == "" ||
		command.Report == nil || command.Artifact == nil || command.Revision == nil || strings.TrimSpace(command.Report.IdempotencyKey) == "" ||
		command.Report.Source != "human" || command.Report.UserID != command.UserID || command.Report.ProjectID != command.ProjectID ||
		command.Report.ReviewerUserID != command.UserID {
		return errors.New("Film video human QC identity is incomplete")
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
