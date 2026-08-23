package service

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestFilmAgentRegistryCompilesAllIntentRoutesIntoExecutableSteps(t *testing.T) {
	svc, _, _, _ := newFilmAgentRuntimeTestService(t)
	if len(svc.filmAgentRegistry.IntentRoutes) != 15 {
		t.Fatalf("Film intent route count = %d, want 15", len(svc.filmAgentRegistry.IntentRoutes))
	}
	for _, route := range svc.filmAgentRegistry.IntentRoutes {
		route := route
		t.Run(route.ID, func(t *testing.T) {
			selectedAgentID := route.PrimaryAgentID
			resolved, agentID, _, _, err := svc.selectFilmIntentRoute(route.TriggerPhrases[0], "", selectedAgentID)
			if err != nil {
				t.Fatalf("selectFilmIntentRoute(): %v", err)
			}
			if resolved.ID != route.ID || agentID != selectedAgentID {
				t.Fatalf("resolved route/Agent = %s/%s, want %s/%s", resolved.ID, agentID, route.ID, selectedAgentID)
			}
			steps, err := svc.compileFilmIntentSteps("run-"+route.ID, route, selectedAgentID, nil, false, time.Now().UTC())
			if err != nil {
				t.Fatalf("compileFilmIntentSteps(): %v", err)
			}
			if len(steps) != len(route.SkillIDs) || len(steps) == 0 {
				t.Fatalf("Step count = %d, want %d", len(steps), len(route.SkillIDs))
			}
			for index, step := range steps {
				var skillIDs []string
				if err := json.Unmarshal([]byte(step.SkillIDsJSON), &skillIDs); err != nil {
					t.Fatalf("decode Step Skill IDs: %v", err)
				}
				if !reflect.DeepEqual(skillIDs, []string{route.SkillIDs[index]}) {
					t.Fatalf("Step %d Skill IDs = %v, want %s", index, skillIDs, route.SkillIDs[index])
				}
				skill, ok := svc.filmAgentRegistry.Skill(skillIDs[0])
				if !ok || !filmContainsString(skill.OwnerAgentIDs, step.AgentID) {
					t.Fatalf("Step %d Agent %s cannot execute Skill %s", index, step.AgentID, skillIDs[0])
				}
				if index == 0 && step.Status != model.AgentStepStatusReady {
					t.Fatalf("first Step status = %s, want ready", step.Status)
				}
				if index > 0 {
					var dependencies []string
					if err := json.Unmarshal([]byte(step.DependsOnStepIDsJSON), &dependencies); err != nil {
						t.Fatalf("decode dependencies: %v", err)
					}
					if !reflect.DeepEqual(dependencies, []string{steps[index-1].ID}) || step.Status != model.AgentStepStatusPlanned {
						t.Fatalf("Step %d dependency/status = %v/%s", index, dependencies, step.Status)
					}
				}
				var expectedOutputs []string
				if len(route.StepOutputArtifactTypes) > 0 {
					expectedOutputs = route.StepOutputArtifactTypes[index]
				} else {
					expectedOutputs = route.OutputArtifactTypes
				}
				var actualOutputs []string
				if err := json.Unmarshal([]byte(step.ExpectedOutputArtifactTypesJSON), &actualOutputs); err != nil {
					t.Fatalf("decode Step output contract: %v", err)
				}
				if !reflect.DeepEqual(actualOutputs, expectedOutputs) {
					t.Fatalf("Step %d output contract = %v, want %v", index, actualOutputs, expectedOutputs)
				}
			}
			var outputs []string
			if err := json.Unmarshal([]byte(steps[len(steps)-1].ExpectedOutputArtifactTypesJSON), &outputs); err != nil {
				t.Fatalf("decode expected outputs: %v", err)
			}
			if !reflect.DeepEqual(outputs, route.OutputArtifactTypes) {
				t.Fatalf("expected outputs = %v, want %v", outputs, route.OutputArtifactTypes)
			}
		})
	}
}

