import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';

export interface VaultConfig {
  vaultPath: string;
}

export function detectWorkspace(dir: string = process.cwd()): { configPath: string; config: VaultConfig } | null {
  const configPath = resolve(dir, '.kv', 'config.json');

  if (!existsSync(configPath)) {
    return null;
  }

  try {
    const raw = readFileSync(configPath, 'utf-8');
    const config: VaultConfig = JSON.parse(raw);

    if (!config.vaultPath || typeof config.vaultPath !== 'string') {
      return null;
    }

    return { configPath, config };
  } catch {
    return null;
  }
}
