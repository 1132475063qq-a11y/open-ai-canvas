package repository

import (
	"errors"
	"strings"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) CreateFilmVideoSequenceReview(command FilmVideoSequenceReviewCreateCommand) (*model.FilmVideoSequenceReview, bool, error) {
	if err := validateFilmVideoSequenceReviewCommand(command); err != nil {
		return nil, false, err
	}
	now := runtimeCommandTime(command.At)
	var stored model.FilmVideoSequenceReview
	idempotent := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var existing model.FilmVideoSequenceReview
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&existing,
			"user_id = ? AND idempotency_key = ?", command.UserID, command.Review.IdempotencyKey).Error
		if err == nil {
			if existing.ProjectID != command.ProjectID || existing.SequenceID != command.Review.SequenceID ||
				existing.ScopeFingerprint != command.Review.ScopeFingerprint || existing.Decision != command.Review.Decision ||
				existing.Action != command.Review.Action || existing.IssueCodesJSON != command.Review.IssueCodesJSON ||
				existing.EvidenceJSON != command.Review.EvidenceJSON || existing.Note != command.Review.Note {
				return ErrFilmProductionStateConflict
			}
			stored = existing
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
		var sequence model.FilmVideoSequence
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&sequence,
			"id = ? AND user_id = ? AND project_id = ?", command.Review.SequenceID, command.UserID, command.ProjectID).Error; err != nil {
			return err
		}
		if command.Review.RootRunID != sequence.RootRunID || command.Review.ProjectID != sequence.ProjectID || command.Review.UserID != sequence.UserID {
			return ErrFilmProductionStateConflict
		}
		var ledger model.FilmContinuityLedger
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&ledger,
			"id = ? AND sequence_id = ? AND project_id = ?", command.Review.LedgerID, sequence.ID, command.ProjectID).Error; err != nil {
			return err
		}
		if ledger.UserID != command.UserID || ledger.RootRunID != sequence.RootRunID {
			return ErrFilmProductionStateConflict
		}
		var slots []model.FilmVideoSlot
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("sequence_id = ?", sequence.ID).Find(&slots).Error; err != nil {
			return err
		}
		if len(slots) == 0 {
			return ErrFilmProductionStateConflict
		}
		for _, slot := range slots {
			if slot.Status != model.FilmVideoSlotStatusAccepted || slot.CurrentAttemptID == "" || slot.ResultID == "" {
				return ErrFilmProductionStateConflict
			}
		}
		if command.Review.ScopeFingerprint == "" {
			return ErrFilmProductionStateConflict
		}
		if _, _, err := createProductionArtifactRevisionTx(tx, ProductionArtifactRevisionCreate{
			UserID: command.UserID, Artifact: command.Artifact, Revision: command.Revision, ExpectedSequence: 0, At: now,
		}); err != nil {
			return err
		}
		review := *command.Review
		review.CreatedAt = now
		if err := tx.Create(&review).Error; err != nil {
			return err
		}
		ledgerStatus := model.FilmContinuityLedgerStatusNeedsYou
		issueCount := 1
		sequenceStatus := model.FilmVideoSequenceStatusNeedsReview
		shotStatus := model.FilmContinuityShotStatusNeedsReview
		if review.Decision == model.FilmVideoSequenceReviewDecisionPass {
			ledgerStatus = model.FilmContinuityLedgerStatusReady
			issueCount = 0
			sequenceStatus = model.FilmVideoSequenceStatusCompleted
			shotStatus = model.FilmContinuityShotStatusReady
		} else if strings.TrimSpace(review.IssueCodesJSON) != "" && strings.TrimSpace(review.IssueCodesJSON) != "[]" {
			issueCount = 1
		}
		if err := tx.Model(&model.FilmContinuityLedger{}).Where("id = ?", ledger.ID).Updates(map[string]any{
			"media_state": model.FilmContinuityMediaStateAvailable, "status": ledgerStatus, "issue_count": issueCount,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.FilmContinuityShotState{}).Where("ledger_id = ?", ledger.ID).
			Updates(map[string]any{"status": shotStatus}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.FilmVideoSequence{}).Where("id = ?", sequence.ID).
			Updates(map[string]any{"status": sequenceStatus, "updated_at": now}).Error; err != nil {
			return err
		}
		var root model.AgentRuntimeRun
		if err := lockAgentRuntimeRun(tx, command.UserID, sequence.RootRunID, &root); err != nil {
			return err
		}
		if err := appendFilmProductionEventTx(tx, root, command.Event, "", now); err != nil {
			return err
		}
		stored = review
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return &stored, idempotent, nil
}

func validateFilmVideoSequenceReviewCommand(command FilmVideoSequenceReviewCreateCommand) error {
	if strings.TrimSpace(command.UserID) == "" || strings.TrimSpace(command.ProjectID) == "" ||
		command.Review == nil || command.Artifact == nil || command.Revision == nil ||
		strings.TrimSpace(command.Review.IdempotencyKey) == "" || strings.TrimSpace(command.Review.SequenceID) == "" ||
		strings.TrimSpace(command.Review.LedgerID) == "" || command.Review.Source != "human" || command.Review.AssessmentKind != "" || command.Review.ModelAttemptID != "" ||
		command.Review.UserID != command.UserID || command.Review.ProjectID != command.ProjectID ||
		command.Review.ReviewerUserID != command.UserID || command.Review.ArtifactID != command.Artifact.ID ||
		command.Review.RevisionID != command.Revision.ID || command.Artifact.UserID != command.UserID ||
		command.Artifact.ProjectID != command.ProjectID || command.Artifact.Domain != "film" ||
		command.Artifact.ArtifactType != "sequence-review" || command.Artifact.LogicalKey != "film-video:sequence-review:"+command.Review.ID ||
		command.Revision.Status != model.ProductionArtifactStatusLocked || command.Revision.SourceRunID != command.Review.RootRunID ||
		strings.TrimSpace(command.Revision.ContentDigest) == "" || strings.TrimSpace(command.Review.RootRunID) == "" || strings.TrimSpace(command.Review.ScopeFingerprint) == "" {
		return errors.New("Film video sequence review identity is incomplete")
	}
	switch command.Review.Decision {
	case model.FilmVideoSequenceReviewDecisionPass:
		if command.Review.Action != model.FilmVideoSequenceReviewActionAccept {
			return ErrFilmProductionStateConflict
		}
	case model.FilmVideoSequenceReviewDecisionUncertain:
		if command.Review.Action != model.FilmVideoSequenceReviewActionHold {
			return ErrFilmProductionStateConflict
		}
	case model.FilmVideoSequenceReviewDecisionFail:
		if command.Review.Action != model.FilmVideoSequenceReviewActionRetry {
			return ErrFilmProductionStateConflict
		}
	default:
		return ErrFilmProductionStateConflict
	}
	return validateAgentRuntimeEventInput(command.Event)
}
