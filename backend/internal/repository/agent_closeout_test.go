package repository

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

func TestCloseAgentRuntimeRootCommitsSummaryEventsAndArchiveTogether(t *testing.T) {
	db := openAgentRuntimeTestDB(t, filepath.Join(t.TempDir(), "agent-closeout.db"))
	repo := New(db)
	bundle, project := createAgentCloseoutTestRoot(t, repo, db, "closeout-root")
	snapshot, err := repo.AgentRuntimeLineageSnapshotForRoot(project.UserID, project.ID, "film", bundle.Run.ID)
	if err != nil {
		t.Fatalf("load closeout snapshot: %v", err)
	}
	fingerprint, err := AgentRuntimeLineageFingerprint(snapshot)
	if err != nil {
		t.Fatalf("fingerprint closeout snapshot: %v", err)
	}
	artifact, revision := agentCloseoutTestSummary(*bundle.Run, fingerprint, time.Now().UTC())
	result, err := repo.CloseAgentRuntimeRoot(AgentRuntimeCloseoutCommand{
		UserID: project.UserID, ProjectID: project.ID, Domain: "film", RootRunID: bundle.Run.ID,
		ExpectedProjectRevision: project.Revision, ExpectedRootRunRevision: bundle.Run.Revision,
		ExpectedEvidenceFingerprint: fingerprint, SummaryArtifact: &artifact, SummaryRevision: &revision,
		QualityHandoffEvent:    AgentRuntimeEventInput{ID: "closeout-hr09", EventType: "handoff.hr09.accepted", ActorType: "user", ActorID: project.UserID},
		CompletionHandoffEvent: AgentRuntimeEventInput{ID: "closeout-hr11", EventType: "handoff.hr11.completed", ActorType: "user", ActorID: project.UserID},
	})
	if err != nil {
		t.Fatalf("CloseAgentRuntimeRoot(): %v", err)
	}
	if result.Project.Status != model.ProjectStatusArchived || result.Project.Revision != project.Revision+1 ||
		result.RootRun.Revision != bundle.Run.Revision+1 || result.RootRun.EventSequence != bundle.Run.EventSequence+2 ||
		result.SummaryRevision.Status != model.ProductionArtifactStatusLocked {
		t.Fatalf("closeout transaction result is incomplete: %#v", result)
	}
	detail, err := repo.AgentRuntimeDetailForUser(project.UserID, bundle.Run.ID)
	if err != nil {
		t.Fatalf("load closed RootRun: %v", err)
	}
	if len(detail.Events) != 3 || detail.Events[1].EventType != "handoff.hr09.accepted" || detail.Events[2].EventType != "handoff.hr11.completed" {
		t.Fatalf("closeout Event sequence is invalid: %#v", detail.Events)
	}
}

func TestCloseAgentRuntimeRootRejectsStaleFingerprintWithoutPartialWrites(t *testing.T) {
	db := openAgentRuntimeTestDB(t, filepath.Join(t.TempDir(), "agent-closeout-stale.db"))
	repo := New(db)
	bundle, project := createAgentCloseoutTestRoot(t, repo, db, "closeout-stale")
	artifact, revision := agentCloseoutTestSummary(*bundle.Run, strings.Repeat("0", 64), time.Now().UTC())
	_, err := repo.CloseAgentRuntimeRoot(AgentRuntimeCloseoutCommand{
		UserID: project.UserID, ProjectID: project.ID, Domain: "film", RootRunID: bundle.Run.ID,
		ExpectedProjectRevision: project.Revision, ExpectedRootRunRevision: bundle.Run.Revision,
		ExpectedEvidenceFingerprint: strings.Repeat("0", 64), SummaryArtifact: &artifact, SummaryRevision: &revision,
		QualityHandoffEvent:    AgentRuntimeEventInput{ID: "stale-hr09", EventType: "handoff.hr09.accepted", ActorType: "user", ActorID: project.UserID},
		CompletionHandoffEvent: AgentRuntimeEventInput{ID: "stale-hr11", EventType: "handoff.hr11.completed", ActorType: "user", ActorID: project.UserID},
	})
	if !errors.Is(err, ErrAgentRuntimeStateConflict) {
		t.Fatalf("stale closeout error = %v", err)
	}
	assertAgentCloseoutRolledBack(t, repo, project, *bundle.Run, artifact.LogicalKey)
}

