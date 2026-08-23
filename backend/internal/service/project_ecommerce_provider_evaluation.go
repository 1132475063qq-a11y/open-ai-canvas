package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

const (
	EcommerceProviderEvaluationStatusDraft    = "draft"
	EcommerceProviderEvaluationStatusRunning  = "running"
	EcommerceProviderEvaluationStatusComplete = "complete"
	EcommerceProviderEvaluationStatusGo       = "go"
	EcommerceProviderEvaluationStatusModify   = "modify"
	EcommerceProviderEvaluationStatusStop     = "stop"
)

type CreateEcommerceProviderEvaluationPlanRequest struct {
	PlanID         string           `json:"planId"`
	IdempotencyKey string           `json:"idempotencyKey"`
	SkillRef       string           `json:"skillRef"`
	Settings       map[string]any   `json:"settings"`
	Cases          []map[string]any `json:"cases"`
	Candidates     []map[string]any `json:"candidates"`
}

type RecordEcommerceProviderEvaluationAttemptRequest struct {
	AttemptID           string         `json:"attemptId"`
	IdempotencyKey      string         `json:"idempotencyKey"`
	CaseID              string         `json:"caseId"`
	CandidateID         string         `json:"candidateId"`
	Variant             string         `json:"variant"`
	Status              string         `json:"status"`
	SettingsFingerprint string         `json:"settingsFingerprint"`
	RequestFingerprint  string         `json:"requestFingerprint"`
	ProviderJobID       string         `json:"providerJobId"`
	ResultRefs          []string       `json:"resultRefs"`
	StartedAt           time.Time      `json:"startedAt"`
	CompletedAt         *time.Time     `json:"completedAt"`
	LatencyMS           int64          `json:"latencyMs"`
	Usage               map[string]any `json:"usage"`
	Cost                map[string]any `json:"cost"`
	Failure             map[string]any `json:"failure"`
	Evidence            string         `json:"evidence"`
}

type RecordEcommerceProviderEvaluationScoreRequest struct {
	ScoreID        string         `json:"scoreId"`
	IdempotencyKey string         `json:"idempotencyKey"`
	AttemptID      string         `json:"attemptId"`
	EvaluatorRef   string         `json:"evaluatorRef"`
	Dimensions     map[string]any `json:"dimensions"`
	Notes          string         `json:"notes"`
	Evidence       string         `json:"evidence"`
	RecordedAt     time.Time      `json:"recordedAt"`
}

type RecordEcommerceProviderEvaluationPreferenceRequest struct {
	PreferenceID       string    `json:"preferenceId"`
	IdempotencyKey     string    `json:"idempotencyKey"`
	CaseID             string    `json:"caseId"`
	ResultRefs         []string  `json:"resultRefs"`
	PreferredResultRef string    `json:"preferredResultRef"`
	Tie                bool      `json:"tie"`
	VoterRef           string    `json:"voterRef"`
	Blinded            bool      `json:"blinded"`
	Evidence           string    `json:"evidence"`
	RecordedAt         time.Time `json:"recordedAt"`
}

type RecordEcommerceProviderEvaluationDecisionRequest struct {
	Decision    string `json:"decision"`
	ReviewerRef string `json:"reviewerRef"`
	Rationale   string `json:"rationale"`
	NextAction  string `json:"nextAction"`
	Evidence    string `json:"evidence"`
}

type EcommerceProviderEvaluationAttemptView struct {
	Attempt    model.EcommerceProviderEvaluationAttempt `json:"attempt"`
	ResultRefs []string                                 `json:"resultRefs"`
	Usage      map[string]any                           `json:"usage,omitempty"`
	Cost       map[string]any                           `json:"cost,omitempty"`
	Failure    map[string]any                           `json:"failure,omitempty"`
}

type EcommerceProviderEvaluationScoreView struct {
	Score      model.EcommerceProviderEvaluationScore `json:"score"`
	Dimensions map[string]any                         `json:"dimensions"`
}

type EcommerceProviderEvaluationPreferenceView struct {
	Preference model.EcommerceProviderEvaluationPreference `json:"preference"`
	ResultRefs []string                                    `json:"resultRefs"`
}

type EcommerceProviderEvaluationCandidateSummary struct {
	CandidateID       string             `json:"candidateId"`
	Attempts          int                `json:"attempts"`
	Succeeded         int                `json:"succeeded"`
	Failed            int                `json:"failed"`
	NeedsYou          int                `json:"needsYou"`
	SuccessRate       *float64           `json:"successRate"`
	AverageLatencyMS  *float64           `json:"averageLatencyMs"`
	KnownCost         map[string]any     `json:"knownCost,omitempty"`
	ScoreCount        int                `json:"scoreCount"`
	DimensionAverages map[string]float64 `json:"dimensionAverages"`
	FailureModes      map[string]int     `json:"failureModes"`
	Evidence          string             `json:"evidence"`
}

type EcommerceProviderEvaluationSummary struct {
	PlanID               string                                        `json:"planId"`
	CandidateSummaries   []EcommerceProviderEvaluationCandidateSummary `json:"candidateSummaries"`
	BlindPreferenceCount int                                           `json:"blindPreferenceCount"`
	Decision             string                                        `json:"decision"`
	HasRecordedResults   bool                                          `json:"hasRecordedResults"`
	HasUnknowns          bool                                          `json:"hasUnknowns"`
}

