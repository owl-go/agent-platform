#!/usr/bin/env node

import { spawn, spawnSync } from 'node:child_process';
import { closeSync, existsSync, mkdirSync, openSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { createHash, createHmac, randomBytes } from 'node:crypto';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const stateRoot = resolve(process.env.AGENT_WORKSPACE_LOCAL_ROOT || join(homedir(), '.local', 'share', 'agent-workspace'));
const stateFile = join(stateRoot, 'state.json');
const processFile = join(stateRoot, 'processes.json');
const configFile = join(stateRoot, 'config', 'platform.yaml');
const identityImportRoot = join(stateRoot, 'config', 'identity-import');
const ports = { api: 19080, worker: 19082, identity: 19081, web: 14173, database: 15439, minio: 19090 };
const webOrigin = `http://127.0.0.1:${ports.web}`;
const identityOrigin = `http://127.0.0.1:${ports.identity}`;
const realmName = 'agent-workspace-local';
const images = {
  postgres: 'postgres@sha256:ef257d85f76e48da1c64832459b59fcaba1a4dac97bf5d7450c77753542eee94',
  keycloak: 'quay.io/keycloak/keycloak@sha256:f1f1f01e472c8a78df40d8f2a49a925274eda4d3d80d5f6edbb5c880ee3c01c6',
};
const containers = {
  database: 'agent-workspace-local-database',
  identity: 'agent-workspace-local-identity',
};
const legacyMinioContainer = 'agent-workspace-local-minio';
const minioRelease = 'RELEASE.2025-10-15T17-29-55Z';
const ownerLabel = 'com.agent-workspace.local=true';

function fail(message) {
  throw new Error(message);
}

function run(command, args, options = {}) {
  const result = spawnSync(command, args, { cwd: root, encoding: 'utf8', ...options });
  if (result.status !== 0) {
    const detail = (result.stderr || result.stdout || result.error?.message || '').trim();
    fail(`${command} ${args.join(' ')} failed${detail ? `: ${detail}` : ''}`);
  }
  return typeof result.stdout === 'string' ? result.stdout.trim() : '';
}

function commandExists(command) {
  const result = spawnSync(command, ['--version'], { encoding: 'utf8' });
  return !result.error;
}

function writePrivate(path, body) {
  writeFileSync(path, body, { mode: 0o600 });
}

function readJSON(path) {
  return JSON.parse(readFileSync(path, 'utf8'));
}

function saveJSON(path, value) {
  writePrivate(path, `${JSON.stringify(value, null, 2)}\n`);
}

export function buildRealm(template, state) {
  const realm = structuredClone(template);
  realm.realm = realmName;
  realm.sslRequired = 'none';
  realm.clients[0].redirectUris = [`${webOrigin}/*`];
  realm.clients[0].webOrigins = [webOrigin];
  realm.clients[0].attributes['post.logout.redirect.uris'] = `${webOrigin}/*`;
  realm.clients[1].secret = state.serviceSecret;
  realm.users[0].credentials[0].value = state.adminPassword;
  realm.users.push({
    username: 'service-account-agent-workspace-admin',
    enabled: true,
    serviceAccountClientId: 'agent-workspace-admin',
    clientRoles: { 'realm-management': ['manage-users', 'view-users', 'query-users'] },
  });
  return realm;
}

export function buildConfig(state) {
  const workspace = join(stateRoot, 'workspaces');
  const knownHosts = join(stateRoot, 'config', 'known_hosts');
  const credentials = join(stateRoot, 'run-credentials');
  const yamlString = value => JSON.stringify(value);
  return `api:
  address: 127.0.0.1:${ports.api}
  read_header_timeout: 5s
  idle_timeout: 60s
  shutdown_timeout: 10s
authentication:
  mode: oidc
  issuer: ${identityOrigin}/realms/${realmName}
  audience: agent-platform-api
  client_id: agent-platform-web
  redirect_uri: ${webOrigin}/auth/callback
  logout_redirect_uri: ${webOrigin}
  signing_algorithms: [RS256]
  discovery_timeout: 10s
  jwks_timeout: 10s
accounts:
  keycloak_base_url: ${identityOrigin}
  realm: ${realmName}
  admin_client_id: agent-workspace-admin
  admin_client_secret: ${state.serviceSecret}
  bootstrap_subject: 11111111-1111-4111-8111-111111111111
  bootstrap_username: platform-admin
  bootstrap_email: platform-admin@agent-workspace.local
  bootstrap_display_name: Platform Administrator
workspace:
  root: ${yamlString(workspace)}
  known_hosts: ${yamlString(knownHosts)}
security:
  data_encryption_key: ${state.dataEncryptionKey}
worker:
  management_address: 127.0.0.1:${ports.worker}
  shutdown_timeout: 10s
  poll_interval: 1s
  runtime_idle_timeout: 30m
  credential_temp_root: ${yamlString(credentials)}
  sandbox_uid: 65532
  sandbox_gid: 65532
  cli_builder:
    enabled: false
  runtimes:
    claude: {available: false}
    codex: {available: false}
    hermes: {available: false}
    openclaw: {available: false}
    pi: {available: false}
database:
  dsn: postgres://local:${state.serviceSecret}@127.0.0.1:${ports.database}/agent_workspace?sslmode=disable
  max_open_connections: 20
  max_idle_connections: 10
  connection_max_idle: 5m
  connection_max_lifetime: 30m
object_store:
  provider: minio
  minio:
    endpoint: 127.0.0.1:${ports.minio}
    access_key: local
    secret_key: ${state.serviceSecret}
    bucket: agent-workspace-local
    secure: false
sandbox:
  runtime: runsc
  egress_network: agent-public-egress
  egress_subnet: 172.30.0.0/24
  egress_controller_socket: ${yamlString(join(stateRoot, 'egress-controller.sock'))}
  resolver_config: ${yamlString(join(stateRoot, 'config', 'sandbox-resolv.conf'))}
  resolver_addresses: [1.1.1.1]
message_channels:
  enabled: false
`;
}

function prepareState() {
  for (const directory of ['bin', 'config', 'config/identity-import', 'logs', 'minio-data', 'run-credentials', 'workspaces']) {
    mkdirSync(join(stateRoot, directory), { recursive: true, mode: 0o700 });
  }
  if (!existsSync(stateFile)) {
    saveJSON(stateFile, {
      version: 1,
      adminPassword: `Local!${randomBytes(16).toString('hex')}`,
      serviceSecret: randomBytes(32).toString('hex'),
      dataEncryptionKey: randomBytes(32).toString('base64'),
    });
  }
  const state = readJSON(stateFile);
  if (state.version !== 1 || !state.adminPassword || !state.serviceSecret || !state.dataEncryptionKey) {
    fail(`Local state is invalid: ${stateFile}`);
  }
  const template = readJSON(join(root, 'deploy', 'platform', 'config', 'keycloak-realm.json'));
  writePrivate(join(identityImportRoot, `${realmName}-realm.json`), `${JSON.stringify(buildRealm(template, state), null, 2)}\n`);
  writePrivate(configFile, buildConfig(state));
  writePrivate(join(stateRoot, 'config', 'known_hosts'), '');
  writePrivate(join(stateRoot, 'config', 'sandbox-resolv.conf'), 'nameserver 1.1.1.1\n');
  writePrivate(join(stateRoot, 'config', 'postgres.env'), `POSTGRES_USER=local\nPOSTGRES_PASSWORD=${state.serviceSecret}\nPOSTGRES_DB=agent_workspace\n`);
  writePrivate(join(stateRoot, 'config', 'keycloak.env'), `KC_BOOTSTRAP_ADMIN_USERNAME=local-admin\nKC_BOOTSTRAP_ADMIN_PASSWORD=${state.serviceSecret}\n`);
  return state;
}

function preflight() {
  for (const command of ['docker', 'go', 'node', 'pnpm', 'python3']) {
    if (!commandExists(command)) fail(`${command} is required for local development`);
  }
  run('docker', ['info', '--format', '{{.ServerVersion}}']);
  const goVersion = run('go', ['-C', 'backend', 'env', 'GOVERSION']);
  if (!/^go1\.(?:2[5-9]|[3-9][0-9])(?:\.|$)/.test(goVersion)) {
    fail(`Go 1.25+ is required; resolved toolchain is ${goVersion}`);
  }
}

function inspectContainer(name, format) {
  const result = spawnSync('docker', ['container', 'inspect', '--format', format, name], { cwd: root, encoding: 'utf8' });
  return result.status === 0 ? result.stdout.trim() : '';
}

function ensureContainer(name, args) {
  const existing = inspectContainer(name, '{{.Name}}');
  if (existing) {
    const owned = inspectContainer(name, '{{ index .Config.Labels "com.agent-workspace.local" }}');
    if (owned !== 'true') fail(`Container ${name} exists but is not owned by the local Agent Workspace`);
    run('docker', ['start', name]);
    return;
  }
  run('docker', ['run', '-d', '--name', name, '--label', ownerLabel, ...args]);
}

function removeOwnedContainer(name) {
  if (!inspectContainer(name, '{{.Name}}')) return;
  const owned = inspectContainer(name, '{{ index .Config.Labels "com.agent-workspace.local" }}');
  if (owned !== 'true') fail(`Container ${name} exists but is not owned by the local Agent Workspace`);
  if (inspectContainer(name, '{{.State.Running}}') === 'true') run('docker', ['stop', name]);
  run('docker', ['rm', name]);
}

async function ready(url, timeout = 120000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    try {
      if ((await fetch(url)).ok) return;
    } catch {}
    await new Promise(resolvePromise => setTimeout(resolvePromise, 500));
  }
  fail(`Service readiness timed out: ${url}. Check ${join(stateRoot, 'logs')}`);
}

