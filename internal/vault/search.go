package vault

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"kv/internal/retrieval"
)

// SearchResult represents a document found in the vault matching a query.
type SearchResult struct {
	File        string
	Score       int
	Title       string
	Snippet     string
	Tags        []string
	Category    string
	Frontmatter map[string]interface{}
}

// ParseFrontmatter parses YAML-like frontmatter from raw markdown content.
func ParseFrontmatter(raw string) (map[string]interface{}, string) {
	fm := make(map[string]interface{})
	content := raw

	// Normalize newlines
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")

	if strings.HasPrefix(normalized, "---\n") {
		parts := strings.SplitN(normalized, "---\n", 3)
		if len(parts) >= 3 {
			fmText := parts[1]
			content = parts[2]

			lines := strings.Split(fmText, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}

				if strings.Contains(line, ":") {
					kv := strings.SplitN(line, ":", 2)
					key := strings.TrimSpace(kv[0])
					val := strings.TrimSpace(kv[1])
					val = strings.Trim(val, `"'`)

					if key == "title" {
						fm["title"] = val
					} else if key == "category" {
						fm["category"] = val
					} else if key == "tags" {
						// e.g. tags: [tag1, tag2] or tags: tag1, tag2
						if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
							val = strings.Trim(val, "[]")
							var tags []string
							for _, t := range strings.Split(val, ",") {
								tClean := strings.TrimSpace(strings.Trim(t, `"'`))
								if tClean != "" {
									tags = append(tags, tClean)
								}
							}
							fm["tags"] = tags
						} else {
							var tags []string
							for _, t := range strings.Split(val, ",") {
								tClean := strings.TrimSpace(strings.Trim(t, `"'`))
								if tClean != "" {
									tags = append(tags, tClean)
								}
							}
							fm["tags"] = tags
						}
					} else {
						fm[key] = val
					}
				}
			}
		}
	}

	return fm, content
}

func extractTitle(content string, fmTitle string, fileName string) string {
	if fmTitle != "" {
		return fmTitle
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}

	// Fallback to base name of the file
	return filepath.Base(fileName)
}

func scoreTokenMatches(content string, query string) int {
	return retrieval.ScoreField(content, query, retrieval.FieldWeights{TokenCount: 10})
}

func extractSnippet(content string, query string) string {
	lowerContent := strings.ToLower(content)
	lowerQuery := strings.ToLower(query)

	idx := strings.Index(lowerContent, lowerQuery)
	runes := []rune(content)

	if idx == -1 {
		limit := 200
		if len(runes) < limit {
			limit = len(runes)
		}
		snippet := string(runes[:limit])
		snippet = strings.ReplaceAll(snippet, "\n", " ")
		return strings.TrimSpace(snippet) + "..."
	}

	// Convert byte index to rune index
	bytePrefix := content[:idx]
	runeIdx := len([]rune(bytePrefix))

	start := runeIdx - 80
	if start < 0 {
		start = 0
	}

	end := runeIdx + len([]rune(query)) + 120
	if end > len(runes) {
		end = len(runes)
	}

	snippet := string(runes[start:end])
	snippet = strings.ReplaceAll(snippet, "\n", " ")
	snippet = strings.TrimSpace(snippet)

	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(runes) {
		snippet = snippet + "..."
	}

	return snippet
}

// FindMdFiles returns all markdown files recursively in the given directory, skipping ignored ones.
func FindMdFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		name := info.Name()
		if info.IsDir() {
			if name == ".git" || name == "node_modules" || name == ".kv" || name == ".opencode" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.ToLower(filepath.Ext(name)) == ".md" {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// Search searches markdown files in the vault and returns sorted SearchResults.
func Search(vaultPath string, query string, maxResults int) ([]SearchResult, error) {
	files, err := FindMdFiles(vaultPath)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	for _, file := range files {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			continue // skip unreadable files
		}

		rawContent := string(data)
		fm, content := ParseFrontmatter(rawContent)

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

		title := extractTitle(content, fmTitle, file)
		score := scoreTokenMatches(content, query)

		lowerContent := strings.ToLower(content)
		lowerQuery := strings.ToLower(query)

		exactMatch := strings.Contains(lowerContent, lowerQuery)
		if exactMatch {
			score += 5
		}

		if score > 0 || exactMatch {
			results = append(results, SearchResult{
				File:        file,
				Score:       score,
				Title:       title,
				Snippet:     extractSnippet(content, query),
				Tags:        tags,
				Category:    fmCategory,
				Frontmatter: fm,
			})
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > maxResults {
		results = results[:maxResults]
	}

	return results, nil
}
