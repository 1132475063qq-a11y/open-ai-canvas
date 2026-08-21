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
		&model.ProductionArtifact{}, &model.ProductionArtifactRevision{},
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("canonical schema is missing %T", table)
		}
	}
}
