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

type FilmVideoQuoteCreateCommand struct {
	Quote *model.FilmVideoQuote
	At    time.Time
}

type FilmVideoSubmitCommand struct {
	UserID            string
	ProjectID         string
	QuoteID           string
	QuoteFingerprint  string
	IdempotencyKey    string
	AttemptID         string
	AttemptArtifactID string
	AttemptRevisionID string
	Task              *model.Task
	ActiveTaskLimit   int
	Event             AgentRuntimeEventInput
	At                time.Time
}

type FilmVideoSubmitResult struct {
	Quote      model.FilmVideoQuote
	Attempt    model.FilmVideoAttempt
	Task       model.Task
	Billing    *model.BillingOrder
	Idempotent bool
}

type filmVideoQuoteScope struct {
	Project      model.Project
	RootRun      model.AgentRuntimeRun
	Sequence     model.FilmVideoSequence
	Slot         model.FilmVideoSlot
	LogicalModel model.LogicalModel
	Revision     model.LogicalModelRevision
	Route        model.LogicalModelRoute
	Channel      model.ModelChannel
	ChannelModel model.ChannelModel
}

func (r *Repository) FilmVideoQuoteForUser(userID string, projectID string, quoteID string) (*model.FilmVideoQuote, error) {
	var quote model.FilmVideoQuote
	if err := r.db.First(&quote, "id = ? AND user_id = ? AND project_id = ?", quoteID, userID, projectID).Error; err != nil {
		return nil, err
	}
	return &quote, nil
}

func (r *Repository) FilmVideoQuoteByIdempotency(userID string, key string) (*model.FilmVideoQuote, error) {
	var quote model.FilmVideoQuote
	if err := r.db.First(&quote, "user_id = ? AND idempotency_key = ?", userID, key).Error; err != nil {
		return nil, err
	}
	return &quote, nil
}

