package repository

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAgentRuntimeBundlePersistsAcrossRepositoryRestart(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "agent-runtime.db")
	db := openAgentRuntimeTestDB(t, databasePath)
	repo := New(db)
	now := time.Now().UTC()
	createAgentRuntimeTestProject(t, db, "project-1", "user-1")
	run := model.AgentRuntimeRun{
		ID: "run-1", UserID: "user-1", ProjectID: "project-1", Domain: "film",
		RegistryID: "film-agent-team", RegistryVersion: "1.3.1", IntentRouteID: "IR-01",
		Status: model.AgentRunStatusReady, Objective: "write a short film", InputJSON: "{}",
		CurrentStepID: "step-1", IdempotencyKey: "request-1", Revision: 1, EventSequence: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	step := model.AgentRuntimeStep{
		ID: "step-1", RunID: run.ID, StepKey: "intent:IR-01", RouteKind: "intent",
		RouteID: "IR-01", AgentID: "narrative_screenwriter", SkillIDsJSON: `["screenwriter"]`,
		Status: model.AgentStepStatusReady, DependsOnStepIDsJSON: "[]",
		InputArtifactRefsJSON: "[]", OutputArtifactRefsJSON: "[]", Revision: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	bundle := AgentRuntimeCreateBundle{
		Run: &run,
		RoutingDecision: &model.AgentRoutingDecision{
			ID: "route-1", RunID: run.ID, IntentRouteID: "IR-01",
			SelectedAgentID: step.AgentID, SelectedSkillIDsJSON: step.SkillIDsJSON,
			InputArtifactRefsJSON: "[]", AlternativesJSON: "[]", Reason: "explicit route",
			Confidence: "confirmed", DecidedByType: "user", DecidedByID: run.UserID, CreatedAt: now,
		},
		Steps: []model.AgentRuntimeStep{step},
		Events: []model.AgentRuntimeEvent{{
			ID: "event-1", UserID: run.UserID, RunID: run.ID, Sequence: 1,
			EventType: "run.created", ActorType: "user", ActorID: run.UserID,
			ToStatus: string(run.Status), PayloadJSON: "{}", CreatedAt: now,
		}},
	}
	if err := repo.CreateAgentRuntimeBundle(bundle); err != nil {
		t.Fatalf("CreateAgentRuntimeBundle() error = %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB(): %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	reopened := openAgentRuntimeTestDB(t, databasePath)
	detail, err := New(reopened).AgentRuntimeDetailForUser(run.UserID, run.ID)
	if err != nil {
		t.Fatalf("AgentRuntimeDetailForUser() error = %v", err)
	}
	if detail.Run.IdempotencyKey != run.IdempotencyKey || len(detail.Steps) != 1 ||
		len(detail.RoutingDecisions) != 1 || len(detail.Events) != 1 {
		t.Fatalf("incomplete restored detail: %#v", detail)
	}
	if detail.Events[0].Sequence != 1 || detail.Steps[0].SkillIDsJSON != `["screenwriter"]` {
		t.Fatalf("restored evidence changed: %#v", detail)
	}
}

func TestAgentRuntimeBundleIsAtomicAndIdempotencyIsUnique(t *testing.T) {
	db := openAgentRuntimeTestDB(t, filepath.Join(t.TempDir(), "agent-runtime.db"))
	repo := New(db)
	now := time.Now().UTC()
	createAgentRuntimeTestProject(t, db, "project-1", "user-1")
	run := model.AgentRuntimeRun{
		ID: "run-invalid", UserID: "user-1", ProjectID: "project-1", Domain: "film",
		RegistryID: "film-agent-team", RegistryVersion: "1.3.1", IntentRouteID: "IR-01",
		Status: model.AgentRunStatusReady, Objective: "objective", InputJSON: "{}",
		CurrentStepID: "step-invalid", IdempotencyKey: "same-request", Revision: 1,
		EventSequence: 1, CreatedAt: now, UpdatedAt: now,
	}
	invalid := AgentRuntimeCreateBundle{
		Run:             &run,
		RoutingDecision: &model.AgentRoutingDecision{ID: "route-invalid", RunID: run.ID},
		Steps:           []model.AgentRuntimeStep{{ID: "step-invalid", RunID: "another-run", StepKey: "intent:IR-01"}},
		Events:          []model.AgentRuntimeEvent{{ID: "event-invalid", UserID: run.UserID, RunID: run.ID, Sequence: 1}},
	}
	if err := repo.CreateAgentRuntimeBundle(invalid); err == nil {
		t.Fatal("invalid bundle unexpectedly persisted")
	}
	if _, err := repo.AgentRuntimeRunForUser(run.UserID, run.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("invalid bundle left a Run behind: %v", err)
	}

	first := agentRuntimeTestBundle("run-first", "same-request", now)
	if err := repo.CreateAgentRuntimeBundle(first); err != nil {
		t.Fatalf("create first idempotent bundle: %v", err)
	}
	duplicate := agentRuntimeTestBundle("run-duplicate", "same-request", now.Add(time.Second))
	if err := repo.CreateAgentRuntimeBundle(duplicate); err == nil {
		t.Fatal("duplicate user idempotency key unexpectedly persisted")
	}
	if _, err := repo.AgentRuntimeRunForUser(run.UserID, duplicate.Run.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("duplicate bundle left a Run behind: %v", err)
	}
	var childRows int64
	if err := db.Model(&model.AgentRuntimeStep{}).Where("run_id = ?", duplicate.Run.ID).Count(&childRows).Error; err != nil {
		t.Fatalf("count duplicate children: %v", err)
	}
	if childRows != 0 {
		t.Fatalf("duplicate bundle left %d Step rows behind", childRows)
	}
}

func TestAgentRuntimeTransitionsFenceStaleRevisionsAndAppendEvents(t *testing.T) {
	db := openAgentRuntimeTestDB(t, filepath.Join(t.TempDir(), "agent-runtime.db"))
	repo := New(db)
	now := time.Now().UTC()
	bundle := agentRuntimeTestBundle("run-transition", "transition-request", now)
	createAgentRuntimeTestProject(t, db, bundle.Run.ProjectID, bundle.Run.UserID)
	if err := repo.CreateAgentRuntimeBundle(bundle); err != nil {
		t.Fatalf("create bundle: %v", err)
	}

	run, err := repo.TransitionAgentRuntimeRun(AgentRuntimeRunTransition{
		UserID: bundle.Run.UserID, RunID: bundle.Run.ID, ExpectedRevision: 1,
		ToStatus: model.AgentRunStatusRunning,
		Event:    AgentRuntimeEventInput{ID: "event-transition-2", EventType: "run.started", ActorType: "runtime", ActorID: "orchestrator"},
		At:       now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("TransitionAgentRuntimeRun(): %v", err)
	}
	if run.Revision != 2 || run.EventSequence != 2 || run.StartedAt == nil {
		t.Fatalf("Run did not advance atomically: %#v", run)
	}
	if _, err := repo.TransitionAgentRuntimeRun(AgentRuntimeRunTransition{
		UserID: bundle.Run.UserID, RunID: bundle.Run.ID, ExpectedRevision: 1,
		ToStatus: model.AgentRunStatusFailed,
		Event:    AgentRuntimeEventInput{ID: "event-stale", EventType: "run.failed", ActorType: "runtime", ActorID: "orchestrator"},
	}); !errors.Is(err, ErrAgentRuntimeStateConflict) {
		t.Fatalf("stale Run transition error = %v", err)
	}

	running := model.AgentRunStatusRunning
	run, step, err := repo.TransitionAgentRuntimeStep(AgentRuntimeStepTransition{
		UserID: bundle.Run.UserID, RunID: bundle.Run.ID, StepID: bundle.Steps[0].ID,
		ExpectedRunRevision: 2, ExpectedStepRevision: 1,
		ToStatus: model.AgentStepStatusRunning, RunStatus: &running,
		Event: AgentRuntimeEventInput{ID: "event-transition-3", EventType: "step.started", ActorType: "runtime", ActorID: "orchestrator"},
		At:    now.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("TransitionAgentRuntimeStep(): %v", err)
	}
	if run.Revision != 3 || run.EventSequence != 3 || step.Revision != 2 || step.Status != model.AgentStepStatusRunning {
		t.Fatalf("Step transition did not advance atomically: run=%#v step=%#v", run, step)
	}
	if _, _, err := repo.TransitionAgentRuntimeStep(AgentRuntimeStepTransition{
		UserID: bundle.Run.UserID, RunID: bundle.Run.ID, StepID: step.ID,
		ExpectedRunRevision: 3, ExpectedStepRevision: 1, ToStatus: model.AgentStepStatusCompleted,
		Event: AgentRuntimeEventInput{ID: "event-step-stale", EventType: "step.completed", ActorType: "runtime", ActorID: "orchestrator"},
	}); !errors.Is(err, ErrAgentRuntimeStateConflict) {
		t.Fatalf("stale Step transition error = %v", err)
	}
	detail, err := repo.AgentRuntimeDetailForUser(bundle.Run.UserID, bundle.Run.ID)
	if err != nil {
		t.Fatalf("load detail: %v", err)
	}
	if len(detail.Events) != 3 || detail.Events[0].Sequence != 1 || detail.Events[1].Sequence != 2 || detail.Events[2].Sequence != 3 {
		t.Fatalf("Event history is not contiguous: %#v", detail.Events)
	}
}

func TestAgentRuntimeAttemptsRemainAppendOnlyAcrossRetry(t *testing.T) {
	db := openAgentRuntimeTestDB(t, filepath.Join(t.TempDir(), "agent-runtime.db"))
	repo := New(db)
	now := time.Now().UTC()
	bundle := agentRuntimeTestBundle("run-attempt", "attempt-request", now)
	createAgentRuntimeTestProject(t, db, bundle.Run.ProjectID, bundle.Run.UserID)
	if err := repo.CreateAgentRuntimeBundle(bundle); err != nil {
		t.Fatalf("create bundle: %v", err)
	}

	run, step, first, err := repo.CreateAgentRuntimeAttempt(AgentRuntimeAttemptCreate{
		UserID: bundle.Run.UserID, RunID: bundle.Run.ID, StepID: bundle.Steps[0].ID,
		ExpectedRunRevision: 1, ExpectedStepRevision: 1,
		Attempt: &model.AgentRuntimeAttempt{ID: "attempt-1", Executor: "skill", InputDigest: "input-1", PromptDigest: "prompt-1", RequestJSON: `{"prompt":"first"}`},
		Event:   AgentRuntimeEventInput{ID: "event-attempt-2", EventType: "attempt.created", ActorType: "runtime", ActorID: "orchestrator"},
		At:      now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("CreateAgentRuntimeAttempt(first): %v", err)
	}
	if first.Number != 1 || first.Revision != 1 || step.AttemptSequence != 1 {
		t.Fatalf("first Attempt allocation is invalid: step=%#v attempt=%#v", step, first)
	}
	runningStep := model.AgentStepStatusRunning
	runningRun := model.AgentRunStatusRunning
	run, step, first, err = repo.TransitionAgentRuntimeAttempt(AgentRuntimeAttemptTransition{
		UserID: bundle.Run.UserID, RunID: run.ID, StepID: step.ID, AttemptID: first.ID,
		ExpectedRunRevision: run.Revision, ExpectedStepRevision: step.Revision, ExpectedAttemptRevision: first.Revision,
		ToStatus: model.AgentAttemptStatusRunning, StepStatus: &runningStep, RunStatus: &runningRun,
		Event: AgentRuntimeEventInput{ID: "event-attempt-3", EventType: "attempt.started", ActorType: "runtime", ActorID: "orchestrator"},
		At:    now.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("start Attempt: %v", err)
	}
	failedStep := model.AgentStepStatusFailed
	failedRun := model.AgentRunStatusFailed
	run, step, first, err = repo.TransitionAgentRuntimeAttempt(AgentRuntimeAttemptTransition{
		UserID: bundle.Run.UserID, RunID: run.ID, StepID: step.ID, AttemptID: first.ID,
		ExpectedRunRevision: run.Revision, ExpectedStepRevision: step.Revision, ExpectedAttemptRevision: first.Revision,
		ToStatus: model.AgentAttemptStatusFailed, StepStatus: &failedStep, RunStatus: &failedRun,
		FailureCode: "provider_timeout", Failure: "provider timed out",
		Event: AgentRuntimeEventInput{ID: "event-attempt-4", EventType: "attempt.failed", ActorType: "runtime", ActorID: "orchestrator"},
		At:    now.Add(3 * time.Second),
	})
	if err != nil {
		t.Fatalf("fail Attempt: %v", err)
	}
	if first.Status != model.AgentAttemptStatusFailed || first.Revision != 3 || step.Status != model.AgentStepStatusFailed || run.Status != model.AgentRunStatusFailed {
		t.Fatalf("failed Attempt was not recorded: run=%#v step=%#v attempt=%#v", run, step, first)
	}
	run, step, second, err := repo.CreateAgentRuntimeAttempt(AgentRuntimeAttemptCreate{
		UserID: bundle.Run.UserID, RunID: run.ID, StepID: step.ID,
		ExpectedRunRevision: run.Revision, ExpectedStepRevision: step.Revision,
		Attempt: &model.AgentRuntimeAttempt{ID: "attempt-2", Executor: "skill", InputDigest: "input-2", PromptDigest: "prompt-2", RequestJSON: `{"prompt":"retry"}`},
		Event:   AgentRuntimeEventInput{ID: "event-attempt-5", EventType: "attempt.created", ActorType: "user", ActorID: bundle.Run.UserID},
		At:      now.Add(4 * time.Second),
	})
	if err != nil {
		t.Fatalf("CreateAgentRuntimeAttempt(retry): %v", err)
	}
	if second.Number != 2 || second.Status != model.AgentAttemptStatusQueued || step.AttemptSequence != 2 || step.Status != model.AgentStepStatusReady || run.Status != model.AgentRunStatusReady {
		t.Fatalf("retry did not create a new ready Attempt: run=%#v step=%#v attempt=%#v", run, step, second)
	}
	var persistedFirst model.AgentRuntimeAttempt
	if err := db.First(&persistedFirst, "id = ?", first.ID).Error; err != nil {
		t.Fatalf("load first Attempt: %v", err)
	}
	if persistedFirst.Status != model.AgentAttemptStatusFailed || persistedFirst.RequestJSON != `{"prompt":"first"}` || persistedFirst.Number != 1 {
		t.Fatalf("retry mutated prior Attempt: %#v", persistedFirst)
	}
}

func TestProductionArtifactRevisionsAreImmutableAndFenced(t *testing.T) {
	db := openAgentRuntimeTestDB(t, filepath.Join(t.TempDir(), "agent-runtime.db"))
	repo := New(db)
	now := time.Now().UTC()
	artifact := &model.ProductionArtifact{
		ID: "artifact-1", UserID: "user-1", ProjectID: "project-1", Domain: "film",
		ArtifactType: "screenplay", LogicalKey: "episode-1-screenplay",
	}
	firstInput := &model.ProductionArtifactRevision{
		ID: "artifact-revision-1", Status: model.ProductionArtifactStatusLocked,
		ContentJSON: `{"title":"v1"}`, ContentDigest: "digest-v1",
		CreatedByType: "agent", CreatedByID: "narrative_screenwriter",
	}
	created, first, err := repo.CreateProductionArtifactRevision(ProductionArtifactRevisionCreate{
		UserID: artifact.UserID, Artifact: artifact, Revision: firstInput, ExpectedSequence: 0, At: now,
	})
	if err != nil {
		t.Fatalf("create first Artifact revision: %v", err)
	}
	if created.RevisionSequence != 1 || created.CurrentRevisionID != first.ID || first.Version != 1 || first.ParentRevisionID != "" {
		t.Fatalf("first Artifact revision is invalid: artifact=%#v revision=%#v", created, first)
	}
	secondInput := &model.ProductionArtifactRevision{
		ID: "artifact-revision-2", Status: model.ProductionArtifactStatusDraft,
		ContentJSON: `{"title":"v2"}`, ContentDigest: "digest-v2",
		CreatedByType: "user", CreatedByID: artifact.UserID,
	}
	created, second, err := repo.CreateProductionArtifactRevision(ProductionArtifactRevisionCreate{
		UserID: artifact.UserID, Artifact: artifact, Revision: secondInput, ExpectedSequence: 1, At: now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("append Artifact revision: %v", err)
	}
	if created.RevisionSequence != 2 || created.CurrentRevisionID != second.ID || second.Version != 2 || second.ParentRevisionID != first.ID {
		t.Fatalf("second Artifact revision is invalid: artifact=%#v revision=%#v", created, second)
	}
	staleInput := &model.ProductionArtifactRevision{
		ID: "artifact-revision-stale", Status: model.ProductionArtifactStatusDraft,
		ContentJSON: `{"title":"stale"}`, ContentDigest: "digest-stale",
		CreatedByType: "user", CreatedByID: artifact.UserID,
	}
	if _, _, err := repo.CreateProductionArtifactRevision(ProductionArtifactRevisionCreate{
		UserID: artifact.UserID, Artifact: artifact, Revision: staleInput, ExpectedSequence: 1,
	}); !errors.Is(err, ErrProductionArtifactConflict) {
		t.Fatalf("stale Artifact append error = %v", err)
	}
	revisions, err := repo.ProductionArtifactRevisionsForUser(artifact.UserID, artifact.ID)
	if err != nil {
		t.Fatalf("load Artifact revisions: %v", err)
	}
	if len(revisions) != 2 || revisions[0].Status != model.ProductionArtifactStatusLocked || revisions[0].ContentJSON != firstInput.ContentJSON {
		t.Fatalf("new revision mutated locked history: %#v", revisions)
	}
}

func TestHumanDecisionPauseAndResumeAreAtomicAndReplaySafe(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "agent-runtime.db")
	db := openAgentRuntimeTestDB(t, databasePath)
	repo := New(db)
	now := time.Now().UTC()
	bundle := agentRuntimeTestBundle("run-decision", "decision-request", now)
	createAgentRuntimeTestProject(t, db, bundle.Run.ProjectID, bundle.Run.UserID)
	if err := repo.CreateAgentRuntimeBundle(bundle); err != nil {
		t.Fatalf("create bundle: %v", err)
	}
	run, step, decision, err := repo.PauseAgentRuntimeForHuman(AgentRuntimeHumanPause{
		UserID: bundle.Run.UserID, RunID: bundle.Run.ID, StepID: bundle.Steps[0].ID,
		ExpectedRunRevision: 1, ExpectedStepRevision: 1,
		Decision: &model.AgentHumanDecision{
			ID: "decision-1", Question: "Choose the story direction", OptionsJSON: `[{"id":"a"},{"id":"b"}]`,
			Recommendation: "a", ImpactRefsJSON: "[]",
		},
		Event: AgentRuntimeEventInput{ID: "event-decision-2", EventType: "human_decision.requested", ActorType: "agent", ActorID: "film_project_lead"},
		At:    now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("PauseAgentRuntimeForHuman(): %v", err)
	}
	if run.Status != model.AgentRunStatusAwaitingHuman || step.Status != model.AgentStepStatusAwaitingHuman || decision.Status != model.AgentHumanDecisionStatusPending {
		t.Fatalf("pause did not persist a coherent state: run=%#v step=%#v decision=%#v", run, step, decision)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB(): %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}
	reopened := openAgentRuntimeTestDB(t, databasePath)
	repo = New(reopened)
	detail, err := repo.AgentRuntimeDetailForUser(bundle.Run.UserID, bundle.Run.ID)
	if err != nil {
		t.Fatalf("restore paused detail: %v", err)
	}
	if detail.Run.Status != model.AgentRunStatusAwaitingHuman || len(detail.HumanDecisions) != 1 || detail.HumanDecisions[0].Status != model.AgentHumanDecisionStatusPending {
		t.Fatalf("paused decision did not survive restart: %#v", detail)
	}
	run, step, decision, err = repo.ResolveAgentRuntimeHumanDecision(AgentRuntimeHumanResolve{
		UserID: bundle.Run.UserID, RunID: bundle.Run.ID, DecisionID: decision.ID,
		ExpectedRunRevision: run.Revision, ExpectedStepRevision: step.Revision, ExpectedDecisionRevision: decision.Revision,
		RunStatus: model.AgentRunStatusReady, StepStatus: model.AgentStepStatusReady, ResponseJSON: `{"optionId":"a"}`,
		Event: AgentRuntimeEventInput{ID: "event-decision-3", EventType: "human_decision.resolved", ActorType: "user", ActorID: bundle.Run.UserID},
		At:    now.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("ResolveAgentRuntimeHumanDecision(): %v", err)
	}
	if run.Status != model.AgentRunStatusReady || step.Status != model.AgentStepStatusReady || decision.Status != model.AgentHumanDecisionStatusResolved || decision.Revision != 2 {
		t.Fatalf("resume did not persist a coherent state: run=%#v step=%#v decision=%#v", run, step, decision)
	}
	if _, _, _, err := repo.ResolveAgentRuntimeHumanDecision(AgentRuntimeHumanResolve{
		UserID: bundle.Run.UserID, RunID: bundle.Run.ID, DecisionID: decision.ID,
		ExpectedRunRevision: 2, ExpectedStepRevision: 2, ExpectedDecisionRevision: 1,
		RunStatus: model.AgentRunStatusReady, StepStatus: model.AgentStepStatusReady, ResponseJSON: `{"optionId":"b"}`,
		Event: AgentRuntimeEventInput{ID: "event-decision-replay", EventType: "human_decision.resolved", ActorType: "user", ActorID: bundle.Run.UserID},
	}); !errors.Is(err, ErrAgentRuntimeStateConflict) {
		t.Fatalf("replayed decision error = %v", err)
	}
	detail, err = repo.AgentRuntimeDetailForUser(bundle.Run.UserID, bundle.Run.ID)
	if err != nil {
		t.Fatalf("load resolved detail: %v", err)
	}
	if len(detail.Events) != 3 || detail.Events[2].Sequence != 3 || detail.Events[2].EventType != "human_decision.resolved" {
		t.Fatalf("decision Event history is invalid: %#v", detail.Events)
	}
}

func TestAgentRuntimeExecutionClaimRecoversTheSameAttemptAndFencesTheOldWorker(t *testing.T) {
	db := openAgentRuntimeTestDB(t, filepath.Join(t.TempDir(), "agent-runtime.db"))
	repo := New(db)
	now := time.Now().UTC()
	bundle := agentRuntimeTestBundle("run-claim", "claim-request", now)
	createAgentRuntimeTestProject(t, db, bundle.Run.ProjectID, bundle.Run.UserID)
	if err := repo.CreateAgentRuntimeBundle(bundle); err != nil {
		t.Fatalf("create bundle: %v", err)
	}

	first, err := repo.ClaimNextAgentRuntimeExecution(AgentRuntimeExecutionClaimCommand{
		Owner: "worker-a", LeaseDuration: time.Minute, AttemptID: "attempt-a", TaskID: "task-a",
		EventID: "event-claim-a", Executor: "executor-a", At: now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("first execution claim: %v", err)
	}
	if first == nil || first.Recovered || first.Attempt.ID != "attempt-a" || first.Attempt.TaskID != "task-a" ||
		first.Attempt.Number != 1 || first.Step.Status != model.AgentStepStatusRunning || first.Run.Status != model.AgentRunStatusRunning {
		t.Fatalf("first execution claim is invalid: %#v", first)
	}

	notExpired, err := repo.ClaimNextAgentRuntimeExecution(AgentRuntimeExecutionClaimCommand{
		Owner: "worker-b", LeaseDuration: time.Minute, AttemptID: "attempt-b", TaskID: "task-b",
		EventID: "event-claim-b-early", Executor: "executor-b", At: now.Add(30 * time.Second),
	})
	if err != nil {
		t.Fatalf("claim while lease is active: %v", err)
	}
	if notExpired != nil {
		t.Fatalf("active lease was stolen: %#v", notExpired)
	}

	recovered, err := repo.ClaimNextAgentRuntimeExecution(AgentRuntimeExecutionClaimCommand{
		Owner: "worker-b", LeaseDuration: time.Minute, AttemptID: "attempt-b", TaskID: "task-b",
		EventID: "event-claim-b-recovered", Executor: "executor-b", At: now.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("recover expired execution claim: %v", err)
	}
	if recovered == nil || !recovered.Recovered || recovered.Attempt.ID != first.Attempt.ID ||
		recovered.Attempt.TaskID != first.Attempt.TaskID || recovered.Attempt.Number != first.Attempt.Number {
		t.Fatalf("expired claim did not resume the original paid execution identity: first=%#v recovered=%#v", first, recovered)
	}
	if recovered.Step.Revision <= first.Step.Revision || recovered.Attempt.Revision <= first.Attempt.Revision ||
		recovered.Run.Revision <= first.Run.Revision {
		t.Fatalf("recovery did not advance fencing revisions: first=%#v recovered=%#v", first, recovered)
	}
	if err := repo.RenewAgentRuntimeExecutionLease(first.Step.ID, first.Attempt.ID, "worker-a", time.Minute); !errors.Is(err, ErrAgentRuntimeLeaseLost) {
		t.Fatalf("old worker lease renewal error = %v, want ErrAgentRuntimeLeaseLost", err)
	}
	if _, err := repo.CompleteAgentRuntimeExecution(AgentRuntimeExecutionCompleteCommand{
		Owner: "worker-a", UserID: first.Run.UserID, RunID: first.Run.ID, StepID: first.Step.ID, AttemptID: first.Attempt.ID,
		ExpectedRunRevision: first.Run.Revision, ExpectedStepRevision: first.Step.Revision, ExpectedAttemptRevision: first.Attempt.Revision,
		ResponseJSON: `{"schemaVersion":1,"summary":"stale","artifacts":[]}`,
		Event:        AgentRuntimeEventInput{ID: "event-stale-completion", EventType: "attempt.succeeded", ActorType: "executor", ActorID: "executor-a"},
		At:           now.Add(3 * time.Minute),
	}); !errors.Is(err, ErrAgentRuntimeStateConflict) && !errors.Is(err, ErrAgentRuntimeLeaseLost) {
		t.Fatalf("old worker completion error = %v, want a fencing conflict", err)
	}

	detail, err := repo.AgentRuntimeDetailForUser(bundle.Run.UserID, bundle.Run.ID)
	if err != nil {
		t.Fatalf("load recovered detail: %v", err)
	}
	if len(detail.Attempts) != 1 || detail.Attempts[0].TaskID != "task-a" || detail.Attempts[0].Status != model.AgentAttemptStatusRunning ||
		len(detail.Events) != 3 || detail.Events[2].EventType != "attempt.recovered" {
		t.Fatalf("recovered execution history is invalid: %#v", detail)
	}
}

func TestCompleteAgentRuntimeExecutionCommitsArtifactsAndUnblocksNextStepAtomically(t *testing.T) {
	db := openAgentRuntimeTestDB(t, filepath.Join(t.TempDir(), "agent-runtime.db"))
	repo := New(db)
	now := time.Now().UTC()
	bundle := agentRuntimeTestBundle("run-complete", "complete-request", now)
	firstStep := bundle.Steps[0]
	secondStep := firstStep
	secondStep.ID = "run-complete-step-2"
	secondStep.StepKey = "intent:IR-03:skill:2"
	secondStep.Position = 1
	secondStep.SkillIDsJSON = `["story_structure"]`
	secondStep.Status = model.AgentStepStatusPlanned
	secondStep.DependsOnStepIDsJSON = `["` + firstStep.ID + `"]`
	secondStep.CreatedAt = now.Add(time.Millisecond)
	secondStep.UpdatedAt = secondStep.CreatedAt
	bundle.Steps = append(bundle.Steps, secondStep)
	createAgentRuntimeTestProject(t, db, bundle.Run.ProjectID, bundle.Run.UserID)
	if err := repo.CreateAgentRuntimeBundle(bundle); err != nil {
		t.Fatalf("create two-step bundle: %v", err)
	}
	claim, err := repo.ClaimNextAgentRuntimeExecution(AgentRuntimeExecutionClaimCommand{
		Owner: "worker-a", LeaseDuration: time.Minute, AttemptID: "attempt-complete", TaskID: "task-complete",
		EventID: "event-complete-claim", Executor: "executor-a", At: now.Add(time.Second),
	})
	if err != nil || claim == nil {
		t.Fatalf("claim first Step: claim=%#v error=%v", claim, err)
	}
	write := agentRuntimeExecutionArtifactWrite(*claim, "script")
	completed, err := repo.CompleteAgentRuntimeExecution(AgentRuntimeExecutionCompleteCommand{
		Owner: "worker-a", UserID: claim.Run.UserID, RunID: claim.Run.ID, StepID: claim.Step.ID, AttemptID: claim.Attempt.ID,
		ExpectedRunRevision: claim.Run.Revision, ExpectedStepRevision: claim.Step.Revision, ExpectedAttemptRevision: claim.Attempt.Revision,
		ResponseJSON:   `{"schemaVersion":1,"summary":"done","artifacts":[{"type":"script","contentText":"scene"}]}`,
		ArtifactWrites: []ProductionArtifactWrite{write},
		Event:          AgentRuntimeEventInput{ID: "event-complete-success", EventType: "attempt.succeeded", ActorType: "executor", ActorID: "executor-a"},
		At:             now.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("complete first Step: %v", err)
	}
	if completed.Run.Status != model.AgentRunStatusReady || completed.Run.CurrentStepID != secondStep.ID ||
		completed.Step.Status != model.AgentStepStatusCompleted || completed.Attempt.Status != model.AgentAttemptStatusSucceeded ||
		len(completed.ReadySteps) != 1 || completed.ReadySteps[0].ID != secondStep.ID ||
		len(completed.Artifacts) != 1 || len(completed.ArtifactRevisions) != 1 {
		t.Fatalf("atomic completion result is incoherent: %#v", completed)
	}
	if completed.ArtifactRevisions[0].Status != model.ProductionArtifactStatusReview ||
		completed.ArtifactRevisions[0].SourceAttemptID != claim.Attempt.ID {
		t.Fatalf("Agent output was not persisted as review evidence: %#v", completed.ArtifactRevisions[0])
	}
	var nextRefs []productionArtifactRef
	if err := json.Unmarshal([]byte(completed.ReadySteps[0].InputArtifactRefsJSON), &nextRefs); err != nil {
		t.Fatalf("decode next Step inputs: %v", err)
	}
	if len(nextRefs) != 1 || nextRefs[0].RevisionID != completed.ArtifactRevisions[0].ID {
		t.Fatalf("next Step did not receive the committed Artifact reference: %#v", nextRefs)
	}
	detail, err := repo.AgentRuntimeDetailForUser(bundle.Run.UserID, bundle.Run.ID)
	if err != nil {
		t.Fatalf("load completed detail: %v", err)
	}
	if len(detail.Events) != 3 || detail.Events[2].EventType != "attempt.succeeded" || len(detail.ArtifactRevisions) != 1 {
		t.Fatalf("completion evidence is incomplete: %#v", detail)
	}
}

func TestCompleteAgentRuntimeExecutionRollsBackEveryFactWhenAnArtifactWriteFails(t *testing.T) {
	db := openAgentRuntimeTestDB(t, filepath.Join(t.TempDir(), "agent-runtime.db"))
	repo := New(db)
	now := time.Now().UTC()
	bundle := agentRuntimeTestBundle("run-rollback", "rollback-request", now)
	createAgentRuntimeTestProject(t, db, bundle.Run.ProjectID, bundle.Run.UserID)
	if err := repo.CreateAgentRuntimeBundle(bundle); err != nil {
		t.Fatalf("create bundle: %v", err)
	}
	claim, err := repo.ClaimNextAgentRuntimeExecution(AgentRuntimeExecutionClaimCommand{
		Owner: "worker-a", LeaseDuration: time.Minute, AttemptID: "attempt-rollback", TaskID: "task-rollback",
		EventID: "event-rollback-claim", Executor: "executor-a", At: now.Add(time.Second),
	})
	if err != nil || claim == nil {
		t.Fatalf("claim Step: claim=%#v error=%v", claim, err)
	}
	valid := agentRuntimeExecutionArtifactWrite(*claim, "script")
	invalid := agentRuntimeExecutionArtifactWrite(*claim, "storyboard")
	invalid.Revision.ContentJSON = ""
	invalid.Revision.ContentText = ""
	if _, err := repo.CompleteAgentRuntimeExecution(AgentRuntimeExecutionCompleteCommand{
		Owner: "worker-a", UserID: claim.Run.UserID, RunID: claim.Run.ID, StepID: claim.Step.ID, AttemptID: claim.Attempt.ID,
		ExpectedRunRevision: claim.Run.Revision, ExpectedStepRevision: claim.Step.Revision, ExpectedAttemptRevision: claim.Attempt.Revision,
		ResponseJSON:   `{"schemaVersion":1,"summary":"invalid","artifacts":[]}`,
		ArtifactWrites: []ProductionArtifactWrite{valid, invalid},
		Event:          AgentRuntimeEventInput{ID: "event-rollback-complete", EventType: "attempt.succeeded", ActorType: "executor", ActorID: "executor-a"},
		At:             now.Add(2 * time.Second),
	}); err == nil {
		t.Fatal("completion with an invalid second Artifact unexpectedly succeeded")
	}

	detail, err := repo.AgentRuntimeDetailForUser(bundle.Run.UserID, bundle.Run.ID)
	if err != nil {
		t.Fatalf("load detail after rollback: %v", err)
	}
	if detail.Run.Revision != claim.Run.Revision || detail.Run.EventSequence != claim.Run.EventSequence ||
		detail.Steps[0].Revision != claim.Step.Revision || detail.Steps[0].Status != model.AgentStepStatusRunning ||
		detail.Attempts[0].Revision != claim.Attempt.Revision || detail.Attempts[0].Status != model.AgentAttemptStatusRunning ||
		len(detail.Artifacts) != 0 || len(detail.ArtifactRevisions) != 0 || len(detail.Events) != 2 {
		t.Fatalf("failed completion left partial facts behind: %#v", detail)
	}
}

func agentRuntimeTestBundle(runID string, idempotencyKey string, now time.Time) AgentRuntimeCreateBundle {
	userID := "user-1"
	stepID := runID + "-step"
	run := &model.AgentRuntimeRun{
		ID: runID, UserID: userID, ProjectID: "project-1", Domain: "film",
		RegistryID: "film-agent-team", RegistryVersion: "1.3.1", RouteKind: "intent", IntentRouteID: "IR-01", RootRunID: runID,
		Status: model.AgentRunStatusReady, Objective: "write a short film", InputJSON: "{}",
		CurrentStepID: stepID, IdempotencyKey: idempotencyKey, Revision: 1, EventSequence: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	step := model.AgentRuntimeStep{
		ID: stepID, RunID: run.ID, StepKey: "intent:IR-01", RouteKind: "intent", RouteID: "IR-01",
		AgentID: "narrative_screenwriter", SkillIDsJSON: `["screenwriter"]`, Status: model.AgentStepStatusReady,
		DependsOnStepIDsJSON: "[]", InputArtifactRefsJSON: "[]", OutputArtifactRefsJSON: "[]", Revision: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	return AgentRuntimeCreateBundle{
		Run: run,
		RoutingDecision: &model.AgentRoutingDecision{
			ID: runID + "-route", RunID: run.ID, RouteKind: "intent", RouteID: "IR-01", IntentRouteID: "IR-01", SelectedAgentID: step.AgentID,
			SelectedSkillIDsJSON: step.SkillIDsJSON, InputArtifactRefsJSON: "[]", AlternativesJSON: "[]",
			Reason: "explicit route", Confidence: "confirmed", DecidedByType: "user", DecidedByID: userID, CreatedAt: now,
		},
		Steps: []model.AgentRuntimeStep{step},
		Events: []model.AgentRuntimeEvent{{
			ID: runID + "-event-1", UserID: userID, RunID: run.ID, Sequence: 1,
			EventType: "run.created", ActorType: "user", ActorID: userID,
			ToStatus: string(run.Status), PayloadJSON: "{}", CreatedAt: now,
		}},
	}
}

func createAgentRuntimeTestProject(t *testing.T, db *gorm.DB, projectID string, userID string) {
	t.Helper()
	now := time.Now().UTC()
	project := model.Project{
		ID: projectID, UserID: userID, Name: projectID, Type: "short_drama", Status: model.ProjectStatusActive,
		Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&project).Error; err != nil {
		t.Fatalf("create Agent Runtime test Project: %v", err)
	}
}

func agentRuntimeExecutionArtifactWrite(claim AgentRuntimeExecutionClaim, artifactType string) ProductionArtifactWrite {
	artifactID := claim.Run.ID + "-artifact-" + artifactType
	revisionID := claim.Run.ID + "-revision-" + artifactType
	return ProductionArtifactWrite{
		Artifact: model.ProductionArtifact{
			ID: artifactID, UserID: claim.Run.UserID, ProjectID: claim.Run.ProjectID, Domain: claim.Run.Domain,
			ArtifactType: artifactType, LogicalKey: "step:" + claim.Step.ID + ":" + artifactType,
		},
		Revision: model.ProductionArtifactRevision{
			ID: revisionID, Status: model.ProductionArtifactStatusReview, ContentJSON: `{"schemaVersion":1}`,
			ContentDigest: "digest-" + artifactType, SourceRunID: claim.Run.ID, SourceStepID: claim.Step.ID,
			SourceAttemptID: claim.Attempt.ID, SourceArtifactRefsJSON: "[]", AuthorityRefsJSON: "[]",
			CreatedByType: "agent", CreatedByID: claim.Step.AgentID,
		},
	}
}

func openAgentRuntimeTestDB(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Project{},
		&model.AgentRuntimeRun{}, &model.AgentRuntimeStep{}, &model.AgentRuntimeAttempt{},
		&model.AgentRoutingDecision{}, &model.AgentHumanDecision{}, &model.AgentRuntimeEvent{},
		&model.AgentHandoffTrigger{},
		&model.ProductionArtifact{}, &model.ProductionArtifactRevision{},
	); err != nil {
		t.Fatalf("migrate agent runtime schema: %v", err)
	}
	return db
}
