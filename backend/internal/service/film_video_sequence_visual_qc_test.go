package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
)

func TestFilmVideoSequenceVisualQCGoldenPathIsPaidAdvisoryAndKeepsHumanAuthority(t *testing.T) {
	fixture := newFilmProductionTestFixture(t)
	sequence := completeFilmVideoSequenceVisualQCSource(t, fixture)
	requests := &atomic.Int64{}
	sawContract := &atomic.Bool{}
	provider := newFilmVideoSequenceVisualQCTestProvider(t, filmVideoSequenceVisualQCTestOutput(sequence), requests, sawContract, 4)
	logicalModel, _ := addFilmVisualQCTestModel(t, fixture, provider.URL+"/v1")
	request := filmVideoSequenceVisualQCTestRequest(t, fixture, sequence, logicalModel.ID)

	quote, err := fixture.Service.CreateFilmVideoSequenceVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-video-sequence-visual-qc-quote-1", request,
	)
	if err != nil {
		t.Fatalf("CreateFilmVideoSequenceVisualQCQuote(): %v", err)
	}
	if quote.Cost.AmountMicrocredits != 180 || len(quote.SlotEvidence) != 2 || len(quote.Dimensions) != len(filmVideoSequenceVisualQCDimensions) {
		t.Fatalf("sequence visual QC quote = %#v", quote)
	}
	protectedResourceIDs := []string{
		quote.SlotEvidence[0].SourceResourceID,
		quote.SlotEvidence[0].SourceImageResourceID,
		quote.SlotEvidence[0].Samples[0].ResourceID,
	}
	referenceSnapshot, err := fixture.Repository.ResourceReferenceSnapshot(fixture.Project.UserID, "", protectedResourceIDs)
	if err != nil {
		t.Fatalf("scan sequence visual QC resource references: %v", err)
	}
	for _, resourceID := range protectedResourceIDs {
		candidate := map[string]struct{}{resourceID: {}}
		protected := false
		for _, document := range referenceSnapshot.Documents {
			if documentReferencesResources(document.PrimaryJSON, candidate) || documentReferencesResources(document.SecondaryJSON, candidate) {
				protected = true
				break
			}
		}
		if !protected {
			t.Fatalf("sequence visual QC resource %s is not protected from deletion: %#v", resourceID, referenceSnapshot)
		}
	}
	replayed, err := fixture.Service.CreateFilmVideoSequenceVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-video-sequence-visual-qc-quote-1", request,
	)
	if err != nil || !replayed.Idempotent || replayed.ID != quote.ID {
		t.Fatalf("sequence visual QC quote replay = %#v, error = %v", replayed, err)
	}
	changed := request
	changed.Slots = append([]FilmVideoSequenceVisualQCSlotInput(nil), request.Slots...)
	changed.Slots[0].SampleFrames = append([]FilmVideoVisualQCSampleInput(nil), request.Slots[0].SampleFrames...)
	changed.Slots[0].SampleFrames[0], changed.Slots[0].SampleFrames[1] = changed.Slots[0].SampleFrames[1], changed.Slots[0].SampleFrames[0]
	if _, err := fixture.Service.CreateFilmVideoSequenceVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-video-sequence-visual-qc-quote-1", changed,
	); authStatus(err) != http.StatusBadRequest && authStatus(err) != http.StatusConflict {
		t.Fatalf("changed sequence visual QC idempotency error = %v", err)
	}
	if _, err := fixture.Service.SubmitFilmVideoSequenceVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, quote.ID, "film-video-sequence-visual-qc-submit-wrong",
		SubmitFilmVideoSequenceVisualQCQuoteRequest{QuoteFingerprint: "wrong"},
	); authStatus(err) != http.StatusConflict {
		t.Fatalf("wrong sequence visual QC fingerprint error = %v", err)
	}
	submitted, err := fixture.Service.SubmitFilmVideoSequenceVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, quote.ID, "film-video-sequence-visual-qc-submit-1",
		SubmitFilmVideoSequenceVisualQCQuoteRequest{QuoteFingerprint: quote.QuoteFingerprint},
	)
	if err != nil {
		t.Fatalf("SubmitFilmVideoSequenceVisualQCQuote(): %v", err)
	}
	if submitted.Attempt.Cost.Status != model.BillingStatusReserved || submitted.Attempt.Attempt.Number != 1 {
		t.Fatalf("submitted sequence visual QC = %#v", submitted)
	}
	if _, err := fixture.Service.RetryTask(fixture.Project.UserID, submitted.Attempt.Task.ID); authStatus(err) != http.StatusBadRequest {
		t.Fatalf("generic sequence visual QC retry error = %v", err)
	}
	if err := fixture.Service.ProcessNextTask(); err != nil {
		t.Fatalf("process sequence visual QC: %v", err)
	}
	if requests.Load() != 1 || !sawContract.Load() {
		t.Fatalf("sequence visual QC provider evidence: requests=%d contract=%v", requests.Load(), sawContract.Load())
	}
	restored := findFilmVideoSequenceView(t, fixture, sequence.Sequence.ID)
	if restored.SequenceReview != nil || restored.Sequence.Status != model.FilmVideoSequenceStatusNeedsReview || len(restored.SequenceVisualQC) != 1 {
		t.Fatalf("model sequence QC changed human authority: %#v", restored)
	}
	visualAttempt := restored.SequenceVisualQC[0]
	if visualAttempt.Attempt.Status != model.FilmProductionAttemptStatusSucceeded || !visualAttempt.Valid || visualAttempt.Report == nil ||
		visualAttempt.Report.Source != "model" || visualAttempt.Report.AssessmentKind != "sequence_visual_continuity_qc" ||
		visualAttempt.Report.ModelAttemptID != visualAttempt.Attempt.ID || visualAttempt.Report.Action != model.FilmVideoSequenceReviewActionHold ||
		visualAttempt.Report.Decision != model.FilmVideoSequenceReviewDecisionUncertain || visualAttempt.Cost.Status != model.BillingStatusSettled {
		t.Fatalf("sequence model QC facts = %#v", visualAttempt)
	}

	dimensions := map[string]any{}
	shots := map[string]any{}
	for _, dimension := range filmContinuityDimensions {
		dimensions[dimension] = "PASS"
	}
	for _, slot := range restored.Slots {
		shots[slot.Slot.ShotID] = "PASS"
	}
	if _, err := fixture.Service.CreateFilmVideoSequenceReview(
		fixture.Project.UserID, fixture.Project.ID, restored.Sequence.ID, "film-video-sequence-human-after-model-1",
		CreateFilmVideoSequenceReviewRequest{
			Decision: model.FilmVideoSequenceReviewDecisionPass, Action: model.FilmVideoSequenceReviewActionAccept,
			Evidence: map[string]any{"dimensions": dimensions, "shots": shots}, Note: "人工检查全部镜头后通过",
		},
	); err != nil {
		t.Fatalf("human sequence review after model QC: %v", err)
	}
	restored = findFilmVideoSequenceView(t, fixture, sequence.Sequence.ID)
	if restored.SequenceReview == nil || restored.SequenceReview.Source != "human" || restored.Sequence.Status != model.FilmVideoSequenceStatusCompleted ||
		len(restored.SequenceVisualQC) != 1 || restored.SequenceVisualQC[0].Report == nil || restored.SequenceVisualQC[0].Report.Source != "model" {
		t.Fatalf("human/model sequence review separation = %#v", restored)
	}

	if err := fixture.DB.Model(&model.FilmVideoSlot{}).Where("id = ?", restored.Slots[0].Slot.ID).Update("revision", restored.Slots[0].Slot.Revision+1).Error; err != nil {
		t.Fatalf("change sequence slot revision: %v", err)
	}
	restored = findFilmVideoSequenceView(t, fixture, sequence.Sequence.ID)
	if restored.SequenceVisualQC[0].Valid || restored.SequenceVisualQC[0].Report.Valid {
		t.Fatalf("old sequence model QC must be invalid after slot revision changes: %#v", restored.SequenceVisualQC[0])
	}
}

