package wiki

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed web/*
var staticFiles embed.FS

// TreeNode represents a file or directory in the vault hierarchy.
type TreeNode struct {
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	IsDir    bool       `json:"is_dir"`
	Children []TreeNode `json:"children,omitempty"`
}

// GraphNode represents a node in the wiki link graph.
type GraphNode struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Group string `json:"group"`
}

// GraphLink represents a directed connection between two wiki documents.
type GraphLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// GraphData encapsulates the nodes and links of the vault graph.
type GraphData struct {
	Nodes []GraphNode `json:"nodes"`
	Links []GraphLink `json:"links"`
}

// Server encapsulates the wiki HTTP server configurations.
type Server struct {
	VaultPath string
	Client    *Client
	Port      int
}

// NewServer creates a new instance of the wiki server.
func NewServer(vaultPath string, client *Client, port int) *Server {
	return &Server{
		VaultPath: vaultPath,
		Client:    client,
		Port:      port,
	}
}

// Start launches the local wiki server.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// 1. Static Files (Embedded frontend)
	subFS, err := fs.Sub(staticFiles, "web")
	if err != nil {
		return fmt.Errorf("failed to sub static files directory: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(subFS)))

	// 2. API Endpoints
	mux.HandleFunc("/api/wiki/docs", s.handleDocs)
	mux.HandleFunc("/api/wiki/doc", s.handleDocContent)
	mux.HandleFunc("/api/wiki/query", s.handleQuery)
	mux.HandleFunc("/api/wiki/chat", s.handleChat)
	mux.HandleFunc("/api/wiki/graph", s.handleGraph)

	addr := fmt.Sprintf("127.0.0.1:%d", s.Port)
	fmt.Printf("LLM Wiki Web Server rodando em http://localhost:%d\n", s.Port)
	fmt.Println("Pressione Ctrl+C para encerrar.")

	return http.ListenAndServe(addr, mux)
}

// handleDocs returns the hierarchical list of documents in the vault.
func (s *Server) handleDocs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tree, err := s.buildVaultTree()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to build vault tree: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tree)
}

// handleDocContent reads and returns the requested markdown file.
func (s *Server) handleDocContent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	relPath := r.URL.Query().Get("path")
	if relPath == "" {
		http.Error(w, "Missing path parameter", http.StatusBadRequest)
		return
	}

	cleanRel := filepath.Clean(relPath)
	if strings.HasPrefix(cleanRel, "..") || filepath.IsAbs(cleanRel) {
		http.Error(w, "Invalid path path", http.StatusBadRequest)
		return
	}

	absPath := filepath.Join(s.VaultPath, cleanRel)
	data, err := ioutil.ReadFile(absPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read file: %v", err), http.StatusNotFound)
		return
	}

	resp := map[string]string{
		"path":    cleanRel,
		"content": string(data),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleQuery runs the local search index and returns ranked results.
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Query string `json:"query"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	index, err := BuildIndex(s.VaultPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to build index: %v", err), http.StatusInternalServerError)
		return
	}

	results := SearchIndex(index, req.Query, 10)

	type SearchClientResult struct {
		Path    string   `json:"path"`
		Title   string   `json:"title"`
		Snippet string   `json:"snippet"`
		Score   int      `json:"score"`
		Tags    []string `json:"tags"`
	}

	var clientResults []SearchClientResult
	for _, r := range results {
		relPath, _ := filepath.Rel(s.VaultPath, r.Entry.Path)
		clientResults = append(clientResults, SearchClientResult{
			Path:    relPath,
			Title:   r.Entry.Title,
			Snippet: extractSnippet(r.Entry.Content, req.Query),
			Score:   r.Score,
			Tags:    r.Entry.Tags,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clientResults)
}

// handleChat processes turn-based chat queries enriched with RAG search context.
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type ChatMessage struct {
		Role string `json:"role"`
		Text string `json:"text"`
	}

	var req struct {
		Query   string        `json:"query"`
		History []ChatMessage `json:"history"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	index, err := BuildIndex(s.VaultPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to build index: %v", err), http.StatusInternalServerError)
		return
	}

	// 1. Search top 4 documents for RAG context, combining lexical (BM25)
	// and, when available, local semantic ranking.
	searchResults := HybridSearchIndex(index, req.Query, 4, DefaultEmbedder())
	var contextChunks []string
	for _, r := range searchResults {
		relPath, _ := filepath.Rel(s.VaultPath, r.Entry.Path)
		contextChunks = append(contextChunks, fmt.Sprintf("Arquivo: %s\nTítulo: %s\nConteúdo:\n%s", relPath, r.Entry.Title, r.Entry.Content))
	}

	// 2. Build conversation history string
	var historyBuilder strings.Builder
	for _, msg := range req.History {
		roleLabel := "Usuário"
		if msg.Role == "model" {
			roleLabel = "Assistente"
		}
		fmt.Fprintf(&historyBuilder, "%s: %s\n", roleLabel, msg.Text)
	}

	// 3. Assemble prompt
	var systemPrompt string
	var userPrompt string

	if len(contextChunks) > 0 {
		systemPrompt = `Você é o assistente inteligente da LLM Wiki do 'kv'.
Responda à pergunta do usuário baseando-se estritamente no contexto da wiki local fornecido.
Diretrizes:
1. Escreva sua resposta em português técnico e amigável, formatada em Markdown.
2. Cite explicitamente as páginas da wiki utilizadas (ex: "04-systems/auth.md").
3. Leve em conta o histórico da conversa para manter o fio condutor.`

		userPrompt = fmt.Sprintf("Contexto da Wiki:\n%s\n\nHistórico do Chat:\n%s\nPergunta Atual: %s",
			strings.Join(contextChunks, "\n\n---\n\n"),
			historyBuilder.String(),
			req.Query,
		)
	} else {
		systemPrompt = `Você é o assistente inteligente da LLM Wiki do 'kv'.
Nenhum documento relevante foi encontrado no cofre. Responda amigavelmente informando isso e dê uma resposta de melhores práticas em português.`
		userPrompt = fmt.Sprintf("Histórico do Chat:\n%s\nPergunta Atual: %s",
			historyBuilder.String(),
			req.Query,
		)
	}

	// 4. Generate answer
	answer, err := s.Client.GenerateContent(context.Background(), systemPrompt, userPrompt)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate content: %v", err), http.StatusInternalServerError)
		return
	}

	resp := map[string]string{
		"answer": answer,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleGraph extracts cross-links and returns nodes and links data for graph visualization.
func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	index, err := BuildIndex(s.VaultPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to build index: %v", err), http.StatusInternalServerError)
		return
	}

	// 1. Build a set of all valid relative file paths in the vault for target verification
	validPaths := make(map[string]bool)
	for _, entry := range index {
		relPath, err := filepath.Rel(s.VaultPath, entry.Path)
		if err == nil {
			// Normalize to use forward slashes
			relPathNorm := filepath.ToSlash(relPath)
			validPaths[relPathNorm] = true
		}
	}

	nodes := []GraphNode{}
	links := []GraphLink{}
	linkSet := make(map[string]bool) // To avoid duplicate links

	// Regex to match markdown links: [text](target.md) or [text](target.md#section)
	mdLinkRegex := regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
	// Regex to match wikilinks: [[target]] or [[target|display]]
	wikiLinkRegex := regexp.MustCompile(`\[\[([^\]|]+)(?:\|[^\]]+)?\]\]`)

	for _, entry := range index {
		relPath, err := filepath.Rel(s.VaultPath, entry.Path)
		if err != nil {
			continue
		}
		relPathNorm := filepath.ToSlash(relPath)

		// Determine the group (top-level directory name)
		group := "root"
		parts := strings.Split(relPathNorm, "/")
		if len(parts) > 1 {
			group = parts[0]
		}

		nodes = append(nodes, GraphNode{
			ID:    relPathNorm,
			Title: entry.Title,
			Group: group,
		})

		// Find links in content
		sourceDir := filepath.Dir(relPathNorm)

		// Helper to resolve and record a target link
		addLink := func(target string) {
			// Remove anchor fragment if exists
			if idx := strings.Index(target, "#"); idx != -1 {
				target = target[:idx]
			}
			target = strings.TrimSpace(target)
			if target == "" {
				return
			}

			// Skip remote URLs or other protocols
			if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "mailto:") {
				return
			}

			// 1. Try directly relative to the vault root (common for Obsidian wikilinks)
			resolvedDirect := filepath.ToSlash(filepath.Clean(target))
			if validPaths[resolvedDirect] {
				if resolvedDirect != relPathNorm {
					linkKey := relPathNorm + " -> " + resolvedDirect
					if !linkSet[linkKey] {
						linkSet[linkKey] = true
						links = append(links, GraphLink{
							Source: relPathNorm,
							Target: resolvedDirect,
						})
					}
				}
				return
			}

			// 2. Otherwise resolve relative to sourceDir (standard markdown relative links)
			var resolved string
			if filepath.IsAbs(target) {
				resolved = filepath.Clean(strings.TrimPrefix(target, "/"))
			} else {
				resolved = filepath.Clean(filepath.Join(sourceDir, target))
			}
			resolvedNorm := filepath.ToSlash(resolved)

			// Validate target exists in our vault index
			if validPaths[resolvedNorm] && resolvedNorm != relPathNorm {
				linkKey := relPathNorm + " -> " + resolvedNorm
				if !linkSet[linkKey] {
					linkSet[linkKey] = true
					links = append(links, GraphLink{
						Source: relPathNorm,
						Target: resolvedNorm,
					})
				}
			}
		}

		// 1. Process standard markdown links
		matches := mdLinkRegex.FindAllStringSubmatch(entry.Content, -1)
		for _, m := range matches {
			if len(m) > 1 {
				addLink(m[1])
			}
		}

		// 2. Process wikilinks
		wikiMatches := wikiLinkRegex.FindAllStringSubmatch(entry.Content, -1)
		for _, m := range wikiMatches {
			if len(m) > 1 {
				// Wikilinks inside Obsidian are usually base name or path from root.
				// If it doesn't end with .md, we append it.
				target := strings.TrimSpace(m[1])
				if !strings.HasSuffix(strings.ToLower(target), ".md") {
					target = target + ".md"
				}
				
				// Let's resolve it. Since it could be a path relative to vault root,
				// check if it exists directly. If not, try relative to current dir.
				resolvedNorm := filepath.ToSlash(filepath.Clean(target))
				if validPaths[resolvedNorm] {
					addLink(target)
				} else {
					// Try relative to current directory
					addLink(target)
				}
			}
		}
	}

	resp := GraphData{
		Nodes: nodes,
		Links: links,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// isKeepableNode returns true if a node should be kept in the tree.
func (s *Server) isKeepableNode(node TreeNode, isRootChild bool) bool {
	if !node.IsDir {
		return strings.ToLower(filepath.Ext(node.Name)) == ".md"
	}
	if len(node.Children) > 0 {
		return true
	}
	if isRootChild {
		canonicalDirs := map[string]bool{
			"00-inbox":      true,
			"01-global":     true,
			"02-domains":    true,
			"03-projects":   true,
			"04-systems":    true,
			"05-decisions":  true,
			"06-agents":     true,
			"07-runbooks":   true,
			"08-prompts":    true,
			"09-templates":  true,
			"10-references": true,
		}
		if canonicalDirs[node.Name] {
			return true
		}
	}
	return false
}

// buildVaultTree structures the vault files in a JSON-serializable tree.
func (s *Server) buildVaultTree() ([]TreeNode, error) {
	files, err := ioutil.ReadDir(s.VaultPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read vault path: %v", err)
	}

	var root []TreeNode
	for _, f := range files {
		fName := f.Name()
		if strings.HasPrefix(fName, ".") || fName == "node_modules" {
			continue
		}

		dirPath := filepath.Join(s.VaultPath, fName)
		node, err := s.walkNode(dirPath, fName)
		if err == nil {
			if s.isKeepableNode(node, true) {
				root = append(root, node)
			}
		}
	}

	return root, nil
}

func (s *Server) walkNode(absPath, relPath string) (TreeNode, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return TreeNode{}, err
	}

	node := TreeNode{
		Name:  info.Name(),
		Path:  relPath,
		IsDir: info.IsDir(),
	}

	if info.IsDir() {
		files, err := ioutil.ReadDir(absPath)
		if err != nil {
			return node, nil
		}

		for _, f := range files {
			fName := f.Name()
			if strings.HasPrefix(fName, ".") || fName == "node_modules" {
				continue // Skip hidden files and dependencies
			}

			childAbs := filepath.Join(absPath, fName)
			childRel := filepath.Join(relPath, fName)

			// Walk files and folders recursively
			childNode, err := s.walkNode(childAbs, childRel)
			if err == nil {
				if s.isKeepableNode(childNode, false) {
					node.Children = append(node.Children, childNode)
				}
			}
		}
	}

	return node, nil
}

func extractSnippet(content, query string) string {
	lowerContent := strings.ToLower(content)
	lowerQuery := strings.ToLower(query)
	
	idx := strings.Index(lowerContent, lowerQuery)
	runes := []rune(content)
	
	if idx == -1 {
		limit := 160
		if len(runes) < limit {
			limit = len(runes)
		}
		return string(runes[:limit]) + "..."
	}

	// Find approximate rune start and end bounds
	bytePrefix := content[:idx]
	runeIdx := len([]rune(bytePrefix))
	
	start := runeIdx - 60
	if start < 0 {
		start = 0
	}

	end := runeIdx + len([]rune(query)) + 100
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
