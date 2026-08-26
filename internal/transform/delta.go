package transform

import "strings"

type DeltaKind string

const (
	DeltaAdded     DeltaKind = "added"
	DeltaModified  DeltaKind = "modified"
	DeltaRemoved   DeltaKind = "removed"
	DeltaRenamed   DeltaKind = "renamed"
	DeltaUnchanged DeltaKind = "unchanged"
)

type Delta struct {
	Kind   DeltaKind `json:"kind"`
	Before *Concept  `json:"before,omitempty"`
	After  *Concept  `json:"after,omitempty"`
}

// CalculateDelta compares parser output without relying on line numbers. A
// rename is recognized only when otherwise-identical concepts have a changed
// stable ID; ambiguous unmatched concepts stay explicit additions/removals.
func CalculateDelta(before, after []Concept) []Delta {
	beforeByKey := make(map[string]Concept, len(before))
	afterByKey := make(map[string]Concept, len(after))
	for _, concept := range before {
		beforeByKey[conceptKey(concept)] = concept
	}
	for _, concept := range after {
		afterByKey[conceptKey(concept)] = concept
	}
	var result []Delta
	var removed, added []Concept
	for _, concept := range before {
		key := conceptKey(concept)
		afterConcept, found := afterByKey[key]
		if !found {
			removed = append(removed, concept)
			continue
		}
		beforeCopy, afterCopy := concept, afterConcept
		kind := DeltaUnchanged
		if !sameConcept(concept, afterConcept) {
			kind = DeltaModified
		}
		result = append(result, Delta{Kind: kind, Before: &beforeCopy, After: &afterCopy})
	}
	for _, concept := range after {
		if _, found := beforeByKey[conceptKey(concept)]; !found {
			added = append(added, concept)
		}
	}
	usedAdded := make([]bool, len(added))
	for _, old := range removed {
		renamed := -1
		for index, newer := range added {
			if !usedAdded[index] && renameCandidate(old, newer) {
				renamed = index
				break
			}
		}
		oldCopy := old
		if renamed >= 0 {
			usedAdded[renamed] = true
			newCopy := added[renamed]
			result = append(result, Delta{Kind: DeltaRenamed, Before: &oldCopy, After: &newCopy})
			continue
		}
		result = append(result, Delta{Kind: DeltaRemoved, Before: &oldCopy})
	}
	for index, concept := range added {
		if !usedAdded[index] {
			copy := concept
			result = append(result, Delta{Kind: DeltaAdded, After: &copy})
		}
	}
	return result
}

func conceptKey(concept Concept) string { return string(concept.Kind) + "\x00" + concept.ID }

func sameConcept(before, after Concept) bool {
	return before.Title == after.Title && strings.TrimSpace(before.Body) == strings.TrimSpace(after.Body) && before.Operation == after.Operation && before.Completed == after.Completed
}

func renameCandidate(before, after Concept) bool {
	body := strings.TrimSpace(before.Body)
	return body != "" && before.Kind == after.Kind && before.ID != after.ID && body == strings.TrimSpace(after.Body) && before.Completed == after.Completed
}
