package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
)

const (
	// These IDs are the canonical keys in the versioned Ecommerce registry.
	// User-facing labels may remain more readable, but persisted Artifact
	// responsibility must resolve to one registry identity.
	EcommerceAgentProductIntelligence = "product_intelligence_agent"
	EcommerceAgentCreativeDirector    = "creative_director_agent"
	EcommerceAgentSceneDirector       = "scene_director_agent"
	EcommerceAgentMotionDirector      = "motion_director_agent"
	EcommerceAgentOrchestrator        = "ecommerce_orchestrator"
)

type ecommerceAssetFact struct {
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	Kind     string         `json:"kind"`
	Category string         `json:"category"`
	Data     map[string]any `json:"data,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type ecommerceRunPlanningInput struct {
	Run              model.EcommerceProductionRun
	Preset           EcommercePreset
	ProductAssets    []ecommerceAssetFact
	SupportingAssets []ecommerceAssetFact
	ModelAssets      []ecommerceAssetFact
	SceneAssets      []ecommerceAssetFact
	BrandAssets      []ecommerceAssetFact
	ProductFacts     map[string]any
	Advanced         map[string]any
	Now              time.Time
}

type ecommerceRunPlan struct {
	Artifacts []model.EcommerceArtifact
	Slots     []model.EcommerceProductionSlot
}

type productIntelligenceAgent struct{}

func (productIntelligenceAgent) analyze(input ecommerceRunPlanningInput) map[string]any {
	products := make([]map[string]any, 0, len(input.ProductAssets))
	for index, asset := range input.ProductAssets {
		products = append(products, map[string]any{
			"assetId": asset.ID, "title": asset.Title, "category": asset.Category,
			"role": map[bool]string{true: "primary", false: "supporting"}[index == 0],
		})
	}
	return map[string]any{
		"schemaVersion": 1,
		"agentId":       EcommerceAgentProductIntelligence,
		"evidence":      map[bool]string{true: "recorded", false: "inferred"}[len(input.ProductFacts) > 0],
		"category":      input.Run.Category,
		"products":      products,
		"recordedFacts": input.ProductFacts,
		"mustPreserve": []string{
			"overall silhouette and proportions", "material and surface finish", "recorded color",
			"package structure", "logo placement and readable text", "interfaces, fasteners and accessories",
		},
		"unknownPolicy": "Unknown product claims, dimensions, ingredients and certifications must remain unknown; never invent them.",
	}
}

type creativeDirectorAgent struct{}

func (creativeDirectorAgent) direct(input ecommerceRunPlanningInput, roles []EcommercePresetShotRole) map[string]any {
	shots := make([]map[string]any, 0, len(roles))
	for index, role := range roles {
		shots = append(shots, map[string]any{
			"position": index + 1, "role": role.Key, "title": role.Title, "framing": role.Framing,
			"direction": role.Direction, "interaction": role.Interaction, "camera": role.Camera,
		})
	}
	return map[string]any{
		"schemaVersion": 1, "agentId": EcommerceAgentCreativeDirector,
		"goal": input.Run.UserGoal, "targetChannel": input.Run.TargetChannel,
		"aspectRatio": input.Run.AspectRatio, "resolution": input.Run.Resolution,
		"pixelSize": input.Run.PixelSize, "kernel": input.Preset.Kernel,
		"visualDirection": fmt.Sprintf("%s。%s", input.Preset.Description, input.Preset.Definition.SceneTemplate),
		"seriesRules": []string{
			"One product identity, one model identity where applicable, and one coherent scene world across the set.",
			"Each slot must use its own mandatory camera contract and carry a distinct commercial information role.",
			"Continuity must never be interpreted as permission to repeat camera angle, pose, crop, subject placement, or background-anchor layout.",
			"Keep lighting direction, palette, styling and product state continuous unless the shot plan explicitly changes them.",
		},
		"shots": shots,
	}
}

type sceneDirectorAgent struct{}

func (sceneDirectorAgent) plan(input ecommerceRunPlanningInput) map[string]any {
	assetRefs := make([]map[string]any, 0, len(input.SceneAssets))
	for _, asset := range input.SceneAssets {
		assetRefs = append(assetRefs, map[string]any{"assetId": asset.ID, "title": asset.Title})
	}
	brief := firstNonEmpty(strings.TrimSpace(input.Run.SceneBrief), input.Preset.Definition.SceneTemplate)
	return map[string]any{
		"schemaVersion": 1, "agentId": EcommerceAgentSceneDirector,
		"brief": brief, "referenceAssets": assetRefs,
		"wideView":     fmt.Sprintf("A coherent establishing view of %s", brief),
		"localViews":   []string{"primary product zone", "interaction zone", "detail and texture zone"},
		"lighting":     map[string]any{"continuity": "locked across the series", "direction": advancedString(input.Advanced, "lighting", "soft directional commercial light with believable practical fill")},
		"palette":      map[string]any{"continuity": "locked across the series", "direction": advancedString(input.Advanced, "palette", "balanced natural color with brand accents")},
		"props":        map[string]any{"policy": "Only props that clarify scale, use or story; never obscure the product."},
		"spatialRules": []string{"Keep major background anchors in stable positions.", "Preserve contact shadows, scale and depth cues.", "Reverse or detail angles must still belong to the same physical space."},
	}
}

type ecommerceSkillExecutor struct{}

func (ecommerceSkillExecutor) compilePrompt(input ecommerceRunPlanningInput, role EcommercePresetShotRole, position int) (string, string) {
	productNames := ecommerceAssetTitles(input.ProductAssets)
	modelDirective := "No person is present."
	if input.Preset.Kernel == EcommerceKernelModelInteraction {
		if len(input.ModelAssets) > 0 {
			modelDirective = "Use the authorized model references only as an identity and styling lock; preserve face, age, body proportions and styling, but never copy their camera angle, pose, crop, subject placement, or background."
		} else {
			modelDirective = fmt.Sprintf("Use one consistent fictional AI model identity throughout the set from this locked text brief: %s. No completed campaign frame may be reused as an identity reference; obey this slot's camera contract independently.", firstNonEmpty(strings.TrimSpace(input.Run.ModelBrief), "natural commercial model appropriate for the product and target audience"))
		}
	}
	brandDirective := "No unrecorded brand claims or decorative text."
	if strings.TrimSpace(input.Run.BrandBrief) != "" || len(input.BrandAssets) > 0 {
		brandDirective = fmt.Sprintf("Brand direction: %s. Preserve only supplied logos and recorded mandatory information.", firstNonEmpty(strings.TrimSpace(input.Run.BrandBrief), "follow the supplied brand references"))
	}
	constraints := input.Preset.Definition.RequiredConstraints
	prompt := strings.Join([]string{
		"Create one production-ready ecommerce campaign photograph.",
		fmt.Sprintf("Series slot %d/%d: %s (%s).", position, input.Run.OutputCount, role.Title, role.Key),
		fmt.Sprintf("Primary product references: %s. Treat every supplied product image as authoritative.", strings.Join(productNames, ", ")),
		fmt.Sprintf("Commercial goal: %s.", firstNonEmpty(strings.TrimSpace(input.Run.UserGoal), "show the product clearly in a credible lifestyle context")),
		fmt.Sprintf("Target: %s, aspect ratio %s, output %s (%s pixels), category %s.", input.Run.TargetChannel, input.Run.AspectRatio, strings.ToUpper(input.Run.Resolution), input.Run.PixelSize, input.Run.Category),
		fmt.Sprintf("Scene: %s.", firstNonEmpty(strings.TrimSpace(input.Run.SceneBrief), input.Preset.Definition.SceneTemplate)),
		fmt.Sprintf("Shot direction: %s. Framing: %s. Interaction: %s.", role.Direction, role.Framing, role.Interaction),
		ecommerceCameraContract(role),
		fmt.Sprintf("Preset interaction rule: %s.", input.Preset.Definition.InteractionTemplate),
		modelDirective,
		brandDirective,
		"Product fidelity constraints: " + strings.Join(constraints.ProductFidelity, "; ") + ".",
		"Identity and physical safety constraints: " + strings.Join(constraints.IdentitySafety, "; ") + ".",
		"Reference-role policy: product images govern product geometry; model images govern identity only; scene images govern environment vocabulary only. Never inherit camera or composition from a reference image.",
		"Series continuity: same product geometry, exact colors, same model identity where present, same scene world, stable lighting direction and commercial color treatment. Every slot must remain compositionally distinct.",
		"Render as a believable high-end commercial photograph with clean detail, natural materials and physically plausible contact.",
	}, "\n")
	negative := strings.TrimSpace(input.Preset.Definition.NegativePrompt)
	if custom := advancedString(input.Advanced, "negativePrompt", ""); custom != "" {
		negative += "，" + custom
	}
	negative += "，重复机位，重复姿势，重复裁切，近似相同构图，复制前序镜头背景布局"
	return prompt, negative
}

func buildEcommerceRunPlan(input ecommerceRunPlanningInput) (ecommerceRunPlan, error) {
	definition := normalizeEcommercePresetDefinition(input.Preset.Definition)
	input.Preset.Definition = definition
	roles := expandEcommerceShotRoles(definition.ShotRoles, input.Run.OutputCount)
	productDNA := productIntelligenceAgent{}.analyze(input)
	creativeDirection := creativeDirectorAgent{}.direct(input, roles)
	scenePack := sceneDirectorAgent{}.plan(input)
	modelProfile := ecommerceModelProfile(input)
	presetSnapshot := map[string]any{
		"schemaVersion": 1, "presetId": input.Preset.ID, "presetKey": input.Preset.PresetKey,
		"presetVersion": input.Preset.Version, "name": input.Preset.Name, "system": input.Preset.System,
		"definition": input.Preset.Definition,
	}

	artifacts := make([]model.EcommerceArtifact, 0, 7)
	productRefs := ecommerceAssetIDs(input.ProductAssets)
	allRefs := uniqueNonEmpty(append(append(append(append([]string{}, productRefs...), ecommerceAssetIDs(input.SupportingAssets)...), ecommerceAssetIDs(input.ModelAssets)...), append(ecommerceAssetIDs(input.SceneAssets), ecommerceAssetIDs(input.BrandAssets)...)...))
	productArtifact, err := plannedEcommerceArtifact(input.Run, EcommerceArtifactTypeProductDNA, "product-dna", productDNA, productRefs, EcommerceAgentProductIntelligence, "")
	if err != nil {
		return ecommerceRunPlan{}, err
	}
	artifacts = append(artifacts, productArtifact)
	if input.Preset.Kernel == EcommerceKernelModelInteraction {
		modelRefs := ecommerceAssetIDs(input.ModelAssets)
		if len(modelRefs) == 0 {
			modelRefs = productRefs
		}
		artifact, artifactErr := plannedEcommerceArtifact(input.Run, EcommerceArtifactTypeModelProfile, "model-profile", modelProfile, modelRefs, EcommerceAgentSceneDirector, "")
		if artifactErr != nil {
			return ecommerceRunPlan{}, artifactErr
		}
		artifacts = append(artifacts, artifact)
	}
	sceneRefs := ecommerceAssetIDs(input.SceneAssets)
	if len(sceneRefs) == 0 {
		sceneRefs = productRefs
	}
	sceneArtifact, err := plannedEcommerceArtifact(input.Run, EcommerceArtifactTypeScenePack, "scene-pack", scenePack, sceneRefs, EcommerceAgentSceneDirector, "")
	if err != nil {
		return ecommerceRunPlan{}, err
	}
	artifacts = append(artifacts, sceneArtifact)
	presetArtifact, err := plannedEcommerceArtifact(input.Run, EcommerceArtifactTypePresetSnapshot, "preset", presetSnapshot, allRefs, EcommerceAgentOrchestrator, input.Preset.Definition.SkillRef)
	if err != nil {
		return ecommerceRunPlan{}, err
	}
	artifacts = append(artifacts, presetArtifact)
	creativeArtifact, err := plannedEcommerceArtifact(input.Run, EcommerceArtifactTypeCreativeDirection, "creative-direction", creativeDirection, allRefs, EcommerceAgentCreativeDirector, input.Preset.Definition.SkillRef)
	if err != nil {
		return ecommerceRunPlan{}, err
	}
	artifacts = append(artifacts, creativeArtifact)

	slots := make([]model.EcommerceProductionSlot, 0, len(roles))
	shotPayload := make([]map[string]any, 0, len(roles))
	executor := ecommerceSkillExecutor{}
	for index, role := range roles {
		prompt, negative := executor.compilePrompt(input, role, index+1)
		cameraJSON, cameraErr := json.Marshal(role.Camera)
		if cameraErr != nil {
			return ecommerceRunPlan{}, BadAuthRequest("电商镜头机位合同无法序列化")
		}
		slot := model.EcommerceProductionSlot{
			ID: newID(), UserID: input.Run.UserID, ProjectID: input.Run.ProjectID, RunID: input.Run.ID,
			Position: index + 1, Role: role.Key, Title: role.Title, CameraJSON: string(cameraJSON), Prompt: prompt, NegativePrompt: negative,
			Status: EcommerceSlotStatusPlanned, QAStatus: EcommerceQAStatusPending, QAIssuesJSON: "[]",
			CreatedAt: input.Now, UpdatedAt: input.Now,
		}
		slots = append(slots, slot)
		shotPayload = append(shotPayload, map[string]any{
			"slotId": slot.ID, "position": slot.Position, "role": slot.Role, "title": slot.Title,
			"framing": role.Framing, "direction": role.Direction, "interaction": role.Interaction, "camera": role.Camera,
			"durationMs": role.DurationMS, "prompt": prompt, "negativePrompt": negative,
		})
	}
	shotPlan := map[string]any{"schemaVersion": 1, "agentId": EcommerceAgentCreativeDirector, "runId": input.Run.ID, "slots": shotPayload}
	shotArtifact, err := plannedEcommerceArtifact(input.Run, EcommerceArtifactTypeCreativeShotPlan, "shot-plan", shotPlan, allRefs, EcommerceAgentCreativeDirector, input.Preset.Definition.SkillRef)
	if err != nil {
		return ecommerceRunPlan{}, err
	}
	artifacts = append(artifacts, shotArtifact)
	generationRequest := map[string]any{
		"schemaVersion": 1, "runId": input.Run.ID, "kernel": input.Run.Kernel,
		"presetSnapshotArtifactId": presetArtifact.ID, "productDnaArtifactId": productArtifact.ID,
		"scenePackArtifactId": sceneArtifact.ID, "shotPlanArtifactId": shotArtifact.ID,
		"aspectRatio": input.Run.AspectRatio, "resolution": input.Run.Resolution,
		"pixelSize": input.Run.PixelSize, "outputCount": input.Run.OutputCount,
		"sourceRefs": allRefs, "state": "awaiting_cost",
	}
	requestArtifact, err := plannedEcommerceArtifact(input.Run, EcommerceArtifactTypeGenerationRequest, "generation-request", generationRequest, allRefs, EcommerceAgentOrchestrator, input.Preset.Definition.SkillRef)
	if err != nil {
		return ecommerceRunPlan{}, err
	}
	artifacts = append(artifacts, requestArtifact)
	return ecommerceRunPlan{Artifacts: artifacts, Slots: slots}, nil
}

func ecommerceModelProfile(input ecommerceRunPlanningInput) map[string]any {
	mode := "ai_created"
	status := "brief_locked"
	if len(input.ModelAssets) > 0 {
		mode = "authorized_upload"
		status = "locked"
	}
	return map[string]any{
		"schemaVersion": 1, "agentId": EcommerceAgentSceneDirector, "mode": mode, "status": status,
		"brief":                   firstNonEmpty(strings.TrimSpace(input.Run.ModelBrief), "natural commercial model appropriate for the product and target audience"),
		"referenceAssets":         ecommerceAssetIDs(input.ModelAssets),
		"identityReferencePolicy": map[bool]string{true: "authorized_model_assets_only", false: "locked_text_brief_only_no_campaign-frame-seed"}[len(input.ModelAssets) > 0],
		"identityRules":           []string{"Preserve face, age, body proportions and distinctive features.", "Use one identity across all accepted slots.", "Identity references constrain the person only and must never constrain camera, pose, crop, or background.", "Do not infer a real person's identity from an AI brief."},
	}
}

func plannedEcommerceArtifact(run model.EcommerceProductionRun, artifactType string, keySuffix string, payload any, sourceRefs []string, agentID string, skillRef string) (model.EcommerceArtifact, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return model.EcommerceArtifact{}, BadAuthRequest("电商 Agent 产物无法序列化")
	}
	refs, _ := json.Marshal(uniqueNonEmpty(sourceRefs))
	authorityRefs, _ := json.Marshal([]string{"run:" + run.ID})
	return model.EcommerceArtifact{
		ID: newID(), ProjectID: run.ProjectID, ArtifactKey: "run:" + run.ID + ":" + keySuffix,
		ArtifactType: artifactType, SchemaVersion: 1, Lifecycle: "finalized", Evidence: "inferred",
		ResponsibleAgentID: agentID, SkillRef: skillRef, PayloadJSON: string(encoded),
		SourceRefsJSON: string(refs), AuthorityRefsJSON: string(authorityRefs), CreatedAt: run.CreatedAt, UpdatedAt: run.UpdatedAt,
	}, nil
}

func expandEcommerceShotRoles(base []EcommercePresetShotRole, count int) []EcommercePresetShotRole {
	if count < 1 {
		count = 1
	}
	if count > 12 {
		count = 12
	}
	result := make([]EcommercePresetShotRole, 0, count)
	for index := 0; index < count; index++ {
		role := normalizeEcommerceShotRole("", base[index%len(base)], index%len(base))
		if index >= len(base) {
			cycle := index/len(base) + 1
			role.Key = fmt.Sprintf("%s_alt_%d", role.Key, cycle)
			role.Title = fmt.Sprintf("%s变化 %d", role.Title, cycle)
			role.Direction += " Use a clearly different camera angle or moment while preserving continuity."
			role.Camera = alternateEcommerceCameraSpec(role.Camera, cycle)
		}
		role.Camera.AvoidReuseOf = make([]string, 0, len(result))
		for _, previous := range result {
			role.Camera.AvoidReuseOf = append(role.Camera.AvoidReuseOf, previous.Key)
		}
		result = append(result, role)
	}
	return result
}

func ecommerceAssetIDs(values []ecommerceAssetFact) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.ID)
	}
	return result
}

func ecommerceAssetTitles(values []ecommerceAssetFact) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, firstNonEmpty(strings.TrimSpace(value.Title), value.ID))
	}
	return result
}

func advancedString(values map[string]any, key string, fallback string) string {
	if value, exists := values[key]; exists {
		if text := strings.TrimSpace(fmt.Sprint(value)); text != "" {
			return text
		}
	}
	return fallback
}
