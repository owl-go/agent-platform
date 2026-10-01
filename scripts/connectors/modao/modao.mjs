#!/usr/bin/env node
import { randomUUID } from 'node:crypto';
import { realpathSync } from 'node:fs';
import { pathToFileURL } from 'node:url';

export const VERSION = '0.1.0';
export const ENDPOINT = 'https://modao.cc/agent-py/ai/mcp';
export const OPERATIONS = Object.freeze({
  'account status': { tool: 'get_account_status', risk: 'low' },
  'task result': { tool: 'get_task_result', risk: 'low' },
  'generate auto': { tool: 'generate', risk: 'high' },
  'generate html': { tool: 'generate_html', risk: 'high' },
  'generate react': { tool: 'generate_react', risk: 'high' },
  'generate prd': { tool: 'generate_prd', risk: 'high' },
  'proto import': { tool: 'import_to_proto', risk: 'high' },
});
const MAX_BYTES = 8 * 1024 * 1024;
const MAX_INPUT = 2 * 1024 * 1024;
const PROTOCOLS = ['2025-03-26', '2025-06-18', '2025-11-25'];

export class CLIError extends Error {
  constructor(type, message, nextAction = 'check_command', retryable = false) {
    super(message);
    Object.assign(this, { type, nextAction, retryable });
  }
}
const invalid = message => new CLIError('invalid_request', message);
const protocolError = () => new CLIError('protocol_error', 'Invalid or unsupported MCP response.', 'check_connection');
export function credential(env) {
  let token;
  if (env.CONNECTOR_CREDENTIALS_JSON !== undefined) {
    let value;
    try { value = JSON.parse(env.CONNECTOR_CREDENTIALS_JSON); } catch { throw invalid('Invalid platform credentials JSON.'); }
    if (!value || typeof value !== 'object' || Array.isArray(value) || Object.keys(value).some(key => key !== 'modao_token')) {
      throw invalid('Platform credentials require only modao_token.');
    }
    token = value.modao_token;
  } else {
    token = env.MODAO_TOKEN;
  }
  if (token === undefined || token === '') return undefined;
  if (typeof token !== 'string' || token.length > 32768 || /[\s\x00-\x1f\x7f]/u.test(token)) {
    throw invalid('Invalid Modao personal-space credential.');
  }
  return token;
}

export function parseCommand(argv) {
  if (argv.length === 0 || argv.length === 1 && ['help', '--help'].includes(argv[0])) return { kind: 'help' };
  if (argv.length === 1 && ['version', '--version', 'init', 'auth', 'status', 'unauth', 'tools'].includes(argv[0])) {
    return { kind: argv[0] === '--version' ? 'version' : argv[0] };
  }
  if (argv[0] === 'schema' && argv.length === 3 && OPERATIONS[argv.slice(1).join(' ')]) {
    return { kind: 'schema', operation: argv.slice(1).join(' ') };
  }
  const operation = argv.slice(0, 2).join(' ');
  if (!OPERATIONS[operation]) throw invalid('Unreviewed command. Run modao --help.');
  const tail = argv.slice(2);
  if (tail.length === 1 && tail[0] === '--help') return { kind: 'help', operation };
  if (tail.length === 0 && operation === 'account status') return { kind: 'call', operation, arguments: {} };
  if (tail.length === 1 && tail[0] === '--stdin') return { kind: 'call', operation, stdin: true };
  if (tail.length === 2 && tail[0] === '--json') return { kind: 'call', operation, arguments: parseArguments(tail[1]) };
  throw invalid('Use exactly --stdin or --json <object>; account status also accepts no arguments.');
}

export function parseArguments(raw) {
  if (typeof raw !== 'string' || Buffer.byteLength(raw) > MAX_INPUT) throw invalid('Arguments exceed 2 MiB.');
  let value;
  try { value = JSON.parse(raw); } catch { throw invalid('Arguments must be valid JSON.'); }
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw invalid('Arguments must be a JSON object.');
  return value;
}

function matchingMessage(value, id) {
  const entries = Array.isArray(value) ? value : [value];
  return entries.find(entry => entry && entry.jsonrpc === '2.0' && entry.id === id);
}
function result(message) {
  if (message.error) {
    const detail = typeof message.error.message === 'string' ? message.error.message.slice(0, 1000) : 'MCP request was rejected.';
    throw new CLIError('upstream_error', detail, 'inspect_task');
  }
  if (!Object.hasOwn(message, 'result')) throw protocolError();
  return message.result;
}

