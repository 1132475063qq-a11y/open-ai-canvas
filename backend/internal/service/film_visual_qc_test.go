package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
)

func TestFilmVisualQCGoldenPathUsesGeneratedImageAndRequiresHumanAcceptance(t *testing.T) {
	fixture := newFilmProductionTestFixture(t)
	providerRequests := &atomic.Int64{}
	sawGeneratedImageFirst := &atomic.Bool{}
	sawAgentAndSkills := &atomic.Bool{}
	provider := newFilmVisualQCTestProvider(t, providerRequests, sawGeneratedImageFirst, sawAgentAndSkills)
	logicalModel, channelModel := addFilmVisualQCTestModel(t, fixture, provider.URL+"/v1")

	source := completeFilmVisualQCSourceImage(t, fixture)
	quoteRequest := CreateFilmVisualQCQuoteRequest{SourceAttemptID: source.Attempt.ID, LogicalModelID: logicalModel.ID}
	quote, err := fixture.Service.CreateFilmVisualQCQuote(fixture.Project.UserID, fixture.Project.ID, "film-visual-qc-quote-0001", quoteRequest)
	if err != nil {
		t.Fatalf("CreateFilmVisualQCQuote(): %v", err)
	}
	if !quote.Cost.Required || quote.Cost.AmountMicrocredits != 180 || quote.QuoteFingerprint == "" || quote.RequestFingerprint == "" ||
		len(quote.Dimensions) != len(filmVisualQCDimensions) || len(quote.ReferenceResourceIDs) == 0 || quote.SourceResultID != source.Result.ID {
		t.Fatalf("visual QC quote did not freeze evidence and cost: %#v", quote)
	}
	replayedQuote, err := fixture.Service.CreateFilmVisualQCQuote(fixture.Project.UserID, fixture.Project.ID, "film-visual-qc-quote-0001", quoteRequest)
	if err != nil || !replayedQuote.Idempotent || replayedQuote.ID != quote.ID {
		t.Fatalf("visual QC quote replay = %#v, error = %v", replayedQuote, err)
	}

	driftQuote, err := fixture.Service.CreateFilmVisualQCQuote(fixture.Project.UserID, fixture.Project.ID, "film-visual-qc-drift-0001", quoteRequest)
	if err != nil {
		t.Fatalf("create visual QC drift quote: %v", err)
	}
	if err := fixture.DB.Model(&model.ChannelModel{}).Where("id = ?", channelModel.ID).
		Updates(map[string]any{"price_version": channelModel.PriceVersion + 1, "unit_price_microcredits": int64(999)}).Error; err != nil {
		t.Fatalf("drift visual QC price: %v", err)
	}
	if _, err := fixture.Service.SubmitFilmVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, driftQuote.ID, "film-visual-qc-submit-drift-0001",
		SubmitFilmVisualQCQuoteRequest{QuoteFingerprint: driftQuote.QuoteFingerprint},
	); authStatus(err) != http.StatusConflict {
		t.Fatalf("visual QC price drift error = %v, want 409", err)
	}
	if err := fixture.DB.Model(&model.ChannelModel{}).Where("id = ?", channelModel.ID).
		Updates(map[string]any{"price_version": channelModel.PriceVersion, "unit_price_microcredits": channelModel.UnitPriceMicrocredits}).Error; err != nil {
		t.Fatalf("restore visual QC price: %v", err)
	}

	expiredQuote, err := fixture.Service.CreateFilmVisualQCQuote(fixture.Project.UserID, fixture.Project.ID, "film-visual-qc-expired-0001", quoteRequest)
	if err != nil {
		t.Fatalf("create visual QC expiry quote: %v", err)
	}
	if err := fixture.DB.Model(&model.FilmVisualQCQuote{}).Where("id = ?", expiredQuote.ID).
		Updates(map[string]any{"expires_at": time.Now().UTC().Add(-time.Minute)}).Error; err != nil {
		t.Fatalf("expire visual QC quote: %v", err)
	}
	if _, err := fixture.Service.SubmitFilmVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, expiredQuote.ID, "film-visual-qc-submit-expired-0001",
		SubmitFilmVisualQCQuoteRequest{QuoteFingerprint: expiredQuote.QuoteFingerprint},
	); authStatus(err) != http.StatusConflict {
		t.Fatalf("expired visual QC submit error = %v, want 409", err)
	}

	submitted, err := fixture.Service.SubmitFilmVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, quote.ID, "film-visual-qc-submit-0001",
		SubmitFilmVisualQCQuoteRequest{QuoteFingerprint: quote.QuoteFingerprint},
	)
	if err != nil {
		t.Fatalf("SubmitFilmVisualQCQuote(): %v", err)
	}
	if submitted.Idempotent || submitted.Attempt.Attempt.Status != model.FilmProductionAttemptStatusQueued || submitted.Attempt.Attempt.Number != 1 ||
		submitted.Attempt.Cost.AmountMicrocredits != 180 {
		t.Fatalf("visual QC submit facts are incomplete: %#v", submitted)
	}
	replayedSubmit, err := fixture.Service.SubmitFilmVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, quote.ID, "film-visual-qc-submit-0001",
		SubmitFilmVisualQCQuoteRequest{QuoteFingerprint: quote.QuoteFingerprint},
	)
	if err != nil || !replayedSubmit.Idempotent || replayedSubmit.Attempt.Attempt.ID != submitted.Attempt.Attempt.ID {
		t.Fatalf("visual QC submit replay = %#v, error = %v", replayedSubmit, err)
	}

	if err := fixture.Service.ProcessNextTask(); err != nil {
		t.Fatalf("process visual QC task: %v", err)
	}
	if providerRequests.Load() != 1 || !sawGeneratedImageFirst.Load() || !sawAgentAndSkills.Load() {
		t.Fatalf("visual QC provider evidence was incomplete: requests=%d generatedFirst=%v authority=%v", providerRequests.Load(), sawGeneratedImageFirst.Load(), sawAgentAndSkills.Load())
	}

	views, err := fixture.Service.ListFilmProductionAttempts(fixture.Project.UserID, fixture.Project.ID, fixture.RootRun.ID, fixture.Shot.ID, 10)
	if err != nil || len(views) != 1 || len(views[0].VisualQCAttempts) != 1 {
		t.Fatalf("visual QC Attempt recovery = %#v, error = %v", views, err)
	}
	view := views[0]
	visualAttempt := view.VisualQCAttempts[0]
	if visualAttempt.Attempt.Status != model.FilmProductionAttemptStatusSucceeded || visualAttempt.Report == nil ||
		visualAttempt.Report.Decision != model.FilmProductionQCDecisionPass || visualAttempt.Report.Action != model.FilmProductionQCActionHold ||
		visualAttempt.Report.AssessmentKind != "visual_semantic_qc" || visualAttempt.Report.ModelAttemptID != visualAttempt.Attempt.ID ||
		view.Accepted || view.CurrentQC == nil || view.CurrentQC.Source != "model" {
		t.Fatalf("model QC must remain advisory: %#v", view)
	}
	if dimensions, ok := visualAttempt.Report.Evidence["dimensions"].([]any); !ok || len(dimensions) != len(filmVisualQCDimensions) {
		t.Fatalf("visual QC dimensions = %#v", visualAttempt.Report.Evidence["dimensions"])
	}
	if visualAttempt.Cost.Status != model.BillingStatusSettled || visualAttempt.Cost.ActualAmountMicrocredits != 180 {
		t.Fatalf("visual QC billing did not settle: %#v", visualAttempt.Cost)
	}

	accepted, err := fixture.Service.CreateFilmProductionHumanQC(
		fixture.Project.UserID, fixture.Project.ID, source.Attempt.ID, "film-human-after-visual-qc-0001",
		CreateFilmProductionQCRequest{Decision: model.FilmProductionQCDecisionPass, Action: model.FilmProductionQCActionAccept, Evidence: map[string]any{"reviewedModelEvidence": true}},
	)
	if err != nil || !accepted.Attempt.Accepted || accepted.Attempt.CurrentQC == nil || accepted.Attempt.CurrentQC.Source != "human" ||
		accepted.Report.AssessmentKind != "human_media_review" {
		t.Fatalf("human acceptance after model QC = %#v, error = %v", accepted, err)
	}
}