func TestFilmVideoSequenceVisualQCRejectsFrozenScopeRouteAndPriceDrift(t *testing.T) {
	fixture := newFilmProductionTestFixture(t)
	sequence := completeFilmVideoSequenceVisualQCSource(t, fixture)
	provider := newFilmVideoSequenceVisualQCTestProvider(t, filmVideoSequenceVisualQCTestOutput(sequence), &atomic.Int64{}, &atomic.Bool{}, 4)
	logicalModel, _ := addFilmVisualQCTestModel(t, fixture, provider.URL+"/v1")
	request := filmVideoSequenceVisualQCTestRequest(t, fixture, sequence, logicalModel.ID)

	sampleQuote, err := fixture.Service.CreateFilmVideoSequenceVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-video-sequence-visual-qc-sample-drift", request,
	)
	if err != nil {
		t.Fatalf("create sample-drift sequence visual QC quote: %v", err)
	}
	sampleResourceID := request.Slots[0].SampleFrames[0].ResourceID
	if err := fixture.DB.Model(&model.Resource{}).Where("id = ?", sampleResourceID).Update("status", model.ResourceStatusDeleted).Error; err != nil {
		t.Fatalf("invalidate sequence sample resource: %v", err)
	}
	if _, err := fixture.Service.SubmitFilmVideoSequenceVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, sampleQuote.ID, "film-video-sequence-visual-qc-sample-drift-submit",
		SubmitFilmVideoSequenceVisualQCQuoteRequest{QuoteFingerprint: sampleQuote.QuoteFingerprint},
	); authStatus(err) != http.StatusConflict {
		t.Fatalf("sequence sample drift submit error = %v", err)
	}
	if err := fixture.DB.Model(&model.Resource{}).Where("id = ?", sampleResourceID).Update("status", model.ResourceStatusReady).Error; err != nil {
		t.Fatalf("restore sequence sample resource: %v", err)
	}

	slotQuote, err := fixture.Service.CreateFilmVideoSequenceVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-video-sequence-visual-qc-slot-drift", request,
	)
	if err != nil {
		t.Fatalf("create slot-drift sequence visual QC quote: %v", err)
	}
	slot := sequence.Slots[0].Slot
	if err := fixture.DB.Model(&model.FilmVideoSlot{}).Where("id = ?", slot.ID).Update("revision", slot.Revision+1).Error; err != nil {
		t.Fatalf("invalidate sequence slot revision: %v", err)
	}
	if _, err := fixture.Service.SubmitFilmVideoSequenceVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, slotQuote.ID, "film-video-sequence-visual-qc-slot-drift-submit",
		SubmitFilmVideoSequenceVisualQCQuoteRequest{QuoteFingerprint: slotQuote.QuoteFingerprint},
	); authStatus(err) != http.StatusConflict {
		t.Fatalf("sequence slot drift submit error = %v", err)
	}
	if err := fixture.DB.Model(&model.FilmVideoSlot{}).Where("id = ?", slot.ID).Update("revision", slot.Revision).Error; err != nil {
		t.Fatalf("restore sequence slot revision: %v", err)
	}

	routeQuote, err := fixture.Service.CreateFilmVideoSequenceVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-video-sequence-visual-qc-route-drift", request,
	)
	if err != nil {
		t.Fatalf("create route-drift sequence visual QC quote: %v", err)
	}
	var storedRouteQuote model.FilmVideoSequenceVisualQCQuote
	if err := fixture.DB.First(&storedRouteQuote, "id = ?", routeQuote.ID).Error; err != nil {
		t.Fatalf("load route-drift sequence visual QC quote: %v", err)
	}
	if err := fixture.DB.Model(&model.LogicalModelRoute{}).Where("id = ?", storedRouteQuote.RouteID).Update("enabled", false).Error; err != nil {
		t.Fatalf("disable frozen sequence QC route: %v", err)
	}
	if _, err := fixture.Service.SubmitFilmVideoSequenceVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, routeQuote.ID, "film-video-sequence-visual-qc-route-drift-submit",
		SubmitFilmVideoSequenceVisualQCQuoteRequest{QuoteFingerprint: routeQuote.QuoteFingerprint},
	); authStatus(err) != http.StatusConflict {
		t.Fatalf("sequence route drift submit error = %v", err)
	}
	if err := fixture.DB.Model(&model.LogicalModelRoute{}).Where("id = ?", storedRouteQuote.RouteID).Update("enabled", true).Error; err != nil {
		t.Fatalf("restore frozen sequence QC route: %v", err)
	}

	priceQuote, err := fixture.Service.CreateFilmVideoSequenceVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-video-sequence-visual-qc-price-drift", request,
	)
	if err != nil {
		t.Fatalf("create price-drift sequence visual QC quote: %v", err)
	}
	var storedPriceQuote model.FilmVideoSequenceVisualQCQuote
	if err := fixture.DB.First(&storedPriceQuote, "id = ?", priceQuote.ID).Error; err != nil {
		t.Fatalf("load price-drift sequence visual QC quote: %v", err)
	}
	if err := fixture.DB.Model(&model.ChannelModel{}).Where("id = ?", storedPriceQuote.ChannelModelID).Update("price_version", storedPriceQuote.ChannelPriceVersion+1).Error; err != nil {
		t.Fatalf("change frozen sequence QC price: %v", err)
	}
	if _, err := fixture.Service.SubmitFilmVideoSequenceVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, priceQuote.ID, "film-video-sequence-visual-qc-price-drift-submit",
		SubmitFilmVideoSequenceVisualQCQuoteRequest{QuoteFingerprint: priceQuote.QuoteFingerprint},
	); authStatus(err) != http.StatusConflict {
		t.Fatalf("sequence price drift submit error = %v", err)
	}
}

