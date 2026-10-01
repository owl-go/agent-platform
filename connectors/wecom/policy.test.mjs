import assert from 'node:assert/strict';
import { mkdirSync, mkdtempSync, rmSync, symlinkSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { test } from 'node:test';
import { invocationRejectionReason, validateInvocation } from './policy.mjs';

const command = (prefix, input) => [...prefix.split(' '), '--json', JSON.stringify(input)];

test('reviewed methods cover every requested product while rejecting undeclared methods', () => {
  for (const prefix of [
    'message aibot send', 'mail send', 'doc contents append', 'doc members update',
    'sheet rows append', 'smartsheet records add', 'smartpage pages update',
    'todo finish', 'calendar schedules free list', 'meeting original get',
    'disk files download', 'contact users search',
  ]) {
    const input = prefix === 'contact users search' ? { keywords: ['张三'] }
      : prefix === 'doc contents append' ? { docid: 'doc-1' }
      : prefix === 'message aibot send' ? { chat_id: 'chat-1', msg_type: 'markdown', markdown: { content: '你好' } }
      : {};
    assert.equal(validateInvocation(command(prefix, input))?.policy.command, prefix);
  }
  assert.equal(validateInvocation(command('mail delete', {})), null);
  assert.equal(validateInvocation(['doc', 'contents', 'append', '--schema']), null);
  assert.equal(validateInvocation(['message', 'aibot', 'sessions', 'list'])?.policy.identity, 'bot');
  assert.equal(validateInvocation(['message', 'aibot', 'sessions', 'list', '--json', '{}']), null);
});

test('message send accepts supported media and rejects missing media or oversized markdown', () => {
  for (const type of ['image', 'file', 'voice', 'video']) {
    assert.ok(validateInvocation(command('message aibot send', { chat_id: 'chat-1', msg_type: type, [type]: { media_id: 'mc-1' } })));
    assert.equal(validateInvocation(command('message aibot send', { chat_id: 'chat-1', msg_type: type, [type]: {} })), null);
  }
  assert.equal(validateInvocation(command('message aibot send', { chat_id: 'chat-1', msg_type: 'markdown', markdown: { content: 'x'.repeat(20481) } })), null);
});

test('file arguments stay inside the current workspace, including through symlinks', () => {
  const root = mkdtempSync(join(tmpdir(), 'wecom-policy-'));
  try {
    const workspace = join(root, 'workspace');
    mkdirSync(workspace);
    const inside = join(workspace, 'data.xlsx');
    const outside = join(root, 'private.txt');
    writeFileSync(inside, 'sheet');
    writeFileSync(outside, 'private');
    symlinkSync(outside, join(workspace, 'shortcut'));
    assert.ok(validateInvocation(command('sheet import', { file_name: 'data.xlsx', file_path: inside }), workspace));
    assert.equal(validateInvocation(command('sheet import', { file_name: 'private.txt', file_path: outside }), workspace), null);
    assert.equal(validateInvocation(command('sheet import', { file_name: 'shortcut', file_path: join(workspace, 'shortcut') }), workspace), null);
    assert.equal(validateInvocation(command('mail send', { attachments: [{ file_path: outside }] }), workspace), null);
    assert.match(invocationRejectionReason(command('smartpage import', { name: 'test', file_path: join(workspace, 'missing.md') }), workspace), /does not exist/);
    assert.match(invocationRejectionReason(command('smartpage import', { name: 'test', file_path: join(workspace, 'shortcut') }), workspace), /invalid arguments/);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});