async function readResponse(response, id) {
  const mime = response.headers.get('content-type')?.split(';')[0].trim();
  if (!['application/json', 'text/event-stream'].includes(mime) || !response.body) throw protocolError();
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '', size = 0;
  try {
    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      size += value.byteLength;
      if (size > MAX_BYTES) throw new CLIError('output_limit', 'MCP output exceeds 8 MiB.', 'inspect_task');
      buffer += decoder.decode(value, { stream: true });
      if (mime === 'text/event-stream') {
        let boundary;
        while ((boundary = /\r?\n\r?\n/.exec(buffer))) {
          const event = buffer.slice(0, boundary.index);
          buffer = buffer.slice(boundary.index + boundary[0].length);
          const data = event.split(/\r?\n/).filter(line => line.startsWith('data:')).map(line => line.slice(5).replace(/^ /, '')).join('\n');
          if (!data) continue;
          let decoded;
          try { decoded = JSON.parse(data); } catch { throw protocolError(); }
          const match = matchingMessage(decoded, id);
          if (match) return result(match);
        }
      }
    }
    if (mime === 'application/json') {
      let decoded;
      try { decoded = JSON.parse(buffer + decoder.decode()); } catch { throw protocolError(); }
      const match = matchingMessage(decoded, id);
      if (match) return result(match);
    }
    throw protocolError();
  } finally { await reader.cancel().catch(() => {}); }
}

export class MCPClient {
  constructor(token, { fetchImpl = fetch, timeoutMs = 125000 } = {}) {
    this.token = token;
    this.fetch = fetchImpl;
    this.signal = AbortSignal.timeout(timeoutMs);
    this.sequence = 0;
    this.protocol = PROTOCOLS[0];
  }
  async request(method, params, notification = false) {
    const id = notification ? undefined : ++this.sequence;
    const headers = { 'Content-Type': 'application/json', Accept: 'application/json, text/event-stream', 'modao-token': this.token };
    if (this.initialized) headers['MCP-Protocol-Version'] = this.protocol;
    if (this.session) headers['Mcp-Session-Id'] = this.session;
    let response;
    try {
      response = await this.fetch(ENDPOINT, {
        method: 'POST', headers, redirect: 'error', signal: this.signal,
        body: JSON.stringify({ jsonrpc: '2.0', ...(notification ? {} : { id }), method, ...(params === undefined ? {} : { params }) }),
      });
    } catch {
      throw new CLIError('transport_error', 'MCP connection failed or timed out; execution outcome may be unknown.', 'inspect_task');
    }
    if (!response.ok) {
      await response.body?.cancel().catch(() => {});
      if ([401, 403].includes(response.status)) throw new CLIError('authorization_required', 'Modao rejected the credential or account permission.', 'connect');
      throw new CLIError('upstream_error', `MCP returned HTTP ${response.status}; execution outcome may be unknown.`, 'inspect_task');
    }
    if (method === 'initialize') {
      const session = response.headers.get('Mcp-Session-Id');
      if (session && (!/^[\x21-\x7e]{1,1024}$/.test(session))) throw protocolError();
      this.session = session;
    }
    if (notification) {
      await response.body?.cancel().catch(() => {});
      if (response.status !== 202) throw protocolError();
      return;
    }
    try { return await readResponse(response, id); } catch (cause) {
      if (cause instanceof CLIError) throw cause;
      throw new CLIError('transport_error', 'MCP response interrupted; execution outcome may be unknown.', 'inspect_task');
    }
  }
  async initialize() {
    const value = await this.request('initialize', { protocolVersion: this.protocol, capabilities: {}, clientInfo: { name: 'agent-workspace-modao-cli', version: VERSION } });
    if (!value || !PROTOCOLS.includes(value.protocolVersion) || !value.capabilities?.tools) throw protocolError();
    this.protocol = value.protocolVersion;
    this.initialized = true;
    await this.request('notifications/initialized', undefined, true);
  }
  async tools() {
    const tools = [], cursors = new Set();
    let cursor, totalBytes = 0;
    for (let page = 0; page < 32; page++) {
      const value = await this.request('tools/list', cursor ? { cursor } : {});
      if (!Array.isArray(value?.tools)) throw protocolError();
      totalBytes += Buffer.byteLength(JSON.stringify(value.tools));
      if (totalBytes > MAX_BYTES || tools.length + value.tools.length > 4096) throw protocolError();
      for (const tool of value.tools) {
        if (!tool || typeof tool.name !== 'string' || !tool.inputSchema || tools.some(item => item.name === tool.name)) throw protocolError();
        tools.push(tool);
      }
      if (!value.nextCursor) return tools.filter(tool => Object.values(OPERATIONS).some(operation => operation.tool === tool.name));
      if (typeof value.nextCursor !== 'string' || cursors.has(value.nextCursor)) throw protocolError();
      cursors.add(value.nextCursor);
      cursor = value.nextCursor;
    }
    throw protocolError();
  }
  async close() {
    if (!this.session) return;
    try {
      const response = await this.fetch(ENDPOINT, { method: 'DELETE', redirect: 'error', signal: AbortSignal.timeout(2000), headers: {
        Accept: 'application/json, text/event-stream', 'modao-token': this.token,
        'Mcp-Session-Id': this.session, 'MCP-Protocol-Version': this.protocol,
      } });
      await response.body?.cancel().catch(() => {});
    } catch { /* Closing a session must not replace the command result. */ }
  }
}

