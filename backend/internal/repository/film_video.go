package repository

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FilmVideoSequenceCreateCommand struct {
	Sequence           *model.FilmVideoSequence
	Slots              []model.FilmVideoSlot
	Artifact           *model.ProductionArtifact
	Revision           *model.ProductionArtifactRevision
	Continuity         *model.FilmContinuityLedger
	ContinuityShots    []model.FilmContinuityShotState
	ContinuityIssues   []model.FilmContinuityIssue
	ContinuityArtifact *model.ProductionArtifact
	ContinuityRevision *model.ProductionArtifactRevision
	Event              AgentRuntimeEventInput
	At                 time.Time
}

type FilmVideoSequenceReviewCreateCommand struct {
	UserID    string
	ProjectID string
	Review    *model.FilmVideoSequenceReview
	Artifact  *model.ProductionArtifact
	Revision  *model.ProductionArtifactRevision
	Event     AgentRuntimeEventInput
	At        time.Time
}

type FilmVideoSequenceDetail struct {
	Sequence         model.FilmVideoSequence                  `json:"sequence"`
	Slots            []FilmVideoSlotDetail                    `json:"slots"`
	Continuity       *FilmContinuityLedgerDetail              `json:"continuity,omitempty"`
	SequenceReview   *model.FilmVideoSequenceReview           `json:"sequenceReview,omitempty"`
	SequenceVisualQC []FilmVideoSequenceVisualQCAttemptDetail `json:"sequenceVisualQcAttempts"`
	ReworkEvents     []model.FilmReworkEvent                  `json:"reworkEvents"`
}

type FilmContinuityLedgerDetail struct {
	Ledger model.FilmContinuityLedger      `json:"ledger"`
	Shots  []model.FilmContinuityShotState `json:"shots"`
	Issues []model.FilmContinuityIssue     `json:"issues"`
}

type FilmVideoSlotDetail struct {
	Slot     model.FilmVideoSlot      `json:"slot"`
	Attempts []FilmVideoAttemptDetail `json:"attempts"`
}

type FilmVideoAttemptDetail struct {
	Attempt          model.FilmVideoAttempt           `json:"attempt"`
	Task             model.Task                       `json:"task"`
	Result           *model.Result                    `json:"result,omitempty"`
	QCReports        []model.FilmVideoQCReport        `json:"qcReports"`
	VisualQCAttempts []FilmVideoVisualQCAttemptDetail `json:"visualQcAttempts"`
	Billing          *model.BillingOrder              `json:"billing,omitempty"`
}