func TestFilmVideoSequenceVisualQCInvalidOutputBecomesUncertainAndQueuedCancelRefunds(t *testing.T) {
	fixture := newFilmProductionTestFixture(t)
	sequence := completeFilmVideoSequenceVisualQCSource(t, fixture)
	provider := newFilmVideoSequenceVisualQCTestProvider(t, []byte(`{"schemaVersion":1,"overallDecision":"PASS","dimensions":[],"transitions":[],"summary":"bad"}`), &atomic.Int64{}, &atomic.Bool{}, 4)
	logicalModel, _ := addFilmVisualQCTestModel(t, fixture, provider.URL+"/v1")
	request := filmVideoSequenceVisualQCTestRequest(t, fixture, sequence, logicalModel.ID)

	expired, err := fixture.Service.CreateFilmVideoSequenceVisualQCQuote(fixture.Project.UserID, fixture.Project.ID, "film-video-sequence-visual-qc-expired", request)
	if err != nil {
		t.Fatalf("create expiring sequence visual QC quote: %v", err)
	}
	if err := fixture.DB.Model(&model.FilmVideoSequenceVisualQCQuote{}).Where("id = ?", expired.ID).Update("expires_at", time.Now().UTC().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("expire sequence visual QC quote: %v", err)
	}
	if _, err := fixture.Service.SubmitFilmVideoSequenceVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, expired.ID, "film-video-sequence-visual-qc-expired-submit",
		SubmitFilmVideoSequenceVisualQCQuoteRequest{QuoteFingerprint: expired.QuoteFingerprint},
	); authStatus(err) != http.StatusConflict {
		t.Fatalf("expired sequence visual QC submit error = %v", err)
	}

	cancelQuote, err := fixture.Service.CreateFilmVideoSequenceVisualQCQuote(fixture.Project.UserID, fixture.Project.ID, "film-video-sequence-visual-qc-cancel", request)
	if err != nil {
		t.Fatalf("create cancellable sequence visual QC quote: %v", err)
	}
	cancelSubmit, err := fixture.Service.SubmitFilmVideoSequenceVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, cancelQuote.ID, "film-video-sequence-visual-qc-cancel-submit",
		SubmitFilmVideoSequenceVisualQCQuoteRequest{QuoteFingerprint: cancelQuote.QuoteFingerprint},
	)
	if err != nil {
		t.Fatalf("submit cancellable sequence visual QC quote: %v", err)
	}
	parallelQuote, err := fixture.Service.CreateFilmVideoSequenceVisualQCQuote(fixture.Project.UserID, fixture.Project.ID, "film-video-sequence-visual-qc-parallel", request)
	if err != nil {
		t.Fatalf("create parallel sequence visual QC quote: %v", err)
	}
	if _, err := fixture.Service.SubmitFilmVideoSequenceVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, parallelQuote.ID, "film-video-sequence-visual-qc-parallel-submit",
		SubmitFilmVideoSequenceVisualQCQuoteRequest{QuoteFingerprint: parallelQuote.QuoteFingerprint},
	); authStatus(err) != http.StatusConflict {
		t.Fatalf("parallel sequence visual QC submit error = %v", err)
	}
	if _, err := fixture.Service.CancelTask(context.Background(), fixture.Project.UserID, cancelSubmit.Attempt.Task.ID); err != nil {
		t.Fatalf("cancel queued sequence visual QC: %v", err)
	}
	restored := findFilmVideoSequenceView(t, fixture, sequence.Sequence.ID)
	if len(restored.SequenceVisualQC) != 1 || restored.SequenceVisualQC[0].Attempt.Status != model.FilmProductionAttemptStatusCancelled ||
		restored.SequenceVisualQC[0].Cost.Status != model.BillingStatusRefunded {
		t.Fatalf("cancelled sequence visual QC facts = %#v", restored.SequenceVisualQC)
	}

	invalidQuote, err := fixture.Service.CreateFilmVideoSequenceVisualQCQuote(fixture.Project.UserID, fixture.Project.ID, "film-video-sequence-visual-qc-invalid", request)
	if err != nil {
		t.Fatalf("create invalid-output sequence visual QC quote: %v", err)
	}
	invalidSubmit, err := fixture.Service.SubmitFilmVideoSequenceVisualQCQuote(
		fixture.Project.UserID, fixture.Project.ID, invalidQuote.ID, "film-video-sequence-visual-qc-invalid-submit",
		SubmitFilmVideoSequenceVisualQCQuoteRequest{QuoteFingerprint: invalidQuote.QuoteFingerprint},
	)
	if err != nil {
		t.Fatalf("submit invalid-output sequence visual QC quote: %v", err)
	}
	if err := fixture.Service.ProcessNextTask(); err == nil || !strings.Contains(err.Error(), "9 个连续性维度") {
		t.Fatalf("invalid sequence visual QC output error = %v", err)
	}
	restored = findFilmVideoSequenceView(t, fixture, sequence.Sequence.ID)
	latest := restored.SequenceVisualQC[len(restored.SequenceVisualQC)-1]
	if latest.Attempt.ID != invalidSubmit.Attempt.Attempt.ID || latest.Attempt.Status != model.FilmProductionAttemptStatusUncertain ||
		latest.Cost.Status != model.BillingStatusUncertain || latest.Report != nil {
		t.Fatalf("invalid sequence visual QC facts = %#v", latest)
	}
}

