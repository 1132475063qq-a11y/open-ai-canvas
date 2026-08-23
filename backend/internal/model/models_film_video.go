package model

import "time"

type FilmVideoSequenceStatus string
type FilmVideoSlotStatus string
type FilmVideoSequenceReviewDecision string
type FilmVideoSequenceReviewAction string

const (
	FilmVideoSequenceStatusReady       FilmVideoSequenceStatus = "ready"
	FilmVideoSequenceStatusGenerating  FilmVideoSequenceStatus = "generating"
	FilmVideoSequenceStatusNeedsReview FilmVideoSequenceStatus = "needs_review"
	FilmVideoSequenceStatusCompleted   FilmVideoSequenceStatus = "completed"

	FilmVideoSlotStatusReady       FilmVideoSlotStatus = "ready"
	FilmVideoSlotStatusQueued      FilmVideoSlotStatus = "queued"
	FilmVideoSlotStatusRunning     FilmVideoSlotStatus = "running"
	FilmVideoSlotStatusNeedsReview FilmVideoSlotStatus = "needs_review"
	FilmVideoSlotStatusAccepted    FilmVideoSlotStatus = "accepted"
	FilmVideoSlotStatusFailed      FilmVideoSlotStatus = "failed"
	FilmVideoSlotStatusCancelled   FilmVideoSlotStatus = "cancelled"
	FilmVideoSlotStatusUncertain   FilmVideoSlotStatus = "uncertain"

	FilmVideoSequenceReviewDecisionPass      FilmVideoSequenceReviewDecision = "PASS"
	FilmVideoSequenceReviewDecisionUncertain FilmVideoSequenceReviewDecision = "UNCERTAIN"
	FilmVideoSequenceReviewDecisionFail      FilmVideoSequenceReviewDecision = "FAIL"

	FilmVideoSequenceReviewActionAccept FilmVideoSequenceReviewAction = "accept"
	FilmVideoSequenceReviewActionHold   FilmVideoSequenceReviewAction = "hold"
	FilmVideoSequenceReviewActionRetry  FilmVideoSequenceReviewAction = "retry"

	ResultKindFilmVideoGeneration = "film_video_generation_result"
)

// FilmVideoSequence is an editable ordering plan over accepted Film images.
// Provider execution and billing remain in Slot Attempts rather than this row.
type FilmVideoSequence struct {
	ID                       string                  `json:"id" gorm:"primaryKey;size:36"`
	UserID                   string                  `json:"userId" gorm:"index;size:36;uniqueIndex:idx_film_video_sequence_user_idempotency,priority:1"`
	IdempotencyKey           string                  `json:"-" gorm:"size:128;uniqueIndex:idx_film_video_sequence_user_idempotency,priority:2"`
	ProjectID                string                  `json:"projectId" gorm:"index;size:36"`
	RootRunID                string                  `json:"rootRunId" gorm:"index;size:36"`
	Title                    string                  `json:"title" gorm:"size:160"`
	AspectRatio              string                  `json:"aspectRatio" gorm:"size:24"`
	TargetDurationMs         int64                   `json:"targetDurationMs"`
	PromptArtifactID         string                  `json:"promptArtifactId" gorm:"index;size:36"`
	PromptArtifactRevisionID string                  `json:"promptArtifactRevisionId" gorm:"index;size:36"`
	PromptArtifactDigest     string                  `json:"promptArtifactDigest" gorm:"size:64"`
	RegistryID               string                  `json:"registryId" gorm:"size:80"`
	RegistryVersion          string                  `json:"registryVersion" gorm:"size:32"`
	RegistryDigest           string                  `json:"registryDigest" gorm:"size:64"`
	RequestFingerprint       string                  `json:"requestFingerprint" gorm:"size:64"`
	ArtifactID               string                  `json:"artifactId" gorm:"index;size:36"`
	ArtifactRevisionID       string                  `json:"artifactRevisionId" gorm:"index;size:36"`
	Status                   FilmVideoSequenceStatus `json:"status" gorm:"index;size:24"`
	Revision                 int64                   `json:"revision"`
	CreatedAt                time.Time               `json:"createdAt" gorm:"index"`
	UpdatedAt                time.Time               `json:"updatedAt"`
}

