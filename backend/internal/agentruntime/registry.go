package agentruntime

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/pelletier/go-toml/v2"
)

const filmAssetRoot = "assets/film/v1.3.1"
const filmManifestPath = filmAssetRoot + "/registry.json"

//go:embed assets/film/v1.3.1
var registryAssets embed.FS

var definitionIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
var intentRouteIDPattern = regexp.MustCompile(`^IR-(0[1-9]|1[0-5])$`)
var handoffRouteIDPattern = regexp.MustCompile(`^HR-(0[1-9]|1[01])$`)

type agentSource struct {
	Name                  string `toml:"name"`
	Description           string `toml:"description"`
	DeveloperInstructions string `toml:"developer_instructions"`
	SandboxMode           string `toml:"sandbox_mode"`
}

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func LoadFilmRegistry() (*Registry, error) {
	data, err := registryAssets.ReadFile(filmManifestPath)
	if err != nil {
		return nil, fmt.Errorf("read film registry: %w", err)
	}
	var registry Registry
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&registry); err != nil {
		return nil, fmt.Errorf("decode film registry: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, errors.New("decode film registry: trailing JSON value")
	}
	digests := []string{digestBytes(data)}
	for index := range registry.Agents {
		definition := &registry.Agents[index]
		sourceData, sourcePath, err := readVersionedSource(definition.InstructionPath)
		if err != nil {
			return nil, fmt.Errorf("agent %s: %w", definition.ID, err)
		}
		var source agentSource
		if err := toml.Unmarshal(sourceData, &source); err != nil {
			return nil, fmt.Errorf("agent %s TOML: %w", definition.ID, err)
		}
		if source.Name != definition.ID {
			return nil, fmt.Errorf("agent %s source declares %q", definition.ID, source.Name)
		}
		if strings.TrimSpace(source.Description) == "" || strings.TrimSpace(source.DeveloperInstructions) == "" {
			return nil, fmt.Errorf("agent %s source is incomplete", definition.ID)
		}
		if definition.SandboxMode != "" && source.SandboxMode != definition.SandboxMode {
			return nil, fmt.Errorf("agent %s sandbox mode mismatch", definition.ID)
		}
		definition.Name = source.Name
		definition.Description = source.Description
		definition.DeveloperInstructions = source.DeveloperInstructions
		definition.SourceDigest = digestBytes(sourceData)
		digests = append(digests, sourcePath+":"+definition.SourceDigest)
	}
	for index := range registry.Skills {
		definition := &registry.Skills[index]
		sourceData, sourcePath, err := readVersionedSource(definition.InstructionPath)
		if err != nil {
			return nil, fmt.Errorf("skill %s: %w", definition.ID, err)
		}
		metadata, instructions, err := parseSkillSource(sourceData)
		if err != nil {
			return nil, fmt.Errorf("skill %s: %w", definition.ID, err)
		}
		if metadata.Name != definition.ID {
			return nil, fmt.Errorf("skill %s source declares %q", definition.ID, metadata.Name)
		}
		treeDigest, err := sourceTreeDigest(path.Dir(sourcePath))
		if err != nil {
			return nil, fmt.Errorf("skill %s source tree: %w", definition.ID, err)
		}
		definition.Name = metadata.Name
		definition.Description = metadata.Description
		definition.Instructions = instructions
		definition.SourceDigest = treeDigest
		digests = append(digests, sourcePath+":"+treeDigest)
	}
	if err := registry.Validate(); err != nil {
		return nil, err
	}
	sort.Strings(digests)
	registry.SourceDigest = digestBytes([]byte(strings.Join(digests, "\n")))
	return &registry, nil
}

