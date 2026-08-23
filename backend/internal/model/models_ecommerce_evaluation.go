package model

import "time"

// EcommerceProviderEvaluationPlan is the durable, provider-neutral record of
// one controlled model comparison. It stores references and measurements, not
// credentials or executable provider configuration.
type EcommerceProviderEvaluationPlan struct {
	ID                  string     `json:"id" gorm:"primaryKey;size:36"`
	UserID              string     `json:"userId" gorm:"index;size:36;uniqueIndex:idx_ecommerce_eval_plan_idempotency,priority:1"`
	ProjectID           string     `json:"projectId" gorm:"index;size:36;uniqueIndex:idx_ecommerce_eval_plan_idempotency,priority:2"`
	IdempotencyKey      string     `json:"-" gorm:"size:160;uniqueIndex:idx_ecommerce_eval_plan_idempotency,priority:3"`
	SkillRef            string     `json:"skillRef" gorm:"index;size:160"`
	Mode                string     `json:"mode" gorm:"index;size:32"`
	Status              string     `json:"status" gorm:"index;size:32"`
	RequestFingerprint  string     `json:"requestFingerprint" gorm:"size:128"`
	SettingsJSON        string     `json:"-" gorm:"type:text"`
	CasesJSON           string     `json:"-" gorm:"type:text"`
	CandidatesJSON      string     `json:"-" gorm:"type:text"`
	Decision            string     `json:"decision,omitempty" gorm:"index;size:16"`
	DecisionReviewerRef string     `json:"decisionReviewerRef,omitempty" gorm:"size:160"`
	DecisionRationale   string     `json:"decisionRationale,omitempty" gorm:"type:text"`
	DecisionNextAction  string     `json:"decisionNextAction,omitempty" gorm:"type:text"`
	DecisionEvidence    string     `json:"decisionEvidence,omitempty" gorm:"size:24"`
	DecisionRecordedAt  *time.Time `json:"decisionRecordedAt,omitempty"`
	CreatedAt           time.Time  `json:"createdAt" gorm:"index"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

// EcommerceProviderEvaluationAttempt is append-only evidence from one
// candidate/case execution. The provider adapter writes only normalized facts.
type EcommerceProviderEvaluationAttempt struct {
	ID                  string     `json:"id" gorm:"primaryKey;size:36"`
	UserID              string     `json:"userId" gorm:"index;size:36;uniqueIndex:idx_ecommerce_eval_attempt_idempotency,priority:1"`
	ProjectID           string     `json:"projectId" gorm:"index;size:36"`
	PlanID              string     `json:"planId" gorm:"index;size:36;uniqueIndex:idx_ecommerce_eval_attempt_idempotency,priority:2"`
	IdempotencyKey      string     `json:"-" gorm:"size:160;uniqueIndex:idx_ecommerce_eval_attempt_idempotency,priority:3"`
	CaseID              string     `json:"caseId" gorm:"index;size:120"`
	CandidateID         string     `json:"candidateId" gorm:"index;size:120"`
	Variant             string     `json:"variant" gorm:"index;size:24"`
	Status              string     `json:"status" gorm:"index;size:24"`
	SettingsFingerprint string     `json:"settingsFingerprint" gorm:"size:128"`
	RequestFingerprint  string     `json:"requestFingerprint" gorm:"size:128"`
	ProviderJobID       string     `json:"providerJobId,omitempty" gorm:"size:180"`
	ResultRefsJSON      string     `json:"-" gorm:"type:text"`
	StartedAt           time.Time  `json:"startedAt" gorm:"index"`
	CompletedAt         *time.Time `json:"completedAt,omitempty"`
	LatencyMS           int64      `json:"latencyMs,omitempty"`
	UsageJSON           string     `json:"-" gorm:"type:text"`
	CostJSON            string     `json:"-" gorm:"type:text"`
	FailureJSON         string     `json:"-" gorm:"type:text"`
	Evidence            string     `json:"evidence" gorm:"index;size:24"`
	CreatedAt           time.Time  `json:"createdAt" gorm:"index"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

// EcommerceProviderEvaluationScore is an immutable human or model assessment
// tied to a succeeded attempt.
type EcommerceProviderEvaluationScore struct {
	ID             string    `json:"id" gorm:"primaryKey;size:36"`
	UserID         string    `json:"userId" gorm:"index;size:36;uniqueIndex:idx_ecommerce_eval_score_idempotency,priority:1"`
	ProjectID      string    `json:"projectId" gorm:"index;size:36"`
	PlanID         string    `json:"planId" gorm:"index;size:36;uniqueIndex:idx_ecommerce_eval_score_idempotency,priority:2"`
	AttemptID      string    `json:"attemptId" gorm:"index;size:36"`
	IdempotencyKey string    `json:"-" gorm:"size:160;uniqueIndex:idx_ecommerce_eval_score_idempotency,priority:3"`
	EvaluatorRef   string    `json:"evaluatorRef" gorm:"size:160"`
	DimensionsJSON string    `json:"-" gorm:"type:text"`
	Notes          string    `json:"notes,omitempty" gorm:"type:text"`
	Evidence       string    `json:"evidence" gorm:"index;size:24"`
	RecordedAt     time.Time `json:"recordedAt" gorm:"index"`
	CreatedAt      time.Time `json:"createdAt"`
}

// EcommerceProviderEvaluationPreference records a blinded pairwise judgment.
type EcommerceProviderEvaluationPreference struct {
	ID                 string    `json:"id" gorm:"primaryKey;size:36"`
	UserID             string    `json:"userId" gorm:"index;size:36;uniqueIndex:idx_ecommerce_eval_preference_idempotency,priority:1"`
	ProjectID          string    `json:"projectId" gorm:"index;size:36"`
	PlanID             string    `json:"planId" gorm:"index;size:36;uniqueIndex:idx_ecommerce_eval_preference_idempotency,priority:2"`
	IdempotencyKey     string    `json:"-" gorm:"size:160;uniqueIndex:idx_ecommerce_eval_preference_idempotency,priority:3"`
	CaseID             string    `json:"caseId" gorm:"index;size:120"`
	ResultRefsJSON     string    `json:"-" gorm:"type:text"`
	PreferredResultRef string    `json:"preferredResultRef,omitempty" gorm:"size:180"`
	Tie                bool      `json:"tie"`
	VoterRef           string    `json:"voterRef" gorm:"size:160"`
	Blinded            bool      `json:"blinded"`
	Evidence           string    `json:"evidence" gorm:"index;size:24"`
	RecordedAt         time.Time `json:"recordedAt" gorm:"index"`
	CreatedAt          time.Time `json:"createdAt"`
}