func (r *Repository) CreateFilmVideoSequence(command FilmVideoSequenceCreateCommand) (*FilmVideoSequenceDetail, bool, error) {
	if err := validateFilmVideoSequenceCreate(command); err != nil {
		return nil, false, err
	}
	now := runtimeCommandTime(command.At)
	idempotent := false
	sequenceID := command.Sequence.ID
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var existing model.FilmVideoSequence
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&existing, "user_id = ? AND idempotency_key = ?", command.Sequence.UserID, command.Sequence.IdempotencyKey).Error
		if err == nil {
			if existing.ProjectID != command.Sequence.ProjectID || existing.RequestFingerprint != command.Sequence.RequestFingerprint {
				return ErrFilmProductionStateConflict
			}
			sequenceID = existing.ID
			idempotent = true
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var project model.Project
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&project, "id = ? AND user_id = ?", command.Sequence.ProjectID, command.Sequence.UserID).Error; err != nil {
			return err
		}
		if project.Type != "short-drama" || project.Status != model.ProjectStatusActive {
			return ErrFilmProductionStateConflict
		}
		var root model.AgentRuntimeRun
		if err := lockAgentRuntimeRun(tx, command.Sequence.UserID, command.Sequence.RootRunID, &root); err != nil {
			return err
		}
		if root.ProjectID != command.Sequence.ProjectID || root.Domain != "film" || root.RootRunID != root.ID ||
			root.RegistryDigest != command.Sequence.RegistryDigest || root.Status == model.AgentRunStatusFailed || root.Status == model.AgentRunStatusCancelled {
			return ErrFilmProductionStateConflict
		}
		if err := validateFilmVideoPromptArtifactTx(tx, command.Sequence); err != nil {
			return err
		}
		for index := range command.Slots {
			slot := &command.Slots[index]
			if err := validateAcceptedFilmImageTx(tx, slot.UserID, slot.ProjectID, slot.SourceImageAttemptID, slot.SourceImageResultID,
				slot.SourceImageResourceID, slot.SourceImageArtifactID, slot.SourceImageRevisionID); err != nil {
				return err
			}
			var shot model.Shot
			if err := tx.First(&shot, "id = ? AND project_id = ?", slot.ShotID, slot.ProjectID).Error; err != nil {
				return err
			}
		}
		if _, _, err := createProductionArtifactRevisionTx(tx, ProductionArtifactRevisionCreate{
			UserID: command.Sequence.UserID, Artifact: command.Artifact, Revision: command.Revision, ExpectedSequence: 0, At: now,
		}); err != nil {
			return err
		}
		if _, _, err := createProductionArtifactRevisionTx(tx, ProductionArtifactRevisionCreate{
			UserID: command.Sequence.UserID, Artifact: command.ContinuityArtifact, Revision: command.ContinuityRevision, ExpectedSequence: 0, At: now,
		}); err != nil {
			return err
		}
		sequence := *command.Sequence
		sequence.CreatedAt = now
		sequence.UpdatedAt = now
		if err := tx.Create(&sequence).Error; err != nil {
			return err
		}
		for index := range command.Slots {
			command.Slots[index].CreatedAt = now
			command.Slots[index].UpdatedAt = now
		}
		if err := tx.Create(&command.Slots).Error; err != nil {
			return err
		}
		continuity := *command.Continuity
		continuity.CreatedAt = now
		if err := tx.Create(&continuity).Error; err != nil {
			return err
		}
		for index := range command.ContinuityShots {
			command.ContinuityShots[index].CreatedAt = now
		}
		if err := tx.Create(&command.ContinuityShots).Error; err != nil {
			return err
		}
		for index := range command.ContinuityIssues {
			command.ContinuityIssues[index].CreatedAt = now
		}
		if len(command.ContinuityIssues) > 0 {
			if err := tx.Create(&command.ContinuityIssues).Error; err != nil {
				return err
			}
		}
		return appendFilmProductionEventTx(tx, root, command.Event, "", now)
	})
	if err != nil {
		return nil, false, err
	}
	detail, err := r.FilmVideoSequenceForUser(command.Sequence.UserID, command.Sequence.ProjectID, sequenceID)
	return detail, idempotent, err
}

func (r *Repository) FilmVideoSequenceForUser(userID string, projectID string, sequenceID string) (*FilmVideoSequenceDetail, error) {
	var sequence model.FilmVideoSequence
	if err := r.db.First(&sequence, "id = ? AND user_id = ? AND project_id = ?", sequenceID, userID, projectID).Error; err != nil {
		return nil, err
	}
	details, err := r.hydrateFilmVideoSequences([]model.FilmVideoSequence{sequence})
	if err != nil {
		return nil, err
	}
	return &details[0], nil
}

func (r *Repository) FilmVideoSequenceByIdempotency(userID string, key string) (*model.FilmVideoSequence, error) {
	var sequence model.FilmVideoSequence
	if err := r.db.First(&sequence, "user_id = ? AND idempotency_key = ?", userID, key).Error; err != nil {
		return nil, err
	}
	return &sequence, nil
}

func (r *Repository) FilmVideoSequencesForProject(userID string, projectID string, rootRunID string, limit int) ([]FilmVideoSequenceDetail, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	query := r.db.Where("user_id = ? AND project_id = ?", userID, projectID)
	if strings.TrimSpace(rootRunID) != "" {
		query = query.Where("root_run_id = ?", strings.TrimSpace(rootRunID))
	}
	var sequences []model.FilmVideoSequence
	if err := query.Order("created_at desc").Limit(limit).Find(&sequences).Error; err != nil {
		return nil, err
	}
	return r.hydrateFilmVideoSequences(sequences)
}

