package wiki

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kv/internal/fsutil"
)

type mockTransport struct {
	roundTripFunc func(req *http.Request) (*http.Response, error)
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.roundTripFunc(req)
}

func createTestVault(t *testing.T) (string, func()) {
	tmpDir, err := ioutil.TempDir("", "kv-wiki-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	// Create directories
	folders := []string{
		"00-inbox",
		"01-global",
		"04-systems",
		"05-decisions",
		"10-references/archive",
	}

	for _, folder := range folders {
		err := fsutil.EnsureDir(filepath.Join(tmpDir, folder))
		if err != nil {
			cleanup()
			t.Fatalf("failed to create folder %s: %v", folder, err)
		}
	}

	// Create marker
	err = ioutil.WriteFile(filepath.Join(tmpDir, ".kv-vault"), []byte(`{"type":"knowledge-vault"}`), 0644)
	if err != nil {
		cleanup()
		t.Fatalf("failed to write vault marker: %v", err)
	}

	return tmpDir, cleanup
}

func TestBuildAndSearchIndex(t *testing.T) {
	vaultPath, cleanup := createTestVault(t)
	defer cleanup()

	// Create a canonical system doc
	sysDoc := filepath.Join(vaultPath, "04-systems", "payments.md")
	sysContent := `---
title: "Payments System"
category: "architecture"
tags: [payment, stripe, checkout]
---

# Payments System

This system handles checkout and subscription services using Stripe API.
The database schema is updated.
`
	if err := ioutil.WriteFile(sysDoc, []byte(sysContent), 0644); err != nil {
		t.Fatalf("failed to write test doc: %v", err)
	}

	// Create a document in inbox (should be ignored by indexer)
	inboxDoc := filepath.Join(vaultPath, "00-inbox", "inbox-note.md")
	inboxContent := `---
title: "Inbox Note"
---
This is a draft about oauth.
`
	if err := ioutil.WriteFile(inboxDoc, []byte(inboxContent), 0644); err != nil {
		t.Fatalf("failed to write inbox doc: %v", err)
	}

	// Build index
	idx, err := BuildIndex(vaultPath)
	if err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	// Verify size (only payments.md should be indexed, inbox-note.md ignored)
	if len(idx) != 1 {
		t.Errorf("expected 1 document in index, got %d", len(idx))
	} else {
		entry := idx[0]
		if entry.Title != "Payments System" {
			t.Errorf("expected title 'Payments System', got '%s'", entry.Title)
		}
		if entry.Category != "architecture" {
			t.Errorf("expected category 'architecture', got '%s'", entry.Category)
		}
		if len(entry.Tags) != 3 || entry.Tags[0] != "payment" {
			t.Errorf("expected tag 'payment', got %v", entry.Tags)
		}
	}

	// Test Search
	results := SearchIndex(idx, "stripe checkout", 5)
	if len(results) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(results))
	}
	if results[0].Entry.Title != "Payments System" {
		t.Errorf("expected search result 'Payments System', got '%s'", results[0].Entry.Title)
	}
	if results[0].Score < 50 {
		t.Errorf("expected high score for query matching Title/Content/Tags, got %d", results[0].Score)
	}

	// Query with no matches
	results = SearchIndex(idx, "non-existent-token", 5)
	if len(results) != 0 {
		t.Errorf("expected 0 results for non-matching query, got %d", len(results))
	}
}

func TestCompileAndLinkMock(t *testing.T) {
	vaultPath, cleanup := createTestVault(t)
	defer cleanup()

	// 1. Create a canonical page so we have something in the index to link to
	sysDoc := filepath.Join(vaultPath, "04-systems", "payments.md")
	sysContent := `---
title: "Payments System"
category: "architecture"
tags: [payment]
---

# Payments System
Handles stripe checkouts.
`
	if err := ioutil.WriteFile(sysDoc, []byte(sysContent), 0644); err != nil {
		t.Fatalf("failed to write test doc: %v", err)
	}

	// 2. Create raw file in inbox
	inboxDoc := filepath.Join(vaultPath, "00-inbox", "nova-auth.md")
	inboxContent := `Adicionando autenticação via JWT para oPayments System.`
	if err := ioutil.WriteFile(inboxDoc, []byte(inboxContent), 0644); err != nil {
		t.Fatalf("failed to write inbox note: %v", err)
	}

	// Setup mock transport
	mockTrans := &mockTransport{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			// Read request body to inspect prompt
			bodyBytes, err := ioutil.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			reqText := string(bodyBytes)

			var respText string
			if strings.Contains(reqText, "decidir se ela deve") {
				// Decision
				respText = `{"action": "CREATE", "target_path": "04-systems/auth-flow.md", "reason": "Novo fluxo de autenticação JWT"}`
			} else if strings.Contains(reqText, "escrever uma NOVA página markdown") {
				// Compilation/generation
				respText = `---
status: canon
created_at: 2026-06-25
updated_at: 2026-06-25
title: "Auth Flow"
category: "systems"
tags: [auth, jwt]
---

# Auth Flow
Este é o fluxo de autenticação. Ele interage com o Payments System.`
			} else if strings.Contains(reqText, "adicionar links relativos de markdown") {
				// Auto linking
				respText = `---
status: canon
created_at: 2026-06-25
updated_at: 2026-06-25
title: "Auth Flow"
category: "systems"
tags: [auth, jwt]
---

# Auth Flow
Este é o fluxo de autenticação. Ele interage com o [Payments System](payments.md).`
			} else {
				respText = "mock general response"
			}

			respJSON := fmt.Sprintf(`{
				"candidates": [
					{
						"content": {
							"parts": [
								{
									"text": %q
								}
							]
						}
					}
				]
			}`, respText)

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(bytes.NewBufferString(respJSON)),
				Header:     make(http.Header),
			}, nil
		},
	}

	// Swap DefaultClient Transport
	oldTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = mockTrans
	defer func() {
		http.DefaultClient.Transport = oldTransport
	}()

	// Instantiate wiki Client
	client := &Client{
		APIKey: "mock-key",
		Model:  "mock-model",
	}

	// Execute CompileInbox
	count, err := CompileInbox(vaultPath, client)
	if err != nil {
		t.Fatalf("CompileInbox failed: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 file to be compiled, got %d", count)
	}

	// Verify target file exists and is linked
	targetFile := filepath.Join(vaultPath, "04-systems", "auth-flow.md")
	if !fsutil.IsFile(targetFile) {
		t.Errorf("compiled target file was not created at %s", targetFile)
	} else {
		contentBytes, _ := ioutil.ReadFile(targetFile)
		content := string(contentBytes)
		if !strings.Contains(content, "[Payments System](payments.md)") {
			t.Errorf("expected target file to be linked to Payments System, got content:\n%s", content)
		}
	}

	// Verify inbox file is moved to archive
	if fsutil.IsFile(inboxDoc) {
		t.Errorf("inbox file was not removed from inbox")
	}

	archiveDoc := filepath.Join(vaultPath, "10-references", "archive", "nova-auth.md")
	if !fsutil.IsFile(archiveDoc) {
		t.Errorf("inbox file was not moved to references archive: %s", archiveDoc)
	}
}

