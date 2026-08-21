package service

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/gorm"
)

func TestFilmAgentCloseoutRequiresCompleteEvidenceAndArchivesAtomically(t *testing.T) {
	svc, repo, db, project := newFilmAgentRuntimeTestService(t)
	created, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-closeout-root-0001", CreateFilmAgentRunRequest{
		Objective: "写一个可交付的原创短片故事", IntentRouteID: "IR-01",
	})
	if err != nil {
		t.Fatalf("CreateFilmAgentRun(): %v", err)
	}
	incomplete, err := svc.PreviewFilmAgentCloseout(project.UserID, project.ID, created.Detail.Run.ID)
	if err != nil {
		t.Fatalf("preview incomplete closeout: %v", err)
	}
	if incomplete.Ready || len(incomplete.Blockers) == 0 {
		t.Fatalf("unfinished root was considered ready: %#v", incomplete)
	}

	prepareReadyFilmCloseoutFixture(t, repo, db, created.Detail)
	preview, err := svc.PreviewFilmAgentCloseout(project.UserID, project.ID, created.Detail.Run.ID)
	if err != nil {
		t.Fatalf("PreviewFilmAgentCloseout(): %v", err)
	}
	if !preview.Ready || preview.Completed || len(preview.Blockers) != 0 || len(preview.EvidenceFingerprint) != 64 || preview.QualityReport == nil {
		t.Fatalf("ready closeout preview is incomplete: %#v", preview)
	}
	for _, required := range []string{"project-requirements", "task", "routing-decision", "script", "qc-report"} {
		if !filmContainsString(preview.RequiredArtifactTypes, required) {
			t.Fatalf("closeout preview missed required Artifact %s: %#v", required, preview.RequiredArtifactTypes)
		}
	}
	if _, err := svc.ConfirmFilmAgentCloseout(project.UserID, project.ID, created.Detail.Run.ID, ConfirmFilmAgentCloseoutRequest{
		ExpectedProjectRevision: preview.ProjectRevision, ExpectedRootRunRevision: preview.RootRunRevision,
		EvidenceFingerprint: preview.EvidenceFingerprint,
	}); authStatus(err) != 400 {
		t.Fatalf("implicit closeout confirmation error = %v, want 400", err)
	}

	closed, err := svc.ConfirmFilmAgentCloseout(project.UserID, project.ID, created.Detail.Run.ID, ConfirmFilmAgentCloseoutRequest{
		ExpectedProjectRevision: preview.ProjectRevision, ExpectedRootRunRevision: preview.RootRunRevision,
		EvidenceFingerprint: preview.EvidenceFingerprint, Confirm: true, Note: "最终交付已核验",
	})
	if err != nil {
		t.Fatalf("ConfirmFilmAgentCloseout(): %v", err)
	}
	if closed.Idempotent || closed.Project.Status != model.ProjectStatusArchived || closed.Project.Revision != preview.ProjectRevision+1 ||
		closed.RootRun.Revision != preview.RootRunRevision+1 || closed.SummaryRevision.Status != model.ProductionArtifactStatusLocked ||
		closed.SummaryRevision.SourceRunID != created.Detail.Run.ID || closed.SummaryArtifact.ArtifactType != "project-summary" {
		t.Fatalf("atomic closeout result is incomplete: %#v", closed)
	}
	var summaryContent struct {
		RootRunID           string                      `json:"rootRunId"`
		EvidenceFingerprint string                      `json:"evidenceFingerprint"`
		OrchestrationRoutes []string                    `json:"orchestrationRoutes"`
		Deliverables        []FilmProductionArtifactRef `json:"deliverables"`
	}
	if err := json.Unmarshal([]byte(closed.SummaryRevision.ContentJSON), &summaryContent); err != nil {
		t.Fatalf("decode project-summary: %v", err)
	}
	if summaryContent.RootRunID != created.Detail.Run.ID || summaryContent.EvidenceFingerprint != preview.EvidenceFingerprint ||
		len(summaryContent.Deliverables) != len(preview.Deliverables) ||
		!filmContainsString(summaryContent.OrchestrationRoutes, "HR-09") || !filmContainsString(summaryContent.OrchestrationRoutes, "HR-11") {
		t.Fatalf("project-summary lost immutable route evidence: %#v", summaryContent)
	}
	detail, err := repo.AgentRuntimeDetailForUser(project.UserID, created.Detail.Run.ID)
	if err != nil {
		t.Fatalf("load closed RootRun: %v", err)
	}
	if len(detail.Events) < 2 || detail.Events[len(detail.Events)-2].EventType != "handoff.hr09.accepted" ||
		detail.Events[len(detail.Events)-1].EventType != "handoff.hr11.completed" {
		t.Fatalf("closeout route events are incomplete: %#v", detail.Events)
	}

	replayed, err := svc.ConfirmFilmAgentCloseout(project.UserID, project.ID, created.Detail.Run.ID, ConfirmFilmAgentCloseoutRequest{
		ExpectedProjectRevision: preview.ProjectRevision, ExpectedRootRunRevision: preview.RootRunRevision,
		EvidenceFingerprint: preview.EvidenceFingerprint, Confirm: true,
	})
	if err != nil {
		t.Fatalf("replay Film closeout: %v", err)
	}
	if !replayed.Idempotent || replayed.SummaryRevision.ID != closed.SummaryRevision.ID || replayed.Project.Status != model.ProjectStatusArchived {
		t.Fatalf("closeout replay created different evidence: %#v", replayed)
	}
	completedPreview, err := svc.PreviewFilmAgentCloseout(project.UserID, project.ID, created.Detail.Run.ID)
	if err != nil || !completedPreview.Completed || completedPreview.SummaryRevision == nil || completedPreview.SummaryRevision.ID != closed.SummaryRevision.ID {
		t.Fatalf("archived closeout preview = %#v, error = %v", completedPreview, err)
	}
	if _, err := svc.FilmAgentRunDetail(project.UserID, project.ID, created.Detail.Run.ID); err != nil {
		t.Fatalf("closed Film history must stay readable: %v", err)
	}
	if _, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-closeout-after-archive", CreateFilmAgentRunRequest{
		Objective: "归档后不应创建新运行", IntentRouteID: "IR-01",
	}); authStatus(err) != 409 {
		t.Fatalf("create after closeout error = %v, want 409", err)
	}
	if _, err := svc.LockFilmAgentArtifact(project.UserID, project.ID, created.Detail.Run.ID, closed.SummaryArtifact.ID, LockFilmAgentArtifactRequest{}); authStatus(err) != 409 {
		t.Fatalf("lock after closeout error = %v, want 409", err)
	}
	afterWrites, err := svc.PreviewFilmAgentCloseout(project.UserID, project.ID, created.Detail.Run.ID)
	if err != nil || afterWrites.SummaryRevision == nil || afterWrites.SummaryRevision.ID != closed.SummaryRevision.ID {
		t.Fatalf("rejected archived writes changed closeout evidence: preview=%#v error=%v", afterWrites, err)
	}
}

