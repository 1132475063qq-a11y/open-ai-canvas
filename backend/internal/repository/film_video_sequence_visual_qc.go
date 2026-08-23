package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FilmVideoSequenceVisualQCQuoteCreateCommand struct {
	Quote *model.FilmVideoSequenceVisualQCQuote
	At    time.Time
}

type FilmVideoSequenceVisualQCSubmitCommand struct {
	UserID           string
	ProjectID        string
	QuoteID          string
	QuoteFingerprint string
	IdempotencyKey   string
	AttemptID        string
	Task             *model.Task
	ActiveTaskLimit  int
	Event            AgentRuntimeEventInput
	At               time.Time
}

type FilmVideoSequenceVisualQCSubmitResult struct {
	Quote      model.FilmVideoSequenceVisualQCQuote
	Attempt    model.FilmVideoSequenceVisualQCAttempt
	Task       model.Task
	Billing    *model.BillingOrder
	Idempotent bool
}

type FilmVideoSequenceVisualQCAttemptDetail struct {
	Attempt model.FilmVideoSequenceVisualQCAttempt
	Task    model.Task
	Report  *model.FilmVideoSequenceReview
	Billing *model.BillingOrder
}

type FilmVideoSequenceVisualQCCompletionCommand struct {
	AttemptID string
	Report    *model.FilmVideoSequenceReview
	Artifact  *model.ProductionArtifact
	Revision  *model.ProductionArtifactRevision
	Event     AgentRuntimeEventInput
	At        time.Time
}

type filmVideoSequenceVisualQCQuoteScope struct {
	Project      model.Project
	RootRun      model.AgentRuntimeRun
	Sequence     model.FilmVideoSequence
	Ledger       model.FilmContinuityLedger
	LogicalModel model.LogicalModel
	Revision     model.LogicalModelRevision
	Route        model.LogicalModelRoute
	Channel      model.ModelChannel
	ChannelModel model.ChannelModel
}

func (r *Repository) FilmVideoSequenceVisualQCQuoteForUser(userID string, projectID string, quoteID string) (*model.FilmVideoSequenceVisualQCQuote, error) {
	var quote model.FilmVideoSequenceVisualQCQuote
	if err := r.db.First(&quote, "id = ? AND user_id = ? AND project_id = ?", quoteID, userID, projectID).Error; err != nil {
		return nil, err
	}
	return &quote, nil
}

func (r *Repository) FilmVideoSequenceVisualQCQuoteByIdempotency(userID string, key string) (*model.FilmVideoSequenceVisualQCQuote, error) {
	var quote model.FilmVideoSequenceVisualQCQuote
	if err := r.db.First(&quote, "user_id = ? AND idempotency_key = ?", userID, key).Error; err != nil {
		return nil, err
	}
	return &quote, nil
}

func (r *Repository) CreateFilmVideoSequenceVisualQCQuote(command FilmVideoSequenceVisualQCQuoteCreateCommand) error {
	if err := validateFilmVideoSequenceVisualQCQuote(command.Quote); err != nil {
		return err
	}
	now := runtimeCommandTime(command.At)
	return r.db.Transaction(func(tx *gorm.DB) error {
		var existing model.FilmVideoSequenceVisualQCQuote
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&existing, "user_id = ? AND idempotency_key = ?", command.Quote.UserID, command.Quote.IdempotencyKey).Error
		if err == nil {
			return ErrFilmProductionQuoteConflict
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if _, err := validateFilmVideoSequenceVisualQCQuoteScopeTx(tx, command.Quote); err != nil {
			return err
		}
		quote := *command.Quote
		quote.Status = model.FilmProductionQuoteStatusPending
		quote.ConsumedAt = nil
		quote.CreatedAt = now
		quote.UpdatedAt = now
		return tx.Create(&quote).Error
	})
}

