package service

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateEcommerceAgentRunIsDurableAndIdempotent(t *testing.T) {
	svc, repo, _, project := newEcommerceAgentRuntimeTestService(t)
	request := CreateEcommerceAgentRunRequest{
		Objective:       "分析授权商品并建立商品 DNA",
		ProductAssetIDs: []string{"product-asset-1"},
		ProductFacts:    map[string]any{"color": "midnight blue", "material": "cotton"},
	}
	created, err := svc.CreateEcommerceAgentRun(project.UserID, project.ID, "ecommerce-ir01-001", request)
	if err != nil {
		t.Fatalf("CreateEcommerceAgentRun(): %v", err)
	}
	if created.Idempotent || created.Detail.Run.Domain != "ecommerce" || created.Detail.Run.RegistryID != "ecommerce-agent-team" ||
		created.Detail.Run.RegistryVersion != "0.1.0" || len(created.Detail.Run.RegistryDigest) != 64 || created.Detail.Run.IntentRouteID != "IR-01" {
		t.Fatalf("unexpected Ecommerce IR-01 Run: %#v", created.Detail.Run)
	}
	if len(created.Detail.Steps) != 1 || created.Detail.Steps[0].AgentID != ecommerceAgentRuntimeAgentID ||
		created.Detail.Steps[0].SkillIDsJSON != `["product-intelligence"]` || created.Detail.Steps[0].Status != model.AgentStepStatusReady ||
		created.Detail.Steps[0].ExpectedOutputArtifactTypesJSON != `["product_dna"]` {
		t.Fatalf("unexpected IR-01 Step: %#v", created.Detail.Steps)
	}
	if len(created.Detail.Events) != 2 || created.Detail.Events[0].Sequence != 1 || created.Detail.Events[1].Sequence != 2 ||
		created.Detail.Events[1].EventType != "run.planned" {
		t.Fatalf("initial Ecommerce runtime events are incomplete: %#v", created.Detail.Events)
	}

	replay, err := svc.CreateEcommerceAgentRun(project.UserID, project.ID, "ecommerce-ir01-001", request)
	if err != nil {
		t.Fatalf("idempotent CreateEcommerceAgentRun(): %v", err)
	}
	if !replay.Idempotent || replay.Detail.Run.ID != created.Detail.Run.ID {
		t.Fatalf("idempotent replay changed Run: first=%s replay=%#v", created.Detail.Run.ID, replay)
	}
	request.ProductFacts["color"] = "red"
	if _, err := svc.CreateEcommerceAgentRun(project.UserID, project.ID, "ecommerce-ir01-001", request); authStatus(err) != 409 {
		t.Fatalf("reused idempotency key with changed input error = %v, want 409", err)
	}

	detail, err := svc.EcommerceAgentRunDetail(project.UserID, project.ID, created.Detail.Run.ID)
	if err != nil {
		t.Fatalf("EcommerceAgentRunDetail(): %v", err)
	}
	if detail.Run.ID != created.Detail.Run.ID || len(detail.RoutingDecisions) != 1 || len(detail.Attempts) != 0 {
		t.Fatalf("read projection is incomplete: %#v", detail)
	}
	_ = repo
}