type EcommerceProviderEvaluationPlanView struct {
	Plan        model.EcommerceProviderEvaluationPlan       `json:"plan"`
	Settings    map[string]any                              `json:"settings"`
	Cases       []map[string]any                            `json:"cases"`
	Candidates  []map[string]any                            `json:"candidates"`
	Attempts    []EcommerceProviderEvaluationAttemptView    `json:"attempts"`
	Scores      []EcommerceProviderEvaluationScoreView      `json:"scores"`
	Preferences []EcommerceProviderEvaluationPreferenceView `json:"blindPreferences"`
	Summary     EcommerceProviderEvaluationSummary          `json:"summary"`
}

type CreateEcommerceProviderEvaluationPlanResult struct {
	Plan       EcommerceProviderEvaluationPlanView `json:"plan"`
	Idempotent bool                                `json:"idempotent"`
}

func (s *Service) ProjectEcommerceProviderEvaluationPlans(userID string, projectID string, limit int) ([]model.EcommerceProviderEvaluationPlan, error) {
	if _, err := s.requireEcommerceProject(userID, projectID); err != nil {
		return nil, err
	}
	plans, err := s.repo.ProjectEcommerceProviderEvaluationPlans(userID, projectID, limit)
	if err != nil {
		return nil, err
	}
	return plans, nil
}

func (s *Service) ProjectEcommerceProviderEvaluationPlan(userID string, projectID string, planID string) (EcommerceProviderEvaluationPlanView, error) {
	if _, err := s.requireEcommerceProject(userID, projectID); err != nil {
		return EcommerceProviderEvaluationPlanView{}, err
	}
	plan, err := s.repo.EcommerceProviderEvaluationPlanForUser(userID, projectID, strings.TrimSpace(planID))
	if err != nil {
		return EcommerceProviderEvaluationPlanView{}, mapEcommerceProviderEvaluationError(err)
	}
	return s.ecommerceProviderEvaluationPlanView(*plan)
}

func (s *Service) CreateProjectEcommerceProviderEvaluationPlan(userID string, projectID string, request CreateEcommerceProviderEvaluationPlanRequest) (CreateEcommerceProviderEvaluationPlanResult, error) {
	if _, err := s.requireEcommerceProject(userID, projectID); err != nil {
		return CreateEcommerceProviderEvaluationPlanResult{}, err
	}
	key := strings.TrimSpace(request.IdempotencyKey)
	if key == "" {
		return CreateEcommerceProviderEvaluationPlanResult{}, BadAuthRequest("Provider Bake-off 计划必须提供幂等键")
	}
	fingerprint, err := evaluationFingerprint(request.SkillRef, request.Settings, request.Cases, request.Candidates)
	if err != nil {
		return CreateEcommerceProviderEvaluationPlanResult{}, err
	}
	if existing, lookupErr := s.repo.EcommerceProviderEvaluationPlanByIdempotency(userID, projectID, key); lookupErr == nil {
		if existing.RequestFingerprint != fingerprint {
			return CreateEcommerceProviderEvaluationPlanResult{}, Conflict("Provider Bake-off 幂等键已用于另一份计划")
		}
		view, viewErr := s.ecommerceProviderEvaluationPlanView(*existing)
		return CreateEcommerceProviderEvaluationPlanResult{Plan: view, Idempotent: true}, viewErr
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return CreateEcommerceProviderEvaluationPlanResult{}, lookupErr
	}
	if err := validateEcommerceProviderEvaluationPlanRequest(projectID, request); err != nil {
		return CreateEcommerceProviderEvaluationPlanResult{}, err
	}
	settingsJSON, _ := json.Marshal(request.Settings)
	casesJSON, _ := json.Marshal(request.Cases)
	candidatesJSON, _ := json.Marshal(request.Candidates)
	now := time.Now().UTC()
	plan := model.EcommerceProviderEvaluationPlan{
		ID: newID(), UserID: userID, ProjectID: projectID, IdempotencyKey: key,
		SkillRef: strings.TrimSpace(request.SkillRef), Mode: evaluationString(request.Settings, "mode"),
		Status: EcommerceProviderEvaluationStatusDraft, RequestFingerprint: fingerprint,
		SettingsJSON: string(settingsJSON), CasesJSON: string(casesJSON), CandidatesJSON: string(candidatesJSON),
		CreatedAt: now, UpdatedAt: now,
	}
	if planID := strings.TrimSpace(request.PlanID); planID != "" {
		if len(planID) > 36 {
			return CreateEcommerceProviderEvaluationPlanResult{}, BadAuthRequest("Provider Bake-off planId 不能超过 36 个字符")
		}
		plan.ID = planID
	}
	if err := s.repo.CreateEcommerceProviderEvaluationPlan(&plan); err != nil {
		return CreateEcommerceProviderEvaluationPlanResult{}, mapEcommerceProviderEvaluationError(err)
	}
	view, err := s.ecommerceProviderEvaluationPlanView(plan)
	return CreateEcommerceProviderEvaluationPlanResult{Plan: view}, err
}

