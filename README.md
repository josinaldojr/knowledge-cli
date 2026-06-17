# kv - CLI do Knowledge Vault

O `kv` é uma ferramenta CLI em Go desenvolvida para inicializar, linkar, descobrir, validar e instalar comandos relacionados a um **Knowledge Vault** (Cofre de Conhecimento) compartilhado, integrado com o editor **Obsidian** e o assistente de agentes **OpenCode**.

Esta ferramenta soluciona o problema de sincronização e descoberta de caminhos do Knowledge Vault em novas máquinas ou diferentes workspaces.

---

## Recursos Principais

* **Descoberta Dinâmica de Vaults**: Localiza o cofre de conhecimento a partir do diretório de trabalho atual respeitando variáveis de ambiente (`KNOWLEDGE_VAULT_PATH`), arquivos marcadores locais (`.knowledge-vault`) ou subdiretórios padrão.
* **Instalação para o OpenCode**: Provisiona automaticamente os comandos e scripts necessários em `~/.config/opencode`.
* **Validação (Doctor)**: Garante a consistência estrutural tanto do cofre quanto das integrações globais do OpenCode.

---

## Instalação Local

Certifique-se de que possui o Go (versão 1.16 ou superior) instalado em sua máquina. Para compilar e instalar globalmente a CLI, execute:

```powershell
go install ./cmd/kv
```

*Nota: Garanta que o diretório `$GOPATH/bin` ou `~/go/bin` esteja adicionado à variável de ambiente `PATH` do seu sistema.*

---

## Guia de Uso Rápido

### 1. Criar um novo Vault
Inicializa a estrutura recomendada de pastas e arquivos base de um Knowledge Vault em um diretório específico:

```powershell
kv vault init .\knowledge-vault
```

Isso criará uma estrutura contendo o arquivo de identidade `.kv-vault` e pastas organizadas (`00-inbox` a `10-references`), além de notas modelo para agentes, ADRs e especificações globais.

### 2. Conectar um Workspace (Linkar)
No diretório de trabalho do seu projeto atual, crie um arquivo marcador de ponte `.knowledge-vault` que aponte para o seu cofre recém-criado:

```powershell
kv workspace init --vault .\knowledge-vault
```

*Se o marcador `.knowledge-vault` já existir no diretório, a CLI automaticamente criará um backup em `.knowledge-vault.bak` antes de substituí-lo.*

### 3. Verificar o Caminho Ativo
Para inspecionar qual caminho de Knowledge Vault está ativo e configurado a partir do seu diretório atual:

```powershell
kv vault path
```

### 4. Executar Diagnósticos do Vault (Doctor)
Para validar a integridade estrutural e de arquivos recomendados do seu Knowledge Vault ativo:

```powershell
kv vault doctor
```

---

## Integração com OpenCode

### Instalação de Scripts e Comandos Globais
A CLI automatiza a escrita do script auxiliar `kv-find.ps1` e dos comandos customizados markdown em `~/.config/opencode/`:

```powershell
kv opencode install
```

Este comando criará:
* `~/.config/opencode/opencode.json` (caso não exista)
* `~/.config/opencode/scripts/kv-find.ps1` (finder portátil)
* 6 Comandos em `~/.config/opencode/commands/` (`knowledge-init.md`, `knowledge-start.md`, `knowledge-plan.md`, `knowledge-migrate.md`, `knowledge-validate.md`, `knowledge-update.md`).

Se os arquivos MD ou PS1 já existirem, serão criados backups com extensão `.bak` antes da atualização.

### Validar Integração OpenCode
Para conferir se todos os componentes e comandos do OpenCode foram instalados e estão acessíveis:

```powershell
kv opencode doctor
```

### Usando no OpenCode
Após a instalação global, navegue até a pasta de qualquer projeto e inicie o console do OpenCode:

```powershell
cd .\algum-projeto
opencode
```

Dentro do prompt do OpenCode, os seguintes comandos com suporte a descoberta de contexto de conhecimento estarão disponíveis para uso imediato:

* `/knowledge-init` — Inicializa um novo repositório apontando para o vault.
* `/knowledge-start project=meu-projeto sources=api-principal target=frontend intent="criar app web"` — Inicia um novo módulo consumindo serviços existentes do vault.
* `/knowledge-plan` — Analisa o repositório atual e planeja o que deve ser migrado para o vault.
* `/knowledge-migrate` — Executa a migração proposta e atualiza o manifesto.
* `/knowledge-validate` — Valida consistência de links, manifestos e referências locais ao vault.
* `/knowledge-update` — Analisa alterações do git diff e recomenda quais decisões/regras de negócio devem ser registradas no vault.
