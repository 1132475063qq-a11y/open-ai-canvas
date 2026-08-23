package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
)

func TestFilmVideoVisualQCGoldenPathFreezesEvidenceAndRequiresHumanDecision(t *testing.T) {
	fixture := newFilmProductionTestFixture(t)
	sequence, sourceAttemptID, videoModelID := completeFilmVideoVisualQCSource(t, fixture)
	requests := &atomic.Int64{}
	sawAuthority := &atomic.Bool{}
	sawSixImages := &atomic.Bool{}
	provider := newFilmVideoVisualQCTestProvider(t, filmVideoVisualQCTestOutput(model.FilmProductionQCDecisionPass), requests, sawAuthority, sawSixImages)
	logicalModel, _ := addFilmVisualQCTestModel(t, fixture, provider.URL+"/v1")
	samples := createFilmVideoVisualQCSamples(t, fixture, []int64{0, 750, 1500, 2250, 3000})

	request := CreateFilmVideoVisualQCQuoteRequest{
		SourceAttemptID: sourceAttemptID,
		LogicalModelID:  logicalModel.ID,
		SampleFrames:    samples,
	}
	quote, err := fixture.Service.CreateFilmVideoVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-video-visual-qc-quote-0001", request,
	)
	if err != nil {
		t.Fatalf("CreateFilmVideoVisualQCQuote(): %v", err)
	}
	if !quote.Cost.Required || quote.Cost.AmountMicrocredits != 180 || quote.QuoteFingerprint == "" || quote.RequestFingerprint == "" ||
		len(quote.SampleFrames) != 5 || len(quote.Dimensions) != len(filmVideoVisualQCDimensions) || quote.SourceAttemptID != sourceAttemptID ||
		!strings.Contains(quote.EvidenceLimitation, "后端未独立解码") {
		t.Fatalf("video visual QC quote did not freeze evidence and cost: %#v", quote)
	}
	var storedQuote model.FilmVideoVisualQCQuote
	if err := fixture.DB.First(&storedQuote, "id = ?", quote.ID).Error; err != nil {
		t.Fatalf("load stored video visual QC quote: %v", err)
	}
	referenceSnapshot, err := fixture.Repository.ResourceReferenceSnapshot(
		fixture.Project.UserID, "", []string{samples[0].ResourceID, storedQuote.SourceResourceID},
	)
	if err != nil {
		t.Fatalf("scan video visual QC resource references: %v", err)
	}
	if len(referenceSnapshot.Documents) == 0 || len(referenceSnapshot.Direct) == 0 {
		t.Fatalf("video visual QC evidence is not protected from resource deletion: %#v", referenceSnapshot)
	}
	replayed, err := fixture.Service.CreateFilmVideoVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-video-visual-qc-quote-0001", request,
	)
	if err != nil || !replayed.Idempotent || replayed.ID != quote.ID {
		t.Fatalf("video visual QC quote replay = %#v, error = %v", replayed, err)
	}
	changed := request
	changed.SampleFrames = append([]FilmVideoVisualQCSampleInput(nil), request.SampleFrames...)
	replacement := createFilmVideoVisualQCSamples(t, fixture, []int64{1500})[0]
	changed.SampleFrames[2].ResourceID = replacement.ResourceID
	if _, err := fixture.Service.CreateFilmVideoVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-video-visual-qc-quote-0001", changed,
	); authStatus(err) != http.StatusConflict {
		t.Fatalf("changed quote replay error = %v, want 409", err)
	}

	driftQuote, err := fixture.Service.CreateFilmVideoVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-video-visual-qc-drift-0001", request,
	)
	if err != nil {
		t.Fatalf("create sample drift quote: %v", err)
	}
	driftResourceID := samples[2].ResourceID
	if err := fixture.DB.Model(&model.Resource{}).Where("id = ?", driftResourceID).
		Update("status", model.ResourceStatusDeleted).Error; err != nil {
		t.Fatalf("invalidate sampled frame: %v", err)
	}
	if _, err := fixture.Service.SubmitFilmVideoVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, driftQuote.ID, "film-video-visual-qc-drift-submit-0001",
		SubmitFilmVideoVisualQCQuoteRequest{QuoteFingerprint: driftQuote.QuoteFingerprint},
	); authStatus(err) != http.StatusConflict {
		t.Fatalf("sample drift submit error = %v, want 409", err)
	}
	if err := fixture.DB.Model(&model.Resource{}).Where("id = ?", driftResourceID).
		Update("status", model.ResourceStatusReady).Error; err != nil {
		t.Fatalf("restore sampled frame: %v", err)
	}

	if _, err := fixture.Service.SubmitFilmVideoVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, quote.ID, "film-video-visual-qc-submit-bad-fingerprint",
		SubmitFilmVideoVisualQCQuoteRequest{QuoteFingerprint: "wrong-fingerprint"},
	); authStatus(err) != http.StatusConflict {
		t.Fatalf("unconfirmed cost fingerprint error = %v, want 409", err)
	}
	submitted, err := fixture.Service.SubmitFilmVideoVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, quote.ID, "film-video-visual-qc-submit-0001",
		SubmitFilmVideoVisualQCQuoteRequest{QuoteFingerprint: quote.QuoteFingerprint},
	)
	if err != nil {
		t.Fatalf("SubmitFilmVideoVisualQCQuote(): %v", err)
	}
	if submitted.Idempotent || submitted.Attempt.Attempt.Status != model.FilmProductionAttemptStatusQueued ||
		submitted.Attempt.Attempt.Number != 1 || submitted.Attempt.Cost.AmountMicrocredits != 180 {
		t.Fatalf("video visual QC submit facts are incomplete: %#v", submitted)
	}
	submitReplay, err := fixture.Service.SubmitFilmVideoVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, quote.ID, "film-video-visual-qc-submit-0001",
		SubmitFilmVideoVisualQCQuoteRequest{QuoteFingerprint: quote.QuoteFingerprint},
	)
	if err != nil || !submitReplay.Idempotent || submitReplay.Attempt.Attempt.ID != submitted.Attempt.Attempt.ID {
		t.Fatalf("video visual QC submit replay = %#v, error = %v", submitReplay, err)
	}
	if _, err := fixture.Service.RetryTask(fixture.Project.UserID, submitted.Attempt.Task.ID); authStatus(err) != http.StatusBadRequest {
		t.Fatalf("generic video visual QC retry error = %v, want 400", err)
	}

	if err := fixture.Service.ProcessNextTask(); err != nil {
		t.Fatalf("process video visual QC task: %v", err)
	}
	if requests.Load() != 1 || !sawAuthority.Load() || !sawSixImages.Load() {
		t.Fatalf("video visual QC provider evidence incomplete: requests=%d authority=%v sixImages=%v", requests.Load(), sawAuthority.Load(), sawSixImages.Load())
	}
	view := findFilmVideoAttemptView(t, fixture, sequence.Sequence.ID, sourceAttemptID)
	if len(view.VisualQCAttempts) != 1 {
		t.Fatalf("recovered video visual QC attempts = %#v", view.VisualQCAttempts)
	}
	visualAttempt := view.VisualQCAttempts[0]
	if visualAttempt.Attempt.Status != model.FilmProductionAttemptStatusSucceeded || !visualAttempt.Valid || visualAttempt.Report == nil ||
		visualAttempt.Report.Decision != model.FilmProductionQCDecisionPass || visualAttempt.Report.Action != model.FilmProductionQCActionHold ||
		visualAttempt.Report.Source != "model" || visualAttempt.Report.AssessmentKind != "video_visual_semantic_qc" ||
		visualAttempt.Report.ModelAttemptID != visualAttempt.Attempt.ID || view.Accepted {
		t.Fatalf("model video QC must remain advisory: %#v", view)
	}
	if dimensions, ok := visualAttempt.Report.Evidence["dimensions"].([]any); !ok || len(dimensions) != len(filmVideoVisualQCDimensions) {
		t.Fatalf("video visual QC dimensions = %#v", visualAttempt.Report.Evidence["dimensions"])
	}
	if sampling, ok := visualAttempt.Report.Evidence["sampling"].(map[string]any); !ok || sampling["method"] != "client_browser_capture" || sampling["provenanceVerifiedByServerDecoder"] != false {
		t.Fatalf("video visual QC sampling provenance = %#v", visualAttempt.Report.Evidence["sampling"])
	}
	if visualAttempt.Cost.Status != model.BillingStatusSettled || visualAttempt.Cost.ActualAmountMicrocredits != 180 {
		t.Fatalf("video visual QC billing did not settle: %#v", visualAttempt.Cost)
	}

	failedQC, err := fixture.Service.CreateFilmVideoHumanQC(
		fixture.Project.UserID, fixture.Project.ID, sourceAttemptID, "film-video-human-after-model-qc-0001",
		CreateFilmProductionQCRequest{
			Decision: model.FilmProductionQCDecisionFail, Action: model.FilmProductionQCActionRetry,
			IssueCodes: []string{"MOTION_REVIEW"}, Evidence: map[string]any{"reviewedModelEvidence": true}, Note: "人工复核后要求重做动作",
		},
	)
	if err != nil || !failedQC.Attempt.RetryAllowed || failedQC.Attempt.CurrentQC == nil || failedQC.Attempt.CurrentQC.Source != "human" {
		t.Fatalf("human decision after model video QC = %#v, error = %v", failedQC, err)
	}
	slotID := sequence.Slots[0].Slot.ID
	retryQuote, err := fixture.Service.CreateFilmVideoQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-video-after-visual-qc-retry-quote",
		CreateFilmVideoQuoteRequest{
			SequenceID: sequence.Sequence.ID, SlotID: slotID, LogicalModelID: videoModelID,
			RetryOfAttemptID: sourceAttemptID, Options: FilmVideoOptions{Resolution: "720p"},
		},
	)
	if err != nil {
		t.Fatalf("create video retry after human QC: %v", err)
	}
	retried, err := fixture.Service.SubmitFilmVideoQuote(
		fixture.Project.UserID, fixture.Project.ID, retryQuote.ID, "film-video-after-visual-qc-retry-submit",
		SubmitFilmVideoQuoteRequest{QuoteFingerprint: retryQuote.QuoteFingerprint},
	)
	if err != nil {
		t.Fatalf("submit video retry after human QC: %v", err)
	}
	if err := fixture.Service.ProcessNextTask(); err != nil {
		t.Fatalf("process video retry after human QC: %v", err)
	}
	oldView := findFilmVideoAttemptView(t, fixture, sequence.Sequence.ID, sourceAttemptID)
	if len(oldView.VisualQCAttempts) != 1 || oldView.VisualQCAttempts[0].Valid {
		t.Fatalf("old model video QC must be invalid after current version changes: %#v", oldView.VisualQCAttempts)
	}
	newView := findFilmVideoAttemptView(t, fixture, sequence.Sequence.ID, retried.Attempt.Attempt.ID)
	if newView.Attempt.Number != 2 || newView.Attempt.RetryOfAttemptID != sourceAttemptID {
		t.Fatalf("video retry lineage = %#v", newView.Attempt)
	}
}

