package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/model"
)

type recordingFilmAgentExecutor struct {
	responses []filmAgentExecutionResponse
	errors    []error
	requests  []filmAgentExecutionRequest
}

func (e *recordingFilmAgentExecutor) Name() string { return "recording-film-executor" }

func (e *recordingFilmAgentExecutor) Execute(_ context.Context, request filmAgentExecutionRequest) (filmAgentExecutionResponse, error) {
	e.requests = append(e.requests, request)
	index := len(e.requests) - 1
	if index < len(e.errors) && e.errors[index] != nil {
		return filmAgentExecutionResponse{}, e.errors[index]
	}
	if index < len(e.responses) {
		return e.responses[index], nil
	}
	return filmAgentExecutionResponse{Text: validFilmAgentExecutionOutput(request), ModelRef: "fake-text-model"}, nil
}

func TestProcessNextFilmAgentStepPersistsValidatedReviewArtifact(t *testing.T) {
	svc, repo, _, project := newFilmAgentRuntimeTestService(t)
	executor := &recordingFilmAgentExecutor{}
	svc.filmAgentExecutor = executor
	created, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-executor-single", CreateFilmAgentRunRequest{
		Objective: "写一个原创短片故事", IntentRouteID: "IR-01", Input: map[string]any{"premise": "停电后的最后一班地铁"},
	})
	if err != nil {
		t.Fatalf("CreateFilmAgentRun(): %v", err)
	}
	processed, err := svc.ProcessNextFilmAgentStep()
	if err != nil || !processed {
		t.Fatalf("ProcessNextFilmAgentStep() = %v, %v", processed, err)
	}
	detail, err := repo.AgentRuntimeDetailForUser(project.UserID, created.Detail.Run.ID)
	if err != nil {
		t.Fatalf("load executed Run: %v", err)
	}
	if detail.Run.Status != model.AgentRunStatusCompleted || len(detail.Steps) != 1 ||
		detail.Steps[0].Status != model.AgentStepStatusCompleted || len(detail.Attempts) != 1 ||
		detail.Attempts[0].Status != model.AgentAttemptStatusSucceeded {
		t.Fatalf("single-Skill execution did not complete coherently: %#v", detail)
	}
	if detail.Attempts[0].TaskID == "" || detail.Attempts[0].Revision != 3 ||
		!strings.Contains(detail.Attempts[0].RequestJSON, `"expectedOutputArtifactTypes":["script"]`) {
		t.Fatalf("Attempt execution identity or request metadata is incomplete: %#v", detail.Attempts[0])
	}
	output := findFilmAgentOutputRevision(t, detail.Artifacts, detail.ArtifactRevisions, "script")
	if output.Status != model.ProductionArtifactStatusReview || output.SourceAttemptID != detail.Attempts[0].ID ||
		output.CreatedByType != "agent" || output.CreatedByID != detail.Steps[0].AgentID {
		t.Fatalf("Agent output is not review-stage execution evidence: %#v", output)
	}
	if len(executor.requests) != 1 || executor.requests[0].AgentID != "narrative_screenwriter" ||
		len(executor.requests[0].SkillIDs) != 1 || executor.requests[0].SkillIDs[0] != "screenwriter" ||
		len(executor.requests[0].InputDigest) != 64 || len(executor.requests[0].PromptDigest) != 64 ||
		!strings.Contains(executor.requests[0].SystemPrompt, "[RUNTIME OUTPUT CONTRACT]") {
		t.Fatalf("Executor received an invalid compiled request: %#v", executor.requests)
	}
	processed, err = svc.ProcessNextFilmAgentStep()
	if err != nil || processed {
		t.Fatalf("completed queue should be empty, got processed=%v error=%v", processed, err)
	}
}

