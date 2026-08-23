package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrEcommerceRunStateConflict = errors.New("ecommerce run state conflict")
	ErrEcommerceQuoteChanged     = errors.New("ecommerce quote changed")
	ErrEcommerceQuoteExpired     = errors.New("ecommerce quote expired")
	ErrEcommerceAttemptConflict  = errors.New("ecommerce attempt conflict")
	ErrEcommerceQAReviewConflict = errors.New("ecommerce QA review conflict")
)

type EcommerceRunAtomicCreateInput struct {
	Run       *model.EcommerceProductionRun
	Slots     []model.EcommerceProductionSlot
	Attempts  []model.EcommerceProductionAttempt
	Artifacts []model.EcommerceArtifact
}

type EcommerceTaskBinding struct {
	SlotID    string
	AttemptID string
	Task      *model.Task
	Order     *model.BillingOrder
}

type EcommerceRunAtomicSubmitInput struct {
	UserID           string
	RunID            string
	QuoteFingerprint string
	ChannelModel     model.ChannelModel
	Bindings         []EcommerceTaskBinding
	ActiveTaskLimit  int
}

type EcommerceRetryAtomicSubmitInput struct {
	UserID           string
	RunID            string
	SlotID           string
	AttemptID        string
	QuoteFingerprint string
	ChannelModel     model.ChannelModel
	Task             *model.Task
	Order            *model.BillingOrder
	ActiveTaskLimit  int
}

func (r *Repository) ProjectEcommercePresetVersions(projectID string) ([]model.EcommercePresetVersion, error) {
	var rows []model.EcommercePresetVersion
	err := r.db.Where("project_id = ?", projectID).Order("preset_key asc, version desc").Find(&rows).Error
	return rows, err
}

func (r *Repository) LatestEcommercePresetVersion(projectID string, presetKey string) (*model.EcommercePresetVersion, error) {
	var row model.EcommercePresetVersion
	err := r.db.Where("project_id = ? AND (preset_key = ? OR id = ?)", projectID, presetKey, presetKey).Order("version desc").First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *Repository) SaveEcommercePresetVersion(row *model.EcommercePresetVersion) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var project model.Project
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&project, "id = ?", row.ProjectID).Error; err != nil {
			return err
		}
		var latest int
		if err := tx.Model(&model.EcommercePresetVersion{}).Where("project_id = ? AND preset_key = ?", row.ProjectID, row.PresetKey).Select("COALESCE(MAX(version), 0)").Scan(&latest).Error; err != nil {
			return err
		}
		row.Version = latest + 1
		if err := tx.Create(row).Error; err != nil {
			return err
		}
		return tx.Model(&model.Project{}).Where("id = ?", row.ProjectID).Updates(map[string]any{"revision": gorm.Expr("revision + 1"), "updated_at": time.Now()}).Error
	})
}

func (r *Repository) EcommerceProductionRunByIdempotency(userID string, projectID string, key string) (*model.EcommerceProductionRun, error) {
	var run model.EcommerceProductionRun
	err := r.db.First(&run, "user_id = ? AND project_id = ? AND idempotency_key = ?", userID, projectID, key).Error
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *Repository) EcommerceProductionRunForUser(userID string, projectID string, runID string) (*model.EcommerceProductionRun, error) {
	var run model.EcommerceProductionRun
	err := r.db.First(&run, "id = ? AND user_id = ? AND project_id = ?", runID, userID, projectID).Error
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *Repository) ProjectEcommerceProductionRuns(userID string, projectID string, limit int) ([]model.EcommerceProductionRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var runs []model.EcommerceProductionRun
	err := r.db.Where("user_id = ? AND project_id = ?", userID, projectID).Order("created_at desc").Limit(limit).Find(&runs).Error
	return runs, err
}

func (r *Repository) EcommerceProductionSlots(runID string) ([]model.EcommerceProductionSlot, error) {
	var slots []model.EcommerceProductionSlot
	err := r.db.Where("run_id = ?", runID).Order("position asc").Find(&slots).Error
	return slots, err
}

func (r *Repository) EcommerceProductionSlotForUser(userID string, runID string, slotID string) (*model.EcommerceProductionSlot, error) {
	var slot model.EcommerceProductionSlot
	err := r.db.First(&slot, "id = ? AND run_id = ? AND user_id = ?", slotID, runID, userID).Error
	if err != nil {
		return nil, err
	}
	return &slot, nil
}

func (r *Repository) EcommerceProductionAttempts(runID string) ([]model.EcommerceProductionAttempt, error) {
	var attempts []model.EcommerceProductionAttempt
	err := r.db.Where("run_id = ?", runID).Order("slot_id asc, attempt_number asc").Find(&attempts).Error
	return attempts, err
}

