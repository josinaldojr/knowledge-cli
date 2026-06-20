import { detectWorkspace } from '../workspace/detect-workspace';
import { search, SearchResult } from '../vault/search';

export function findCommand(query: string): void {
  const workspace = detectWorkspace();

  if (!workspace) {
    console.error('Erro: Nenhum workspace OpenCode encontrado. Execute "kv init --vault <caminho>" primeiro.');
    process.exit(1);
  }

  const { vaultPath } = workspace.config;
  console.log(`Buscando no vault "${vaultPath}" por: "${query}"\n`);

  const results: SearchResult[] = search(vaultPath, query, 10);

  if (results.length === 0) {
    console.log('Nenhum documento encontrado.');
    return;
  }

  console.log(`Encontrados ${results.length} documento(s):\n`);

  for (let i = 0; i < results.length; i++) {
    const r = results[i];
    console.log(`${i + 1}. ${r.title}`);
    console.log(`   Arquivo: ${r.file}`);
    console.log(`   Score: ${r.score}`);
    if (r.frontmatter.tags) {
      const tags = Array.isArray(r.frontmatter.tags)
        ? r.frontmatter.tags.join(', ')
        : String(r.frontmatter.tags);
      console.log(`   Tags: ${tags}`);
    }
    console.log(`   Trecho: ${r.snippet}`);
    console.log('');
  }
}
