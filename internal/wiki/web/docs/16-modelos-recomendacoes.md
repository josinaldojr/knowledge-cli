# Guia de Modelos e Recomendações

O `knowledge-cli` (`kv`) integra-se nativamente com a infraestrutura do **OpenCode**, permitindo que você escolha entre diversos modelos de linguagem (LLMs) especializados para guiar execuções de sessões, tarefas e workflows. Cada modelo possui pontos fortes específicos, velocidades distintas e políticas de custos variadas.

Este guia serve como referência para orientar qual modelo utilizar para cada atividade de desenvolvimento e ciclo de engenharia de software.

---

## 🚀 Como Selecionar um Modelo

Ao executar os comandos de execução, você pode passar explicitamente o modelo através da flag `--model` (ou `-m` nas chamadas subjacentes do OpenCode). Se a flag for omitida em um terminal interativo (TTY), o CLI exibirá um menu interativo para seleção:

```bash
# Executando um workflow específico com o DeepSeek V4 Pro
kv workflow run refactor-auth --prompt "Mudar JWT para HttpOnly Cookies" --model opencode-go/deepseek-v4-pro

# Executando uma task de uma sessão
kv task run feature-auth 002-add-login --runner opencode --model openai/gpt-5.5-pro
```

---

## 📊 Matriz de Recomendação por Atividade

| Modelo | Atividade Principal Recomendada | Força (1-5) | Custo / Acesso | Contexto / Escopo |
| :--- | :--- | :--- | :--- | :--- |
| **`openai/gpt-5.5-pro`** | Arquitetura Complexa, Refatoração de Larga Escala, Raciocínio Lógico Avançado | ⭐⭐⭐⭐⭐ (5/5) | Premium (Créditos) | Longo |
| **`openai/gpt-5.5` / `gpt-5.5-fast`**| Desenvolvimento Geral, Correção de Bugs Complexos, Implementação de Features | ⭐⭐⭐⭐☆ (4/5) | Premium (Créditos) | Médio-Longo |
| **`openai/gpt-5.4-mini`** | Correções de Sintaxe, Geração de Boilerplate, Tarefas Simples e Rápidas | ⭐⭐⭐☆☆ (3/5) | Econômico | Médio |
| **`opencode-go/deepseek-v4-pro`** | Matemática, Algoritmos Complexos, Estruturas de Dados, Backend Eficiente | ⭐⭐⭐⭐⭐ (5/5) | Equilibrado | Longo |
| **`opencode/deepseek-v4-flash-free`**| Prototipagem Rápida, Testes Unitários Simples, Autocompletes em Massa | ⭐⭐⭐☆☆ (3/5) | **Gratuito** | Curto-Médio |
| **`opencode-go/kimi-k2.7-code`** | Análise Multiarquivos, Leitura Completa de Repositório (Contexto Gigante) | ⭐⭐⭐⭐⭐ (5/5) | Premium (Créditos) | **Extremo (Context Window)** |
| **`opencode-go/qwen3.7-max`** | Integrações de API, Internacionalização, Escrita de Código Bilingue / Documentação | ⭐⭐⭐⭐☆ (4/5) | Equilibrado | Longo |
| **`opencode-go/glm-5.2`** | Execução Agentica Estrita, Scripts Autônomos de DevOps, Tool Calling | ⭐⭐⭐⭐☆ (4/5) | Equilibrado | Longo |

---

## 🔍 Detalhamento por Família de Modelos

### 1. OpenAI (Família GPT-5)
Modelos de ponta do ecossistema OpenAI, ideais para problemas difíceis que exigem planejamento sofisticado e conformidade estrita de regras.

*   **`openai/gpt-5.5-pro`**: O modelo mais inteligente disponível. Excelente para planejar refatorações de arquiteturas legadas, criar diagramas de fluxo de dados de alta precisão e resolver bugs complexos de concorrência ou performance.
*   **`openai/gpt-5.5` & `openai/gpt-5.5-fast`**: Modelos focados no dia a dia do desenvolvimento. A versão `fast` entrega respostas em uma fração do tempo com perda mínima de precisão.
*   **`openai/gpt-5.4` / `gpt-5.4-fast`**: Geração anterior estável. Útil se houver necessidade de compatibilidade retroativa ou para manter estabilidade em tarefas consolidadas.
*   **`openai/gpt-5.4-mini` & `gpt-5.4-mini-fast`**: Modelos leves e incrivelmente rápidos. Use-os para preencher testes unitários básicos, fazer lintings manuais, formatar JSONs ou documentar funções simples.
*   **`openai/gpt-5.3-codex-spark`**: Modelo altamente especializado em completar códigos na linha onde o cursor está posicionado, com baixa latência.

