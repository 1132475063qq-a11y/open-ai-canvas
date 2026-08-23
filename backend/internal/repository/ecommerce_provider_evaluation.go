package repository

import (
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

func (r *Repository) EcommerceProviderEvaluationPlanByIdempotency(userID string, projectID string, key string) (*model.EcommerceProviderEvaluationPlan, error) {
	var plan model.EcommerceProviderEvaluationPlan
	if err := r.db.First(&plan, "user_id = ? AND project_id = ? AND idempotency_key = ?", userID, projectID, key).Error; err != nil {
		return nil, err
	}
	return &plan, nil
}

func (r *Repository) EcommerceProviderEvaluationPlanForUser(userID string, projectID string, planID string) (*model.EcommerceProviderEvaluationPlan, error) {
	var plan model.EcommerceProviderEvaluationPlan
	if err := r.db.First(&plan, "id = ? AND user_id = ? AND project_id = ?", planID, userID, projectID).Error; err != nil {
		return nil, err
	}
	return &plan, nil
}

func (r *Repository) ProjectEcommerceProviderEvaluationPlans(userID string, projectID string, limit int) ([]model.EcommerceProviderEvaluationPlan, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var plans []model.EcommerceProviderEvaluationPlan
	err := r.db.Where("user_id = ? AND project_id = ?", userID, projectID).Order("created_at desc").Limit(limit).Find(&plans).Error
	return plans, err
}

func (r *Repository) CreateEcommerceProviderEvaluationPlan(plan *model.EcommerceProviderEvaluationPlan) error {
	return r.db.Create(plan).Error
}

func (r *Repository) EcommerceProviderEvaluationAttempts(planID string) ([]model.EcommerceProviderEvaluationAttempt, error) {
	var attempts []model.EcommerceProviderEvaluationAttempt
	err := r.db.Where("plan_id = ?", planID).Order("started_at asc, created_at asc").Find(&attempts).Error
	return attempts, err
}

func (r *Repository) EcommerceProviderEvaluationAttemptForUser(userID string, projectID string, planID string, attemptID string) (*model.EcommerceProviderEvaluationAttempt, error) {
	var attempt model.EcommerceProviderEvaluationAttempt
	if err := r.db.First(&attempt, "id = ? AND user_id = ? AND project_id = ? AND plan_id = ?", attemptID, userID, projectID, planID).Error; err != nil {
		return nil, err
	}
	return &attempt, nil
}

func (r *Repository) EcommerceProviderEvaluationAttemptByIdempotency(planID string, key string) (*model.EcommerceProviderEvaluationAttempt, error) {
	var attempt model.EcommerceProviderEvaluationAttempt
	if err := r.db.First(&attempt, "plan_id = ? AND idempotency_key = ?", planID, key).Error; err != nil {
		return nil, err
	}
	return &attempt, nil
}

func (r *Repository) CreateEcommerceProviderEvaluationAttempt(attempt *model.EcommerceProviderEvaluationAttempt) error {
	return r.db.Create(attempt).Error
}

func (r *Repository) SetEcommerceProviderEvaluationPlanStatus(planID string, status string, now time.Time) error {
	return r.db.Model(&model.EcommerceProviderEvaluationPlan{}).
		Where("id = ? AND decision = ''", planID).
		Updates(map[string]any{"status": status, "updated_at": now}).Error
}

func (r *Repository) EcommerceProviderEvaluationScores(planID string) ([]model.EcommerceProviderEvaluationScore, error) {
	var scores []model.EcommerceProviderEvaluationScore
	err := r.db.Where("plan_id = ?", planID).Order("recorded_at asc, created_at asc").Find(&scores).Error
	return scores, err
}

func (r *Repository) EcommerceProviderEvaluationScoreByIdempotency(planID string, key string) (*model.EcommerceProviderEvaluationScore, error) {
	var score model.EcommerceProviderEvaluationScore
	if err := r.db.First(&score, "plan_id = ? AND idempotency_key = ?", planID, key).Error; err != nil {
		return nil, err
	}
	return &score, nil
}

func (r *Repository) CreateEcommerceProviderEvaluationScore(score *model.EcommerceProviderEvaluationScore) error {
	return r.db.Create(score).Error
}

func (r *Repository) EcommerceProviderEvaluationPreferences(planID string) ([]model.EcommerceProviderEvaluationPreference, error) {
	var preferences []model.EcommerceProviderEvaluationPreference
	err := r.db.Where("plan_id = ?", planID).Order("recorded_at asc, created_at asc").Find(&preferences).Error
	return preferences, err
}

func (r *Repository) EcommerceProviderEvaluationPreferenceByIdempotency(planID string, key string) (*model.EcommerceProviderEvaluationPreference, error) {
	var preference model.EcommerceProviderEvaluationPreference
	if err := r.db.First(&preference, "plan_id = ? AND idempotency_key = ?", planID, key).Error; err != nil {
		return nil, err
	}
	return &preference, nil
}

func (r *Repository) CreateEcommerceProviderEvaluationPreference(preference *model.EcommerceProviderEvaluationPreference) error {
	return r.db.Create(preference).Error
}

func (r *Repository) RecordEcommerceProviderEvaluationDecision(planID string, decision string, reviewerRef string, rationale string, nextAction string, evidence string, now time.Time) error {
	result := r.db.Model(&model.EcommerceProviderEvaluationPlan{}).
		Where("id = ? AND decision = ''", planID).
		Updates(map[string]any{
			"status":                decision,
			"decision":              decision,
			"decision_reviewer_ref": reviewerRef,
			"decision_rationale":    rationale,
			"decision_next_action":  nextAction,
			"decision_evidence":     evidence,
			"decision_recorded_at":  now,
			"updated_at":            now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrInvalidData
	}
	return nil
}
