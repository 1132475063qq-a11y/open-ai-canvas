package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"infinite-canvas/backend/internal/agentruntime"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/gorm"
)

const (
	filmAgentWorkerConcurrency = 2
	filmAgentLeaseDuration     = 90 * time.Second
	filmAgentLeaseRenewal      = 30 * time.Second
	filmAgentExecutionTimeout  = 20 * time.Minute
	maxFilmAgentPromptBytes    = 1 << 20
)

type filmAgentExecutionRequest struct {
	UserID                      string
	ProjectID                   string
	RunID                       string
	StepID                      string
	AttemptID                   string
	TaskID                      string
	LogicalModelID              string
	AgentID                     string
	SkillIDs                    []string
	ExpectedOutputArtifactTypes []string
	SystemPrompt                string
	Prompt                      string
	InputDigest                 string
	PromptDigest                string
}

type filmAgentExecutionResponse struct {
	Text     string
	ModelRef string
}

type filmAgentExecutor interface {
	Name() string
	Execute(context.Context, filmAgentExecutionRequest) (filmAgentExecutionResponse, error)
}

type filmAgentExecutionError struct {
	Code    string
	Message string
	Err     error
}

// filmProviderUnavailableError is a stable, non-success outcome. It is used
// for missing/disabled logical routes so API/runtime evidence can say
// NOT_AVAILABLE without manufacturing text or media artifacts.
type filmProviderUnavailableError struct {
	Message string
	Err     error
}

func (e *filmProviderUnavailableError) Error() string {
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "NOT_AVAILABLE: Film Provider is not configured"
}

func (e *filmProviderUnavailableError) Unwrap() error { return e.Err }

func (e *filmAgentExecutionError) Error() string {
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e *filmAgentExecutionError) Unwrap() error { return e.Err }

type queuedFilmAgentExecutor struct {
	service      *Service
	pollInterval time.Duration
}

func (e *queuedFilmAgentExecutor) Name() string { return "managed-task-queue-v1" }

func (e *queuedFilmAgentExecutor) Execute(ctx context.Context, request filmAgentExecutionRequest) (filmAgentExecutionResponse, error) {
	if strings.TrimSpace(request.LogicalModelID) == "" {
		return filmAgentExecutionResponse{}, &filmAgentExecutionError{Code: "film_text_model_required", Message: "Film Agent Run 未选择可用的逻辑文本模型"}
	}
	task, err := e.ensureTask(request)
	if err != nil {
		if filmAgentProviderUnavailable(err) {
			return filmAgentExecutionResponse{}, &filmProviderUnavailableError{Message: "NOT_AVAILABLE: Film Provider 未配置或不可用", Err: err}
		}
		return filmAgentExecutionResponse{}, &filmAgentExecutionError{Code: "film_task_dispatch_failed", Message: e.service.UserFacingErrorMessage(err), Err: err}
	}
	pollInterval := e.pollInterval
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		switch task.Status {
		case model.TaskStatusSucceeded:
			var payload map[string]any
			if err := json.Unmarshal([]byte(task.ResultJSON), &payload); err != nil {
				return filmAgentExecutionResponse{}, &filmAgentExecutionError{Code: "film_task_result_invalid", Message: "Film Agent 文本任务结果不是有效 JSON", Err: err}
			}
			text, _ := payload["text"].(string)
			if strings.TrimSpace(text) == "" {
				return filmAgentExecutionResponse{}, &filmAgentExecutionError{Code: "film_task_result_empty", Message: "Film Agent 文本任务没有返回内容"}
			}
			return filmAgentExecutionResponse{Text: text, ModelRef: task.Model}, nil
		case model.TaskStatusFailed:
			return filmAgentExecutionResponse{}, &filmAgentExecutionError{Code: "film_provider_task_failed", Message: defaultString(strings.TrimSpace(task.Error), "Film Agent 文本模型任务失败")}
		case model.TaskStatusCancelled:
			return filmAgentExecutionResponse{}, &filmAgentExecutionError{Code: "film_provider_task_cancelled", Message: "Film Agent 文本模型任务已取消"}
		}
		select {
		case <-ctx.Done():
			return filmAgentExecutionResponse{}, ctx.Err()
		case <-ticker.C:
			latest, err := e.service.repo.Task(task.ID)
			if err != nil {
				return filmAgentExecutionResponse{}, err
			}
			task = latest
		}
	}
}

