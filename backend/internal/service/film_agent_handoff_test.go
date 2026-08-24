package service

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

func TestFilmHandoffDoesNotCreateAChildRunWhenTheTextModelBecomesUnavailable(t *testing.T) {
	svc, repo, db, project := newFilmAgentRuntimeTestService(t)
	svc.filmAgentExecutor = &recordingFilmAgentExecutor{}
	created, err := createFilmAgentRunWithTestModel(svc, project.UserID, project.ID, "film-handoff-model-loss", CreateFilmAgentRunRequest{
		Objective: "写一个原创短片故事", IntentRouteID: "IR-01",
	})
	if err != nil {
		t.Fatalf("create Film Handoff model-loss fixture: %v", err)
	}
	if processed, err := svc.ProcessNextFilmAgentStep(); !processed || err != nil {
		t.Fatalf("execute Film Handoff model-loss root = %v, %v", processed, err)
	}
	detail, err := repo.AgentRuntimeDetailForUser(project.UserID, created.Detail.Run.ID)
	if err != nil {
		t.Fatalf("load Film Handoff model-loss root: %v", err)
	}
	artifact, review := findCurrentFilmArtifact(t, detail, "script")
	locked, err := svc.LockFilmAgentArtifact(project.UserID, project.ID, detail.Run.ID, artifact.ID, LockFilmAgentArtifactRequest{
		ExpectedRunRevision: detail.Run.Revision, ExpectedArtifactSequence: artifact.RevisionSequence, ExpectedRevisionID: review.ID,
	})
	if err != nil {
		t.Fatalf("lock Film Handoff model-loss script: %v", err)
	}
	if err := db.Model(&model.LogicalModelRoute{}).Where("id = ?", filmAgentTestRouteID).Update("enabled", false).Error; err != nil {
		t.Fatalf("disable Film text route before Handoff: %v", err)
	}
	svc.invalidateRouteCatalog()
	if processed, err := svc.ProcessNextFilmAgentHandoff(); !processed || err == nil {
		t.Fatalf("Handoff with unavailable text model = %v, %v", processed, err)
	}
	runs, err := repo.ProjectAgentRuntimeRunsForDomain(project.UserID, project.ID, "film", 10)
	if err != nil {
		t.Fatalf("list Film Runs after rejected Handoff: %v", err)
	}
	if len(runs) != 1 || runs[0].ID != created.Detail.Run.ID {
		t.Fatalf("unavailable text model created a child Run: %#v", runs)
	}
	trigger, err := repo.AgentHandoffTrigger(locked.Trigger.ID)
	if err != nil {
		t.Fatalf("load rejected Film Handoff Trigger: %v", err)
	}
	if trigger.Status != model.AgentHandoffTriggerStatusPending || trigger.FailureCode != "film_handoff_processing_failed" || trigger.NextAttemptAt == nil || !strings.Contains(trigger.Failure, "NOT_AVAILABLE") {
		t.Fatalf("rejected Film Handoff did not preserve retry evidence: %#v", trigger)
	}
}