func TestFilmAgentCloseoutBlocksPendingHumanDecision(t *testing.T) {
	svc, repo, _, project := newFilmAgentRuntimeTestService(t)
	created, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-closeout-pending-human", CreateFilmAgentRunRequest{
		Objective: "写一个待用户确认的原创短片故事", IntentRouteID: "IR-01", ReviewBeforeExecution: true,
	})
	if err != nil {
		t.Fatalf("CreateFilmAgentRun(): %v", err)
	}
	preview, err := svc.PreviewFilmAgentCloseout(project.UserID, project.ID, created.Detail.Run.ID)
	if err != nil {
		t.Fatalf("PreviewFilmAgentCloseout(): %v", err)
	}
	decision := created.Detail.HumanDecisions[0]
	if preview.Ready || !filmCloseoutHasBlocker(preview, "pending_human_decision", created.Detail.Run.ID) {
		t.Fatalf("pending HumanDecision did not block closeout: %#v", preview)
	}
	if _, err := svc.ConfirmFilmAgentCloseout(project.UserID, project.ID, created.Detail.Run.ID, ConfirmFilmAgentCloseoutRequest{
		ExpectedProjectRevision: preview.ProjectRevision, ExpectedRootRunRevision: preview.RootRunRevision,
		EvidenceFingerprint: preview.EvidenceFingerprint, Confirm: true,
	}); authStatus(err) != 409 {
		t.Fatalf("confirm with pending HumanDecision %s error = %v, want 409", decision.ID, err)
	}
	currentProject, err := repo.ProjectForUser(project.UserID, project.ID)
	if err != nil || currentProject.Status != model.ProjectStatusActive || currentProject.Revision != preview.ProjectRevision {
		t.Fatalf("blocked HumanDecision closeout changed project: project=%#v error=%v", currentProject, err)
	}
	if _, err := repo.ProductionArtifactByLogicalKey(project.UserID, project.ID, "film", filmProjectSummaryLogicalKey(created.Detail.Run.ID)); !errorsIsRecordNotFound(err) {
		t.Fatalf("blocked HumanDecision closeout persisted a summary: %v", err)
	}
}

