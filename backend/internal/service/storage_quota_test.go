package service

import (
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestValidateStructuredStorageQuotaRejectsBytesAndCounts(t *testing.T) {
	policy := defaultRuntimePolicy().Resource
	usage := repository.UserStorageUsage{AssetBytes: megabytes(policy.StructuredDataMB) - 8, AssetCount: policy.AssetCount}
	if err := validateStructuredStorageQuotaWithPolicy(usage, "asset", false, 9, policy); err == nil {
		t.Fatal("validateStructuredStorageQuota() byte error = nil")
	}
	if err := validateStructuredStorageQuotaWithPolicy(usage, "asset", true, 0, policy); err == nil {
		t.Fatal("validateStructuredStorageQuota() count error = nil")
	}
}

func TestValidateStructuredStorageQuotaAllowsReplacementThatShrinksData(t *testing.T) {
	policy := defaultRuntimePolicy().Resource
	usage := repository.UserStorageUsage{AssetBytes: megabytes(policy.StructuredDataMB), AssetCount: policy.AssetCount}
	if err := validateStructuredStorageQuotaWithPolicy(usage, "asset", false, -1, policy); err != nil {
		t.Fatalf("validateStructuredStorageQuota() error = %v", err)
	}
}

func TestValidateTaskStorageQuotaRejectsHistoryGrowth(t *testing.T) {
	policy := defaultRuntimePolicy().Resource
	if err := validateTaskStorageQuotaWithPolicy(repository.UserStorageUsage{TaskCount: policy.TaskCount}, 0, policy); err == nil {
		t.Fatal("validateTaskStorageQuota() count error = nil")
	}
	if err := validateTaskStorageQuotaWithPolicy(repository.UserStorageUsage{TaskBytes: gigabytes(policy.TaskDataGB)}, 1, policy); err == nil {
		t.Fatal("validateTaskStorageQuota() byte error = nil")
	}
	if err := validateAPICallLogQuotaWithPolicy(repository.UserStorageUsage{APICallCount: policy.APICallLogCount}, 0, policy); err == nil {
		t.Fatal("validateAPICallLogQuota() count error = nil")
	}
}

func TestUserStorageUsageCountsPersistedPayloads(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SystemSetting{}, &model.Asset{}, &model.CanvasProject{}, &model.Session{}, &model.Message{}, &model.Task{}, &model.TaskLog{}, &model.Result{}, &model.ApiCallLog{}, &model.TaskTextDelta{}); err != nil {
		t.Fatal(err)
	}
	items := []any{
		&model.Asset{ID: "asset-1", UserID: "user-1", PayloadJSON: "abcd"},
		&model.CanvasProject{ID: "canvas-1", UserID: "user-1", PayloadJSON: "xy"},
		&model.Session{ID: "session-1", UserID: "user-1", Prompt: "p", CanvasSnapshotJSON: "{}"},
		&model.Message{ID: "message-1", UserID: "user-1", SessionID: "session-1", Content: "hi", Payload: "z"},
		&model.Task{ID: "task-1", UserID: "user-1", Prompt: "p", InputJSON: "{}", ResultJSON: "{}"},
		&model.TaskLog{ID: "log-1", UserID: "user-1", TaskID: "task-1", Message: "m", Payload: "p"},
		&model.Result{ID: "result-1", UserID: "user-1", TaskID: "task-1", URL: "u", Payload: "r"},
		&model.ApiCallLog{ID: "api-log-1", UserID: "user-1", Path: "p", Model: "m", ProviderRequestID: "i", Error: "e", UpstreamURL: "u"},
	}
	for _, item := range items {
		if err := db.Create(item).Error; err != nil {
			t.Fatal(err)
		}
	}
	usage, err := repository.New(db).UserStorageUsage("user-1")
	if err != nil {
		t.Fatal(err)
	}
	if usage.AssetCount != 1 || usage.AssetBytes != 4 || usage.CanvasCount != 1 || usage.CanvasBytes != 2 || usage.SessionCount != 1 || usage.SessionBytes != 6 || usage.TaskCount != 1 || usage.TaskBytes != 14 || usage.APICallCount != 1 {
		t.Fatalf("UserStorageUsage() = %#v", usage)
	}
}

