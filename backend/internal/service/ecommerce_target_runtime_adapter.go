package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

// EcommerceTargetRunAdapterRequest allows a caller to override only fields
// that are intentionally user-facing. The locked target Artifact remains the
// authority for product/source facts and the runtime pin.
type EcommerceTargetRunAdapterRequest struct {
	IdempotencyKey string                     `json:"idempotencyKey,omitempty"`
	Run            CreateEcommerceRunRequest `json:"run"`
}
// CreateProjectEcommerceRunFromTargetArtifact is the sole adapter from the
// generic target Artifact domain into the existing Ecommerce media aggregate.
// It creates a normal planning Run; it does not call a Provider and cannot
// mark media ready without a real Provider task/result.
func (s *Service) CreateProjectEcommerceRunFromTargetArtifact(userID string, projectID string, revisionID string, request EcommerceTargetRunAdapterRequest) (*EcommerceRunView, error) {
	if _, err := s.requireEcommerceProject(userID, projectID); err != nil {
		return nil, err
	}
	artifact, revision, err := s.repo.ProductionArtifactRevisionForUser(userID, strings.TrimSpace(revisionID))
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && (artifact.ProjectID != projectID || artifact.Domain != EcommerceTargetDomain)) {
		return nil, NotFound("Ecommerce target Artifact revision 不存在")
	}
	if err != nil {
		return nil, err
	}
	if revision.Status != model.ProductionArtifactStatusLocked {
		return nil, Conflict("只有 LOCKED Ecommerce target Artifact 才能适配生产 Run")
	}
	if err := validateEcommerceTargetArtifactContent(*artifact, *revision); err != nil {
		return nil, err
	}

	runRequest, err := decodeEcommerceTargetRunRequest(revision.ContentJSON)
	if err != nil {
		return nil, err
	}
	applyEcommerceTargetRunOverrides(&runRequest, request.Run)
	if key := strings.TrimSpace(request.IdempotencyKey); key != "" {
		runRequest.IdempotencyKey = key
	}
	if strings.TrimSpace(runRequest.IdempotencyKey) == "" {
		runRequest.IdempotencyKey = "ecommerce-target:" + revision.ID
	}
	if runRequest.Advanced == nil {
		runRequest.Advanced = map[string]any{}
	}
	runRequest.Advanced["targetArtifactId"] = artifact.ID
	runRequest.Advanced["targetArtifactRevisionId"] = revision.ID
	runRequest.Advanced["targetArtifactDigest"] = revision.ContentDigest
	runRequest.Advanced["targetArtifactType"] = artifact.ArtifactType
	runRequest.Advanced["targetRegistryId"] = EcommerceAgentRegistryID
	runRequest.Advanced["targetRegistryVersion"] = EcommerceAgentRegistryVersion
	runRequest.Advanced["targetRegistryDigest"] = ecommerceTargetRegistry.SourceDigest
	return s.CreateProjectEcommerceRun(userID, projectID, runRequest)
}

// AdaptEcommerceTargetArtifactToRun is a descriptive alias kept for callers
// that treat this operation as an adapter rather than a create endpoint.
func (s *Service) AdaptEcommerceTargetArtifactToRun(userID string, projectID string, revisionID string, request EcommerceTargetRunAdapterRequest) (*EcommerceRunView, error) {
	return s.CreateProjectEcommerceRunFromTargetArtifact(userID, projectID, revisionID, request)
}

func validateEcommerceTargetArtifactContent(artifact model.ProductionArtifact, revision model.ProductionArtifactRevision) error {
	if strings.TrimSpace(artifact.ArtifactType) == "" || strings.TrimSpace(revision.ContentJSON) == "" {
		return BadAuthRequest("Ecommerce target Artifact 缺少类型或内容")
	}
	if artifact.ArtifactType != EcommerceTargetArtifactType && artifact.ArtifactType != EcommerceTargetRequestType && artifact.ArtifactType != EcommerceTargetPlanType && !strings.HasPrefix(artifact.ArtifactType, "ecommerce-") {
		return BadAuthRequest("Artifact 类型不是 Ecommerce target 合同")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(revision.ContentJSON), &payload); err != nil {
		return BadAuthRequest("Ecommerce target Artifact 内容无效")
	}
	if registry, ok := payload["registry"].(map[string]any); ok {
		if fmt.Sprint(registry["id"]) != EcommerceAgentRegistryID || fmt.Sprint(registry["version"]) != EcommerceAgentRegistryVersion || fmt.Sprint(registry["digest"]) != ecommerceTargetRegistry.SourceDigest {
			return Conflict("Ecommerce target Artifact registry pin 已过期")
		}
	}
	if len(normalizedEcommerceTargetAssetIDs(payload)) == 0 {
		return BadAuthRequest("Ecommerce target Artifact 必须包含产品 source asset refs")
	}
	return nil
}

