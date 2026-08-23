package model

import "time"

type FilmContinuityMediaState string
type FilmContinuityLedgerStatus string
type FilmContinuityShotStatus string
type FilmReworkStatus string

const (
	FilmContinuityMediaStateStructuredOnly FilmContinuityMediaState = "structured_only"
	FilmContinuityMediaStateAvailable      FilmContinuityMediaState = "available"

	FilmContinuityLedgerStatusReady    FilmContinuityLedgerStatus = "ready"
	FilmContinuityLedgerStatusNeedsYou FilmContinuityLedgerStatus = "needs_you"

	FilmContinuityShotStatusReady       FilmContinuityShotStatus = "ready"
	FilmContinuityShotStatusNeedsReview FilmContinuityShotStatus = "needs_review"

	FilmReworkStatusOpen       FilmReworkStatus = "open"
	FilmReworkStatusResolved   FilmReworkStatus = "resolved"
	FilmReworkStatusSuperseded FilmReworkStatus = "superseded"
)

// FilmContinuityLedger is the current structured preflight projection over one
// video sequence. Its immutable Artifact revisions preserve prior projections;
// it does not claim visual continuity until real media is reviewed.
type FilmContinuityLedger struct {
	ID                 string                     `json:"id" gorm:"primaryKey;size:36"`
	UserID             string                     `json:"userId" gorm:"index;size:36"`
	ProjectID          string                     `json:"projectId" gorm:"index;size:36"`
	RootRunID          string                     `json:"rootRunId" gorm:"index;size:36"`
	SequenceID         string                     `json:"sequenceId" gorm:"uniqueIndex;size:36"`
	MediaState         FilmContinuityMediaState   `json:"mediaState" gorm:"index;size:24"`
	Status             FilmContinuityLedgerStatus `json:"status" gorm:"index;size:24"`
	IssueCount         int                        `json:"issueCount"`
	SourceArtifactRefs string                     `json:"-" gorm:"type:text"`
	ArtifactID         string                     `json:"artifactId" gorm:"uniqueIndex;size:36"`
	ArtifactRevisionID string                     `json:"artifactRevisionId" gorm:"uniqueIndex;size:36"`
	CreatedAt          time.Time                  `json:"createdAt" gorm:"index"`
}

type FilmContinuityShotState struct {
	ID                 string                   `json:"id" gorm:"primaryKey;size:36"`
	UserID             string                   `json:"userId" gorm:"index;size:36"`
	ProjectID          string                   `json:"projectId" gorm:"index;size:36"`
	RootRunID          string                   `json:"rootRunId" gorm:"index;size:36"`
	SequenceID         string                   `json:"sequenceId" gorm:"index;size:36"`
	LedgerID           string                   `json:"ledgerId" gorm:"index;size:36;uniqueIndex:idx_film_continuity_shot_position,priority:1"`
	ShotID             string                   `json:"shotId" gorm:"index;size:36"`
	Position           int                      `json:"position" gorm:"uniqueIndex:idx_film_continuity_shot_position,priority:2"`
	ReadInJSON         string                   `json:"-" gorm:"type:text"`
	WriteOutJSON       string                   `json:"-" gorm:"type:text"`
	DimensionStateJSON string                   `json:"-" gorm:"type:text"`
	ReferenceLockJSON  string                   `json:"-" gorm:"type:text"`
	Status             FilmContinuityShotStatus `json:"status" gorm:"index;size:24"`
	CreatedAt          time.Time                `json:"createdAt"`
}

type FilmContinuityIssue struct {
	ID             string    `json:"id" gorm:"primaryKey;size:36"`
	UserID         string    `json:"userId" gorm:"index;size:36"`
	ProjectID      string    `json:"projectId" gorm:"index;size:36"`
	RootRunID      string    `json:"rootRunId" gorm:"index;size:36"`
	SequenceID     string    `json:"sequenceId" gorm:"index;size:36"`
	LedgerID       string    `json:"ledgerId" gorm:"index;size:36"`
	ShotID         string    `json:"shotId,omitempty" gorm:"index;size:36"`
	Dimension      string    `json:"dimension" gorm:"index;size:40"`
	Severity       string    `json:"severity" gorm:"index;size:24"`
	Code           string    `json:"code" gorm:"index;size:80"`
	Message        string    `json:"message" gorm:"type:text"`
	Authority      string    `json:"authority" gorm:"size:160"`
	Owner          string    `json:"owner" gorm:"size:80"`
	RepairStatus   string    `json:"repairStatus" gorm:"index;size:24"`
	SourceRefsJSON string    `json:"-" gorm:"type:text"`
	CreatedAt      time.Time `json:"createdAt"`
}

// FilmReworkEvent points to the smallest retry/revision scope justified by a
// concrete QC report. It is append-only; later resolution creates a new fact.
type FilmReworkEvent struct {
	ID                 string           `json:"id" gorm:"primaryKey;size:36"`
	UserID             string           `json:"userId" gorm:"index;size:36"`
	ProjectID          string           `json:"projectId" gorm:"index;size:36"`
	RootRunID          string           `json:"rootRunId" gorm:"index;size:36"`
	SequenceID         string           `json:"sequenceId,omitempty" gorm:"index;size:36"`
	SlotID             string           `json:"slotId,omitempty" gorm:"index;size:36"`
	ShotID             string           `json:"shotId,omitempty" gorm:"index;size:36"`
	AttemptID          string           `json:"attemptId" gorm:"index;size:36"`
	ResultID           string           `json:"resultId" gorm:"index;size:36"`
	QCReportID         string           `json:"qcReportId" gorm:"uniqueIndex;size:36"`
	MediaType          string           `json:"mediaType" gorm:"index;size:24"`
	Source             string           `json:"source" gorm:"index;size:24"`
	Severity           string           `json:"severity" gorm:"index;size:24"`
	ReasonCode         string           `json:"reasonCode" gorm:"index;size:80"`
	Owner              string           `json:"owner" gorm:"size:80"`
	RepairScopeJSON    string           `json:"-" gorm:"type:text"`
	RecheckGateJSON    string           `json:"-" gorm:"type:text"`
	Status             FilmReworkStatus `json:"status" gorm:"index;size:24"`
	ArtifactID         string           `json:"artifactId" gorm:"uniqueIndex;size:36"`
	ArtifactRevisionID string           `json:"artifactRevisionId" gorm:"uniqueIndex;size:36"`
	CreatedAt          time.Time        `json:"createdAt" gorm:"index"`
}