function startInfrastructure() {
  removeOwnedContainer(legacyMinioContainer);
  ensureContainer(containers.database, [
    '--env-file', join(stateRoot, 'config', 'postgres.env'),
    '-p', `127.0.0.1:${ports.database}:5432`,
    '-v', 'agent-workspace-local-database:/var/lib/postgresql/data',
    images.postgres,
  ]);
  removeOwnedContainer(containers.identity);
  ensureContainer(containers.identity, [
    '--env-file', join(stateRoot, 'config', 'keycloak.env'),
    '-p', `127.0.0.1:${ports.identity}:8080`,
    '-v', 'agent-workspace-local-identity:/opt/keycloak/data',
    '-v', `${identityImportRoot}:/opt/keycloak/data/import:ro`,
    '-v', `${join(stateRoot, 'themes')}:/opt/keycloak/themes:ro`,
    images.keycloak, 'start-dev', '--import-realm',
  ]);
}

async function waitForInfrastructure() {
  await Promise.all([
    ready(`${identityOrigin}/realms/${realmName}/.well-known/openid-configuration`),
    ready(`http://127.0.0.1:${ports.minio}/minio/health/ready`),
  ]);
  run('docker', ['exec', containers.database, 'pg_isready', '-U', 'local', '-d', 'agent_workspace']);
}