func TestFilmVideoVisualQCInvalidOutputBecomesUncertainWithoutRetry(t *testing.T) {
	fixture := newFilmProductionTestFixture(t)
	sequence, sourceAttemptID, _ := completeFilmVideoVisualQCSource(t, fixture)
	requests := &atomic.Int64{}
	provider := newFilmVideoVisualQCTestProvider(t, []byte(`{"schemaVersion":1,"overallDecision":"PASS","dimensions":[],"summary":"incomplete"}`), requests, &atomic.Bool{}, &atomic.Bool{})
	logicalModel, _ := addFilmVisualQCTestModel(t, fixture, provider.URL+"/v1")
	samples := createFilmVideoVisualQCSamples(t, fixture, []int64{0, 750, 1500, 2250, 3000})
	quote, err := fixture.Service.CreateFilmVideoVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-video-visual-qc-invalid-quote",
		CreateFilmVideoVisualQCQuoteRequest{SourceAttemptID: sourceAttemptID, LogicalModelID: logicalModel.ID, SampleFrames: samples},
	)
	if err != nil {
		t.Fatalf("create invalid-output video visual QC quote: %v", err)
	}
	submitted, err := fixture.Service.SubmitFilmVideoVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, quote.ID, "film-video-visual-qc-invalid-submit",
		SubmitFilmVideoVisualQCQuoteRequest{QuoteFingerprint: quote.QuoteFingerprint},
	)
	if err != nil {
		t.Fatalf("submit invalid-output video visual QC quote: %v", err)
	}
	if err := fixture.Service.ProcessNextTask(); err == nil || !strings.Contains(err.Error(), "10 个语义维度") {
		t.Fatalf("invalid video visual QC provider output error = %v", err)
	}
	if requests.Load() != 1 {
		t.Fatalf("invalid video visual QC request count = %d, want 1", requests.Load())
	}
	view := findFilmVideoAttemptView(t, fixture, sequence.Sequence.ID, sourceAttemptID)
	if len(view.VisualQCAttempts) != 1 {
		t.Fatalf("invalid-output video visual QC recovery = %#v", view.VisualQCAttempts)
	}
	visualAttempt := view.VisualQCAttempts[0]
	if visualAttempt.Attempt.ID != submitted.Attempt.Attempt.ID || visualAttempt.Attempt.Status != model.FilmProductionAttemptStatusUncertain ||
		visualAttempt.Cost.Status != model.BillingStatusUncertain || visualAttempt.Report != nil {
		t.Fatalf("invalid-output video visual QC facts = %#v", visualAttempt)
	}
	if _, err := fixture.Service.RetryTask(fixture.Project.UserID, visualAttempt.Attempt.TaskID); authStatus(err) != http.StatusBadRequest || !strings.Contains(err.Error(), "重新报价") {
		t.Fatalf("generic invalid video visual QC retry error = %v", err)
	}
}

