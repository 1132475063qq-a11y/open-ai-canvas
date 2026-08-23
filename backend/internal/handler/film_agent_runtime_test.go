package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
	"infinite-canvas/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const handlerFilmAgentLogicalModelID = "logical-film-agent-handler"

func TestFilmAgentRuntimeHTTPContract(t *testing.T) {
	router, cookie, project, repo, db := newFilmAgentRuntimeTestRouter(t)

	unauthorized := filmAgentRuntimeRequest(t, router, http.MethodGet, "/api/projects/"+project.ID+"/film/agent-runtime/catalog", "", "", "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized catalog status = %d, body = %s", unauthorized.Code, unauthorized.Body.String())
	}

	catalogResponse := filmAgentRuntimeRequest(t, router, http.MethodGet, "/api/projects/"+project.ID+"/film/agent-runtime/catalog", "", cookie, "")
	if catalogResponse.Code != http.StatusOK {
		t.Fatalf("catalog status = %d, body = %s", catalogResponse.Code, catalogResponse.Body.String())
	}
	var catalogEnvelope struct {
		Code int                             `json:"code"`
		Data service.FilmAgentRuntimeCatalog `json:"data"`
	}
	decodeFilmAgentRuntimeResponse(t, catalogResponse, &catalogEnvelope)
	if catalogEnvelope.Code != 0 || catalogEnvelope.Data.Registry.AgentCount != 9 || catalogEnvelope.Data.Registry.SkillCount != 17 ||
		len(catalogEnvelope.Data.IntentRoutes) != 15 || len(catalogEnvelope.Data.HandoffRoutes) != 11 {
		t.Fatalf("catalog response is incomplete: %#v", catalogEnvelope)
	}
	encodedCatalog := catalogResponse.Body.String()
	if strings.Contains(encodedCatalog, "developerInstructions") || strings.Contains(encodedCatalog, "instructions\"") {
		t.Fatal("catalog leaked executable Agent or Skill instructions")
	}

	missingKey := filmAgentRuntimeRequest(t, router, http.MethodPost, "/api/projects/"+project.ID+"/film/agent-runs", `{"objective":"写一个原创短片故事","intentRouteId":"IR-01"}`, cookie, "")
	if missingKey.Code != http.StatusBadRequest {
		t.Fatalf("missing idempotency status = %d, body = %s", missingKey.Code, missingKey.Body.String())
	}
	missingModel := filmAgentRuntimeRequest(t, router, http.MethodPost, "/api/projects/"+project.ID+"/film/agent-runs", `{"objective":"写一个原创短片故事","intentRouteId":"IR-01"}`, cookie, "film-http-missing-model")
	if missingModel.Code != http.StatusBadRequest {
		t.Fatalf("missing logical text model status = %d, body = %s", missingModel.Code, missingModel.Body.String())
	}

	createBody := fmt.Sprintf(`{"objective":"写一个原创短片故事","intentRouteId":"IR-01","logicalModelId":%q,"reviewBeforeExecution":true}`, handlerFilmAgentLogicalModelID)
	createResponse := filmAgentRuntimeRequest(t, router, http.MethodPost, "/api/projects/"+project.ID+"/film/agent-runs", createBody, cookie, "film-http-create-0001")
	if createResponse.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", createResponse.Code, createResponse.Body.String())
	}
	var createEnvelope struct {
		Code int                              `json:"code"`
		Data service.FilmAgentRunCreateResult `json:"data"`
	}
	decodeFilmAgentRuntimeResponse(t, createResponse, &createEnvelope)
	created := createEnvelope.Data.Detail
	if created.Run.ID == "" || created.Run.Status != model.AgentRunStatusAwaitingHuman || len(created.Steps) != 1 || len(created.HumanDecisions) != 1 {
		t.Fatalf("created HTTP Run is incomplete: %#v", created)
	}

	replayResponse := filmAgentRuntimeRequest(t, router, http.MethodPost, "/api/projects/"+project.ID+"/film/agent-runs", createBody, cookie, "film-http-create-0001")
	if replayResponse.Code != http.StatusOK {
		t.Fatalf("idempotent replay status = %d, body = %s", replayResponse.Code, replayResponse.Body.String())
	}
	var replayEnvelope struct {
		Data service.FilmAgentRunCreateResult `json:"data"`
	}
	decodeFilmAgentRuntimeResponse(t, replayResponse, &replayEnvelope)
	if !replayEnvelope.Data.Idempotent || replayEnvelope.Data.Detail.Run.ID != created.Run.ID {
		t.Fatalf("HTTP replay was not idempotent: %#v", replayEnvelope)
	}

	listResponse := filmAgentRuntimeRequest(t, router, http.MethodGet, "/api/projects/"+project.ID+"/film/agent-runs?limit=10", "", cookie, "")
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), created.Run.ID) {
		t.Fatalf("list status = %d, body = %s", listResponse.Code, listResponse.Body.String())
	}
	badLimit := filmAgentRuntimeRequest(t, router, http.MethodGet, "/api/projects/"+project.ID+"/film/agent-runs?limit=101", "", cookie, "")
	if badLimit.Code != http.StatusBadRequest {
		t.Fatalf("bad limit status = %d, body = %s", badLimit.Code, badLimit.Body.String())
	}
	detailResponse := filmAgentRuntimeRequest(t, router, http.MethodGet, "/api/projects/"+project.ID+"/film/agent-runs/"+created.Run.ID, "", cookie, "")
	if detailResponse.Code != http.StatusOK || !strings.Contains(detailResponse.Body.String(), created.Run.RegistryDigest) {
		t.Fatalf("detail status = %d, body = %s", detailResponse.Code, detailResponse.Body.String())
	}

	decision := created.HumanDecisions[0]
	resolveBody := fmt.Sprintf(`{"expectedRunRevision":%d,"expectedStepRevision":%d,"expectedDecisionRevision":%d,"action":"approve","response":{"note":"通过"}}`, created.Run.Revision, created.Steps[0].Revision, decision.Revision)
	resolveResponse := filmAgentRuntimeRequest(t, router, http.MethodPost, "/api/projects/"+project.ID+"/film/agent-runs/"+created.Run.ID+"/decisions/"+decision.ID+"/resolve", resolveBody, cookie, "")
	if resolveResponse.Code != http.StatusOK {
		t.Fatalf("resolve status = %d, body = %s", resolveResponse.Code, resolveResponse.Body.String())
	}
	var resolveEnvelope struct {
		Data repository.AgentRuntimeDetail `json:"data"`
	}
	decodeFilmAgentRuntimeResponse(t, resolveResponse, &resolveEnvelope)
	if resolveEnvelope.Data.Run.Status != model.AgentRunStatusReady || resolveEnvelope.Data.HumanDecisions[0].Status != model.AgentHumanDecisionStatusResolved {
		t.Fatalf("resolved HTTP Run is incoherent: %#v", resolveEnvelope.Data)
	}
	lockArtifact, lockRevision := createFilmAgentHandlerLockFixture(t, repo, db, resolveEnvelope.Data)
	missingLockEvidence := filmAgentRuntimeRequest(t, router, http.MethodPost,
		"/api/projects/"+project.ID+"/film/agent-runs/"+created.Run.ID+"/artifacts/"+lockArtifact.ID+"/lock",
		`{"expectedRunRevision":1}`, cookie, "")
	if missingLockEvidence.Code != http.StatusBadRequest {
		t.Fatalf("missing lock evidence status = %d, body = %s", missingLockEvidence.Code, missingLockEvidence.Body.String())
	}
	lockBody := fmt.Sprintf(`{"expectedRunRevision":%d,"expectedArtifactSequence":%d,"expectedRevisionId":%q}`,
		resolveEnvelope.Data.Run.Revision, lockArtifact.RevisionSequence, lockRevision.ID)
	lockResponse := filmAgentRuntimeRequest(t, router, http.MethodPost,
		"/api/projects/"+project.ID+"/film/agent-runs/"+created.Run.ID+"/artifacts/"+lockArtifact.ID+"/lock",
		lockBody, cookie, "")
	if lockResponse.Code != http.StatusOK {
		t.Fatalf("lock Artifact status = %d, body = %s", lockResponse.Code, lockResponse.Body.String())
	}
	var lockEnvelope struct {
		Data repository.ProductionArtifactLockResult `json:"data"`
	}
	decodeFilmAgentRuntimeResponse(t, lockResponse, &lockEnvelope)
	if lockEnvelope.Data.SourceRevision.ID != lockRevision.ID || lockEnvelope.Data.SourceRevision.Status != model.ProductionArtifactStatusReview ||
		lockEnvelope.Data.LockedRevision.Status != model.ProductionArtifactStatusLocked ||
		lockEnvelope.Data.LockedRevision.ParentRevisionID != lockRevision.ID || lockEnvelope.Data.Trigger.Status != model.AgentHandoffTriggerStatusPending {
		t.Fatalf("HTTP lock response is incomplete: %#v", lockEnvelope.Data)
	}
	staleLock := filmAgentRuntimeRequest(t, router, http.MethodPost,
		"/api/projects/"+project.ID+"/film/agent-runs/"+created.Run.ID+"/artifacts/"+lockArtifact.ID+"/lock",
		lockBody, cookie, "")
	if staleLock.Code != http.StatusConflict {
		t.Fatalf("stale lock status = %d, body = %s", staleLock.Code, staleLock.Body.String())
	}

	missingRollbackKey := filmAgentRuntimeRequest(t, router, http.MethodPost,
		"/api/projects/"+project.ID+"/film/agent-runs/"+created.Run.ID+"/artifacts/"+lockArtifact.ID+"/rollback",
		fmt.Sprintf(`{"expectedRunRevision":%d,"expectedArtifactSequence":%d,"expectedCurrentRevisionId":%q,"targetRevisionId":%q}`,
			lockEnvelope.Data.Run.Revision, lockEnvelope.Data.Artifact.RevisionSequence, lockEnvelope.Data.Artifact.CurrentRevisionID, lockRevision.ID), cookie, "")
	if missingRollbackKey.Code != http.StatusBadRequest {
		t.Fatalf("missing rollback idempotency status = %d, body = %s", missingRollbackKey.Code, missingRollbackKey.Body.String())
	}
	rollbackBody := fmt.Sprintf(`{"expectedRunRevision":%d,"expectedArtifactSequence":%d,"expectedCurrentRevisionId":%q,"targetRevisionId":%q}`,
		lockEnvelope.Data.Run.Revision, lockEnvelope.Data.Artifact.RevisionSequence, lockEnvelope.Data.Artifact.CurrentRevisionID, lockRevision.ID)
	rollbackResponse := filmAgentRuntimeRequest(t, router, http.MethodPost,
		"/api/projects/"+project.ID+"/film/agent-runs/"+created.Run.ID+"/artifacts/"+lockArtifact.ID+"/rollback",
		rollbackBody, cookie, "film-http-rollback-0001")
	if rollbackResponse.Code != http.StatusOK {
		t.Fatalf("rollback Artifact status = %d, body = %s", rollbackResponse.Code, rollbackResponse.Body.String())
	}
	var rollbackEnvelope struct {
		Data repository.ProductionArtifactRollbackResult `json:"data"`
	}
	decodeFilmAgentRuntimeResponse(t, rollbackResponse, &rollbackEnvelope)
	if rollbackEnvelope.Data.Idempotent || rollbackEnvelope.Data.TargetRevision.ID != lockRevision.ID ||
		rollbackEnvelope.Data.RollbackRevision.ParentRevisionID != lockEnvelope.Data.Artifact.CurrentRevisionID ||
		rollbackEnvelope.Data.RollbackRevision.ContentDigest != lockRevision.ContentDigest ||
		rollbackEnvelope.Data.Artifact.CurrentRevisionID != rollbackEnvelope.Data.RollbackRevision.ID {
		t.Fatalf("rollback response is incomplete: %#v", rollbackEnvelope.Data)
	}
	replayRollback := filmAgentRuntimeRequest(t, router, http.MethodPost,
		"/api/projects/"+project.ID+"/film/agent-runs/"+created.Run.ID+"/artifacts/"+lockArtifact.ID+"/rollback",
		rollbackBody, cookie, "film-http-rollback-0001")
	if replayRollback.Code != http.StatusOK {
		t.Fatalf("rollback replay status = %d, body = %s", replayRollback.Code, replayRollback.Body.String())
	}
	var replayRollbackEnvelope struct {
		Data repository.ProductionArtifactRollbackResult `json:"data"`
	}
	decodeFilmAgentRuntimeResponse(t, replayRollback, &replayRollbackEnvelope)
	if !replayRollbackEnvelope.Data.Idempotent || replayRollbackEnvelope.Data.RollbackRevision.ID != rollbackEnvelope.Data.RollbackRevision.ID ||
		replayRollbackEnvelope.Data.Run.Revision != rollbackEnvelope.Data.Run.Revision {
		t.Fatalf("rollback replay was not idempotent: %#v", replayRollbackEnvelope.Data)
	}

	oversizedBody := `{"objective":"` + strings.Repeat("x", filmAgentRunRequestLimit) + `","intentRouteId":"IR-01"}`
	oversized := filmAgentRuntimeRequest(t, router, http.MethodPost, "/api/projects/"+project.ID+"/film/agent-runs", oversizedBody, cookie, "film-http-large-0001")
	if oversized.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized status = %d, body = %s", oversized.Code, oversized.Body.String())
	}
}