func (r *Repository) EcommerceProductionAttemptForTask(taskID string) (*model.EcommerceProductionAttempt, error) {
	var attempt model.EcommerceProductionAttempt
	err := r.db.First(&attempt, "task_id = ?", taskID).Error
	if err != nil {
		return nil, err
	}
	return &attempt, nil
}

func (r *Repository) EcommerceProductionAttemptForUser(userID string, runID string, slotID string, attemptID string) (*model.EcommerceProductionAttempt, error) {
	var attempt model.EcommerceProductionAttempt
	err := r.db.First(&attempt, "id = ? AND run_id = ? AND slot_id = ? AND user_id = ?", attemptID, runID, slotID, userID).Error
	if err != nil {
		return nil, err
	}
	return &attempt, nil
}

func (r *Repository) ActiveEcommerceRunTaskIDs(userID string, projectID string, runID string) ([]string, error) {
	var ids []string
	err := r.db.Model(&model.Task{}).
		Select("tasks.id").
		Joins("JOIN ecommerce_production_attempts ON ecommerce_production_attempts.task_id = tasks.id").
		Where("ecommerce_production_attempts.user_id = ? AND ecommerce_production_attempts.project_id = ? AND ecommerce_production_attempts.run_id = ?", userID, projectID, runID).
		Where("tasks.status IN ?", []model.TaskStatus{model.TaskStatusScheduled, model.TaskStatusQueued, model.TaskStatusRunning}).
		Order("tasks.created_at asc").
		Pluck("tasks.id", &ids).Error
	return ids, err
}

func (r *Repository) FinalizeEcommerceRunCancellation(userID string, projectID string, runID string, now time.Time) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var run model.EcommerceProductionRun
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&run, "id = ? AND user_id = ? AND project_id = ?", runID, userID, projectID).Error; err != nil {
			return err
		}
		if run.Status == "cancelled" {
			return nil
		}
		if run.Status == "ready" {
			return ErrEcommerceRunStateConflict
		}
		var active int64
		if err := tx.Model(&model.Task{}).
			Joins("JOIN ecommerce_production_attempts ON ecommerce_production_attempts.task_id = tasks.id").
			Where("ecommerce_production_attempts.run_id = ? AND tasks.status IN ?", runID, []model.TaskStatus{model.TaskStatusScheduled, model.TaskStatusQueued, model.TaskStatusRunning}).
			Count(&active).Error; err != nil {
			return err
		}
		if active > 0 {
			return ErrEcommerceRunStateConflict
		}
		if err := tx.Model(&model.EcommerceProductionAttempt{}).
			Where("run_id = ? AND status IN ?", runID, []string{"planned", "awaiting_cost", "scheduled", "queued", "running"}).
			Updates(map[string]any{"status": "cancelled", "completed_at": &now, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.EcommerceProductionSlot{}).
			Where("run_id = ? AND accepted = ? AND status IN ?", runID, false, []string{"planned", "scheduled", "queued", "running", "failed"}).
			Updates(map[string]any{"status": "cancelled", "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&model.EcommerceProductionRun{}).Where("id = ?", runID).Updates(map[string]any{
			"status": "cancelled", "error": "", "completed_at": &now, "updated_at": now,
		}).Error
	})
}