func (registry *Registry) Validate() error {
	if registry.SchemaVersion != "1.0" {
		return fmt.Errorf("unsupported agent registry schema %q", registry.SchemaVersion)
	}
	if !definitionIDPattern.MatchString(registry.ID) || !versionPattern.MatchString(registry.Version) {
		return errors.New("agent registry identity or version is invalid")
	}
	if registry.Domain != "film" && registry.Domain != "ecommerce" {
		return fmt.Errorf("unsupported agent registry domain %q", registry.Domain)
	}
	if registry.ExpectedCounts != (RegistryCounts{
		Agents: len(registry.Agents), Skills: len(registry.Skills),
		IntentRoutes: len(registry.IntentRoutes), HandoffRoutes: len(registry.HandoffRoutes),
	}) {
		return errors.New("agent registry counts do not match the manifest")
	}

	agents := make(map[string]AgentDefinition, len(registry.Agents))
	for _, agent := range registry.Agents {
		if !definitionIDPattern.MatchString(agent.ID) || agents[agent.ID].ID != "" {
			return fmt.Errorf("invalid or duplicate agent %q", agent.ID)
		}
		if err := rejectDuplicateStrings("agent "+agent.ID+" Skill", agent.SkillIDs); err != nil {
			return err
		}
		if agent.Name != agent.ID || strings.TrimSpace(agent.DeveloperInstructions) == "" {
			return fmt.Errorf("agent %s has no verified source instructions", agent.ID)
		}
		agents[agent.ID] = agent
	}

	skills := make(map[string]SkillDefinition, len(registry.Skills))
	for _, skill := range registry.Skills {
		if !definitionIDPattern.MatchString(skill.ID) || skills[skill.ID].ID != "" {
			return fmt.Errorf("invalid or duplicate skill %q", skill.ID)
		}
		if !versionPattern.MatchString(skill.Version) || skill.Name != skill.ID || strings.TrimSpace(skill.Instructions) == "" {
			return fmt.Errorf("skill %s has no verified source instructions or version", skill.ID)
		}
		if len(skill.OwnerAgentIDs) == 0 {
			return fmt.Errorf("skill %s has no owner", skill.ID)
		}
		if err := rejectDuplicateStrings("skill "+skill.ID+" owner", skill.OwnerAgentIDs); err != nil {
			return err
		}
		for _, ownerID := range skill.OwnerAgentIDs {
			agent, ok := agents[ownerID]
			if !ok {
				return fmt.Errorf("skill %s references unknown owner %s", skill.ID, ownerID)
			}
			if !contains(agent.SkillIDs, skill.ID) {
				return fmt.Errorf("skill %s ownership is missing from agent %s", skill.ID, ownerID)
			}
		}
		skills[skill.ID] = skill
	}
	for _, agent := range registry.Agents {
		for _, skillID := range uniqueStrings(agent.SkillIDs) {
			skill, ok := skills[skillID]
			if !ok {
				return fmt.Errorf("agent %s references unknown skill %s", agent.ID, skillID)
			}
			if !contains(skill.OwnerAgentIDs, agent.ID) {
				return fmt.Errorf("agent %s is not an owner of skill %s", agent.ID, skillID)
			}
		}
	}

	artifacts := make(map[string]ArtifactTypeDefinition, len(registry.ArtifactTypes))
	for _, artifact := range registry.ArtifactTypes {
		if !definitionIDPattern.MatchString(artifact.ID) || artifacts[artifact.ID].ID != "" {
			return fmt.Errorf("invalid or duplicate artifact type %q", artifact.ID)
		}
		if artifact.Domain == "" {
			return fmt.Errorf("artifact type %s has no domain", artifact.ID)
		}
		if _, ok := agents[artifact.Responsible]; !ok {
			return fmt.Errorf("artifact type %s references unknown responsible %s", artifact.ID, artifact.Responsible)
		}
		artifacts[artifact.ID] = artifact
	}
	for alias, canonical := range registry.ArtifactAliases {
		if !definitionIDPattern.MatchString(alias) {
			return fmt.Errorf("invalid artifact alias %q", alias)
		}
		if _, ok := artifacts[canonical]; !ok {
			return fmt.Errorf("artifact alias %s targets unknown type %s", alias, canonical)
		}
	}

	intentRoutes := make(map[string]struct{}, len(registry.IntentRoutes))
	for _, route := range registry.IntentRoutes {
		if !intentRouteIDPattern.MatchString(route.ID) {
			return fmt.Errorf("invalid intent route %q", route.ID)
		}
		if _, duplicate := intentRoutes[route.ID]; duplicate {
			return fmt.Errorf("duplicate intent route %s", route.ID)
		}
		intentRoutes[route.ID] = struct{}{}
		if strings.TrimSpace(route.Name) == "" || len(route.TriggerPhrases) == 0 {
			return fmt.Errorf("intent route %s has no name or triggers", route.ID)
		}
		if _, ok := agents[route.PrimaryAgentID]; !ok {
			return fmt.Errorf("intent route %s references unknown primary agent %s", route.ID, route.PrimaryAgentID)
		}
		if len(route.CandidateAgentIDs) > 0 && !contains(route.CandidateAgentIDs, route.PrimaryAgentID) {
			return fmt.Errorf("intent route %s candidates exclude its primary agent", route.ID)
		}
		if err := validateAgentIDs(route.ID, route.CandidateAgentIDs, agents); err != nil {
			if len(route.CandidateAgentIDs) > 0 {
				return err
			}
		}
		if err := validateSkillIDs(route.ID, route.SkillIDs, skills, true); err != nil {
			return err
		}
		if len(route.StepOutputArtifactTypes) == 0 {
			if len(route.SkillIDs) > 1 {
				return fmt.Errorf("intent route %s must declare per-step output Artifact types for multiple Skills", route.ID)
			}
		} else {
			if len(route.StepOutputArtifactTypes) != len(route.SkillIDs) {
				return fmt.Errorf("intent route %s per-step output Artifact contract count does not match Skill count", route.ID)
			}
			for index, outputs := range route.StepOutputArtifactTypes {
				if err := validateArtifactIDs(fmt.Sprintf("%s Step %d", route.ID, index), outputs, artifacts, true); err != nil {
					return err
				}
			}
			if !sameStrings(route.StepOutputArtifactTypes[len(route.StepOutputArtifactTypes)-1], route.OutputArtifactTypes) {
				return fmt.Errorf("intent route %s final per-step output contract must match route output Artifact types", route.ID)
			}
		}
		routeAgentIDs := append([]string{route.PrimaryAgentID}, route.CandidateAgentIDs...)
		for _, skillID := range route.SkillIDs {
			if !intersects(skills[skillID].OwnerAgentIDs, routeAgentIDs) {
				return fmt.Errorf("intent route %s has no selected Agent that owns skill %s", route.ID, skillID)
			}
		}
		if err := validateArtifactIDs(route.ID, append(append([]string{}, route.RequiredInputArtifactTypes...), route.OptionalInputArtifactTypes...), artifacts, false); err != nil {
			return err
		}
		if err := validateArtifactIDs(route.ID, route.OutputArtifactTypes, artifacts, true); err != nil {
			return err
		}
		if route.RequiresDisambiguation && len(route.CandidateAgentIDs) < 2 {
			return fmt.Errorf("intent route %s cannot disambiguate fewer than two agents", route.ID)
		}
	}

	handoffRoutes := make(map[string]struct{}, len(registry.HandoffRoutes))
	for _, route := range registry.HandoffRoutes {
		if !handoffRouteIDPattern.MatchString(route.ID) {
			return fmt.Errorf("invalid handoff route %q", route.ID)
		}
		if _, duplicate := handoffRoutes[route.ID]; duplicate {
			return fmt.Errorf("duplicate handoff route %s", route.ID)
		}
		handoffRoutes[route.ID] = struct{}{}
		if strings.TrimSpace(route.Name) == "" {
			return fmt.Errorf("handoff route %s has no name", route.ID)
		}
		if err := validateAgentIDs(route.ID, route.FromAgentIDs, agents); err != nil {
			return err
		}
		if err := validateAgentIDs(route.ID, route.ToAgentIDs, agents); err != nil {
			return err
		}
		if route.ExecutionMode != "agent" && route.ExecutionMode != "orchestration" {
			return fmt.Errorf("handoff route %s has invalid execution mode", route.ID)
		}
		if err := validateSkillIDs(route.ID, route.SkillIDs, skills, route.ExecutionMode == "agent"); err != nil {
			return err
		}
		for _, skillID := range route.SkillIDs {
			if !intersects(skills[skillID].OwnerAgentIDs, route.ToAgentIDs) {
				return fmt.Errorf("handoff route %s has no receiving Agent that owns skill %s", route.ID, skillID)
			}
		}
		if err := validateArtifactIDs(route.ID, route.InputArtifactTypes, artifacts, true); err != nil {
			return err
		}
		if err := validateHandoffInputContract(route, artifacts); err != nil {
			return err
		}
		if err := validateArtifactIDs(route.ID, route.OutputArtifactTypes, artifacts, true); err != nil {
			return err
		}
		if route.MessageType != "request" && route.MessageType != "notification" && route.MessageType != "completion_notification" {
			return fmt.Errorf("handoff route %s has invalid message type", route.ID)
		}
		if route.Fanout != "single" && route.Fanout != "parallel" && route.Fanout != "dynamic" {
			return fmt.Errorf("handoff route %s has invalid fanout", route.ID)
		}
	}
	return nil
}

