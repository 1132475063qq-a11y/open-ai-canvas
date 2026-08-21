package model

import "time"

type AgentRuntimeRunStatus string
type AgentRuntimeStepStatus string
type AgentRuntimeAttemptStatus string
type ProductionArtifactStatus string
type AgentHumanDecisionStatus string
type AgentHandoffTriggerStatus string

const (
	AgentRunStatusPlanning      AgentRuntimeRunStatus = "planning"
	AgentRunStatusReady         AgentRuntimeRunStatus = "ready"
	AgentRunStatusRunning       AgentRuntimeRunStatus = "running"
	AgentRunStatusAwaitingHuman AgentRuntimeRunStatus = "awaiting_human"
	AgentRunStatusCompleted     AgentRuntimeRunStatus = "completed"
	AgentRunStatusFailed        AgentRuntimeRunStatus = "failed"
	AgentRunStatusCancelled     AgentRuntimeRunStatus = "cancelled"

	AgentStepStatusPlanned       AgentRuntimeStepStatus = "planned"
	AgentStepStatusReady         AgentRuntimeStepStatus = "ready"
	AgentStepStatusRunning       AgentRuntimeStepStatus = "running"
	AgentStepStatusAwaitingHuman AgentRuntimeStepStatus = "awaiting_human"
	AgentStepStatusCompleted     AgentRuntimeStepStatus = "completed"
	AgentStepStatusFailed        AgentRuntimeStepStatus = "failed"
	AgentStepStatusCancelled     AgentRuntimeStepStatus = "cancelled"
	AgentStepStatusSkipped       AgentRuntimeStepStatus = "skipped"

	AgentAttemptStatusQueued    AgentRuntimeAttemptStatus = "queued"
	AgentAttemptStatusRunning   AgentRuntimeAttemptStatus = "running"
	AgentAttemptStatusSucceeded AgentRuntimeAttemptStatus = "succeeded"
	AgentAttemptStatusFailed    AgentRuntimeAttemptStatus = "failed"
	AgentAttemptStatusCancelled AgentRuntimeAttemptStatus = "cancelled"

	ProductionArtifactStatusDraft      ProductionArtifactStatus = "draft"
	ProductionArtifactStatusReview     ProductionArtifactStatus = "review"
	ProductionArtifactStatusLocked     ProductionArtifactStatus = "locked"
	ProductionArtifactStatusSuperseded ProductionArtifactStatus = "superseded"
	ProductionArtifactStatusArchived   ProductionArtifactStatus = "archived"

	AgentHumanDecisionStatusPending   AgentHumanDecisionStatus = "pending"
	AgentHumanDecisionStatusResolved  AgentHumanDecisionStatus = "resolved"
	AgentHumanDecisionStatusCancelled AgentHumanDecisionStatus = "cancelled"

	AgentHandoffTriggerStatusPending    AgentHandoffTriggerStatus = "pending"
	AgentHandoffTriggerStatusProcessing AgentHandoffTriggerStatus = "processing"
	AgentHandoffTriggerStatusCompleted  AgentHandoffTriggerStatus = "completed"
	AgentHandoffTriggerStatusFailed     AgentHandoffTriggerStatus = "failed"
)

// AgentRuntimeRun is the durable orchestration root. Provider credentials and
// mutable canvas documents must never be stored in InputJSON.
type AgentRuntimeRun struct {
	ID              string                `json:"id" gorm:"primaryKey;size:36"`
	UserID          string                `json:"userId" gorm:"index;size:36;uniqueIndex:idx_agent_runs_user_idempotency,priority:1"`
	ProjectID       string                `json:"projectId" gorm:"index;size:36"`
	CanvasID        string                `json:"canvasId,omitempty" gorm:"index;size:80"`
	Domain          string                `json:"domain" gorm:"index;size:32"`
	RegistryID      string                `json:"registryId" gorm:"size:80"`
	RegistryVersion string                `json:"registryVersion" gorm:"size:32"`
	RegistryDigest  string                `json:"registryDigest" gorm:"size:64"`
	RouteKind       string                `json:"routeKind" gorm:"index;size:24"`
	IntentRouteID   string                `json:"intentRouteId" gorm:"index;size:24"`
	HandoffRouteID  string                `json:"handoffRouteId,omitempty" gorm:"index;size:24"`
	RootRunID       string                `json:"rootRunId" gorm:"index;size:36"`
	ParentRunID     string                `json:"parentRunId,omitempty" gorm:"index;size:36"`
	Status          AgentRuntimeRunStatus `json:"status" gorm:"index;size:24"`
	Objective       string                `json:"objective" gorm:"type:text"`
	InputJSON       string                `json:"inputJson" gorm:"type:text"`
	CurrentStepID   string                `json:"currentStepId,omitempty" gorm:"index;size:36"`
	IdempotencyKey  string                `json:"idempotencyKey" gorm:"size:128;uniqueIndex:idx_agent_runs_user_idempotency,priority:2"`
	Revision        int64                 `json:"revision"`
	EventSequence   int64                 `json:"eventSequence"`
	FailureCode     string                `json:"failureCode,omitempty" gorm:"size:80"`
	Failure         string                `json:"failure,omitempty" gorm:"type:text"`
	StartedAt       *time.Time            `json:"startedAt,omitempty"`
	CompletedAt     *time.Time            `json:"completedAt,omitempty"`
	CreatedAt       time.Time             `json:"createdAt" gorm:"index"`
	UpdatedAt       time.Time             `json:"updatedAt" gorm:"index"`
}

