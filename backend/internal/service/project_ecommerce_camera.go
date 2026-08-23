package service

import (
	"fmt"
	"strings"
)

const ecommerceCameraContractMarker = "CAMERA CONTRACT (mandatory)"

// EcommerceShotCameraSpec turns shot diversity into an executable contract
// instead of leaving camera interpretation to free-form prompt wording.
type EcommerceShotCameraSpec struct {
	Azimuth       string   `json:"azimuth"`
	Elevation     string   `json:"elevation"`
	CameraHeight  string   `json:"cameraHeight"`
	Lens          string   `json:"lens"`
	Distance      string   `json:"distance"`
	SubjectRegion string   `json:"subjectRegion"`
	SubjectFill   string   `json:"subjectFill"`
	Pose          string   `json:"pose"`
	Composition   string   `json:"composition"`
	AvoidReuseOf  []string `json:"avoidReuseOf,omitempty"`
}

func normalizeEcommercePresetDefinition(definition EcommercePresetDefinition) EcommercePresetDefinition {
	definition.SchemaVersion = EcommercePresetSchemaVersion
	roles := make([]EcommercePresetShotRole, len(definition.ShotRoles))
	previousKeys := make([]string, 0, len(definition.ShotRoles))
	for index, role := range definition.ShotRoles {
		role = normalizeEcommerceShotRole(definition.Kernel, role, index)
		role.Camera.AvoidReuseOf = uniqueNonEmpty(append(role.Camera.AvoidReuseOf, previousKeys...))
		roles[index] = role
		if key := strings.TrimSpace(role.Key); key != "" {
			previousKeys = append(previousKeys, key)
		}
	}
	definition.ShotRoles = roles
	return definition
}

func normalizeEcommerceShotRole(kernel string, role EcommercePresetShotRole, index int) EcommercePresetShotRole {
	fallback := defaultEcommerceCameraSpec(kernel, role.Key, index)
	camera := role.Camera
	camera.Azimuth = firstNonEmpty(strings.TrimSpace(camera.Azimuth), fallback.Azimuth)
	camera.Elevation = firstNonEmpty(strings.TrimSpace(camera.Elevation), fallback.Elevation)
	camera.CameraHeight = firstNonEmpty(strings.TrimSpace(camera.CameraHeight), fallback.CameraHeight)
	camera.Lens = firstNonEmpty(strings.TrimSpace(camera.Lens), fallback.Lens)
	camera.Distance = firstNonEmpty(strings.TrimSpace(camera.Distance), fallback.Distance)
	camera.SubjectRegion = firstNonEmpty(strings.TrimSpace(camera.SubjectRegion), inferEcommerceSubjectRegion(role), fallback.SubjectRegion)
	camera.SubjectFill = firstNonEmpty(strings.TrimSpace(camera.SubjectFill), fallback.SubjectFill)
	camera.Pose = firstNonEmpty(strings.TrimSpace(camera.Pose), fallback.Pose)
	camera.Composition = firstNonEmpty(strings.TrimSpace(camera.Composition), fallback.Composition)
	if len(camera.AvoidReuseOf) == 0 {
		camera.AvoidReuseOf = append([]string(nil), fallback.AvoidReuseOf...)
	}
	role.Camera = camera
	return role
}

