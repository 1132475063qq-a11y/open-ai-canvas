package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"infinite-canvas/backend/internal/agentruntime"
	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrAgentRuntimeStateConflict     = errors.New("agent runtime state changed concurrently")
	ErrAgentRuntimeInvalidTransition = errors.New("agent runtime state transition is invalid")
	ErrAgentRuntimeActiveDecision    = errors.New("agent runtime already has a pending human decision")
	ErrAgentRuntimeLeaseLost         = errors.New("agent runtime execution lease is no longer owned")
	ErrProductionArtifactConflict    = errors.New("production artifact changed concurrently")
)

type AgentRuntimeCreateBundle struct {
	Run               *model.AgentRuntimeRun
	RoutingDecision   *model.AgentRoutingDecision
	Steps             []model.AgentRuntimeStep
	HumanDecision     *model.AgentHumanDecision
	Events            []model.AgentRuntimeEvent
	Artifacts         []model.ProductionArtifact
	ArtifactRevisions []model.ProductionArtifactRevision
}

type AgentRuntimeDetail struct {
	Run               model.AgentRuntimeRun              `json:"run"`
	RoutingDecisions  []model.AgentRoutingDecision       `json:"routingDecisions"`
	Steps             []model.AgentRuntimeStep           `json:"steps"`
	Attempts          []model.AgentRuntimeAttempt        `json:"attempts"`
	HumanDecisions    []model.AgentHumanDecision         `json:"humanDecisions"`
	Events            []model.AgentRuntimeEvent          `json:"events"`
	Artifacts         []model.ProductionArtifact         `json:"artifacts"`
	ArtifactRevisions []model.ProductionArtifactRevision `json:"artifactRevisions"`
}

type AgentRuntimeEventInput struct {
	ID          string
	EventType   string
	ActorType   string
	ActorID     string
	PayloadJSON string
	StepID      string
	AttemptID   string
}

type AgentRuntimeRunTransition struct {
	UserID           string
	RunID            string
	ExpectedRevision int64
	ToStatus         model.AgentRuntimeRunStatus
	CurrentStepID    string
	FailureCode      string
	Failure          string
	Event            AgentRuntimeEventInput
	At               time.Time
}

type AgentRuntimeStepTransition struct {
	UserID               string
	RunID                string
	StepID               string
	ExpectedRunRevision  int64
	ExpectedStepRevision int64
	ToStatus             model.AgentRuntimeStepStatus
	RunStatus            *model.AgentRuntimeRunStatus
	FailureCode          string
	Failure              string
	Event                AgentRuntimeEventInput
	At                   time.Time
}

type AgentRuntimeAttemptCreate struct {
	UserID               string
	RunID                string
	StepID               string
	ExpectedRunRevision  int64
	ExpectedStepRevision int64
	Attempt              *model.AgentRuntimeAttempt
	Event                AgentRuntimeEventInput
	At                   time.Time
}

type AgentRuntimeAttemptTransition struct {
	UserID                  string
	RunID                   string
	StepID                  string
	AttemptID               string
	ExpectedRunRevision     int64
	ExpectedStepRevision    int64
	ExpectedAttemptRevision int64
	ToStatus                model.AgentRuntimeAttemptStatus
	StepStatus              *model.AgentRuntimeStepStatus
	RunStatus               *model.AgentRuntimeRunStatus
	FailureCode             string
	Failure                 string
	ResponseJSON            string
	Event                   AgentRuntimeEventInput
	At                      time.Time
}

type ProductionArtifactRevisionCreate struct {
	UserID           string
	Artifact         *model.ProductionArtifact
	Revision         *model.ProductionArtifactRevision
	ExpectedSequence int
	At               time.Time
}

type AgentRuntimeHumanPause struct {
	UserID               string
	RunID                string
	StepID               string
	ExpectedRunRevision  int64
	ExpectedStepRevision int64
	Decision             *model.AgentHumanDecision
	Event                AgentRuntimeEventInput
	At                   time.Time
}

type AgentRuntimeHumanResolve struct {
	UserID                   string
	RunID                    string
	DecisionID               string
	ExpectedRunRevision      int64
	ExpectedStepRevision     int64
	ExpectedDecisionRevision int64
	RunStatus                model.AgentRuntimeRunStatus
	StepStatus               model.AgentRuntimeStepStatus
	ResponseJSON             string
	Event                    AgentRuntimeEventInput
	At                       time.Time
}

type AgentRuntimeExecutionClaimCommand struct {
	Owner         string
	LeaseDuration time.Duration
	AttemptID     string
	TaskID        string
	EventID       string
	Executor      string
	At            time.Time
}

type AgentRuntimeExecutionClaim struct {
	Run       model.AgentRuntimeRun
	Step      model.AgentRuntimeStep
	Attempt   model.AgentRuntimeAttempt
	Recovered bool
}

type AgentRuntimeAttemptMetadata struct {
	Owner                   string
	RunID                   string
	StepID                  string
	AttemptID               string
	ExpectedAttemptRevision int64
	Executor                string
	ModelRef                string
	InputDigest             string
	PromptDigest            string
	RequestJSON             string
	At                      time.Time
}

type ProductionArtifactWrite struct {
	Artifact         model.ProductionArtifact
	Revision         model.ProductionArtifactRevision
	ExpectedSequence int
}

type AgentRuntimeExecutionCompleteCommand struct {
	Owner                   string
	UserID                  string
	RunID                   string
	StepID                  string
	AttemptID               string
	ExpectedRunRevision     int64
	ExpectedStepRevision    int64
	ExpectedAttemptRevision int64
	ResponseJSON            string
	ArtifactWrites          []ProductionArtifactWrite
	Event                   AgentRuntimeEventInput
	At                      time.Time
}

type AgentRuntimeExecutionCompleteResult struct {
	Run               model.AgentRuntimeRun
	Step              model.AgentRuntimeStep
	Attempt           model.AgentRuntimeAttempt
	ReadySteps        []model.AgentRuntimeStep
	Artifacts         []model.ProductionArtifact
	ArtifactRevisions []model.ProductionArtifactRevision
}

type AgentRuntimeExecutionFailCommand struct {
	Owner                   string
	UserID                  string
	RunID                   string
	StepID                  string
	AttemptID               string
	ExpectedRunRevision     int64
	ExpectedStepRevision    int64
	ExpectedAttemptRevision int64
	FailureCode             string
	Failure                 string
	Event                   AgentRuntimeEventInput
	At                      time.Time
}

func (r *Repository) CreateAgentRuntimeBundle(bundle AgentRuntimeCreateBundle) error {
	if err := validateAgentRuntimeCreateBundle(bundle); err != nil {
		return err
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(bundle.Run).Error; err != nil {
			return err
		}
		if err := tx.Create(bundle.RoutingDecision).Error; err != nil {
			return err
		}
		if err := tx.Create(&bundle.Steps).Error; err != nil {
			return err
		}
		if bundle.HumanDecision != nil {
			if err := tx.Create(bundle.HumanDecision).Error; err != nil {
				return err
			}
		}
		if len(bundle.Artifacts) > 0 {
			if err := tx.Create(&bundle.Artifacts).Error; err != nil {
				return err
			}
		}
		if len(bundle.ArtifactRevisions) > 0 {
			if err := tx.Create(&bundle.ArtifactRevisions).Error; err != nil {
				return err
			}
		}
		return tx.Create(&bundle.Events).Error
	})
}

func (r *Repository) AgentRuntimeRunForUser(userID string, runID string) (*model.AgentRuntimeRun, error) {
	var run model.AgentRuntimeRun
	if err := r.db.First(&run, "id = ? AND user_id = ?", runID, userID).Error; err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *Repository) AgentRuntimeRunByIdempotency(userID string, key string) (*model.AgentRuntimeRun, error) {
	var run model.AgentRuntimeRun
	if err := r.db.First(&run, "user_id = ? AND idempotency_key = ?", userID, key).Error; err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *Repository) ProjectAgentRuntimeRuns(userID string, projectID string, limit int) ([]model.AgentRuntimeRun, error) {
	return r.ProjectAgentRuntimeRunsForDomain(userID, projectID, "", limit)
}

func (r *Repository) ProjectAgentRuntimeRunsForDomain(userID string, projectID string, domain string, limit int) ([]model.AgentRuntimeRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var runs []model.AgentRuntimeRun
	query := r.db.Where("user_id = ? AND project_id = ?", userID, projectID)
	if domain = strings.TrimSpace(domain); domain != "" {
		query = query.Where("domain = ?", domain)
	}
	err := query.
		Order("created_at desc").
		Limit(limit).
		Find(&runs).Error
	return runs, err
}

func (r *Repository) AgentRuntimeDetailForUser(userID string, runID string) (AgentRuntimeDetail, error) {
	run, err := r.AgentRuntimeRunForUser(userID, runID)
	if err != nil {
		return AgentRuntimeDetail{}, err
	}
	detail := AgentRuntimeDetail{Run: *run}
	if err := r.db.Where("run_id = ?", runID).Order("created_at asc").Find(&detail.RoutingDecisions).Error; err != nil {
		return AgentRuntimeDetail{}, err
	}
	if err := r.db.Where("run_id = ?", runID).Order("position asc, created_at asc").Find(&detail.Steps).Error; err != nil {
		return AgentRuntimeDetail{}, err
	}
	if err := r.db.Where("run_id = ?", runID).Order("created_at asc").Find(&detail.Attempts).Error; err != nil {
		return AgentRuntimeDetail{}, err
	}
	if err := r.db.Where("run_id = ?", runID).Order("created_at asc").Find(&detail.HumanDecisions).Error; err != nil {
		return AgentRuntimeDetail{}, err
	}
	if err := r.db.Where("run_id = ?", runID).Order("sequence asc").Find(&detail.Events).Error; err != nil {
		return AgentRuntimeDetail{}, err
	}
	artifactIDs := r.db.Model(&model.ProductionArtifactRevision{}).
		Select("artifact_id").
		Where("source_run_id = ?", runID)
	if err := r.db.Where("id IN (?)", artifactIDs).Order("created_at asc").Find(&detail.Artifacts).Error; err != nil {
		return AgentRuntimeDetail{}, err
	}
	if err := r.db.Where("source_run_id = ?", runID).Order("artifact_id asc, version asc").Find(&detail.ArtifactRevisions).Error; err != nil {
		return AgentRuntimeDetail{}, err
	}
	return detail, nil
}