func TestFilmAgentCloseoutHTTPContract(t *testing.T) {
	router, cookie, project, repo, db := newFilmAgentRuntimeTestRouter(t)
	createResponse := filmAgentRuntimeRequest(t, router, http.MethodPost, "/api/projects/"+project.ID+"/film/agent-runs",
		`{"objective":"检查连续性","intentRouteId":"IR-12"}`, cookie, "film-http-closeout-0001")
	if createResponse.Code != http.StatusOK {
		t.Fatalf("create closeout RootRun status = %d, body = %s", createResponse.Code, createResponse.Body.String())
	}
	var createEnvelope struct {
		Data service.FilmAgentRunCreateResult `json:"data"`
	}
	decodeFilmAgentRuntimeResponse(t, createResponse, &createEnvelope)
	detail := createEnvelope.Data.Detail
	prepareFilmAgentHandlerCloseoutFixture(t, repo, db, detail)

	path := "/api/projects/" + project.ID + "/film/agent-runs/" + detail.Run.ID + "/closeout"
	unauthorized := filmAgentRuntimeRequest(t, router, http.MethodGet, path, "", "", "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized closeout status = %d, body = %s", unauthorized.Code, unauthorized.Body.String())
	}
	previewResponse := filmAgentRuntimeRequest(t, router, http.MethodGet, path, "", cookie, "")
	if previewResponse.Code != http.StatusOK {
		t.Fatalf("closeout preview status = %d, body = %s", previewResponse.Code, previewResponse.Body.String())
	}
	var previewEnvelope struct {
		Data service.FilmAgentCloseoutPreview `json:"data"`
	}
	decodeFilmAgentRuntimeResponse(t, previewResponse, &previewEnvelope)
	preview := previewEnvelope.Data
	if !preview.Ready || preview.Completed || len(preview.EvidenceFingerprint) != 64 || preview.QualityReport == nil {
		t.Fatalf("HTTP closeout preview is incomplete: %#v", preview)
	}
	missingConfirmationBody := fmt.Sprintf(`{"expectedProjectRevision":%d,"expectedRootRunRevision":%d,"evidenceFingerprint":%q}`,
		preview.ProjectRevision, preview.RootRunRevision, preview.EvidenceFingerprint)
	missingConfirmation := filmAgentRuntimeRequest(t, router, http.MethodPost, path, missingConfirmationBody, cookie, "")
	if missingConfirmation.Code != http.StatusBadRequest {
		t.Fatalf("implicit closeout status = %d, body = %s", missingConfirmation.Code, missingConfirmation.Body.String())
	}
	confirmBody := fmt.Sprintf(`{"expectedProjectRevision":%d,"expectedRootRunRevision":%d,"evidenceFingerprint":%q,"confirm":true,"note":"验收通过"}`,
		preview.ProjectRevision, preview.RootRunRevision, preview.EvidenceFingerprint)
	confirmResponse := filmAgentRuntimeRequest(t, router, http.MethodPost, path, confirmBody, cookie, "")
	if confirmResponse.Code != http.StatusOK {
		t.Fatalf("confirm closeout status = %d, body = %s", confirmResponse.Code, confirmResponse.Body.String())
	}
	var confirmEnvelope struct {
		Data service.FilmAgentCloseoutResult `json:"data"`
	}
	decodeFilmAgentRuntimeResponse(t, confirmResponse, &confirmEnvelope)
	if confirmEnvelope.Data.Idempotent || confirmEnvelope.Data.Project.Status != model.ProjectStatusArchived ||
		confirmEnvelope.Data.SummaryRevision.Status != model.ProductionArtifactStatusLocked {
		t.Fatalf("HTTP closeout result is incomplete: %#v", confirmEnvelope.Data)
	}
	replayResponse := filmAgentRuntimeRequest(t, router, http.MethodPost, path, confirmBody, cookie, "")
	if replayResponse.Code != http.StatusOK {
		t.Fatalf("replay closeout status = %d, body = %s", replayResponse.Code, replayResponse.Body.String())
	}
	var replayEnvelope struct {
		Data service.FilmAgentCloseoutResult `json:"data"`
	}
	decodeFilmAgentRuntimeResponse(t, replayResponse, &replayEnvelope)
	if !replayEnvelope.Data.Idempotent || replayEnvelope.Data.SummaryRevision.ID != confirmEnvelope.Data.SummaryRevision.ID {
		t.Fatalf("HTTP closeout replay created different evidence: %#v", replayEnvelope.Data)
	}
}

