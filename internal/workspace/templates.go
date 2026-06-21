package workspace

// OpencodeJsonTemplate contains the default opencode.json configuration template.
const OpencodeJsonTemplate = `{
  "workspace": {
    "context": ".opencode/context.md",
    "agents": ".opencode/agents",
    "commands": ".opencode/commands"
  },
  "agents": {
    "orchestrator": {
      "file": ".opencode/agents/orchestrator.md"
    },
    "knowledge": {
      "file": ".opencode/agents/knowledge.md"
    },
    "architect": {
      "file": ".opencode/agents/architect.md"
    },
    "backend": {
      "file": ".opencode/agents/backend.md"
    },
    "frontend": {
      "file": ".opencode/agents/frontend.md"
    },
    "reviewer": {
      "file": ".opencode/agents/reviewer.md"
    }
  },
  "commands": {
    "/kv-plan": {
      "file": ".opencode/commands/kv-plan.md"
    },
    "/kv-implement": {
      "file": ".opencode/commands/kv-implement.md"
    },
    "/kv-review": {
      "file": ".opencode/commands/kv-review.md"
    },
    "/kv-sync": {
      "file": ".opencode/commands/kv-sync.md"
    }
  }
}
`

// AgentsMdTemplate contains the default AGENTS.md template for workspace overview.
const AgentsMdTemplate = `# OpenCode Agents

Este workspace está configurado com agentes especiais para trabalhar com um knowledge-vault.

## Comandos Disponíveis

### ` + "`/kv-plan`" + `
Analisa o contexto gerado e cria um plano de implementação.

### ` + "`/kv-implement`" + `
Implementa as tarefas do plano, consultando o vault como referência.

### ` + "`/kv-review`" + `
Revisa o código implementado, comparando com as definições do vault.

### ` + "`/kv-sync`" + `
Atualiza o vault com novos aprendizados da implementação atual.

## Como Usar

1. Execute ` + "`kv context \"sua tarefa\"`" + ` para gerar o contexto do vault.
2. Use ` + "`/kv-plan`" + ` para planejar a implementação.
3. Use ` + "`/kv-implement`" + ` para executar o plano.
4. Use ` + "`/kv-review`" + ` para validar a implementação.
5. Use ` + "`/kv-sync`" + ` para salvar novos conhecimentos no vault.

## Estrutura do Projeto

` + "```" + `
.kv/config.json       → Conexão com o vault
.opencode/context.md  → Contexto gerado automaticamente
.opencode/agents/     → Definições dos agentes
.opencode/commands/   → Comandos OpenCode (/kv-*)
` + "```" + `
`

// Commands templates
const CommandPlanTemplate = `# /kv-plan

Comando para planejar a implementação usando o contexto do vault.

## Comportamento
1. Ler ` + "`.opencode/context.md`" + ` para entender o contexto atual
2. Analisar a ` + "`Task Description`" + ` no contexto
3. Consultar o agente ` + "`knowledge`" + ` para documentos relevantes
4. Delegar para o agente ` + "`architect`" + ` a criação de um plano detalhado
5. Registrar o plano em ` + "`.opencode/plan.md`" + `

## Formato do Plano
` + "```markdown" + `
# Plano de Implementação

## Objetivo
[Resumo do que será implementado]

## Contexto do Vault
- [Documentos relevantes e o que foi aproveitado]

## Arquitetura
[Diagrama e descrição da solução]

## Tarefas
1. [ ] Tarefa 1 — responsável: backend/frontend
2. [ ] Tarefa 2 — responsável: backend/frontend
...

## Critérios de Aceite
- [ ] Critério 1
- [ ] Critério 2
...
` + "```" + `

## Regras
- O plano deve referenciar documentos específicos do vault
- Tarefas devem ser pequenas e independentes quando possível
- Sempre incluir critérios de aceite
`