// ClaimNextAgentRuntimeExecution fences one ready Step, or recovers one whose
// lease expired. The active Attempt is created or resumed in the same
// transaction, so a process restart cannot create a second provider request.
func (r *Repository) ClaimNextAgentRuntimeExecution(command AgentRuntimeExecutionClaimCommand) (*AgentRuntimeExecutionClaim, error) {
	if strings.TrimSpace(command.Owner) == "" || command.LeaseDuration <= 0 || strings.TrimSpace(command.AttemptID) == "" ||
		strings.TrimSpace(command.TaskID) == "" || strings.TrimSpace(command.EventID) == "" || strings.TrimSpace(command.Executor) == "" {
		return nil, errors.New("agent runtime execution claim is incomplete")
	}
	now := runtimeCommandTime(command.At)
	leaseExpiresAt := now.Add(command.LeaseDuration)
	var result AgentRuntimeExecutionClaim
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var candidate model.AgentRuntimeStep
		query := tx.Model(&model.AgentRuntimeStep{}).
			Select("agent_runtime_steps.*").
			Joins("JOIN agent_runtime_runs ON agent_runtime_runs.id = agent_runtime_steps.run_id").
			Joins("JOIN projects ON projects.id = agent_runtime_runs.project_id AND projects.user_id = agent_runtime_runs.user_id").
			Where("agent_runtime_runs.domain = ? AND projects.status <> ?", "film", model.ProjectStatusArchived).
			Where(`(
				(agent_runtime_steps.status = ? AND agent_runtime_runs.status IN ?) OR
				(agent_runtime_steps.status = ? AND agent_runtime_runs.status = ? AND (agent_runtime_steps.lease_expires_at IS NULL OR agent_runtime_steps.lease_expires_at <= ?))
			)`, model.AgentStepStatusReady, []model.AgentRuntimeRunStatus{model.AgentRunStatusReady, model.AgentRunStatusRunning},
				model.AgentStepStatusRunning, model.AgentRunStatusRunning, now).
			Order("agent_runtime_runs.created_at asc, agent_runtime_steps.position asc, agent_runtime_steps.created_at asc").
			Limit(1)
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

		var run model.AgentRuntimeRun
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&run, "id = ?", candidate.RunID).Error; err != nil {
			return err
		}
		var step model.AgentRuntimeStep
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&step, "id = ? AND run_id = ?", candidate.ID, run.ID).Error; err != nil {
			return err
		}
		recovered := step.Status == model.AgentStepStatusRunning
		if step.Status == model.AgentStepStatusReady {
			if run.Status != model.AgentRunStatusReady && run.Status != model.AgentRunStatusRunning {
				return ErrAgentRuntimeStateConflict
			}
		} else if step.Status != model.AgentStepStatusRunning || run.Status != model.AgentRunStatusRunning ||
			(step.LeaseExpiresAt != nil && step.LeaseExpiresAt.After(now)) {
			return ErrAgentRuntimeStateConflict
		}

		var attempt model.AgentRuntimeAttempt
		attemptFound := tx.Where("step_id = ? AND status IN ?", step.ID, []model.AgentRuntimeAttemptStatus{model.AgentAttemptStatusQueued, model.AgentAttemptStatusRunning}).
			Order("number desc").Limit(1).Find(&attempt)
		if attemptFound.Error != nil {
			return attemptFound.Error
		}
		if attemptFound.RowsAffected == 0 {
			attempt = model.AgentRuntimeAttempt{
				ID: command.AttemptID, RunID: run.ID, StepID: step.ID, Number: step.AttemptSequence + 1,
				TaskID: command.TaskID, Status: model.AgentAttemptStatusRunning, Executor: command.Executor,
				RequestJSON: "{}", Revision: 1, StartedAt: &now, CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Create(&attempt).Error; err != nil {
				return err
			}
		} else {
			if attempt.Status == model.AgentAttemptStatusRunning {
				recovered = true
			}
			updates := map[string]any{
				"status":     model.AgentAttemptStatusRunning,
				"executor":   command.Executor,
				"started_at": gorm.Expr("COALESCE(started_at, ?)", now),
				"revision":   gorm.Expr("revision + ?", 1),
				"updated_at": now,
			}
			if strings.TrimSpace(attempt.TaskID) == "" {
				updates["task_id"] = command.TaskID
			}
			updated := tx.Model(&model.AgentRuntimeAttempt{}).
				Where("id = ? AND step_id = ? AND revision = ? AND status IN ?", attempt.ID, step.ID, attempt.Revision, []model.AgentRuntimeAttemptStatus{model.AgentAttemptStatusQueued, model.AgentAttemptStatusRunning}).
				Updates(updates)
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return ErrAgentRuntimeStateConflict
			}
		}

		stepUpdates := map[string]any{
			"status":           model.AgentStepStatusRunning,
			"attempt_sequence": max(step.AttemptSequence, attempt.Number),
			"lease_owner":      command.Owner,
			"lease_expires_at": leaseExpiresAt,
			"failure_code":     "",
			"failure":          "",
			"completed_at":     nil,
			"started_at":       gorm.Expr("COALESCE(started_at, ?)", now),
			"revision":         gorm.Expr("revision + ?", 1),
			"updated_at":       now,
		}
		stepQuery := tx.Model(&model.AgentRuntimeStep{}).Where("id = ? AND run_id = ? AND revision = ? AND status = ?", step.ID, run.ID, step.Revision, step.Status)
		if step.Status == model.AgentStepStatusRunning {
			stepQuery = stepQuery.Where("lease_expires_at IS NULL OR lease_expires_at <= ?", now)
		}
		stepUpdated := stepQuery.Updates(stepUpdates)
		if stepUpdated.Error != nil {
			return stepUpdated.Error
		}
		if stepUpdated.RowsAffected != 1 {
			return ErrAgentRuntimeStateConflict
		}
		if !agentruntime.CanTransitionRun(run.Status, model.AgentRunStatusRunning) {
			return fmt.Errorf("%w: Run %s -> %s", ErrAgentRuntimeInvalidTransition, run.Status, model.AgentRunStatusRunning)
		}
		runUpdates := runTransitionUpdates(run, model.AgentRunStatusRunning, step.ID, "", "", now)
		runUpdates["revision"] = gorm.Expr("revision + ?", 1)
		runUpdates["event_sequence"] = gorm.Expr("event_sequence + ?", 1)
		runUpdated := tx.Model(&model.AgentRuntimeRun{}).
			Where("id = ? AND revision = ? AND status = ?", run.ID, run.Revision, run.Status).
			Updates(runUpdates)
		if runUpdated.Error != nil {
			return runUpdated.Error
		}
		if runUpdated.RowsAffected != 1 {
			return ErrAgentRuntimeStateConflict
		}
		if err := tx.First(&attempt, "id = ?", attempt.ID).Error; err != nil {
			return err
		}
		eventType := "attempt.started"
		if recovered {
			eventType = "attempt.recovered"
		}
		if err := appendAgentRuntimeEvent(tx, run, AgentRuntimeEventInput{
			ID: command.EventID, EventType: eventType, ActorType: "worker", ActorID: command.Owner,
			StepID: step.ID, AttemptID: attempt.ID,
			PayloadJSON: mustRepositoryJSON(map[string]any{"attemptNumber": attempt.Number, "recovered": recovered}),
		}, run.EventSequence+1, step.ID, string(step.Status), string(model.AgentStepStatusRunning), now); err != nil {
			return err
		}
		if err := tx.First(&result.Run, "id = ?", run.ID).Error; err != nil {
			return err
		}
		if err := tx.First(&result.Step, "id = ?", step.ID).Error; err != nil {
			return err
		}
		if err := tx.First(&result.Attempt, "id = ?", attempt.ID).Error; err != nil {
			return err
		}
		result.Recovered = recovered
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result.Run.ID == "" {
		return nil, nil
	}
	return &result, nil
}