func TestFilmAgentCloseoutDoesNotExposeAnotherUsersProject(t *testing.T) {
	svc, _, _, project := newFilmAgentRuntimeTestService(t)
	created, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-closeout-owner", CreateFilmAgentRunRequest{
		Objective: "写一个原创短片故事", IntentRouteID: "IR-01",
	})
	if err != nil {
		t.Fatalf("CreateFilmAgentRun(): %v", err)
	}
	ownerPreview, err := svc.PreviewFilmAgentCloseout(project.UserID, project.ID, created.Detail.Run.ID)
	if err != nil {
		t.Fatalf("owner closeout preview: %v", err)
	}
	foreignUserID := "user-film-other"
	if _, err := svc.PreviewFilmAgentCloseout(foreignUserID, project.ID, created.Detail.Run.ID); authStatus(err) != 404 {
		t.Fatalf("foreign closeout preview error = %v, want 404", err)
	}
	if _, err := svc.ConfirmFilmAgentCloseout(foreignUserID, project.ID, created.Detail.Run.ID, ConfirmFilmAgentCloseoutRequest{
		ExpectedProjectRevision: ownerPreview.ProjectRevision, ExpectedRootRunRevision: ownerPreview.RootRunRevision,
		EvidenceFingerprint: ownerPreview.EvidenceFingerprint, Confirm: true,
	}); authStatus(err) != 404 {
		t.Fatalf("foreign closeout confirmation error = %v, want 404", err)
	}
}

func TestFilmAgentCloseoutRejectsRegistryDrift(t *testing.T) {
	svc, repo, db, project := newFilmAgentRuntimeTestService(t)
	created, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-closeout-registry-drift", CreateFilmAgentRunRequest{
		Objective: "写一个原创短片故事", IntentRouteID: "IR-01",
	})
	if err != nil {
		t.Fatalf("CreateFilmAgentRun(): %v", err)
	}
	if err := db.Model(&model.AgentRuntimeRun{}).Where("id = ?", created.Detail.Run.ID).Updates(map[string]any{
		"registry_version": "9.9.9", "revision": gorm.Expr("revision + 1"),
	}).Error; err != nil {
		t.Fatalf("drift RootRun Registry version: %v", err)
	}
	if _, err := svc.PreviewFilmAgentCloseout(project.UserID, project.ID, created.Detail.Run.ID); authStatus(err) != 409 {
		t.Fatalf("Registry drift closeout preview error = %v, want 409", err)
	}
	currentProject, err := repo.ProjectForUser(project.UserID, project.ID)
	if err != nil || currentProject.Status != model.ProjectStatusActive || currentProject.Revision != project.Revision {
		t.Fatalf("Registry drift preview changed project: project=%#v error=%v", currentProject, err)
	}
	if _, err := repo.ProductionArtifactByLogicalKey(project.UserID, project.ID, "film", filmProjectSummaryLogicalKey(created.Detail.Run.ID)); !errorsIsRecordNotFound(err) {
		t.Fatalf("Registry drift preview persisted a summary: %v", err)
	}
}

