package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/gorm"
)

var filmEvidenceFingerprintPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type FilmAgentCloseoutBlocker struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	RunID        string `json:"runId,omitempty"`
	StepID       string `json:"stepId,omitempty"`
	ArtifactType string `json:"artifactType,omitempty"`
}

type FilmAgentCloseoutPreview struct {
	ProjectID             string                            `json:"projectId"`
	RootRunID             string                            `json:"rootRunId"`
	ProjectRevision       int64                             `json:"projectRevision"`
	RootRunRevision       int64                             `json:"rootRunRevision"`
	Ready                 bool                              `json:"ready"`
	Completed             bool                              `json:"completed"`
	EvidenceFingerprint   string                            `json:"evidenceFingerprint"`
	RequiredArtifactTypes []string                          `json:"requiredArtifactTypes"`
	Deliverables          []FilmProductionArtifactRef       `json:"deliverables"`
	QualityReport         *FilmProductionArtifactRef        `json:"qualityReport,omitempty"`
	Blockers              []FilmAgentCloseoutBlocker        `json:"blockers"`
	SummaryArtifact       *model.ProductionArtifact         `json:"summaryArtifact,omitempty"`
	SummaryRevision       *model.ProductionArtifactRevision `json:"summaryRevision,omitempty"`
}

type ConfirmFilmAgentCloseoutRequest struct {
	ExpectedProjectRevision int64  `json:"expectedProjectRevision"`
	ExpectedRootRunRevision int64  `json:"expectedRootRunRevision"`
	EvidenceFingerprint     string `json:"evidenceFingerprint"`
	Confirm                 bool   `json:"confirm"`
	Note                    string `json:"note"`
}

type FilmAgentCloseoutResult struct {
	Project         model.Project                    `json:"project"`
	RootRun         model.AgentRuntimeRun            `json:"rootRun"`
	SummaryArtifact model.ProductionArtifact         `json:"summaryArtifact"`
	SummaryRevision model.ProductionArtifactRevision `json:"summaryRevision"`
	Idempotent      bool                             `json:"idempotent"`
}

func (s *Service) PreviewFilmAgentCloseout(userID string, projectID string, rootRunID string) (FilmAgentCloseoutPreview, error) {
	if err := s.ValidateRuntime(); err != nil {
		return FilmAgentCloseoutPreview{}, err
	}
	project, err := s.filmProjectForUser(userID, projectID)
	if err != nil {
		return FilmAgentCloseoutPreview{}, err
	}
	root, err := s.requireFilmAgentRootRun(userID, projectID, rootRunID)
	if err != nil {
		return FilmAgentCloseoutPreview{}, err
	}
	if artifact, revision, fingerprint, found, lookupErr := s.existingFilmAgentCloseout(userID, projectID, root.ID); lookupErr != nil {
		return FilmAgentCloseoutPreview{}, lookupErr
	} else if found {
		return FilmAgentCloseoutPreview{
			ProjectID: project.ID, RootRunID: root.ID, ProjectRevision: project.Revision, RootRunRevision: root.Revision,
			Ready: true, Completed: true, EvidenceFingerprint: fingerprint, Blockers: []FilmAgentCloseoutBlocker{},
			SummaryArtifact: artifact, SummaryRevision: revision,
		}, nil
	}
	if err := s.validateFilmCloseoutRegistry(*root); err != nil {
		return FilmAgentCloseoutPreview{}, err
	}
	snapshot, err := s.repo.AgentRuntimeLineageSnapshotForRoot(userID, projectID, "film", root.ID)
	if err != nil {
		return FilmAgentCloseoutPreview{}, err
	}
	return s.evaluateFilmAgentCloseout(snapshot)
}