func TestFilmVideoVisualQCQueuedCancelRefundsAndRejectsParallelAttempt(t *testing.T) {
	fixture := newFilmProductionTestFixture(t)
	sequence, sourceAttemptID, _ := completeFilmVideoVisualQCSource(t, fixture)
	provider := newFilmVideoVisualQCTestProvider(t, filmVideoVisualQCTestOutput(model.FilmProductionQCDecisionPass), &atomic.Int64{}, &atomic.Bool{}, &atomic.Bool{})
	logicalModel, _ := addFilmVisualQCTestModel(t, fixture, provider.URL+"/v1")
	samples := createFilmVideoVisualQCSamples(t, fixture, []int64{0, 750, 1500, 2250, 3000})
	request := CreateFilmVideoVisualQCQuoteRequest{SourceAttemptID: sourceAttemptID, LogicalModelID: logicalModel.ID, SampleFrames: samples}
	expiredQuote, err := fixture.Service.CreateFilmVideoVisualQCQuote(fixture.Project.UserID, fixture.Project.ID, "film-video-visual-qc-expired-quote", request)
	if err != nil {
		t.Fatalf("create expiring video visual QC quote: %v", err)
	}
	if err := fixture.DB.Model(&model.FilmVideoVisualQCQuote{}).Where("id = ?", expiredQuote.ID).
		Update("expires_at", time.Now().UTC().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("expire video visual QC quote: %v", err)
	}
	if _, err := fixture.Service.SubmitFilmVideoVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, expiredQuote.ID, "film-video-visual-qc-expired-submit",
		SubmitFilmVideoVisualQCQuoteRequest{QuoteFingerprint: expiredQuote.QuoteFingerprint},
	); authStatus(err) != http.StatusConflict {
		t.Fatalf("expired video visual QC submit error = %v, want 409", err)
	}
	quote, err := fixture.Service.CreateFilmVideoVisualQCQuote(fixture.Project.UserID, fixture.Project.ID, "film-video-visual-qc-cancel-quote", request)
	if err != nil {
		t.Fatalf("create cancellable video visual QC quote: %v", err)
	}
	submitted, err := fixture.Service.SubmitFilmVideoVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, quote.ID, "film-video-visual-qc-cancel-submit",
		SubmitFilmVideoVisualQCQuoteRequest{QuoteFingerprint: quote.QuoteFingerprint},
	)
	if err != nil {
		t.Fatalf("submit cancellable video visual QC quote: %v", err)
	}
	parallelQuote, err := fixture.Service.CreateFilmVideoVisualQCQuote(fixture.Project.UserID, fixture.Project.ID, "film-video-visual-qc-parallel-quote", request)
	if err != nil {
		t.Fatalf("create parallel video visual QC quote: %v", err)
	}
	if _, err := fixture.Service.SubmitFilmVideoVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, parallelQuote.ID, "film-video-visual-qc-parallel-submit",
		SubmitFilmVideoVisualQCQuoteRequest{QuoteFingerprint: parallelQuote.QuoteFingerprint},
	); authStatus(err) != http.StatusConflict {
		t.Fatalf("parallel video visual QC submit error = %v, want 409", err)
	}
	cancelled, err := fixture.Service.CancelTask(context.Background(), fixture.Project.UserID, submitted.Attempt.Attempt.TaskID)
	if err != nil || cancelled.Status != model.TaskStatusCancelled {
		t.Fatalf("cancel queued video visual QC task = %#v, error = %v", cancelled, err)
	}
	view := findFilmVideoAttemptView(t, fixture, sequence.Sequence.ID, sourceAttemptID)
	if len(view.VisualQCAttempts) != 1 || view.VisualQCAttempts[0].Attempt.Status != model.FilmProductionAttemptStatusCancelled ||
		view.VisualQCAttempts[0].Cost.Status != model.BillingStatusRefunded || view.VisualQCAttempts[0].Cost.RefundedAmountMicrocredits != 180 {
		t.Fatalf("cancelled video visual QC facts = %#v", view.VisualQCAttempts)
	}
}

