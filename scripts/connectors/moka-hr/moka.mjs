#!/usr/bin/env node
import { randomUUID } from 'node:crypto';
import { realpathSync } from 'node:fs';
import { pathToFileURL } from 'node:url';

export const VERSION = '0.1.0';
export const HOST = 'https://api.mokahr.com';
const MAX_INPUT = 64 * 1024, MAX_OUTPUT = 8 * 1024 * 1024;
// Types, routes and query names are reviewed against the official ATS API.
export const OPERATIONS = {
  'jobs list': { path: '/v1/jobs/{orgId}', fields: { mode: 'mode!', keyword: 'text', limit: 'limit', offset: 'offset' } },
  'jobs get': { path: '/v1/jobs/{orgId}/{jobId}', fields: { jobId: 'segment!' } },
  'jobs fields': { path: '/v1/jobs/{orgId}/{jobId}', fields: { jobId: 'segment!' }, select: 'customFields' },
  'candidates search': { path: '/v1/data/ehrApplications', fields: { stage: 'stage!', email: 'text', phone: 'text', movedAtStartTime: 'date', movedAtEndTime: 'date', order: 'order', limit: 'pageLimit', next: 'text' } },
  'candidates get': { path: '/v1/data/ehrApplications', fields: { applicationId: 'ids!' } },
  'candidates stage': { path: '/v1/data/ehrApplications', fields: { applicationId: 'ids!' }, select: 'stage' },
  'applications list': { path: '/candidate/v1/getApplicationStates', method: 'POST', fields: { candidateId: 'id!' } },
  'pipelines list': { path: '/v2/pipelines/getPipelinesList', fields: {} },
  'stages list': { path: '/v2/stage/getStagesList', fields: {} },
  'departments list': { path: '/v1/departments', fields: {} },
  'offers fields': { path: '/v1/offers/custom_fields', fields: {} },
  'pools list': { path: '/v1/talentPool/list', fields: {} },
  'pools candidates': { path: '/v1/talentPool/candidates', fields: { talentPoolIds: 'idArray!', archivedAtStart: 'date!', archivedAtEnd: 'date!' } },
  'interviews list': { path: '/v1/interviews', fields: { startDate: 'date', endDate: 'date', createStartDate: 'date', createEndDate: 'date', hireMode: 'hireMode' } },
};

export class CLIError extends Error {
  constructor(type, message, nextAction = 'check_command', retryable = false) {
    super(message);
    Object.assign(this, { type, nextAction, retryable });
  }
}
const invalid = message => new CLIError('invalid_request', message);
const protocol = () => new CLIError('protocol_error', 'Moka returned an unexpected response.', 'check_connection');

function segment(value) {
  return typeof value === 'string' && /^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$/.test(value);
}
export function credentials(env) {
  let value;
  if (env.CONNECTOR_CREDENTIALS_JSON !== undefined) {
    try { value = JSON.parse(env.CONNECTOR_CREDENTIALS_JSON); } catch { throw invalid('Invalid platform credentials JSON.'); }
    if (!value || typeof value !== 'object' || Array.isArray(value) || Object.keys(value).some(key => !['moka_api_key', 'moka_org_id'].includes(key))) {
      throw invalid('Platform credentials accept only moka_api_key and moka_org_id.');
    }
  } else value = { moka_api_key: env.MOKA_API_KEY, moka_org_id: env.MOKA_ORG_ID };
  const key = value.moka_api_key, orgId = value.moka_org_id;
  if (key !== undefined && key !== '' && (typeof key !== 'string' || key.length > 8192 || /[\s:\x00-\x1f\x7f]/u.test(key))) {
    throw invalid('Invalid Moka API Key.');
  }
  if (orgId !== undefined && orgId !== '' && !segment(orgId)) throw invalid('Invalid Moka organization ID.');
  return { key: key || undefined, orgId: orgId || undefined };
}

export function parseArguments(raw) {
  if (typeof raw !== 'string' || Buffer.byteLength(raw) > MAX_INPUT) throw invalid('Input exceeds 64 KiB.');
  let value;
  try { value = JSON.parse(raw); } catch { throw invalid('Input must be valid JSON.'); }
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw invalid('Input must be a JSON object.');
  return value;
}