func (r *Repository) SubmitFilmVideoSequenceVisualQCQuote(command FilmVideoSequenceVisualQCSubmitCommand) (*FilmVideoSequenceVisualQCSubmitResult, error) {
	if strings.TrimSpace(command.UserID) == "" || strings.TrimSpace(command.ProjectID) == "" || strings.TrimSpace(command.QuoteID) == "" ||
		strings.TrimSpace(command.QuoteFingerprint) == "" || strings.TrimSpace(command.IdempotencyKey) == "" || strings.TrimSpace(command.AttemptID) == "" ||
		command.Task == nil || command.ActiveTaskLimit < 1 {
		return nil, errors.New("Film video sequence visual QC submit identity is incomplete")
	}
	if err := validateAgentRuntimeEventInput(command.Event); err != nil {
		return nil, err
	}
	now := runtimeCommandTime(command.At)
	result := FilmVideoSequenceVisualQCSubmitResult{}
	expired := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var replay model.FilmVideoSequenceVisualQCAttempt
		replayErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&replay, "user_id = ? AND idempotency_key = ?", command.UserID, command.IdempotencyKey).Error
		if replayErr == nil {
			if replay.ProjectID != command.ProjectID || replay.QuoteID != command.QuoteID {
				return ErrFilmProductionStateConflict
			}
			return loadFilmVideoSequenceVisualQCSubmitResultTx(tx, replay, true, &result)
		}
		if !errors.Is(replayErr, gorm.ErrRecordNotFound) {
			return replayErr
		}

		var quote model.FilmVideoSequenceVisualQCQuote
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&quote, "id = ? AND user_id = ? AND project_id = ?", command.QuoteID, command.UserID, command.ProjectID).Error; err != nil {
			return err
		}
		if quote.QuoteFingerprint != command.QuoteFingerprint {
			return ErrFilmProductionQuoteConflict
		}
		if quote.Status == model.FilmProductionQuoteStatusConsumed {
			var existing model.FilmVideoSequenceVisualQCAttempt
			if err := tx.First(&existing, "quote_id = ?", quote.ID).Error; err != nil || existing.IdempotencyKey != command.IdempotencyKey {
				return ErrFilmProductionQuoteConsumed
			}
			return loadFilmVideoSequenceVisualQCSubmitResultTx(tx, existing, true, &result)
		}
		if quote.Status != model.FilmProductionQuoteStatusPending {
			return ErrFilmProductionQuoteConsumed
		}
		if !quote.ExpiresAt.After(now) {
			if err := tx.Model(&model.FilmVideoSequenceVisualQCQuote{}).Where("id = ? AND status = ?", quote.ID, model.FilmProductionQuoteStatusPending).
				Updates(map[string]any{"status": model.FilmProductionQuoteStatusExpired, "updated_at": now}).Error; err != nil {
				return err
			}
			expired = true
			return nil
		}
		scope, err := validateFilmVideoSequenceVisualQCQuoteScopeTx(tx, &quote)
		if err != nil {
			return err
		}
		var active int64
		if err := tx.Model(&model.FilmVideoSequenceVisualQCAttempt{}).Where("sequence_id = ? AND status IN ?", quote.SequenceID, []model.FilmProductionAttemptStatus{
			model.FilmProductionAttemptStatusQueued, model.FilmProductionAttemptStatusRunning, model.FilmProductionAttemptStatusUncertain,
		}).Count(&active).Error; err != nil {
			return err
		}
		if active != 0 {
			return ErrFilmProductionActiveAttempt
		}
		if err := enforceActiveTaskLimit(tx, command.UserID, command.ActiveTaskLimit); err != nil {
			return err
		}
		if err := validateFilmVideoSequenceVisualQCTaskForQuote(*command.Task, quote, command.AttemptID); err != nil {
			return err
		}
		billing, err := filmVideoSequenceVisualQCBillingFromQuote(quote)
		if err != nil {
			return err
		}
		task := *command.Task
		if billing != nil {
			if err := validateFilmVideoSequenceVisualQCBillingSnapshot(*billing, quote, scope); err != nil {
				return err
			}
			if err := reserveBillingOrder(tx, billing); err != nil {
				return err
			}
			task.BillingOrderID = billing.ID
		}
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		var maxNumber int
		if err := tx.Model(&model.FilmVideoSequenceVisualQCAttempt{}).Where("sequence_id = ?", quote.SequenceID).Select("COALESCE(MAX(number), 0)").Scan(&maxNumber).Error; err != nil {
			return err
		}
		attempt := filmVideoSequenceVisualQCAttemptFromQuote(quote, command, maxNumber+1, task.BillingOrderID, now)
		if err := tx.Create(&attempt).Error; err != nil {
			return err
		}
		consumed := tx.Model(&model.FilmVideoSequenceVisualQCQuote{}).Where("id = ? AND status = ?", quote.ID, model.FilmProductionQuoteStatusPending).
			Updates(map[string]any{"status": model.FilmProductionQuoteStatusConsumed, "consumed_at": now, "updated_at": now})
		if consumed.Error != nil {
			return consumed.Error
		}
		if consumed.RowsAffected != 1 {
			return ErrFilmProductionQuoteConsumed
		}
		if err := appendFilmProductionEventTx(tx, scope.RootRun, command.Event, attempt.ID, now); err != nil {
			return err
		}
		quote.Status = model.FilmProductionQuoteStatusConsumed
		quote.ConsumedAt = &now
		result = FilmVideoSequenceVisualQCSubmitResult{Quote: quote, Attempt: attempt, Task: task, Billing: billing}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if expired {
		return nil, ErrFilmProductionQuoteExpired
	}
	return &result, nil
}

func (r *Repository) FilmVideoSequenceVisualQCAttemptByTaskID(taskID string) (*model.FilmVideoSequenceVisualQCAttempt, error) {
	var attempt model.FilmVideoSequenceVisualQCAttempt
	if err := r.db.First(&attempt, "task_id = ?", taskID).Error; err != nil {
		return nil, err
	}
	return &attempt, nil
}

func (r *Repository) FilmVideoSequenceVisualQCAttemptByIdempotency(userID string, key string) (*model.FilmVideoSequenceVisualQCAttempt, error) {
	var attempt model.FilmVideoSequenceVisualQCAttempt
	if err := r.db.First(&attempt, "user_id = ? AND idempotency_key = ?", userID, key).Error; err != nil {
		return nil, err
	}
	return &attempt, nil
}

