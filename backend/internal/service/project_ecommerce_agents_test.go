package service

import (
	"strings"
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestExpandEcommerceShotRolesAvoidsDuplicateCameraContracts(t *testing.T) {
	preset := systemEcommercePresets()[0]
	roles := expandEcommerceShotRoles(preset.Definition.ShotRoles, 12)
	if len(roles) != 12 {
		t.Fatalf("got %d shot roles, want 12", len(roles))
	}

	keys := make(map[string]struct{}, len(roles))
	contracts := make(map[string]struct{}, len(roles))
	for index, role := range roles {
		if _, exists := keys[role.Key]; exists {
			t.Fatalf("role %d reused key %q", index+1, role.Key)
		}
		keys[role.Key] = struct{}{}
		contract := strings.Join([]string{
			role.Camera.Azimuth,
			role.Camera.Elevation,
			role.Camera.CameraHeight,
			role.Camera.Lens,
			role.Camera.Distance,
			role.Camera.SubjectRegion,
			role.Camera.SubjectFill,
			role.Camera.Pose,
			role.Camera.Composition,
		}, "|")
		if _, exists := contracts[contract]; exists {
			t.Fatalf("role %d reused camera contract", index+1)
		}
		contracts[contract] = struct{}{}
		if index > 0 && len(role.Camera.AvoidReuseOf) < index {
			t.Fatalf("role %d does not record all previous roles in avoidReuseOf", index+1)
		}
	}
}

func TestEcommerceSkillPromptCarriesCameraAndDiversityRules(t *testing.T) {
	preset := systemEcommercePresets()[0]
	input := ecommerceRunPlanningInput{Run: modelRunFixtureForPrompt(), Preset: preset, ProductAssets: []ecommerceAssetFact{{ID: "asset-product", Title: "Product"}}}
	prompt, negative := (ecommerceSkillExecutor{}).compilePrompt(input, expandEcommerceShotRoles(preset.Definition.ShotRoles, 6)[5], 6)
	if !strings.Contains(prompt, ecommerceCameraContractMarker) || !strings.Contains(prompt, "forbidden reuse") {
		t.Fatalf("compiled prompt does not contain executable camera diversity contract: %s", prompt)
	}
	if !strings.Contains(negative, "重复机位") {
		t.Fatalf("negative prompt does not block duplicate compositions: %s", negative)
	}
}

func modelRunFixtureForPrompt() model.EcommerceProductionRun {
	return model.EcommerceProductionRun{
		OutputCount:   6,
		Kernel:        EcommerceKernelModelInteraction,
		AspectRatio:   "9:16",
		Resolution:    "4k",
		PixelSize:     "2160x3840",
		TargetChannel: "taobao_jd",
		Category:      "apparel",
		UserGoal:      "Show the product in a credible lifestyle context",
		ModelBrief:    "natural commercial model",
	}
}