func TestLockedFilmArtifactsDriveDurableHandoffRunsWithoutDuplicateRoutes(t *testing.T) {
	svc, repo, _, project := newFilmAgentRuntimeTestService(t)
	executor := &recordingFilmAgentExecutor{}
	svc.filmAgentExecutor = executor
	created, err := createFilmAgentRunWithTestModel(svc, project.UserID, project.ID, "film-handoff-root", CreateFilmAgentRunRequest{
		Objective: "写一个原创短片故事", IntentRouteID: "IR-01", Input: map[string]any{"premise": "一场被遗忘的毕业演出"},
	})
	if err != nil {
		t.Fatalf("CreateFilmAgentRun(): %v", err)
	}
	if created.Detail.Run.RouteKind != "intent" || created.Detail.Run.RootRunID != created.Detail.Run.ID || created.Detail.Run.ParentRunID != "" {
		t.Fatalf("root Intent Run lineage is incomplete: %#v", created.Detail.Run)
	}
	if processed, err := svc.ProcessNextFilmAgentStep(); !processed || err != nil {
		t.Fatalf("execute root script = %v, %v", processed, err)
	}
	rootDetail, err := repo.AgentRuntimeDetailForUser(project.UserID, created.Detail.Run.ID)
	if err != nil {
		t.Fatalf("load root output: %v", err)
	}
	scriptArtifact, scriptReview := findCurrentFilmArtifact(t, rootDetail, "script")
	lockedScript, err := svc.LockFilmAgentArtifact(project.UserID, project.ID, rootDetail.Run.ID, scriptArtifact.ID, LockFilmAgentArtifactRequest{
		ExpectedRunRevision: rootDetail.Run.Revision, ExpectedArtifactSequence: scriptArtifact.RevisionSequence, ExpectedRevisionID: scriptReview.ID,
	})
	if err != nil {
		t.Fatalf("LockFilmAgentArtifact(script): %v", err)
	}
	if lockedScript.SourceRevision.Status != model.ProductionArtifactStatusReview ||
		lockedScript.LockedRevision.Status != model.ProductionArtifactStatusLocked || lockedScript.LockedRevision.ParentRevisionID != scriptReview.ID {
		t.Fatalf("script lock did not preserve REVIEW evidence: %#v", lockedScript)
	}
	if processed, err := svc.ProcessNextFilmAgentHandoff(); !processed || err != nil {
		t.Fatalf("process script Handoff Trigger = %v, %v", processed, err)
	}
	firstTrigger, err := repo.AgentHandoffTrigger(lockedScript.Trigger.ID)
	if err != nil {
		t.Fatalf("load completed script Trigger: %v", err)
	}
	var firstScheduled []string
	if err := json.Unmarshal([]byte(firstTrigger.ScheduledRunIDsJSON), &firstScheduled); err != nil {
		t.Fatalf("decode first scheduled Runs: %v", err)
	}
	if firstTrigger.Status != model.AgentHandoffTriggerStatusCompleted || len(firstScheduled) != 1 {
		t.Fatalf("script Trigger did not schedule exactly HR-03: %#v", firstTrigger)
	}
	directorRun := findFilmHandoffRun(t, repo, project.UserID, project.ID, "HR-03")
	directorDetail, err := repo.AgentRuntimeDetailForUser(project.UserID, directorRun.ID)
	if err != nil {
		t.Fatalf("load HR-03 Run: %v", err)
	}
	if directorRun.RootRunID != rootDetail.Run.ID || directorRun.ParentRunID != rootDetail.Run.ID || directorRun.RouteKind != "handoff" ||
		directorRun.IntentRouteID != "" || len(directorDetail.Steps) != 1 || directorDetail.Steps[0].AgentID != "director_storyboard_artist" ||
		directorDetail.Steps[0].SkillIDsJSON != `["director-storyboard"]` {
		t.Fatalf("HR-03 lineage or executable contract is invalid: run=%#v detail=%#v", directorRun, directorDetail)
	}
	var directorInputs []FilmProductionArtifactRef
	if err := json.Unmarshal([]byte(directorDetail.Steps[0].InputArtifactRefsJSON), &directorInputs); err != nil {
		t.Fatalf("decode HR-03 inputs: %v", err)
	}
	if len(directorInputs) != 1 || directorInputs[0].RevisionID != lockedScript.LockedRevision.ID || directorInputs[0].Status != "locked" {
		t.Fatalf("HR-03 did not consume the immutable locked script revision: %#v", directorInputs)
	}
	if processed, err := svc.ProcessNextFilmAgentStep(); !processed || err != nil {
		t.Fatalf("execute HR-03 = %v, %v", processed, err)
	}
	directorDetail, err = repo.AgentRuntimeDetailForUser(project.UserID, directorRun.ID)
	if err != nil {
		t.Fatalf("load completed HR-03: %v", err)
	}
	storyboardArtifact, storyboardReview := findCurrentFilmArtifact(t, directorDetail, "storyboard")
	lockedStoryboard, err := svc.LockFilmAgentArtifact(project.UserID, project.ID, directorRun.ID, storyboardArtifact.ID, LockFilmAgentArtifactRequest{
		ExpectedRunRevision: directorDetail.Run.Revision, ExpectedArtifactSequence: storyboardArtifact.RevisionSequence, ExpectedRevisionID: storyboardReview.ID,
	})
	if err != nil {
		t.Fatalf("LockFilmAgentArtifact(storyboard): %v", err)
	}
	if processed, err := svc.ProcessNextFilmAgentHandoff(); !processed || err != nil {
		t.Fatalf("process storyboard Handoff Trigger = %v, %v", processed, err)
	}
	visualRun := findFilmHandoffRun(t, repo, project.UserID, project.ID, "HR-04")
	soundRun := findFilmHandoffRun(t, repo, project.UserID, project.ID, "HR-05")
	visualDetail, err := repo.AgentRuntimeDetailForUser(project.UserID, visualRun.ID)
	if err != nil {
		t.Fatalf("load HR-04: %v", err)
	}
	soundDetail, err := repo.AgentRuntimeDetailForUser(project.UserID, soundRun.ID)
	if err != nil {
		t.Fatalf("load HR-05: %v", err)
	}
	if visualRun.ParentRunID != directorRun.ID || soundRun.ParentRunID != directorRun.ID || visualRun.RootRunID != rootDetail.Run.ID || soundRun.RootRunID != rootDetail.Run.ID {
		t.Fatalf("fanout Handoff Runs lost lineage: visual=%#v sound=%#v", visualRun, soundRun)
	}
	visualMetadata, err := filmAgentRunMetadata(visualRun)
	if err != nil {
		t.Fatalf("decode HR-04 execution metadata: %v", err)
	}
	soundMetadata, err := filmAgentRunMetadata(soundRun)
	if err != nil {
		t.Fatalf("decode HR-05 execution metadata: %v", err)
	}
	if visualMetadata.TriggerID == "" || soundMetadata.TriggerID == "" || visualMetadata.JoinKey == "" || soundMetadata.JoinKey == "" ||
		visualMetadata.Fanout != "parallel" || soundMetadata.Fanout != "parallel" || visualMetadata.RootRunID != rootDetail.Run.ID || soundMetadata.ParentRunID != directorRun.ID {
		t.Fatalf("fanout execution metadata is incomplete: visual=%#v sound=%#v", visualMetadata, soundMetadata)
	}
	if len(visualDetail.Steps) != 1 || visualDetail.Steps[0].SkillIDsJSON != `["character-visual-design","scene-asset-design"]` ||
		visualDetail.Steps[0].AgentID != "visual_development_designer" {
		t.Fatalf("HR-04 did not compile its complete Skill contract into one execution: %#v", visualDetail.Steps)
	}
	var soundInputs []FilmProductionArtifactRef
	if err := json.Unmarshal([]byte(soundDetail.Steps[0].InputArtifactRefsJSON), &soundInputs); err != nil {
		t.Fatalf("decode HR-05 inputs: %v", err)
	}
	if len(soundInputs) != 2 || soundInputs[0].Type != "script" || soundInputs[1].Type != "storyboard" ||
		soundInputs[0].Status != "locked" || soundInputs[1].RevisionID != lockedStoryboard.LockedRevision.ID {
		t.Fatalf("HR-05 AND contract did not receive locked script + storyboard: %#v", soundInputs)
	}

	runs, err := repo.ProjectAgentRuntimeRunsForDomain(project.UserID, project.ID, "film", 50)
	if err != nil {
		t.Fatalf("list Film Runs: %v", err)
	}
	counts := make(map[string]int)
	for _, run := range runs {
		counts[run.HandoffRouteID]++
		if run.HandoffRouteID == "HR-09" || run.HandoffRouteID == "HR-10" || run.HandoffRouteID == "HR-11" {
			t.Fatalf("orchestration-only Handoff route was executed as an Agent Run: %#v", run)
		}
	}
	if counts["HR-03"] != 1 || counts["HR-04"] != 1 || counts["HR-05"] != 1 {
		t.Fatalf("Handoff idempotency or fanout count is invalid: %#v", counts)
	}
}