func (r *Repository) filmVideoSequenceVisualQCAttemptsForSequences(sequenceIDs []string) (map[string][]FilmVideoSequenceVisualQCAttemptDetail, error) {
	result := make(map[string][]FilmVideoSequenceVisualQCAttemptDetail)
	if len(sequenceIDs) == 0 {
		return result, nil
	}
	var attempts []model.FilmVideoSequenceVisualQCAttempt
	if err := r.db.Where("sequence_id IN ?", sequenceIDs).Order("sequence_id asc, number asc").Find(&attempts).Error; err != nil {
		return nil, err
	}
	if len(attempts) == 0 {
		return result, nil
	}
	taskIDs := make([]string, 0, len(attempts))
	reportIDs := make([]string, 0, len(attempts))
	orderIDs := make([]string, 0, len(attempts))
	for _, attempt := range attempts {
		taskIDs = append(taskIDs, attempt.TaskID)
		if attempt.ReportID != "" {
			reportIDs = append(reportIDs, attempt.ReportID)
		}
		if attempt.BillingOrderID != "" {
			orderIDs = append(orderIDs, attempt.BillingOrderID)
		}
	}
	var tasks []model.Task
	var reports []model.FilmVideoSequenceReview
	var orders []model.BillingOrder
	if err := r.db.Where("id IN ?", taskIDs).Find(&tasks).Error; err != nil {
		return nil, err
	}
	if len(reportIDs) > 0 {
		if err := r.db.Where("id IN ?", reportIDs).Find(&reports).Error; err != nil {
			return nil, err
		}
	}
	if len(orderIDs) > 0 {
		if err := r.db.Where("id IN ?", orderIDs).Find(&orders).Error; err != nil {
			return nil, err
		}
	}
	taskByID := make(map[string]model.Task, len(tasks))
	reportByID := make(map[string]model.FilmVideoSequenceReview, len(reports))
	orderByID := make(map[string]model.BillingOrder, len(orders))
	for _, task := range tasks {
		taskByID[task.ID] = task
	}
	for _, report := range reports {
		reportByID[report.ID] = report
	}
	for _, order := range orders {
		orderByID[order.ID] = order
	}
	for _, attempt := range attempts {
		detail := FilmVideoSequenceVisualQCAttemptDetail{Attempt: attempt, Task: taskByID[attempt.TaskID]}
		if report, ok := reportByID[attempt.ReportID]; ok {
			copy := report
			detail.Report = &copy
		}
		if order, ok := orderByID[attempt.BillingOrderID]; ok {
			copy := order
			detail.Billing = &copy
		}
		result[attempt.SequenceID] = append(result[attempt.SequenceID], detail)
	}
	return result, nil
}

func (r *Repository) SaveFilmVideoSequenceVisualQCTaskCompletion(task *model.Task, expected model.TaskStatus, command FilmVideoSequenceVisualQCCompletionCommand) error {
	if task == nil || task.Type != model.FilmVisualQCTaskTypeSequence || command.Report == nil || command.Artifact == nil || command.Revision == nil {
		return errors.New("Film video sequence visual QC completion identity is incomplete")
	}
	now := runtimeCommandTime(command.At)
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := saveTaskCompletionTx(tx, task, expected, nil, nil, nil); err != nil {
			return err
		}
		var attempt model.FilmVideoSequenceVisualQCAttempt
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&attempt, "id = ? AND task_id = ?", command.AttemptID, task.ID).Error; err != nil {
			return err
		}
		if attempt.Status != model.FilmProductionAttemptStatusQueued && attempt.Status != model.FilmProductionAttemptStatusRunning {
			return ErrFilmProductionStateConflict
		}
		report := command.Report
		if report.UserID != attempt.UserID || report.ProjectID != attempt.ProjectID || report.RootRunID != attempt.RootRunID || report.SequenceID != attempt.SequenceID ||
			report.LedgerID != attempt.LedgerID || report.Source != "model" || report.AssessmentKind != "sequence_visual_continuity_qc" ||
			report.ModelAttemptID != attempt.ID || report.Action != model.FilmVideoSequenceReviewActionHold || report.ScopeFingerprint != attempt.ScopeFingerprint ||
			report.ArtifactID != command.Artifact.ID || report.RevisionID != command.Revision.ID || command.Artifact.UserID != attempt.UserID ||
			command.Artifact.ProjectID != attempt.ProjectID || command.Artifact.Domain != "film" || command.Artifact.ArtifactType != "sequence-review" ||
			command.Revision.Status != model.ProductionArtifactStatusLocked || command.Revision.SourceRunID != attempt.RootRunID || command.Revision.SourceAttemptID != attempt.ID {
			return ErrFilmProductionStateConflict
		}
		if report.Decision != model.FilmVideoSequenceReviewDecisionPass && report.Decision != model.FilmVideoSequenceReviewDecisionUncertain && report.Decision != model.FilmVideoSequenceReviewDecisionFail {
			return ErrFilmProductionStateConflict
		}
		if _, _, err := createProductionArtifactRevisionTx(tx, ProductionArtifactRevisionCreate{UserID: attempt.UserID, Artifact: command.Artifact, Revision: command.Revision, ExpectedSequence: 0, At: now}); err != nil {
			return err
		}
		if err := tx.Create(report).Error; err != nil {
			return err
		}
		updated := tx.Model(&model.FilmVideoSequenceVisualQCAttempt{}).Where("id = ? AND task_id = ? AND status IN ?", attempt.ID, task.ID, []model.FilmProductionAttemptStatus{
			model.FilmProductionAttemptStatusQueued, model.FilmProductionAttemptStatusRunning,
		}).Updates(map[string]any{
			"status": model.FilmProductionAttemptStatusSucceeded, "report_id": report.ID,
			"report_artifact_id": command.Artifact.ID, "report_revision_id": command.Revision.ID,
			"error": "", "completed_at": now, "updated_at": now,
		})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrFilmProductionStateConflict
		}
		var root model.AgentRuntimeRun
		if err := lockAgentRuntimeRun(tx, attempt.UserID, attempt.RootRunID, &root); err != nil {
			return err
		}
		return appendFilmProductionEventTx(tx, root, command.Event, attempt.ID, now)
	})
}

func markFilmVideoSequenceVisualQCTaskAttemptRunningTx(tx *gorm.DB, taskID string, at time.Time) error {
	result := tx.Model(&model.FilmVideoSequenceVisualQCAttempt{}).Where("task_id = ? AND status = ?", taskID, model.FilmProductionAttemptStatusQueued).
		Updates(map[string]any{"status": model.FilmProductionAttemptStatusRunning, "started_at": at, "updated_at": at})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrFilmProductionStateConflict
	}
	return nil
}

