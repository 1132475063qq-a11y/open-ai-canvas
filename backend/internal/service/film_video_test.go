package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

func TestExtractFilmVideoPromptsRequiresShotScopedVideoPrompt(t *testing.T) {
	prompts, err := extractFilmVideoPrompts(`{"prompts":[{"shot_id":"SHOT-001","video_prompt":"camera pushes in"},{"shotId":"SHOT-002","animationPrompt":"actor turns left"}]}`)
	if err != nil {
		t.Fatalf("extractFilmVideoPrompts(): %v", err)
	}
	if prompts["SHOT-001"] != "camera pushes in" || prompts["SHOT-002"] != "actor turns left" {
		t.Fatalf("prompts = %#v", prompts)
	}
	if _, err := extractFilmVideoPrompts(`{"video_prompt":"missing shot identity"}`); err == nil {
		t.Fatal("unscoped video prompt must be rejected")
	}
}

func TestRebuildFilmContinuityLedgerRebindsStructuredIdsAfterReorder(t *testing.T) {
	at := time.Unix(1_700_000_000, 0).UTC()
	sequence := model.FilmVideoSequence{
		ID: "sequence-continuity-rebind", UserID: "user-continuity-rebind", ProjectID: "project-continuity-rebind", RootRunID: "run-continuity-rebind",
		PromptArtifactID: "prompt-artifact", PromptArtifactRevisionID: "prompt-revision", PromptArtifactDigest: "prompt-digest",
		RegistryID: "film-registry", RegistryVersion: "1.3.1", RegistryDigest: "registry-digest",
		ArtifactID: "sequence-artifact", ArtifactRevisionID: "sequence-revision",
	}
	shots := []model.Shot{
		{ID: "shot-continuity-1", Title: "First", Description: "first shot"},
		{ID: "shot-continuity-2", Title: "Second", Description: "second shot"},
	}
	slots := []model.FilmVideoSlot{
		{ID: "slot-continuity-1", UserID: sequence.UserID, ProjectID: sequence.ProjectID, RootRunID: sequence.RootRunID, SequenceID: sequence.ID, Position: 0, ShotID: shots[0].ID, Prompt: "first prompt", DurationMs: 2_000, SourceImageAttemptID: "image-attempt-1", SourceImageResultID: "image-result-1", SourceImageResourceID: "image-resource-1", SourceImageArtifactID: "image-artifact-1", SourceImageRevisionID: "image-revision-1"},
		{ID: "slot-continuity-2", UserID: sequence.UserID, ProjectID: sequence.ProjectID, RootRunID: sequence.RootRunID, SequenceID: sequence.ID, Position: 1, ShotID: shots[1].ID, Prompt: "second prompt", DurationMs: 3_000, SourceImageAttemptID: "image-attempt-2", SourceImageResultID: "image-result-2", SourceImageResourceID: "image-resource-2", SourceImageArtifactID: "image-artifact-2", SourceImageRevisionID: "image-revision-2"},
	}
	ledger, states, issues, _, revision, err := buildFilmContinuityLedger(sequence, slots, shots, at)
	if err != nil {
		t.Fatalf("buildFilmContinuityLedger(): %v", err)
	}
	updatedSlots := []model.FilmVideoSlot{slots[1], slots[0]}
	updatedSlots[0].Position = 0
	updatedSlots[1].Position = 1
	updatedShots := []model.Shot{shots[1], shots[0]}
	updatedLedger, updatedStates, updatedIssues, _, updatedRevision, err := rebuildFilmContinuityLedgerForSequenceUpdate(
		sequence, updatedSlots, updatedShots, repository.FilmContinuityLedgerDetail{Ledger: *ledger, Shots: states, Issues: issues}, at.Add(time.Minute),
	)
	if err != nil {
		t.Fatalf("rebuildFilmContinuityLedgerForSequenceUpdate(): %v", err)
	}
	if updatedLedger.ID != ledger.ID || updatedLedger.ArtifactRevisionID != updatedRevision.ID || updatedRevision.ID == revision.ID {
		t.Fatalf("updated ledger identity = %#v, revision = %#v", updatedLedger, updatedRevision)
	}
	stateIDByShot := map[string]string{states[0].ShotID: states[0].ID, states[1].ShotID: states[1].ID}
	issueIDByShot := map[string]string{issues[0].ShotID: issues[0].ID, issues[1].ShotID: issues[1].ID}
	var content filmContinuityArtifactContent
	if err := json.Unmarshal([]byte(updatedRevision.ContentJSON), &content); err != nil {
		t.Fatalf("decode updated continuity revision: %v", err)
	}
	if content.LedgerID != ledger.ID || len(content.Shots) != 2 || len(content.Issues) != 2 {
		t.Fatalf("updated continuity content = %#v", content)
	}
	for index, state := range updatedStates {
		if state.ID != stateIDByShot[state.ShotID] || content.Shots[index].ShotID != state.ShotID || content.Shots[index].StateID != state.ID {
			t.Fatalf("updated state %d = %#v, artifact state = %#v", index, state, content.Shots[index])
		}
	}
	for index, issue := range updatedIssues {
		if issue.ID != issueIDByShot[issue.ShotID] || content.Issues[index].ShotID != issue.ShotID || content.Issues[index].IssueID != issue.ID {
			t.Fatalf("updated issue %d = %#v, artifact issue = %#v", index, issue, content.Issues[index])
		}
	}
}

func TestResolveFilmVideoMusicResourceKeepsOwnedAudioSnapshot(t *testing.T) {
	fixture := newFilmProductionTestFixture(t)
	music := model.Resource{
		ID: "film-music-resource", UserID: fixture.Project.UserID, Kind: "audio", Status: model.ResourceStatusReady,
		MimeType: "audio/mpeg", DurationMs: 12_500, Size: 128,
	}
	if err := fixture.DB.Create(&music).Error; err != nil {
		t.Fatalf("create music resource: %v", err)
	}
	resourceID, durationMs, err := fixture.Service.resolveFilmVideoMusicResource(fixture.Project.UserID, music.ID)
	if err != nil || resourceID != music.ID || durationMs != music.DurationMs {
		t.Fatalf("resolve music resource = %q, %d, error = %v", resourceID, durationMs, err)
	}

	image := model.Resource{
		ID: "film-image-resource", UserID: fixture.Project.UserID, Kind: "image", Status: model.ResourceStatusReady,
		MimeType: "image/png", DurationMs: 0, Size: 128,
	}
	if err := fixture.DB.Create(&image).Error; err != nil {
		t.Fatalf("create image resource: %v", err)
	}
	if _, _, err := fixture.Service.resolveFilmVideoMusicResource(fixture.Project.UserID, image.ID); authStatus(err) != http.StatusConflict {
		t.Fatalf("non-audio music resource error = %v, want 409", err)
	}
}