func TestParseFilmVisualQCResultRejectsNonContractOutput(t *testing.T) {
	valid := filmVisualQCProviderPayload(t, filmVisualQCTestOutput())
	if output, err := parseFilmVisualQCResult(valid); err != nil || output.OverallDecision != model.FilmProductionQCDecisionPass {
		t.Fatalf("valid visual QC output = %#v, error = %v", output, err)
	}
	tests := map[string][]byte{
		"markdown fence":  []byte("{\"mode\":\"text\",\"text\":\"```json\\n{}\\n```\"}"),
		"unknown field":   []byte(`{"mode":"text","text":"{\"schemaVersion\":1,\"overallDecision\":\"PASS\",\"dimensions\":[],\"summary\":\"ok\",\"action\":\"accept\"}"}`),
		"trailing json":   []byte(`{"mode":"text","text":"{} {}"}`),
		"wrong aggregate": filmVisualQCProviderPayload(t, filmVisualQCTestOutputWithDecision(model.FilmProductionQCDecisionUncertain)),
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := parseFilmVisualQCResult(raw); err == nil {
				t.Fatal("non-contract visual QC output was accepted")
			}
		})
	}
}

func TestFilmVisualQCQueuedCancelRefundsAndRejectsParallelAttempt(t *testing.T) {
	fixture := newFilmProductionTestFixture(t)
	provider := newFilmVisualQCTestProvider(t, &atomic.Int64{}, &atomic.Bool{}, &atomic.Bool{})
	logicalModel, _ := addFilmVisualQCTestModel(t, fixture, provider.URL+"/v1")
	source := completeFilmVisualQCSourceImage(t, fixture)

	quote, err := fixture.Service.CreateFilmVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-visual-qc-cancel-quote", CreateFilmVisualQCQuoteRequest{SourceAttemptID: source.Attempt.ID, LogicalModelID: logicalModel.ID},
	)
	if err != nil {
		t.Fatalf("create cancellable visual QC quote: %v", err)
	}
	submitted, err := fixture.Service.SubmitFilmVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, quote.ID, "film-visual-qc-cancel-submit", SubmitFilmVisualQCQuoteRequest{QuoteFingerprint: quote.QuoteFingerprint},
	)
	if err != nil {
		t.Fatalf("submit cancellable visual QC quote: %v", err)
	}
	parallelQuote, err := fixture.Service.CreateFilmVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-visual-qc-parallel-quote", CreateFilmVisualQCQuoteRequest{SourceAttemptID: source.Attempt.ID, LogicalModelID: logicalModel.ID},
	)
	if err != nil {
		t.Fatalf("create parallel visual QC quote: %v", err)
	}
	if _, err := fixture.Service.SubmitFilmVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, parallelQuote.ID, "film-visual-qc-parallel-submit", SubmitFilmVisualQCQuoteRequest{QuoteFingerprint: parallelQuote.QuoteFingerprint},
	); authStatus(err) != http.StatusConflict {
		t.Fatalf("parallel visual QC submit error = %v, want 409", err)
	}
	cancelled, err := fixture.Service.CancelTask(context.Background(), fixture.Project.UserID, submitted.Attempt.Attempt.TaskID)
	if err != nil || cancelled.Status != model.TaskStatusCancelled {
		t.Fatalf("cancel queued visual QC task = %#v, error = %v", cancelled, err)
	}
	views, err := fixture.Service.ListFilmProductionAttempts(fixture.Project.UserID, fixture.Project.ID, fixture.RootRun.ID, fixture.Shot.ID, 10)
	if err != nil || len(views) != 1 || len(views[0].VisualQCAttempts) != 1 {
		t.Fatalf("recover cancelled visual QC = %#v, error = %v", views, err)
	}
	visualAttempt := views[0].VisualQCAttempts[0]
	if visualAttempt.Attempt.Status != model.FilmProductionAttemptStatusCancelled || visualAttempt.Cost.Status != model.BillingStatusRefunded ||
		visualAttempt.Cost.RefundedAmountMicrocredits != 180 {
		t.Fatalf("cancelled visual QC facts = %#v", visualAttempt)
	}
}

