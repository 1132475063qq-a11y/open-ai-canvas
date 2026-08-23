package service

import (
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEcommerceProviderEvaluationLifecycleIsDurableAndIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+newID()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open evaluation database: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Project{}, &model.EcommerceProviderEvaluationPlan{}, &model.EcommerceProviderEvaluationAttempt{},
		&model.EcommerceProviderEvaluationScore{}, &model.EcommerceProviderEvaluationPreference{},
	); err != nil {
		t.Fatalf("migrate evaluation database: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	project := model.Project{ID: "project-evaluation", UserID: "user-evaluation", Name: "Provider Evaluation", Type: model.ProjectTypeEcommerce, Status: model.ProjectStatusActive, Revision: 1, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&project).Error; err != nil {
		t.Fatalf("create evaluation project: %v", err)
	}
	svc := New(repository.New(db), t.TempDir())
	req := CreateEcommerceProviderEvaluationPlanRequest{
		IdempotencyKey: "evaluation-plan-key",
		SkillRef:       "ecommerce/top-wear/v2",
		Settings: map[string]any{
			"mode": "MODEL_INTERACTION", "width": 2160, "height": 3840, "quality": "high", "outputCount": 1,
			"referenceAssetIds": []string{"asset-product"}, "promptTemplateVersion": "skill-v2", "seedPolicy": "unavailable",
		},
		Cases:      []map[string]any{{"caseId": "case-socks", "projectId": project.ID, "fixtureRevision": "fixture-1", "sourceAssetIds": []string{"asset-product"}, "productImageId": "asset-product", "userGoal": "natural product shot"}},
		Candidates: []map[string]any{{"candidateId": "candidate-gpt-image-2", "adapterId": "openai-image", "modelRef": "gpt-image-2", "availability": "available"}},
	}
	created, err := svc.CreateProjectEcommerceProviderEvaluationPlan(project.UserID, project.ID, req)
	if err != nil {
		t.Fatalf("create provider evaluation plan: %v", err)
	}
	if created.Idempotent || created.Plan.Plan.ID == "" || created.Plan.Plan.Status != EcommerceProviderEvaluationStatusDraft {
		t.Fatalf("unexpected plan create result: %+v", created)
	}
	replayed, err := svc.CreateProjectEcommerceProviderEvaluationPlan(project.UserID, project.ID, req)
	if err != nil || !replayed.Idempotent || replayed.Plan.Plan.ID != created.Plan.Plan.ID {
		t.Fatalf("idempotent plan replay failed: result=%+v err=%v", replayed, err)
	}

	started := now.Add(time.Second)
	completed := started.Add(1500 * time.Millisecond)
	attemptReq := RecordEcommerceProviderEvaluationAttemptRequest{
		IdempotencyKey: "evaluation-attempt-key", CaseID: "case-socks", CandidateID: "candidate-gpt-image-2", Variant: "skill", Status: "succeeded",
		SettingsFingerprint: "settings-fingerprint", RequestFingerprint: "request-fingerprint", ResultRefs: []string{"result-a", "result-b"},
		StartedAt: started, CompletedAt: &completed, LatencyMS: 1500, Cost: map[string]any{"status": "recorded", "amount": 1, "currency": "credit"}, Evidence: "recorded",
	}
	view, idempotent, err := svc.RecordProjectEcommerceProviderEvaluationAttempt(project.UserID, project.ID, created.Plan.Plan.ID, attemptReq)
	if err != nil || idempotent || len(view.Attempts) != 1 || view.Summary.HasRecordedResults == false {
		t.Fatalf("record evaluation attempt failed: idempotent=%v view=%+v err=%v", idempotent, view, err)
	}
	replayedAttempt, idempotent, err := svc.RecordProjectEcommerceProviderEvaluationAttempt(project.UserID, project.ID, created.Plan.Plan.ID, attemptReq)
	if err != nil || !idempotent || len(replayedAttempt.Attempts) != 1 {
		t.Fatalf("idempotent attempt replay failed: idempotent=%v err=%v", idempotent, err)
	}

	dimensions := map[string]any{"referenceFidelity": 5, "productIdentity": 5, "commercialQuality": 4, "instructionFollowing": 4, "variationAbility": 3, "physicalPlausibility": 4, "aiArtifactSeverity": 4, "apiStability": 5}
	view, _, err = svc.RecordProjectEcommerceProviderEvaluationScore(project.UserID, project.ID, created.Plan.Plan.ID, RecordEcommerceProviderEvaluationScoreRequest{IdempotencyKey: "evaluation-score-key", AttemptID: view.Attempts[0].Attempt.ID, EvaluatorRef: "human:reviewer-1", Dimensions: dimensions, Evidence: "recorded", RecordedAt: completed})
	if err != nil || len(view.Scores) != 1 || view.Summary.CandidateSummaries[0].ScoreCount != 1 || view.Summary.CandidateSummaries[0].KnownCost["currency"] != "credit" {
		t.Fatalf("record evaluation score failed: view=%+v err=%v", view, err)
	}
	view, _, err = svc.RecordProjectEcommerceProviderEvaluationPreference(project.UserID, project.ID, created.Plan.Plan.ID, RecordEcommerceProviderEvaluationPreferenceRequest{
		IdempotencyKey: "evaluation-preference-key", CaseID: "case-socks", ResultRefs: []string{"result-a", "result-b"}, PreferredResultRef: "result-a", VoterRef: "human:reviewer-1", Blinded: true, Evidence: "recorded", RecordedAt: completed,
	})
	if err != nil || len(view.Preferences) != 1 {
		t.Fatalf("record blind preference failed: view=%+v err=%v", view, err)
	}
	view, err = svc.RecordProjectEcommerceProviderEvaluationDecision(project.UserID, project.ID, created.Plan.Plan.ID, RecordEcommerceProviderEvaluationDecisionRequest{Decision: EcommerceProviderEvaluationStatusGo, ReviewerRef: "human:reviewer-1", Rationale: "stable commercial result", NextAction: "use candidate in the next golden path", Evidence: "recorded"})
	if err != nil || view.Plan.Decision != EcommerceProviderEvaluationStatusGo || view.Summary.Decision != EcommerceProviderEvaluationStatusGo {
		t.Fatalf("record evaluation decision failed: view=%+v err=%v", view, err)
	}
}

func TestEcommerceProviderEvaluationRejectsInvalidScore(t *testing.T) {
	if err := validateEcommerceProviderEvaluationScore(RecordEcommerceProviderEvaluationScoreRequest{EvaluatorRef: "reviewer", Dimensions: map[string]any{"referenceFidelity": 6}}); err == nil {
		t.Fatal("out-of-range evaluation score was accepted")
	}
}
