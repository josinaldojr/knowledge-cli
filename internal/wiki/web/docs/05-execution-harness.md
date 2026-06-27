# Execução Controlada & Auditoria (`kv run`)

O motor central de execução do `kv` garante que nenhuma modificação física ocorra fora das boundaries especificadas no contrato da sessão, protegendo o codebase local contra alterações acidentais.

---

## 🔬 Execução Simulada (Dry Run)

Sempre que planejar uma nova execução, é altamente recomendado rodar uma simulação primeiro:

```bash
kv run --session session-auth-refactor --dry-run
```

O comando `--dry-run` não invoca os agentes ou ferramentas de escrita. Em vez disso, ele exibe um sumário de auditoria na tela:
- **Tamanho Estimado do Contexto**: Tokens e bytes do arquivo compilado.
- **Boundaries Ativas**: Lista de caminhos de leitura e escrita permitidos.
- **Lista de Validações Físicas**: Se as aplicações e os runbooks do cofre estão saudáveis.
- **Verificação de Políticas de Segurança**: Status das diretrizes do Policy Engine.

---

## 🏃 Execução de Produção

Para executar a sessão de forma real:

```bash
kv run --session session-auth-refactor
```

### O que acontece durante a execução ativa?
1. **Ativação do Runner (OpenCode)**: O `kv` resolve qual runner de agente utilizar (padrão: `opencode`). Ele lê o arquivo `.kv/sessions/<session-id>/opencode.md` contendo as diretrizes e executa o binário do **OpenCode** local via `opencode run "<prompt>"`.
2. **Herdamento de Streams**: O processo subprocessado herda `os.Stdin`, `os.Stdout` e `os.Stderr`, mantendo a interatividade total para consentimento de permissões de leitura/escrita de arquivos e comandos diretamente na janela de terminal atual.
3. **Auditoria e Finalização**: Uma vez concluído o processamento do OpenCode, o `kv` recupera o fluxo de controle e realiza a auditoria pós-execução:
   - **Quality Gates**: Executa automaticamente os testes de qualidade (unitários, linters) da aplicação.
   - **Diff Summarizer**: Analisa as mudanças de código e cria um resumo estruturado em `diff-summary.md` respeitando as restrições físicas de boundaries.
   - **Gravação do Log de Auditoria (`audit.jsonl`)**: Registra timestamps e estados das etapas realizadas de forma append-only.
   - **Session Report**: Une o objetivo, status das boundaries, testes rodados e diffs para gerar o relatório consolidado `report.md`.

---

## 📋 Estrutura do Log `audit.jsonl`

O arquivo de auditoria registra os passos executados em formato JSON Lines para facilitar integrações automatizadas de monitoramento:

```json
{"timestamp":"2026-06-26T22:30:15Z","event":"file_write","path":"./apps/api-gateway/auth.go","size_bytes":1024,"status":"success"}
{"timestamp":"2026-06-26T22:31:02Z","event":"command_exec","cmd":"go test ./...","app":"api-gateway","exit_code":0,"status":"completed"}
{"timestamp":"2026-06-26T22:31:40Z","event":"policy_violation","action":"network_request","target":"google.com","status":"blocked"}
```

> [!WARNING]
> O arquivo `audit.jsonl` é gerado em modo append-only. Tentar corromper ou deletar registros de auditoria durante a execução ativa da sessão disparará um aviso crítico de segurança.