func TestFilmMultiShotGoldenPathLinksRetryVideoContinuityAndRecovery(t *testing.T) {
	fixture := newFilmProductionTestFixture(t)
	now := time.Now().UTC()
	secondShot := model.Shot{
		ID: "shot-film-production-2", ProjectID: fixture.Project.ID, Title: "Reaction shot", Position: 2,
		DurationMs: 3_000, Status: "ready", CreatedAt: now, UpdatedAt: now,
	}
	if err := fixture.DB.Create(&secondShot).Error; err != nil {
		t.Fatalf("create second Film shot: %v", err)
	}

	promptArtifact, promptRevision := createFilmProductionArtifactFixture(t, fixture.Repository, fixture.Project, fixture.RootRun,
		"multi-shot-prompts", "image-prompt-pack", `{"shots":[
			{"shotId":"shot-film-production","prompt_text":"wide establishing shot, preserve the same character and costume"},
			{"shotId":"shot-film-production-2","prompt_text":"force-provider-failure reaction close-up, preserve the same character and costume"}
		]}`)
	fixture.ProviderFailureRemaining.Store(1)

	imageRequest := func(shotID string, promptRevisionID string, retryOf string) CreateFilmProductionImageQuoteRequest {
		return CreateFilmProductionImageQuoteRequest{
			RootRunID: fixture.RootRun.ID, ShotID: shotID,
			StoryboardArtifactRevisionID: fixture.StoryboardRevision.ID, PromptArtifactRevisionID: promptRevisionID,
			FeasibilityRevisionID: fixture.FeasibilityRevision.ID, LogicalModelID: fixture.LogicalModel.ID,
			RetryOfAttemptID: retryOf, Options: FilmProductionImageOptions{Size: "1024x1024", Quality: "high"},
		}
	}

	firstQuote, err := fixture.Service.CreateFilmProductionImageQuote(
		fixture.Project.UserID, fixture.Project.ID, "multi-shot-image-quote-1", imageRequest(fixture.Shot.ID, promptRevision.ID, ""),
	)
	if err != nil {
		t.Fatalf("create first image quote: %v", err)
	}
	firstSubmit, err := fixture.Service.SubmitFilmProductionImageQuote(
		fixture.Project.UserID, fixture.Project.ID, firstQuote.ID, "multi-shot-image-submit-1",
		SubmitFilmProductionImageQuoteRequest{QuoteFingerprint: firstQuote.QuoteFingerprint},
	)
	if err != nil {
		t.Fatalf("submit first image: %v", err)
	}
	if err := fixture.Service.ProcessNextTask(); err != nil {
		t.Fatalf("generate first image: %v", err)
	}
	firstImage, err := fixture.Repository.FilmProductionAttemptForUser(fixture.Project.UserID, fixture.Project.ID, firstSubmit.Attempt.Attempt.ID)
	if err != nil || firstImage.Attempt.RootRunID != fixture.RootRun.ID || firstImage.Attempt.ShotID != fixture.Shot.ID || firstImage.Attempt.Status != model.FilmProductionAttemptStatusSucceeded {
		t.Fatalf("first image lineage/status = %#v, error = %v", firstImage, err)
	}
	firstAccepted, err := fixture.Service.CreateFilmProductionHumanQC(
		fixture.Project.UserID, fixture.Project.ID, firstImage.Attempt.ID, "multi-shot-image-qc-1",
		CreateFilmProductionQCRequest{Decision: model.FilmProductionQCDecisionPass, Action: model.FilmProductionQCActionAccept, Evidence: map[string]any{"shotReviewed": true}},
	)
	if err != nil || !firstAccepted.Attempt.Accepted {
		t.Fatalf("accept first image: %v", err)
	}

	secondQuote, err := fixture.Service.CreateFilmProductionImageQuote(
		fixture.Project.UserID, fixture.Project.ID, "multi-shot-image-quote-2", imageRequest(secondShot.ID, promptRevision.ID, ""),
	)
	if err != nil {
		t.Fatalf("create second image quote: %v", err)
	}
	secondSubmit, err := fixture.Service.SubmitFilmProductionImageQuote(
		fixture.Project.UserID, fixture.Project.ID, secondQuote.ID, "multi-shot-image-submit-2",
		SubmitFilmProductionImageQuoteRequest{QuoteFingerprint: secondQuote.QuoteFingerprint},
	)
	if err != nil {
		t.Fatalf("submit second image: %v", err)
	}
	if err := fixture.Service.ProcessNextTask(); err == nil {
		t.Fatal("controlled second image provider failure must surface to the worker")
	}
	failedSecond, err := fixture.Repository.FilmProductionAttemptForUser(fixture.Project.UserID, fixture.Project.ID, secondSubmit.Attempt.Attempt.ID)
	if err != nil || failedSecond.Attempt.Status != model.FilmProductionAttemptStatusFailed || failedSecond.Billing == nil || failedSecond.Billing.Status != model.BillingStatusRefunded {
		t.Fatalf("failed second image facts = %#v, error = %v", failedSecond, err)
	}
	firstAfterFailure, err := fixture.Repository.FilmProductionAttemptForUser(fixture.Project.UserID, fixture.Project.ID, firstImage.Attempt.ID)
	if err != nil || firstAfterFailure.Result == nil || firstAfterFailure.Attempt.Status != model.FilmProductionAttemptStatusSucceeded {
		t.Fatalf("first accepted image was changed by partial failure = %#v, error = %v", firstAfterFailure, err)
	}

	retryContent := `{"shots":[
		{"shotId":"shot-film-production","prompt_text":"wide establishing shot, preserve the same character and costume"},
		{"shotId":"shot-film-production-2","prompt_text":"reaction close-up, preserve the same character and costume after the previous shot"}
	]}`
	retryRevision := &model.ProductionArtifactRevision{
		ID: "multi-shot-prompts-revision-2", Status: model.ProductionArtifactStatusLocked,
		ContentJSON: retryContent, ContentDigest: digestBytesHex([]byte(retryContent)), SourceRunID: fixture.RootRun.ID,
		SourceArtifactRefsJSON: "[]", AuthorityRefsJSON: "[]", CreatedByType: "human", CreatedByID: fixture.Project.UserID,
	}
	_, retryPromptRevision, err := fixture.Repository.CreateProductionArtifactRevision(repository.ProductionArtifactRevisionCreate{
		UserID: fixture.Project.UserID, Artifact: &promptArtifact, Revision: retryRevision, ExpectedSequence: 1, At: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("append second-shot retry Prompt revision: %v", err)
	}
	retryQuote, err := fixture.Service.CreateFilmProductionImageQuote(
		fixture.Project.UserID, fixture.Project.ID, "multi-shot-image-retry-quote-2", imageRequest(secondShot.ID, retryPromptRevision.ID, failedSecond.Attempt.ID),
	)
	if err != nil {
		t.Fatalf("create second image retry quote: %v", err)
	}
	retriedImage, err := fixture.Service.SubmitFilmProductionImageQuote(
		fixture.Project.UserID, fixture.Project.ID, retryQuote.ID, "multi-shot-image-retry-submit-2",
		SubmitFilmProductionImageQuoteRequest{QuoteFingerprint: retryQuote.QuoteFingerprint},
	)
	if err != nil {
		t.Fatalf("submit second image retry: %v", err)
	}
	if retriedImage.Attempt.Attempt.Number != 2 || retriedImage.Attempt.Attempt.RetryOfAttemptID != failedSecond.Attempt.ID {
		t.Fatalf("second image retry lineage = %#v", retriedImage.Attempt.Attempt)
	}
	if err := fixture.Service.ProcessNextTask(); err != nil {
		t.Fatalf("generate retried second image: %v", err)
	}
	retriedSecond, err := fixture.Repository.FilmProductionAttemptForUser(fixture.Project.UserID, fixture.Project.ID, retriedImage.Attempt.Attempt.ID)
	if err != nil || retriedSecond.Attempt.Status != model.FilmProductionAttemptStatusSucceeded || retriedSecond.Attempt.RootRunID != fixture.RootRun.ID {
		t.Fatalf("retried second image facts = %#v, error = %v", retriedSecond, err)
	}
	if _, err := fixture.Service.CreateFilmProductionHumanQC(
		fixture.Project.UserID, fixture.Project.ID, retriedSecond.Attempt.ID, "multi-shot-image-qc-2",
		CreateFilmProductionQCRequest{Decision: model.FilmProductionQCDecisionPass, Action: model.FilmProductionQCActionAccept, Evidence: map[string]any{"shotReviewed": true}},
	); err != nil {
		t.Fatalf("accept retried second image: %v", err)
	}

	_, videoPromptRevision := createFilmProductionArtifactFixture(t, fixture.Repository, fixture.Project, fixture.RootRun,
		"multi-shot-video-prompts", "ai-video-prompts", `{"prompts":[
			{"shot_id":"shot-film-production","video_prompt":"slow establishing movement, preserve character, costume, prop and screen direction"},
			{"shot_id":"shot-film-production-2","video_prompt":"subtle reaction and slow push in, preserve character, costume, prop and screen direction"}
		]}`)
	videoModelID, videoRequestCount := addFilmVideoModel(t, fixture)
	sequence, err := fixture.Service.CreateFilmVideoSequence(
		fixture.Project.UserID, fixture.Project.ID, "multi-shot-video-sequence-1", CreateFilmVideoSequenceRequest{
			RootRunID: fixture.RootRun.ID, PromptArtifactRevisionID: videoPromptRevision.ID, Title: "Two-shot continuity test",
			AspectRatio: "9:16", TargetDurationMs: 6_000,
			Slots: []CreateFilmVideoSequenceSlotRequest{
				{ShotID: fixture.Shot.ID, SourceImageAttemptID: firstImage.Attempt.ID, DurationMs: 3_000},
				{ShotID: secondShot.ID, SourceImageAttemptID: retriedSecond.Attempt.ID, DurationMs: 3_000},
			},
		},
	)
	if err != nil {
		t.Fatalf("create two-shot video sequence: %v", err)
	}
	if sequence.Sequence.RootRunID != fixture.RootRun.ID || len(sequence.Slots) != 2 || len(sequence.Continuity.Shots) != 2 || len(sequence.Continuity.Issues) != 2 {
		t.Fatalf("two-shot sequence lineage/continuity = %#v", sequence)
	}

	videoAttempts := make([]FilmVideoAttemptView, 0, len(sequence.Slots))
	for index, slot := range sequence.Slots {
		slotSuffix := []string{"1", "2"}[index]
		quote, err := fixture.Service.CreateFilmVideoQuote(
			fixture.Project.UserID, fixture.Project.ID, "multi-shot-video-quote-"+slotSuffix,
			CreateFilmVideoQuoteRequest{SequenceID: sequence.Sequence.ID, SlotID: slot.Slot.ID, LogicalModelID: videoModelID, Options: FilmVideoOptions{Resolution: "720p"}},
		)
		if err != nil {
			t.Fatalf("create video quote %d: %v", index+1, err)
		}
		submitted, err := fixture.Service.SubmitFilmVideoQuote(
			fixture.Project.UserID, fixture.Project.ID, quote.ID, "multi-shot-video-submit-"+slotSuffix,
			SubmitFilmVideoQuoteRequest{QuoteFingerprint: quote.QuoteFingerprint},
		)
		if err != nil {
			t.Fatalf("submit video %d: %v", index+1, err)
		}
		if err := fixture.Service.ProcessNextTask(); err != nil {
			t.Fatalf("generate video %d: %v", index+1, err)
		}
		videoDetail, err := fixture.Repository.FilmVideoAttemptForUser(fixture.Project.UserID, fixture.Project.ID, submitted.Attempt.Attempt.ID)
		if err != nil || videoDetail.Attempt.RootRunID != fixture.RootRun.ID || videoDetail.Attempt.SequenceID != sequence.Sequence.ID || videoDetail.Attempt.Status != model.FilmProductionAttemptStatusSucceeded {
			t.Fatalf("video %d facts = %#v, error = %v", index+1, videoDetail, err)
		}
		videoAttempts = append(videoAttempts, submitted.Attempt)
	}
	if videoRequestCount.Load() != 2 {
		t.Fatalf("video provider request count = %d, want 2", videoRequestCount.Load())
	}

	for index, attempt := range videoAttempts {
		slotSuffix := []string{"1", "2"}[index]
		accepted, err := fixture.Service.CreateFilmVideoHumanQC(
			fixture.Project.UserID, fixture.Project.ID, attempt.Attempt.ID, "multi-shot-video-qc-"+slotSuffix,
			CreateFilmProductionQCRequest{Decision: model.FilmProductionQCDecisionPass, Action: model.FilmProductionQCActionAccept, Evidence: map[string]any{"shotReviewed": true}},
		)
		if err != nil || !accepted.Attempt.Accepted {
			t.Fatalf("accept video %d = %#v, error = %v", index+1, accepted, err)
		}
	}

	dimensions := map[string]any{}
	for _, dimension := range filmContinuityDimensions {
		dimensions[dimension] = "PASS"
	}
	reviewed, err := fixture.Service.CreateFilmVideoSequenceReview(
		fixture.Project.UserID, fixture.Project.ID, sequence.Sequence.ID, "multi-shot-video-review-1", CreateFilmVideoSequenceReviewRequest{
			Decision: model.FilmVideoSequenceReviewDecisionPass, Action: model.FilmVideoSequenceReviewActionAccept,
			Evidence: map[string]any{"dimensions": dimensions, "shots": map[string]any{fixture.Shot.ID: "PASS", secondShot.ID: "PASS"}},
		},
	)
	if err != nil || !reviewed.Review.Valid || reviewed.Sequence.Sequence.Status != model.FilmVideoSequenceStatusCompleted || reviewed.Sequence.Continuity.Ledger.Status != model.FilmContinuityLedgerStatusReady {
		t.Fatalf("two-shot continuity review = %#v, error = %v", reviewed, err)
	}
	restored, err := fixture.Service.ListFilmVideoSequences(fixture.Project.UserID, fixture.Project.ID, fixture.RootRun.ID, 10)
	if err != nil || len(restored) != 1 || restored[0].Sequence.RootRunID != fixture.RootRun.ID || len(restored[0].Slots) != 2 || restored[0].SequenceReview == nil || !restored[0].SequenceReview.Valid {
		t.Fatalf("restored two-shot sequence = %#v, error = %v", restored, err)
	}
}

func TestFilmVideoGoldenPathRequiresAcceptedImageAndPersistsQC(t *testing.T) {
	fixture := newFilmProductionTestFixture(t)
	imageQuote, err := fixture.Service.CreateFilmProductionImageQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-video-image-quote-0001", fixture.quoteRequest(fixture.PromptRevision.ID, ""),
	)
	if err != nil {
		t.Fatalf("create source image quote: %v", err)
	}
	imageSubmit, err := fixture.Service.SubmitFilmProductionImageQuote(
		fixture.Project.UserID, fixture.Project.ID, imageQuote.ID, "film-video-image-submit-0001",
		SubmitFilmProductionImageQuoteRequest{QuoteFingerprint: imageQuote.QuoteFingerprint},
	)
	if err != nil {
		t.Fatalf("submit source image: %v", err)
	}
	if err := fixture.Service.ProcessNextTask(); err != nil {
		t.Fatalf("generate source image: %v", err)
	}
	_, videoPromptRevision := createFilmProductionArtifactFixture(t, fixture.Repository, fixture.Project, fixture.RootRun,
		"video-prompts", "ai-video-prompts", `{"prompts":[{"shot_id":"shot-film-production","video_prompt":"subtle breathing, slow push in, preserve identity"}]}`)
	sequenceRequest := CreateFilmVideoSequenceRequest{
		RootRunID: fixture.RootRun.ID, PromptArtifactRevisionID: videoPromptRevision.ID, AspectRatio: "9:16", TargetDurationMs: 3_000,
		Slots: []CreateFilmVideoSequenceSlotRequest{{ShotID: fixture.Shot.ID, SourceImageAttemptID: imageSubmit.Attempt.Attempt.ID, DurationMs: 3_000}},
	}
	if _, err := fixture.Service.CreateFilmVideoSequence(
		fixture.Project.UserID, fixture.Project.ID, "film-video-sequence-before-qc", sequenceRequest,
	); authStatus(err) != http.StatusConflict {
		t.Fatalf("unaccepted source image sequence error = %v, want 409", err)
	}
	if _, err := fixture.Service.CreateFilmProductionHumanQC(
		fixture.Project.UserID, fixture.Project.ID, imageSubmit.Attempt.Attempt.ID, "film-video-source-qc-0001",
		CreateFilmProductionQCRequest{Decision: model.FilmProductionQCDecisionPass, Action: model.FilmProductionQCActionAccept},
	); err != nil {
		t.Fatalf("accept source image: %v", err)
	}
	videoModelID, requestCount := addFilmVideoModel(t, fixture)
	sequence, err := fixture.Service.CreateFilmVideoSequence(
		fixture.Project.UserID, fixture.Project.ID, "film-video-sequence-0001", sequenceRequest,
	)
	if err != nil {
		t.Fatalf("CreateFilmVideoSequence(): %v", err)
	}
	if len(sequence.Slots) != 1 || sequence.Sequence.Status != model.FilmVideoSequenceStatusReady || sequence.Sequence.AspectRatio != "9:16" {
		t.Fatalf("sequence = %#v", sequence)
	}
	if sequence.Continuity == nil || sequence.Continuity.Ledger.MediaState != model.FilmContinuityMediaStateStructuredOnly ||
		sequence.Continuity.Ledger.Status != model.FilmContinuityLedgerStatusNeedsYou || len(sequence.Continuity.Shots) != 1 ||
		len(sequence.Continuity.Issues) != 1 || sequence.Continuity.Issues[0].Code != "SEMANTIC_CONTINUITY_UNVERIFIED" {
		t.Fatalf("continuity Ledger = %#v", sequence.Continuity)
	}
	replayed, err := fixture.Service.CreateFilmVideoSequence(
		fixture.Project.UserID, fixture.Project.ID, "film-video-sequence-0001", sequenceRequest,
	)
	if err != nil || !replayed.Idempotent || replayed.Sequence.ID != sequence.Sequence.ID {
		t.Fatalf("sequence replay = %#v, error = %v", replayed, err)
	}
	quote, err := fixture.Service.CreateFilmVideoQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-video-quote-0001",
		CreateFilmVideoQuoteRequest{SequenceID: sequence.Sequence.ID, SlotID: sequence.Slots[0].Slot.ID, LogicalModelID: videoModelID, Options: FilmVideoOptions{Resolution: "720p"}},
	)
	if err != nil {
		t.Fatalf("CreateFilmVideoQuote(): %v", err)
	}
	if !quote.Cost.Required || quote.Cost.AmountMicrocredits != 300 || quote.DurationMs != 3_000 || quote.AspectRatio != "9:16" {
		t.Fatalf("video quote = %#v", quote)
	}
	submitted, err := fixture.Service.SubmitFilmVideoQuote(
		fixture.Project.UserID, fixture.Project.ID, quote.ID, "film-video-submit-0001",
		SubmitFilmVideoQuoteRequest{QuoteFingerprint: quote.QuoteFingerprint},
	)
	if err != nil {
		t.Fatalf("SubmitFilmVideoQuote(): %v", err)
	}
	if submitted.Attempt.Attempt.Status != model.FilmProductionAttemptStatusQueued || submitted.Attempt.Attempt.Number != 1 {
		t.Fatalf("submitted video Attempt = %#v", submitted)
	}
	if _, err := fixture.Service.RetryTask(fixture.Project.UserID, submitted.Attempt.Task.ID); authStatus(err) != http.StatusBadRequest {
		t.Fatalf("generic video retry error = %v", err)
	}
	if err := fixture.Service.ProcessNextTask(); err != nil {
		t.Fatalf("generate Film video: %v", err)
	}
	if requestCount.Load() != 1 {
		t.Fatalf("video provider create requests = %d, want 1", requestCount.Load())
	}
	detail, err := fixture.Repository.FilmVideoAttemptForUser(fixture.Project.UserID, fixture.Project.ID, submitted.Attempt.Attempt.ID)
	if err != nil {
		t.Fatalf("load video Attempt: %v", err)
	}
	if detail.Attempt.Status != model.FilmProductionAttemptStatusSucceeded || detail.Task.Status != model.TaskStatusSucceeded ||
		detail.Result == nil || detail.Result.Kind != model.ResultKindFilmVideoGeneration || !strings.HasPrefix(detail.Result.URL, "/api/resources/") ||
		len(detail.QCReports) != 1 || detail.QCReports[0].Decision != model.FilmProductionQCDecisionNotAssessable ||
		detail.QCReports[0].AssessmentKind != "technical_media_qc" || detail.QCReports[0].MediaState != model.FilmContinuityMediaStateAvailable ||
		!strings.Contains(detail.QCReports[0].IssueCodesJSON, "VISUAL_SEMANTIC_NOT_ASSESSED") {
		t.Fatalf("video completion facts = %#v", detail)
	}
	afterGeneration, err := fixture.Repository.FilmVideoSequenceForUser(fixture.Project.UserID, fixture.Project.ID, sequence.Sequence.ID)
	if err != nil || afterGeneration.Sequence.Status != model.FilmVideoSequenceStatusNeedsReview || afterGeneration.Slots[0].Slot.Status != model.FilmVideoSlotStatusNeedsReview || len(afterGeneration.ReworkEvents) != 0 {
		t.Fatalf("sequence after generation = %#v, error = %v", afterGeneration, err)
	}
	accepted, err := fixture.Service.CreateFilmVideoHumanQC(
		fixture.Project.UserID, fixture.Project.ID, detail.Attempt.ID, "film-video-qc-accept-0001",
		CreateFilmProductionQCRequest{Decision: model.FilmProductionQCDecisionPass, Action: model.FilmProductionQCActionAccept},
	)
	if err != nil || !accepted.Attempt.Accepted {
		t.Fatalf("accept Film video = %#v, error = %v", accepted, err)
	}
	awaitingSequenceReview, err := fixture.Repository.FilmVideoSequenceForUser(fixture.Project.UserID, fixture.Project.ID, sequence.Sequence.ID)
	if err != nil || awaitingSequenceReview.Sequence.Status != model.FilmVideoSequenceStatusNeedsReview ||
		awaitingSequenceReview.Slots[0].Slot.Status != model.FilmVideoSlotStatusAccepted || awaitingSequenceReview.SequenceReview != nil {
		t.Fatalf("sequence awaiting continuity review = %#v, error = %v", awaitingSequenceReview, err)
	}
	dimensions := map[string]any{}
	for _, dimension := range filmContinuityDimensions {
		dimensions[dimension] = "PASS"
	}
	sequenceReviewRequest := CreateFilmVideoSequenceReviewRequest{
		Decision: model.FilmVideoSequenceReviewDecisionPass, Action: model.FilmVideoSequenceReviewActionAccept,
		Evidence: map[string]any{"dimensions": dimensions, "shots": map[string]any{fixture.Shot.ID: "PASS"}},
	}
	reviewed, err := fixture.Service.CreateFilmVideoSequenceReview(
		fixture.Project.UserID, fixture.Project.ID, sequence.Sequence.ID, "film-video-sequence-review-0001", sequenceReviewRequest,
	)
	if err != nil || reviewed.Review.Decision != model.FilmVideoSequenceReviewDecisionPass || !reviewed.Review.Valid ||
		reviewed.Sequence.Continuity == nil || reviewed.Sequence.Continuity.Ledger.Status != model.FilmContinuityLedgerStatusReady ||
		reviewed.Sequence.Continuity.Ledger.MediaState != model.FilmContinuityMediaStateAvailable {
		t.Fatalf("sequence review = %#v, error = %v", reviewed, err)
	}
	replayedReview, err := fixture.Service.CreateFilmVideoSequenceReview(
		fixture.Project.UserID, fixture.Project.ID, sequence.Sequence.ID, "film-video-sequence-review-0001", sequenceReviewRequest,
	)
	if err != nil || !replayedReview.Idempotent || replayedReview.Review.ID != reviewed.Review.ID {
		t.Fatalf("sequence review replay = %#v, error = %v", replayedReview, err)
	}
	if err := fixture.DB.Model(&model.FilmVideoSlot{}).Where("id = ?", sequence.Slots[0].Slot.ID).
		Update("status", model.FilmVideoSlotStatusNeedsReview).Error; err != nil {
		t.Fatalf("invalidate sequence review scope: %v", err)
	}
	stale, err := fixture.Service.ListFilmVideoSequences(fixture.Project.UserID, fixture.Project.ID, sequence.Sequence.RootRunID, 10)
	if err != nil || len(stale) != 1 || stale[0].SequenceReview == nil || stale[0].SequenceReview.Valid {
		t.Fatalf("stale sequence review = %#v, error = %v", stale, err)
	}
	updated, err := fixture.Service.UpdateFilmVideoSequence(
		fixture.Project.UserID, fixture.Project.ID, sequence.Sequence.ID,
		UpdateFilmVideoSequenceRequest{
			ExpectedRevision: stale[0].Sequence.Revision, Title: stale[0].Sequence.Title, AspectRatio: stale[0].Sequence.AspectRatio,
			TargetDurationMs: 4_000, Slots: []UpdateFilmVideoSequenceSlotRequest{{SlotID: sequence.Slots[0].Slot.ID, DurationMs: 4_000}},
		},
	)
	if err != nil || updated.Sequence.Revision != stale[0].Sequence.Revision+1 || updated.Sequence.ArtifactRevisionID == stale[0].Sequence.ArtifactRevisionID ||
		updated.Continuity.Ledger.ArtifactRevisionID == stale[0].Continuity.Ledger.ArtifactRevisionID || updated.Slots[0].Slot.Status != model.FilmVideoSlotStatusReady ||
		updated.Slots[0].Slot.CurrentAttemptID != "" || updated.Slots[0].Slot.Revision != 2 {
		t.Fatalf("updated video sequence = %#v, error = %v", updated, err)
	}
	if updated.Slots[0].Attempts[0].Accepted || updated.SequenceReview == nil || updated.SequenceReview.Valid {
		t.Fatalf("updated video sequence should stale the old attempt and review = %#v", updated)
	}
	updatedQuote, err := fixture.Service.CreateFilmVideoQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-video-quote-after-plan-update",
		CreateFilmVideoQuoteRequest{SequenceID: updated.Sequence.ID, SlotID: updated.Slots[0].Slot.ID, LogicalModelID: videoModelID, Options: FilmVideoOptions{Resolution: "720p"}},
	)
	if err != nil || updatedQuote.DurationMs != 4_000 || updatedQuote.RetryOfAttemptID != "" {
		t.Fatalf("quote after plan update = %#v, error = %v", updatedQuote, err)
	}
	if _, err := fixture.Service.UpdateFilmVideoSequence(
		fixture.Project.UserID, fixture.Project.ID, sequence.Sequence.ID,
		UpdateFilmVideoSequenceRequest{ExpectedRevision: stale[0].Sequence.Revision},
	); authStatus(err) != http.StatusConflict {
		t.Fatalf("stale plan update error = %v, want 409", err)
	}
}