func TestParseFilmVideoVisualQCResultRejectsNonContractOutput(t *testing.T) {
	valid := filmVideoVisualQCProviderPayload(t, filmVideoVisualQCTestOutput(model.FilmProductionQCDecisionPass))
	if output, err := parseFilmVideoVisualQCResult(valid); err != nil || output.OverallDecision != model.FilmProductionQCDecisionPass {
		t.Fatalf("valid video visual QC output = %#v, error = %v", output, err)
	}
	wrongAggregate := filmVideoVisualQCTestOutput(model.FilmProductionQCDecisionUncertain)
	var wrongAggregateValue map[string]any
	if err := json.Unmarshal(wrongAggregate, &wrongAggregateValue); err != nil {
		t.Fatal(err)
	}
	wrongAggregateValue["overallDecision"] = model.FilmProductionQCDecisionPass
	wrongAggregate, _ = json.Marshal(wrongAggregateValue)
	tests := map[string][]byte{
		"markdown fence":  []byte("{\"mode\":\"text\",\"text\":\"```json\\n{}\\n```\"}"),
		"unknown field":   []byte(`{"mode":"text","text":"{\"schemaVersion\":1,\"overallDecision\":\"PASS\",\"dimensions\":[],\"summary\":\"ok\",\"action\":\"accept\"}"}`),
		"trailing json":   []byte(`{"mode":"text","text":"{} {}"}`),
		"wrong aggregate": filmVideoVisualQCProviderPayload(t, wrongAggregate),
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := parseFilmVideoVisualQCResult(raw); err == nil {
				t.Fatal("non-contract video visual QC output was accepted")
			}
		})
	}
}

