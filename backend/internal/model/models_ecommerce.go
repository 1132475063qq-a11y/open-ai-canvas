package model

import "time"

// EcommerceArtifact 是电商创意域的独立事实源。
// 它不能复用 FilmArtifact，因为两条 Domain 的版本、Skill 和 QA 语义不同。
type EcommerceArtifact struct {
	ID                 string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID          string    `json:"projectId" gorm:"index;size:36;uniqueIndex:idx_ecommerce_artifact_version,priority:1"`
	ArtifactKey        string    `json:"artifactKey" gorm:"size:180;uniqueIndex:idx_ecommerce_artifact_version,priority:2"`
	ArtifactType       string    `json:"artifactType" gorm:"index;size:64;uniqueIndex:idx_ecommerce_artifact_version,priority:3"`
	SchemaVersion      int       `json:"schemaVersion" gorm:"index"`
	Revision           int       `json:"revision" gorm:"uniqueIndex:idx_ecommerce_artifact_version,priority:4"`
	Lifecycle          string    `json:"lifecycle" gorm:"index;size:24"`
	Evidence           string    `json:"evidence" gorm:"index;size:24"`
	ResponsibleAgentID string    `json:"responsibleAgentId,omitempty" gorm:"index;size:80"`
	SkillRef           string    `json:"skillRef,omitempty" gorm:"size:160"`
	PayloadJSON        string    `json:"payloadJson" gorm:"type:text"`
	SourceRefsJSON     string    `json:"sourceRefsJson" gorm:"type:text"`
	AuthorityRefsJSON  string    `json:"authorityRefsJson" gorm:"type:text"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// EcommercePresetVersion stores user-owned copies of a system preset. System
// presets live in the versioned server catalog; every user edit appends a new
// row so an existing production run can always resolve its exact skill input.
type EcommercePresetVersion struct {
	ID             string    `json:"id" gorm:"primaryKey;size:36"`
	UserID         string    `json:"userId" gorm:"index;size:36"`
	ProjectID      string    `json:"projectId" gorm:"index;size:36;uniqueIndex:idx_ecommerce_preset_version,priority:1"`
	PresetKey      string    `json:"presetKey" gorm:"size:120;uniqueIndex:idx_ecommerce_preset_version,priority:2"`
	SourcePresetID string    `json:"sourcePresetId" gorm:"size:120"`
	Version        int       `json:"version" gorm:"uniqueIndex:idx_ecommerce_preset_version,priority:3"`
	Name           string    `json:"name" gorm:"size:160"`
	Kernel         string    `json:"kernel" gorm:"index;size:32"`
	Category       string    `json:"category" gorm:"index;size:48"`
	Description    string    `json:"description" gorm:"size:500"`
	DefinitionJSON string    `json:"definitionJson" gorm:"type:text"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// EcommerceProductionRun is the server-owned aggregate for one coherent set.
// The canvas stores only its ID; task, quote, approval and result facts remain
// recoverable after the browser is closed.
type EcommerceProductionRun struct {
	ID                          string     `json:"id" gorm:"primaryKey;size:36"`
	UserID                      string     `json:"userId" gorm:"index;size:36;uniqueIndex:idx_ecommerce_run_idempotency,priority:1"`
	ProjectID                   string     `json:"projectId" gorm:"index;size:36;uniqueIndex:idx_ecommerce_run_idempotency,priority:2"`
	IdempotencyKey              string     `json:"idempotencyKey" gorm:"size:160;uniqueIndex:idx_ecommerce_run_idempotency,priority:3"`
	Status                      string     `json:"status" gorm:"index;size:32"`
	RegistryID                  string     `json:"registryId" gorm:"index;size:80"`
	RegistryVersion             string     `json:"registryVersion" gorm:"size:32"`
	RegistryDigest              string     `json:"registryDigest" gorm:"size:64"`
	Kernel                      string     `json:"kernel" gorm:"index;size:32"`
	Category                    string     `json:"category" gorm:"index;size:48"`
	PresetID                    string     `json:"presetId" gorm:"index;size:120"`
	PresetVersion               int        `json:"presetVersion"`
	TargetChannel               string     `json:"targetChannel" gorm:"size:32"`
	AspectRatio                 string     `json:"aspectRatio" gorm:"size:16"`
	Resolution                  string     `json:"resolution" gorm:"size:16"`
	PixelSize                   string     `json:"pixelSize" gorm:"size:24"`
	OutputCount                 int        `json:"outputCount"`
	ReviewBeforeGeneration      bool       `json:"reviewBeforeGeneration"`
	ProductAssetIDsJSON         string     `json:"productAssetIdsJson" gorm:"type:text"`
	SupportingAssetIDsJSON      string     `json:"supportingAssetIdsJson" gorm:"type:text"`
	ModelAssetIDsJSON           string     `json:"modelAssetIdsJson" gorm:"type:text"`
	SceneAssetIDsJSON           string     `json:"sceneAssetIdsJson" gorm:"type:text"`
	BrandAssetIDsJSON           string     `json:"brandAssetIdsJson" gorm:"type:text"`
	UserGoal                    string     `json:"userGoal" gorm:"type:text"`
	ModelMode                   string     `json:"modelMode" gorm:"size:24"`
	ModelBrief                  string     `json:"modelBrief" gorm:"type:text"`
	SceneBrief                  string     `json:"sceneBrief" gorm:"type:text"`
	BrandBrief                  string     `json:"brandBrief" gorm:"type:text"`
	AdvancedJSON                string     `json:"advancedJson" gorm:"type:text"`
	ProductDNAArtifactID        string     `json:"productDnaArtifactId" gorm:"index;size:36"`
	ModelProfileArtifactID      string     `json:"modelProfileArtifactId,omitempty" gorm:"index;size:36"`
	ScenePackArtifactID         string     `json:"scenePackArtifactId" gorm:"index;size:36"`
	PresetSnapshotArtifactID    string     `json:"presetSnapshotArtifactId" gorm:"index;size:36"`
	GenerationRequestArtifactID string     `json:"generationRequestArtifactId" gorm:"index;size:36"`
	MotionPlanArtifactID        string     `json:"motionPlanArtifactId,omitempty" gorm:"index;size:36"`
	VideoSequenceArtifactID     string     `json:"videoSequenceArtifactId,omitempty" gorm:"index;size:36"`
	QuoteFingerprint            string     `json:"quoteFingerprint,omitempty" gorm:"index;size:64"`
	QuoteExpiresAt              *time.Time `json:"quoteExpiresAt,omitempty" gorm:"index"`
	QuoteChannelID              string     `json:"quoteChannelId,omitempty" gorm:"size:36"`
	QuoteChannelModelID         string     `json:"quoteChannelModelId,omitempty" gorm:"size:36"`
	QuoteModel                  string     `json:"quoteModel,omitempty" gorm:"size:120"`
	QuotePriceVersion           int64      `json:"quotePriceVersion,omitempty"`
	QuoteUnitMicrocredits       int64      `json:"quoteUnitMicrocredits,omitempty"`
	QuoteTotalMicrocredits      int64      `json:"quoteTotalMicrocredits,omitempty"`
	Error                       string     `json:"error,omitempty" gorm:"type:text"`
	SubmittedAt                 *time.Time `json:"submittedAt,omitempty"`
	CompletedAt                 *time.Time `json:"completedAt,omitempty"`
	CreatedAt                   time.Time  `json:"createdAt" gorm:"index"`
	UpdatedAt                   time.Time  `json:"updatedAt"`
}

// EcommerceProductionSlot is one independently retryable position in a set.
// Its current pointers are mutable projections; every execution is preserved
// by EcommerceProductionAttempt and immutable generated_asset artifacts.
type EcommerceProductionSlot struct {
	ID                  string     `json:"id" gorm:"primaryKey;size:36"`
	UserID              string     `json:"userId" gorm:"index;size:36"`
	ProjectID           string     `json:"projectId" gorm:"index;size:36"`
	RunID               string     `json:"runId" gorm:"index;size:36;uniqueIndex:idx_ecommerce_slot_position,priority:1"`
	Position            int        `json:"position" gorm:"uniqueIndex:idx_ecommerce_slot_position,priority:2"`
	Role                string     `json:"role" gorm:"index;size:48"`
	Title               string     `json:"title" gorm:"size:160"`
	CameraJSON          string     `json:"cameraJson" gorm:"type:text"`
	Prompt              string     `json:"prompt" gorm:"type:text"`
	NegativePrompt      string     `json:"negativePrompt" gorm:"type:text"`
	Status              string     `json:"status" gorm:"index;size:32"`
	QAStatus            string     `json:"qaStatus" gorm:"index;size:24"`
	QAIssuesJSON        string     `json:"qaIssuesJson" gorm:"type:text"`
	QANote              string     `json:"qaNote" gorm:"type:text"`
	Accepted            bool       `json:"accepted" gorm:"index"`
	AcceptedAttemptID   string     `json:"acceptedAttemptId,omitempty" gorm:"index;size:36"`
	AcceptedAt          *time.Time `json:"acceptedAt,omitempty"`
	ActiveAttemptID     string     `json:"activeAttemptId,omitempty" gorm:"index;size:36"`
	ActiveTaskID        string     `json:"activeTaskId,omitempty" gorm:"index;size:36"`
	ResultURL           string     `json:"resultUrl,omitempty" gorm:"type:text"`
	ResultPayloadJSON   string     `json:"resultPayloadJson,omitempty" gorm:"type:text"`
	CompositionHash     string     `json:"compositionHash,omitempty" gorm:"size:64"`
	CompositionHashAlg  string     `json:"compositionHashAlgorithm,omitempty" gorm:"size:32"`
	DuplicateOfSlotID   string     `json:"duplicateOfSlotId,omitempty" gorm:"index;size:36"`
	CompositionDistance int        `json:"compositionDistance,omitempty"`
	RegenerationReason  string     `json:"regenerationReason,omitempty" gorm:"size:500"`
	GeneratedAssetID    string     `json:"generatedAssetId,omitempty" gorm:"index;size:36"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

// EcommerceProductionAttempt is append-only execution history for a slot. A
// paid retry first creates an awaiting_cost row and receives a task only after
// its own quote fingerprint is confirmed.
type EcommerceProductionAttempt struct {
	ID                       string     `json:"id" gorm:"primaryKey;size:36"`
	UserID                   string     `json:"userId" gorm:"index;size:36"`
	ProjectID                string     `json:"projectId" gorm:"index;size:36"`
	RunID                    string     `json:"runId" gorm:"index;size:36"`
	SlotID                   string     `json:"slotId" gorm:"index;size:36;uniqueIndex:idx_ecommerce_attempt_number,priority:1"`
	AttemptNumber            int        `json:"attemptNumber" gorm:"uniqueIndex:idx_ecommerce_attempt_number,priority:2"`
	Kind                     string     `json:"kind" gorm:"size:24"`
	Status                   string     `json:"status" gorm:"index;size:32"`
	Prompt                   string     `json:"prompt" gorm:"type:text"`
	NegativePrompt           string     `json:"negativePrompt" gorm:"type:text"`
	QuoteFingerprint         string     `json:"quoteFingerprint,omitempty" gorm:"index;size:64"`
	QuoteExpiresAt           *time.Time `json:"quoteExpiresAt,omitempty" gorm:"index"`
	QuoteChannelID           string     `json:"quoteChannelId,omitempty" gorm:"size:36"`
	QuoteChannelModelID      string     `json:"quoteChannelModelId,omitempty" gorm:"size:36"`
	QuoteModel               string     `json:"quoteModel,omitempty" gorm:"size:120"`
	QuotePriceVersion        int64      `json:"quotePriceVersion,omitempty"`
	QuoteAmountMicrocredits  int64      `json:"quoteAmountMicrocredits,omitempty"`
	TaskID                   string     `json:"taskId,omitempty" gorm:"index;size:36"`
	BillingOrderID           string     `json:"billingOrderId,omitempty" gorm:"index;size:36"`
	ProviderRequestID        string     `json:"providerRequestId,omitempty" gorm:"index;size:160"`
	ResultID                 string     `json:"resultId,omitempty" gorm:"index;size:36"`
	GeneratedAssetArtifactID string     `json:"generatedAssetArtifactId,omitempty" gorm:"index;size:36"`
	ResultURL                string     `json:"resultUrl,omitempty" gorm:"type:text"`
	ResultPayloadJSON        string     `json:"resultPayloadJson,omitempty" gorm:"type:text"`
	Error                    string     `json:"error,omitempty" gorm:"type:text"`
	StartedAt                *time.Time `json:"startedAt,omitempty"`
	CompletedAt              *time.Time `json:"completedAt,omitempty"`
	CreatedAt                time.Time  `json:"createdAt" gorm:"index"`
	UpdatedAt                time.Time  `json:"updatedAt"`
}