func validateHandoffInputContract(route HandoffRouteDefinition, artifacts map[string]ArtifactTypeDefinition) error {
	switch route.InputResolutionMode {
	case "static", "project_start", "project_completion":
	default:
		return fmt.Errorf("handoff route %s has invalid input resolution mode", route.ID)
	}
	if route.InputResolutionMode == "project_completion" {
		if route.ExecutionMode != "orchestration" || len(route.RequiredInputArtifactGroups) != 0 {
			return fmt.Errorf("handoff route %s project completion inputs must be resolved by orchestration", route.ID)
		}
		return nil
	}
	if len(route.RequiredInputArtifactGroups) == 0 {
		return fmt.Errorf("handoff route %s has no required input Artifact groups", route.ID)
	}
	flattened := make([]string, 0, len(route.InputArtifactTypes))
	seen := make(map[string]struct{}, len(route.InputArtifactTypes))
	for groupIndex, group := range route.RequiredInputArtifactGroups {
		if len(group) == 0 {
			return fmt.Errorf("handoff route %s input group %d is empty", route.ID, groupIndex)
		}
		if err := validateArtifactIDs(route.ID, group, artifacts, true); err != nil {
			return err
		}
		for _, artifactType := range group {
			if _, duplicate := seen[artifactType]; duplicate {
				return fmt.Errorf("handoff route %s repeats input Artifact type %s across groups", route.ID, artifactType)
			}
			seen[artifactType] = struct{}{}
			flattened = append(flattened, artifactType)
		}
	}
	if len(flattened) != len(route.InputArtifactTypes) {
		return fmt.Errorf("handoff route %s grouped inputs do not cover its input Artifact catalog", route.ID)
	}
	for _, artifactType := range route.InputArtifactTypes {
		if _, ok := seen[artifactType]; !ok {
			return fmt.Errorf("handoff route %s grouped inputs omit Artifact type %s", route.ID, artifactType)
		}
	}
	return nil
}