func TestFilmVisualQCInvalidProviderOutputBecomesUncertainWithoutRetry(t *testing.T) {
	fixture := newFilmProductionTestFixture(t)
	requests := &atomic.Int64{}
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": `{"schemaVersion":1,"overallDecision":"PASS","dimensions":[],"summary":"incomplete"}`}}},
		})
	}))
	t.Cleanup(provider.Close)
	logicalModel, _ := addFilmVisualQCTestModel(t, fixture, provider.URL+"/v1")
	source := completeFilmVisualQCSourceImage(t, fixture)
	quote, err := fixture.Service.CreateFilmVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-visual-qc-invalid-quote", CreateFilmVisualQCQuoteRequest{SourceAttemptID: source.Attempt.ID, LogicalModelID: logicalModel.ID},
	)
	if err != nil {
		t.Fatalf("create invalid-output visual QC quote: %v", err)
	}
	submitted, err := fixture.Service.SubmitFilmVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, quote.ID, "film-visual-qc-invalid-submit", SubmitFilmVisualQCQuoteRequest{QuoteFingerprint: quote.QuoteFingerprint},
	)
	if err != nil {
		t.Fatalf("submit invalid-output visual QC quote: %v", err)
	}
	if err := fixture.Service.ProcessNextTask(); err == nil || !strings.Contains(err.Error(), "八个语义维度") {
		t.Fatalf("invalid visual QC provider output error = %v", err)
	}
	if requests.Load() != 1 {
		t.Fatalf("invalid visual QC request count = %d, want 1", requests.Load())
	}
	views, err := fixture.Service.ListFilmProductionAttempts(fixture.Project.UserID, fixture.Project.ID, fixture.RootRun.ID, fixture.Shot.ID, 10)
	if err != nil || len(views) != 1 || len(views[0].VisualQCAttempts) != 1 {
		t.Fatalf("recover invalid-output visual QC = %#v, error = %v", views, err)
	}
	visualAttempt := views[0].VisualQCAttempts[0]
	if visualAttempt.Attempt.ID != submitted.Attempt.Attempt.ID || visualAttempt.Attempt.Status != model.FilmProductionAttemptStatusUncertain ||
		visualAttempt.Cost.Status != model.BillingStatusUncertain || visualAttempt.Report != nil {
		t.Fatalf("invalid-output visual QC facts = %#v", visualAttempt)
	}
	if _, err := fixture.Service.RetryTask(fixture.Project.UserID, visualAttempt.Attempt.TaskID); authStatus(err) != http.StatusBadRequest || !strings.Contains(err.Error(), "重新报价") {
		t.Fatalf("generic visual QC retry error = %v", err)
	}
}