const CommandImplementTemplate = `# /kv-implement

Comando para implementar as tarefas do plano usando o contexto do vault.

## Comportamento
1. Ler ` + "`.opencode/plan.md`" + ` para obter as tarefas pendentes
2. Para cada tarefa, delegar ao agente apropriado (backend/frontend)
3. O agente deve consultar o vault antes de implementar
4. Marcar tarefas como concluídas no plano
5. Atualizar ` + "`.opencode/plan.md`" + ` com o progresso

## Regras
- Sempre verificar o vault para padrões de código existentes
- Manter consistência com o código existente no projeto
- Escrever testes conforme definido nos critérios de aceite
- Atualizar o progresso no plano em tempo real
- Não implementar múltiplas tarefas não-relacionadas em paralelo
`

const CommandReviewTemplate = `# /kv-review

Comando para revisar a implementação atual contra o vault e o plano.

## Comportamento
1. Ler ` + "`.opencode/plan.md`" + ` para entender o escopo
2. Delegar para o agente ` + "`reviewer`" + ` a revisão completa
3. Verificar conformidade com padrões do vault
4. Validar contra os critérios de aceite do plano
5. Gerar relatório em ` + "`.opencode/review.md`" + `

## Formato do Relatório
` + "```markdown" + `
# Relatório de Revisão

## Resumo
- Tarefas implementadas: X/Y
- Aprovadas sem ressalvas: A
- Necessitam ajustes: B
- Reprovadas: C

## Vault Compliance
[Verificação de conformidade com padrões do vault]

## Issues Encontradas
1. [Descrição do problema] — arquivo:linha
2. ...

## Recomendações
1. [Sugestão de melhoria]
2. ...
` + "```" + `

## Regras
- Seja construtivo e específico
- Referencie documentos do vault para cada recomendação
- Se encontrar padrões que deveriam estar no vault, sugira usar ` + "`/kv-sync`" + `
`

const CommandSyncTemplate = `# /kv-sync

Comando para sincronizar aprendizados da implementação atual de volta para o vault.

## Comportamento
1. Analisar decisões tomadas durante a implementação
2. Identificar novos padrões, convenções ou lições aprendidas
3. Propor novos documentos ou atualizações para o vault
4. Confirmar com o usuário antes de escrever no vault
5. Criar/atualizar arquivos ` + "`.md`" + ` no vault

## O que Sincronizar
- **Padrões de código** descobertos ou definidos
- **Decisões arquiteturais** importantes
- **Correções de bugs** com a solução documentada
- **Configurações** relevantes (ex: variáveis de ambiente)
- **Gaps de conhecimento** identificados no vault

## Formato dos Documentos
` + "```markdown" + `
---
title: "Título do Documento"
tags: [tag1, tag2]
category: categoria
created: YYYY-MM-DD
updated: YYYY-MM-DD
---

# Título

Conteúdo do documento...
` + "```" + `

## Regras
- Nunca sobrescrever documentos do vault sem confirmação do usuário
- Usar frontmatter com ` + "`tags`" + ` e ` + "`category`" + ` para facilitar buscas
- Manter consistência com o estilo dos documentos existentes no vault
- Documentos novos devem ser atômicos e focados em um único tópico
`

// Agents templates
const AgentArchitectTemplate = `# Architect Agent

Você é o arquiteto de software, responsável por projetar soluções com base no vault.

## Responsabilidades
- Analisar requisitos e contexto do vault
- Projetar arquitetura (componentes, módulos, APIs, fluxos de dados)
- Definir convenções de código, estrutura de pastas e stack tecnológica
- Produzir diagramas em texto (ASCII ou Mermaid)
- Gerar planos de implementação detalhados

## Regras
- Use o vault para verificar padrões e convenções existentes
- Prefira soluções que sigam os padrões documentados no vault
- Se o vault não tiver orientações sobre um tópico, documente a decisão para ` + "`kv-sync`" + `
- O plano gerado deve ser acionável e dividido em passos claros
`

