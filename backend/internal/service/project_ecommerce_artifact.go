package service

import (
	"encoding/json"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
)

const (
	EcommerceArtifactTypeProductUpload     = "product_upload"
	EcommerceArtifactTypeProductDNA        = "product_dna"
	EcommerceArtifactTypeModelProfile      = "model_profile"
	EcommerceArtifactTypeScenePack         = "scene_pack"
	EcommerceArtifactTypePresetSnapshot    = "preset_snapshot"
	EcommerceArtifactTypeGenerationRequest = "generation_request"
	EcommerceArtifactTypeCreativeDirection = "creative_direction"
	EcommerceArtifactTypeScenePlan         = "scene_plan"
	EcommerceArtifactTypeCreativeShotPlan  = "creative_shot_plan"
	EcommerceArtifactTypeGenerationJob     = "generation_job"
	EcommerceArtifactTypeGeneratedAsset    = "generated_asset"
	EcommerceArtifactTypeQAReport          = "qa_report"
	EcommerceArtifactTypeMotionPlan        = "motion_plan"
	EcommerceArtifactTypeVideoSequence     = "video_sequence"
)

type SaveProjectEcommerceArtifactRequest struct {
	ArtifactKey        string         `json:"artifactKey"`
	ArtifactType       string         `json:"artifactType"`
	SchemaVersion      int            `json:"schemaVersion"`
	Lifecycle          string         `json:"lifecycle"`
	Evidence           string         `json:"evidence"`
	ResponsibleAgentID string         `json:"responsibleAgentId"`
	SkillRef           string         `json:"skillRef"`
	Payload            map[string]any `json:"payload"`
	SourceRefs         []string       `json:"sourceRefs"`
	AuthorityRefs      []string       `json:"authorityRefs"`
}

func (s *Service) SaveProjectEcommerceArtifact(userID string, projectID string, req SaveProjectEcommerceArtifactRequest) (model.EcommerceArtifact, error) {
	project, err := s.repo.ProjectForUser(userID, projectID)
	if err != nil {
		return model.EcommerceArtifact{}, err
	}
	if project.Type != model.ProjectTypeEcommerce {
		return model.EcommerceArtifact{}, BadAuthRequest("电商 Artifact 只能保存到 ecommerce 项目")
	}

	artifactKey := strings.TrimSpace(req.ArtifactKey)
	if artifactKey == "" {
		return model.EcommerceArtifact{}, BadAuthRequest("电商 Artifact key 不能为空")
	}
	artifactType := strings.TrimSpace(req.ArtifactType)
	if !validEcommerceArtifactType(artifactType) {
		return model.EcommerceArtifact{}, BadAuthRequest("不支持的电商 Artifact 类型")
	}
	if req.SchemaVersion < 1 {
		return model.EcommerceArtifact{}, BadAuthRequest("电商 Artifact schemaVersion 必须是正整数")
	}
	lifecycle := strings.TrimSpace(req.Lifecycle)
	if lifecycle == "" {
		lifecycle = "draft"
	}
	if !validEcommerceLifecycle(lifecycle) {
		return model.EcommerceArtifact{}, BadAuthRequest("不支持的电商 Artifact 生命周期")
	}
	evidence := strings.TrimSpace(req.Evidence)
	if evidence == "" {
		evidence = "unknown"
	}
	if !validEcommerceEvidence(evidence) {
		return model.EcommerceArtifact{}, BadAuthRequest("不支持的电商 Artifact 证据等级")
	}
	if len(req.Payload) == 0 {
		return model.EcommerceArtifact{}, BadAuthRequest("电商 Artifact 内容不能为空")
	}
	sourceRefs := normalizeEcommerceRefs(req.SourceRefs)
	if len(sourceRefs) == 0 {
		return model.EcommerceArtifact{}, BadAuthRequest("电商 Artifact 必须关联 source refs")
	}
	payloadJSON, err := json.Marshal(req.Payload)
	if err != nil {
		return model.EcommerceArtifact{}, BadAuthRequest("电商 Artifact 内容格式无效")
	}
	sourceRefsJSON, err := json.Marshal(sourceRefs)
	if err != nil {
		return model.EcommerceArtifact{}, err
	}
	authorityRefsJSON, err := json.Marshal(normalizeEcommerceRefs(req.AuthorityRefs))
	if err != nil {
		return model.EcommerceArtifact{}, err
	}
	now := time.Now()
	artifact := model.EcommerceArtifact{
		ID:                 newID(),
		ProjectID:          projectID,
		ArtifactKey:        artifactKey,
		ArtifactType:       artifactType,
		SchemaVersion:      req.SchemaVersion,
		Lifecycle:          lifecycle,
		Evidence:           evidence,
		ResponsibleAgentID: strings.TrimSpace(req.ResponsibleAgentID),
		SkillRef:           canonicalEcommerceSkillRef(s.ecommerceAgentRegistry, req.SkillRef),
		PayloadJSON:        string(payloadJSON),
		SourceRefsJSON:     string(sourceRefsJSON),
		AuthorityRefsJSON:  string(authorityRefsJSON),
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.repo.SaveEcommerceArtifactVersion(&artifact); err != nil {
		return model.EcommerceArtifact{}, err
	}
	return artifact, nil
}

func (s *Service) ProjectEcommerceArtifacts(userID string, projectID string) ([]model.EcommerceArtifact, error) {
	project, err := s.repo.ProjectForUser(userID, projectID)
	if err != nil {
		return nil, err
	}
	if project.Type != model.ProjectTypeEcommerce {
		return nil, BadAuthRequest("该项目不是 ecommerce 项目")
	}
	return s.repo.ProjectEcommerceArtifacts(projectID)
}

func normalizeEcommerceRefs(refs []string) []string {
	result := make([]string, 0, len(refs))
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		value := strings.TrimSpace(ref)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func validEcommerceArtifactType(value string) bool {
	switch value {
	case EcommerceArtifactTypeProductUpload,
		EcommerceArtifactTypeProductDNA,
		EcommerceArtifactTypeModelProfile,
		EcommerceArtifactTypeScenePack,
		EcommerceArtifactTypePresetSnapshot,
		EcommerceArtifactTypeGenerationRequest,
		EcommerceArtifactTypeCreativeDirection,
		EcommerceArtifactTypeScenePlan,
		EcommerceArtifactTypeCreativeShotPlan,
		EcommerceArtifactTypeGenerationJob,
		EcommerceArtifactTypeGeneratedAsset,
		EcommerceArtifactTypeQAReport,
		EcommerceArtifactTypeMotionPlan,
		EcommerceArtifactTypeVideoSequence:
		return true
	default:
		return false
	}
}

func validEcommerceLifecycle(value string) bool {
	switch value {
	case "draft", "review", "finalized", "superseded", "archived":
		return true
	default:
		return false
	}
}

func validEcommerceEvidence(value string) bool {
	switch value {
	case "recorded", "inferred", "unknown":
		return true
	default:
		return false
	}
}
