package repository

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
)

func TestEcommerceAgentRuntimeClaimScopesDomainAndRecoversExpiredLease(t *testing.T) {
	db := openAgentRuntimeTestDB(t, filepath.Join(t.TempDir(), "ecommerce-agent-runtime.db"))
	repo := New(db)
	now := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)

	ecommerce := ecommerceAgentRuntimeTestBundle("ecommerce-claim", "ecommerce-claim-request", "ecommerce-project", now)
	film := agentRuntimeTestBundle("film-claim", "film-claim-request", now)
	film.Run.ProjectID = "film-project"
	createAgentRuntimeTestProject(t, db, ecommerce.Run.ProjectID, ecommerce.Run.UserID)
	createAgentRuntimeTestProject(t, db, film.Run.ProjectID, film.Run.UserID)
	if err := repo.CreateAgentRuntimeBundle(ecommerce); err != nil {
		t.Fatalf("create ecommerce bundle: %v", err)
	}
	if err := repo.CreateAgentRuntimeBundle(film); err != nil {
		t.Fatalf("create film bundle: %v", err)
	}

	first, err := repo.ClaimNextAgentRuntimeExecution(AgentRuntimeExecutionClaimCommand{
		Domain: "ecommerce", Owner: "ecommerce-worker-a", LeaseDuration: time.Minute,
		AttemptID: "ecommerce-attempt-a", TaskID: "ecommerce-task-a", EventID: "ecommerce-claim-a",
		Executor: "ecommerce-executor", At: now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("claim ecommerce execution: %v", err)
	}
	if first == nil || first.Run.Domain != "ecommerce" || first.Run.ID != ecommerce.Run.ID || first.Step.Status != model.AgentStepStatusRunning ||
		first.Attempt.ID != "ecommerce-attempt-a" || first.Attempt.Number != 1 || first.Recovered {
		t.Fatalf("ecommerce claim crossed a domain or has invalid state: %#v", first)
	}

	notExpired, err := repo.ClaimNextAgentRuntimeExecution(AgentRuntimeExecutionClaimCommand{
		Domain: "ecommerce", Owner: "ecommerce-worker-b", LeaseDuration: time.Minute,
		AttemptID: "ecommerce-attempt-b", TaskID: "ecommerce-task-b", EventID: "ecommerce-claim-b-early",
		Executor: "ecommerce-executor", At: now.Add(30 * time.Second),
	})
	if err != nil {
		t.Fatalf("claim ecommerce execution before lease expiry: %v", err)
	}
	if notExpired != nil {
		t.Fatalf("active ecommerce lease was stolen: %#v", notExpired)
	}

	recovered, err := repo.ClaimNextAgentRuntimeExecution(AgentRuntimeExecutionClaimCommand{
		Domain: "ecommerce", Owner: "ecommerce-worker-b", LeaseDuration: time.Minute,
		AttemptID: "ecommerce-attempt-b", TaskID: "ecommerce-task-b", EventID: "ecommerce-claim-b-recovered",
		Executor: "ecommerce-executor", At: now.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("recover expired ecommerce execution: %v", err)
	}
	if recovered == nil || !recovered.Recovered || recovered.Run.Domain != "ecommerce" || recovered.Attempt.ID != first.Attempt.ID ||
		recovered.Attempt.TaskID != first.Attempt.TaskID || recovered.Attempt.Number != first.Attempt.Number {
		t.Fatalf("expired ecommerce lease did not recover the same Attempt: first=%#v recovered=%#v", first, recovered)
	}
	if recovered.Run.Revision <= first.Run.Revision || recovered.Step.Revision <= first.Step.Revision || recovered.Attempt.Revision <= first.Attempt.Revision {
		t.Fatalf("ecommerce recovery did not advance fencing revisions: first=%#v recovered=%#v", first, recovered)
	}

	filmDetail, err := repo.AgentRuntimeDetailForUser(film.Run.UserID, film.Run.ID)
	if err != nil {
		t.Fatalf("load film detail after ecommerce claims: %v", err)
	}
	if filmDetail.Run.Domain != "film" || filmDetail.Run.Status != model.AgentRunStatusReady || filmDetail.Steps[0].Status != model.AgentStepStatusReady || len(filmDetail.Attempts) != 0 || len(filmDetail.Events) != 1 {
		t.Fatalf("ecommerce worker claimed or changed a film Run: %#v", filmDetail)
	}

	ecommerceDetail, err := repo.AgentRuntimeDetailForUser(ecommerce.Run.UserID, ecommerce.Run.ID)
	if err != nil {
		t.Fatalf("load ecommerce detail after recovery: %v", err)
	}
	if len(ecommerceDetail.Attempts) != 1 || len(ecommerceDetail.Events) != 3 || ecommerceDetail.Events[1].EventType != "attempt.started" || ecommerceDetail.Events[2].EventType != "attempt.recovered" {
		t.Fatalf("ecommerce recovery evidence is not append-only: %#v", ecommerceDetail)
	}
}

func TestEcommerceAgentRuntimeCompletionAndFailureFactsAreAppendOnly(t *testing.T) {
	t.Run("completion", func(t *testing.T) {
		db := openAgentRuntimeTestDB(t, filepath.Join(t.TempDir(), "ecommerce-complete.db"))
		repo := New(db)
		now := time.Date(2026, time.January, 2, 12, 0, 0, 0, time.UTC)
		bundle := ecommerceAgentRuntimeTestBundle("ecommerce-complete", "ecommerce-complete-request", "ecommerce-complete-project", now)
		createAgentRuntimeTestProject(t, db, bundle.Run.ProjectID, bundle.Run.UserID)
		if err := repo.CreateAgentRuntimeBundle(bundle); err != nil {
			t.Fatalf("create ecommerce completion bundle: %v", err)
		}
		claim, err := repo.ClaimNextAgentRuntimeExecution(AgentRuntimeExecutionClaimCommand{
			Domain: "ecommerce", Owner: "ecommerce-complete-worker", LeaseDuration: time.Minute,
			AttemptID: "ecommerce-complete-attempt", TaskID: "ecommerce-complete-task", EventID: "ecommerce-complete-claim",
			Executor: "ecommerce-executor", At: now.Add(time.Second),
		})
		if err != nil || claim == nil {
			t.Fatalf("claim ecommerce completion execution: claim=%#v error=%v", claim, err)
		}
		completed, err := repo.CompleteAgentRuntimeExecution(AgentRuntimeExecutionCompleteCommand{
			Owner: claim.Step.LeaseOwner, UserID: claim.Run.UserID, RunID: claim.Run.ID, StepID: claim.Step.ID, AttemptID: claim.Attempt.ID,
			ExpectedRunRevision: claim.Run.Revision, ExpectedStepRevision: claim.Step.Revision, ExpectedAttemptRevision: claim.Attempt.Revision,
			ResponseJSON: `{"schemaVersion":1,"summary":"ecommerce complete","artifacts":[]}`,
			Event:        AgentRuntimeEventInput{ID: "ecommerce-complete-event", EventType: "attempt.succeeded", ActorType: "executor", ActorID: "ecommerce-executor"},
			At:           now.Add(2 * time.Second),
		})
		if err != nil {
			t.Fatalf("complete ecommerce execution: %v", err)
		}
		if completed.Run.Status != model.AgentRunStatusCompleted || completed.Step.Status != model.AgentStepStatusCompleted || completed.Attempt.Status != model.AgentAttemptStatusSucceeded {
			t.Fatalf("completion did not persist terminal state: %#v", completed)
		}

		if _, err := repo.CompleteAgentRuntimeExecution(AgentRuntimeExecutionCompleteCommand{
			Owner: claim.Step.LeaseOwner, UserID: claim.Run.UserID, RunID: claim.Run.ID, StepID: claim.Step.ID, AttemptID: claim.Attempt.ID,
			ExpectedRunRevision: claim.Run.Revision, ExpectedStepRevision: claim.Step.Revision, ExpectedAttemptRevision: claim.Attempt.Revision,
			ResponseJSON: `{"schemaVersion":1,"summary":"replay","artifacts":[]}`,
			Event:        AgentRuntimeEventInput{ID: "ecommerce-complete-replay", EventType: "attempt.succeeded", ActorType: "executor", ActorID: "ecommerce-executor"},
			At:           now.Add(3 * time.Second),
		}); !errors.Is(err, ErrAgentRuntimeStateConflict) && !errors.Is(err, ErrAgentRuntimeLeaseLost) {
			t.Fatalf("replayed completion error = %v, want a fencing conflict", err)
		}

		detail, err := repo.AgentRuntimeDetailForUser(bundle.Run.UserID, bundle.Run.ID)
		if err != nil {
			t.Fatalf("load ecommerce completion detail: %v", err)
		}
		if len(detail.Attempts) != 1 || detail.Attempts[0].ID != claim.Attempt.ID || detail.Attempts[0].Status != model.AgentAttemptStatusSucceeded ||
			detail.Attempts[0].ResponseJSON != `{"schemaVersion":1,"summary":"ecommerce complete","artifacts":[]}` || len(detail.Events) != 3 || detail.Events[2].Sequence != 3 || detail.Events[2].EventType != "attempt.succeeded" {
			t.Fatalf("completion replay changed append-only facts: %#v", detail)
		}
	})

	t.Run("failure", func(t *testing.T) {
		db := openAgentRuntimeTestDB(t, filepath.Join(t.TempDir(), "ecommerce-fail.db"))
		repo := New(db)
		now := time.Date(2026, time.January, 3, 12, 0, 0, 0, time.UTC)
		bundle := ecommerceAgentRuntimeTestBundle("ecommerce-fail", "ecommerce-fail-request", "ecommerce-fail-project", now)
		createAgentRuntimeTestProject(t, db, bundle.Run.ProjectID, bundle.Run.UserID)
		if err := repo.CreateAgentRuntimeBundle(bundle); err != nil {
			t.Fatalf("create ecommerce failure bundle: %v", err)
		}
		claim, err := repo.ClaimNextAgentRuntimeExecution(AgentRuntimeExecutionClaimCommand{
			Domain: "ecommerce", Owner: "ecommerce-fail-worker", LeaseDuration: time.Minute,
			AttemptID: "ecommerce-fail-attempt", TaskID: "ecommerce-fail-task", EventID: "ecommerce-fail-claim",
			Executor: "ecommerce-executor", At: now.Add(time.Second),
		})
		if err != nil || claim == nil {
			t.Fatalf("claim ecommerce failure execution: claim=%#v error=%v", claim, err)
		}
		failed, err := repo.FailAgentRuntimeExecution(AgentRuntimeExecutionFailCommand{
			Owner: claim.Step.LeaseOwner, UserID: claim.Run.UserID, RunID: claim.Run.ID, StepID: claim.Step.ID, AttemptID: claim.Attempt.ID,
			ExpectedRunRevision: claim.Run.Revision, ExpectedStepRevision: claim.Step.Revision, ExpectedAttemptRevision: claim.Attempt.Revision,
			FailureCode: "ecommerce_provider_timeout", Failure: "provider request timed out",
			Event:       AgentRuntimeEventInput{ID: "ecommerce-fail-event", EventType: "attempt.failed", ActorType: "executor", ActorID: "ecommerce-executor"},
			At:          now.Add(2 * time.Second),
		})
		if err != nil {
			t.Fatalf("fail ecommerce execution: %v", err)
		}
		if failed == nil || failed.Run.Status != model.AgentRunStatusFailed || failed.Step.Status != model.AgentStepStatusFailed || failed.Attempt.Status != model.AgentAttemptStatusFailed || failed.Attempt.FailureCode != "ecommerce_provider_timeout" {
			t.Fatalf("failure did not persist terminal state: %#v", failed)
		}

		if _, err := repo.FailAgentRuntimeExecution(AgentRuntimeExecutionFailCommand{
			Owner: claim.Step.LeaseOwner, UserID: claim.Run.UserID, RunID: claim.Run.ID, StepID: claim.Step.ID, AttemptID: claim.Attempt.ID,
			ExpectedRunRevision: claim.Run.Revision, ExpectedStepRevision: claim.Step.Revision, ExpectedAttemptRevision: claim.Attempt.Revision,
			FailureCode: "replayed_failure", Failure: "should not overwrite",
			Event:       AgentRuntimeEventInput{ID: "ecommerce-fail-replay", EventType: "attempt.failed", ActorType: "executor", ActorID: "ecommerce-executor"},
			At:          now.Add(3 * time.Second),
		}); !errors.Is(err, ErrAgentRuntimeStateConflict) && !errors.Is(err, ErrAgentRuntimeLeaseLost) {
			t.Fatalf("replayed failure error = %v, want a fencing conflict", err)
		}

		detail, err := repo.AgentRuntimeDetailForUser(bundle.Run.UserID, bundle.Run.ID)
		if err != nil {
			t.Fatalf("load ecommerce failure detail: %v", err)
		}
		if len(detail.Attempts) != 1 || detail.Attempts[0].ID != claim.Attempt.ID || detail.Attempts[0].Status != model.AgentAttemptStatusFailed ||
			detail.Attempts[0].FailureCode != "ecommerce_provider_timeout" || detail.Attempts[0].Failure != "provider request timed out" || len(detail.Events) != 3 || detail.Events[2].Sequence != 3 || detail.Events[2].EventType != "attempt.failed" {
			t.Fatalf("failure replay changed append-only facts: %#v", detail)
		}
	})
}

func ecommerceAgentRuntimeTestBundle(runID string, idempotencyKey string, projectID string, now time.Time) AgentRuntimeCreateBundle {
	bundle := agentRuntimeTestBundle(runID, idempotencyKey, now)
	bundle.Run.ProjectID = projectID
	bundle.Run.Domain = "ecommerce"
	bundle.Run.RegistryID = "ecommerce-agent-team"
	bundle.Run.RegistryVersion = "1.0.0"
	bundle.Run.RouteKind = "ecommerce"
	bundle.Run.IntentRouteID = "EC-01"
	bundle.Run.Objective = "produce ecommerce creative"
	bundle.Steps[0].RouteKind = "ecommerce"
	bundle.Steps[0].RouteID = "EC-01"
	bundle.Steps[0].AgentID = "product_intelligence"
	bundle.RoutingDecision.RouteKind = "ecommerce"
	bundle.RoutingDecision.RouteID = "EC-01"
	bundle.RoutingDecision.IntentRouteID = "EC-01"
	bundle.RoutingDecision.SelectedAgentID = bundle.Steps[0].AgentID
	return bundle
}
