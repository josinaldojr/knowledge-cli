package retrieval

import "sort"

// rrfK is the standard Reciprocal Rank Fusion damping constant (Cormack et
// al., 2009's recommended default). It only affects how quickly the
// contribution of a rank falls off; results stay in the same order for any
// reasonable value, so it is not exposed as a tuning knob.
const rrfK = 60.0

// FuseRanked combines any number of independently ranked result lists (e.g.
// a BM25/lexical ranking and a cosine-similarity/semantic ranking) into one
// ranking using Reciprocal Rank Fusion. RRF only needs each input's rank
// order, not its raw scores, which sidesteps the problem of BM25 and cosine
// similarity living on incomparable scales. A document's fused score is the
// sum of 1/(rrfK+rank) over every list it appears in, so ranking well in
// more than one list compounds.
func FuseRanked(rankings ...[]Scored) []Scored {
	fused := map[string]float64{}
	for _, ranking := range rankings {
		for rank, item := range ranking {
			fused[item.ID] += 1.0 / (rrfK + float64(rank+1))
		}
	}
	results := make([]Scored, 0, len(fused))
	for id, score := range fused {
		results = append(results, Scored{ID: id, Score: score})
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].ID < results[j].ID
	})
	return results
}
