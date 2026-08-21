package service

import (
	"errors"

	"infinite-canvas/backend/internal/agentruntime"
)

type AgentRuntimeRegistrySummary struct {
	ID                string `json:"id"`
	Version           string `json:"version"`
	Domain            string `json:"domain"`
	SourceDigest      string `json:"sourceDigest"`
	AgentCount        int    `json:"agentCount"`
	SkillCount        int    `json:"skillCount"`
	IntentRouteCount  int    `json:"intentRouteCount"`
	HandoffRouteCount int    `json:"handoffRouteCount"`
}

type FilmAgentCatalogAgent struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	SkillIDs    []string `json:"skillIds"`
	SandboxMode string   `json:"sandboxMode,omitempty"`
}

type FilmAgentCatalogSkill struct {
	ID            string   `json:"id"`
	Version       string   `json:"version"`
	Description   string   `json:"description"`
	OwnerAgentIDs []string `json:"ownerAgentIds"`
}

type FilmAgentRuntimeCatalog struct {
	Registry      AgentRuntimeRegistrySummary           `json:"registry"`
	Agents        []FilmAgentCatalogAgent               `json:"agents"`
	Skills        []FilmAgentCatalogSkill               `json:"skills"`
	IntentRoutes  []agentruntime.IntentRouteDefinition  `json:"intentRoutes"`
	HandoffRoutes []agentruntime.HandoffRouteDefinition `json:"handoffRoutes"`
	ArtifactTypes []agentruntime.ArtifactTypeDefinition `json:"artifactTypes"`
}

func (s *Service) FilmAgentRuntimeRegistrySummary() (AgentRuntimeRegistrySummary, error) {
	if err := s.ValidateRuntime(); err != nil {
		return AgentRuntimeRegistrySummary{}, err
	}
	if s.filmAgentRegistry == nil {
		return AgentRuntimeRegistrySummary{}, errors.New("Film Agent Runtime 注册表未初始化")
	}
	return AgentRuntimeRegistrySummary{
		ID: s.filmAgentRegistry.ID, Version: s.filmAgentRegistry.Version, Domain: s.filmAgentRegistry.Domain,
		SourceDigest: s.filmAgentRegistry.SourceDigest, AgentCount: len(s.filmAgentRegistry.Agents),
		SkillCount: len(s.filmAgentRegistry.Skills), IntentRouteCount: len(s.filmAgentRegistry.IntentRoutes),
		HandoffRouteCount: len(s.filmAgentRegistry.HandoffRoutes),
	}, nil
}

func (s *Service) FilmAgentRuntimeCatalog(userID string, projectID string) (FilmAgentRuntimeCatalog, error) {
	if _, err := s.filmProjectForUser(userID, projectID); err != nil {
		return FilmAgentRuntimeCatalog{}, err
	}
	summary, err := s.FilmAgentRuntimeRegistrySummary()
	if err != nil {
		return FilmAgentRuntimeCatalog{}, err
	}
	catalog := FilmAgentRuntimeCatalog{
		Registry:      summary,
		Agents:        make([]FilmAgentCatalogAgent, 0, len(s.filmAgentRegistry.Agents)),
		Skills:        make([]FilmAgentCatalogSkill, 0, len(s.filmAgentRegistry.Skills)),
		IntentRoutes:  append([]agentruntime.IntentRouteDefinition(nil), s.filmAgentRegistry.IntentRoutes...),
		HandoffRoutes: append([]agentruntime.HandoffRouteDefinition(nil), s.filmAgentRegistry.HandoffRoutes...),
		ArtifactTypes: append([]agentruntime.ArtifactTypeDefinition(nil), s.filmAgentRegistry.ArtifactTypes...),
	}
	for _, definition := range s.filmAgentRegistry.Agents {
		catalog.Agents = append(catalog.Agents, FilmAgentCatalogAgent{
			ID: definition.ID, Description: definition.Description,
			SkillIDs: append([]string(nil), definition.SkillIDs...), SandboxMode: definition.SandboxMode,
		})
	}
	for _, definition := range s.filmAgentRegistry.Skills {
		catalog.Skills = append(catalog.Skills, FilmAgentCatalogSkill{
			ID: definition.ID, Version: definition.Version, Description: definition.Description,
			OwnerAgentIDs: append([]string(nil), definition.OwnerAgentIDs...),
		})
	}
	return catalog, nil
}