func TestSaveTaskCompletionPersistsRelatedRowsTogether(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SystemSetting{}, &model.Asset{}, &model.CanvasProject{}, &model.Session{}, &model.Message{}, &model.Task{}, &model.TaskLog{}, &model.Result{}, &model.ApiCallLog{}, &model.TaskTextDelta{}); err != nil {
		t.Fatal(err)
	}
	session := model.Session{ID: "session-1", UserID: "user-1", Status: model.SessionStatusActive}
	task := model.Task{ID: "task-1", UserID: "user-1", SessionID: session.ID, Status: model.TaskStatusRunning, InputJSON: `{"mode":"text"}`}
	if err := db.Create(&session).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{repo: repository.New(db)}
	if err := svc.saveTaskCompletionWithinStorageQuota(&task, []byte(`{"ok":true}`), []byte(`[{"op":"add"}]`), true); err != nil {
		t.Fatal(err)
	}
	var messageCount int64
	var resultCount int64
	if err := db.Model(&model.Message{}).Count(&messageCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Result{}).Count(&resultCount).Error; err != nil {
		t.Fatal(err)
	}
	if task.Status != model.TaskStatusSucceeded || messageCount != 1 || resultCount != 2 {
		t.Fatalf("completion = status:%s messages:%d results:%d", task.Status, messageCount, resultCount)
	}
}

func TestSaveEcommerceTaskCompletionCreatesResultWithoutSession(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.SystemSetting{}, &model.Asset{}, &model.CanvasProject{}, &model.Session{}, &model.Message{},
		&model.Task{}, &model.TaskLog{}, &model.Result{}, &model.ApiCallLog{}, &model.TaskTextDelta{},
		&model.EcommerceProductionRun{}, &model.EcommerceProductionSlot{}, &model.EcommerceProductionAttempt{}, &model.EcommerceArtifact{},
	); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	run := model.EcommerceProductionRun{
		ID: "ecommerce-run-1", UserID: "user-1", ProjectID: "project-1", Status: "generating",
		OutputCount: 1, CreatedAt: now, UpdatedAt: now,
	}
	slot := model.EcommerceProductionSlot{
		ID: "ecommerce-slot-1", UserID: run.UserID, ProjectID: run.ProjectID, RunID: run.ID,
		Position: 1, Role: "hero", Status: "running", QAStatus: "PENDING",
		ActiveAttemptID: "ecommerce-attempt-1", ActiveTaskID: "ecommerce-task-1", CreatedAt: now, UpdatedAt: now,
	}
	attempt := model.EcommerceProductionAttempt{
		ID: "ecommerce-attempt-1", UserID: run.UserID, ProjectID: run.ProjectID, RunID: run.ID, SlotID: slot.ID,
		AttemptNumber: 1, Kind: "initial", Status: "running", TaskID: "ecommerce-task-1", CreatedAt: now, UpdatedAt: now,
	}
	task := model.Task{
		ID: "ecommerce-task-1", UserID: run.UserID, ProjectID: run.ProjectID, DomainProjectID: run.ProjectID,
		Type: "canvas_image", Status: model.TaskStatusRunning, Provider: model.TaskProviderEcommerce,
		InputJSON: `{"mode":"image"}`, CreatedAt: now, UpdatedAt: now,
	}
	for _, value := range []any{&run, &slot, &attempt, &task} {
		if err := db.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}

	svc := &Service{repo: repository.New(db)}
	payload := []byte(`{"mode":"image","images":[{"url":"/api/resources/generated-1/file","width":1024,"height":1824}]}`)
	if err := svc.saveTaskCompletionWithinStorageQuota(&task, payload, nil, false); err != nil {
		t.Fatalf("saveTaskCompletionWithinStorageQuota() error = %v", err)
	}

	var result model.Result
	if err := db.Where("task_id = ?", task.ID).First(&result).Error; err != nil {
		t.Fatal(err)
	}
	if result.Kind != model.ResultKindEcommerceAsset {
		t.Fatalf("result kind = %q, want %q", result.Kind, model.ResultKindEcommerceAsset)
	}
	var savedAttempt model.EcommerceProductionAttempt
	if err := db.First(&savedAttempt, "id = ?", attempt.ID).Error; err != nil {
		t.Fatal(err)
	}
	if savedAttempt.Status != "succeeded" || savedAttempt.ResultID != result.ID {
		t.Fatalf("attempt completion = status:%s result:%s", savedAttempt.Status, savedAttempt.ResultID)
	}
	var savedSlot model.EcommerceProductionSlot
	if err := db.First(&savedSlot, "id = ?", slot.ID).Error; err != nil {
		t.Fatal(err)
	}
	if savedSlot.Status != "qa" || savedSlot.ResultPayloadJSON == "" || savedSlot.ResultURL != "/api/resources/generated-1/file" {
		t.Fatalf("slot completion = status:%s url:%q payload:%q", savedSlot.Status, savedSlot.ResultURL, savedSlot.ResultPayloadJSON)
	}
}