func defaultEcommerceCameraSpec(kernel string, roleKey string, index int) EcommerceShotCameraSpec {
	roleIndex := ecommerceCameraRoleIndex(roleKey, index)
	if kernel == EcommerceKernelStillLife {
		values := []EcommerceShotCameraSpec{
			{Azimuth: "front three-quarter at 20 degrees", Elevation: "slightly above the product", CameraHeight: "at product-center height", Lens: "70mm", Distance: "medium-close", SubjectRegion: "product", SubjectFill: "60-70% of frame height", Pose: "stable hero presentation", Composition: "clean asymmetrical hero composition with clear negative space"},
			{Azimuth: "left three-quarter at 45 degrees", Elevation: "eye-level to the setting", CameraHeight: "tabletop height", Lens: "28mm", Distance: "wide environmental", SubjectRegion: "product_in_environment", SubjectFill: "30-45% of frame height", Pose: "credible in-context placement", Composition: "layered foreground, product zone, and background anchors"},
			{Azimuth: "strict right-side profile at 90 degrees", Elevation: "level", CameraHeight: "at product-center height", Lens: "50mm", Distance: "medium", SubjectRegion: "product_and_scale_context", SubjectFill: "50-60% of frame height", Pose: "side-oriented display", Composition: "profile composition with scale cue on the opposite side"},
			{Azimuth: "right front three-quarter at 55 degrees", Elevation: "low oblique", CameraHeight: "just below product center", Lens: "40mm", Distance: "medium-close", SubjectRegion: "product_in_use", SubjectFill: "55-70% of frame height", Pose: "one physically credible use-state moment", Composition: "diagonal action composition with visible contact and force"},
			{Azimuth: "top-front detail angle", Elevation: "steep 55-degree top-down", CameraHeight: "macro working height", Lens: "100mm macro", Distance: "extreme close-up", SubjectRegion: "product_detail", SubjectFill: "75-90% of frame height", Pose: "static detail inspection", Composition: "single-detail macro crop without recreating the hero layout"},
			{Azimuth: "rear three-quarter at 135 degrees", Elevation: "level to slightly low", CameraHeight: "at product-base height", Lens: "85mm", Distance: "close", SubjectRegion: "product_reverse_or_side", SubjectFill: "65-80% of frame height", Pose: "reverse-side information view", Composition: "compressed reverse composition anchored to the opposite frame edge"},
		}
		return values[roleIndex%len(values)]
	}
	values := []EcommerceShotCameraSpec{
		{Azimuth: "front three-quarter at 30 degrees", Elevation: "level", CameraHeight: "centered on the featured product region", Lens: "50mm", Distance: "medium-close", SubjectRegion: "full_body_or_featured_product_region", SubjectFill: "65-75% of frame height", Pose: "balanced hero pose with a clear product silhouette", Composition: "asymmetrical hero composition with the product as the first visual read"},
		{Azimuth: "left three-quarter at 60 degrees", Elevation: "slightly low", CameraHeight: "below waist height", Lens: "26mm", Distance: "wide environmental", SubjectRegion: "full_body_in_environment", SubjectFill: "35-50% of frame height", Pose: "natural movement through the environment", Composition: "wide layered scene with strong spatial anchors and visible product"},
		{Azimuth: "strict right-side profile at 90 degrees", Elevation: "level", CameraHeight: "centered on the featured product region", Lens: "85mm", Distance: "medium", SubjectRegion: "person_product_relationship", SubjectFill: "60-70% of frame height", Pose: "profile pose distinct from the hero", Composition: "compressed side-profile composition with a different crop boundary"},
		{Azimuth: "rear three-quarter at 135 degrees", Elevation: "low", CameraHeight: "knee or product-use height", Lens: "35mm", Distance: "medium-close action", SubjectRegion: "product_interaction", SubjectFill: "60-75% of frame height", Pose: "one natural action at its clearest physical beat", Composition: "diagonal action composition with visible contact and weight"},
		{Azimuth: "top-front detail angle", Elevation: "steep 45-degree top-down", CameraHeight: "macro working height", Lens: "100mm macro", Distance: "extreme close-up", SubjectRegion: "product_detail", SubjectFill: "80-90% of frame height", Pose: "detail held still and unobstructed", Composition: "isolated texture or construction detail, never a crop of the hero frame"},
		{Azimuth: "opposite-side profile at 270 degrees", Elevation: "ground-level or product-level", CameraHeight: "aligned with the product base", Lens: "70mm", Distance: "close", SubjectRegion: "product_side_or_reverse", SubjectFill: "70-85% of frame height", Pose: "supplementary side or reverse pose", Composition: "opposite-edge composition covering information absent from all earlier shots"},
	}
	return values[roleIndex%len(values)]
}

func ecommerceCameraRoleIndex(roleKey string, fallback int) int {
	base := strings.ToLower(strings.TrimSpace(roleKey))
	for _, suffix := range []string{"_alt_2", "_alt_3", "_alt_4"} {
		base = strings.TrimSuffix(base, suffix)
	}
	keys := []string{"hero", "environment", "medium", "action", "detail", "supplement"}
	for index, key := range keys {
		if base == key {
			return index
		}
	}
	if fallback < 0 {
		return 0
	}
	return fallback
}