func (r *Repository) ApplyEcommerceRunQuote(run *model.EcommerceProductionRun, attempts []model.EcommerceProductionAttempt) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var current model.EcommerceProductionRun
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, "id = ? AND user_id = ? AND project_id = ?", run.ID, run.UserID, run.ProjectID).Error; err != nil {
			return err
		}
		if current.SubmittedAt != nil || (current.Status != "needs_you" && current.Status != "awaiting_cost" && current.Status != "awaiting_review") {
			return ErrEcommerceRunStateConflict
		}
		status := "awaiting_cost"
		if current.ReviewBeforeGeneration {
			status = "awaiting_review"
		}
		if err := tx.Model(&model.EcommerceProductionRun{}).Where("id = ?", current.ID).Updates(map[string]any{
			"status": status, "quote_fingerprint": run.QuoteFingerprint, "quote_expires_at": run.QuoteExpiresAt,
			"quote_channel_id": run.QuoteChannelID, "quote_channel_model_id": run.QuoteChannelModelID,
			"quote_model": run.QuoteModel, "quote_price_version": run.QuotePriceVersion,
			"quote_unit_microcredits": run.QuoteUnitMicrocredits, "quote_total_microcredits": run.QuoteTotalMicrocredits,
			"error": "", "updated_at": run.UpdatedAt,
		}).Error; err != nil {
			return err
		}
		for _, attempt := range attempts {
			if err := tx.Model(&model.EcommerceProductionAttempt{}).
				Where("id = ? AND run_id = ? AND task_id = ''", attempt.ID, current.ID).
				Updates(map[string]any{
					"status": "awaiting_cost", "quote_fingerprint": attempt.QuoteFingerprint, "quote_expires_at": attempt.QuoteExpiresAt,
					"quote_channel_id": attempt.QuoteChannelID, "quote_channel_model_id": attempt.QuoteChannelModelID,
					"quote_model": attempt.QuoteModel, "quote_price_version": attempt.QuotePriceVersion,
					"quote_amount_microcredits": attempt.QuoteAmountMicrocredits, "updated_at": run.UpdatedAt,
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) ApproveEcommerceRunPlan(userID string, projectID string, runID string, now time.Time) error {
	result := r.db.Model(&model.EcommerceProductionRun{}).
		Where("id = ? AND user_id = ? AND project_id = ? AND status = ?", runID, userID, projectID, "awaiting_review").
		Updates(map[string]any{"status": "awaiting_cost", "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrEcommerceRunStateConflict
	}
	return nil
}

func (r *Repository) CreateEcommerceProductionRunAtomic(input EcommerceRunAtomicCreateInput) error {
	if input.Run == nil || len(input.Slots) == 0 || len(input.Attempts) != len(input.Slots) {
		return gorm.ErrInvalidData
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		var project model.Project
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&project, "id = ? AND user_id = ?", input.Run.ProjectID, input.Run.UserID).Error; err != nil {
			return err
		}
		if project.Type != model.ProjectTypeEcommerce || project.Status != model.ProjectStatusActive {
			return ErrEcommerceRunStateConflict
		}
		var existing int64
		if err := tx.Model(&model.EcommerceProductionRun{}).Where("user_id = ? AND project_id = ? AND idempotency_key = ?", input.Run.UserID, input.Run.ProjectID, input.Run.IdempotencyKey).Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return ErrEcommerceRunStateConflict
		}
		for index := range input.Artifacts {
			if err := appendEcommerceArtifactVersion(tx, &input.Artifacts[index]); err != nil {
				return err
			}
			switch input.Artifacts[index].ArtifactType {
			case "product_dna":
				input.Run.ProductDNAArtifactID = input.Artifacts[index].ID
			case "model_profile":
				input.Run.ModelProfileArtifactID = input.Artifacts[index].ID
			case "scene_pack":
				input.Run.ScenePackArtifactID = input.Artifacts[index].ID
			case "preset_snapshot":
				input.Run.PresetSnapshotArtifactID = input.Artifacts[index].ID
			case "generation_request":
				input.Run.GenerationRequestArtifactID = input.Artifacts[index].ID
			}
		}
		if err := tx.Create(input.Run).Error; err != nil {
			return err
		}
		if err := tx.Create(&input.Slots).Error; err != nil {
			return err
		}
		if err := tx.Create(&input.Attempts).Error; err != nil {
			return err
		}
		return tx.Model(&model.Project{}).Where("id = ?", input.Run.ProjectID).Updates(map[string]any{"revision": gorm.Expr("revision + 1"), "updated_at": input.Run.UpdatedAt}).Error
	})
}

func appendEcommerceArtifactVersion(tx *gorm.DB, artifact *model.EcommerceArtifact) error {
	var latest int
	if err := tx.Model(&model.EcommerceArtifact{}).
		Where("project_id = ? AND artifact_key = ? AND artifact_type = ?", artifact.ProjectID, artifact.ArtifactKey, artifact.ArtifactType).
		Select("COALESCE(MAX(revision), 0)").Scan(&latest).Error; err != nil {
		return err
	}
	artifact.Revision = latest + 1
	return tx.Create(artifact).Error
}

