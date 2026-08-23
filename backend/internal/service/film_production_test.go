package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"infinite-canvas/backend/internal/database"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type filmProductionTestFixture struct {
	Service                  *Service
	Repository               *repository.Repository
	DB                       *gorm.DB
	Project                  model.Project
	RootRun                  model.AgentRuntimeRun
	Shot                     model.Shot
	StoryboardRevision       model.ProductionArtifactRevision
	PromptArtifact           model.ProductionArtifact
	PromptRevision           model.ProductionArtifactRevision
	FeasibilityRevision      model.ProductionArtifactRevision
	LogicalModel             model.LogicalModel
	ChannelModel             model.ChannelModel
	ProviderRequestCount     *atomic.Int64
	ProviderFailureRemaining *atomic.Int64
}

func TestFilmProductionImageGoldenPathPersistsWorkerResultQCAndPaidRetry(t *testing.T) {
	fixture := newFilmProductionTestFixture(t)
	request := fixture.quoteRequest(fixture.PromptRevision.ID, "")

	quote, err := fixture.Service.CreateFilmProductionImageQuote(fixture.Project.UserID, fixture.Project.ID, "film-quote-golden-0001", request)
	if err != nil {
		t.Fatalf("CreateFilmProductionImageQuote(): %v", err)
	}
	if quote.Count != 1 || !quote.Cost.Required || quote.Cost.AmountMicrocredits != 250 || quote.Cost.Status != model.BillingStatusReserved ||
		quote.QuoteFingerprint == "" || quote.RequestFingerprint == "" || quote.Options.Size != "1024x1024" {
		t.Fatalf("quote did not freeze generation and cost facts: %#v", quote)
	}
	replayedQuote, err := fixture.Service.CreateFilmProductionImageQuote(fixture.Project.UserID, fixture.Project.ID, "film-quote-golden-0001", request)
	if err != nil || !replayedQuote.Idempotent || replayedQuote.ID != quote.ID {
		t.Fatalf("quote replay = %#v, error = %v", replayedQuote, err)
	}
	changedRequest := request
	changedRequest.Options.Quality = "medium"
	if _, err := fixture.Service.CreateFilmProductionImageQuote(fixture.Project.UserID, fixture.Project.ID, "film-quote-golden-0001", changedRequest); authStatus(err) != http.StatusConflict {
		t.Fatalf("changed quote replay error = %v, want 409", err)
	}
	if _, err := fixture.Service.ListFilmProductionAttempts("another-user", fixture.Project.ID, "", "", 10); authStatus(err) != http.StatusNotFound {
		t.Fatalf("foreign attempt list error = %v, want 404", err)
	}

	driftQuote, err := fixture.Service.CreateFilmProductionImageQuote(fixture.Project.UserID, fixture.Project.ID, "film-quote-drift-0001", request)
	if err != nil {
		t.Fatalf("create drift quote: %v", err)
	}
	if err := fixture.DB.Model(&model.ChannelModel{}).Where("id = ?", fixture.ChannelModel.ID).
		Updates(map[string]any{"price_version": fixture.ChannelModel.PriceVersion + 1, "unit_price_microcredits": int64(999)}).Error; err != nil {
		t.Fatalf("drift channel price: %v", err)
	}
	if _, err := fixture.Service.SubmitFilmProductionImageQuote(
		fixture.Project.UserID, fixture.Project.ID, driftQuote.ID, "film-submit-drift-0001",
		SubmitFilmProductionImageQuoteRequest{QuoteFingerprint: driftQuote.QuoteFingerprint},
	); authStatus(err) != http.StatusConflict {
		t.Fatalf("price drift submit error = %v, want 409", err)
	}
	if err := fixture.DB.Model(&model.ChannelModel{}).Where("id = ?", fixture.ChannelModel.ID).
		Updates(map[string]any{"price_version": fixture.ChannelModel.PriceVersion, "unit_price_microcredits": fixture.ChannelModel.UnitPriceMicrocredits}).Error; err != nil {
		t.Fatalf("restore channel price: %v", err)
	}

	expiredQuote, err := fixture.Service.CreateFilmProductionImageQuote(fixture.Project.UserID, fixture.Project.ID, "film-quote-expired-0001", request)
	if err != nil {
		t.Fatalf("create expiry quote: %v", err)
	}
	if err := fixture.DB.Model(&model.FilmProductionQuote{}).Where("id = ?", expiredQuote.ID).
		Updates(map[string]any{"expires_at": time.Now().UTC().Add(-time.Minute)}).Error; err != nil {
		t.Fatalf("expire quote: %v", err)
	}
	if _, err := fixture.Service.SubmitFilmProductionImageQuote(
		fixture.Project.UserID, fixture.Project.ID, expiredQuote.ID, "film-submit-expired-0001",
		SubmitFilmProductionImageQuoteRequest{QuoteFingerprint: expiredQuote.QuoteFingerprint},
	); authStatus(err) != http.StatusConflict {
		t.Fatalf("expired submit error = %v, want 409", err)
	}
	var persistedExpired model.FilmProductionQuote
	if err := fixture.DB.First(&persistedExpired, "id = ?", expiredQuote.ID).Error; err != nil || persistedExpired.Status != model.FilmProductionQuoteStatusExpired {
		t.Fatalf("expired quote status = %s, error = %v", persistedExpired.Status, err)
	}

	submitted, err := fixture.Service.SubmitFilmProductionImageQuote(
		fixture.Project.UserID, fixture.Project.ID, quote.ID, "film-submit-golden-0001",
		SubmitFilmProductionImageQuoteRequest{QuoteFingerprint: quote.QuoteFingerprint},
	)
	if err != nil {
		t.Fatalf("SubmitFilmProductionImageQuote(): %v", err)
	}
	if submitted.Idempotent || submitted.Attempt.Attempt.Status != model.FilmProductionAttemptStatusQueued || submitted.Attempt.Attempt.Number != 1 {
		t.Fatalf("submitted Attempt is incomplete: %#v", submitted)
	}
	replayedSubmit, err := fixture.Service.SubmitFilmProductionImageQuote(
		fixture.Project.UserID, fixture.Project.ID, quote.ID, "film-submit-golden-0001",
		SubmitFilmProductionImageQuoteRequest{QuoteFingerprint: quote.QuoteFingerprint},
	)
	if err != nil || !replayedSubmit.Idempotent || replayedSubmit.Attempt.Attempt.ID != submitted.Attempt.Attempt.ID {
		t.Fatalf("submit replay = %#v, error = %v", replayedSubmit, err)
	}

	if err := fixture.Service.ProcessNextTask(); err != nil {
		t.Fatalf("ProcessNextTask(): %v", err)
	}
	if fixture.ProviderRequestCount.Load() != 1 {
		t.Fatalf("provider request count = %d, want 1", fixture.ProviderRequestCount.Load())
	}
	detail, err := fixture.Repository.FilmProductionAttemptForUser(fixture.Project.UserID, fixture.Project.ID, submitted.Attempt.Attempt.ID)
	if err != nil {
		t.Fatalf("load completed Attempt: %v", err)
	}
	if detail.Attempt.Status != model.FilmProductionAttemptStatusSucceeded || detail.Task.Status != model.TaskStatusSucceeded ||
		detail.Result == nil || detail.Result.Kind != model.ResultKindFilmGeneration || detail.Result.Availability != model.ResultAvailabilityReady ||
		!strings.HasPrefix(detail.Result.URL, "/api/resources/") || len(detail.QCReports) != 1 ||
		detail.QCReports[0].Decision != model.FilmProductionQCDecisionNotAssessable || detail.QCReports[0].Action != model.FilmProductionQCActionHold {
		t.Fatalf("Worker completion facts are incomplete: %#v", detail)
	}
	if detail.Attempt.ResultArtifactID == "" || detail.Attempt.ResultRevisionID == "" || detail.Result.ArtifactID != detail.Attempt.ResultArtifactID {
		t.Fatalf("Result Artifact linkage is incomplete: attempt=%#v result=%#v", detail.Attempt, detail.Result)
	}
	resultArtifact, err := fixture.Repository.ProductionArtifactForUser(fixture.Project.UserID, detail.Attempt.ResultArtifactID)
	if err != nil || resultArtifact.ArtifactType != "generation-result" || resultArtifact.CurrentRevisionID != detail.Attempt.ResultRevisionID {
		t.Fatalf("generation-result Artifact = %#v, error = %v", resultArtifact, err)
	}
	if detail.Billing == nil || detail.Billing.Status != model.BillingStatusSettled || detail.Billing.ActualAmountMicrocredits != 250 {
		t.Fatalf("settled Film cost = %#v", detail.Billing)
	}
	if _, err := fixture.Service.RetryTask(fixture.Project.UserID, detail.Task.ID); authStatus(err) != http.StatusBadRequest || !strings.Contains(err.Error(), "重新报价") {
		t.Fatalf("generic Film retry error = %v", err)
	}

	accepted, err := fixture.Service.CreateFilmProductionHumanQC(
		fixture.Project.UserID, fixture.Project.ID, detail.Attempt.ID, "film-qc-accept-0001",
		CreateFilmProductionQCRequest{Decision: model.FilmProductionQCDecisionPass, Action: model.FilmProductionQCActionAccept, Evidence: map[string]any{"reviewed": true}},
	)
	if err != nil {
		t.Fatalf("accept Film result: %v", err)
	}
	if accepted.Idempotent || !accepted.Attempt.Accepted || accepted.Attempt.CurrentQC == nil || accepted.Attempt.CurrentQC.Source != "human" {
		t.Fatalf("accepted Attempt is incomplete: %#v", accepted)
	}
	replayedQC, err := fixture.Service.CreateFilmProductionHumanQC(
		fixture.Project.UserID, fixture.Project.ID, detail.Attempt.ID, "film-qc-accept-0001",
		CreateFilmProductionQCRequest{Decision: model.FilmProductionQCDecisionPass, Action: model.FilmProductionQCActionAccept, Evidence: map[string]any{"reviewed": true}},
	)
	if err != nil || !replayedQC.Idempotent || replayedQC.Report.ID != accepted.Report.ID {
		t.Fatalf("QC replay = %#v, error = %v", replayedQC, err)
	}
	if _, err := fixture.Service.CreateFilmProductionHumanQC(
		fixture.Project.UserID, fixture.Project.ID, detail.Attempt.ID, "film-qc-accept-0001",
		CreateFilmProductionQCRequest{Decision: model.FilmProductionQCDecisionPass, Action: model.FilmProductionQCActionAccept, Note: "changed"},
	); authStatus(err) != http.StatusConflict {
		t.Fatalf("changed QC replay error = %v, want 409", err)
	}

	failedQC, err := fixture.Service.CreateFilmProductionHumanQC(
		fixture.Project.UserID, fixture.Project.ID, detail.Attempt.ID, "film-qc-retry-0001",
		CreateFilmProductionQCRequest{
			Decision: model.FilmProductionQCDecisionFail, Action: model.FilmProductionQCActionRetry,
			IssueCodes: []string{"CONTINUITY_MISMATCH"}, Note: "修正镜头连续性后重新生成",
		},
	)
	if err != nil || !failedQC.Attempt.RetryAllowed || failedQC.Attempt.Accepted {
		t.Fatalf("failed QC did not open paid retry: result=%#v error=%v", failedQC, err)
	}
	newPromptRevision := fixture.appendPromptRevision(t, `{"shots":[{"shotId":"shot-film-production","prompt_text":"revised cinematic close-up, exact character continuity"}]}`)
	retryRequest := fixture.quoteRequest(newPromptRevision.ID, detail.Attempt.ID)
	retryQuote, err := fixture.Service.CreateFilmProductionImageQuote(fixture.Project.UserID, fixture.Project.ID, "film-quote-retry-0001", retryRequest)
	if err != nil {
		t.Fatalf("create paid retry quote: %v", err)
	}
	if retryQuote.RetryOfAttemptID != detail.Attempt.ID || !retryQuote.Cost.Required || retryQuote.Cost.AmountMicrocredits != 250 {
		t.Fatalf("paid retry quote is incomplete: %#v", retryQuote)
	}
	retried, err := fixture.Service.SubmitFilmProductionImageQuote(
		fixture.Project.UserID, fixture.Project.ID, retryQuote.ID, "film-submit-retry-0001",
		SubmitFilmProductionImageQuoteRequest{QuoteFingerprint: retryQuote.QuoteFingerprint},
	)
	if err != nil {
		t.Fatalf("submit paid retry: %v", err)
	}
	if retried.Attempt.Attempt.Number != 2 || retried.Attempt.Attempt.RetryOfAttemptID != detail.Attempt.ID ||
		retried.Attempt.Attempt.TaskID == detail.Attempt.TaskID || retried.Attempt.Attempt.QuoteID == detail.Attempt.QuoteID {
		t.Fatalf("paid retry did not create append-only facts: %#v", retried)
	}
	cancelled, err := fixture.Service.CancelTask(context.Background(), fixture.Project.UserID, retried.Attempt.Attempt.TaskID)
	if err != nil || cancelled.Status != model.TaskStatusCancelled {
		t.Fatalf("cancel queued paid retry = %#v, error = %v", cancelled, err)
	}
	cancelledDetail, err := fixture.Repository.FilmProductionAttemptForUser(fixture.Project.UserID, fixture.Project.ID, retried.Attempt.Attempt.ID)
	if err != nil || cancelledDetail.Attempt.Status != model.FilmProductionAttemptStatusCancelled ||
		cancelledDetail.Billing == nil || cancelledDetail.Billing.Status != model.BillingStatusRefunded {
		t.Fatalf("cancelled Film facts = %#v, error = %v", cancelledDetail, err)
	}
}

