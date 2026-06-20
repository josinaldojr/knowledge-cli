# OpenCode Agents

Este workspace está configurado com agentes especiais para trabalhar com um knowledge-vault.

## Comandos Disponíveis

### `/kv-plan`
Analisa o contexto gerado e cria um plano de implementação.

### `/kv-implement`
Implementa as tarefas do plano, consultando o vault como referência.

### `/kv-review`
Revisa o código implementado, comparando com as definições do vault.

### `/kv-sync`
Atualiza o vault com novos aprendizados da implementação atual.

## Como Usar

1. Execute `kv context "sua tarefa"` para gerar o contexto do vault.
2. Use `/kv-plan` para planejar a implementação.
3. Use `/kv-implement` para executar o plano.
4. Use `/kv-review` para validar a implementação.
5. Use `/kv-sync` para salvar novos conhecimentos no vault.

## Estrutura do Projeto

```
.kv/config.json       → Conexão com o vault
.opencode/context.md  → Contexto gerado automaticamente
.opencode/agents/     → Definições dos agentes
.opencode/commands/   → Comandos OpenCode (/kv-*)
```
