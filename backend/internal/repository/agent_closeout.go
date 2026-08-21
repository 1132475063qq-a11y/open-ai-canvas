package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AgentRuntimeLineageSnapshot struct {
	Project          model.Project
	RootRun          model.AgentRuntimeRun
	ProjectFilmRuns  []model.AgentRuntimeRun
	Runs             []model.AgentRuntimeRun
	Steps            []model.AgentRuntimeStep
	Attempts         []model.AgentRuntimeAttempt
	RoutingDecisions []model.AgentRoutingDecision
	HumanDecisions   []model.AgentHumanDecision
	HandoffTriggers  []model.AgentHandoffTrigger
	CurrentArtifacts []ProductionArtifactRevisionFact
}

type AgentRuntimeCloseoutCommand struct {
	UserID                      string
	ProjectID                   string
	Domain                      string
	RootRunID                   string
	ExpectedProjectRevision     int64
	ExpectedRootRunRevision     int64
	ExpectedEvidenceFingerprint string
	SummaryArtifact             *model.ProductionArtifact
	SummaryRevision             *model.ProductionArtifactRevision
	QualityHandoffEvent         AgentRuntimeEventInput
	CompletionHandoffEvent      AgentRuntimeEventInput
	At                          time.Time
}

type AgentRuntimeCloseoutResult struct {
	Project         model.Project                    `json:"project"`
	RootRun         model.AgentRuntimeRun            `json:"rootRun"`
	SummaryArtifact model.ProductionArtifact         `json:"summaryArtifact"`
	SummaryRevision model.ProductionArtifactRevision `json:"summaryRevision"`
}

func (r *Repository) AgentRuntimeLineageSnapshotForRoot(userID string, projectID string, domain string, rootRunID string) (AgentRuntimeLineageSnapshot, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(projectID) == "" || strings.TrimSpace(domain) == "" || strings.TrimSpace(rootRunID) == "" {
		return AgentRuntimeLineageSnapshot{}, errors.New("agent runtime lineage snapshot scope is incomplete")
	}
	var snapshot AgentRuntimeLineageSnapshot
	err := r.db.Transaction(func(tx *gorm.DB) error {
		loaded, err := agentRuntimeLineageSnapshotTx(tx, userID, projectID, domain, rootRunID, false)
		if err != nil {
			return err
		}
		snapshot = loaded
		return nil
	})
	return snapshot, err
}