func TestFilmProductionQCUncertainRequiresHoldAndEvidence(t *testing.T) {
	if _, _, _, err := normalizeFilmProductionQCInput(CreateFilmProductionQCRequest{
		Decision: model.FilmProductionQCDecisionUncertain, Action: model.FilmProductionQCActionRetry,
	}); authStatus(err) != http.StatusBadRequest {
		t.Fatalf("UNCERTAIN with retry error = %v, want 400", err)
	}
	issueCodes, evidence, note, err := normalizeFilmProductionQCInput(CreateFilmProductionQCRequest{
		Decision: model.FilmProductionQCDecisionUncertain, Action: model.FilmProductionQCActionHold,
		IssueCodes: []string{"IDENTITY_REVIEW"}, Evidence: map[string]any{"reviewedFrames": 3}, Note: "需要人工对比相邻镜头",
	})
	if err != nil || len(issueCodes) != 1 || issueCodes[0] != "IDENTITY_REVIEW" || evidence["reviewedFrames"] != 3 || note == "" {
		t.Fatalf("UNCERTAIN normalization = codes=%v evidence=%v note=%q error=%v", issueCodes, evidence, note, err)
	}
}

func TestFilmProductionSubmitRejectsReferenceDriftBeforeBilling(t *testing.T) {
	for _, status := range []model.ResourceStatus{model.ResourceStatusFailed, model.ResourceStatusDeleted} {
		t.Run(string(status), func(t *testing.T) {
			fixture := newFilmProductionTestFixture(t)
			now := time.Now().UTC()
			resource := model.Resource{
				ID: "film-reference-" + string(status), UserID: fixture.Project.UserID,
				Kind: "image", Status: model.ResourceStatusReady, Provider: "local",
				ObjectKey: "film/reference.png", MimeType: "image/png", Size: 128, Width: 640, Height: 960,
				CreatedAt: now, UpdatedAt: now,
			}
			if err := fixture.DB.Create(&resource).Error; err != nil {
				t.Fatalf("create reference resource: %v", err)
			}
			request := fixture.quoteRequest(fixture.PromptRevision.ID, "")
			request.ReferenceResourceIDs = []string{resource.ID}
			quote, err := fixture.Service.CreateFilmProductionImageQuote(
				fixture.Project.UserID, fixture.Project.ID, "film-quote-reference-"+string(status), request,
			)
			if err != nil {
				t.Fatalf("create quote with ready reference: %v", err)
			}
			if err := fixture.DB.Model(&model.Resource{}).Where("id = ?", resource.ID).
				Updates(map[string]any{"status": status, "updated_at": now.Add(time.Second)}).Error; err != nil {
				t.Fatalf("invalidate reference resource: %v", err)
			}

			_, err = fixture.Service.SubmitFilmProductionImageQuote(
				fixture.Project.UserID, fixture.Project.ID, quote.ID, "film-submit-reference-"+string(status),
				SubmitFilmProductionImageQuoteRequest{QuoteFingerprint: quote.QuoteFingerprint},
			)
			if authStatus(err) != http.StatusConflict {
				t.Fatalf("reference drift submit error = %v, want 409", err)
			}

			for name, target := range map[string]any{
				"task": &model.Task{}, "attempt": &model.FilmProductionAttempt{},
				"billing order": &model.BillingOrder{}, "credit ledger": &model.CreditLedgerEntry{},
			} {
				var count int64
				if err := fixture.DB.Model(target).Count(&count).Error; err != nil || count != 0 {
					t.Fatalf("%s count = %d, error = %v; submit must fail before writes", name, count, err)
				}
			}
			var account model.CreditAccount
			if err := fixture.DB.First(&account, "user_id = ?", fixture.Project.UserID).Error; err != nil ||
				account.AvailableMicrocredits != 10_000 || account.ReservedMicrocredits != 0 || account.Version != 1 {
				t.Fatalf("credit account changed after rejected submit: %#v, error = %v", account, err)
			}
			var persistedQuote model.FilmProductionQuote
			if err := fixture.DB.First(&persistedQuote, "id = ?", quote.ID).Error; err != nil || persistedQuote.Status != model.FilmProductionQuoteStatusPending {
				t.Fatalf("rejected quote status = %s, error = %v", persistedQuote.Status, err)
			}
		})
	}
}