func (r *Repository) hydrateFilmVideoSequences(sequences []model.FilmVideoSequence) ([]FilmVideoSequenceDetail, error) {
	details := make([]FilmVideoSequenceDetail, len(sequences))
	if len(sequences) == 0 {
		return details, nil
	}
	sequenceIDs := make([]string, 0, len(sequences))
	for index, sequence := range sequences {
		details[index].Sequence = sequence
		sequenceIDs = append(sequenceIDs, sequence.ID)
	}
	var slots []model.FilmVideoSlot
	if err := r.db.Where("sequence_id IN ?", sequenceIDs).Order("sequence_id asc, position asc").Find(&slots).Error; err != nil {
		return nil, err
	}
	slotIDs := make([]string, 0, len(slots))
	for _, slot := range slots {
		slotIDs = append(slotIDs, slot.ID)
	}
	attemptDetails, err := r.hydrateFilmVideoAttemptsForSlots(slotIDs)
	if err != nil {
		return nil, err
	}
	attemptsBySlot := make(map[string][]FilmVideoAttemptDetail)
	for _, attempt := range attemptDetails {
		attemptsBySlot[attempt.Attempt.SlotID] = append(attemptsBySlot[attempt.Attempt.SlotID], attempt)
	}
	slotsBySequence := make(map[string][]FilmVideoSlotDetail)
	for _, slot := range slots {
		slotsBySequence[slot.SequenceID] = append(slotsBySequence[slot.SequenceID], FilmVideoSlotDetail{
			Slot: slot, Attempts: attemptsBySlot[slot.ID],
		})
	}
	for index := range details {
		details[index].Slots = slotsBySequence[details[index].Sequence.ID]
		if details[index].Slots == nil {
			details[index].Slots = []FilmVideoSlotDetail{}
		}
	}
	continuityBySequence, err := r.hydrateFilmContinuityForSequences(sequenceIDs)
	if err != nil {
		return nil, err
	}
	reviewBySequence, err := r.hydrateFilmVideoSequenceReviews(sequenceIDs)
	if err != nil {
		return nil, err
	}
	sequenceVisualQC, err := r.filmVideoSequenceVisualQCAttemptsForSequences(sequenceIDs)
	if err != nil {
		return nil, err
	}
	reworkBySequence, err := r.filmReworkEventsForSequences(sequenceIDs)
	if err != nil {
		return nil, err
	}
	for index := range details {
		if continuity, ok := continuityBySequence[details[index].Sequence.ID]; ok {
			copy := continuity
			details[index].Continuity = &copy
		}
		if review, ok := reviewBySequence[details[index].Sequence.ID]; ok {
			copy := review
			details[index].SequenceReview = &copy
		}
		details[index].SequenceVisualQC = sequenceVisualQC[details[index].Sequence.ID]
		if details[index].SequenceVisualQC == nil {
			details[index].SequenceVisualQC = []FilmVideoSequenceVisualQCAttemptDetail{}
		}
		details[index].ReworkEvents = reworkBySequence[details[index].Sequence.ID]
		if details[index].ReworkEvents == nil {
			details[index].ReworkEvents = []model.FilmReworkEvent{}
		}
	}
	return details, nil
}

func (r *Repository) hydrateFilmVideoSequenceReviews(sequenceIDs []string) (map[string]model.FilmVideoSequenceReview, error) {
	result := make(map[string]model.FilmVideoSequenceReview)
	if len(sequenceIDs) == 0 {
		return result, nil
	}
	var reviews []model.FilmVideoSequenceReview
	if err := r.db.Where("sequence_id IN ? AND source = ?", sequenceIDs, "human").Order("created_at desc, id desc").Find(&reviews).Error; err != nil {
		return nil, err
	}
	for _, review := range reviews {
		if _, exists := result[review.SequenceID]; !exists {
			result[review.SequenceID] = review
		}
	}
	return result, nil
}

