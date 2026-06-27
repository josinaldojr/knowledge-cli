# Diff Summarizer (Sumarizador de Alterações)

Entender o que a IA alterou no codebase pode ser cansativo se você analisar apenas o `git diff` bruto de milhares de linhas de código. O `kv` possui um **Diff Summarizer** inteligente que compila e explica as modificações locais de forma estruturada.

---

## ⚡ Comando de Sumarização

Para gerar o resumo das alterações de uma sessão ativa, execute:

```bash
kv diff summarize --session session-auth-refactor
```

### O que o comando realiza?
1. **Lê o Git Diff**: Captura as modificações locais no repositório restritas aos caminhos permitidos (`boundary.allowed_paths`) da sessão.
2. **Classifica Alterações**: Identifica e cataloga novas funções, assinaturas modificadas, mudanças em arquivos de configuração e alterações nos testes unitários.
3. **Análise de Risco**: Aponta potenciais riscos de regressão no código com base no impacto das modificações.
4. **Cria o Sumário**: Salva o relatório consolidado em `.kv/sessions/<session-id>/diff-summary.md`.

---

## 📑 Exemplo de Saída (`diff-summary.md`)

O documento final gerado pela sumarização contém as seguintes seções estruturadas:

```markdown
# Resumo de Alterações - Session session-auth-refactor

## 📁 Arquivos Modificados
- `apps/api-gateway/auth.go`
- `apps/api-gateway/auth_test.go`

## ⚙️ Alterações Estruturais
- **Nova Struct**: `JWTAuthenticator` adicionada em `auth.go`.
- **Nova Assinatura**: `AuthenticateRequest(r *http.Request) (*UserClaims, error)`.

## 🧪 Cobertura de Testes
- Adicionado teste unitário `TestJWTAuthenticator_Success` em `auth_test.go` cobrindo fluxos de expiração de token.

## ⚠️ Análise de Riscos
- **Risco Médio**: A alteração no middleware de autenticação pode impactar as chamadas legadas da API se os headers de autorização estiverem ausentes. Recomenda-se rodar os testes da aplicação integradora.
```
