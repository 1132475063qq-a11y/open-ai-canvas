package agentruntime

import "strings"

func (registry *Registry) Agent(id string) (AgentDefinition, bool) {
	id = strings.TrimSpace(id)
	for _, definition := range registry.Agents {
		if definition.ID == id {
			return definition, true
		}
	}
	return AgentDefinition{}, false
}

func (registry *Registry) Skill(id string) (SkillDefinition, bool) {
	id = strings.TrimSpace(id)
	for _, definition := range registry.Skills {
		if definition.ID == id {
			return definition, true
		}
	}
	return SkillDefinition{}, false
}

func (registry *Registry) IntentRoute(id string) (IntentRouteDefinition, bool) {
	id = strings.TrimSpace(id)
	for _, definition := range registry.IntentRoutes {
		if definition.ID == id {
			return definition, true
		}
	}
	return IntentRouteDefinition{}, false
}

func (registry *Registry) HandoffRoute(id string) (HandoffRouteDefinition, bool) {
	id = strings.TrimSpace(id)
	for _, definition := range registry.HandoffRoutes {
		if definition.ID == id {
			return definition, true
		}
	}
	return HandoffRouteDefinition{}, false
}
