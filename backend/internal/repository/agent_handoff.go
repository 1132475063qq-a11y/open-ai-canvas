package repository

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrAgentHandoffTriggerLeaseLost = errors.New("agent handoff trigger lease is no longer owned")

type ProductionArtifactLockCommand struct {
	UserID                   string
	ProjectID                string
	Domain                   string
	RunID                    string
	ArtifactID               string
	ExpectedRunRevision      int64
	ExpectedArtifactSequence int
	ExpectedRevisionID       string
	LockedRevisionID         string
	TriggerID                string
	Event                    AgentRuntimeEventInput
	At                       time.Time
}

type ProductionArtifactLockResult struct {
	Run            model.AgentRuntimeRun            `json:"run"`
	Artifact       model.ProductionArtifact         `json:"artifact"`
	SourceRevision model.ProductionArtifactRevision `json:"sourceRevision"`
	LockedRevision model.ProductionArtifactRevision `json:"lockedRevision"`
	Trigger        model.AgentHandoffTrigger        `json:"trigger"`
}

type AgentHandoffTriggerClaimCommand struct {
	Owner         string
	Domain        string
	LeaseDuration time.Duration
	At            time.Time
}

type AgentHandoffTriggerClaim struct {
	Trigger   model.AgentHandoffTrigger
	Recovered bool
}

type AgentHandoffTriggerCompleteCommand struct {
	Owner            string
	TriggerID        string
	ExpectedRevision int64
	ScheduledRunIDs  []string
	At               time.Time
}

type AgentHandoffTriggerFailCommand struct {
	Owner            string
	TriggerID        string
	ExpectedRevision int64
	FailureCode      string
	Failure          string
	RetryAt          *time.Time
	Terminal         bool
	At               time.Time
}

type ProductionArtifactRevisionFact struct {
	Artifact      model.ProductionArtifact
	Revision      model.ProductionArtifactRevision
	SourceAgentID string
}