func (s *Service) ConfirmFilmAgentCloseout(userID string, projectID string, rootRunID string, request ConfirmFilmAgentCloseoutRequest) (FilmAgentCloseoutResult, error) {
	if err := s.ValidateRuntime(); err != nil {
		return FilmAgentCloseoutResult{}, err
	}
	if !request.Confirm {
		return FilmAgentCloseoutResult{}, BadAuthRequest("最终收口必须由用户明确确认")
	}
	fingerprint := strings.ToLower(strings.TrimSpace(request.EvidenceFingerprint))
	if request.ExpectedProjectRevision < 1 || request.ExpectedRootRunRevision < 1 || !filmEvidenceFingerprintPattern.MatchString(fingerprint) {
		return FilmAgentCloseoutResult{}, BadAuthRequest("最终收口必须携带预检返回的项目版本、RootRun 版本和证据指纹")
	}
	note := strings.TrimSpace(request.Note)
	if len([]rune(note)) > 2000 {
		return FilmAgentCloseoutResult{}, BadAuthRequest("最终确认备注不能超过 2000 字")
	}
	project, err := s.filmProjectForUser(userID, projectID)
	if err != nil {
		return FilmAgentCloseoutResult{}, err
	}
	root, err := s.requireFilmAgentRootRun(userID, projectID, rootRunID)
	if err != nil {
		return FilmAgentCloseoutResult{}, err
	}
	if existing, ok, err := s.replayFilmAgentCloseout(userID, *project, *root, fingerprint); err != nil || ok {
		return existing, err
	}
	if err := s.validateFilmCloseoutRegistry(*root); err != nil {
		return FilmAgentCloseoutResult{}, err
	}
	if project.Status == model.ProjectStatusArchived {
		return FilmAgentCloseoutResult{}, conflictError("项目已归档，但没有与本次证据匹配的 Film 收口记录")
	}
	snapshot, err := s.repo.AgentRuntimeLineageSnapshotForRoot(userID, projectID, "film", root.ID)
	if err != nil {
		return FilmAgentCloseoutResult{}, err
	}
	preview, err := s.evaluateFilmAgentCloseout(snapshot)
	if err != nil {
		return FilmAgentCloseoutResult{}, err
	}
	if request.ExpectedProjectRevision != preview.ProjectRevision || request.ExpectedRootRunRevision != preview.RootRunRevision || fingerprint != preview.EvidenceFingerprint {
		return FilmAgentCloseoutResult{}, conflictError("Film 收口证据已变化，请重新预检并确认")
	}
	if !preview.Ready {
		return FilmAgentCloseoutResult{}, conflictError("Film 项目尚未满足最终收口条件")
	}
	artifact, revision, err := buildFilmProjectSummaryArtifact(userID, snapshot, preview, note, time.Now().UTC())
	if err != nil {
		return FilmAgentCloseoutResult{}, err
	}
	qcRef := FilmProductionArtifactRef{}
	if preview.QualityReport != nil {
		qcRef = *preview.QualityReport
	}
	closed, err := s.repo.CloseAgentRuntimeRoot(repository.AgentRuntimeCloseoutCommand{
		UserID: userID, ProjectID: projectID, Domain: "film", RootRunID: root.ID,
		ExpectedProjectRevision: request.ExpectedProjectRevision, ExpectedRootRunRevision: request.ExpectedRootRunRevision,
		ExpectedEvidenceFingerprint: fingerprint, SummaryArtifact: &artifact, SummaryRevision: &revision,
		QualityHandoffEvent: repository.AgentRuntimeEventInput{
			ID: newID(), EventType: "handoff.hr09.accepted", ActorType: "user", ActorID: userID,
			PayloadJSON: mustFilmJSON(map[string]any{"handoffRouteId": "HR-09", "qualityReport": qcRef, "evidenceFingerprint": fingerprint}),
		},
		CompletionHandoffEvent: repository.AgentRuntimeEventInput{
			ID: newID(), EventType: "handoff.hr11.completed", ActorType: "user", ActorID: userID,
			PayloadJSON: mustFilmJSON(map[string]any{"handoffRouteId": "HR-11", "summaryArtifactId": artifact.ID, "summaryRevisionId": revision.ID, "evidenceFingerprint": fingerprint}),
		},
		At: time.Now().UTC(),
	})
	if err != nil {
		if existing, ok, replayErr := s.replayFilmAgentCloseout(userID, *project, *root, fingerprint); replayErr != nil || ok {
			return existing, replayErr
		}
		return FilmAgentCloseoutResult{}, mapAgentRuntimeRepositoryError(err)
	}
	return FilmAgentCloseoutResult{
		Project: closed.Project, RootRun: closed.RootRun, SummaryArtifact: closed.SummaryArtifact,
		SummaryRevision: closed.SummaryRevision,
	}, nil
}

func (s *Service) requireFilmAgentRootRun(userID string, projectID string, rootRunID string) (*model.AgentRuntimeRun, error) {
	run, err := s.repo.AgentRuntimeRunForUser(userID, strings.TrimSpace(rootRunID))
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && (run.ProjectID != projectID || run.Domain != "film")) {
		return nil, NotFound("Film Agent RootRun 不存在")
	}
	if err != nil {
		return nil, err
	}
	if run.RootRunID != "" && run.RootRunID != run.ID {
		return nil, BadAuthRequest("最终收口必须使用根 Intent Run")
	}
	return run, nil
}

