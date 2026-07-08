# Comparativo: Sessões vs. Workflows

No `kv`, existem duas abstrações principais para organizar, executar e testar o desenvolvimento assistido por inteligência artificial: **Sessões** e **Workflows**. Compreender a diferença conceitual e prática entre elas é essencial para escolher a abordagem correta para cada cenário de desenvolvimento e validação.

---

## 🔍 Resumo de Alto Nível

A diferença fundamental reside na **granularidade**, **ciclo de vida** e **rigor de governança**:

- **Sessões (Sessions)**: São focadas em **limites físicos de execução e isolamento (sandbox)**. Permitem definir um espaço restrito de arquivos e diretórios (`boundaries`) nos quais a IA pode atuar para resolver um único objetivo operacional direto, rodando testes rápidos e automáticos via *Quality Gates*.
- **Workflows**: São focados em **processos estruturados e documentação persistente no Git**. Dividem uma entrega complexa de engenharia em etapas incrementais e ordenadas (da concepção à memorização), guiadas por tarefas (`tasks`) com critérios de aceitação e revisões formais de código.

---

## ⚔️ Tabela Comparativa Detalhada

| Critério | Sessões (Sessions) | Workflows & Tasks |
| :--- | :--- | :--- |
| **Escopo de Ação** | Um único objetivo direto e pontual para um conjunto de aplicações. | Um ciclo de engenharia completo (PRD, Spec, Tasks, Review, Memorize). |
| **Isolamento (`Boundary`)** | Extremamente restrito por caminhos físicos permitidos (`allowed_paths`, `writable_paths`). | Baseado nos arquivos listados na propriedade `sources` da tarefa enriquecida. |
| **Governança no Git** | Baixa. Os logs e configurações ficam em `.kv/sessions/` para fins de auditoria, mas não guiam o histórico. | Alta. Toda a especificação técnica (`techspec.md`), PRD, tarefas e arquivos de revisão ficam no repositório. |
| **Estrutura de Execução** | Direta: o runner atua diretamente no contexto compilado. | Dividida em 7 fases rígidas de conformidade. |
| **Garantia de Qualidade** | Execução de comandos automáticos configurados no contrato (`kv quality run`). | Processo iterativo de aceitação em duas fases distintas (*Review* e *Adjustments*). |
| **Wiki / Vault** | Consulta RAG de runbooks no Vault de Conhecimento para apoiar o contexto. | Além da consulta, promove ativamente novas memórias e decisões para o Vault na fase *Memorize*. |

---

## 🚦 O Ciclo de Testes e Validação em Detalhes

### A. Testes em Sessões (Quality Gates)
O ciclo de testes em sessões é voltado para **validação imediata e automatizada**.
1. Ao inicializar uma sessão (`kv session init`), o CLI detecta a stack (por exemplo, Go ou Node.js) e sugere comandos padrão na propriedade `quality.commands` de `.kv/sessions/<session-id>/session.yaml`:
   ```yaml
   quality:
     enabled: true
     commands:
       - go test ./...
       - go vet ./...
   ```
2. Após o runner (como o **OpenCode**) executar as alterações sugeridas, você ou o harness executa:
   ```bash
   kv quality run --session <session-id>
   ```
3. O `kv` executa esses comandos dentro do diretório raiz da primeira aplicação selecionada. Se algum comando falhar ou se violar uma política de segurança monitorada pelo *Policy Engine* (ex: acesso não autorizado a rede), o teste falha, e o resultado é registrado no log append-only (`audit.jsonl`).

### B. Testes em Workflows (Revisão e Ajustes)
O ciclo de testes em workflows é **iterativo, orientado a critérios de aceitação e persistente**.
1. Na fase de **Specs (Fase 3)**, a IA gera as tarefas em arquivos markdown sob `.kv/workflows/<slug>/tasks/`. Cada tarefa contém uma lista de critérios de aceitação específicos (`acceptanceCriteria`).
2. Durante a **Phase 5: Review**, o CLI gera automaticamente arquivos de revisão sob `reviews/<task-id>.review.md`. A IA executa os comandos do plano de validação técnica e analisa o `git diff` gerado.
3. Para cada critério de aceitação satisfeito e teste que passa, o checklist no arquivo de revisão é atualizado para concluído:
   ```markdown
   - [x] O endpoint /auth/login retorna JWT válido
   - [ ] Cobertura de testes unitários superior a 90% (Pendente)
   ```
4. Se algum item falhar, o status do arquivo de revisão se mantém como `pending-review`.
5. Na **Phase 6: Adjustments**, o CLI detecta automaticamente todas as revisões pendentes e reativa o runner, dando instruções claras baseadas nas falhas de teste e critérios não atendidos. O runner altera o código-fonte, roda os testes localmente e, quando aprovado, atualiza o checklist para `[x]`, fechando o ciclo de qualidade.

---

## 🎯 Quando Usar Cada Abordagem?

### Use Sessões se:
* Você precisa consertar um bug rápido ou aplicar um hotfix localizado no código.
* Deseja refatorar uma função ou classe específica onde a suite de testes já existe e é de rápida execução.
* Quer garantir total segurança limitando estritamente os diretórios de escrita do agente.
* Não há necessidade de documentar reuniões de produto (PRD) ou decisões de design antes de começar a codificar.

### Use Workflows se:
* Você está desenvolvendo uma nova funcionalidade complexa (feature) a partir do zero.
* As tarefas exigem alterações estruturais na arquitetura da aplicação ou banco de dados que precisam de documentação técnica formal (`techspec.md`) aprovada.
* A validação depende de critérios de aceitação funcionais detalhados, e não apenas do sucesso de um test runner.
* Você deseja reter os aprendizados e decisões de arquitetura tomadas durante a implementação diretamente no **Knowledge Vault** para que futuras execuções de IA tenham acesso a esse contexto.