func (s *Service) RecordProjectEcommerceProviderEvaluationAttempt(userID string, projectID string, planID string, request RecordEcommerceProviderEvaluationAttemptRequest) (EcommerceProviderEvaluationPlanView, bool, error) {
	plan, err := s.ecommerceProviderEvaluationPlanForUser(userID, projectID, planID)
	if err != nil {
		return EcommerceProviderEvaluationPlanView{}, false, err
	}
	if plan.Decision != "" {
		return EcommerceProviderEvaluationPlanView{}, false, Conflict("Provider Bake-off 已经完成决策，不能继续写入证据")
	}
	key := strings.TrimSpace(request.IdempotencyKey)
	if key == "" {
		return EcommerceProviderEvaluationPlanView{}, false, BadAuthRequest("Provider Bake-off Attempt 必须提供幂等键")
	}
	if _, lookupErr := s.repo.EcommerceProviderEvaluationAttemptByIdempotency(plan.ID, key); lookupErr == nil {
		view, viewErr := s.ecommerceProviderEvaluationPlanView(*plan)
		return view, true, viewErr
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return EcommerceProviderEvaluationPlanView{}, false, lookupErr
	}
	if err := validateEcommerceProviderEvaluationAttempt(plan, request); err != nil {
		return EcommerceProviderEvaluationPlanView{}, false, err
	}
	resultRefsJSON, _ := json.Marshal(uniqueNonEmpty(request.ResultRefs))
	usageJSON, _ := json.Marshal(nonNilMap(request.Usage))
	costJSON, _ := json.Marshal(nonNilMap(request.Cost))
	failureJSON, _ := json.Marshal(nonNilMap(request.Failure))
	now := time.Now().UTC()
	startedAt := request.StartedAt
	if startedAt.IsZero() {
		startedAt = now
	}
	attempt := model.EcommerceProviderEvaluationAttempt{
		ID: newID(), UserID: userID, ProjectID: projectID, PlanID: plan.ID, IdempotencyKey: key,
		CaseID: strings.TrimSpace(request.CaseID), CandidateID: strings.TrimSpace(request.CandidateID), Variant: strings.TrimSpace(request.Variant),
		Status: strings.TrimSpace(request.Status), SettingsFingerprint: strings.TrimSpace(request.SettingsFingerprint), RequestFingerprint: strings.TrimSpace(request.RequestFingerprint),
		ProviderJobID: strings.TrimSpace(request.ProviderJobID), ResultRefsJSON: string(resultRefsJSON), StartedAt: startedAt,
		CompletedAt: request.CompletedAt, LatencyMS: request.LatencyMS, UsageJSON: string(usageJSON), CostJSON: string(costJSON), FailureJSON: string(failureJSON),
		Evidence: normalizedEvaluationEvidence(request.Evidence), CreatedAt: now, UpdatedAt: now,
	}
	if id := strings.TrimSpace(request.AttemptID); id != "" {
		if len(id) > 36 {
			return EcommerceProviderEvaluationPlanView{}, false, BadAuthRequest("Provider Bake-off attemptId 不能超过 36 个字符")
		}
		attempt.ID = id
	}
	if err := s.repo.CreateEcommerceProviderEvaluationAttempt(&attempt); err != nil {
		return EcommerceProviderEvaluationPlanView{}, false, mapEcommerceProviderEvaluationError(err)
	}
	if err := s.repo.SetEcommerceProviderEvaluationPlanStatus(plan.ID, EcommerceProviderEvaluationStatusRunning, now); err != nil {
		return EcommerceProviderEvaluationPlanView{}, false, err
	}
	plan.Status = EcommerceProviderEvaluationStatusRunning
	plan.UpdatedAt = now
	view, err := s.ecommerceProviderEvaluationPlanView(*plan)
	return view, false, err
}