func (r *Repository) SubmitEcommerceProductionRunAtomic(input EcommerceRunAtomicSubmitInput) error {
	if len(input.Bindings) == 0 || input.ActiveTaskLimit < 1 {
		return gorm.ErrInvalidData
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		var run model.EcommerceProductionRun
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&run, "id = ? AND user_id = ?", input.RunID, input.UserID).Error; err != nil {
			return err
		}
		if run.Status != "awaiting_cost" {
			return ErrEcommerceRunStateConflict
		}
		if run.QuoteFingerprint == "" || run.QuoteFingerprint != input.QuoteFingerprint {
			return ErrEcommerceQuoteChanged
		}
		if run.QuoteExpiresAt == nil || !run.QuoteExpiresAt.After(time.Now()) {
			return ErrEcommerceQuoteExpired
		}
		if run.QuoteChannelModelID != input.ChannelModel.ID || run.QuoteModel != input.ChannelModel.ModelKey || run.QuotePriceVersion != input.ChannelModel.PriceVersion {
			return ErrEcommerceQuoteChanged
		}
		var active int64
		if err := tx.Model(&model.Task{}).Where("user_id = ? AND status IN ?", input.UserID, []model.TaskStatus{model.TaskStatusQueued, model.TaskStatusRunning}).Count(&active).Error; err != nil {
			return err
		}
		available := input.ActiveTaskLimit - int(active)
		now := time.Now()
		for _, binding := range input.Bindings {
			if binding.Task == nil {
				return gorm.ErrInvalidData
			}
			status := model.TaskStatusScheduled
			if available > 0 {
				status = model.TaskStatusQueued
				available--
			}
			binding.Task.Status = status
			binding.Task.Stage = map[bool]string{true: "等待队列调度", false: "系列任务已锁价，等待调度"}[status == model.TaskStatusQueued]
			if binding.Order != nil {
				if err := reserveBillingOrder(tx, binding.Order); err != nil {
					return err
				}
				binding.Task.BillingOrderID = binding.Order.ID
			}
			if err := tx.Create(binding.Task).Error; err != nil {
				return err
			}
			attemptUpdates := map[string]any{
				"status": string(status), "task_id": binding.Task.ID, "billing_order_id": binding.Task.BillingOrderID,
				"updated_at": now,
			}
			updated := tx.Model(&model.EcommerceProductionAttempt{}).
				Where("id = ? AND run_id = ? AND slot_id = ? AND status = ?", binding.AttemptID, run.ID, binding.SlotID, "awaiting_cost").Updates(attemptUpdates)
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return ErrEcommerceAttemptConflict
			}
			slotStatus := "scheduled"
			if status == model.TaskStatusQueued {
				slotStatus = "queued"
			}
			if err := tx.Model(&model.EcommerceProductionSlot{}).Where("id = ? AND run_id = ?", binding.SlotID, run.ID).Updates(map[string]any{
				"status": slotStatus, "active_attempt_id": binding.AttemptID, "active_task_id": binding.Task.ID,
				"accepted": false, "accepted_attempt_id": "", "accepted_at": nil, "qa_status": "PENDING", "qa_issues_json": "[]", "qa_note": "", "updated_at": now,
			}).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.EcommerceProductionRun{}).Where("id = ?", run.ID).Updates(map[string]any{"status": "generating", "submitted_at": &now, "updated_at": now}).Error
	})
}

func (r *Repository) CreateEcommerceRetryQuoteAttempt(attempt *model.EcommerceProductionAttempt) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var slot model.EcommerceProductionSlot
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&slot, "id = ? AND run_id = ? AND user_id = ?", attempt.SlotID, attempt.RunID, attempt.UserID).Error; err != nil {
			return err
		}
		if slot.Status == "queued" || slot.Status == "running" || slot.Status == "scheduled" {
			return ErrEcommerceRunStateConflict
		}
		var latest int
		if err := tx.Model(&model.EcommerceProductionAttempt{}).Where("slot_id = ?", slot.ID).Select("COALESCE(MAX(attempt_number), 0)").Scan(&latest).Error; err != nil {
			return err
		}
		attempt.AttemptNumber = latest + 1
		return tx.Create(attempt).Error
	})
}