func (r *Repository) hydrateFilmContinuityForSequences(sequenceIDs []string) (map[string]FilmContinuityLedgerDetail, error) {
	result := make(map[string]FilmContinuityLedgerDetail)
	if len(sequenceIDs) == 0 {
		return result, nil
	}
	var ledgers []model.FilmContinuityLedger
	if err := r.db.Where("sequence_id IN ?", sequenceIDs).Find(&ledgers).Error; err != nil {
		return nil, err
	}
	if len(ledgers) == 0 {
		return result, nil
	}
	ledgerIDs := make([]string, 0, len(ledgers))
	for _, ledger := range ledgers {
		ledgerIDs = append(ledgerIDs, ledger.ID)
		result[ledger.SequenceID] = FilmContinuityLedgerDetail{Ledger: ledger, Shots: []model.FilmContinuityShotState{}, Issues: []model.FilmContinuityIssue{}}
	}
	var shots []model.FilmContinuityShotState
	var issues []model.FilmContinuityIssue
	if err := r.db.Where("ledger_id IN ?", ledgerIDs).Order("ledger_id asc, position asc").Find(&shots).Error; err != nil {
		return nil, err
	}
	if err := r.db.Where("ledger_id IN ?", ledgerIDs).Order("created_at asc, id asc").Find(&issues).Error; err != nil {
		return nil, err
	}
	sequenceByLedger := make(map[string]string, len(ledgers))
	for _, ledger := range ledgers {
		sequenceByLedger[ledger.ID] = ledger.SequenceID
	}
	for _, shot := range shots {
		sequenceID := sequenceByLedger[shot.LedgerID]
		detail := result[sequenceID]
		detail.Shots = append(detail.Shots, shot)
		result[sequenceID] = detail
	}
	for _, issue := range issues {
		sequenceID := sequenceByLedger[issue.LedgerID]
		detail := result[sequenceID]
		detail.Issues = append(detail.Issues, issue)
		result[sequenceID] = detail
	}
	return result, nil
}

func (r *Repository) filmReworkEventsForSequences(sequenceIDs []string) (map[string][]model.FilmReworkEvent, error) {
	result := make(map[string][]model.FilmReworkEvent)
	if len(sequenceIDs) == 0 {
		return result, nil
	}
	var events []model.FilmReworkEvent
	if err := r.db.Where("sequence_id IN ?", sequenceIDs).Order("created_at desc, id desc").Find(&events).Error; err != nil {
		return nil, err
	}
	for _, event := range events {
		result[event.SequenceID] = append(result[event.SequenceID], event)
	}
	return result, nil
}