func TestAgentRuntimeLineageFingerprintCoversRegistryIdentity(t *testing.T) {
	db := openAgentRuntimeTestDB(t, filepath.Join(t.TempDir(), "agent-closeout-registry-fingerprint.db"))
	repo := New(db)
	bundle, project := createAgentCloseoutTestRoot(t, repo, db, "closeout-registry-fingerprint")
	snapshot, err := repo.AgentRuntimeLineageSnapshotForRoot(project.UserID, project.ID, "film", bundle.Run.ID)
	if err != nil {
		t.Fatalf("load registry fingerprint snapshot: %v", err)
	}
	original, err := AgentRuntimeLineageFingerprint(snapshot)
	if err != nil {
		t.Fatalf("fingerprint original snapshot: %v", err)
	}
	mutations := map[string]func(*model.AgentRuntimeRun){
		"registry id":      func(run *model.AgentRuntimeRun) { run.RegistryID = "other-agent-team" },
		"registry version": func(run *model.AgentRuntimeRun) { run.RegistryVersion = "9.9.9" },
		"registry digest":  func(run *model.AgentRuntimeRun) { run.RegistryDigest = strings.Repeat("f", 64) },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changed := snapshot
			changed.Runs = append([]model.AgentRuntimeRun(nil), snapshot.Runs...)
			mutate(&changed.RootRun)
			mutate(&changed.Runs[0])
			fingerprint, err := AgentRuntimeLineageFingerprint(changed)
			if err != nil {
				t.Fatalf("fingerprint changed snapshot: %v", err)
			}
			if fingerprint == original {
				t.Fatalf("%s did not change closeout fingerprint", name)
			}
		})
	}
}

func TestCloseAgentRuntimeRootRollsBackWhenSummaryIdentityConflicts(t *testing.T) {
	db := openAgentRuntimeTestDB(t, filepath.Join(t.TempDir(), "agent-closeout-summary-conflict.db"))
	repo := New(db)
	bundle, project := createAgentCloseoutTestRoot(t, repo, db, "closeout-conflict")
	now := time.Now().UTC()
	existingArtifact, existingRevision := agentCloseoutTestSummary(*bundle.Run, strings.Repeat("a", 64), now)
	existingArtifact.ID = "existing-summary-artifact"
	existingRevision.ID = "existing-summary-revision"
	if _, _, err := repo.CreateProductionArtifactRevision(ProductionArtifactRevisionCreate{
		UserID: project.UserID, Artifact: &existingArtifact, Revision: &existingRevision, ExpectedSequence: 0, At: now,
	}); err != nil {
		t.Fatalf("create conflicting summary identity: %v", err)
	}
	snapshot, err := repo.AgentRuntimeLineageSnapshotForRoot(project.UserID, project.ID, "film", bundle.Run.ID)
	if err != nil {
		t.Fatalf("load conflict snapshot: %v", err)
	}
	fingerprint, err := AgentRuntimeLineageFingerprint(snapshot)
	if err != nil {
		t.Fatalf("fingerprint conflict snapshot: %v", err)
	}
	artifact, revision := agentCloseoutTestSummary(*bundle.Run, fingerprint, now.Add(time.Second))
	artifact.ID = "duplicate-logical-summary-artifact"
	revision.ID = "duplicate-logical-summary-revision"
	_, err = repo.CloseAgentRuntimeRoot(AgentRuntimeCloseoutCommand{
		UserID: project.UserID, ProjectID: project.ID, Domain: "film", RootRunID: bundle.Run.ID,
		ExpectedProjectRevision: project.Revision, ExpectedRootRunRevision: bundle.Run.Revision,
		ExpectedEvidenceFingerprint: fingerprint, SummaryArtifact: &artifact, SummaryRevision: &revision,
		QualityHandoffEvent:    AgentRuntimeEventInput{ID: "conflict-hr09", EventType: "handoff.hr09.accepted", ActorType: "user", ActorID: project.UserID},
		CompletionHandoffEvent: AgentRuntimeEventInput{ID: "conflict-hr11", EventType: "handoff.hr11.completed", ActorType: "user", ActorID: project.UserID},
	})
	if err == nil {
		t.Fatal("duplicate project-summary logical identity unexpectedly closed project")
	}
	currentProject, projectErr := repo.ProjectForUser(project.UserID, project.ID)
	currentRoot, rootErr := repo.AgentRuntimeRunForUser(project.UserID, bundle.Run.ID)
	if projectErr != nil || rootErr != nil || currentProject.Status != model.ProjectStatusActive || currentProject.Revision != project.Revision ||
		currentRoot.Revision != bundle.Run.Revision || currentRoot.EventSequence != bundle.Run.EventSequence {
		t.Fatalf("summary conflict left partial closeout: project=%#v root=%#v errors=%v/%v", currentProject, currentRoot, projectErr, rootErr)
	}
}