func (registry *Registry) CanonicalArtifactType(value string) (string, bool) {
	value = strings.TrimSpace(value)
	for _, artifact := range registry.ArtifactTypes {
		if artifact.ID == value {
			return value, true
		}
	}
	canonical, ok := registry.ArtifactAliases[value]
	return canonical, ok
}

func readVersionedSource(relative string) ([]byte, string, error) {
	clean := path.Clean(relative)
	if clean == "." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
		return nil, "", errors.New("source path escapes the registry")
	}
	full := path.Join(filmAssetRoot, clean)
	if !strings.HasPrefix(full, filmAssetRoot+"/source/") {
		return nil, "", errors.New("source path is outside the versioned source directory")
	}
	data, err := registryAssets.ReadFile(full)
	if err != nil {
		return nil, "", err
	}
	return data, full, nil
}

func parseSkillSource(data []byte) (skillFrontmatter, string, error) {
	const marker = "\n---\n"
	text := string(data)
	if !strings.HasPrefix(text, "---\n") {
		return skillFrontmatter{}, "", errors.New("missing YAML frontmatter")
	}
	end := strings.Index(text[4:], marker)
	if end < 0 {
		return skillFrontmatter{}, "", errors.New("unterminated YAML frontmatter")
	}
	end += 4
	var metadata skillFrontmatter
	if err := yaml.Unmarshal([]byte(text[4:end]), &metadata); err != nil {
		return skillFrontmatter{}, "", err
	}
	instructions := strings.TrimSpace(text[end+len(marker):])
	if strings.TrimSpace(metadata.Name) == "" || strings.TrimSpace(metadata.Description) == "" || instructions == "" {
		return skillFrontmatter{}, "", errors.New("incomplete skill source")
	}
	return metadata, instructions, nil
}

func sourceTreeDigest(root string) (string, error) {
	var entries []string
	err := fs.WalkDir(registryAssets, root, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := registryAssets.ReadFile(filePath)
		if err != nil {
			return err
		}
		entries = append(entries, filePath+":"+digestBytes(data))
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(entries) < 2 {
		return "", errors.New("skill source tree has no supporting files")
	}
	sort.Strings(entries)
	return digestBytes([]byte(strings.Join(entries, "\n"))), nil
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validateAgentIDs(owner string, ids []string, agents map[string]AgentDefinition) error {
	if len(ids) == 0 {
		return fmt.Errorf("%s has no Agent references", owner)
	}
	if err := rejectDuplicateStrings(owner+" Agent", ids); err != nil {
		return err
	}
	for _, id := range ids {
		if _, ok := agents[id]; !ok {
			return fmt.Errorf("%s references unknown agent %s", owner, id)
		}
	}
	return nil
}

func validateSkillIDs(owner string, ids []string, skills map[string]SkillDefinition, required bool) error {
	if required && len(ids) == 0 {
		return fmt.Errorf("%s has no Skill references", owner)
	}
	if err := rejectDuplicateStrings(owner+" Skill", ids); err != nil {
		return err
	}
	for _, id := range ids {
		if _, ok := skills[id]; !ok {
			return fmt.Errorf("%s references unknown skill %s", owner, id)
		}
	}
	return nil
}

func validateArtifactIDs(owner string, ids []string, artifacts map[string]ArtifactTypeDefinition, required bool) error {
	if required && len(ids) == 0 {
		return fmt.Errorf("%s has no Artifact references", owner)
	}
	if err := rejectDuplicateStrings(owner+" Artifact", ids); err != nil {
		return err
	}
	for _, id := range ids {
		if _, ok := artifacts[id]; !ok {
			return fmt.Errorf("%s references unknown artifact type %s", owner, id)
		}
	}
	return nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func intersects(left []string, right []string) bool {
	for _, value := range left {
		if contains(right, value) {
			return true
		}
	}
	return false
}

func sameStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func rejectDuplicateStrings(owner string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s references an empty ID", owner)
		}
		if _, duplicate := seen[value]; duplicate {
			return fmt.Errorf("%s references duplicate ID %s", owner, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}
