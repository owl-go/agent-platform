import assert from 'node:assert/strict';
import test from 'node:test';

process.env.AGENT_WORKSPACE_LOCAL_ROOT = '/tmp/agent workspace local test';
const { buildConfig, buildRealm } = await import('./local-platform.mjs');

const state = {
  adminPassword: 'local-password',
  serviceSecret: 'service-secret',
  dataEncryptionKey: 'encryption-key',
};

test('local realm uses loopback redirects and materialized credentials', () => {
  const realm = buildRealm({
    realm: 'production',
    sslRequired: 'external',
    clients: [
      { redirectUris: [], webOrigins: [], attributes: {} },
      { secret: '${KEYCLOAK_SERVICE_CLIENT_SECRET}' },
    ],
    users: [{ credentials: [{ value: '${PLATFORM_ADMIN_PASSWORD}' }] }],
  }, state);

  assert.equal(realm.realm, 'agent-workspace-local');
  assert.equal(realm.sslRequired, 'none');
  assert.deepEqual(realm.clients[0].redirectUris, ['http://127.0.0.1:14173/*']);
  assert.deepEqual(realm.clients[0].webOrigins, ['http://127.0.0.1:14173']);
  assert.equal(realm.clients[1].secret, state.serviceSecret);
  assert.equal(realm.users[0].credentials[0].value, state.adminPassword);
  assert.equal(realm.users[1].serviceAccountClientId, 'agent-workspace-admin');
});

test('local configuration binds to loopback and disables every Runtime Engine', () => {
  const config = buildConfig(state);

  assert.match(config, /address: 127\.0\.0\.1:19080/);
  assert.match(config, /issuer: http:\/\/127\.0\.0\.1:19081\/realms\/agent-workspace-local/);
  assert.match(config, /endpoint: 127\.0\.0\.1:19090/);
  assert.match(config, /root: "\/tmp\/agent workspace local test\/workspaces"/);
  assert.equal((config.match(/available: false/g) || []).length, 5);
  assert.doesNotMatch(config, /available: true/);
  assert.match(config, /runtime: runsc/);
});