func TestFilmIntentRouterHandlesNaturalLanguageFillersWithoutHidingAmbiguity(t *testing.T) {
	svc, _, _, _ := newFilmAgentRuntimeTestService(t)
	route, agentID, _, confidence, err := svc.selectFilmIntentRoute("请帮我审查一下这个剧本，重点看人物动机", "", "")
	if err != nil {
		t.Fatalf("select natural Film intent: %v", err)
	}
	if route.ID != "IR-06" || agentID != "quality_control_editor" || confidence != "deterministic_similarity" {
		t.Fatalf("natural Film intent = %s/%s/%s, want IR-06/quality_control_editor/deterministic_similarity", route.ID, agentID, confidence)
	}
	if _, _, _, _, err := svc.selectFilmIntentRoute("请设计角色并设计场景", "", ""); err == nil {
		t.Fatal("ambiguous Film intent unexpectedly selected a Route")
	}
}

func TestFilterFilmRouteInputRefsKeepsOnlyDeclaredAuthority(t *testing.T) {
	svc, _, _, _ := newFilmAgentRuntimeTestService(t)
	route, ok := svc.filmAgentRegistry.IntentRoute("IR-15")
	if !ok {
		t.Fatal("IR-15 is missing")
	}
	refs := []FilmProductionArtifactRef{
		{RevisionID: "feasibility", Type: "production-feasibility-report"},
		{RevisionID: "result", Type: "generation-result"},
		{RevisionID: "script", Type: "script"},
		{RevisionID: "routing", Type: "routing-decision"},
	}
	filtered := filterFilmRouteInputRefs(route, refs)
	if len(filtered) != 3 || filtered[0].RevisionID != "feasibility" || filtered[1].RevisionID != "result" || filtered[2].RevisionID != "script" {
		t.Fatalf("filtered Film refs = %#v", filtered)
	}
}

func TestFilmAgentEveryIntentRouteExecutesThroughDurableRuntime(t *testing.T) {
	svc, repo, _, project := newFilmAgentRuntimeTestService(t)

	for _, route := range svc.filmAgentRegistry.IntentRoutes {
		route := route
		t.Run(route.ID, func(t *testing.T) {
			inputRevisionIDs := make([]string, 0, len(route.RequiredInputArtifactTypes))
			for index, artifactType := range route.RequiredInputArtifactTypes {
				revision := createFilmTestArtifactRevisionOfType(t, repo, project, route.ID+"-input-"+string(rune('a'+index)), artifactType)
				inputRevisionIDs = append(inputRevisionIDs, revision.ID)
			}

			executor := &recordingFilmAgentExecutor{}
			svc.filmAgentExecutor = executor
			created, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-route-exec-"+route.ID, CreateFilmAgentRunRequest{
				Objective: "执行 " + route.Name, IntentRouteID: route.ID, AgentID: route.PrimaryAgentID,
				Input: map[string]any{"fixture": route.ID}, InputArtifactRevisionIDs: inputRevisionIDs,
			})
			if err != nil {
				t.Fatalf("CreateFilmAgentRun(): %v", err)
			}

			for processed := true; processed; {
				processed, err = svc.ProcessNextFilmAgentStep()
				if err != nil {
					t.Fatalf("ProcessNextFilmAgentStep(): %v", err)
				}
			}
			detail, err := repo.AgentRuntimeDetailForUser(project.UserID, created.Detail.Run.ID)
			if err != nil {
				t.Fatalf("AgentRuntimeDetailForUser(): %v", err)
			}
			if detail.Run.Status != model.AgentRunStatusCompleted || len(detail.Attempts) != len(detail.Steps) {
				t.Fatalf("route did not complete durably: status=%s steps=%d attempts=%d", detail.Run.Status, len(detail.Steps), len(detail.Attempts))
			}
			for _, step := range detail.Steps {
				if step.Status != model.AgentStepStatusCompleted {
					t.Fatalf("Step %s status = %s, want completed", step.ID, step.Status)
				}
			}
			for _, attempt := range detail.Attempts {
				if attempt.Status != model.AgentAttemptStatusSucceeded || attempt.ResponseJSON == "" {
					t.Fatalf("Attempt %s is not successful evidence: %#v", attempt.ID, attempt)
				}
			}
			for _, artifactType := range route.OutputArtifactTypes {
				output := findFilmAgentOutputRevision(t, detail.Artifacts, detail.ArtifactRevisions, artifactType)
				if output.Status != model.ProductionArtifactStatusReview || output.SourceAttemptID == "" {
					t.Fatalf("output %s is not review-stage durable evidence: %#v", artifactType, output)
				}
			}
			if len(executor.requests) != len(detail.Steps) {
				t.Fatalf("executor request count = %d, want %d", len(executor.requests), len(detail.Steps))
			}
		})
	}
}

