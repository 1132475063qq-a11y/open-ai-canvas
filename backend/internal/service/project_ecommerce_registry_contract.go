package service

import (
	"fmt"
	"strings"

	"infinite-canvas/backend/internal/agentruntime"
	"infinite-canvas/backend/internal/model"
)

// validateEcommerceRunRegistryPin protects mutating production operations
// from silently interpreting an older Run with a newer Agent/Skill manifest.
// Legacy runs created before registry pinning have all three fields empty and
// remain readable/mutable for backwards compatibility; partially populated
// pins are rejected because they cannot establish an auditable identity.
func (s *Service) validateEcommerceRunRegistryPin(run model.EcommerceProductionRun) error {
	if s == nil || s.ecommerceAgentRegistry == nil {
		return fmt.Errorf("Ecommerce Agent Runtime 注册表未初始化")
	}
	pin := []string{strings.TrimSpace(run.RegistryID), strings.TrimSpace(run.RegistryVersion), strings.TrimSpace(run.RegistryDigest)}
	if pin[0] == "" && pin[1] == "" && pin[2] == "" {
		return nil
	}
	if pin[0] == "" || pin[1] == "" || pin[2] == "" {
		return Conflict("该 Ecommerce Run 的注册表 pin 不完整，请重新创建规划")
	}
	registry := s.ecommerceAgentRegistry
	if pin[0] != registry.ID || pin[1] != registry.Version || pin[2] != registry.SourceDigest {
		return Conflict("该 Ecommerce Run 使用了已过期的 Agent/Skill 注册表，请重新规划")
	}
	return nil
}

// Generic Ecommerce Agent Runs are created only after the registry is pinned.
// Unlike the older paid-production Run, an unpinned runtime execution cannot
// be resumed safely because its executor and output contract are registry
// facts. Keep legacy paid Runs readable, but fence an Ecommerce Agent worker
// before it prepares or completes an Attempt when the pin is absent or stale.
func (s *Service) validateEcommerceAgentRuntimeRunRegistryPin(run model.AgentRuntimeRun) error {
	if s == nil || s.ecommerceAgentRegistry == nil {
		return fmt.Errorf("Ecommerce Agent Runtime 注册表未初始化")
	}
	pin := []string{strings.TrimSpace(run.RegistryID), strings.TrimSpace(run.RegistryVersion), strings.TrimSpace(run.RegistryDigest)}
	registry := s.ecommerceAgentRegistry
	if pin[0] == "" || pin[1] == "" || pin[2] == "" || pin[0] != registry.ID || pin[1] != registry.Version || pin[2] != registry.SourceDigest {
		return Conflict("该 Ecommerce Agent Run 使用了缺失或已过期的 Agent/Skill 注册表，不能继续执行")
	}
	return nil
}

// canonicalEcommerceSkillRef converts a frontend/display Skill reference to
// the immutable ID and version owned by the Ecommerce registry. Unknown refs
// are preserved for user-owned custom presets; they are not treated as
// executable built-in Skills until a custom registry is introduced.
func canonicalEcommerceSkillRef(registry *agentruntime.Registry, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" || registry == nil {
		return ref
	}
	base, versionHint := ref, ""
	if at := strings.LastIndexByte(ref, '@'); at > 0 {
		base, versionHint = strings.TrimSpace(ref[:at]), strings.TrimSpace(ref[at+1:])
	}
	base = strings.TrimPrefix(base, "ecommerce.skill.")
	canonical, ok := registry.CanonicalSkillID(base)
	if !ok {
		return ref
	}
	for _, skill := range registry.Skills {
		if skill.ID != canonical {
			continue
		}
		// A legacy major-only suffix such as @1 remains readable, but the
		// persisted contract always records the exact manifest version.
		if versionHint == "" || versionHint == skill.Version || versionHint == strings.SplitN(skill.Version, ".", 2)[0] {
			return canonical + "@" + skill.Version
		}
		return ref
	}
	return ref
}

func (s *Service) canonicalizeEcommerceArtifacts(artifacts []model.EcommerceArtifact) {
	if s == nil || s.ecommerceAgentRegistry == nil {
		return
	}
	for index := range artifacts {
		artifacts[index].SkillRef = canonicalEcommerceSkillRef(s.ecommerceAgentRegistry, artifacts[index].SkillRef)
	}
}