func (s *Service) RecordProjectEcommerceProviderEvaluationScore(userID string, projectID string, planID string, request RecordEcommerceProviderEvaluationScoreRequest) (EcommerceProviderEvaluationPlanView, bool, error) {
	plan, err := s.ecommerceProviderEvaluationPlanForUser(userID, projectID, planID)
	if err != nil {
		return EcommerceProviderEvaluationPlanView{}, false, err
	}
	if plan.Decision != "" {
		return EcommerceProviderEvaluationPlanView{}, false, Conflict("Provider Bake-off 已经完成决策，不能继续写入评分")
	}
	key := strings.TrimSpace(request.IdempotencyKey)
	if key == "" {
		return EcommerceProviderEvaluationPlanView{}, false, BadAuthRequest("Provider Bake-off 评分必须提供幂等键")
	}
	if _, lookupErr := s.repo.EcommerceProviderEvaluationScoreByIdempotency(plan.ID, key); lookupErr == nil {
		view, viewErr := s.ecommerceProviderEvaluationPlanView(*plan)
		return view, true, viewErr
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return EcommerceProviderEvaluationPlanView{}, false, lookupErr
	}
	attempt, err := s.repo.EcommerceProviderEvaluationAttemptForUser(userID, projectID, plan.ID, strings.TrimSpace(request.AttemptID))
	if err != nil {
		return EcommerceProviderEvaluationPlanView{}, false, mapEcommerceProviderEvaluationError(err)
	}
	if attempt.Status != "succeeded" {
		return EcommerceProviderEvaluationPlanView{}, false, BadAuthRequest("只有 succeeded 的 Provider Bake-off Attempt 才能评分")
	}
	if err := validateEcommerceProviderEvaluationScore(request); err != nil {
		return EcommerceProviderEvaluationPlanView{}, false, err
	}
	dimensionsJSON, _ := json.Marshal(request.Dimensions)
	now := request.RecordedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	score := model.EcommerceProviderEvaluationScore{
		ID: newID(), UserID: userID, ProjectID: projectID, PlanID: plan.ID, AttemptID: attempt.ID,
		IdempotencyKey: key, EvaluatorRef: strings.TrimSpace(request.EvaluatorRef), DimensionsJSON: string(dimensionsJSON),
		Notes: strings.TrimSpace(request.Notes), Evidence: normalizedEvaluationEvidence(request.Evidence), RecordedAt: now, CreatedAt: now,
	}
	if id := strings.TrimSpace(request.ScoreID); id != "" {
		if len(id) > 36 {
			return EcommerceProviderEvaluationPlanView{}, false, BadAuthRequest("Provider Bake-off scoreId 不能超过 36 个字符")
		}
		score.ID = id
	}
	if err := s.repo.CreateEcommerceProviderEvaluationScore(&score); err != nil {
		return EcommerceProviderEvaluationPlanView{}, false, mapEcommerceProviderEvaluationError(err)
	}
	view, err := s.ecommerceProviderEvaluationPlanView(*plan)
	return view, false, err
}

func (s *Service) RecordProjectEcommerceProviderEvaluationPreference(userID string, projectID string, planID string, request RecordEcommerceProviderEvaluationPreferenceRequest) (EcommerceProviderEvaluationPlanView, bool, error) {
	plan, err := s.ecommerceProviderEvaluationPlanForUser(userID, projectID, planID)
	if err != nil {
		return EcommerceProviderEvaluationPlanView{}, false, err
	}
	if plan.Decision != "" {
		return EcommerceProviderEvaluationPlanView{}, false, Conflict("Provider Bake-off 已经完成决策，不能继续写入盲测")
	}
	key := strings.TrimSpace(request.IdempotencyKey)
	if key == "" {
		return EcommerceProviderEvaluationPlanView{}, false, BadAuthRequest("Provider Bake-off 盲测必须提供幂等键")
	}
	if _, lookupErr := s.repo.EcommerceProviderEvaluationPreferenceByIdempotency(plan.ID, key); lookupErr == nil {
		view, viewErr := s.ecommerceProviderEvaluationPlanView(*plan)
		return view, true, viewErr
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return EcommerceProviderEvaluationPlanView{}, false, lookupErr
	}
	attempts, err := s.repo.EcommerceProviderEvaluationAttempts(plan.ID)
	if err != nil {
		return EcommerceProviderEvaluationPlanView{}, false, err
	}
	if err := validateEcommerceProviderEvaluationPreference(plan, attempts, request); err != nil {
		return EcommerceProviderEvaluationPlanView{}, false, err
	}
	refsJSON, _ := json.Marshal(uniqueNonEmpty(request.ResultRefs))
	now := request.RecordedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	preference := model.EcommerceProviderEvaluationPreference{
		ID: newID(), UserID: userID, ProjectID: projectID, PlanID: plan.ID, IdempotencyKey: key,
		CaseID: strings.TrimSpace(request.CaseID), ResultRefsJSON: string(refsJSON), PreferredResultRef: strings.TrimSpace(request.PreferredResultRef),
		Tie: request.Tie, VoterRef: strings.TrimSpace(request.VoterRef), Blinded: true,
		Evidence: normalizedEvaluationEvidence(request.Evidence), RecordedAt: now, CreatedAt: now,
	}
	if id := strings.TrimSpace(request.PreferenceID); id != "" {
		if len(id) > 36 {
			return EcommerceProviderEvaluationPlanView{}, false, BadAuthRequest("Provider Bake-off preferenceId 不能超过 36 个字符")
		}
		preference.ID = id
	}
	if err := s.repo.CreateEcommerceProviderEvaluationPreference(&preference); err != nil {
		return EcommerceProviderEvaluationPlanView{}, false, mapEcommerceProviderEvaluationError(err)
	}
	view, err := s.ecommerceProviderEvaluationPlanView(*plan)
	return view, false, err
}

