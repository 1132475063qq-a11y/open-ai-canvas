package repository

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

func TestLockProductionArtifactForHandoffAppendsImmutableRevisionEventAndTrigger(t *testing.T) {
	db := openAgentRuntimeTestDB(t, filepath.Join(t.TempDir(), "agent-handoff.db"))
	repo := New(db)
	now := time.Now().UTC()
	bundle := agentRuntimeTestBundle("run-lock", "lock-request", now)
	bundle.Run.RootRunID = bundle.Run.ID
	createAgentRuntimeTestProject(t, db, bundle.Run.ProjectID, bundle.Run.UserID)
	if err := repo.CreateAgentRuntimeBundle(bundle); err != nil {
		t.Fatalf("create bundle: %v", err)
	}
	completeAgentHandoffTestSource(t, db, *bundle.Run, bundle.Steps[0], now)
	artifact, review := createAgentHandoffTestArtifact(t, repo, *bundle.Run, bundle.Steps[0], "script", model.ProductionArtifactStatusReview, now.Add(time.Second))

	locked, err := repo.LockProductionArtifactForHandoff(ProductionArtifactLockCommand{
		UserID: bundle.Run.UserID, ProjectID: bundle.Run.ProjectID, Domain: bundle.Run.Domain,
		RunID: bundle.Run.ID, ArtifactID: artifact.ID, ExpectedRunRevision: bundle.Run.Revision,
		ExpectedArtifactSequence: artifact.RevisionSequence, ExpectedRevisionID: review.ID,
		LockedRevisionID: "revision-locked", TriggerID: "trigger-locked",
		Event: AgentRuntimeEventInput{ID: "event-artifact-locked", EventType: "artifact.locked", ActorType: "user", ActorID: bundle.Run.UserID},
		At:    now.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("LockProductionArtifactForHandoff(): %v", err)
	}
	if locked.Run.Revision != 2 || locked.Run.EventSequence != 2 || locked.Artifact.RevisionSequence != 2 ||
		locked.Artifact.CurrentRevisionID != locked.LockedRevision.ID {
		t.Fatalf("lock did not advance Run and Artifact atomically: %#v", locked)
	}
	if locked.SourceRevision.ID != review.ID || locked.SourceRevision.Status != model.ProductionArtifactStatusReview ||
		locked.LockedRevision.Status != model.ProductionArtifactStatusLocked || locked.LockedRevision.ParentRevisionID != review.ID ||
		locked.LockedRevision.ContentJSON != review.ContentJSON || locked.LockedRevision.ContentDigest != review.ContentDigest ||
		locked.LockedRevision.CreatedByType != "user" || locked.LockedRevision.CreatedByID != bundle.Run.UserID {
		t.Fatalf("lock mutated or failed to preserve immutable review evidence: %#v", locked)
	}
	if locked.Trigger.Status != model.AgentHandoffTriggerStatusPending || locked.Trigger.RevisionID != locked.LockedRevision.ID ||
		locked.Trigger.RootRunID != bundle.Run.ID || locked.Trigger.SourceAgentID != bundle.Steps[0].AgentID ||
		locked.Trigger.ScheduledRunIDsJSON != "[]" {
		t.Fatalf("durable Handoff Trigger is incomplete: %#v", locked.Trigger)
	}
	detail, err := repo.AgentRuntimeDetailForUser(bundle.Run.UserID, bundle.Run.ID)
	if err != nil {
		t.Fatalf("load locked detail: %v", err)
	}
	if len(detail.ArtifactRevisions) != 2 || detail.ArtifactRevisions[0].Status != model.ProductionArtifactStatusReview ||
		detail.ArtifactRevisions[1].Status != model.ProductionArtifactStatusLocked || len(detail.Events) != 2 ||
		detail.Events[1].EventType != "artifact.locked" || detail.Events[1].FromStatus != "review" || detail.Events[1].ToStatus != "locked" {
		t.Fatalf("lock evidence is incomplete: %#v", detail)
	}

	if _, err := repo.LockProductionArtifactForHandoff(ProductionArtifactLockCommand{
		UserID: bundle.Run.UserID, ProjectID: bundle.Run.ProjectID, Domain: bundle.Run.Domain,
		RunID: bundle.Run.ID, ArtifactID: artifact.ID, ExpectedRunRevision: bundle.Run.Revision,
		ExpectedArtifactSequence: 1, ExpectedRevisionID: review.ID, LockedRevisionID: "revision-replay", TriggerID: "trigger-replay",
		Event: AgentRuntimeEventInput{ID: "event-replay", EventType: "artifact.locked", ActorType: "user", ActorID: bundle.Run.UserID},
	}); !errors.Is(err, ErrAgentRuntimeStateConflict) {
		t.Fatalf("stale lock replay error = %v, want Run revision conflict", err)
	}
}

func TestLockProductionArtifactForHandoffRollsBackWhenTriggerWriteFails(t *testing.T) {
	db := openAgentRuntimeTestDB(t, filepath.Join(t.TempDir(), "agent-handoff-rollback.db"))
	repo := New(db)
	now := time.Now().UTC()
	bundle := agentRuntimeTestBundle("run-lock-rollback", "lock-rollback-request", now)
	bundle.Run.RootRunID = bundle.Run.ID
	createAgentRuntimeTestProject(t, db, bundle.Run.ProjectID, bundle.Run.UserID)
	if err := repo.CreateAgentRuntimeBundle(bundle); err != nil {
		t.Fatalf("create bundle: %v", err)
	}
	completeAgentHandoffTestSource(t, db, *bundle.Run, bundle.Steps[0], now)
	artifact, review := createAgentHandoffTestArtifact(t, repo, *bundle.Run, bundle.Steps[0], "script", model.ProductionArtifactStatusReview, now.Add(time.Second))
	duplicate := model.AgentHandoffTrigger{
		ID: "trigger-duplicate", UserID: bundle.Run.UserID, ProjectID: bundle.Run.ProjectID, Domain: "film", RootRunID: bundle.Run.ID,
		ArtifactID: "other-artifact", RevisionID: "other-revision", SourceRunID: bundle.Run.ID, SourceStepID: bundle.Steps[0].ID,
		SourceAgentID: bundle.Steps[0].AgentID, Status: model.AgentHandoffTriggerStatusCompleted,
		ScheduledRunIDsJSON: "[]", Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&duplicate).Error; err != nil {
		t.Fatalf("create duplicate Trigger identity: %v", err)
	}
	if _, err := repo.LockProductionArtifactForHandoff(ProductionArtifactLockCommand{
		UserID: bundle.Run.UserID, ProjectID: bundle.Run.ProjectID, Domain: bundle.Run.Domain,
		RunID: bundle.Run.ID, ArtifactID: artifact.ID, ExpectedRunRevision: bundle.Run.Revision,
		ExpectedArtifactSequence: artifact.RevisionSequence, ExpectedRevisionID: review.ID,
		LockedRevisionID: "revision-must-rollback", TriggerID: duplicate.ID,
		Event: AgentRuntimeEventInput{ID: "event-must-rollback", EventType: "artifact.locked", ActorType: "user", ActorID: bundle.Run.UserID},
	}); err == nil {
		t.Fatal("lock with duplicate Trigger identity unexpectedly succeeded")
	}
	persisted, err := repo.ProductionArtifactForUser(bundle.Run.UserID, artifact.ID)
	if err != nil {
		t.Fatalf("load Artifact after rollback: %v", err)
	}
	revisions, err := repo.ProductionArtifactRevisionsForUser(bundle.Run.UserID, artifact.ID)
	if err != nil {
		t.Fatalf("load revisions after rollback: %v", err)
	}
	run, err := repo.AgentRuntimeRunForUser(bundle.Run.UserID, bundle.Run.ID)
	if err != nil {
		t.Fatalf("load Run after rollback: %v", err)
	}
	if persisted.CurrentRevisionID != review.ID || persisted.RevisionSequence != 1 || len(revisions) != 1 ||
		run.Revision != bundle.Run.Revision || run.EventSequence != bundle.Run.EventSequence {
		t.Fatalf("failed Trigger write left partial lock facts: artifact=%#v revisions=%#v run=%#v", persisted, revisions, run)
	}
}

func TestLockProductionArtifactForHandoffRejectsUnfinishedExecutionEvidence(t *testing.T) {
	db := openAgentRuntimeTestDB(t, filepath.Join(t.TempDir(), "agent-handoff-unfinished.db"))
	repo := New(db)
	now := time.Now().UTC()
	bundle := agentRuntimeTestBundle("run-lock-unfinished", "lock-unfinished-request", now)
	bundle.Run.RootRunID = bundle.Run.ID
	createAgentRuntimeTestProject(t, db, bundle.Run.ProjectID, bundle.Run.UserID)
	if err := repo.CreateAgentRuntimeBundle(bundle); err != nil {
		t.Fatalf("create bundle: %v", err)
	}
	artifact, review := createAgentHandoffTestArtifact(t, repo, *bundle.Run, bundle.Steps[0], "script", model.ProductionArtifactStatusReview, now.Add(time.Second))
	if _, err := repo.LockProductionArtifactForHandoff(ProductionArtifactLockCommand{
		UserID: bundle.Run.UserID, ProjectID: bundle.Run.ProjectID, Domain: bundle.Run.Domain,
		RunID: bundle.Run.ID, ArtifactID: artifact.ID, ExpectedRunRevision: bundle.Run.Revision,
		ExpectedArtifactSequence: 1, ExpectedRevisionID: review.ID, LockedRevisionID: "revision-unfinished", TriggerID: "trigger-unfinished",
		Event: AgentRuntimeEventInput{ID: "event-unfinished", EventType: "artifact.locked", ActorType: "user", ActorID: bundle.Run.UserID},
	}); !errors.Is(err, ErrProductionArtifactConflict) {
		t.Fatalf("unfinished source evidence lock error = %v", err)
	}
	persisted, err := repo.ProductionArtifactForUser(bundle.Run.UserID, artifact.ID)
	if err != nil {
		t.Fatalf("load unfinished Artifact: %v", err)
	}
	if persisted.CurrentRevisionID != review.ID || persisted.RevisionSequence != 1 {
		t.Fatalf("unfinished source evidence was partially locked: %#v", persisted)
	}
}

func TestAgentHandoffTriggerLeaseRecoveryFencesOldWorkerAndCompletesOnce(t *testing.T) {
	db := openAgentRuntimeTestDB(t, filepath.Join(t.TempDir(), "agent-handoff-lease.db"))
	repo := New(db)
	now := time.Now().UTC()
	bundle := agentRuntimeTestBundle("run-trigger", "trigger-request", now)
	bundle.Run.RootRunID = bundle.Run.ID
	createAgentRuntimeTestProject(t, db, bundle.Run.ProjectID, bundle.Run.UserID)
	if err := repo.CreateAgentRuntimeBundle(bundle); err != nil {
		t.Fatalf("create bundle: %v", err)
	}
	completeAgentHandoffTestSource(t, db, *bundle.Run, bundle.Steps[0], now)
	artifact, review := createAgentHandoffTestArtifact(t, repo, *bundle.Run, bundle.Steps[0], "script", model.ProductionArtifactStatusReview, now.Add(time.Second))
	locked, err := repo.LockProductionArtifactForHandoff(ProductionArtifactLockCommand{
		UserID: bundle.Run.UserID, ProjectID: bundle.Run.ProjectID, Domain: bundle.Run.Domain,
		RunID: bundle.Run.ID, ArtifactID: artifact.ID, ExpectedRunRevision: bundle.Run.Revision,
		ExpectedArtifactSequence: 1, ExpectedRevisionID: review.ID, LockedRevisionID: "revision-trigger-locked", TriggerID: "trigger-lease",
		Event: AgentRuntimeEventInput{ID: "event-trigger-lock", EventType: "artifact.locked", ActorType: "user", ActorID: bundle.Run.UserID},
		At:    now.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("lock Artifact: %v", err)
	}
	first, err := repo.ClaimNextAgentHandoffTrigger(AgentHandoffTriggerClaimCommand{
		Owner: "handoff-worker-a", Domain: "film", LeaseDuration: time.Minute, At: now.Add(3 * time.Second),
	})
	if err != nil || first == nil || first.Recovered || first.Trigger.ID != locked.Trigger.ID || first.Trigger.AttemptCount != 1 {
		t.Fatalf("first Trigger claim = %#v, error = %v", first, err)
	}
	early, err := repo.ClaimNextAgentHandoffTrigger(AgentHandoffTriggerClaimCommand{
		Owner: "handoff-worker-b", Domain: "film", LeaseDuration: time.Minute, At: now.Add(30 * time.Second),
	})
	if err != nil || early != nil {
		t.Fatalf("active Trigger lease was claimable: claim=%#v error=%v", early, err)
	}
	recovered, err := repo.ClaimNextAgentHandoffTrigger(AgentHandoffTriggerClaimCommand{
		Owner: "handoff-worker-b", Domain: "film", LeaseDuration: time.Minute, At: now.Add(2 * time.Minute),
	})
	if err != nil || recovered == nil || !recovered.Recovered || recovered.Trigger.ID != first.Trigger.ID ||
		recovered.Trigger.AttemptCount != 2 || recovered.Trigger.Revision <= first.Trigger.Revision {
		t.Fatalf("expired Trigger lease did not recover coherently: first=%#v recovered=%#v error=%v", first, recovered, err)
	}
	if _, err := repo.CompleteAgentHandoffTrigger(AgentHandoffTriggerCompleteCommand{
		Owner: "handoff-worker-a", TriggerID: first.Trigger.ID, ExpectedRevision: first.Trigger.Revision,
		ScheduledRunIDs: []string{"run-stale"}, At: now.Add(2*time.Minute + time.Second),
	}); !errors.Is(err, ErrAgentHandoffTriggerLeaseLost) {
		t.Fatalf("stale Trigger worker completion error = %v", err)
	}
	completed, err := repo.CompleteAgentHandoffTrigger(AgentHandoffTriggerCompleteCommand{
		Owner: "handoff-worker-b", TriggerID: recovered.Trigger.ID, ExpectedRevision: recovered.Trigger.Revision,
		ScheduledRunIDs: []string{"handoff-run-b", "handoff-run-a"}, At: now.Add(2*time.Minute + time.Second),
	})
	if err != nil {
		t.Fatalf("complete recovered Trigger: %v", err)
	}
	var runIDs []string
	if err := json.Unmarshal([]byte(completed.ScheduledRunIDsJSON), &runIDs); err != nil {
		t.Fatalf("decode scheduled Run IDs: %v", err)
	}
	if completed.Status != model.AgentHandoffTriggerStatusCompleted || completed.CompletedAt == nil ||
		len(runIDs) != 2 || runIDs[0] != "handoff-run-a" || runIDs[1] != "handoff-run-b" {
		t.Fatalf("completed Trigger is invalid: %#v", completed)
	}
	if _, err := repo.CompleteAgentHandoffTrigger(AgentHandoffTriggerCompleteCommand{
		Owner: "handoff-worker-b", TriggerID: recovered.Trigger.ID, ExpectedRevision: recovered.Trigger.Revision,
	}); !errors.Is(err, ErrAgentHandoffTriggerLeaseLost) {
		t.Fatalf("Trigger completion replay error = %v", err)
	}
}

func TestAgentHandoffTriggerFailureCanRetryThenBecomeTerminal(t *testing.T) {
	db := openAgentRuntimeTestDB(t, filepath.Join(t.TempDir(), "agent-handoff-failure.db"))
	repo := New(db)
	now := time.Now().UTC()
	bundle := agentRuntimeTestBundle("run-trigger-failure", "trigger-failure-request", now)
	bundle.Run.RootRunID = bundle.Run.ID
	createAgentRuntimeTestProject(t, db, bundle.Run.ProjectID, bundle.Run.UserID)
	if err := repo.CreateAgentRuntimeBundle(bundle); err != nil {
		t.Fatalf("create bundle: %v", err)
	}
	completeAgentHandoffTestSource(t, db, *bundle.Run, bundle.Steps[0], now)
	artifact, review := createAgentHandoffTestArtifact(t, repo, *bundle.Run, bundle.Steps[0], "script", model.ProductionArtifactStatusReview, now.Add(time.Second))
	locked, err := repo.LockProductionArtifactForHandoff(ProductionArtifactLockCommand{
		UserID: bundle.Run.UserID, ProjectID: bundle.Run.ProjectID, Domain: bundle.Run.Domain,
		RunID: bundle.Run.ID, ArtifactID: artifact.ID, ExpectedRunRevision: bundle.Run.Revision,
		ExpectedArtifactSequence: 1, ExpectedRevisionID: review.ID, LockedRevisionID: "revision-trigger-failure", TriggerID: "trigger-failure",
		Event: AgentRuntimeEventInput{ID: "event-trigger-failure", EventType: "artifact.locked", ActorType: "user", ActorID: bundle.Run.UserID},
		At:    now.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("lock Artifact: %v", err)
	}
	claim, err := repo.ClaimNextAgentHandoffTrigger(AgentHandoffTriggerClaimCommand{
		Owner: "worker-failure", Domain: "film", LeaseDuration: time.Minute, At: now.Add(3 * time.Second),
	})
	if err != nil || claim == nil || claim.Trigger.ID != locked.Trigger.ID {
		t.Fatalf("claim Trigger: claim=%#v error=%v", claim, err)
	}
	retryAt := now.Add(2 * time.Minute)
	pending, err := repo.FailAgentHandoffTrigger(AgentHandoffTriggerFailCommand{
		Owner: "worker-failure", TriggerID: claim.Trigger.ID, ExpectedRevision: claim.Trigger.Revision,
		FailureCode: "temporary", Failure: "temporary database outage", RetryAt: &retryAt, At: now.Add(4 * time.Second),
	})
	if err != nil {
		t.Fatalf("record retryable Trigger failure: %v", err)
	}
	if pending.Status != model.AgentHandoffTriggerStatusPending || pending.NextAttemptAt == nil || pending.FailureCode != "temporary" {
		t.Fatalf("retryable Trigger failure state is invalid: %#v", pending)
	}
	if early, err := repo.ClaimNextAgentHandoffTrigger(AgentHandoffTriggerClaimCommand{
		Owner: "worker-early", Domain: "film", LeaseDuration: time.Minute, At: now.Add(time.Minute),
	}); err != nil || early != nil {
		t.Fatalf("Trigger was claimable before retry time: claim=%#v error=%v", early, err)
	}
	retryClaim, err := repo.ClaimNextAgentHandoffTrigger(AgentHandoffTriggerClaimCommand{
		Owner: "worker-terminal", Domain: "film", LeaseDuration: time.Minute, At: retryAt.Add(time.Second),
	})
	if err != nil || retryClaim == nil || retryClaim.Trigger.AttemptCount != 2 {
		t.Fatalf("claim retryable Trigger: claim=%#v error=%v", retryClaim, err)
	}
	failed, err := repo.FailAgentHandoffTrigger(AgentHandoffTriggerFailCommand{
		Owner: "worker-terminal", TriggerID: retryClaim.Trigger.ID, ExpectedRevision: retryClaim.Trigger.Revision,
		FailureCode: "permanent", Failure: "registry contract is unavailable", Terminal: true, At: retryAt.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("record terminal Trigger failure: %v", err)
	}
	if failed.Status != model.AgentHandoffTriggerStatusFailed || failed.CompletedAt == nil || failed.NextAttemptAt != nil || failed.FailureCode != "permanent" {
		t.Fatalf("terminal Trigger failure state is invalid: %#v", failed)
	}
	if next, err := repo.ClaimNextAgentHandoffTrigger(AgentHandoffTriggerClaimCommand{
		Owner: "worker-after-terminal", Domain: "film", LeaseDuration: time.Minute, At: retryAt.Add(time.Hour),
	}); err != nil || next != nil {
		t.Fatalf("terminal Trigger was claimable: claim=%#v error=%v", next, err)
	}
}

func TestLockedProductionArtifactFactsForRootNeverMixesAnotherRoot(t *testing.T) {
	db := openAgentRuntimeTestDB(t, filepath.Join(t.TempDir(), "agent-root-artifacts.db"))
	repo := New(db)
	now := time.Now().UTC()
	createAgentRuntimeTestProject(t, db, "project-1", "user-1")
	first := agentRuntimeTestBundle("root-a", "root-a-request", now)
	first.Run.RootRunID = first.Run.ID
	second := agentRuntimeTestBundle("root-b", "root-b-request", now.Add(time.Second))
	second.Run.RootRunID = second.Run.ID
	if err := repo.CreateAgentRuntimeBundle(first); err != nil {
		t.Fatalf("create first root: %v", err)
	}
	if err := repo.CreateAgentRuntimeBundle(second); err != nil {
		t.Fatalf("create second root: %v", err)
	}
	_, firstRevision := createAgentHandoffTestArtifact(t, repo, *first.Run, first.Steps[0], "script", model.ProductionArtifactStatusLocked, now.Add(2*time.Second))
	_, secondRevision := createAgentHandoffTestArtifact(t, repo, *second.Run, second.Steps[0], "script", model.ProductionArtifactStatusLocked, now.Add(3*time.Second))

	facts, err := repo.LockedProductionArtifactFactsForRoot("user-1", "project-1", "film", first.Run.ID)
	if err != nil {
		t.Fatalf("query first root locked Artifacts: %v", err)
	}
	if len(facts) != 1 || facts[0].Revision.ID != firstRevision.ID || facts[0].Revision.ID == secondRevision.ID ||
		facts[0].SourceAgentID != first.Steps[0].AgentID {
		t.Fatalf("root Artifact query crossed lineages: %#v", facts)
	}
	if _, err := repo.LockedProductionArtifactFactsForRoot("user-1", "project-1", "film", "missing-root"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("missing root query error = %v", err)
	}
}

func createAgentHandoffTestArtifact(t *testing.T, repo *Repository, run model.AgentRuntimeRun, step model.AgentRuntimeStep, artifactType string, status model.ProductionArtifactStatus, at time.Time) (model.ProductionArtifact, model.ProductionArtifactRevision) {
	t.Helper()
	artifact := &model.ProductionArtifact{
		ID: run.ID + "-artifact-" + artifactType, UserID: run.UserID, ProjectID: run.ProjectID, Domain: run.Domain,
		ArtifactType: artifactType, LogicalKey: "run:" + run.ID + ":" + artifactType,
	}
	revision := &model.ProductionArtifactRevision{
		ID: run.ID + "-revision-" + artifactType, Status: status,
		ContentJSON: `{"schemaVersion":1,"artifactType":"` + artifactType + `"}`, ContentDigest: "digest-" + run.ID + "-" + artifactType,
		SourceRunID: run.ID, SourceStepID: step.ID, SourceAttemptID: run.ID + "-source-attempt", SourceArtifactRefsJSON: "[]", AuthorityRefsJSON: "[]",
		CreatedByType: "agent", CreatedByID: step.AgentID,
	}
	persistedArtifact, persistedRevision, err := repo.CreateProductionArtifactRevision(ProductionArtifactRevisionCreate{
		UserID: run.UserID, Artifact: artifact, Revision: revision, ExpectedSequence: 0, At: at,
	})
	if err != nil {
		t.Fatalf("create Handoff test Artifact: %v", err)
	}
	return *persistedArtifact, *persistedRevision
}

func completeAgentHandoffTestSource(t *testing.T, db *gorm.DB, run model.AgentRuntimeRun, step model.AgentRuntimeStep, at time.Time) {
	t.Helper()
	if err := db.Model(&model.AgentRuntimeRun{}).Where("id = ?", run.ID).
		Updates(map[string]any{"status": model.AgentRunStatusCompleted, "completed_at": at}).Error; err != nil {
		t.Fatalf("complete Handoff test Run: %v", err)
	}
	if err := db.Model(&model.AgentRuntimeStep{}).Where("id = ?", step.ID).
		Updates(map[string]any{"status": model.AgentStepStatusCompleted, "attempt_sequence": 1, "completed_at": at}).Error; err != nil {
		t.Fatalf("complete Handoff test Step: %v", err)
	}
	attempt := model.AgentRuntimeAttempt{
		ID: run.ID + "-source-attempt", RunID: run.ID, StepID: step.ID, Number: 1,
		Status: model.AgentAttemptStatusSucceeded, Executor: "test", InputDigest: "input", PromptDigest: "prompt",
		RequestJSON: "{}", ResponseJSON: "{}", Revision: 1, StartedAt: &at, CompletedAt: &at, CreatedAt: at, UpdatedAt: at,
	}
	if err := db.Create(&attempt).Error; err != nil {
		t.Fatalf("create Handoff test source Attempt: %v", err)
	}
}