func TestCreateFilmAgentRunPersistsLockedInputAndIsIdempotent(t *testing.T) {
	svc, repo, _, project := newFilmAgentRuntimeTestService(t)
	request := CreateFilmAgentRunRequest{
		Objective:     "写一个原创短片故事",
		IntentRouteID: "IR-01",
		Input:         map[string]any{"premise": "一位快递员发现时间停滞"},
	}
	created, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-create-0001", request)
	if err != nil {
		t.Fatalf("CreateFilmAgentRun(): %v", err)
	}
	if created.Idempotent || created.Detail.Run.Domain != "film" || created.Detail.Run.RegistryID != "film-agent-team" ||
		created.Detail.Run.RegistryVersion != "1.3.1" || len(created.Detail.Run.RegistryDigest) != 64 || created.Detail.Run.IntentRouteID != "IR-01" {
		t.Fatalf("unexpected created Run: %#v", created)
	}
	if len(created.Detail.RoutingDecisions) != 1 || len(created.Detail.Steps) != 1 || len(created.Detail.Events) != 3 ||
		len(created.Detail.Artifacts) != 3 || len(created.Detail.ArtifactRevisions) != 3 {
		t.Fatalf("created Run evidence is incomplete: %#v", created.Detail)
	}
	step := created.Detail.Steps[0]
	if step.AgentID != "narrative_screenwriter" || step.SkillIDsJSON != `["screenwriter"]` || step.Status != model.AgentStepStatusReady {
		t.Fatalf("unexpected compiled Step: %#v", step)
	}
	artifact, revision := findCurrentFilmArtifact(t, created.Detail, "project-requirements")
	if artifact.ArtifactType != "project-requirements" || artifact.CurrentRevisionID != revision.ID || artifact.RevisionSequence != 1 ||
		revision.Status != model.ProductionArtifactStatusLocked || revision.SourceRunID != created.Detail.Run.ID || revision.SourceStepID != step.ID || len(revision.ContentDigest) != 64 {
		t.Fatalf("project-requirements Artifact is not locked evidence: artifact=%#v revision=%#v", artifact, revision)
	}
	for _, artifactType := range []string{"task", "routing-decision"} {
		startArtifact, startRevision := findCurrentFilmArtifact(t, created.Detail, artifactType)
		if startRevision.Status != model.ProductionArtifactStatusLocked || startRevision.SourceRunID != created.Detail.Run.ID ||
			startRevision.SourceStepID != step.ID || startArtifact.LogicalKey != "run:"+created.Detail.Run.ID+":"+artifactType {
			t.Fatalf("HR-10 %s Artifact is incomplete: artifact=%#v revision=%#v", artifactType, startArtifact, startRevision)
		}
	}
	if created.Detail.Events[1].EventType != "handoff.hr10.completed" {
		t.Fatalf("HR-10 orchestration Event is missing: %#v", created.Detail.Events)
	}
	var persistedInput map[string]any
	if err := json.Unmarshal([]byte(created.Detail.Run.InputJSON), &persistedInput); err != nil {
		t.Fatalf("decode persisted Run input: %v", err)
	}
	if persistedInput["requestDigest"] == "" || persistedInput["schemaVersion"] != float64(1) {
		t.Fatalf("persisted Run input lacks request identity: %#v", persistedInput)
	}

	replayed, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-create-0001", request)
	if err != nil {
		t.Fatalf("idempotent replay: %v", err)
	}
	replayedRequirements, _ := findCurrentFilmArtifact(t, replayed.Detail, "project-requirements")
	if !replayed.Idempotent || replayed.Detail.Run.ID != created.Detail.Run.ID || replayedRequirements.ID != artifact.ID {
		t.Fatalf("idempotent replay created different facts: %#v", replayed)
	}
	if _, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-create-0001", CreateFilmAgentRunRequest{
		Objective: "写另一个原创短片故事", IntentRouteID: "IR-01",
	}); authStatus(err) != 409 {
		t.Fatalf("changed request with reused idempotency key error = %v, want 409", err)
	}
	loaded, err := repo.AgentRuntimeDetailForUser(project.UserID, created.Detail.Run.ID)
	if err != nil || len(loaded.Events) != 3 || len(loaded.ArtifactRevisions) != 3 {
		t.Fatalf("restored Run evidence = %#v, error = %v", loaded, err)
	}
}