func TestFilmAgentCloseoutRejectsStaleEvidenceWithoutPartialArchive(t *testing.T) {
	svc, repo, db, project := newFilmAgentRuntimeTestService(t)
	created, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-closeout-stale-0001", CreateFilmAgentRunRequest{
		Objective: "写一个可交付的原创短片故事", IntentRouteID: "IR-01",
	})
	if err != nil {
		t.Fatalf("CreateFilmAgentRun(): %v", err)
	}
	triggerID := prepareReadyFilmCloseoutFixture(t, repo, db, created.Detail)
	preview, err := svc.PreviewFilmAgentCloseout(project.UserID, project.ID, created.Detail.Run.ID)
	if err != nil || !preview.Ready {
		t.Fatalf("ready preview = %#v, error = %v", preview, err)
	}
	if err := db.Model(&model.AgentHandoffTrigger{}).Where("id = ?", triggerID).
		Update("revision", gorm.Expr("revision + 1")).Error; err != nil {
		t.Fatalf("advance Handoff Trigger evidence: %v", err)
	}
	if _, err := svc.ConfirmFilmAgentCloseout(project.UserID, project.ID, created.Detail.Run.ID, ConfirmFilmAgentCloseoutRequest{
		ExpectedProjectRevision: preview.ProjectRevision, ExpectedRootRunRevision: preview.RootRunRevision,
		EvidenceFingerprint: preview.EvidenceFingerprint, Confirm: true,
	}); authStatus(err) != 409 {
		t.Fatalf("stale closeout error = %v, want 409", err)
	}
	currentProject, err := repo.ProjectForUser(project.UserID, project.ID)
	if err != nil || currentProject.Status != model.ProjectStatusActive || currentProject.Revision != preview.ProjectRevision {
		t.Fatalf("stale closeout partially archived project: project=%#v error=%v", currentProject, err)
	}
	if _, err := repo.ProductionArtifactByLogicalKey(project.UserID, project.ID, "film", filmProjectSummaryLogicalKey(created.Detail.Run.ID)); !errorsIsRecordNotFound(err) {
		t.Fatalf("stale closeout left a project-summary behind: %v", err)
	}
}

func TestFilmAgentCloseoutNeverMixesAnotherRootRun(t *testing.T) {
	svc, repo, db, project := newFilmAgentRuntimeTestService(t)
	first, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-closeout-first-0001", CreateFilmAgentRunRequest{
		Objective: "写一个可交付的原创短片故事", IntentRouteID: "IR-01",
	})
	if err != nil {
		t.Fatalf("create first RootRun: %v", err)
	}
	prepareReadyFilmCloseoutFixture(t, repo, db, first.Detail)
	second, err := svc.CreateFilmAgentRun(project.UserID, project.ID, "film-closeout-second-0001", CreateFilmAgentRunRequest{
		Objective: "写另一个原创短片故事", IntentRouteID: "IR-01",
	})
	if err != nil {
		t.Fatalf("create second RootRun: %v", err)
	}
	preview, err := svc.PreviewFilmAgentCloseout(project.UserID, project.ID, first.Detail.Run.ID)
	if err != nil {
		t.Fatalf("preview first RootRun: %v", err)
	}
	if preview.Ready || !filmCloseoutHasBlocker(preview, "other_root_run", second.Detail.Run.ID) {
		t.Fatalf("another RootRun was mixed into closeout: %#v", preview)
	}
}