func (r *Repository) SubmitEcommerceRetryAtomic(input EcommerceRetryAtomicSubmitInput) error {
	if input.Task == nil || input.ActiveTaskLimit < 1 {
		return gorm.ErrInvalidData
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		var attempt model.EcommerceProductionAttempt
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&attempt, "id = ? AND run_id = ? AND slot_id = ? AND user_id = ?", input.AttemptID, input.RunID, input.SlotID, input.UserID).Error; err != nil {
			return err
		}
		if attempt.Status != "awaiting_cost" || attempt.QuoteFingerprint != input.QuoteFingerprint {
			return ErrEcommerceQuoteChanged
		}
		if attempt.QuoteExpiresAt == nil || !attempt.QuoteExpiresAt.After(time.Now()) {
			return ErrEcommerceQuoteExpired
		}
		if attempt.QuoteChannelModelID != input.ChannelModel.ID || attempt.QuoteModel != input.ChannelModel.ModelKey || attempt.QuotePriceVersion != input.ChannelModel.PriceVersion {
			return ErrEcommerceQuoteChanged
		}
		var active int64
		if err := tx.Model(&model.Task{}).Where("user_id = ? AND status IN ?", input.UserID, []model.TaskStatus{model.TaskStatusQueued, model.TaskStatusRunning}).Count(&active).Error; err != nil {
			return err
		}
		status := model.TaskStatusQueued
		if active >= int64(input.ActiveTaskLimit) {
			status = model.TaskStatusScheduled
		}
		now := time.Now()
		input.Task.Status = status
		input.Task.Stage = map[bool]string{true: "等待队列调度", false: "重试已锁价，等待调度"}[status == model.TaskStatusQueued]
		if input.Order != nil {
			if err := reserveBillingOrder(tx, input.Order); err != nil {
				return err
			}
			input.Task.BillingOrderID = input.Order.ID
		}
		if err := tx.Create(input.Task).Error; err != nil {
			return err
		}
		updated := tx.Model(&model.EcommerceProductionAttempt{}).Where("id = ? AND status = ?", attempt.ID, "awaiting_cost").Updates(map[string]any{
			"status": string(status), "task_id": input.Task.ID, "billing_order_id": input.Task.BillingOrderID, "updated_at": now,
		})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrEcommerceAttemptConflict
		}
		slotStatus := "queued"
		if status == model.TaskStatusScheduled {
			slotStatus = "scheduled"
		}
		if err := tx.Model(&model.EcommerceProductionSlot{}).Where("id = ? AND run_id = ?", input.SlotID, input.RunID).Updates(map[string]any{
			"status": slotStatus, "active_attempt_id": attempt.ID, "active_task_id": input.Task.ID,
			"accepted": false, "accepted_attempt_id": "", "accepted_at": nil, "qa_status": "PENDING", "qa_issues_json": "[]", "qa_note": "", "regeneration_reason": attempt.Kind, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&model.EcommerceProductionRun{}).Where("id = ? AND user_id = ?", input.RunID, input.UserID).Updates(map[string]any{"status": "generating", "completed_at": nil, "updated_at": now}).Error
	})
}

func (r *Repository) PromoteScheduledEcommerceTasks(activeTaskLimit int) error {
	if activeTaskLimit < 1 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		var tasks []model.Task
		if err := tx.Where("provider = ? AND status = ?", model.TaskProviderEcommerce, model.TaskStatusScheduled).Order("created_at asc").Limit(100).Find(&tasks).Error; err != nil {
			return err
		}
		activeByUser := map[string]int64{}
		for _, task := range tasks {
			active, exists := activeByUser[task.UserID]
			if !exists {
				if err := tx.Model(&model.Task{}).Where("user_id = ? AND status IN ?", task.UserID, []model.TaskStatus{model.TaskStatusQueued, model.TaskStatusRunning}).Count(&active).Error; err != nil {
					return err
				}
			}
			if active >= int64(activeTaskLimit) {
				activeByUser[task.UserID] = active
				continue
			}
			var attempt model.EcommerceProductionAttempt
			if err := tx.First(&attempt, "task_id = ?", task.ID).Error; err != nil {
				return err
			}
			var run model.EcommerceProductionRun
			if err := tx.First(&run, "id = ?", attempt.RunID).Error; err != nil {
				return err
			}
			if run.ModelMode == "ai" {
				var slot model.EcommerceProductionSlot
				if err := tx.First(&slot, "id = ?", attempt.SlotID).Error; err != nil {
					return err
				}
				if slot.Position > 1 {
					var seed model.EcommerceProductionSlot
					if err := tx.First(&seed, "run_id = ? AND position = 1", run.ID).Error; err != nil {
						return err
					}
					if strings.TrimSpace(seed.ResultPayloadJSON) == "" {
						activeByUser[task.UserID] = active
						continue
					}
				}
			}
			now := time.Now()
			updated := tx.Model(&model.Task{}).Where("id = ? AND status = ?", task.ID, model.TaskStatusScheduled).Updates(map[string]any{"status": model.TaskStatusQueued, "stage": "等待队列调度", "updated_at": now})
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected == 0 {
				continue
			}
			if err := tx.Model(&model.EcommerceProductionAttempt{}).Where("id = ?", attempt.ID).Updates(map[string]any{"status": "queued", "updated_at": now}).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.EcommerceProductionSlot{}).Where("id = ?", attempt.SlotID).Updates(map[string]any{"status": "queued", "updated_at": now}).Error; err != nil {
				return err
			}
			activeByUser[task.UserID] = active + 1
		}
		return nil
	})
}

func claimEcommerceProductionAttempt(tx *gorm.DB, task *model.Task, now time.Time) error {
	if task == nil || task.Provider != model.TaskProviderEcommerce {
		return nil
	}
	var attempt model.EcommerceProductionAttempt
	if err := tx.First(&attempt, "task_id = ?", task.ID).Error; err != nil {
		return err
	}
	updates := map[string]any{"status": "running", "started_at": gorm.Expr("COALESCE(started_at, ?)", now), "updated_at": now}
	if err := tx.Model(&model.EcommerceProductionAttempt{}).Where("id = ? AND status IN ?", attempt.ID, []string{"queued", "running"}).Updates(updates).Error; err != nil {
		return err
	}
	return tx.Model(&model.EcommerceProductionSlot{}).Where("id = ?", attempt.SlotID).Updates(map[string]any{"status": "running", "updated_at": now}).Error
}