func TestFilmProductionRejectsMissingMediaAndNeverFallsBack(t *testing.T) {
	tests := []string{
		`{"mode":"image","images":[]}`,
		`{"mode":"image","images":[{"dataUrl":"data:image/png;base64,abc"}]}`,
		`{"mode":"image","images":[{"url":"/api/resources/one/file"},{"url":"/api/resources/two/file"}]}`,
		`{"mode":"video","images":[{"url":"https://media.example.com/out.png"}]}`,
	}
	for _, raw := range tests {
		if _, err := filmProductionMediaFactFromResult([]byte(raw)); !errors.Is(err, repository.ErrFilmProductionMediaMissing) {
			t.Fatalf("media payload %s error = %v, want media missing", raw, err)
		}
	}
	media, err := filmProductionMediaFactFromResult([]byte(`{"mode":"image","images":[{"url":"https://media.example.com/out.png","mimeType":"image/png"}]}`))
	if err != nil || media.URL != "https://media.example.com/out.png" {
		t.Fatalf("valid remote media = %#v, error = %v", media, err)
	}

	svc := &Service{}
	task := &model.Task{Type: model.FilmProductionTaskTypeImage}
	routeAttempt := &model.RouteAttempt{DispatchState: "rejected_no_job"}
	next, err := svc.nextRouteAttemptAfterFailure(task, routeAttempt, errors.New("provider rejected request"))
	if err != nil || next != nil {
		t.Fatalf("Film fallback result = %#v, error = %v", next, err)
	}
	if _, err := svc.switchTaskToNextRoute(task, nil); err == nil || !strings.Contains(err.Error(), "路由已冻结") {
		t.Fatalf("direct Film route switch error = %v", err)
	}
}

