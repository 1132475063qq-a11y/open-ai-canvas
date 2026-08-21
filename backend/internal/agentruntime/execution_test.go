package agentruntime

import (
	"strings"
	"testing"
)

func TestParseExecutionOutputValidatesExactArtifactContract(t *testing.T) {
	raw := "```json\n" + `{
  "schemaVersion": 1,
  "summary": "A causally complete first draft.",
  "artifacts": [
    {"type":"script","title":"Episode 1","content":{"scenes":[{"id":"scene-1"}]},"contentText":"INT. STATION - NIGHT"}
  ]
}` + "\n```"
	output, err := ParseExecutionOutput(raw, []string{"script"})
	if err != nil {
		t.Fatalf("ParseExecutionOutput() error = %v", err)
	}
	if output.SchemaVersion != 1 || output.Artifacts[0].Type != "script" {
		t.Fatalf("unexpected output: %#v", output)
	}
	canonical, err := CanonicalExecutionOutputJSON(output)
	if err != nil || !strings.Contains(canonical, `"schemaVersion":1`) {
		t.Fatalf("canonical output = %q, error = %v", canonical, err)
	}
}

func TestParseExecutionOutputRejectsSchemaAndArtifactDrift(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "unknown field", raw: `{"schemaVersion":1,"summary":"ok","artifacts":[],"status":"locked"}`, want: "unknown field"},
		{name: "missing expected", raw: `{"schemaVersion":1,"summary":"ok","artifacts":[]}`, want: "missing Artifact type script"},
		{name: "unexpected", raw: `{"schemaVersion":1,"summary":"ok","artifacts":[{"type":"storyboard","contentText":"x"}]}`, want: "unexpected type"},
		{name: "duplicate", raw: `{"schemaVersion":1,"summary":"ok","artifacts":[{"type":"script","contentText":"x"},{"type":"script","contentText":"y"}]}`, want: "duplicated"},
		{name: "credential", raw: `{"schemaVersion":1,"summary":"ok","artifacts":[{"type":"script","content":{"api_key":"invented"}}]}`, want: "credential-like"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseExecutionOutput(test.raw, []string{"script"})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ParseExecutionOutput() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestParseExecutionOutputAllowsSummaryOnlyIntermediateStep(t *testing.T) {
	output, err := ParseExecutionOutput(`{"schemaVersion":1,"summary":"Brief analysis completed.","artifacts":[]}`, nil)
	if err != nil || len(output.Artifacts) != 0 {
		t.Fatalf("summary-only output = %#v, error = %v", output, err)
	}
}
