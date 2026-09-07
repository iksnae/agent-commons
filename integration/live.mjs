// SPDX-License-Identifier: MPL-2.0

// Explicitly opted-in real-runtime acceptance. Creates only private temporary
// state and a scratch target; never attaches to existing interactive sessions.
import { mkdtemp, mkdir, writeFile, readFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { spawn } from 'node:child_process';
import http from 'node:http';
import assert from 'node:assert/strict';

if (process.env.AGENT_COMMONS_LIVE !== '1') {
  throw new Error('Set AGENT_COMMONS_LIVE=1 to authorize actual Claude and Codex model runs.');
}
const root = await mkdtemp(join(tmpdir(), 'ac-live-'));
const state = join(root, 'state');
const target = join(root, 'target');
const socket = join(state, 'service.sock');
const binary = resolve('bin/agent-commons');
await mkdir(join(target, '.claude', 'agents'), { recursive: true });
await mkdir(join(target, '.codex', 'agents'), { recursive: true });
const role = '# Communication tester\nThis scratch project tests message delivery. Its project word is SEQUOIA. Answer briefly, include SEQUOIA and the message nonce. Use the provided context tool when requested. Do not send further messages or create tasks.\n';
await writeFile(join(target, '.claude', 'agents', 'tester.md'), role);
await writeFile(join(target, '.codex', 'agents', 'tester.md'), role);
let server;
let logs = '';
let operator;

const sleep = ms => new Promise(r => setTimeout(r, ms));
function call(token, method, params = {}) {
  return new Promise((resolve, reject) => {
    const req = http.request({ socketPath: socket, path: '/rpc', method: 'POST',
      headers: { authorization: `Bearer ${token}`, 'content-type': 'application/json' } }, res => {
      let body = '';
      res.on('data', chunk => body += chunk);
      res.on('end', () => {
        try {
          const value = JSON.parse(body);
          if (res.statusCode !== 200 || value.error) reject(new Error(`${method}: ${body}`));
          else resolve(value.result);
        } catch (err) { reject(err); }
      });
    });
    req.on('error', reject);
    req.end(JSON.stringify({ method, params }));
  });
}
async function start() {
  server = spawn(binary, ['serve', '--state', state], { stdio: ['ignore', 'pipe', 'pipe'] });
  server.stdout.on('data', b => logs += b);
  server.stderr.on('data', b => logs += b);
  server.on('error', e => logs += String(e));
  for (let i = 0; i < 100; i++) {
    if (server.exitCode !== null) throw new Error(`service exited: ${logs}`);
    try {
      operator = (await readFile(join(state, 'operator.token'), 'utf8')).trim();
      await call(operator, 'sessions.list');
      return;
    } catch { await sleep(50); }
  }
  throw new Error(`service not ready: ${logs}`);
}
async function stop() {
  if (!server || server.exitCode !== null) return;
  await new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('service failed to stop')), 15000);
    server.once('exit', () => { clearTimeout(timer); resolve(); });
    server.kill('SIGTERM');
  });
}
function deliveries(value) { return Array.isArray(value) ? value : value.messages ?? value.deliveries; }
async function waitRoundTrip(sender, recipient, nonce) {
  for (let i = 0; i < 180; i++) {
    const incoming = deliveries(await call(operator, 'inbox.list', { sessionId: recipient }));
    const outgoing = deliveries(await call(operator, 'inbox.list', { sessionId: sender }));
    const request = incoming.find(x => x.text.includes(nonce) && x.kind === 'message');
    const reply = outgoing.find(x => x.text.includes(nonce) && x.kind === 'result');
    for (const d of [request, reply]) {
      if (d && ['failed', 'interrupted'].includes(d.status)) throw new Error(`${nonce} ${d.to}: ${d.error}`);
    }
    if (reply && ['completed', 'read', 'succeeded'].includes(reply.status)) {
      assert.match(request.output, /SEQUOIA/);
      assert.match(request.output, /AMBER/);
      assert.match(reply.output, /SEQUOIA/);
      const evidence = { nonce, request: { id: request.id, status: request.status, output: request.output },
        reply: { id: reply.id, status: reply.status, output: reply.output } };
      console.log(JSON.stringify(evidence));
      return evidence;
    }
    if (i % 15 === 0) console.log(JSON.stringify({ nonce, waiting: true, request: request?.status, reply: reply?.status }));
    await sleep(1000);
  }
  throw new Error(`${nonce}: timed out waiting for automatically executed response`);
}

try {
  await start();
  const registered = {};
  for (const runtime of ['claude', 'codex']) {
    registered[runtime] = await call(operator, 'sessions.register', {
      id: runtime, target, team: 'acceptance', role: 'tester', runtime, mode: 'managed',
      instructions: 'This is a communication-only test. For a result message acknowledge receipt including the nonce; do not send a reply tool call. For a new request, use context.get on the named context then answer with its color and the project word.'
    });
  }
  await call(operator, 'context.put', { id: 'brief', target, text: 'The shared context color is AMBER.', expectedVersion: 0 });
  const evidence = [];
  for (const [sender, recipient, nonce] of [['codex', 'claude', 'RETURN-001'], ['claude', 'codex', 'RETURN-002']]) {
    const params = { to: recipient, text: `${nonce}: Read shared context brief with the context.get MCP tool. Return its color, the project word, and this nonce.`, idempotencyKey: nonce, contextId: 'brief', contextVersion: 1 };
    const token = registered[sender].token;
    assert.ok(token, 'registration must provide the scoped session token');
    const first = await call(token, 'messages.send', params);
    const duplicate = await call(token, 'messages.send', params);
    assert.equal(first.id, duplicate.id);
    evidence.push(await waitRoundTrip(sender, recipient, nonce));
    await stop();
    await start();
  }
  const sessions = await call(operator, 'sessions.list');
  console.log(JSON.stringify({ success: true, state: root, sessions, evidence }));
} finally {
  await stop();
  console.log(`Private acceptance receipts retained at ${root}`);
}
