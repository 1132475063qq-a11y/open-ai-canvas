package repository

import (
	"errors"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrFilmProductionQuoteConflict   = errors.New("film production quote conflict")
	ErrFilmProductionQuoteExpired    = errors.New("film production quote expired")
	ErrFilmProductionQuoteConsumed   = errors.New("film production quote consumed")
	ErrFilmProductionQuoteDrift      = errors.New("film production quote facts changed")
	ErrFilmProductionActiveAttempt   = errors.New("film production attempt already active")
	ErrFilmProductionRetryNotAllowed = errors.New("film production retry not allowed")
	ErrFilmProductionStateConflict   = errors.New("film production state conflict")
	ErrFilmProductionMediaMissing    = errors.New("film production media is unavailable")
)

type FilmProductionQuoteCreateCommand struct {
	Quote *model.FilmProductionQuote
	At    time.Time
}

type FilmProductionSubmitCommand struct {
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

type FilmProductionSubmitResult struct {
	Quote      model.FilmProductionQuote
	Attempt    model.FilmProductionAttempt
	Task       model.Task
	Billing    *model.BillingOrder
	Idempotent bool
}

type FilmProductionAttemptDetail struct {
	Attempt         model.FilmProductionAttempt
	Task            model.Task
	Result          *model.Result
	QCReports       []model.FilmProductionQCReport
	VisualQCAttempts []FilmVisualQCAttemptDetail
	Billing         *model.BillingOrder
}

type FilmProductionCompletionCommand struct {
	AttemptID      string
	Result         *model.Result
	ResultArtifact *model.ProductionArtifact
	ResultRevision *model.ProductionArtifactRevision
	QCReport       *model.FilmProductionQCReport
	QCArtifact     *model.ProductionArtifact
	QCRevision     *model.ProductionArtifactRevision
	Event          AgentRuntimeEventInput
	At             time.Time
}

type FilmProductionHumanQCCommand struct {
	UserID    string
	ProjectID string
	AttemptID string
	Report    *model.FilmProductionQCReport
	Artifact  *model.ProductionArtifact
	Revision  *model.ProductionArtifactRevision
	Event     AgentRuntimeEventInput
	At        time.Time
}

type filmProductionQuoteScope struct {
	Project      model.Project
	RootRun      model.AgentRuntimeRun
	Shot         model.Shot
	LogicalModel model.LogicalModel
	Revision     model.LogicalModelRevision
	Route        model.LogicalModelRoute
	Channel      model.ModelChannel
	ChannelModel model.ChannelModel
}

func (r *Repository) FilmProductionQuoteForUser(userID string, projectID string, quoteID string) (*model.FilmProductionQuote, error) {
	var quote model.FilmProductionQuote
	if err := r.db.First(&quote, "id = ? AND user_id = ? AND project_id = ?", quoteID, userID, projectID).Error; err != nil {
		return nil, err
	}
	return &quote, nil
}

func (r *Repository) FilmProductionQuoteByIdempotency(userID string, key string) (*model.FilmProductionQuote, error) {
	var quote model.FilmProductionQuote
	if err := r.db.First(&quote, "user_id = ? AND idempotency_key = ?", userID, key).Error; err != nil {
		return nil, err
	}
	return &quote, nil
}

func (r *Repository) CreateFilmProductionQuote(command FilmProductionQuoteCreateCommand) error {
	if err := validateFilmProductionQuote(command.Quote); err != nil {
		return err
	}
	now := runtimeCommandTime(command.At)
	return r.db.Transaction(func(tx *gorm.DB) error {
		var existing model.FilmProductionQuote
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&existing, "user_id = ? AND idempotency_key = ?", command.Quote.UserID, command.Quote.IdempotencyKey).Error
		if err == nil {
			return ErrFilmProductionQuoteConflict
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if _, err := validateFilmProductionQuoteScopeTx(tx, command.Quote, true); err != nil {
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

func (r *Repository) SubmitFilmProductionQuote(command FilmProductionSubmitCommand) (*FilmProductionSubmitResult, error) {
	if err := validateFilmProductionSubmitCommand(command); err != nil {
		return nil, err
	}
	now := runtimeCommandTime(command.At)
	result := FilmProductionSubmitResult{}
	expired := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var replay model.FilmProductionAttempt
		replayErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&replay, "user_id = ? AND idempotency_key = ?", command.UserID, command.IdempotencyKey).Error
		if replayErr == nil {
			if replay.ProjectID != command.ProjectID || replay.QuoteID != command.QuoteID {
				return ErrFilmProductionStateConflict
			}
			return loadFilmProductionSubmitResultTx(tx, replay, true, &result)
		}
		if !errors.Is(replayErr, gorm.ErrRecordNotFound) {
			return replayErr
		}

		var quote model.FilmProductionQuote
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&quote, "id = ? AND user_id = ? AND project_id = ?", command.QuoteID, command.UserID, command.ProjectID).Error; err != nil {
			return err
		}
		if quote.QuoteFingerprint != command.QuoteFingerprint {
			return ErrFilmProductionQuoteConflict
		}
		if quote.Status == model.FilmProductionQuoteStatusConsumed {
			var existing model.FilmProductionAttempt
			if err := tx.First(&existing, "quote_id = ?", quote.ID).Error; err != nil {
				return ErrFilmProductionQuoteConsumed
			}
			if existing.IdempotencyKey != command.IdempotencyKey {
				return ErrFilmProductionQuoteConsumed
			}
			return loadFilmProductionSubmitResultTx(tx, existing, true, &result)
		}
		if quote.Status != model.FilmProductionQuoteStatusPending {
			return ErrFilmProductionQuoteConsumed
		}
		if !quote.ExpiresAt.After(now) {
			updated := tx.Model(&model.FilmProductionQuote{}).
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
		scope, err := validateFilmProductionQuoteScopeTx(tx, &quote, true)
		if err != nil {
			return err
		}
		if err := validateFilmProductionRetryTx(tx, quote); err != nil {
			return err
		}
		var active int64
		if err := tx.Model(&model.FilmProductionAttempt{}).
			Where("root_run_id = ? AND shot_id = ? AND status IN ?", quote.RootRunID, quote.ShotID, []model.FilmProductionAttemptStatus{
				model.FilmProductionAttemptStatusQueued,
				model.FilmProductionAttemptStatusRunning,
				model.FilmProductionAttemptStatusUncertain,
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
		if err := tx.Model(&model.FilmProductionAttempt{}).
			Where("root_run_id = ? AND shot_id = ?", quote.RootRunID, quote.ShotID).
			Select("COALESCE(MAX(number), 0)").Scan(&maxNumber).Error; err != nil {
			return err
		}
		task := *command.Task
		if err := validateFilmProductionTaskForQuote(task, quote, command.AttemptID); err != nil {
			return err
		}
		billing, err := filmProductionBillingFromQuote(quote)
		if err != nil {
			return err
		}
		if billing != nil {
			if err := validateFilmProductionBillingSnapshot(*billing, quote, scope); err != nil {
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

		attempt := filmProductionAttemptFromQuote(quote, command, maxNumber+1, task.BillingOrderID, now)
		artifact, revision, err := createFilmProductionAttemptArtifact(quote, attempt, command, now)
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
		consumed := tx.Model(&model.FilmProductionQuote{}).
			Where("id = ? AND status = ?", quote.ID, model.FilmProductionQuoteStatusPending).
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
		quote.UpdatedAt = now
		result = FilmProductionSubmitResult{Quote: quote, Attempt: attempt, Task: task, Billing: billing}
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

func (r *Repository) FilmProductionAttemptByTaskID(taskID string) (*model.FilmProductionAttempt, error) {
	var attempt model.FilmProductionAttempt
	if err := r.db.First(&attempt, "task_id = ?", taskID).Error; err != nil {
		return nil, err
	}
	return &attempt, nil
}

func (r *Repository) FilmProductionAttemptByIdempotency(userID string, key string) (*model.FilmProductionAttempt, error) {
	var attempt model.FilmProductionAttempt
	if err := r.db.First(&attempt, "user_id = ? AND idempotency_key = ?", userID, key).Error; err != nil {
		return nil, err
	}
	return &attempt, nil
}

func (r *Repository) FilmProductionAttemptForUser(userID string, projectID string, attemptID string) (*FilmProductionAttemptDetail, error) {
	var attempt model.FilmProductionAttempt
	if err := r.db.First(&attempt, "id = ? AND user_id = ? AND project_id = ?", attemptID, userID, projectID).Error; err != nil {
		return nil, err
	}
	details, err := r.hydrateFilmProductionAttempts([]model.FilmProductionAttempt{attempt})
	if err != nil {
		return nil, err
	}
	return &details[0], nil
}

func (r *Repository) FilmProductionAttemptsForProject(userID string, projectID string, rootRunID string, shotID string, limit int) ([]FilmProductionAttemptDetail, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var attempts []model.FilmProductionAttempt
	query := r.db.Where("user_id = ? AND project_id = ?", userID, projectID)
	if strings.TrimSpace(rootRunID) != "" {
		query = query.Where("root_run_id = ?", strings.TrimSpace(rootRunID))
	}
	if strings.TrimSpace(shotID) != "" {
		query = query.Where("shot_id = ?", strings.TrimSpace(shotID))
	}
	if err := query.Order("created_at desc").Limit(limit).Find(&attempts).Error; err != nil {
		return nil, err
	}
	return r.hydrateFilmProductionAttempts(attempts)
}

func (r *Repository) hydrateFilmProductionAttempts(attempts []model.FilmProductionAttempt) ([]FilmProductionAttemptDetail, error) {
	details := make([]FilmProductionAttemptDetail, len(attempts))
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
	var tasks []model.Task
	if err := r.db.Where("id IN ?", taskIDs).Find(&tasks).Error; err != nil {
		return nil, err
	}
	var results []model.Result
	if err := r.db.Where("attempt_id IN ? AND kind = ?", attemptIDs, model.ResultKindFilmGeneration).Find(&results).Error; err != nil {
		return nil, err
	}
	var reports []model.FilmProductionQCReport
	if err := r.db.Where("attempt_id IN ?", attemptIDs).Order("created_at asc").Find(&reports).Error; err != nil {
		return nil, err
	}
	visualQCAttempts, err := r.filmVisualQCAttemptsForSources(attemptIDs)
	if err != nil {
		return nil, err
	}
	var orders []model.BillingOrder
	if len(orderIDs) > 0 {
		if err := r.db.Where("id IN ?", orderIDs).Find(&orders).Error; err != nil {
			return nil, err
		}
	}
	taskByID := make(map[string]model.Task, len(tasks))
	resultByAttempt := make(map[string]model.Result, len(results))
	reportsByAttempt := make(map[string][]model.FilmProductionQCReport)
	orderByID := make(map[string]model.BillingOrder, len(orders))
	for _, task := range tasks {
		taskByID[task.ID] = task
	}
	for _, item := range results {
		resultByAttempt[item.AttemptID] = item
	}
	for _, report := range reports {
		reportsByAttempt[report.AttemptID] = append(reportsByAttempt[report.AttemptID], report)
	}
	for _, order := range orders {
		orderByID[order.ID] = order
	}
	for index := range details {
		details[index].Task = taskByID[details[index].Attempt.TaskID]
		if item, ok := resultByAttempt[details[index].Attempt.ID]; ok {
			result := item
			details[index].Result = &result
		}
		details[index].QCReports = reportsByAttempt[details[index].Attempt.ID]
		details[index].VisualQCAttempts = visualQCAttempts[details[index].Attempt.ID]
		if order, ok := orderByID[details[index].Attempt.BillingOrderID]; ok {
			billing := order
			details[index].Billing = &billing
		}
	}
	return details, nil
}
