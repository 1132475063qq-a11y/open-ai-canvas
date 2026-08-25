package model

import "time"

type FilmProductionQuoteStatus string
type FilmProductionAttemptStatus string
type FilmProductionQCDecision string
type FilmProductionQCAction string

const (
	FilmProductionQuoteStatusPending  FilmProductionQuoteStatus = "pending"
	FilmProductionQuoteStatusConsumed FilmProductionQuoteStatus = "consumed"
	FilmProductionQuoteStatusExpired  FilmProductionQuoteStatus = "expired"

	FilmProductionAttemptStatusQueued    FilmProductionAttemptStatus = "queued"
	FilmProductionAttemptStatusRunning   FilmProductionAttemptStatus = "running"
	FilmProductionAttemptStatusSucceeded FilmProductionAttemptStatus = "succeeded"
	FilmProductionAttemptStatusFailed    FilmProductionAttemptStatus = "failed"
	FilmProductionAttemptStatusCancelled FilmProductionAttemptStatus = "cancelled"
	FilmProductionAttemptStatusUncertain FilmProductionAttemptStatus = "uncertain"

	FilmProductionQCDecisionPass          FilmProductionQCDecision = "PASS"
	FilmProductionQCDecisionUncertain     FilmProductionQCDecision = "UNCERTAIN"
	FilmProductionQCDecisionFail          FilmProductionQCDecision = "FAIL"
	FilmProductionQCDecisionNotAssessable FilmProductionQCDecision = "NOT_ASSESSABLE"

	FilmProductionQCActionAccept FilmProductionQCAction = "accept"
	FilmProductionQCActionRetry  FilmProductionQCAction = "retry"
	FilmProductionQCActionHold   FilmProductionQCAction = "hold"

	ResultKindFilmGeneration     = "film_generation_result"
	ResultAvailabilityReady      = "AVAILABLE"
	ResultAvailabilityUnknown    = "UNKNOWN"
	FilmAgentTaskType            = "canvas_text_film_agent"
	FilmProductionTaskTypeImage  = "canvas_image_film_production"
	FilmProductionTaskTypeVideo  = "canvas_video_film_production"
	FilmVisualQCTaskTypeImage    = "canvas_text_film_visual_qc"
	FilmVisualQCTaskTypeVideo    = "canvas_text_film_video_visual_qc"
	FilmVisualQCTaskTypeSequence = "canvas_text_film_video_sequence_visual_qc"
)

func IsFilmProductionTaskType(taskType string) bool {
	return taskType == FilmProductionTaskTypeImage || taskType == FilmProductionTaskTypeVideo
}

func IsFilmAgentTaskType(taskType string) bool {
	return taskType == FilmAgentTaskType
}

// IsFilmManagedTaskType identifies Film tasks whose route, billing and retry
// identity is frozen by a domain Quote rather than the generic task controls.
func IsFilmManagedTaskType(taskType string) bool {
	return IsFilmProductionTaskType(taskType) || taskType == FilmVisualQCTaskTypeImage || taskType == FilmVisualQCTaskTypeVideo || taskType == FilmVisualQCTaskTypeSequence
}