func (r *Repository) hydrateFilmVideoAttemptsForSlots(slotIDs []string) ([]FilmVideoAttemptDetail, error) {
	if len(slotIDs) == 0 {
		return []FilmVideoAttemptDetail{}, nil
	}
	var attempts []model.FilmVideoAttempt
	if err := r.db.Where("slot_id IN ?", slotIDs).Order("slot_id asc, number desc").Find(&attempts).Error; err != nil {
		return nil, err
	}
	details := make([]FilmVideoAttemptDetail, len(attempts))
	if len(attempts) == 0 {
		return details, nil
	}
	attemptIDs := make([]string, 0, len(attempts))
	taskIDs := make([]string, 0, len(attempts))
	orderIDs := make([]string, 0, len(attempts))
	for index, attempt := range attempts {
		details[index].Attempt = attempt
		attemptIDs = append(attemptIDs, attempt.ID)
		taskIDs = append(taskIDs, attempt.TaskID)
		if attempt.BillingOrderID != "" {
			orderIDs = append(orderIDs, attempt.BillingOrderID)
		}
	}
	visualQCAttempts, err := r.filmVideoVisualQCAttemptsForSources(attemptIDs)
	if err != nil {
		return nil, err
	}
	var tasks []model.Task
	var results []model.Result
	var reports []model.FilmVideoQCReport
	var orders []model.BillingOrder
	if err := r.db.Where("id IN ?", taskIDs).Find(&tasks).Error; err != nil {
		return nil, err
	}
	if err := r.db.Where("attempt_id IN ? AND kind = ?", attemptIDs, model.ResultKindFilmVideoGeneration).Find(&results).Error; err != nil {
		return nil, err
	}
	if err := r.db.Where("attempt_id IN ?", attemptIDs).Order("created_at asc").Find(&reports).Error; err != nil {
		return nil, err
	}
	if len(orderIDs) > 0 {
		if err := r.db.Where("id IN ?", orderIDs).Find(&orders).Error; err != nil {
			return nil, err
		}
	}
	taskByID := make(map[string]model.Task, len(tasks))
	resultByAttempt := make(map[string]model.Result, len(results))
	reportsByAttempt := make(map[string][]model.FilmVideoQCReport)
	orderByID := make(map[string]model.BillingOrder, len(orders))
	for _, task := range tasks {
		taskByID[task.ID] = task
	}
	for _, result := range results {
		resultByAttempt[result.AttemptID] = result
	}
	for _, report := range reports {
		reportsByAttempt[report.AttemptID] = append(reportsByAttempt[report.AttemptID], report)
	}
	for _, order := range orders {
		orderByID[order.ID] = order
	}
	for index := range details {
		details[index].Task = taskByID[details[index].Attempt.TaskID]
		if result, ok := resultByAttempt[details[index].Attempt.ID]; ok {
			copy := result
			details[index].Result = &copy
		}
		details[index].QCReports = reportsByAttempt[details[index].Attempt.ID]
		if details[index].QCReports == nil {
			details[index].QCReports = []model.FilmVideoQCReport{}
		}
		details[index].VisualQCAttempts = visualQCAttempts[details[index].Attempt.ID]
		if details[index].VisualQCAttempts == nil {
			details[index].VisualQCAttempts = []FilmVideoVisualQCAttemptDetail{}
		}
		if order, ok := orderByID[details[index].Attempt.BillingOrderID]; ok {
			copy := order
			details[index].Billing = &copy
		}
	}
	return details, nil
}