func filmAgentProviderUnavailable(err error) bool {
	if errors.Is(err, repository.ErrLogicalModelUnavailable) {
		return true
	}
	var authErr *AuthError
	if errors.As(err, &authErr) {
		message := strings.ToLower(strings.TrimSpace(authErr.Message))
		return strings.Contains(message, "模型不可用") || strings.Contains(message, "model") && strings.Contains(message, "不可用")
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "logical model is unavailable") || strings.Contains(message, "provider is not configured")
}

func (e *queuedFilmAgentExecutor) ensureTask(request filmAgentExecutionRequest) (*model.Task, error) {
	if existing, err := e.service.repo.Task(request.TaskID); err == nil {
		return e.verifyTask(existing, request)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	input := map[string]any{
		"mode":   "text",
		"prompt": request.Prompt,
		"config": map[string]any{"systemPrompt": request.SystemPrompt},
		"metadata": map[string]any{
			"filmAgentRunId": request.RunID, "filmAgentStepId": request.StepID,
			"filmAgentAttemptId": request.AttemptID, "filmAgentPromptDigest": request.PromptDigest,
		},
	}
	task, err := e.service.createTaskWithID(request.UserID, request.TaskID, CreateTaskRequest{
		ProjectID: request.ProjectID, Type: "canvas_text_film_agent", Operation: "film_agent_skill",
		Prompt: request.Prompt, Provider: "managed", LogicalModelID: request.LogicalModelID, Input: input,
	})
	if err == nil {
		return task, nil
	}
	if existing, lookupErr := e.service.repo.Task(request.TaskID); lookupErr == nil {
		return e.verifyTask(existing, request)
	}
	return nil, err
}

func (e *queuedFilmAgentExecutor) verifyTask(task *model.Task, request filmAgentExecutionRequest) (*model.Task, error) {
	if task.UserID != request.UserID || task.ProjectID != request.ProjectID || task.Type != "canvas_text_film_agent" || task.Operation != "film_agent_skill" {
		return nil, errors.New("Film Agent Attempt TaskID references a different task")
	}
	decrypted, err := e.service.decryptTaskInputJSON(task.InputJSON)
	if err != nil {
		return nil, err
	}
	var input struct {
		Metadata map[string]any `json:"metadata"`
	}
	if err := json.Unmarshal([]byte(decrypted), &input); err != nil {
		return nil, err
	}
	checks := map[string]string{
		"filmAgentRunId": request.RunID, "filmAgentStepId": request.StepID,
		"filmAgentAttemptId": request.AttemptID, "filmAgentPromptDigest": request.PromptDigest,
	}
	for key, expected := range checks {
		if strings.TrimSpace(fmt.Sprint(input.Metadata[key])) != expected {
			return nil, errors.New("Film Agent task execution identity does not match its Attempt")
		}
	}
	return task, nil
}

func (s *Service) startFilmAgentWorker() {
	if s.filmAgentExecutor == nil || s.filmAgentRegistry == nil {
		return
	}
	go func() {
		slots := make(chan struct{}, filmAgentWorkerConcurrency)
		dispatch := func() {
			for len(slots) < filmAgentWorkerConcurrency {
				release, acquired, err := s.coordinator.acquire(context.Background(), "film-agent-workers", filmAgentWorkerConcurrency, filmAgentExecutionTimeout)
				if err != nil || !acquired {
					return
				}
				claim, err := s.claimNextFilmAgentExecution()
				if err != nil || claim == nil {
					release()
					if err != nil {
						log.Printf("claim Film Agent execution failed: %v", err)
					}
					return
				}
				slots <- struct{}{}
				go func(claim *repository.AgentRuntimeExecutionClaim) {
					defer func() { <-slots; release() }()
					if err := s.processClaimedFilmAgentExecution(claim); err != nil && !errors.Is(err, repository.ErrAgentRuntimeLeaseLost) {
						log.Printf("process Film Agent execution run=%s step=%s failed: %v", claim.Run.ID, claim.Step.ID, err)
					}
				}(claim)
			}
		}
		dispatch()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			dispatch()
		}
	}()
}

func (s *Service) ProcessNextFilmAgentStep() (bool, error) {
	claim, err := s.claimNextFilmAgentExecution()
	if err != nil || claim == nil {
		return false, err
	}
	return true, s.processClaimedFilmAgentExecution(claim)
}