async function ensureBucket(state) {
  const host = `127.0.0.1:${ports.minio}`;
  const date = new Date().toISOString().replace(/[:-]|\.\d{3}/g, '');
  const day = date.slice(0, 8);
  const emptyHash = createHash('sha256').update('').digest('hex');
  const canonical = `PUT\n/agent-workspace-local\n\nhost:${host}\nx-amz-content-sha256:${emptyHash}\nx-amz-date:${date}\n\nhost;x-amz-content-sha256;x-amz-date\n${emptyHash}`;
  const scope = `${day}/us-east-1/s3/aws4_request`;
  const hmac = (key, value) => createHmac('sha256', key).update(value).digest();
  const signingKey = hmac(hmac(hmac(hmac(`AWS4${state.serviceSecret}`, day), 'us-east-1'), 's3'), 'aws4_request');
  const signature = hmac(signingKey, `AWS4-HMAC-SHA256\n${date}\n${scope}\n${createHash('sha256').update(canonical).digest('hex')}`).toString('hex');
  const response = await fetch(`http://${host}/agent-workspace-local`, {
    method: 'PUT',
    headers: {
      'x-amz-date': date,
      'x-amz-content-sha256': emptyHash,
      Authorization: `AWS4-HMAC-SHA256 Credential=local/${scope}, SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature=${signature}`,
    },
  });
  if (!response.ok && response.status !== 409) fail(`Create private local bucket failed with status ${response.status}`);
}

