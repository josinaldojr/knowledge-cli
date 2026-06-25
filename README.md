# kv - AI Development Harness (Vault-Native)

O `kv` é uma ferramenta CLI local-first e markdown-first escrita em Go que funciona como um harness de desenvolvimento assistido por IA. Ele permite orquestrar contexto, gerenciar workflows versionáveis, estruturar tasks com schemas baseados em frontmatter YAML, compilar context packs enriquecidos e integrar execuções diretamente com runners (como o **OpenCode**).

Esta ferramenta se conecta a um **Knowledge Vault** (Cofre de Conhecimento) compartilhado para cruzar decisões de arquitetura (ADRs), runbooks e padrões organizacionais com o código fonte do repositório local.

---

## Recursos Principais

- **Vault-Native & Local-First**: Toda a configuração do workspace e dos workflows é versionável pelo Git (markdown-first).
- **Workspace Config (`kv-workspace.yaml`)**: Declara e centraliza o mapeamento de múltiplos microsserviços/aplicações no projeto atual de forma declarativa.
- **Sessões Multi-App**: Permite criar sessões operacionais focadas em objetivos específicos, vinculando apenas as aplicações necessárias e delimitando o espaço físico de leitura/escrita do Agent (`boundary.allowed_paths`).
- **Context Builder Enriquecido**: Compila o contexto geral da sessão, gerando manifestos JSON, árvores de arquivos recursivas com limites de profundidade e exclusão de pastas pesadas (como `node_modules`, `dist`, `.git`), e sugere arquivos candidatos a alteração por proximidade ao objetivo.
- **Workflows Versionáveis (Legado)**: Organiza os fluxos de trabalho sob `.kv/workflows/<slug>/` gerando artefatos base (`idea.md`, `prd.md`, `techspec.md`).

---

## Instalação

Certifique-se de ter o Go instalado (versão 1.16 ou superior). No diretório do projeto, execute:

```bash
go build -o kv ./cmd/kv
```

Ou instale globalmente no seu sistema:

```bash
go install ./cmd/kv
```

---

## Guia de Uso: Multi-App & Sessões (MVP Completo)

Abaixo, descrevemos o fluxo completo de ponta a ponta para inicializar um workspace, abrir sessões de desenvolvimento delimitadas por boundaries físicas e políticas de segurança, compilar o contexto, validar a execução automatizada com testes (quality gates) e gerar relatórios de progresso auditados.

### Fluxo Básico de Execução (Ponta a Ponta)

```bash
# 1. Escanear o workspace para detectar aplicações e stacks
kv workspace scan

# 2. Inicializar o contrato de uma nova sessão
kv session init --id session-auth-refactor --goal "Refatorar autenticação do serviço" --apps api-payments=./apps/api-payments --vault ./vault/auth --writable ./apps/api-payments

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
- Em execução normal, valida as boundaries, escreve eventos no log de auditoria append-only (`audit.jsonl`), executa os Quality Gates, o Diff Summarizer e gera o relatório final.

#### 5. Quality Gates (`kv quality run`)
Executa os comandos de qualidade (testes unitários, linters) do contrato da sessão a partir dos diretórios de suas respectivas aplicações, validando a segurança com o Policy Engine antes da execução e registrando o resultado no log de auditoria.

#### 6. Diff Summarizer (`kv diff summarize`)
Analisa o `git diff` associado aos caminhos permitidos da sessão e compila um resumo estruturado com arquivos modificados, novas funções/estruturas, alterações nos testes e análise de riscos de regressão em `.kv/sessions/<session-id>/diff-summary.md`.

#### 7. Relatório de Progresso (`kv session report`)
Consolida o progresso geral da sessão (objetivo, status de validação física das boundaries, comandos de qualidade executados, riscos detectados, diff summary e próximos passos recomendados) em `.kv/sessions/<session-id>/report.md`.

---


## Fluxo de Trabalho por Tasks (Legado)

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
```bash
kv task run feature-auth 002-add-login --runner opencode
```

---

## Outros Comandos Utilitários

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
