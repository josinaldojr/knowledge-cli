#!/usr/bin/env node

import { Command } from 'commander';
import { initCommand } from './commands/init';
import { findCommand } from './commands/find';
import { contextCommand } from './commands/context';

const pkg = require('../package.json');

const program = new Command();

program
  .name('kv')
  .description('OpenCode Vault Harness — CLI para conectar OpenCode a um knowledge-vault local')
  .version(pkg.version);

program
  .command('init')
  .description('Inicializa um workspace OpenCode conectado a um knowledge-vault')
  .requiredOption('--vault <path>', 'Caminho absoluto para o knowledge-vault local')
  .action((options: { vault: string }) => {
    initCommand(options.vault);
  });

program
  .command('find')
  .description('Busca documentos no vault conectado')
  .argument('<query>', 'Termo de busca')
  .action((query: string) => {
    findCommand(query);
  });

program
  .command('context')
  .description('Gera .opencode/context.md com contexto do vault para uma tarefa')
  .argument('<task>', 'Descrição da tarefa')
  .action((task: string) => {
    contextCommand(task);
  });

program.parse();

export { program };
