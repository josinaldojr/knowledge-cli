# Workspace & Auto-Scanner

Para que a ferramenta `kv` entenda a estrutura física de aplicações dentro de um repositório, ela utiliza um manifesto centralizador chamado `kv-workspace.yaml`. Em vez de preenchê-lo manualmente, o `kv` fornece um mecanismo de varredura inteligente que detecta as stacks tecnológicas locais.

---

## 🔍 Comando de Varredura

Execute o comando na raiz de seu repositório:

```bash
kv workspace scan
```

### O que o comando realiza?
1. **Identifica Subdiretórios**: Examina pastas do repositório em busca de assinaturas conhecidas.
2. **Detecta Stacks**: Associa as tecnologias de acordo com os seguintes arquivos presentes:
   - `go.mod` -> **Go**
   - `package.json` -> **Node/JS/TS**
   - `pom.xml` -> **Java**
   - `requirements.txt` / `Pipfile` -> **Python**
   - `Dockerfile` -> **Docker/Infra**
3. **Gera a Estrutura**: Cria ou complementa o arquivo `kv-workspace.yaml` na raiz do projeto.

---

## 📄 Estrutura do `kv-workspace.yaml`

Um exemplo prático de um arquivo `kv-workspace.yaml` gerado:

```yaml
workspace:
  name: meu-projeto-distribuido
  apps:
    api-gateway:
      path: ./apps/api-gateway
      stack: go
    web-dashboard:
      path: ./web-dashboard
      stack: node
    jobs-worker:
      path: ./workers/jobs-worker
      stack: python
```

> [!TIP]
> Caso possua serviços adicionais que usam stacks customizadas ou subpastas complexas, você pode editar diretamente o arquivo `kv-workspace.yaml` para refletir as necessidades da sua infraestrutura local.
