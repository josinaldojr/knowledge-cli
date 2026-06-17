package opencode

// FindScriptTemplate is the content of the kv-find.ps1 script.
const FindScriptTemplate = `param(
  [string]$Start = (Get-Location).Path
)

$VaultMarker = ".kv-vault"

function Is-KnowledgeVault {
  param([string]$Path)

  if (-not (Test-Path $Path -PathType Container)) {
    return $false
  }

  $marker = Join-Path $Path $VaultMarker

  if (-not (Test-Path $marker -PathType Leaf)) {
    return $false
  }

  return $true
}

function Resolve-PathSafe {
  param(
    [string]$Base,
    [string]$Value
  )

  if ([string]::IsNullOrWhiteSpace($Value)) {
    return $null
  }

  if ([System.IO.Path]::IsPathRooted($Value)) {
    return [System.IO.Path]::GetFullPath($Value)
  }

  return [System.IO.Path]::GetFullPath((Join-Path $Base $Value))
}

function Try-EmitVault {
  param([string]$Candidate)

  if ([string]::IsNullOrWhiteSpace($Candidate)) {
    return $false
  }

  $full = [System.IO.Path]::GetFullPath($Candidate)

  if (Is-KnowledgeVault $full) {
    Write-Output $full
    return $true
  }

  return $false
}

# 1. Environment variable wins
if ($env:KNOWLEDGE_VAULT_PATH) {
  $candidate = [System.IO.Path]::GetFullPath($env:KNOWLEDGE_VAULT_PATH)

  if (Is-KnowledgeVault $candidate) {
    Write-Output $candidate
    exit 0
  }

  Write-Error "KNOWLEDGE_VAULT_PATH is set but does not point to a valid Knowledge Vault. Expected marker '$VaultMarker' at: $candidate"
  exit 1
}

$current = [System.IO.DirectoryInfo]::new([System.IO.Path]::GetFullPath($Start))

while ($null -ne $current) {
  # 2. Workspace marker: .knowledge-vault
  # This file should contain either an absolute path or a path relative to the directory where the marker exists.
  $workspaceMarker = Join-Path $current.FullName ".knowledge-vault"

  if (Test-Path $workspaceMarker -PathType Leaf) {
    $raw = (Get-Content $workspaceMarker -Raw).Trim()
    $candidate = Resolve-PathSafe $current.FullName $raw

    if (Try-EmitVault $candidate) {
      exit 0
    }

    Write-Error ".knowledge-vault points to an invalid Knowledge Vault. Expected marker '$VaultMarker' at: $candidate"
    exit 1
  }

  # 3. Current directory itself may be the vault
  if (Try-EmitVault $current.FullName) {
    exit 0
  }

  # 4. Direct child named knowledge-vault
  $directCandidate = Join-Path $current.FullName "knowledge-vault"

  if (Try-EmitVault $directCandidate) {
    exit 0
  }

  $current = $current.Parent
}

Write-Error "Knowledge Vault not found.

Fix one of the following:

1. Set KNOWLEDGE_VAULT_PATH to your vault path:
   setx KNOWLEDGE_VAULT_PATH ""C:\path\to\knowledge-vault""

2. Create a .knowledge-vault marker file in your workspace containing the path to your vault.

3. Initialize a vault with:
   kv vault init ./knowledge-vault

A valid vault must contain the marker file: $VaultMarker"
exit 1
`

// InitTemplate is the content of knowledge-init.md
const InitTemplate = `---
description: Inicializa um novo repo conectado ao Knowledge Vault
---

Descubra o Knowledge Vault usando a saída abaixo:

!` + "`" + `powershell -ExecutionPolicy Bypass -File "$env:USERPROFILE\.config\opencode\scripts\kv-find.ps1"` + "`" + `

Use o caminho retornado acima como Knowledge Vault.

Inicialize este repositório para usar o Knowledge Vault como fonte canônica de conhecimento.

O comando deve:
1. Analisar o repo atual.
2. Identificar ou inferir Project, System, Domain, Repo Type, Primary Language e Stack.
3. Criar ou propor arquivos mínimos no vault.
4. Criar ou atualizar AGENTS.md no repo atual.
5. Criar .migration/knowledge-init-report.md.

Regras:
- Não invente informações com baixa confiança.
- Se não conseguir identificar algo, marque como Needs Review.
- Não sobrescreva arquivos existentes sem preservar conteúdo.
- Não apagar arquivos.
- Não alterar código.
- Criar notas novas com status: needs-review.

Gere o relatório em:

.migration/knowledge-init-report.md
`