func (s *Service) evaluateFilmAgentCloseout(snapshot repository.AgentRuntimeLineageSnapshot) (FilmAgentCloseoutPreview, error) {
	fingerprint, err := repository.AgentRuntimeLineageFingerprint(snapshot)
	if err != nil {
		return FilmAgentCloseoutPreview{}, err
	}
	preview := FilmAgentCloseoutPreview{
		ProjectID: snapshot.Project.ID, RootRunID: snapshot.RootRun.ID, ProjectRevision: snapshot.Project.Revision,
		RootRunRevision: snapshot.RootRun.Revision, EvidenceFingerprint: fingerprint,
		RequiredArtifactTypes: []string{}, Deliverables: []FilmProductionArtifactRef{}, Blockers: []FilmAgentCloseoutBlocker{},
	}
	expectedModes := map[string]string{"HR-09": "static", "HR-10": "project_start", "HR-11": "project_completion"}
	for _, routeID := range []string{"HR-09", "HR-10", "HR-11"} {
		route, ok := s.filmAgentRegistry.HandoffRoute(routeID)
		if !ok || route.ExecutionMode != "orchestration" || route.InputResolutionMode != expectedModes[routeID] {
			return FilmAgentCloseoutPreview{}, fmt.Errorf("Film AgentTeam %s 收口合同无效", routeID)
		}
	}
	if snapshot.Project.Status != model.ProjectStatusActive {
		preview.Blockers = append(preview.Blockers, FilmAgentCloseoutBlocker{Code: "project_not_active", Message: "项目不是可收口的 active 状态"})
	}
	lineageRunIDs := make(map[string]struct{}, len(snapshot.Runs))
	for _, run := range snapshot.Runs {
		lineageRunIDs[run.ID] = struct{}{}
	}
	for _, run := range snapshot.ProjectFilmRuns {
		if _, belongs := lineageRunIDs[run.ID]; !belongs {
			preview.Blockers = append(preview.Blockers, FilmAgentCloseoutBlocker{
				Code: "other_root_run", Message: "项目中存在不属于本次 RootRun 的 Film 运行链路，不能混合归档", RunID: run.ID,
			})
		}
	}
	for _, run := range snapshot.Runs {
		if run.Status != model.AgentRunStatusCompleted {
			preview.Blockers = append(preview.Blockers, FilmAgentCloseoutBlocker{
				Code: "lineage_run_incomplete", Message: "运行链路中仍有未完成的 Run", RunID: run.ID,
			})
		}
	}
	succeededByStep := make(map[string]bool)
	for _, attempt := range snapshot.Attempts {
		if attempt.Status == model.AgentAttemptStatusSucceeded {
			succeededByStep[attempt.StepID] = true
		}
	}
	for _, step := range snapshot.Steps {
		if step.Status != model.AgentStepStatusCompleted {
			preview.Blockers = append(preview.Blockers, FilmAgentCloseoutBlocker{
				Code: "lineage_step_incomplete", Message: "运行链路中仍有未完成的 Step", RunID: step.RunID, StepID: step.ID,
			})
		} else if !succeededByStep[step.ID] {
			preview.Blockers = append(preview.Blockers, FilmAgentCloseoutBlocker{
				Code: "successful_attempt_missing", Message: "已完成 Step 缺少成功 Attempt 证据", RunID: step.RunID, StepID: step.ID,
			})
		}
	}
	for _, decision := range snapshot.HumanDecisions {
		if decision.Status == model.AgentHumanDecisionStatusPending {
			preview.Blockers = append(preview.Blockers, FilmAgentCloseoutBlocker{
				Code: "pending_human_decision", Message: "仍有未处理的人工决定", RunID: decision.RunID, StepID: decision.StepID,
			})
		}
	}
	for _, trigger := range snapshot.HandoffTriggers {
		if trigger.Status != model.AgentHandoffTriggerStatusCompleted {
			preview.Blockers = append(preview.Blockers, FilmAgentCloseoutBlocker{
				Code: "handoff_unsettled", Message: "仍有未完成或失败的 Handoff Trigger", RunID: trigger.SourceRunID,
			})
		}
	}

	currentByType := make(map[string][]repository.ProductionArtifactRevisionFact)
	currentByStepAndType := make(map[string][]repository.ProductionArtifactRevisionFact)
	for _, fact := range snapshot.CurrentArtifacts {
		currentByType[fact.Artifact.ArtifactType] = append(currentByType[fact.Artifact.ArtifactType], fact)
		key := fact.Revision.SourceStepID + "\x00" + fact.Artifact.ArtifactType
		currentByStepAndType[key] = append(currentByStepAndType[key], fact)
	}
	requiredTypes := make(map[string]struct{})
	deliverableByRevision := make(map[string]FilmProductionArtifactRef)
	requireFact := func(artifactType string, fact repository.ProductionArtifactRevisionFact, found bool, runID string, stepID string) {
		requiredTypes[artifactType] = struct{}{}
		if !found || fact.Revision.Status != model.ProductionArtifactStatusLocked {
			preview.Blockers = append(preview.Blockers, FilmAgentCloseoutBlocker{
				Code: "required_artifact_unlocked", Message: "必需交付物不存在或尚未锁定", RunID: runID, StepID: stepID, ArtifactType: artifactType,
			})
			return
		}
		ref := filmArtifactFactRef(fact)
		deliverableByRevision[ref.RevisionID] = ref
	}
	for _, artifactType := range []string{"project-requirements", "task", "routing-decision"} {
		fact, found := newestLockedFilmArtifactFact(currentByType[artifactType], snapshot.RootRun.ID, "")
		requireFact(artifactType, fact, found, snapshot.RootRun.ID, "")
	}
	for _, step := range snapshot.Steps {
		var outputTypes []string
		if err := json.Unmarshal([]byte(step.ExpectedOutputArtifactTypesJSON), &outputTypes); err != nil {
			return FilmAgentCloseoutPreview{}, fmt.Errorf("解析 Film Step %s 输出合同：%w", step.ID, err)
		}
		for _, artifactType := range outputTypes {
			fact, found := newestLockedFilmArtifactFact(currentByStepAndType[step.ID+"\x00"+artifactType], "", "")
			requireFact(artifactType, fact, found, step.RunID, step.ID)
		}
	}
	qcCandidates := append(append([]repository.ProductionArtifactRevisionFact(nil), currentByType["qc-report"]...), currentByType["final-qc-report"]...)
	qcFact, qcFound := newestLockedFilmArtifactFact(qcCandidates, "", "quality_control_editor")
	if !qcFound {
		preview.Blockers = append(preview.Blockers, FilmAgentCloseoutBlocker{Code: "quality_report_missing", Message: "缺少质量审查 Agent 锁定的 QC 报告"})
	} else {
		qcRef := filmArtifactFactRef(qcFact)
		preview.QualityReport = &qcRef
		deliverableByRevision[qcRef.RevisionID] = qcRef
		requiredTypes[qcRef.Type] = struct{}{}
		hr09Settled := false
		for _, trigger := range snapshot.HandoffTriggers {
			if trigger.RevisionID == qcFact.Revision.ID && trigger.Status == model.AgentHandoffTriggerStatusCompleted {
				hr09Settled = true
				break
			}
		}
		if !hr09Settled {
			preview.Blockers = append(preview.Blockers, FilmAgentCloseoutBlocker{
				Code: "quality_handoff_missing", Message: "QC 报告尚未完成 HR-09 交接", RunID: qcFact.Revision.SourceRunID, ArtifactType: qcFact.Artifact.ArtifactType,
			})
		}
	}
	for artifactType := range requiredTypes {
		preview.RequiredArtifactTypes = append(preview.RequiredArtifactTypes, artifactType)
	}
	sort.Strings(preview.RequiredArtifactTypes)
	for _, ref := range deliverableByRevision {
		preview.Deliverables = append(preview.Deliverables, ref)
	}
	sort.Slice(preview.Deliverables, func(i, j int) bool {
		if preview.Deliverables[i].Type == preview.Deliverables[j].Type {
			return preview.Deliverables[i].RevisionID < preview.Deliverables[j].RevisionID
		}
		return preview.Deliverables[i].Type < preview.Deliverables[j].Type
	})
	preview.Ready = len(preview.Blockers) == 0
	return preview, nil
}