func TestProcessNextFilmAgentStepAutomaticallyAdvancesIR03Skills(t *testing.T) {
	svc, repo, _, project := newFilmAgentRuntimeTestService(t)
	executor := &recordingFilmAgentExecutor{}
	svc.filmAgentExecutor = executor
	created, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-executor-ir03", CreateFilmAgentRunRequest{
		Objective: "写一个故事型TVC剧本", IntentRouteID: "IR-03", AgentID: "tvc_creative_director",
	})
	if err != nil {
		t.Fatalf("CreateFilmAgentRun(IR-03): %v", err)
	}
	if len(created.Detail.Steps) != 2 {
		t.Fatalf("IR-03 Step count = %d, want 2", len(created.Detail.Steps))
	}
	processed, err := svc.ProcessNextFilmAgentStep()
	if err != nil || !processed {
		t.Fatalf("process IR-03 first Skill = %v, %v", processed, err)
	}
	afterFirst, err := repo.AgentRuntimeDetailForUser(project.UserID, created.Detail.Run.ID)
	if err != nil {
		t.Fatalf("load IR-03 after first Skill: %v", err)
	}
	if afterFirst.Run.Status != model.AgentRunStatusReady || afterFirst.Steps[0].Status != model.AgentStepStatusCompleted ||
		afterFirst.Steps[1].Status != model.AgentStepStatusReady || len(afterFirst.Attempts) != 1 {
		t.Fatalf("IR-03 second Skill was not unblocked: %#v", afterFirst)
	}
	processed, err = svc.ProcessNextFilmAgentStep()
	if err != nil || !processed {
		t.Fatalf("process IR-03 second Skill = %v, %v", processed, err)
	}
	completed, err := repo.AgentRuntimeDetailForUser(project.UserID, created.Detail.Run.ID)
	if err != nil {
		t.Fatalf("load completed IR-03: %v", err)
	}
	if completed.Run.Status != model.AgentRunStatusCompleted || len(completed.Attempts) != 2 ||
		completed.Steps[1].Status != model.AgentStepStatusCompleted || completed.Attempts[1].Status != model.AgentAttemptStatusSucceeded {
		t.Fatalf("IR-03 did not complete both Skills: %#v", completed)
	}
	output := findFilmAgentOutputRevision(t, completed.Artifacts, completed.ArtifactRevisions, "tvc-script")
	if output.Status != model.ProductionArtifactStatusReview {
		t.Fatalf("IR-03 output status = %s, want review", output.Status)
	}
	if len(executor.requests) != 2 || executor.requests[0].SkillIDs[0] != "tvc" || executor.requests[1].SkillIDs[0] != "screenwriter" ||
		len(executor.requests[0].ExpectedOutputArtifactTypes) != 0 || len(executor.requests[1].ExpectedOutputArtifactTypes) != 1 ||
		executor.requests[1].ExpectedOutputArtifactTypes[0] != "tvc-script" {
		t.Fatalf("IR-03 compiled execution order is invalid: %#v", executor.requests)
	}
	var secondPrompt struct {
		DependencyResults []map[string]any `json:"dependencyResults"`
	}
	if err := json.Unmarshal([]byte(executor.requests[1].Prompt), &secondPrompt); err != nil {
		t.Fatalf("decode IR-03 second prompt: %v", err)
	}
	if len(secondPrompt.DependencyResults) != 1 || secondPrompt.DependencyResults[0]["stepId"] != completed.Steps[0].ID {
		t.Fatalf("IR-03 second Skill did not receive first Skill evidence: %#v", secondPrompt.DependencyResults)
	}
}

func TestProcessNextFilmAgentStepRecordsInvalidStructuredOutputAsFailure(t *testing.T) {
	svc, repo, _, project := newFilmAgentRuntimeTestService(t)
	executor := &recordingFilmAgentExecutor{responses: []filmAgentExecutionResponse{{
		Text:     `{"schemaVersion":1,"summary":"invalid","artifacts":[{"type":"script","contentText":"draft"}],"status":"locked"}`,
		ModelRef: "fake-text-model",
	}}}
	svc.filmAgentExecutor = executor
	created, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-executor-invalid", CreateFilmAgentRunRequest{
		Objective: "写一个原创短片故事", IntentRouteID: "IR-01",
	})
	if err != nil {
		t.Fatalf("CreateFilmAgentRun(): %v", err)
	}
	processed, processErr := svc.ProcessNextFilmAgentStep()
	if !processed || processErr == nil || !strings.Contains(processErr.Error(), "结构化 Artifact 校验") {
		t.Fatalf("invalid output process result = %v, %v", processed, processErr)
	}
	detail, err := repo.AgentRuntimeDetailForUser(project.UserID, created.Detail.Run.ID)
	if err != nil {
		t.Fatalf("load failed Run: %v", err)
	}
	if detail.Run.Status != model.AgentRunStatusFailed || detail.Run.FailureCode != "film_structured_output_invalid" ||
		detail.Steps[0].Status != model.AgentStepStatusFailed || detail.Attempts[0].Status != model.AgentAttemptStatusFailed ||
		detail.Attempts[0].FailureCode != "film_structured_output_invalid" || len(detail.Artifacts) != 3 ||
		len(detail.ArtifactRevisions) != 3 || detail.Events[len(detail.Events)-1].EventType != "attempt.failed" {
		t.Fatalf("invalid model output did not leave a complete failure record: %#v", detail)
	}
}