func inferEcommerceSubjectRegion(role EcommercePresetShotRole) string {
	value := strings.ToLower(strings.Join([]string{role.Title, role.Framing, role.Direction, role.Interaction}, " "))
	for _, term := range []string{"全身", "三分之二身", "full body", "three-quarter body"} {
		if strings.Contains(value, term) {
			return "full_body_with_product_priority"
		}
	}
	for _, term := range []string{"袜", "鞋", "脚", "足", "小腿", "脚踝", "sock", "shoe", "foot", "ankle", "lower leg"} {
		if strings.Contains(value, term) {
			return "feet_and_lower_body"
		}
	}
	for _, term := range []string{"手持", "手部", "握持", "hand", "holding"} {
		if strings.Contains(value, term) {
			return "hands_and_product"
		}
	}
	for _, term := range []string{"脸", "面部", "妆", "口红", "项链", "耳环", "face", "makeup", "necklace", "earring"} {
		if strings.Contains(value, term) {
			return "face_and_upper_body"
		}
	}
	for _, term := range []string{"微距", "特写", "细节", "macro", "detail", "close-up"} {
		if strings.Contains(value, term) {
			return "product_detail"
		}
	}
	return ""
}

func ecommerceCameraContract(role EcommercePresetShotRole) string {
	avoid := "no earlier slot"
	if len(role.Camera.AvoidReuseOf) > 0 {
		avoid = strings.Join(role.Camera.AvoidReuseOf, ", ")
	}
	return strings.Join([]string{
		ecommerceCameraContractMarker + ":",
		fmt.Sprintf("- camera azimuth: %s", role.Camera.Azimuth),
		fmt.Sprintf("- camera elevation: %s", role.Camera.Elevation),
		fmt.Sprintf("- camera height: %s", role.Camera.CameraHeight),
		fmt.Sprintf("- lens and distance: %s; %s", role.Camera.Lens, role.Camera.Distance),
		fmt.Sprintf("- subject region and occupancy: %s; %s", role.Camera.SubjectRegion, role.Camera.SubjectFill),
		fmt.Sprintf("- pose/blocking: %s", role.Camera.Pose),
		fmt.Sprintf("- composition: %s", role.Camera.Composition),
		fmt.Sprintf("- forbidden reuse: do not copy the camera angle, pose, crop, subject placement, or background-anchor layout of %s", avoid),
		"Continuity means the same product, person, scene world, and lighting logic; it never means repeating a previous composition.",
	}, "\n")
}

func ensureEcommerceCameraContract(prompt string, kernel string, slotRole string, title string, camera EcommerceShotCameraSpec) (string, EcommerceShotCameraSpec) {
	role := normalizeEcommerceShotRole(kernel, EcommercePresetShotRole{
		Key: slotRole, Title: title, Framing: prompt, Direction: prompt, Interaction: prompt, Camera: camera,
	}, ecommerceCameraRoleIndex(slotRole, 0))
	if strings.Contains(prompt, ecommerceCameraContractMarker) {
		return prompt, role.Camera
	}
	return strings.TrimSpace(prompt) + "\n" + ecommerceCameraContract(role), role.Camera
}

func alternateEcommerceCameraSpec(camera EcommerceShotCameraSpec, cycle int) EcommerceShotCameraSpec {
	camera.Azimuth = fmt.Sprintf("opposite-side cycle %d variation, at least 70 degrees away from %s", cycle, camera.Azimuth)
	camera.Elevation = fmt.Sprintf("a visibly different elevation level from %s", camera.Elevation)
	camera.CameraHeight = fmt.Sprintf("a clearly different camera height from %s", camera.CameraHeight)
	camera.Lens = fmt.Sprintf("a different focal-length class from %s", camera.Lens)
	camera.Distance = fmt.Sprintf("a different camera-to-subject distance from %s", camera.Distance)
	camera.SubjectFill = fmt.Sprintf("at least 15 percentage points different from %s", camera.SubjectFill)
	camera.Pose = fmt.Sprintf("cycle %d alternate moment; do not repeat the earlier blocking", cycle)
	camera.Composition = fmt.Sprintf("cycle %d opposite-third composition with new crop boundaries and background anchors", cycle)
	return camera
}
