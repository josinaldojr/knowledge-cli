package wiki

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kv/internal/fsutil"
)

// CompileDecision represents the LLM's decision on how to compile a raw document.
type CompileDecision struct {
	Action     string `json:"action"`      // "CREATE" or "UPDATE"
	TargetPath string `json:"target_path"` // e.g. "04-systems/auth-service.md"
	Reason     string `json:"reason"`
}

// CompileInbox scans the 00-inbox folder and compiles each file into the wiki canonical directories.
func CompileInbox(vaultPath string, client *Client) (int, error) {
	inboxDir := filepath.Join(vaultPath, "00-inbox")
	if !fsutil.IsDir(inboxDir) {
		return 0, fmt.Errorf("inbox directory '%s' does not exist", inboxDir)
	}

	files, err := ioutil.ReadDir(inboxDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read inbox directory: %v", err)
	}

	var rawFiles []string
	for _, f := range files {
		if !f.IsDir() {
			ext := strings.ToLower(filepath.Ext(f.Name()))
			if ext == ".md" || ext == ".txt" {
				rawFiles = append(rawFiles, filepath.Join(inboxDir, f.Name()))
			}
		}
	}

	if len(rawFiles) == 0 {
		return 0, nil
	}

	// Build current wiki index to identify related files
	wikiIndex, err := BuildIndex(vaultPath)
	if err != nil {
		return 0, fmt.Errorf("failed to build wiki index: %v", err)
	}

	compiledCount := 0
	ctx := context.Background()

	for _, file := range rawFiles {
		fmt.Printf("Compilando %s...\n", filepath.Base(file))

		data, err := ioutil.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro ao ler arquivo %s: %v\n", file, err)
			continue
		}
		rawContent := string(data)

		// 1. Find related files
		searchResults := SearchIndex(wikiIndex, rawContent, 5)
		relatedDocsList := ""
		for _, r := range searchResults {
			relPath, _ := filepath.Rel(vaultPath, r.Entry.Path)
			relatedDocsList += fmt.Sprintf("- Caminho: %s, Título: %s\n", relPath, r.Entry.Title)
		}

		// 2. Decide: Create or Update
		decision, err := getCompileDecision(ctx, client, rawContent, relatedDocsList)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro ao decidir compilação para %s: %v\n", filepath.Base(file), err)
			continue
		}

		fmt.Printf("   Ação: %s em %s (Motivo: %s)\n", decision.Action, decision.TargetPath, decision.Reason)

		// Clean and validate target path
		cleanTargetRel := filepath.Clean(decision.TargetPath)
		if strings.HasPrefix(cleanTargetRel, "..") || filepath.IsAbs(cleanTargetRel) {
			fmt.Fprintf(os.Stderr, "Erro: caminho de destino inválido '%s'\n", decision.TargetPath)
			continue
		}

		targetAbsPath := filepath.Join(vaultPath, cleanTargetRel)

		// Ensure containing directory exists
		if err := os.MkdirAll(filepath.Dir(targetAbsPath), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Erro ao criar pasta para %s: %v\n", cleanTargetRel, err)
			continue
		}

		// If LLM asks to UPDATE but file doesn't exist, fallback to CREATE
		if decision.Action == "UPDATE" && !fsutil.IsFile(targetAbsPath) {
			decision.Action = "CREATE"
		}

		var finalContent string
		currentDate := time.Now().Format("2006-01-02")

		if decision.Action == "CREATE" {
			finalContent, err = generateNewPage(ctx, client, rawContent, cleanTargetRel, currentDate)
		} else {
			existingData, err := ioutil.ReadFile(targetAbsPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Erro ao ler arquivo existente %s: %v\n", cleanTargetRel, err)
				continue
			}
			finalContent, err = mergePageContent(ctx, client, rawContent, string(existingData), cleanTargetRel, currentDate)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro ao gerar conteúdo para %s: %v\n", cleanTargetRel, err)
			continue
		}

		// Write compilation result to file
		if err := ioutil.WriteFile(targetAbsPath, []byte(finalContent), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Erro ao salvar arquivo compilado %s: %v\n", cleanTargetRel, err)
			continue
		}

		// Execute link generation on the newly compiled file
		_ = AutoLinkFile(ctx, client, vaultPath, targetAbsPath, wikiIndex)

		// 3. Archive raw input file
		archiveDir := filepath.Join(vaultPath, "10-references", "archive")
		if err := os.MkdirAll(archiveDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Erro ao criar pasta de arquivo histórico: %v\n", err)
			continue
		}

		archiveDest := filepath.Join(archiveDir, filepath.Base(file))
		// If archive destiny already exists, append timestamp to make it unique
		if fsutil.IsFile(archiveDest) {
			timestamp := time.Now().Format("20060102-150405")
			ext := filepath.Ext(file)
			base := strings.TrimSuffix(filepath.Base(file), ext)
			archiveDest = filepath.Join(archiveDir, fmt.Sprintf("%s-%s%s", base, timestamp, ext))
		}

		if err := os.Rename(file, archiveDest); err != nil {
			fmt.Fprintf(os.Stderr, "Erro ao mover arquivo %s para arquivo histórico: %v\n", filepath.Base(file), err)
			continue
		}

		fmt.Printf("   Compilado com sucesso e arquivado em %s\n", filepath.Base(archiveDest))
		compiledCount++
	}

	return compiledCount, nil
}