export async function execute(argv, { env = process.env, stdin = process.stdin, fetchImpl = fetch, timeoutMs } = {}) {
  const command = parseCommand(argv);
  if (command.kind === 'help') return { name: 'modao', version: VERSION, usage: 'modao <command> [--json <object> | --stdin]',
    operations: command.operation ? { [command.operation]: OPERATIONS[command.operation] } : OPERATIONS,
    discovery: ['tools', 'schema <command>'], credential: 'MODAO_TOKEN or platform CONNECTOR_CREDENTIALS_JSON; no credential arguments',
    approval: 'Managed high-risk commands require platform one-use approval and --target. Direct local CLI use is outside that broker.' };
  if (['version', 'init'].includes(command.kind)) return { name: 'agent-workspace-modao-cli', version: VERSION };
  if (command.kind === 'unauth') return { authenticated: false, next_action: 'disconnect_platform_authorization', upstream_revoked: false };
  const token = credential(env);
  if (!token) {
    if (command.kind === 'status') return { authenticated: false, next_action: 'connect' };
    throw new CLIError('authorization_required', 'Connect a Modao personal-space credential first.', 'connect');
  }
  if (command.stdin) {
    const chunks = [];
    let size = 0;
    for await (const chunk of stdin) {
      size += Buffer.byteLength(chunk);
      if (size > MAX_INPUT) throw invalid('Arguments exceed 2 MiB.');
      chunks.push(Buffer.from(chunk));
    }
    command.arguments = parseArguments(Buffer.concat(chunks).toString('utf8'));
  }
  const client = new MCPClient(token, { fetchImpl, timeoutMs });
  try {
    await client.initialize();
    const tools = await client.tools();
    if (command.kind === 'tools') return tools.map(tool => ({ ...tool, command: Object.keys(OPERATIONS).find(key => OPERATIONS[key].tool === tool.name) }));
    const operation = ['auth', 'status'].includes(command.kind) ? 'account status' : command.operation;
    const tool = tools.find(item => item.name === OPERATIONS[operation].tool);
    if (!tool) throw new CLIError('upstream_unsupported', 'The reviewed tool is absent from the current authorized catalog.', 'check_connection');
    if (command.kind === 'schema') return tool;
    const value = await client.request('tools/call', { name: tool.name, arguments: command.arguments || {} });
    if (!value || !Array.isArray(value.content)) throw protocolError();
    if (value.isError) {
      const detail = value.content.find(item => item?.type === 'text' && typeof item.text === 'string')?.text.slice(0, 1000);
      throw new CLIError('tool_error', detail || 'Modao tool reported failure; inspect account/task before retrying.', 'inspect_task');
    }
    return ['auth', 'status'].includes(command.kind) ? { authenticated: true, account: value } : value;
  } finally { await client.close(); }
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
  const secrets = [env.MODAO_TOKEN, env.CONNECTOR_CREDENTIALS_JSON];
  try { secrets.push(credential(env)); } catch { /* Invalid credentials must not appear in errors. */ }
  for (const secret of secrets.filter(value => typeof value === 'string' && value.length > 0)) {
    serialized = serialized.split(JSON.stringify(secret).slice(1, -1)).join('[REDACTED]');
  }
  return { text: serialized, exitCode: output.ok ? 0 : 1 };
}
if (process.argv[1] && import.meta.url === pathToFileURL(realpathSync(process.argv[1])).href) {
  const value = await run(process.argv.slice(2));
  process.stdout.write(value.text + '\n');
  process.exitCode = value.exitCode;
}
