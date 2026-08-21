package service

import (
	"strings"
	"testing"

	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestServiceLoadsVerifiedFilmAgentRegistryAtStartup(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:service-film-registry?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	svc := New(repository.New(db), t.TempDir())
	if err := svc.ValidateRuntime(); err != nil {
		t.Fatalf("ValidateRuntime(): %v", err)
	}
	summary, err := svc.FilmAgentRuntimeRegistrySummary()
	if err != nil {
		t.Fatalf("FilmAgentRuntimeRegistrySummary(): %v", err)
	}
	if summary.ID != "film-agent-team" || summary.Version != "1.3.1" || summary.Domain != "film" ||
		summary.AgentCount != 9 || summary.SkillCount != 17 || summary.IntentRouteCount != 15 || summary.HandoffRouteCount != 11 || len(summary.SourceDigest) != 64 {
		t.Fatalf("unexpected Film registry summary: %#v", summary)
	}
}

func TestServiceRuntimeValidationRejectsMutatedFilmRegistry(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:service-invalid-film-registry?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	svc := New(repository.New(db), t.TempDir())
	svc.filmAgentRegistry.IntentRoutes[0].PrimaryAgentID = "missing_agent"
	err = svc.ValidateRuntime()
	if err == nil || !strings.Contains(err.Error(), "unknown primary agent") {
		t.Fatalf("ValidateRuntime() error = %v, want invalid Film registry", err)
	}
}