func (s *Service) RecordProjectEcommerceProviderEvaluationDecision(userID string, projectID string, planID string, request RecordEcommerceProviderEvaluationDecisionRequest) (EcommerceProviderEvaluationPlanView, error) {
	plan, err := s.ecommerceProviderEvaluationPlanForUser(userID, projectID, planID)
	if err != nil {
		return EcommerceProviderEvaluationPlanView{}, err
	}
	if _, err := s.repo.EcommerceProviderEvaluationAttempts(plan.ID); err != nil {
		return EcommerceProviderEvaluationPlanView{}, err
	}
	if strings.TrimSpace(request.Decision) != EcommerceProviderEvaluationStatusGo && strings.TrimSpace(request.Decision) != EcommerceProviderEvaluationStatusModify && strings.TrimSpace(request.Decision) != EcommerceProviderEvaluationStatusStop {
		return EcommerceProviderEvaluationPlanView{}, BadAuthRequest("Provider Bake-off 决策只能是 go、modify 或 stop")
	}
	if strings.TrimSpace(request.ReviewerRef) == "" || strings.TrimSpace(request.Rationale) == "" || strings.TrimSpace(request.NextAction) == "" {
		return EcommerceProviderEvaluationPlanView{}, BadAuthRequest("Provider Bake-off 决策必须包含 reviewer、rationale 和 nextAction")
	}
	attempts, err := s.repo.EcommerceProviderEvaluationAttempts(plan.ID)
	if err != nil {
		return EcommerceProviderEvaluationPlanView{}, err
	}
	if len(attempts) == 0 {
		return EcommerceProviderEvaluationPlanView{}, BadAuthRequest("Provider Bake-off 决策必须建立在至少一次 Attempt 之上")
	}
	now := time.Now().UTC()
	if err := s.repo.RecordEcommerceProviderEvaluationDecision(plan.ID, strings.TrimSpace(request.Decision), strings.TrimSpace(request.ReviewerRef), strings.TrimSpace(request.Rationale), strings.TrimSpace(request.NextAction), normalizedEvaluationEvidence(request.Evidence), now); err != nil {
		if errors.Is(err, gorm.ErrInvalidData) {
			return EcommerceProviderEvaluationPlanView{}, Conflict("Provider Bake-off 决策已被其他操作记录，请刷新")
		}
		return EcommerceProviderEvaluationPlanView{}, err
	}
	plan.Decision = strings.TrimSpace(request.Decision)
	plan.Status = plan.Decision
	plan.DecisionReviewerRef = strings.TrimSpace(request.ReviewerRef)
	plan.DecisionRationale = strings.TrimSpace(request.Rationale)
	plan.DecisionNextAction = strings.TrimSpace(request.NextAction)
	plan.DecisionEvidence = normalizedEvaluationEvidence(request.Evidence)
	plan.DecisionRecordedAt = &now
	view, err := s.ecommerceProviderEvaluationPlanView(*plan)
	return view, err
}

func (s *Service) ecommerceProviderEvaluationPlanForUser(userID string, projectID string, planID string) (*model.EcommerceProviderEvaluationPlan, error) {
	if _, err := s.requireEcommerceProject(userID, projectID); err != nil {
		return nil, err
	}
	plan, err := s.repo.EcommerceProviderEvaluationPlanForUser(userID, projectID, strings.TrimSpace(planID))
	if err != nil {
		return nil, mapEcommerceProviderEvaluationError(err)
	}
	return plan, nil
}

func (s *Service) ecommerceProviderEvaluationPlanView(plan model.EcommerceProviderEvaluationPlan) (EcommerceProviderEvaluationPlanView, error) {
	settings := map[string]any{}
	cases := []map[string]any{}
	candidates := []map[string]any{}
	if err := json.Unmarshal([]byte(plan.SettingsJSON), &settings); err != nil {
		return EcommerceProviderEvaluationPlanView{}, fmt.Errorf("解析 Provider Bake-off settings 失败：%w", err)
	}
	if err := json.Unmarshal([]byte(plan.CasesJSON), &cases); err != nil {
		return EcommerceProviderEvaluationPlanView{}, fmt.Errorf("解析 Provider Bake-off cases 失败：%w", err)
	}
	if err := json.Unmarshal([]byte(plan.CandidatesJSON), &candidates); err != nil {
		return EcommerceProviderEvaluationPlanView{}, fmt.Errorf("解析 Provider Bake-off candidates 失败：%w", err)
	}
	attemptRows, err := s.repo.EcommerceProviderEvaluationAttempts(plan.ID)
	if err != nil {
		return EcommerceProviderEvaluationPlanView{}, err
	}
	scoreRows, err := s.repo.EcommerceProviderEvaluationScores(plan.ID)
	if err != nil {
		return EcommerceProviderEvaluationPlanView{}, err
	}
	preferenceRows, err := s.repo.EcommerceProviderEvaluationPreferences(plan.ID)
	if err != nil {
		return EcommerceProviderEvaluationPlanView{}, err
	}
	attempts := make([]EcommerceProviderEvaluationAttemptView, 0, len(attemptRows))
	for _, row := range attemptRows {
		var resultRefs []string
		var usage, cost, failure map[string]any
		if err := json.Unmarshal([]byte(row.ResultRefsJSON), &resultRefs); err != nil {
			return EcommerceProviderEvaluationPlanView{}, err
		}
		if err := json.Unmarshal([]byte(row.UsageJSON), &usage); err != nil {
			return EcommerceProviderEvaluationPlanView{}, err
		}
		if err := json.Unmarshal([]byte(row.CostJSON), &cost); err != nil {
			return EcommerceProviderEvaluationPlanView{}, err
		}
		if err := json.Unmarshal([]byte(row.FailureJSON), &failure); err != nil {
			return EcommerceProviderEvaluationPlanView{}, err
		}
		attempts = append(attempts, EcommerceProviderEvaluationAttemptView{Attempt: row, ResultRefs: resultRefs, Usage: usage, Cost: cost, Failure: failure})
	}
	scores := make([]EcommerceProviderEvaluationScoreView, 0, len(scoreRows))
	for _, row := range scoreRows {
		var dimensions map[string]any
		if err := json.Unmarshal([]byte(row.DimensionsJSON), &dimensions); err != nil {
			return EcommerceProviderEvaluationPlanView{}, err
		}
		scores = append(scores, EcommerceProviderEvaluationScoreView{Score: row, Dimensions: dimensions})
	}
	preferences := make([]EcommerceProviderEvaluationPreferenceView, 0, len(preferenceRows))
	for _, row := range preferenceRows {
		var resultRefs []string
		if err := json.Unmarshal([]byte(row.ResultRefsJSON), &resultRefs); err != nil {
			return EcommerceProviderEvaluationPlanView{}, err
		}
		preferences = append(preferences, EcommerceProviderEvaluationPreferenceView{Preference: row, ResultRefs: resultRefs})
	}
	return EcommerceProviderEvaluationPlanView{
		Plan: plan, Settings: settings, Cases: cases, Candidates: candidates, Attempts: attempts, Scores: scores, Preferences: preferences,
		Summary: summarizeEcommerceProviderEvaluation(plan, candidates, attempts, scores, preferences),
	}, nil
}