func TestProcessNextEcommerceAgentStepPersistsDeterministicProductDNAWithoutProvider(t *testing.T) {
	svc, repo, db, project := newEcommerceAgentRuntimeTestService(t)
	created, err := svc.CreateEcommerceAgentRun(project.UserID, project.ID, "ecommerce-ir01-002", CreateEcommerceAgentRunRequest{
		Objective:       "识别商品事实",
		ProductAssetIDs: []string{"asset-z", "asset-a"},
		ProductFacts:    map[string]any{"color": "black", "logo": "recorded"},
	})
	if err != nil {
		t.Fatalf("CreateEcommerceAgentRun(): %v", err)
	}
	processed, err := svc.ProcessNextEcommerceAgentStep()
	if err != nil || !processed {
		t.Fatalf("ProcessNextEcommerceAgentStep() = %v, %v", processed, err)
	}
	detail, err := repo.AgentRuntimeDetailForUser(project.UserID, created.Detail.Run.ID)
	if err != nil {
		t.Fatalf("load executed Ecommerce Run: %v", err)
	}
	if detail.Run.Status != model.AgentRunStatusCompleted || len(detail.Steps) != 1 || detail.Steps[0].Status != model.AgentStepStatusCompleted ||
		len(detail.Attempts) != 1 || detail.Attempts[0].Status != model.AgentAttemptStatusSucceeded {
		t.Fatalf("IR-01 execution did not complete durably: %#v", detail)
	}
	attempt := detail.Attempts[0]
	if attempt.Executor != ecommerceAgentRuntimeExecutor || attempt.ModelRef != ecommerceAgentRuntimeModelRef || attempt.TaskID == "" ||
		!strings.HasPrefix(attempt.TaskID, "deterministic:") || !strings.Contains(attempt.RequestJSON, `"provider":"none"`) ||
		strings.Contains(attempt.RequestJSON, "logicalModelId") {
		t.Fatalf("deterministic Attempt identity is invalid: %#v", attempt)
	}
	if len(detail.Artifacts) != 1 || detail.Artifacts[0].ArtifactType != ecommerceAgentRuntimeOutputType || len(detail.ArtifactRevisions) != 1 {
		t.Fatalf("ProductDNA Artifact was not persisted: artifacts=%#v revisions=%#v", detail.Artifacts, detail.ArtifactRevisions)
	}
	revision := detail.ArtifactRevisions[0]
	if revision.Status != model.ProductionArtifactStatusReview || revision.SourceRunID != detail.Run.ID || revision.SourceAttemptID != attempt.ID || revision.CreatedByID != ecommerceAgentRuntimeAgentID {
		t.Fatalf("ProductDNA revision evidence is incomplete: %#v", revision)
	}
	var payload struct {
		Content struct {
			Evidence        string   `json:"evidence"`
			ProductAssetIDs []string `json:"productAssetIds"`
			Unknowns        []string `json:"unknowns"`
		} `json:"content"`
	}
	if err := json.Unmarshal([]byte(revision.ContentJSON), &payload); err != nil {
		t.Fatalf("decode ProductDNA revision: %v", err)
	}
	if payload.Content.Evidence != "recorded" || len(payload.Content.ProductAssetIDs) != 2 || len(payload.Content.Unknowns) == 0 {
		t.Fatalf("ProductDNA evidence/unknown policy is incomplete: %#v", payload)
	}
	var taskCount int64
	if err := db.Model(&model.Task{}).Where("user_id = ?", project.UserID).Count(&taskCount).Error; err != nil {
		t.Fatalf("count provider Tasks: %v", err)
	}
	if taskCount != 0 {
		t.Fatalf("provider-free executor created %d Task rows", taskCount)
	}
	processed, err = svc.ProcessNextEcommerceAgentStep()
	if err != nil || processed {
		t.Fatalf("completed Ecommerce queue should be empty, got processed=%v error=%v", processed, err)
	}
}

