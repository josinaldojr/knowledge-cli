# /kv-sync

Comando para sincronizar aprendizados da implementação atual de volta para o vault.

## Comportamento
1. Analisar decisões tomadas durante a implementação
2. Identificar novos padrões, convenções ou lições aprendidas
3. Propor novos documentos ou atualizações para o vault
4. Confirmar com o usuário antes de escrever no vault
5. Criar/atualizar arquivos `.md` no vault

## O que Sincronizar
- **Padrões de código** descobertos ou definidos
- **Decisões arquiteturais** importantes
- **Correções de bugs** com a solução documentada
- **Configurações** relevantes (ex: variáveis de ambiente)
- **Gaps de conhecimento** identificados no vault

## Formato dos Documentos
```markdown
---
title: "Título do Documento"
tags: [tag1, tag2]
category: categoria
created: YYYY-MM-DD
updated: YYYY-MM-DD
---

# Título

Conteúdo do documento...
```

## Regras
- Nunca sobrescrever documentos do vault sem confirmação do usuário
- Usar frontmatter com `tags` e `category` para facilitar buscas
- Manter consistência com o estilo dos documentos existentes no vault
- Documentos novos devem ser atômicos e focados em um único tópico
