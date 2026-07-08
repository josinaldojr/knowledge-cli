# Interface de Usuário de Terminal (TUI)

O `kv` CLI possui uma **Interface de Usuário de Terminal (TUI)** interativa, construída sobre as bibliotecas Bubble Tea e Lip Gloss do ecossistema Charm. Ela permite gerenciar workspaces, controlar sessões, configurar e disparar tasks, e rodar comandos da LLM Wiki sem precisar digitar comandos longos com dezenas de flags.

---

## 🚀 Como Iniciar a TUI

Para abrir o painel interativo, execute o comando abaixo a partir do seu terminal no diretório do projeto:

```bash
kv tui
```

*Dica: Como a TUI é o comportamento padrão do CLI, apenas digitar `kv` sem nenhum argumento também abrirá o painel interativo.*

---

## 🎨 Layout e Divisão da Tela

A interface é dividida em quatro partes principais:

```txt
┌────────────────────────────────────────────────────────────────────────┐
│  KV AI DEVELOPMENT HARNESS  [Status: Workspace / Vault / Sessions]    │ <-- 1. Cabeçalho
├──────────────────────────────┬─────────────────────────────────────────┤
│ CATEGORIES                   │ DETALHES DO COMANDO / FORMULÁRIO        │
│ 📁 Workspace & Apps          │                                         │
│ 📁 Knowledge Vault           │ Mostra a descrição, o template de       │
│ 📁 Sessions                  │ comando real do CLI, e campos de        │ <-- 2 e 3. Sidebar
│                              │ formulário para preencher os argumentos │      e Painel
│ COMMANDS                     │ e flags necessárias para execução.      │      de Conteúdo
│ ▶ Initialize Workspace       │                                         │
│   Scan Apps                  │                                         │
└──────────────────────────────┴─────────────────────────────────────────┘
│ help text: arrows: navigate • tab: edit form • enter: run • q: quit    │ <-- 4. Rodapé
└────────────────────────────────────────────────────────────────────────┘
```

1. **Cabeçalho (Header)**: Barra superior contendo o status atualizado do workspace ativo (nome do projeto, caminho físico do Knowledge Vault associado e contagem de sessões salvas).
2. **Barra Lateral (Sidebar)**: Lista de Categorias de comandos na parte superior e a lista de comandos filtrados sob a categoria ativa na parte inferior.
3. **Painel de Conteúdo (Content Area)**: Exibe os metadados do comando selecionado ou o formulário de parâmetros. Quando executado, transforma-se no painel de console.
4. **Rodapé (Footer)**: Atalhos rápidos de teclado baseados no contexto em que você está navegando.

---

## ⌨️ Atalhos de Teclado e Navegação

### 1. Navegando na Sidebar (Seleção de Comandos)
Quando o foco está na barra lateral esquerda (indicado pelo indicador `▶` nos comandos):
- **Setas `↑` / `↓`**: Navega pela lista de comandos da categoria ativa.
- **Setas `←` / `→`**: Alterna entre as categorias de comandos (ex: pula de "Workspace & Apps" para "Sessions").
- **`TAB`**: Move o foco para o painel de formulário/conteúdo à direita para configurar parâmetros.
- **`ENTER`**: Seleciona o comando. Se o comando não tiver parâmetros, ele é executado imediatamente; se possuir parâmetros, abre o formulário.
- **`q`** ou **`Ctrl + C`**: Sai da TUI.

### 2. Preenchendo o Formulário de Parâmetros
Quando o foco está no painel de formulário à direita (inserindo flags e argumentos):
- **`TAB` / `Shift + TAB`**: Avança ou retrocede entre os campos do formulário (inputs de texto, booleans, dropdowns, etc.).
- **Setas `↑` / `↓` ou `j` / `k`**: Navega entre as opções disponíveis em campos de seleção dinâmica (dropdowns), como escolher workflows criados, tasks criadas ou modelos do OpenCode.
- **Setas `←` / `→` ou `Espaço`**: Altera opções em campos booleanos ou confirma seleções em listas multiselect.
- **`ESC`**: Cancela o formulário e retorna o foco para a Sidebar de comandos.
- **`ENTER`**: Confirma as entradas do formulário e inicia a execução do comando.

### 3. Painel de Execução (Logs e Console)
Durante ou após a execução do comando selecionado:
- **Spinner animado (`⠋`)**: Indica que o processo está em execução em segundo plano.
- **`↑` / `↓`** ou **`j` / `k`**: Rola a caixa de logs do Console se o output exceder o tamanho visível do terminal.
- **`ENTER` / `ESC`**: Para comandos comuns, retorna para a Sidebar de comandos. Para daemons/comandos de longa execução (como `kv wiki serve`), interrompe a execução do servidor e retorna à Sidebar.
- **`Ctrl + C`**: Interrompe qualquer comando rodando imediatamente e fecha a TUI.