func newFilmProductionTestFixture(t *testing.T) filmProductionTestFixture {
	t.Helper()
	t.Setenv("CANVAS_ALLOW_PRIVATE_UPSTREAMS", "true")
	imageBase64 := filmProductionTestPNG(t)
	requestCount := &atomic.Int64{}
	failureRemaining := &atomic.Int64{}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requestCount.Add(1)
		if request.Method != http.MethodPost || request.URL.Path != "/v1/images/generations" {
			http.NotFound(w, request)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body["n"] != float64(1) {
			http.Error(w, "invalid image request", http.StatusBadRequest)
			return
		}
		prompt := strings.TrimSpace(stringValue(body["prompt"]))
		if prompt == "" {
			http.Error(w, "invalid image request", http.StatusBadRequest)
			return
		}
		if strings.Contains(prompt, "force-provider-failure") && failureRemaining.Load() > 0 {
			failureRemaining.Add(-1)
			http.Error(w, "controlled provider failure", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"b64_json": imageBase64}}})
	}))
	t.Cleanup(provider.Close)

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "film-production.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open Film Production database: %v", err)
	}
	if err := db.AutoMigrate(database.Models()...); err != nil {
		t.Fatalf("migrate Film Production database: %v", err)
	}
	repo := repository.New(db)
	svc := NewWithRuntimeCapabilities(repo, t.TempDir(), RuntimeCapabilities{desktopLocalChannels: true})
	if err := svc.ValidateRuntime(); err != nil {
		t.Fatalf("ValidateRuntime(): %v", err)
	}
	now := time.Now().UTC()
	user := model.User{ID: "user-film-production", Username: "film-production", DisplayName: "Film Production", Role: model.UserRoleUser, Status: model.UserStatusActive, CreatedAt: now, UpdatedAt: now}
	project := model.Project{
		ID: "project-film-production", UserID: user.ID, Name: "Film Production Project", Type: "short-drama", AspectRatio: "9:16",
		SourceType: "blank", Status: model.ProjectStatusActive, Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	root := model.AgentRuntimeRun{
		ID: "root-film-production", UserID: user.ID, ProjectID: project.ID, Domain: "film", RootRunID: "root-film-production",
		RegistryID: svc.filmAgentRegistry.ID, RegistryVersion: svc.filmAgentRegistry.Version, RegistryDigest: svc.filmAgentRegistry.SourceDigest,
		RouteKind: "intent", IntentRouteID: "IR-07", Status: model.AgentRunStatusCompleted,
		Objective: "生成短剧镜头图片", InputJSON: `{}`, IdempotencyKey: "root-film-production-key", Revision: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	shot := model.Shot{ID: "shot-film-production", ProjectID: project.ID, Title: "Hero shot", Position: 1, DurationMs: 3000, Status: "ready", CreatedAt: now, UpdatedAt: now}
	channel := model.ModelChannel{
		ID: "channel-film-production", UserID: "admin", Scope: model.ChannelScopeSystem, Enabled: true, Name: "Film Image Provider",
		BaseURL: provider.URL + "/v1", AllowLocalChannel: true, APIKey: "film-image-key", APIFormat: "openai",
		ConcurrencyLimit: 2, ModelsJSON: `["film-image-provider"]`, CreatedAt: now, UpdatedAt: now,
	}
	capabilityConfig := &ModelCapabilityConfig{Version: 1, Image: DefaultImageCapabilityConfig(string(model.ChannelInterfaceOpenAIImage), "film-image-provider")}
	capabilityConfig.Image.MaxOutputs = 1
	capabilityConfigJSON, err := json.Marshal(capabilityConfig)
	if err != nil {
		t.Fatalf("encode image channel capability: %v", err)
	}
	capabilitySpec, err := CapabilitySpecFromModelCapabilityConfig(capabilityConfig, "image")
	if err != nil {
		t.Fatalf("project image capability: %v", err)
	}
	capabilitySpecJSON, err := json.Marshal(capabilitySpec)
	if err != nil {
		t.Fatalf("encode image logical capability: %v", err)
	}
	channelModel := model.ChannelModel{
		ID: "channel-model-film-production", ChannelID: channel.ID, ModelKey: "film-image-provider", DisplayName: "Film Image Provider",
		Capability: "image", Protocol: model.ChannelInterfaceOpenAIImage, BillingMode: "fixed_request",
		UnitPriceMicrocredits: 125, PriceConfigured: true, Enabled: true, PriceVersion: 1,
		CapabilityConfigJSON: string(capabilityConfigJSON), CapabilityVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	revision := model.LogicalModelRevision{
		ID: "logical-revision-film-production", LogicalModelID: "logical-film-production", Version: 1,
		CapabilitySpecJSON: string(capabilitySpecJSON), DefaultOptionsJSON: `{}`, CreatedBy: "admin", CreatedAt: now,
	}
	logicalModel := model.LogicalModel{
		ID: revision.LogicalModelID, Code: "film-image", Name: "Film Image", Capability: "image", Enabled: true,
		RevisionSequence: 1, ActiveRevisionID: revision.ID, PricePolicy: "unified", BillingMode: "fixed_request",
		UnitPriceMicrocredits: 250, CreatedAt: now, UpdatedAt: now,
	}
	route := model.LogicalModelRoute{
		ID: "logical-route-film-production", LogicalModelRevisionID: revision.ID, ChannelModelID: channelModel.ID,
		Enabled: true, Priority: 100, Weight: 100, CreatedAt: now, UpdatedAt: now,
	}
	featureSetting := model.SystemSetting{
		Key:       featureAvailabilitySettingKey,
		ValueJSON: `{"shortDramaEnabled":true,"taskCenterEnabled":true,"creditsEnabled":true,"customChannelsEnabled":true}`,
		UpdatedBy: "test", CreatedAt: now, UpdatedAt: now,
	}
	credit := model.CreditAccount{UserID: user.ID, AvailableMicrocredits: 10_000, Version: 1, CreatedAt: now, UpdatedAt: now}
	for _, item := range []any{&user, &project, &root, &shot, &channel, &channelModel, &logicalModel, &revision, &route, &featureSetting, &credit} {
		if err := db.Create(item).Error; err != nil {
			t.Fatalf("create Film Production fixture %T: %v", item, err)
		}
	}
	_, storyboardRevision := createFilmProductionArtifactFixture(t, repo, project, root, "storyboard", "storyboard", `{"shots":[{"shotId":"shot-film-production"}]}`)
	prompt, promptRevision := createFilmProductionArtifactFixture(t, repo, project, root, "prompt", "image-prompt-pack", `{"shots":[{"shotId":"shot-film-production","prompt_text":"cinematic close-up, exact character continuity"}]}`)
	_, feasibilityRevision := createFilmProductionArtifactFixture(t, repo, project, root, "feasibility", "production-feasibility-report", `{"status":"READY"}`)
	return filmProductionTestFixture{
		Service: svc, Repository: repo, DB: db, Project: project, RootRun: root, Shot: shot,
		StoryboardRevision: storyboardRevision, PromptArtifact: prompt, PromptRevision: promptRevision,
		FeasibilityRevision: feasibilityRevision, LogicalModel: logicalModel, ChannelModel: channelModel,
		ProviderRequestCount: requestCount, ProviderFailureRemaining: failureRemaining,
	}
}

func (fixture filmProductionTestFixture) quoteRequest(promptRevisionID string, retryOfAttemptID string) CreateFilmProductionImageQuoteRequest {
	return CreateFilmProductionImageQuoteRequest{
		RootRunID: fixture.RootRun.ID, ShotID: fixture.Shot.ID,
		StoryboardArtifactRevisionID: fixture.StoryboardRevision.ID, PromptArtifactRevisionID: promptRevisionID,
		FeasibilityRevisionID: fixture.FeasibilityRevision.ID, LogicalModelID: fixture.LogicalModel.ID,
		RetryOfAttemptID: retryOfAttemptID, Options: FilmProductionImageOptions{Size: "1024x1024", Quality: "high"},
	}
}

func (fixture *filmProductionTestFixture) appendPromptRevision(t *testing.T, content string) model.ProductionArtifactRevision {
	t.Helper()
	revision := &model.ProductionArtifactRevision{
		ID: "prompt-film-production-revision-2", Status: model.ProductionArtifactStatusLocked,
		ContentJSON: content, ContentDigest: digestBytesHex([]byte(content)), SourceRunID: fixture.RootRun.ID,
		SourceArtifactRefsJSON: "[]", AuthorityRefsJSON: "[]", CreatedByType: "human", CreatedByID: fixture.Project.UserID,
	}
	artifact, stored, err := fixture.Repository.CreateProductionArtifactRevision(repository.ProductionArtifactRevisionCreate{
		UserID: fixture.Project.UserID, Artifact: &fixture.PromptArtifact, Revision: revision, ExpectedSequence: 1, At: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("append Prompt revision: %v", err)
	}
	fixture.PromptArtifact = *artifact
	fixture.PromptRevision = *stored
	return *stored
}

func createFilmProductionArtifactFixture(t *testing.T, repo *repository.Repository, project model.Project, root model.AgentRuntimeRun, logicalKey string, artifactType string, content string) (model.ProductionArtifact, model.ProductionArtifactRevision) {
	t.Helper()
	artifact := &model.ProductionArtifact{
		ID: logicalKey + "-film-production-artifact", UserID: project.UserID, ProjectID: project.ID,
		Domain: "film", ArtifactType: artifactType, LogicalKey: "film-production-fixture:" + logicalKey,
	}
	revision := &model.ProductionArtifactRevision{
		ID: logicalKey + "-film-production-revision", Status: model.ProductionArtifactStatusLocked,
		ContentJSON: content, ContentDigest: digestBytesHex([]byte(content)), SourceRunID: root.ID,
		SourceArtifactRefsJSON: "[]", AuthorityRefsJSON: "[]", CreatedByType: "human", CreatedByID: project.UserID,
	}
	storedArtifact, storedRevision, err := repo.CreateProductionArtifactRevision(repository.ProductionArtifactRevisionCreate{
		UserID: project.UserID, Artifact: artifact, Revision: revision, ExpectedSequence: 0, At: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("create %s Artifact: %v", artifactType, err)
	}
	return *storedArtifact, *storedRevision
}

func filmProductionTestPNG(t *testing.T) string {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, 2, 2))
	canvas.Set(0, 0, color.RGBA{R: 220, G: 40, B: 60, A: 255})
	var output bytes.Buffer
	if err := png.Encode(&output, canvas); err != nil {
		t.Fatalf("encode Film Production test PNG: %v", err)
	}
	return base64.StdEncoding.EncodeToString(output.Bytes())
}
