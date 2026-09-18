package wiki

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// AskWiki queries the local wiki index for context and uses the Gemini API to answer the query.
func AskWiki(vaultPath string, client *Client, query string) (string, error) {
	index, err := BuildIndex(vaultPath)
	if err != nil {
		return "", fmt.Errorf("failed to build wiki index: %v", err)
	}

	// 1. Search top 4 related files, combining lexical (BM25) and, when
	// available, local semantic ranking.
	searchResults := HybridSearchIndex(index, query, 4, DefaultEmbedder())

	var contextChunks []string
	var sources []string

	for _, r := range searchResults {
		relPath, err := filepath.Rel(vaultPath, r.Entry.Path)
		if err != nil {
			relPath = r.Entry.Path
		}
		sources = append(sources, relPath)

		// Format context chunk
		chunk := fmt.Sprintf("Arquivo: %s\nTítulo: %s\nConteúdo:\n%s", relPath, r.Entry.Title, r.Entry.Content)
		contextChunks = append(contextChunks, chunk)
	}

	var systemPrompt string
	var userPrompt string

	if len(contextChunks) > 0 {
		systemPrompt = `Você é o Assistente de Perguntas e Respostas da LLM Wiki do 'kv'.
Sua tarefa é responder à pergunta do usuário baseando-se estritamente nas seções do contexto fornecidas, as quais foram recuperadas da wiki local do Knowledge Vault.

Diretrizes:
1. Responda em português de forma clara, técnica e concisa.
2. Cite explicitamente quais arquivos da wiki (ex: "04-systems/auth.md") contêm as informações que fundamentam sua resposta.
3. Se o contexto não contiver informações suficientes para responder à pergunta, mencione isso claramente e liste os arquivos que encontrou por aproximação de palavras-chave.`

		userPrompt = fmt.Sprintf("Contexto Recuperado da Wiki:\n%s\n\nPergunta: %s", strings.Join(contextChunks, "\n\n---\n\n"), query)
	} else {
		systemPrompt = `Você é o Assistente de Perguntas e Respostas da LLM Wiki do 'kv'.
O usuário fez uma pergunta, mas não encontramos nenhuma página correspondente no índice local da Wiki do Knowledge Vault.

Diretrizes:
1. Responda em português informando que nenhum documento correspondente foi encontrado no cofre de conhecimento local.
2. Forneça uma resposta geral ou de melhores práticas sobre o assunto, se souber, e sugere quais tipos de notas ou ADRs deveriam ser criados para documentar este tópico no futuro.`

		userPrompt = fmt.Sprintf("Pergunta: %s", query)
	}

	ctx := context.Background()
	answer, err := client.GenerateContent(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", fmt.Errorf("failed to get answer from LLM: %v", err)
	}

	return answer, nil
}