func buildFilmProjectSummaryArtifact(userID string, snapshot repository.AgentRuntimeLineageSnapshot, preview FilmAgentCloseoutPreview, note string, at time.Time) (model.ProductionArtifact, model.ProductionArtifactRevision, error) {
	type runEvidence struct {
		RunID          string `json:"runId"`
		RouteKind      string `json:"routeKind"`
		IntentRouteID  string `json:"intentRouteId,omitempty"`
		HandoffRouteID string `json:"handoffRouteId,omitempty"`
		ParentRunID    string `json:"parentRunId,omitempty"`
	}
	type routingEvidence struct {
		DecisionID     string   `json:"decisionId"`
		RunID          string   `json:"runId"`
		RouteID        string   `json:"routeId"`
		SelectedAgent  string   `json:"selectedAgentId"`
		SelectedSkills []string `json:"selectedSkillIds"`
	}
	runs := make([]runEvidence, 0, len(snapshot.Runs))
	for _, run := range snapshot.Runs {
		runs = append(runs, runEvidence{
			RunID: run.ID, RouteKind: run.RouteKind, IntentRouteID: run.IntentRouteID,
			HandoffRouteID: run.HandoffRouteID, ParentRunID: run.ParentRunID,
		})
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].RunID < runs[j].RunID })
	routing := make([]routingEvidence, 0, len(snapshot.RoutingDecisions))
	for _, decision := range snapshot.RoutingDecisions {
		var skills []string
		if err := json.Unmarshal([]byte(decision.SelectedSkillIDsJSON), &skills); err != nil {
			return model.ProductionArtifact{}, model.ProductionArtifactRevision{}, err
		}
		routing = append(routing, routingEvidence{
			DecisionID: decision.ID, RunID: decision.RunID, RouteID: decision.RouteID,
			SelectedAgent: decision.SelectedAgentID, SelectedSkills: skills,
		})
	}
	sort.Slice(routing, func(i, j int) bool { return routing[i].DecisionID < routing[j].DecisionID })
	content, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "artifactType": "project-summary", "rootRunId": snapshot.RootRun.ID,
		"project":             map[string]any{"id": snapshot.Project.ID, "name": snapshot.Project.Name, "type": snapshot.Project.Type, "aspectRatio": snapshot.Project.AspectRatio},
		"objective":           snapshot.RootRun.Objective,
		"registry":            map[string]any{"id": snapshot.RootRun.RegistryID, "version": snapshot.RootRun.RegistryVersion, "digest": snapshot.RootRun.RegistryDigest},
		"orchestrationRoutes": []string{"HR-10", "HR-09", "HR-11"},
		"evidenceFingerprint": preview.EvidenceFingerprint, "runs": runs, "routingDecisions": routing,
		"deliverables": preview.Deliverables, "qualityReport": preview.QualityReport,
		"confirmation": map[string]any{"confirmedByUserId": userID, "confirmedAt": at.Format(time.RFC3339Nano), "note": note},
	})
	if err != nil {
		return model.ProductionArtifact{}, model.ProductionArtifactRevision{}, err
	}
	artifactID := newID()
	revisionID := newID()
	artifact := model.ProductionArtifact{
		ID: artifactID, UserID: userID, ProjectID: snapshot.Project.ID, Domain: "film", ArtifactType: "project-summary",
		LogicalKey: filmProjectSummaryLogicalKey(snapshot.RootRun.ID), CurrentRevisionID: revisionID, RevisionSequence: 1,
		CreatedAt: at, UpdatedAt: at,
	}
	authorityRefs := []FilmProductionArtifactRef{}
	if preview.QualityReport != nil {
		authorityRefs = append(authorityRefs, *preview.QualityReport)
	}
	revision := model.ProductionArtifactRevision{
		ID: revisionID, ArtifactID: artifactID, Version: 1, Status: model.ProductionArtifactStatusLocked,
		ContentJSON: string(content), ContentText: fmt.Sprintf("项目《%s》已由用户确认收口，共 %d 项锁定交付物。", snapshot.Project.Name, len(preview.Deliverables)),
		ContentDigest: digestBytesHex(content), SourceRunID: snapshot.RootRun.ID,
		SourceArtifactRefsJSON: mustFilmJSON(preview.Deliverables), AuthorityRefsJSON: mustFilmJSON(authorityRefs),
		CreatedByType: "user", CreatedByID: userID, CreatedAt: at,
	}
	return artifact, revision, nil
}