func (r *Repository) PrepareClaimedAgentRuntimeAttempt(command AgentRuntimeAttemptMetadata) (*model.AgentRuntimeAttempt, error) {
	if strings.TrimSpace(command.Owner) == "" || strings.TrimSpace(command.RunID) == "" || strings.TrimSpace(command.StepID) == "" ||
		strings.TrimSpace(command.AttemptID) == "" || command.ExpectedAttemptRevision < 1 || strings.TrimSpace(command.Executor) == "" ||
		strings.TrimSpace(command.InputDigest) == "" || strings.TrimSpace(command.PromptDigest) == "" || strings.TrimSpace(command.RequestJSON) == "" {
		return nil, errors.New("claimed agent runtime Attempt metadata is incomplete")
	}
	now := runtimeCommandTime(command.At)
	var result model.AgentRuntimeAttempt
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var step model.AgentRuntimeStep
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&step, "id = ? AND run_id = ?", command.StepID, command.RunID).Error; err != nil {
			return err
		}
		if step.Status != model.AgentStepStatusRunning || step.LeaseOwner != command.Owner {
			return ErrAgentRuntimeLeaseLost
		}
		updated := tx.Model(&model.AgentRuntimeAttempt{}).
			Where("id = ? AND run_id = ? AND step_id = ? AND status = ? AND revision = ?", command.AttemptID, command.RunID, command.StepID, model.AgentAttemptStatusRunning, command.ExpectedAttemptRevision).
			Updates(map[string]any{
				"executor": command.Executor, "model_ref": strings.TrimSpace(command.ModelRef),
				"input_digest": command.InputDigest, "prompt_digest": command.PromptDigest,
				"request_json": command.RequestJSON, "revision": gorm.Expr("revision + ?", 1), "updated_at": now,
			})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrAgentRuntimeStateConflict
		}
		return tx.First(&result, "id = ?", command.AttemptID).Error
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *Repository) RenewAgentRuntimeExecutionLease(stepID string, attemptID string, owner string, leaseDuration time.Duration) error {
	if strings.TrimSpace(stepID) == "" || strings.TrimSpace(attemptID) == "" || strings.TrimSpace(owner) == "" || leaseDuration <= 0 {
		return errors.New("agent runtime lease renewal is incomplete")
	}
	now := time.Now().UTC()
	updated := r.db.Model(&model.AgentRuntimeStep{}).
		Where("id = ? AND status = ? AND lease_owner = ? AND EXISTS (SELECT 1 FROM agent_runtime_attempts WHERE id = ? AND step_id = agent_runtime_steps.id AND status = ?)",
			stepID, model.AgentStepStatusRunning, owner, attemptID, model.AgentAttemptStatusRunning).
		Updates(map[string]any{"lease_expires_at": now.Add(leaseDuration), "updated_at": now})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return ErrAgentRuntimeLeaseLost
	}
	return nil
}