// LockProductionArtifactForHandoff preserves the REVIEW revision and appends a
// LOCKED revision, its audit Event, and the durable Handoff outbox Trigger in a
// single transaction.
func (r *Repository) LockProductionArtifactForHandoff(command ProductionArtifactLockCommand) (*ProductionArtifactLockResult, error) {
	if err := validateProductionArtifactLockCommand(command); err != nil {
		return nil, err
	}
	now := runtimeCommandTime(command.At)
	result := &ProductionArtifactLockResult{}
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
		if run.Revision != command.ExpectedRunRevision {
			return ErrAgentRuntimeStateConflict
		}
		rootRunID := strings.TrimSpace(run.RootRunID)
		if rootRunID == "" {
			rootRunID = run.ID
		}

		var artifact model.ProductionArtifact
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&artifact,
			"id = ? AND user_id = ? AND project_id = ? AND domain = ?",
			command.ArtifactID, command.UserID, command.ProjectID, command.Domain).Error; err != nil {
			return err
		}
		if artifact.RevisionSequence != command.ExpectedArtifactSequence || artifact.CurrentRevisionID != command.ExpectedRevisionID {
			return ErrProductionArtifactConflict
		}
		var sourceRevision model.ProductionArtifactRevision
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&sourceRevision,
			"id = ? AND artifact_id = ?", command.ExpectedRevisionID, artifact.ID).Error; err != nil {
			return err
		}
		if sourceRevision.Status != model.ProductionArtifactStatusReview || sourceRevision.SourceRunID != run.ID || strings.TrimSpace(sourceRevision.SourceStepID) == "" {
			return ErrProductionArtifactConflict
		}
		var sourceStep model.AgentRuntimeStep
		if err := tx.First(&sourceStep, "id = ? AND run_id = ?", sourceRevision.SourceStepID, run.ID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProductionArtifactConflict
		} else if err != nil {
			return err
		}
		if strings.TrimSpace(sourceStep.AgentID) == "" || sourceStep.Status != model.AgentStepStatusCompleted || strings.TrimSpace(sourceRevision.SourceAttemptID) == "" {
			return ErrProductionArtifactConflict
		}
		var sourceAttempt model.AgentRuntimeAttempt
		if err := tx.First(&sourceAttempt, "id = ? AND run_id = ? AND step_id = ?", sourceRevision.SourceAttemptID, run.ID, sourceStep.ID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProductionArtifactConflict
		} else if err != nil {
			return err
		}
		if sourceAttempt.Status != model.AgentAttemptStatusSucceeded {
			return ErrProductionArtifactConflict
		}

		lockedInput := model.ProductionArtifactRevision{
			ID: command.LockedRevisionID, Status: model.ProductionArtifactStatusLocked,
			ContentJSON: sourceRevision.ContentJSON, ContentText: sourceRevision.ContentText, ContentDigest: sourceRevision.ContentDigest,
			SourceRunID: sourceRevision.SourceRunID, SourceStepID: sourceRevision.SourceStepID, SourceAttemptID: sourceRevision.SourceAttemptID,
			SourceArtifactRefsJSON: sourceRevision.SourceArtifactRefsJSON, AuthorityRefsJSON: sourceRevision.AuthorityRefsJSON,
			CreatedByType: "user", CreatedByID: command.UserID,
		}
		lockedArtifact, lockedRevision, err := createProductionArtifactRevisionTx(tx, ProductionArtifactRevisionCreate{
			UserID: command.UserID, Artifact: &artifact, Revision: &lockedInput,
			ExpectedSequence: command.ExpectedArtifactSequence, At: now,
		})
		if err != nil {
			return err
		}
		trigger := model.AgentHandoffTrigger{
			ID: command.TriggerID, UserID: command.UserID, ProjectID: command.ProjectID, Domain: command.Domain,
			RootRunID: rootRunID, ArtifactID: artifact.ID, RevisionID: lockedRevision.ID,
			SourceRunID: run.ID, SourceStepID: sourceStep.ID, SourceAgentID: sourceStep.AgentID,
			Status: model.AgentHandoffTriggerStatusPending, ScheduledRunIDsJSON: "[]", Revision: 1,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&trigger).Error; err != nil {
			return err
		}

		updated := tx.Model(&model.AgentRuntimeRun{}).
			Where("id = ? AND user_id = ? AND revision = ?", run.ID, run.UserID, run.Revision).
			Updates(map[string]any{
				"revision": gorm.Expr("revision + ?", 1), "event_sequence": gorm.Expr("event_sequence + ?", 1), "updated_at": now,
			})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrAgentRuntimeStateConflict
		}
		event := command.Event
		event.StepID = sourceStep.ID
		if strings.TrimSpace(event.PayloadJSON) == "" {
			event.PayloadJSON = mustRepositoryJSON(map[string]any{
				"artifactId": artifact.ID, "artifactType": artifact.ArtifactType,
				"sourceRevisionId": sourceRevision.ID, "lockedRevisionId": lockedRevision.ID,
				"contentDigest": lockedRevision.ContentDigest, "triggerId": trigger.ID, "rootRunId": rootRunID,
			})
		}
		if err := appendAgentRuntimeEvent(tx, run, event, run.EventSequence+1, sourceStep.ID,
			string(model.ProductionArtifactStatusReview), string(model.ProductionArtifactStatusLocked), now); err != nil {
			return err
		}
		if err := tx.First(&result.Run, "id = ?", run.ID).Error; err != nil {
			return err
		}
		result.Artifact = *lockedArtifact
		result.SourceRevision = sourceRevision
		result.LockedRevision = *lockedRevision
		result.Trigger = trigger
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ClaimNextAgentHandoffTrigger claims a pending Trigger or recovers one whose
// lease expired. The revision increment fences the previous worker.
func (r *Repository) ClaimNextAgentHandoffTrigger(command AgentHandoffTriggerClaimCommand) (*AgentHandoffTriggerClaim, error) {
	if strings.TrimSpace(command.Owner) == "" || strings.TrimSpace(command.Domain) == "" || command.LeaseDuration <= 0 {
		return nil, errors.New("agent Handoff Trigger claim is incomplete")
	}
	now := runtimeCommandTime(command.At)
	leaseExpiresAt := now.Add(command.LeaseDuration)
	result := &AgentHandoffTriggerClaim{}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var candidate model.AgentHandoffTrigger
		query := tx.Model(&model.AgentHandoffTrigger{}).
			Select("agent_handoff_triggers.*").
			Joins("JOIN projects ON projects.id = agent_handoff_triggers.project_id AND projects.user_id = agent_handoff_triggers.user_id").
			Where("agent_handoff_triggers.domain = ? AND projects.status <> ?", command.Domain, model.ProjectStatusArchived).
			Where(`(
				(agent_handoff_triggers.status = ? AND (agent_handoff_triggers.next_attempt_at IS NULL OR agent_handoff_triggers.next_attempt_at <= ?)) OR
				(agent_handoff_triggers.status = ? AND (agent_handoff_triggers.lease_expires_at IS NULL OR agent_handoff_triggers.lease_expires_at <= ?))
			)`, model.AgentHandoffTriggerStatusPending, now, model.AgentHandoffTriggerStatusProcessing, now).
			Order("agent_handoff_triggers.created_at asc").Limit(1)
		if r.Dialect() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		}
		found := query.Find(&candidate)
		if found.Error != nil {
			return found.Error
		}
		if found.RowsAffected == 0 {
			return nil
		}
		recovered := candidate.Status == model.AgentHandoffTriggerStatusProcessing
		updateQuery := tx.Model(&model.AgentHandoffTrigger{}).
			Where("id = ? AND revision = ? AND status = ?", candidate.ID, candidate.Revision, candidate.Status)
		if recovered {
			updateQuery = updateQuery.Where("lease_expires_at IS NULL OR lease_expires_at <= ?", now)
		}
		updated := updateQuery.Updates(map[string]any{
			"status": model.AgentHandoffTriggerStatusProcessing, "attempt_count": gorm.Expr("attempt_count + ?", 1),
			"lease_owner": command.Owner, "lease_expires_at": leaseExpiresAt, "next_attempt_at": nil,
			"revision": gorm.Expr("revision + ?", 1), "updated_at": now,
		})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrAgentRuntimeStateConflict
		}
		if err := tx.First(&result.Trigger, "id = ?", candidate.ID).Error; err != nil {
			return err
		}
		result.Recovered = recovered
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result.Trigger.ID == "" {
		return nil, nil
	}
	return result, nil
}

