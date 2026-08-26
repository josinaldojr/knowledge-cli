package transform

import (
	"testing"

	"kv/internal/hook"
)

func TestNormalizeProviderSummaryRetainsVerifiedClaimsAndObjectiveOmissions(t *testing.T) {
	added := Concept{Kind: ConceptRequirement, ID: "automatic-session", Title: "Automatic session"}
	removed := Concept{Kind: ConceptRequirement, ID: "manual-start", Title: "Manual start"}
	deltas := []Delta{
		{Kind: DeltaAdded, After: &added},
		{Kind: DeltaRemoved, Before: &removed},
	}
	summary := hook.ProviderSummary{Version: 1, Claims: []hook.ProviderClaim{
		{Kind: "requirement", ID: "automatic-session", Status: "added", Statement: "Added automatic registration."},
		{Kind: "decision", ID: "not-in-artifact", Status: "added", Statement: "Unsupported claim."},
	}}
	normalized := NormalizeProviderSummary(summary, deltas)
	if len(normalized.Supported) != 1 || normalized.Supported[0].ID != "automatic-session" {
		t.Fatalf("supported = %#v", normalized.Supported)
	}
	if len(normalized.Rejected) != 1 || normalized.Rejected[0].ID != "not-in-artifact" {
		t.Fatalf("rejected = %#v", normalized.Rejected)
	}
	if len(normalized.Objective) != 1 || normalized.Objective[0].Kind != DeltaRemoved {
		t.Fatalf("objective = %#v", normalized.Objective)
	}
}

func TestNormalizeProviderSummaryRejectsClaimsAbsentFromAfterRevision(t *testing.T) {
	before := Concept{Kind: ConceptRequirement, ID: "removed", Title: "Removed"}
	deltas := []Delta{{Kind: DeltaRemoved, Before: &before}}
	summary := hook.ProviderSummary{Version: 1, Claims: []hook.ProviderClaim{{Kind: "requirement", ID: "removed", Status: "added"}}}
	normalized := NormalizeProviderSummary(summary, deltas)
	if len(normalized.Rejected) != 1 || len(normalized.Objective) != 1 {
		t.Fatalf("normalized = %#v", normalized)
	}
}