func completeEcommerceProductionTask(tx *gorm.DB, task *model.Task, results []model.Result, completedAt time.Time) error {
	if task == nil || task.Provider != model.TaskProviderEcommerce {
		return nil
	}
	var attempt model.EcommerceProductionAttempt
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&attempt, "task_id = ?", task.ID).Error; err != nil {
		return err
	}
	var generated *model.Result
	for index := range results {
		if results[index].TaskID == task.ID && results[index].Kind == model.ResultKindEcommerceAsset {
			generated = &results[index]
			break
		}
	}
	if generated == nil {
		return gorm.ErrInvalidData
	}
	var run model.EcommerceProductionRun
	if err := tx.First(&run, "id = ?", attempt.RunID).Error; err != nil {
		return err
	}
	qaStatus := "UNCERTAIN"
	qaIssues := []string{"visual_review_required"}
	qaNote := ""
	resolutionCheck := map[string]any{"requestedResolution": run.Resolution, "requestedPixelSize": run.PixelSize, "status": "not_requested"}
	if strings.TrimSpace(run.PixelSize) != "" {
		expectedWidth, expectedHeight, expectedOK := ecommercePixelDimensions(run.PixelSize)
		actualWidth, actualHeight, actualOK := ecommerceResultDimensions(generated.Payload)
		resolutionCheck["actualWidth"], resolutionCheck["actualHeight"] = actualWidth, actualHeight
		switch {
		case !expectedOK || !actualOK:
			qaIssues = append(qaIssues, "resolution_unverified")
			qaNote = fmt.Sprintf("请求 %s %s；结果未提供可核验的像素尺寸", strings.ToUpper(run.Resolution), run.PixelSize)
			resolutionCheck["status"] = "unverified"
		case !ecommerceDimensionsMeetTarget(actualWidth, actualHeight, expectedWidth, expectedHeight):
			qaStatus = "FAIL"
			qaIssues = append(qaIssues, "resolution_mismatch")
			qaNote = fmt.Sprintf("要求至少 %s 且保持同画幅，实际 %dx%d", run.PixelSize, actualWidth, actualHeight)
			resolutionCheck["status"] = "mismatch"
		default:
			resolutionCheck["status"] = "verified"
		}
	}
	payload := map[string]any{
		"schemaVersion": 1, "runId": attempt.RunID, "slotId": attempt.SlotID,
		"attemptId": attempt.ID, "attemptNumber": attempt.AttemptNumber, "taskId": task.ID,
		"resultId": generated.ID, "url": generated.URL, "result": json.RawMessage(generated.Payload),
		"resolutionCheck": resolutionCheck,
	}
	if hash, algorithm, ok := ecommerceResultCompositionFingerprint(generated.Payload); ok {
		payload["compositionCheck"] = map[string]any{"algorithm": algorithm, "hash": hash, "status": "series_comparison_pending"}
	}
	resultURL := strings.TrimSpace(generated.URL)
	if resultURL == "" {
		resultURL = ecommerceResultURLFromPayload(generated.Payload)
	}
	payload["url"] = resultURL
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	refs, _ := json.Marshal([]string{"run:" + attempt.RunID, "slot:" + attempt.SlotID, "task:" + task.ID})
	artifact := model.EcommerceArtifact{
		ID: newRepositoryID(), ProjectID: attempt.ProjectID, ArtifactKey: "run:" + attempt.RunID + ":slot:" + attempt.SlotID + ":generated",
		ArtifactType: "generated_asset", SchemaVersion: 1, Lifecycle: "finalized", Evidence: "recorded",
		ResponsibleAgentID: "ecommerce_orchestrator", PayloadJSON: string(encoded), SourceRefsJSON: string(refs), AuthorityRefsJSON: "[]",
		CreatedAt: completedAt, UpdatedAt: completedAt,
	}
	if err := appendEcommerceArtifactVersion(tx, &artifact); err != nil {
		return err
	}
	if err := tx.Model(&model.EcommerceProductionAttempt{}).Where("id = ?", attempt.ID).Updates(map[string]any{
		"status": "succeeded", "provider_request_id": task.ProviderRequestID, "result_id": generated.ID,
		"generated_asset_artifact_id": artifact.ID, "result_url": resultURL, "result_payload_json": generated.Payload,
		"error": "", "completed_at": &completedAt, "updated_at": completedAt,
	}).Error; err != nil {
		return err
	}
	issuesJSON, err := json.Marshal(qaIssues)
	if err != nil {
		return err
	}
	if err := tx.Model(&model.EcommerceProductionSlot{}).Where("id = ?", attempt.SlotID).Updates(map[string]any{
		"status": "qa", "qa_status": qaStatus, "qa_issues_json": string(issuesJSON), "qa_note": qaNote,
		"result_url": resultURL, "result_payload_json": generated.Payload, "generated_asset_id": artifact.ID, "updated_at": completedAt,
		"composition_hash": "", "composition_hash_alg": "", "duplicate_of_slot_id": "", "composition_distance": 0,
	}).Error; err != nil {
		return err
	}
	if err := reconcileEcommerceCompositionQA(tx, attempt.RunID, completedAt); err != nil {
		return err
	}
	return refreshEcommerceProductionRunStatus(tx, attempt.RunID, completedAt)
}