func TestCreateAgentRuntimeBundleRejectsArchivedProjectInsideTransaction(t *testing.T) {
	db := openAgentRuntimeTestDB(t, filepath.Join(t.TempDir(), "agent-archived-bundle.db"))
	repo := New(db)
	bundle := agentRuntimeTestBundle("archived-project-run", "archived-project-request", time.Now().UTC())
	createAgentRuntimeTestProject(t, db, bundle.Run.ProjectID, bundle.Run.UserID)
	if err := db.Model(&model.Project{}).Where("id = ?", bundle.Run.ProjectID).Update("status", model.ProjectStatusArchived).Error; err != nil {
		t.Fatalf("archive test Project: %v", err)
	}
	if err := repo.CreateAgentRuntimeBundle(bundle); !errors.Is(err, ErrAgentRuntimeProjectArchived) {
		t.Fatalf("create Run in archived Project error = %v", err)
	}
	if _, err := repo.AgentRuntimeRunForUser(bundle.Run.UserID, bundle.Run.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("archived Project left a Run behind: %v", err)
	}
}

func createAgentCloseoutTestRoot(t *testing.T, repo *Repository, db *gorm.DB, runID string) (AgentRuntimeCreateBundle, model.Project) {
	t.Helper()
	now := time.Now().UTC()
	bundle := agentRuntimeTestBundle(runID, runID+"-request", now)
	bundle.Run.RootRunID = bundle.Run.ID
	bundle.Run.Status = model.AgentRunStatusCompleted
	bundle.Run.CompletedAt = &now
	bundle.Steps[0].Status = model.AgentStepStatusCompleted
	bundle.Steps[0].AttemptSequence = 1
	bundle.Steps[0].CompletedAt = &now
	createAgentRuntimeTestProject(t, db, bundle.Run.ProjectID, bundle.Run.UserID)
	if err := repo.CreateAgentRuntimeBundle(bundle); err != nil {
		t.Fatalf("create closeout RootRun: %v", err)
	}
	attempt := model.AgentRuntimeAttempt{
		ID: runID + "-attempt", RunID: runID, StepID: bundle.Steps[0].ID, Number: 1,
		Status: model.AgentAttemptStatusSucceeded, Executor: "test", InputDigest: "input", PromptDigest: "prompt",
		RequestJSON: "{}", ResponseJSON: "{}", Revision: 1, StartedAt: &now, CompletedAt: &now, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&attempt).Error; err != nil {
		t.Fatalf("create closeout Attempt: %v", err)
	}
	project, err := repo.ProjectForUser(bundle.Run.UserID, bundle.Run.ProjectID)
	if err != nil {
		t.Fatalf("load closeout Project: %v", err)
	}
	return bundle, *project
}

func agentCloseoutTestSummary(root model.AgentRuntimeRun, fingerprint string, at time.Time) (model.ProductionArtifact, model.ProductionArtifactRevision) {
	artifact := model.ProductionArtifact{
		ID: root.ID + "-summary-artifact", UserID: root.UserID, ProjectID: root.ProjectID, Domain: root.Domain,
		ArtifactType: "project-summary", LogicalKey: "root:" + root.ID + ":project-summary",
	}
	revision := model.ProductionArtifactRevision{
		ID: root.ID + "-summary-revision", Status: model.ProductionArtifactStatusLocked,
		ContentJSON:   `{"schemaVersion":1,"artifactType":"project-summary","evidenceFingerprint":"` + fingerprint + `"}`,
		ContentDigest: "summary-" + fingerprint, SourceRunID: root.ID, SourceArtifactRefsJSON: "[]", AuthorityRefsJSON: "[]",
		CreatedByType: "user", CreatedByID: root.UserID, CreatedAt: at,
	}
	return artifact, revision
}

func assertAgentCloseoutRolledBack(t *testing.T, repo *Repository, project model.Project, root model.AgentRuntimeRun, logicalKey string) {
	t.Helper()
	currentProject, projectErr := repo.ProjectForUser(project.UserID, project.ID)
	currentRoot, rootErr := repo.AgentRuntimeRunForUser(project.UserID, root.ID)
	if projectErr != nil || rootErr != nil || currentProject.Status != model.ProjectStatusActive || currentProject.Revision != project.Revision ||
		currentRoot.Revision != root.Revision || currentRoot.EventSequence != root.EventSequence {
		t.Fatalf("failed closeout left partial state: project=%#v root=%#v errors=%v/%v", currentProject, currentRoot, projectErr, rootErr)
	}
	if _, err := repo.ProductionArtifactByLogicalKey(project.UserID, project.ID, root.Domain, logicalKey); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("failed closeout persisted a summary: %v", err)
	}
}