// AgentRuntimeStep is a planned Agent/Skill invocation or deterministic
// orchestration action. StepKey remains stable across retries.
type AgentRuntimeStep struct {
	ID                              string                 `json:"id" gorm:"primaryKey;size:36"`
	RunID                           string                 `json:"runId" gorm:"index;size:36;uniqueIndex:idx_agent_steps_run_key,priority:1"`
	StepKey                         string                 `json:"stepKey" gorm:"size:120;uniqueIndex:idx_agent_steps_run_key,priority:2"`
	Position                        int                    `json:"position" gorm:"index"`
	RouteKind                       string                 `json:"routeKind" gorm:"size:24"`
	RouteID                         string                 `json:"routeId" gorm:"index;size:24"`
	AgentID                         string                 `json:"agentId" gorm:"index;size:80"`
	SkillIDsJSON                    string                 `json:"skillIdsJson" gorm:"type:text"`
	Status                          AgentRuntimeStepStatus `json:"status" gorm:"index;size:24"`
	DependsOnStepIDsJSON            string                 `json:"dependsOnStepIdsJson" gorm:"type:text"`
	InputArtifactRefsJSON           string                 `json:"inputArtifactRefsJson" gorm:"type:text"`
	ExpectedOutputArtifactTypesJSON string                 `json:"expectedOutputArtifactTypesJson" gorm:"type:text"`
	OutputArtifactRefsJSON          string                 `json:"outputArtifactRefsJson" gorm:"type:text"`
	AttemptSequence                 int                    `json:"attemptSequence"`
	LeaseOwner                      string                 `json:"-" gorm:"index;size:120"`
	LeaseExpiresAt                  *time.Time             `json:"-" gorm:"index"`
	Revision                        int64                  `json:"revision"`
	FailureCode                     string                 `json:"failureCode,omitempty" gorm:"size:80"`
	Failure                         string                 `json:"failure,omitempty" gorm:"type:text"`
	StartedAt                       *time.Time             `json:"startedAt,omitempty"`
	CompletedAt                     *time.Time             `json:"completedAt,omitempty"`
	CreatedAt                       time.Time              `json:"createdAt"`
	UpdatedAt                       time.Time              `json:"updatedAt"`
}

// AgentRuntimeAttempt is append-only execution history. A paid retry creates a
// new row and never rewrites an earlier provider outcome.
type AgentRuntimeAttempt struct {
	ID           string                    `json:"id" gorm:"primaryKey;size:36"`
	RunID        string                    `json:"runId" gorm:"index;size:36"`
	StepID       string                    `json:"stepId" gorm:"index;size:36;uniqueIndex:idx_agent_attempts_step_number,priority:1"`
	Number       int                       `json:"number" gorm:"uniqueIndex:idx_agent_attempts_step_number,priority:2"`
	TaskID       string                    `json:"taskId,omitempty" gorm:"index;size:36"`
	Status       AgentRuntimeAttemptStatus `json:"status" gorm:"index;size:24"`
	Executor     string                    `json:"executor" gorm:"size:40"`
	ModelRef     string                    `json:"modelRef,omitempty" gorm:"size:160"`
	InputDigest  string                    `json:"inputDigest" gorm:"size:64"`
	PromptDigest string                    `json:"promptDigest" gorm:"size:64"`
	RequestJSON  string                    `json:"requestJson" gorm:"type:text"`
	ResponseJSON string                    `json:"responseJson" gorm:"type:text"`
	FailureCode  string                    `json:"failureCode,omitempty" gorm:"size:80"`
	Failure      string                    `json:"failure,omitempty" gorm:"type:text"`
	Revision     int64                     `json:"revision"`
	StartedAt    *time.Time                `json:"startedAt,omitempty"`
	CompletedAt  *time.Time                `json:"completedAt,omitempty"`
	CreatedAt    time.Time                 `json:"createdAt"`
	UpdatedAt    time.Time                 `json:"updatedAt"`
}