export function parseCommand(argv) {
  if (!argv.length || argv.length === 1 && ['help', '--help'].includes(argv[0])) return { kind: 'help' };
  if (argv.length === 1 && ['version', '--version', 'init', 'auth', 'status', 'unauth', 'tools'].includes(argv[0])) {
    return { kind: argv[0] === '--version' ? 'version' : argv[0] };
  }
  if (argv[0] === 'schema' && argv.length === 3 && Object.hasOwn(OPERATIONS, argv.slice(1).join(' '))) {
    return { kind: 'schema', operation: argv.slice(1).join(' ') };
  }
  const operation = argv.slice(0, 2).join(' ');
  if (!Object.hasOwn(OPERATIONS, operation)) throw invalid('Unreviewed command. Run moka --help.');
  const tail = argv.slice(2);
  if (tail.length === 1 && tail[0] === '--help') return { kind: 'help', operation };
  if (!tail.length) return { kind: 'call', operation, arguments: {} };
  if (tail.length === 1 && tail[0] === '--stdin') return { kind: 'call', operation, stdin: true };
  if (tail.length === 2 && tail[0] === '--json') return { kind: 'call', operation, arguments: parseArguments(tail[1]) };
  throw invalid('Use exactly --json <object> or --stdin; no URL, header or credential arguments.');
}

function validDate(value) {
  if (typeof value !== 'string' || !/^\d{4}-\d{2}-\d{2}(?:T\d{2}:\d{2}:\d{2}(?:\.\d{1,3})?(?:Z|[+-]\d{2}:\d{2}))?$/.test(value)) return false;
  const ms = Date.parse(value);
  if (!Number.isFinite(ms)) return false;
  // Validate the calendar component separately; Date.parse normalizes February 30.
  return new Date(Date.parse(value.slice(0, 10))).toISOString().slice(0, 10) === value.slice(0, 10);
}
function validateValue(type, value) {
  switch (type.replace('!', '')) {
    case 'text': return typeof value === 'string' && value.length > 0 && value.length <= 4096 && !/[\x00-\x1f\x7f]/.test(value);
    case 'segment': return segment(value);
    case 'id': return Number.isSafeInteger(value) && value > 0;
    case 'ids': return typeof value === 'string' && /^[1-9][0-9]*(?:,[1-9][0-9]*){0,19}$/.test(value) && value.split(',').every(item => Number.isSafeInteger(Number(item)));
    case 'idArray': return Array.isArray(value) && value.length > 0 && value.length <= 20 && value.every(item => Number.isSafeInteger(item) && item > 0);
    case 'date': return validDate(value);
    case 'limit': return Number.isInteger(value) && value >= 1 && value <= 100;
    case 'pageLimit': return Number.isInteger(value) && value >= 1 && value <= 20;
    case 'offset': return Number.isSafeInteger(value) && value >= 0 && value <= 100000;
    case 'mode': return ['social', 'campus'].includes(value);
    case 'stage': return ['offer', 'pending_checkin', 'all'].includes(value);
    case 'order': return ['ASC', 'DESC'].includes(value);
    case 'hireMode': return [1, 2].includes(value);
    default: return false;
  }
}

function dateRange(args, start, end, maxDays) {
  if (args[start] === undefined && args[end] === undefined) return false;
  if (args[start] === undefined || args[end] === undefined) throw invalid(`Provide both ${start} and ${end}.`);
  const elapsed = Date.parse(args[end].slice(0, 10)) - Date.parse(args[start].slice(0, 10));
  if (elapsed < 0 || maxDays && elapsed > maxDays * 86400000) throw invalid('Invalid or excessive date range.');
  return true;
}

