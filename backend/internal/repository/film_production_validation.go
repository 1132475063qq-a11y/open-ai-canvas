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

func validateFilmProductionQuote(quote *model.FilmProductionQuote) error {
	if quote == nil || strings.TrimSpace(quote.ID) == "" || strings.TrimSpace(quote.UserID) == "" ||
		strings.TrimSpace(quote.IdempotencyKey) == "" || strings.TrimSpace(quote.ProjectID) == "" ||
		strings.TrimSpace(quote.RootRunID) == "" || strings.TrimSpace(quote.ShotID) == "" || strings.TrimSpace(quote.TaskID) == "" ||
		strings.TrimSpace(quote.StoryboardRevisionID) == "" || strings.TrimSpace(quote.PromptRevisionID) == "" ||
		strings.TrimSpace(quote.FeasibilityRevisionID) == "" || strings.TrimSpace(quote.LogicalModelID) == "" ||
		strings.TrimSpace(quote.LogicalModelRevisionID) == "" || strings.TrimSpace(quote.RouteID) == "" ||
		strings.TrimSpace(quote.ChannelID) == "" || strings.TrimSpace(quote.ChannelModelID) == "" ||
		strings.TrimSpace(quote.RequestJSON) == "" || strings.TrimSpace(quote.RequestFingerprint) == "" ||
		strings.TrimSpace(quote.QuoteFingerprint) == "" || quote.Capability != "image" || quote.ExpiresAt.IsZero() {
		return errors.New("Film Production quote identity is incomplete")
	}
	return nil
}

func validateFilmProductionSubmitCommand(command FilmProductionSubmitCommand) error {
	if strings.TrimSpace(command.UserID) == "" || strings.TrimSpace(command.ProjectID) == "" || strings.TrimSpace(command.QuoteID) == "" ||
		strings.TrimSpace(command.QuoteFingerprint) == "" || strings.TrimSpace(command.IdempotencyKey) == "" ||
		strings.TrimSpace(command.AttemptID) == "" || strings.TrimSpace(command.AttemptArtifactID) == "" ||
		strings.TrimSpace(command.AttemptRevisionID) == "" || command.Task == nil || command.ActiveTaskLimit < 1 {
		return errors.New("Film Production submit identity is incomplete")
	}
	return validateAgentRuntimeEventInput(command.Event)
}

