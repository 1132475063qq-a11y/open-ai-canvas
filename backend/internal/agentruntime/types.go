package agentruntime

type Registry struct {
	SchemaVersion   string                   `json:"schemaVersion"`
	ID              string                   `json:"id"`
	Version         string                   `json:"version"`
	Domain          string                   `json:"domain"`
	ExpectedCounts  RegistryCounts           `json:"expectedCounts"`
	Agents          []AgentDefinition        `json:"agents"`
	Skills          []SkillDefinition        `json:"skills"`
	SkillAliases    map[string]string        `json:"skillAliases"`
	ArtifactTypes   []ArtifactTypeDefinition `json:"artifactTypes"`
	ArtifactAliases map[string]string        `json:"artifactAliases"`
	IntentRoutes    []IntentRouteDefinition  `json:"intentRoutes"`
	HandoffRoutes   []HandoffRouteDefinition `json:"handoffRoutes"`
	SourceDigest    string                   `json:"-"`
}

type RegistryCounts struct {
	Agents        int `json:"agents"`
	Skills        int `json:"skills"`
	IntentRoutes  int `json:"intentRoutes"`
	HandoffRoutes int `json:"handoffRoutes"`
}

type AgentDefinition struct {
	ID                    string   `json:"id"`
	InstructionPath       string   `json:"instructionPath"`
	SkillIDs              []string `json:"skillIds"`
	SandboxMode           string   `json:"sandboxMode,omitempty"`
	Name                  string   `json:"-"`
	Description           string   `json:"-"`
	DeveloperInstructions string   `json:"-"`
	SourceDigest          string   `json:"-"`
}

type SkillDefinition struct {
	ID              string   `json:"id"`
	Version         string   `json:"version"`
	InstructionPath string   `json:"instructionPath"`
	OwnerAgentIDs   []string `json:"ownerAgentIds"`
	Name            string   `json:"-"`
	Description     string   `json:"-"`
	Instructions    string   `json:"-"`
	SourceDigest    string   `json:"-"`
}

type ArtifactTypeDefinition struct {
	ID          string `json:"id"`
	Domain      string `json:"domain"`
	Responsible string `json:"responsible"`
}

type IntentRouteDefinition struct {
	ID                         string     `json:"id"`
	Name                       string     `json:"name"`
	TriggerPhrases             []string   `json:"triggerPhrases"`
	PrimaryAgentID             string     `json:"primaryAgentId"`
	CandidateAgentIDs          []string   `json:"candidateAgentIds,omitempty"`
	SkillIDs                   []string   `json:"skillIds"`
	StepOutputArtifactTypes    [][]string `json:"stepOutputArtifactTypes,omitempty"`
	RequiredInputArtifactTypes []string   `json:"requiredInputArtifactTypes"`
	OptionalInputArtifactTypes []string   `json:"optionalInputArtifactTypes"`
	OutputArtifactTypes        []string   `json:"outputArtifactTypes"`
	RequiresDisambiguation     bool       `json:"requiresDisambiguation,omitempty"`
}

type HandoffRouteDefinition struct {
	ID                          string     `json:"id"`
	Name                        string     `json:"name"`
	FromAgentIDs                []string   `json:"fromAgentIds"`
	ToAgentIDs                  []string   `json:"toAgentIds"`
	SkillIDs                    []string   `json:"skillIds"`
	InputArtifactTypes          []string   `json:"inputArtifactTypes"`
	RequiredInputArtifactGroups [][]string `json:"requiredInputArtifactGroups,omitempty"`
	InputResolutionMode         string     `json:"inputResolutionMode"`
	OutputArtifactTypes         []string   `json:"outputArtifactTypes"`
	MessageType                 string     `json:"messageType"`
	ExecutionMode               string     `json:"executionMode"`
	RequiresLockedInput         bool       `json:"requiresLockedInput"`
	Fanout                      string     `json:"fanout"`
}