### 2. DeepSeek (Família DeepSeek V4)
Famosa pela eficiência matemática e de programação, oferecendo um excelente custo-benefício.

*   **`opencode-go/deepseek-v4-pro`**: Ideal para desenvolvimento de algoritmos puros, lógica de banco de dados SQL/NoSQL complexa, e otimização de performance em Go e Rust.
*   **`opencode-go/deepseek-v4-flash` / `opencode/deepseek-v4-flash-free`**: Modelos rápidos para validações imediatas. A versão `free` é perfeita para estudantes e desenvolvedores fazendo pequenos ajustes sem queimar cota de tokens.

### 3. Moonshot (Família Kimi K2)
Modelos com capacidades de contexto massivas, ideais para projetos com muitos arquivos interligados.

*   **`opencode-go/kimi-k2.7-code`**: O melhor modelo para ler uma base de código inteira. Se a sua sessão ou tarefa exige analisar múltiplos arquivos em diretórios diferentes simultaneamente, este modelo brilha devido à sua enorme janela de contexto e persistência de atenção.
*   **`opencode-go/kimi-k2.6`**: Variante de contexto geral, recomendada para processar relatórios longos, logs de erro volumosos ou compilar documentações extensas baseadas no histórico de commits do repositório.

### 4. Alibaba (Família Qwen 3)
Modelos extremamente potentes e versáteis, com destaque para a internacionalização e a escrita de especificações técnicas robustas.

*   **`opencode-go/qwen3.7-max`**: Competidor direto dos maiores modelos do mercado. Excelente suporte a português brasileiro e inglês, com alta taxa de acerto em lógica de negócios corporativa e formatação de contratos.
*   **`opencode-go/qwen3.7-plus` & `qwen3.6-plus`**: Excelentes para geração de boilerplate, criação de controllers de APIs CRUD e scaffolding inicial de projetos.

### 5. Zhipu (Família GLM 5)
Destacam-se pelo alinhamento a chamadas de funções (Function Calling) e raciocínio agentico.

*   **`opencode-go/glm-5.2` & `glm-5.1`**: Projetados para executar fluxos agenticos interativos. Se a tarefa exige que o runner tome decisões lógicas sequenciais em terminais virtuais, rode comandos de shell específicos de compilação ou organize etapas de migração de dados, o GLM segue as diretivas do contrato do `kv` com precisão.

### 6. Mimo & MiniMax
Modelos intermediários versáteis e robustos.

*   **`opencode-go/mimo-v2.5-pro` & `opencode-go/mimo-v2.5` / `opencode/mimo-v2.5-free`**: Excelentes para tarefas gerais de média complexidade, criação de páginas HTML/CSS com bom senso visual e testes de validação unitária.
*   **`opencode-go/minimax-m3` & `minimax-m2.7`**: Especialistas em linguagem natural fluida. Altamente recomendados para refinar a escrita de notas na inbox antes de enviá-las para a compilação da Wiki via `kv wiki compile`.

### 7. Modelos Especiais & Gratuitos (Free-Tier)
Modelos focados em acessibilidade ou propósitos altamente específicos.

*   **`opencode/big-pickle`**: Modelo otimizado para tratamento de dados estruturados em massa (pipelines ETL, scripts python pesados de conversão de dados).
*   **`opencode/nemotron-3-ultra-free`**: Modelo focado em respostas detalhadas, raciocínio lógico avançado e arquitetura de sistemas. Ótimo recurso gratuito com inteligência premium.
*   **`opencode/north-mini-code-free`**: Otimizado para pequenos ajustes e fixes rápidos de código sem custo.
*   **`opencode/hy3-free`**: Modelo ágil e econômico para experimentação inicial.

---

## 💡 Dicas de Uso e Boas Práticas

1.  **Use Modelos `-free` para Rascunhos**: Para tarefas muito simples (ex: adicionar comentários, mudar nomes de variáveis simples, pequenos fixes sugeridos por linter), economize seus créditos de produção usando `opencode/deepseek-v4-flash-free` ou `opencode/mimo-v2.5-free`.
2.  **Combine com o Quality Gates**: Ao usar modelos rápidos ou mais simples, certifique-se de ter um bom suite de testes configurado na sua Session (`quality.commands`). Se o modelo introduzir alguma inconsistência, o harness do `kv` detectará a falha imediatamente e você poderá rodar novamente ou ajustar.
3.  **Use Modelos `Pro`/`Max` para Specs**: Na hora de rodar as fases iniciais de planejamento e especificação de workflows, use modelos altamente analíticos (como `openai/gpt-5.5-pro` ou `opencode-go/deepseek-v4-pro`). Um bom plano de implementação inicial reduz significativamente o trabalho do runner e a chance de loops de correção na fase de código.