func (s *Service) claimNextFilmAgentExecution() (*repository.AgentRuntimeExecutionClaim, error) {
	if err := s.ValidateRuntime(); err != nil {
		return nil, err
	}
	return s.repo.ClaimNextAgentRuntimeExecution(repository.AgentRuntimeExecutionClaimCommand{
		Owner: s.workerID + ":film", LeaseDuration: filmAgentLeaseDuration,
		AttemptID: newID(), TaskID: newID(), EventID: newID(), Executor: s.filmAgentExecutor.Name(), At: time.Now().UTC(),
	})
}

func (s *Service) processClaimedFilmAgentExecution(claim *repository.AgentRuntimeExecutionClaim) error {
	ctx, cancel := context.WithTimeout(context.Background(), filmAgentExecutionTimeout)
	defer cancel()
	leaseDone := make(chan struct{})
	leaseLost := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(filmAgentLeaseRenewal)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := s.repo.RenewAgentRuntimeExecutionLease(claim.Step.ID, claim.Attempt.ID, s.workerID+":film", filmAgentLeaseDuration); err != nil {
					leaseLost <- err
					cancel()
					return
				}
			case <-leaseDone:
				return
			}
		}
	}()
	defer close(leaseDone)

	request, err := s.buildFilmAgentExecutionRequest(*claim)
	attemptRevision := claim.Attempt.Revision
	if err == nil {
		requestMetadata := mustFilmJSON(map[string]any{
			"schemaVersion": 1, "logicalModelId": request.LogicalModelID, "agentId": request.AgentID,
			"skillIds": request.SkillIDs, "expectedOutputArtifactTypes": request.ExpectedOutputArtifactTypes,
			"inputDigest": request.InputDigest, "promptDigest": request.PromptDigest,
			"routeKind": claim.Step.RouteKind, "routeId": claim.Step.RouteID, "rootRunId": claim.Run.RootRunID,
		})
		prepared, prepareErr := s.repo.PrepareClaimedAgentRuntimeAttempt(repository.AgentRuntimeAttemptMetadata{
			Owner: s.workerID + ":film", RunID: claim.Run.ID, StepID: claim.Step.ID, AttemptID: claim.Attempt.ID,
			ExpectedAttemptRevision: claim.Attempt.Revision, Executor: s.filmAgentExecutor.Name(), ModelRef: request.LogicalModelID,
			InputDigest: request.InputDigest, PromptDigest: request.PromptDigest, RequestJSON: requestMetadata, At: time.Now().UTC(),
		})
		if prepareErr != nil {
			err = prepareErr
		} else {
			attemptRevision = prepared.Revision
		}
	}
	var response filmAgentExecutionResponse
	if err == nil {
		response, err = s.filmAgentExecutor.Execute(ctx, request)
	}
	var output agentruntime.ExecutionOutput
	if err == nil {
		output, err = agentruntime.ParseExecutionOutput(response.Text, request.ExpectedOutputArtifactTypes)
		if err != nil {
			err = &filmAgentExecutionError{Code: "film_structured_output_invalid", Message: "Film Agent 返回内容未通过结构化 Artifact 校验", Err: err}
		}
	}
	select {
	case leaseErr := <-leaseLost:
		return leaseErr
	default:
	}
	if err != nil {
		return s.failClaimedFilmAgentExecution(*claim, attemptRevision, err)
	}
	responseJSON, err := agentruntime.CanonicalExecutionOutputJSON(output)
	if err != nil {
		return s.failClaimedFilmAgentExecution(*claim, attemptRevision, err)
	}
	writes, err := s.buildFilmAgentArtifactWrites(*claim, output)
	if err != nil {
		return s.failClaimedFilmAgentExecution(*claim, attemptRevision, err)
	}
	_, err = s.repo.CompleteAgentRuntimeExecution(repository.AgentRuntimeExecutionCompleteCommand{
		Owner: s.workerID + ":film", UserID: claim.Run.UserID, RunID: claim.Run.ID, StepID: claim.Step.ID, AttemptID: claim.Attempt.ID,
		ExpectedRunRevision: claim.Run.Revision, ExpectedStepRevision: claim.Step.Revision, ExpectedAttemptRevision: attemptRevision,
		ResponseJSON: responseJSON, ArtifactWrites: writes,
		Event: repository.AgentRuntimeEventInput{
			ID: newID(), EventType: "attempt.succeeded", ActorType: "executor", ActorID: s.filmAgentExecutor.Name(),
			PayloadJSON: mustFilmJSON(map[string]any{"summary": output.Summary, "artifactTypes": request.ExpectedOutputArtifactTypes, "modelRef": response.ModelRef}),
		},
		At: time.Now().UTC(),
	})
	return err
}

