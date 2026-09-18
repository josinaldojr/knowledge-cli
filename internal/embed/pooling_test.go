package embed

import (
	"math"
	"testing"
)

func TestMeanPoolIgnoresPadding(t *testing.T) {
	tokens := [][]float32{
		{2, 4}, // [CLS]
		{4, 8}, // real token
		{0, 0}, // padding, must be excluded
	}
	mask := []int64{1, 1, 0}
	got := MeanPool(tokens, mask)
	want := []float32{3, 6}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("MeanPool()[%d] = %v, want %v (got=%v)", i, got[i], want[i], got)
		}
	}
}

func TestMeanPoolEmpty(t *testing.T) {
	if got := MeanPool(nil, nil); got != nil {
		t.Errorf("expected nil for empty input, got %v", got)
	}
}

func TestL2NormalizeUnitLength(t *testing.T) {
	v := L2Normalize([]float32{3, 4})
	if math.Abs(float64(v[0])-0.6) > 1e-6 || math.Abs(float64(v[1])-0.8) > 1e-6 {
		t.Errorf("expected [0.6, 0.8], got %v", v)
	}
}

func TestCosineSimilarityIdenticalVectors(t *testing.T) {
	a := []float32{1, 2, 3}
	if got := CosineSimilarity(a, a); math.Abs(got-1.0) > 1e-6 {
		t.Errorf("expected cosine similarity 1.0 for identical vectors, got %v", got)
	}
}

func TestCosineSimilarityOrthogonalVectors(t *testing.T) {
	if got := CosineSimilarity([]float32{1, 0}, []float32{0, 1}); got != 0 {
		t.Errorf("expected cosine similarity 0 for orthogonal vectors, got %v", got)
	}
}

func TestCosineSimilarityMismatchedLength(t *testing.T) {
	if got := CosineSimilarity([]float32{1, 2}, []float32{1}); got != 0 {
		t.Errorf("expected 0 for mismatched-length vectors, got %v", got)
	}
}