func completeFilmVideoVisualQCSource(t *testing.T, fixture filmProductionTestFixture) (FilmVideoSequenceView, string, string) {
	t.Helper()
	imageAttemptID, promptRevisionID := createAcceptedFilmVideoSource(t, fixture)
	videoModelID, _ := addFilmVideoModel(t, fixture)
	sequence, err := fixture.Service.CreateFilmVideoSequence(
		fixture.Project.UserID, fixture.Project.ID, "film-video-visual-qc-sequence",
		CreateFilmVideoSequenceRequest{
			RootRunID: fixture.RootRun.ID, PromptArtifactRevisionID: promptRevisionID, AspectRatio: "9:16", TargetDurationMs: 3000,
			Slots: []CreateFilmVideoSequenceSlotRequest{{ShotID: fixture.Shot.ID, SourceImageAttemptID: imageAttemptID, DurationMs: 3000}},
		},
	)
	if err != nil {
		t.Fatalf("create video visual QC sequence: %v", err)
	}
	quote, err := fixture.Service.CreateFilmVideoQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-video-visual-qc-source-quote",
		CreateFilmVideoQuoteRequest{
			SequenceID: sequence.Sequence.ID, SlotID: sequence.Slots[0].Slot.ID, LogicalModelID: videoModelID,
			Options: FilmVideoOptions{Resolution: "720p"},
		},
	)
	if err != nil {
		t.Fatalf("create video visual QC source quote: %v", err)
	}
	submitted, err := fixture.Service.SubmitFilmVideoQuote(
		fixture.Project.UserID, fixture.Project.ID, quote.ID, "film-video-visual-qc-source-submit",
		SubmitFilmVideoQuoteRequest{QuoteFingerprint: quote.QuoteFingerprint},
	)
	if err != nil {
		t.Fatalf("submit video visual QC source: %v", err)
	}
	if err := fixture.Service.ProcessNextTask(); err != nil {
		t.Fatalf("generate video visual QC source: %v", err)
	}
	return sequence, submitted.Attempt.Attempt.ID, videoModelID
}