export function requestSpec(operation, args, orgId) {
  const policy = OPERATIONS[operation];
  if (!policy || !args || typeof args !== 'object' || Array.isArray(args)) throw invalid('Invalid operation arguments.');
  for (const name of Object.keys(args)) if (!Object.hasOwn(policy.fields, name)) throw invalid(`Unreviewed parameter: ${name}.`);
  for (const [name, type] of Object.entries(policy.fields)) {
    if (args[name] === undefined) { if (type.endsWith('!')) throw invalid(`Missing ${name}.`); }
    else if (!validateValue(type, args[name])) throw invalid(`Invalid ${name}.`);
  }
  let path = policy.path;
  if (path.includes('{orgId}')) {
    if (!segment(orgId)) throw new CLIError('configuration_required', 'Connect the Moka organization ID for job queries.', 'connect');
    path = path.replace('{orgId}', orgId);
  }
  if (args.jobId) path = path.replace('{jobId}', args.jobId);
  if (operation === 'interviews list') {
    const interview = dateRange(args, 'startDate', 'endDate', 31);
    const created = dateRange(args, 'createStartDate', 'createEndDate', 31);
    if (!interview && !created) throw invalid('Provide an interview or creation date range of at most 31 days.');
  }
  dateRange(args, 'archivedAtStart', 'archivedAtEnd');
  dateRange(args, 'movedAtStartTime', 'movedAtEndTime');
  const url = new URL('/api-platform' + path, HOST);
  const method = policy.method || 'GET';
  if (method === 'GET') {
    for (const [name, value] of Object.entries(args)) {
      if (name !== 'jobId') url.searchParams.set(name, Array.isArray(value) ? JSON.stringify(value) : String(value));
    }
    if (operation === 'candidates search' && args.limit === undefined) url.searchParams.set('limit', '20');
    if (operation === 'jobs list' && args.limit === undefined) url.searchParams.set('limit', '50');
  }
  return { url: url.href, method, ...(method === 'POST' ? { body: JSON.stringify(args) } : {}) };
}

export async function readJSON(response) {
  if (!response.body || !/^application\/(?:json|[a-z0-9.+-]+\+json)(?:;|$)/i.test(response.headers.get('content-type') || '')) throw protocol();
  const reader = response.body.getReader();
  let size = 0;
  const chunks = [];
  try {
    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      size += value.byteLength;
      if (size > MAX_OUTPUT) throw new CLIError('output_limit', 'Moka output exceeds 8 MiB; narrow the query.', 'narrow_query');
      chunks.push(Buffer.from(value));
    }
    try {
      const body = JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(Buffer.concat(chunks)));
      if (!body || typeof body !== 'object') throw protocol();
      return body;
    } catch { throw protocol(); }
  } finally { await reader.cancel().catch(() => {}); }
}

async function query(operation, args, grant, { fetchImpl, timeoutMs }) {
  const spec = requestSpec(operation, args, grant.orgId);
  const headers = { Accept: 'application/json', Authorization: 'Basic ' + Buffer.from(grant.key + ':').toString('base64') };
  if (spec.method === 'POST') headers['Content-Type'] = 'application/json';
  let response;
  try { response = await fetchImpl(spec.url, { method: spec.method, body: spec.body, headers, redirect: 'error', signal: AbortSignal.timeout(timeoutMs) }); }
  catch { throw new CLIError('transport_error', 'Moka connection failed or timed out.', 'check_connection', true); }
  if (!response.ok) {
    await response.body?.cancel().catch(() => {});
    if (response.status === 401) throw new CLIError('authorization_required', 'Moka rejected the API Key.', 'connect');
    if (response.status === 403) throw new CLIError('permission_denied', 'This Moka API is not permitted for the connected enterprise.', 'contact_moka_csm');
    if (response.status === 404) throw new CLIError('not_found', 'Moka resource or API was not found.', 'check_identifier');
    throw new CLIError('upstream_error', `Moka returned HTTP ${response.status}.`, 'retry_later', response.status === 429 || response.status >= 500);
  }
  let body;
  try { body = await readJSON(response); } catch (cause) {
    if (cause instanceof CLIError) throw cause;
    throw new CLIError('transport_error', 'Moka response was interrupted.', 'check_connection', true);
  }
  if (body.success === false || Object.hasOwn(body, 'code') && ![0, 200].includes(body.code)) {
    throw new CLIError('upstream_error', 'Moka rejected the request at the business layer.', 'check_connection');
  }
  if (OPERATIONS[operation].select === 'customFields') {
    if (!Object.hasOwn(body, 'customFields')) throw protocol();
    return { jobId: args.jobId, customFields: body.customFields };
  }
  if (OPERATIONS[operation].select === 'stage') {
    if (!Array.isArray(body.data)) throw protocol();
    return { data: body.data.map(item => ({ applicationId: item.applicationId, candidateId: item.candidateId, stageName: item.stageName })) };
  }
  return body;
}