type FilmVideoSlot struct {
	ID                    string              `json:"id" gorm:"primaryKey;size:36"`
	UserID                string              `json:"userId" gorm:"index;size:36"`
	ProjectID             string              `json:"projectId" gorm:"index;size:36"`
	RootRunID             string              `json:"rootRunId" gorm:"index;size:36"`
	SequenceID            string              `json:"sequenceId" gorm:"index;size:36;uniqueIndex:idx_film_video_slot_sequence_position,priority:1"`
	Position              int                 `json:"position" gorm:"uniqueIndex:idx_film_video_slot_sequence_position,priority:2"`
	ShotID                string              `json:"shotId" gorm:"index;size:36"`
	Prompt                string              `json:"prompt" gorm:"type:text"`
	DurationMs            int64               `json:"durationMs"`
	SourceImageAttemptID  string              `json:"sourceImageAttemptId" gorm:"index;size:36"`
	SourceImageResultID   string              `json:"sourceImageResultId" gorm:"index;size:36"`
	SourceImageResourceID string              `json:"sourceImageResourceId" gorm:"index;size:80"`
	SourceImageArtifactID string              `json:"sourceImageArtifactId" gorm:"index;size:36"`
	SourceImageRevisionID string              `json:"sourceImageRevisionId" gorm:"index;size:36"`
	CurrentAttemptID      string              `json:"currentAttemptId,omitempty" gorm:"index;size:36"`
	ResultID              string              `json:"resultId,omitempty" gorm:"index;size:36"`
	ResultArtifactID      string              `json:"resultArtifactId,omitempty" gorm:"index;size:36"`
	ResultRevisionID      string              `json:"resultRevisionId,omitempty" gorm:"index;size:36"`
	Status                FilmVideoSlotStatus `json:"status" gorm:"index;size:24"`
	Revision              int64               `json:"revision"`
	CreatedAt             time.Time           `json:"createdAt"`
	UpdatedAt             time.Time           `json:"updatedAt"`
}

// FilmVideoSequenceReview is an append-only human verdict over the whole
// sequence. ScopeFingerprint makes a prior PASS stale as soon as a slot or
// the continuity ledger changes.
type FilmVideoSequenceReview struct {
	ID               string                          `json:"id" gorm:"primaryKey;size:36"`
	UserID           string                          `json:"userId" gorm:"index;size:36;uniqueIndex:idx_film_video_sequence_review_user_idempotency,priority:1"`
	IdempotencyKey   string                          `json:"-" gorm:"size:128;uniqueIndex:idx_film_video_sequence_review_user_idempotency,priority:2"`
	ProjectID        string                          `json:"projectId" gorm:"index;size:36"`
	RootRunID        string                          `json:"rootRunId" gorm:"index;size:36"`
	SequenceID       string                          `json:"sequenceId" gorm:"index;size:36"`
	LedgerID         string                          `json:"ledgerId" gorm:"index;size:36"`
	Decision         FilmVideoSequenceReviewDecision `json:"decision" gorm:"index;size:24"`
	Action           FilmVideoSequenceReviewAction   `json:"action" gorm:"index;size:24"`
	IssueCodesJSON   string                          `json:"-" gorm:"type:text"`
	EvidenceJSON     string                          `json:"-" gorm:"type:text"`
	Note             string                          `json:"note" gorm:"type:text"`
	Source           string                          `json:"source" gorm:"index;size:24"`
	AssessmentKind   string                          `json:"assessmentKind,omitempty" gorm:"index;size:40"`
	ModelAttemptID   string                          `json:"modelAttemptId,omitempty" gorm:"index;size:36"`
	ReviewerUserID   string                          `json:"reviewerUserId,omitempty" gorm:"index;size:36"`
	ScopeFingerprint string                          `json:"scopeFingerprint" gorm:"size:64"`
	ArtifactID       string                          `json:"artifactId" gorm:"index;size:36"`
	RevisionID       string                          `json:"revisionId" gorm:"index;size:36"`
	CreatedAt        time.Time                       `json:"createdAt" gorm:"index"`
}

