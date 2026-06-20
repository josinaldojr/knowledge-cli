# /kv-plan

Comando para planejar a implementação usando o contexto do vault.

## Comportamento
1. Ler `.opencode/context.md` para entender o contexto atual
2. Analisar a `Task Description` no contexto
3. Consultar o agente `knowledge` para documentos relevantes
4. Delegar para o agente `architect` a criação de um plano detalhado
5. Registrar o plano em `.opencode/plan.md`

## Formato do Plano
```markdown
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
```

## Regras
- O plano deve referenciar documentos específicos do vault
- Tarefas devem ser pequenas e independentes quando possível
- Sempre incluir critérios de aceite