func validateFilmVideoSequenceCreate(command FilmVideoSequenceCreateCommand) error {
	if command.Sequence == nil || command.Artifact == nil || command.Revision == nil || command.Continuity == nil ||
		command.ContinuityArtifact == nil || command.ContinuityRevision == nil || len(command.Slots) == 0 ||
		len(command.ContinuityShots) != len(command.Slots) ||
		strings.TrimSpace(command.Sequence.ID) == "" || strings.TrimSpace(command.Sequence.UserID) == "" ||
		strings.TrimSpace(command.Sequence.IdempotencyKey) == "" || strings.TrimSpace(command.Sequence.ProjectID) == "" ||
		strings.TrimSpace(command.Sequence.RootRunID) == "" || strings.TrimSpace(command.Sequence.PromptArtifactID) == "" ||
		strings.TrimSpace(command.Sequence.PromptArtifactRevisionID) == "" || strings.TrimSpace(command.Sequence.PromptArtifactDigest) == "" ||
		strings.TrimSpace(command.Sequence.RequestFingerprint) == "" ||
		command.Sequence.Revision != 1 || command.Sequence.Status != model.FilmVideoSequenceStatusReady ||
		command.Sequence.ArtifactID != command.Artifact.ID || command.Sequence.ArtifactRevisionID != command.Revision.ID {
		return errors.New("Film video sequence identity is incomplete")
	}
	if command.Continuity.UserID != command.Sequence.UserID || command.Continuity.ProjectID != command.Sequence.ProjectID ||
		command.Continuity.RootRunID != command.Sequence.RootRunID || command.Continuity.SequenceID != command.Sequence.ID ||
		command.Continuity.MediaState != model.FilmContinuityMediaStateStructuredOnly ||
		command.Continuity.ArtifactID != command.ContinuityArtifact.ID || command.Continuity.ArtifactRevisionID != command.ContinuityRevision.ID ||
		command.Continuity.IssueCount != len(command.ContinuityIssues) {
		return errors.New("Film continuity Ledger identity is incomplete")
	}
	if err := validateAgentRuntimeEventInput(command.Event); err != nil {
		return err
	}
	seenPositions := make(map[int]bool, len(command.Slots))
	for _, slot := range command.Slots {
		if slot.UserID != command.Sequence.UserID || slot.ProjectID != command.Sequence.ProjectID || slot.RootRunID != command.Sequence.RootRunID ||
			slot.SequenceID != command.Sequence.ID || slot.Position < 0 || seenPositions[slot.Position] || strings.TrimSpace(slot.ShotID) == "" ||
			strings.TrimSpace(slot.Prompt) == "" || slot.DurationMs < 1000 || strings.TrimSpace(slot.SourceImageAttemptID) == "" ||
			strings.TrimSpace(slot.SourceImageResultID) == "" || strings.TrimSpace(slot.SourceImageResourceID) == "" ||
			strings.TrimSpace(slot.SourceImageArtifactID) == "" || strings.TrimSpace(slot.SourceImageRevisionID) == "" ||
			slot.Status != model.FilmVideoSlotStatusReady || slot.Revision != 1 {
			return errors.New("Film video slot identity is incomplete")
		}
		seenPositions[slot.Position] = true
	}
	for _, shot := range command.ContinuityShots {
		if shot.UserID != command.Sequence.UserID || shot.ProjectID != command.Sequence.ProjectID ||
			shot.RootRunID != command.Sequence.RootRunID || shot.SequenceID != command.Sequence.ID ||
			shot.LedgerID != command.Continuity.ID || shot.Position < 0 || strings.TrimSpace(shot.ShotID) == "" ||
			strings.TrimSpace(shot.ReadInJSON) == "" || strings.TrimSpace(shot.WriteOutJSON) == "" ||
			strings.TrimSpace(shot.DimensionStateJSON) == "" || strings.TrimSpace(shot.ReferenceLockJSON) == "" {
			return errors.New("Film continuity shot state is incomplete")
		}
	}
	for _, issue := range command.ContinuityIssues {
		if issue.UserID != command.Sequence.UserID || issue.ProjectID != command.Sequence.ProjectID ||
			issue.RootRunID != command.Sequence.RootRunID || issue.SequenceID != command.Sequence.ID ||
			issue.LedgerID != command.Continuity.ID || strings.TrimSpace(issue.Code) == "" || strings.TrimSpace(issue.Owner) == "" {
			return errors.New("Film continuity issue is incomplete")
		}
	}
	return nil
}

func validateFilmVideoPromptArtifactTx(tx *gorm.DB, sequence *model.FilmVideoSequence) error {
	var artifact model.ProductionArtifact
	var revision model.ProductionArtifactRevision
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&artifact,
		"id = ? AND user_id = ? AND project_id = ? AND domain = ?", sequence.PromptArtifactID, sequence.UserID, sequence.ProjectID, "film").Error; err != nil {
		return err
	}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&revision,
		"id = ? AND artifact_id = ?", sequence.PromptArtifactRevisionID, sequence.PromptArtifactID).Error; err != nil {
		return err
	}
	if (artifact.ArtifactType != "ai-video-prompts" && artifact.ArtifactType != "prompt-manifest") ||
		artifact.CurrentRevisionID != revision.ID || revision.Status != model.ProductionArtifactStatusLocked ||
		revision.ContentDigest != sequence.PromptArtifactDigest {
		return ErrFilmProductionQuoteDrift
	}
	return nil
}

