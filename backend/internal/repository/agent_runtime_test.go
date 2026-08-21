package repository

import (
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

func agentRuntimeTestBundle(runID string, idempotencyKey string, now time.Time) AgentRuntimeCreateBundle {
	userID := "user-1"
	stepID := runID + "-step"
	run := &model.AgentRuntimeRun{
		ID: runID, UserID: userID, ProjectID: "project-1", Domain: "film",
		RegistryID: "film-agent-team", RegistryVersion: "1.3.1", IntentRouteID: "IR-01",
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
			ID: runID + "-route", RunID: run.ID, IntentRouteID: "IR-01", SelectedAgentID: step.AgentID,
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

func openAgentRuntimeTestDB(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(
		&model.AgentRuntimeRun{}, &model.AgentRuntimeStep{}, &model.AgentRuntimeAttempt{},
		&model.AgentRoutingDecision{}, &model.AgentHumanDecision{}, &model.AgentRuntimeEvent{},
		&model.ProductionArtifact{}, &model.ProductionArtifactRevision{},
	); err != nil {
		t.Fatalf("migrate agent runtime schema: %v", err)
	}
	return db
}
