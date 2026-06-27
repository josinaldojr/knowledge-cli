# LLM Wiki (Paradigma de Compilação Incremental)

O `kv` implementa o conceito de **LLM Wiki** proposto por Andrej Karpathy. Em vez de utilizar apenas buscas RAG reativas que recuperam pedaços de texto temporários no momento da consulta, o sistema compila e consolida de forma incremental e persistente as notas brutas em páginas de documentação canônicas (Markdown) totalmente interligadas no seu cofre de conhecimento.

---

## 📥 1. Compilação de Notas Brutas (`kv wiki compile`)

Se você possui notas desorganizadas ou rascunhos de reuniões e decisões técnicas, basta colocá-los no diretório `00-inbox/` do seu Vault e rodar:

```bash
kv wiki compile
```

### O que o compilador faz?
1. **Analisa Fatos**: A IA lê as notas brutas e identifica tópicos de arquitetura, decisões técnicas e novos conceitos.
2. **Atualiza ou Cria Páginas**:
   - Se o conceito já existir na Wiki (ex: `04-systems/auth.md`), ela mescla os fatos novos de forma coerente, resolvendo contradições e atualizando a data de modificação no frontmatter YAML.
   - Se for um novo conceito, ela cria uma nova página na subpasta adequada (ex: `04-systems/`, `07-runbooks/`).
3. **Arquiva Originais**: Move os rascunhos processados de `00-inbox/` para `10-references/archive/` para manter o fluxo limpo.

---

## 🔗 2. Atualização de Links Cruzados (`kv wiki link`)

Para manter a Wiki interligada (ideal para navegação visual em editores como Obsidian ou VS Code), o comando `kv wiki link` escaneia as páginas canônicas do cofre e insere caminhos relativos de markdown de forma automática em termos técnicos e menções a outros documentos:

```bash
kv wiki link
```

*Exemplo: O texto "Verifique as configurações em Auth Flow" é convertido para "Verifique as configurações em [Auth Flow](../04-systems/auth-flow.md)".*

---

## 💬 3. Consultando a Wiki via CLI (`kv wiki ask`)

Você pode fazer perguntas diretas para a sua Wiki pela linha de comando:

```bash
kv wiki ask "Quais são os padrões de autenticação do nosso projeto?"
```

A IA buscará os documentos relevantes no cofre, extrairá o contexto (RAG) e gerará uma resposta em português técnico citando os arquivos de origem.

---

## 🖥️ 4. Interface Web Local-First (`kv wiki serve`)

Para subir a interface web premium e navegar de forma interativa por toda a Wiki:

```bash
kv wiki serve --port 8080
```

### Recursos da Interface Web:
- **Árvore Dinâmica (Vault Explorer)**: Navegação recursiva do Vault.
- **Renderização Markdown em Tempo Real**: Renderização com suporte a tabelas, listas e frontmatters.
- **Busca Global Instantânea (Cmd+K)**: Pesquisa de texto com score de relevância e snippets de código.
- **Wiki Copilot (Chat Lateral RAG)**: Assistente de chat integrado com o cofre técnico que mantém o histórico de conversação.
