import { writeFileSync, mkdirSync, existsSync } from 'node:fs';
import { resolve } from 'node:path';
import { detectWorkspace } from '../workspace/detect-workspace';
import { renderContext } from '../vault/render-context';

export function contextCommand(taskDescription: string): void {
  const workspace = detectWorkspace();

  if (!workspace) {
    console.error('Erro: Nenhum workspace OpenCode encontrado. Execute "kv init --vault <caminho>" primeiro.');
    process.exit(1);
  }

  const { vaultPath } = workspace.config;

  if (!existsSync(vaultPath)) {
    console.error(`Erro: O vault "${vaultPath}" não existe mais. Verifique sua configuração.`);
    process.exit(1);
  }

  console.log(`Gerando contexto para: "${taskDescription}"...`);

  const contextMd = renderContext(vaultPath, taskDescription);

  const opencodeDir = resolve(process.cwd(), '.opencode');
  mkdirSync(opencodeDir, { recursive: true });

  const contextPath = resolve(opencodeDir, 'context.md');
  writeFileSync(contextPath, contextMd, 'utf-8');

  console.log(`Contexto gerado: ${contextPath}`);
  console.log(`O OpenCode lerá este arquivo automaticamente ao iniciar.`);
}
