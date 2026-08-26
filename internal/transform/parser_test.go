package transform

import "testing"

func TestParseSpecCapturesDeltaRequirementsAndScenarios(t *testing.T) {
	concepts := ParseSpec("## ADDED Requirements\n\n### Requirement: Session ID\nA stable ID.\n\n#### Scenario: First event\n- **WHEN** observed\n- **THEN** persist\n\n## REMOVED Requirements\n### Requirement: Legacy start\n")
	if len(concepts) != 3 {
		t.Fatalf("concepts = %#v", concepts)
	}
	if concepts[0].Kind != ConceptRequirement || concepts[0].Operation != "ADDED" || concepts[0].Body != "A stable ID." {
		t.Fatalf("requirement = %#v", concepts[0])
	}
	if concepts[1].Kind != ConceptScenario || concepts[1].Operation != "ADDED" {
		t.Fatalf("scenario = %#v", concepts[1])
	}
	if concepts[2].Operation != "REMOVED" {
		t.Fatalf("removed = %#v", concepts[2])
	}
}

func TestParseArtifactParsesProposalDesignAndTasks(t *testing.T) {
	proposal := ParseArtifact("proposal", "## Why\nNeed automatic memory.\n## Impact\nNew store.")
	if len(proposal) != 2 || proposal[0].Title != "Why" || proposal[0].Body != "Need automatic memory." {
		t.Fatalf("proposal = %#v", proposal)
	}
	design := ParseArtifact("design", "### 1. Separate domains\nKeep adapters thin.")
	if len(design) != 1 || design[0].Title != "1. Separate domains" {
		t.Fatalf("design = %#v", design)
	}
	tasks := ParseArtifact("tasks", "- [x] 1.1 Complete\n- [ ] 1.2 Pending\n- [-] ignored")
	if len(tasks) != 2 || !tasks[0].Completed || tasks[1].Completed || tasks[1].ID != "1.2" {
		t.Fatalf("tasks = %#v", tasks)
	}
}

func TestParserHandlesUnicodeReorderedAndPartialArtifacts(t *testing.T) {
	spec := "## MODIFIED Requirements\n#### Scenario: Ordem invertida\n- **WHEN** café\n\n### Requirement: Sessão automática\nTexto ✓\n\n### Malformed heading\n"
	concepts := ParseSpec(spec)
	if len(concepts) != 2 || concepts[0].Kind != ConceptScenario || concepts[1].ID != "sessão-automática" {
		t.Fatalf("concepts = %#v", concepts)
	}
	if concepts[0].Operation != "MODIFIED" || concepts[1].Body != "Texto ✓\n\n### Malformed heading" {
		t.Fatalf("parsed partial artifact = %#v", concepts)
	}
}
