package retrieval

import (
	"math"
	"sort"
)

// BM25 tuning constants (Okapi BM25 defaults).
const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

// BM25Index is a corpus-level lexical index. Unlike ScoreField, which scores
// one field against a query in isolation, BM25 needs corpus statistics
// (document frequency, average document length) computed up front, so
// documents are added once via NewBM25Index and then scored many times.
type BM25Index struct {
	docs      []bm25Doc
	docFreq   map[string]int
	avgDocLen float64
}

type bm25Doc struct {
	id       string
	termFreq map[string]int
	length   int
}

// NewBM25Index builds a BM25 index from a set of (id, text) documents. Text
// is tokenized with Words, the same whitespace tokenizer vault and wiki
// search already use.
func NewBM25Index(docs map[string]string) *BM25Index {
	idx := &BM25Index{docFreq: map[string]int{}}
	var totalLen int
	for id, text := range docs {
		terms := Words(text)
		tf := map[string]int{}
		for _, term := range terms {
			tf[term]++
		}
		for term := range tf {
			idx.docFreq[term]++
		}
		idx.docs = append(idx.docs, bm25Doc{id: id, termFreq: tf, length: len(terms)})
		totalLen += len(terms)
	}
	if len(idx.docs) > 0 {
		idx.avgDocLen = float64(totalLen) / float64(len(idx.docs))
	}
	return idx
}

// Scored is a document ranked against a query.
type Scored struct {
	ID    string
	Score float64
}

// Search scores every indexed document against query and returns matches
// (score > 0) sorted by descending score, most relevant first.
func (idx *BM25Index) Search(query string) []Scored {
	queryTerms := Words(query)
	if len(queryTerms) == 0 || len(idx.docs) == 0 {
		return nil
	}
	n := float64(len(idx.docs))

	idf := make(map[string]float64, len(queryTerms))
	for _, term := range queryTerms {
		if _, ok := idf[term]; ok {
			continue
		}
		df := float64(idx.docFreq[term])
		// BM25's standard IDF variant, floored at ~0 so very common terms
		// never push a document's score negative.
		idf[term] = math.Log(1 + (n-df+0.5)/(df+0.5))
	}

	var results []Scored
	for _, doc := range idx.docs {
		score := 0.0
		for _, term := range queryTerms {
			tf := float64(doc.termFreq[term])
			if tf == 0 {
				continue
			}
			norm := 1 - bm25B + bm25B*(float64(doc.length)/idx.avgDocLen)
			score += idf[term] * (tf * (bm25K1 + 1)) / (tf + bm25K1*norm)
		}
		if score > 0 {
			results = append(results, Scored{ID: doc.id, Score: score})
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].ID < results[j].ID
	})
	return results
}
