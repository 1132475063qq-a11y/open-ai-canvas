package service

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	"infinite-canvas/backend/internal/agentruntime"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/gorm"
)

const (
	ecommerceAgentHandoffLeaseDuration = 30 * time.Second
	ecommerceAgentHandoffMaxAttempts   = 5
)

type ecommerceHandoffSelection struct {
	Refs        []EcommerceTargetArtifactRef
	Facts       []repository.ProductionArtifactRevisionFact
	ParentRunID string
}

// ProcessNextEcommerceAgentHandoff claims one durable locked-Artifact trigger
// and creates idempotent child Runs for every ready Ecommerce Handoff route.
// It is safe to call from a worker, a test, or a recovery pass after restart.
func (s *Service) ProcessNextEcommerceAgentHandoff() (bool, error) {
	claim, err := s.repo.ClaimNextAgentHandoffTrigger(repository.AgentHandoffTriggerClaimCommand{Owner: s.workerID + ":ecommerce-handoff", Domain: EcommerceTargetDomain, LeaseDuration: ecommerceAgentHandoffLeaseDuration, At: time.Now().UTC()})
	if err != nil || claim == nil {
		return false, err
	}
	if err := s.processClaimedEcommerceAgentHandoff(claim); err != nil {
		if errors.Is(err, repository.ErrAgentHandoffTriggerLeaseLost) {
			return true, err
		}
		now := time.Now().UTC()
		terminal := claim.Trigger.AttemptCount >= ecommerceAgentHandoffMaxAttempts
		var retryAt *time.Time
		if !terminal {
			next := now.Add(ecommerceHandoffRetryDelay(claim.Trigger.AttemptCount))
			retryAt = &next
		}
		_, failErr := s.repo.FailAgentHandoffTrigger(repository.AgentHandoffTriggerFailCommand{Owner: s.workerID + ":ecommerce-handoff", TriggerID: claim.Trigger.ID, ExpectedRevision: claim.Trigger.Revision, FailureCode: "ecommerce_handoff_processing_failed", Failure: truncateRunes(err.Error(), 2000), RetryAt: retryAt, Terminal: terminal, At: now})
		if failErr != nil {
			return true, errors.Join(err, failErr)
		}
		return true, err
	}
	return true, nil
}

func ecommerceHandoffRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 6 {
		attempt = 6
	}
	return time.Duration(1<<uint(attempt-1)) * time.Second
}

func (s *Service) processClaimedEcommerceAgentHandoff(claim *repository.AgentHandoffTriggerClaim) error {
	trigger := claim.Trigger
	if trigger.Domain != EcommerceTargetDomain || trigger.RootRunID == "" || trigger.ProjectID == "" || trigger.UserID == "" {
		return errors.New("Ecommerce Handoff Trigger scope is incomplete")
	}
	if _, err := s.requireEcommerceProject(trigger.UserID, trigger.ProjectID); err != nil {
		return err
	}
	root, err := s.repo.AgentRuntimeRunForUser(trigger.UserID, trigger.RootRunID)
	if err != nil {
		return err
	}
	if root.ProjectID != trigger.ProjectID || root.Domain != EcommerceTargetDomain || (root.RootRunID != "" && root.RootRunID != root.ID) {
		return errors.New("Ecommerce Handoff root Run scope is invalid")
	}
	if root.RegistryID != ecommerceTargetRegistry.ID || root.RegistryVersion != ecommerceTargetRegistry.Version || root.RegistryDigest != ecommerceTargetRegistry.SourceDigest {
		return errors.New("Ecommerce Handoff root Run is pinned to a different registry")
	}
	artifact, lockedRevision, err := s.repo.ProductionArtifactRevisionForUser(trigger.UserID, trigger.RevisionID)
	if err != nil {
		return err
	}
	if artifact.ID != trigger.ArtifactID || artifact.ProjectID != trigger.ProjectID || artifact.Domain != EcommerceTargetDomain || lockedRevision.Status != model.ProductionArtifactStatusLocked || lockedRevision.SourceRunID != trigger.SourceRunID || lockedRevision.SourceStepID != trigger.SourceStepID {
		return errors.New("Ecommerce Handoff Trigger no longer matches its immutable locked Artifact")
	}
	facts, err := s.repo.LockedProductionArtifactFactsForRoot(trigger.UserID, trigger.ProjectID, EcommerceTargetDomain, trigger.RootRunID)
	if err != nil {
		return err
	}
	scheduled := make([]string, 0, len(ecommerceTargetRegistry.HandoffRoutes))
	for _, route := range ecommerceTargetRegistry.HandoffRoutes {
		selection, ready := resolveEcommerceHandoffSelection(route, facts)
		if !ready {
			continue
		}
		runID, err := s.ensureEcommerceHandoffRun(trigger, *root, route, selection)
		if err != nil {
			return err
		}
		scheduled = append(scheduled, runID)
	}
	_, err = s.repo.CompleteAgentHandoffTrigger(repository.AgentHandoffTriggerCompleteCommand{Owner: s.workerID + ":ecommerce-handoff", TriggerID: trigger.ID, ExpectedRevision: trigger.Revision, ScheduledRunIDs: scheduled, At: time.Now().UTC()})
	return err
}