func TestFilmAgentRunReviewPauseResumeAndStaleReplay(t *testing.T) {
	svc, _, _, project := newFilmAgentRuntimeTestService(t)
	created, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-review-0001", CreateFilmAgentRunRequest{
		Objective: "写一个原创短片故事", IntentRouteID: "IR-01", ReviewBeforeExecution: true,
	})
	if err != nil {
		t.Fatalf("CreateFilmAgentRun(): %v", err)
	}
	if created.Detail.Run.Status != model.AgentRunStatusAwaitingHuman || len(created.Detail.HumanDecisions) != 1 ||
		created.Detail.Steps[0].Status != model.AgentStepStatusAwaitingHuman {
		t.Fatalf("review Run did not pause coherently: %#v", created.Detail)
	}
	decision := created.Detail.HumanDecisions[0]
	resolved, err := svc.ResolveFilmAgentDecision(project.UserID, project.ID, created.Detail.Run.ID, decision.ID, ResolveFilmAgentDecisionRequest{
		ExpectedRunRevision: created.Detail.Run.Revision, ExpectedStepRevision: created.Detail.Steps[0].Revision,
		ExpectedDecisionRevision: decision.Revision, Action: "approve", Response: map[string]any{"note": "方案通过"},
	})
	if err != nil {
		t.Fatalf("ResolveFilmAgentDecision(): %v", err)
	}
	if resolved.Run.Status != model.AgentRunStatusReady || resolved.Steps[0].Status != model.AgentStepStatusReady ||
		resolved.HumanDecisions[0].Status != model.AgentHumanDecisionStatusResolved || resolved.Run.EventSequence != 4 {
		t.Fatalf("review Run did not resume coherently: %#v", resolved)
	}
	if _, err := svc.ResolveFilmAgentDecision(project.UserID, project.ID, resolved.Run.ID, decision.ID, ResolveFilmAgentDecisionRequest{
		ExpectedRunRevision: created.Detail.Run.Revision, ExpectedStepRevision: created.Detail.Steps[0].Revision,
		ExpectedDecisionRevision: decision.Revision, Action: "approve",
	}); authStatus(err) != 409 {
		t.Fatalf("stale Decision replay error = %v, want 409", err)
	}
}

func TestFilmAgentRunRequiresLockedRouteInputs(t *testing.T) {
	svc, repo, _, project := newFilmAgentRuntimeTestService(t)
	draft := createFilmTestArtifactRevision(t, repo, project, "script-draft", model.ProductionArtifactStatusDraft)
	if _, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-script-draft", CreateFilmAgentRunRequest{
		Objective: "把剧本做成分镜", IntentRouteID: "IR-07", InputArtifactRevisionIDs: []string{draft.ID},
	}); authStatus(err) != 409 {
		t.Fatalf("draft required input error = %v, want 409", err)
	}
	locked := createFilmTestArtifactRevision(t, repo, project, "script-locked", model.ProductionArtifactStatusLocked)
	created, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-script-locked", CreateFilmAgentRunRequest{
		Objective: "把剧本做成分镜", IntentRouteID: "IR-07", InputArtifactRevisionIDs: []string{locked.ID},
	})
	if err != nil {
		t.Fatalf("create Run with locked script: %v", err)
	}
	if created.Detail.Steps[0].AgentID != "director_storyboard_artist" || created.Detail.Steps[0].SkillIDsJSON != `["director-storyboard"]` {
		t.Fatalf("IR-07 compiled unexpected Step: %#v", created.Detail.Steps[0])
	}
	var outputs []string
	if err := json.Unmarshal([]byte(created.Detail.Steps[0].ExpectedOutputArtifactTypesJSON), &outputs); err != nil {
		t.Fatalf("decode IR-07 outputs: %v", err)
	}
	if !reflect.DeepEqual(outputs, []string{"storyboard", "shot-decision-sheet"}) {
		t.Fatalf("IR-07 outputs = %v", outputs)
	}
}

