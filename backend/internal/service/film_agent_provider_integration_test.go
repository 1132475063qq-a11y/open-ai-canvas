package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"infinite-canvas/backend/internal/database"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type filmAgentProcessResult struct {
	processed bool
	err       error
}

func TestQueuedFilmAgentExecutorRunsThroughLogicalModelTaskProviderAndArtifact(t *testing.T) {
	t.Setenv("CANVAS_ALLOW_PRIVATE_UPSTREAMS", "false")
	providerOutput := `{"schemaVersion":1,"summary":"Provider generated screenplay","artifacts":[{"type":"script","title":"Last Train","content":{"scenes":[{"id":"scene-1"}]},"contentText":"INT. STATION - NIGHT"}]}`
	var requestCount atomic.Int32
	var requestMu sync.Mutex
	var providerRequest map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer film-provider-key" {
			http.Error(w, "invalid authorization", http.StatusUnauthorized)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		requestMu.Lock()
		providerRequest = body
		requestMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-film-agent-1",
			"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": providerOutput}}},
			"usage":   map[string]any{"prompt_tokens": 120, "completion_tokens": 80, "total_tokens": 200},
		})
	}))
	defer upstream.Close()

	svc, repo, db, project, logicalModel, channelModel, logicalRoute := newFilmAgentProviderIntegrationService(t, upstream.URL+"/v1")
	queued, ok := svc.filmAgentExecutor.(*queuedFilmAgentExecutor)
	if !ok {
		t.Fatalf("default Film executor type = %T", svc.filmAgentExecutor)
	}
	queued.pollInterval = 5 * time.Millisecond
	created, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-provider-e2e", CreateFilmAgentRunRequest{
		Objective: "写一个原创短片故事", IntentRouteID: "IR-01", LogicalModelID: logicalModel.ID,
		Input: map[string]any{"premise": "午夜地铁站里时间突然停止"},
	})
	if err != nil {
		t.Fatalf("CreateFilmAgentRun(): %v", err)
	}

	filmDone := make(chan filmAgentProcessResult, 1)
	go func() {
		processed, processErr := svc.ProcessNextFilmAgentStep()
		filmDone <- filmAgentProcessResult{processed: processed, err: processErr}
	}()
	task := waitForFilmAgentProviderTask(t, db, filmDone)
	if task.LogicalModelID != logicalModel.ID || task.LogicalModelRevisionID != logicalModel.ActiveRevisionID ||
		task.RouteID != logicalRoute.ID || task.ChannelModelID != channelModel.ID || task.Status != model.TaskStatusQueued {
		t.Fatalf("Film provider Task did not preserve logical routing identity: %#v", task)
	}
	if err := svc.ProcessNextTask(); err != nil {
		t.Fatalf("ProcessNextTask(): %v", err)
	}
	select {
	case result := <-filmDone:
		if !result.processed || result.err != nil {
			t.Fatalf("ProcessNextFilmAgentStep() = %v, %v", result.processed, result.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Film Agent executor did not observe the completed provider Task")
	}

	persistedTask, err := repo.Task(task.ID)
	if err != nil {
		t.Fatalf("load provider Task: %v", err)
	}
	if persistedTask.Status != model.TaskStatusSucceeded || persistedTask.Type != "canvas_text_film_agent" ||
		persistedTask.Operation != "film_agent_skill" || persistedTask.Attempts != 1 || persistedTask.ResultJSON == "" {
		t.Fatalf("provider Task terminal record is invalid: %#v", persistedTask)
	}
	var taskResult map[string]any
	if err := json.Unmarshal([]byte(persistedTask.ResultJSON), &taskResult); err != nil || taskResult["text"] != providerOutput {
		t.Fatalf("provider Task result = %#v, error=%v", taskResult, err)
	}

	routeAttempts, err := repo.RouteAttempts(task.ID, 1)
	if err != nil {
		t.Fatalf("load Task route attempts: %v", err)
	}
	if len(routeAttempts) != 1 || routeAttempts[0].RouteID != logicalRoute.ID ||
		routeAttempts[0].ChannelModelID != channelModel.ID || routeAttempts[0].Status != "succeeded" ||
		routeAttempts[0].DispatchState != "accepted" {
		t.Fatalf("logical route execution evidence is invalid: %#v", routeAttempts)
	}
	var apiLogs []model.ApiCallLog
	if err := db.Where("task_id = ?", task.ID).Find(&apiLogs).Error; err != nil || len(apiLogs) != 1 ||
		apiLogs[0].Status != model.ApiCallStatusSucceeded || apiLogs[0].ChannelID != channelModel.ChannelID ||
		apiLogs[0].Model != channelModel.ModelKey || apiLogs[0].Path != "/v1/chat/completions" {
		t.Fatalf("provider API audit rows = %#v, error=%v", apiLogs, err)
	}

	detail, err := repo.AgentRuntimeDetailForUser(project.UserID, created.Detail.Run.ID)
	if err != nil {
		t.Fatalf("load completed Film Run: %v", err)
	}
	if detail.Run.Status != model.AgentRunStatusCompleted || len(detail.Attempts) != 1 ||
		detail.Attempts[0].TaskID != task.ID || detail.Attempts[0].Status != model.AgentAttemptStatusSucceeded ||
		detail.Attempts[0].ModelRef != logicalModel.ID {
		t.Fatalf("Film Attempt is not linked to the real provider Task: %#v", detail)
	}
	output := findFilmAgentOutputRevision(t, detail.Artifacts, detail.ArtifactRevisions, "script")
	if output.Status != model.ProductionArtifactStatusReview || output.SourceAttemptID != detail.Attempts[0].ID {
		t.Fatalf("provider output Artifact evidence is invalid: %#v", output)
	}

	if requestCount.Load() != 1 {
		t.Fatalf("provider request count = %d, want 1", requestCount.Load())
	}
	requestMu.Lock()
	capturedRequest := providerRequest
	requestMu.Unlock()
	if capturedRequest["model"] != channelModel.ModelKey {
		t.Fatalf("provider model = %v, want %s", capturedRequest["model"], channelModel.ModelKey)
	}
	messages, ok := capturedRequest["messages"].([]any)
	if !ok || len(messages) != 2 {
		t.Fatalf("provider messages = %#v", capturedRequest["messages"])
	}
	systemMessage, _ := messages[0].(map[string]any)
	userMessage, _ := messages[1].(map[string]any)
	if systemMessage["role"] != "system" || userMessage["role"] != "user" ||
		!containsStringValue(systemMessage["content"], "[AGENT narrative_screenwriter]") ||
		!containsStringValue(systemMessage["content"], "[SKILL screenwriter") ||
		!containsStringValue(userMessage["content"], `"expectedOutputArtifactTypes":["script"]`) {
		t.Fatalf("compiled Agent/Skill prompts did not reach the provider: %#v", messages)
	}
}

func newFilmAgentProviderIntegrationService(t *testing.T, providerBaseURL string) (*Service, *repository.Repository, *gorm.DB, model.Project, model.LogicalModel, model.ChannelModel, model.LogicalModelRoute) {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "film-agent-provider.db")
	db, err := gorm.Open(sqlite.Open("file:"+databasePath+"?_busy_timeout=5000&_journal_mode=WAL"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open Film provider integration database: %v", err)
	}
	if err := db.AutoMigrate(database.Models()...); err != nil {
		t.Fatalf("migrate Film provider integration database: %v", err)
	}
	now := time.Now().UTC()
	user := model.User{ID: "user-film-provider", Username: "film-provider-user", DisplayName: "Film Provider User", Role: model.UserRoleUser, Status: model.UserStatusActive, CreatedAt: now, UpdatedAt: now}
	project := model.Project{
		ID: "project-film-provider", UserID: user.ID, Name: "Film Provider Project", Type: "short-drama", AspectRatio: "9:16",
		SourceType: "blank", Status: model.ProjectStatusActive, Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	channel := model.ModelChannel{
		ID: "channel-film-provider", UserID: "admin", Scope: model.ChannelScopeSystem, Enabled: true, Name: "Film Test Provider",
		BaseURL: providerBaseURL, AllowLocalChannel: true, APIKey: "film-provider-key", APIFormat: "openai",
		ConcurrencyLimit: 2, ModelsJSON: `["film-text-model"]`, CreatedAt: now, UpdatedAt: now,
	}
	capabilityConfig := &ModelCapabilityConfig{Version: 1, Text: &TextCapabilityConfig{References: TextReferenceConfig{PromptMaxChars: 2_000_000}}}
	capabilityConfigJSON, err := json.Marshal(capabilityConfig)
	if err != nil {
		t.Fatalf("encode channel capability: %v", err)
	}
	capabilitySpec, err := CapabilitySpecFromModelCapabilityConfig(capabilityConfig, "text")
	if err != nil {
		t.Fatalf("project channel capability: %v", err)
	}
	capabilitySpecJSON, err := json.Marshal(capabilitySpec)
	if err != nil {
		t.Fatalf("encode logical capability: %v", err)
	}
	channelModel := model.ChannelModel{
		ID: "channel-model-film-provider", ChannelID: channel.ID, ModelKey: "film-text-model", DisplayName: "Film Text Model",
		Capability: "text", Protocol: model.ChannelInterfaceChatCompletion, BillingMode: "fixed_request",
		UnitPriceMicrocredits: 100, PriceConfigured: true, Enabled: true, PriceVersion: 1,
		CapabilityConfigJSON: string(capabilityConfigJSON), CapabilityVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	revision := model.LogicalModelRevision{
		ID: "logical-revision-film-provider", LogicalModelID: "logical-film-provider", Version: 1,
		CapabilitySpecJSON: string(capabilitySpecJSON), DefaultOptionsJSON: `{}`, CreatedBy: "admin", CreatedAt: now,
	}
	logicalModel := model.LogicalModel{
		ID: revision.LogicalModelID, Code: "film-provider-model", Name: "Film Provider Model", Capability: "text", Enabled: true,
		RevisionSequence: 1, ActiveRevisionID: revision.ID, PricePolicy: "unified", BillingMode: "fixed_request",
		UnitPriceMicrocredits: 100, CreatedAt: now, UpdatedAt: now,
	}
	logicalRoute := model.LogicalModelRoute{
		ID: "logical-route-film-provider", LogicalModelRevisionID: revision.ID, ChannelModelID: channelModel.ID,
		Enabled: true, Priority: 100, Weight: 100, CreatedAt: now, UpdatedAt: now,
	}
	featureSetting := model.SystemSetting{
		Key:       featureAvailabilitySettingKey,
		ValueJSON: `{"shortDramaEnabled":true,"taskCenterEnabled":true,"creditsEnabled":false,"customChannelsEnabled":true,"frontendModelsEnabled":true}`,
		UpdatedBy: "test", CreatedAt: now, UpdatedAt: now,
	}
	for _, item := range []any{&user, &project, &channel, &channelModel, &logicalModel, &revision, &logicalRoute, &featureSetting} {
		if err := db.Create(item).Error; err != nil {
			t.Fatalf("create Film provider integration fixture %T: %v", item, err)
		}
	}
	repo := repository.New(db)
	svc := NewWithRuntimeCapabilities(repo, t.TempDir(), RuntimeCapabilities{desktopLocalChannels: true})
	if err := svc.ValidateRuntime(); err != nil {
		t.Fatalf("ValidateRuntime(): %v", err)
	}
	return svc, repo, db, project, logicalModel, channelModel, logicalRoute
}

func waitForFilmAgentProviderTask(t *testing.T, db *gorm.DB, filmDone <-chan filmAgentProcessResult) model.Task {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		var task model.Task
		err := db.Where("type = ?", "canvas_text_film_agent").Order("created_at desc").First(&task).Error
		if err == nil {
			return task
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("find queued Film provider Task: %v", err)
		}
		select {
		case result := <-filmDone:
			t.Fatalf("Film Agent execution ended before creating its Task: processed=%v error=%v", result.processed, result.err)
		case <-ticker.C:
		case <-deadline.C:
			t.Fatal("Film Agent executor did not create a provider Task")
		}
	}
}

func containsStringValue(value any, fragment string) bool {
	text, ok := value.(string)
	return ok && strings.Contains(text, fragment)
}
