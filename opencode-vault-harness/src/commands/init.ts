import { existsSync, cpSync, mkdirSync, readdirSync, statSync } from 'node:fs';
import { resolve, join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { writeVaultConfig } from '../workspace/write-config';

function copyDirRecursive(src: string, dest: string): void {
  if (!existsSync(src)) return;

  mkdirSync(dest, { recursive: true });

  const entries = readdirSync(src);
  for (const entry of entries) {
    const srcPath = join(src, entry);
    const destPath = join(dest, entry);
    const stat = statSync(srcPath);

    if (stat.isDirectory()) {
      copyDirRecursive(srcPath, destPath);
    } else {
      cpSync(srcPath, destPath);
    }
  }
}

export function initCommand(vaultPath: string): void {
  const targetDir = process.cwd();
  const resolvedVaultPath = resolve(vaultPath);

  if (!existsSync(resolvedVaultPath)) {
    console.error(`Erro: O vault "${resolvedVaultPath}" não existe.`);
    process.exit(1);
  }

  console.log(`Inicializando workspace OpenCode em: ${targetDir}`);
  console.log(`Conectado ao vault: ${resolvedVaultPath}`);

  const configPath = writeVaultConfig(targetDir, resolvedVaultPath);
  console.log(`Configuração criada: ${configPath}`);

  const templatesDir = resolve(__dirname, '..', '..', 'templates', 'opencode');
  const opencodeDir = resolve(targetDir, '.opencode');

  if (!existsSync(templatesDir)) {
    console.warn(`Aviso: Templates não encontrados em "${templatesDir}".`);
    console.warn('Criando .opencode/ com arquivos padrão...');

    mkdirSync(opencodeDir, { recursive: true });
    return;
  }

  console.log(`Copiando templates para: ${opencodeDir}`);
  copyDirRecursive(templatesDir, opencodeDir);

  console.log('');
  console.log('Workspace inicializado com sucesso!');
  console.log('');
  console.log('Próximos passos:');
  console.log('  1. Execute "kv context <sua-tarefa>" para gerar contexto do vault.');
  console.log('  2. Abra o diretório com OpenCode para usar os agentes configurados.');
}