func TestFilmAgentRunRejectsSecretsAndKeepsDomainIsolation(t *testing.T) {
	svc, repo, _, project := newFilmAgentRuntimeTestService(t)
	if _, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-secret-0001", CreateFilmAgentRunRequest{
		Objective: "写一个原创短片故事", IntentRouteID: "IR-01",
		Input: map[string]any{"config": map[string]any{"api_key": "must-not-persist"}},
	}); authStatus(err) != 400 {
		t.Fatalf("secret input error = %v, want 400", err)
	}
	now := time.Now().UTC()
	ecommerceProject := model.Project{
		ID: "project-ecommerce", UserID: project.UserID, Name: "Ecommerce", Type: "ecommerce", AspectRatio: "9:16",
		Status: model.ProjectStatusActive, Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.CreateProject(&ecommerceProject); err != nil {
		t.Fatalf("create Ecommerce project: %v", err)
	}
	if _, err := svc.CreateFilmAgentRun(project.UserID, ecommerceProject.ID, "film-domain-0001", CreateFilmAgentRunRequest{
		Objective: "写一个原创短片故事", IntentRouteID: "IR-01",
	}); authStatus(err) != 404 {
		t.Fatalf("Film Run in Ecommerce project error = %v, want 404", err)
	}
	created, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-domain-0002", CreateFilmAgentRunRequest{
		Objective: "写一个原创短片故事", IntentRouteID: "IR-01",
	})
	if err != nil {
		t.Fatalf("create Film Run: %v", err)
	}
	foreign := model.AgentRuntimeRun{
		ID: "ecommerce-run", UserID: project.UserID, ProjectID: project.ID, Domain: "ecommerce",
		RegistryID: "ecommerce-agent-team", RegistryVersion: "1.0.0", RegistryDigest: digestString("ecommerce"),
		IntentRouteID: "EC-01", Status: model.AgentRunStatusReady, Objective: "product campaign", InputJSON: `{}`,
		IdempotencyKey: "ecommerce-run-0001", Revision: 1, CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
	}
	if err := repo.Create(&foreign); err != nil {
		t.Fatalf("insert foreign-domain Run: %v", err)
	}
	runs, err := svc.ListFilmAgentRuns(project.UserID, project.ID, 50)
	if err != nil {
		t.Fatalf("ListFilmAgentRuns(): %v", err)
	}
	if len(runs) != 1 || runs[0].ID != created.Detail.Run.ID || runs[0].Domain != "film" {
		t.Fatalf("Film Run list leaked another domain: %#v", runs)
	}
	if _, err := svc.FilmAgentRunDetail(project.UserID, project.ID, foreign.ID); authStatus(err) != 404 {
		t.Fatalf("foreign-domain detail error = %v, want 404", err)
	}
}

func TestArchivedFilmProjectKeepsHistoryReadableButRejectsWrites(t *testing.T) {
	svc, repo, _, project := newFilmAgentRuntimeTestService(t)
	created, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-archive-0001", CreateFilmAgentRunRequest{
		Objective: "写一个原创短片故事", IntentRouteID: "IR-01", ReviewBeforeExecution: true,
	})
	if err != nil {
		t.Fatalf("CreateFilmAgentRun(): %v", err)
	}
	project.Status = model.ProjectStatusArchived
	project.Revision++
	project.UpdatedAt = time.Now().UTC()
	if err := repo.UpdateProject(&project); err != nil {
		t.Fatalf("archive Film project: %v", err)
	}
	if _, err := svc.FilmAgentRunDetail(project.UserID, project.ID, created.Detail.Run.ID); err != nil {
		t.Fatalf("archived Run detail must stay readable: %v", err)
	}
	if runs, err := svc.ListFilmAgentRuns(project.UserID, project.ID, 10); err != nil || len(runs) != 1 {
		t.Fatalf("archived Run list = %#v, error = %v", runs, err)
	}
	if _, err := svc.FilmAgentRuntimeCatalog(project.UserID, project.ID); err != nil {
		t.Fatalf("archived Film catalog must stay readable: %v", err)
	}
	if _, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-archive-0002", CreateFilmAgentRunRequest{
		Objective: "写一个原创短片故事", IntentRouteID: "IR-01",
	}); authStatus(err) != 409 {
		t.Fatalf("create in archived project error = %v, want 409", err)
	}
	decision := created.Detail.HumanDecisions[0]
	if _, err := svc.ResolveFilmAgentDecision(project.UserID, project.ID, created.Detail.Run.ID, decision.ID, ResolveFilmAgentDecisionRequest{
		ExpectedRunRevision: created.Detail.Run.Revision, ExpectedStepRevision: created.Detail.Steps[0].Revision,
		ExpectedDecisionRevision: decision.Revision, Action: "approve",
	}); authStatus(err) != 409 {
		t.Fatalf("resolve in archived project error = %v, want 409", err)
	}
}

