# /kv-implement

Comando para implementar as tarefas do plano usando o contexto do vault.

## Comportamento
1. Ler `.opencode/plan.md` para obter as tarefas pendentes
2. Para cada tarefa, delegar ao agente apropriado (backend/frontend)
3. O agente deve consultar o vault antes de implementar
4. Marcar tarefas como concluídas no plano
5. Atualizar `.opencode/plan.md` com o progresso

## Regras
- Sempre verificar o vault para padrões de código existentes
- Manter consistência com o código existente no projeto
- Escrever testes conforme definido nos critérios de aceite
- Atualizar o progresso no plano em tempo real
- Não implementar múltiplas tarefas não-relacionadas em paralelo
