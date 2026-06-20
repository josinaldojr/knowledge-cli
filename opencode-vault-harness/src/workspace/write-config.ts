import { mkdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';

export function writeVaultConfig(dir: string, vaultPath: string): string {
  const kvDir = resolve(dir, '.kv');
  mkdirSync(kvDir, { recursive: true });

  const configPath = resolve(kvDir, 'config.json');
  const config = { vaultPath };

  writeFileSync(configPath, JSON.stringify(config, null, 2) + '\n', 'utf-8');
  return configPath;
}