// FilmVideoQuote freezes one image-to-video request and its selected price.
// It stores only a resource snapshot and never stores Provider credentials.
type FilmVideoQuote struct {
	ID                       string                    `json:"id" gorm:"primaryKey;size:36"`
	UserID                   string                    `json:"userId" gorm:"index;size:36;uniqueIndex:idx_film_video_quote_user_idempotency,priority:1"`
	IdempotencyKey           string                    `json:"-" gorm:"size:128;uniqueIndex:idx_film_video_quote_user_idempotency,priority:2"`
	ProjectID                string                    `json:"projectId" gorm:"index;size:36"`
	RootRunID                string                    `json:"rootRunId" gorm:"index;size:36"`
	SequenceID               string                    `json:"sequenceId" gorm:"index;size:36"`
	SequenceRevision         int64                     `json:"sequenceRevision"`
	SlotID                   string                    `json:"slotId" gorm:"index;size:36"`
	SlotRevision             int64                     `json:"slotRevision"`
	RetryOfAttemptID         string                    `json:"retryOfAttemptId,omitempty" gorm:"index;size:36"`
	TaskID                   string                    `json:"-" gorm:"uniqueIndex;size:36"`
	SourceImageAttemptID     string                    `json:"sourceImageAttemptId" gorm:"index;size:36"`
	SourceImageResultID      string                    `json:"sourceImageResultId" gorm:"index;size:36"`
	SourceImageResourceID    string                    `json:"sourceImageResourceId" gorm:"index;size:80"`
	SourceImageArtifactID    string                    `json:"sourceImageArtifactId" gorm:"index;size:36"`
	SourceImageRevisionID    string                    `json:"sourceImageRevisionId" gorm:"index;size:36"`
	PromptArtifactID         string                    `json:"promptArtifactId" gorm:"index;size:36"`
	PromptArtifactRevisionID string                    `json:"promptArtifactRevisionId" gorm:"index;size:36"`
	PromptArtifactDigest     string                    `json:"promptArtifactDigest" gorm:"size:64"`
	RegistryDigest           string                    `json:"registryDigest" gorm:"size:64"`
	LogicalModelID           string                    `json:"logicalModelId" gorm:"index;size:36"`
	LogicalModelRevisionID   string                    `json:"logicalModelRevisionId" gorm:"index;size:36"`
	RouteID                  string                    `json:"-" gorm:"index;size:36"`
	ChannelID                string                    `json:"-" gorm:"index;size:36"`
	ChannelModelID           string                    `json:"-" gorm:"index;size:36"`
	Model                    string                    `json:"model" gorm:"size:120"`
	ProviderModel            string                    `json:"-" gorm:"size:120"`
	Protocol                 ChannelInterfaceType      `json:"-" gorm:"size:40"`
	CapabilityVersion        int64                     `json:"capabilityVersion"`
	ChannelPriceVersion      int64                     `json:"channelPriceVersion"`
	RequestJSON              string                    `json:"-" gorm:"type:text"`
	RequestFingerprint       string                    `json:"requestFingerprint" gorm:"size:64"`
	BillingJSON              string                    `json:"-" gorm:"type:text"`
	QuoteFingerprint         string                    `json:"quoteFingerprint" gorm:"size:64"`
	Status                   FilmProductionQuoteStatus `json:"status" gorm:"index;size:24"`
	ExpiresAt                time.Time                 `json:"expiresAt" gorm:"index"`
	ConsumedAt               *time.Time                `json:"consumedAt,omitempty"`
	CreatedAt                time.Time                 `json:"createdAt" gorm:"index"`
	UpdatedAt                time.Time                 `json:"updatedAt"`
}

