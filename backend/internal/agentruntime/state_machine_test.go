package agentruntime

import (
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestRunStateMachineAllowsRecoveryWithoutTerminalRegression(t *testing.T) {
	allowed := [][2]model.AgentRuntimeRunStatus{
		{model.AgentRunStatusPlanning, model.AgentRunStatusReady},
		{model.AgentRunStatusReady, model.AgentRunStatusRunning},
		{model.AgentRunStatusRunning, model.AgentRunStatusAwaitingHuman},
		{model.AgentRunStatusAwaitingHuman, model.AgentRunStatusRunning},
		{model.AgentRunStatusRunning, model.AgentRunStatusFailed},
		{model.AgentRunStatusFailed, model.AgentRunStatusReady},
		{model.AgentRunStatusRunning, model.AgentRunStatusCompleted},
	}
	for _, transition := range allowed {
		if !CanTransitionRun(transition[0], transition[1]) {
			t.Errorf("expected run transition %s -> %s", transition[0], transition[1])
		}
	}
	rejected := [][2]model.AgentRuntimeRunStatus{
		{model.AgentRunStatusPlanning, model.AgentRunStatusCompleted},
		{model.AgentRunStatusCompleted, model.AgentRunStatusRunning},
		{model.AgentRunStatusCancelled, model.AgentRunStatusReady},
	}
	for _, transition := range rejected {
		if CanTransitionRun(transition[0], transition[1]) {
			t.Errorf("unexpected run transition %s -> %s", transition[0], transition[1])
		}
	}
}

func TestStepAndAttemptStateMachinesKeepRetriesAppendOnly(t *testing.T) {
	if !CanTransitionStep(model.AgentStepStatusFailed, model.AgentStepStatusReady) {
		t.Fatal("failed step must be retryable")
	}
	if CanTransitionStep(model.AgentStepStatusCompleted, model.AgentStepStatusRunning) {
		t.Fatal("completed step must not be reopened")
	}
	if !CanTransitionAttempt(model.AgentAttemptStatusQueued, model.AgentAttemptStatusRunning) ||
		!CanTransitionAttempt(model.AgentAttemptStatusRunning, model.AgentAttemptStatusSucceeded) {
		t.Fatal("attempt forward lifecycle is incomplete")
	}
	if CanTransitionAttempt(model.AgentAttemptStatusFailed, model.AgentAttemptStatusQueued) ||
		CanTransitionAttempt(model.AgentAttemptStatusSucceeded, model.AgentAttemptStatusRunning) {
		t.Fatal("terminal attempt must not be reused for retry")
	}
}

func TestLockedArtifactAndResolvedDecisionCannotRegress(t *testing.T) {
	if CanTransitionArtifact(model.ProductionArtifactStatusLocked, model.ProductionArtifactStatusDraft) {
		t.Fatal("locked artifact revision must not return to draft")
	}
	if !CanTransitionArtifact(model.ProductionArtifactStatusLocked, model.ProductionArtifactStatusSuperseded) {
		t.Fatal("locked artifact must support superseding with a new revision")
	}
	if !CanTransitionDecision(model.AgentHumanDecisionStatusPending, model.AgentHumanDecisionStatusResolved) {
		t.Fatal("pending decision must be resolvable")
	}
	if CanTransitionDecision(model.AgentHumanDecisionStatusResolved, model.AgentHumanDecisionStatusPending) {
		t.Fatal("resolved decision must not return to pending")
	}
}