func summarizeEcommerceProviderEvaluation(plan model.EcommerceProviderEvaluationPlan, candidates []map[string]any, attempts []EcommerceProviderEvaluationAttemptView, scores []EcommerceProviderEvaluationScoreView, preferences []EcommerceProviderEvaluationPreferenceView) EcommerceProviderEvaluationSummary {
	summary := EcommerceProviderEvaluationSummary{PlanID: plan.ID, BlindPreferenceCount: len(preferences), Decision: "pending"}
	if plan.Decision != "" {
		summary.Decision = plan.Decision
	}
	for _, candidate := range candidates {
		candidateID := evaluationString(candidate, "candidateId")
		item := EcommerceProviderEvaluationCandidateSummary{CandidateID: candidateID, DimensionAverages: map[string]float64{}, FailureModes: map[string]int{}, Evidence: "recorded"}
		var latencyTotal int64
		var latencyCount int
		var scoreCount int
		var costTotal float64
		costCurrency := ""
		costCurrencyConsistent := true
		costCount := 0
		for _, attempt := range attempts {
			if attempt.Attempt.CandidateID != candidateID {
				continue
			}
			item.Attempts++
			switch attempt.Attempt.Status {
			case "succeeded":
				item.Succeeded++
				summary.HasRecordedResults = summary.HasRecordedResults || len(attempt.ResultRefs) > 0
			case "failed":
				item.Failed++
				var failure map[string]any
				if json.Unmarshal([]byte(attempt.Attempt.FailureJSON), &failure) == nil {
					code := evaluationString(failure, "code")
					if code == "" {
						code = "unknown"
					}
					item.FailureModes[code]++
				}
			case "needs_you":
				item.NeedsYou++
			}
			if attempt.Attempt.LatencyMS > 0 {
				latencyTotal += attempt.Attempt.LatencyMS
				latencyCount++
			}
			if attempt.Attempt.Evidence == "unknown" {
				item.Evidence = "unknown"
				summary.HasUnknowns = true
			} else if attempt.Attempt.Evidence == "inferred" && item.Evidence == "recorded" {
				item.Evidence = "inferred"
			}
			var cost map[string]any
			if json.Unmarshal([]byte(attempt.Attempt.CostJSON), &cost) == nil && evaluationString(cost, "status") == "recorded" {
				amount, amountOK := evaluationNumber(cost["amount"])
				currency := evaluationString(cost, "currency")
				if amountOK && currency != "" {
					if costCurrency == "" {
						costCurrency = currency
					} else if costCurrency != currency {
						costCurrencyConsistent = false
					}
					costTotal += amount
					costCount++
				}
			}
		}
		if item.Attempts > 0 {
			rate := float64(item.Succeeded) / float64(item.Attempts)
			item.SuccessRate = &rate
		}
		if latencyCount > 0 {
			average := float64(latencyTotal) / float64(latencyCount)
			item.AverageLatencyMS = &average
		}
		for _, score := range scores {
			attemptMatches := false
			for _, attempt := range attempts {
				if attempt.Attempt.ID == score.Score.AttemptID && attempt.Attempt.CandidateID == candidateID {
					attemptMatches = true
					break
				}
			}
			if !attemptMatches {
				continue
			}
			scoreCount++
			for key, value := range score.Dimensions {
				if number, ok := evaluationNumber(value); ok {
					item.DimensionAverages[key] += number
				}
			}
		}
		item.ScoreCount = scoreCount
		if scoreCount > 0 {
			for key, value := range item.DimensionAverages {
				item.DimensionAverages[key] = value / float64(scoreCount)
			}
		}
		if costCount > 0 && costCurrencyConsistent {
			item.KnownCost = map[string]any{"amount": costTotal, "currency": costCurrency}
		}
		if evaluationString(candidate, "availability") == "unknown" {
			item.Evidence = "unknown"
			summary.HasUnknowns = true
		}
		summary.CandidateSummaries = append(summary.CandidateSummaries, item)
	}
	for _, attempt := range attempts {
		var cost map[string]any
		if attempt.Attempt.Evidence == "unknown" || (json.Unmarshal([]byte(attempt.Attempt.CostJSON), &cost) == nil && evaluationString(cost, "status") == "unavailable") {
			summary.HasUnknowns = true
		}
	}
	return summary
}