func TestProcessNextEcommerceAgentStepFailsDurablyOnCorruptInput(t *testing.T) {
	svc, repo, db, project := newEcommerceAgentRuntimeTestService(t)
	created, err := svc.CreateEcommerceAgentRun(project.UserID, project.ID, "ecommerce-ir01-003", CreateEcommerceAgentRunRequest{Objective: "识别商品事实"})
	if err != nil {
		t.Fatalf("CreateEcommerceAgentRun(): %v", err)
	}
	if err := db.Model(&model.AgentRuntimeRun{}).Where("id = ?", created.Detail.Run.ID).Update("input_json", "{corrupt").Error; err != nil {
		t.Fatalf("corrupt test input: %v", err)
	}
	processed, processErr := svc.ProcessNextEcommerceAgentStep()
	if !processed || processErr == nil {
		t.Fatalf("corrupt input process result = %v, %v", processed, processErr)
	}
	detail, err := repo.AgentRuntimeDetailForUser(project.UserID, created.Detail.Run.ID)
	if err != nil {
		t.Fatalf("load failed Ecommerce Run: %v", err)
	}
	if detail.Run.Status != model.AgentRunStatusFailed || detail.Run.FailureCode != "ecommerce_product_intelligence_failed" ||
		detail.Steps[0].Status != model.AgentStepStatusFailed || detail.Attempts[0].Status != model.AgentAttemptStatusFailed ||
		detail.Attempts[0].FailureCode != "ecommerce_product_intelligence_failed" || detail.Events[len(detail.Events)-1].EventType != "attempt.failed" {
		t.Fatalf("corrupt input did not leave a complete failure record: %#v", detail)
	}
}

func TestProcessNextEcommerceAgentStepFencesRegistryDrift(t *testing.T) {
	svc, repo, db, project := newEcommerceAgentRuntimeTestService(t)
	created, err := svc.CreateEcommerceAgentRun(project.UserID, project.ID, "ecommerce-ir01-drift", CreateEcommerceAgentRunRequest{Objective: "识别商品事实"})
	if err != nil {
		t.Fatalf("CreateEcommerceAgentRun(): %v", err)
	}
	if err := db.Model(&model.AgentRuntimeRun{}).Where("id = ?", created.Detail.Run.ID).Update("registry_digest", "stale-registry").Error; err != nil {
		t.Fatalf("drift test mutation: %v", err)
	}
	processed, processErr := svc.ProcessNextEcommerceAgentStep()
	if !processed || processErr == nil || authStatus(processErr) != 409 {
		t.Fatalf("registry drift process result = %v, %v; want processed=true and 409", processed, processErr)
	}
	detail, err := repo.AgentRuntimeDetailForUser(project.UserID, created.Detail.Run.ID)
	if err != nil {
		t.Fatalf("load drifted Ecommerce Run: %v", err)
	}
	if detail.Run.Status != model.AgentRunStatusFailed || detail.Run.FailureCode != "ecommerce_product_intelligence_failed" ||
		detail.Steps[0].Status != model.AgentStepStatusFailed || detail.Attempts[0].Status != model.AgentAttemptStatusFailed {
		t.Fatalf("registry drift did not leave a fenced failure fact: %#v", detail)
	}
}

func newEcommerceAgentRuntimeTestService(t *testing.T) (*Service, *repository.Repository, *gorm.DB, model.Project) {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "ecommerce-agent-runtime.db")
	db, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open Ecommerce Agent Runtime database: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Project{}, &model.Task{},
		&model.AgentRuntimeRun{}, &model.AgentRuntimeStep{}, &model.AgentRuntimeAttempt{},
		&model.AgentRoutingDecision{}, &model.AgentHumanDecision{}, &model.AgentRuntimeEvent{},
		&model.AgentHandoffTrigger{}, &model.ProductionArtifact{}, &model.ProductionArtifactRevision{},
	); err != nil {
		t.Fatalf("migrate Ecommerce Agent Runtime database: %v", err)
	}
	now := time.Now().UTC()
	project := model.Project{ID: "project-ecommerce-agent", UserID: "user-ecommerce-agent", Name: "Ecommerce Agent Project", Type: model.ProjectTypeEcommerce, Status: model.ProjectStatusActive, AspectRatio: "1:1", Revision: 1, CreatedAt: now, UpdatedAt: now}
	repo := repository.New(db)
	if err := repo.CreateProject(&project); err != nil {
		t.Fatalf("create Ecommerce project: %v", err)
	}
	svc := New(repo, t.TempDir())
	if err := svc.validateEcommerceAgentRegistry(); err != nil {
		t.Fatalf("validate Ecommerce registry: %v", err)
	}
	return svc, repo, db, project
}