func validateAcceptedFilmImageTx(tx *gorm.DB, userID string, projectID string, attemptID string, resultID string, resourceID string, artifactID string, revisionID string) error {
	var attempt model.FilmProductionAttempt
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&attempt,
		"id = ? AND user_id = ? AND project_id = ?", attemptID, userID, projectID).Error; err != nil {
		return err
	}
	if attempt.Status != model.FilmProductionAttemptStatusSucceeded || attempt.ResultID != resultID ||
		attempt.ResultArtifactID != artifactID || attempt.ResultRevisionID != revisionID {
		return ErrFilmProductionStateConflict
	}
	var result model.Result
	if err := tx.First(&result, "id = ? AND attempt_id = ? AND kind = ? AND availability = ?",
		resultID, attemptID, model.ResultKindFilmGeneration, model.ResultAvailabilityReady).Error; err != nil {
		return ErrFilmProductionMediaMissing
	}
	if !filmVideoResultReferencesResource(result.Payload, resourceID) {
		return ErrFilmProductionMediaMissing
	}
	var latestHuman model.FilmProductionQCReport
	if err := tx.Where("attempt_id = ? AND source = ?", attemptID, "human").Order("created_at desc, id desc").First(&latestHuman).Error; err != nil {
		return ErrFilmProductionStateConflict
	}
	if latestHuman.Decision != model.FilmProductionQCDecisionPass || latestHuman.Action != model.FilmProductionQCActionAccept || latestHuman.ResultID != resultID {
		return ErrFilmProductionStateConflict
	}
	var resource model.Resource
	if err := tx.First(&resource, "id = ? AND user_id = ?", resourceID, userID).Error; err != nil {
		return ErrFilmProductionMediaMissing
	}
	if resource.Status != model.ResourceStatusReady || resource.Kind != "image" || !strings.HasPrefix(resource.MimeType, "image/") {
		return ErrFilmProductionMediaMissing
	}
	return nil
}

func filmVideoResultReferencesResource(raw string, resourceID string) bool {
	var value any
	if strings.TrimSpace(resourceID) == "" || json.Unmarshal([]byte(raw), &value) != nil {
		return false
	}
	var visit func(any) bool
	visit = func(item any) bool {
		switch typed := item.(type) {
		case map[string]any:
			if id, _ := typed["resourceId"].(string); id == resourceID {
				return true
			}
			if key, _ := typed["storageKey"].(string); key == "resource:"+resourceID {
				return true
			}
			for _, child := range typed {
				if visit(child) {
					return true
				}
			}
		case []any:
			for _, child := range typed {
				if visit(child) {
					return true
				}
			}
		}
		return false
	}
	return visit(value)
}

func aggregateFilmVideoSequenceStatusTx(tx *gorm.DB, sequenceID string, at time.Time) error {
	var slots []model.FilmVideoSlot
	if err := tx.Select("status").Where("sequence_id = ?", sequenceID).Find(&slots).Error; err != nil {
		return err
	}
	if len(slots) == 0 {
		return ErrFilmProductionStateConflict
	}
	status := model.FilmVideoSequenceStatusReady
	allAccepted := true
	hasGenerating := false
	hasReview := false
	for _, slot := range slots {
		if slot.Status != model.FilmVideoSlotStatusAccepted {
			allAccepted = false
		}
		switch slot.Status {
		case model.FilmVideoSlotStatusQueued, model.FilmVideoSlotStatusRunning:
			hasGenerating = true
		case model.FilmVideoSlotStatusNeedsReview, model.FilmVideoSlotStatusFailed, model.FilmVideoSlotStatusCancelled, model.FilmVideoSlotStatusUncertain:
			hasReview = true
		}
	}
	switch {
	case allAccepted:
		// 单镜验收只证明各槽位可用；整组完成仍需人工连续性裁决。
		status = model.FilmVideoSequenceStatusNeedsReview
	case hasGenerating:
		status = model.FilmVideoSequenceStatusGenerating
	case hasReview:
		status = model.FilmVideoSequenceStatusNeedsReview
	}
	return tx.Model(&model.FilmVideoSequence{}).Where("id = ?", sequenceID).
		Updates(map[string]any{"status": status, "updated_at": at}).Error
}

func mustFilmVideoJSON(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