func TestParseFilmVideoSequenceVisualQCRejectsUnsupportedAudioAndShotOrder(t *testing.T) {
	evidence := []model.FilmVideoSequenceVisualQCSlotSnapshot{{ShotID: "shot-1"}, {ShotID: "shot-2"}}
	validOutput := filmVideoSequenceVisualQCTestOutput(FilmVideoSequenceView{Slots: []FilmVideoSlotView{{Slot: model.FilmVideoSlot{ShotID: "shot-1"}}, {Slot: model.FilmVideoSlot{ShotID: "shot-2"}}}})
	validPayload := filmVideoVisualQCProviderPayload(t, validOutput)
	if output, err := parseFilmVideoSequenceVisualQCResult(validPayload, evidence); err != nil || output.OverallDecision != model.FilmProductionQCDecisionUncertain {
		t.Fatalf("valid sequence visual QC output = %#v, error = %v", output, err)
	}

	var value map[string]any
	if err := json.Unmarshal(validOutput, &value); err != nil {
		t.Fatal(err)
	}
	dimensions := value["dimensions"].([]any)
	audio := dimensions[len(dimensions)-1].(map[string]any)
	audio["decision"] = "PASS"
	audio["issueCodes"] = []any{}
	value["overallDecision"] = "PASS"
	audioPass, _ := json.Marshal(value)
	if _, err := parseFilmVideoSequenceVisualQCResult(filmVideoVisualQCProviderPayload(t, audioPass), evidence); err == nil || !strings.Contains(err.Error(), "音频证据") {
		t.Fatalf("audio PASS sequence QC error = %v", err)
	}

	if err := json.Unmarshal(validOutput, &value); err != nil {
		t.Fatal(err)
	}
	transition := value["transitions"].([]any)[0].(map[string]any)
	transition["fromShotId"], transition["toShotId"] = transition["toShotId"], transition["fromShotId"]
	wrongOrder, _ := json.Marshal(value)
	if _, err := parseFilmVideoSequenceVisualQCResult(filmVideoVisualQCProviderPayload(t, wrongOrder), evidence); err == nil || !strings.Contains(err.Error(), "镜头顺序") {
		t.Fatalf("wrong transition order error = %v", err)
	}
	unknown := []byte(`{"mode":"text","text":"{\"schemaVersion\":1,\"overallDecision\":\"UNCERTAIN\",\"dimensions\":[],\"transitions\":[],\"summary\":\"x\",\"action\":\"accept\"}"}`)
	if _, err := parseFilmVideoSequenceVisualQCResult(unknown, evidence); err == nil {
		t.Fatal("unknown sequence visual QC field was accepted")
	}
}

