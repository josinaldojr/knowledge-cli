# Utilitários do Vault

O **Knowledge Vault** (Cofre de Conhecimento) do `kv` segue uma estrutura semântica rígida de diretórios numerados para manter a documentação corporativa e técnica limpa e escalável.

---

## 📂 Estrutura Canônica do Vault

Um Vault saudável deve possuir a seguinte estrutura:

```txt
knowledge-vault/
  00-inbox/         # Notas brutas e rascunhos pendentes de compilação
  01-global/        # Visão geral do negócio, termos globais e conceitos macro
  02-domains/       # Domínios de negócio e fronteiras lógicas
  03-projects/      # Iniciativas e metas ativas
  04-systems/       # Documentações de arquitetura de software e microsserviços
  05-decisions/     # Decisões de Arquitetura (ADRs) estruturadas
  06-agents/        # Instruções de comportamento e personas para IAs
  07-runbooks/      # Guias de operação passo a passo e procedimentos de incidentes
  08-prompts/       # Prompts estruturados e templates reutilizáveis
  09-templates/     # Modelos de ADRs, PRDs e especificações técnicas
  10-references/    # Materiais externos, links e arquivos arquivados (archive/)
```

---

## 🛠️ Comandos de Gerenciamento

O `kv` fornece ferramentas de manutenção para garantir que seu cofre de conhecimento local esteja sempre íntegro.

### 1. Inicializar um Novo Vault (`kv vault init`)
Cria a estrutura de diretórios padrão descrita acima em um caminho especificado:
```bash
kv vault init ./meu-novo-vault
```

### 2. Associar um Vault Externo (`kv vault attach`)
Vincula um cofre de conhecimento externo compartilhado (ex: clonado de outro repositório Git) ao workspace atual:
```bash
kv vault attach ../repositorio-vault-global
```

### 3. Diagnóstico de Saúde do Vault (`kv vault doctor`)
Examina o cofre associado ao workspace e valida se ele cumpre a estrutura de pastas obrigatória, alertando sobre pastas ausentes ou mal configuradas:
```bash
kv vault doctor
```

### 4. Consultar Caminho Ativo (`kv vault path`)
Imprime o caminho absoluto no disco para o cofre configurado no momento:
```bash
kv vault path
```

### 5. Busca Rápida (`kv find`)
Realiza buscas textuais rápidas diretamente dentro dos arquivos Markdown do cofre:
```bash
kv find "padrão de autenticação"
```