func TestFilmHandoffGroupedInputResolutionUsesORWithinGroupsAndANDBetweenGroups(t *testing.T) {
	svc, _, _, _ := newFilmAgentRuntimeTestService(t)
	now := time.Now().UTC()
	fact := func(id string, artifactType string, agentID string, at time.Time) repository.ProductionArtifactRevisionFact {
		return repository.ProductionArtifactRevisionFact{
			Artifact: model.ProductionArtifact{ID: "artifact-" + id, ArtifactType: artifactType},
			Revision: model.ProductionArtifactRevision{
				ID: "revision-" + id, Version: 1, Status: model.ProductionArtifactStatusLocked,
				ContentDigest: "digest-" + id, SourceRunID: "run-" + id, CreatedAt: at,
			},
			SourceAgentID: agentID,
		}
	}
	script := fact("script", "script", "narrative_screenwriter", now)
	tvcScript := fact("tvc", "tvc-script", "narrative_screenwriter", now.Add(time.Second))
	storyboard := fact("storyboard", "storyboard", "director_storyboard_artist", now.Add(2*time.Second))
	character := fact("character", "character-design", "visual_development_designer", now)
	scene := fact("scene", "scene-design", "visual_development_designer", now.Add(time.Second))
	style := fact("style", "visual-style-guide", "visual_development_designer", now.Add(2*time.Second))

	hr03, _ := svc.filmAgentRegistry.HandoffRoute("HR-03")
	selection, ready := resolveFilmHandoffSelection(hr03, []repository.ProductionArtifactRevisionFact{script, tvcScript})
	if !ready || len(selection.Refs) != 1 || selection.Refs[0].Type != "tvc-script" {
		t.Fatalf("HR-03 OR group did not choose the latest valid script alternative: %#v ready=%v", selection, ready)
	}
	hr05, _ := svc.filmAgentRegistry.HandoffRoute("HR-05")
	if _, ready := resolveFilmHandoffSelection(hr05, []repository.ProductionArtifactRevisionFact{storyboard}); ready {
		t.Fatal("HR-05 became ready without its script AND group")
	}
	selection, ready = resolveFilmHandoffSelection(hr05, []repository.ProductionArtifactRevisionFact{script, storyboard})
	if !ready || !reflect.DeepEqual([]string{selection.Refs[0].Type, selection.Refs[1].Type}, []string{"script", "storyboard"}) || selection.ParentRunID != storyboard.Revision.SourceRunID {
		t.Fatalf("HR-05 AND groups or source parent are invalid: %#v ready=%v", selection, ready)
	}
	hr06, _ := svc.filmAgentRegistry.HandoffRoute("HR-06")
	if _, ready := resolveFilmHandoffSelection(hr06, []repository.ProductionArtifactRevisionFact{character, scene}); ready {
		t.Fatal("HR-06 became ready with only two of three required groups")
	}
	selection, ready = resolveFilmHandoffSelection(hr06, []repository.ProductionArtifactRevisionFact{character, scene, style})
	if !ready || len(selection.Refs) != 3 {
		t.Fatalf("HR-06 did not require all three visual Artifact groups: %#v ready=%v", selection, ready)
	}
	for _, routeID := range []string{"HR-09", "HR-10", "HR-11"} {
		route, _ := svc.filmAgentRegistry.HandoffRoute(routeID)
		if isAutomaticFilmHandoffRoute(route) {
			t.Fatalf("%s must remain orchestration-only", routeID)
		}
	}
}

func findCurrentFilmArtifact(t *testing.T, detail repository.AgentRuntimeDetail, artifactType string) (model.ProductionArtifact, model.ProductionArtifactRevision) {
	t.Helper()
	for _, artifact := range detail.Artifacts {
		if artifact.ArtifactType != artifactType {
			continue
		}
		for _, revision := range detail.ArtifactRevisions {
			if revision.ID == artifact.CurrentRevisionID {
				return artifact, revision
			}
		}
	}
	t.Fatalf("current Film Artifact %s not found in detail: %#v", artifactType, detail)
	return model.ProductionArtifact{}, model.ProductionArtifactRevision{}
}

func findFilmHandoffRun(t *testing.T, repo *repository.Repository, userID string, projectID string, routeID string) model.AgentRuntimeRun {
	t.Helper()
	runs, err := repo.ProjectAgentRuntimeRunsForDomain(userID, projectID, "film", 50)
	if err != nil {
		t.Fatalf("list Film Runs for %s: %v", routeID, err)
	}
	for _, run := range runs {
		if run.HandoffRouteID == routeID {
			return run
		}
	}
	t.Fatalf("Film Handoff Run %s was not created: %#v", routeID, runs)
	return model.AgentRuntimeRun{}
}