func (s *Service) failClaimedFilmAgentExecution(claim repository.AgentRuntimeExecutionClaim, attemptRevision int64, executionErr error) error {
	if errors.Is(executionErr, repository.ErrAgentRuntimeLeaseLost) || errors.Is(executionErr, repository.ErrAgentRuntimeStateConflict) {
		return executionErr
	}
	code, message := filmAgentFailureDetails(executionErr)
	_, err := s.repo.FailAgentRuntimeExecution(repository.AgentRuntimeExecutionFailCommand{
		Owner: s.workerID + ":film", UserID: claim.Run.UserID, RunID: claim.Run.ID, StepID: claim.Step.ID, AttemptID: claim.Attempt.ID,
		ExpectedRunRevision: claim.Run.Revision, ExpectedStepRevision: claim.Step.Revision, ExpectedAttemptRevision: attemptRevision,
		FailureCode: code, Failure: message,
		Event: repository.AgentRuntimeEventInput{
			ID: newID(), EventType: "attempt.failed", ActorType: "executor", ActorID: s.filmAgentExecutor.Name(),
			PayloadJSON: mustFilmJSON(map[string]any{"failureCode": code, "failure": message}),
		},
		At: time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	return executionErr
}

func filmAgentFailureDetails(err error) (string, string) {
	var unavailable *filmProviderUnavailableError
	if errors.As(err, &unavailable) {
		return "film_provider_not_available", truncateRunes(unavailable.Error(), 2000)
	}
	var executionErr *filmAgentExecutionError
	if errors.As(err, &executionErr) {
		return defaultString(strings.TrimSpace(executionErr.Code), "film_executor_failed"), truncateRunes(executionErr.Error(), 2000)
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "film_executor_timeout", "Film Agent 执行超时"
	case errors.Is(err, context.Canceled):
		return "film_executor_cancelled", "Film Agent 执行已取消"
	default:
		return "film_executor_failed", truncateRunes(err.Error(), 2000)
	}
}

func (s *Service) buildFilmAgentExecutionRequest(claim repository.AgentRuntimeExecutionClaim) (filmAgentExecutionRequest, error) {
	var runEnvelope struct {
		LogicalModelID string         `json:"logicalModelId"`
		Input          map[string]any `json:"input"`
		Execution      filmAgentExecutionMetadata `json:"execution"`
	}
	if err := json.Unmarshal([]byte(claim.Run.InputJSON), &runEnvelope); err != nil {
		return filmAgentExecutionRequest{}, fmt.Errorf("decode Film Agent Run input: %w", err)
	}
	if runEnvelope.Execution.Mode == "plan_only" {
		return filmAgentExecutionRequest{}, &filmProviderUnavailableError{Message: "NOT_AVAILABLE: Film Agent Run is plan-only; Provider execution is gated"}
	}
	if runEnvelope.Execution.RegistryDigest != "" && runEnvelope.Execution.RegistryDigest != claim.Run.RegistryDigest {
		return filmAgentExecutionRequest{}, errors.New("Film Agent execution metadata registry digest does not match Run")
	}
	var skillIDs []string
	if err := json.Unmarshal([]byte(claim.Step.SkillIDsJSON), &skillIDs); err != nil || len(skillIDs) == 0 {
		return filmAgentExecutionRequest{}, errors.New("Film Agent Step has no executable Skill")
	}
	var expectedTypes []string
	if err := json.Unmarshal([]byte(claim.Step.ExpectedOutputArtifactTypesJSON), &expectedTypes); err != nil {
		return filmAgentExecutionRequest{}, fmt.Errorf("decode Film Agent output contract: %w", err)
	}
	agent, ok := s.filmAgentRegistry.Agent(claim.Step.AgentID)
	if !ok {
		return filmAgentExecutionRequest{}, errors.New("Film Agent definition is no longer registered")
	}
	skills := make([]agentruntime.SkillDefinition, 0, len(skillIDs))
	for _, skillID := range skillIDs {
		skill, ok := s.filmAgentRegistry.Skill(skillID)
		if !ok || !filmContainsString(skill.OwnerAgentIDs, claim.Step.AgentID) {
			return filmAgentExecutionRequest{}, fmt.Errorf("Agent %s cannot execute Skill %s", claim.Step.AgentID, skillID)
		}
		skills = append(skills, skill)
	}
	artifacts, err := s.filmAgentPromptArtifacts(claim)
	if err != nil {
		return filmAgentExecutionRequest{}, err
	}
	dependencies, err := s.filmAgentDependencyResults(claim)
	if err != nil {
		return filmAgentExecutionRequest{}, err
	}
	promptPayload := map[string]any{
		"schemaVersion": 1, "objective": claim.Run.Objective, "runInput": runEnvelope.Input,
		"inputArtifacts": artifacts, "dependencyResults": dependencies,
		"expectedOutputArtifactTypes": expectedTypes,
	}
	promptJSON, err := json.Marshal(promptPayload)
	if err != nil {
		return filmAgentExecutionRequest{}, err
	}
	var system strings.Builder
	system.WriteString("You are executing a versioned Film AgentTeam step. Follow the Agent and Skill instructions below, preserve all locked facts, and do not claim media generation without real evidence.\n\n")
	system.WriteString("[AGENT " + agent.ID + "]\n" + agent.DeveloperInstructions + "\n")
	for _, skill := range skills {
		system.WriteString("\n[SKILL " + skill.ID + " v" + skill.Version + "]\n" + skill.Instructions + "\n")
	}
	system.WriteString("\n[RUNTIME OUTPUT CONTRACT]\nReturn exactly one JSON object and no prose outside it. Use schemaVersion 1, a non-empty summary, and an artifacts array. Emit exactly one artifact for every expected type and no other types. Each artifact may contain type, title, content (a JSON object), and contentText. Never emit IDs, versions, status, responsible, authority, timestamps, provider credentials, or claims that an unexecuted media task succeeded; the runtime assigns those facts. Expected types: ")
	system.WriteString(mustFilmJSON(expectedTypes))
	systemPrompt := system.String()
	if len(promptJSON)+len(systemPrompt) > maxFilmAgentPromptBytes {
		return filmAgentExecutionRequest{}, errors.New("Film Agent execution context exceeds 1 MiB")
	}
	inputDigest := digestBytesHex(promptJSON)
	promptDigest := digestString(systemPrompt + "\n" + string(promptJSON))
	return filmAgentExecutionRequest{
		UserID: claim.Run.UserID, ProjectID: claim.Run.ProjectID, RunID: claim.Run.ID, StepID: claim.Step.ID,
		AttemptID: claim.Attempt.ID, TaskID: claim.Attempt.TaskID, LogicalModelID: strings.TrimSpace(runEnvelope.LogicalModelID),
		AgentID: agent.ID, SkillIDs: skillIDs, ExpectedOutputArtifactTypes: expectedTypes,
		SystemPrompt: systemPrompt, Prompt: string(promptJSON), InputDigest: inputDigest, PromptDigest: promptDigest,
	}, nil
}

type filmAgentPromptArtifact struct {
	Ref         FilmProductionArtifactRef `json:"ref"`
	Content     any                       `json:"content,omitempty"`
	ContentText string                    `json:"contentText,omitempty"`
}

func (s *Service) filmAgentPromptArtifacts(claim repository.AgentRuntimeExecutionClaim) ([]filmAgentPromptArtifact, error) {
	var refs []FilmProductionArtifactRef
	if err := json.Unmarshal([]byte(claim.Step.InputArtifactRefsJSON), &refs); err != nil {
		return nil, fmt.Errorf("decode Film Agent input Artifact refs: %w", err)
	}
	result := make([]filmAgentPromptArtifact, 0, len(refs))
	for _, ref := range refs {
		artifact, revision, err := s.repo.ProductionArtifactRevisionForUser(claim.Run.UserID, ref.RevisionID)
		if err != nil {
			return nil, err
		}
		if artifact.ProjectID != claim.Run.ProjectID || artifact.Domain != "film" || artifact.ID != ref.ArtifactID ||
			artifact.ArtifactType != ref.Type || revision.ContentDigest != ref.Digest {
			return nil, errors.New("Film Agent input Artifact reference no longer matches persisted evidence")
		}
		item := filmAgentPromptArtifact{Ref: ref, ContentText: revision.ContentText}
		if strings.TrimSpace(revision.ContentJSON) != "" {
			if err := json.Unmarshal([]byte(revision.ContentJSON), &item.Content); err != nil {
				return nil, fmt.Errorf("decode Film Artifact %s content: %w", revision.ID, err)
			}
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *Service) filmAgentDependencyResults(claim repository.AgentRuntimeExecutionClaim) ([]map[string]any, error) {
	var dependencyIDs []string
	if err := json.Unmarshal([]byte(claim.Step.DependsOnStepIDsJSON), &dependencyIDs); err != nil {
		return nil, err
	}
	if len(dependencyIDs) == 0 {
		return []map[string]any{}, nil
	}
	detail, err := s.repo.AgentRuntimeDetailForUser(claim.Run.UserID, claim.Run.ID)
	if err != nil {
		return nil, err
	}
	latest := make(map[string]model.AgentRuntimeAttempt)
	for _, attempt := range detail.Attempts {
		if attempt.Status == model.AgentAttemptStatusSucceeded && attempt.Number >= latest[attempt.StepID].Number {
			latest[attempt.StepID] = attempt
		}
	}
	result := make([]map[string]any, 0, len(dependencyIDs))
	for _, stepID := range dependencyIDs {
		attempt, ok := latest[stepID]
		if !ok {
			return nil, fmt.Errorf("Film Agent dependency Step %s has no successful Attempt", stepID)
		}
		var output any
		if err := json.Unmarshal([]byte(attempt.ResponseJSON), &output); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{"stepId": stepID, "attemptId": attempt.ID, "output": output})
	}
	return result, nil
}

func (s *Service) buildFilmAgentArtifactWrites(claim repository.AgentRuntimeExecutionClaim, output agentruntime.ExecutionOutput) ([]repository.ProductionArtifactWrite, error) {
	writes := make([]repository.ProductionArtifactWrite, 0, len(output.Artifacts))
	for _, draft := range output.Artifacts {
		logicalKey := "step:" + claim.Step.ID + ":" + draft.Type
		artifact := model.ProductionArtifact{
			ID: newID(), UserID: claim.Run.UserID, ProjectID: claim.Run.ProjectID, Domain: "film",
			ArtifactType: draft.Type, LogicalKey: logicalKey,
		}
		expectedSequence := 0
		if existing, err := s.repo.ProductionArtifactByLogicalKey(claim.Run.UserID, claim.Run.ProjectID, "film", logicalKey); err == nil {
			if existing.ArtifactType != draft.Type {
				return nil, errors.New("Film Agent output Artifact logical identity changed type")
			}
			artifact = *existing
			expectedSequence = existing.RevisionSequence
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		contentJSON, err := json.Marshal(map[string]any{
			"schemaVersion": 1, "artifactType": draft.Type, "title": strings.TrimSpace(draft.Title),
			"summary": output.Summary, "content": draft.Content,
		})
		if err != nil {
			return nil, err
		}
		authorityRefs := []map[string]any{{
			"kind": "registry", "id": claim.Run.RegistryID, "version": claim.Run.RegistryVersion, "digest": claim.Run.RegistryDigest,
		}, {"kind": "agent", "id": claim.Step.AgentID}}
		var skillIDs []string
		_ = json.Unmarshal([]byte(claim.Step.SkillIDsJSON), &skillIDs)
		for _, skillID := range skillIDs {
			skill, _ := s.filmAgentRegistry.Skill(skillID)
			authorityRefs = append(authorityRefs, map[string]any{"kind": "skill", "id": skill.ID, "version": skill.Version, "digest": skill.SourceDigest})
		}
		revision := model.ProductionArtifactRevision{
			ID: newID(), Status: model.ProductionArtifactStatusReview, ContentJSON: string(contentJSON), ContentText: draft.ContentText,
			ContentDigest: digestString(string(contentJSON) + "\n" + draft.ContentText),
			SourceRunID:   claim.Run.ID, SourceStepID: claim.Step.ID, SourceAttemptID: claim.Attempt.ID,
			SourceArtifactRefsJSON: claim.Step.InputArtifactRefsJSON, AuthorityRefsJSON: mustFilmJSON(authorityRefs),
			CreatedByType: "agent", CreatedByID: claim.Step.AgentID,
		}
		writes = append(writes, repository.ProductionArtifactWrite{Artifact: artifact, Revision: revision, ExpectedSequence: expectedSequence})
	}
	return writes, nil
}