// CompleteAgentRuntimeExecution commits validated output, immutable Artifact
// revisions, the terminal Attempt/Step facts, and every newly unblocked Step in
// one transaction.
func (r *Repository) CompleteAgentRuntimeExecution(command AgentRuntimeExecutionCompleteCommand) (*AgentRuntimeExecutionCompleteResult, error) {
	if strings.TrimSpace(command.Owner) == "" || strings.TrimSpace(command.UserID) == "" || strings.TrimSpace(command.RunID) == "" ||
		strings.TrimSpace(command.StepID) == "" || strings.TrimSpace(command.AttemptID) == "" || command.ExpectedRunRevision < 1 ||
		command.ExpectedStepRevision < 1 || command.ExpectedAttemptRevision < 1 || strings.TrimSpace(command.ResponseJSON) == "" {
		return nil, errors.New("agent runtime execution completion is incomplete")
	}
	if err := validateAgentRuntimeEventInput(command.Event); err != nil {
		return nil, err
	}
	now := runtimeCommandTime(command.At)
	result := &AgentRuntimeExecutionCompleteResult{}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var run model.AgentRuntimeRun
		if err := lockAgentRuntimeRun(tx, command.UserID, command.RunID, &run); err != nil {
			return err
		}
		if run.Revision != command.ExpectedRunRevision || run.Status != model.AgentRunStatusRunning {
			return ErrAgentRuntimeStateConflict
		}
		var step model.AgentRuntimeStep
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&step, "id = ? AND run_id = ?", command.StepID, run.ID).Error; err != nil {
			return err
		}
		if step.Revision != command.ExpectedStepRevision || step.Status != model.AgentStepStatusRunning || step.LeaseOwner != command.Owner {
			return ErrAgentRuntimeLeaseLost
		}
		var attempt model.AgentRuntimeAttempt
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&attempt, "id = ? AND run_id = ? AND step_id = ?", command.AttemptID, run.ID, step.ID).Error; err != nil {
			return err
		}
		if attempt.Revision != command.ExpectedAttemptRevision || attempt.Status != model.AgentAttemptStatusRunning {
			return ErrAgentRuntimeStateConflict
		}

		refs := make([]productionArtifactRef, 0, len(command.ArtifactWrites))
		for index := range command.ArtifactWrites {
			write := command.ArtifactWrites[index]
			if write.Artifact.UserID != run.UserID || write.Artifact.ProjectID != run.ProjectID || write.Artifact.Domain != run.Domain ||
				write.Revision.SourceRunID != run.ID || write.Revision.SourceStepID != step.ID || write.Revision.SourceAttemptID != attempt.ID {
				return errors.New("agent runtime output Artifact is outside the execution scope")
			}
			artifact, revision, err := createProductionArtifactRevisionTx(tx, ProductionArtifactRevisionCreate{
				UserID: run.UserID, Artifact: &write.Artifact, Revision: &write.Revision,
				ExpectedSequence: write.ExpectedSequence, At: now,
			})
			if err != nil {
				return err
			}
			result.Artifacts = append(result.Artifacts, *artifact)
			result.ArtifactRevisions = append(result.ArtifactRevisions, *revision)
			refs = append(refs, productionArtifactRef{
				ArtifactID: artifact.ID, RevisionID: revision.ID, Type: artifact.ArtifactType,
				Version: revision.Version, Digest: revision.ContentDigest, Status: string(revision.Status),
			})
		}
		outputRefsJSON := mustRepositoryJSON(refs)
		attemptUpdated := tx.Model(&model.AgentRuntimeAttempt{}).
			Where("id = ? AND revision = ? AND status = ?", attempt.ID, attempt.Revision, attempt.Status).
			Updates(map[string]any{
				"status": model.AgentAttemptStatusSucceeded, "response_json": command.ResponseJSON,
				"failure_code": "", "failure": "", "completed_at": now,
				"revision": gorm.Expr("revision + ?", 1), "updated_at": now,
			})
		if attemptUpdated.Error != nil {
			return attemptUpdated.Error
		}
		if attemptUpdated.RowsAffected != 1 {
			return ErrAgentRuntimeStateConflict
		}
		stepUpdated := tx.Model(&model.AgentRuntimeStep{}).
			Where("id = ? AND revision = ? AND status = ? AND lease_owner = ?", step.ID, step.Revision, step.Status, command.Owner).
			Updates(map[string]any{
				"status": model.AgentStepStatusCompleted, "output_artifact_refs_json": outputRefsJSON,
				"failure_code": "", "failure": "", "completed_at": now,
				"lease_owner": "", "lease_expires_at": nil,
				"revision": gorm.Expr("revision + ?", 1), "updated_at": now,
			})
		if stepUpdated.Error != nil {
			return stepUpdated.Error
		}
		if stepUpdated.RowsAffected != 1 {
			return ErrAgentRuntimeLeaseLost
		}

		var steps []model.AgentRuntimeStep
		if err := tx.Where("run_id = ?", run.ID).Order("position asc, created_at asc").Find(&steps).Error; err != nil {
			return err
		}
		statusByID := make(map[string]model.AgentRuntimeStepStatus, len(steps))
		for _, item := range steps {
			statusByID[item.ID] = item.Status
		}
		combinedInputRefs, err := combineProductionArtifactRefs(step.InputArtifactRefsJSON, outputRefsJSON)
		if err != nil {
			return err
		}
		readyIDs := make([]string, 0)
		for _, item := range steps {
			if item.Status != model.AgentStepStatusPlanned {
				continue
			}
			var dependencies []string
			if err := json.Unmarshal([]byte(item.DependsOnStepIDsJSON), &dependencies); err != nil {
				return fmt.Errorf("decode Step %s dependencies: %w", item.ID, err)
			}
			unblocked := len(dependencies) > 0
			for _, dependencyID := range dependencies {
				status := statusByID[dependencyID]
				if status != model.AgentStepStatusCompleted && status != model.AgentStepStatusSkipped {
					unblocked = false
					break
				}
			}
			if !unblocked {
				continue
			}
			updated := tx.Model(&model.AgentRuntimeStep{}).
				Where("id = ? AND run_id = ? AND revision = ? AND status = ?", item.ID, run.ID, item.Revision, item.Status).
				Updates(map[string]any{
					"status": model.AgentStepStatusReady, "input_artifact_refs_json": combinedInputRefs,
					"revision": gorm.Expr("revision + ?", 1), "updated_at": now,
				})
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return ErrAgentRuntimeStateConflict
			}
			statusByID[item.ID] = model.AgentStepStatusReady
			readyIDs = append(readyIDs, item.ID)
		}

		nextRunStatus := model.AgentRunStatusCompleted
		currentStepID := step.ID
		unfinished := false
		for _, item := range steps {
			status := statusByID[item.ID]
			if status == model.AgentStepStatusRunning {
				nextRunStatus = model.AgentRunStatusRunning
				currentStepID = item.ID
				unfinished = true
				break
			}
			if status == model.AgentStepStatusReady && !unfinished {
				nextRunStatus = model.AgentRunStatusReady
				currentStepID = item.ID
				unfinished = true
			}
			if status == model.AgentStepStatusPlanned {
				unfinished = true
			}
		}
		if unfinished && nextRunStatus == model.AgentRunStatusCompleted {
			return errors.New("agent runtime Step dependency graph is blocked")
		}
		if !agentruntime.CanTransitionRun(run.Status, nextRunStatus) {
			return fmt.Errorf("%w: Run %s -> %s", ErrAgentRuntimeInvalidTransition, run.Status, nextRunStatus)
		}
		runUpdates := runTransitionUpdates(run, nextRunStatus, currentStepID, "", "", now)
		runUpdates["revision"] = gorm.Expr("revision + ?", 1)
		runUpdates["event_sequence"] = gorm.Expr("event_sequence + ?", 1)
		runUpdated := tx.Model(&model.AgentRuntimeRun{}).
			Where("id = ? AND revision = ? AND status = ?", run.ID, run.Revision, run.Status).
			Updates(runUpdates)
		if runUpdated.Error != nil {
			return runUpdated.Error
		}
		if runUpdated.RowsAffected != 1 {
			return ErrAgentRuntimeStateConflict
		}
		event := command.Event
		event.StepID = step.ID
		event.AttemptID = attempt.ID
		if strings.TrimSpace(event.PayloadJSON) == "" {
			event.PayloadJSON = mustRepositoryJSON(map[string]any{"outputArtifactRefs": refs, "readyStepIds": readyIDs})
		}
		if err := appendAgentRuntimeEvent(tx, run, event, run.EventSequence+1, step.ID, string(model.AgentStepStatusRunning), string(model.AgentStepStatusCompleted), now); err != nil {
			return err
		}
		if err := tx.First(&result.Run, "id = ?", run.ID).Error; err != nil {
			return err
		}
		if err := tx.First(&result.Step, "id = ?", step.ID).Error; err != nil {
			return err
		}
		if err := tx.First(&result.Attempt, "id = ?", attempt.ID).Error; err != nil {
			return err
		}
		if len(readyIDs) > 0 {
			if err := tx.Where("id IN ?", readyIDs).Order("position asc, created_at asc").Find(&result.ReadySteps).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *Repository) FailAgentRuntimeExecution(command AgentRuntimeExecutionFailCommand) (*AgentRuntimeExecutionClaim, error) {
	if strings.TrimSpace(command.Owner) == "" || strings.TrimSpace(command.UserID) == "" || strings.TrimSpace(command.RunID) == "" ||
		strings.TrimSpace(command.StepID) == "" || strings.TrimSpace(command.AttemptID) == "" || command.ExpectedRunRevision < 1 ||
		command.ExpectedStepRevision < 1 || command.ExpectedAttemptRevision < 1 || strings.TrimSpace(command.FailureCode) == "" {
		return nil, errors.New("agent runtime execution failure is incomplete")
	}
	if err := validateAgentRuntimeEventInput(command.Event); err != nil {
		return nil, err
	}
	now := runtimeCommandTime(command.At)
	result := &AgentRuntimeExecutionClaim{}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var run model.AgentRuntimeRun
		if err := lockAgentRuntimeRun(tx, command.UserID, command.RunID, &run); err != nil {
			return err
		}
		if run.Revision != command.ExpectedRunRevision || run.Status != model.AgentRunStatusRunning {
			return ErrAgentRuntimeStateConflict
		}
		var step model.AgentRuntimeStep
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&step, "id = ? AND run_id = ?", command.StepID, run.ID).Error; err != nil {
			return err
		}
		if step.Revision != command.ExpectedStepRevision || step.Status != model.AgentStepStatusRunning || step.LeaseOwner != command.Owner {
			return ErrAgentRuntimeLeaseLost
		}
		var attempt model.AgentRuntimeAttempt
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&attempt, "id = ? AND run_id = ? AND step_id = ?", command.AttemptID, run.ID, step.ID).Error; err != nil {
			return err
		}
		if attempt.Revision != command.ExpectedAttemptRevision || attempt.Status != model.AgentAttemptStatusRunning {
			return ErrAgentRuntimeStateConflict
		}
		attemptUpdated := tx.Model(&model.AgentRuntimeAttempt{}).
			Where("id = ? AND revision = ? AND status = ?", attempt.ID, attempt.Revision, attempt.Status).
			Updates(map[string]any{
				"status": model.AgentAttemptStatusFailed, "failure_code": command.FailureCode,
				"failure": strings.TrimSpace(command.Failure), "completed_at": now,
				"revision": gorm.Expr("revision + ?", 1), "updated_at": now,
			})
		if attemptUpdated.Error != nil {
			return attemptUpdated.Error
		}
		if attemptUpdated.RowsAffected != 1 {
			return ErrAgentRuntimeStateConflict
		}
		stepUpdated := tx.Model(&model.AgentRuntimeStep{}).
			Where("id = ? AND revision = ? AND status = ? AND lease_owner = ?", step.ID, step.Revision, step.Status, command.Owner).
			Updates(map[string]any{
				"status": model.AgentStepStatusFailed, "failure_code": command.FailureCode, "failure": strings.TrimSpace(command.Failure),
				"completed_at": now, "lease_owner": "", "lease_expires_at": nil,
				"revision": gorm.Expr("revision + ?", 1), "updated_at": now,
			})
		if stepUpdated.Error != nil {
			return stepUpdated.Error
		}
		if stepUpdated.RowsAffected != 1 {
			return ErrAgentRuntimeLeaseLost
		}
		runUpdates := runTransitionUpdates(run, model.AgentRunStatusFailed, step.ID, command.FailureCode, command.Failure, now)
		runUpdates["revision"] = gorm.Expr("revision + ?", 1)
		runUpdates["event_sequence"] = gorm.Expr("event_sequence + ?", 1)
		runUpdated := tx.Model(&model.AgentRuntimeRun{}).
			Where("id = ? AND revision = ? AND status = ?", run.ID, run.Revision, run.Status).
			Updates(runUpdates)
		if runUpdated.Error != nil {
			return runUpdated.Error
		}
		if runUpdated.RowsAffected != 1 {
			return ErrAgentRuntimeStateConflict
		}
		event := command.Event
		event.StepID = step.ID
		event.AttemptID = attempt.ID
		if err := appendAgentRuntimeEvent(tx, run, event, run.EventSequence+1, step.ID, string(model.AgentStepStatusRunning), string(model.AgentStepStatusFailed), now); err != nil {
			return err
		}
		if err := tx.First(&result.Run, "id = ?", run.ID).Error; err != nil {
			return err
		}
		if err := tx.First(&result.Step, "id = ?", step.ID).Error; err != nil {
			return err
		}
		return tx.First(&result.Attempt, "id = ?", attempt.ID).Error
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// TransitionAgentRuntimeRun applies a fenced state change and its audit event
// in one transaction. A stale caller cannot overwrite a newer Run revision.
func (r *Repository) TransitionAgentRuntimeRun(command AgentRuntimeRunTransition) (*model.AgentRuntimeRun, error) {
	if strings.TrimSpace(command.UserID) == "" || strings.TrimSpace(command.RunID) == "" || command.ExpectedRevision < 1 {
		return nil, errors.New("agent runtime Run transition identity is incomplete")
	}
	if err := validateAgentRuntimeEventInput(command.Event); err != nil {
		return nil, err
	}
	now := runtimeCommandTime(command.At)
	var result model.AgentRuntimeRun
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var run model.AgentRuntimeRun
		if err := lockAgentRuntimeRun(tx, command.UserID, command.RunID, &run); err != nil {
			return err
		}
		if run.Revision != command.ExpectedRevision {
			return ErrAgentRuntimeStateConflict
		}
		if !agentruntime.CanTransitionRun(run.Status, command.ToStatus) {
			return fmt.Errorf("%w: Run %s -> %s", ErrAgentRuntimeInvalidTransition, run.Status, command.ToStatus)
		}
		updates := runTransitionUpdates(run, command.ToStatus, command.CurrentStepID, command.FailureCode, command.Failure, now)
		updates["revision"] = gorm.Expr("revision + ?", 1)
		updates["event_sequence"] = gorm.Expr("event_sequence + ?", 1)
		updated := tx.Model(&model.AgentRuntimeRun{}).
			Where("id = ? AND user_id = ? AND revision = ? AND status = ?", run.ID, run.UserID, run.Revision, run.Status).
			Updates(updates)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrAgentRuntimeStateConflict
		}
		if err := appendAgentRuntimeEvent(tx, run, command.Event, run.EventSequence+1, "", string(run.Status), string(command.ToStatus), now); err != nil {
			return err
		}
		return tx.First(&result, "id = ?", run.ID).Error
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// TransitionAgentRuntimeStep changes a Step and optionally its Run status.
// The Run revision also advances because every Step fact appends a Run event.
func (r *Repository) TransitionAgentRuntimeStep(command AgentRuntimeStepTransition) (*model.AgentRuntimeRun, *model.AgentRuntimeStep, error) {
	if strings.TrimSpace(command.UserID) == "" || strings.TrimSpace(command.RunID) == "" || strings.TrimSpace(command.StepID) == "" ||
		command.ExpectedRunRevision < 1 || command.ExpectedStepRevision < 1 {
		return nil, nil, errors.New("agent runtime Step transition identity is incomplete")
	}
	if err := validateAgentRuntimeEventInput(command.Event); err != nil {
		return nil, nil, err
	}
	now := runtimeCommandTime(command.At)
	var resultRun model.AgentRuntimeRun
	var resultStep model.AgentRuntimeStep
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var run model.AgentRuntimeRun
		if err := lockAgentRuntimeRun(tx, command.UserID, command.RunID, &run); err != nil {
			return err
		}
		if run.Revision != command.ExpectedRunRevision || agentruntime.IsTerminalRun(run.Status) {
			return ErrAgentRuntimeStateConflict
		}
		var step model.AgentRuntimeStep
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&step, "id = ? AND run_id = ?", command.StepID, run.ID).Error; err != nil {
			return err
		}
		if step.Revision != command.ExpectedStepRevision {
			return ErrAgentRuntimeStateConflict
		}
		if !agentruntime.CanTransitionStep(step.Status, command.ToStatus) {
			return fmt.Errorf("%w: Step %s -> %s", ErrAgentRuntimeInvalidTransition, step.Status, command.ToStatus)
		}
		if command.RunStatus != nil && !agentruntime.CanTransitionRun(run.Status, *command.RunStatus) {
			return fmt.Errorf("%w: Run %s -> %s", ErrAgentRuntimeInvalidTransition, run.Status, *command.RunStatus)
		}
		stepUpdates := stepTransitionUpdates(step, command.ToStatus, command.FailureCode, command.Failure, now)
		stepUpdates["revision"] = gorm.Expr("revision + ?", 1)
		stepUpdated := tx.Model(&model.AgentRuntimeStep{}).
			Where("id = ? AND run_id = ? AND revision = ? AND status = ?", step.ID, run.ID, step.Revision, step.Status).
			Updates(stepUpdates)
		if stepUpdated.Error != nil {
			return stepUpdated.Error
		}
		if stepUpdated.RowsAffected != 1 {
			return ErrAgentRuntimeStateConflict
		}
		runUpdates := map[string]any{
			"current_step_id": step.ID,
			"revision":        gorm.Expr("revision + ?", 1),
			"event_sequence":  gorm.Expr("event_sequence + ?", 1),
			"updated_at":      now,
		}
		if command.RunStatus != nil {
			for key, value := range runTransitionUpdates(run, *command.RunStatus, step.ID, command.FailureCode, command.Failure, now) {
				runUpdates[key] = value
			}
		}
		runUpdated := tx.Model(&model.AgentRuntimeRun{}).
			Where("id = ? AND user_id = ? AND revision = ? AND status = ?", run.ID, run.UserID, run.Revision, run.Status).
			Updates(runUpdates)
		if runUpdated.Error != nil {
			return runUpdated.Error
		}
		if runUpdated.RowsAffected != 1 {
			return ErrAgentRuntimeStateConflict
		}
		if err := appendAgentRuntimeEvent(tx, run, command.Event, run.EventSequence+1, step.ID, string(step.Status), string(command.ToStatus), now); err != nil {
			return err
		}
		if err := tx.First(&resultRun, "id = ?", run.ID).Error; err != nil {
			return err
		}
		return tx.First(&resultStep, "id = ?", step.ID).Error
	})
	if err != nil {
		return nil, nil, err
	}
	return &resultRun, &resultStep, nil
}