func completeFilmVideoSequenceVisualQCSource(t *testing.T, fixture filmProductionTestFixture) FilmVideoSequenceView {
	t.Helper()
	now := time.Now().UTC()
	secondShot := model.Shot{
		ID: "shot-film-sequence-visual-qc-2", ProjectID: fixture.Project.ID, Title: "Second shot", Position: 2,
		DurationMs: 3_000, Status: "ready", CreatedAt: now, UpdatedAt: now,
	}
	if err := fixture.DB.Create(&secondShot).Error; err != nil {
		t.Fatalf("create sequence visual QC second shot: %v", err)
	}
	_, imagePromptRevision := createFilmProductionArtifactFixture(t, fixture.Repository, fixture.Project, fixture.RootRun,
		"sequence-visual-qc-image-prompts", "image-prompt-pack", `{"shots":[
			{"shotId":"shot-film-production","prompt_text":"same actor in a red coat, wide shot"},
			{"shotId":"shot-film-sequence-visual-qc-2","prompt_text":"same actor in the same red coat, medium shot"}
		]}`)
	imageAttemptIDs := make([]string, 0, 2)
	for index, shotID := range []string{fixture.Shot.ID, secondShot.ID} {
		quote, err := fixture.Service.CreateFilmProductionImageQuote(
			fixture.Project.UserID, fixture.Project.ID, fmt.Sprintf("sequence-visual-qc-image-quote-%d", index+1),
			CreateFilmProductionImageQuoteRequest{
				RootRunID: fixture.RootRun.ID, ShotID: shotID, StoryboardArtifactRevisionID: fixture.StoryboardRevision.ID,
				PromptArtifactRevisionID: imagePromptRevision.ID, FeasibilityRevisionID: fixture.FeasibilityRevision.ID,
				LogicalModelID: fixture.LogicalModel.ID, Options: FilmProductionImageOptions{Size: "1024x1024", Quality: "high"},
			},
		)
		if err != nil {
			t.Fatalf("create sequence visual QC image quote %d: %v", index+1, err)
		}
		submitted, err := fixture.Service.SubmitFilmProductionImageQuote(
			fixture.Project.UserID, fixture.Project.ID, quote.ID, fmt.Sprintf("sequence-visual-qc-image-submit-%d", index+1),
			SubmitFilmProductionImageQuoteRequest{QuoteFingerprint: quote.QuoteFingerprint},
		)
		if err != nil {
			t.Fatalf("submit sequence visual QC image %d: %v", index+1, err)
		}
		if err := fixture.Service.ProcessNextTask(); err != nil {
			t.Fatalf("generate sequence visual QC image %d: %v", index+1, err)
		}
		if _, err := fixture.Service.CreateFilmProductionHumanQC(
			fixture.Project.UserID, fixture.Project.ID, submitted.Attempt.Attempt.ID, fmt.Sprintf("sequence-visual-qc-image-qc-%d", index+1),
			CreateFilmProductionQCRequest{Decision: model.FilmProductionQCDecisionPass, Action: model.FilmProductionQCActionAccept, Evidence: map[string]any{"reviewed": true}},
		); err != nil {
			t.Fatalf("accept sequence visual QC image %d: %v", index+1, err)
		}
		imageAttemptIDs = append(imageAttemptIDs, submitted.Attempt.Attempt.ID)
	}
	_, videoPromptRevision := createFilmProductionArtifactFixture(t, fixture.Repository, fixture.Project, fixture.RootRun,
		"sequence-visual-qc-video-prompts", "ai-video-prompts", `{"prompts":[
			{"shot_id":"shot-film-production","video_prompt":"actor walks screen right, preserve red coat and lighting"},
			{"shot_id":"shot-film-sequence-visual-qc-2","video_prompt":"continue movement screen right, preserve red coat and lighting"}
		]}`)
	videoModelID, _ := addFilmVideoModel(t, fixture)
	sequence, err := fixture.Service.CreateFilmVideoSequence(
		fixture.Project.UserID, fixture.Project.ID, "sequence-visual-qc-video-sequence",
		CreateFilmVideoSequenceRequest{
			RootRunID: fixture.RootRun.ID, PromptArtifactRevisionID: videoPromptRevision.ID, Title: "Sequence visual QC test",
			AspectRatio: "9:16", TargetDurationMs: 6_000,
			Slots: []CreateFilmVideoSequenceSlotRequest{
				{ShotID: fixture.Shot.ID, SourceImageAttemptID: imageAttemptIDs[0], DurationMs: 3_000},
				{ShotID: secondShot.ID, SourceImageAttemptID: imageAttemptIDs[1], DurationMs: 3_000},
			},
		},
	)
	if err != nil {
		t.Fatalf("create sequence visual QC video sequence: %v", err)
	}
	for index, slot := range sequence.Slots {
		quote, err := fixture.Service.CreateFilmVideoQuote(
			fixture.Project.UserID, fixture.Project.ID, fmt.Sprintf("sequence-visual-qc-video-quote-%d", index+1),
			CreateFilmVideoQuoteRequest{SequenceID: sequence.Sequence.ID, SlotID: slot.Slot.ID, LogicalModelID: videoModelID, Options: FilmVideoOptions{Resolution: "720p"}},
		)
		if err != nil {
			t.Fatalf("create sequence visual QC video quote %d: %v", index+1, err)
		}
		submitted, err := fixture.Service.SubmitFilmVideoQuote(
			fixture.Project.UserID, fixture.Project.ID, quote.ID, fmt.Sprintf("sequence-visual-qc-video-submit-%d", index+1),
			SubmitFilmVideoQuoteRequest{QuoteFingerprint: quote.QuoteFingerprint},
		)
		if err != nil {
			t.Fatalf("submit sequence visual QC video %d: %v", index+1, err)
		}
		if err := fixture.Service.ProcessNextTask(); err != nil {
			t.Fatalf("generate sequence visual QC video %d: %v", index+1, err)
		}
		if _, err := fixture.Service.CreateFilmVideoHumanQC(
			fixture.Project.UserID, fixture.Project.ID, submitted.Attempt.Attempt.ID, fmt.Sprintf("sequence-visual-qc-video-qc-%d", index+1),
			CreateFilmProductionQCRequest{Decision: model.FilmProductionQCDecisionPass, Action: model.FilmProductionQCActionAccept, Evidence: map[string]any{"reviewed": true}},
		); err != nil {
			t.Fatalf("accept sequence visual QC video %d: %v", index+1, err)
		}
	}
	return findFilmVideoSequenceView(t, fixture, sequence.Sequence.ID)
}