func (r *Repository) CreateFilmVideoQuote(command FilmVideoQuoteCreateCommand) error {
	if err := validateFilmVideoQuote(command.Quote); err != nil {
		return err
	}
	now := runtimeCommandTime(command.At)
	return r.db.Transaction(func(tx *gorm.DB) error {
		var existing model.FilmVideoQuote
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&existing, "user_id = ? AND idempotency_key = ?", command.Quote.UserID, command.Quote.IdempotencyKey).Error
		if err == nil {
			return ErrFilmProductionQuoteConflict
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if _, err := validateFilmVideoQuoteScopeTx(tx, command.Quote); err != nil {
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

func (r *Repository) SubmitFilmVideoQuote(command FilmVideoSubmitCommand) (*FilmVideoSubmitResult, error) {
	if err := validateFilmVideoSubmitCommand(command); err != nil {
		return nil, err
	}
	now := runtimeCommandTime(command.At)
	result := FilmVideoSubmitResult{}
	expired := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var replay model.FilmVideoAttempt
		replayErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&replay, "user_id = ? AND idempotency_key = ?", command.UserID, command.IdempotencyKey).Error
		if replayErr == nil {
			if replay.ProjectID != command.ProjectID || replay.QuoteID != command.QuoteID {
				return ErrFilmProductionStateConflict
			}
			return loadFilmVideoSubmitResultTx(tx, replay, true, &result)
		}
		if !errors.Is(replayErr, gorm.ErrRecordNotFound) {
			return replayErr
		}

		var quote model.FilmVideoQuote
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&quote, "id = ? AND user_id = ? AND project_id = ?", command.QuoteID, command.UserID, command.ProjectID).Error; err != nil {
			return err
		}
		if quote.QuoteFingerprint != command.QuoteFingerprint {
			return ErrFilmProductionQuoteConflict
		}
		if quote.Status == model.FilmProductionQuoteStatusConsumed {
			var existing model.FilmVideoAttempt
			if err := tx.First(&existing, "quote_id = ?", quote.ID).Error; err != nil {
				return ErrFilmProductionQuoteConsumed
			}
			if existing.IdempotencyKey != command.IdempotencyKey {
				return ErrFilmProductionQuoteConsumed
			}
			return loadFilmVideoSubmitResultTx(tx, existing, true, &result)
		}
		if quote.Status != model.FilmProductionQuoteStatusPending {
			return ErrFilmProductionQuoteConsumed
		}
		if !quote.ExpiresAt.After(now) {
			updated := tx.Model(&model.FilmVideoQuote{}).
				Where("id = ? AND status = ?", quote.ID, model.FilmProductionQuoteStatusPending).
				Updates(map[string]any{"status": model.FilmProductionQuoteStatusExpired, "updated_at": now})
			if updated.Error != nil {
				return updated.Error
			}
			expired = true
			result.Quote = quote
			result.Quote.Status = model.FilmProductionQuoteStatusExpired
			return nil
		}
		scope, err := validateFilmVideoQuoteScopeTx(tx, &quote)
		if err != nil {
			return err
		}
		if err := validateFilmVideoRetryTx(tx, quote); err != nil {
			return err
		}
		var active int64
		if err := tx.Model(&model.FilmVideoAttempt{}).Where("slot_id = ? AND status IN ?", quote.SlotID, []model.FilmProductionAttemptStatus{
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
		var maxNumber int
		if err := tx.Model(&model.FilmVideoAttempt{}).Where("slot_id = ?", quote.SlotID).
			Select("COALESCE(MAX(number), 0)").Scan(&maxNumber).Error; err != nil {
			return err
		}
		task := *command.Task
		if err := validateFilmVideoTaskForQuote(task, quote, command.AttemptID); err != nil {
			return err
		}
		billing, err := filmVideoBillingFromQuote(quote)
		if err != nil {
			return err
		}
		if billing != nil {
			if err := validateFilmVideoBillingSnapshot(*billing, quote, scope); err != nil {
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
		attempt := filmVideoAttemptFromQuote(quote, command, maxNumber+1, task.BillingOrderID, now)
		artifact, revision, err := createFilmVideoAttemptArtifact(quote, attempt, command, now)
		if err != nil {
			return err
		}
		attempt.AttemptArtifactID = artifact.ID
		attempt.AttemptRevisionID = revision.ID
		if err := tx.Create(&attempt).Error; err != nil {
			return err
		}
		if _, _, err := createProductionArtifactRevisionTx(tx, ProductionArtifactRevisionCreate{
			UserID: quote.UserID, Artifact: &artifact, Revision: &revision, ExpectedSequence: 0, At: now,
		}); err != nil {
			return err
		}
		updatedSlot := tx.Model(&model.FilmVideoSlot{}).
			Where("id = ? AND sequence_id = ? AND revision = ? AND status NOT IN ?", quote.SlotID, quote.SequenceID, quote.SlotRevision,
				[]model.FilmVideoSlotStatus{model.FilmVideoSlotStatusQueued, model.FilmVideoSlotStatusRunning}).
			Updates(map[string]any{
				"current_attempt_id": attempt.ID, "result_id": "", "result_artifact_id": "", "result_revision_id": "",
				"status": model.FilmVideoSlotStatusQueued, "updated_at": now,
			})
		if updatedSlot.Error != nil {
			return updatedSlot.Error
		}
		if updatedSlot.RowsAffected != 1 {
			return ErrFilmProductionStateConflict
		}
		consumed := tx.Model(&model.FilmVideoQuote{}).
			Where("id = ? AND status = ?", quote.ID, model.FilmProductionQuoteStatusPending).
			Updates(map[string]any{"status": model.FilmProductionQuoteStatusConsumed, "consumed_at": now, "updated_at": now})
		if consumed.Error != nil {
			return consumed.Error
		}
		if consumed.RowsAffected != 1 {
			return ErrFilmProductionQuoteConsumed
		}
		if err := aggregateFilmVideoSequenceStatusTx(tx, quote.SequenceID, now); err != nil {
			return err
		}
		if err := appendFilmProductionEventTx(tx, scope.RootRun, command.Event, attempt.ID, now); err != nil {
			return err
		}
		quote.Status = model.FilmProductionQuoteStatusConsumed
		quote.ConsumedAt = &now
		quote.UpdatedAt = now
		result = FilmVideoSubmitResult{Quote: quote, Attempt: attempt, Task: task, Billing: billing}
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

func (r *Repository) FilmVideoAttemptByTaskID(taskID string) (*model.FilmVideoAttempt, error) {
	var attempt model.FilmVideoAttempt
	if err := r.db.First(&attempt, "task_id = ?", taskID).Error; err != nil {
		return nil, err
	}
	return &attempt, nil
}

func (r *Repository) FilmVideoAttemptByIdempotency(userID string, key string) (*model.FilmVideoAttempt, error) {
	var attempt model.FilmVideoAttempt
	if err := r.db.First(&attempt, "user_id = ? AND idempotency_key = ?", userID, key).Error; err != nil {
		return nil, err
	}
	return &attempt, nil
}

func (r *Repository) FilmVideoAttemptForUser(userID string, projectID string, attemptID string) (*FilmVideoAttemptDetail, error) {
	var attempt model.FilmVideoAttempt
	if err := r.db.First(&attempt, "id = ? AND user_id = ? AND project_id = ?", attemptID, userID, projectID).Error; err != nil {
		return nil, err
	}
	details, err := r.hydrateFilmVideoAttemptsForSlots([]string{attempt.SlotID})
	if err != nil {
		return nil, err
	}
	for index := range details {
		if details[index].Attempt.ID == attemptID {
			return &details[index], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func validateFilmVideoQuote(quote *model.FilmVideoQuote) error {
	if quote == nil || strings.TrimSpace(quote.ID) == "" || strings.TrimSpace(quote.UserID) == "" ||
		strings.TrimSpace(quote.IdempotencyKey) == "" || strings.TrimSpace(quote.ProjectID) == "" || strings.TrimSpace(quote.RootRunID) == "" ||
		strings.TrimSpace(quote.SequenceID) == "" || quote.SequenceRevision < 1 || strings.TrimSpace(quote.SlotID) == "" || quote.SlotRevision < 1 ||
		strings.TrimSpace(quote.TaskID) == "" || strings.TrimSpace(quote.SourceImageAttemptID) == "" || strings.TrimSpace(quote.SourceImageResultID) == "" ||
		strings.TrimSpace(quote.SourceImageResourceID) == "" || strings.TrimSpace(quote.SourceImageArtifactID) == "" || strings.TrimSpace(quote.SourceImageRevisionID) == "" ||
		strings.TrimSpace(quote.PromptArtifactID) == "" || strings.TrimSpace(quote.PromptArtifactRevisionID) == "" || strings.TrimSpace(quote.PromptArtifactDigest) == "" ||
		strings.TrimSpace(quote.RegistryDigest) == "" || strings.TrimSpace(quote.LogicalModelID) == "" || strings.TrimSpace(quote.LogicalModelRevisionID) == "" ||
		strings.TrimSpace(quote.RouteID) == "" || strings.TrimSpace(quote.ChannelID) == "" || strings.TrimSpace(quote.ChannelModelID) == "" ||
		strings.TrimSpace(quote.RequestJSON) == "" || strings.TrimSpace(quote.RequestFingerprint) == "" || strings.TrimSpace(quote.QuoteFingerprint) == "" || quote.ExpiresAt.IsZero() {
		return errors.New("Film video quote identity is incomplete")
	}
	return nil
}

func validateFilmVideoSubmitCommand(command FilmVideoSubmitCommand) error {
	if strings.TrimSpace(command.UserID) == "" || strings.TrimSpace(command.ProjectID) == "" || strings.TrimSpace(command.QuoteID) == "" ||
		strings.TrimSpace(command.QuoteFingerprint) == "" || strings.TrimSpace(command.IdempotencyKey) == "" || strings.TrimSpace(command.AttemptID) == "" ||
		strings.TrimSpace(command.AttemptArtifactID) == "" || strings.TrimSpace(command.AttemptRevisionID) == "" || command.Task == nil || command.ActiveTaskLimit < 1 {
		return errors.New("Film video submit identity is incomplete")
	}
	return validateAgentRuntimeEventInput(command.Event)
}

func validateFilmVideoQuoteScopeTx(tx *gorm.DB, quote *model.FilmVideoQuote) (filmVideoQuoteScope, error) {
	scope := filmVideoQuoteScope{}
	lock := clause.Locking{Strength: "UPDATE"}
	if err := tx.Clauses(lock).First(&scope.Project, "id = ? AND user_id = ?", quote.ProjectID, quote.UserID).Error; err != nil {
		return scope, err
	}
	if scope.Project.Type != "short-drama" || scope.Project.Status != model.ProjectStatusActive {
		return scope, ErrFilmProductionQuoteDrift
	}
	if err := tx.Clauses(lock).First(&scope.RootRun, "id = ? AND user_id = ? AND project_id = ? AND domain = ?", quote.RootRunID, quote.UserID, quote.ProjectID, "film").Error; err != nil {
		return scope, err
	}
	if scope.RootRun.RootRunID != scope.RootRun.ID || scope.RootRun.RegistryDigest != quote.RegistryDigest ||
		scope.RootRun.Status == model.AgentRunStatusFailed || scope.RootRun.Status == model.AgentRunStatusCancelled {
		return scope, ErrFilmProductionQuoteDrift
	}
	if err := tx.Clauses(lock).First(&scope.Sequence, "id = ? AND user_id = ? AND project_id = ?", quote.SequenceID, quote.UserID, quote.ProjectID).Error; err != nil {
		return scope, err
	}
	if scope.Sequence.RootRunID != quote.RootRunID || scope.Sequence.Revision != quote.SequenceRevision ||
		scope.Sequence.PromptArtifactID != quote.PromptArtifactID || scope.Sequence.PromptArtifactRevisionID != quote.PromptArtifactRevisionID ||
		scope.Sequence.PromptArtifactDigest != quote.PromptArtifactDigest || scope.Sequence.RegistryDigest != quote.RegistryDigest {
		return scope, ErrFilmProductionQuoteDrift
	}
	if err := tx.Clauses(lock).First(&scope.Slot, "id = ? AND sequence_id = ?", quote.SlotID, quote.SequenceID).Error; err != nil {
		return scope, err
	}
	if scope.Slot.Revision != quote.SlotRevision || scope.Slot.RootRunID != quote.RootRunID ||
		scope.Slot.SourceImageAttemptID != quote.SourceImageAttemptID || scope.Slot.SourceImageResultID != quote.SourceImageResultID ||
		scope.Slot.SourceImageResourceID != quote.SourceImageResourceID || scope.Slot.SourceImageArtifactID != quote.SourceImageArtifactID ||
		scope.Slot.SourceImageRevisionID != quote.SourceImageRevisionID {
		return scope, ErrFilmProductionQuoteDrift
	}
	if err := validateFilmVideoPromptArtifactTx(tx, &scope.Sequence); err != nil {
		return scope, err
	}
	if err := validateAcceptedFilmImageTx(tx, quote.UserID, quote.ProjectID, quote.SourceImageAttemptID, quote.SourceImageResultID,
		quote.SourceImageResourceID, quote.SourceImageArtifactID, quote.SourceImageRevisionID); err != nil {
		return scope, err
	}
	if err := tx.Clauses(lock).First(&scope.LogicalModel, "id = ?", quote.LogicalModelID).Error; err != nil {
		return scope, err
	}
	if !scope.LogicalModel.Enabled || scope.LogicalModel.ArchivedAt != nil || scope.LogicalModel.ActiveRevisionID != quote.LogicalModelRevisionID ||
		scope.LogicalModel.Code != quote.Model || scope.LogicalModel.Capability != "video" {
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
	if !scope.ChannelModel.Enabled || scope.ChannelModel.ModelKey != quote.ProviderModel || scope.ChannelModel.Capability != "video" ||
		scope.ChannelModel.Protocol != quote.Protocol || scope.ChannelModel.CapabilityVersion != quote.CapabilityVersion || scope.ChannelModel.PriceVersion != quote.ChannelPriceVersion {
		return scope, ErrFilmProductionQuoteDrift
	}
	if err := tx.Clauses(lock).First(&scope.Channel, "id = ? AND scope = ? AND enabled = ?", quote.ChannelID, model.ChannelScopeSystem, true).Error; err != nil {
		return scope, err
	}
	if err := validateFilmVideoRequestSnapshotTx(tx, quote, scope.Slot); err != nil {
		return scope, err
	}
	if billing, err := filmVideoBillingFromQuote(*quote); err != nil {
		return scope, err
	} else if billing != nil {
		if err := validateFilmVideoBillingSnapshot(*billing, *quote, scope); err != nil {
			return scope, err
		}
	}
	return scope, nil
}

func validateFilmVideoRequestSnapshotTx(tx *gorm.DB, quote *model.FilmVideoQuote, slot model.FilmVideoSlot) error {
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
			VideoSeconds string `json:"videoSeconds"`
			APIKey       string `json:"apiKey"`
			SecretKey    string `json:"secretKey"`
		} `json:"config"`
		Metadata struct {
			RootRunID  string `json:"filmRootRunId"`
			SequenceID string `json:"filmVideoSequenceId"`
			SlotID     string `json:"filmVideoSlotId"`
		} `json:"metadata"`
	}
	if json.Unmarshal([]byte(quote.RequestJSON), &input) != nil || input.Mode != "video" || strings.TrimSpace(input.Prompt) == "" ||
		input.Prompt != slot.Prompt || len(input.ReferenceImages) != 1 || input.Config.ChannelID != quote.ChannelID ||
		strings.TrimPrefix(input.Config.Model, "models/") != quote.ProviderModel || input.Config.APIKey != "" || input.Config.SecretKey != "" ||
		input.Metadata.RootRunID != quote.RootRunID || input.Metadata.SequenceID != quote.SequenceID || input.Metadata.SlotID != quote.SlotID {
		return ErrFilmProductionQuoteDrift
	}
	reference := input.ReferenceImages[0]
	if reference.ID != quote.SourceImageResourceID || reference.StorageKey != "resource:"+quote.SourceImageResourceID || reference.DataURL != "" || reference.URL != "" {
		return ErrFilmProductionQuoteDrift
	}
	var resource model.Resource
	if err := tx.First(&resource, "id = ? AND user_id = ?", quote.SourceImageResourceID, quote.UserID).Error; err != nil {
		return ErrFilmProductionMediaMissing
	}
	if reference.MimeType != resource.MimeType || reference.Bytes != resource.Size || reference.Width != resource.Width || reference.Height != resource.Height {
		return ErrFilmProductionQuoteDrift
	}
	return nil
}

func validateFilmVideoRetryTx(tx *gorm.DB, quote model.FilmVideoQuote) error {
	var attempts int64
	if err := tx.Model(&model.FilmVideoAttempt{}).Where("slot_id = ?", quote.SlotID).Count(&attempts).Error; err != nil {
		return err
	}
	if quote.RetryOfAttemptID == "" {
		if attempts != 0 {
			return ErrFilmProductionRetryNotAllowed
		}
		return nil
	}
	var source model.FilmVideoAttempt
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&source,
		"id = ? AND user_id = ? AND project_id = ? AND sequence_id = ? AND slot_id = ?",
		quote.RetryOfAttemptID, quote.UserID, quote.ProjectID, quote.SequenceID, quote.SlotID).Error; err != nil {
		return err
	}
	var latestNumber int
	if err := tx.Model(&model.FilmVideoAttempt{}).Where("slot_id = ?", quote.SlotID).
		Select("COALESCE(MAX(number), 0)").Scan(&latestNumber).Error; err != nil {
		return err
	}
	if source.Number != latestNumber {
		return ErrFilmProductionRetryNotAllowed
	}
	switch source.Status {
	case model.FilmProductionAttemptStatusFailed, model.FilmProductionAttemptStatusCancelled:
		return nil
	case model.FilmProductionAttemptStatusSucceeded:
		var report model.FilmVideoQCReport
		if err := tx.Where("attempt_id = ? AND source = ?", source.ID, "human").Order("created_at desc, id desc").First(&report).Error; err != nil {
			return ErrFilmProductionRetryNotAllowed
		}
		if report.Decision != model.FilmProductionQCDecisionFail || report.Action != model.FilmProductionQCActionRetry {
			return ErrFilmProductionRetryNotAllowed
		}
		return nil
	default:
		return ErrFilmProductionRetryNotAllowed
	}
}

func validateFilmVideoTaskForQuote(task model.Task, quote model.FilmVideoQuote, attemptID string) error {
	if task.ID != quote.TaskID || task.UserID != quote.UserID || task.ProjectID != quote.ProjectID ||
		task.Type != model.FilmProductionTaskTypeVideo || task.Operation != "image_to_video" || task.Status != model.TaskStatusQueued ||
		task.LogicalModelID != quote.LogicalModelID || task.LogicalModelRevisionID != quote.LogicalModelRevisionID ||
		task.RouteID != quote.RouteID || task.ChannelModelID != quote.ChannelModelID || task.RouteRun != 1 ||
		task.Model != quote.Model || task.Provider != "managed" || strings.TrimSpace(task.InputJSON) == "" {
		return ErrFilmProductionStateConflict
	}
	var input struct {
		Mode     string `json:"mode"`
		Prompt   string `json:"prompt"`
		Metadata struct {
			QuoteID    string `json:"filmVideoQuoteId"`
			AttemptID  string `json:"filmVideoAttemptId"`
			RootRunID  string `json:"filmRootRunId"`
			SequenceID string `json:"filmVideoSequenceId"`
			SlotID     string `json:"filmVideoSlotId"`
		} `json:"metadata"`
	}
	if json.Unmarshal([]byte(task.InputJSON), &input) != nil || input.Mode != "video" || strings.TrimSpace(input.Prompt) == "" ||
		task.Prompt != input.Prompt || input.Metadata.QuoteID != quote.ID || input.Metadata.AttemptID != attemptID ||
		input.Metadata.RootRunID != quote.RootRunID || input.Metadata.SequenceID != quote.SequenceID || input.Metadata.SlotID != quote.SlotID {
		return ErrFilmProductionStateConflict
	}
	return nil
}

func filmVideoBillingFromQuote(quote model.FilmVideoQuote) (*model.BillingOrder, error) {
	if strings.TrimSpace(quote.BillingJSON) == "" || strings.TrimSpace(quote.BillingJSON) == "null" {
		return nil, nil
	}
	var order model.BillingOrder
	if json.Unmarshal([]byte(quote.BillingJSON), &order) != nil {
		return nil, ErrFilmProductionQuoteDrift
	}
	return &order, nil
}

func validateFilmVideoBillingSnapshot(order model.BillingOrder, quote model.FilmVideoQuote, scope filmVideoQuoteScope) error {
	if strings.TrimSpace(order.ID) == "" || strings.TrimSpace(order.IdempotencyKey) == "" || order.UserID != quote.UserID ||
		order.TaskID != quote.TaskID || order.ChannelID != quote.ChannelID || order.ChannelModelID != quote.ChannelModelID ||
		order.Model != quote.Model || order.Capability != "video" || order.Status != model.BillingStatusReserved ||
		order.AmountMicrocredits <= 0 || order.ReservedAmountMicrocredits != order.AmountMicrocredits || order.Quantity <= 0 {
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
		if order.PriceVersion != int64(scope.Revision.Version) || order.BillingMode != scope.LogicalModel.BillingMode ||
			order.UnitPriceMicrocredits != scope.LogicalModel.UnitPriceMicrocredits || order.InputTokenPriceMicrocredits != scope.LogicalModel.InputPriceMicrocredits ||
			order.OutputTokenPriceMicrocredits != scope.LogicalModel.OutputPriceMicrocredits || order.CachedTokenPriceMicrocredits != scope.LogicalModel.CachedPriceMicrocredits {
			return ErrFilmProductionQuoteDrift
		}
	default:
		return ErrFilmProductionQuoteDrift
	}
	return nil
}

func filmVideoAttemptFromQuote(quote model.FilmVideoQuote, command FilmVideoSubmitCommand, number int, billingOrderID string, at time.Time) model.FilmVideoAttempt {
	return model.FilmVideoAttempt{
		ID: command.AttemptID, UserID: quote.UserID, IdempotencyKey: command.IdempotencyKey, ProjectID: quote.ProjectID,
		RootRunID: quote.RootRunID, SequenceID: quote.SequenceID, SlotID: quote.SlotID, Number: number,
		RetryOfAttemptID: quote.RetryOfAttemptID, QuoteID: quote.ID, TaskID: quote.TaskID, BillingOrderID: billingOrderID,
		SourceImageAttemptID: quote.SourceImageAttemptID, SourceImageResultID: quote.SourceImageResultID,
		SourceImageResourceID: quote.SourceImageResourceID, SourceImageRevisionID: quote.SourceImageRevisionID,
		PromptArtifactID: quote.PromptArtifactID, PromptRevisionID: quote.PromptArtifactRevisionID, PromptDigest: quote.PromptArtifactDigest,
		LogicalModelID: quote.LogicalModelID, LogicalModelRevisionID: quote.LogicalModelRevisionID,
		RouteID: quote.RouteID, ChannelID: quote.ChannelID, ChannelModelID: quote.ChannelModelID,
		Model: quote.Model, ProviderModel: quote.ProviderModel, Protocol: quote.Protocol,
		CapabilityVersion: quote.CapabilityVersion, ChannelPriceVersion: quote.ChannelPriceVersion,
		RequestFingerprint: quote.RequestFingerprint, Status: model.FilmProductionAttemptStatusQueued, CreatedAt: at, UpdatedAt: at,
	}
}

func createFilmVideoAttemptArtifact(quote model.FilmVideoQuote, attempt model.FilmVideoAttempt, command FilmVideoSubmitCommand, at time.Time) (model.ProductionArtifact, model.ProductionArtifactRevision, error) {
	content, err := json.Marshal(map[string]any{
		"schemaVersion": 2, "artifactType": "generation-attempt", "mediaType": "video", "attemptId": attempt.ID,
		"sequenceId": attempt.SequenceID, "slotId": attempt.SlotID, "attemptNumber": attempt.Number,
		"retryOf": emptyStringAsNil(attempt.RetryOfAttemptID), "taskId": attempt.TaskID, "executionState": "SUBMITTED",
		"requestFingerprint": attempt.RequestFingerprint,
		"logicalModel":       map[string]any{"id": attempt.LogicalModelID, "revisionId": attempt.LogicalModelRevisionID, "code": attempt.Model},
		"promptRef":          map[string]any{"artifactId": quote.PromptArtifactID, "revisionId": quote.PromptArtifactRevisionID, "digest": quote.PromptArtifactDigest},
		"sourceImageRef":     map[string]any{"attemptId": quote.SourceImageAttemptID, "resultId": quote.SourceImageResultID, "resourceId": quote.SourceImageResourceID},
		"resultId":           nil, "mediaRef": nil, "resultState": "UNKNOWN", "qcVerdict": "NOT_ASSESSABLE",
	})
	if err != nil {
		return model.ProductionArtifact{}, model.ProductionArtifactRevision{}, err
	}
	artifact := model.ProductionArtifact{
		ID: command.AttemptArtifactID, UserID: quote.UserID, ProjectID: quote.ProjectID, Domain: "film",
		ArtifactType: "generation-attempt", LogicalKey: "film-video:attempt:" + attempt.ID,
	}
	revision := model.ProductionArtifactRevision{
		ID: command.AttemptRevisionID, Status: model.ProductionArtifactStatusLocked, ContentJSON: string(content), ContentDigest: filmProductionDigest(content),
		SourceRunID: quote.RootRunID, SourceAttemptID: attempt.ID,
		SourceArtifactRefsJSON: mustFilmVideoJSON([]map[string]any{
			{"artifactId": quote.PromptArtifactID, "revisionId": quote.PromptArtifactRevisionID, "type": "ai-video-prompts"},
			{"artifactId": quote.SourceImageArtifactID, "revisionId": quote.SourceImageRevisionID, "type": "generation-result"},
		}),
		AuthorityRefsJSON: mustFilmVideoJSON([]map[string]any{{"kind": "registry", "digest": quote.RegistryDigest}, {"kind": "runtime", "id": "film-video-orchestrator-v1"}}),
		CreatedByType:     "runtime", CreatedByID: "film-video-orchestrator-v1", CreatedAt: at,
	}
	return artifact, revision, nil
}

func loadFilmVideoSubmitResultTx(tx *gorm.DB, attempt model.FilmVideoAttempt, idempotent bool, result *FilmVideoSubmitResult) error {
	var quote model.FilmVideoQuote
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
	*result = FilmVideoSubmitResult{Quote: quote, Attempt: attempt, Task: task, Billing: billing, Idempotent: idempotent}
	return nil
}