func normalizedEcommerceTargetAssetIDs(payload map[string]any) []string {
	for _, key := range []string{"productAssetIds", "productAssetIDs", "sourceAssetRefs", "sourceAssetIds", "product_asset_ids"} {
		if raw, ok := payload[key].([]any); ok {
			ids := make([]string, 0, len(raw))
			for _, item := range raw {
				if value := strings.TrimSpace(fmt.Sprint(item)); value != "" && value != "<nil>" {
					ids = append(ids, value)
				}
			}
			if len(ids) > 0 {
				return uniqueNonEmpty(ids)
			}
		}
	}
	return nil
}

func decodeEcommerceTargetRunRequest(raw string) (CreateEcommerceRunRequest, error) {
	var envelope map[string]any
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return CreateEcommerceRunRequest{}, BadAuthRequest("Ecommerce target Run 请求内容无效")
	}
	for _, key := range []string{"runRequest", "ecommerceRun", "run", "payload"} {
		if nested, ok := envelope[key].(map[string]any); ok {
			envelope = nested
			break
		}
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return CreateEcommerceRunRequest{}, err
	}
	var request CreateEcommerceRunRequest
	if err := json.Unmarshal(encoded, &request); err != nil {
		return CreateEcommerceRunRequest{}, BadAuthRequest("Ecommerce target Run 请求字段无效")
	}
	if len(request.ProductAssetIDs) == 0 {
		if ids := normalizedEcommerceTargetAssetIDs(envelope); len(ids) > 0 {
			request.ProductAssetIDs = ids
		}
	}
	return request, nil
}

func applyEcommerceTargetRunOverrides(target *CreateEcommerceRunRequest, override CreateEcommerceRunRequest) {
	if len(override.ProductAssetIDs) > 0 { target.ProductAssetIDs = override.ProductAssetIDs }
	if len(override.SupportingAssetIDs) > 0 { target.SupportingAssetIDs = override.SupportingAssetIDs }
	if len(override.ModelAssetIDs) > 0 { target.ModelAssetIDs = override.ModelAssetIDs }
	if len(override.SceneAssetIDs) > 0 { target.SceneAssetIDs = override.SceneAssetIDs }
	if len(override.BrandAssetIDs) > 0 { target.BrandAssetIDs = override.BrandAssetIDs }
	if strings.TrimSpace(override.PresetID) != "" { target.PresetID = override.PresetID }
	if strings.TrimSpace(override.Category) != "" { target.Category = override.Category }
	if strings.TrimSpace(override.TargetChannel) != "" { target.TargetChannel = override.TargetChannel }
	if strings.TrimSpace(override.AspectRatio) != "" { target.AspectRatio = override.AspectRatio }
	if strings.TrimSpace(override.Resolution) != "" { target.Resolution = override.Resolution }
	if override.OutputCount != 0 { target.OutputCount = override.OutputCount }
	if override.ReviewBeforeGeneration { target.ReviewBeforeGeneration = true }
	if strings.TrimSpace(override.UserGoal) != "" { target.UserGoal = override.UserGoal }
	if strings.TrimSpace(override.ModelMode) != "" { target.ModelMode = override.ModelMode }
	if strings.TrimSpace(override.ModelBrief) != "" { target.ModelBrief = override.ModelBrief }
	if strings.TrimSpace(override.SceneBrief) != "" { target.SceneBrief = override.SceneBrief }
	if strings.TrimSpace(override.BrandBrief) != "" { target.BrandBrief = override.BrandBrief }
	if len(override.ProductFacts) > 0 { target.ProductFacts = override.ProductFacts }
	if len(override.Advanced) > 0 { if target.Advanced == nil { target.Advanced = map[string]any{} }; for key, value := range override.Advanced { target.Advanced[key] = value } }
	if strings.TrimSpace(override.ChannelID) != "" { target.ChannelID = override.ChannelID }
	if strings.TrimSpace(override.Model) != "" { target.Model = override.Model }
}
