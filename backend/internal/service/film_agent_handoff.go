package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"infinite-canvas/backend/internal/agentruntime"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/gorm"
)

const (
	filmAgentHandoffWorkerConcurrency = 2
	filmAgentHandoffLeaseDuration     = 30 * time.Second
	filmAgentHandoffMaxAttempts       = 5
)

var automaticFilmHandoffRouteIDs = map[string]struct{}{
	"HR-01": {}, "HR-02": {}, "HR-03": {}, "HR-04": {},
	"HR-05": {}, "HR-06": {}, "HR-07": {}, "HR-08": {},
}

type filmHandoffSelection struct {
	Refs        []FilmProductionArtifactRef
	Facts       []repository.ProductionArtifactRevisionFact
	ParentRunID string
}

func (s *Service) startFilmAgentHandoffWorker() {
	if s.filmAgentRegistry == nil {
		return
	}
	go func() {
		slots := make(chan struct{}, filmAgentHandoffWorkerConcurrency)
		dispatch := func() {
			for len(slots) < filmAgentHandoffWorkerConcurrency {
				release, acquired, err := s.coordinator.acquire(context.Background(), "film-agent-handoff-workers", filmAgentHandoffWorkerConcurrency, filmAgentHandoffLeaseDuration)
				if err != nil || !acquired {
					return
				}
				claim, err := s.claimNextFilmAgentHandoffTrigger()
				if err != nil || claim == nil {
					release()
					if err != nil {
						log.Printf("claim Film Agent Handoff Trigger failed: %v", err)
					}
					return
				}
				slots <- struct{}{}
				go func(claim *repository.AgentHandoffTriggerClaim) {
					defer func() { <-slots; release() }()
					if err := s.handleClaimedFilmAgentHandoffTrigger(claim); err != nil && !errors.Is(err, repository.ErrAgentHandoffTriggerLeaseLost) {
						log.Printf("process Film Agent Handoff Trigger id=%s failed: %v", claim.Trigger.ID, err)
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

func (s *Service) ProcessNextFilmAgentHandoff() (bool, error) {
	claim, err := s.claimNextFilmAgentHandoffTrigger()
	if err != nil || claim == nil {
		return false, err
	}
	return true, s.handleClaimedFilmAgentHandoffTrigger(claim)
}

func (s *Service) claimNextFilmAgentHandoffTrigger() (*repository.AgentHandoffTriggerClaim, error) {
	if err := s.ValidateRuntime(); err != nil {
		return nil, err
	}
	return s.repo.ClaimNextAgentHandoffTrigger(repository.AgentHandoffTriggerClaimCommand{
		Owner: s.workerID + ":film-handoff", Domain: "film", LeaseDuration: filmAgentHandoffLeaseDuration, At: time.Now().UTC(),
	})
}

func (s *Service) handleClaimedFilmAgentHandoffTrigger(claim *repository.AgentHandoffTriggerClaim) error {
	err := s.processClaimedFilmAgentHandoffTrigger(claim)
	if err == nil {
		return nil
	}
	if errors.Is(err, repository.ErrAgentHandoffTriggerLeaseLost) {
		return err
	}
	now := time.Now().UTC()
	terminal := claim.Trigger.AttemptCount >= filmAgentHandoffMaxAttempts
	var retryAt *time.Time
	if !terminal {
		delay := filmAgentHandoffRetryDelay(claim.Trigger.AttemptCount)
		next := now.Add(delay)
		retryAt = &next
	}
	_, failErr := s.repo.FailAgentHandoffTrigger(repository.AgentHandoffTriggerFailCommand{
		Owner: s.workerID + ":film-handoff", TriggerID: claim.Trigger.ID, ExpectedRevision: claim.Trigger.Revision,
		FailureCode: "film_handoff_processing_failed", Failure: truncateRunes(err.Error(), 2000),
		RetryAt: retryAt, Terminal: terminal, At: now,
	})
	if failErr != nil {
		return errors.Join(err, failErr)
	}
	return err
}

func (s *Service) processClaimedFilmAgentHandoffTrigger(claim *repository.AgentHandoffTriggerClaim) error {
	trigger := claim.Trigger
	if trigger.Domain != "film" || trigger.RootRunID == "" || trigger.ProjectID == "" || trigger.UserID == "" {
		return errors.New("Film Handoff Trigger scope is incomplete")
	}
	if _, err := s.requireMutableFilmProject(trigger.UserID, trigger.ProjectID); err != nil {
		return err
	}
	root, err := s.repo.AgentRuntimeRunForUser(trigger.UserID, trigger.RootRunID)
	if err != nil {
		return err
	}
	if root.ProjectID != trigger.ProjectID || root.Domain != "film" || (root.RootRunID != "" && root.RootRunID != root.ID) {
		return errors.New("Film Handoff Trigger root Run scope is invalid")
	}
	if root.RegistryID != s.filmAgentRegistry.ID || root.RegistryVersion != s.filmAgentRegistry.Version || root.RegistryDigest != s.filmAgentRegistry.SourceDigest {
		return errors.New("Film Handoff root Run is pinned to a different AgentTeam registry version")
	}
	artifact, lockedRevision, err := s.repo.ProductionArtifactRevisionForUser(trigger.UserID, trigger.RevisionID)
	if err != nil {
		return err
	}
	if artifact.ID != trigger.ArtifactID || artifact.ProjectID != trigger.ProjectID || artifact.Domain != "film" ||
		lockedRevision.Status != model.ProductionArtifactStatusLocked || lockedRevision.SourceRunID != trigger.SourceRunID ||
		lockedRevision.SourceStepID != trigger.SourceStepID {
		return errors.New("Film Handoff Trigger no longer matches its immutable locked Artifact")
	}
	facts, err := s.repo.LockedProductionArtifactFactsForRoot(trigger.UserID, trigger.ProjectID, "film", trigger.RootRunID)
	if err != nil {
		return err
	}
	scheduledRunIDs := make([]string, 0)
	for _, route := range s.filmAgentRegistry.HandoffRoutes {
		if !isAutomaticFilmHandoffRoute(route) {
			continue
		}
		selection, ready := resolveFilmHandoffSelection(route, facts)
		if !ready {
			continue
		}
		runID, err := s.ensureFilmHandoffRun(trigger, *root, route, selection)
		if err != nil {
			return err
		}
		scheduledRunIDs = append(scheduledRunIDs, runID)
	}
	_, err = s.repo.CompleteAgentHandoffTrigger(repository.AgentHandoffTriggerCompleteCommand{
		Owner: s.workerID + ":film-handoff", TriggerID: trigger.ID, ExpectedRevision: trigger.Revision,
		ScheduledRunIDs: scheduledRunIDs, At: time.Now().UTC(),
	})
	return err
}

func isAutomaticFilmHandoffRoute(route agentruntime.HandoffRouteDefinition) bool {
	_, allowed := automaticFilmHandoffRouteIDs[route.ID]
	return allowed && route.ExecutionMode == "agent" && route.InputResolutionMode == "static" && route.RequiresLockedInput
}

func resolveFilmHandoffSelection(route agentruntime.HandoffRouteDefinition, facts []repository.ProductionArtifactRevisionFact) (filmHandoffSelection, bool) {
	selection := filmHandoffSelection{
		Refs:  make([]FilmProductionArtifactRef, 0, len(route.RequiredInputArtifactGroups)),
		Facts: make([]repository.ProductionArtifactRevisionFact, 0, len(route.RequiredInputArtifactGroups)),
	}
	for _, group := range route.RequiredInputArtifactGroups {
		candidates := make([]repository.ProductionArtifactRevisionFact, 0)
		for _, fact := range facts {
			if fact.Revision.Status == model.ProductionArtifactStatusLocked && filmContainsString(group, fact.Artifact.ArtifactType) {
				candidates = append(candidates, fact)
			}
		}
		if len(candidates) == 0 {
			return filmHandoffSelection{}, false
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].Revision.CreatedAt.Equal(candidates[j].Revision.CreatedAt) {
				return candidates[i].Revision.ID > candidates[j].Revision.ID
			}
			return candidates[i].Revision.CreatedAt.After(candidates[j].Revision.CreatedAt)
		})
		selected := candidates[0]
		selection.Facts = append(selection.Facts, selected)
		selection.Refs = append(selection.Refs, FilmProductionArtifactRef{
			ArtifactID: selected.Artifact.ID, RevisionID: selected.Revision.ID, Type: selected.Artifact.ArtifactType,
			Version: selected.Revision.Version, Digest: selected.Revision.ContentDigest, Status: string(selected.Revision.Status),
		})
	}
	var parentFact *repository.ProductionArtifactRevisionFact
	for index := range selection.Facts {
		fact := &selection.Facts[index]
		if !filmContainsString(route.FromAgentIDs, fact.SourceAgentID) {
			continue
		}
		if parentFact == nil || fact.Revision.CreatedAt.After(parentFact.Revision.CreatedAt) ||
			(fact.Revision.CreatedAt.Equal(parentFact.Revision.CreatedAt) && fact.Revision.ID > parentFact.Revision.ID) {
			parentFact = fact
		}
	}
	if parentFact == nil || strings.TrimSpace(parentFact.Revision.SourceRunID) == "" {
		return filmHandoffSelection{}, false
	}
	selection.ParentRunID = parentFact.Revision.SourceRunID
	return selection, true
}

func (s *Service) ensureFilmHandoffRun(trigger model.AgentHandoffTrigger, root model.AgentRuntimeRun, route agentruntime.HandoffRouteDefinition, selection filmHandoffSelection) (string, error) {
	parent, err := s.repo.AgentRuntimeRunForUser(trigger.UserID, selection.ParentRunID)
	if err != nil {
		return "", err
	}
	if parent.ProjectID != root.ProjectID || parent.Domain != root.Domain || (parent.ID != root.ID && parent.RootRunID != root.ID) {
		return "", errors.New("Film Handoff parent Run is outside the root lineage")
	}
	agentID, err := s.filmHandoffReceivingAgent(route)
	if err != nil {
		return "", err
	}
	requestIdentity, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "rootRunId": root.ID, "handoffRouteId": route.ID,
		"inputArtifactRefs": selection.Refs, "registryDigest": root.RegistryDigest,
	})
	if err != nil {
		return "", err
	}
	requestDigest := digestBytesHex(requestIdentity)
	idempotencyKey := "film-handoff:" + route.ID + ":" + requestDigest
	if existing, lookupErr := s.repo.AgentRuntimeRunByIdempotency(trigger.UserID, idempotencyKey); lookupErr == nil {
		if existing.ProjectID != root.ProjectID || existing.Domain != "film" || existing.RouteKind != "handoff" ||
			existing.HandoffRouteID != route.ID || existing.RootRunID != root.ID || filmRunRequestDigest(existing.InputJSON) != requestDigest {
			return "", errors.New("Film Handoff idempotency identity conflicts with an existing Run")
		}
		return existing.ID, nil
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return "", lookupErr
	}
	logicalModelID, err := filmAgentRunLogicalModelID(*parent)
	if err != nil {
		return "", err
	}
	if logicalModelID == "" {
		logicalModelID, err = filmAgentRunLogicalModelID(root)
		if err != nil {
			return "", err
		}
	}
	runID := newID()
	createdAt := time.Now().UTC()
	step := model.AgentRuntimeStep{
		ID: newID(), RunID: runID, StepKey: "handoff:" + route.ID, Position: 0,
		RouteKind: "handoff", RouteID: route.ID, AgentID: agentID, SkillIDsJSON: mustFilmJSON(route.SkillIDs),
		Status: model.AgentStepStatusReady, DependsOnStepIDsJSON: "[]", InputArtifactRefsJSON: mustFilmJSON(selection.Refs),
		ExpectedOutputArtifactTypesJSON: mustFilmJSON(route.OutputArtifactTypes), OutputArtifactRefsJSON: "[]",
		Revision: 1, CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	canvasID := strings.TrimSpace(parent.CanvasID)
	if canvasID == "" {
		canvasID = strings.TrimSpace(root.CanvasID)
	}
	runInputJSON, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "requestDigest": requestDigest, "logicalModelId": logicalModelID,
		"input": map[string]any{
			"handoffRouteId": route.ID, "handoffName": route.Name, "rootRunId": root.ID, "parentRunId": parent.ID,
		},
		"inputArtifactRefs": selection.Refs,
	})
	if err != nil {
		return "", err
	}
	run := &model.AgentRuntimeRun{
		ID: runID, UserID: trigger.UserID, ProjectID: root.ProjectID, CanvasID: canvasID, Domain: "film",
		RegistryID: root.RegistryID, RegistryVersion: root.RegistryVersion, RegistryDigest: root.RegistryDigest,
		RouteKind: "handoff", HandoffRouteID: route.ID, RootRunID: root.ID, ParentRunID: parent.ID,
		Status: model.AgentRunStatusReady, Objective: truncateRunes(route.Name+"："+root.Objective, 4000), InputJSON: string(runInputJSON),
		CurrentStepID: step.ID, IdempotencyKey: idempotencyKey, Revision: 1, EventSequence: 2,
		CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	routing := &model.AgentRoutingDecision{
		ID: newID(), RunID: run.ID, RouteKind: "handoff", RouteID: route.ID, SelectedAgentID: agentID,
		SelectedSkillIDsJSON: mustFilmJSON(route.SkillIDs), InputArtifactRefsJSON: mustFilmJSON(selection.Refs),
		AlternativesJSON: mustFilmJSON(route.ToAgentIDs), Reason: "已锁定 Artifact 满足 " + route.ID + " 的结构化输入合同",
		Confidence: "confirmed", DecidedByType: "runtime", DecidedByID: "film-handoff-orchestrator-v1", CreatedAt: createdAt,
	}
	events := []model.AgentRuntimeEvent{
		{ID: newID(), UserID: run.UserID, RunID: run.ID, Sequence: 1, EventType: "run.created", ActorType: "runtime", ActorID: "film-handoff-orchestrator-v1", ToStatus: string(model.AgentRunStatusPlanning), PayloadJSON: mustFilmJSON(map[string]any{"rootRunId": root.ID, "parentRunId": parent.ID, "triggerId": trigger.ID}), CreatedAt: createdAt},
		{ID: newID(), UserID: run.UserID, RunID: run.ID, Sequence: 2, StepID: step.ID, EventType: "handoff.planned", ActorType: "runtime", ActorID: "film-handoff-orchestrator-v1", FromStatus: string(model.AgentRunStatusPlanning), ToStatus: string(model.AgentRunStatusReady), PayloadJSON: mustFilmJSON(map[string]any{"handoffRouteId": route.ID, "agentId": agentID, "skillIds": route.SkillIDs, "inputArtifactRefs": selection.Refs}), CreatedAt: createdAt},
	}
	bundle := repository.AgentRuntimeCreateBundle{Run: run, RoutingDecision: routing, Steps: []model.AgentRuntimeStep{step}, Events: events}
	if err := s.repo.CreateAgentRuntimeBundle(bundle); err != nil {
		existing, lookupErr := s.repo.AgentRuntimeRunByIdempotency(trigger.UserID, idempotencyKey)
		if lookupErr == nil && existing.ProjectID == root.ProjectID && existing.Domain == "film" && existing.HandoffRouteID == route.ID &&
			existing.RootRunID == root.ID && filmRunRequestDigest(existing.InputJSON) == requestDigest {
			return existing.ID, nil
		}
		return "", err
	}
	return run.ID, nil
}

func (s *Service) filmHandoffReceivingAgent(route agentruntime.HandoffRouteDefinition) (string, error) {
	for _, candidate := range route.ToAgentIDs {
		ownsEverySkill := true
		for _, skillID := range route.SkillIDs {
			skill, ok := s.filmAgentRegistry.Skill(skillID)
			if !ok || !filmContainsString(skill.OwnerAgentIDs, candidate) {
				ownsEverySkill = false
				break
			}
		}
		if ownsEverySkill {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("Film Handoff %s 没有单一接收 Agent 可执行完整 Skill 合同", route.ID)
}

func filmAgentRunLogicalModelID(run model.AgentRuntimeRun) (string, error) {
	var envelope struct {
		LogicalModelID string `json:"logicalModelId"`
	}
	if err := json.Unmarshal([]byte(run.InputJSON), &envelope); err != nil {
		return "", fmt.Errorf("decode Film Agent Run model inheritance: %w", err)
	}
	return strings.TrimSpace(envelope.LogicalModelID), nil
}

func filmAgentHandoffRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 8 {
		attempt = 8
	}
	return time.Duration(1<<(attempt-1)) * time.Second
}
