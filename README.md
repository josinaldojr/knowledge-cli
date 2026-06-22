# kv - AI Development Harness (Vault-Native)

O `kv` é uma ferramenta CLI local-first e markdown-first escrita em Go, inspirada conceitualmente no Compozy, que funciona como um harness de desenvolvimento assistido por IA. Ele permite orquestrar contexto, gerenciar workflows versionáveis, estruturar tasks com schemas baseados em frontmatter YAML, compilar context packs enriquecidos e integrar execuções diretamente com runners (como o **OpenCode**).

Esta ferramenta se conecta a um **Knowledge Vault** (Cofre de Conhecimento) compartilhado para cruzar decisões de arquitetura (ADRs), runbooks e padrões organizacionais com o código fonte do repositório local.

---

## Recursos Principais

- **Vault-Native & Local-First**: Toda a configuração do workspace e dos workflows é versionável pelo Git (markdown-first).
- **Workspace Config (`.kv/config.yaml`)**: Centraliza o apontamento para o Knowledge Vault externo de forma simples e direta.
- **Workflows Versionáveis**: Organiza os fluxos de trabalho sob `.kv/workflows/<slug>/` gerando automaticamente artefatos base (`idea.md`, `prd.md`, `techspec.md`) e subpastas para `tasks`, `context`, `reviews` e `memory`.
- **Schema de Tasks YAML**: Permite declarar metadados ricos em markdown/frontmatter para as tarefas (complexidade, dependências, runner, sources, critérios de aceitação).
- **Enriquecimento de Tasks (`kv task enrich`)**: Gera um *Context Pack* compilando a definição da task, o conteúdo dos arquivos de código referenciados em `sources`, decisões de arquitetura e runbooks do vault e o plano de validação.
- **Context Builder (`kv context build`)**: Compila o context pack ativamente gerando o arquivo `.opencode/context.md` que o OpenCode consome automaticamente como contexto.
- **Task Runner (`kv task run`)**: Dispara a execução das tasks utilizando adapters (inicialmente suportando o runner `opencode`).

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

## Guia de Uso

### 1. Inicializar o Workspace
No diretório raiz do seu repositório/projeto:

```bash
kv init
```
*Opcional: Você pode passar o parâmetro `--vault <caminho>` para já conectar o vault durante a inicialização.*

Esse comando criará o diretório `.kv/` e o arquivo `.kv/config.yaml`.

### 2. Conectar um Knowledge Vault Externo
Caso queira conectar um cofre de conhecimento em um diretório externo:

```bash
kv vault attach ../path-to-your-vault
```
*Isso validará o vault (deve conter o marcador `.kv-vault`) e atualizará a chave `vault_path` no seu `.kv/config.yaml`.*

### 3. Criar um Novo Workflow
Crie um fluxo de trabalho estruturado e versionável para uma feature/bugfix específica:

```bash
kv workflow new feature-auth
```
Esse comando criará a pasta `.kv/workflows/feature-auth/` contendo:
- `idea.md` (Visão geral da ideia)
- `prd.md` (Product Requirement Document)
- `techspec.md` (Especificação técnica e plano de validação)
- `tasks/` (Diretório para tasks em formato markdown/frontmatter, pré-populada com `001-setup.md`)
- `context/` (Context packs gerados pelo enricher)
- `reviews/` (Reviews de código e critérios de aceitação)
- `memory/` (Registros de aprendizados)

### 4. Definir e Enriquecer Tasks
As tarefas residem sob `.kv/workflows/<slug>/tasks/<task-id>.md`. Elas utilizam frontmatter YAML para definir dependências e arquivos de código fonte impactados:

```yaml
---
id: 002-add-login
title: "Implement login backend routing"
status: todo
type: feature
complexity: medium
dependencies:
  - 001-setup
agent/runner: opencode
sources:
  - src/auth/login.go
  - src/auth/session.go
acceptanceCriteria:
  - "Endpoint POST /api/login resolves successfully"
  - "Returns valid JWT token on success"
---

# Descrição da Tarefa

Adicione o roteamento e a validação de credenciais de usuário.
```

Para enriquecer a task com arquivos do repositório local, conhecimento do Vault e decisões anteriores:

```bash
kv task enrich feature-auth 002-add-login
```
*Isso criará o Context Pack compilado em `.kv/workflows/feature-auth/context/002-add-login.context.md` e alterará o status da task para `in-progress`.*

### 5. Compilar o Contexto para o Runner
Gere a ponte de contexto que o OpenCode lerá ao iniciar:

```bash
kv context build feature-auth 002-add-login
```
*Esse comando lê o context pack gerado e compila o arquivo `.opencode/context.md` na raiz do projeto.*

### 6. Executar a Task com o Runner
Para preparar e executar a tarefa utilizando o runner definido:

```bash
kv task run feature-auth 002-add-login --runner opencode
```

---

## Outros Comandos Herdados/Utilitários

### Busca no Vault
Para fazer buscas textuais rápidas por arquivos markdown dentro do cofre de conhecimento configurado:
```bash
kv find "padrão de autenticação"
```

### Inicialização e Doctor de Vault
```bash
kv vault init ./knowledge-vault   # Cria uma nova estrutura de vault limpa
kv vault doctor                   # Valida se a estrutura do vault está saudável e completa
kv vault path                     # Imprime o caminho absoluto do vault ativo
```

### Integração Global OpenCode
```bash
kv opencode install               # Instala templates e scripts auxiliares globais em ~/.config/opencode/
kv opencode doctor                # Valida se os scripts globais estão corretamente instalados
```
