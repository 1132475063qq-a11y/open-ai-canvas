package service

import (
	"strings"
	"testing"
	"time"
)

func TestEcommerceTargetRegistryHasFiveIntentAndHandoffContracts(t *testing.T) {
	registry, err := (&Service{}).EcommerceTargetRuntimeRegistry()
	if err != nil {
		t.Fatalf("EcommerceTargetRuntimeRegistry(): %v", err)
	}
	if registry.Domain != EcommerceTargetDomain || len(registry.Skills) != 5 || len(registry.IntentRoutes) != 5 || len(registry.HandoffRoutes) != 5 || len(registry.SourceDigest) != 64 {
		t.Fatalf("unexpected Ecommerce registry: %#v", registry)
	}
	for _, route := range registry.HandoffRoutes {
		if !route.RequiresLockedInput || route.ExecutionMode != "agent" || len(route.RequiredInputArtifactGroups) == 0 {
			t.Fatalf("handoff %s is not locked-input agent contract: %#v", route.ID, route)
		}
	}
}
func TestCompileEcommerceTargetDAGKeepsAllRoutesInOneRootRun(t *testing.T) {
	steps, err := CompileEcommerceTargetDAG("target-root", []EcommerceTargetArtifactRef{{RevisionID: "locked-product", Type: EcommerceArtifactTypeProductUpload, Status: "locked"}}, false, time.Unix(10, 0))
	if err != nil {
		t.Fatalf("CompileEcommerceTargetDAG(): %v", err)
	}
	if len(steps) != 10 {
		t.Fatalf("step count = %d, want 10", len(steps))
	}
	for index, step := range steps {
		if step.RunID != "target-root" || step.Position != index || strings.TrimSpace(step.ExpectedOutputArtifactTypesJSON) == "" {
			t.Fatalf("step %d lost root/output contract: %#v", index, step)
		}
	}
	if steps[0].RouteKind != "intent" || steps[1].RouteKind != "handoff" || steps[9].RouteID != "HR-05" {
		t.Fatalf("DAG route order = %#v / %#v / %#v", steps[0], steps[1], steps[9])
	}
	if steps[0].Status != "ready" || steps[1].Status != "planned" {
		t.Fatalf("initial statuses = %s/%s", steps[0].Status, steps[1].Status)
	}
}

func TestNormalizeEcommerceTargetInputRejectsProviderSecrets(t *testing.T) {
	if _, err := normalizeEcommerceTargetInput(map[string]any{"authorization": "secret"}); err == nil {
		t.Fatal("target input with authorization must be rejected")
	}
}