func TestEvaluateFilmVideoTechnicalQCCreatesOnlyEvidenceBasedFailures(t *testing.T) {
	sequence := model.FilmVideoSequence{ID: "sequence-1", AspectRatio: "9:16"}
	slot := model.FilmVideoSlot{ID: "slot-1", ShotID: "shot-1", DurationMs: 3_000}
	decision, issueCodes, evidence, _, hardFailure := evaluateFilmVideoTechnicalQC(
		filmProductionMediaFact{MimeType: "video/mp4", Width: 1920, Height: 1080, DurationMs: 6_000}, sequence, slot,
	)
	if decision != model.FilmProductionQCDecisionFail || hardFailure != "VIDEO_ASPECT_RATIO_MISMATCH" ||
		!slices.Contains(issueCodes, "VIDEO_ASPECT_RATIO_MISMATCH") || evidence["mediaAvailable"] != true {
		t.Fatalf("technical QC mismatch = decision %s issues %#v evidence %#v failure %s", decision, issueCodes, evidence, hardFailure)
	}
	decision, issueCodes, _, _, hardFailure = evaluateFilmVideoTechnicalQC(
		filmProductionMediaFact{MimeType: "video/mp4", Width: 1080, Height: 1920, DurationMs: 3_000}, sequence, slot,
	)
	if decision != model.FilmProductionQCDecisionNotAssessable || hardFailure != "" ||
		!slices.Contains(issueCodes, "VISUAL_SEMANTIC_NOT_ASSESSED") || slices.Contains(issueCodes, "VIDEO_ASPECT_RATIO_MISMATCH") {
		t.Fatalf("technical QC pass boundary = decision %s issues %#v failure %s", decision, issueCodes, hardFailure)
	}
}

