package service

import (
	"testing"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestValidateEcommerceRunRegistryPinPreservesLegacyAndRejectsDrift(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:ecommerce-registry-pin-contract?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	svc := New(repository.New(db), t.TempDir())
	registry := svc.ecommerceAgentRegistry
	if registry == nil {
		t.Fatal("Ecommerce registry was not loaded")
	}

	if err := svc.validateEcommerceRunRegistryPin(model.EcommerceProductionRun{}); err != nil {
		t.Fatalf("legacy unpinned Run should remain compatible: %v", err)
	}
	current := model.EcommerceProductionRun{RegistryID: registry.ID, RegistryVersion: registry.Version, RegistryDigest: registry.SourceDigest}
	if err := svc.validateEcommerceRunRegistryPin(current); err != nil {
		t.Fatalf("current registry pin rejected: %v", err)
	}
	partial := current
	partial.RegistryDigest = ""
	if err := svc.validateEcommerceRunRegistryPin(partial); authStatus(err) != 409 {
		t.Fatalf("partial registry pin error = %v, want 409", err)
	}
	drifted := current
	drifted.RegistryDigest = "different"
	if err := svc.validateEcommerceRunRegistryPin(drifted); authStatus(err) != 409 {
		t.Fatalf("drifted registry pin error = %v, want 409", err)
	}
}
