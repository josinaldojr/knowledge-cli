import { readFileSync } from 'node:fs';
import { resolve, relative } from 'node:path';
import fg from 'fast-glob';
import matter from 'gray-matter';

export interface Document {
  path: string;
  relativePath: string;
  title: string;
  content: string;
  frontmatter: Record<string, unknown>;
}

export interface LoadedVault {
  vaultPath: string;
  documents: Document[];
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

export function loadDocuments(vaultPath: string): LoadedVault {
  const allMdFiles = fg.sync('**/*.md', {
    cwd: vaultPath,
    absolute: true,
    ignore: ['node_modules/**', '.git/**', '.kv/**', '.opencode/**'],
  });

  const documents: Document[] = [];

  for (const file of allMdFiles) {
    try {
      const raw = readFileSync(file, 'utf-8');
      const { data: frontmatter, content } = matter(raw);

      documents.push({
        path: file,
        relativePath: relative(vaultPath, file),
        title: extractTitle(content, frontmatter.title as string | undefined, file),
        content,
        frontmatter,
      });
    } catch {
      continue;
    }
  }

  return { vaultPath, documents };
}