// ProductionArtifact is a stable logical identity. Its revisions are
// immutable and carry the actual content.
type ProductionArtifact struct {
	ID                string    `json:"id" gorm:"primaryKey;size:36"`
	UserID            string    `json:"userId" gorm:"index;size:36"`
	ProjectID         string    `json:"projectId" gorm:"index;size:36;uniqueIndex:idx_production_artifacts_scope_key,priority:1"`
	Domain            string    `json:"domain" gorm:"index;size:32;uniqueIndex:idx_production_artifacts_scope_key,priority:2"`
	ArtifactType      string    `json:"artifactType" gorm:"index;size:80"`
	LogicalKey        string    `json:"logicalKey" gorm:"size:160;uniqueIndex:idx_production_artifacts_scope_key,priority:3"`
	CurrentRevisionID string    `json:"currentRevisionId,omitempty" gorm:"index;size:36"`
	RevisionSequence  int       `json:"revisionSequence"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt" gorm:"index"`
}

type ProductionArtifactRevision struct {
	ID                     string                   `json:"id" gorm:"primaryKey;size:36"`
	ArtifactID             string                   `json:"artifactId" gorm:"index;size:36;uniqueIndex:idx_production_artifact_revisions_number,priority:1"`
	Version                int                      `json:"version" gorm:"uniqueIndex:idx_production_artifact_revisions_number,priority:2"`
	Status                 ProductionArtifactStatus `json:"status" gorm:"index;size:24"`
	ContentJSON            string                   `json:"contentJson" gorm:"type:text"`
	ContentText            string                   `json:"contentText" gorm:"type:text"`
	ContentDigest          string                   `json:"contentDigest" gorm:"size:64"`
	SourceRunID            string                   `json:"sourceRunId,omitempty" gorm:"index;size:36"`
	SourceStepID           string                   `json:"sourceStepId,omitempty" gorm:"index;size:36"`
	SourceAttemptID        string                   `json:"sourceAttemptId,omitempty" gorm:"index;size:36"`
	ParentRevisionID       string                   `json:"parentRevisionId,omitempty" gorm:"index;size:36"`
	SourceArtifactRefsJSON string                   `json:"sourceArtifactRefsJson" gorm:"type:text"`
	AuthorityRefsJSON      string                   `json:"authorityRefsJson" gorm:"type:text"`
	CreatedByType          string                   `json:"createdByType" gorm:"size:24"`
	CreatedByID            string                   `json:"createdById" gorm:"size:80"`
	CreatedAt              time.Time                `json:"createdAt"`
}

type AgentRoutingDecision struct {
	ID                    string    `json:"id" gorm:"primaryKey;size:36"`
	RunID                 string    `json:"runId" gorm:"index;size:36"`
	RouteKind             string    `json:"routeKind" gorm:"index;size:24"`
	RouteID               string    `json:"routeId" gorm:"index;size:24"`
	IntentRouteID         string    `json:"intentRouteId" gorm:"index;size:24"`
	SelectedAgentID       string    `json:"selectedAgentId" gorm:"size:80"`
	SelectedSkillIDsJSON  string    `json:"selectedSkillIdsJson" gorm:"type:text"`
	InputArtifactRefsJSON string    `json:"inputArtifactRefsJson" gorm:"type:text"`
	AlternativesJSON      string    `json:"alternativesJson" gorm:"type:text"`
	Reason                string    `json:"reason" gorm:"type:text"`
	Confidence            string    `json:"confidence" gorm:"size:24"`
	DecidedByType         string    `json:"decidedByType" gorm:"size:24"`
	DecidedByID           string    `json:"decidedById" gorm:"size:80"`
	CreatedAt             time.Time `json:"createdAt"`
}

