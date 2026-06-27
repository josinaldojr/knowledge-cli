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

Para executar a sessão de forma real (integrando com o runner ativo):

```bash
kv run --session session-auth-refactor
```

### O que acontece durante a execução ativa?
1. **Ativação do Sandbox**: O framework monitora chamadas de sistema e acessos a arquivos.
2. **Gravação do Log de Auditoria (`audit.jsonl`)**: Cada arquivo lido, escrito ou comando executado pelo runner é imediatamente registrado com timestamp no log de auditoria append-only.
3. **Validação das Restrições do Policy Engine**: Se o runner tentar realizar ações proibidas (ex: realizar conexões externas à rede quando `allow_network` for `false`), o processo é imediatamente bloqueado.

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
