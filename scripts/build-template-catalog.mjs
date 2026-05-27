import { createHash } from 'node:crypto';
import { mkdir, readdir, copyFile, readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';

const root = process.cwd();
const sourceDir = path.join(root, 'internal', 'registry', 'templates');
const docsTemplatesDir = path.join(root, 'docs', 'templates');

await mkdir(docsTemplatesDir, { recursive: true });

const files = (await readdir(sourceDir))
  .filter((file) => file.endsWith('.yaml'))
  .sort();

const templates = [];

for (const file of files) {
  const sourcePath = path.join(sourceDir, file);
  const content = await readFile(sourcePath, 'utf8');
  await copyFile(sourcePath, path.join(docsTemplatesDir, file));
  templates.push({
    id: field(content, 'id') || file.replace(/\.yaml$/, ''),
    name: field(content, 'name') || file.replace(/\.yaml$/, ''),
    description: field(content, 'description'),
    category: field(content, 'category'),
    icon: field(content, 'icon'),
    image: field(content, 'image'),
    url: `templates/${file}`,
    sha256: createHash('sha256').update(content).digest('hex'),
    content,
  });
}

await writeFile(
  path.join(docsTemplatesDir, 'index.json'),
  `${JSON.stringify({ version: 1, generated_at: new Date().toISOString(), templates }, null, 2)}\n`,
);

function field(content, key) {
  const match = content.match(new RegExp(`^${key}:\\s*(.+?)\\s*$`, 'm'));
  if (!match) return '';
  return match[1].replace(/^['"]|['"]$/g, '');
}