export function maskSensitive(value) {
  if (Array.isArray(value)) return value.map(maskSensitive);
  if (!value || typeof value !== 'object') return value;
  return Object.fromEntries(Object.entries(value).map(([key, item]) => {
    if (/phone|mobile|citizenId|idCard|identityNumber/i.test(key) && typeof item === 'string') {
      return [key, item.length > 7 ? item.slice(0, 3) + '*'.repeat(item.length - 7) + item.slice(-4) : '[MASKED]'];
    }
    return [key, maskSensitive(item)];
  }));
}

export async function execute(argv, { env = process.env, stdin = process.stdin, fetchImpl = fetch, timeoutMs = 30000 } = {}) {
  const command = parseCommand(argv);
  if (command.kind === 'help') return { name: 'moka', version: VERSION, usage: 'moka <command> [--json <object> | --stdin]',
    operations: command.operation ? { [command.operation]: OPERATIONS[command.operation] } : OPERATIONS,
    credential: 'Platform CONNECTOR_CREDENTIALS_JSON: moka_api_key, moka_org_id; local MOKA_API_KEY and MOKA_ORG_ID.',
    identity: 'Enterprise API Key; platform User ownership does not confer Moka per-user data permissions.' };
  if (['version', 'init'].includes(command.kind)) return { name: 'agent-workspace-moka-cli', version: VERSION };
  if (command.kind === 'tools') return OPERATIONS;
  if (command.kind === 'schema') return OPERATIONS[command.operation];
  if (command.kind === 'unauth') return { authenticated: false, upstream_revoked: false, next_action: 'disconnect_platform_authorization' };
  const grant = credentials(env);
  if (!grant.key) {
    if (command.kind === 'status') return { authenticated: false, next_action: 'connect' };
    throw new CLIError('authorization_required', 'Connect an enterprise Moka API Key first.', 'connect');
  }
  if (['auth', 'status'].includes(command.kind)) {
    const result = await query('departments list', {}, grant, { fetchImpl, timeoutMs });
    if (!Array.isArray(result.departments)) throw protocol();
    return { authenticated: true, identity: 'enterprise_api_key', organization_configured: !!grant.orgId };
  }
  if (command.stdin) {
    const chunks = [];
    let size = 0;
    for await (const chunk of stdin) {
      size += Buffer.byteLength(chunk);
      if (size > MAX_INPUT) throw invalid('Input exceeds 64 KiB.');
      chunks.push(Buffer.from(chunk));
    }
    command.arguments = parseArguments(Buffer.concat(chunks).toString('utf8'));
  }
  return maskSensitive(await query(command.operation, command.arguments, grant, { fetchImpl, timeoutMs }));
}

export async function run(argv, options = {}) {
  const requestID = randomUUID();
  let output;
  try { output = { ok: true, data: await execute(argv, options), request_id: requestID, warnings: [] }; }
  catch (cause) {
    const error = cause instanceof CLIError ? cause : new CLIError('internal_error', 'CLI execution failed.', 'check_connection');
    output = { ok: false, error: { type: error.type, message: error.message, retryable: error.retryable, next_action: error.nextAction }, request_id: requestID };
  }
  let serialized = JSON.stringify(output);
  const env = options.env || process.env;
  const secrets = [env.MOKA_API_KEY, env.CONNECTOR_CREDENTIALS_JSON];
  try {
    const { key } = credentials(env);
    if (key) secrets.push(key, Buffer.from(key + ':').toString('base64'), 'Basic ' + Buffer.from(key + ':').toString('base64'));
  } catch { /* Invalid credentials never enter an upstream message. */ }
  for (const secret of secrets.filter(value => typeof value === 'string' && value.length > 0).sort((a, b) => b.length - a.length)) {
    serialized = serialized.split(JSON.stringify(secret).slice(1, -1)).join('[REDACTED]');
  }
  return { text: serialized, exitCode: output.ok ? 0 : 1 };
}

if (process.argv[1] && import.meta.url === pathToFileURL(realpathSync(process.argv[1])).href) {
  const result = await run(process.argv.slice(2));
  process.stdout.write(result.text + '\n');
  process.exitCode = result.exitCode;
}
