# kv - Automatic MCP Engineering Memory

`kv` is a local stdio MCP server for evidence-backed engineering memory. It observes OpenSpec operations, records only verified changes to OpenSpec artifacts, and returns relevant decisions, constraints, risks, and history to the connected provider. The normal MCP workflow needs no `kv init` and creates no `.kv` files in the observed repository.

KV discovers the workspace from the provider working directory and keeps its operational SQLite database, snapshots, and retry spool in global user storage. OpenSpec remains canonical: KV does not edit proposals, designs, specs, tasks, or archives. It does not store transcripts, chain of thought, arbitrary conversation, or non-OpenSpec files.

Separately from MCP memory, `kv` also ships a project-local **harness**: a control layer that declares what a coding agent may read/write, builds its context, runs it non-interactively inside those boundaries, validates the result with real test commands, and audits everything to an append-only log. See [Como o Harness Funciona](#como-o-harness-funciona) below, and [Legacy CLI status](#legacy-cli-status) for how it relates to MCP memory.

---

## MCP Engineering Memory

- **Automatic sessions:** a provider session is registered on its first valid lifecycle event, using a native session ID when available or an adapter correlation ID otherwise.
- **Verified OpenSpec history:** before/after hooks capture resolved OpenSpec artifacts, calculate structural deltas, and retain evidence-linked memory.
- **Scoped assistance:** retrieval is limited to the logical workspace by default and ranks current decisions over superseded history.
- **Reliable degradation:** unavailable MCP assistance does not block provider work. Recordable lifecycle events are spooled locally and can be retried.

See [automatic MCP memory](docs/automatic-mcp-memory.md) for storage, privacy, limits, administration, and troubleshooting. See the [provider capability matrix](docs/provider-capability-matrix.md) for current provider support boundaries.

---

## Como o Harness Funciona

O harness é a camada de controle que fica entre você e o agente de codificação (Claude Code, OpenCode ou Codex): ele declara o que o agente pode ler e escrever, monta o contexto que ele recebe, executa o agente sem interação manual, valida o resultado com testes reais (não com o julgamento de uma LLM) e audita tudo em um log append-only. Diferente da Memória MCP (automática, silenciosa, focada em OpenSpec), o harness é acionado explicitamente por você via CLI e produz artefatos versionáveis em Git sob `.kv/`.

Toda execução — seja uma Sessão (`kv run`) ou uma Task de Workflow (`kv task run`/`kv workflow run`) — passa pelo mesmo pipeline:

```
kv-workspace.yaml        session.yaml                .kv/sessions/<id>/
 (apps + stacks,   ──▶   (contrato da        ──▶      context.md
  opcional)                sessão)                    opencode.md (prompt do runner)
                                                              │
                                                              ▼
                                                    runner (opencode run /
                                                     claude -p / codex exec)
                                                              │
                              ┌───────────────────────────────┼──────────────────────────┐
                              ▼                                ▼                          ▼
                       Policy Engine                    Quality Gates              Diff Summarizer
                  (bloqueia comando perigoso,        (roda os comandos de       (resume o git diff
                   .env, dependências, etc.)           teste/lint reais)         dos paths permitidos)
                              │                                │                          │
                              └───────────────────────────────┴──────────────────────────┘
                                                              ▼
                                            audit.jsonl (log append-only) + report.md
```

- **Workspace (`kv-workspace.yaml`)**: registro declarativo de quais aplicações existem no repositório/monorepo e em qual stack (`go.mod`, `package.json`, `pom.xml`, ...). Opcional para um projeto único — ver [Usando em Projetos e Workspaces](#usando-em-projetos-e-workspaces).
- **Sessão (`session.yaml`)**: o contrato que a execução segue — quais apps estão envolvidas, qual Vault de conhecimento está associado, qual provider roda o agente (`agent.provider`: `opencode`, `claude-code` ou `codex`), quais caminhos são fisicamente permitidos/escritos/só-leitura (`boundary`), quais comandos validam o resultado (`quality.commands`) e quais restrições de segurança se aplicam (`policy`).
- **Boundary físico**: antes e depois da execução, o `kv` confere via `git diff` real que nenhum arquivo fora de `allowed_paths`/`writable_paths` foi tocado, e rejeita a sessão se houver violação — isso não é uma instrução no prompt, é uma checagem contra o estado real do repositório.
- **Context Builder**: compila o objetivo, os arquivos candidatos (por proximidade ao objetivo) e as diretrizes do Vault em `context.md` e no prompt específico do runner (`opencode.md`), respeitando o orçamento de contexto (`agent.context_budget`).
- **Runner**: executa o agente de forma não-interativa dentro dos boundaries — `opencode run`, `claude -p` (Claude Code) ou `codex exec` (Codex), conforme `agent.provider`.
- **Policy Engine**: durante os Quality Gates, intercepta comandos perigosos (`rm -rf`, `sudo`), leitura de `.env`, instalação de dependências, migrações e uso de Docker, bloqueando ou pedindo confirmação conforme `policy.*` no contrato da sessão.
- **Quality Gates**: roda de fato os comandos de teste/lint inferidos da stack (ex: `go test ./...`, `npm test`) — o critério de sucesso é o exit code do comando, não a opinião de uma LLM sobre o código.
- **Diff Summarizer**: resume o `git diff` restrito aos paths permitidos da sessão: arquivos alterados, funções/testes novos, riscos de regressão.
- **Auditoria e Relatório**: cada evento (criação, validação, boundary check, execução, quality gates) é gravado em `.kv/sessions/<id>/audit.jsonl` (append-only, nunca reescrito); `report.md` consolida tudo em um relatório final legível.

O mesmo pipeline vale para Workflows/Tasks (`kv task run`, `kv workflow run`), mudando apenas a granularidade e a governança ao redor — ver [Comparativo: Sessões vs. Workflows](#comparativo-sessões-vs-workflows).

---

## Instalação

KV requires Go 1.26.3 or later to build from this checkout. Install the binary where the provider can find `kv` on `PATH`:

```bash
go install ./cmd/kv
```

For a local build instead:

```bash
go build -o kv ./cmd/kv
```

### OpenCode

OpenCode is the only provider with a KV CLI installer in this release:

```bash
kv opencode install
kv mcp doctor --provider opencode
```

The installer merges a `kv` local stdio MCP entry into `~/.config/opencode/opencode.json` without replacing existing entries. Remove only that MCP entry with `kv opencode uninstall`.

### Claude Code and Codex

Claude Code and Codex adapters use the same `kv mcp` server, but this release does not expose CLI install or uninstall commands for them. Configure their MCP entry manually, preserving existing configuration, then verify it with:

```bash
kv mcp doctor --provider claude-code
kv mcp doctor --provider codex
```

Claude Code is checked at `~/.claude/settings.json` for a `mcpServers.kv` entry. Codex is checked at `~/.codex/config.toml` for an `[mcp_servers.kv]` table. Both entries must launch `kv mcp`. Claude Code has a native-session adapter when its hook environment provides `CLAUDE_SESSION_ID`; Codex uses a process-lifetime correlation ID because no native session ID is assumed. Refer to the capability matrix before enabling lifecycle hooks.

### MCP Administration and Troubleshooting

```bash
kv mcp status                         # data path, database, queued events
kv mcp doctor --provider all          # binary and configuration checks
kv events retry                       # deliver due queued lifecycle events
kv mcp reconcile --timeout 24h        # mark inactive sessions/operations stale
kv workspace status                   # current global workspace record
kv session list                       # global provider sessions for this workspace
kv change history                     # OpenSpec transformation history
kv memory search "authentication"      # evidence-linked engineering memory
```

If the doctor reports `binary=false`, install `kv` on `PATH`. If it reports `mcp_configured=false`, run the OpenCode installer or add the provider's MCP entry manually. Pending events mean a provider could not obtain an MCP acknowledgement; retry them after the server/configuration is available.

MCP requests and eligible OpenSpec snapshots are limited to 1 MiB. KV offers lexical local retrieval only, has no cross-machine synchronization, and does not capture arbitrary code changes or provider discussion. Archive consolidation is advisory and may report degraded assistance rather than blocking an OpenSpec archive.

### Legacy CLI Status

`kv init`, project-local `.kv` sessions and workflows, task memory drafts, direct Vault promotion, and the Wiki remain supported legacy workflows. They are not automatically imported into, read by, or written by the global MCP store. Use MCP memory for verified OpenSpec transformations; use the legacy commands when their project-local session, Vault, or curated Markdown workflow is required. There is no scheduled removal of legacy commands.

---

## Usando em Projetos e Workspaces

O harness tem dois pontos de entrada para abrir uma sessão, dependendo se você está em um projeto único ou em um workspace com várias aplicações. Os dois produzem o mesmo `session.yaml` e seguem o mesmo pipeline descrito acima — a diferença é só como as apps são declaradas.

| | `kv session init` | `kv session start` |
| --- | --- | --- |
| Exige `kv-workspace.yaml`? | Não — se não encontrar um em nenhum diretório pai, usa o diretório atual como raiz | Sim — as apps precisam estar registradas via `kv workspace scan`/`kv workspace init` |
| Como aponta as apps | `--apps nome=caminho[,nome=caminho...]` (caminhos livres, qualquer nome) | `--apps <id>,<id>` (IDs já registrados no workspace) |
| Quando usar | Projeto único, ajuste pontual, prototipagem rápida | Monorepo ou múltiplos serviços, sessões recorrentes que reaproveitam o mesmo registro de apps |

### Projeto Único (sem workspace)

Sem `kv-workspace.yaml`, `kv session init` funciona direto na raiz do projeto atual:

```bash
cd meu-projeto

# 1. Cria a sessão apontando para o próprio projeto
kv session init --id fix-login-bug --goal "Corrigir bug de sessão expirada no login" \
  --apps meu-projeto=. --writable . --provider claude-code

# 2. Valida os caminhos declarados no contrato
kv session validate --session fix-login-bug

# 3. Compila o contexto (context.md + opencode.md)
kv context build --session fix-login-bug

# 4. Confere em dry-run o que seria executado, sem rodar o agente
kv run --session fix-login-bug --dry-run

# 5. Executa de verdade: o agente só enxerga/altera o que está no boundary
kv run --session fix-login-bug

# 6. Relatório final: objetivo, boundary, quality gates e diff
cat .kv/sessions/fix-login-bug/report.md
```

### Workspace Multi-App (monorepo ou múltiplos serviços)

Quando o repositório contém várias aplicações — um monorepo, ou pastas irmãs de microsserviços — registre-as primeiro em um workspace:

```bash
cd meu-monorepo

# 1. Cria o kv-workspace.yaml vazio (opcionalmente já associando um Vault)
kv workspace init --vault ./vault

# 2. Detecta automaticamente as apps por assinatura de stack (go.mod, package.json, ...)
kv workspace scan

# 3. Confere o que foi registrado (nome, ID, caminho, stack)
kv workspace show
```

Os IDs gerados (ex: `api-payments`, `web-frontend`) são usados para abrir sessões que atravessam só as apps necessárias ao objetivo:

```bash
# 4. Abre a sessão vinculando apenas as apps envolvidas
kv session start --goal "Sincronizar contrato de API entre payments e frontend" \
  --apps api-payments,web-frontend --provider opencode

# 5-9. o restante do fluxo é idêntico ao de um projeto único
kv session validate --session <id-gerado>
kv context build --session <id-gerado>
kv run --session <id-gerado> --dry-run
kv run --session <id-gerado>
kv session report --session <id-gerado>
```

O `session start` mostra o ID gerado (`sess-<timestamp>-<hex>`) na saída de `Session started successfully!`; use-o nos comandos seguintes.

---

## Guia de Uso: Multi-App & Sessões (MVP Completo)

Referência detalhada de cada comando do pipeline acima (workspace, sessão, contexto, execução, quality gates, diff, relatório).

### Fluxo Básico de Execução (Ponta a Ponta)

```bash
# 1. Escanear o workspace para detectar aplicações e stacks
kv workspace scan

# 2. Inicializar o contrato de uma nova sessão (--provider aceita opencode, claude-code ou codex; padrão: opencode)
kv session init --id session-auth-refactor --goal "Refatorar autenticação do serviço" --apps api-payments=./apps/api-payments --vault ./vault/auth --writable ./apps/api-payments --provider claude-code

# 3. Validar se as regras da sessão estão corretas (existência de caminhos, permissões, etc.)
kv session validate --session session-auth-refactor

# 4. Compilar o contexto da sessão (context.md, opencode.md)
kv context build --session session-auth-refactor

# 5. Executar dry run para simulação do contexto
kv run --session session-auth-refactor --dry-run

# 6. Executar o fluxo da sessão (limites físicos, logs de auditoria)
kv run --session session-auth-refactor

# 7. Rodar testes e verificações de qualidade
kv quality run --session session-auth-refactor

# 8. Gerar resumo do diff de alterações
kv diff summarize --session session-auth-refactor

# 9. Gerar o relatório final consolidado da sessão
kv session report --session session-auth-refactor
```

### Detalhamento dos Recursos

#### 1. Mapeamento Automático (`kv workspace scan`)
Varre o repositório em busca de arquivos de assinatura de stacks (`go.mod`, `package.json`, `pom.xml`, `requirements.txt`, `Dockerfile`) e registra de forma automatizada as aplicações e suas stacks em `kv-workspace.yaml`.

#### 2. Contratos de Sessão (`kv session init` / `kv session start`)
Cria o arquivo `.kv/sessions/<session-id>/session.yaml` contendo:
- **`apps`**: Mapeamento das aplicações envolvidas.
- **`vault`**: Fontes de documentação/conhecimento associadas.
- **`boundary`**: Caminhos permitidos (`allowed_paths`), caminhos de escrita (`writable_paths`) e caminhos readonly (`readonly_paths`).
- **`quality`**: Comandos de testes automatizados inferidos a partir da stack (ex: `go test ./...` ou `npm test`).
- **`policy`**: Restrições do Policy Engine (redes, leitura de envs, permissão de exclusão, docker, etc.).

Também cria a estrutura inicial de diretórios e arquivos de sessão:
```txt
.kv/
  sessions/
    session-auth-refactor/
      session.yaml
      context.md
      opencode.md
      audit.jsonl
      report.md
```

#### 3. Validação do Contrato (`kv session validate`)
Analisa a integridade da configuração da sessão e garante que:
- Todos os caminhos (apps, vault, boundary) existem fisicamente no disco.
- Os caminhos de escrita (`writable_paths`) estão contidos nos caminhos permitidos (`allowed_paths`).
- Não há sobreposição conflituosa entre caminhos de leitura e escrita.

#### 4. Execução Controlada (`kv run`)
Executa o fluxo da sessão utilizando o runner selecionado.
- Com a flag `--dry-run`, exibe um resumo detalhado contendo tamanho estimado do contexto, boundaries de leitura/escrita configuradas, comandos de qualidade e status geral de validação sem executar o agente.
- Em execução normal com o runner **OpenCode** (padrão), o `kv` realiza a validação de boundaries físicas, lê o conteúdo do prompt gerado em `.kv/sessions/<session-id>/opencode.md`, e **executa o comando `opencode run "<prompt>"`** herdando o terminal interativo.
- **Seleção de Modelo**: Você pode especificar qual modelo deve ser executado usando a flag `--model <model>`. Se a flag for omitida, o runner for OpenCode e o terminal for interativo (TTY), o CLI exibirá um menu de seleção interativo para você escolher qual modelo do OpenCode deseja usar antes do início da execução.
- **Provider do Agente**: o runner efetivamente usado é `sess.agent.provider` (definido em `kv session init/start --provider`). `opencode` (padrão) executa `opencode run`; `claude-code` executa `claude -p` (modo não-interativo); `codex` executa `codex exec`. Os três seguem o mesmo fluxo: validação de boundaries, Quality Gates, Diff Summarizer e relatório final.
- Após a execução do agente, o `kv` roda os Quality Gates, o Diff Summarizer, audita os logs no arquivo append-only (`audit.jsonl`) e gera o relatório final.

#### 5. Quality Gates (`kv quality run`)
Executa os comandos de qualidade (testes unitários, linters) do contrato da sessão a partir dos diretórios de suas respectivas aplicações, validando a segurança com o Policy Engine antes da execução e registrando o resultado no log de auditoria.

#### 6. Diff Summarizer (`kv diff summarize`)
Analisa o `git diff` associado aos caminhos permitidos da sessão e compila um resumo estruturado com arquivos modificados, novas funções/estruturas, alterações nos testes e análise de riscos de regressão em `.kv/sessions/<session-id>/diff-summary.md`.

#### 7. Relatório de Progresso (`kv session report`)
Consolida o progresso geral da sessão (objetivo, status de validação física das boundaries, comandos de qualidade executados, riscos detectados, diff summary e próximos passos recomendados) em `.kv/sessions/<session-id>/report.md`.

---


## Fluxo de Trabalho por Workflows e Tasks (Estruturado/Versionável)

### 1. Criar um Novo Workflow
Crie um fluxo de trabalho estruturado para uma feature/bugfix específica:

```bash
kv workflow new feature-auth
```

### 2. Definir e Enriquecer Tasks
As tarefas residem sob `.kv/workflows/<slug>/tasks/<task-id>.md`. Para enriquecer a task com arquivos locais e runbooks do Vault:

```bash
kv task enrich feature-auth 002-add-login
```

### 3. Compilar Contexto de Task
Gere a ponte de contexto legada para o runner:

```bash
kv context build feature-auth 002-add-login
```
*Gera `.opencode/context.md` na raiz do projeto.*

### 4. Executar a Task
Executa a task no runner selecionado (`--runner opencode|claude-code|codex`, padrão `opencode`), instruindo o agente a ler o contexto gerado em `.opencode/context.md` e realizar as mudanças necessárias:
```bash
kv task run feature-auth 002-add-login --runner opencode [--model <model>]
kv task run feature-auth 002-add-login --runner claude-code [--model <model>]
kv task run feature-auth 002-add-login --runner codex [--model <model>]
```
- **Seleção Interativa**: Se você omitir o slug do workflow ou o ID da task (ex: apenas `kv task run` ou `kv task enrich`), e estiver em um terminal interativo (TTY), o CLI exibirá um menu interativo para você selecionar o workflow e a task criada correspondente, além do modelo a ser executado.

### 5. Executar o Workflow Completo (7 Fases)
Para rodar de ponta a ponta o fluxo de trabalho de 7 fases automatizado:
```bash
kv workflow run feature-auth --prompt "Refatorar autenticação usando tokens JWT" [--model <model>]
```
> `kv workflow run` ainda invoca exclusivamente o runner OpenCode em todas as 7 fases; `kv session`/`kv task run` já suportam `claude-code` e `codex` (ver seções 4 e "Execução Controlada" acima). Tornar o workflow de 7 fases agnóstico de provider é um trabalho maior, ainda não coberto nesta release.

O executor de workflows orquestrará as seguintes fases sequencialmente utilizando o runner selecionado (ex: **OpenCode**):
1. **Idea (Concepção)**: Cria a ideia básica em `idea.md`.
2. **PRD (Product Requirements Document)**: Define requisitos e escopo em `prd.md`.
3. **Specs (Technical Specification)**: Detalha a arquitetura técnica em `techspec.md` e gera as tarefas a serem executadas em `tasks/`.
4. **Implementation (Implementação)**: Executa as tarefas sequencialmente (processando cada uma com enriquecimento de contexto e execução).
5. **Review (Revisão)**: Cria templates de revisão e valida se os critérios de aceitação foram cumpridos em `reviews/`.
6. **Adjustments (Ajustes)**: Ajusta o código caso alguma tarefa tenha falhado no review.
7. **Memorize (Memorizar)**: Compila as memórias e aprendizados em `memory/` e promove os resultados para o Vault.

#### Painel de Progresso no Terminal
Ao iniciar o workflow e a cada transição de fase, o CLI renderiza um painel dinâmico no terminal exibindo o status de todas as etapas:
- `[X] Done` (em verde) para fases concluídas.
- `[>] Running` (em roxo) para a fase ativa.
- `[ ] Pending` (em cinza) para fases futuras.

#### Heartbeat de Execução
Para processos de longa duração no runner, o `kv` exibe um heartbeat de progresso no terminal a cada 5 segundos:
```text
⏳ [OpenCode] Running agents in parallel... (5s elapsed)
```
Isso fornece feedback visual contínuo e evita timeouts durante execuções pesadas.

---

## LLM Wiki (Compilação Incremental - Paradigma Karpathy)

O `kv` implementa o conceito de **LLM Wiki** proposto por Andrej Karpathy. Em vez de utilizar apenas buscas RAG reativas que recuperam pedaços de texto temporários na hora da consulta, o sistema compila e consolida de forma incremental e persistente as notas brutas em páginas de documentação canônicas (Markdown) totalmente interligadas no seu cofre de conhecimento.

### Comandos da Wiki

Para interagir com as APIs da LLM, defina a chave do Gemini no ambiente: `export GEMINI_API_KEY="sua-chave"`.

#### 1. Compilar Notas da Inbox (`kv wiki compile`)
Consome rascunhos ou notas desorganizadas colocados em `00-inbox/`. A IA decide se deve criar novas páginas (em subpastas como `04-systems/` ou `07-runbooks/`) ou mesclar novos fatos em páginas existentes, atualizando as datas de modificação no frontmatter e resolvendo contradições. Os arquivos originais são movidos para `10-references/archive/`.
```bash
# Processa as notas da inbox e atualiza a Wiki
kv wiki compile
```

#### 2. Atualizar Links Cruzados (`kv wiki link`)
Escaneia as páginas canônicas do cofre e insere de forma automática caminhos relativos de markdown (ex: `[Auth Flow](../04-systems/auth-flow.md)`) nas referências aos termos técnicos, mantendo a Wiki interligada (ideal para navegação no Obsidian ou VS Code).
```bash
# Atualiza todas as referências cruzadas da Wiki
kv wiki link
```

#### 3. Consultar a Wiki via Linha de Comando (`kv wiki ask`)
Pesquisa na Wiki local (busca híbrida: BM25 léxico + similaridade semântica via embeddings locais, com fallback automático para léxico puro quando o embedder não está disponível) e monta um contexto rico para a LLM responder à sua pergunta citando os arquivos de origem. Veja [docs/hybrid-retrieval.md](docs/hybrid-retrieval.md) para setup do modelo local e o status de validação.
```bash
kv wiki ask "Como funciona a autenticação JWT?"
```

#### 4. Interface Web Local-First (`kv wiki serve`)
Sobe uma interface Web premium local-first com tema dark e glassmorphism. Permite navegar na árvore de documentos do Vault, visualizar arquivos markdown renderizados na hora, realizar pesquisas globais ultra rápidas (**Cmd + K**) e ter um chat conversacional RAG com histórico de mensagens.
```bash
# Inicia a interface interativa (padrão: porta 8080)
kv wiki serve --port 8080
```

## Comparativo: Sessões vs. Workflows

O `kv` oferece duas maneiras complementares para organizar a execução e os testes assistidos por IA:

* **Sessões (Sessions)**: Foco operacional direto com delimitação física rígida (`boundary`) de arquivos. Os testes são executados sob demanda através de **Quality Gates** (`kv quality run`) configurados no contrato de sessão. Recomendado para correções de bugs, pequenas refatorações ou desenvolvimento ágil.
* **Workflows & Tasks**: Foco em processo, governança persistente no Git e ciclo estruturado de 7 fases (*Idea*, *PRD*, *Specs*, *Implementation*, *Review*, *Adjustments*, *Memorize*). Os testes e validações de critérios de aceitação ocorrem nativamente nas fases de *Review* e *Adjustments*. Recomendado para novas features complexas ou arquiteturas que exijam documentação técnica e auditoria de código.

*(Para um detalhamento aprofundado, acesse o capítulo 15 da Wiki Web local rodando `kv wiki serve`)*.

---

## Outros Comandos Utilitários

### Exportar ou Importar Memória Global

O store MCP global pode ser transferido sem criar ou exigir `.kv/` no projeto. O
formato JSON versionado contém metadados legíveis de sessões e memórias OpenSpec
com suas fontes (change, path lógico e hash de revisão). IDs nativos do provider,
paths absolutos, transcripts e conteúdo de snapshots não são exportados.

```bash
# Exporta somente o workspace global associado ao diretório atual.
kv data export --file ./kv-knowledge.json

# Valida todo o documento antes de importar para o store global local.
kv data import --file ./kv-knowledge.json
```

O arquivo de exportação não é sobrescrito; escolha outro caminho para preservar
um export existente.

### Busca no Vault
Para fazer buscas textuais rápidas por arquivos markdown dentro do cofre de conhecimento configurado:
```bash
kv find "padrão de autenticação"
```

### Inicialização, Associação e Doctor de Vault
```bash
kv vault init ./knowledge-vault   # Cria uma nova estrutura de vault limpa
kv vault attach ./knowledge-vault # Associa um cofre de conhecimento externo ao workspace atual (configurando caminhos relativos)
kv vault doctor                   # Valida se a estrutura do vault está saudável e completa
kv vault path                     # Imprime o caminho absoluto do vault ativo
```

### Integração Global OpenCode
```bash
kv opencode install               # Instala templates e scripts auxiliares globais em ~/.config/opencode/
kv opencode doctor                # Valida se os scripts globais estão corretamente instalados
```
