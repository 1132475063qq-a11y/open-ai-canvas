package repository

import (
	"errors"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ProductionArtifactRollbackCommand describes a compare-and-swap rollback of
// one Film Artifact. The historical revision is never changed; the repository
// appends a new revision whose content is copied from TargetRevisionID.
type ProductionArtifactRollbackCommand struct {
	UserID                    string
	ProjectID                 string
	Domain                    string
	RunID                     string
	ArtifactID                string
	TargetRevisionID          string
	ExpectedRunRevision       int64
	ExpectedArtifactSequence  int
	ExpectedCurrentRevisionID string
	RollbackRevisionID        string
	EventID                   string
	Event                     AgentRuntimeEventInput
	At                        time.Time
}

type ProductionArtifactRollbackResult struct {
	Run              model.AgentRuntimeRun            `json:"run"`
	Artifact         model.ProductionArtifact         `json:"artifact"`
	TargetRevision   model.ProductionArtifactRevision `json:"targetRevision"`
	RollbackRevision model.ProductionArtifactRevision `json:"rollbackRevision"`
	Idempotent       bool                             `json:"idempotent"`
}

// RollbackProductionArtifactRevision appends a new immutable revision and
// records the operation in the owning Run's append-only event stream.
// Replaying the same deterministic RollbackRevisionID returns the original
// result without advancing the Artifact or Run a second time.
func (r *Repository) RollbackProductionArtifactRevision(command ProductionArtifactRollbackCommand) (*ProductionArtifactRollbackResult, error) {
	if err := validateProductionArtifactRollbackCommand(command); err != nil {
		return nil, err
	}
	now := runtimeCommandTime(command.At)
	result := &ProductionArtifactRollbackResult{}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var project model.Project
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&project,
			"id = ? AND user_id = ?", command.ProjectID, command.UserID).Error; err != nil {
			return err
		}
		if project.Status == model.ProjectStatusArchived {
			return ErrAgentRuntimeProjectArchived
		}

		var run model.AgentRuntimeRun
		if err := lockAgentRuntimeRun(tx, command.UserID, command.RunID, &run); err != nil {
			return err
		}
		if run.ProjectID != command.ProjectID || run.Domain != command.Domain {
			return gorm.ErrRecordNotFound
		}

		var artifact model.ProductionArtifact
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&artifact,
			"id = ? AND user_id = ? AND project_id = ? AND domain = ?",
			command.ArtifactID, command.UserID, command.ProjectID, command.Domain).Error; err != nil {
			return err
		}
		var target model.ProductionArtifactRevision
		if err := tx.First(&target, "id = ? AND artifact_id = ?", command.TargetRevisionID, artifact.ID).Error; err != nil {
			return err
		}

		// The revision ID is derived from the caller's idempotency key. A
		// matching existing row is therefore a replay, even when the caller's
		// compare-and-swap values are now stale because another action followed it.
		var replay model.ProductionArtifactRevision
		replayErr := tx.First(&replay, "id = ? AND artifact_id = ?", command.RollbackRevisionID, artifact.ID).Error
		if replayErr == nil {
			if !sameRollbackRevision(replay, target, command.UserID, command.ExpectedCurrentRevisionID) {
				return ErrProductionArtifactConflict
			}
			if err := tx.First(&result.Run, "id = ?", run.ID).Error; err != nil {
				return err
			}
			result.Artifact = artifact
			result.TargetRevision = target
			result.RollbackRevision = replay
			result.Idempotent = true
			return nil
		}
		if !errors.Is(replayErr, gorm.ErrRecordNotFound) {
			return replayErr
		}

		if run.Revision != command.ExpectedRunRevision ||
			artifact.RevisionSequence != command.ExpectedArtifactSequence ||
			artifact.CurrentRevisionID != command.ExpectedCurrentRevisionID ||
			artifact.CurrentRevisionID == target.ID {
			return ErrProductionArtifactConflict
		}
		if target.SourceRunID != run.ID && target.SourceRunID != run.RootRunID {
			return ErrProductionArtifactConflict
		}

		rollback := model.ProductionArtifactRevision{
			ID:                     command.RollbackRevisionID,
			Status:                 target.Status,
			ContentJSON:            target.ContentJSON,
			ContentText:            target.ContentText,
			ContentDigest:          target.ContentDigest,
			SourceRunID:            target.SourceRunID,
			SourceStepID:           target.SourceStepID,
			SourceAttemptID:        target.SourceAttemptID,
			SourceArtifactRefsJSON: target.SourceArtifactRefsJSON,
			AuthorityRefsJSON:      target.AuthorityRefsJSON,
			CreatedByType:          "user",
			CreatedByID:            command.UserID,
		}
		persistedArtifact, persistedRevision, err := createProductionArtifactRevisionTx(tx, ProductionArtifactRevisionCreate{
			UserID: command.UserID, Artifact: &artifact, Revision: &rollback,
			ExpectedSequence: command.ExpectedArtifactSequence, At: now,
		})
		if err != nil {
			return err
		}

		updated := tx.Model(&model.AgentRuntimeRun{}).
			Where("id = ? AND user_id = ? AND revision = ?", run.ID, run.UserID, run.Revision).
			Updates(map[string]any{
				"revision":       gorm.Expr("revision + ?", 1),
				"event_sequence": gorm.Expr("event_sequence + ?", 1),
				"updated_at":     now,
			})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrAgentRuntimeStateConflict
		}
		event := command.Event
		if strings.TrimSpace(event.PayloadJSON) == "" {
			event.PayloadJSON = mustRepositoryJSON(map[string]any{
				"artifactId": command.ArtifactID, "targetRevisionId": target.ID,
				"sourceCurrentRevisionId": command.ExpectedCurrentRevisionID,
				"rollbackRevisionId":      persistedRevision.ID,
				"contentDigest":           persistedRevision.ContentDigest,
			})
		}
		if err := appendAgentRuntimeEvent(tx, run, event, run.EventSequence+1, "",
			string(target.Status), string(persistedRevision.Status), now); err != nil {
			return err
		}
		if err := tx.First(&result.Run, "id = ?", run.ID).Error; err != nil {
			return err
		}
		result.Artifact = *persistedArtifact
		result.TargetRevision = target
		result.RollbackRevision = *persistedRevision
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func validateProductionArtifactRollbackCommand(command ProductionArtifactRollbackCommand) error {
	if strings.TrimSpace(command.UserID) == "" || strings.TrimSpace(command.ProjectID) == "" ||
		strings.TrimSpace(command.Domain) == "" || strings.TrimSpace(command.RunID) == "" ||
		strings.TrimSpace(command.ArtifactID) == "" || strings.TrimSpace(command.TargetRevisionID) == "" ||
		strings.TrimSpace(command.ExpectedCurrentRevisionID) == "" || strings.TrimSpace(command.RollbackRevisionID) == "" ||
		command.ExpectedRunRevision < 1 || command.ExpectedArtifactSequence < 1 {
		return errors.New("production Artifact rollback identity is incomplete")
	}
	if err := validateAgentRuntimeEventInput(command.Event); err != nil {
		return err
	}
	return nil
}

func sameRollbackRevision(replay model.ProductionArtifactRevision, target model.ProductionArtifactRevision, userID string, expectedCurrentRevisionID string) bool {
	return replay.CreatedByType == "user" && replay.CreatedByID == userID &&
		replay.ParentRevisionID == expectedCurrentRevisionID &&
		replay.ContentDigest == target.ContentDigest && replay.ContentJSON == target.ContentJSON &&
		replay.ContentText == target.ContentText && replay.Status == target.Status &&
		replay.SourceRunID == target.SourceRunID && replay.SourceStepID == target.SourceStepID &&
		replay.SourceAttemptID == target.SourceAttemptID
}