func markFilmVideoSequenceVisualQCTaskAttemptTerminalTx(tx *gorm.DB, taskID string, status model.FilmProductionAttemptStatus, errorText string, at time.Time) error {
	result := tx.Model(&model.FilmVideoSequenceVisualQCAttempt{}).Where("task_id = ? AND status IN ?", taskID, []model.FilmProductionAttemptStatus{
		model.FilmProductionAttemptStatusQueued, model.FilmProductionAttemptStatusRunning, model.FilmProductionAttemptStatusUncertain,
	}).Updates(map[string]any{"status": status, "error": errorText, "completed_at": at, "updated_at": at})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrFilmProductionStateConflict
	}
	return nil
}

func validateFilmVideoSequenceVisualQCQuote(quote *model.FilmVideoSequenceVisualQCQuote) error {
	if quote == nil || strings.TrimSpace(quote.ID) == "" || strings.TrimSpace(quote.UserID) == "" || strings.TrimSpace(quote.IdempotencyKey) == "" ||
		strings.TrimSpace(quote.ProjectID) == "" || strings.TrimSpace(quote.RootRunID) == "" || strings.TrimSpace(quote.SequenceID) == "" || quote.SequenceRevision < 1 ||
		strings.TrimSpace(quote.LedgerID) == "" || strings.TrimSpace(quote.LedgerArtifactID) == "" || strings.TrimSpace(quote.LedgerRevisionID) == "" ||
		strings.TrimSpace(quote.LedgerDigest) == "" || strings.TrimSpace(quote.ScopeFingerprint) == "" || strings.TrimSpace(quote.PromptArtifactID) == "" ||
		strings.TrimSpace(quote.PromptRevisionID) == "" || strings.TrimSpace(quote.PromptDigest) == "" || strings.TrimSpace(quote.RegistryID) == "" ||
		strings.TrimSpace(quote.RegistryVersion) == "" || strings.TrimSpace(quote.RegistryDigest) == "" || strings.TrimSpace(quote.LogicalModelID) == "" ||
		strings.TrimSpace(quote.LogicalModelRevisionID) == "" || strings.TrimSpace(quote.RouteID) == "" || strings.TrimSpace(quote.ChannelID) == "" ||
		strings.TrimSpace(quote.ChannelModelID) == "" || strings.TrimSpace(quote.SlotEvidenceJSON) == "" || strings.TrimSpace(quote.ReferenceResourceIDsJSON) == "" ||
		strings.TrimSpace(quote.TaskID) == "" || strings.TrimSpace(quote.RequestJSON) == "" || strings.TrimSpace(quote.RequestFingerprint) == "" ||
		strings.TrimSpace(quote.QuoteFingerprint) == "" || quote.ExpiresAt.IsZero() {
		return errors.New("Film video sequence visual QC quote identity is incomplete")
	}
	return nil
}