func prepareReadyFilmCloseoutFixture(t *testing.T, repo *repository.Repository, db *gorm.DB, rootDetail repository.AgentRuntimeDetail) string {
	t.Helper()
	now := time.Now().UTC()
	root := rootDetail.Run
	rootStep := rootDetail.Steps[0]
	rootAttempt := model.AgentRuntimeAttempt{
		ID: root.ID + "-closeout-attempt", RunID: root.ID, StepID: rootStep.ID, Number: 1,
		Status: model.AgentAttemptStatusSucceeded, Executor: "closeout-fixture", InputDigest: "root-input", PromptDigest: "root-prompt",
		RequestJSON: "{}", ResponseJSON: "{}", Revision: 1, StartedAt: &now, CompletedAt: &now, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&rootAttempt).Error; err != nil {
		t.Fatalf("create root closeout Attempt: %v", err)
	}
	if err := db.Model(&model.AgentRuntimeStep{}).Where("id = ?", rootStep.ID).Updates(map[string]any{
		"status": model.AgentStepStatusCompleted, "attempt_sequence": 1, "completed_at": now,
	}).Error; err != nil {
		t.Fatalf("complete root closeout Step: %v", err)
	}
	if err := db.Model(&model.AgentRuntimeRun{}).Where("id = ?", root.ID).Updates(map[string]any{
		"status": model.AgentRunStatusCompleted, "completed_at": now,
	}).Error; err != nil {
		t.Fatalf("complete root closeout Run: %v", err)
	}
	createFilmCloseoutArtifact(t, repo, root, rootStep, rootAttempt, "script", now.Add(time.Second))

	qcRunID := root.ID + "-qc-run"
	qcStepID := root.ID + "-qc-step"
	qcCompletedAt := now.Add(2 * time.Second)
	qcRun := model.AgentRuntimeRun{
		ID: qcRunID, UserID: root.UserID, ProjectID: root.ProjectID, CanvasID: root.CanvasID, Domain: "film",
		RegistryID: root.RegistryID, RegistryVersion: root.RegistryVersion, RegistryDigest: root.RegistryDigest,
		RouteKind: "handoff", HandoffRouteID: "HR-08", RootRunID: root.ID, ParentRunID: root.ID,
		Status: model.AgentRunStatusCompleted, Objective: "最终质量审查", InputJSON: `{"requestDigest":"closeout-qc"}`,
		CurrentStepID: qcStepID, IdempotencyKey: "film-closeout-qc:" + root.ID, Revision: 1, EventSequence: 1,
		StartedAt: &now, CompletedAt: &qcCompletedAt, CreatedAt: now, UpdatedAt: qcCompletedAt,
	}
	qcStep := model.AgentRuntimeStep{
		ID: qcStepID, RunID: qcRunID, StepKey: "handoff:HR-08", Position: 0, RouteKind: "handoff", RouteID: "HR-08",
		AgentID: "quality_control_editor", SkillIDsJSON: `["continuity-check"]`, Status: model.AgentStepStatusCompleted,
		DependsOnStepIDsJSON: "[]", InputArtifactRefsJSON: "[]", ExpectedOutputArtifactTypesJSON: `["qc-report"]`,
		OutputArtifactRefsJSON: "[]", AttemptSequence: 1, Revision: 1, StartedAt: &now, CompletedAt: &qcCompletedAt,
		CreatedAt: now, UpdatedAt: qcCompletedAt,
	}
	qcBundle := repository.AgentRuntimeCreateBundle{
		Run: &qcRun,
		RoutingDecision: &model.AgentRoutingDecision{
			ID: root.ID + "-qc-routing", RunID: qcRunID, RouteKind: "handoff", RouteID: "HR-08",
			SelectedAgentID: qcStep.AgentID, SelectedSkillIDsJSON: qcStep.SkillIDsJSON, InputArtifactRefsJSON: "[]", AlternativesJSON: "[]",
			Reason: "测试质量收口链路", Confidence: "confirmed", DecidedByType: "runtime", DecidedByID: "closeout-fixture", CreatedAt: now,
		},
		Steps: []model.AgentRuntimeStep{qcStep},
		Events: []model.AgentRuntimeEvent{{
			ID: root.ID + "-qc-event", UserID: root.UserID, RunID: qcRunID, Sequence: 1,
			EventType: "run.completed", ActorType: "runtime", ActorID: "closeout-fixture", ToStatus: string(model.AgentRunStatusCompleted),
			PayloadJSON: "{}", CreatedAt: qcCompletedAt,
		}},
	}
	if err := repo.CreateAgentRuntimeBundle(qcBundle); err != nil {
		t.Fatalf("create QC closeout Run: %v", err)
	}
	qcAttempt := model.AgentRuntimeAttempt{
		ID: root.ID + "-qc-attempt", RunID: qcRunID, StepID: qcStepID, Number: 1,
		Status: model.AgentAttemptStatusSucceeded, Executor: "closeout-fixture", InputDigest: "qc-input", PromptDigest: "qc-prompt",
		RequestJSON: "{}", ResponseJSON: "{}", Revision: 1, StartedAt: &now, CompletedAt: &qcCompletedAt, CreatedAt: now, UpdatedAt: qcCompletedAt,
	}
	if err := db.Create(&qcAttempt).Error; err != nil {
		t.Fatalf("create QC closeout Attempt: %v", err)
	}
	_, qcRevision := createFilmCloseoutArtifact(t, repo, qcRun, qcStep, qcAttempt, "qc-report", qcCompletedAt)
	triggerID := root.ID + "-qc-trigger"
	trigger := model.AgentHandoffTrigger{
		ID: triggerID, UserID: root.UserID, ProjectID: root.ProjectID, Domain: "film", RootRunID: root.ID,
		ArtifactID: qcRevision.ArtifactID, RevisionID: qcRevision.ID, SourceRunID: qcRunID, SourceStepID: qcStepID,
		SourceAgentID: qcStep.AgentID, Status: model.AgentHandoffTriggerStatusCompleted, AttemptCount: 1,
		ScheduledRunIDsJSON: "[]", Revision: 1, CompletedAt: &qcCompletedAt, CreatedAt: qcCompletedAt, UpdatedAt: qcCompletedAt,
	}
	if err := db.Create(&trigger).Error; err != nil {
		t.Fatalf("create completed QC Handoff Trigger: %v", err)
	}
	return triggerID
}

