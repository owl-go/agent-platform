'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const { credential, validateArguments } = require('./launcher.cjs');

test('credential bridge accepts one UserToken and rejects other identities without echoing secrets', () => {
  assert.deepEqual(credential({ CONNECTOR_CREDENTIALS_JSON: '{"user_token":"opaque-value"}' }), {kind: 'legacy', token: 'opaque-value'});
  for (const value of ['{}', 'null', '[]', '{"user_token":"a b"}', '{"user_token":"opaque-value","host":"https://evil.example"}', '{"access_token":"opaque-value"}', 'opaque-value']) {
    assert.throws(() => credential({ CONNECTOR_CREDENTIALS_JSON: value }), error => !error.message.includes('opaque-value'));
  }
});

test('business operations cannot escape to endpoint, credential, debug or maintenance commands', () => {
  for (const args of [
    ['auth', 'login'], ['config', 'set', '--host', 'https://evil.example'], ['setup'], ['skill', 'update'],
    ['task', 'query', '--config=/tmp/other-account'], ['task', 'query', '-dh'],
    ['task', 'query', '--', '--config', '/tmp/other-account'],
    ['task', 'query', '--host', 'https://evil.example'], ['task', 'query', '--debug=true'],
    ['tools', 'call', 'teambition.task.delete'], ['--version', 'task', 'move'],
    ['project', '--help', 'delete'], ['tools', 'call', 'teambition.docs.get', 'teambition.task.delete'],
  ]) assert.throws(() => validateArguments(args), args.join(' '));
});

test('reviewed reads, writes and document stdin remain distinct', () => {
  assert.equal(validateArguments(['task', 'query', '--json']).risk, 'low');
  assert.equal(validateArguments(['task', 'create', '--help']).risk, 'high');
  assert.equal(validateArguments(['task', 'move', '--help']).risk, 'high');
  assert.equal(validateArguments(['tools', 'call', 'teambition.docs.get', '--arguments-file', '-', '--json']).risk, 'low');
});

test('OAuth credentials stay distinct from legacy tokens and expired grants fail before execution', () => {
 const value = { access_token: 'oauth-test-token', refresh_token: 'refresh-test', client_id: 'dcr_test', access_expires_at: new Date(Date.now()+60000).toISOString() };
 assert.deepEqual(credential({CONNECTOR_CREDENTIALS_JSON: JSON.stringify(value)}), {kind:'oauth',token:'oauth-test-token'});
 assert.throws(() => credential({CONNECTOR_CREDENTIALS_JSON: JSON.stringify({...value, access_expires_at:'2000-01-01T00:00:00Z'})}), /AUTH_EXPIRED/);
 assert.throws(() => credential({CONNECTOR_CREDENTIALS_JSON: JSON.stringify({...value, host:'https://evil.example'})}));
});
