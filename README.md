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

## Guia de Uso: Multi-App & Sessões (MVP)

Abaixo, descrevemos o fluxo completo para inicializar um workspace multi-app, registrar aplicações, abrir sessões de desenvolvimento delimitadas e compilar os contextos.

### 1. Inicializar o Workspace Declarativo
Cria o arquivo `kv-workspace.yaml` na raiz do seu diretório atual:

```bash
kv workspace init
```
*Se você deseja linkar também um Knowledge Vault legado de forma simultânea:*
```bash
kv workspace init --vault ../path-to-your-vault
```
*(Se o arquivo `kv-workspace.yaml` já existir, o CLI solicitará confirmação interativa antes de sobrescrevê-lo).*

### 2. Registrar Aplicações
Adicione os microsserviços ou aplicações que compõem o repositório ao workspace atual. Todas as flags são obrigatórias:

```bash
kv app add --id api-backend --name "Backend API" --path src/backend --type service --stack go
kv app add --id web-portal --name "Frontend Portal" --path src/frontend --type frontend --stack nextjs
```
*O comando valida duplicidade de IDs e garante que os caminhos (`--path`) realmente existem no disco.*

### 3. Visualizar Configurações do Workspace
Para ver o workspace declarativo ativo e todas as aplicações listadas:

```bash
# Ver detalhes estruturados do workspace
kv workspace show

# Listar aplicações cadastradas em formato de tabela
kv app list
```

### 4. Iniciar Sessões Operacionais
Crie uma sessão focada em um objetivo de desenvolvimento delimitado. Você deve especificar o objetivo (`--goal`) e as aplicações envolvidas (`--apps` separadas por vírgula):

```bash
kv session start --goal "Refatorar JWT e autenticação" --apps api-backend
```
*Isso criará uma pasta `.kv/sessions/<session-id>/session.yaml` contendo a data de criação, o objetivo, o status `active` e os limites de segurança físicos (`boundary.allowed_paths`), rejeitando apps não registrados.*

### 5. Compilar o Contexto da Sessão (Context Builder)
Gere toda a estrutura de contextos Markdown de que a inteligência artificial precisa para atuar na sessão:

```bash
kv context build --session <session-id>
```
Este comando compilará e salvará os seguintes arquivos na pasta da sessão:
- `.kv/sessions/<session-id>/context/global.context.md`: Visão geral do objetivo da sessão, lista de apps, boundaries e regras gerais para o Agent.
- `.kv/sessions/<session-id>/context/apps/<app-id>.context.md`: Detalhes da aplicação, árvore de arquivos filtrada (ignorando pastas gigantes e com limite de profundidade de até 4 níveis), resumo do vault associado (se existir e limitado a 10KB de leitura) e arquivos sugeridos/candidatos para alteração.
- `.kv/sessions/<session-id>/context/context-manifest.json`: Manifesto de mapeamento estruturado da sessão.

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
