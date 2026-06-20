import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import fg from 'fast-glob';
import matter from 'gray-matter';

export interface SearchResult {
  file: string;
  score: number;
  title: string;
  snippet: string;
  frontmatter: Record<string, unknown>;
}

function extractTitle(content: string, frontmatterTitle: string | undefined, filePath: string): string {
  if (frontmatterTitle && frontmatterTitle.trim()) {
    return frontmatterTitle.trim();
  }
  const match = content.match(/^#\s+(.+)$/m);
  if (match) {
    return match[1].trim();
  }
  return filePath;
}

function extractSnippet(content: string, query: string): string {
  const lowerContent = content.toLowerCase();
  const lowerQuery = query.toLowerCase();
  const idx = lowerContent.indexOf(lowerQuery);

  if (idx === -1) {
    return content.slice(0, 200).replace(/\n/g, ' ').trim() + '...';
  }

  const start = Math.max(0, idx - 80);
  const end = Math.min(content.length, idx + query.length + 120);
  let snippet = content.slice(start, end).replace(/\n/g, ' ').trim();

  if (start > 0) snippet = '...' + snippet;
  if (end < content.length) snippet = snippet + '...';

  return snippet;
}

function scoreTokenMatches(content: string, query: string): number {
  const tokens = query.toLowerCase().split(/\s+/);
  const lowerContent = content.toLowerCase();
  let score = 0;

  for (const token of tokens) {
    const regex = new RegExp(token.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'gi');
    const matches = lowerContent.match(regex);
    if (matches) {
      score += matches.length * 10;
    }
  }

  return score;
}

export function search(vaultPath: string, query: string, maxResults: number = 10): SearchResult[] {
  const allMdFiles = fg.sync('**/*.md', {
    cwd: vaultPath,
    absolute: true,
    ignore: ['node_modules/**', '.git/**', '.kv/**', '.opencode/**'],
  });

  const results: SearchResult[] = [];

  for (const file of allMdFiles) {
    try {
      const raw = readFileSync(file, 'utf-8');
      const { data: frontmatter, content } = matter(raw);

      const title = extractTitle(content, frontmatter.title as string | undefined, file);
      const snippet = extractSnippet(content, query);
      const score = scoreTokenMatches(content, query);

      if (score > 0 || content.toLowerCase().includes(query.toLowerCase())) {
        results.push({
          file,
          score: score + (content.toLowerCase().includes(query.toLowerCase()) ? 5 : 0),
          title,
          snippet,
          frontmatter,
        });
      }
    } catch {
      continue;
    }
  }

  results.sort((a, b) => b.score - a.score);
  return results.slice(0, maxResults);
}