// FilmProductionQuote freezes a provider-neutral image request, its selected
// logical route, and its price. It never stores provider credentials.
type FilmProductionQuote struct {
	ID                     string                    `json:"id" gorm:"primaryKey;size:36"`
	UserID                 string                    `json:"userId" gorm:"index;size:36;uniqueIndex:idx_film_quote_user_idempotency,priority:1"`
	IdempotencyKey         string                    `json:"-" gorm:"size:128;uniqueIndex:idx_film_quote_user_idempotency,priority:2"`
	ProjectID              string                    `json:"projectId" gorm:"index;size:36"`
	RootRunID              string                    `json:"rootRunId" gorm:"index;size:36"`
	ShotID                 string                    `json:"shotId" gorm:"index;size:36"`
	RetryOfAttemptID       string                    `json:"retryOfAttemptId,omitempty" gorm:"index;size:36"`
	TaskID                 string                    `json:"-" gorm:"uniqueIndex;size:36"`
	StoryboardArtifactID   string                    `json:"storyboardArtifactId" gorm:"index;size:36"`
	StoryboardRevisionID   string                    `json:"storyboardRevisionId" gorm:"index;size:36"`
	StoryboardDigest       string                    `json:"storyboardDigest" gorm:"size:64"`
	PromptArtifactID       string                    `json:"promptArtifactId" gorm:"index;size:36"`
	PromptRevisionID       string                    `json:"promptRevisionId" gorm:"index;size:36"`
	PromptDigest           string                    `json:"promptDigest" gorm:"size:64"`
	FeasibilityArtifactID  string                    `json:"feasibilityArtifactId" gorm:"index;size:36"`
	FeasibilityRevisionID  string                    `json:"feasibilityRevisionId" gorm:"index;size:36"`
	FeasibilityDigest      string                    `json:"feasibilityDigest" gorm:"size:64"`
	RegistryID             string                    `json:"registryId" gorm:"size:80"`
	RegistryVersion        string                    `json:"registryVersion" gorm:"size:32"`
	RegistryDigest         string                    `json:"registryDigest" gorm:"size:64"`
	LogicalModelID         string                    `json:"logicalModelId" gorm:"index;size:36"`
	LogicalModelRevisionID string                    `json:"logicalModelRevisionId" gorm:"index;size:36"`
	RouteID                string                    `json:"-" gorm:"index;size:36"`
	ChannelID              string                    `json:"-" gorm:"index;size:36"`
	ChannelModelID         string                    `json:"-" gorm:"index;size:36"`
	Model                  string                    `json:"model" gorm:"size:120"`
	ProviderModel          string                    `json:"-" gorm:"size:120"`
	Capability             string                    `json:"capability" gorm:"size:32"`
	Protocol               ChannelInterfaceType      `json:"-" gorm:"size:40"`
	CapabilityVersion      int64                     `json:"capabilityVersion"`
	ChannelPriceVersion    int64                     `json:"channelPriceVersion"`
	RequestJSON            string                    `json:"-" gorm:"type:text"`
	RequestFingerprint     string                    `json:"requestFingerprint" gorm:"size:64"`
	BillingJSON            string                    `json:"-" gorm:"type:text"`
	QuoteFingerprint       string                    `json:"quoteFingerprint" gorm:"size:64"`
	Status                 FilmProductionQuoteStatus `json:"status" gorm:"index;size:24"`
	ExpiresAt              time.Time                 `json:"expiresAt" gorm:"index"`
	ConsumedAt             *time.Time                `json:"consumedAt,omitempty"`
	CreatedAt              time.Time                 `json:"createdAt" gorm:"index"`
	UpdatedAt              time.Time                 `json:"updatedAt"`
}

