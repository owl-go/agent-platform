#!/usr/bin/env node
'use strict';

const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const { spawnSync } = require('node:child_process');
const policy = require('./capabilities.json');
const blocked = new Set(['--config', '--host', '--token', '--resource', '--x-canary', '--debug', '-d', '--']);

function credential(environment) {
  let value;
  try { value = JSON.parse(environment.CONNECTOR_CREDENTIALS_JSON || '{}'); }
  catch { throw new Error('Connector credentials must be valid JSON'); }
  if (!value || typeof value !== 'object' || Array.isArray(value) ||
      Object.keys(value).some(key => key !== 'user_token') ||
      typeof value.user_token !== 'string' || !value.user_token.trim() ||
      value.user_token.length > 32768 || /[\s\x00-\x1f\x7f]/.test(value.user_token)) {
    throw new Error('Configure this Installation with a non-empty user_token');
  }
  return value.user_token;
}

function validateArguments(args) {
  const matched = policy.find(item => item.argv_prefix.every((arg, index) => args[index] === arg));
  if (!matched || args.some(arg => /[\x00\r\n]/.test(arg) || blocked.has(arg.split('=')[0]) || /^-[^-]*d/.test(arg))) {
    throw new Error('Command or identity/endpoint override is outside this Connector revision');
  }
  const prefix = matched.argv_prefix;
  const tail = args.slice(prefix.length);
  if ((prefix.length === 1 || prefix.at(-1) === '--help') && tail.length !== 0) {
    throw new Error('Diagnostic command must have no trailing arguments');
  }
  if (prefix[0] === 'tools' && prefix[1] === 'call') {
    for (let i = 0; i < tail.length; i++) {
      if (['--json', '--help', '-h'].includes(tail[i])) continue;
      if (tail[i] === '--arguments-json' || tail[i] === '--arguments-file') {
        if (++i >= tail.length) throw new Error('Missing document tool arguments');
        continue;
      }
      if (tail[i].startsWith('--arguments-json=') || tail[i].startsWith('--arguments-file=')) continue;
      throw new Error('Only the pinned read-only document tool is allowed');
    }
  }
  return matched;
}

function main(args, environment = process.env) {
  if (args.length === 1 && args[0] === '--help') {
    console.log('Agent Workspace Teambition CLI 0.3.3\nReviewed commands (business help requires authorization):');
    for (const item of policy) console.log(`  ${item.argv_prefix.join(' ')} [${item.risk}]`);
    return 0;
  }
  if (args[0] === 'platform') {
    if (args.length !== 2 || !['init', 'auth', 'status', 'unauth'].includes(args[1])) throw new Error('Unknown lifecycle command');
    if (args[1] === 'status') {
      let configured = false;
      try { credential(environment); configured = true; } catch {}
      console.log(JSON.stringify({ configured, verification: 'credential_shape_only' }));
    } else {
      console.log(JSON.stringify({ ok: true, authorization_mode: 'provided', next_action: 'Manage encrypted user_token through Connector Installation' }));
    }
    return 0;
  }
  validateArguments(args);
  const arch = { x64: 'x64', arm64: 'arm64' }[process.arch];
  if (process.platform !== 'linux' || !arch) throw new Error('This bundle requires Linux x64 or arm64');
  // An ephemeral HOME keeps CLI discovery caches and files out of the Workspace.
  const temporary = fs.mkdtempSync(path.join(os.tmpdir(), 'teambition-command-'));
  try {
    const childEnv = { PATH: environment.PATH, HOME: temporary, XDG_CONFIG_HOME: path.join(temporary, '.config'),
      TEAMBITION_MCP_HOST: 'https://open.teambition.com/api' };
    if (args[0] !== '--version') childEnv.TEAMBITION_MCP_TOKEN = credential(environment);
    const result = spawnSync(path.join(__dirname, `teambition-linux-${arch}`), args,
      { env: childEnv, stdio: 'inherit', timeout: 120000 });
    if (result.error) throw new Error('Teambition CLI process failed or timed out');
    return result.status ?? 1;
  } finally {
    fs.rmSync(temporary, { recursive: true, force: true });
  }
}

module.exports = { credential, validateArguments, main };
if (require.main === module) {
  try { process.exitCode = main(process.argv.slice(2)); }
  catch (error) { console.error(error.message); process.exitCode = 1; }
}