type AgentHumanDecision struct {
	ID               string                   `json:"id" gorm:"primaryKey;size:36"`
	RunID            string                   `json:"runId" gorm:"index;size:36"`
	StepID           string                   `json:"stepId,omitempty" gorm:"index;size:36"`
	Status           AgentHumanDecisionStatus `json:"status" gorm:"index;size:24"`
	Question         string                   `json:"question" gorm:"type:text"`
	OptionsJSON      string                   `json:"optionsJson" gorm:"type:text"`
	Recommendation   string                   `json:"recommendation" gorm:"type:text"`
	ResponseJSON     string                   `json:"responseJson" gorm:"type:text"`
	ImpactRefsJSON   string                   `json:"impactRefsJson" gorm:"type:text"`
	Revision         int64                    `json:"revision"`
	ResolvedByUserID string                   `json:"resolvedByUserId,omitempty" gorm:"size:36"`
	ResolvedAt       *time.Time               `json:"resolvedAt,omitempty"`
	CreatedAt        time.Time                `json:"createdAt"`
	UpdatedAt        time.Time                `json:"updatedAt"`
}

// AgentRuntimeEvent is append-only evidence for state changes and execution
// facts. Sequence is allocated transactionally per Run.
type AgentRuntimeEvent struct {
	ID          string    `json:"id" gorm:"primaryKey;size:36"`
	UserID      string    `json:"userId" gorm:"index;size:36"`
	RunID       string    `json:"runId" gorm:"index;size:36;uniqueIndex:idx_agent_runtime_events_run_sequence,priority:1"`
	Sequence    int64     `json:"sequence" gorm:"uniqueIndex:idx_agent_runtime_events_run_sequence,priority:2"`
	StepID      string    `json:"stepId,omitempty" gorm:"index;size:36"`
	AttemptID   string    `json:"attemptId,omitempty" gorm:"index;size:36"`
	EventType   string    `json:"eventType" gorm:"index;size:80"`
	ActorType   string    `json:"actorType" gorm:"size:24"`
	ActorID     string    `json:"actorId" gorm:"size:80"`
	FromStatus  string    `json:"fromStatus,omitempty" gorm:"size:24"`
	ToStatus    string    `json:"toStatus,omitempty" gorm:"size:24"`
	PayloadJSON string    `json:"payloadJson" gorm:"type:text"`
	CreatedAt   time.Time `json:"createdAt" gorm:"index"`
}

// AgentHandoffTrigger is a durable outbox fact created in the same transaction
// as a user-approved locked Artifact revision. Processing may be retried without
// creating duplicate Handoff Runs because each Run has a deterministic
// idempotency key derived from its route and immutable input revisions.
type AgentHandoffTrigger struct {
	ID                  string                    `json:"id" gorm:"primaryKey;size:36"`
	UserID              string                    `json:"userId" gorm:"index;size:36"`
	ProjectID           string                    `json:"projectId" gorm:"index;size:36"`
	Domain              string                    `json:"domain" gorm:"index;size:32"`
	RootRunID           string                    `json:"rootRunId" gorm:"index;size:36"`
	ArtifactID          string                    `json:"artifactId" gorm:"index;size:36"`
	RevisionID          string                    `json:"revisionId" gorm:"uniqueIndex;size:36"`
	SourceRunID         string                    `json:"sourceRunId" gorm:"index;size:36"`
	SourceStepID        string                    `json:"sourceStepId" gorm:"index;size:36"`
	SourceAgentID       string                    `json:"sourceAgentId" gorm:"index;size:80"`
	Status              AgentHandoffTriggerStatus `json:"status" gorm:"index;size:24"`
	AttemptCount        int                       `json:"attemptCount"`
	LeaseOwner          string                    `json:"-" gorm:"index;size:120"`
	LeaseExpiresAt      *time.Time                `json:"-" gorm:"index"`
	NextAttemptAt       *time.Time                `json:"nextAttemptAt,omitempty" gorm:"index"`
	ScheduledRunIDsJSON string                    `json:"scheduledRunIdsJson" gorm:"type:text"`
	FailureCode         string                    `json:"failureCode,omitempty" gorm:"size:80"`
	Failure             string                    `json:"failure,omitempty" gorm:"type:text"`
	Revision            int64                     `json:"revision"`
	CompletedAt         *time.Time                `json:"completedAt,omitempty"`
	CreatedAt           time.Time                 `json:"createdAt" gorm:"index"`
	UpdatedAt           time.Time                 `json:"updatedAt" gorm:"index"`
}
