// SPDX-License-Identifier: MPL-2.0
import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, mkdir, writeFile, rm, symlink } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';

const checker = fileURLToPath(new URL('./check-docs.mjs', import.meta.url));

test('documentation gate follows local guides and rejects broken bundle navigation', async t => {
  const root = await mkdtemp(path.join(tmpdir(), 'commons-docs-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  for (const name of ['README.md', 'docs/README.md', 'START-HERE.md', 'plugins/agent-commons/README.md']) {
    await mkdir(path.dirname(path.join(root, name)), { recursive: true });
    await writeFile(path.join(root, name), '# Guide\n');
  }
  const run = () => spawnSync(process.execPath, [checker, root], { encoding: 'utf8', timeout: 5000 });
  assert.equal(run().status, 0);
  await writeFile(path.join(root, 'README.md'), '[Guide](docs/README.md)\n');
  assert.equal(run().status, 0);
  await writeFile(path.join(root, 'docs/README.md'), '[Missing](missing.md)\n');
  assert.equal(run().status, 1);
  await writeFile(path.join(root, 'docs/README.md'), '[Escape](../../outside.md)\n');
  assert.equal(run().status, 1);
  await writeFile(path.join(root, 'docs/README.md'), '```md\n[Example](missing.md)\n```\n');
  assert.equal(run().status, 0);
  await writeFile(path.join(root, 'README.md'), '# Guide\n'.repeat(151));
  assert.equal(run().status, 1);
  await symlink(checker, path.join(root, 'external.md'));
  await writeFile(path.join(root, 'README.md'), '[External](external.md)\n');
  assert.equal(run().status, 1);
});
