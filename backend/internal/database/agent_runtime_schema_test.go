package database

import (
	"testing"

	"infinite-canvas/backend/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAgentRuntimeModelsArePartOfCanonicalSchema(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/runtime.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatalf("MigrateSchema() error = %v", err)
	}
	for _, table := range []any{
		&model.AgentRuntimeRun{}, &model.AgentRuntimeStep{}, &model.AgentRuntimeAttempt{},
		&model.AgentRoutingDecision{}, &model.AgentHumanDecision{}, &model.AgentRuntimeEvent{},
		&model.AgentHandoffTrigger{},
		&model.ProductionArtifact{}, &model.ProductionArtifactRevision{},
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("canonical schema is missing %T", table)
		}
	}
}

func TestAgentRuntimeMigrationBackfillsIntentRouteLineage(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/runtime-backfill.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&model.AgentRuntimeRun{}, &model.AgentRoutingDecision{}); err != nil {
		t.Fatalf("create pre-backfill schema: %v", err)
	}
	run := model.AgentRuntimeRun{
		ID: "legacy-intent-run", UserID: "legacy-user", ProjectID: "legacy-project", Domain: "film",
		RegistryID: "film-agent-team", RegistryVersion: "1.3.1", IntentRouteID: "IR-01",
		Status: model.AgentRunStatusCompleted, Objective: "legacy", InputJSON: "{}", IdempotencyKey: "legacy-intent-request",
		Revision: 1,
	}
	decision := model.AgentRoutingDecision{ID: "legacy-route", RunID: run.ID, IntentRouteID: "IR-01"}
	if err := db.Create(&run).Error; err != nil {
		t.Fatalf("create legacy Run: %v", err)
	}
	if err := db.Create(&decision).Error; err != nil {
		t.Fatalf("create legacy RoutingDecision: %v", err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatalf("MigrateSchema() error = %v", err)
	}
	var migratedRun model.AgentRuntimeRun
	var migratedDecision model.AgentRoutingDecision
	if err := db.First(&migratedRun, "id = ?", run.ID).Error; err != nil {
		t.Fatalf("load migrated Run: %v", err)
	}
	if err := db.First(&migratedDecision, "id = ?", decision.ID).Error; err != nil {
		t.Fatalf("load migrated RoutingDecision: %v", err)
	}
	if migratedRun.RouteKind != "intent" || migratedRun.RootRunID != migratedRun.ID ||
		migratedDecision.RouteKind != "intent" || migratedDecision.RouteID != "IR-01" {
		t.Fatalf("legacy Agent Runtime lineage was not backfilled: run=%#v decision=%#v", migratedRun, migratedDecision)
	}
}
