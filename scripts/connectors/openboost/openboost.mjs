#!/usr/bin/env node
import { randomUUID } from 'node:crypto';
import { realpathSync } from 'node:fs';
import { pathToFileURL } from 'node:url';

export const VERSION = '0.1.0';
export const ENDPOINT = 'https://mcp.microdata-inc.com/mcp-servers/openboost-all-mcp';
export const OPERATIONS = Object.freeze({
  "account status": {
    "tool": "account_info",
    "risk": "low"
  },
  "call account_catalog": {
    "tool": "account_catalog",
    "risk": "low"
  },
  "call account_links": {
    "tool": "account_links",
    "risk": "low"
  },
  "call account_order_status": {
    "tool": "account_order_status",
    "risk": "low"
  },
  "call account_orders": {
    "tool": "account_orders",
    "risk": "low"
  },
  "call amz_aba_research_trends": {
    "tool": "amz_aba_research_trends",
    "risk": "low"
  },
  "call amz_aba_research_weekly": {
    "tool": "amz_aba_research_weekly",
    "risk": "low"
  },
  "call amz_category_query": {
    "tool": "amz_category_query",
    "risk": "low"
  },
  "call amz_category_query_v2": {
    "tool": "amz_category_query_v2",
    "risk": "low"
  },
  "call amz_google_trends": {
    "tool": "amz_google_trends",
    "risk": "low"
  },
  "call amz_hot_amz_hot_cat_tree": {
    "tool": "amz_hot_amz_hot_cat_tree",
    "risk": "low"
  },
  "call amz_hot_amz_hot_list_v2": {
    "tool": "amz_hot_amz_hot_list_v2",
    "risk": "low"
  },
  "call amz_keyword_miner": {
    "tool": "amz_keyword_miner",
    "risk": "low"
  },
  "call amz_keyword_order": {
    "tool": "amz_keyword_order",
    "risk": "low"
  },
  "call amz_keyword_research": {
    "tool": "amz_keyword_research",
    "risk": "low"
  },
  "call amz_keyword_research_trends": {
    "tool": "amz_keyword_research_trends",
    "risk": "low"
  },
  "call amz_market_brand": {
    "tool": "amz_market_brand",
    "risk": "low"
  },
  "call amz_market_goods": {
    "tool": "amz_market_goods",
    "risk": "low"
  },
  "call amz_market_price": {
    "tool": "amz_market_price",
    "risk": "low"
  },
  "call amz_market_rating": {
    "tool": "amz_market_rating",
    "risk": "low"
  },
  "call amz_market_ratings": {
    "tool": "amz_market_ratings",
    "risk": "low"
  },
  "call amz_market_research": {
    "tool": "amz_market_research",
    "risk": "low"
  },
  "call amz_market_seller": {
    "tool": "amz_market_seller",
    "risk": "low"
  },
  "call amz_market_seller_location": {
    "tool": "amz_market_seller_location",
    "risk": "low"
  },
  "call amz_market_seller_type": {
    "tool": "amz_market_seller_type",
    "risk": "low"
  },
  "call amz_market_shelf_time": {
    "tool": "amz_market_shelf_time",
    "risk": "low"
  },
  "call amz_market_shelf_trend": {
    "tool": "amz_market_shelf_trend",
    "risk": "low"
  },
  "call amz_market_statistics": {
    "tool": "amz_market_statistics",
    "risk": "low"
  },
  "call amz_product_competitor": {
    "tool": "amz_product_competitor",
    "risk": "low"
  },
  "call amz_product_selection": {
    "tool": "amz_product_selection",
    "risk": "low"
  },
  "call amz_research_monthly": {
    "tool": "amz_research_monthly",
    "risk": "low"
  },
  "call amz_review_query": {
    "tool": "amz_review_query",
    "risk": "low"
  },
  "call amz_sales_prediction_bsr": {
    "tool": "amz_sales_prediction_bsr",
    "risk": "low"
  },
  "call amz_sales_query": {
    "tool": "amz_sales_query",
    "risk": "low"
  },
  "call amz_sku_query": {
    "tool": "amz_sku_query",
    "risk": "low"
  },
  "call amz_traffic_extend": {
    "tool": "amz_traffic_extend",
    "risk": "low"
  },
  "call amz_traffic_keyword": {
    "tool": "amz_traffic_keyword",
    "risk": "low"
  },
  "call amz_traffic_keyword_stat": {
    "tool": "amz_traffic_keyword_stat",
    "risk": "low"
  },
  "call amz_traffic_listing_page": {
    "tool": "amz_traffic_listing_page",
    "risk": "low"
  },
  "call amz_traffic_listing_stat": {
    "tool": "amz_traffic_listing_stat",
    "risk": "low"
  },
  "call amz_traffic_source": {
    "tool": "amz_traffic_source",
    "risk": "low"
  },
  "call instagram_fetch_hashtag_posts": {
    "tool": "instagram_fetch_hashtag_posts",
    "risk": "low"
  },
  "call instagram_fetch_post_comments": {
    "tool": "instagram_fetch_post_comments",
    "risk": "low"
  },
  "call instagram_fetch_post_info": {
    "tool": "instagram_fetch_post_info",
    "risk": "low"
  },
  "call instagram_fetch_reel_info": {
    "tool": "instagram_fetch_reel_info",
    "risk": "low"
  },
  "call instagram_fetch_user_info": {
    "tool": "instagram_fetch_user_info",
    "risk": "low"
  },
  "call instagram_fetch_user_reels": {
    "tool": "instagram_fetch_user_reels",
    "risk": "low"
  },
  "call patent_abstract_image": {
    "tool": "patent_abstract_image",
    "risk": "low"
  },
  "call patent_abstract_translated": {
    "tool": "patent_abstract_translated",
    "risk": "low"
  },
  "call patent_bibliography": {
    "tool": "patent_bibliography",
    "risk": "low"
  },
  "call patent_claim_data": {
    "tool": "patent_claim_data",
    "risk": "low"
  },
  "call patent_claim_translated": {
    "tool": "patent_claim_translated",
    "risk": "low"
  },
  "call patent_description": {
    "tool": "patent_description",
    "risk": "low"
  },
  "call patent_description_translated": {
    "tool": "patent_description_translated",
    "risk": "low"
  },
  "call patent_family": {
    "tool": "patent_family",
    "risk": "low"
  },
  "call patent_fulltext_image": {
    "tool": "patent_fulltext_image",
    "risk": "low"
  },
  "call patent_image_search_multiple": {
    "tool": "patent_image_search_multiple",
    "risk": "low"
  },
  "call patent_image_search_single": {
    "tool": "patent_image_search_single",
    "risk": "low"
  },
  "call patent_legal_status": {
    "tool": "patent_legal_status",
    "risk": "low"
  },
  "call patent_pdf": {
    "tool": "patent_pdf",
    "risk": "low"
  },
  "call patent_query_count": {
    "tool": "patent_query_count",
    "risk": "low"
  },
  "call patent_query_search": {
    "tool": "patent_query_search",
    "risk": "low"
  },
  "call patent_query_search_with_agg": {
    "tool": "patent_query_search_with_agg",
    "risk": "low"
  },
  "call patent_semantic_search": {
    "tool": "patent_semantic_search",
    "risk": "low"
  },
  "call reddit_fetch_post_comments": {
    "tool": "reddit_fetch_post_comments",
    "risk": "low"
  },
  "call reddit_fetch_post_details": {
    "tool": "reddit_fetch_post_details",
    "risk": "low"
  },
  "call tk_expert_sale_show": {
    "tool": "tk_expert_sale_show",
    "risk": "low"
  },
  "call tt_commodity_detail": {
    "tool": "tt_commodity_detail",
    "risk": "low"
  },
  "call tt_commodity_expert_draw_expert_trend": {
    "tool": "tt_commodity_expert_draw_expert_trend",
    "risk": "low"
  },
  "call tt_commodity_get_commodity_cat_tree": {
    "tool": "tt_commodity_get_commodity_cat_tree",
    "risk": "low"
  },
  "call tt_commodity_info_list": {
    "tool": "tt_commodity_info_list",
    "risk": "low"
  },
  "call tt_commodity_sales_trend": {
    "tool": "tt_commodity_sales_trend",
    "risk": "low"
  },
  "call tt_commodity_statistics_get_category_statistics": {
    "tool": "tt_commodity_statistics_get_category_statistics",
    "risk": "low"
  },
  "call tt_commodity_statistics_get_expert_statistics": {
    "tool": "tt_commodity_statistics_get_expert_statistics",
    "risk": "low"
  },
  "call tt_commodity_statistics_get_price_statistics": {
    "tool": "tt_commodity_statistics_get_price_statistics",
    "risk": "low"
  },
  "call tt_commodity_statistics_get_total_statistics": {
    "tool": "tt_commodity_statistics_get_total_statistics",
    "risk": "low"
  },
  "call tt_commodity_voice_analyze": {
    "tool": "tt_commodity_voice_analyze",
    "risk": "low"
  },
  "call tt_commodity_voice_read": {
    "tool": "tt_commodity_voice_read",
    "risk": "low"
  },
  "call tt_expert_basic_analyze": {
    "tool": "tt_expert_basic_analyze",
    "risk": "low"
  },
  "call tt_expert_category_analyze": {
    "tool": "tt_expert_category_analyze",
    "risk": "low"
  },
  "call tt_expert_detail": {
    "tool": "tt_expert_detail",
    "risk": "low"
  },
  "call tt_expert_fans_analyze": {
    "tool": "tt_expert_fans_analyze",
    "risk": "low"
  },
  "call tt_expert_fans_portrait": {
    "tool": "tt_expert_fans_portrait",
    "risk": "low"
  },
  "call tt_expert_info_list": {
    "tool": "tt_expert_info_list",
    "risk": "low"
  },
  "call tt_hashtag_list": {
    "tool": "tt_hashtag_list",
    "risk": "low"
  },
  "call tt_shop_commodity_account_list": {
    "tool": "tt_shop_commodity_account_list",
    "risk": "low"
  },
  "call tt_shop_detail": {
    "tool": "tt_shop_detail",
    "risk": "low"
  },
  "call tt_shop_info_list": {
    "tool": "tt_shop_info_list",
    "risk": "low"
  },
  "call tt_shop_market_info": {
    "tool": "tt_shop_market_info",
    "risk": "low"
  },
  "call tt_shop_sales_trend": {
    "tool": "tt_shop_sales_trend",
    "risk": "low"
  },
  "call tt_video_content": {
    "tool": "tt_video_content",
    "risk": "low"
  },
  "call tt_video_detail": {
    "tool": "tt_video_detail",
    "risk": "low"
  },
  "call tt_video_info_list": {
    "tool": "tt_video_info_list",
    "risk": "low"
  },
  "call tt_video_new_video_summary": {
    "tool": "tt_video_new_video_summary",
    "risk": "low"
  },
  "call tt_video_script_diy_query": {
    "tool": "tt_video_script_diy_query",
    "risk": "low"
  },
  "call tt_video_voice_read": {
    "tool": "tt_video_voice_read",
    "risk": "low"
  },
  "call tt_video_voice_summary": {
    "tool": "tt_video_voice_summary",
    "risk": "low"
  },
  "call youtube_fetch_post_comments": {
    "tool": "youtube_fetch_post_comments",
    "risk": "low"
  },
  "call youtube_fetch_post_info": {
    "tool": "youtube_fetch_post_info",
    "risk": "low"
  },
  "call youtube_fetch_user_info": {
    "tool": "youtube_fetch_user_info",
    "risk": "low"
  }
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
    if (!value || typeof value !== 'object' || Array.isArray(value) || Object.keys(value).some(key => key !== 'openboost_secret_key')) {
      throw invalid('Platform credentials require only openboost_secret_key.');
    }
    token = value.openboost_secret_key;
  } else {
    token = env.OPENBOOST_SECRET_KEY;
  }
  if (token === undefined || token === '') return undefined;
  if (typeof token !== 'string' || token.length > 32768 || /[\s\x00-\x1f\x7f]/u.test(token)) {
    throw invalid('Invalid OpenBoost credential.');
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
  if (!OPERATIONS[operation]) throw invalid('Unreviewed command. Run openboost --help.');
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
    const headers = { 'Content-Type': 'application/json', Accept: 'application/json, text/event-stream', 'secret-key': this.token };
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
      if ([401, 403].includes(response.status)) throw new CLIError('authorization_required', 'OpenBoost rejected the credential or account permission.', 'connect');
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
    const value = await this.request('initialize', { protocolVersion: this.protocol, capabilities: {}, clientInfo: { name: 'agent-workspace-openboost-cli', version: VERSION } });
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
        Accept: 'application/json, text/event-stream', 'secret-key': this.token,
        'Mcp-Session-Id': this.session, 'MCP-Protocol-Version': this.protocol,
      } });
      await response.body?.cancel().catch(() => {});
    } catch { /* Closing a session must not replace the command result. */ }
  }
}