func TestFilmVisualQCDecisionAggregationCoversPassUncertainAndFail(t *testing.T) {
	task := model.Task{ID: "visual-qc-task", UserID: "user", ProjectID: "project", Type: model.FilmVisualQCTaskTypeImage}
	attempt := model.FilmVisualQCAttempt{
		ID: "visual-qc-attempt", TaskID: task.ID, UserID: task.UserID, ProjectID: task.ProjectID,
		RootRunID: "root", ShotID: "shot", SourceAttemptID: "source", SourceResultID: "result",
		SourceResultArtifactID: "result-artifact", SourceResultRevisionID: "result-revision", SourceResultDigest: "digest",
		RegistryID: "registry", RegistryVersion: "1", RegistryDigest: "registry-digest",
		LogicalModelID: "logical", LogicalModelRevisionID: "logical-revision", RouteID: "route", ChannelModelID: "channel-model", Model: "vision-model",
	}
	tests := []struct {
		name      string
		dimension model.FilmProductionQCDecision
		overall   model.FilmProductionQCDecision
	}{
		{name: "pass", dimension: model.FilmProductionQCDecisionPass, overall: model.FilmProductionQCDecisionPass},
		{name: "uncertain", dimension: model.FilmProductionQCDecisionUncertain, overall: model.FilmProductionQCDecisionUncertain},
		{name: "fail", dimension: model.FilmProductionQCDecisionFail, overall: model.FilmProductionQCDecisionFail},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output := filmVisualQCTestOutputForDecision(test.dimension, test.overall)
			parsed, err := parseFilmVisualQCResult(filmVisualQCProviderPayload(t, output))
			if err != nil || parsed.OverallDecision != test.overall {
				t.Fatalf("parse %s output = %#v, error = %v", test.name, parsed, err)
			}
			command, err := buildFilmVisualQCCompletion(task, attempt, parsed, time.Now().UTC())
			if err != nil || command.Report.Decision != test.overall || command.Report.Action != model.FilmProductionQCActionHold {
				t.Fatalf("build %s completion = %#v, error = %v", test.name, command.Report, err)
			}
		})
	}
}

