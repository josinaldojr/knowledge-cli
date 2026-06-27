# Integração Global OpenCode

O `kv` funciona de forma nativa e integrada com o **OpenCode**, servindo como o compilador e fornecedor de contexto estruturado para execuções automatizadas em ambientes virtuais de codificação.

---

## 🔧 Instalando Scripts e Templates

Para instalar os scripts auxiliares globais e templates de integração no seu sistema operacional (armazenados em `~/.config/opencode/`):

```bash
kv opencode install
```

### O que é instalado?
- **Scripts de Auxílio**: Utilitários para que o OpenCode consiga se comunicar com as APIs do `kv` de forma transparente.
- **Templates de Contexto**: Arquivos que ajudam o runner a ler os arquivos de boundary e session gerados no diretório local `.kv/`.

---

## 🔍 Verificação de Instalação (`kv opencode doctor`)

Para diagnosticar se a integração global com o OpenCode está corretamente configurada na máquina de desenvolvimento:

```bash
kv opencode doctor
```

### O que o comando valida?
- Se o diretório `~/.config/opencode/` existe.
- Se os scripts obrigatórios estão com permissão de execução.
- Se o binário do `kv` está acessível a partir do path global do runner do OpenCode.

---

## 🚀 Execução Integrada (`kv run` & `kv task run`)

O `kv` não é apenas um compilador estático de contexto; ele atua como o harness de orquestração completo. Ao iniciar a execução de uma sessão ou tarefa, o adapter de runner do `kv` assume o controle e delega a execução ao agente do OpenCode de forma nativa.

### 1. Execução de Sessão (`kv run --session <id>`)
Ao executar o comando de sessão:
1. O `kv` valida as boundaries físicas no workspace.
2. Carrega o prompt de instrução gerado em `.kv/sessions/<session-id>/opencode.md`.
3. Invoca sob o capô o executável:
   ```bash
   opencode run "<conteúdo do opencode.md>"
   ```
4. Compartilha os fluxos de entrada e saída padrão (`os.Stdin`, `os.Stdout` e `os.Stderr`) diretamente com a sua sessão de terminal atual. Isso garante que o agente do OpenCode consiga pedir permissões ao usuário de forma interativa, exibir o progresso do raciocínio e aceitar cancelamento direto.
5. Após o término da execução do agente, o `kv` executa os testes de qualidade (Quality Gates), extrai o diff de alterações (Diff Summarizer) e consolida os resultados no relatório final em `report.md`.

### 2. Execução de Task (`kv task run <workflow> <task-id>`)
Para fluxos de trabalho tradicionais baseados em tarefas (legado):
1. O `kv` compila o pacote de contexto da tarefa e salva em `.opencode/context.md`.
2. Executa a ferramenta sob o capô:
   ```bash
   opencode run "Please read the task context file at `.opencode/context.md` and complete the task instructions described there."
   ```
3. O agente consome o arquivo de contexto unificado na raiz do repositório para planejar e aplicar as alterações necessárias.
