package transform

import "kv/internal/hook"

// NormalizedSummary separates provider claims that are supported by objective
// OpenSpec deltas from unsupported claims, and preserves unclaimed deltas as
// objective omissions for downstream diagnostics and memory creation.
type NormalizedSummary struct {
	Supported []hook.ProviderClaim `json:"supported"`
	Objective []Delta              `json:"objective_omissions"`
	Rejected  []hook.ProviderClaim `json:"rejected"`
}

func NormalizeProviderSummary(summary hook.ProviderSummary, deltas []Delta) NormalizedSummary {
	result := NormalizedSummary{}
	matched := make([]bool, len(deltas))
	for _, claim := range summary.Claims {
		index := matchingDelta(claim, deltas, matched)
		if index < 0 {
			result.Rejected = append(result.Rejected, claim)
			continue
		}
		matched[index] = true
		result.Supported = append(result.Supported, claim)
	}
	for index, delta := range deltas {
		if !matched[index] && delta.Kind != DeltaUnchanged {
			result.Objective = append(result.Objective, delta)
		}
	}
	return result
}

func matchingDelta(claim hook.ProviderClaim, deltas []Delta, matched []bool) int {
	for index, delta := range deltas {
		if matched[index] || string(delta.Kind) != claim.Status {
			continue
		}
		concept := delta.After
		if concept == nil {
			concept = delta.Before
		}
		if concept != nil && string(concept.Kind) == claim.Kind && concept.ID == claim.ID {
			return index
		}
	}
	return -1
}