func getCompileDecision(ctx context.Context, client *Client, content, relatedDocs string) (*CompileDecision, error) {
	systemPrompt := `Você é o compilador de IA do 'kv' Knowledge Vault Wiki, baseado no conceito de LLM Wiki do Andrej Karpathy.
Sua tarefa é analisar uma nova nota bruta/fonte e decidir se ela deve:
1. Criar uma NOVA página de wiki canônica (ex: em 01-global/, 02-domains/, 03-projects/, 04-systems/, 05-decisions/, 07-runbooks/, 08-prompts/).
2. ATUALIZAR/mesclar em uma página de wiki existente.

Se for criar uma nova página, escolha a pasta numerada correta e um nome de arquivo em kebab-case (ex: "04-systems/auth-service.md").
Se for atualizar, use exatamente o caminho da página existente na lista fornecida.

Responda ESTRITAMENTE em formato JSON com esta estrutura:
{
  "action": "CREATE" ou "UPDATE",
  "target_path": "caminho/relativo/para/arquivo.md",
  "reason": "Breve justificativa em português"
}`

	userPrompt := fmt.Sprintf("Nova Nota:\n---\n%s\n---\n\nPáginas existentes relacionadas encontradas no índice:\n%s", content, relatedDocs)

	resp, err := client.GenerateContent(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	// Clean JSON markdown blocks if any (e.g. ```json ... ```)
	resp = strings.TrimSpace(resp)
	if strings.HasPrefix(resp, "```") {
		lines := strings.Split(resp, "\n")
		if len(lines) >= 3 && strings.Contains(lines[0], "json") {
			resp = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	var decision CompileDecision
	if err := json.Unmarshal([]byte(resp), &decision); err != nil {
		// Fallback parse se falhar por causa de caracteres ou tags extras
		if strings.Contains(resp, `"CREATE"`) {
			decision.Action = "CREATE"
		} else {
			decision.Action = "UPDATE"
		}

		// Regex/string search simples para extrair target_path
		idx := strings.Index(resp, `"target_path"`)
		if idx != -1 {
			sub := resp[idx:]
			colonIdx := strings.Index(sub, ":")
			if colonIdx != -1 {
				sub = sub[colonIdx+1:]
				start := strings.Index(sub, `"`)
				if start != -1 {
					sub = sub[start+1:]
					end := strings.Index(sub, `"`)
					if end != -1 {
						decision.TargetPath = sub[:end]
					}
				}
			}
		}
		decision.Reason = "Fallback parsing de resposta JSON"
		if decision.TargetPath == "" {
			return nil, fmt.Errorf("failed to parse decision JSON: %v. Raw response: %s", err, resp)
		}
	}

	return &decision, nil
}

func generateNewPage(ctx context.Context, client *Client, rawContent, targetPath, currentDate string) (string, error) {
	systemPrompt := fmt.Sprintf(`Você é o compilador do 'kv' Knowledge Vault Wiki.
Sua tarefa é escrever uma NOVA página markdown canônica com base na nota bruta fornecida.
Diretrizes:
1. Você DEVE adicionar um bloco YAML frontmatter no topo exato com:
   ---
   status: canon
   created_at: %s
   updated_at: %s
   title: "Título apropriado da página"
   category: "categoria correspondente"
   tags: [tag1, tag2]
   ---
2. Use títulos de markdown claros e bem estruturados.
3. Mantenha as informações técnicas organizadas, claras e factuais, baseando-se estritamente na nota.
4. Escreva a página em português.
5. Retorne APENAS o código markdown completo da página, sem explicações adicionais fora do bloco markdown.`, currentDate, currentDate)

	userPrompt := fmt.Sprintf("Caminho sugerido: %s\n\nNota Bruta:\n%s", targetPath, rawContent)

	return client.GenerateContent(ctx, systemPrompt, userPrompt)
}

func mergePageContent(ctx context.Context, client *Client, rawContent, existingContent, targetPath, currentDate string) (string, error) {
	systemPrompt := fmt.Sprintf(`Você é o compilador do 'kv' Knowledge Vault Wiki.
Sua tarefa é MESCLAR as novas informações de uma nota bruta em uma página de wiki markdown EXISTENTE.
Diretrizes:
1. Integre o conteúdo da nota de maneira lógica nas seções apropriadas do documento original.
2. Atualize o campo 'updated_at' no frontmatter YAML para '%s'. Não modifique o campo 'created_at'.
3. Mantenha todos os metadados existentes no frontmatter, apenas alterando/adicionando tags se relevante.
4. Se houver contradições menores, resolva-as usando a informação mais recente/detalhada. Se houver uma contradição de arquitetura ou técnica grave que você não consiga resolver sozinho, adicione uma seção no final chamada "## Contradições / Pendências" descrevendo o conflito.
5. Não remova nenhuma informação útil existente que não tenha sido diretamente superada.
6. Retorne APENAS o código markdown completo atualizado, sem introduções ou explicações fora dele.`, currentDate)

	userPrompt := fmt.Sprintf("Caminho do Arquivo: %s\n\nConteúdo Existente:\n---\n%s\n---\n\nNova Nota Bruta:\n---\n%s\n---", targetPath, existingContent, rawContent)

	return client.GenerateContent(ctx, systemPrompt, userPrompt)
}