func TestFilmProductionHTTPContract(t *testing.T) {
	router, cookie, project, _, _ := newFilmAgentRuntimeTestRouter(t)
	basePath := "/api/projects/" + project.ID + "/film/production"

	unauthorized := filmAgentRuntimeRequest(t, router, http.MethodGet, basePath+"/attempts", "", "", "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized Film Production list status = %d, body = %s", unauthorized.Code, unauthorized.Body.String())
	}
	list := filmAgentRuntimeRequest(t, router, http.MethodGet, basePath+"/attempts?limit=10", "", cookie, "")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"attempts":[]`) {
		t.Fatalf("Film Production list status = %d, body = %s", list.Code, list.Body.String())
	}
	badLimit := filmAgentRuntimeRequest(t, router, http.MethodGet, basePath+"/attempts?limit=101", "", cookie, "")
	if badLimit.Code != http.StatusBadRequest {
		t.Fatalf("Film Production bad limit status = %d, body = %s", badLimit.Code, badLimit.Body.String())
	}

	missingQuoteKey := filmAgentRuntimeRequest(t, router, http.MethodPost, basePath+"/image-quotes", `{}`, cookie, "")
	if missingQuoteKey.Code != http.StatusBadRequest {
		t.Fatalf("Film Production missing quote key status = %d, body = %s", missingQuoteKey.Code, missingQuoteKey.Body.String())
	}
	missingQuote := filmAgentRuntimeRequest(t, router, http.MethodPost, basePath+"/image-quotes/missing/submit", `{"quoteFingerprint":"missing"}`, cookie, "film-http-submit-0001")
	if missingQuote.Code != http.StatusNotFound {
		t.Fatalf("Film Production missing quote status = %d, body = %s", missingQuote.Code, missingQuote.Body.String())
	}
	missingAttempt := filmAgentRuntimeRequest(t, router, http.MethodPost, basePath+"/attempts/missing/qc", `{"decision":"PASS","action":"accept"}`, cookie, "film-http-qc-0001")
	if missingAttempt.Code != http.StatusNotFound {
		t.Fatalf("Film Production missing Attempt status = %d, body = %s", missingAttempt.Code, missingAttempt.Body.String())
	}
	unauthorizedVisualQC := filmAgentRuntimeRequest(t, router, http.MethodPost, basePath+"/visual-qc-quotes", `{}`, "", "film-http-visual-qc-0001")
	if unauthorizedVisualQC.Code != http.StatusUnauthorized {
		t.Fatalf("Film visual QC unauthorized status = %d, body = %s", unauthorizedVisualQC.Code, unauthorizedVisualQC.Body.String())
	}
	missingVisualQCKey := filmAgentRuntimeRequest(t, router, http.MethodPost, basePath+"/visual-qc-quotes", `{}`, cookie, "")
	if missingVisualQCKey.Code != http.StatusBadRequest {
		t.Fatalf("Film visual QC missing key status = %d, body = %s", missingVisualQCKey.Code, missingVisualQCKey.Body.String())
	}
	missingVisualQCQuote := filmAgentRuntimeRequest(t, router, http.MethodPost, basePath+"/visual-qc-quotes/missing/submit", `{"quoteFingerprint":"missing"}`, cookie, "film-http-visual-qc-submit-0001")
	if missingVisualQCQuote.Code != http.StatusNotFound {
		t.Fatalf("Film visual QC missing quote status = %d, body = %s", missingVisualQCQuote.Code, missingVisualQCQuote.Body.String())
	}
	unauthorizedVideoVisualQC := filmAgentRuntimeRequest(t, router, http.MethodPost, basePath+"/video-visual-qc-quotes", `{}`, "", "film-http-video-visual-qc-0001")
	if unauthorizedVideoVisualQC.Code != http.StatusUnauthorized {
		t.Fatalf("Film video visual QC unauthorized status = %d, body = %s", unauthorizedVideoVisualQC.Code, unauthorizedVideoVisualQC.Body.String())
	}
	missingVideoVisualQCKey := filmAgentRuntimeRequest(t, router, http.MethodPost, basePath+"/video-visual-qc-quotes", `{}`, cookie, "")
	if missingVideoVisualQCKey.Code != http.StatusBadRequest {
		t.Fatalf("Film video visual QC missing key status = %d, body = %s", missingVideoVisualQCKey.Code, missingVideoVisualQCKey.Body.String())
	}
	missingVideoVisualQCQuote := filmAgentRuntimeRequest(t, router, http.MethodPost, basePath+"/video-visual-qc-quotes/missing/submit", `{"quoteFingerprint":"missing"}`, cookie, "film-http-video-visual-qc-submit-0001")
	if missingVideoVisualQCQuote.Code != http.StatusNotFound {
		t.Fatalf("Film video visual QC missing quote status = %d, body = %s", missingVideoVisualQCQuote.Code, missingVideoVisualQCQuote.Body.String())
	}
	unauthorizedSequenceVisualQC := filmAgentRuntimeRequest(t, router, http.MethodPost, basePath+"/video-sequence-visual-qc-quotes", `{}`, "", "film-http-sequence-visual-qc-0001")
	if unauthorizedSequenceVisualQC.Code != http.StatusUnauthorized {
		t.Fatalf("Film sequence visual QC unauthorized status = %d, body = %s", unauthorizedSequenceVisualQC.Code, unauthorizedSequenceVisualQC.Body.String())
	}
	missingSequenceVisualQCKey := filmAgentRuntimeRequest(t, router, http.MethodPost, basePath+"/video-sequence-visual-qc-quotes", `{}`, cookie, "")
	if missingSequenceVisualQCKey.Code != http.StatusBadRequest {
		t.Fatalf("Film sequence visual QC missing key status = %d, body = %s", missingSequenceVisualQCKey.Code, missingSequenceVisualQCKey.Body.String())
	}
	missingSequenceVisualQCQuote := filmAgentRuntimeRequest(t, router, http.MethodPost, basePath+"/video-sequence-visual-qc-quotes/missing/submit", `{"quoteFingerprint":"missing"}`, cookie, "film-http-sequence-visual-qc-submit-0001")
	if missingSequenceVisualQCQuote.Code != http.StatusNotFound {
		t.Fatalf("Film sequence visual QC missing quote status = %d, body = %s", missingSequenceVisualQCQuote.Code, missingSequenceVisualQCQuote.Body.String())
	}

	oversizedBody := `{"rootRunId":"` + strings.Repeat("x", filmProductionQuoteRequestLimit) + `"}`
	oversized := filmAgentRuntimeRequest(t, router, http.MethodPost, basePath+"/image-quotes", oversizedBody, cookie, "film-http-large-0001")
	if oversized.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("Film Production oversized status = %d, body = %s", oversized.Code, oversized.Body.String())
	}
}

func newFilmAgentRuntimeTestRouter(t *testing.T) (*gin.Engine, string, model.Project, *repository.Repository, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "film-agent-handler.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open handler database: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.AuthSession{}, &model.Project{}, &model.CanvasProject{},
		&model.ModelChannel{}, &model.ChannelModel{},
		&model.LogicalModel{}, &model.LogicalModelRevision{}, &model.LogicalModelRoute{},
		&model.AgentRuntimeRun{}, &model.AgentRuntimeStep{}, &model.AgentRuntimeAttempt{},
		&model.AgentRoutingDecision{}, &model.AgentHumanDecision{}, &model.AgentRuntimeEvent{},
		&model.AgentHandoffTrigger{},
		&model.ProductionArtifact{}, &model.ProductionArtifactRevision{},
		&model.FilmProductionQuote{}, &model.FilmProductionAttempt{}, &model.FilmProductionQCReport{},
		&model.FilmVisualQCQuote{}, &model.FilmVisualQCAttempt{},
		&model.FilmVideoSequence{}, &model.FilmVideoSlot{}, &model.FilmVideoQuote{}, &model.FilmVideoAttempt{}, &model.FilmVideoQCReport{},
		&model.FilmVideoVisualQCQuote{}, &model.FilmVideoVisualQCAttempt{},
		&model.FilmVideoSequenceVisualQCQuote{}, &model.FilmVideoSequenceVisualQCAttempt{}, &model.FilmVideoSequenceReview{},
		&model.FilmContinuityLedger{}, &model.FilmContinuityShotState{}, &model.FilmContinuityIssue{}, &model.FilmReworkEvent{},
		&model.Task{}, &model.Result{}, &model.BillingOrder{},
	); err != nil {
		t.Fatalf("migrate handler database: %v", err)
	}
	now := time.Now().UTC()
	seedFilmAgentHandlerTextModel(t, db, now)
	repo := repository.New(db)
	svc := service.New(repo, t.TempDir())
	if err := svc.ValidateRuntime(); err != nil {
		t.Fatalf("ValidateRuntime(): %v", err)
	}
	user := model.User{
		ID: "handler-user", Username: "film-handler", DisplayName: "Film Handler",
		Role: model.UserRoleUser, Status: model.UserStatusActive, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.Create(&user); err != nil {
		t.Fatalf("create handler user: %v", err)
	}
	const token = "handler-session-token"
	tokenDigest := sha256.Sum256([]byte(token))
	session := model.AuthSession{
		ID: "handler-session", UserID: user.ID, TokenHash: hex.EncodeToString(tokenDigest[:]),
		ExpiresAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.Create(&session); err != nil {
		t.Fatalf("create handler session: %v", err)
	}
	project := model.Project{
		ID: "handler-project", UserID: user.ID, Name: "Handler Film", Type: "short-drama", AspectRatio: "9:16",
		SourceType: "blank", Status: model.ProjectStatusActive, Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.CreateProject(&project); err != nil {
		t.Fatalf("create handler project: %v", err)
	}
	router := gin.New()
	api := router.Group("/api")
	RegisterFilmAgentRuntimeRoutes(api, svc)
	RegisterFilmProductionRoutes(api, svc)
	return router, service.SessionCookieName + "=" + session.ID + "." + token, project, repo, db
}

func seedFilmAgentHandlerTextModel(t *testing.T, db *gorm.DB, now time.Time) {
	t.Helper()
	capabilityConfig := &service.ModelCapabilityConfig{Version: 1, Text: &service.TextCapabilityConfig{References: service.TextReferenceConfig{PromptMaxChars: 2_000_000}}}
	capabilityConfigJSON, err := json.Marshal(capabilityConfig)
	if err != nil {
		t.Fatalf("encode handler Film text capability: %v", err)
	}
	capabilitySpec, err := service.CapabilitySpecFromModelCapabilityConfig(capabilityConfig, "text")
	if err != nil {
		t.Fatalf("project handler Film text capability: %v", err)
	}
	capabilitySpecJSON, err := json.Marshal(capabilitySpec)
	if err != nil {
		t.Fatalf("encode handler Film logical capability: %v", err)
	}
	channel := model.ModelChannel{
		ID: "channel-film-agent-handler", UserID: "admin", Scope: model.ChannelScopeSystem, Enabled: true,
		Name: "Film Handler Test Provider", BaseURL: "https://film-handler.test.invalid", APIFormat: "openai",
		ConcurrencyLimit: 2, ModelsJSON: `["film-handler-text"]`, CreatedAt: now, UpdatedAt: now,
	}
	channelModel := model.ChannelModel{
		ID: "channel-model-film-agent-handler", ChannelID: channel.ID, ModelKey: "film-handler-text", DisplayName: "Film Handler Text",
		Capability: "text", Protocol: model.ChannelInterfaceChatCompletion, BillingMode: "fixed_request",
		UnitPriceMicrocredits: 1, PriceConfigured: true, Enabled: true, PriceVersion: 1,
		CapabilityConfigJSON: string(capabilityConfigJSON), CapabilityVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	revision := model.LogicalModelRevision{
		ID: "logical-revision-film-agent-handler", LogicalModelID: handlerFilmAgentLogicalModelID, Version: 1,
		CapabilitySpecJSON: string(capabilitySpecJSON), DefaultOptionsJSON: `{}`, CreatedBy: "test", CreatedAt: now,
	}
	logicalModel := model.LogicalModel{
		ID: handlerFilmAgentLogicalModelID, Code: "film-agent-handler-text", Name: "Film Handler Text", Capability: "text", Enabled: true,
		RevisionSequence: 1, ActiveRevisionID: revision.ID, PricePolicy: "unified", BillingMode: "fixed_request",
		UnitPriceMicrocredits: 1, CreatedAt: now, UpdatedAt: now,
	}
	route := model.LogicalModelRoute{
		ID: "logical-route-film-agent-handler", LogicalModelRevisionID: revision.ID, ChannelModelID: channelModel.ID,
		Enabled: true, Priority: 100, Weight: 100, CreatedAt: now, UpdatedAt: now,
	}
	for _, item := range []any{&channel, &channelModel, &logicalModel, &revision, &route} {
		if err := db.Create(item).Error; err != nil {
			t.Fatalf("create handler Film text model fixture %T: %v", item, err)
		}
	}
}

func createFilmAgentHandlerLockFixture(t *testing.T, repo *repository.Repository, db *gorm.DB, detail repository.AgentRuntimeDetail) (model.ProductionArtifact, model.ProductionArtifactRevision) {
	t.Helper()
	now := time.Now().UTC()
	step := detail.Steps[0]
	if err := db.Model(&model.AgentRuntimeRun{}).Where("id = ?", detail.Run.ID).
		Updates(map[string]any{"status": model.AgentRunStatusCompleted, "completed_at": now}).Error; err != nil {
		t.Fatalf("complete handler fixture Run: %v", err)
	}
	if err := db.Model(&model.AgentRuntimeStep{}).Where("id = ?", step.ID).
		Updates(map[string]any{"status": model.AgentStepStatusCompleted, "attempt_sequence": 1, "completed_at": now}).Error; err != nil {
		t.Fatalf("complete handler fixture Step: %v", err)
	}
	attempt := model.AgentRuntimeAttempt{
		ID: "handler-lock-attempt", RunID: detail.Run.ID, StepID: step.ID, Number: 1,
		Status: model.AgentAttemptStatusSucceeded, Executor: "handler-fixture", InputDigest: "input", PromptDigest: "prompt",
		RequestJSON: "{}", ResponseJSON: "{}", Revision: 1, StartedAt: &now, CompletedAt: &now, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.Create(&attempt); err != nil {
		t.Fatalf("create handler fixture Attempt: %v", err)
	}
	artifact := &model.ProductionArtifact{
		ID: "handler-script-artifact", UserID: detail.Run.UserID, ProjectID: detail.Run.ProjectID, Domain: "film",
		ArtifactType: "script", LogicalKey: "handler:script",
	}
	revision := &model.ProductionArtifactRevision{
		ID: "handler-script-review", Status: model.ProductionArtifactStatusReview,
		ContentJSON: `{"schemaVersion":1,"artifactType":"script"}`, ContentDigest: "handler-script-digest",
		SourceRunID: detail.Run.ID, SourceStepID: step.ID, SourceAttemptID: attempt.ID,
		SourceArtifactRefsJSON: "[]", AuthorityRefsJSON: "[]", CreatedByType: "agent", CreatedByID: step.AgentID,
	}
	persistedArtifact, persistedRevision, err := repo.CreateProductionArtifactRevision(repository.ProductionArtifactRevisionCreate{
		UserID: detail.Run.UserID, Artifact: artifact, Revision: revision, ExpectedSequence: 0, At: now,
	})
	if err != nil {
		t.Fatalf("create handler fixture Artifact: %v", err)
	}
	return *persistedArtifact, *persistedRevision
}

func prepareFilmAgentHandlerCloseoutFixture(t *testing.T, repo *repository.Repository, db *gorm.DB, detail repository.AgentRuntimeDetail) {
	t.Helper()
	now := time.Now().UTC()
	step := detail.Steps[0]
	attempt := model.AgentRuntimeAttempt{
		ID: detail.Run.ID + "-http-closeout-attempt", RunID: detail.Run.ID, StepID: step.ID, Number: 1,
		Status: model.AgentAttemptStatusSucceeded, Executor: "handler-closeout-fixture", InputDigest: "input", PromptDigest: "prompt",
		RequestJSON: "{}", ResponseJSON: "{}", Revision: 1, StartedAt: &now, CompletedAt: &now, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.Create(&attempt); err != nil {
		t.Fatalf("create handler closeout Attempt: %v", err)
	}
	if err := db.Model(&model.AgentRuntimeStep{}).Where("id = ?", step.ID).Updates(map[string]any{
		"status": model.AgentStepStatusCompleted, "attempt_sequence": 1, "completed_at": now,
	}).Error; err != nil {
		t.Fatalf("complete handler closeout Step: %v", err)
	}
	if err := db.Model(&model.AgentRuntimeRun{}).Where("id = ?", detail.Run.ID).Updates(map[string]any{
		"status": model.AgentRunStatusCompleted, "completed_at": now,
	}).Error; err != nil {
		t.Fatalf("complete handler closeout Run: %v", err)
	}
	createArtifact := func(artifactType string) (model.ProductionArtifact, model.ProductionArtifactRevision) {
		artifact := &model.ProductionArtifact{
			ID: detail.Run.ID + "-http-artifact-" + artifactType, UserID: detail.Run.UserID, ProjectID: detail.Run.ProjectID,
			Domain: "film", ArtifactType: artifactType, LogicalKey: "run:" + detail.Run.ID + ":" + artifactType,
		}
		revision := &model.ProductionArtifactRevision{
			ID: detail.Run.ID + "-http-revision-" + artifactType, Status: model.ProductionArtifactStatusLocked,
			ContentJSON: `{"schemaVersion":1,"artifactType":"` + artifactType + `"}`, ContentDigest: "http-digest-" + artifactType,
			SourceRunID: detail.Run.ID, SourceStepID: step.ID, SourceAttemptID: attempt.ID,
			SourceArtifactRefsJSON: "[]", AuthorityRefsJSON: "[]", CreatedByType: "agent", CreatedByID: step.AgentID,
		}
		persistedArtifact, persistedRevision, err := repo.CreateProductionArtifactRevision(repository.ProductionArtifactRevisionCreate{
			UserID: detail.Run.UserID, Artifact: artifact, Revision: revision, ExpectedSequence: 0, At: now,
		})
		if err != nil {
			t.Fatalf("create handler closeout %s Artifact: %v", artifactType, err)
		}
		return *persistedArtifact, *persistedRevision
	}
	createArtifact("continuity-report")
	createArtifact("continuity-ledger")
	qcArtifact, qcRevision := createArtifact("qc-report")
	trigger := model.AgentHandoffTrigger{
		ID: detail.Run.ID + "-http-qc-trigger", UserID: detail.Run.UserID, ProjectID: detail.Run.ProjectID, Domain: "film",
		RootRunID: detail.Run.ID, ArtifactID: qcArtifact.ID, RevisionID: qcRevision.ID,
		SourceRunID: detail.Run.ID, SourceStepID: step.ID, SourceAgentID: step.AgentID,
		Status: model.AgentHandoffTriggerStatusCompleted, AttemptCount: 1, ScheduledRunIDsJSON: "[]", Revision: 1,
		CompletedAt: &now, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.Create(&trigger); err != nil {
		t.Fatalf("create handler closeout QC Trigger: %v", err)
	}
}

func filmAgentRuntimeRequest(t *testing.T, router http.Handler, method string, path string, body string, cookie string, idempotencyKey string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if cookie != "" {
		request.Header.Set("Cookie", cookie)
	}
	if idempotencyKey != "" {
		request.Header.Set("X-Idempotency-Key", idempotencyKey)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func decodeFilmAgentRuntimeResponse(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response %q: %v", response.Body.String(), err)
	}
}