// CreateAgentRuntimeAttempt allocates the next immutable Attempt number. A
// retry always creates a new row; no prior request or response is overwritten.
func (r *Repository) CreateAgentRuntimeAttempt(command AgentRuntimeAttemptCreate) (*model.AgentRuntimeRun, *model.AgentRuntimeStep, *model.AgentRuntimeAttempt, error) {
	if strings.TrimSpace(command.UserID) == "" || strings.TrimSpace(command.RunID) == "" || strings.TrimSpace(command.StepID) == "" ||
		command.ExpectedRunRevision < 1 || command.ExpectedStepRevision < 1 || command.Attempt == nil || strings.TrimSpace(command.Attempt.ID) == "" {
		return nil, nil, nil, errors.New("agent runtime Attempt creation identity is incomplete")
	}
	if err := validateAgentRuntimeEventInput(command.Event); err != nil {
		return nil, nil, nil, err
	}
	now := runtimeCommandTime(command.At)
	var resultRun model.AgentRuntimeRun
	var resultStep model.AgentRuntimeStep
	var resultAttempt model.AgentRuntimeAttempt
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var run model.AgentRuntimeRun
		if err := lockAgentRuntimeRun(tx, command.UserID, command.RunID, &run); err != nil {
			return err
		}
		if run.Revision != command.ExpectedRunRevision || agentruntime.IsTerminalRun(run.Status) {
			return ErrAgentRuntimeStateConflict
		}
		var step model.AgentRuntimeStep
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&step, "id = ? AND run_id = ?", command.StepID, run.ID).Error; err != nil {
			return err
		}
		if step.Revision != command.ExpectedStepRevision {
			return ErrAgentRuntimeStateConflict
		}
		if step.Status != model.AgentStepStatusReady && step.Status != model.AgentStepStatusFailed {
			return fmt.Errorf("%w: cannot create Attempt for Step in %s", ErrAgentRuntimeInvalidTransition, step.Status)
		}
		var activeAttempts int64
		if err := tx.Model(&model.AgentRuntimeAttempt{}).
			Where("step_id = ? AND status IN ?", step.ID, []model.AgentRuntimeAttemptStatus{model.AgentAttemptStatusQueued, model.AgentAttemptStatusRunning}).
			Count(&activeAttempts).Error; err != nil {
			return err
		}
		if activeAttempts != 0 {
			return ErrAgentRuntimeStateConflict
		}
		attempt := *command.Attempt
		attempt.RunID = run.ID
		attempt.StepID = step.ID
		attempt.Number = step.AttemptSequence + 1
		attempt.Status = model.AgentAttemptStatusQueued
		attempt.Revision = 1
		attempt.CreatedAt = now
		attempt.UpdatedAt = now
		attempt.StartedAt = nil
		attempt.CompletedAt = nil
		attempt.ResponseJSON = ""
		attempt.FailureCode = ""
		attempt.Failure = ""
		if err := tx.Create(&attempt).Error; err != nil {
			return err
		}
		stepUpdated := tx.Model(&model.AgentRuntimeStep{}).
			Where("id = ? AND run_id = ? AND revision = ? AND status = ?", step.ID, run.ID, step.Revision, step.Status).
			Updates(map[string]any{
				"attempt_sequence": attempt.Number,
				"status":           model.AgentStepStatusReady,
				"failure_code":     "",
				"failure":          "",
				"completed_at":     nil,
				"revision":         gorm.Expr("revision + ?", 1),
				"updated_at":       now,
			})
		if stepUpdated.Error != nil {
			return stepUpdated.Error
		}
		if stepUpdated.RowsAffected != 1 {
			return ErrAgentRuntimeStateConflict
		}
		runUpdates := map[string]any{
			"current_step_id": step.ID,
			"revision":        gorm.Expr("revision + ?", 1),
			"event_sequence":  gorm.Expr("event_sequence + ?", 1),
			"updated_at":      now,
		}
		if run.Status == model.AgentRunStatusFailed {
			for key, value := range runTransitionUpdates(run, model.AgentRunStatusReady, step.ID, "", "", now) {
				runUpdates[key] = value
			}
		}
		runUpdated := tx.Model(&model.AgentRuntimeRun{}).
			Where("id = ? AND user_id = ? AND revision = ? AND status = ?", run.ID, run.UserID, run.Revision, run.Status).
			Updates(runUpdates)
		if runUpdated.Error != nil {
			return runUpdated.Error
		}
		if runUpdated.RowsAffected != 1 {
			return ErrAgentRuntimeStateConflict
		}
		event := command.Event
		event.StepID = step.ID
		event.AttemptID = attempt.ID
		if err := appendAgentRuntimeEvent(tx, run, event, run.EventSequence+1, step.ID, "", string(attempt.Status), now); err != nil {
			return err
		}
		if err := tx.First(&resultRun, "id = ?", run.ID).Error; err != nil {
			return err
		}
		if err := tx.First(&resultStep, "id = ?", step.ID).Error; err != nil {
			return err
		}
		return tx.First(&resultAttempt, "id = ?", attempt.ID).Error
	})
	if err != nil {
		return nil, nil, nil, err
	}
	return &resultRun, &resultStep, &resultAttempt, nil
}