func validateEcommerceProviderEvaluationPlanRequest(projectID string, request CreateEcommerceProviderEvaluationPlanRequest) error {
	if strings.TrimSpace(request.SkillRef) == "" {
		return BadAuthRequest("Provider Bake-off skillRef 不能为空")
	}
	if len(request.Settings) == 0 || evaluationString(request.Settings, "mode") == "" {
		return BadAuthRequest("Provider Bake-off settings 不完整")
	}
	mode := evaluationString(request.Settings, "mode")
	if mode != EcommerceKernelModelInteraction && mode != EcommerceKernelStillLife {
		return BadAuthRequest("Provider Bake-off mode 只能是 MODEL_INTERACTION 或 STILL_LIFE")
	}
	if positiveEvaluationInt(request.Settings, "width") < 1 || positiveEvaluationInt(request.Settings, "height") < 1 {
		return BadAuthRequest("Provider Bake-off 必须提供正整数 width 和 height")
	}
	outputCount := positiveEvaluationInt(request.Settings, "outputCount")
	if outputCount < 1 || outputCount > 12 {
		return BadAuthRequest("Provider Bake-off outputCount 必须在 1-12 之间")
	}
	if evaluationString(request.Settings, "quality") == "" || evaluationString(request.Settings, "promptTemplateVersion") == "" {
		return BadAuthRequest("Provider Bake-off settings 必须提供 quality 和 promptTemplateVersion")
	}
	if len(evaluationStringSlice(request.Settings, "referenceAssetIds")) == 0 {
		return BadAuthRequest("Provider Bake-off 至少需要一张 referenceAssetId")
	}
	if len(request.Cases) == 0 || len(request.Candidates) == 0 {
		return BadAuthRequest("Provider Bake-off 至少需要一个 case 和一个候选模型")
	}
	caseIDs := map[string]struct{}{}
	for _, item := range request.Cases {
		caseID := evaluationString(item, "caseId")
		if caseID == "" || item["projectId"] != projectID || evaluationString(item, "fixtureRevision") == "" || evaluationString(item, "productImageId") == "" {
			return BadAuthRequest("Provider Bake-off case 缺少项目、fixture 或商品主图")
		}
		if _, exists := caseIDs[caseID]; exists {
			return BadAuthRequest("Provider Bake-off caseId 必须唯一")
		}
		caseIDs[caseID] = struct{}{}
		sourceRefs := evaluationStringSlice(item, "sourceAssetIds")
		if !containsEvaluationString(sourceRefs, evaluationString(item, "productImageId")) {
			return BadAuthRequest("Provider Bake-off productImageId 必须属于 sourceAssetIds")
		}
	}
	candidateIDs := map[string]struct{}{}
	for _, item := range request.Candidates {
		candidateID := evaluationString(item, "candidateId")
		if candidateID == "" || evaluationString(item, "adapterId") == "" || evaluationString(item, "modelRef") == "" {
			return BadAuthRequest("Provider Bake-off candidate 缺少 candidateId、adapterId 或 modelRef")
		}
		if _, exists := candidateIDs[candidateID]; exists {
			return BadAuthRequest("Provider Bake-off candidateId 必须唯一")
		}
		candidateIDs[candidateID] = struct{}{}
	}
	return nil
}

func validateEcommerceProviderEvaluationAttempt(plan *model.EcommerceProviderEvaluationPlan, request RecordEcommerceProviderEvaluationAttemptRequest) error {
	if request.StartedAt.IsZero() {
		return BadAuthRequest("Provider Bake-off Attempt 必须提供 startedAt")
	}
	if !containsEvaluationID(plan.CasesJSON, "caseId", request.CaseID) || !containsEvaluationID(plan.CandidatesJSON, "candidateId", request.CandidateID) {
		return NotFound("Provider Bake-off case 或候选模型不存在")
	}
	if request.Variant != "basic" && request.Variant != "expert" && request.Variant != "skill" {
		return BadAuthRequest("Provider Bake-off variant 只能是 basic、expert 或 skill")
	}
	if request.Status != "queued" && request.Status != "running" && request.Status != "succeeded" && request.Status != "failed" && request.Status != "cancelled" && request.Status != "needs_you" {
		return BadAuthRequest("Provider Bake-off Attempt status 无效")
	}
	if strings.TrimSpace(request.SettingsFingerprint) == "" || strings.TrimSpace(request.RequestFingerprint) == "" {
		return BadAuthRequest("Provider Bake-off Attempt 必须冻结 settingsFingerprint 和 requestFingerprint")
	}
	if request.LatencyMS < 0 {
		return BadAuthRequest("Provider Bake-off latencyMs 不能为负数")
	}
	if request.CompletedAt != nil && request.CompletedAt.Before(request.StartedAt) {
		return BadAuthRequest("Provider Bake-off completedAt 不能早于 startedAt")
	}
	if request.Status == "succeeded" && len(uniqueNonEmpty(request.ResultRefs)) == 0 {
		return BadAuthRequest("succeeded 的 Provider Bake-off Attempt 必须有 resultRefs")
	}
	if request.Status == "failed" && len(request.Failure) == 0 {
		return BadAuthRequest("failed 的 Provider Bake-off Attempt 必须有 failure")
	}
	return nil
}

