# Integração Global OpenCode

O `kv` funciona de forma nativa e integrada com o **OpenCode**, servindo como o compilador e fornecedor de contexto estruturado para execuções automatizadas em ambientes virtuais de codificação.

---

## 🔧 Instalando Scripts e Templates

Para instalar os scripts auxiliares globais e templates de integração no seu sistema operacional (armazenados em `~/.config/opencode/`):

```bash
kv opencode install
```

### O que é instalado?
- **Scripts de Auxílio**: Utilitários para que o OpenCode consiga se comunicar com as APIs do `kv` de forma transparente.
- **Templates de Contexto**: Arquivos que ajudam o runner a ler os arquivos de boundary e session gerados no diretório local `.kv/`.

---

## 🔍 Verificação de Instalação (`kv opencode doctor`)

Para diagnosticar se a integração global com o OpenCode está corretamente configurada na máquina de desenvolvimento:

```bash
kv opencode doctor
```

### O que o comando valida?
- Se o diretório `~/.config/opencode/` existe.
- Se os scripts obrigatórios estão com permissão de execução.
- Se o binário do `kv` está acessível a partir do path global do runner do OpenCode.