func TestBuildFilmVideoCompletionCreatesMinimalReworkEvent(t *testing.T) {
	task := model.Task{ID: "task-1", UserID: "user-1", ProjectID: "project-1", Type: model.FilmProductionTaskTypeVideo}
	attempt := model.FilmVideoAttempt{
		ID: "attempt-1", UserID: task.UserID, ProjectID: task.ProjectID, RootRunID: "run-1", SequenceID: "sequence-1",
		SlotID: "slot-1", TaskID: task.ID, AttemptArtifactID: "attempt-artifact", AttemptRevisionID: "attempt-revision",
		PromptArtifactID: "prompt-artifact", PromptRevisionID: "prompt-revision", PromptDigest: "prompt-digest",
	}
	sequence := model.FilmVideoSequence{ID: attempt.SequenceID, AspectRatio: "9:16"}
	slot := model.FilmVideoSlot{ID: attempt.SlotID, SequenceID: attempt.SequenceID, ShotID: "shot-1", DurationMs: 3_000}
	raw := []byte(`{"mode":"video","video":{"url":"/api/resources/video-1/file","resourceId":"video-1","storageKey":"resource:video-1","mimeType":"video/mp4","width":1920,"height":1080,"durationMs":3000}}`)
	_, command, err := buildFilmVideoCompletion(task, attempt, sequence, slot, raw, time.Now().UTC())
	if err != nil {
		t.Fatalf("buildFilmVideoCompletion(): %v", err)
	}
	if command.QCReport.Decision != model.FilmProductionQCDecisionFail || command.ReworkEvent == nil ||
		command.QCReport.ReworkEventID != command.ReworkEvent.ID || command.ReworkEvent.SlotID != slot.ID ||
		command.ReworkEvent.ReasonCode != "VIDEO_ASPECT_RATIO_MISMATCH" || command.ReworkArtifact == nil || command.ReworkRevision == nil {
		t.Fatalf("technical rework command = %#v", command)
	}
}

