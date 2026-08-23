package service

import (
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNormalizeEcommerceQAReviewEnforcesDimensionGate(t *testing.T) {
	run := model.EcommerceProductionRun{Kernel: EcommerceKernelModelInteraction, OutputCount: 6}
	req := validEcommerceQARequest(run, "qa-review-0001", "attempt-1", "result-1")

	req.Dimensions[0].Score = intPointer(3)
	req.Dimensions[0].Verdict = EcommerceQAVerdictUncertain
	if _, err := normalizeEcommerceQAReview(run, req); err == nil {
		t.Fatal("PASS review accepted an UNCERTAIN product-fidelity dimension")
	}

	req = validEcommerceQARequest(run, "qa-review-0002", "attempt-1", "result-1")
	req.Dimensions[0].Score = nil
	req.Dimensions[0].Verdict = EcommerceQAVerdictNotApplicable
	if _, err := normalizeEcommerceQAReview(run, req); err == nil {
		t.Fatal("required product-fidelity dimension accepted NOT_APPLICABLE")
	}

	req = validEcommerceQARequest(run, "qa-review-0003", "attempt-1", "result-1")
	req.Dimensions = req.Dimensions[:len(req.Dimensions)-1]
	if _, err := normalizeEcommerceQAReview(run, req); err == nil {
		t.Fatal("review accepted an incomplete dimension set")
	}
}

func TestNormalizeEcommerceQAReviewAllowsStillLifeNotApplicableDimensions(t *testing.T) {
	run := model.EcommerceProductionRun{Kernel: EcommerceKernelStillLife, OutputCount: 1}
	req := validEcommerceQARequest(run, "qa-review-still-life", "attempt-1", "result-1")
	normalized, err := normalizeEcommerceQAReview(run, req)
	if err != nil {
		t.Fatalf("normalize still-life QA review: %v", err)
	}
	for _, dimension := range normalized.Dimensions {
		if (dimension.Key == "model_identity" || dimension.Key == "anatomy_contact" || dimension.Key == "series_consistency") && dimension.Verdict != EcommerceQAVerdictNotApplicable {
			t.Fatalf("dimension %s should be NOT_APPLICABLE, got %s", dimension.Key, dimension.Verdict)
		}
	}
}

func TestReviewProjectEcommerceSlotPersistsHistoryAndReplaysIdempotently(t *testing.T) {
	svc, db, project, run, slot, attempt := newEcommerceQAService(t)
	req := validEcommerceQARequest(run, "qa-review-integration-0001", attempt.ID, attempt.ResultID)

	view, err := svc.ReviewProjectEcommerceSlot(project.UserID, project.ID, run.ID, slot.ID, req)
	if err != nil {
		t.Fatalf("review ecommerce slot: %v", err)
	}
	if len(view.Slots) != 1 || len(view.Slots[0].Reviews) != 1 {
		t.Fatalf("review history not projected: %+v", view.Slots)
	}
	review := view.Slots[0].Reviews[0]
	if review.SchemaVersion != ecommerceQAReportSchemaVersion || review.ResultID != attempt.ResultID || review.RuntimeEvidence.APIStabilityScore != 5 {
		t.Fatalf("unexpected structured QA review: %+v", review)
	}
	if !view.Slots[0].Slot.Accepted || view.Run.Status != EcommerceRunStatusReady {
		t.Fatalf("PASS review did not accept the slot: slot=%+v run=%+v", view.Slots[0].Slot, view.Run)
	}

	var artifactCount int64
	if err := db.Model(&model.EcommerceArtifact{}).Where("artifact_type = ?", EcommerceArtifactTypeQAReport).Count(&artifactCount).Error; err != nil {
		t.Fatalf("count QA artifacts: %v", err)
	}
	var updatedProject model.Project
	if err := db.First(&updatedProject, "id = ?", project.ID).Error; err != nil {
		t.Fatalf("load updated project: %v", err)
	}
	if artifactCount != 1 || updatedProject.Revision != 2 {
		t.Fatalf("first review did not append exactly one project revision: artifacts=%d revision=%d", artifactCount, updatedProject.Revision)
	}

	if _, err := svc.ReviewProjectEcommerceSlot(project.UserID, project.ID, run.ID, slot.ID, req); err != nil {
		t.Fatalf("idempotent QA replay failed: %v", err)
	}
	if err := db.Model(&model.EcommerceArtifact{}).Where("artifact_type = ?", EcommerceArtifactTypeQAReport).Count(&artifactCount).Error; err != nil {
		t.Fatalf("recount QA artifacts: %v", err)
	}
	if err := db.First(&updatedProject, "id = ?", project.ID).Error; err != nil {
		t.Fatalf("reload updated project: %v", err)
	}
	if artifactCount != 1 || updatedProject.Revision != 2 {
		t.Fatalf("replay created another fact: artifacts=%d revision=%d", artifactCount, updatedProject.Revision)
	}

	req.Note = "different review content"
	if _, err := svc.ReviewProjectEcommerceSlot(project.UserID, project.ID, run.ID, slot.ID, req); authStatus(err) != 409 {
		t.Fatalf("reused reviewId should conflict, got %v", err)
	}
}

func TestReviewProjectEcommerceSlotRejectsStaleResultIdentity(t *testing.T) {
	svc, _, project, run, slot, attempt := newEcommerceQAService(t)
	req := validEcommerceQARequest(run, "qa-review-stale-result", attempt.ID, "stale-result")
	if _, err := svc.ReviewProjectEcommerceSlot(project.UserID, project.ID, run.ID, slot.ID, req); authStatus(err) != 409 {
		t.Fatalf("stale result identity should conflict, got %v", err)
	}
}

func newEcommerceQAService(t *testing.T) (*Service, *gorm.DB, model.Project, model.EcommerceProductionRun, model.EcommerceProductionSlot, model.EcommerceProductionAttempt) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+newID()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open ecommerce QA database: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Project{}, &model.ModelChannel{}, &model.ChannelModel{}, &model.BillingOrder{},
		&model.Task{}, &model.EcommerceArtifact{}, &model.EcommerceProductionRun{},
		&model.EcommerceProductionSlot{}, &model.EcommerceProductionAttempt{},
	); err != nil {
		t.Fatalf("migrate ecommerce QA database: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	project := model.Project{
		ID: "project-ecommerce-qa", UserID: "user-ecommerce-qa", Name: "Ecommerce QA",
		Type: model.ProjectTypeEcommerce, Status: model.ProjectStatusActive, AspectRatio: "9:16",
		Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	run := model.EcommerceProductionRun{
		ID: "run-ecommerce-qa", UserID: project.UserID, ProjectID: project.ID, IdempotencyKey: "run-key",
		Status: EcommerceRunStatusQA, Kernel: EcommerceKernelModelInteraction, Category: "apparel",
		PresetID: "top-wear", PresetVersion: 1, AspectRatio: "9:16", Resolution: "4k", OutputCount: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	slot := model.EcommerceProductionSlot{
		ID: "slot-ecommerce-qa", UserID: project.UserID, ProjectID: project.ID, RunID: run.ID,
		Position: 1, Role: "hero", Title: "Hero", Status: EcommerceSlotStatusQA,
		QAStatus: EcommerceQAStatusUncertain, QAIssuesJSON: `["visual_review_required"]`,
		ActiveAttemptID: "attempt-ecommerce-qa", ActiveTaskID: "task-ecommerce-qa",
		ResultURL: "https://example.test/result.png", GeneratedAssetID: "generated-asset-ecommerce-qa",
		CreatedAt: now, UpdatedAt: now,
	}
	startedAt := now.Add(-2 * time.Second)
	completedAt := now
	task := model.Task{
		ID: slot.ActiveTaskID, UserID: project.UserID, DomainProjectID: project.ID,
		Type: "image", Status: model.TaskStatusSucceeded, Provider: model.TaskProviderEcommerce,
		ProviderRequestID: "provider-request-qa", StartedAt: &startedAt, CompletedAt: &completedAt,
		CreatedAt: startedAt, UpdatedAt: now,
	}
	attempt := model.EcommerceProductionAttempt{
		ID: slot.ActiveAttemptID, UserID: project.UserID, ProjectID: project.ID, RunID: run.ID, SlotID: slot.ID,
		AttemptNumber: 1, Kind: "initial", Status: "succeeded", TaskID: task.ID,
		ProviderRequestID: task.ProviderRequestID, ResultID: "result-ecommerce-qa",
		GeneratedAssetArtifactID: slot.GeneratedAssetID, ResultURL: slot.ResultURL,
		StartedAt: &startedAt, CompletedAt: &completedAt, CreatedAt: startedAt, UpdatedAt: now,
	}
	generated := model.EcommerceArtifact{
		ID: slot.GeneratedAssetID, ProjectID: project.ID, ArtifactKey: "run:" + run.ID + ":slot:" + slot.ID + ":generated",
		ArtifactType: EcommerceArtifactTypeGeneratedAsset, SchemaVersion: 1, Revision: 1,
		Lifecycle: "finalized", Evidence: "recorded", PayloadJSON: `{"resultId":"result-ecommerce-qa"}`,
		SourceRefsJSON: "[]", AuthorityRefsJSON: "[]", CreatedAt: now, UpdatedAt: now,
	}
	for _, value := range []any{&project, &run, &slot, &task, &attempt, &generated} {
		if err := db.Create(value).Error; err != nil {
			t.Fatalf("create ecommerce QA fixture %T: %v", value, err)
		}
	}
	return New(repository.New(db), t.TempDir()), db, project, run, slot, attempt
}

func validEcommerceQARequest(run model.EcommerceProductionRun, reviewID string, attemptID string, resultID string) ReviewEcommerceSlotRequest {
	dimensions := make([]EcommerceQADimensionAssessment, 0, len(ecommerceQADimensionDefinitions))
	for _, definition := range ecommerceQADimensionDefinitions {
		assessment := EcommerceQADimensionAssessment{Key: definition.Key}
		if ecommerceQADimensionRequired(run, definition.Key) || definition.Key == "logo_text" {
			assessment.Score = intPointer(4)
			assessment.Verdict = EcommerceQAVerdictPass
		} else {
			assessment.Verdict = EcommerceQAVerdictNotApplicable
		}
		dimensions = append(dimensions, assessment)
	}
	return ReviewEcommerceSlotRequest{
		ReviewID: reviewID, AttemptID: attemptID, ResultID: resultID,
		Decision: EcommerceQAStatusPass, Action: "accept", Dimensions: dimensions,
	}
}

func intPointer(value int) *int { return &value }

func TestEcommerceQAReplayConflictMapsToHTTPConflict(t *testing.T) {
	if status := authStatus(mapEcommerceRuntimeError(repository.ErrEcommerceQAReviewConflict)); status != 409 {
		t.Fatalf("QA review conflict status = %d, want 409", status)
	}
}