// TransitionAgentRuntimeAttempt updates one Attempt and, when requested, its
// owning Step and Run. All three optimistic fences and the Event share a
// transaction so provider callbacks cannot partially advance production.
func (r *Repository) TransitionAgentRuntimeAttempt(command AgentRuntimeAttemptTransition) (*model.AgentRuntimeRun, *model.AgentRuntimeStep, *model.AgentRuntimeAttempt, error) {
	if strings.TrimSpace(command.UserID) == "" || strings.TrimSpace(command.RunID) == "" || strings.TrimSpace(command.StepID) == "" || strings.TrimSpace(command.AttemptID) == "" ||
		command.ExpectedRunRevision < 1 || command.ExpectedAttemptRevision < 1 || (command.StepStatus != nil && command.ExpectedStepRevision < 1) {
		return nil, nil, nil, errors.New("agent runtime Attempt transition identity is incomplete")
	}
	if err := validateAgentRuntimeEventInput(command.Event); err != nil {
		return nil, nil, nil, err
	}
	now := runtimeCommandTime(command.At)
	var resultRun model.AgentRuntimeRun
	var resultStep model.AgentRuntimeStep
	var resultAttempt model.AgentRuntimeAttempt
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var run model.AgentRuntimeRun
		if err := lockAgentRuntimeRun(tx, command.UserID, command.RunID, &run); err != nil {
			return err
		}
		if run.Revision != command.ExpectedRunRevision || agentruntime.IsTerminalRun(run.Status) {
			return ErrAgentRuntimeStateConflict
		}
		var step model.AgentRuntimeStep
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&step, "id = ? AND run_id = ?", command.StepID, run.ID).Error; err != nil {
			return err
		}
		if command.StepStatus != nil && step.Revision != command.ExpectedStepRevision {
			return ErrAgentRuntimeStateConflict
		}
		var attempt model.AgentRuntimeAttempt
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&attempt, "id = ? AND run_id = ? AND step_id = ?", command.AttemptID, run.ID, step.ID).Error; err != nil {
			return err
		}
		if attempt.Revision != command.ExpectedAttemptRevision {
			return ErrAgentRuntimeStateConflict
		}
		if !agentruntime.CanTransitionAttempt(attempt.Status, command.ToStatus) {
			return fmt.Errorf("%w: Attempt %s -> %s", ErrAgentRuntimeInvalidTransition, attempt.Status, command.ToStatus)
		}
		if command.StepStatus != nil && !agentruntime.CanTransitionStep(step.Status, *command.StepStatus) {
			return fmt.Errorf("%w: Step %s -> %s", ErrAgentRuntimeInvalidTransition, step.Status, *command.StepStatus)
		}
		if command.RunStatus != nil && !agentruntime.CanTransitionRun(run.Status, *command.RunStatus) {
			return fmt.Errorf("%w: Run %s -> %s", ErrAgentRuntimeInvalidTransition, run.Status, *command.RunStatus)
		}
		attemptUpdates := attemptTransitionUpdates(attempt, command.ToStatus, command.ResponseJSON, command.FailureCode, command.Failure, now)
		attemptUpdates["revision"] = gorm.Expr("revision + ?", 1)
		attemptUpdated := tx.Model(&model.AgentRuntimeAttempt{}).
			Where("id = ? AND run_id = ? AND step_id = ? AND revision = ? AND status = ?", attempt.ID, run.ID, step.ID, attempt.Revision, attempt.Status).
			Updates(attemptUpdates)
		if attemptUpdated.Error != nil {
			return attemptUpdated.Error
		}
		if attemptUpdated.RowsAffected != 1 {
			return ErrAgentRuntimeStateConflict
		}
		if command.StepStatus != nil {
			stepUpdates := stepTransitionUpdates(step, *command.StepStatus, command.FailureCode, command.Failure, now)
			stepUpdates["revision"] = gorm.Expr("revision + ?", 1)
			stepUpdated := tx.Model(&model.AgentRuntimeStep{}).
				Where("id = ? AND run_id = ? AND revision = ? AND status = ?", step.ID, run.ID, step.Revision, step.Status).
				Updates(stepUpdates)
			if stepUpdated.Error != nil {
				return stepUpdated.Error
			}
			if stepUpdated.RowsAffected != 1 {
				return ErrAgentRuntimeStateConflict
			}
		}
		runUpdates := map[string]any{
			"current_step_id": step.ID,
			"revision":        gorm.Expr("revision + ?", 1),
			"event_sequence":  gorm.Expr("event_sequence + ?", 1),
			"updated_at":      now,
		}
		if command.RunStatus != nil {
			for key, value := range runTransitionUpdates(run, *command.RunStatus, step.ID, command.FailureCode, command.Failure, now) {
				runUpdates[key] = value
			}
		}
		runUpdated := tx.Model(&model.AgentRuntimeRun{}).
			Where("id = ? AND user_id = ? AND revision = ? AND status = ?", run.ID, run.UserID, run.Revision, run.Status).
			Updates(runUpdates)
		if runUpdated.Error != nil {
			return runUpdated.Error
		}
		if runUpdated.RowsAffected != 1 {
			return ErrAgentRuntimeStateConflict
		}
		event := command.Event
		event.StepID = step.ID
		event.AttemptID = attempt.ID
		if err := appendAgentRuntimeEvent(tx, run, event, run.EventSequence+1, step.ID, string(attempt.Status), string(command.ToStatus), now); err != nil {
			return err
		}
		if err := tx.First(&resultRun, "id = ?", run.ID).Error; err != nil {
			return err
		}
		if err := tx.First(&resultStep, "id = ?", step.ID).Error; err != nil {
			return err
		}
		return tx.First(&resultAttempt, "id = ?", attempt.ID).Error
	})
	if err != nil {
		return nil, nil, nil, err
	}
	return &resultRun, &resultStep, &resultAttempt, nil
}

// CreateProductionArtifactRevision creates a stable Artifact on version one or
// appends an immutable revision to an existing Artifact. Existing revision
// rows are never updated, including when a locked revision is superseded.
func (r *Repository) CreateProductionArtifactRevision(command ProductionArtifactRevisionCreate) (*model.ProductionArtifact, *model.ProductionArtifactRevision, error) {
	if err := validateProductionArtifactRevisionCreate(command); err != nil {
		return nil, nil, err
	}
	var resultArtifact model.ProductionArtifact
	var resultRevision model.ProductionArtifactRevision
	err := r.db.Transaction(func(tx *gorm.DB) error {
		artifact, revision, err := createProductionArtifactRevisionTx(tx, command)
		if err != nil {
			return err
		}
		resultArtifact = *artifact
		resultRevision = *revision
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return &resultArtifact, &resultRevision, nil
}

func createProductionArtifactRevisionTx(tx *gorm.DB, command ProductionArtifactRevisionCreate) (*model.ProductionArtifact, *model.ProductionArtifactRevision, error) {
	if err := validateProductionArtifactRevisionCreate(command); err != nil {
		return nil, nil, err
	}
	now := runtimeCommandTime(command.At)
	artifactInput := *command.Artifact
	revision := *command.Revision
	var existing model.ProductionArtifact
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&existing, "id = ?", artifactInput.ID).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		if command.ExpectedSequence != 0 {
			return nil, nil, ErrProductionArtifactConflict
		}
		revision.ArtifactID = artifactInput.ID
		revision.Version = 1
		revision.ParentRevisionID = ""
		revision.CreatedAt = now
		artifactInput.CurrentRevisionID = revision.ID
		artifactInput.RevisionSequence = 1
		artifactInput.CreatedAt = now
		artifactInput.UpdatedAt = now
		if err := tx.Create(&artifactInput).Error; err != nil {
			return nil, nil, err
		}
		if err := tx.Create(&revision).Error; err != nil {
			return nil, nil, err
		}
	case err != nil:
		return nil, nil, err
	default:
		if existing.UserID != command.UserID || existing.ProjectID != artifactInput.ProjectID || existing.Domain != artifactInput.Domain ||
			existing.ArtifactType != artifactInput.ArtifactType || existing.LogicalKey != artifactInput.LogicalKey {
			return nil, nil, gorm.ErrRecordNotFound
		}
		if existing.RevisionSequence != command.ExpectedSequence || existing.CurrentRevisionID == "" {
			return nil, nil, ErrProductionArtifactConflict
		}
		revision.ArtifactID = existing.ID
		revision.Version = existing.RevisionSequence + 1
		revision.ParentRevisionID = existing.CurrentRevisionID
		revision.CreatedAt = now
		updated := tx.Model(&model.ProductionArtifact{}).
			Where("id = ? AND user_id = ? AND revision_sequence = ? AND current_revision_id = ?", existing.ID, command.UserID, existing.RevisionSequence, existing.CurrentRevisionID).
			Updates(map[string]any{
				"current_revision_id": revision.ID,
				"revision_sequence":   gorm.Expr("revision_sequence + ?", 1),
				"updated_at":          now,
			})
		if updated.Error != nil {
			return nil, nil, updated.Error
		}
		if updated.RowsAffected != 1 {
			return nil, nil, ErrProductionArtifactConflict
		}
		if err := tx.Create(&revision).Error; err != nil {
			return nil, nil, err
		}
		artifactInput = existing
	}
	var resultArtifact model.ProductionArtifact
	var resultRevision model.ProductionArtifactRevision
	if err := tx.First(&resultArtifact, "id = ? AND user_id = ?", artifactInput.ID, command.UserID).Error; err != nil {
		return nil, nil, err
	}
	if err := tx.First(&resultRevision, "id = ? AND artifact_id = ?", revision.ID, artifactInput.ID).Error; err != nil {
		return nil, nil, err
	}
	return &resultArtifact, &resultRevision, nil
}

func (r *Repository) ProductionArtifactForUser(userID string, artifactID string) (*model.ProductionArtifact, error) {
	var artifact model.ProductionArtifact
	if err := r.db.First(&artifact, "id = ? AND user_id = ?", artifactID, userID).Error; err != nil {
		return nil, err
	}
	return &artifact, nil
}

func (r *Repository) ProductionArtifactByLogicalKey(userID string, projectID string, domain string, logicalKey string) (*model.ProductionArtifact, error) {
	var artifact model.ProductionArtifact
	if err := r.db.First(&artifact, "user_id = ? AND project_id = ? AND domain = ? AND logical_key = ?", userID, projectID, domain, logicalKey).Error; err != nil {
		return nil, err
	}
	return &artifact, nil
}

func (r *Repository) ProductionArtifactRevisionForUser(userID string, revisionID string) (*model.ProductionArtifact, *model.ProductionArtifactRevision, error) {
	var artifact model.ProductionArtifact
	var revision model.ProductionArtifactRevision
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&revision, "id = ?", revisionID).Error; err != nil {
			return err
		}
		return tx.First(&artifact, "id = ? AND user_id = ?", revision.ArtifactID, userID).Error
	})
	if err != nil {
		return nil, nil, err
	}
	return &artifact, &revision, nil
}

func (r *Repository) ProductionArtifactRevisionsForUser(userID string, artifactID string) ([]model.ProductionArtifactRevision, error) {
	if _, err := r.ProductionArtifactForUser(userID, artifactID); err != nil {
		return nil, err
	}
	var revisions []model.ProductionArtifactRevision
	err := r.db.Where("artifact_id = ?", artifactID).Order("version asc").Find(&revisions).Error
	return revisions, err
}