func TestRetryFilmAgentStepExecutesANewAttemptAfterStructuredOutputFailure(t *testing.T) {
	svc, repo, _, project := newFilmAgentRuntimeTestService(t)
	executor := &recordingFilmAgentExecutor{responses: []filmAgentExecutionResponse{{
		Text:     `{"schemaVersion":1,"summary":"missing output","artifacts":[]}`,
		ModelRef: "fake-text-model",
	}}}
	svc.filmAgentExecutor = executor
	created, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-executor-retry", CreateFilmAgentRunRequest{
		Objective: "写一个原创短片故事", IntentRouteID: "IR-01",
	})
	if err != nil {
		t.Fatalf("CreateFilmAgentRun(): %v", err)
	}
	if processed, err := svc.ProcessNextFilmAgentStep(); !processed || err == nil {
		t.Fatalf("first execution should fail, got processed=%v error=%v", processed, err)
	}
	failed, err := repo.AgentRuntimeDetailForUser(project.UserID, created.Detail.Run.ID)
	if err != nil {
		t.Fatalf("load first failure: %v", err)
	}
	retried, err := svc.RetryFilmAgentStep(project.UserID, project.ID, failed.Run.ID, failed.Steps[0].ID, RetryFilmAgentStepRequest{
		ExpectedRunRevision: failed.Run.Revision, ExpectedStepRevision: failed.Steps[0].Revision, Reason: "使用修正后的结构化输出重试",
	})
	if err != nil {
		t.Fatalf("RetryFilmAgentStep(): %v", err)
	}
	if len(retried.Attempts) != 2 || retried.Attempts[0].Status != model.AgentAttemptStatusFailed ||
		retried.Attempts[1].Status != model.AgentAttemptStatusQueued || retried.Attempts[0].ID == retried.Attempts[1].ID {
		t.Fatalf("manual retry did not append a new Attempt: %#v", retried.Attempts)
	}
	if processed, err := svc.ProcessNextFilmAgentStep(); !processed || err != nil {
		t.Fatalf("retry execution = %v, %v", processed, err)
	}
	completed, err := repo.AgentRuntimeDetailForUser(project.UserID, created.Detail.Run.ID)
	if err != nil {
		t.Fatalf("load successful retry: %v", err)
	}
	if completed.Run.Status != model.AgentRunStatusCompleted || len(completed.Attempts) != 2 ||
		completed.Attempts[0].Status != model.AgentAttemptStatusFailed || completed.Attempts[1].Status != model.AgentAttemptStatusSucceeded ||
		completed.Attempts[0].TaskID == "" || completed.Attempts[1].TaskID == "" ||
		completed.Attempts[0].TaskID == completed.Attempts[1].TaskID {
		t.Fatalf("retry Attempt history or provider task identity is invalid: %#v", completed.Attempts)
	}
	findFilmAgentOutputRevision(t, completed.Artifacts, completed.ArtifactRevisions, "script")
}

func validFilmAgentExecutionOutput(request filmAgentExecutionRequest) string {
	artifacts := make([]map[string]any, 0, len(request.ExpectedOutputArtifactTypes))
	for _, artifactType := range request.ExpectedOutputArtifactTypes {
		artifacts = append(artifacts, map[string]any{
			"type": artifactType, "title": "Test output",
			"content":     map[string]any{"generatedByAgent": request.AgentID, "skillIds": request.SkillIDs},
			"contentText": "Validated output for " + artifactType,
		})
	}
	encoded, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "summary": "Validated deterministic test output", "artifacts": artifacts,
	})
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func findFilmAgentOutputRevision(t *testing.T, artifacts []model.ProductionArtifact, revisions []model.ProductionArtifactRevision, artifactType string) model.ProductionArtifactRevision {
	t.Helper()
	artifactID := ""
	for _, artifact := range artifacts {
		if artifact.ArtifactType == artifactType {
			artifactID = artifact.ID
			break
		}
	}
	if artifactID == "" {
		t.Fatalf("Film Agent output Artifact %s was not persisted: %#v", artifactType, artifacts)
	}
	for _, revision := range revisions {
		if revision.ArtifactID == artifactID {
			return revision
		}
	}
	t.Fatalf("Film Agent output Artifact %s has no revision: %#v", artifactType, revisions)
	return model.ProductionArtifactRevision{}
}