type FilmVideoAttempt struct {
	ID                     string                      `json:"id" gorm:"primaryKey;size:36"`
	UserID                 string                      `json:"userId" gorm:"index;size:36;uniqueIndex:idx_film_video_attempt_user_idempotency,priority:1"`
	IdempotencyKey         string                      `json:"idempotencyKey" gorm:"size:128;uniqueIndex:idx_film_video_attempt_user_idempotency,priority:2"`
	ProjectID              string                      `json:"projectId" gorm:"index;size:36"`
	RootRunID              string                      `json:"rootRunId" gorm:"index;size:36"`
	SequenceID             string                      `json:"sequenceId" gorm:"index;size:36"`
	SlotID                 string                      `json:"slotId" gorm:"index;size:36;uniqueIndex:idx_film_video_attempt_slot_number,priority:1"`
	Number                 int                         `json:"number" gorm:"uniqueIndex:idx_film_video_attempt_slot_number,priority:2"`
	RetryOfAttemptID       string                      `json:"retryOfAttemptId,omitempty" gorm:"index;size:36"`
	QuoteID                string                      `json:"quoteId" gorm:"uniqueIndex;size:36"`
	TaskID                 string                      `json:"taskId" gorm:"uniqueIndex;size:36"`
	BillingOrderID         string                      `json:"billingOrderId,omitempty" gorm:"index;size:36"`
	SourceImageAttemptID   string                      `json:"sourceImageAttemptId" gorm:"index;size:36"`
	SourceImageResultID    string                      `json:"sourceImageResultId" gorm:"index;size:36"`
	SourceImageResourceID  string                      `json:"sourceImageResourceId" gorm:"index;size:80"`
	SourceImageRevisionID  string                      `json:"sourceImageRevisionId" gorm:"index;size:36"`
	PromptArtifactID       string                      `json:"promptArtifactId" gorm:"index;size:36"`
	PromptRevisionID       string                      `json:"promptRevisionId" gorm:"index;size:36"`
	PromptDigest           string                      `json:"promptDigest" gorm:"size:64"`
	LogicalModelID         string                      `json:"logicalModelId" gorm:"index;size:36"`
	LogicalModelRevisionID string                      `json:"logicalModelRevisionId" gorm:"index;size:36"`
	RouteID                string                      `json:"-" gorm:"index;size:36"`
	ChannelID              string                      `json:"-" gorm:"index;size:36"`
	ChannelModelID         string                      `json:"-" gorm:"index;size:36"`
	Model                  string                      `json:"model" gorm:"size:120"`
	ProviderModel          string                      `json:"-" gorm:"size:120"`
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

type FilmVideoQCReport struct {
	ID             string                   `json:"id" gorm:"primaryKey;size:36"`
	UserID         string                   `json:"userId" gorm:"index;size:36;uniqueIndex:idx_film_video_qc_user_idempotency,priority:1"`
	IdempotencyKey string                   `json:"-" gorm:"size:128;uniqueIndex:idx_film_video_qc_user_idempotency,priority:2"`
	ProjectID      string                   `json:"projectId" gorm:"index;size:36"`
	RootRunID      string                   `json:"rootRunId" gorm:"index;size:36"`
	SequenceID     string                   `json:"sequenceId" gorm:"index;size:36"`
	SlotID         string                   `json:"slotId" gorm:"index;size:36"`
	AttemptID      string                   `json:"attemptId" gorm:"index;size:36"`
	ResultID       string                   `json:"resultId" gorm:"index;size:36"`
	Decision       FilmProductionQCDecision `json:"decision" gorm:"index;size:24"`
	Action         FilmProductionQCAction   `json:"action" gorm:"index;size:24"`
	IssueCodesJSON string                   `json:"-" gorm:"type:text"`
	EvidenceJSON   string                   `json:"-" gorm:"type:text"`
	Note           string                   `json:"note" gorm:"type:text"`
	Source         string                   `json:"source" gorm:"index;size:24"`
	AssessmentKind string                   `json:"assessmentKind" gorm:"index;size:32"`
	MediaState     FilmContinuityMediaState `json:"mediaState" gorm:"index;size:24"`
	ReworkEventID  string                   `json:"reworkEventId,omitempty" gorm:"index;size:36"`
	ModelAttemptID string                   `json:"modelAttemptId,omitempty" gorm:"index;size:36"`
	ReviewerUserID string                   `json:"reviewerUserId,omitempty" gorm:"index;size:36"`
	ArtifactID     string                   `json:"artifactId" gorm:"index;size:36"`
	RevisionID     string                   `json:"revisionId" gorm:"index;size:36"`
	CreatedAt      time.Time                `json:"createdAt" gorm:"index"`
}

// FilmVideoVisualQCQuote freezes one generated video revision, its browser-
// sampled frame evidence, the selected multimodal route, and the price.
type FilmVideoVisualQCQuote struct {
	ID                       string                    `json:"id" gorm:"primaryKey;size:36"`
	UserID                   string                    `json:"userId" gorm:"index;size:36;uniqueIndex:idx_film_video_visual_qc_quote_user_idempotency,priority:1"`
	IdempotencyKey           string                    `json:"-" gorm:"size:128;uniqueIndex:idx_film_video_visual_qc_quote_user_idempotency,priority:2"`
	ProjectID                string                    `json:"projectId" gorm:"index;size:36"`
	RootRunID                string                    `json:"rootRunId" gorm:"index;size:36"`
	SequenceID               string                    `json:"sequenceId" gorm:"index;size:36"`
	SequenceRevision         int64                     `json:"sequenceRevision"`
	SlotID                   string                    `json:"slotId" gorm:"index;size:36"`
	SlotRevision             int64                     `json:"slotRevision"`
	ShotID                   string                    `json:"shotId" gorm:"index;size:36"`
	SourceAttemptID          string                    `json:"sourceAttemptId" gorm:"index;size:36"`
	SourceResultID           string                    `json:"sourceResultId" gorm:"index;size:36"`
	SourceResourceID         string                    `json:"sourceResourceId" gorm:"index;size:80"`
	SourceResultArtifactID   string                    `json:"sourceResultArtifactId" gorm:"index;size:36"`
	SourceResultRevisionID   string                    `json:"sourceResultRevisionId" gorm:"index;size:36"`
	SourceResultDigest       string                    `json:"sourceResultDigest" gorm:"size:64"`
	SourceImageArtifactID    string                    `json:"sourceImageArtifactId" gorm:"index;size:36"`
	SourceImageRevisionID    string                    `json:"sourceImageRevisionId" gorm:"index;size:36"`
	PromptArtifactID         string                    `json:"promptArtifactId" gorm:"index;size:36"`
	PromptRevisionID         string                    `json:"promptRevisionId" gorm:"index;size:36"`
	PromptDigest             string                    `json:"promptDigest" gorm:"size:64"`
	RegistryID               string                    `json:"registryId" gorm:"size:80"`
	RegistryVersion          string                    `json:"registryVersion" gorm:"size:32"`
	RegistryDigest           string                    `json:"registryDigest" gorm:"size:64"`
	LogicalModelID           string                    `json:"logicalModelId" gorm:"index;size:36"`
	LogicalModelRevisionID   string                    `json:"logicalModelRevisionId" gorm:"index;size:36"`
	RouteID                  string                    `json:"-" gorm:"index;size:36"`
	ChannelID                string                    `json:"-" gorm:"index;size:36"`
	ChannelModelID           string                    `json:"-" gorm:"index;size:36"`
	Model                    string                    `json:"model" gorm:"size:120"`
	ProviderModel            string                    `json:"-" gorm:"size:120"`
	Protocol                 ChannelInterfaceType      `json:"-" gorm:"size:40"`
	CapabilityVersion        int64                     `json:"capabilityVersion"`
	ChannelPriceVersion      int64                     `json:"channelPriceVersion"`
	SampleFramesJSON         string                    `json:"-" gorm:"type:text"`
	ReferenceResourceIDsJSON string                    `json:"-" gorm:"type:text"`
	TaskID                   string                    `json:"-" gorm:"uniqueIndex;size:36"`
	RequestJSON              string                    `json:"-" gorm:"type:text"`
	RequestFingerprint       string                    `json:"requestFingerprint" gorm:"size:64"`
	BillingJSON              string                    `json:"-" gorm:"type:text"`
	QuoteFingerprint         string                    `json:"quoteFingerprint" gorm:"size:64"`
	Status                   FilmProductionQuoteStatus `json:"status" gorm:"index;size:24"`
	ExpiresAt                time.Time                 `json:"expiresAt" gorm:"index"`
	ConsumedAt               *time.Time                `json:"consumedAt,omitempty"`
	CreatedAt                time.Time                 `json:"createdAt" gorm:"index"`
	UpdatedAt                time.Time                 `json:"updatedAt"`
}

// FilmVideoVisualQCAttempt is append-only. Invalid model output remains an
// uncertain paid attempt and can only be rerun through a new quote.
type FilmVideoVisualQCAttempt struct {
	ID                     string                      `json:"id" gorm:"primaryKey;size:36"`
	UserID                 string                      `json:"userId" gorm:"index;size:36;uniqueIndex:idx_film_video_visual_qc_attempt_user_idempotency,priority:1"`
	IdempotencyKey         string                      `json:"-" gorm:"size:128;uniqueIndex:idx_film_video_visual_qc_attempt_user_idempotency,priority:2"`
	ProjectID              string                      `json:"projectId" gorm:"index;size:36"`
	RootRunID              string                      `json:"rootRunId" gorm:"index;size:36"`
	SequenceID             string                      `json:"sequenceId" gorm:"index;size:36"`
	SlotID                 string                      `json:"slotId" gorm:"index;size:36"`
	ShotID                 string                      `json:"shotId" gorm:"index;size:36"`
	SourceAttemptID        string                      `json:"sourceAttemptId" gorm:"index;size:36;uniqueIndex:idx_film_video_visual_qc_attempt_number,priority:1"`
	SourceResultID         string                      `json:"sourceResultId" gorm:"index;size:36"`
	SourceResourceID       string                      `json:"sourceResourceId" gorm:"index;size:80"`
	SourceResultArtifactID string                      `json:"sourceResultArtifactId" gorm:"index;size:36"`
	SourceResultRevisionID string                      `json:"sourceResultRevisionId" gorm:"index;size:36"`
	SourceResultDigest     string                      `json:"sourceResultDigest" gorm:"size:64"`
	SourceImageArtifactID  string                      `json:"sourceImageArtifactId" gorm:"index;size:36"`
	SourceImageRevisionID  string                      `json:"sourceImageRevisionId" gorm:"index;size:36"`
	PromptArtifactID       string                      `json:"promptArtifactId" gorm:"index;size:36"`
	PromptRevisionID       string                      `json:"promptRevisionId" gorm:"index;size:36"`
	PromptDigest           string                      `json:"promptDigest" gorm:"size:64"`
	SampleFramesJSON       string                      `json:"-" gorm:"type:text"`
	Number                 int                         `json:"number" gorm:"uniqueIndex:idx_film_video_visual_qc_attempt_number,priority:2"`
	QuoteID                string                      `json:"quoteId" gorm:"uniqueIndex;size:36"`
	TaskID                 string                      `json:"taskId" gorm:"uniqueIndex;size:36"`
	BillingOrderID         string                      `json:"billingOrderId,omitempty" gorm:"index;size:36"`
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
	Protocol               ChannelInterfaceType        `json:"-" gorm:"size:40"`
	CapabilityVersion      int64                       `json:"capabilityVersion"`
	ChannelPriceVersion    int64                       `json:"channelPriceVersion"`
	RequestFingerprint     string                      `json:"requestFingerprint" gorm:"size:64"`
	ReportID               string                      `json:"reportId,omitempty" gorm:"index;size:36"`
	ReportArtifactID       string                      `json:"reportArtifactId,omitempty" gorm:"index;size:36"`
	ReportRevisionID       string                      `json:"reportRevisionId,omitempty" gorm:"index;size:36"`
	Status                 FilmProductionAttemptStatus `json:"status" gorm:"index;size:24"`
	Error                  string                      `json:"error,omitempty" gorm:"type:text"`
	StartedAt              *time.Time                  `json:"startedAt,omitempty"`
	CompletedAt            *time.Time                  `json:"completedAt,omitempty"`
	CreatedAt              time.Time                   `json:"createdAt" gorm:"index"`
	UpdatedAt              time.Time                   `json:"updatedAt"`
}
