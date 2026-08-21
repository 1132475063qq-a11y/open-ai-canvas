package agentruntime

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

const ExecutionOutputSchemaVersion = 1

const maxExecutionOutputBytes = 2 << 20
const maxExecutionSummaryRunes = 4000
const maxExecutionArtifactTextBytes = 1 << 20

// ExecutionOutput is the provider-neutral result contract for one Agent/Skill
// invocation. Identity, revision, status, and authority fields are assigned by
// the runtime and are never trusted from model output.
type ExecutionOutput struct {
	SchemaVersion int                      `json:"schemaVersion"`
	Summary       string                   `json:"summary"`
	Artifacts     []ExecutionArtifactDraft `json:"artifacts"`
}

type ExecutionArtifactDraft struct {
	Type        string         `json:"type"`
	Title       string         `json:"title,omitempty"`
	Content     map[string]any `json:"content,omitempty"`
	ContentText string         `json:"contentText,omitempty"`
}

func ParseExecutionOutput(raw string, expectedArtifactTypes []string) (ExecutionOutput, error) {
	var output ExecutionOutput
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return output, errors.New("Agent execution returned an empty response")
	}
	if len(raw) > maxExecutionOutputBytes || !utf8.ValidString(raw) {
		return output, errors.New("Agent execution response is invalid or exceeds 2 MiB")
	}
	raw = unwrapJSONFence(raw)
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(&output); err != nil {
		return ExecutionOutput{}, fmt.Errorf("decode Agent execution output: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ExecutionOutput{}, errors.New("Agent execution output contains trailing JSON values")
	}
	if err := ValidateExecutionOutput(output, expectedArtifactTypes); err != nil {
		return ExecutionOutput{}, err
	}
	return output, nil
}

func ValidateExecutionOutput(output ExecutionOutput, expectedArtifactTypes []string) error {
	if output.SchemaVersion != ExecutionOutputSchemaVersion {
		return fmt.Errorf("unsupported Agent execution schema version %d", output.SchemaVersion)
	}
	output.Summary = strings.TrimSpace(output.Summary)
	if output.Summary == "" || len([]rune(output.Summary)) > maxExecutionSummaryRunes {
		return errors.New("Agent execution summary is empty or exceeds 4000 characters")
	}
	expected := make(map[string]struct{}, len(expectedArtifactTypes))
	for _, artifactType := range expectedArtifactTypes {
		artifactType = strings.TrimSpace(artifactType)
		if artifactType == "" {
			return errors.New("expected Agent execution Artifact type is empty")
		}
		if _, duplicate := expected[artifactType]; duplicate {
			return fmt.Errorf("expected Agent execution Artifact type %s is duplicated", artifactType)
		}
		expected[artifactType] = struct{}{}
	}
	actual := make(map[string]struct{}, len(output.Artifacts))
	for index, artifact := range output.Artifacts {
		artifact.Type = strings.TrimSpace(artifact.Type)
		if _, ok := expected[artifact.Type]; !ok {
			return fmt.Errorf("Agent execution Artifact %d has unexpected type %q", index, artifact.Type)
		}
		if _, duplicate := actual[artifact.Type]; duplicate {
			return fmt.Errorf("Agent execution Artifact type %s is duplicated", artifact.Type)
		}
		actual[artifact.Type] = struct{}{}
		if len([]rune(strings.TrimSpace(artifact.Title))) > 200 {
			return fmt.Errorf("Agent execution Artifact %s title exceeds 200 characters", artifact.Type)
		}
		if len(artifact.ContentText) > maxExecutionArtifactTextBytes {
			return fmt.Errorf("Agent execution Artifact %s text exceeds 1 MiB", artifact.Type)
		}
		if len(artifact.Content) == 0 && strings.TrimSpace(artifact.ContentText) == "" {
			return fmt.Errorf("Agent execution Artifact %s has no content", artifact.Type)
		}
		if containsCredentialField(artifact.Content) {
			return fmt.Errorf("Agent execution Artifact %s contains a credential-like field", artifact.Type)
		}
	}
	for artifactType := range expected {
		if _, ok := actual[artifactType]; !ok {
			return fmt.Errorf("Agent execution output is missing Artifact type %s", artifactType)
		}
	}
	return nil
}

func CanonicalExecutionOutputJSON(output ExecutionOutput) (string, error) {
	encoded, err := json.Marshal(output)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func unwrapJSONFence(raw string) string {
	if !strings.HasPrefix(raw, "```") || !strings.HasSuffix(raw, "```") {
		return raw
	}
	firstNewline := strings.IndexByte(raw, '\n')
	if firstNewline < 0 {
		return raw
	}
	header := strings.TrimSpace(raw[3:firstNewline])
	if header != "" && !strings.EqualFold(header, "json") {
		return raw
	}
	return strings.TrimSpace(raw[firstNewline+1 : len(raw)-3])
}

func containsCredentialField(value any) bool {
	switch item := value.(type) {
	case map[string]any:
		for key, child := range item {
			normalized := strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(key))
			switch normalized {
			case "apikey", "secretkey", "accesskey", "authorization", "accesstoken", "refreshtoken", "bearertoken":
				return true
			}
			if containsCredentialField(child) {
				return true
			}
		}
	case []any:
		for _, child := range item {
			if containsCredentialField(child) {
				return true
			}
		}
	}
	return false
}