func (s *Service) existingFilmAgentCloseout(userID string, projectID string, rootRunID string) (*model.ProductionArtifact, *model.ProductionArtifactRevision, string, bool, error) {
	artifact, err := s.repo.ProductionArtifactByLogicalKey(userID, projectID, "film", filmProjectSummaryLogicalKey(rootRunID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, "", false, nil
	}
	if err != nil {
		return nil, nil, "", false, err
	}
	loadedArtifact, revision, err := s.repo.ProductionArtifactRevisionForUser(userID, artifact.CurrentRevisionID)
	if err != nil {
		return nil, nil, "", false, err
	}
	var content struct {
		RootRunID           string `json:"rootRunId"`
		EvidenceFingerprint string `json:"evidenceFingerprint"`
	}
	if revision.Status != model.ProductionArtifactStatusLocked || loadedArtifact.ID != artifact.ID || json.Unmarshal([]byte(revision.ContentJSON), &content) != nil ||
		content.RootRunID != rootRunID || !filmEvidenceFingerprintPattern.MatchString(content.EvidenceFingerprint) {
		return nil, nil, "", false, errors.New("Film project-summary 收口证据损坏")
	}
	return artifact, revision, content.EvidenceFingerprint, true, nil
}

func (s *Service) replayFilmAgentCloseout(userID string, project model.Project, root model.AgentRuntimeRun, fingerprint string) (FilmAgentCloseoutResult, bool, error) {
	artifact, revision, storedFingerprint, found, err := s.existingFilmAgentCloseout(userID, project.ID, root.ID)
	if err != nil || !found {
		return FilmAgentCloseoutResult{}, false, err
	}
	if storedFingerprint != fingerprint {
		return FilmAgentCloseoutResult{}, false, conflictError("该 RootRun 已使用另一份证据完成收口")
	}
	currentProject, err := s.repo.ProjectForUser(userID, project.ID)
	if err != nil {
		return FilmAgentCloseoutResult{}, false, err
	}
	currentRoot, err := s.repo.AgentRuntimeRunForUser(userID, root.ID)
	if err != nil {
		return FilmAgentCloseoutResult{}, false, err
	}
	if currentProject.Status != model.ProjectStatusArchived {
		return FilmAgentCloseoutResult{}, false, errors.New("Film 收口 Artifact 与项目归档状态不一致")
	}
	return FilmAgentCloseoutResult{
		Project: *currentProject, RootRun: *currentRoot, SummaryArtifact: *artifact,
		SummaryRevision: *revision, Idempotent: true,
	}, true, nil
}

func filmProjectSummaryLogicalKey(rootRunID string) string {
	return "root:" + rootRunID + ":project-summary"
}

func (s *Service) validateFilmCloseoutRegistry(root model.AgentRuntimeRun) error {
	if root.RegistryID != s.filmAgentRegistry.ID || root.RegistryVersion != s.filmAgentRegistry.Version || root.RegistryDigest != s.filmAgentRegistry.SourceDigest {
		return conflictError("RootRun 使用的 Film AgentTeam 版本与当前运行时不一致，不能直接收口")
	}
	return nil
}

func filmArtifactFactRef(fact repository.ProductionArtifactRevisionFact) FilmProductionArtifactRef {
	return FilmProductionArtifactRef{
		ArtifactID: fact.Artifact.ID, RevisionID: fact.Revision.ID, Type: fact.Artifact.ArtifactType,
		Version: fact.Revision.Version, Digest: fact.Revision.ContentDigest, Status: string(fact.Revision.Status),
	}
}

func newestLockedFilmArtifactFact(facts []repository.ProductionArtifactRevisionFact, sourceRunID string, sourceAgentID string) (repository.ProductionArtifactRevisionFact, bool) {
	candidates := make([]repository.ProductionArtifactRevisionFact, 0, len(facts))
	for _, fact := range facts {
		if fact.Revision.Status != model.ProductionArtifactStatusLocked || (sourceRunID != "" && fact.Revision.SourceRunID != sourceRunID) ||
			(sourceAgentID != "" && fact.SourceAgentID != sourceAgentID) {
			continue
		}
		candidates = append(candidates, fact)
	}
	if len(candidates) == 0 {
		return repository.ProductionArtifactRevisionFact{}, false
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Revision.CreatedAt.Equal(candidates[j].Revision.CreatedAt) {
			return candidates[i].Revision.ID > candidates[j].Revision.ID
		}
		return candidates[i].Revision.CreatedAt.After(candidates[j].Revision.CreatedAt)
	})
	return candidates[0], true
}