// StartTemplate is the content of knowledge-start.md
const StartTemplate = `---
description: Inicia um novo repo usando conhecimento existente do Knowledge Vault
---

Descubra o Knowledge Vault usando a saída abaixo:

!` + "`" + `powershell -ExecutionPolicy Bypass -File "$env:USERPROFILE\.config\opencode\scripts\kv-find.ps1"` + "`" + `

Use o caminho retornado acima como Knowledge Vault.

Inicialize este repositório usando conhecimento existente do Knowledge Vault como base.

Entrada esperada:

/knowledge-start project=<project> sources=<source-systems> target=<target-system> intent="<descrição do que será construído>"

Argumentos aceitos:
- project
- sources
- target
- intent
- stack
- constraints
- output

O comando deve:
1. Validar se project existe em 03-projects/{project}/.
2. Validar se cada source existe em 04-systems/{source}/.
3. Criar ou propor o sistema target em 04-systems/{target}/.
4. Ler o manifest do projeto.
5. Ler notas relevantes dos sistemas source.
6. Interpretar a intenção do usuário.
7. Gerar ou atualizar AGENTS.md no repo atual.
8. Gerar .context/source-knowledge-context.md.
9. Gerar .context/bootstrap-plan.md.
10. Gerar .migration/knowledge-start-report.md.

Regras:
- Não assumir tipo de aplicação sem evidência.
- Não hardcodar tipo de aplicação.
- Inferir o tipo do target a partir de intent, target, stack e arquivos existentes.
- Se a inferência tiver baixa confiança, marcar como Needs Review.
- Não inventar endpoints, payloads, status codes, fluxos, entidades ou decisões.
- Não apagar arquivos.
- Não alterar código sem instrução explícita.

Gere o relatório em:

.migration/knowledge-start-report.md
`

// PlanTemplate is the content of knowledge-plan.md
const PlanTemplate = `---
description: Planeja a migração de docs do repo atual para o Knowledge Vault
---

Descubra o Knowledge Vault usando a saída abaixo:

!` + "`" + `powershell -ExecutionPolicy Bypass -File "$env:USERPROFILE\.config\opencode\scripts\kv-find.ps1"` + "`" + `

Use o caminho retornado acima como Knowledge Vault.

Analise este repositório e planeje a migração da documentação existente para o Knowledge Vault global.

Tarefas:
1. Encontrar arquivos de documentação relevantes.
2. Classificar cada arquivo encontrado.
3. Sugerir destino real dentro do Knowledge Vault encontrado.
4. Identificar duplicidades, contradições e documentos obsoletos.
5. Propor um knowledge-manifest.md.
6. Propor um AGENTS.md mínimo para este repo usando caminho relativo até o vault.
7. Listar quais docs devem permanecer no repo.

Regras:
- Não altere arquivos.
- Não apague arquivos.
- Não invente conhecimento.
- Se faltar informação, marque como Needs Review.
- Se o script não localizar o vault, pare e explique como configurar KNOWLEDGE_VAULT_PATH ou .knowledge-vault.

Gere o resultado em:

.migration/knowledge-migration-plan.md
`