---

## 🛠️ Exemplo Prático: Criando uma Sessão pela TUI

Abaixo está o fluxo passo a passo para inicializar uma sessão de desenvolvimento sem digitar nenhuma flag manualmente:

1. **Abra o TUI**: Digite `kv`.
2. **Navegue até a Categoria**: Use a seta `→` até selecionar a categoria **Sessions**.
3. **Selecione o Comando**: Use a seta `↓` até destacar o comando **Initialize Session** (`session init`) e pressione `ENTER`.
4. **Preencha o Formulário**:
   - No campo **Session ID**, digite o id desejado (ex: `sess-tui-test`).
   - Pressione `TAB`. No campo **Goal**, descreva o objetivo do teste.
   - Pressione `TAB`. No campo de seleção de aplicações (**Apps**), use as setas para selecionar a aplicação correta e pressione `TAB`.
   - Configure o caminho do **Vault** e as permissões de escrita (**Writable Paths**).
5. **Execute**: Pressione `ENTER`. A tela mudará para o console de execução mostrando as etapas de criação da pasta e o contrato gerado com sucesso.
6. **Retorne**: Pressione `ESC` para voltar à lista principal.

---

---

> [!NOTE]
> Se você disparar comandos da LLM Wiki (como `wiki compile` ou `wiki ask`) de dentro da TUI, ela utilizará a variável de ambiente `GEMINI_API_KEY` do terminal pai. Certifique-se de exportá-la antes de rodar o `kv tui`.

---

## 📖 Dicionário de Comandos da TUI

Abaixo está o detalhamento de cada comando disponível no painel lateral esquerdo da TUI, organizado pelas categorias originais, acompanhado de exemplos de como configurá-los.

### 🏢 1. Workspace & Apps

Esta categoria reúne comandos para gerenciar a estrutura local do projeto e catalogar os microsserviços.

*   **Initialize AI Harness (`init`)**
    *   **O que faz**: Inicializa a pasta de configuração oculta `.kv` na raiz do diretório.
    *   **Campos na TUI**:
        *   `Vault Path` (Opcional): Caminho físico para associar um cofre existente.
    *   **Exemplo**: Deixe em branco se for criar o cofre depois, ou informe `../vault-conhecimento`.

*   **Initialize Workspace (`workspace init`)**
    *   **O que faz**: Cria o arquivo manifesto de mapeamento `kv-workspace.yaml` na raiz do diretório.
    *   **Campos na TUI**:
        *   `Vault Path` (Opcional): Associa o diretório padrão de conhecimento técnico.
    *   **Exemplo**: Pressione `ENTER` diretamente para inicializar um workspace simples na raiz.

*   **Show Workspace Status (`workspace show`)**
    *   **O que faz**: Mostra quais aplicações estão registradas no repositório, seus caminhos físicos e tecnologias identificadas.
    *   **Campos na TUI**: Não possui parâmetros. Executa imediatamente.

*   **Scan Apps (`workspace scan`)**
    *   **O que faz**: Varre o repositório em busca de assinaturas de linguagens (como `go.mod`, `package.json`, `requirements.txt`) e preenche o arquivo `kv-workspace.yaml` de forma automática.
    *   **Campos na TUI**: Não possui parâmetros. Executa imediatamente.

*   **List Apps (`app list`)**
    *   **O que faz**: Exibe no console uma lista simplificada de todas as aplicações cadastradas e ativas.
    *   **Campos na TUI**: Não possui parâmetros. Executa imediatamente.

*   **Add Application (`app add`)**
    *   **O que faz**: Cadastra de forma manual uma nova aplicação no workspace.
    *   **Campos na TUI**:
        *   `App ID` (Obrigatório): Identificador único (ex: `gateway-service`).
        *   `App Name` (Obrigatório): Nome descritivo (ex: `API Gateway`).
        *   `App Directory Path` (Obrigatório): Pasta da aplicação (ex: `./apps/gateway`).
        *   `App Type` (Obrigatório): Tipo da stack (ex: `service` ou `library`).
        *   `Tech Stack` (Obrigatório): Tecnologia subjacente (ex: `go` ou `typescript`).

---

### 📂 2. Knowledge Vault

Operações que interagem diretamente com o seu cofre de conhecimento Markdown local (decisões de arquitetura ADRs, runbooks e guidelines).

*   **Attach Knowledge Vault (`vault attach`)**
    *   **O que faz**: Associa um cofre de conhecimento externo ao workspace atual.
    *   **Campos na TUI**:
        *   `Vault Directory Path` (Obrigatório): Caminho da pasta de documentação (ex: `../meu-obsidian-vault`).

