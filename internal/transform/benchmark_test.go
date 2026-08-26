package transform

import "testing"

func BenchmarkAfterHookDeltaProcessing(b *testing.B) {
	before := make([]Concept, 100)
	after := make([]Concept, 100)
	for index := range before {
		before[index] = Concept{Kind: ConceptRequirement, ID: string(rune(index)), Title: "Requirement", Body: "The system shall retain evidence."}
		after[index] = before[index]
		if index%10 == 0 {
			after[index].Body = "The system shall retain verified evidence."
		}
	}
	b.ReportAllocs()
	for b.Loop() {
		_ = CalculateDelta(before, after)
	}
}
