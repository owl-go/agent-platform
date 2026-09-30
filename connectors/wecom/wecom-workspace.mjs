#!/usr/bin/env node
import { spawnSync } from 'node:child_process';
import { createHash, randomBytes } from 'node:crypto';
import { mkdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { redact } from './redact.mjs';
import { invocationRejectionReason, validateInvocation } from './policy.mjs';

const args = process.argv.slice(2);
const directory = dirname(fileURLToPath(import.meta.url));
const platformPackage = process.platform === 'linux' && process.arch === 'arm64' ? 'cli-linux-arm64' : process.platform === 'linux' && process.arch === 'x64' ? 'cli-linux-x64' : '';
if (!platformPackage) {
  process.stderr.write('This bundle supports only its pinned Linux architecture.\n');
  process.exit(1);
}
const binary = join(directory, '..', '@wecom', platformPackage, 'bin', 'wecom-cli');

if (args.length === 1 && args[0] === '--help') {
  const check = spawnSync(binary, ['--version'], { encoding: 'utf8', timeout: 10000 });
  if (check.status !== 0 || !check.stdout?.includes('1.3.4')) process.exit(1);
  process.stdout.write('wecom-workspace 1.4.2 (@wecom/cli 1.3.4): reviewed commands are declared in cli.json\n');
  process.exit(0);
}
if (args.length === 1 && args[0] === '--version') {
  process.stdout.write('wecom-workspace 1.4.2 (@wecom/cli 1.3.4)\n');
  process.exit(0);
}

let botID = '';
let secret = '';
try {
  const credentials = JSON.parse(process.env.CONNECTOR_CREDENTIALS_JSON ?? '{}');
  if (credentials && typeof credentials === 'object' && !Array.isArray(credentials) && typeof credentials.bot_id === 'string' && typeof credentials.secret === 'string') {
    botID = credentials.bot_id.trim();
    secret = credentials.secret.trim();
  }
} catch {
  // The platform supplies bounded JSON; fail closed if it is absent or malformed.
}

if (args[0] === 'platform' && args[1] === 'status' && args.length === 2) {
  process.stdout.write(JSON.stringify({ credentials_present: botID.length > 0 && secret.length > 0 }) + '\n');
  process.exit(0);
}
if (args[0] === 'platform' && (args[1] === 'authorize' || args[1] === 'revoke') && args.length === 2) {
  process.stderr.write('Manage this authorization through the Connector Installation.\n');
  process.exit(1);
}

const invocation = validateInvocation(args);
if (!invocation) {
  process.stderr.write(invocationRejectionReason(args) + '\n');
  process.exit(1);
}
if (!botID || !secret) {
  process.stderr.write('An active Connector Authorization is required.\n');
  process.exit(1);
}
const prefix = invocation.policy.command;
const workspace = '/workspace';

const time = Math.floor(Date.now() / 1000);
const nonce = `cli_${Date.now()}_${randomBytes(4).toString('hex')}`;
const signature = createHash('sha256').update(`${secret}${botID}${time}${nonce}`).digest('hex');
let token = '';
try {
  const response = await fetch('https://qyapi.weixin.qq.com/cgi-bin/aibot/cli/get_cli_config', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ bot_id: botID, time, nonce, signature, bind_source: 1 }),
    signal: AbortSignal.timeout(15000),
  });
  if (!response.ok) throw new Error('HTTP error');
  const body = await response.json();
  if (body.errcode != null && body.errcode !== 0 || typeof body.token !== 'string' || !body.token) throw new Error('authorization denied');
  token = body.token;
} catch {
  process.stderr.write('WeCom authorization failed; check the Bot credentials and permissions.\n');
  process.exit(1);
}
const env = {
  PATH: process.env.PATH ?? '/usr/bin:/bin',
  HOME: '/tmp',
  TMPDIR: '/tmp',
  LANG: 'C.UTF-8',
  WECOM_CLI_CONFIG_DIR: '/tmp/wecom-connector-config',
  WECOM_CLI_ACCESS_TOKEN: token,
};
let commandArgs = args;
if (['disk files download', 'media download'].includes(prefix)) {
  const output = join(workspace, '.wecom-downloads', randomBytes(8).toString('hex'));
  mkdirSync(output, { recursive: true, mode: 0o700 });
  commandArgs = [...args, '--output-dir', output];
}
const result = spawnSync(binary, commandArgs, { encoding: 'utf8', env, maxBuffer: 8 * 1024 * 1024, timeout: 180000 });
if (result.error) {
  process.stderr.write('Unable to start the pinned WeCom CLI.\n');
  process.exit(1);
}
process.stdout.write(redact(result.stdout, [secret, token, botID]));
process.stderr.write(redact(result.stderr, [secret, token, botID]));
process.exit(result.status ?? 1);
