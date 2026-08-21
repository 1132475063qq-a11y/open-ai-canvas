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

func TestFilmAgentRuntimeHTTPContract(t *testing.T) {
	router, cookie, project := newFilmAgentRuntimeTestRouter(t)

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

	createResponse := filmAgentRuntimeRequest(t, router, http.MethodPost, "/api/projects/"+project.ID+"/film/agent-runs", `{"objective":"写一个原创短片故事","intentRouteId":"IR-01","reviewBeforeExecution":true}`, cookie, "film-http-create-0001")
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

	replayResponse := filmAgentRuntimeRequest(t, router, http.MethodPost, "/api/projects/"+project.ID+"/film/agent-runs", `{"objective":"写一个原创短片故事","intentRouteId":"IR-01","reviewBeforeExecution":true}`, cookie, "film-http-create-0001")
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

	oversizedBody := `{"objective":"` + strings.Repeat("x", filmAgentRunRequestLimit) + `","intentRouteId":"IR-01"}`
	oversized := filmAgentRuntimeRequest(t, router, http.MethodPost, "/api/projects/"+project.ID+"/film/agent-runs", oversizedBody, cookie, "film-http-large-0001")
	if oversized.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized status = %d, body = %s", oversized.Code, oversized.Body.String())
	}
}

func newFilmAgentRuntimeTestRouter(t *testing.T) (*gin.Engine, string, model.Project) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "film-agent-handler.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open handler database: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.AuthSession{}, &model.Project{}, &model.CanvasProject{},
		&model.AgentRuntimeRun{}, &model.AgentRuntimeStep{}, &model.AgentRuntimeAttempt{},
		&model.AgentRoutingDecision{}, &model.AgentHumanDecision{}, &model.AgentRuntimeEvent{},
		&model.ProductionArtifact{}, &model.ProductionArtifactRevision{},
	); err != nil {
		t.Fatalf("migrate handler database: %v", err)
	}
	repo := repository.New(db)
	svc := service.New(repo, t.TempDir())
	if err := svc.ValidateRuntime(); err != nil {
		t.Fatalf("ValidateRuntime(): %v", err)
	}
	now := time.Now().UTC()
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
	return router, service.SessionCookieName + "=" + session.ID + "." + token, project
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