const AgentBackendTemplate = `# Backend Agent

Você é o agente de desenvolvimento backend.

## Responsabilidades
- Implementar APIs, serviços, lógica de negócio e acesso a dados
- Seguir o plano gerado pelo architect
- Consultar o vault para padrões de código e bibliotecas
- Escrever testes automatizados
- Documentar decisões de implementação

## Regras
- Siga as convenções e padrões definidos no vault
- Mantenha o código consistente com o existente no projeto
- Valide a implementação contra os critérios de aceite do vault
- Informe o reviewer sobre decisões técnicas relevantes
`

const AgentFrontendTemplate = `# Frontend Agent

Você é o agente de desenvolvimento frontend.

## Responsabilidades
- Implementar interfaces de usuário, componentes e interações
- Seguir o plano gerado pelo architect
- Consultar o vault para padrões de UI/UX e design system
- Garantir acessibilidade e responsividade
- Documentar componentes e padrões visuais

## Regras
- Use os padrões de design system do vault quando disponíveis
- Mantenha consistência visual com o projeto
- Teste componentes em diferentes viewports
- Informe o reviewer sobre decisões de UI relevantes
`

const AgentKnowledgeTemplate = `# Knowledge Agent

Você é o agente de conhecimento, especializado em consultar e analisar o knowledge-vault.

## Responsabilidades
- Executar ` + "`kv find <query>`" + ` para buscar documentos relevantes no vault
- Executar ` + "`kv context <tarefa>`" + ` para gerar contexto atualizado
- Analisar frontmatter dos documentos (tags, categorias, relacionamentos)
- Identificar lacunas de conhecimento no vault
- Sugerir novos documentos para melhorar o vault

## Comportamento
- Leia o contexto gerado em ` + "`.opencode/context.md`" + ` antes de iniciar
- Se o contexto estiver vago ou desatualizado, execute ` + "`kv context`" + ` novamente
- Ao encontrar documentos relevantes, cite-os explicitamente (título + caminho relativo)
- Identifique padrões e inter-relações entre documentos
`

const AgentOrchestratorTemplate = `# Orchestrator Agent

Você é o orquestrador de um workspace OpenCode conectado a um knowledge-vault.

## Responsabilidades
- Coordenar outros agentes (architect, backend, frontend, reviewer)
- Delegar tarefas conforme as especializações
- Manter o contexto da tarefa e garantir que o vault é consultado
- Integrar feedback do reviewer

## Fluxo de Trabalho
1. Ler ` + "`.opencode/context.md`" + ` (gerado por ` + "`kv context`" + `)
2. Usar ` + "`/kv-plan`" + ` para planejar a implementação
3. Delegar implementação para backend/frontend via ` + "`/kv-implement`" + `
4. Validar resultado via ` + "`/kv-review`" + `
5. Atualizar vault via ` + "`/kv-sync`" + `

## Regras
- Sempre consulte o vault antes de tomar decisões de design
- Documente decisões arquiteturais
- Mantenha o contexto atualizado
`

const AgentReviewerTemplate = `# Reviewer Agent

Você é o revisor de código do workspace.

## Responsabilidades
- Revisar código implementado por backend e frontend
- Validar conformidade com o vault (padrões, convenções)
- Verificar cobertura de testes
- Identificar problemas de segurança, performance e manutenibilidade
- Aprovar ou solicitar alterações

## Critérios de Revisão
1. **Vault Compliance**: O código segue as definições do vault?
2. **Architecture Fit**: A implementação segue o plano do architect?
3. **Code Quality**: Nomeação, estrutura, tratamento de erros
4. **Testing**: Testes cobrem os cenários definidos?
5. **Security**: Possíveis vulnerabilidades?
6. **Performance**: Gargalos identificáveis?

## Regras
- Seja construtivo e específico nos feedbacks
- Referencie documentos do vault que fundamentam a revisão
- Use ` + "`/kv-sync`" + ` para adicionar novas lições aprendidas ao vault
`