func TestFilmVideoTechnicalQCFailureBlocksHumanAcceptance(t *testing.T) {
	reports := []model.FilmVideoQCReport{{
		Source: "system", AssessmentKind: "technical_media_qc", Decision: model.FilmProductionQCDecisionFail,
	}}
	if !filmVideoHasBlockingTechnicalQC(reports) {
		t.Fatal("technical media QC failure must block human acceptance")
	}
	reports[0].Decision = model.FilmProductionQCDecisionNotAssessable
	if filmVideoHasBlockingTechnicalQC(reports) {
		t.Fatal("NOT_ASSESSABLE technical QC must remain eligible for human media review")
	}
}

func TestFilmVideoQuoteRejectsExpiryAndSourceDriftAndRequiresPaidRetry(t *testing.T) {
	fixture := newFilmProductionTestFixture(t)
	imageAttemptID, videoPromptRevisionID := createAcceptedFilmVideoSource(t, fixture)
	videoModelID, _ := addFilmVideoModel(t, fixture)
	sequence, err := fixture.Service.CreateFilmVideoSequence(
		fixture.Project.UserID, fixture.Project.ID, "film-video-guard-sequence-0001",
		CreateFilmVideoSequenceRequest{
			RootRunID: fixture.RootRun.ID, PromptArtifactRevisionID: videoPromptRevisionID,
			AspectRatio: "9:16", TargetDurationMs: 3_000,
			Slots: []CreateFilmVideoSequenceSlotRequest{{ShotID: fixture.Shot.ID, SourceImageAttemptID: imageAttemptID, DurationMs: 3_000}},
		},
	)
	if err != nil {
		t.Fatalf("create guarded video sequence: %v", err)
	}
	slot := sequence.Slots[0].Slot
	quoteRequest := CreateFilmVideoQuoteRequest{
		SequenceID: sequence.Sequence.ID, SlotID: slot.ID, LogicalModelID: videoModelID,
		Options: FilmVideoOptions{Resolution: "720p"},
	}

	expiredQuote, err := fixture.Service.CreateFilmVideoQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-video-expired-quote-0001", quoteRequest,
	)
	if err != nil {
		t.Fatalf("create expiring video quote: %v", err)
	}
	if err := fixture.DB.Model(&model.FilmVideoQuote{}).Where("id = ?", expiredQuote.ID).
		Update("expires_at", time.Now().UTC().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("expire video quote: %v", err)
	}
	if _, err := fixture.Service.SubmitFilmVideoQuote(
		fixture.Project.UserID, fixture.Project.ID, expiredQuote.ID, "film-video-expired-submit-0001",
		SubmitFilmVideoQuoteRequest{QuoteFingerprint: expiredQuote.QuoteFingerprint},
	); authStatus(err) != http.StatusConflict {
		t.Fatalf("expired video quote error = %v, want 409", err)
	}
	var persistedExpired model.FilmVideoQuote
	if err := fixture.DB.First(&persistedExpired, "id = ?", expiredQuote.ID).Error; err != nil || persistedExpired.Status != model.FilmProductionQuoteStatusExpired {
		t.Fatalf("expired video quote = %#v, error = %v", persistedExpired, err)
	}

	driftQuote, err := fixture.Service.CreateFilmVideoQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-video-drift-quote-0001", quoteRequest,
	)
	if err != nil {
		t.Fatalf("create source drift quote: %v", err)
	}
	if err := fixture.DB.Model(&model.Resource{}).Where("id = ?", slot.SourceImageResourceID).
		Update("status", model.ResourceStatusDeleted).Error; err != nil {
		t.Fatalf("invalidate video source image: %v", err)
	}
	if _, err := fixture.Service.SubmitFilmVideoQuote(
		fixture.Project.UserID, fixture.Project.ID, driftQuote.ID, "film-video-drift-submit-0001",
		SubmitFilmVideoQuoteRequest{QuoteFingerprint: driftQuote.QuoteFingerprint},
	); authStatus(err) != http.StatusConflict {
		t.Fatalf("source drift submit error = %v, want 409", err)
	}
	if err := fixture.DB.Model(&model.Resource{}).Where("id = ?", slot.SourceImageResourceID).
		Update("status", model.ResourceStatusReady).Error; err != nil {
		t.Fatalf("restore video source image: %v", err)
	}

	quote, err := fixture.Service.CreateFilmVideoQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-video-retry-source-quote-0001", quoteRequest,
	)
	if err != nil {
		t.Fatalf("create first video quote: %v", err)
	}
	first, err := fixture.Service.SubmitFilmVideoQuote(
		fixture.Project.UserID, fixture.Project.ID, quote.ID, "film-video-retry-source-submit-0001",
		SubmitFilmVideoQuoteRequest{QuoteFingerprint: quote.QuoteFingerprint},
	)
	if err != nil {
		t.Fatalf("submit first video quote: %v", err)
	}
	if err := fixture.Service.ProcessNextTask(); err != nil {
		t.Fatalf("generate first retry source video: %v", err)
	}
	failedQC, err := fixture.Service.CreateFilmVideoHumanQC(
		fixture.Project.UserID, fixture.Project.ID, first.Attempt.Attempt.ID, "film-video-retry-qc-0001",
		CreateFilmProductionQCRequest{
			Decision: model.FilmProductionQCDecisionFail, Action: model.FilmProductionQCActionRetry,
			IssueCodes: []string{"CONTINUITY_MISMATCH"}, Note: "人物动作连续性需要修复",
		},
	)
	if err != nil || !failedQC.Attempt.RetryAllowed {
		t.Fatalf("video QC did not open paid retry: result=%#v error=%v", failedQC, err)
	}
	quoteRequest.RetryOfAttemptID = first.Attempt.Attempt.ID
	retryQuote, err := fixture.Service.CreateFilmVideoQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-video-paid-retry-quote-0001", quoteRequest,
	)
	if err != nil {
		t.Fatalf("create paid video retry quote: %v", err)
	}
	if retryQuote.RetryOfAttemptID != first.Attempt.Attempt.ID || !retryQuote.Cost.Required || retryQuote.Cost.AmountMicrocredits != 300 {
		t.Fatalf("paid video retry quote = %#v", retryQuote)
	}
	retried, err := fixture.Service.SubmitFilmVideoQuote(
		fixture.Project.UserID, fixture.Project.ID, retryQuote.ID, "film-video-paid-retry-submit-0001",
		SubmitFilmVideoQuoteRequest{QuoteFingerprint: retryQuote.QuoteFingerprint},
	)
	if err != nil {
		t.Fatalf("submit paid video retry: %v", err)
	}
	if retried.Attempt.Attempt.Number != 2 || retried.Attempt.Attempt.RetryOfAttemptID != first.Attempt.Attempt.ID {
		t.Fatalf("paid video retry Attempt = %#v", retried)
	}
	cancelled, err := fixture.Service.CancelTask(context.Background(), fixture.Project.UserID, retried.Attempt.Task.ID)
	if err != nil || cancelled.Status != model.TaskStatusCancelled {
		t.Fatalf("cancel queued video retry = %#v, error = %v", cancelled, err)
	}
	cancelledDetail, err := fixture.Repository.FilmVideoAttemptForUser(fixture.Project.UserID, fixture.Project.ID, retried.Attempt.Attempt.ID)
	if err != nil || cancelledDetail.Attempt.Status != model.FilmProductionAttemptStatusCancelled ||
		cancelledDetail.Billing == nil || cancelledDetail.Billing.Status != model.BillingStatusRefunded {
		t.Fatalf("cancelled video retry facts = %#v, error = %v", cancelledDetail, err)
	}
	refreshed, err := fixture.Repository.FilmVideoSequenceForUser(fixture.Project.UserID, fixture.Project.ID, sequence.Sequence.ID)
	if err != nil || refreshed.Slots[0].Slot.Status != model.FilmVideoSlotStatusCancelled {
		t.Fatalf("cancelled video slot = %#v, error = %v", refreshed, err)
	}
}