func TestRetryFilmAgentStepCreatesANewAttempt(t *testing.T) {
	svc, repo, _, project := newFilmAgentRuntimeTestService(t)
	created, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-retry-0001", CreateFilmAgentRunRequest{
		Objective: "写一个原创短片故事", IntentRouteID: "IR-01",
	})
	if err != nil {
		t.Fatalf("CreateFilmAgentRun(): %v", err)
	}
	run := created.Detail.Run
	step := created.Detail.Steps[0]
	runPtr, stepPtr, attempt, err := repo.CreateAgentRuntimeAttempt(repository.AgentRuntimeAttemptCreate{
		UserID: project.UserID, RunID: run.ID, StepID: step.ID,
		ExpectedRunRevision: run.Revision, ExpectedStepRevision: step.Revision,
		Attempt: &model.AgentRuntimeAttempt{ID: "attempt-first", Executor: "agent-runtime", InputDigest: digestString("input"), PromptDigest: digestString("prompt"), RequestJSON: `{}`},
		Event:   repository.AgentRuntimeEventInput{ID: "event-attempt-first", EventType: "attempt.created", ActorType: "runtime", ActorID: "orchestrator"},
	})
	if err != nil {
		t.Fatalf("create first Attempt: %v", err)
	}
	runningRun := model.AgentRunStatusRunning
	runningStep := model.AgentStepStatusRunning
	runPtr, stepPtr, attempt, err = repo.TransitionAgentRuntimeAttempt(repository.AgentRuntimeAttemptTransition{
		UserID: project.UserID, RunID: runPtr.ID, StepID: stepPtr.ID, AttemptID: attempt.ID,
		ExpectedRunRevision: runPtr.Revision, ExpectedStepRevision: stepPtr.Revision, ExpectedAttemptRevision: attempt.Revision,
		ToStatus: model.AgentAttemptStatusRunning, RunStatus: &runningRun, StepStatus: &runningStep,
		Event: repository.AgentRuntimeEventInput{ID: "event-attempt-running", EventType: "attempt.started", ActorType: "runtime", ActorID: "orchestrator"},
	})
	if err != nil {
		t.Fatalf("start first Attempt: %v", err)
	}
	failedRun := model.AgentRunStatusFailed
	failedStep := model.AgentStepStatusFailed
	runPtr, stepPtr, _, err = repo.TransitionAgentRuntimeAttempt(repository.AgentRuntimeAttemptTransition{
		UserID: project.UserID, RunID: runPtr.ID, StepID: stepPtr.ID, AttemptID: attempt.ID,
		ExpectedRunRevision: runPtr.Revision, ExpectedStepRevision: stepPtr.Revision, ExpectedAttemptRevision: attempt.Revision,
		ToStatus: model.AgentAttemptStatusFailed, RunStatus: &failedRun, StepStatus: &failedStep,
		FailureCode: "executor_failed", Failure: "test failure",
		Event: repository.AgentRuntimeEventInput{ID: "event-attempt-failed", EventType: "attempt.failed", ActorType: "runtime", ActorID: "orchestrator"},
	})
	if err != nil {
		t.Fatalf("fail first Attempt: %v", err)
	}
	retried, err := svc.RetryFilmAgentStep(project.UserID, project.ID, runPtr.ID, stepPtr.ID, RetryFilmAgentStepRequest{
		ExpectedRunRevision: runPtr.Revision, ExpectedStepRevision: stepPtr.Revision, Reason: "修正执行上下文后重试",
	})
	if err != nil {
		t.Fatalf("RetryFilmAgentStep(): %v", err)
	}
	if retried.Run.Status != model.AgentRunStatusReady || retried.Steps[0].Status != model.AgentStepStatusReady || len(retried.Attempts) != 2 {
		t.Fatalf("retry state is incomplete: %#v", retried)
	}
	if retried.Attempts[0].Number != 1 || retried.Attempts[0].Status != model.AgentAttemptStatusFailed ||
		retried.Attempts[1].Number != 2 || retried.Attempts[1].Status != model.AgentAttemptStatusQueued {
		t.Fatalf("retry mutated Attempt history: %#v", retried.Attempts)
	}
}

