package transform

import "testing"

func TestCalculateDeltaClassifiesConceptLifecycle(t *testing.T) {
	before := []Concept{
		{Kind: ConceptRequirement, ID: "same", Title: "Same", Body: "body"},
		{Kind: ConceptRequirement, ID: "changed", Title: "Changed", Body: "old"},
		{Kind: ConceptRequirement, ID: "removed", Title: "Removed"},
		{Kind: ConceptDecision, ID: "old-name", Title: "Old", Body: "shared"},
		{Kind: ConceptTask, ID: "1.1", Title: "Task", Completed: false},
	}
	after := []Concept{
		{Kind: ConceptRequirement, ID: "same", Title: "Same", Body: "body"},
		{Kind: ConceptRequirement, ID: "changed", Title: "Changed", Body: "new"},
		{Kind: ConceptRequirement, ID: "added", Title: "Added"},
		{Kind: ConceptDecision, ID: "new-name", Title: "New", Body: "shared"},
		{Kind: ConceptTask, ID: "1.1", Title: "Task", Completed: true},
	}
	deltas := CalculateDelta(before, after)
	want := map[DeltaKind]int{DeltaUnchanged: 1, DeltaModified: 2, DeltaRemoved: 1, DeltaAdded: 1, DeltaRenamed: 1}
	got := map[DeltaKind]int{}
	for _, delta := range deltas {
		got[delta.Kind]++
	}
	for kind, count := range want {
		if got[kind] != count {
			t.Fatalf("%s = %d, want %d; deltas = %#v", kind, got[kind], count, deltas)
		}
	}
}