*   **Create New Vault (`vault init`)**
    *   **O que faz**: Cria a estrutura inicial de pastas recomendada de um Knowledge Vault em uma nova pasta.
    *   **Campos na TUI**:
        *   `New Vault Path` (Obrigatório): Caminho onde criar o cofre (ex: `./knowledge-base`).

*   **Show Vault Path (`vault path`)**
    *   **O que faz**: Imprime o caminho absoluto no disco para o cofre que está ativo no momento.
    *   **Campos na TUI**: Não possui parâmetros. Executa imediatamente.

*   **Validate Vault Health (`vault doctor`)**
    *   **O que faz**: Analisa se o cofre ativo possui todas as estruturas necessárias (pastas de decisões, sistemas, runbooks, templates) e se os links de Markdown estão íntegros.
    *   **Campos na TUI**: Não possui parâmetros. Executa imediatamente.

*   **Search Vault Files (`find`)**
    *   **O que faz**: Realiza uma pesquisa por correspondência textual nos arquivos de Markdown do cofre.
    *   **Campos na TUI**:
        *   `Search Query` (Obrigatório): Termos de pesquisa (ex: `docker setup`).

---

### 🚀 3. Session Management

Comandos voltados ao ciclo de vida e boundaries físicas de sessões de desenvolvimento com IA.

*   **Init Session Manually (`session init`)**
    *   **O que faz**: Inicializa o contrato de sessão (`session.yaml`) especificando caminhos detalhados.
    *   **Campos na TUI**:
        *   `Session ID` (Opcional): Nome da sessão (ex: `sess-auth-fix`).
        *   `Session Goal` (Obrigatório): Objetivo técnico detalhado.
        *   `Select Apps` (Opcional): Seletor multi-select para escolher os microsserviços associados.
        *   `Vault Sources` (Opcional): Arquivos específicos do Vault a ler (ex: `07-runbooks/deploy.md`).
        *   `Writable Paths` (Opcional): Onde a IA tem permissão física de escrita (ex: `./apps/auth`).

*   **Start Session (`session start`)**
    *   **O que faz**: Inicia uma sessão interativa simplificada, gerando os limites de boundary automaticamente com base nas aplicações selecionadas.
    *   **Campos na TUI**:
        *   `Objective / Goal` (Obrigatório): O objetivo da alteração a ser repassado ao agente.
        *   `Select Workspace Apps` (Obrigatório): Escolha múltipla das aplicações envolvidas.

*   **Run Session Agent (`run`)**
    *   **O que faz**: Executa as tarefas da sessão no runner configurado.
    *   **Campos na TUI**:
        *   `Session ID` (Obrigatório - Dropdown dinâmico): Seleciona uma das sessões em aberto.
        *   `Dry Run` (Opcional - Boolean): Se ativado (`true`), faz apenas a validação estática de contexto sem disparar o agente de IA.

*   **Validate Session Contract (`session validate`)**
    *   **O que faz**: Checa se todas as permissões de caminhos de escrita (`writable_paths`) e leitura (`allowed_paths`) da sessão são fisicamente seguras e se as pastas realmente existem.
    *   **Campos na TUI**:
        *   `Session ID` (Obrigatório - Dropdown dinâmico): A sessão a validar.

*   **Show Session Diff (`session diff`)**
    *   **O que faz**: Exibe as alterações de código feitas até o momento nos arquivos contidos no escopo da sessão.
    *   **Campos na TUI**:
        *   `Session ID` (Obrigatório - Dropdown dinâmico): A sessão ativa.
        *   `Include Untracked` (Opcional - Boolean): Se ativado, também mostra arquivos novos criados que ainda não foram comitados no Git.

*   **Generate Session Report (`session report`)**
    *   **O que faz**: Consolida o andamento e auditoria da sessão no arquivo `.kv/sessions/<id>/report.md`.
    *   **Campos na TUI**:
        *   `Session ID` (Obrigatório - Dropdown dinâmico): A sessão ativa.

---

### 🚦 4. Quality & Boundaries

Comandos focados em testar o código produzido e avaliar a conformidade de segurança e riscos.

*   **Validate Boundary Violations (`boundary validate`)**
    *   **O que faz**: Compara as alterações ativas no Git com o contrato da sessão. Caso a IA tenha alterado algum arquivo fora das `writable_paths`, ele aponta o erro de violação.
    *   **Campos na TUI**:
        *   `Session ID` (Obrigatório - Dropdown dinâmico): A sessão ativa.
        *   `Include Untracked` (Opcional - Boolean): Se deve incluir arquivos não rastreados nas checagens.

*   **Run Quality Gates (`quality run`)**
    *   **O que faz**: Dispara os comandos de testes (como `go test` ou `npm test`) dentro das pastas de cada aplicação selecionada no contrato da sessão.
    *   **Campos na TUI**:
        *   `Session ID` (Obrigatório - Dropdown dinâmico): A sessão ativa.

