package vault

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"kv/internal/fsutil"
)

// DocumentMetadata represents a brief description of a vault file for the overview.
type DocumentMetadata struct {
	RelativePath string
	Title        string
}

// GenerateContext resolves the vault path, searches matching files, and renders a context.md file in the workspace's .opencode/ folder.
func GenerateContext(taskDescription string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %v", err)
	}

	// 1. Discover active vault
	result, err := FindVault(cwd)
	if err != nil {
		return fmt.Errorf("no active Knowledge Vault found. Please initialize a vault first or set KNOWLEDGE_VAULT_PATH: %v", err)
	}

	vaultPath := result.Path
	if !fsutil.IsDir(vaultPath) {
		return fmt.Errorf("the resolved vault path '%s' does not exist", vaultPath)
	}

	fmt.Printf("Gerando contexto para: \"%s\"...\n", taskDescription)

	// 2. Load all vault documents for the overview table
	mdFiles, err := FindMdFiles(vaultPath)
	if err != nil {
		return fmt.Errorf("failed to load vault documents: %v", err)
	}

	var allDocs []DocumentMetadata
	for _, file := range mdFiles {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			continue
		}
		fm, content := ParseFrontmatter(string(data))
		fmTitle, _ := fm["title"].(string)
		title := extractTitle(content, fmTitle, file)
		relPath, err := filepath.Rel(vaultPath, file)
		if err != nil {
			relPath = file
		}
		allDocs = append(allDocs, DocumentMetadata{
			RelativePath: filepath.Clean(relPath),
			Title:        title,
		})
	}

	// 3. Search relevant matching documents
	searchResults, err := Search(vaultPath, taskDescription, 8)
	if err != nil {
		return fmt.Errorf("search failed: %v", err)
	}

	// 4. Render context template
	var sb strings.Builder
	sb.WriteString("> Este contexto foi gerado automaticamente pelo `kv context`.\n")
	sb.WriteString(fmt.Sprintf("> **Vault**: `%s`\n", filepath.Clean(vaultPath)))
	sb.WriteString(fmt.Sprintf("> **Total de documentos no vault**: %d\n", len(allDocs)))
	sb.WriteString(fmt.Sprintf("> **Consulta**: \"%s\"\n\n", taskDescription))

	sb.WriteString("## Task Description\n\n")
	sb.WriteString(taskDescription + "\n\n")

	// Render relevant search results
	if len(searchResults) == 0 {
		sb.WriteString("*(Nenhum documento relevante encontrado no vault.)*\n\n")
	} else {
		sb.WriteString("## Documentos Relevantes do Vault\n\n")
		for _, r := range searchResults {
			sb.WriteString(fmt.Sprintf("### %s\n", r.Title))
			sb.WriteString(fmt.Sprintf("- **Arquivo**: `%s`\n", filepath.Clean(r.File)))
			if len(r.Tags) > 0 {
				sb.WriteString(fmt.Sprintf("- **Tags**: %s\n", strings.Join(r.Tags, ", ")))
			}
			sb.WriteString(fmt.Sprintf("- **Score**: %d\n", r.Score))
			sb.WriteString(fmt.Sprintf("\n> %s\n\n", r.Snippet))
			sb.WriteString("---\n\n")
		}
	}

	// Render overview table of all documents
	if len(allDocs) > 0 {
		sb.WriteString("## Visão Geral do Vault\n\n")
		sb.WriteString("| # | Documento | Título |\n")
		sb.WriteString("|---|-----------|--------|\n")
		for i, doc := range allDocs {
			sb.WriteString(fmt.Sprintf("| %d | `%s` | %s |\n", i+1, doc.RelativePath, doc.Title))
		}
		sb.WriteString("\n")
	}

	// 5. Write to .opencode/context.md in the current workspace
	opencodeDir := filepath.Join(cwd, ".opencode")
	if err := fsutil.EnsureDir(opencodeDir); err != nil {
		return fmt.Errorf("failed to ensure .opencode directory: %v", err)
	}

	contextPath := filepath.Join(opencodeDir, "context.md")
	if err := fsutil.WriteFile(contextPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write context file: %v", err)
	}

	fmt.Printf("Contexto gerado: %s\n", contextPath)
	fmt.Println("O OpenCode lerá este arquivo automaticamente ao iniciar.")

	return nil
}