func validateFilmProductionQuoteScopeTx(tx *gorm.DB, quote *model.FilmProductionQuote, requireCurrentArtifacts bool) (filmProductionQuoteScope, error) {
	scope := filmProductionQuoteScope{}
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
	if scope.RootRun.RootRunID != scope.RootRun.ID || scope.RootRun.RegistryID != quote.RegistryID ||
		scope.RootRun.RegistryVersion != quote.RegistryVersion || scope.RootRun.RegistryDigest != quote.RegistryDigest ||
		scope.RootRun.Status == model.AgentRunStatusFailed || scope.RootRun.Status == model.AgentRunStatusCancelled {
		return scope, ErrFilmProductionQuoteDrift
	}
	if err := tx.Clauses(lock).First(&scope.Shot, "id = ? AND project_id = ?", quote.ShotID, quote.ProjectID).Error; err != nil {
		return scope, err
	}
	expectations := []struct {
		artifactID   string
		revisionID   string
		digest       string
		allowedTypes []string
	}{
		{quote.StoryboardArtifactID, quote.StoryboardRevisionID, quote.StoryboardDigest, []string{"storyboard"}},
		{quote.PromptArtifactID, quote.PromptRevisionID, quote.PromptDigest, []string{"image-prompt-pack", "prompt-manifest"}},
		{quote.FeasibilityArtifactID, quote.FeasibilityRevisionID, quote.FeasibilityDigest, []string{"production-feasibility-report"}},
	}
	for _, expectation := range expectations {
		var revision model.ProductionArtifactRevision
		if err := tx.Clauses(lock).First(&revision, "id = ? AND artifact_id = ?", expectation.revisionID, expectation.artifactID).Error; err != nil {
			return scope, err
		}
		var artifact model.ProductionArtifact
		if err := tx.Clauses(lock).First(&artifact, "id = ? AND user_id = ? AND project_id = ? AND domain = ?", expectation.artifactID, quote.UserID, quote.ProjectID, "film").Error; err != nil {
			return scope, err
		}
		if revision.Status != model.ProductionArtifactStatusLocked || revision.ContentDigest != expectation.digest ||
			!filmProductionContains(expectation.allowedTypes, artifact.ArtifactType) ||
			(requireCurrentArtifacts && artifact.CurrentRevisionID != revision.ID) {
			return scope, ErrFilmProductionQuoteDrift
		}
	}
	if err := tx.Clauses(lock).First(&scope.LogicalModel, "id = ?", quote.LogicalModelID).Error; err != nil {
		return scope, err
	}
	if !scope.LogicalModel.Enabled || scope.LogicalModel.ArchivedAt != nil || scope.LogicalModel.ActiveRevisionID != quote.LogicalModelRevisionID || scope.LogicalModel.Code != quote.Model || scope.LogicalModel.Capability != "image" {
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
	if !scope.ChannelModel.Enabled || scope.ChannelModel.ModelKey != quote.ProviderModel || scope.ChannelModel.Capability != "image" ||
		scope.ChannelModel.Protocol != quote.Protocol || scope.ChannelModel.CapabilityVersion != quote.CapabilityVersion ||
		scope.ChannelModel.PriceVersion != quote.ChannelPriceVersion {
		return scope, ErrFilmProductionQuoteDrift
	}
	if err := tx.Clauses(lock).First(&scope.Channel, "id = ? AND scope = ? AND enabled = ?", quote.ChannelID, model.ChannelScopeSystem, true).Error; err != nil {
		return scope, err
	}
	if err := validateFilmProductionRequestSnapshot(quote); err != nil {
		return scope, err
	}
	if err := validateFilmProductionReferenceSnapshotsTx(tx, quote); err != nil {
		return scope, err
	}
	if billing, err := filmProductionBillingFromQuote(*quote); err != nil {
		return scope, err
	} else if billing != nil {
		if err := validateFilmProductionBillingSnapshot(*billing, *quote, scope); err != nil {
			return scope, err
		}
	}
	return scope, nil
}

func validateFilmProductionRequestSnapshot(quote *model.FilmProductionQuote) error {
	var input struct {
		Mode   string `json:"mode"`
		Prompt string `json:"prompt"`
		Config struct {
			ChannelID string `json:"channelId"`
			Model     string `json:"model"`
			Count     string `json:"count"`
			APIKey    string `json:"apiKey"`
			SecretKey string `json:"secretKey"`
		} `json:"config"`
		Metadata struct {
			RootRunID string `json:"filmRootRunId"`
			ShotID    string `json:"filmShotId"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal([]byte(quote.RequestJSON), &input); err != nil {
		return ErrFilmProductionQuoteDrift
	}
	if input.Mode != "image" || strings.TrimSpace(input.Prompt) == "" || input.Config.ChannelID != quote.ChannelID ||
		strings.TrimPrefix(input.Config.Model, "models/") != quote.ProviderModel || input.Config.Count != "1" ||
		input.Config.APIKey != "" || input.Config.SecretKey != "" || input.Metadata.RootRunID != quote.RootRunID || input.Metadata.ShotID != quote.ShotID {
		return ErrFilmProductionQuoteDrift
	}
	return nil
}

func validateFilmProductionReferenceSnapshotsTx(tx *gorm.DB, quote *model.FilmProductionQuote) error {
	var input struct {
		ReferenceImages []struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			DataURL    string `json:"dataUrl"`
			URL        string `json:"url"`
			StorageKey string `json:"storageKey"`
			MimeType   string `json:"mimeType"`
			Bytes      int64  `json:"bytes"`
			Width      int    `json:"width"`
			Height     int    `json:"height"`
		} `json:"referenceImages"`
	}
	if err := json.Unmarshal([]byte(quote.RequestJSON), &input); err != nil {
		return ErrFilmProductionQuoteDrift
	}
	seen := make(map[string]bool, len(input.ReferenceImages))
	for _, reference := range input.ReferenceImages {
		resourceID := strings.TrimPrefix(strings.TrimSpace(reference.StorageKey), "resource:")
		if resourceID == "" || reference.StorageKey != "resource:"+resourceID || reference.ID != resourceID || seen[resourceID] ||
			strings.TrimSpace(reference.DataURL) != "" || strings.TrimSpace(reference.URL) != "" {
			return ErrFilmProductionQuoteDrift
		}
		seen[resourceID] = true
		var resource model.Resource
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&resource, "id = ? AND user_id = ?", resourceID, quote.UserID).Error; err != nil {
			return ErrFilmProductionQuoteDrift
		}
		if resource.Status != model.ResourceStatusReady || resource.Kind != "image" || !strings.HasPrefix(resource.MimeType, "image/") ||
			reference.Type != resource.MimeType || reference.MimeType != resource.MimeType || reference.Bytes != resource.Size ||
			reference.Width != resource.Width || reference.Height != resource.Height {
			return ErrFilmProductionQuoteDrift
		}
	}
	return nil
}

func filmProductionBillingFromQuote(quote model.FilmProductionQuote) (*model.BillingOrder, error) {
	if strings.TrimSpace(quote.BillingJSON) == "" || strings.TrimSpace(quote.BillingJSON) == "null" {
		return nil, nil
	}
	var order model.BillingOrder
	if err := json.Unmarshal([]byte(quote.BillingJSON), &order); err != nil {
		return nil, ErrFilmProductionQuoteDrift
	}
	return &order, nil
}

func validateFilmProductionBillingSnapshot(order model.BillingOrder, quote model.FilmProductionQuote, scope filmProductionQuoteScope) error {
	if strings.TrimSpace(order.ID) == "" || strings.TrimSpace(order.IdempotencyKey) == "" ||
		order.UserID != quote.UserID || order.TaskID != quote.TaskID || order.ChannelID != quote.ChannelID ||
		order.ChannelModelID != quote.ChannelModelID || order.Model != quote.Model || order.Capability != "image" ||
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

func validateFilmProductionRetryTx(tx *gorm.DB, quote model.FilmProductionQuote) error {
	var attempts int64
	if err := tx.Model(&model.FilmProductionAttempt{}).
		Where("root_run_id = ? AND shot_id = ?", quote.RootRunID, quote.ShotID).Count(&attempts).Error; err != nil {
		return err
	}
	if quote.RetryOfAttemptID == "" {
		if attempts != 0 {
			return ErrFilmProductionRetryNotAllowed
		}
		return nil
	}
	var source model.FilmProductionAttempt
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&source,
		"id = ? AND user_id = ? AND project_id = ? AND root_run_id = ? AND shot_id = ?",
		quote.RetryOfAttemptID, quote.UserID, quote.ProjectID, quote.RootRunID, quote.ShotID,
	).Error; err != nil {
		return err
	}
	var latestNumber int
	if err := tx.Model(&model.FilmProductionAttempt{}).
		Where("root_run_id = ? AND shot_id = ?", quote.RootRunID, quote.ShotID).
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
		var report model.FilmProductionQCReport
		if err := tx.Where("attempt_id = ? AND source = ?", source.ID, "human").Order("created_at desc").First(&report).Error; err != nil {
			return ErrFilmProductionRetryNotAllowed
		}
		if report.Decision != model.FilmProductionQCDecisionFail || report.Action != model.FilmProductionQCActionRetry || source.PromptRevisionID == quote.PromptRevisionID {
			return ErrFilmProductionRetryNotAllowed
		}
		return nil
	default:
		return ErrFilmProductionRetryNotAllowed
	}
}

func validateFilmProductionTaskForQuote(task model.Task, quote model.FilmProductionQuote, attemptID string) error {
	if task.ID != quote.TaskID || task.UserID != quote.UserID || task.ProjectID != quote.ProjectID ||
		task.Type != model.FilmProductionTaskTypeImage || task.Operation != "image" || task.Status != model.TaskStatusQueued ||
		task.LogicalModelID != quote.LogicalModelID || task.LogicalModelRevisionID != quote.LogicalModelRevisionID ||
		task.RouteID != quote.RouteID || task.ChannelModelID != quote.ChannelModelID || task.RouteRun != 1 ||
		task.Model != quote.Model || task.Provider != "managed" || strings.TrimSpace(task.InputJSON) == "" {
		return ErrFilmProductionStateConflict
	}
	var input struct {
		Prompt   string `json:"prompt"`
		Metadata struct {
			QuoteID   string `json:"filmQuoteId"`
			AttemptID string `json:"filmAttemptId"`
			RootRunID string `json:"filmRootRunId"`
			ShotID    string `json:"filmShotId"`
		} `json:"metadata"`
	}
	if json.Unmarshal([]byte(task.InputJSON), &input) != nil || strings.TrimSpace(input.Prompt) == "" ||
		task.Prompt != input.Prompt || input.Metadata.QuoteID != quote.ID || input.Metadata.RootRunID != quote.RootRunID ||
		input.Metadata.ShotID != quote.ShotID || input.Metadata.AttemptID != attemptID {
		return ErrFilmProductionStateConflict
	}
	return nil
}

func filmProductionAttemptFromQuote(quote model.FilmProductionQuote, command FilmProductionSubmitCommand, number int, billingOrderID string, at time.Time) model.FilmProductionAttempt {
	return model.FilmProductionAttempt{
		ID: command.AttemptID, UserID: quote.UserID, ProjectID: quote.ProjectID, RootRunID: quote.RootRunID, ShotID: quote.ShotID,
		Number: number, RetryOfAttemptID: quote.RetryOfAttemptID, QuoteID: quote.ID, TaskID: quote.TaskID,
		BillingOrderID: billingOrderID, IdempotencyKey: command.IdempotencyKey,
		StoryboardArtifactID: quote.StoryboardArtifactID, StoryboardRevisionID: quote.StoryboardRevisionID, StoryboardDigest: quote.StoryboardDigest,
		PromptArtifactID: quote.PromptArtifactID, PromptRevisionID: quote.PromptRevisionID, PromptDigest: quote.PromptDigest,
		FeasibilityArtifactID: quote.FeasibilityArtifactID, FeasibilityRevisionID: quote.FeasibilityRevisionID, FeasibilityDigest: quote.FeasibilityDigest,
		RegistryID: quote.RegistryID, RegistryVersion: quote.RegistryVersion, RegistryDigest: quote.RegistryDigest,
		LogicalModelID: quote.LogicalModelID, LogicalModelRevisionID: quote.LogicalModelRevisionID,
		RouteID: quote.RouteID, ChannelID: quote.ChannelID, ChannelModelID: quote.ChannelModelID,
		Model: quote.Model, ProviderModel: quote.ProviderModel, Capability: quote.Capability, Protocol: quote.Protocol,
		CapabilityVersion: quote.CapabilityVersion, ChannelPriceVersion: quote.ChannelPriceVersion,
		RequestFingerprint: quote.RequestFingerprint, Status: model.FilmProductionAttemptStatusQueued,
		CreatedAt: at, UpdatedAt: at,
	}
}

func createFilmProductionAttemptArtifact(quote model.FilmProductionQuote, attempt model.FilmProductionAttempt, command FilmProductionSubmitCommand, at time.Time) (model.ProductionArtifact, model.ProductionArtifactRevision, error) {
	content, err := json.Marshal(map[string]any{
		"schemaVersion": 2, "artifactType": "generation-attempt", "attemptId": attempt.ID,
		"shotId": attempt.ShotID, "attemptNumber": attempt.Number, "retryOf": emptyStringAsNil(attempt.RetryOfAttemptID),
		"taskId": attempt.TaskID, "executionState": "SUBMITTED", "requestFingerprint": attempt.RequestFingerprint,
		"logicalModel": map[string]any{"id": attempt.LogicalModelID, "revisionId": attempt.LogicalModelRevisionID, "code": attempt.Model},
		"promptRef":    map[string]any{"artifactId": attempt.PromptArtifactID, "revisionId": attempt.PromptRevisionID, "digest": attempt.PromptDigest},
		"resultId":     nil, "mediaRef": nil, "resultState": "UNKNOWN", "qcVerdict": "NOT_ASSESSABLE",
	})
	if err != nil {
		return model.ProductionArtifact{}, model.ProductionArtifactRevision{}, err
	}
	sourceRefs := filmProductionQuoteArtifactRefs(quote)
	authority := []map[string]any{
		{"kind": "registry", "id": quote.RegistryID, "version": quote.RegistryVersion, "digest": quote.RegistryDigest},
		{"kind": "runtime", "id": "film-production-orchestrator-v1"},
	}
	artifact := model.ProductionArtifact{
		ID: command.AttemptArtifactID, UserID: quote.UserID, ProjectID: quote.ProjectID, Domain: "film",
		ArtifactType: "generation-attempt", LogicalKey: "film-production:attempt:" + attempt.ID,
	}
	revision := model.ProductionArtifactRevision{
		ID: command.AttemptRevisionID, Status: model.ProductionArtifactStatusLocked, ContentJSON: string(content), ContentDigest: filmProductionDigest(content),
		SourceRunID: quote.RootRunID, SourceAttemptID: attempt.ID, SourceArtifactRefsJSON: mustRepositoryJSON(sourceRefs), AuthorityRefsJSON: mustRepositoryJSON(authority),
		CreatedByType: "runtime", CreatedByID: "film-production-orchestrator-v1", CreatedAt: at,
	}
	return artifact, revision, nil
}

func filmProductionQuoteArtifactRefs(quote model.FilmProductionQuote) []map[string]any {
	return []map[string]any{
		{"artifactId": quote.StoryboardArtifactID, "revisionId": quote.StoryboardRevisionID, "type": "storyboard", "digest": quote.StoryboardDigest},
		{"artifactId": quote.PromptArtifactID, "revisionId": quote.PromptRevisionID, "type": "prompt", "digest": quote.PromptDigest},
		{"artifactId": quote.FeasibilityArtifactID, "revisionId": quote.FeasibilityRevisionID, "type": "production-feasibility-report", "digest": quote.FeasibilityDigest},
	}
}

func loadFilmProductionSubmitResultTx(tx *gorm.DB, attempt model.FilmProductionAttempt, idempotent bool, result *FilmProductionSubmitResult) error {
	var quote model.FilmProductionQuote
	if err := tx.First(&quote, "id = ?", attempt.QuoteID).Error; err != nil {
		return err
	}
	var task model.Task
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
	*result = FilmProductionSubmitResult{Quote: quote, Attempt: attempt, Task: task, Billing: billing, Idempotent: idempotent}
	return nil
}

func appendFilmProductionEventTx(tx *gorm.DB, root model.AgentRuntimeRun, event AgentRuntimeEventInput, attemptID string, at time.Time) error {
	if err := validateAgentRuntimeEventInput(event); err != nil {
		return err
	}
	updated := tx.Model(&model.AgentRuntimeRun{}).
		Where("id = ? AND user_id = ? AND revision = ? AND event_sequence = ?", root.ID, root.UserID, root.Revision, root.EventSequence).
		Updates(map[string]any{"revision": gorm.Expr("revision + ?", 1), "event_sequence": gorm.Expr("event_sequence + ?", 1), "updated_at": at})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return ErrFilmProductionStateConflict
	}
	event.AttemptID = attemptID
	return appendAgentRuntimeEvent(tx, root, event, root.EventSequence+1, "", string(root.Status), string(root.Status), at)
}

func filmProductionAccessibleMediaURL(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "/api/resources/") || strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://")
}

func filmProductionContains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func filmProductionDigest(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func emptyStringAsNil(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