func validateFilmVideoSequenceVisualQCQuoteScopeTx(tx *gorm.DB, quote *model.FilmVideoSequenceVisualQCQuote) (filmVideoSequenceVisualQCQuoteScope, error) {
	scope := filmVideoSequenceVisualQCQuoteScope{}
	lock := clause.Locking{Strength: "UPDATE"}
	if err := tx.Clauses(lock).First(&scope.Project, "id = ? AND user_id = ?", quote.ProjectID, quote.UserID).Error; err != nil {
		return scope, err
	}
	if scope.Project.Type != model.ProjectTypeShortDrama || scope.Project.Status != model.ProjectStatusActive {
		return scope, ErrFilmProductionQuoteDrift
	}
	if err := tx.Clauses(lock).First(&scope.RootRun, "id = ? AND user_id = ? AND project_id = ? AND domain = ?", quote.RootRunID, quote.UserID, quote.ProjectID, "film").Error; err != nil {
		return scope, err
	}
	if scope.RootRun.RootRunID != scope.RootRun.ID || scope.RootRun.RegistryID != quote.RegistryID || scope.RootRun.RegistryVersion != quote.RegistryVersion ||
		scope.RootRun.RegistryDigest != quote.RegistryDigest || scope.RootRun.Status == model.AgentRunStatusFailed || scope.RootRun.Status == model.AgentRunStatusCancelled {
		return scope, ErrFilmProductionQuoteDrift
	}
	if err := tx.Clauses(lock).First(&scope.Sequence, "id = ? AND user_id = ? AND project_id = ?", quote.SequenceID, quote.UserID, quote.ProjectID).Error; err != nil {
		return scope, err
	}
	if scope.Sequence.RootRunID != quote.RootRunID || scope.Sequence.Revision != quote.SequenceRevision || scope.Sequence.PromptArtifactID != quote.PromptArtifactID ||
		scope.Sequence.PromptArtifactRevisionID != quote.PromptRevisionID || scope.Sequence.PromptArtifactDigest != quote.PromptDigest || scope.Sequence.RegistryID != quote.RegistryID ||
		scope.Sequence.RegistryVersion != quote.RegistryVersion || scope.Sequence.RegistryDigest != quote.RegistryDigest {
		return scope, ErrFilmProductionQuoteDrift
	}
	if err := tx.Clauses(lock).First(&scope.Ledger, "id = ? AND sequence_id = ? AND project_id = ?", quote.LedgerID, quote.SequenceID, quote.ProjectID).Error; err != nil {
		return scope, err
	}
	if scope.Ledger.UserID != quote.UserID || scope.Ledger.RootRunID != quote.RootRunID || scope.Ledger.ArtifactID != quote.LedgerArtifactID || scope.Ledger.ArtifactRevisionID != quote.LedgerRevisionID {
		return scope, ErrFilmProductionQuoteDrift
	}
	if err := validateFilmVideoSequenceVisualQCArtifactTx(tx, quote.UserID, quote.ProjectID, quote.PromptArtifactID, quote.PromptRevisionID, quote.PromptDigest, ""); err != nil {
		return scope, err
	}
	if err := validateFilmVideoSequenceVisualQCArtifactTx(tx, quote.UserID, quote.ProjectID, quote.LedgerArtifactID, quote.LedgerRevisionID, quote.LedgerDigest, "continuity-ledger"); err != nil {
		return scope, err
	}
	var slots []model.FilmVideoSlot
	if err := tx.Clauses(lock).Where("sequence_id = ?", quote.SequenceID).Order("position asc").Find(&slots).Error; err != nil {
		return scope, err
	}
	var evidence []model.FilmVideoSequenceVisualQCSlotSnapshot
	if json.Unmarshal([]byte(quote.SlotEvidenceJSON), &evidence) != nil || len(evidence) == 0 || len(evidence) != len(slots) {
		return scope, ErrFilmProductionQuoteDrift
	}
	currentScope, err := filmVideoSequenceScopeFingerprintRows(scope.Sequence, slots, scope.Ledger)
	if err != nil || currentScope != quote.ScopeFingerprint {
		return scope, ErrFilmProductionQuoteDrift
	}
	referenceIDs := make([]string, 0)
	for index := range evidence {
		if err := validateFilmVideoSequenceVisualQCSlotSnapshotTx(tx, quote, slots[index], evidence[index], &referenceIDs); err != nil {
			return scope, err
		}
	}
	if mustFilmRepositoryJSON(referenceIDs) != quote.ReferenceResourceIDsJSON {
		return scope, ErrFilmProductionQuoteDrift
	}
	if err := tx.Clauses(lock).First(&scope.LogicalModel, "id = ?", quote.LogicalModelID).Error; err != nil {
		return scope, err
	}
	if !scope.LogicalModel.Enabled || scope.LogicalModel.ArchivedAt != nil || scope.LogicalModel.ActiveRevisionID != quote.LogicalModelRevisionID ||
		scope.LogicalModel.Code != quote.Model || scope.LogicalModel.Capability != "text" {
		return scope, ErrFilmProductionQuoteDrift
	}
	if err := tx.Clauses(lock).First(&scope.Revision, "id = ? AND logical_model_id = ?", quote.LogicalModelRevisionID, quote.LogicalModelID).Error; err != nil {
		return scope, err
	}
	if err := tx.Clauses(lock).First(&scope.Route, "id = ? AND logical_model_revision_id = ? AND channel_model_id = ?", quote.RouteID, quote.LogicalModelRevisionID, quote.ChannelModelID).Error; err != nil {
		return scope, err
	}
	if !scope.Route.Enabled || scope.Route.Weight <= 0 {
		return scope, ErrFilmProductionQuoteDrift
	}
	if err := tx.Clauses(lock).First(&scope.ChannelModel, "id = ? AND channel_id = ?", quote.ChannelModelID, quote.ChannelID).Error; err != nil {
		return scope, err
	}
	if !scope.ChannelModel.Enabled || scope.ChannelModel.ModelKey != quote.ProviderModel || scope.ChannelModel.Capability != "text" || scope.ChannelModel.Protocol != quote.Protocol ||
		scope.ChannelModel.CapabilityVersion != quote.CapabilityVersion || scope.ChannelModel.PriceVersion != quote.ChannelPriceVersion {
		return scope, ErrFilmProductionQuoteDrift
	}
	if err := tx.Clauses(lock).First(&scope.Channel, "id = ? AND scope = ? AND enabled = ?", quote.ChannelID, model.ChannelScopeSystem, true).Error; err != nil {
		return scope, err
	}
	if err := validateFilmVideoSequenceVisualQCRequestSnapshotTx(tx, quote, referenceIDs); err != nil {
		return scope, err
	}
	if billing, err := filmVideoSequenceVisualQCBillingFromQuote(*quote); err != nil {
		return scope, err
	} else if billing != nil {
		if err := validateFilmVideoSequenceVisualQCBillingSnapshot(*billing, *quote, scope); err != nil {
			return scope, err
		}
	}
	return scope, nil
}

