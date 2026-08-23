package model

import "time"

type FilmVideoSequenceVisualQCSampleSnapshot struct {
	TimeMs     int64  `json:"timeMs"`
	ResourceID string `json:"resourceId"`
	MimeType   string `json:"mimeType"`
	Size       int64  `json:"size"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	ETag       string `json:"etag"`
}

type FilmVideoSequenceVisualQCSlotSnapshot struct {
	Position                  int                                       `json:"position"`
	SlotID                    string                                    `json:"slotId"`
	SlotRevision              int64                                     `json:"slotRevision"`
	ShotID                    string                                    `json:"shotId"`
	DurationMs                int64                                     `json:"durationMs"`
	SourceAttemptID           string                                    `json:"sourceAttemptId"`
	SourceResultID            string                                    `json:"sourceResultId"`
	SourceResourceID          string                                    `json:"sourceResourceId"`
	SourceResourceMimeType    string                                    `json:"sourceResourceMimeType"`
	SourceResourceSize        int64                                     `json:"sourceResourceSize"`
	SourceResourceDurationMs  int64                                     `json:"sourceResourceDurationMs"`
	SourceResourceETag        string                                    `json:"sourceResourceEtag"`
	SourceResultArtifactID    string                                    `json:"sourceResultArtifactId"`
	SourceResultRevisionID    string                                    `json:"sourceResultRevisionId"`
	SourceResultDigest        string                                    `json:"sourceResultDigest"`
	SourceImageResourceID     string                                    `json:"sourceImageResourceId"`
	SourceImageArtifactID     string                                    `json:"sourceImageArtifactId"`
	SourceImageRevisionID     string                                    `json:"sourceImageRevisionId"`
	SourceImageRevisionDigest string                                    `json:"sourceImageRevisionDigest"`
	Samples                   []FilmVideoSequenceVisualQCSampleSnapshot `json:"samples"`
}

// FilmVideoSequenceVisualQCQuote freezes a whole accepted video sequence,
// browser-sampled cross-shot evidence, the selected multimodal route and cost.
type FilmVideoSequenceVisualQCQuote struct {
	ID                       string                    `json:"id" gorm:"primaryKey;size:36"`
	UserID                   string                    `json:"userId" gorm:"index;size:36;uniqueIndex:idx_film_video_sequence_visual_qc_quote_user_idempotency,priority:1"`
	IdempotencyKey           string                    `json:"-" gorm:"size:128;uniqueIndex:idx_film_video_sequence_visual_qc_quote_user_idempotency,priority:2"`
	ProjectID                string                    `json:"projectId" gorm:"index;size:36"`
	RootRunID                string                    `json:"rootRunId" gorm:"index;size:36"`
	SequenceID               string                    `json:"sequenceId" gorm:"index;size:36"`
	SequenceRevision         int64                     `json:"sequenceRevision"`
	LedgerID                 string                    `json:"ledgerId" gorm:"index;size:36"`
	LedgerArtifactID         string                    `json:"ledgerArtifactId" gorm:"index;size:36"`
	LedgerRevisionID         string                    `json:"ledgerRevisionId" gorm:"index;size:36"`
	LedgerDigest             string                    `json:"ledgerDigest" gorm:"size:64"`
	ScopeFingerprint         string                    `json:"scopeFingerprint" gorm:"size:64"`
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
	SlotEvidenceJSON         string                    `json:"-" gorm:"type:text"`
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

// FilmVideoSequenceVisualQCAttempt is append-only. A malformed paid model
// response becomes uncertain and can only be rerun through a fresh quote.
type FilmVideoSequenceVisualQCAttempt struct {
	ID                     string                      `json:"id" gorm:"primaryKey;size:36"`
	UserID                 string                      `json:"userId" gorm:"index;size:36;uniqueIndex:idx_film_video_sequence_visual_qc_attempt_user_idempotency,priority:1"`
	IdempotencyKey         string                      `json:"-" gorm:"size:128;uniqueIndex:idx_film_video_sequence_visual_qc_attempt_user_idempotency,priority:2"`
	ProjectID              string                      `json:"projectId" gorm:"index;size:36"`
	RootRunID              string                      `json:"rootRunId" gorm:"index;size:36"`
	SequenceID             string                      `json:"sequenceId" gorm:"index;size:36;uniqueIndex:idx_film_video_sequence_visual_qc_attempt_number,priority:1"`
	SequenceRevision       int64                       `json:"sequenceRevision"`
	LedgerID               string                      `json:"ledgerId" gorm:"index;size:36"`
	LedgerArtifactID       string                      `json:"ledgerArtifactId" gorm:"index;size:36"`
	LedgerRevisionID       string                      `json:"ledgerRevisionId" gorm:"index;size:36"`
	LedgerDigest           string                      `json:"ledgerDigest" gorm:"size:64"`
	ScopeFingerprint       string                      `json:"scopeFingerprint" gorm:"size:64"`
	PromptArtifactID       string                      `json:"promptArtifactId" gorm:"index;size:36"`
	PromptRevisionID       string                      `json:"promptRevisionId" gorm:"index;size:36"`
	PromptDigest           string                      `json:"promptDigest" gorm:"size:64"`
	SlotEvidenceJSON       string                      `json:"-" gorm:"type:text"`
	Number                 int                         `json:"number" gorm:"uniqueIndex:idx_film_video_sequence_visual_qc_attempt_number,priority:2"`
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