func ecommerceResultURLFromPayload(payloadJSON string) string {
	var payload map[string]any
	if json.Unmarshal([]byte(payloadJSON), &payload) != nil {
		return ""
	}
	images, _ := payload["images"].([]any)
	if len(images) == 0 {
		return ""
	}
	image, _ := images[0].(map[string]any)
	if image == nil {
		return ""
	}
	for _, key := range []string{"url", "dataUrl", "content", "coverUrl"} {
		if value, ok := image[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func ecommercePixelDimensions(value string) (int, int, bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(strings.ReplaceAll(value, "×", "x"))), "x")
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, widthErr := strconv.Atoi(parts[0])
	height, heightErr := strconv.Atoi(parts[1])
	return width, height, widthErr == nil && heightErr == nil && width > 0 && height > 0
}

func ecommerceResultDimensions(payloadJSON string) (int, int, bool) {
	var payload map[string]any
	if json.Unmarshal([]byte(payloadJSON), &payload) != nil {
		return 0, 0, false
	}
	images, _ := payload["images"].([]any)
	if len(images) == 0 {
		return 0, 0, false
	}
	image, _ := images[0].(map[string]any)
	if image == nil {
		return 0, 0, false
	}
	width, widthOK := image["width"].(float64)
	height, heightOK := image["height"].(float64)
	if !widthOK || !heightOK || width <= 0 || height <= 0 {
		return 0, 0, false
	}
	return int(width), int(height), true
}

func ecommerceDimensionsMeetTarget(actualWidth int, actualHeight int, expectedWidth int, expectedHeight int) bool {
	if actualWidth < expectedWidth || actualHeight < expectedHeight {
		return false
	}
	difference := actualWidth*expectedHeight - expectedWidth*actualHeight
	if difference < 0 {
		difference = -difference
	}
	return difference*100 <= expectedWidth*actualHeight
}

func finishEcommerceProductionTask(tx *gorm.DB, task *model.Task, status model.TaskStatus, errorText string, completedAt time.Time) error {
	if task == nil || task.Provider != model.TaskProviderEcommerce {
		return nil
	}
	var attempt model.EcommerceProductionAttempt
	if err := tx.First(&attempt, "task_id = ?", task.ID).Error; err != nil {
		return err
	}
	attemptStatus := string(status)
	slotStatus := string(status)
	if err := tx.Model(&model.EcommerceProductionAttempt{}).Where("id = ?", attempt.ID).Updates(map[string]any{
		"status": attemptStatus, "provider_request_id": task.ProviderRequestID, "error": errorText, "completed_at": &completedAt, "updated_at": completedAt,
	}).Error; err != nil {
		return err
	}
	if err := tx.Model(&model.EcommerceProductionSlot{}).Where("id = ? AND active_attempt_id = ?", attempt.SlotID, attempt.ID).Updates(map[string]any{
		"status": slotStatus, "qa_status": "FAIL", "qa_issues_json": `["generation_failed"]`, "qa_note": errorText, "updated_at": completedAt,
	}).Error; err != nil {
		return err
	}
	return refreshEcommerceProductionRunStatus(tx, attempt.RunID, completedAt)
}

func refreshEcommerceProductionRunStatus(tx *gorm.DB, runID string, now time.Time) error {
	var slots []model.EcommerceProductionSlot
	if err := tx.Where("run_id = ?", runID).Find(&slots).Error; err != nil {
		return err
	}
	status := "ready"
	completed := true
	succeeded := 0
	for _, slot := range slots {
		switch slot.Status {
		case "planned", "scheduled", "queued", "running":
			status = "generating"
			completed = false
		case "qa":
			succeeded++
			if status != "generating" {
				status = "qa"
			}
		case "accepted":
			succeeded++
		case "failed", "cancelled":
			if status != "generating" && status != "qa" {
				status = "needs_you"
			}
		default:
			status = "needs_you"
		}
		if !slot.Accepted {
			completed = false
			if status == "ready" {
				status = "needs_you"
			}
		}
	}
	if succeeded == 0 && status != "generating" {
		status = "failed"
	}
	updates := map[string]any{"status": status, "updated_at": now}
	if completed && status == "ready" {
		updates["completed_at"] = &now
	} else {
		updates["completed_at"] = nil
	}
	return tx.Model(&model.EcommerceProductionRun{}).Where("id = ?", runID).Updates(updates).Error
}

type EcommerceSlotQAInput struct {
	UserID     string
	ProjectID  string
	RunID      string
	SlotID     string
	AttemptID  string
	Decision   string
	Action     string
	IssuesJSON string
	Note       string
	Artifact   *model.EcommerceArtifact
	Now        time.Time
}

func (r *Repository) RecordEcommerceSlotQA(input EcommerceSlotQAInput) error {
	if input.Artifact == nil {
		return gorm.ErrInvalidData
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		var existing model.EcommerceArtifact
		existingErr := tx.First(&existing, "id = ?", input.Artifact.ID).Error
		if existingErr == nil {
			if existing.ProjectID == input.Artifact.ProjectID && existing.ArtifactKey == input.Artifact.ArtifactKey &&
				existing.ArtifactType == input.Artifact.ArtifactType && existing.PayloadJSON == input.Artifact.PayloadJSON {
				return nil
			}
			return ErrEcommerceQAReviewConflict
		}
		if !errors.Is(existingErr, gorm.ErrRecordNotFound) {
			return existingErr
		}
		var slot model.EcommerceProductionSlot
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&slot, "id = ? AND run_id = ? AND project_id = ? AND user_id = ?", input.SlotID, input.RunID, input.ProjectID, input.UserID).Error; err != nil {
			return err
		}
		var attempt model.EcommerceProductionAttempt
		if err := tx.First(&attempt, "id = ? AND slot_id = ? AND run_id = ? AND status = ?", input.AttemptID, slot.ID, input.RunID, "succeeded").Error; err != nil {
			return err
		}
		if err := appendEcommerceArtifactVersion(tx, input.Artifact); err != nil {
			return err
		}
		accepted := input.Action == "accept" && input.Decision == "PASS"
		status := "qa"
		var acceptedAt *time.Time
		acceptedAttemptID := ""
		if accepted {
			status = "accepted"
			acceptedAt = &input.Now
			acceptedAttemptID = attempt.ID
		}
		if err := tx.Model(&model.EcommerceProductionSlot{}).Where("id = ?", slot.ID).Updates(map[string]any{
			"status": status, "qa_status": input.Decision, "qa_issues_json": input.IssuesJSON, "qa_note": input.Note,
			"accepted": accepted, "accepted_attempt_id": acceptedAttemptID, "accepted_at": acceptedAt,
			"active_attempt_id": attempt.ID, "active_task_id": attempt.TaskID,
			"result_url": attempt.ResultURL, "result_payload_json": attempt.ResultPayloadJSON,
			"generated_asset_id": attempt.GeneratedAssetArtifactID, "updated_at": input.Now,
		}).Error; err != nil {
			return err
		}
		if err := refreshEcommerceProductionRunStatus(tx, input.RunID, input.Now); err != nil {
			return err
		}
		result := tx.Model(&model.Project{}).
			Where("id = ? AND user_id = ?", input.ProjectID, input.UserID).
			Updates(map[string]any{"revision": gorm.Expr("revision + 1"), "updated_at": input.Now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (r *Repository) SaveEcommerceVideoPlan(run *model.EcommerceProductionRun, artifacts []model.EcommerceArtifact) error {
	if run == nil || len(artifacts) != 2 {
		return gorm.ErrInvalidData
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		var current model.EcommerceProductionRun
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, "id = ? AND user_id = ? AND project_id = ?", run.ID, run.UserID, run.ProjectID).Error; err != nil {
			return err
		}
		for index := range artifacts {
			if err := appendEcommerceArtifactVersion(tx, &artifacts[index]); err != nil {
				return err
			}
			switch artifacts[index].ArtifactType {
			case "motion_plan":
				run.MotionPlanArtifactID = artifacts[index].ID
			case "video_sequence":
				run.VideoSequenceArtifactID = artifacts[index].ID
			}
		}
		return tx.Model(&model.EcommerceProductionRun{}).Where("id = ?", current.ID).Updates(map[string]any{
			"motion_plan_artifact_id": run.MotionPlanArtifactID, "video_sequence_artifact_id": run.VideoSequenceArtifactID, "updated_at": run.UpdatedAt,
		}).Error
	})
}
