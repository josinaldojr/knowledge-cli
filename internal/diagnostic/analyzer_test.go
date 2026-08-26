package diagnostic

import (
	"kv/internal/hook"
	"testing"
)

func TestAnalyzeReturnsAdvisoryEvidenceFindings(t *testing.T) {
	findings := Analyze([]Artifact{{LogicalType: "proposal", Contents: "## Why\nScope"}, {LogicalType: "design", Contents: "### 1. Use SQLite\n\n### 2. Do not use SQLite\n"}, {LogicalType: "tasks", Contents: "- [ ] 1.1 unrelated work"}}, hook.ProviderSummary{Version: 1, Claims: []hook.ProviderClaim{{Kind: "requirement", ID: "missing", Status: "added"}}})
	if len(findings) < 4 {
		t.Fatalf("findings = %#v", findings)
	}
}
