# Ciclo de Vida de Sessões & Multi-App

Uma **Sessão** no `kv` é um contexto delimitado de desenvolvimento focado em um objetivo específico (ex: "Refatorar login" ou "Corrigir vazamento de memória"). Ela reúne as aplicações envolvidas, os runbooks do Vault e define regras rígidas de segurança física.

---

## 🚀 Inicializando uma Sessão

Para criar uma nova sessão de desenvolvimento, utilize o comando `kv session init` (ou o alias `kv session start`):

```bash
kv session init \
  --id session-auth-refactor \
  --goal "Implementar autenticação baseada em JWT no gateway de pagamentos" \
  --apps api-gateway=./apps/api-gateway,payments-api=./apps/payments-api \
  --vault ./vault/runbooks \
  --writable ./apps/api-gateway,./apps/payments-api
```

### Parâmetros Suportados:
- `--id`: Identificador único da sessão (gerará a pasta correspondente).
- `--goal`: Objetivo principal da sessão técnica.
- `--apps`: Lista chave-valor de aplicações do workspace envolvidas na tarefa.
- `--vault`: Caminho para a pasta local de documentações associada.
- `--writable`: Caminhos onde o Agent tem permissão física de escrita.

---

## 📂 Estrutura de Diretórios da Sessão

Ao inicializar, a seguinte estrutura física de arquivos é gerada sob `.kv/sessions/<session-id>/`:

```txt
.kv/
  sessions/
    session-auth-refactor/
      session.yaml        # Contrato declarativo da sessão
      context.md          # Contexto copilado para o Agent
      opencode.md         # Manifesto otimizado para o runner OpenCode
      audit.jsonl         # Logs de auditoria append-only
      report.md           # Relatório final consolidado
```

---

## 📑 O Contrato `session.yaml`

O arquivo `session.yaml` gerado declara as regras que os agentes de IA devem obedecer:

```yaml
id: session-auth-refactor
goal: "Implementar autenticação baseada em JWT no gateway de pagamentos"
apps:
  api-gateway: ./apps/api-gateway
  payments-api: ./apps/payments-api
vault_path: ./vault/runbooks
boundary:
  allowed_paths:
    - ./apps/api-gateway
    - ./apps/payments-api
    - ./vault/runbooks
  writable_paths:
    - ./apps/api-gateway
    - ./apps/payments-api
  readonly_paths:
    - ./vault/runbooks
quality:
  commands:
    - app: api-gateway
      cmd: go test ./...
    - app: payments-api
      cmd: npm test
policy:
  allow_network: false
  allow_env_read: true
  allow_delete: true
```

---

## 🔍 Validando o Contrato

Antes de entregar a tarefa para a IA executar, você deve garantir que a sessão está íntegra:

```bash
kv session validate --session session-auth-refactor
```

### O que é verificado?
- Se todos os diretórios e caminhos informados no contrato existem fisicamente no disco.
- Se os caminhos de escrita (`writable_paths`) estão contidos estritamente dentro dos caminhos permitidos (`allowed_paths`).
- Se não há conflitos ou sobreposições perigosas de permissão.
- Se o Policy Engine do `kv` está íntegro.
