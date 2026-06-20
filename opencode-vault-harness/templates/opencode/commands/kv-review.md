# /kv-review

Comando para revisar a implementação atual contra o vault e o plano.

## Comportamento
1. Ler `.opencode/plan.md` para entender o escopo
2. Delegar para o agente `reviewer` a revisão completa
3. Verificar conformidade com padrões do vault
4. Validar contra os critérios de aceite do plano
5. Gerar relatório em `.opencode/review.md`

## Formato do Relatório
```markdown
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
```

## Regras
- Seja construtivo e específico
- Referencie documentos do vault para cada recomendação
- Se encontrar padrões que deveriam estar no vault, sugira usar `/kv-sync`
