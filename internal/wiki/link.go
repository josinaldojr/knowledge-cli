package wiki

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// AutoLinkFile reads a markdown file, prompts the LLM to add relative links to other wiki files,
// and saves the updated content.
func AutoLinkFile(ctx context.Context, client *Client, vaultPath, filePath string, otherEntries []IndexEntry) error {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %v", filePath, err)
	}
	content := string(data)

	relSourcePath, err := filepath.Rel(vaultPath, filePath)
	if err != nil {
		relSourcePath = filePath
	}

	// 1. Build a list of available pages with titles and their relative paths from the vault root
	var availablePages []string
	for _, entry := range otherEntries {
		if entry.Path == filePath {
			continue // Don't link to itself
		}

		relPath, err := filepath.Rel(vaultPath, entry.Path)
		if err != nil {
			relPath = entry.Path
		}
		availablePages = append(availablePages, fmt.Sprintf("- Título: \"%s\", Caminho no Vault: \"%s\", Tags: %v", entry.Title, relPath, entry.Tags))
	}

	if len(availablePages) == 0 {
		return nil // Nothing to link to
	}

	systemPrompt := fmt.Sprintf(`Você é o gerador de links automáticos do 'kv' Knowledge Vault Wiki.
Sua tarefa é analisar o markdown de um arquivo e adicionar links relativos de markdown para outras páginas da wiki que forem mencionadas no texto.

Caminho relativo do arquivo atual: %s

Lista de páginas disponíveis no cofre:
%s

Regras estritas:
1. Calcule os links relativos corretos a partir da localização do arquivo atual (%s) para a página de destino.
   - Exemplo 1: Se o arquivo atual estiver em "04-systems/payments.md" e você quiser linkar para "05-decisions/adr-001.md", o caminho relativo correto é "../05-decisions/adr-001.md".
   - Exemplo 2: Se o arquivo atual estiver em "01-global/rules.md" e a destino em "01-global/security.md", o caminho relativo correto é "security.md".
2. Não altere o texto original de forma alguma. Apenas converta os termos/conceitos correspondentes em links markdown (ex: "padrão de autenticação" se torna "[padrão de autenticação](../01-global/auth.md)").
3. NÃO adicione links a textos dentro de blocos de código (fenced code blocks) ou dentro do frontmatter YAML.
4. NÃO recrie links que já existem no documento.
5. Retorne APENAS o código markdown completo atualizado, sem nenhuma introdução ou nota explicativa.`, relSourcePath, strings.Join(availablePages, "\n"), relSourcePath)

	userPrompt := fmt.Sprintf("Conteúdo do arquivo para atualizar links:\n---\n%s\n---", content)

	updatedContent, err := client.GenerateContent(ctx, systemPrompt, userPrompt)
	if err != nil {
		return fmt.Errorf("failed to generate links for %s: %v", filePath, err)
	}

	// Clean code block markers if returned by Gemini
	updatedContent = strings.TrimSpace(updatedContent)
	if strings.HasPrefix(updatedContent, "```markdown") && strings.HasSuffix(updatedContent, "```") {
		updatedContent = strings.TrimPrefix(updatedContent, "```markdown")
		updatedContent = strings.TrimSuffix(updatedContent, "```")
		updatedContent = strings.TrimSpace(updatedContent)
	} else if strings.HasPrefix(updatedContent, "```") && strings.HasSuffix(updatedContent, "```") {
		updatedContent = strings.TrimPrefix(updatedContent, "```")
		updatedContent = strings.TrimSuffix(updatedContent, "```")
		updatedContent = strings.TrimSpace(updatedContent)
	}

	if updatedContent == "" {
		return fmt.Errorf("gemini returned empty content for link generation of %s", filePath)
	}

	// Write updated content if there are changes
	if updatedContent != content {
		if err := ioutil.WriteFile(filePath, []byte(updatedContent), 0644); err != nil {
			return fmt.Errorf("failed to write updated file %s: %v", filePath, err)
		}
		fmt.Printf("   Links atualizados em %s\n", relSourcePath)
	}

	return nil
}

// AutoLinkAll runs a linking pass over all canonical wiki files.
func AutoLinkAll(vaultPath string, client *Client) error {
	index, err := BuildIndex(vaultPath)
	if err != nil {
		return err
	}

	if len(index) <= 1 {
		fmt.Println("Não há páginas suficientes para criar referências cruzadas.")
		return nil
	}

	ctx := context.Background()
	fmt.Printf("Iniciando varredura de links cruzados em %d arquivo(s)...\n", len(index))

	for _, entry := range index {
		err := AutoLinkFile(ctx, client, vaultPath, entry.Path, index)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro ao processar links para %s: %v\n", filepath.Base(entry.Path), err)
		}
	}

	fmt.Println("Varredura de links cruzados concluída com sucesso.")
	return nil
}