func AgentRuntimeLineageFingerprint(snapshot AgentRuntimeLineageSnapshot) (string, error) {
	type runFact struct {
		ID              string                      `json:"id"`
		RootRunID       string                      `json:"rootRunId"`
		ParentRunID     string                      `json:"parentRunId"`
		RegistryID      string                      `json:"registryId"`
		RegistryVersion string                      `json:"registryVersion"`
		RegistryDigest  string                      `json:"registryDigest"`
		RouteKind       string                      `json:"routeKind"`
		IntentRouteID   string                      `json:"intentRouteId"`
		HandoffRouteID  string                      `json:"handoffRouteId"`
		Status          model.AgentRuntimeRunStatus `json:"status"`
		Revision        int64                       `json:"revision"`
		EventSequence   int64                       `json:"eventSequence"`
	}
	type stepFact struct {
		ID                      string                       `json:"id"`
		RunID                   string                       `json:"runId"`
		RouteID                 string                       `json:"routeId"`
		AgentID                 string                       `json:"agentId"`
		SkillIDsJSON            string                       `json:"skillIdsJson"`
		Status                  model.AgentRuntimeStepStatus `json:"status"`
		ExpectedOutputTypesJSON string                       `json:"expectedOutputTypesJson"`
		AttemptSequence         int                          `json:"attemptSequence"`
		Revision                int64                        `json:"revision"`
	}
	type attemptFact struct {
		ID       string                          `json:"id"`
		RunID    string                          `json:"runId"`
		StepID   string                          `json:"stepId"`
		Number   int                             `json:"number"`
		TaskID   string                          `json:"taskId"`
		Status   model.AgentRuntimeAttemptStatus `json:"status"`
		Revision int64                           `json:"revision"`
	}
	type routingFact struct {
		ID                    string `json:"id"`
		RunID                 string `json:"runId"`
		RouteKind             string `json:"routeKind"`
		RouteID               string `json:"routeId"`
		SelectedAgentID       string `json:"selectedAgentId"`
		SelectedSkillIDsJSON  string `json:"selectedSkillIdsJson"`
		InputArtifactRefsJSON string `json:"inputArtifactRefsJson"`
		Reason                string `json:"reason"`
		Confidence            string `json:"confidence"`
	}
	type decisionFact struct {
		ID       string                         `json:"id"`
		RunID    string                         `json:"runId"`
		StepID   string                         `json:"stepId"`
		Status   model.AgentHumanDecisionStatus `json:"status"`
		Revision int64                          `json:"revision"`
	}
	type triggerFact struct {
		ID                  string                          `json:"id"`
		RootRunID           string                          `json:"rootRunId"`
		RevisionID          string                          `json:"revisionId"`
		SourceRunID         string                          `json:"sourceRunId"`
		Status              model.AgentHandoffTriggerStatus `json:"status"`
		AttemptCount        int                             `json:"attemptCount"`
		ScheduledRunIDsJSON string                          `json:"scheduledRunIdsJson"`
		Revision            int64                           `json:"revision"`
	}
	type artifactFact struct {
		ArtifactID       string                         `json:"artifactId"`
		ArtifactType     string                         `json:"artifactType"`
		LogicalKey       string                         `json:"logicalKey"`
		RevisionSequence int                            `json:"revisionSequence"`
		RevisionID       string                         `json:"revisionId"`
		Version          int                            `json:"version"`
		Status           model.ProductionArtifactStatus `json:"status"`
		ContentDigest    string                         `json:"contentDigest"`
		SourceRunID      string                         `json:"sourceRunId"`
		SourceStepID     string                         `json:"sourceStepId"`
		SourceAttemptID  string                         `json:"sourceAttemptId"`
	}

	runs := func(items []model.AgentRuntimeRun) []runFact {
		result := make([]runFact, 0, len(items))
		for _, item := range items {
			result = append(result, runFact{
				ID: item.ID, RootRunID: item.RootRunID, ParentRunID: item.ParentRunID, RouteKind: item.RouteKind,
				RegistryID: item.RegistryID, RegistryVersion: item.RegistryVersion, RegistryDigest: item.RegistryDigest,
				IntentRouteID: item.IntentRouteID, HandoffRouteID: item.HandoffRouteID, Status: item.Status,
				Revision: item.Revision, EventSequence: item.EventSequence,
			})
		}
		sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
		return result
	}
	steps := make([]stepFact, 0, len(snapshot.Steps))
	for _, item := range snapshot.Steps {
		steps = append(steps, stepFact{
			ID: item.ID, RunID: item.RunID, RouteID: item.RouteID, AgentID: item.AgentID,
			SkillIDsJSON: item.SkillIDsJSON, Status: item.Status,
			ExpectedOutputTypesJSON: item.ExpectedOutputArtifactTypesJSON,
			AttemptSequence:         item.AttemptSequence, Revision: item.Revision,
		})
	}
	sort.Slice(steps, func(i, j int) bool { return steps[i].ID < steps[j].ID })
	attempts := make([]attemptFact, 0, len(snapshot.Attempts))
	for _, item := range snapshot.Attempts {
		attempts = append(attempts, attemptFact{
			ID: item.ID, RunID: item.RunID, StepID: item.StepID, Number: item.Number,
			TaskID: item.TaskID, Status: item.Status, Revision: item.Revision,
		})
	}
	sort.Slice(attempts, func(i, j int) bool { return attempts[i].ID < attempts[j].ID })
	routing := make([]routingFact, 0, len(snapshot.RoutingDecisions))
	for _, item := range snapshot.RoutingDecisions {
		routing = append(routing, routingFact{
			ID: item.ID, RunID: item.RunID, RouteKind: item.RouteKind, RouteID: item.RouteID,
			SelectedAgentID: item.SelectedAgentID, SelectedSkillIDsJSON: item.SelectedSkillIDsJSON,
			InputArtifactRefsJSON: item.InputArtifactRefsJSON, Reason: item.Reason, Confidence: item.Confidence,
		})
	}
	sort.Slice(routing, func(i, j int) bool { return routing[i].ID < routing[j].ID })
	decisions := make([]decisionFact, 0, len(snapshot.HumanDecisions))
	for _, item := range snapshot.HumanDecisions {
		decisions = append(decisions, decisionFact{ID: item.ID, RunID: item.RunID, StepID: item.StepID, Status: item.Status, Revision: item.Revision})
	}
	sort.Slice(decisions, func(i, j int) bool { return decisions[i].ID < decisions[j].ID })
	triggers := make([]triggerFact, 0, len(snapshot.HandoffTriggers))
	for _, item := range snapshot.HandoffTriggers {
		triggers = append(triggers, triggerFact{
			ID: item.ID, RootRunID: item.RootRunID, RevisionID: item.RevisionID, SourceRunID: item.SourceRunID,
			Status: item.Status, AttemptCount: item.AttemptCount, ScheduledRunIDsJSON: item.ScheduledRunIDsJSON, Revision: item.Revision,
		})
	}
	sort.Slice(triggers, func(i, j int) bool { return triggers[i].ID < triggers[j].ID })
	artifacts := make([]artifactFact, 0, len(snapshot.CurrentArtifacts))
	for _, item := range snapshot.CurrentArtifacts {
		artifacts = append(artifacts, artifactFact{
			ArtifactID: item.Artifact.ID, ArtifactType: item.Artifact.ArtifactType, LogicalKey: item.Artifact.LogicalKey,
			RevisionSequence: item.Artifact.RevisionSequence, RevisionID: item.Revision.ID, Version: item.Revision.Version,
			Status: item.Revision.Status, ContentDigest: item.Revision.ContentDigest, SourceRunID: item.Revision.SourceRunID,
			SourceStepID: item.Revision.SourceStepID, SourceAttemptID: item.Revision.SourceAttemptID,
		})
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].ArtifactID < artifacts[j].ArtifactID })

	payload := struct {
		SchemaVersion    int                 `json:"schemaVersion"`
		ProjectID        string              `json:"projectId"`
		ProjectType      string              `json:"projectType"`
		ProjectStatus    model.ProjectStatus `json:"projectStatus"`
		ProjectRevision  int64               `json:"projectRevision"`
		RootRunID        string              `json:"rootRunId"`
		RegistryDigest   string              `json:"registryDigest"`
		ProjectFilmRuns  []runFact           `json:"projectFilmRuns"`
		Runs             []runFact           `json:"runs"`
		Steps            []stepFact          `json:"steps"`
		Attempts         []attemptFact       `json:"attempts"`
		RoutingDecisions []routingFact       `json:"routingDecisions"`
		HumanDecisions   []decisionFact      `json:"humanDecisions"`
		HandoffTriggers  []triggerFact       `json:"handoffTriggers"`
		CurrentArtifacts []artifactFact      `json:"currentArtifacts"`
	}{
		SchemaVersion: 1, ProjectID: snapshot.Project.ID, ProjectType: snapshot.Project.Type,
		ProjectStatus: snapshot.Project.Status, ProjectRevision: snapshot.Project.Revision,
		RootRunID: snapshot.RootRun.ID, RegistryDigest: snapshot.RootRun.RegistryDigest,
		ProjectFilmRuns: runs(snapshot.ProjectFilmRuns), Runs: runs(snapshot.Runs), Steps: steps,
		Attempts: attempts, RoutingDecisions: routing, HumanDecisions: decisions,
		HandoffTriggers: triggers, CurrentArtifacts: artifacts,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

// CloseAgentRuntimeRoot writes the immutable summary, HR-09/HR-11 events, and
// project archive state together. The in-transaction fingerprint prevents a
// preview from closing over newer runtime evidence.
func (r *Repository) CloseAgentRuntimeRoot(command AgentRuntimeCloseoutCommand) (*AgentRuntimeCloseoutResult, error) {
	if err := validateAgentRuntimeCloseoutCommand(command); err != nil {
		return nil, err
	}
	now := runtimeCommandTime(command.At)
	result := &AgentRuntimeCloseoutResult{}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		snapshot, err := agentRuntimeLineageSnapshotTx(tx, command.UserID, command.ProjectID, command.Domain, command.RootRunID, true)
		if err != nil {
			return err
		}
		fingerprint, err := AgentRuntimeLineageFingerprint(snapshot)
		if err != nil {
			return err
		}
		if snapshot.Project.Revision != command.ExpectedProjectRevision || snapshot.RootRun.Revision != command.ExpectedRootRunRevision ||
			fingerprint != command.ExpectedEvidenceFingerprint || snapshot.Project.Status != model.ProjectStatusActive {
			return ErrAgentRuntimeStateConflict
		}
		artifact, revision, err := createProductionArtifactRevisionTx(tx, ProductionArtifactRevisionCreate{
			UserID: command.UserID, Artifact: command.SummaryArtifact, Revision: command.SummaryRevision,
			ExpectedSequence: 0, At: now,
		})
		if err != nil {
			return err
		}
		rootUpdated := tx.Model(&model.AgentRuntimeRun{}).
			Where("id = ? AND user_id = ? AND revision = ? AND event_sequence = ?", snapshot.RootRun.ID, command.UserID, snapshot.RootRun.Revision, snapshot.RootRun.EventSequence).
			Updates(map[string]any{
				"revision": gorm.Expr("revision + ?", 1), "event_sequence": gorm.Expr("event_sequence + ?", 2), "updated_at": now,
			})
		if rootUpdated.Error != nil {
			return rootUpdated.Error
		}
		if rootUpdated.RowsAffected != 1 {
			return ErrAgentRuntimeStateConflict
		}
		if err := appendAgentRuntimeEvent(tx, snapshot.RootRun, command.QualityHandoffEvent, snapshot.RootRun.EventSequence+1, "", "", "", now); err != nil {
			return err
		}
		if err := appendAgentRuntimeEvent(tx, snapshot.RootRun, command.CompletionHandoffEvent, snapshot.RootRun.EventSequence+2, "", "", "", now); err != nil {
			return err
		}
		projectUpdated := tx.Model(&model.Project{}).
			Where("id = ? AND user_id = ? AND revision = ? AND status = ?", snapshot.Project.ID, command.UserID, snapshot.Project.Revision, model.ProjectStatusActive).
			Updates(map[string]any{
				"status": model.ProjectStatusArchived, "revision": gorm.Expr("revision + ?", 1), "updated_at": now,
			})
		if projectUpdated.Error != nil {
			return projectUpdated.Error
		}
		if projectUpdated.RowsAffected != 1 {
			return ErrAgentRuntimeStateConflict
		}
		if err := tx.First(&result.Project, "id = ? AND user_id = ?", command.ProjectID, command.UserID).Error; err != nil {
			return err
		}
		if err := tx.First(&result.RootRun, "id = ? AND user_id = ?", command.RootRunID, command.UserID).Error; err != nil {
			return err
		}
		result.SummaryArtifact = *artifact
		result.SummaryRevision = *revision
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func agentRuntimeLineageSnapshotTx(tx *gorm.DB, userID string, projectID string, domain string, rootRunID string, lock bool) (AgentRuntimeLineageSnapshot, error) {
	snapshot := AgentRuntimeLineageSnapshot{}
	query := func(value any) *gorm.DB {
		result := tx
		if lock {
			result = result.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		return result
	}
	if err := query(&snapshot.Project).First(&snapshot.Project, "id = ? AND user_id = ?", projectID, userID).Error; err != nil {
		return snapshot, err
	}
	if err := query(&snapshot.RootRun).First(&snapshot.RootRun, "id = ? AND user_id = ? AND project_id = ? AND domain = ?", rootRunID, userID, projectID, domain).Error; err != nil {
		return snapshot, err
	}
	if snapshot.RootRun.RootRunID != "" && snapshot.RootRun.RootRunID != snapshot.RootRun.ID {
		return snapshot, ErrAgentRuntimeStateConflict
	}
	projectRunsQuery := query(&snapshot.ProjectFilmRuns).Where("user_id = ? AND project_id = ? AND domain = ?", userID, projectID, domain).Order("id asc")
	if err := projectRunsQuery.Find(&snapshot.ProjectFilmRuns).Error; err != nil {
		return snapshot, err
	}
	runsQuery := query(&snapshot.Runs).
		Where("user_id = ? AND project_id = ? AND domain = ? AND (id = ? OR root_run_id = ?)", userID, projectID, domain, rootRunID, rootRunID).
		Order("id asc")
	if err := runsQuery.Find(&snapshot.Runs).Error; err != nil {
		return snapshot, err
	}
	runIDs := make([]string, 0, len(snapshot.Runs))
	for _, run := range snapshot.Runs {
		runIDs = append(runIDs, run.ID)
	}
	if len(runIDs) == 0 {
		return snapshot, gorm.ErrRecordNotFound
	}
	if err := query(&snapshot.Steps).Where("run_id IN ?", runIDs).Order("id asc").Find(&snapshot.Steps).Error; err != nil {
		return snapshot, err
	}
	if err := query(&snapshot.Attempts).Where("run_id IN ?", runIDs).Order("id asc").Find(&snapshot.Attempts).Error; err != nil {
		return snapshot, err
	}
	if err := query(&snapshot.RoutingDecisions).Where("run_id IN ?", runIDs).Order("id asc").Find(&snapshot.RoutingDecisions).Error; err != nil {
		return snapshot, err
	}
	if err := query(&snapshot.HumanDecisions).Where("run_id IN ?", runIDs).Order("id asc").Find(&snapshot.HumanDecisions).Error; err != nil {
		return snapshot, err
	}
	if err := query(&snapshot.HandoffTriggers).Where("user_id = ? AND project_id = ? AND domain = ? AND root_run_id = ?", userID, projectID, domain, rootRunID).Order("id asc").Find(&snapshot.HandoffTriggers).Error; err != nil {
		return snapshot, err
	}
	var artifacts []model.ProductionArtifact
	artifactQuery := query(&artifacts).
		Model(&model.ProductionArtifact{}).
		Select("production_artifacts.*").
		Joins("JOIN production_artifact_revisions current_revision ON current_revision.id = production_artifacts.current_revision_id").
		Where("production_artifacts.user_id = ? AND production_artifacts.project_id = ? AND production_artifacts.domain = ?", userID, projectID, domain).
		Where("current_revision.source_run_id IN ?", runIDs).
		Order("production_artifacts.id asc")
	if err := artifactQuery.Find(&artifacts).Error; err != nil {
		return snapshot, err
	}
	if len(artifacts) == 0 {
		return snapshot, nil
	}
	revisionIDs := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		revisionIDs = append(revisionIDs, artifact.CurrentRevisionID)
	}
	var revisions []model.ProductionArtifactRevision
	if err := query(&revisions).Where("id IN ?", revisionIDs).Find(&revisions).Error; err != nil {
		return snapshot, err
	}
	revisionByID := make(map[string]model.ProductionArtifactRevision, len(revisions))
	for _, revision := range revisions {
		revisionByID[revision.ID] = revision
	}
	agentByStepID := make(map[string]string, len(snapshot.Steps))
	for _, step := range snapshot.Steps {
		agentByStepID[step.ID] = step.AgentID
	}
	snapshot.CurrentArtifacts = make([]ProductionArtifactRevisionFact, 0, len(artifacts))
	for _, artifact := range artifacts {
		revision, ok := revisionByID[artifact.CurrentRevisionID]
		if !ok {
			return snapshot, errors.New("current production Artifact snapshot is inconsistent")
		}
		snapshot.CurrentArtifacts = append(snapshot.CurrentArtifacts, ProductionArtifactRevisionFact{
			Artifact: artifact, Revision: revision, SourceAgentID: agentByStepID[revision.SourceStepID],
		})
	}
	return snapshot, nil
}

func validateAgentRuntimeCloseoutCommand(command AgentRuntimeCloseoutCommand) error {
	if strings.TrimSpace(command.UserID) == "" || strings.TrimSpace(command.ProjectID) == "" || strings.TrimSpace(command.Domain) == "" ||
		strings.TrimSpace(command.RootRunID) == "" || command.ExpectedProjectRevision < 1 || command.ExpectedRootRunRevision < 1 ||
		len(strings.TrimSpace(command.ExpectedEvidenceFingerprint)) != 64 || command.SummaryArtifact == nil || command.SummaryRevision == nil {
		return errors.New("agent runtime closeout command is incomplete")
	}
	if command.SummaryArtifact.UserID != command.UserID || command.SummaryArtifact.ProjectID != command.ProjectID ||
		command.SummaryArtifact.Domain != command.Domain || command.SummaryArtifact.ArtifactType != "project-summary" ||
		command.SummaryRevision.SourceRunID != command.RootRunID || command.SummaryRevision.Status != model.ProductionArtifactStatusLocked {
		return errors.New("agent runtime closeout summary identity is invalid")
	}
	if err := validateAgentRuntimeEventInput(command.QualityHandoffEvent); err != nil {
		return err
	}
	return validateAgentRuntimeEventInput(command.CompletionHandoffEvent)
}