func filmVideoSequenceVisualQCTestRequest(t *testing.T, fixture filmProductionTestFixture, sequence FilmVideoSequenceView, logicalModelID string) CreateFilmVideoSequenceVisualQCQuoteRequest {
	t.Helper()
	slots := make([]FilmVideoSequenceVisualQCSlotInput, 0, len(sequence.Slots))
	for _, slot := range sequence.Slots {
		slots = append(slots, FilmVideoSequenceVisualQCSlotInput{
			SlotID:       slot.Slot.ID,
			SampleFrames: createFilmVideoVisualQCSamples(t, fixture, []int64{0, 3000}),
		})
	}
	return CreateFilmVideoSequenceVisualQCQuoteRequest{SequenceID: sequence.Sequence.ID, LogicalModelID: logicalModelID, Slots: slots}
}

func findFilmVideoSequenceView(t *testing.T, fixture filmProductionTestFixture, sequenceID string) FilmVideoSequenceView {
	t.Helper()
	sequences, err := fixture.Service.ListFilmVideoSequences(fixture.Project.UserID, fixture.Project.ID, fixture.RootRun.ID, 10)
	if err != nil {
		t.Fatalf("recover Film video sequences: %v", err)
	}
	for _, sequence := range sequences {
		if sequence.Sequence.ID == sequenceID {
			return sequence
		}
	}
	t.Fatalf("Film video sequence %s was not recovered", sequenceID)
	return FilmVideoSequenceView{}
}

