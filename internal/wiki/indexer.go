package wiki

import (
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"

	"kv/internal/retrieval"
	"kv/internal/vault"
)

// IndexEntry represents a parsed document in the search index.
type IndexEntry struct {
	Path        string
	Title       string
	Category    string
	Tags        []string
	Content     string
	Frontmatter map[string]interface{}
}

// BuildIndex walks the vault and parses all markdown files to build a search index in memory.
func BuildIndex(vaultPath string) ([]IndexEntry, error) {
	files, err := vault.FindMdFiles(vaultPath)
	if err != nil {
		return nil, err
	}

	var index []IndexEntry
	for _, file := range files {
		// Skip inbox and references archive when building the general wiki index,
		// so we only query compiled canonical pages.
		rel, err := filepath.Rel(vaultPath, file)
		if err == nil {
			if strings.HasPrefix(rel, "00-inbox") || strings.HasPrefix(rel, "10-references/archive") {
				continue
			}
		}

		data, err := ioutil.ReadFile(file)
		if err != nil {
			continue // Skip unreadable files
		}

		rawContent := string(data)
		fm, content := vault.ParseFrontmatter(rawContent)

		fmTitle, _ := fm["title"].(string)
		fmCategory, _ := fm["category"].(string)
		var tags []string
		if rawTags, ok := fm["tags"]; ok {
			if list, isList := rawTags.([]string); isList {
				tags = list
			} else if str, isStr := rawTags.(string); isStr {
				tags = []string{str}
			}
		}

		// Extract title using vault helper or fallback
		title := fmTitle
		if title == "" {
			// Find first H1
			lines := strings.Split(content, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "# ") {
					title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
					break
				}
			}
			if title == "" {
				title = filepath.Base(file)
			}
		}

		index = append(index, IndexEntry{
			Path:        file,
			Title:       title,
			Category:    fmCategory,
			Tags:        tags,
			Content:     content,
			Frontmatter: fm,
		})
	}

	return index, nil
}

// SearchResult represents a scored match from the search index.
type SearchResult struct {
	Entry IndexEntry
	Score int
}

// SearchIndex scores all index entries against a query and returns the top N results.
func SearchIndex(index []IndexEntry, query string, topN int) []SearchResult {
	if len(index) == 0 || strings.TrimSpace(query) == "" {
		return nil
	}

	var scoredResults []SearchResult

	for _, entry := range index {
		// 1. Exact phrase match in title/content (highest priority), plus
		// 2. per-token matches in title, tags, and content. Single-letter
		// tokens are skipped to avoid spurious matches.
		score := retrieval.ScoreField(entry.Title, query, retrieval.FieldWeights{PhraseMatch: 150, TokenHit: 40, MinTokenLength: 2})
		score += retrieval.ScoreField(entry.Content, query, retrieval.FieldWeights{PhraseMatch: 50, TokenCount: 5, MinTokenLength: 2})
		for _, tag := range entry.Tags {
			score += retrieval.ScoreField(tag, query, retrieval.FieldWeights{TokenHit: 30, MinTokenLength: 2})
		}

		if score > 0 {
			scoredResults = append(scoredResults, SearchResult{
				Entry: entry,
				Score: score,
			})
		}
	}

	// Sort results by score descending
	sort.Slice(scoredResults, func(i, j int) bool {
		return scoredResults[i].Score > scoredResults[j].Score
	})

	if len(scoredResults) > topN {
		scoredResults = scoredResults[:topN]
	}

	return scoredResults
}