func validateFilmVideoSequenceVisualQCSlotSnapshotTx(tx *gorm.DB, quote *model.FilmVideoSequenceVisualQCQuote, slot model.FilmVideoSlot, snapshot model.FilmVideoSequenceVisualQCSlotSnapshot, referenceIDs *[]string) error {
	lock := clause.Locking{Strength: "UPDATE"}
	if slot.Position != snapshot.Position || slot.ID != snapshot.SlotID || slot.Revision != snapshot.SlotRevision || slot.ShotID != snapshot.ShotID ||
		slot.DurationMs != snapshot.DurationMs || slot.Status != model.FilmVideoSlotStatusAccepted || slot.CurrentAttemptID != snapshot.SourceAttemptID ||
		slot.ResultID != snapshot.SourceResultID || slot.ResultArtifactID != snapshot.SourceResultArtifactID || slot.ResultRevisionID != snapshot.SourceResultRevisionID ||
		slot.SourceImageResourceID != snapshot.SourceImageResourceID || slot.SourceImageArtifactID != snapshot.SourceImageArtifactID || slot.SourceImageRevisionID != snapshot.SourceImageRevisionID ||
		len(snapshot.Samples) < 2 || len(snapshot.Samples) > 4 {
		return ErrFilmProductionQuoteDrift
	}
	var attempt model.FilmVideoAttempt
	if err := tx.Clauses(lock).First(&attempt, "id = ? AND user_id = ? AND project_id = ?", snapshot.SourceAttemptID, quote.UserID, quote.ProjectID).Error; err != nil {
		return err
	}
	if attempt.Status != model.FilmProductionAttemptStatusSucceeded || attempt.RootRunID != quote.RootRunID || attempt.SequenceID != quote.SequenceID || attempt.SlotID != slot.ID ||
		attempt.ResultID != snapshot.SourceResultID || attempt.ResultArtifactID != snapshot.SourceResultArtifactID || attempt.ResultRevisionID != snapshot.SourceResultRevisionID {
		return ErrFilmProductionQuoteDrift
	}
	var result model.Result
	if err := tx.Clauses(lock).First(&result, "id = ? AND attempt_id = ? AND availability = ?", snapshot.SourceResultID, snapshot.SourceAttemptID, model.ResultAvailabilityReady).Error; err != nil {
		return ErrFilmProductionMediaMissing
	}
	if result.ArtifactID != snapshot.SourceResultArtifactID || result.ArtifactRevisionID != snapshot.SourceResultRevisionID || !filmVideoResultReferencesResource(result.Payload, snapshot.SourceResourceID) {
		return ErrFilmProductionQuoteDrift
	}
	if err := validateFilmVideoSequenceVisualQCArtifactTx(tx, quote.UserID, quote.ProjectID, snapshot.SourceResultArtifactID, snapshot.SourceResultRevisionID, snapshot.SourceResultDigest, "generation-result"); err != nil {
		return err
	}
	if err := validateFilmVideoSequenceVisualQCArtifactTx(tx, quote.UserID, quote.ProjectID, snapshot.SourceImageArtifactID, snapshot.SourceImageRevisionID, snapshot.SourceImageRevisionDigest, "generation-result"); err != nil {
		return err
	}
	var video model.Resource
	if err := tx.Clauses(lock).First(&video, "id = ? AND user_id = ?", snapshot.SourceResourceID, quote.UserID).Error; err != nil {
		return ErrFilmProductionMediaMissing
	}
	if video.Status != model.ResourceStatusReady || video.Kind != "video" || !strings.HasPrefix(strings.ToLower(video.MimeType), "video/") ||
		video.MimeType != snapshot.SourceResourceMimeType || video.Size != snapshot.SourceResourceSize || video.DurationMs != snapshot.SourceResourceDurationMs || video.ETag != snapshot.SourceResourceETag {
		return ErrFilmProductionQuoteDrift
	}
	var sourceImage model.Resource
	if err := tx.Clauses(lock).First(&sourceImage, "id = ? AND user_id = ?", snapshot.SourceImageResourceID, quote.UserID).Error; err != nil {
		return ErrFilmProductionMediaMissing
	}
	if sourceImage.Status != model.ResourceStatusReady || sourceImage.Kind != "image" || !strings.HasPrefix(strings.ToLower(sourceImage.MimeType), "image/") {
		return ErrFilmProductionMediaMissing
	}
	previousTime := int64(-1)
	for _, sample := range snapshot.Samples {
		if sample.TimeMs < 0 || sample.TimeMs > snapshot.DurationMs || sample.TimeMs <= previousTime || strings.TrimSpace(sample.ResourceID) == "" {
			return ErrFilmProductionQuoteDrift
		}
		previousTime = sample.TimeMs
		var resource model.Resource
		if err := tx.Clauses(lock).First(&resource, "id = ? AND user_id = ?", sample.ResourceID, quote.UserID).Error; err != nil {
			return ErrFilmProductionMediaMissing
		}
		if resource.Status != model.ResourceStatusReady || resource.Kind != "image" || !strings.HasPrefix(strings.ToLower(resource.MimeType), "image/") ||
			resource.MimeType != sample.MimeType || resource.Size != sample.Size || resource.Width != sample.Width || resource.Height != sample.Height || resource.ETag != sample.ETag {
			return ErrFilmProductionQuoteDrift
		}
		*referenceIDs = append(*referenceIDs, sample.ResourceID)
	}
	return nil
}

func validateFilmVideoSequenceVisualQCArtifactTx(tx *gorm.DB, userID string, projectID string, artifactID string, revisionID string, digest string, artifactType string) error {
	lock := clause.Locking{Strength: "UPDATE"}
	var artifact model.ProductionArtifact
	var revision model.ProductionArtifactRevision
	if err := tx.Clauses(lock).First(&artifact, "id = ? AND user_id = ? AND project_id = ? AND domain = ?", artifactID, userID, projectID, "film").Error; err != nil {
		return err
	}
	if err := tx.Clauses(lock).First(&revision, "id = ? AND artifact_id = ?", revisionID, artifactID).Error; err != nil {
		return err
	}
	if (artifactType != "" && artifact.ArtifactType != artifactType) || artifact.CurrentRevisionID != revision.ID || revision.Status != model.ProductionArtifactStatusLocked || revision.ContentDigest != digest {
		return ErrFilmProductionQuoteDrift
	}
	return nil
}