*   **Summarize Diff (`diff summarize`)**
    *   **O que faz**: Envia o diff de código da sessão para a LLM consolidar em um resumo estruturado de alterações de arquitetura e possíveis pontos de falha.
    *   **Campos na TUI**:
        *   `Session ID` (Obrigatório - Dropdown dinâmico): A sessão ativa.

---

### 📋 5. Tasks & Workflows

A categoria estruturada para desenvolvimento incremental focado em versionamento de tarefas no repositório.

*   **Create Workflow (`workflow new`)**
    *   **O que faz**: Inicializa a estrutura de um novo fluxo de trabalho sob `.kv/workflows/`.
    *   **Campos na TUI**:
        *   `Workflow Slug` (Obrigatório): Identificador slug (ex: `refatorar-pedidos`).

*   **Enrich Task (`task enrich`)**
    *   **O que faz**: Analisa a especificação da tarefa e compila os arquivos descritos na propriedade `sources` e runbooks associados.
    *   **Campos na TUI**:
        *   `Workflow Slug` (Obrigatório): O slug do workflow contendo a tarefa.
        *   `Task ID` (Obrigatório): Identificador da tarefa (ex: `002-add-handler`).

*   **Run Task (`task run`)**
    *   **O que faz**: Inicia o runner interativo do OpenCode configurando a ponte de contexto da tarefa em `.opencode/context.md`.
    *   **Campos na TUI**:
        *   `Workflow Slug` (Obrigatório): O slug do workflow.
        *   `Task ID` (Obrigatório): Identificador da tarefa.
        *   `Runner Type` (Opcional - Dropdown): O runner correspondente (padrão: `opencode`).

*   **Build Context (`context build`)**
    *   **O que faz**: Gera manualmente o arquivo consolidado de contexto `.opencode/context.md` de uma sessão ou de uma tarefa específica.
    *   **Campos na TUI**:
        *   `Session ID` (Opcional): ID da sessão se estiver compilando contexto de sessão.
        *   `Workflow Slug (Task Only)` (Opcional): Slug do workflow para contexto de tarefa.
        *   `Task ID (Task Only)` (Opcional): ID da tarefa para contexto de tarefa.

---

### 🧠 6. LLM Wiki

Comandos associados ao paradigma local-first da LLM Wiki (concepção Karpathy de compilação incremental de documentos técnicos).

*   **Compile Wiki Notes (`wiki compile`)**
    *   **O que faz**: Lê as notas cruas colocadas no diretório `00-inbox/` e usa a LLM para fundir ou estruturar novas páginas canônicas no cofre.
    *   **Campos na TUI**: Não possui parâmetros. Executa imediatamente.

*   **Auto Link Wiki Pages (`wiki link`)**
    *   **O que faz**: Varre todo o cofre de conhecimento inserindo links relativos de Markdown em termos técnicos que coincidem com títulos de outros tópicos da wiki.
    *   **Campos na TUI**: Não possui parâmetros. Executa imediatamente.

*   **Ask LLM Wiki (`wiki ask`)**
    *   **O que faz**: Interface de linha de comando para fazer perguntas à LLM com base nos arquivos locais do cofre de conhecimento (RAG local).
    *   **Campos na TUI**:
        *   `Question` (Obrigatório): Pergunta sobre arquitetura/padrões (ex: `Qual é o padrão de autenticação do gateway?`).

*   **Serve Web Wiki (`wiki serve`)**
    *   **O que faz**: Inicia o servidor web da wiki interativa premium e o mantém rodando.
    *   **Campos na TUI**:
        *   `Server Port` (Opcional): Porta do servidor HTTP (padrão: `8080`).
    *   **Exemplo**: Ao rodar na TUI, o console mostrará que o servidor está escutando em `http://localhost:8080`. Para desligar o servidor de dentro da TUI e retornar, pressione `ENTER` ou `ESC`.

---

### 🏃 7. OpenCode Runner

Comandos auxiliares para manter a integração do runner OpenCode e scripts de orquestração saudáveis no ambiente do usuário.

*   **Install OpenCode Integration (`opencode install`)**
    *   **O que faz**: Copia scripts utilitários e templates padrão do harness para o diretório global de configurações do usuário (`~/.config/opencode/`).
    *   **Campos na TUI**: Não possui parâmetros. Executa imediatamente.

*   **Verify Workspace Integration (`opencode doctor`)**
    *   **O que faz**: Executa diagnósticos de integridade para confirmar se o CLI do OpenCode está instalado, se o daemon responde corretamente e se há conflitos nas permissões de diretório.
    *   **Campos na TUI**: Não possui parâmetros. Executa imediatamente.

