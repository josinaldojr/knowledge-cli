package embed

import "math"

// MeanPool reduces a [seqLen][hiddenSize] token embedding matrix (a BERT
// model's last_hidden_state for one sequence) to a single sentence vector by
// averaging over non-padding positions, per attentionMask. This is the
// pooling strategy sentence-transformers models (including
// all-MiniLM-L6-v2) are trained and documented to use; using CLS-only
// pooling or unmasked averaging instead would silently degrade retrieval
// quality even though it would still compile and run.
func MeanPool(tokenEmbeddings [][]float32, attentionMask []int64) []float32 {
	if len(tokenEmbeddings) == 0 {
		return nil
	}
	hiddenSize := len(tokenEmbeddings[0])
	sum := make([]float64, hiddenSize)
	var count float64
	for i, vec := range tokenEmbeddings {
		if i < len(attentionMask) && attentionMask[i] == 0 {
			continue
		}
		for j, v := range vec {
			sum[j] += float64(v)
		}
		count++
	}
	if count == 0 {
		count = 1
	}
	pooled := make([]float32, hiddenSize)
	for i, v := range sum {
		pooled[i] = float32(v / count)
	}
	return pooled
}

// L2Normalize scales v to unit length in place and returns it, so that a dot
// product between two normalized vectors equals their cosine similarity.
// sentence-transformers embeddings are normalized this way by convention.
func L2Normalize(v []float32) []float32 {
	var sumSquares float64
	for _, x := range v {
		sumSquares += float64(x) * float64(x)
	}
	norm := math.Sqrt(sumSquares)
	if norm == 0 {
		return v
	}
	for i, x := range v {
		v[i] = float32(float64(x) / norm)
	}
	return v
}

// CosineSimilarity returns the cosine similarity of two equal-length
// vectors. If both inputs are already L2-normalized, this reduces to their
// dot product.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