func filmVideoSequenceScopeFingerprintRows(sequence model.FilmVideoSequence, slots []model.FilmVideoSlot, ledger model.FilmContinuityLedger) (string, error) {
	type slotScope struct {
		ID               string                    `json:"id"`
		Position         int                       `json:"position"`
		CurrentAttemptID string                    `json:"currentAttemptId"`
		ResultID         string                    `json:"resultId"`
		Status           model.FilmVideoSlotStatus `json:"status"`
		Revision         int64                     `json:"revision"`
	}
	scope := struct {
		SequenceID       string      `json:"sequenceId"`
		SequenceRevision int64       `json:"sequenceRevision"`
		Slots            []slotScope `json:"slots"`
		LedgerID         string      `json:"ledgerId"`
		LedgerRevisionID string      `json:"ledgerRevisionId"`
	}{SequenceID: sequence.ID, SequenceRevision: sequence.Revision, LedgerID: ledger.ID, LedgerRevisionID: ledger.ArtifactRevisionID}
	for _, slot := range slots {
		scope.Slots = append(scope.Slots, slotScope{ID: slot.ID, Position: slot.Position, CurrentAttemptID: slot.CurrentAttemptID, ResultID: slot.ResultID, Status: slot.Status, Revision: slot.Revision})
	}
	encoded, err := json.Marshal(scope)
	if err != nil {
		return "", err
	}
	return filmRepositoryDigest(encoded), nil
}

func validateFilmVideoSequenceVisualQCRequestSnapshotTx(tx *gorm.DB, quote *model.FilmVideoSequenceVisualQCQuote, expectedResourceIDs []string) error {
	var input struct {
		Mode            string `json:"mode"`
		Prompt          string `json:"prompt"`
		ReferenceImages []struct {
			ID         string `json:"id"`
			DataURL    string `json:"dataUrl"`
			URL        string `json:"url"`
			StorageKey string `json:"storageKey"`
			MimeType   string `json:"mimeType"`
			Bytes      int64  `json:"bytes"`
			Width      int    `json:"width"`
			Height     int    `json:"height"`
		} `json:"referenceImages"`
		Config struct {
			ChannelID    string `json:"channelId"`
			Model        string `json:"model"`
			APIKey       string `json:"apiKey"`
			SecretKey    string `json:"secretKey"`
			SystemPrompt string `json:"systemPrompt"`
		} `json:"config"`
		Metadata struct {
			RootRunID  string `json:"filmRootRunId"`
			SequenceID string `json:"filmVideoSequenceId"`
		} `json:"metadata"`
	}
	if json.Unmarshal([]byte(quote.RequestJSON), &input) != nil || input.Mode != "text" || strings.TrimSpace(input.Prompt) == "" || strings.TrimSpace(input.Config.SystemPrompt) == "" ||
		input.Config.ChannelID != quote.ChannelID || strings.TrimPrefix(input.Config.Model, "models/") != quote.ProviderModel || input.Config.APIKey != "" || input.Config.SecretKey != "" ||
		input.Metadata.RootRunID != quote.RootRunID || input.Metadata.SequenceID != quote.SequenceID || len(input.ReferenceImages) != len(expectedResourceIDs) {
		return ErrFilmProductionQuoteDrift
	}
	for index, reference := range input.ReferenceImages {
		resourceID := strings.TrimPrefix(strings.TrimSpace(reference.StorageKey), "resource:")
		if resourceID != expectedResourceIDs[index] || reference.StorageKey != "resource:"+resourceID || reference.ID != resourceID || reference.DataURL != "" || reference.URL != "" {
			return ErrFilmProductionQuoteDrift
		}
		var resource model.Resource
		if err := tx.First(&resource, "id = ? AND user_id = ?", resourceID, quote.UserID).Error; err != nil {
			return ErrFilmProductionMediaMissing
		}
		if resource.Status != model.ResourceStatusReady || resource.Kind != "image" || reference.MimeType != resource.MimeType || reference.Bytes != resource.Size || reference.Width != resource.Width || reference.Height != resource.Height {
			return ErrFilmProductionQuoteDrift
		}
	}
	return nil
}

func validateFilmVideoSequenceVisualQCTaskForQuote(task model.Task, quote model.FilmVideoSequenceVisualQCQuote, attemptID string) error {
	if task.ID != quote.TaskID || task.UserID != quote.UserID || task.ProjectID != quote.ProjectID || task.Type != model.FilmVisualQCTaskTypeSequence ||
		task.Operation != "film_video_sequence_visual_qc" || task.Status != model.TaskStatusQueued || task.LogicalModelID != quote.LogicalModelID ||
		task.LogicalModelRevisionID != quote.LogicalModelRevisionID || task.RouteID != quote.RouteID || task.ChannelModelID != quote.ChannelModelID ||
		task.RouteRun != 1 || task.Model != quote.Model || task.Provider != "managed" || strings.TrimSpace(task.InputJSON) == "" {
		return ErrFilmProductionStateConflict
	}
	var input struct {
		Mode     string `json:"mode"`
		Prompt   string `json:"prompt"`
		Metadata struct {
			QuoteID    string `json:"filmVideoSequenceVisualQCQuoteId"`
			AttemptID  string `json:"filmVideoSequenceVisualQCAttemptId"`
			SequenceID string `json:"filmVideoSequenceId"`
		} `json:"metadata"`
	}
	if json.Unmarshal([]byte(task.InputJSON), &input) != nil || input.Mode != "text" || strings.TrimSpace(input.Prompt) == "" || task.Prompt != input.Prompt ||
		input.Metadata.QuoteID != quote.ID || input.Metadata.AttemptID != attemptID || input.Metadata.SequenceID != quote.SequenceID {
		return ErrFilmProductionStateConflict
	}
	return nil
}