func newFilmAgentRuntimeTestService(t *testing.T) (*Service, *repository.Repository, *gorm.DB, model.Project) {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "film-agent-runtime.db")
	db, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open Film Agent Runtime database: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Project{}, &model.CanvasProject{},
		&model.AgentRuntimeRun{}, &model.AgentRuntimeStep{}, &model.AgentRuntimeAttempt{},
		&model.AgentRoutingDecision{}, &model.AgentHumanDecision{}, &model.AgentRuntimeEvent{},
		&model.AgentHandoffTrigger{},
		&model.ProductionArtifact{}, &model.ProductionArtifactRevision{},
	); err != nil {
		t.Fatalf("migrate Film Agent Runtime database: %v", err)
	}
	repo := repository.New(db)
	svc := New(repo, t.TempDir())
	if err := svc.ValidateRuntime(); err != nil {
		t.Fatalf("ValidateRuntime(): %v", err)
	}
	now := time.Now().UTC()
	project := model.Project{
		ID: "project-film", UserID: "user-film", Name: "Film Project", Type: "short-drama", AspectRatio: "9:16",
		SourceType: "blank", Status: model.ProjectStatusActive, Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.CreateProject(&project); err != nil {
		t.Fatalf("create Film project: %v", err)
	}
	return svc, repo, db, project
}

func createFilmTestArtifactRevisionOfType(t *testing.T, repo *repository.Repository, project model.Project, logicalKey string, artifactType string) model.ProductionArtifactRevision {
	t.Helper()
	artifact := &model.ProductionArtifact{
		ID: logicalKey + "-artifact", UserID: project.UserID, ProjectID: project.ID, Domain: "film",
		ArtifactType: artifactType, LogicalKey: logicalKey,
	}
	revisionInput := &model.ProductionArtifactRevision{
		ID: logicalKey + "-revision", Status: model.ProductionArtifactStatusLocked, ContentJSON: `{"fixture":true}`,
		ContentDigest: digestString(logicalKey), CreatedByType: "user", CreatedByID: project.UserID,
	}
	_, revision, err := repo.CreateProductionArtifactRevision(repository.ProductionArtifactRevisionCreate{
		UserID: project.UserID, Artifact: artifact, Revision: revisionInput, ExpectedSequence: 0,
	})
	if err != nil {
		t.Fatalf("create Film test input Artifact %s: %v", artifactType, err)
	}
	return *revision
}

func createFilmTestArtifactRevision(t *testing.T, repo *repository.Repository, project model.Project, logicalKey string, status model.ProductionArtifactStatus) model.ProductionArtifactRevision {
	t.Helper()
	artifact := &model.ProductionArtifact{
		ID: logicalKey + "-artifact", UserID: project.UserID, ProjectID: project.ID, Domain: "film",
		ArtifactType: "script", LogicalKey: logicalKey,
	}
	revisionInput := &model.ProductionArtifactRevision{
		ID: logicalKey + "-revision", Status: status, ContentJSON: `{"title":"Episode 1"}`,
		ContentDigest: digestString(logicalKey), CreatedByType: "user", CreatedByID: project.UserID,
	}
	_, revision, err := repo.CreateProductionArtifactRevision(repository.ProductionArtifactRevisionCreate{
		UserID: project.UserID, Artifact: artifact, Revision: revisionInput, ExpectedSequence: 0,
	})
	if err != nil {
		t.Fatalf("create Film test Artifact: %v", err)
	}
	return *revision
}

func authStatus(err error) int {
	var authErr *AuthError
	if errors.As(err, &authErr) {
		return authErr.Status
	}
	return 0
}
