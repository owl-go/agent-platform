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

test('fresh source installations use the configured admin without renaming retained legacy data', () => {
  const template = { clients: [{ attributes: {} }, {}], users: [{ credentials: [{}] }] };
  const fresh = buildRealm(template, { ...state, adminUsername: 'admin', adminPassword: 'admin123' });
  assert.equal(fresh.users[0].username, 'admin');
  assert.equal(fresh.users[0].credentials[0].value, 'admin123');
  assert.equal(buildRealm(template, state).users[0].username, 'platform-admin');
  assert.match(buildConfig({ ...state, adminUsername: 'admin' }), /bootstrap_username: admin\n/);
});

test('local configuration binds to loopback and disables engines without an image', () => {
  const config = buildConfig(state);

  assert.match(config, /address: 127\.0\.0\.1:19080/);
  assert.match(config, /issuer: http:\/\/127\.0\.0\.1:19081\/realms\/agent-workspace-local/);
  assert.match(config, /endpoint: 127\.0\.0\.1:19090/);
  assert.match(config, /root: "\/tmp\/agent workspace local test\/workspaces"/);
  assert.equal((config.match(/available: false/g) || []).length, 5);
  assert.doesNotMatch(config, /available: true/);
  assert.match(config, /profile: local\n  runtime: runc/);
});

test('local Worker uses Docker host services and enables pinned engines only', () => {
  const image = `sha256:${'a'.repeat(64)}`;
  const config = buildConfig(state, { container: true, runtimeImage: image, engines: ['codex'] });
  assert.match(config, /management_address: 0\.0\.0\.0:19082/);
  assert.match(config, /endpoint: host\.docker\.internal:19090/);
  assert.match(config, /codex: \{available: true, native_resume: false, image_digest: "sha256:a{64}"/);
  assert.equal((config.match(/available: false/g) || []).length, 4);
});
