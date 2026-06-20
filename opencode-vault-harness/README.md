# OpenCode Vault Harness

CLI que inicializa um workspace OpenCode conectado a um knowledge-vault local.

## Instalação

```bash
cd opencode-vault-harness
npm install
npm run build
```

(Opcional) Link global para usar `kv` em qualquer diretório:
```bash
npm link
```

## Comandos

### `kv init`

Inicializa um workspace OpenCode no diretório atual, conectado a um knowledge-vault.

```bash
kv init --vault ~/knowledge-vault
```

Isso cria:
- `.kv/config.json` → armazena o caminho do vault
- `.opencode/` → templates de agentes e comandos

### `kv find`

Busca documentos no vault por termo de pesquisa.

```bash
kv find "autenticação google"
```

Retorna lista de documentos relevantes com título, score e trecho.

### `kv context`

Gera `.opencode/context.md` com contexto do vault para uma tarefa específica.

```bash
kv context "criar login social com google"
```

O OpenCode lê este arquivo automaticamente ao iniciar no diretório.

## Como Usar com OpenCode

1. **Inicialize o workspace:**
   ```bash
   mkdir meu-projeto && cd meu-projeto
   kv init --vault ~/knowledge-vault
   ```

2. **Gere o contexto para sua tarefa:**
   ```bash
   kv context "implementar sistema de autenticação"
   ```

3. **Abra com OpenCode:**
   ```bash
   opencode .
   ```

4. **Use os comandos dentro do OpenCode:**
   - `/kv-plan` — cria um plano de implementação baseado no vault
   - `/kv-implement` — implementa as tarefas do plano
   - `/kv-review` — revisa a implementação contra o vault
   - `/kv-sync` — salva aprendizados de volta no vault

## Estrutura do Projeto

```
opencode-vault-harness/
├── src/
│   ├── cli.ts                      # Entry point
│   ├── commands/
│   │   ├── init.ts                 # kv init
│   │   ├── find.ts                 # kv find
│   │   └── context.ts              # kv context
│   ├── vault/
│   │   ├── search.ts               # Busca textual nos .md
│   │   ├── load-documents.ts       # Carrega todos os docs do vault
│   │   └── render-context.ts       # Gera .opencode/context.md
│   └── workspace/
│       ├── detect-workspace.ts     # Detecta .kv/config.json
│       └── write-config.ts         # Escreve .kv/config.json
├── templates/
│   └── opencode/
│       ├── AGENTS.md               # Guia do workspace
│       ├── opencode.json           # Config do OpenCode
│       ├── agents/                 # Definições dos agentes
│       │   ├── orchestrator.md
│       │   ├── knowledge.md
│       │   ├── architect.md
│       │   ├── backend.md
│       │   ├── frontend.md
│       │   └── reviewer.md
│       └── commands/               # Comandos /kv-*
│           ├── kv-plan.md
│           ├── kv-implement.md
│           ├── kv-review.md
│           └── kv-sync.md
├── package.json
├── tsconfig.json
└── README.md
```

## Requisitos

- Node.js 18+
- npm 9+

## Compatibilidade

Windows, macOS e Linux.

## Licença

MIT
