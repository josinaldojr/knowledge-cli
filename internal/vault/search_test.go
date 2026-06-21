package vault

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	// Simple tags and title
	raw := `---
title: "My Great Note"
tags: [tag1, tag2]
category: project
---
# Actual Content
This is the body of the markdown.`

	fm, content := ParseFrontmatter(raw)

	if fm["title"] != "My Great Note" {
		t.Errorf("expected title 'My Great Note', got '%v'", fm["title"])
	}
	if fm["category"] != "project" {
		t.Errorf("expected category 'project', got '%v'", fm["category"])
	}
	tags, ok := fm["tags"].([]string)
	if !ok || len(tags) != 2 || tags[0] != "tag1" || tags[1] != "tag2" {
		t.Errorf("expected tags [tag1, tag2], got '%v'", fm["tags"])
	}
	if !strings.Contains(content, "Actual Content") {
		t.Errorf("expected content to contain body, got '%s'", content)
	}

	// Plain title and comma separated tags
	raw2 := `---
title: Simple Note
tags: tag3, tag4
---
Body content`
	fm2, _ := ParseFrontmatter(raw2)
	if fm2["title"] != "Simple Note" {
		t.Errorf("expected title 'Simple Note', got '%v'", fm2["title"])
	}
	tags2, ok := fm2["tags"].([]string)
	if !ok || len(tags2) != 2 || tags2[0] != "tag3" || tags2[1] != "tag4" {
		t.Errorf("expected tags [tag3, tag4], got '%v'", fm2["tags"])
	}
}

func TestScoreTokenMatches(t *testing.T) {
	content := "This is a backend development project. The backend uses Go."
	
	// single match
	score := scoreTokenMatches(content, "development")
	if score != 10 {
		t.Errorf("expected score 10, got %d", score)
	}

	// multiple matches
	score2 := scoreTokenMatches(content, "backend")
	if score2 != 20 {
		t.Errorf("expected score 20, got %d", score2)
	}

	// case insensitivity and multiple tokens
	score3 := scoreTokenMatches(content, "BACKEND DEVELOPMENT")
	if score3 != 30 {
		t.Errorf("expected score 30, got %d", score3)
	}
}

func TestExtractSnippet(t *testing.T) {
	content := "A very long document that talks about many backend concepts. For example, database connections and concurrency in Go. Here is the search term: backend-concurrency. Following this, there are more explanations."
	
	// Term found
	snippet := extractSnippet(content, "backend-concurrency")
	if !strings.Contains(snippet, "backend-concurrency") {
		t.Errorf("expected snippet to contain term, got '%s'", snippet)
	}

	// Term not found (fallback to beginning)
	snippetNotFound := extractSnippet(content, "non-existent-term")
	if !strings.HasPrefix(snippetNotFound, "A very long") {
		t.Errorf("expected fallback snippet to start with start of content, got '%s'", snippetNotFound)
	}
}

func TestSearch(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "search-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test vault structure
	createFakeVault(t, tmpDir)

	// Add some documents
	doc1 := `---
title: Backend Guidelines
tags: [backend, go]
---
# Backend
Rules for writing backend APIs in Go.`
	err = ioutil.WriteFile(filepath.Join(tmpDir, "backend-api.md"), []byte(doc1), 0644)
	if err != nil {
		t.Fatalf("failed to write doc1: %v", err)
	}

	doc2 := `---
title: Frontend Setup
tags: [frontend, react]
---
# Frontend UI
Setup guides for React applications.`
	err = ioutil.WriteFile(filepath.Join(tmpDir, "frontend-ui.md"), []byte(doc2), 0644)
	if err != nil {
		t.Fatalf("failed to write doc2: %v", err)
	}

	// Search for backend
	res, err := Search(tmpDir, "backend", 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(res) != 1 {
		t.Errorf("expected 1 result, got %d", len(res))
	} else {
		if res[0].Title != "Backend Guidelines" {
			t.Errorf("expected title 'Backend Guidelines', got '%s'", res[0].Title)
		}
	}
}
