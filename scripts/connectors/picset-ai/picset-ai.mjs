#!/usr/bin/env node
import { randomUUID } from 'node:crypto';
import { readFileSync, realpathSync } from 'node:fs';
import { pathToFileURL } from 'node:url';

export const VERSION = '0.1.0';
export const BASE = 'https://picsetai.cn/functions/v1/agent-api-v1/';
export const OPERATIONS = Object.freeze(JSON.parse(readFileSync(new URL('./operations.json', import.meta.url), 'utf8')));
const SCENES = ['product_main', 'product_detail', 'ad_image', 'style_replicate_reference', 'style_replicate_product', 'sku_replace_reference', 'sku_replace_product', 'image_refinement', 'image_translation', 'canvas_image', 'image_layer'];
const UUID = /^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/i;
const KEY = /^[A-Za-z0-9._:-]{8,128}$/;
const MAX_INPUT = 65536;
export class CLIError extends Error {
  constructor(type, message, nextAction = 'check_command', retryable = false) {
    super(message); Object.assign(this, { type, nextAction, retryable });
  }
}
const invalid = message => new CLIError('invalid_request', message);
const object = value => value !== null && typeof value === 'object' && !Array.isArray(value);
export function credential(env) {
  let key = env.PICSET_API_KEY;
  if (env.CONNECTOR_CREDENTIALS_JSON !== undefined) {
    let value;
    try { value = JSON.parse(env.CONNECTOR_CREDENTIALS_JSON); } catch { throw invalid('Invalid platform credential JSON.'); }
    if (!object(value) || Object.keys(value).length !== 1 || !Object.hasOwn(value, 'picset_api_key')) throw invalid('Credentials require only picset_api_key.');
    key = value.picset_api_key;
  }
  if (key === undefined || key === '') return undefined;
  if (typeof key !== 'string' || !/^sk_live_[A-Za-z0-9_-]+$/.test(key) || key.length > 4096) throw invalid('Invalid Picset Agent Secret Key.');
  return key;
}
export function parseArguments(raw) {
  if (typeof raw !== 'string' || Buffer.byteLength(raw) > MAX_INPUT) throw invalid('JSON input exceeds 64 KiB.');
  let value;
  try { value = JSON.parse(raw); } catch { throw invalid('Expected valid JSON.'); }
  if (!object(value)) throw invalid('Expected a JSON object.');
  return value;
}
export function validateArguments(operation, args) {
  const spec = OPERATIONS[operation];
  if (!spec || !object(args) || Object.keys(args).some(k => !spec.fields.includes(k)) || spec.required.some(k => !Object.hasOwn(args, k))) throw invalid('Unknown or missing API fields. Use schema <operation>.');
  for (const [field, scene] of Object.entries(spec.image_scenes)) {
    if (args[field] === undefined) continue;
    const many = field.endsWith('images');
    const images = many ? args[field] : [args[field]];
    if (!Array.isArray(images) || images.length < 1 || images.length > (field === 'reference_images' ? 9 : many ? 6 : 1)) throw invalid('Invalid image count.');
    for (const image of images) {
      if (!object(image) || Object.keys(image).length !== 1 || !validPath(image.oss_path, scene)) throw invalid('Images require only an original oss_path in the matching upload scene.');
    }
  }
  for (const field of ['requirements', 'prompt', 'product_description', 'text_changes']) {
    if (args[field] === undefined) continue;
    const limit = operation.startsWith('image-layer') ? 1000 : 4000;
    if (typeof args[field] !== 'string' || [...args[field].trim()].length > limit || spec.required.includes(field) && !args[field].trim()) throw invalid('Invalid text field length.');
  }
  if (operation === 'sku-replace' && ['requirements', 'product_description', 'text_changes'].reduce((n, k) => n + [...(args[k] || '')].length, 0) > 12000) throw invalid('Combined text exceeds 12000 characters.');
  if (args.scene !== undefined && !SCENES.includes(args.scene)) throw invalid('Unknown upload scene.');
  if (operation === 'image-audit' && !validPath(args.oss_path, args.scene)) throw invalid('Invalid scene or oss_path.');
  if (operation === 'request' && !UUID.test(args.request_id)) throw invalid('request_id must be a UUID.');
  if (args.white_background !== undefined && typeof args.white_background !== 'boolean') throw invalid('white_background must be boolean.');
  if (args.output_count !== undefined && (!Number.isInteger(args.output_count) || !(args.output_count >= 1 && args.output_count <= 16 || operation === 'ad-image' && [20,25,30,35,40,45,50].includes(args.output_count)))) throw invalid('Invalid output_count.');
  if (args.rectangles !== undefined && (!Array.isArray(args.rectangles) || args.rectangles.length < 1 || args.rectangles.length > 20 || args.rectangles.some(r => !Array.isArray(r) || r.length !== 4 || r.some(v => !Number.isInteger(v) || v < 0 || v > 999) || r[0] >= r[2] || r[1] >= r[3]))) throw invalid('Invalid normalized rectangles.');
  for (const field of ['model','aspect_ratio','resolution','speed_mode','gpt_quality','target_language','target_platform','ad_type']) if (args[field] !== undefined && typeof args[field] !== 'string') throw invalid('Expected string generation parameters.');
  if (args.model === 'nova-img-2' && args.resolution !== undefined || operation === 'canvas-image' && args.model === 'nova-img-2-vip' && args.gpt_quality !== undefined || args.gpt_quality === 'auto') throw invalid('Incompatible model parameters.');
  return args;
}
function validPath(value, scene) {
  return typeof value === 'string' && value.length <= 1024 && !/[\s\\\x00-\x1f\x7f?#]/.test(value) && !value.includes('..') && /^(?:[A-Za-z0-9_-]+\/)*temp\/[^/]+\/agent-uploads\//.test(value) && value.includes(`/agent-uploads/${scene}/`) && value.split('/').at(-1).length > 0;
}
export function parseCommand(argv) {
  if (argv.length === 0 || argv.length === 1 && ['help','--help'].includes(argv[0])) return { kind: 'help' };
  if (argv.length === 1 && ['version','--version','init','auth','status','unauth','operations'].includes(argv[0])) return { kind: argv[0] === '--version' ? 'version' : argv[0] };
  if (argv.length === 2 && argv[0] === 'schema' && OPERATIONS[argv[1]]) return { kind:'schema', operation:argv[1] };
  if (!OPERATIONS[argv[0]]) throw invalid('Unreviewed command. Use picset-ai --help.');
  if (argv.length === 2 && argv[1] === '--help') return { kind:'schema', operation:argv[0] };
  let args, stdin = false, key;
  const tail = argv.slice(1);
  if (tail[0] === '--stdin') { stdin = true; tail.shift(); }
  else if (tail[0] === '--json' && tail.length >= 2) { args = parseArguments(tail[1]); tail.splice(0,2); }
  else throw invalid('Use --json <object> or --stdin.');
  if (tail.length === 2 && tail[0] === '--idempotency-key' && KEY.test(tail[1]) && OPERATIONS[argv[0]].paid) key = tail[1];
  else if (tail.length) throw invalid('Invalid arguments or idempotency key.');
  if (OPERATIONS[argv[0]].paid && !key) throw invalid('Paid operations require a saved --idempotency-key (8-128 safe characters).');
  return { kind:'call', operation:argv[0], args, stdin, key };
}
async function responseJSON(response) {
  if (!response.headers.get('content-type')?.toLowerCase().includes('application/json') || !response.body) throw new CLIError('protocol_error', 'Expected JSON from Picset.', 'check_connection');
  const reader = response.body.getReader(); let size = 0; const chunks = [];
  try {
    while (true) { const {value,done} = await reader.read(); if (done) break; size += value.byteLength; if (size > 8*1024*1024) throw new CLIError('output_limit','Response exceeds 8 MiB.'); chunks.push(Buffer.from(value)); }
    let value; try { value = JSON.parse(Buffer.concat(chunks).toString('utf8')); } catch { throw new CLIError('protocol_error','Invalid upstream JSON.','check_connection'); }
    if (!object(value)) throw new CLIError('protocol_error','Expected an upstream object.','check_connection');
    return value;
  } finally { await reader.cancel().catch(() => {}); }
}
export async function api(operation, args, key, {fetchImpl = fetch, timeoutMs = 55000, idempotencyKey} = {}) {
  const spec = OPERATIONS[operation];
  const path = operation === 'request' ? `requests/${args.request_id}` : operation;
  const headers = {Authorization:`Bearer ${key}`, Accept:'application/json'};
  if (spec.method === 'POST') headers['Content-Type'] = 'application/json';
  if (idempotencyKey) headers['Idempotency-Key'] = idempotencyKey;
  let response;
  try { response = await fetchImpl(BASE + path, {method:spec.method, headers, redirect:'error', signal:AbortSignal.timeout(timeoutMs), ...(spec.method === 'POST' ? {body:JSON.stringify(args)} : {})}); }
  catch { throw new CLIError('transport_error', 'Connection interrupted; retry paid requests only with the same key and body.', 'retry_same_request', true); }
  let value;
  try { value = await responseJSON(response); } catch (cause) { if (cause instanceof CLIError) throw cause; throw new CLIError('transport_error','Response interrupted; preserve the same key and body.','retry_same_request',true); }
  if (!response.ok) {
    const code = typeof value.error === 'string' && /^[A-Z0-9_]{1,80}$/.test(value.error) ? value.error : 'UPSTREAM_ERROR';
    const action = response.status === 401 ? 'connect' : response.status === 409 ? 'restore_original_request' : [429,503].includes(response.status) ? 'retry_same_request' : 'check_request';
    throw new CLIError(code, `Picset returned HTTP ${response.status}: ${code}.`, action, [429,503].includes(response.status));
  }
  if (spec.paid && (response.status !== 202 || !UUID.test(value.request_id) || value.idempotency_key !== idempotencyKey)) throw new CLIError('protocol_error','Invalid task acceptance; preserve your key and body.','retry_same_request');
  if (operation === 'request' && (value.request_id !== args.request_id || !['queued','analyzing','generating','completed','failed'].includes(value.status))) throw new CLIError('protocol_error','Invalid task status.','check_request');
  return value;
}
export async function execute(argv, options = {}) {
  const command = parseCommand(argv);
  if (command.kind === 'help') return {name:'picset-ai',version:VERSION,usage:'picset-ai <operation> --json <object> [--idempotency-key <saved-key>]', operations:OPERATIONS, upload:'PUT to OSS is external in this revision; no arbitrary URL/file upload.', approval:'Use agent-cli and platform one-use approval for high-risk operations.'};
  if (['version','init'].includes(command.kind)) return {name:'agent-workspace-picset-ai-cli',version:VERSION};
  if (command.kind === 'operations') return OPERATIONS;
  if (command.kind === 'schema') return OPERATIONS[command.operation];
  if (command.kind === 'unauth') return {authenticated:false, upstream_revoked:false, next_action:'disconnect_platform_authorization'};
  const key = credential(options.env || process.env);
  if (['status','auth'].includes(command.kind)) return {authenticated:Boolean(key), upstream_verified:false, next_action:key ? 'query_owned_request' : 'connect'};
  if (!key) throw new CLIError('authorization_required','Connect a Picset Agent Secret Key.','connect');
  if (command.stdin) {
    const chunks=[]; let size=0;
    for await (const chunk of options.stdin || process.stdin) { size += Buffer.byteLength(chunk); if (size > MAX_INPUT) throw invalid('JSON input exceeds 64 KiB.'); chunks.push(Buffer.from(chunk)); }
    command.args = parseArguments(Buffer.concat(chunks).toString('utf8'));
  }
  return api(command.operation,validateArguments(command.operation,command.args),key,{...options,idempotencyKey:command.key});
}
export async function run(argv, options = {}) {
  let output;
  const requestID = randomUUID();
  try { output = {ok:true,data:await execute(argv,options),request_id:requestID,warnings:[]}; }
  catch (cause) { const err = cause instanceof CLIError ? cause : new CLIError('internal_error','CLI failed.'); output={ok:false,error:{type:err.type,message:err.message,retryable:err.retryable,next_action:err.nextAction},request_id:requestID}; }
  const env = options.env || process.env;
  let text = JSON.stringify(output);
  const secrets = [env.PICSET_API_KEY,env.CONNECTOR_CREDENTIALS_JSON];
  try { secrets.push(credential(env)); } catch { /* No raw credential errors. */ }
  for (const secret of secrets.filter(v => typeof v === 'string' && v)) text = text.split(JSON.stringify(secret).slice(1,-1)).join('[REDACTED]');
  return {text,exitCode:output.ok ? 0 : 1};
}
if (process.argv[1] && import.meta.url === pathToFileURL(realpathSync(process.argv[1])).href) {
  const result = await run(process.argv.slice(2)); process.stdout.write(result.text+'\n'); process.exitCode=result.exitCode;
}