func resolveEcommerceHandoffSelection(route agentruntime.HandoffRouteDefinition, facts []repository.ProductionArtifactRevisionFact) (ecommerceHandoffSelection, bool) {
	selection := ecommerceHandoffSelection{Refs: make([]EcommerceTargetArtifactRef, 0, len(route.RequiredInputArtifactGroups)), Facts: make([]repository.ProductionArtifactRevisionFact, 0, len(route.RequiredInputArtifactGroups))}
	for _, group := range route.RequiredInputArtifactGroups {
		candidates := make([]repository.ProductionArtifactRevisionFact, 0)
		for _, fact := range facts {
			if fact.Revision.Status != model.ProductionArtifactStatusLocked || !ecommerceContainsString(group, fact.Artifact.ArtifactType) {
				continue
			}
			candidates = append(candidates, fact)
		}
		if len(candidates) == 0 {
			return ecommerceHandoffSelection{}, false
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].Revision.CreatedAt.Equal(candidates[j].Revision.CreatedAt) {
				return candidates[i].Revision.ID > candidates[j].Revision.ID
			}
			return candidates[i].Revision.CreatedAt.After(candidates[j].Revision.CreatedAt)
		})
		selected := candidates[0]
		selection.Facts = append(selection.Facts, selected)
		selection.Refs = append(selection.Refs, EcommerceTargetArtifactRef{ArtifactID: selected.Artifact.ID, RevisionID: selected.Revision.ID, Type: selected.Artifact.ArtifactType, Version: selected.Revision.Version, Digest: selected.Revision.ContentDigest, Status: string(selected.Revision.Status)})
	}
	var parent *repository.ProductionArtifactRevisionFact
	for index := range selection.Facts {
		fact := &selection.Facts[index]
		if !ecommerceContainsString(route.FromAgentIDs, fact.SourceAgentID) || strings.TrimSpace(fact.Revision.SourceRunID) == "" {
			continue
		}
		if parent == nil || fact.Revision.CreatedAt.After(parent.Revision.CreatedAt) || (fact.Revision.CreatedAt.Equal(parent.Revision.CreatedAt) && fact.Revision.ID > parent.Revision.ID) {
			parent = fact
		}
	}
	if parent == nil {
		return ecommerceHandoffSelection{}, false
	}
	selection.ParentRunID = parent.Revision.SourceRunID
	return selection, true
}

func ecommerceContainsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (s *Service) ensureEcommerceHandoffRun(trigger model.AgentHandoffTrigger, root model.AgentRuntimeRun, route agentruntime.HandoffRouteDefinition, selection ecommerceHandoffSelection) (string, error) {
	parent, err := s.repo.AgentRuntimeRunForUser(trigger.UserID, selection.ParentRunID)
	if err != nil {
		return "", err
	}
	if parent.ProjectID != root.ProjectID || parent.Domain != EcommerceTargetDomain || (parent.ID != root.ID && parent.RootRunID != root.ID) || parent.RegistryID != root.RegistryID || parent.RegistryVersion != root.RegistryVersion || parent.RegistryDigest != root.RegistryDigest {
		return "", errors.New("Ecommerce Handoff parent Run is outside the pinned root lineage")
	}
	identity, err := json.Marshal(map[string]any{"schemaVersion": 1, "rootRunId": root.ID, "handoffRouteId": route.ID, "inputArtifactRefs": selection.Refs, "registryDigest": root.RegistryDigest})
	if err != nil {
		return "", err
	}
	requestDigest := ecommerceDigestBytes(identity)
	idempotencyKey := "ecommerce-handoff:" + route.ID + ":" + requestDigest
	if existing, lookupErr := s.repo.AgentRuntimeRunByIdempotency(trigger.UserID, idempotencyKey); lookupErr == nil {
		if existing.ProjectID != root.ProjectID || existing.Domain != EcommerceTargetDomain || existing.RouteKind != "handoff" || existing.HandoffRouteID != route.ID || existing.RootRunID != root.ID || ecommerceTargetRunRequestDigest(existing.InputJSON) != requestDigest {
			return "", errors.New("Ecommerce Handoff idempotency identity conflicts with an existing Run")
		}
		return existing.ID, nil
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return "", lookupErr
	}
	now := time.Now().UTC()
	runID := newID()
	stepID := newID()
	runInput, _ := json.Marshal(map[string]any{"schemaVersion": 1, "requestDigest": requestDigest, "inputArtifactRefs": selection.Refs, "registry": map[string]string{"id": root.RegistryID, "version": root.RegistryVersion, "digest": root.RegistryDigest}})
	run := &model.AgentRuntimeRun{ID: runID, UserID: trigger.UserID, ProjectID: root.ProjectID, CanvasID: firstNonEmpty(parent.CanvasID, root.CanvasID), Domain: EcommerceTargetDomain, RegistryID: root.RegistryID, RegistryVersion: root.RegistryVersion, RegistryDigest: root.RegistryDigest, RouteKind: "handoff", HandoffRouteID: route.ID, RootRunID: root.ID, ParentRunID: parent.ID, Status: model.AgentRunStatusReady, Objective: route.Name, InputJSON: string(runInput), CurrentStepID: stepID, IdempotencyKey: idempotencyKey, Revision: 1, EventSequence: 1, CreatedAt: now, UpdatedAt: now}
	step := &model.AgentRuntimeStep{ID: stepID, RunID: runID, StepKey: "handoff:" + route.ID, Position: 0, RouteKind: "handoff", RouteID: route.ID, AgentID: route.ToAgentIDs[0], SkillIDsJSON: mustEcommerceJSON(route.SkillIDs), Status: model.AgentStepStatusReady, DependsOnStepIDsJSON: "[]", InputArtifactRefsJSON: mustEcommerceJSON(selection.Refs), ExpectedOutputArtifactTypesJSON: mustEcommerceJSON(route.OutputArtifactTypes), OutputArtifactRefsJSON: "[]", Revision: 1, CreatedAt: now, UpdatedAt: now}
	routing := &model.AgentRoutingDecision{ID: newID(), RunID: runID, RouteKind: "handoff", RouteID: route.ID, SelectedAgentID: step.AgentID, SelectedSkillIDsJSON: step.SkillIDsJSON, InputArtifactRefsJSON: step.InputArtifactRefsJSON, AlternativesJSON: "[]", Reason: "locked Ecommerce Artifact handoff", Confidence: "deterministic_match", DecidedByType: "runtime", DecidedByID: EcommerceAgentRegistryID, CreatedAt: now}
	events := []model.AgentRuntimeEvent{{ID: newID(), UserID: trigger.UserID, RunID: runID, Sequence: 1, StepID: step.ID, EventType: "handoff.scheduled", ActorType: "runtime", ActorID: EcommerceAgentRegistryID, ToStatus: string(model.AgentRunStatusReady), PayloadJSON: mustEcommerceJSON(map[string]any{"handoffRouteId": route.ID, "inputArtifactRefs": selection.Refs}), CreatedAt: now}}
	if err := s.repo.CreateAgentRuntimeBundle(repository.AgentRuntimeCreateBundle{Run: run, RoutingDecision: routing, Steps: []model.AgentRuntimeStep{*step}, Events: events}); err != nil {
		if existing, lookupErr := s.repo.AgentRuntimeRunByIdempotency(trigger.UserID, idempotencyKey); lookupErr == nil {
			return existing.ID, nil
		}
		return "", err
	}
	return runID, nil
}

func ecommerceDigestBytes(value []byte) string {
	// Keep this helper local to Ecommerce so target/handoff identity cannot
	// accidentally inherit Film request canonicalization.
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