function live(pid) {
  if (!Number.isInteger(pid) || pid <= 1) return false;
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
}

function commandFor(pid) {
  const result = spawnSync('ps', ['-p', String(pid), '-o', 'command='], { encoding: 'utf8' });
  return result.status === 0 ? result.stdout.trim() : '';
}

function ownedProcess(name, pid) {
  const command = commandFor(pid);
  if (!command) return false;
  if (name === 'web') return command.includes('pnpm') && command.includes('exec vite') && command.includes(`--port ${ports.web}`);
  return command.includes(join(stateRoot, 'bin', name));
}

function stopProcesses() {
  if (!existsSync(processFile)) return;
  const processes = readJSON(processFile);
  for (const [name, pid] of Object.entries(processes)) {
    if (!live(pid) || !ownedProcess(name, pid)) continue;
    try { process.kill(-pid, 'SIGTERM'); } catch {}
  }
  const deadline = Date.now() + 5000;
  while (Date.now() < deadline && Object.entries(processes).some(([name, pid]) => live(pid) && ownedProcess(name, pid))) {
    Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, 100);
  }
  for (const [name, pid] of Object.entries(processes)) {
    if (live(pid) && ownedProcess(name, pid)) {
      try { process.kill(-pid, 'SIGKILL'); } catch {}
    }
  }
  saveJSON(processFile, {});
}

function spawnService(name, command, args, env = {}) {
  const log = openSync(join(stateRoot, 'logs', `${name}.log`), 'a', 0o600);
  const child = spawn(command, args, {
    cwd: root,
    env: { ...process.env, ...env },
    detached: true,
    stdio: ['ignore', log, log],
  });
  closeSync(log);
  child.unref();
  return child.pid;
}

function buildApplications() {
  rmSync(join(stateRoot, 'themes'), { recursive: true, force: true });
  run('python3', ['scripts/build-identity-theme.py', join(stateRoot, 'themes')]);
  if (!existsSync(join(root, 'frontend', 'node_modules'))) {
    run('pnpm', ['--dir', 'frontend', 'install', '--frozen-lockfile'], { stdio: 'inherit' });
  }
  const minioBinary = join(stateRoot, 'bin', 'minio');
  const minioMarker = join(stateRoot, 'bin', 'minio.version');
  const installedMinio = existsSync(minioMarker) ? readFileSync(minioMarker, 'utf8').trim() : '';
  if (!existsSync(minioBinary) || installedMinio !== minioRelease) {
    run('go', ['install', `github.com/minio/minio@${minioRelease}`], {
      env: { ...process.env, GOBIN: join(stateRoot, 'bin'), GOTOOLCHAIN: 'auto' },
      stdio: 'inherit',
    });
    writePrivate(minioMarker, `${minioRelease}\n`);
  }
  run('go', ['-C', 'backend', 'build', '-o', join(stateRoot, 'bin', 'api'), './cmd/api'], { stdio: 'inherit' });
  run('go', ['-C', 'backend', 'build', '-o', join(stateRoot, 'bin', 'worker'), './cmd/worker'], { stdio: 'inherit' });
}

