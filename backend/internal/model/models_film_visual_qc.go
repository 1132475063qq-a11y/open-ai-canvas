package model

import "time"

// FilmVisualQCQuote freezes the generated image, its locked production
// evidence, the multimodal route and the price before a visual review runs.
type FilmVisualQCQuote struct {
	ID                       string                    `json:"id" gorm:"primaryKey;size:36"`
	UserID                   string                    `json:"userId" gorm:"index;size:36;uniqueIndex:idx_film_visual_qc_quote_user_idempotency,priority:1"`
	IdempotencyKey           string                    `json:"-" gorm:"size:128;uniqueIndex:idx_film_visual_qc_quote_user_idempotency,priority:2"`
	ProjectID                string                    `json:"projectId" gorm:"index;size:36"`
	RootRunID                string                    `json:"rootRunId" gorm:"index;size:36"`
	ShotID                   string                    `json:"shotId" gorm:"index;size:36"`
	SourceAttemptID          string                    `json:"sourceAttemptId" gorm:"index;size:36"`
	SourceResultID           string                    `json:"sourceResultId" gorm:"index;size:36"`
	SourceResourceID         string                    `json:"sourceResourceId,omitempty" gorm:"index;size:80"`
	SourceResultArtifactID   string                    `json:"sourceResultArtifactId" gorm:"index;size:36"`
	SourceResultRevisionID   string                    `json:"sourceResultRevisionId" gorm:"index;size:36"`
	SourceResultDigest       string                    `json:"sourceResultDigest" gorm:"size:64"`
	StoryboardArtifactID     string                    `json:"storyboardArtifactId" gorm:"index;size:36"`
	StoryboardRevisionID     string                    `json:"storyboardRevisionId" gorm:"index;size:36"`
	StoryboardDigest         string                    `json:"storyboardDigest" gorm:"size:64"`
	PromptArtifactID         string                    `json:"promptArtifactId" gorm:"index;size:36"`
	PromptRevisionID         string                    `json:"promptRevisionId" gorm:"index;size:36"`
	PromptDigest             string                    `json:"promptDigest" gorm:"size:64"`
	FeasibilityArtifactID    string                    `json:"feasibilityArtifactId" gorm:"index;size:36"`
	FeasibilityRevisionID    string                    `json:"feasibilityRevisionId" gorm:"index;size:36"`
	FeasibilityDigest        string                    `json:"feasibilityDigest" gorm:"size:64"`
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

// FilmVisualQCAttempt is append-only. A rerun always consumes a new quote and
// creates a new task so cost and evidence history cannot be overwritten.
type FilmVisualQCAttempt struct {
	ID                     string                      `json:"id" gorm:"primaryKey;size:36"`
	UserID                 string                      `json:"userId" gorm:"index;size:36;uniqueIndex:idx_film_visual_qc_attempt_user_idempotency,priority:1"`
	IdempotencyKey         string                      `json:"-" gorm:"size:128;uniqueIndex:idx_film_visual_qc_attempt_user_idempotency,priority:2"`
	ProjectID              string                      `json:"projectId" gorm:"index;size:36"`
	RootRunID              string                      `json:"rootRunId" gorm:"index;size:36"`
	ShotID                 string                      `json:"shotId" gorm:"index;size:36"`
	SourceAttemptID        string                      `json:"sourceAttemptId" gorm:"index;size:36;uniqueIndex:idx_film_visual_qc_attempt_number,priority:1"`
	SourceResultID         string                      `json:"sourceResultId" gorm:"index;size:36"`
	SourceResourceID       string                      `json:"sourceResourceId,omitempty" gorm:"index;size:80"`
	SourceResultArtifactID string                      `json:"sourceResultArtifactId" gorm:"index;size:36"`
	SourceResultRevisionID string                      `json:"sourceResultRevisionId" gorm:"index;size:36"`
	SourceResultDigest     string                      `json:"sourceResultDigest" gorm:"size:64"`
	Number                 int                         `json:"number" gorm:"uniqueIndex:idx_film_visual_qc_attempt_number,priority:2"`
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