export async function execute(argv, { env = process.env, stdin = process.stdin, fetchImpl = fetch, timeoutMs } = {}) {
  const command = parseCommand(argv);
  if (command.kind === 'help') return { name: 'openboost', version: VERSION, usage: 'openboost <command> [--json <object> | --stdin]',
    operations: command.operation ? { [command.operation]: OPERATIONS[command.operation] } : OPERATIONS,
    discovery: ['tools', 'schema <command>'], credential: 'OPENBOOST_SECRET_KEY or platform CONNECTOR_CREDENTIALS_JSON; no credential arguments',
    approval: 'Use agent-cli for managed invocations. Payment order creation is excluded from this revision.' };
  if (['version', 'init'].includes(command.kind)) return { name: 'agent-workspace-openboost-cli', version: VERSION };
  if (command.kind === 'unauth') return { authenticated: false, next_action: 'disconnect_platform_authorization', upstream_revoked: false };
  const token = credential(env);
  if (!token) {
    if (command.kind === 'status') return { authenticated: false, next_action: 'connect' };
    throw new CLIError('authorization_required', 'Connect a OpenBoost credential first.', 'connect');
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
      if (detail && /status: (401|403)\b/.test(detail)) throw new CLIError('authorization_required', 'OpenBoost rejected the credential or service entitlement.', 'connect');
      throw new CLIError('tool_error', detail || 'OpenBoost tool reported failure; inspect account/task before retrying.', 'inspect_task');
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
  const secrets = [env.OPENBOOST_SECRET_KEY, env.CONNECTOR_CREDENTIALS_JSON];
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
