# Quality Gates (Portais de Qualidade)

Garantir que as modificações de IA não quebrem a build ou introduzam bugs de regressão é essencial. O `kv` implementa os **Quality Gates**, permitindo que comandos de verificação automatizados rodem sob regras rígidas do Policy Engine antes que a sessão seja encerrada.

---

## 🚦 Comandos de Qualidade

Os Quality Gates são configurados no arquivo `session.yaml` de cada sessão, sob a diretiva `quality.commands`. O `kv` infere automaticamente esses comandos com base na stack detectada durante a inicialização, mas você pode personalizá-los.

### Exemplo de Configuração de Qualidade:
```yaml
quality:
  commands:
    - app: api-gateway
      cmd: go test -v ./...
    - app: api-gateway
      cmd: golangci-lint run
```

---

## ⚡ Rodando as Verificações

Para executar todas as baterias de testes e validações de qualidade configuradas para a sessão atual, execute:

```bash
kv quality run --session session-auth-refactor
```

### O que o comando realiza?
1. **Isolamento de Diretório**: Navega automaticamente para a pasta correspondente a cada aplicação declarada (ex: `apps/api-gateway`).
2. **Execução Supervisionada**: Dispara o comando de teste (`cmd`) garantindo as restrições de segurança do Policy Engine (por exemplo, bloqueando scripts maliciosos de acessar a rede ou deletar arquivos fora do escopo).
3. **Coleta de Resultados**: Registra saídas, códigos de erro (`exit_code`) e o status da qualidade (Passed/Failed) no console e no log de auditoria.

---

## 🛡️ Policy Engine nos Testes

Ao contrário de execuções de terminal comuns, comandos de qualidade disparados pelo `kv quality run` são monitorados pelo **Policy Engine** do `kv`. Se um teste unitário tentar baixar pacotes externos em uma sessão configurada como `allow_network: false`, a ação será barrada para prevenir vazamento de dados ou injeção de dependências em tempo de teste.
