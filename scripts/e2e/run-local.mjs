import { spawn, spawnSync } from 'node:child_process';
import { mkdtempSync, writeFileSync, mkdirSync, readFileSync, rmSync, openSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { randomBytes, createHash, createHmac } from 'node:crypto';
const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const temp = mkdtempSync(resolve(tmpdir(), 'agent-workspace-e2e-'));
const output = resolve(root, 'output/playwright');
mkdirSync(output, { recursive: true });
const children = [], containers = [];
const password = `E2e!${randomBytes(16).toString('hex')}`;
const secret = randomBytes(24).toString('hex');
const ports = { api: 19080, identity: 19081, web: 14173, db: 15439, minio: 19090 };
const web = `http://127.0.0.1:${ports.web}`;
const identity = `http://127.0.0.1:${ports.identity}`;
const realm = 'agent-workspace-e2e';
function run(command, args, options = {}) {
  const r = spawnSync(command, args, { cwd: root, encoding: 'utf8', ...options });
  if (r.status !== 0) throw new Error(`${command} failed: ${r.stderr || r.error || r.stdout}`);
  return r.stdout.trim();
}
function start(command, args, name, env = {}) {
  const fd = openSync(resolve(temp, `${name}.log`), 'w', 0o600);
  const child = spawn(command, args, { cwd: root, env: { ...process.env, ...env }, stdio: ['ignore', fd, fd] });
  children.push(child);
  return child;
}
async function ready(url, timeout = 120000) {
  const end = Date.now() + timeout;
  while (Date.now() < end) {
    try { if ((await fetch(url)).ok) return; } catch {}
    await new Promise(r => setTimeout(r, 500));
  }
  throw new Error(`Service readiness timed out: ${url}; private logs: ${temp}`);
}
function docker(name, args) {
  const full = `aw-e2e-${process.pid}-${name}`;
  containers.push(full);
  run('docker', ['run', '-d', '--name', full, ...args]);
  return full;
}
async function bucket() {
  const host = `127.0.0.1:${ports.minio}`;
  const date = new Date().toISOString().replace(/[:-]|\.\d{3}/g, '');
  const day = date.slice(0, 8), hash = createHash('sha256').update('').digest('hex');
  const canonical = `PUT\n/agent-workspace-e2e\n\nhost:${host}\nx-amz-content-sha256:${hash}\nx-amz-date:${date}\n\nhost;x-amz-content-sha256;x-amz-date\n${hash}`;
  const scope = `${day}/us-east-1/s3/aws4_request`;
  const hmac = (key, value) => createHmac('sha256', key).update(value).digest();
  const key = hmac(hmac(hmac(hmac(`AWS4${secret}`, day), 'us-east-1'), 's3'), 'aws4_request');
  const signature = hmac(key, `AWS4-HMAC-SHA256\n${date}\n${scope}\n${createHash('sha256').update(canonical).digest('hex')}`).toString('hex');
  const r = await fetch(`http://${host}/agent-workspace-e2e`, { method: 'PUT', headers: { 'x-amz-date': date, 'x-amz-content-sha256': hash, Authorization: `AWS4-HMAC-SHA256 Credential=e2e/${scope}, SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature=${signature}` } });
  if (!r.ok) throw new Error(`Create private test bucket: ${r.status}`);
}
let exitCode = 1;
try {
  const seed = JSON.parse(readFileSync(resolve(root, 'deploy/platform/config/keycloak-realm.json'), 'utf8'));
  seed.realm = realm; seed.sslRequired = 'none';
  seed.clients[0].redirectUris = [`${web}/*`]; seed.clients[0].webOrigins = [web];
  seed.clients[0].attributes['post.logout.redirect.uris'] = `${web}/*`;
  seed.clients[1].secret = secret;
  seed.users[0].credentials[0].value = password;
  seed.users.push({ username: 'service-account-agent-workspace-admin', enabled: true, serviceAccountClientId: 'agent-workspace-admin', clientRoles: { 'realm-management': ['manage-users', 'view-users', 'query-users'] } });
  writeFileSync(resolve(temp, 'realm.json'), JSON.stringify(seed), { mode: 0o644 });
  writeFileSync(resolve(temp, 'db.env'), `POSTGRES_USER=e2e\nPOSTGRES_PASSWORD=${secret}\nPOSTGRES_DB=e2e\n`, { mode: 0o600 });
  writeFileSync(resolve(temp, 'minio.env'), `MINIO_ROOT_USER=e2e\nMINIO_ROOT_PASSWORD=${secret}\n`, { mode: 0o600 });
  run('python3', ['scripts/build-identity-theme.py', resolve(temp, 'themes')]);
  console.log('Starting isolated PostgreSQL, Keycloak and MinIO...');
  const db = docker('db', ['--env-file', resolve(temp, 'db.env'), '-p', `127.0.0.1:${ports.db}:5432`, 'postgres:17-alpine']);
  docker('identity', ['-p', `127.0.0.1:${ports.identity}:8080`, '-v', `${temp}/realm.json:/opt/keycloak/data/import/realm.json:ro`, '-v', `${temp}/themes:/opt/keycloak/themes:ro`, process.env.E2E_KEYCLOAK_IMAGE || 'quay.io/keycloak/keycloak@sha256:f1f1f01e472c8a78df40d8f2a49a925274eda4d3d80d5f6edbb5c880ee3c01c6', 'start-dev', '--import-realm']);
  docker('minio', ['--env-file', resolve(temp, 'minio.env'), '-p', `127.0.0.1:${ports.minio}:9000`, 'minio/minio:latest', 'server', '/data']);
  await Promise.all([ready(`${identity}/realms/${realm}/.well-known/openid-configuration`), ready(`http://127.0.0.1:${ports.minio}/minio/health/ready`)]);
  run('docker', ['exec', db, 'pg_isready', '-U', 'e2e']);
  await bucket();
  mkdirSync(resolve(temp, 'workspace')); writeFileSync(resolve(temp, 'known_hosts'), '');
  const config = `api:\n  address: 127.0.0.1:${ports.api}\n  read_header_timeout: 5s\n  idle_timeout: 60s\n  shutdown_timeout: 10s\nauthentication:\n  mode: oidc\n  issuer: ${identity}/realms/${realm}\n  audience: agent-platform-api\n  client_id: agent-platform-web\n  redirect_uri: ${web}/auth/callback\n  logout_redirect_uri: ${web}\n  signing_algorithms: [RS256]\n  discovery_timeout: 10s\n  jwks_timeout: 10s\naccounts:\n  keycloak_base_url: ${identity}\n  realm: ${realm}\n  admin_client_id: agent-workspace-admin\n  admin_client_secret: ${secret}\n  bootstrap_subject: ${seed.users[0].id}\n  bootstrap_username: platform-admin\n  bootstrap_email: platform-admin@agent-workspace.local\n  bootstrap_display_name: Platform Administrator\nworkspace:\n  root: ${temp}/workspace\n  known_hosts: ${temp}/known_hosts\nsecurity:\n  data_encryption_key: ${randomBytes(32).toString('base64')}\nworker:\n  runtimes:\n    claude: {available: false}\n    codex: {available: false}\n    hermes: {available: false}\n    openclaw: {available: false}\n    pi: {available: false}\ndatabase:\n  dsn: postgres://e2e:${secret}@127.0.0.1:${ports.db}/e2e?sslmode=disable\n  max_open_connections: 20\n  max_idle_connections: 10\n  connection_max_idle: 5m\n  connection_max_lifetime: 30m\nobject_store:\n  provider: minio\n  minio:\n    endpoint: 127.0.0.1:${ports.minio}\n    access_key: e2e\n    secret_key: ${secret}\n    bucket: agent-workspace-e2e\n    secure: false\n`;
  writeFileSync(resolve(temp, 'platform.yaml'), config, { mode: 0o600 });
  console.log('Building and starting current API and web source...');
  run('go', ['-C', 'backend', 'build', '-o', resolve(temp, 'api'), './cmd/api']);
  start(resolve(temp, 'api'), ['-config', resolve(temp, 'platform.yaml')], 'api');
  await ready(`http://127.0.0.1:${ports.api}/healthz`, 30000);
  const env = { VITE_OIDC_AUTHORITY: `${identity}/realms/${realm}`, VITE_OIDC_CLIENT_ID: 'agent-platform-web', VITE_OIDC_REDIRECT_URI: `${web}/auth/callback`, VITE_OIDC_POST_LOGOUT_REDIRECT_URI: web, VITE_API_PROXY_TARGET: `http://127.0.0.1:${ports.api}` };
  start('pnpm', ['--dir', 'frontend', 'exec', 'vite', '--host', '127.0.0.1', '--port', String(ports.web), '--strictPort'], 'web', env);
  await ready(web, 30000);
  const result = spawn('pnpm', ['--dir', 'frontend', 'exec', 'playwright', 'test', ...process.argv.slice(2)], { cwd: root, stdio: 'inherit', env: { ...process.env, E2E_BASE_URL: web, E2E_PASSWORD: password, E2E_IDENTITY: identity, E2E_REALM: realm, E2E_CLIENT_SECRET: secret } });
  children.push(result);
  exitCode = await new Promise(r => result.on('exit', code => r(code ?? 1)));
} catch (error) { console.error(error.message); }
finally {
  for (const child of children) child.kill('SIGTERM');
  for (const name of containers.reverse()) spawnSync('docker', ['rm', '-f', '-v', name], { stdio: 'ignore' });
  if (exitCode === 0) rmSync(temp, { recursive: true, force: true });
  else console.error(`Private diagnostic files retained at ${temp}; do not publish credentials or logs.`);
}
process.exitCode = exitCode;