func newFilmVideoSequenceVisualQCTestProvider(t *testing.T, output []byte, requests *atomic.Int64, sawContract *atomic.Bool, wantImages int) *httptest.Server {
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
			user, _ := messages[len(messages)-1].(map[string]any)
			content, _ := user["content"].([]any)
			imageCount := 0
			for _, rawPart := range content {
				part, _ := rawPart.(map[string]any)
				if part["type"] == "image_url" {
					imageCount++
				}
			}
			sawContract.Store(
				strings.Contains(systemText, "[AGENT quality_control_editor]") &&
					strings.Contains(systemText, "[SKILL continuity-check") &&
					strings.Contains(systemText, "lip_sync_audio_continuity") && imageCount == wantImages,
			)
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": string(output)}}},
		})
	}))
	t.Cleanup(server.Close)
	return server
}

func filmVideoSequenceVisualQCTestOutput(sequence FilmVideoSequenceView) []byte {
	dimensions := make([]map[string]any, 0, len(filmVideoSequenceVisualQCDimensions))
	for _, dimension := range filmVideoSequenceVisualQCDimensions {
		decision := model.FilmProductionQCDecisionPass
		issueCodes := []string{}
		if dimension == "lip_sync_audio_continuity" {
			decision = model.FilmProductionQCDecisionUncertain
			issueCodes = []string{"AUDIO_EVIDENCE_MISSING"}
		}
		dimensions = append(dimensions, map[string]any{
			"dimension": dimension, "decision": decision, "issueCodes": issueCodes,
			"observations": []string{"有序首尾帧中未发现该维度的明确跨镜错误"}, "rationale": "现有浏览器采样证据支持该建议结论",
		})
	}
	transitions := make([]map[string]any, 0, len(sequence.Slots)-1)
	for index := 1; index < len(sequence.Slots); index++ {
		transitions = append(transitions, map[string]any{
			"fromShotId": sequence.Slots[index-1].Slot.ShotID, "toShotId": sequence.Slots[index].Slot.ShotID,
			"decision": model.FilmProductionQCDecisionPass, "issueCodes": []string{},
			"observations": []string{"前一镜尾帧与后一镜首帧的可见状态可承接"}, "rationale": "相邻采样帧未见明确跳变",
		})
	}
	encoded, _ := json.Marshal(map[string]any{
		"schemaVersion": 1, "overallDecision": model.FilmProductionQCDecisionUncertain,
		"dimensions": dimensions, "transitions": transitions, "summary": "模型整组连续性检查完成，音频维度仍需人工裁决。",
	})
	return encoded
}
