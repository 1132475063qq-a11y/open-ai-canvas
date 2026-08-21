package agentruntime

import "infinite-canvas/backend/internal/model"

func CanTransitionRun(current model.AgentRuntimeRunStatus, next model.AgentRuntimeRunStatus) bool {
	if current == next {
		return true
	}
	allowed := map[model.AgentRuntimeRunStatus]map[model.AgentRuntimeRunStatus]bool{
		model.AgentRunStatusPlanning: {
			model.AgentRunStatusReady: true, model.AgentRunStatusAwaitingHuman: true,
			model.AgentRunStatusFailed: true, model.AgentRunStatusCancelled: true,
		},
		model.AgentRunStatusReady: {
			model.AgentRunStatusRunning: true, model.AgentRunStatusAwaitingHuman: true,
			model.AgentRunStatusFailed: true, model.AgentRunStatusCancelled: true,
		},
		model.AgentRunStatusRunning: {
			model.AgentRunStatusReady: true, model.AgentRunStatusAwaitingHuman: true, model.AgentRunStatusCompleted: true,
			model.AgentRunStatusFailed: true, model.AgentRunStatusCancelled: true,
		},
		model.AgentRunStatusAwaitingHuman: {
			model.AgentRunStatusReady: true, model.AgentRunStatusRunning: true,
			model.AgentRunStatusFailed: true, model.AgentRunStatusCancelled: true,
		},
		model.AgentRunStatusFailed: {
			model.AgentRunStatusReady: true, model.AgentRunStatusCancelled: true,
		},
	}
	return allowed[current][next]
}

func CanTransitionStep(current model.AgentRuntimeStepStatus, next model.AgentRuntimeStepStatus) bool {
	if current == next {
		return true
	}
	allowed := map[model.AgentRuntimeStepStatus]map[model.AgentRuntimeStepStatus]bool{
		model.AgentStepStatusPlanned: {
			model.AgentStepStatusReady: true, model.AgentStepStatusCancelled: true,
			model.AgentStepStatusSkipped: true,
		},
		model.AgentStepStatusReady: {
			model.AgentStepStatusRunning: true, model.AgentStepStatusAwaitingHuman: true,
			model.AgentStepStatusCancelled: true, model.AgentStepStatusSkipped: true,
		},
		model.AgentStepStatusRunning: {
			model.AgentStepStatusAwaitingHuman: true, model.AgentStepStatusCompleted: true,
			model.AgentStepStatusFailed: true, model.AgentStepStatusCancelled: true,
		},
		model.AgentStepStatusAwaitingHuman: {
			model.AgentStepStatusReady: true, model.AgentStepStatusRunning: true,
			model.AgentStepStatusFailed: true, model.AgentStepStatusCancelled: true,
		},
		model.AgentStepStatusFailed: {
			model.AgentStepStatusReady: true, model.AgentStepStatusRunning: true,
			model.AgentStepStatusCancelled: true,
		},
	}
	return allowed[current][next]
}

func CanTransitionAttempt(current model.AgentRuntimeAttemptStatus, next model.AgentRuntimeAttemptStatus) bool {
	if current == next {
		return true
	}
	allowed := map[model.AgentRuntimeAttemptStatus]map[model.AgentRuntimeAttemptStatus]bool{
		model.AgentAttemptStatusQueued: {
			model.AgentAttemptStatusRunning: true, model.AgentAttemptStatusFailed: true,
			model.AgentAttemptStatusCancelled: true,
		},
		model.AgentAttemptStatusRunning: {
			model.AgentAttemptStatusSucceeded: true, model.AgentAttemptStatusFailed: true,
			model.AgentAttemptStatusCancelled: true,
		},
	}
	return allowed[current][next]
}

func CanTransitionArtifact(current model.ProductionArtifactStatus, next model.ProductionArtifactStatus) bool {
	if current == next {
		return true
	}
	allowed := map[model.ProductionArtifactStatus]map[model.ProductionArtifactStatus]bool{
		model.ProductionArtifactStatusDraft: {
			model.ProductionArtifactStatusReview: true, model.ProductionArtifactStatusLocked: true,
			model.ProductionArtifactStatusSuperseded: true, model.ProductionArtifactStatusArchived: true,
		},
		model.ProductionArtifactStatusReview: {
			model.ProductionArtifactStatusDraft: true, model.ProductionArtifactStatusLocked: true,
			model.ProductionArtifactStatusSuperseded: true, model.ProductionArtifactStatusArchived: true,
		},
		model.ProductionArtifactStatusLocked: {
			model.ProductionArtifactStatusSuperseded: true, model.ProductionArtifactStatusArchived: true,
		},
		model.ProductionArtifactStatusSuperseded: {
			model.ProductionArtifactStatusArchived: true,
		},
	}
	return allowed[current][next]
}

func CanTransitionDecision(current model.AgentHumanDecisionStatus, next model.AgentHumanDecisionStatus) bool {
	if current == next {
		return true
	}
	return current == model.AgentHumanDecisionStatusPending &&
		(next == model.AgentHumanDecisionStatusResolved || next == model.AgentHumanDecisionStatusCancelled)
}

func IsTerminalRun(status model.AgentRuntimeRunStatus) bool {
	return status == model.AgentRunStatusCompleted || status == model.AgentRunStatusCancelled
}

func IsTerminalStep(status model.AgentRuntimeStepStatus) bool {
	return status == model.AgentStepStatusCompleted || status == model.AgentStepStatusCancelled || status == model.AgentStepStatusSkipped
}

func IsTerminalAttempt(status model.AgentRuntimeAttemptStatus) bool {
	return status == model.AgentAttemptStatusSucceeded || status == model.AgentAttemptStatusFailed || status == model.AgentAttemptStatusCancelled
}