func filmVideoSequenceVisualQCBillingFromQuote(quote model.FilmVideoSequenceVisualQCQuote) (*model.BillingOrder, error) {
	if strings.TrimSpace(quote.BillingJSON) == "" || strings.TrimSpace(quote.BillingJSON) == "null" {
		return nil, nil
	}
	var order model.BillingOrder
	if json.Unmarshal([]byte(quote.BillingJSON), &order) != nil {
		return nil, ErrFilmProductionQuoteDrift
	}
	return &order, nil
}

func validateFilmVideoSequenceVisualQCBillingSnapshot(order model.BillingOrder, quote model.FilmVideoSequenceVisualQCQuote, scope filmVideoSequenceVisualQCQuoteScope) error {
	if strings.TrimSpace(order.ID) == "" || strings.TrimSpace(order.IdempotencyKey) == "" || order.UserID != quote.UserID || order.TaskID != quote.TaskID ||
		order.ChannelID != quote.ChannelID || order.ChannelModelID != quote.ChannelModelID || order.Model != quote.Model || order.Capability != "text" ||
		order.Status != model.BillingStatusReserved || order.AmountMicrocredits <= 0 || order.ReservedAmountMicrocredits != order.AmountMicrocredits || order.Quantity <= 0 {
		return ErrFilmProductionQuoteDrift
	}
	switch scope.LogicalModel.PricePolicy {
	case "channel":
		if !scope.ChannelModel.PriceConfigured || order.PriceVersion != scope.ChannelModel.PriceVersion || order.BillingMode != scope.ChannelModel.BillingMode ||
			order.UnitPriceMicrocredits != scope.ChannelModel.UnitPriceMicrocredits || order.InputTokenPriceMicrocredits != scope.ChannelModel.InputTokenPriceMicrocredits ||
			order.OutputTokenPriceMicrocredits != scope.ChannelModel.OutputTokenPriceMicrocredits || order.CachedTokenPriceMicrocredits != scope.ChannelModel.CachedTokenPriceMicrocredits {
			return ErrFilmProductionQuoteDrift
		}
	case "unified":
		if order.PriceVersion != int64(scope.Revision.Version) || order.BillingMode != scope.LogicalModel.BillingMode || order.UnitPriceMicrocredits != scope.LogicalModel.UnitPriceMicrocredits ||
			order.InputTokenPriceMicrocredits != scope.LogicalModel.InputPriceMicrocredits || order.OutputTokenPriceMicrocredits != scope.LogicalModel.OutputPriceMicrocredits ||
			order.CachedTokenPriceMicrocredits != scope.LogicalModel.CachedPriceMicrocredits {
			return ErrFilmProductionQuoteDrift
		}
	default:
		return ErrFilmProductionQuoteDrift
	}
	return nil
}

func filmVideoSequenceVisualQCAttemptFromQuote(quote model.FilmVideoSequenceVisualQCQuote, command FilmVideoSequenceVisualQCSubmitCommand, number int, billingOrderID string, at time.Time) model.FilmVideoSequenceVisualQCAttempt {
	return model.FilmVideoSequenceVisualQCAttempt{
		ID: command.AttemptID, UserID: quote.UserID, IdempotencyKey: command.IdempotencyKey, ProjectID: quote.ProjectID, RootRunID: quote.RootRunID,
		SequenceID: quote.SequenceID, SequenceRevision: quote.SequenceRevision, LedgerID: quote.LedgerID, LedgerArtifactID: quote.LedgerArtifactID,
		LedgerRevisionID: quote.LedgerRevisionID, LedgerDigest: quote.LedgerDigest, ScopeFingerprint: quote.ScopeFingerprint,
		PromptArtifactID: quote.PromptArtifactID, PromptRevisionID: quote.PromptRevisionID, PromptDigest: quote.PromptDigest, SlotEvidenceJSON: quote.SlotEvidenceJSON,
		Number: number, QuoteID: quote.ID, TaskID: quote.TaskID, BillingOrderID: billingOrderID, RegistryID: quote.RegistryID,
		RegistryVersion: quote.RegistryVersion, RegistryDigest: quote.RegistryDigest, LogicalModelID: quote.LogicalModelID,
		LogicalModelRevisionID: quote.LogicalModelRevisionID, RouteID: quote.RouteID, ChannelID: quote.ChannelID, ChannelModelID: quote.ChannelModelID,
		Model: quote.Model, ProviderModel: quote.ProviderModel, Protocol: quote.Protocol, CapabilityVersion: quote.CapabilityVersion,
		ChannelPriceVersion: quote.ChannelPriceVersion, RequestFingerprint: quote.RequestFingerprint, Status: model.FilmProductionAttemptStatusQueued,
		CreatedAt: at, UpdatedAt: at,
	}
}

func loadFilmVideoSequenceVisualQCSubmitResultTx(tx *gorm.DB, attempt model.FilmVideoSequenceVisualQCAttempt, idempotent bool, result *FilmVideoSequenceVisualQCSubmitResult) error {
	var quote model.FilmVideoSequenceVisualQCQuote
	var task model.Task
	if err := tx.First(&quote, "id = ?", attempt.QuoteID).Error; err != nil {
		return err
	}
	if err := tx.First(&task, "id = ?", attempt.TaskID).Error; err != nil {
		return err
	}
	var billing *model.BillingOrder
	if attempt.BillingOrderID != "" {
		var order model.BillingOrder
		if err := tx.First(&order, "id = ?", attempt.BillingOrderID).Error; err != nil {
			return err
		}
		billing = &order
	}
	*result = FilmVideoSequenceVisualQCSubmitResult{Quote: quote, Attempt: attempt, Task: task, Billing: billing, Idempotent: idempotent}
	return nil
}

func filmRepositoryDigest(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}