// MigrateTemplate is the content of knowledge-migrate.md
const MigrateTemplate = `---
description: Aplica o plano de migração para o Knowledge Vault
---

Descubra o Knowledge Vault usando a saída abaixo:

!` + "`" + `powershell -ExecutionPolicy Bypass -File "$env:USERPROFILE\.config\opencode\scripts\kv-find.ps1"` + "`" + `

Use o caminho retornado acima como Knowledge Vault.

Aplique a migração descrita em .migration/knowledge-migration-plan.md usando o Knowledge Vault encontrado.

Regras:
- Copiar primeiro, nunca mover.
- Criar arquivos Markdown no Knowledge Vault encontrado.
- Adicionar frontmatter YAML em cada nota canônica.
- Preservar source_repo e source_path.
- Criar ou atualizar knowledge-manifest.md.
- Criar ou atualizar AGENTS.md no repo atual.
- O AGENTS.md deve usar caminho relativo entre o repo atual e o vault encontrado.
- Não apagar docs antigos.
- Marcar conflitos como Needs Review.
- Não inventar decisões arquiteturais.
- Não alterar código.

Ao final, gerar:

.migration/knowledge-migration-report.md
`

// ValidateTemplate is the content of knowledge-validate.md
const ValidateTemplate = `---
description: Valida estrutura de conhecimento do projeto
---

Descubra o Knowledge Vault usando a saída abaixo:

!` + "`" + `powershell -ExecutionPolicy Bypass -File "$env:USERPROFILE\.config\opencode\scripts\kv-find.ps1"` + "`" + `

Use o caminho retornado acima como Knowledge Vault.

Valide a estrutura de conhecimento deste projeto usando o Knowledge Vault encontrado.

Verifique:
1. Se existe AGENTS.md no repo.
2. Se o AGENTS.md aponta para o Knowledge Vault correto.
3. Se o projeto possui knowledge-manifest.md.
4. Se as notas citadas existem.
5. Se notas canônicas possuem frontmatter.
6. Se há links quebrados.
7. Se há documentação duplicada entre repo e vault.
8. Se há seções Needs Review.
9. Se o caminho relativo no AGENTS.md está correto.

Não altere arquivos.

Gere um relatório em:

.migration/knowledge-validation-report.md
`

// UpdateTemplate is the content of knowledge-update.md
const UpdateTemplate = `---
description: Analisa mudanças do repo e sugere atualizações no Knowledge Vault
---

Descubra o Knowledge Vault usando a saída abaixo:

!` + "`" + `powershell -ExecutionPolicy Bypass -File "$env:USERPROFILE\.config\opencode\scripts\kv-find.ps1"` + "`" + `

Use o caminho retornado acima como Knowledge Vault.

Analise as mudanças atuais deste repositório e gere um relatório de impacto no Knowledge Vault.

Antes de analisar:
1. Leia o AGENTS.md deste repo, se existir.
2. Extraia Project, System, Domain, Repo Type, Knowledge Manifest e Knowledge Impact Map.
3. Localize e leia o knowledge-manifest.md correspondente no Knowledge Vault.
4. Não invente projeto/sistema. Se não conseguir identificar, marque como Needs Review.

Analise:
- git status
- git diff --stat
- git diff
- arquivos alterados, criados ou removidos
- AGENTS.md
- knowledge-manifest.md
- notas do vault diretamente sugeridas pelo Knowledge Impact Map

Recomende atualização do Knowledge Vault quando houver:
- mudança de regra de negócio;
- mudança de contrato público;
- mudança de endpoint, payload, status code ou erro;
- mudança de schema, migration, relacionamento ou índice;
- mudança de autenticação, autorização, sessão, token ou RBAC;
- mudança arquitetural ou nova dependência relevante;
- mudança em deploy, Docker, env vars, CI/CD ou observabilidade;
- mudança em fluxo crítico de usuário;
- decisão técnica que precisará ser lembrada depois.

Não recomende atualização para:
- formatação;
- lint;
- rename sem mudança semântica;
- refatoração interna sem mudança de comportamento;
- arquivo gerado;
- ajuste pequeno sem impacto em produto, contrato, arquitetura ou operação.

Ações possíveis:
- no-action
- review
- update
- create
- needs-adr
- needs-manifest-update
- needs-human-decision

Regras:
- Não altere arquivos.
- Não escreva no Knowledge Vault.
- Não invente documentação ausente.
- Não invente decisões arquiteturais.
- Toda recomendação deve ter evidência concreta.

Gere o relatório em:

.migration/knowledge-update-report.md
`