func (r *Repository) CompleteAgentHandoffTrigger(command AgentHandoffTriggerCompleteCommand) (*model.AgentHandoffTrigger, error) {
	if strings.TrimSpace(command.Owner) == "" || strings.TrimSpace(command.TriggerID) == "" || command.ExpectedRevision < 1 {
		return nil, errors.New("agent Handoff Trigger completion is incomplete")
	}
	runIDs := append([]string(nil), command.ScheduledRunIDs...)
	sort.Strings(runIDs)
	for index, runID := range runIDs {
		if strings.TrimSpace(runID) == "" || (index > 0 && runID == runIDs[index-1]) {
			return nil, errors.New("agent Handoff Trigger scheduled Run IDs are invalid")
		}
	}
	now := runtimeCommandTime(command.At)
	updated := r.db.Model(&model.AgentHandoffTrigger{}).
		Where("id = ? AND revision = ? AND status = ? AND lease_owner = ?", command.TriggerID, command.ExpectedRevision, model.AgentHandoffTriggerStatusProcessing, command.Owner).
		Updates(map[string]any{
			"status": model.AgentHandoffTriggerStatusCompleted, "scheduled_run_ids_json": mustRepositoryJSON(runIDs),
			"failure_code": "", "failure": "", "lease_owner": "", "lease_expires_at": nil,
			"completed_at": now, "revision": gorm.Expr("revision + ?", 1), "updated_at": now,
		})
	if updated.Error != nil {
		return nil, updated.Error
	}
	if updated.RowsAffected != 1 {
		return nil, ErrAgentHandoffTriggerLeaseLost
	}
	return r.AgentHandoffTrigger(command.TriggerID)
}

func (r *Repository) FailAgentHandoffTrigger(command AgentHandoffTriggerFailCommand) (*model.AgentHandoffTrigger, error) {
	if strings.TrimSpace(command.Owner) == "" || strings.TrimSpace(command.TriggerID) == "" || command.ExpectedRevision < 1 ||
		strings.TrimSpace(command.FailureCode) == "" || strings.TrimSpace(command.Failure) == "" {
		return nil, errors.New("agent Handoff Trigger failure is incomplete")
	}
	now := runtimeCommandTime(command.At)
	status := model.AgentHandoffTriggerStatusPending
	var retryAt any
	if command.Terminal {
		status = model.AgentHandoffTriggerStatusFailed
	} else {
		if command.RetryAt == nil || !command.RetryAt.After(now) {
			return nil, errors.New("retryable agent Handoff Trigger failure requires a future retry time")
		}
		retryAt = command.RetryAt.UTC()
	}
	updates := map[string]any{
		"status": status, "failure_code": strings.TrimSpace(command.FailureCode), "failure": strings.TrimSpace(command.Failure),
		"lease_owner": "", "lease_expires_at": nil, "next_attempt_at": retryAt,
		"revision": gorm.Expr("revision + ?", 1), "updated_at": now,
	}
	if command.Terminal {
		updates["completed_at"] = now
	}
	updated := r.db.Model(&model.AgentHandoffTrigger{}).
		Where("id = ? AND revision = ? AND status = ? AND lease_owner = ?", command.TriggerID, command.ExpectedRevision, model.AgentHandoffTriggerStatusProcessing, command.Owner).
		Updates(updates)
	if updated.Error != nil {
		return nil, updated.Error
	}
	if updated.RowsAffected != 1 {
		return nil, ErrAgentHandoffTriggerLeaseLost
	}
	return r.AgentHandoffTrigger(command.TriggerID)
}

func (r *Repository) AgentHandoffTrigger(triggerID string) (*model.AgentHandoffTrigger, error) {
	var trigger model.AgentHandoffTrigger
	if err := r.db.First(&trigger, "id = ?", triggerID).Error; err != nil {
		return nil, err
	}
	return &trigger, nil
}

