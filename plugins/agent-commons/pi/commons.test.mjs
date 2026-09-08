// SPDX-License-Identifier: MPL-2.0
import assert from 'node:assert/strict';
import test from 'node:test';
import { installCommons } from './commons.mjs';

const session = 'a196ca83-c293-43d8-995a-7b7f6089ca21';
function fixture(execute, env = { AGENT_COMMONS_CONNECTION: '/private/role.json', AGENT_COMMONS_BINARY: '/bin/commons' }) {
  const handlers = new Map(), messages = [], calls = [];
  installCommons({ on: (event, handler) => handlers.set(event, handler), sendMessage: (...args) => messages.push(args) }, {
    env, execute: async (...args) => { calls.push(args); return execute(...args); },
  });
  return { messages, calls, start: (reason = 'startup', id = session, parentSession) => handlers.get('session_start')({ reason }, { cwd: '/project', sessionManager: { getSessionId: () => id, getHeader: () => ({ id, parentSession }) } }) };
}

test('check-in uses exact native identity and emits only fixed guidance', async () => {
  const f = fixture(async () => ({ stdout: JSON.stringify({ identity: 'enrolled', attachment: { nativeId: session, runtime: 'pi' }, inbox: 'PRIVATE BODY', token: 'SECRET' }) }));
  await f.start();
  assert.deepEqual(f.calls[0][1], ['check-in', '--config', '/private/role.json', '--runtime', 'pi', '--native-session', session, '--launch-directory', '/project']);
  assert.equal(f.calls[0][2].shell, false);
  assert.equal(f.calls[0][2].timeout, 5000);
  assert.equal(f.messages[0][1].triggerTurn, false);
  assert.match(f.messages[0][0].content, /inbox/);
  assert.doesNotMatch(JSON.stringify(f.messages), /PRIVATE BODY|SECRET/);
});

test('failure is visible without replay or raw diagnostic leakage', async () => {
  const f = fixture(async () => { throw new Error('SECRET STDERR'); });
  await f.start();
  assert.equal(f.calls.length, 1);
  assert.match(f.messages[0][0].content, /failed/);
  assert.doesNotMatch(JSON.stringify(f.messages), /SECRET STDERR/);
});

test('missing configuration stays inactive; partial configuration fails closed', async () => {
  for (const env of [{}, { AGENT_COMMONS_CONNECTION: '/role' }]) {
    const f = fixture(() => assert.fail('must not execute'), env);
    await f.start();
    assert.equal(f.messages.length, Object.keys(env).length ? 1 : 0);
  }
});

test('without a connection the binary resolves the role from the project', async () => {
  const f = fixture(async () => ({ stdout: JSON.stringify({ identity: 'enrolled', attachment: { nativeId: session, runtime: 'pi' } }) }),
    { AGENT_COMMONS_BINARY: '/bin/commons' });
  await f.start();
  assert.deepEqual(f.calls[0][1], ['check-in', '--runtime', 'pi', '--native-session', session, '--launch-directory', '/project']);
  assert.match(f.messages[0][0].content, /inbox/);
});

test('a relative connection is refused rather than passed through', async () => {
  const f = fixture(() => assert.fail('must not execute'),
    { AGENT_COMMONS_CONNECTION: 'role.json', AGENT_COMMONS_BINARY: '/bin/commons' });
  await f.start();
  // The child must never run: a refusal that merely fails inside execute is
  // indistinguishable from passing a bad path through to the binary.
  assert.equal(f.calls.length, 0);
  assert.match(f.messages[0][0].content, /failed/);
});

test('forks, new sessions and invalid native IDs never attach', async () => {
  for (const [reason, id] of [['fork', session], ['new', session], ['startup', 'latest']]) {
    const f = fixture(() => assert.fail('must not execute'));
    await f.start(reason, id);
    assert.match(f.messages[0][0].content, /failed/);
  }
});

test('mismatched attachment cannot produce successful guidance', async () => {
  const f = fixture(async () => ({ stdout: JSON.stringify({ identity: 'enrolled', attachment: { nativeId: 'other', runtime: 'pi' } }) }));
  await f.start();
  assert.match(f.messages[0][0].content, /failed/);
});

test('startup on a forked transcript does not attach even after lease expiry', async () => {
  const f = fixture(() => assert.fail('must not execute'));
  await f.start('startup', session, '/parent/session.jsonl');
  assert.equal(f.calls.length, 0);
  assert.match(f.messages[0][0].content, /failed/);
});