func createFilmVideoVisualQCSamples(t *testing.T, fixture filmProductionTestFixture, times []int64) []FilmVideoVisualQCSampleInput {
	t.Helper()
	pngBytes, err := base64.StdEncoding.DecodeString(filmProductionTestPNG(t))
	if err != nil {
		t.Fatalf("decode sampled frame fixture: %v", err)
	}
	samples := make([]FilmVideoVisualQCSampleInput, 0, len(times))
	for index, timeMs := range times {
		resource, err := fixture.Service.storeResource(
			fixture.Project.UserID, "image", "video-qc-frame.png", "image/png", int64(len(pngBytes)), 2, 2, 0, bytes.NewReader(pngBytes),
		)
		if err != nil {
			t.Fatalf("store sampled frame %d: %v", index+1, err)
		}
		samples = append(samples, FilmVideoVisualQCSampleInput{TimeMs: timeMs, ResourceID: resource.ID})
	}
	return samples
}

func findFilmVideoAttemptView(t *testing.T, fixture filmProductionTestFixture, sequenceID string, attemptID string) FilmVideoAttemptView {
	t.Helper()
	sequences, err := fixture.Service.ListFilmVideoSequences(fixture.Project.UserID, fixture.Project.ID, fixture.RootRun.ID, 10)
	if err != nil {
		t.Fatalf("recover Film video sequences: %v", err)
	}
	for _, sequence := range sequences {
		if sequence.Sequence.ID != sequenceID {
			continue
		}
		for _, slot := range sequence.Slots {
			for _, attempt := range slot.Attempts {
				if attempt.Attempt.ID == attemptID {
					return attempt
				}
			}
		}
	}
	t.Fatalf("Film video Attempt %s was not recovered", attemptID)
	return FilmVideoAttemptView{}
}

func newFilmVideoVisualQCTestProvider(t *testing.T, output []byte, requests *atomic.Int64, sawAuthority *atomic.Bool, sawSixImages *atomic.Bool) *httptest.Server {
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
			sawAuthority.Store(strings.Contains(systemText, "[AGENT quality_control_editor]") && strings.Contains(systemText, "[SKILL continuity-check") && strings.Contains(systemText, "temporal_continuity"))
			user, _ := messages[len(messages)-1].(map[string]any)
			content, _ := user["content"].([]any)
			imageCount := 0
			for _, rawPart := range content {
				part, _ := rawPart.(map[string]any)
				if part["type"] == "image_url" {
					imageCount++
				}
			}
			sawSixImages.Store(imageCount == 6)
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": string(output)}}},
		})
	}))
	t.Cleanup(server.Close)
	return server
}

func filmVideoVisualQCTestOutput(firstDecision model.FilmProductionQCDecision) []byte {
	dimensions := make([]map[string]any, 0, len(filmVideoVisualQCDimensions))
	overall := model.FilmProductionQCDecisionPass
	for index, dimension := range filmVideoVisualQCDimensions {
		decision := model.FilmProductionQCDecisionPass
		issueCodes := []string{}
		if index == 0 && firstDecision != model.FilmProductionQCDecisionPass {
			decision = firstDecision
			issueCodes = []string{"IDENTITY_REVIEW"}
			overall = firstDecision
		}
		dimensions = append(dimensions, map[string]any{
			"dimension": dimension, "decision": decision, "issueCodes": issueCodes,
			"observations": []string{"有序采样帧中未发现该维度的明确错误"}, "rationale": "现有采样证据支持该结论",
		})
	}
	encoded, _ := json.Marshal(map[string]any{
		"schemaVersion": 1, "overallDecision": overall, "dimensions": dimensions, "summary": "模型十维视频检查完成，仍需人工裁决。",
	})
	return encoded
}

func filmVideoVisualQCProviderPayload(t *testing.T, output []byte) []byte {
	t.Helper()
	encoded, err := json.Marshal(map[string]any{"mode": "text", "text": string(output)})
	if err != nil {
		t.Fatalf("encode video visual QC provider payload: %v", err)
	}
	return encoded
}
