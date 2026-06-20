import { loadDocuments, LoadedVault } from './load-documents';
import { search, SearchResult } from './search';

function renderSearchResults(results: SearchResult[]): string {
  if (results.length === 0) {
    return '*(Nenhum documento relevante encontrado no vault.)*\n';
  }

  let out = '## Documentos Relevantes do Vault\n\n';

  for (const r of results) {
    out += `### ${r.title}\n`;
    out += `- **Arquivo**: \`${r.file}\`\n`;
    if (r.frontmatter.tags) {
      const tags = Array.isArray(r.frontmatter.tags)
        ? r.frontmatter.tags.join(', ')
        : String(r.frontmatter.tags);
      out += `- **Tags**: ${tags}\n`;
    }
    out += `- **Score**: ${r.score}\n`;
    out += `\n> ${r.snippet}\n\n`;
    out += `---\n\n`;
  }

  return out;
}

function renderDocumentContent(doc: { title: string; relativePath: string; content: string }): string {
  const lines = doc.content.split('\n');
  if (lines.length <= 20) {
    return `### ${doc.title}\n*(fonte: \`${doc.relativePath}\`)*\n\n${doc.content}\n\n---\n\n`;
  }

  const preview = lines.slice(0, 20).join('\n');
  return `### ${doc.title}\n*(fonte: \`${doc.relativePath}\`)*\n\n${preview}\n\n*... (documento truncado, ${lines.length} linhas no total)*\n\n---\n\n`;
}

export function renderContext(vaultPath: string, taskDescription: string): string {
  const vault: LoadedVault = loadDocuments(vaultPath);
  const results: SearchResult[] = search(vaultPath, taskDescription, 8);

  let md = '';

  md += '> Este contexto foi gerado automaticamente pelo `kv context`.\n';
  md += `> **Vault**: \`${vaultPath}\`\n`;
  md += `> **Total de documentos no vault**: ${vault.documents.length}\n`;
  md += `> **Consulta**: "${taskDescription}"\n\n`;

  md += '## Task Description\n\n';
  md += `${taskDescription}\n\n`;

  md += renderSearchResults(results);

  if (vault.documents.length > 0) {
    md += '## Visão Geral do Vault\n\n';
    md += `| # | Documento | Título |\n`;
    md += `|---|-----------|--------|\n`;

    for (let i = 0; i < vault.documents.length; i++) {
      const d = vault.documents[i];
      md += `| ${i + 1} | \`${d.relativePath}\` | ${d.title} |\n`;
    }

    md += '\n';
  }

  return md;
}
