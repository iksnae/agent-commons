// SPDX-License-Identifier: MPL-2.0
// Check the shipped navigation graph without fetching links or running examples.
import { readFile, stat, realpath } from 'node:fs/promises';
import path from 'node:path';

const root = await realpath(process.argv[2] || process.cwd());
const pending = ['README.md', 'docs/README.md', 'START-HERE.md', 'plugins/agent-commons/README.md'];
const visited = new Set();
const failures = [];
while (pending.length) {
  const relative = pending.pop();
  if (visited.has(relative)) continue;
  visited.add(relative);
  const file = path.resolve(root, relative);
  if (!file.startsWith(root + path.sep)) {
    failures.push(`path escapes bundle: ${relative}`);
    continue;
  }
  let text;
  try { text = await readFile(file, 'utf8'); }
  catch { failures.push(`missing guide: ${relative}`); continue; }
  if (['README.md', 'plugins/agent-commons/README.md'].includes(relative) && text.split('\n').length > 150) {
    failures.push(`${relative}: move detailed guidance out of the README`);
  }
  // Fenced examples are not navigation links.
  text = text.replace(/```[^]*?```/g, '');
  for (const match of text.matchAll(/\[[^\]]*\]\(([^\s)]+)\)/g)) {
    const href = match[1];
    if (/^[a-z]+:/i.test(href)) continue;
    const [name] = href.split('#');
    if (!name) continue;
    const target = path.resolve(path.dirname(file), decodeURIComponent(name));
    if (!target.startsWith(root + path.sep)) {
      failures.push(`${relative}: escaping link ${href}`);
      continue;
    }
    try {
      if (!(await stat(target)).isFile()) throw new Error('not a file');
      if (!(await realpath(target)).startsWith(root + path.sep)) throw new Error('external symlink');
      if (target.endsWith('.md')) pending.push(path.relative(root, target));
    } catch { failures.push(`${relative}: missing link ${href}`); }
  }
}
if (failures.length) {
  process.stderr.write(failures.join('\n') + '\n');
  process.exitCode = 1;
} else {
  process.stdout.write(`Documentation links verified across ${visited.size} guides.\n`);
}