func newFilmVisualQCTestProvider(t *testing.T, requests *atomic.Int64, sawGeneratedImageFirst *atomic.Bool, sawAgentAndSkills *atomic.Bool) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.Method != http.MethodPost || request.URL.Path != "/v1/chat/completions" {
			http.NotFound(response, request)
			return
		}
		var body map[string]any
		if json.NewDecoder(request.Body).Decode(&body) != nil {
			http.Error(response, "invalid request", http.StatusBadRequest)
			return
		}
		messages, _ := body["messages"].([]any)
		if len(messages) >= 2 {
			system, _ := messages[0].(map[string]any)
			systemText, _ := system["content"].(string)
			sawAgentAndSkills.Store(strings.Contains(systemText, "[AGENT quality_control_editor]") && strings.Contains(systemText, "[SKILL continuity-check"))
			user, _ := messages[len(messages)-1].(map[string]any)
			content, _ := user["content"].([]any)
			if len(content) >= 2 {
				imagePart, _ := content[1].(map[string]any)
				imageURL, _ := imagePart["image_url"].(map[string]any)
				url, _ := imageURL["url"].(string)
				sawGeneratedImageFirst.Store(imagePart["type"] == "image_url" && strings.HasPrefix(url, "data:image/png;base64,"))
			}
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": string(filmVisualQCTestOutput())}}},
		})
	}))
	t.Cleanup(server.Close)
	return server
}

