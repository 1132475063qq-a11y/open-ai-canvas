package agentruntime

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestFilmRegistryLoadsCompleteExecutableDefinitions(t *testing.T) {
	registry, err := LoadFilmRegistry()
	if err != nil {
		t.Fatalf("LoadFilmRegistry() error = %v", err)
	}
	if registry.ID != "film-agent-team" || registry.Version != "1.3.1" || registry.Domain != "film" {
		t.Fatalf("unexpected registry identity: %#v", registry)
	}
	if got := len(registry.Agents); got != 9 {
		t.Fatalf("agents = %d, want 9", got)
	}
	if got := len(registry.Skills); got != 17 {
		t.Fatalf("skills = %d, want 17", got)
	}
	if got := len(registry.IntentRoutes); got != 15 {
		t.Fatalf("intent routes = %d, want 15", got)
	}
	if got := len(registry.HandoffRoutes); got != 11 {
		t.Fatalf("handoff routes = %d, want 11", got)
	}
	if len(registry.SourceDigest) != 64 {
		t.Fatalf("source digest length = %d, want 64", len(registry.SourceDigest))
	}
	for _, agent := range registry.Agents {
		if agent.Name != agent.ID || agent.Description == "" || agent.DeveloperInstructions == "" || len(agent.SourceDigest) != 64 {
			t.Fatalf("agent source was not compiled: %#v", agent)
		}
	}
	for _, skill := range registry.Skills {
		if skill.Name != skill.ID || skill.Description == "" || skill.Instructions == "" || len(skill.SourceDigest) != 64 {
			t.Fatalf("skill source was not compiled: %#v", skill)
		}
	}
}

func TestFilmRegistryHasExactAgentSkillAndRouteIDs(t *testing.T) {
	registry, err := LoadFilmRegistry()
	if err != nil {
		t.Fatalf("LoadFilmRegistry() error = %v", err)
	}
	assertIDs(t, agentIDs(registry), []string{
		"ai_production_supervisor", "director_storyboard_artist", "film_project_lead",
		"narrative_screenwriter", "quality_control_editor", "short_drama_planner",
		"sound_designer", "tvc_creative_director", "visual_development_designer",
	})
	assertIDs(t, skillIDs(registry), []string{
		"ai-production-feasibility-review", "ai-video-shot-prompt", "character-acting-system",
		"character-visual-design", "cinedance-video-director", "continuity-check",
		"director-storyboard", "final-film-quality-review", "lira-image-prompts",
		"scene-asset-design", "screenwriter", "script-review", "short-drama-planning",
		"sound-design", "story-type-engine", "tvc", "worldbuilding-management",
	})
	assertIDs(t, intentRouteIDs(registry), numberedIDs("IR-", 15))
	assertIDs(t, handoffRouteIDs(registry), numberedIDs("HR-", 11))
}

func TestFilmRegistryCanonicalizesOnlyDeclaredArtifactAliases(t *testing.T) {
	registry, err := LoadFilmRegistry()
	if err != nil {
		t.Fatalf("LoadFilmRegistry() error = %v", err)
	}
	if got, ok := registry.CanonicalArtifactType("worldbuilding-bible"); !ok || got != "canon-bible" {
		t.Fatalf("canonical alias = %q, %v", got, ok)
	}
	if got, ok := registry.CanonicalArtifactType("storyboard"); !ok || got != "storyboard" {
		t.Fatalf("canonical type = %q, %v", got, ok)
	}
	if got, ok := registry.CanonicalArtifactType("made-up-result"); ok || got != "" {
		t.Fatalf("unknown artifact unexpectedly resolved to %q", got)
	}
}

func TestRegistryValidationRejectsBrokenExecutableReferences(t *testing.T) {
	registry, err := LoadFilmRegistry()
	if err != nil {
		t.Fatalf("LoadFilmRegistry() error = %v", err)
	}
	registry.IntentRoutes[0].OutputArtifactTypes = []string{"made-up-result"}
	if err := registry.Validate(); err == nil || !strings.Contains(err.Error(), "unknown artifact type") {
		t.Fatalf("Validate() error = %v, want unknown artifact type", err)
	}
}

func TestRegistryValidationRequiresContractsForMultipleIntentSkills(t *testing.T) {
	registry, err := LoadFilmRegistry()
	if err != nil {
		t.Fatalf("LoadFilmRegistry() error = %v", err)
	}
	for index := range registry.IntentRoutes {
		if registry.IntentRoutes[index].ID == "IR-03" {
			registry.IntentRoutes[index].StepOutputArtifactTypes = nil
			break
		}
	}
	if err := registry.Validate(); err == nil || !strings.Contains(err.Error(), "per-step output Artifact types") {
		t.Fatalf("Validate() error = %v, want missing multi-Skill output contract", err)
	}
}

func agentIDs(registry *Registry) []string {
	result := make([]string, 0, len(registry.Agents))
	for _, item := range registry.Agents {
		result = append(result, item.ID)
	}
	return result
}

func skillIDs(registry *Registry) []string {
	result := make([]string, 0, len(registry.Skills))
	for _, item := range registry.Skills {
		result = append(result, item.ID)
	}
	return result
}

func intentRouteIDs(registry *Registry) []string {
	result := make([]string, 0, len(registry.IntentRoutes))
	for _, item := range registry.IntentRoutes {
		result = append(result, item.ID)
	}
	return result
}

func handoffRouteIDs(registry *Registry) []string {
	result := make([]string, 0, len(registry.HandoffRoutes))
	for _, item := range registry.HandoffRoutes {
		result = append(result, item.ID)
	}
	return result
}

func numberedIDs(prefix string, count int) []string {
	result := make([]string, 0, count)
	for index := 1; index <= count; index++ {
		result = append(result, prefix+string(rune('0'+index/10))+string(rune('0'+index%10)))
	}
	return result
}

func assertIDs(t *testing.T, got []string, want []string) {
	t.Helper()
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs = %v, want %v", got, want)
	}
}