async function start() {
  preflight();
  const state = prepareState();
  stopProcesses();
  buildApplications();
  startInfrastructure();
  const processes = {};
  processes.minio = spawnService('minio', join(stateRoot, 'bin', 'minio'), [
    'server', join(stateRoot, 'minio-data'),
    '--address', `127.0.0.1:${ports.minio}`,
    '--console-address', `127.0.0.1:${ports.minio + 1}`,
  ], { MINIO_ROOT_USER: 'local', MINIO_ROOT_PASSWORD: state.serviceSecret });
  saveJSON(processFile, processes);
  await waitForInfrastructure();
  await ensureBucket(state);
  const sharedEnvironment = { AGENT_WORKSPACE_RESOURCE_ROOT: root };
  processes.api = spawnService('api', join(stateRoot, 'bin', 'api'), ['-config', configFile], sharedEnvironment);
  saveJSON(processFile, processes);
  await ready(`http://127.0.0.1:${ports.api}/readyz`, 60000);
  processes.worker = spawnService('worker', join(stateRoot, 'bin', 'worker'), ['-config', configFile], sharedEnvironment);
  saveJSON(processFile, processes);
  await ready(`http://127.0.0.1:${ports.worker}/readyz`, 60000);
  processes.web = spawnService('web', 'pnpm', ['--dir', 'frontend', 'exec', 'vite', '--host', '127.0.0.1', '--port', String(ports.web), '--strictPort'], {
    VITE_OIDC_AUTHORITY: `${identityOrigin}/realms/${realmName}`,
    VITE_OIDC_CLIENT_ID: 'agent-platform-web',
    VITE_OIDC_REDIRECT_URI: `${webOrigin}/auth/callback`,
    VITE_OIDC_POST_LOGOUT_REDIRECT_URI: webOrigin,
    VITE_API_PROXY_TARGET: `http://127.0.0.1:${ports.api}`,
  });
  saveJSON(processFile, processes);
  await ready(webOrigin, 60000);
  console.log(`Agent Workspace is running at ${webOrigin}`);
  console.log(`Run "make local-credentials" to show the local login. State: ${stateRoot}`);
  console.log('Runtime Engines remain unavailable because this Docker daemon has no runsc runtime.');
}

function stop() {
  stopProcesses();
  for (const name of Object.values(containers)) {
    if (inspectContainer(name, '{{.Name}}')) run('docker', ['stop', name]);
  }
  console.log('Local Agent Workspace stopped. Persistent Docker volumes and local state were kept.');
}

function status() {
  const processState = existsSync(processFile) ? readJSON(processFile) : {};
  for (const name of ['minio', 'api', 'worker', 'web']) {
    const pid = processState[name];
    console.log(`${name}: ${live(pid) && ownedProcess(name, pid) ? `running (pid ${pid})` : 'stopped'}`);
  }
  for (const [name, container] of Object.entries(containers)) {
    const running = inspectContainer(container, '{{.State.Running}}') === 'true';
    console.log(`${name}: ${running ? 'running' : 'stopped'}`);
  }
  console.log(`URL: ${webOrigin}`);
  console.log(`State: ${stateRoot}`);
}

function credentials() {
  if (!existsSync(stateFile)) fail('Local state does not exist; run make local-up first');
  const state = readJSON(stateFile);
  console.log(`URL: ${webOrigin}`);
  console.log('Username: platform-admin');
  console.log(`Password: ${state.adminPassword}`);
}

function usage() {
  console.log('Usage: node scripts/local-platform.mjs <start|stop|status|credentials>');
}

async function main() {
  switch (process.argv[2] || 'start') {
    case 'start': await start(); break;
    case 'stop': stop(); break;
    case 'status': status(); break;
    case 'credentials': credentials(); break;
    case '--help': case '-h': case 'help': usage(); break;
    default: usage(); process.exitCode = 2;
  }
}

if (process.argv[1] && pathToFileURL(resolve(process.argv[1])).href === import.meta.url) {
  main().catch(error => {
    console.error(error.message);
    process.exitCode = 1;
  });
}
