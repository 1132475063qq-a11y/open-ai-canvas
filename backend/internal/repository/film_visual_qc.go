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

type FilmVisualQCQuoteCreateCommand struct {
	Quote *model.FilmVisualQCQuote
	At    time.Time
}

type FilmVisualQCSubmitCommand struct {
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

type FilmVisualQCSubmitResult struct {
	Quote      model.FilmVisualQCQuote
	Attempt    model.FilmVisualQCAttempt
	Task       model.Task
	Billing    *model.BillingOrder
	Idempotent bool
}

type FilmVisualQCAttemptDetail struct {
	Attempt model.FilmVisualQCAttempt
	Task    model.Task
	Report  *model.FilmProductionQCReport
	Billing *model.BillingOrder
}

type FilmVisualQCCompletionCommand struct {
	AttemptID string
	Report    *model.FilmProductionQCReport
	Artifact  *model.ProductionArtifact
	Revision  *model.ProductionArtifactRevision
	Event     AgentRuntimeEventInput
	At        time.Time
}

type filmVisualQCQuoteScope struct {
	Project        model.Project
	RootRun        model.AgentRuntimeRun
	Source         model.FilmProductionAttempt
	Result         model.Result
	ResultArtifact model.ProductionArtifact
	ResultRevision model.ProductionArtifactRevision
	LogicalModel   model.LogicalModel
	Revision       model.LogicalModelRevision
	Route          model.LogicalModelRoute
	Channel        model.ModelChannel
	ChannelModel   model.ChannelModel
}

func (r *Repository) FilmVisualQCQuoteForUser(userID string, projectID string, quoteID string) (*model.FilmVisualQCQuote, error) {
	var quote model.FilmVisualQCQuote
	if err := r.db.First(&quote, "id = ? AND user_id = ? AND project_id = ?", quoteID, userID, projectID).Error; err != nil {
		return nil, err
	}
	return &quote, nil
}

func (r *Repository) FilmVisualQCQuoteByIdempotency(userID string, key string) (*model.FilmVisualQCQuote, error) {
	var quote model.FilmVisualQCQuote
	if err := r.db.First(&quote, "user_id = ? AND idempotency_key = ?", userID, key).Error; err != nil {
		return nil, err
	}
	return &quote, nil
}

func (r *Repository) CreateFilmVisualQCQuote(command FilmVisualQCQuoteCreateCommand) error {
	if err := validateFilmVisualQCQuote(command.Quote); err != nil {
		return err
	}
	now := runtimeCommandTime(command.At)
	return r.db.Transaction(func(tx *gorm.DB) error {
		var existing model.FilmVisualQCQuote
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&existing, "user_id = ? AND idempotency_key = ?", command.Quote.UserID, command.Quote.IdempotencyKey).Error
		if err == nil {
			return ErrFilmProductionQuoteConflict
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if _, err := validateFilmVisualQCQuoteScopeTx(tx, command.Quote); err != nil {
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

func (r *Repository) SubmitFilmVisualQCQuote(command FilmVisualQCSubmitCommand) (*FilmVisualQCSubmitResult, error) {
	if strings.TrimSpace(command.UserID) == "" || strings.TrimSpace(command.ProjectID) == "" || strings.TrimSpace(command.QuoteID) == "" ||
		strings.TrimSpace(command.QuoteFingerprint) == "" || strings.TrimSpace(command.IdempotencyKey) == "" || strings.TrimSpace(command.AttemptID) == "" ||
		command.Task == nil || command.ActiveTaskLimit < 1 {
		return nil, errors.New("Film visual QC submit identity is incomplete")
	}
	if err := validateAgentRuntimeEventInput(command.Event); err != nil {
		return nil, err
	}
	now := runtimeCommandTime(command.At)
	result := FilmVisualQCSubmitResult{}
	expired := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var replay model.FilmVisualQCAttempt
		replayErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&replay, "user_id = ? AND idempotency_key = ?", command.UserID, command.IdempotencyKey).Error
		if replayErr == nil {
			if replay.ProjectID != command.ProjectID || replay.QuoteID != command.QuoteID {
				return ErrFilmProductionStateConflict
			}
			return loadFilmVisualQCSubmitResultTx(tx, replay, true, &result)
		}
		if !errors.Is(replayErr, gorm.ErrRecordNotFound) {
			return replayErr
		}

		var quote model.FilmVisualQCQuote
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&quote, "id = ? AND user_id = ? AND project_id = ?", command.QuoteID, command.UserID, command.ProjectID).Error; err != nil {
			return err
		}
		if quote.QuoteFingerprint != command.QuoteFingerprint {
			return ErrFilmProductionQuoteConflict
		}
		if quote.Status == model.FilmProductionQuoteStatusConsumed {
			var existing model.FilmVisualQCAttempt
			if err := tx.First(&existing, "quote_id = ?", quote.ID).Error; err != nil || existing.IdempotencyKey != command.IdempotencyKey {
				return ErrFilmProductionQuoteConsumed
			}
			return loadFilmVisualQCSubmitResultTx(tx, existing, true, &result)
		}
		if quote.Status != model.FilmProductionQuoteStatusPending {
			return ErrFilmProductionQuoteConsumed
		}
		if !quote.ExpiresAt.After(now) {
			if err := tx.Model(&model.FilmVisualQCQuote{}).Where("id = ? AND status = ?", quote.ID, model.FilmProductionQuoteStatusPending).
				Updates(map[string]any{"status": model.FilmProductionQuoteStatusExpired, "updated_at": now}).Error; err != nil {
				return err
			}
			expired = true
			return nil
		}
		scope, err := validateFilmVisualQCQuoteScopeTx(tx, &quote)
		if err != nil {
			return err
		}
		var active int64
		if err := tx.Model(&model.FilmVisualQCAttempt{}).Where("source_attempt_id = ? AND status IN ?", quote.SourceAttemptID, []model.FilmProductionAttemptStatus{
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
		if err := validateFilmVisualQCTaskForQuote(*command.Task, quote, command.AttemptID); err != nil {
			return err
		}
		billing, err := filmVisualQCBillingFromQuote(quote)
		if err != nil {
			return err
		}
		task := *command.Task
		if billing != nil {
			if err := validateFilmVisualQCBillingSnapshot(*billing, quote, scope); err != nil {
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
		if err := tx.Model(&model.FilmVisualQCAttempt{}).Where("source_attempt_id = ?", quote.SourceAttemptID).Select("COALESCE(MAX(number), 0)").Scan(&maxNumber).Error; err != nil {
			return err
		}
		attempt := filmVisualQCAttemptFromQuote(quote, command, maxNumber+1, task.BillingOrderID, now)
		if err := tx.Create(&attempt).Error; err != nil {
			return err
		}
		consumed := tx.Model(&model.FilmVisualQCQuote{}).Where("id = ? AND status = ?", quote.ID, model.FilmProductionQuoteStatusPending).
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
		result = FilmVisualQCSubmitResult{Quote: quote, Attempt: attempt, Task: task, Billing: billing}
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

func (r *Repository) FilmVisualQCAttemptByTaskID(taskID string) (*model.FilmVisualQCAttempt, error) {
	var attempt model.FilmVisualQCAttempt
	if err := r.db.First(&attempt, "task_id = ?", taskID).Error; err != nil {
		return nil, err
	}
	return &attempt, nil
}

func (r *Repository) FilmVisualQCAttemptByIdempotency(userID string, key string) (*model.FilmVisualQCAttempt, error) {
	var attempt model.FilmVisualQCAttempt
	if err := r.db.First(&attempt, "user_id = ? AND idempotency_key = ?", userID, key).Error; err != nil {
		return nil, err
	}
	return &attempt, nil
}

func (r *Repository) filmVisualQCAttemptsForSources(sourceAttemptIDs []string) (map[string][]FilmVisualQCAttemptDetail, error) {
	result := make(map[string][]FilmVisualQCAttemptDetail)
	if len(sourceAttemptIDs) == 0 {
		return result, nil
	}
	var attempts []model.FilmVisualQCAttempt
	if err := r.db.Where("source_attempt_id IN ?", sourceAttemptIDs).Order("created_at asc, id asc").Find(&attempts).Error; err != nil {
		return nil, err
	}
	if len(attempts) == 0 {
		return result, nil
	}
	taskIDs := make([]string, 0, len(attempts))
	reportIDs := make([]string, 0, len(attempts))
	billingIDs := make([]string, 0, len(attempts))
	for _, attempt := range attempts {
		taskIDs = append(taskIDs, attempt.TaskID)
		if attempt.ReportID != "" {
			reportIDs = append(reportIDs, attempt.ReportID)
		}
		if attempt.BillingOrderID != "" {
			billingIDs = append(billingIDs, attempt.BillingOrderID)
		}
	}
	var tasks []model.Task
	var reports []model.FilmProductionQCReport
	var billings []model.BillingOrder
	if err := r.db.Where("id IN ?", taskIDs).Find(&tasks).Error; err != nil {
		return nil, err
	}
	if len(reportIDs) > 0 {
		if err := r.db.Where("id IN ?", reportIDs).Find(&reports).Error; err != nil {
			return nil, err
		}
	}
	if len(billingIDs) > 0 {
		if err := r.db.Where("id IN ?", billingIDs).Find(&billings).Error; err != nil {
			return nil, err
		}
	}
	taskByID := make(map[string]model.Task, len(tasks))
	reportByID := make(map[string]model.FilmProductionQCReport, len(reports))
	billingByID := make(map[string]model.BillingOrder, len(billings))
	for _, task := range tasks {
		taskByID[task.ID] = task
	}
	for _, report := range reports {
		reportByID[report.ID] = report
	}
	for _, billing := range billings {
		billingByID[billing.ID] = billing
	}
	for _, attempt := range attempts {
		detail := FilmVisualQCAttemptDetail{Attempt: attempt, Task: taskByID[attempt.TaskID]}
		if report, ok := reportByID[attempt.ReportID]; ok {
			copy := report
			detail.Report = &copy
		}
		if billing, ok := billingByID[attempt.BillingOrderID]; ok {
			copy := billing
			detail.Billing = &copy
		}
		result[attempt.SourceAttemptID] = append(result[attempt.SourceAttemptID], detail)
	}
	return result, nil
}

func (r *Repository) SaveFilmVisualQCTaskCompletion(task *model.Task, expected model.TaskStatus, command FilmVisualQCCompletionCommand) error {
	if task == nil || task.Type != model.FilmVisualQCTaskTypeImage || command.Report == nil || command.Artifact == nil || command.Revision == nil {
		return errors.New("Film visual QC completion identity is incomplete")
	}
	now := runtimeCommandTime(command.At)
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := saveTaskCompletionTx(tx, task, expected, nil, nil, nil); err != nil {
			return err
		}
		var attempt model.FilmVisualQCAttempt
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&attempt, "id = ? AND task_id = ?", command.AttemptID, task.ID).Error; err != nil {
			return err
		}
		if attempt.Status != model.FilmProductionAttemptStatusQueued && attempt.Status != model.FilmProductionAttemptStatusRunning {
			return ErrFilmProductionStateConflict
		}
		report := command.Report
		if report.UserID != attempt.UserID || report.ProjectID != attempt.ProjectID || report.RootRunID != attempt.RootRunID || report.ShotID != attempt.ShotID ||
			report.AttemptID != attempt.SourceAttemptID || report.ResultID != attempt.SourceResultID || report.Source != "model" ||
			report.AssessmentKind != "visual_semantic_qc" || report.ModelAttemptID != attempt.ID || report.Action != model.FilmProductionQCActionHold ||
			report.ArtifactID != command.Artifact.ID || report.RevisionID != command.Revision.ID ||
			command.Artifact.UserID != attempt.UserID || command.Artifact.ProjectID != attempt.ProjectID || command.Artifact.Domain != "film" ||
			command.Artifact.ArtifactType != "generation-qc-report" || command.Revision.Status != model.ProductionArtifactStatusLocked ||
			command.Revision.SourceRunID != attempt.RootRunID || command.Revision.SourceAttemptID != attempt.ID {
			return ErrFilmProductionStateConflict
		}
		if report.Decision != model.FilmProductionQCDecisionPass && report.Decision != model.FilmProductionQCDecisionUncertain && report.Decision != model.FilmProductionQCDecisionFail {
			return ErrFilmProductionStateConflict
		}
		if _, _, err := createProductionArtifactRevisionTx(tx, ProductionArtifactRevisionCreate{
			UserID: attempt.UserID, Artifact: command.Artifact, Revision: command.Revision, ExpectedSequence: 0, At: now,
		}); err != nil {
			return err
		}
		if err := tx.Create(report).Error; err != nil {
			return err
		}
		updated := tx.Model(&model.FilmVisualQCAttempt{}).Where("id = ? AND task_id = ? AND status IN ?", attempt.ID, task.ID, []model.FilmProductionAttemptStatus{
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

func markFilmVisualQCTaskAttemptTerminalTx(tx *gorm.DB, taskID string, status model.FilmProductionAttemptStatus, errorText string, at time.Time) error {
	updates := map[string]any{"status": status, "error": errorText, "completed_at": at, "updated_at": at}
	result := tx.Model(&model.FilmVisualQCAttempt{}).Where("task_id = ? AND status IN ?", taskID, []model.FilmProductionAttemptStatus{
		model.FilmProductionAttemptStatusQueued, model.FilmProductionAttemptStatusRunning, model.FilmProductionAttemptStatusUncertain,
	}).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrFilmProductionStateConflict
	}
	return nil
}

func markFilmVisualQCTaskAttemptRunningTx(tx *gorm.DB, taskID string, at time.Time) error {
	result := tx.Model(&model.FilmVisualQCAttempt{}).
		Where("task_id = ? AND status = ?", taskID, model.FilmProductionAttemptStatusQueued).
		Updates(map[string]any{"status": model.FilmProductionAttemptStatusRunning, "started_at": at, "updated_at": at})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrFilmProductionStateConflict
	}
	return nil
}

func validateFilmVisualQCQuote(quote *model.FilmVisualQCQuote) error {
	if quote == nil || strings.TrimSpace(quote.ID) == "" || strings.TrimSpace(quote.UserID) == "" || strings.TrimSpace(quote.IdempotencyKey) == "" ||
		strings.TrimSpace(quote.ProjectID) == "" || strings.TrimSpace(quote.RootRunID) == "" || strings.TrimSpace(quote.ShotID) == "" ||
		strings.TrimSpace(quote.SourceAttemptID) == "" || strings.TrimSpace(quote.SourceResultID) == "" || strings.TrimSpace(quote.SourceResultArtifactID) == "" ||
		strings.TrimSpace(quote.SourceResultRevisionID) == "" || strings.TrimSpace(quote.SourceResultDigest) == "" || strings.TrimSpace(quote.TaskID) == "" ||
		strings.TrimSpace(quote.RegistryID) == "" || strings.TrimSpace(quote.RegistryVersion) == "" || strings.TrimSpace(quote.RegistryDigest) == "" ||
		strings.TrimSpace(quote.LogicalModelID) == "" || strings.TrimSpace(quote.LogicalModelRevisionID) == "" || strings.TrimSpace(quote.RouteID) == "" ||
		strings.TrimSpace(quote.ChannelID) == "" || strings.TrimSpace(quote.ChannelModelID) == "" || strings.TrimSpace(quote.RequestJSON) == "" ||
		strings.TrimSpace(quote.RequestFingerprint) == "" || strings.TrimSpace(quote.QuoteFingerprint) == "" || quote.ExpiresAt.IsZero() {
		return errors.New("Film visual QC quote identity is incomplete")
	}
	return nil
}

func validateFilmVisualQCQuoteScopeTx(tx *gorm.DB, quote *model.FilmVisualQCQuote) (filmVisualQCQuoteScope, error) {
	scope := filmVisualQCQuoteScope{}
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
	if scope.RootRun.RootRunID != scope.RootRun.ID || scope.RootRun.RegistryID != quote.RegistryID || scope.RootRun.RegistryVersion != quote.RegistryVersion || scope.RootRun.RegistryDigest != quote.RegistryDigest ||
		scope.RootRun.Status == model.AgentRunStatusFailed || scope.RootRun.Status == model.AgentRunStatusCancelled {
		return scope, ErrFilmProductionQuoteDrift
	}
	if err := tx.Clauses(lock).First(&scope.Source, "id = ? AND user_id = ? AND project_id = ?", quote.SourceAttemptID, quote.UserID, quote.ProjectID).Error; err != nil {
		return scope, err
	}
	if scope.Source.Status != model.FilmProductionAttemptStatusSucceeded || scope.Source.RootRunID != quote.RootRunID || scope.Source.ShotID != quote.ShotID ||
		scope.Source.ResultID != quote.SourceResultID || scope.Source.ResultArtifactID != quote.SourceResultArtifactID || scope.Source.ResultRevisionID != quote.SourceResultRevisionID ||
		scope.Source.StoryboardArtifactID != quote.StoryboardArtifactID || scope.Source.StoryboardRevisionID != quote.StoryboardRevisionID || scope.Source.StoryboardDigest != quote.StoryboardDigest ||
		scope.Source.PromptArtifactID != quote.PromptArtifactID || scope.Source.PromptRevisionID != quote.PromptRevisionID || scope.Source.PromptDigest != quote.PromptDigest ||
		scope.Source.FeasibilityArtifactID != quote.FeasibilityArtifactID || scope.Source.FeasibilityRevisionID != quote.FeasibilityRevisionID || scope.Source.FeasibilityDigest != quote.FeasibilityDigest {
		return scope, ErrFilmProductionQuoteDrift
	}
	if err := tx.Clauses(lock).First(&scope.Result, "id = ? AND attempt_id = ? AND availability = ?", quote.SourceResultID, quote.SourceAttemptID, model.ResultAvailabilityReady).Error; err != nil {
		return scope, ErrFilmProductionMediaMissing
	}
	if scope.Result.ArtifactID != quote.SourceResultArtifactID || scope.Result.ArtifactRevisionID != quote.SourceResultRevisionID {
		return scope, ErrFilmProductionQuoteDrift
	}
	if err := tx.Clauses(lock).First(&scope.ResultArtifact, "id = ? AND user_id = ? AND project_id = ? AND domain = ?", quote.SourceResultArtifactID, quote.UserID, quote.ProjectID, "film").Error; err != nil {
		return scope, err
	}
	if err := tx.Clauses(lock).First(&scope.ResultRevision, "id = ? AND artifact_id = ?", quote.SourceResultRevisionID, quote.SourceResultArtifactID).Error; err != nil {
		return scope, err
	}
	if scope.ResultArtifact.ArtifactType != "generation-result" || scope.ResultArtifact.CurrentRevisionID != scope.ResultRevision.ID || scope.ResultRevision.Status != model.ProductionArtifactStatusLocked || scope.ResultRevision.ContentDigest != quote.SourceResultDigest {
		return scope, ErrFilmProductionQuoteDrift
	}
	for _, expected := range []struct{ artifactID, revisionID, digest string }{
		{quote.StoryboardArtifactID, quote.StoryboardRevisionID, quote.StoryboardDigest},
		{quote.PromptArtifactID, quote.PromptRevisionID, quote.PromptDigest},
		{quote.FeasibilityArtifactID, quote.FeasibilityRevisionID, quote.FeasibilityDigest},
	} {
		var revision model.ProductionArtifactRevision
		if err := tx.Clauses(lock).First(&revision, "id = ? AND artifact_id = ?", expected.revisionID, expected.artifactID).Error; err != nil {
			return scope, err
		}
		if revision.Status != model.ProductionArtifactStatusLocked || revision.ContentDigest != expected.digest {
			return scope, ErrFilmProductionQuoteDrift
		}
	}
	if err := tx.Clauses(lock).First(&scope.LogicalModel, "id = ?", quote.LogicalModelID).Error; err != nil {
		return scope, err
	}
	if !scope.LogicalModel.Enabled || scope.LogicalModel.ArchivedAt != nil || scope.LogicalModel.ActiveRevisionID != quote.LogicalModelRevisionID || scope.LogicalModel.Code != quote.Model || scope.LogicalModel.Capability != "text" {
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
	if err := validateFilmVisualQCRequestSnapshotTx(tx, quote); err != nil {
		return scope, err
	}
	if billing, err := filmVisualQCBillingFromQuote(*quote); err != nil {
		return scope, err
	} else if billing != nil {
		if err := validateFilmVisualQCBillingSnapshot(*billing, *quote, scope); err != nil {
			return scope, err
		}
	}
	return scope, nil
}

func validateFilmVisualQCRequestSnapshotTx(tx *gorm.DB, quote *model.FilmVisualQCQuote) error {
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
			RootRunID       string `json:"filmRootRunId"`
			ShotID          string `json:"filmShotId"`
			SourceAttemptID string `json:"filmVisualQCSourceAttemptId"`
			SourceResultID  string `json:"filmVisualQCSourceResultId"`
		} `json:"metadata"`
	}
	if json.Unmarshal([]byte(quote.RequestJSON), &input) != nil || input.Mode != "text" || strings.TrimSpace(input.Prompt) == "" || strings.TrimSpace(input.Config.SystemPrompt) == "" ||
		input.Config.ChannelID != quote.ChannelID || strings.TrimPrefix(input.Config.Model, "models/") != quote.ProviderModel || input.Config.APIKey != "" || input.Config.SecretKey != "" ||
		input.Metadata.RootRunID != quote.RootRunID || input.Metadata.ShotID != quote.ShotID || input.Metadata.SourceAttemptID != quote.SourceAttemptID || input.Metadata.SourceResultID != quote.SourceResultID || len(input.ReferenceImages) == 0 {
		return ErrFilmProductionQuoteDrift
	}
	resourceIDs := make([]string, 0, len(input.ReferenceImages))
	for _, reference := range input.ReferenceImages {
		resourceID := strings.TrimPrefix(strings.TrimSpace(reference.StorageKey), "resource:")
		if resourceID == "" || reference.StorageKey != "resource:"+resourceID || reference.ID != resourceID || reference.DataURL != "" || reference.URL != "" {
			return ErrFilmProductionQuoteDrift
		}
		var resource model.Resource
		if err := tx.First(&resource, "id = ? AND user_id = ?", resourceID, quote.UserID).Error; err != nil {
			return ErrFilmProductionMediaMissing
		}
		if resource.Status != model.ResourceStatusReady || resource.Kind != "image" || reference.MimeType != resource.MimeType || reference.Bytes != resource.Size || reference.Width != resource.Width || reference.Height != resource.Height {
			return ErrFilmProductionQuoteDrift
		}
		resourceIDs = append(resourceIDs, resourceID)
	}
	if mustFilmRepositoryJSON(resourceIDs) != quote.ReferenceResourceIDsJSON || (quote.SourceResourceID != "" && resourceIDs[0] != quote.SourceResourceID) {
		return ErrFilmProductionQuoteDrift
	}
	return nil
}

func validateFilmVisualQCTaskForQuote(task model.Task, quote model.FilmVisualQCQuote, attemptID string) error {
	if task.ID != quote.TaskID || task.UserID != quote.UserID || task.ProjectID != quote.ProjectID || task.Type != model.FilmVisualQCTaskTypeImage || task.Operation != "film_visual_qc" ||
		task.Status != model.TaskStatusQueued || task.LogicalModelID != quote.LogicalModelID || task.LogicalModelRevisionID != quote.LogicalModelRevisionID || task.RouteID != quote.RouteID ||
		task.ChannelModelID != quote.ChannelModelID || task.RouteRun != 1 || task.Model != quote.Model || task.Provider != "managed" || strings.TrimSpace(task.InputJSON) == "" {
		return ErrFilmProductionStateConflict
	}
	var input struct {
		Mode     string `json:"mode"`
		Prompt   string `json:"prompt"`
		Metadata struct {
			QuoteID         string `json:"filmVisualQCQuoteId"`
			AttemptID       string `json:"filmVisualQCAttemptId"`
			SourceAttemptID string `json:"filmVisualQCSourceAttemptId"`
			SourceResultID  string `json:"filmVisualQCSourceResultId"`
		} `json:"metadata"`
	}
	if json.Unmarshal([]byte(task.InputJSON), &input) != nil || input.Mode != "text" || strings.TrimSpace(input.Prompt) == "" || task.Prompt != input.Prompt ||
		input.Metadata.QuoteID != quote.ID || input.Metadata.AttemptID != attemptID || input.Metadata.SourceAttemptID != quote.SourceAttemptID || input.Metadata.SourceResultID != quote.SourceResultID {
		return ErrFilmProductionStateConflict
	}
	return nil
}

func filmVisualQCBillingFromQuote(quote model.FilmVisualQCQuote) (*model.BillingOrder, error) {
	if strings.TrimSpace(quote.BillingJSON) == "" || strings.TrimSpace(quote.BillingJSON) == "null" {
		return nil, nil
	}
	var order model.BillingOrder
	if json.Unmarshal([]byte(quote.BillingJSON), &order) != nil {
		return nil, ErrFilmProductionQuoteDrift
	}
	return &order, nil
}

func validateFilmVisualQCBillingSnapshot(order model.BillingOrder, quote model.FilmVisualQCQuote, scope filmVisualQCQuoteScope) error {
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

func filmVisualQCAttemptFromQuote(quote model.FilmVisualQCQuote, command FilmVisualQCSubmitCommand, number int, billingOrderID string, at time.Time) model.FilmVisualQCAttempt {
	return model.FilmVisualQCAttempt{
		ID: command.AttemptID, UserID: quote.UserID, IdempotencyKey: command.IdempotencyKey, ProjectID: quote.ProjectID, RootRunID: quote.RootRunID,
		ShotID: quote.ShotID, SourceAttemptID: quote.SourceAttemptID, SourceResultID: quote.SourceResultID, SourceResourceID: quote.SourceResourceID,
		SourceResultArtifactID: quote.SourceResultArtifactID, SourceResultRevisionID: quote.SourceResultRevisionID, SourceResultDigest: quote.SourceResultDigest,
		Number: number, QuoteID: quote.ID, TaskID: quote.TaskID, BillingOrderID: billingOrderID,
		RegistryID: quote.RegistryID, RegistryVersion: quote.RegistryVersion, RegistryDigest: quote.RegistryDigest,
		LogicalModelID: quote.LogicalModelID, LogicalModelRevisionID: quote.LogicalModelRevisionID, RouteID: quote.RouteID, ChannelID: quote.ChannelID,
		ChannelModelID: quote.ChannelModelID, Model: quote.Model, ProviderModel: quote.ProviderModel, Protocol: quote.Protocol,
		CapabilityVersion: quote.CapabilityVersion, ChannelPriceVersion: quote.ChannelPriceVersion, RequestFingerprint: quote.RequestFingerprint,
		Status: model.FilmProductionAttemptStatusQueued, CreatedAt: at, UpdatedAt: at,
	}
}

func loadFilmVisualQCSubmitResultTx(tx *gorm.DB, attempt model.FilmVisualQCAttempt, idempotent bool, result *FilmVisualQCSubmitResult) error {
	var quote model.FilmVisualQCQuote
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
	*result = FilmVisualQCSubmitResult{Quote: quote, Attempt: attempt, Task: task, Billing: billing, Idempotent: idempotent}
	return nil
}

func mustFilmRepositoryJSON(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