// PauseAgentRuntimeForHuman creates the pending decision and pauses the Run
// (and optional Step) atomically. At most one unresolved decision exists per
// Run, avoiding ambiguous resume targets after restart.
func (r *Repository) PauseAgentRuntimeForHuman(command AgentRuntimeHumanPause) (*model.AgentRuntimeRun, *model.AgentRuntimeStep, *model.AgentHumanDecision, error) {
	if strings.TrimSpace(command.UserID) == "" || strings.TrimSpace(command.RunID) == "" || command.ExpectedRunRevision < 1 ||
		command.Decision == nil || strings.TrimSpace(command.Decision.ID) == "" || strings.TrimSpace(command.Decision.Question) == "" ||
		(command.StepID != "" && command.ExpectedStepRevision < 1) {
		return nil, nil, nil, errors.New("agent runtime HumanDecision pause identity is incomplete")
	}
	if err := validateAgentRuntimeEventInput(command.Event); err != nil {
		return nil, nil, nil, err
	}
	now := runtimeCommandTime(command.At)
	var resultRun model.AgentRuntimeRun
	var resultStep model.AgentRuntimeStep
	var resultDecision model.AgentHumanDecision
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var run model.AgentRuntimeRun
		if err := lockAgentRuntimeRun(tx, command.UserID, command.RunID, &run); err != nil {
			return err
		}
		if run.Revision != command.ExpectedRunRevision || !agentruntime.CanTransitionRun(run.Status, model.AgentRunStatusAwaitingHuman) {
			return ErrAgentRuntimeStateConflict
		}
		var pending int64
		if err := tx.Model(&model.AgentHumanDecision{}).
			Where("run_id = ? AND status = ?", run.ID, model.AgentHumanDecisionStatusPending).
			Count(&pending).Error; err != nil {
			return err
		}
		if pending != 0 {
			return ErrAgentRuntimeActiveDecision
		}
		var step *model.AgentRuntimeStep
		if command.StepID != "" {
			var item model.AgentRuntimeStep
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, "id = ? AND run_id = ?", command.StepID, run.ID).Error; err != nil {
				return err
			}
			if item.Revision != command.ExpectedStepRevision || !agentruntime.CanTransitionStep(item.Status, model.AgentStepStatusAwaitingHuman) {
				return ErrAgentRuntimeStateConflict
			}
			step = &item
		}
		decision := *command.Decision
		decision.RunID = run.ID
		decision.StepID = command.StepID
		decision.Status = model.AgentHumanDecisionStatusPending
		decision.ResponseJSON = ""
		decision.Revision = 1
		decision.ResolvedByUserID = ""
		decision.ResolvedAt = nil
		decision.CreatedAt = now
		decision.UpdatedAt = now
		if err := tx.Create(&decision).Error; err != nil {
			return err
		}
		if step != nil {
			updated := tx.Model(&model.AgentRuntimeStep{}).
				Where("id = ? AND run_id = ? AND revision = ? AND status = ?", step.ID, run.ID, step.Revision, step.Status).
				Updates(map[string]any{
					"status":     model.AgentStepStatusAwaitingHuman,
					"revision":   gorm.Expr("revision + ?", 1),
					"updated_at": now,
				})
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return ErrAgentRuntimeStateConflict
			}
		}
		runUpdates := runTransitionUpdates(run, model.AgentRunStatusAwaitingHuman, command.StepID, "", "", now)
		runUpdates["revision"] = gorm.Expr("revision + ?", 1)
		runUpdates["event_sequence"] = gorm.Expr("event_sequence + ?", 1)
		updated := tx.Model(&model.AgentRuntimeRun{}).
			Where("id = ? AND user_id = ? AND revision = ? AND status = ?", run.ID, run.UserID, run.Revision, run.Status).
			Updates(runUpdates)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrAgentRuntimeStateConflict
		}
		event := command.Event
		event.StepID = command.StepID
		fromStatus := string(run.Status)
		if step != nil {
			fromStatus = string(step.Status)
		}
		if err := appendAgentRuntimeEvent(tx, run, event, run.EventSequence+1, command.StepID, fromStatus, string(model.AgentRunStatusAwaitingHuman), now); err != nil {
			return err
		}
		if err := tx.First(&resultRun, "id = ?", run.ID).Error; err != nil {
			return err
		}
		if step != nil {
			if err := tx.First(&resultStep, "id = ?", step.ID).Error; err != nil {
				return err
			}
		}
		return tx.First(&resultDecision, "id = ?", decision.ID).Error
	})
	if err != nil {
		return nil, nil, nil, err
	}
	var stepResult *model.AgentRuntimeStep
	if resultStep.ID != "" {
		stepResult = &resultStep
	}
	return &resultRun, stepResult, &resultDecision, nil
}

// ResolveAgentRuntimeHumanDecision records the user's answer before resuming
// the paused Run and Step. Replaying the same stale decision revision fails.
func (r *Repository) ResolveAgentRuntimeHumanDecision(command AgentRuntimeHumanResolve) (*model.AgentRuntimeRun, *model.AgentRuntimeStep, *model.AgentHumanDecision, error) {
	if strings.TrimSpace(command.UserID) == "" || strings.TrimSpace(command.RunID) == "" || strings.TrimSpace(command.DecisionID) == "" ||
		command.ExpectedRunRevision < 1 || command.ExpectedDecisionRevision < 1 || strings.TrimSpace(command.ResponseJSON) == "" {
		return nil, nil, nil, errors.New("agent runtime HumanDecision resolution identity is incomplete")
	}
	if err := validateAgentRuntimeEventInput(command.Event); err != nil {
		return nil, nil, nil, err
	}
	now := runtimeCommandTime(command.At)
	var resultRun model.AgentRuntimeRun
	var resultStep model.AgentRuntimeStep
	var resultDecision model.AgentHumanDecision
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var run model.AgentRuntimeRun
		if err := lockAgentRuntimeRun(tx, command.UserID, command.RunID, &run); err != nil {
			return err
		}
		if run.Revision != command.ExpectedRunRevision || run.Status != model.AgentRunStatusAwaitingHuman ||
			!agentruntime.CanTransitionRun(run.Status, command.RunStatus) {
			return ErrAgentRuntimeStateConflict
		}
		var decision model.AgentHumanDecision
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&decision, "id = ? AND run_id = ?", command.DecisionID, run.ID).Error; err != nil {
			return err
		}
		if decision.Revision != command.ExpectedDecisionRevision || decision.Status != model.AgentHumanDecisionStatusPending {
			return ErrAgentRuntimeStateConflict
		}
		var step *model.AgentRuntimeStep
		if decision.StepID != "" {
			if command.ExpectedStepRevision < 1 {
				return errors.New("agent runtime HumanDecision resolution Step revision is missing")
			}
			var item model.AgentRuntimeStep
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, "id = ? AND run_id = ?", decision.StepID, run.ID).Error; err != nil {
				return err
			}
			if item.Revision != command.ExpectedStepRevision || item.Status != model.AgentStepStatusAwaitingHuman ||
				!agentruntime.CanTransitionStep(item.Status, command.StepStatus) {
				return ErrAgentRuntimeStateConflict
			}
			step = &item
		}
		resolvedAt := now
		decisionUpdated := tx.Model(&model.AgentHumanDecision{}).
			Where("id = ? AND run_id = ? AND revision = ? AND status = ?", decision.ID, run.ID, decision.Revision, decision.Status).
			Updates(map[string]any{
				"status":              model.AgentHumanDecisionStatusResolved,
				"response_json":       command.ResponseJSON,
				"resolved_by_user_id": command.UserID,
				"resolved_at":         &resolvedAt,
				"revision":            gorm.Expr("revision + ?", 1),
				"updated_at":          now,
			})
		if decisionUpdated.Error != nil {
			return decisionUpdated.Error
		}
		if decisionUpdated.RowsAffected != 1 {
			return ErrAgentRuntimeStateConflict
		}
		if step != nil {
			stepUpdates := stepTransitionUpdates(*step, command.StepStatus, "", "", now)
			stepUpdates["revision"] = gorm.Expr("revision + ?", 1)
			stepUpdated := tx.Model(&model.AgentRuntimeStep{}).
				Where("id = ? AND run_id = ? AND revision = ? AND status = ?", step.ID, run.ID, step.Revision, step.Status).
				Updates(stepUpdates)
			if stepUpdated.Error != nil {
				return stepUpdated.Error
			}
			if stepUpdated.RowsAffected != 1 {
				return ErrAgentRuntimeStateConflict
			}
		}
		runUpdates := runTransitionUpdates(run, command.RunStatus, decision.StepID, "", "", now)
		runUpdates["revision"] = gorm.Expr("revision + ?", 1)
		runUpdates["event_sequence"] = gorm.Expr("event_sequence + ?", 1)
		runUpdated := tx.Model(&model.AgentRuntimeRun{}).
			Where("id = ? AND user_id = ? AND revision = ? AND status = ?", run.ID, run.UserID, run.Revision, run.Status).
			Updates(runUpdates)
		if runUpdated.Error != nil {
			return runUpdated.Error
		}
		if runUpdated.RowsAffected != 1 {
			return ErrAgentRuntimeStateConflict
		}
		event := command.Event
		event.StepID = decision.StepID
		if err := appendAgentRuntimeEvent(tx, run, event, run.EventSequence+1, decision.StepID, string(model.AgentRunStatusAwaitingHuman), string(command.RunStatus), now); err != nil {
			return err
		}
		if err := tx.First(&resultRun, "id = ?", run.ID).Error; err != nil {
			return err
		}
		if step != nil {
			if err := tx.First(&resultStep, "id = ?", step.ID).Error; err != nil {
				return err
			}
		}
		return tx.First(&resultDecision, "id = ?", decision.ID).Error
	})
	if err != nil {
		return nil, nil, nil, err
	}
	var stepResult *model.AgentRuntimeStep
	if resultStep.ID != "" {
		stepResult = &resultStep
	}
	return &resultRun, stepResult, &resultDecision, nil
}

type productionArtifactRef struct {
	ArtifactID string `json:"artifactId"`
	RevisionID string `json:"revisionId"`
	Type       string `json:"type"`
	Version    int    `json:"version"`
	Digest     string `json:"digest"`
	Status     string `json:"status"`
}

func combineProductionArtifactRefs(leftJSON string, rightJSON string) (string, error) {
	combined := make([]productionArtifactRef, 0)
	seen := make(map[string]struct{})
	for _, raw := range []string{leftJSON, rightJSON} {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var refs []productionArtifactRef
		if err := json.Unmarshal([]byte(raw), &refs); err != nil {
			return "", err
		}
		for _, ref := range refs {
			if strings.TrimSpace(ref.RevisionID) == "" {
				return "", errors.New("production Artifact reference has no revision ID")
			}
			if _, duplicate := seen[ref.RevisionID]; duplicate {
				continue
			}
			seen[ref.RevisionID] = struct{}{}
			combined = append(combined, ref)
		}
	}
	return mustRepositoryJSON(combined), nil
}

func mustRepositoryJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func lockAgentRuntimeRun(tx *gorm.DB, userID string, runID string, run *model.AgentRuntimeRun) error {
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(run, "id = ? AND user_id = ?", runID, userID).Error
}

func appendAgentRuntimeEvent(tx *gorm.DB, run model.AgentRuntimeRun, input AgentRuntimeEventInput, sequence int64, fallbackStepID string, fromStatus string, toStatus string, at time.Time) error {
	stepID := strings.TrimSpace(input.StepID)
	if stepID == "" {
		stepID = fallbackStepID
	}
	payload := strings.TrimSpace(input.PayloadJSON)
	if payload == "" {
		payload = "{}"
	}
	event := model.AgentRuntimeEvent{
		ID: input.ID, UserID: run.UserID, RunID: run.ID, Sequence: sequence,
		StepID: stepID, AttemptID: strings.TrimSpace(input.AttemptID), EventType: strings.TrimSpace(input.EventType),
		ActorType: strings.TrimSpace(input.ActorType), ActorID: strings.TrimSpace(input.ActorID),
		FromStatus: fromStatus, ToStatus: toStatus, PayloadJSON: payload, CreatedAt: at,
	}
	return tx.Create(&event).Error
}

func validateAgentRuntimeEventInput(input AgentRuntimeEventInput) error {
	if strings.TrimSpace(input.ID) == "" || strings.TrimSpace(input.EventType) == "" || strings.TrimSpace(input.ActorType) == "" || strings.TrimSpace(input.ActorID) == "" {
		return errors.New("agent runtime Event identity is incomplete")
	}
	return nil
}

func validateProductionArtifactRevisionCreate(command ProductionArtifactRevisionCreate) error {
	if strings.TrimSpace(command.UserID) == "" || command.Artifact == nil || command.Revision == nil || command.ExpectedSequence < 0 {
		return errors.New("production Artifact revision identity is incomplete")
	}
	artifact := command.Artifact
	revision := command.Revision
	if strings.TrimSpace(artifact.ID) == "" || artifact.UserID != command.UserID || strings.TrimSpace(artifact.ProjectID) == "" ||
		strings.TrimSpace(artifact.Domain) == "" || strings.TrimSpace(artifact.ArtifactType) == "" || strings.TrimSpace(artifact.LogicalKey) == "" ||
		strings.TrimSpace(revision.ID) == "" || (strings.TrimSpace(revision.ContentJSON) == "" && strings.TrimSpace(revision.ContentText) == "") ||
		strings.TrimSpace(revision.ContentDigest) == "" || strings.TrimSpace(revision.CreatedByType) == "" || strings.TrimSpace(revision.CreatedByID) == "" {
		return errors.New("production Artifact revision identity or content is incomplete")
	}
	switch revision.Status {
	case model.ProductionArtifactStatusDraft, model.ProductionArtifactStatusReview, model.ProductionArtifactStatusLocked, model.ProductionArtifactStatusArchived:
		return nil
	default:
		return fmt.Errorf("%w: unsupported initial Artifact revision status %s", ErrAgentRuntimeInvalidTransition, revision.Status)
	}
}

func runtimeCommandTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value.UTC()
}

func runTransitionUpdates(run model.AgentRuntimeRun, next model.AgentRuntimeRunStatus, currentStepID string, failureCode string, failure string, at time.Time) map[string]any {
	updates := map[string]any{"status": next, "updated_at": at}
	if currentStepID != "" {
		updates["current_step_id"] = currentStepID
	}
	if next == model.AgentRunStatusRunning && run.StartedAt == nil {
		updates["started_at"] = at
	}
	if next == model.AgentRunStatusCompleted || next == model.AgentRunStatusCancelled {
		updates["completed_at"] = at
	} else if run.CompletedAt != nil {
		updates["completed_at"] = nil
	}
	if next == model.AgentRunStatusFailed {
		updates["failure_code"] = strings.TrimSpace(failureCode)
		updates["failure"] = strings.TrimSpace(failure)
	} else {
		updates["failure_code"] = ""
		updates["failure"] = ""
	}
	return updates
}

func stepTransitionUpdates(step model.AgentRuntimeStep, next model.AgentRuntimeStepStatus, failureCode string, failure string, at time.Time) map[string]any {
	updates := map[string]any{"status": next, "updated_at": at}
	if next == model.AgentStepStatusRunning && step.StartedAt == nil {
		updates["started_at"] = at
	}
	if agentruntime.IsTerminalStep(next) || next == model.AgentStepStatusFailed {
		updates["completed_at"] = at
	} else if step.CompletedAt != nil {
		updates["completed_at"] = nil
	}
	if next == model.AgentStepStatusFailed {
		updates["failure_code"] = strings.TrimSpace(failureCode)
		updates["failure"] = strings.TrimSpace(failure)
	} else {
		updates["failure_code"] = ""
		updates["failure"] = ""
	}
	return updates
}

func attemptTransitionUpdates(attempt model.AgentRuntimeAttempt, next model.AgentRuntimeAttemptStatus, responseJSON string, failureCode string, failure string, at time.Time) map[string]any {
	updates := map[string]any{"status": next, "updated_at": at}
	if next == model.AgentAttemptStatusRunning && attempt.StartedAt == nil {
		updates["started_at"] = at
	}
	if agentruntime.IsTerminalAttempt(next) {
		updates["completed_at"] = at
	}
	if next == model.AgentAttemptStatusSucceeded {
		updates["response_json"] = responseJSON
		updates["failure_code"] = ""
		updates["failure"] = ""
	} else if next == model.AgentAttemptStatusFailed {
		updates["failure_code"] = strings.TrimSpace(failureCode)
		updates["failure"] = strings.TrimSpace(failure)
	}
	return updates
}

func validateAgentRuntimeCreateBundle(bundle AgentRuntimeCreateBundle) error {
	if bundle.Run == nil || bundle.RoutingDecision == nil {
		return errors.New("agent runtime bundle requires a Run and RoutingDecision")
	}
	runID := bundle.Run.ID
	if runID == "" || bundle.Run.UserID == "" || bundle.Run.ProjectID == "" || bundle.Run.IdempotencyKey == "" {
		return errors.New("agent runtime Run identity is incomplete")
	}
	if bundle.RoutingDecision.RunID != runID || bundle.RoutingDecision.ID == "" {
		return errors.New("routing decision is not bound to the Run")
	}
	if len(bundle.Steps) == 0 {
		return errors.New("agent runtime bundle requires at least one Step")
	}
	stepIDs := make(map[string]struct{}, len(bundle.Steps))
	stepKeys := make(map[string]struct{}, len(bundle.Steps))
	for _, step := range bundle.Steps {
		if step.ID == "" || step.RunID != runID || step.StepKey == "" {
			return errors.New("agent runtime Step identity is incomplete")
		}
		if _, duplicate := stepIDs[step.ID]; duplicate {
			return fmt.Errorf("duplicate agent runtime Step ID %s", step.ID)
		}
		if _, duplicate := stepKeys[step.StepKey]; duplicate {
			return fmt.Errorf("duplicate agent runtime Step key %s", step.StepKey)
		}
		stepIDs[step.ID] = struct{}{}
		stepKeys[step.StepKey] = struct{}{}
	}
	if bundle.HumanDecision != nil {
		if bundle.HumanDecision.ID == "" || bundle.HumanDecision.RunID != runID {
			return errors.New("HumanDecision is not bound to the Run")
		}
		if bundle.HumanDecision.StepID != "" {
			if _, ok := stepIDs[bundle.HumanDecision.StepID]; !ok {
				return errors.New("HumanDecision references a Step outside the Run")
			}
		}
	}
	if len(bundle.Events) == 0 || int64(len(bundle.Events)) != bundle.Run.EventSequence {
		return errors.New("agent runtime initial Event sequence is incomplete")
	}
	for index, event := range bundle.Events {
		if event.ID == "" || event.RunID != runID || event.UserID != bundle.Run.UserID || event.Sequence != int64(index+1) {
			return errors.New("agent runtime initial Events are not contiguous")
		}
		if event.StepID != "" {
			if _, ok := stepIDs[event.StepID]; !ok {
				return errors.New("agent runtime Event references a Step outside the Run")
			}
		}
	}
	artifactIDs := make(map[string]model.ProductionArtifact, len(bundle.Artifacts))
	for _, artifact := range bundle.Artifacts {
		if artifact.ID == "" || artifact.UserID != bundle.Run.UserID || artifact.ProjectID != bundle.Run.ProjectID || artifact.Domain != bundle.Run.Domain || artifact.RevisionSequence != 1 || artifact.CurrentRevisionID == "" {
			return errors.New("agent runtime initial Artifact identity is incomplete")
		}
		if _, duplicate := artifactIDs[artifact.ID]; duplicate {
			return fmt.Errorf("duplicate agent runtime Artifact ID %s", artifact.ID)
		}
		artifactIDs[artifact.ID] = artifact
	}
	if len(bundle.ArtifactRevisions) != len(bundle.Artifacts) {
		return errors.New("agent runtime initial Artifact revisions are incomplete")
	}
	for _, revision := range bundle.ArtifactRevisions {
		artifact, ok := artifactIDs[revision.ArtifactID]
		if !ok || revision.ID != artifact.CurrentRevisionID || revision.Version != 1 || revision.SourceRunID != runID {
			return errors.New("agent runtime initial Artifact revision is not bound to the Run")
		}
	}
	return nil
}