func TestAskWikiMock(t *testing.T) {
	vaultPath, cleanup := createTestVault(t)
	defer cleanup()

	// Create a document
	docPath := filepath.Join(vaultPath, "04-systems", "payments.md")
	docContent := `---
title: "Payments System"
---
Handles credit card processing.
`
	if err := ioutil.WriteFile(docPath, []byte(docContent), 0644); err != nil {
		t.Fatalf("failed to write test doc: %v", err)
	}

	mockTrans := &mockTransport{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			respJSON := `{
				"candidates": [
					{
						"content": {
							"parts": [
								{
									"text": "O sistema de pagamentos suporta processamento de cartão de crédito."
								}
							]
						}
					}
				]
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(bytes.NewBufferString(respJSON)),
				Header:     make(http.Header),
			}, nil
		},
	}

	oldTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = mockTrans
	defer func() {
		http.DefaultClient.Transport = oldTransport
	}()

	client := &Client{
		APIKey: "mock-key",
		Model:  "mock-model",
	}

	answer, err := AskWiki(vaultPath, client, "Como funciona o cartão de crédito?")
	if err != nil {
		t.Fatalf("AskWiki failed: %v", err)
	}

	expected := "O sistema de pagamentos suporta processamento de cartão de crédito."
	if answer != expected {
		t.Errorf("expected answer '%s', got '%s'", expected, answer)
	}
}

func TestHandleGraph(t *testing.T) {
	vaultPath, cleanup := createTestVault(t)
	defer cleanup()

	// 1. Create note A that links to B
	docAPath := filepath.Join(vaultPath, "04-systems", "payments.md")
	docAContent := `---
title: "Payments System"
---
# Payments
This system links to [Auth Flow](auth-flow.md).
`
	if err := ioutil.WriteFile(docAPath, []byte(docAContent), 0644); err != nil {
		t.Fatalf("failed to write doc A: %v", err)
	}

	// 2. Create note B that links to A via wikilink
	docBPath := filepath.Join(vaultPath, "04-systems", "auth-flow.md")
	docBContent := `---
title: "Auth Flow"
---
# Auth Flow
This links back to [[04-systems/payments]].
`
	if err := ioutil.WriteFile(docBPath, []byte(docBContent), 0644); err != nil {
		t.Fatalf("failed to write doc B: %v", err)
	}

	// Instantiate Server
	server := NewServer(vaultPath, nil, 8080)

	// Create request
	req, err := http.NewRequest("GET", "/api/wiki/graph", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// Record response
	w := httptest.NewRecorder()
	server.handleGraph(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", resp.StatusCode)
	}

	// Parse body
	var data GraphData
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	// Assertions
	if len(data.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(data.Nodes))
	}

	nodesMap := make(map[string]GraphNode)
	for _, n := range data.Nodes {
		nodesMap[n.ID] = n
	}

	if _, ok := nodesMap["04-systems/payments.md"]; !ok {
		t.Errorf("expected node '04-systems/payments.md' to exist")
	} else if nodesMap["04-systems/payments.md"].Title != "Payments System" {
		t.Errorf("expected title 'Payments System', got '%s'", nodesMap["04-systems/payments.md"].Title)
	}

	if _, ok := nodesMap["04-systems/auth-flow.md"]; !ok {
		t.Errorf("expected node '04-systems/auth-flow.md' to exist")
	}

	// There should be 2 links
	if len(data.Links) != 2 {
		t.Errorf("expected 2 links, got %d", len(data.Links))
	}
}