func createFilmCloseoutArtifact(t *testing.T, repo *repository.Repository, run model.AgentRuntimeRun, step model.AgentRuntimeStep, attempt model.AgentRuntimeAttempt, artifactType string, at time.Time) (model.ProductionArtifact, model.ProductionArtifactRevision) {
	t.Helper()
	artifact := &model.ProductionArtifact{
		ID: run.ID + "-closeout-artifact-" + artifactType, UserID: run.UserID, ProjectID: run.ProjectID, Domain: "film",
		ArtifactType: artifactType, LogicalKey: "run:" + run.ID + ":" + artifactType,
	}
	revision := &model.ProductionArtifactRevision{
		ID: run.ID + "-closeout-revision-" + artifactType, Status: model.ProductionArtifactStatusLocked,
		ContentJSON: `{"schemaVersion":1,"artifactType":"` + artifactType + `"}`, ContentDigest: digestString(run.ID + artifactType),
		SourceRunID: run.ID, SourceStepID: step.ID, SourceAttemptID: attempt.ID, SourceArtifactRefsJSON: "[]", AuthorityRefsJSON: "[]",
		CreatedByType: "agent", CreatedByID: step.AgentID,
	}
	persistedArtifact, persistedRevision, err := repo.CreateProductionArtifactRevision(repository.ProductionArtifactRevisionCreate{
		UserID: run.UserID, Artifact: artifact, Revision: revision, ExpectedSequence: 0, At: at,
	})
	if err != nil {
		t.Fatalf("create closeout %s Artifact: %v", artifactType, err)
	}
	return *persistedArtifact, *persistedRevision
}

func filmCloseoutHasBlocker(preview FilmAgentCloseoutPreview, code string, runID string) bool {
	for _, blocker := range preview.Blockers {
		if blocker.Code == code && (runID == "" || blocker.RunID == runID) {
			return true
		}
	}
	return false
}

func errorsIsRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