func createAcceptedFilmVideoSource(t *testing.T, fixture filmProductionTestFixture) (string, string) {
	t.Helper()
	imageQuote, err := fixture.Service.CreateFilmProductionImageQuote(
		fixture.Project.UserID, fixture.Project.ID, "film-video-guard-image-quote-0001", fixture.quoteRequest(fixture.PromptRevision.ID, ""),
	)
	if err != nil {
		t.Fatalf("create guarded source image quote: %v", err)
	}
	imageSubmit, err := fixture.Service.SubmitFilmProductionImageQuote(
		fixture.Project.UserID, fixture.Project.ID, imageQuote.ID, "film-video-guard-image-submit-0001",
		SubmitFilmProductionImageQuoteRequest{QuoteFingerprint: imageQuote.QuoteFingerprint},
	)
	if err != nil {
		t.Fatalf("submit guarded source image: %v", err)
	}
	if err := fixture.Service.ProcessNextTask(); err != nil {
		t.Fatalf("generate guarded source image: %v", err)
	}
	if _, err := fixture.Service.CreateFilmProductionHumanQC(
		fixture.Project.UserID, fixture.Project.ID, imageSubmit.Attempt.Attempt.ID, "film-video-guard-source-qc-0001",
		CreateFilmProductionQCRequest{Decision: model.FilmProductionQCDecisionPass, Action: model.FilmProductionQCActionAccept},
	); err != nil {
		t.Fatalf("accept guarded source image: %v", err)
	}
	_, promptRevision := createFilmProductionArtifactFixture(t, fixture.Repository, fixture.Project, fixture.RootRun,
		"video-prompts-guard", "ai-video-prompts", `{"prompts":[{"shot_id":"shot-film-production","video_prompt":"subtle breathing, slow push in, preserve identity"}]}`)
	return imageSubmit.Attempt.Attempt.ID, promptRevision.ID
}

