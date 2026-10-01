#!/usr/bin/env node
'use strict';
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { randomBytes, createCipheriv } = require('node:crypto');
const { spawn } = require('node:child_process');
const policy = require('./capabilities.json');

function credential(env) {
  let value;
  try { value = JSON.parse(env.CONNECTOR_CREDENTIALS_JSON || '{}'); } catch { throw new Error('Invalid Connector credentials'); }
  const allowed = ['access_token', 'access_expires_at', 'user_id', 'is_domestic'];
  if (!value || Array.isArray(value) || Object.keys(value).some(k => !allowed.includes(k)) ||
      typeof value.access_token !== 'string' || !/^[A-Za-z0-9._~+/-]+={0,}$/.test(value.access_token) || value.access_token.length > 32768 ||
      typeof value.is_domestic !== 'string' || value.is_domestic.length > 8 ||
      typeof value.user_id !== 'string' || !value.user_id || value.user_id.length > 512 || /[\x00-\x1f\x7f]/.test(value.user_id) ||
      !Number.isFinite(Date.parse(value.access_expires_at))) throw new Error('Connect this Installation through browser authorization');
  if (Date.parse(value.access_expires_at) <= Date.now() + 180000) throw new Error('AUTH_EXPIRED: refresh this Installation authorization');
  return { is_domestic:value.is_domestic, token: value.access_token, user_id: value.user_id, expires_at: Math.floor(Date.parse(value.access_expires_at) / 1000) };
}
function validateArguments(args) {
  if (args.some(a => /[\x00\r\n]/.test(a))) throw new Error('Unsafe command argument');
  const match = policy.find(p => p.argv_prefix.every((a,i) => args[i] === a));
  if (!match) throw new Error('Command is outside this Connector revision');
  if (['--version', '--help', 'info'].includes(args[0]) && args.length !== 1) throw new Error('Diagnostic accepts no trailing arguments');
  if (args[0] === 'help' && !(args.length === 3 && policy.some(p => p.argv_prefix.length === 2 && p.argv_prefix.every((a,i)=>args[i+1]===a)))) throw new Error('Help requires a reviewed leaf command');
  return match;
}
function restoreManagedKeychain(home, grant) {
  // The native CLI renews within 24 h. The platform retains the actual expiry;
  // this command-only cache defers native renewal beyond its bounded lifetime.
  restoreKeychain(home, {...grant, expires_at: grant.expires_at + 86400});
}
function restoreKeychain(home, grant) {
  const directory = path.join(home, 'data', 'camscanner-cli');
  fs.mkdirSync(directory, { recursive: true, mode: 0o700 });
  const key = randomBytes(32), nonce = randomBytes(12);
  const cipher = createCipheriv('aes-256-gcm', key, nonce);
  const plaintext = Buffer.from(JSON.stringify(grant));
  const encrypted = Buffer.concat([nonce, cipher.update(plaintext), cipher.final(), cipher.getAuthTag()]);
  plaintext.fill(0);
  fs.writeFileSync(path.join(directory, 'master.key'), key, { mode: 0o600 });
  key.fill(0);
  fs.writeFileSync(path.join(directory, Buffer.from('camscanner-cli:default').toString('hex') + '.enc'), encrypted, { mode: 0o600 });
}
async function main(args, environment=process.env) {
  if (args[0] === 'platform') {
    if (args.length !== 2 || !['init','auth','status','unauth'].includes(args[1])) throw new Error('Invalid lifecycle command');
    let configured = false;
    try { credential(environment); configured=true; } catch {}
    console.log(JSON.stringify(args[1] === 'status' ? {configured,verification:'credential_shape_only'} : {ok:true,next_action:'Use the platform browser authorization flow'}));
    return 0;
  }
  validateArguments(args);
  if (process.platform !== 'linux' || !['x64','arm64'].includes(process.arch)) throw new Error('Requires Linux x64 or arm64');
  const home = fs.mkdtempSync(path.join(os.tmpdir(), 'camscanner-command-'));
  try {
    const diagnostic = ['--help','--version','info','help'].includes(args[0]);
    if (!diagnostic) restoreManagedKeychain(home, credential(environment));
    const env = {PATH: environment.PATH, HOME:home, XDG_DATA_HOME:path.join(home,'data'), XDG_CONFIG_HOME:path.join(home,'config')};
    return await new Promise((resolve,reject) => {
      const child = spawn(path.join(__dirname, `camscanner-linux-${process.arch}`), args, {env,stdio:'inherit'});
      const stop = () => child.kill('SIGTERM');
      process.once('SIGTERM',stop);process.once('SIGINT',stop);
      const timer = setTimeout(()=>child.kill('SIGKILL'),120000);
      const cleanup=()=>{clearTimeout(timer);process.removeListener('SIGTERM',stop);process.removeListener('SIGINT',stop);};
      child.once('error',()=>{cleanup();reject(new Error('CamScanner CLI process failed'));});
      child.once('close',code=>{cleanup();resolve(code??1);});
    });
  } finally { fs.rmSync(home,{recursive:true,force:true}); }
}
module.exports={credential,validateArguments,restoreKeychain,restoreManagedKeychain,main};
if(require.main===module)main(process.argv.slice(2)).then(code=>{process.exitCode=code;}).catch(e=>{console.error(e.message);process.exitCode=1;});
