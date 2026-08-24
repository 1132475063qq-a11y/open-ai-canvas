package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

// validateEcommerceRunEvidence is called immediately before a quote can be
// issued or consumed. It fences stale/hand-edited plan rows and verifies that
// a target-adapted Run still points at its immutable locked revision.
func (s *Service) validateEcommerceRunEvidence(userID, projectID string, run model.EcommerceProductionRun, requireTarget bool) error {
	if run.ProjectID != projectID || run.UserID != userID {
		return NotFound("Ecommerce Run 不属于当前项目")
	}
	for label, id := range map[string]string{
		"product DNA": run.ProductDNAArtifactID,
		"scene pack": run.ScenePackArtifactID,
		"preset snapshot": run.PresetSnapshotArtifactID,
		"generation request": run.GenerationRequestArtifactID,
	} {
		if strings.TrimSpace(id) == "" {
			return Conflict("Ecommerce " + label + " evidence 缺失，不能报价或提交")
		}
		artifact, err := s.repo.EcommerceArtifactForProjectByID(projectID, id)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Conflict("Ecommerce " + label + " evidence 不存在，不能报价或提交")
		}
		if err != nil {
			return err
		}
		if artifact.Lifecycle != "finalized" || artifact.SchemaVersion < 1 || artifact.Evidence == "unknown" {
			return Conflict("Ecommerce " + label + " evidence 尚未 finalized")
		}
		var sourceRefs []string
		if err := json.Unmarshal([]byte(artifact.SourceRefsJSON), &sourceRefs); err != nil || len(normalizeEcommerceRefs(sourceRefs)) == 0 {
			return Conflict("Ecommerce " + label + " 缺少 source refs")
		}
	}
	metadata := map[string]any{}
	if strings.TrimSpace(run.AdvancedJSON) != "" {
		if err := json.Unmarshal([]byte(run.AdvancedJSON), &metadata); err != nil {
			return Conflict("Ecommerce Run target evidence 格式无效")
		}
	}
	targetRevisionID := strings.TrimSpace(fmt.Sprint(metadata["targetArtifactRevisionId"]))
	if targetRevisionID == "" || targetRevisionID == "<nil>" {
		if requireTarget {
			return Conflict("Ecommerce Run 缺少锁定的 target Artifact evidence")
		}
		return nil
	}
	artifact, revision, err := s.repo.ProductionArtifactRevisionForUser(userID, targetRevisionID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Conflict("Ecommerce target Artifact revision 不存在")
	}
	if err != nil {
		return err
	}
	if artifact.ProjectID != projectID || artifact.Domain != EcommerceTargetDomain || revision.Status != model.ProductionArtifactStatusLocked {
		return Conflict("Ecommerce target Artifact 必须仍是当前项目的 LOCKED revision")
	}
	if digest := strings.TrimSpace(fmt.Sprint(metadata["targetArtifactDigest"])); digest != "" && digest != "<nil>" && digest != revision.ContentDigest {
		return Conflict("Ecommerce target Artifact digest 已变化，请重新适配")
	}
	if fmt.Sprint(metadata["targetRegistryId"]) != EcommerceAgentRegistryID || fmt.Sprint(metadata["targetRegistryVersion"]) != EcommerceAgentRegistryVersion || fmt.Sprint(metadata["targetRegistryDigest"]) != ecommerceTargetRegistry.SourceDigest {
		return Conflict("Ecommerce target Runtime registry pin 已过期")
	}
	return nil
}