// FilmProductionAttempt is the append-only paid execution identity. Worker
// lease recovery may update its state, but a user retry always creates a new
// Task and a new Attempt row.
type FilmProductionAttempt struct {
	ID                     string                      `json:"id" gorm:"primaryKey;size:36"`
	UserID                 string                      `json:"userId" gorm:"index;size:36;uniqueIndex:idx_film_attempt_user_idempotency,priority:1"`
	ProjectID              string                      `json:"projectId" gorm:"index;size:36"`
	RootRunID              string                      `json:"rootRunId" gorm:"index;size:36;uniqueIndex:idx_film_attempt_scope_number,priority:1"`
	ShotID                 string                      `json:"shotId" gorm:"index;size:36;uniqueIndex:idx_film_attempt_scope_number,priority:2"`
	Number                 int                         `json:"number" gorm:"uniqueIndex:idx_film_attempt_scope_number,priority:3"`
	RetryOfAttemptID       string                      `json:"retryOfAttemptId,omitempty" gorm:"index;size:36"`
	QuoteID                string                      `json:"quoteId" gorm:"uniqueIndex;size:36"`
	TaskID                 string                      `json:"taskId" gorm:"uniqueIndex;size:36"`
	BillingOrderID         string                      `json:"billingOrderId,omitempty" gorm:"index;size:36"`
	IdempotencyKey         string                      `json:"idempotencyKey" gorm:"size:128;uniqueIndex:idx_film_attempt_user_idempotency,priority:2"`
	StoryboardArtifactID   string                      `json:"storyboardArtifactId" gorm:"index;size:36"`
	StoryboardRevisionID   string                      `json:"storyboardRevisionId" gorm:"index;size:36"`
	StoryboardDigest       string                      `json:"storyboardDigest" gorm:"size:64"`
	PromptArtifactID       string                      `json:"promptArtifactId" gorm:"index;size:36"`
	PromptRevisionID       string                      `json:"promptRevisionId" gorm:"index;size:36"`
	PromptDigest           string                      `json:"promptDigest" gorm:"size:64"`
	FeasibilityArtifactID  string                      `json:"feasibilityArtifactId" gorm:"index;size:36"`
	FeasibilityRevisionID  string                      `json:"feasibilityRevisionId" gorm:"index;size:36"`
	FeasibilityDigest      string                      `json:"feasibilityDigest" gorm:"size:64"`
	RegistryID             string                      `json:"registryId" gorm:"size:80"`
	RegistryVersion        string                      `json:"registryVersion" gorm:"size:32"`
	RegistryDigest         string                      `json:"registryDigest" gorm:"size:64"`
	LogicalModelID         string                      `json:"logicalModelId" gorm:"index;size:36"`
	LogicalModelRevisionID string                      `json:"logicalModelRevisionId" gorm:"index;size:36"`
	RouteID                string                      `json:"-" gorm:"index;size:36"`
	ChannelID              string                      `json:"-" gorm:"index;size:36"`
	ChannelModelID         string                      `json:"-" gorm:"index;size:36"`
	Model                  string                      `json:"model" gorm:"size:120"`
	ProviderModel          string                      `json:"-" gorm:"size:120"`
	Capability             string                      `json:"capability" gorm:"size:32"`
	Protocol               ChannelInterfaceType        `json:"-" gorm:"size:40"`
	CapabilityVersion      int64                       `json:"capabilityVersion"`
	ChannelPriceVersion    int64                       `json:"channelPriceVersion"`
	RequestFingerprint     string                      `json:"requestFingerprint" gorm:"size:64"`
	AttemptArtifactID      string                      `json:"attemptArtifactId" gorm:"index;size:36"`
	AttemptRevisionID      string                      `json:"attemptRevisionId" gorm:"index;size:36"`
	ResultID               string                      `json:"resultId,omitempty" gorm:"index;size:36"`
	ResultArtifactID       string                      `json:"resultArtifactId,omitempty" gorm:"index;size:36"`
	ResultRevisionID       string                      `json:"resultRevisionId,omitempty" gorm:"index;size:36"`
	Status                 FilmProductionAttemptStatus `json:"status" gorm:"index;size:24"`
	Error                  string                      `json:"error,omitempty" gorm:"type:text"`
	StartedAt              *time.Time                  `json:"startedAt,omitempty"`
	CompletedAt            *time.Time                  `json:"completedAt,omitempty"`
	CreatedAt              time.Time                   `json:"createdAt" gorm:"index"`
	UpdatedAt              time.Time                   `json:"updatedAt"`
}

// FilmProductionQCReport is append-only. The newest human row is the current
// acceptance fact; automated NOT_ASSESSABLE never implies acceptance.
type FilmProductionQCReport struct {
	ID             string                   `json:"id" gorm:"primaryKey;size:36"`
	UserID         string                   `json:"userId" gorm:"index;size:36;uniqueIndex:idx_film_qc_user_idempotency,priority:1"`
	ProjectID      string                   `json:"projectId" gorm:"index;size:36"`
	RootRunID      string                   `json:"rootRunId" gorm:"index;size:36"`
	ShotID         string                   `json:"shotId" gorm:"index;size:36"`
	AttemptID      string                   `json:"attemptId" gorm:"index;size:36"`
	ResultID       string                   `json:"resultId" gorm:"index;size:36"`
	Decision       FilmProductionQCDecision `json:"decision" gorm:"index;size:24"`
	Action         FilmProductionQCAction   `json:"action" gorm:"index;size:24"`
	IssueCodesJSON string                   `json:"-" gorm:"type:text"`
	EvidenceJSON   string                   `json:"-" gorm:"type:text"`
	Note           string                   `json:"note" gorm:"type:text"`
	Source         string                   `json:"source" gorm:"index;size:24"`
	AssessmentKind string                   `json:"assessmentKind,omitempty" gorm:"index;size:32"`
	ModelAttemptID string                   `json:"modelAttemptId,omitempty" gorm:"index;size:36"`
	ReviewerUserID string                   `json:"reviewerUserId,omitempty" gorm:"index;size:36"`
	IdempotencyKey string                   `json:"idempotencyKey,omitempty" gorm:"size:128;uniqueIndex:idx_film_qc_user_idempotency,priority:2"`
	ArtifactID     string                   `json:"artifactId" gorm:"index;size:36"`
	RevisionID     string                   `json:"revisionId" gorm:"index;size:36"`
	CreatedAt      time.Time                `json:"createdAt" gorm:"index"`
}