func validateEcommerceProviderEvaluationScore(request RecordEcommerceProviderEvaluationScoreRequest) error {
	if strings.TrimSpace(request.EvaluatorRef) == "" {
		return BadAuthRequest("Provider Bake-off evaluatorRef 不能为空")
	}
	for _, key := range []string{"referenceFidelity", "productIdentity", "commercialQuality", "instructionFollowing", "variationAbility", "physicalPlausibility", "aiArtifactSeverity", "apiStability"} {
		value, ok := evaluationNumber(request.Dimensions[key])
		if !ok || value < 1 || value > 5 || value != float64(int(value)) {
			return BadAuthRequest("Provider Bake-off 评分维度必须是 1-5 的整数")
		}
	}
	return nil
}

func validateEcommerceProviderEvaluationPreference(plan *model.EcommerceProviderEvaluationPlan, attempts []model.EcommerceProviderEvaluationAttempt, request RecordEcommerceProviderEvaluationPreferenceRequest) error {
	if !request.Blinded || strings.TrimSpace(request.VoterRef) == "" {
		return BadAuthRequest("Provider Bake-off 盲测必须记录 blinded=true 和 voterRef")
	}
	refs := uniqueNonEmpty(request.ResultRefs)
	if len(refs) < 2 || !containsEvaluationID(plan.CasesJSON, "caseId", request.CaseID) {
		return BadAuthRequest("Provider Bake-off 盲测至少需要同一 case 的两个结果")
	}
	if request.Tie && strings.TrimSpace(request.PreferredResultRef) != "" {
		return BadAuthRequest("平票盲测不能提供 preferredResultRef")
	}
	if !request.Tie && !containsEvaluationString(refs, strings.TrimSpace(request.PreferredResultRef)) {
		return BadAuthRequest("非平票盲测必须选择 resultRefs 中的 preferredResultRef")
	}
	availableResults := map[string]struct{}{}
	for _, attempt := range attempts {
		if attempt.CaseID != strings.TrimSpace(request.CaseID) || attempt.Status != "succeeded" {
			continue
		}
		var resultRefs []string
		if json.Unmarshal([]byte(attempt.ResultRefsJSON), &resultRefs) == nil {
			for _, resultRef := range resultRefs {
				availableResults[resultRef] = struct{}{}
			}
		}
	}
	for _, resultRef := range refs {
		if _, exists := availableResults[resultRef]; !exists {
			return BadAuthRequest("Provider Bake-off 盲测结果必须来自同一 case 的 succeeded Attempt")
		}
	}
	return nil
}

func mapEcommerceProviderEvaluationError(err error) error {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return NotFound("Provider Bake-off 记录不存在")
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return Conflict("Provider Bake-off 记录已存在，请刷新后重试")
	default:
		return err
	}
}

func evaluationFingerprint(skillRef string, settings map[string]any, cases []map[string]any, candidates []map[string]any) (string, error) {
	payload, err := json.Marshal(struct {
		SkillRef   string           `json:"skillRef"`
		Settings   map[string]any   `json:"settings"`
		Cases      []map[string]any `json:"cases"`
		Candidates []map[string]any `json:"candidates"`
	}{strings.TrimSpace(skillRef), settings, cases, candidates})
	if err != nil {
		return "", BadAuthRequest("Provider Bake-off 计划内容无法序列化")
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func evaluationString(value map[string]any, key string) string {
	if value == nil {
		return ""
	}
	raw, exists := value[key]
	if !exists || raw == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(raw))
}

func evaluationStringSlice(value map[string]any, key string) []string {
	items := []string{}
	if raw, ok := value[key].([]any); ok {
		for _, item := range raw {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" {
				items = append(items, text)
			}
		}
	}
	if raw, ok := value[key].([]string); ok {
		return uniqueNonEmpty(raw)
	}
	return uniqueNonEmpty(items)
}

func positiveEvaluationInt(value map[string]any, key string) int {
	number, ok := evaluationNumber(value[key])
	if !ok || number != float64(int(number)) {
		return 0
	}
	return int(number)
}

func evaluationNumber(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int:
		return float64(number), true
	case int64:
		return float64(number), true
	default:
		return 0, false
	}
}

func normalizedEvaluationEvidence(value string) string {
	switch strings.TrimSpace(value) {
	case "recorded", "inferred", "unknown":
		return strings.TrimSpace(value)
	default:
		return "unknown"
	}
}

func nonNilMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func containsEvaluationID(raw string, key string, target string) bool {
	var values []map[string]any
	if json.Unmarshal([]byte(raw), &values) != nil {
		return false
	}
	for _, value := range values {
		if evaluationString(value, key) == strings.TrimSpace(target) {
			return true
		}
	}
	return false
}

func containsEvaluationString(values []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