// LockedProductionArtifactFactsForRoot returns only current LOCKED revisions
// produced by Runs in one root lineage. It never draws from another root Run,
// project, user, or domain.
func (r *Repository) LockedProductionArtifactFactsForRoot(userID string, projectID string, domain string, rootRunID string) ([]ProductionArtifactRevisionFact, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(projectID) == "" || strings.TrimSpace(domain) == "" || strings.TrimSpace(rootRunID) == "" {
		return nil, errors.New("root Run Artifact query scope is incomplete")
	}
	var facts []ProductionArtifactRevisionFact
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var root model.AgentRuntimeRun
		if err := tx.First(&root, "id = ? AND user_id = ? AND project_id = ? AND domain = ?", rootRunID, userID, projectID, domain).Error; err != nil {
			return err
		}
		if root.RootRunID != "" && root.RootRunID != root.ID {
			return fmt.Errorf("%w: requested Run is not a root Run", ErrAgentRuntimeStateConflict)
		}
		var runIDs []string
		if err := tx.Model(&model.AgentRuntimeRun{}).
			Where("user_id = ? AND project_id = ? AND domain = ? AND (root_run_id = ? OR id = ?)", userID, projectID, domain, rootRunID, rootRunID).
			Pluck("id", &runIDs).Error; err != nil {
			return err
		}
		var artifacts []model.ProductionArtifact
		if err := tx.Model(&model.ProductionArtifact{}).
			Select("production_artifacts.*").
			Joins("JOIN production_artifact_revisions current_revision ON current_revision.id = production_artifacts.current_revision_id").
			Where("production_artifacts.user_id = ? AND production_artifacts.project_id = ? AND production_artifacts.domain = ?", userID, projectID, domain).
			Where("current_revision.status = ? AND current_revision.source_run_id IN ?", model.ProductionArtifactStatusLocked, runIDs).
			Order("current_revision.created_at asc, production_artifacts.id asc").Find(&artifacts).Error; err != nil {
			return err
		}
		if len(artifacts) == 0 {
			facts = []ProductionArtifactRevisionFact{}
			return nil
		}
		revisionIDs := make([]string, 0, len(artifacts))
		for _, artifact := range artifacts {
			revisionIDs = append(revisionIDs, artifact.CurrentRevisionID)
		}
		var revisions []model.ProductionArtifactRevision
		if err := tx.Where("id IN ?", revisionIDs).Find(&revisions).Error; err != nil {
			return err
		}
		revisionByID := make(map[string]model.ProductionArtifactRevision, len(revisions))
		stepIDs := make([]string, 0, len(revisions))
		for _, revision := range revisions {
			revisionByID[revision.ID] = revision
			if revision.SourceStepID != "" {
				stepIDs = append(stepIDs, revision.SourceStepID)
			}
		}
		var steps []model.AgentRuntimeStep
		if len(stepIDs) > 0 {
			if err := tx.Where("id IN ? AND run_id IN ?", stepIDs, runIDs).Find(&steps).Error; err != nil {
				return err
			}
		}
		agentByStepID := make(map[string]string, len(steps))
		for _, step := range steps {
			agentByStepID[step.ID] = step.AgentID
		}
		facts = make([]ProductionArtifactRevisionFact, 0, len(artifacts))
		for _, artifact := range artifacts {
			revision, ok := revisionByID[artifact.CurrentRevisionID]
			if !ok || revision.Status != model.ProductionArtifactStatusLocked {
				return errors.New("locked production Artifact snapshot is inconsistent")
			}
			facts = append(facts, ProductionArtifactRevisionFact{
				Artifact: artifact, Revision: revision, SourceAgentID: agentByStepID[revision.SourceStepID],
			})
		}
		return nil
	})
	return facts, err
}

func validateProductionArtifactLockCommand(command ProductionArtifactLockCommand) error {
	if strings.TrimSpace(command.UserID) == "" || strings.TrimSpace(command.ProjectID) == "" || strings.TrimSpace(command.Domain) == "" ||
		strings.TrimSpace(command.RunID) == "" || strings.TrimSpace(command.ArtifactID) == "" || command.ExpectedRunRevision < 1 ||
		command.ExpectedArtifactSequence < 1 || strings.TrimSpace(command.ExpectedRevisionID) == "" ||
		strings.TrimSpace(command.LockedRevisionID) == "" || strings.TrimSpace(command.TriggerID) == "" {
		return errors.New("production Artifact lock command is incomplete")
	}
	if command.ExpectedRevisionID == command.LockedRevisionID {
		return errors.New("locked production Artifact revision must have a new identity")
	}
	return validateAgentRuntimeEventInput(command.Event)
}