func addFilmVisualQCTestModel(t *testing.T, fixture filmProductionTestFixture, baseURL string) (model.LogicalModel, model.ChannelModel) {
	t.Helper()
	now := time.Now().UTC()
	capabilityConfig := &ModelCapabilityConfig{
		Version: 1,
		Text: &TextCapabilityConfig{References: TextReferenceConfig{
			PromptMaxChars: 2_000_000, MaxImages: 16, MaxImageBytes: 20 * 1024 * 1024,
		}},
	}
	capabilityConfigJSON, err := json.Marshal(capabilityConfig)
	if err != nil {
		t.Fatalf("encode visual QC channel capability: %v", err)
	}
	capabilitySpec, err := CapabilitySpecFromModelCapabilityConfig(capabilityConfig, "text")
	if err != nil {
		t.Fatalf("project visual QC capability: %v", err)
	}
	capabilitySpecJSON, err := json.Marshal(capabilitySpec)
	if err != nil {
		t.Fatalf("encode visual QC logical capability: %v", err)
	}
	channel := model.ModelChannel{
		ID: "channel-film-visual-qc", UserID: "admin", Scope: model.ChannelScopeSystem, Enabled: true, Name: "Film Visual QC Provider",
		BaseURL: baseURL, AllowLocalChannel: true, APIKey: "film-visual-qc-key", APIFormat: "openai",
		ConcurrencyLimit: 2, ModelsJSON: `["film-visual-qc-provider"]`, CreatedAt: now, UpdatedAt: now,
	}
	channelModel := model.ChannelModel{
		ID: "channel-model-film-visual-qc", ChannelID: channel.ID, ModelKey: "film-visual-qc-provider", DisplayName: "Film Visual QC Provider",
		Capability: "text", Protocol: model.ChannelInterfaceChatCompletion, BillingMode: "fixed_request",
		UnitPriceMicrocredits: 180, PriceConfigured: true, Enabled: true, PriceVersion: 1,
		CapabilityConfigJSON: string(capabilityConfigJSON), CapabilityVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	revision := model.LogicalModelRevision{
		ID: "logical-revision-film-visual-qc", LogicalModelID: "logical-film-visual-qc", Version: 1,
		CapabilitySpecJSON: string(capabilitySpecJSON), DefaultOptionsJSON: `{}`, CreatedBy: "admin", CreatedAt: now,
	}
	logicalModel := model.LogicalModel{
		ID: revision.LogicalModelID, Code: "film-visual-qc", Name: "Film Visual QC", Capability: "text", Enabled: true,
		RevisionSequence: 1, ActiveRevisionID: revision.ID, PricePolicy: "unified", BillingMode: "fixed_request",
		UnitPriceMicrocredits: 180, CreatedAt: now, UpdatedAt: now,
	}
	route := model.LogicalModelRoute{
		ID: "logical-route-film-visual-qc", LogicalModelRevisionID: revision.ID, ChannelModelID: channelModel.ID,
		Enabled: true, Priority: 100, Weight: 100, CreatedAt: now, UpdatedAt: now,
	}
	for _, item := range []any{&channel, &channelModel, &logicalModel, &revision, &route} {
		if err := fixture.DB.Create(item).Error; err != nil {
			t.Fatalf("create visual QC fixture %T: %v", item, err)
		}
	}
	fixture.Service.invalidateRouteCatalog()
	return logicalModel, channelModel
}

func completeFilmVisualQCSourceImage(t *testing.T, fixture filmProductionTestFixture) *FilmProductionAttemptView {
	t.Helper()
	quote, err := fixture.Service.CreateFilmProductionImageQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-visual-qc-source-quote", fixture.quoteRequest(fixture.PromptRevision.ID, ""),
	)
	if err != nil {
		t.Fatalf("create visual QC source quote: %v", err)
	}
	submitted, err := fixture.Service.SubmitFilmProductionImageQuote(
		fixture.Project.UserID, fixture.Project.ID, quote.ID, "film-visual-qc-source-submit",
		SubmitFilmProductionImageQuoteRequest{QuoteFingerprint: quote.QuoteFingerprint},
	)
	if err != nil {
		t.Fatalf("submit visual QC source quote: %v", err)
	}
	if err := fixture.Service.ProcessNextTask(); err != nil {
		t.Fatalf("process visual QC source image: %v", err)
	}
	views, err := fixture.Service.ListFilmProductionAttempts(fixture.Project.UserID, fixture.Project.ID, fixture.RootRun.ID, fixture.Shot.ID, 10)
	if err != nil || len(views) != 1 || views[0].Attempt.ID != submitted.Attempt.Attempt.ID || views[0].Result == nil {
		t.Fatalf("recover visual QC source image = %#v, error = %v", views, err)
	}
	return &views[0]
}

func filmVisualQCTestOutput() []byte {
	return filmVisualQCTestOutputWithDecision(model.FilmProductionQCDecisionPass)
}

func filmVisualQCTestOutputWithDecision(overall model.FilmProductionQCDecision) []byte {
	return filmVisualQCTestOutputForDecision(model.FilmProductionQCDecisionPass, overall)
}

func filmVisualQCTestOutputForDecision(dimensionDecision model.FilmProductionQCDecision, overall model.FilmProductionQCDecision) []byte {
	dimensions := make([]map[string]any, 0, len(filmVisualQCDimensions))
	for index, dimension := range filmVisualQCDimensions {
		decision := model.FilmProductionQCDecisionPass
		issueCodes := []string{}
		if index == 0 && dimensionDecision != model.FilmProductionQCDecisionPass {
			decision = dimensionDecision
			issueCodes = []string{"IDENTITY_REVIEW"}
		}
		dimensions = append(dimensions, map[string]any{
			"dimension": dimension, "decision": decision, "issueCodes": issueCodes,
			"observations": []string{"可见区域未发现该维度的明确错误"}, "rationale": "当前图片证据支持通过",
		})
	}
	encoded, _ := json.Marshal(map[string]any{
		"schemaVersion": 1, "overallDecision": overall, "dimensions": dimensions, "summary": "模型八维检查完成，仍需人工裁决。",
	})
	return encoded
}

func filmVisualQCProviderPayload(t *testing.T, output []byte) []byte {
	t.Helper()
	encoded, err := json.Marshal(map[string]any{"mode": "text", "text": string(output)})
	if err != nil {
		t.Fatalf("encode visual QC provider payload: %v", err)
	}
	return encoded
}