func addFilmVideoModel(t *testing.T, fixture filmProductionTestFixture) (string, *atomic.Int64) {
	t.Helper()
	requestCount := &atomic.Int64{}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.Method + " " + request.URL.Path {
		case "POST /v1/videos":
			requestCount.Add(1)
			if err := request.ParseMultipartForm(4 << 20); err != nil || request.FormValue("model") != "film-video-provider" || request.FormValue("prompt") == "" {
				http.Error(w, "invalid video request", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"film-video-provider-task","status":"queued"}`))
		case "GET /v1/videos/film-video-provider-task":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"film-video-provider-task","status":"completed"}`))
		case "GET /v1/videos/film-video-provider-task/content":
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte("video"))
		default:
			http.NotFound(w, request)
		}
	}))
	t.Cleanup(provider.Close)
	now := time.Now().UTC()
	channel := model.ModelChannel{
		ID: "channel-film-video", UserID: "admin", Scope: model.ChannelScopeSystem, Enabled: true, Name: "Film Video Provider",
		BaseURL: provider.URL + "/v1", AllowLocalChannel: true, APIKey: "film-video-key", APIFormat: "openai",
		ConcurrencyLimit: 2, ModelsJSON: `["film-video-provider"]`, CreatedAt: now, UpdatedAt: now,
	}
	capabilityConfig := DefaultModelCapabilityConfigForModel(string(model.ChannelInterfaceNewAPIVideo), "film-video-provider")
	capabilityConfigJSON, err := json.Marshal(capabilityConfig)
	if err != nil {
		t.Fatal(err)
	}
	capabilitySpec, err := CapabilitySpecFromModelCapabilityConfig(capabilityConfig, "video")
	if err != nil {
		t.Fatal(err)
	}
	capabilitySpecJSON, err := json.Marshal(capabilitySpec)
	if err != nil {
		t.Fatal(err)
	}
	channelModel := model.ChannelModel{
		ID: "channel-model-film-video", ChannelID: channel.ID, ModelKey: "film-video-provider", DisplayName: "Film Video Provider",
		Capability: "video", Protocol: model.ChannelInterfaceNewAPIVideo, BillingMode: "per_second",
		UnitPriceMicrocredits: 75, PriceConfigured: true, Enabled: true, PriceVersion: 1,
		CapabilityConfigJSON: string(capabilityConfigJSON), CapabilityVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	revision := model.LogicalModelRevision{
		ID: "logical-revision-film-video", LogicalModelID: "logical-film-video", Version: 1,
		CapabilitySpecJSON: string(capabilitySpecJSON), DefaultOptionsJSON: `{}`, CreatedBy: "admin", CreatedAt: now,
	}
	logicalModel := model.LogicalModel{
		ID: revision.LogicalModelID, Code: "film-video", Name: "Film Video", Capability: "video", Enabled: true,
		RevisionSequence: 1, ActiveRevisionID: revision.ID, PricePolicy: "unified", BillingMode: "per_second",
		UnitPriceMicrocredits: 100, CreatedAt: now, UpdatedAt: now,
	}
	route := model.LogicalModelRoute{
		ID: "logical-route-film-video", LogicalModelRevisionID: revision.ID, ChannelModelID: channelModel.ID,
		Enabled: true, Priority: 100, Weight: 100, CreatedAt: now, UpdatedAt: now,
	}
	for _, item := range []any{&channel, &channelModel, &logicalModel, &revision, &route} {
		if err := fixture.DB.Create(item).Error; err != nil {
			t.Fatalf("create Film video fixture %T: %v", item, err)
		}
	}
	fixture.Service.invalidateRouteCatalog()
	return logicalModel.ID, requestCount
}
