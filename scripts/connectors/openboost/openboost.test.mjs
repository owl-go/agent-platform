import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { Readable } from 'node:stream';
import { test } from 'node:test';
import { credential, ENDPOINT, execute, OPERATIONS, parseCommand, run } from './openboost.mjs';

const TOKEN = 'openboost_fixture_secret';
const env = { CONNECTOR_CREDENTIALS_JSON: JSON.stringify({ openboost_secret_key: TOKEN }) };

async function fixture(t, { sse = false, status = 200, missing = false, toolError = false, authorizationError = false, stateless = false, mismatch = false, hang = false, redirect = false } = {}) {
  const seen = [];
  const server = createServer(async (request, response) => {
    assert.equal(request.url, '/mcp');
    assert.equal(request.headers['secret-key'], TOKEN);
    assert.equal(request.headers.authorization, undefined);
    assert.equal(request.headers.cookie, undefined);
    assert.equal(request.headers.accept, 'application/json, text/event-stream');
    if (request.method === 'DELETE') {
      assert.equal(request.headers['mcp-session-id'], stateless ? undefined : 'fixture-session');
      seen.push({ method: 'DELETE' });
      response.writeHead(405).end();
      return;
    }
    assert.equal(request.method, 'POST');
    assert.equal(request.headers['content-type'], 'application/json');
    const chunks = [];
    for await (const chunk of request) chunks.push(chunk);
    const message = JSON.parse(Buffer.concat(chunks).toString());
    seen.push(message);
    if (message.method !== 'initialize') {
      assert.equal(request.headers['mcp-session-id'], stateless ? undefined : 'fixture-session');
      assert.equal(request.headers['mcp-protocol-version'], '2025-03-26');
    }
    if (status !== 200) { response.writeHead(status).end(TOKEN); return; }
    if (redirect) { response.writeHead(302, { Location: 'http://127.0.0.1:1/credential-leak' }).end(); return; }
    if (hang) return;
    if (message.method === 'notifications/initialized') {
      assert.equal(Object.hasOwn(message, 'id'), false);
      response.writeHead(202).end();
      return;
    }
    let result;
    if (message.method === 'initialize') {
      assert.deepEqual(message.params.capabilities, {});
      assert.equal(message.params.protocolVersion, '2025-03-26');
      if (!stateless) response.setHeader('Mcp-Session-Id', 'fixture-session');
      result = { protocolVersion: '2025-03-26', capabilities: { tools: {} } };
    } else if (message.method === 'tools/list') {
      const tools = Object.values(OPERATIONS).map(value => ({ name: value.tool, inputSchema: { type: 'object', properties: { text: { type: 'string' } } } }));
      result = { tools: missing ? [] : [...tools, { name: 'unreviewed_write', inputSchema: { type: 'object' } }] };
    } else if (message.method === 'tools/call') {
      assert(Object.values(OPERATIONS).some(value => value.tool === message.params.name));
      result = { content: [{ type: 'text', text: JSON.stringify({ task_id: '真实任务', credential_reflection: TOKEN, arguments: message.params.arguments }) }], ...(toolError ? { isError: true } : {}) };
      if (authorizationError) result = { content: [{ type: 'text', text: 'call failed, status: 401, response: {"msg":"缺少 Authorization 或 secret-key"}' }], isError: true };
    } else assert.fail('unreviewed MCP method');
    const payload = JSON.stringify({ jsonrpc: '2.0', id: mismatch ? -1 : message.id, result });
    if (!sse) { response.writeHead(200, { 'Content-Type': 'application/json' }).end(payload); return; }
    response.writeHead(200, { 'Content-Type': 'text/event-stream; charset=utf-8' });
    response.write('event: message\r\ndata: {"jsonrpc":"2.0","method":"notifications/progress"}\r\n\r\n');
    const bytes = Buffer.from(`event: message\r\ndata: ${payload}\r\n\r\n`);
    // Split UTF-8 and CRLF delimiters, and keep the stream open after the result.
    for (let index = 0; index < bytes.length; index += 3) response.write(bytes.subarray(index, index + 3));
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  t.after(() => { server.closeAllConnections(); server.close(); });
  const fetchImpl = (url, options) => {
    assert.equal(url, ENDPOINT);
    assert.equal(options.redirect, 'error');
    return fetch(`http://127.0.0.1:${server.address().port}/mcp`, options);
  };
  return { fetchImpl, seen };
}

test('strict upstream handshake, session headers, result and cleanup', async t => {
  const { fetchImpl, seen } = await fixture(t);
  const result = await execute(['call', 'amz_sku_query', '--json', '{"text":"产品原型"}'], { env, fetchImpl });
  assert.equal(JSON.parse(result.content[0].text).arguments.text, '产品原型');
  assert.deepEqual(seen.map(value => value.method), ['initialize', 'notifications/initialized', 'tools/list', 'tools/call', 'DELETE']);
  assert.equal(seen[3].params.name, 'amz_sku_query');
});

test('SSE completes without waiting for stream close and handles split UTF-8', async t => {
  const { fetchImpl } = await fixture(t, { sse: true });
  const value = await run(['call', 'tt_video_detail', '--json', '{"task_id":"真实任务"}'], { env, fetchImpl, timeoutMs: 2000 });
  assert.equal(value.exitCode, 0);
  assert(value.text.includes('真实任务'));
  assert(!value.text.includes(TOKEN));
  assert(value.text.includes('[REDACTED]'));
  JSON.parse(value.text);
});

test('stdin keeps HTML and shell metacharacters as data', async t => {
  const { fetchImpl, seen } = await fixture(t);
  const html = '<html>$(touch /tmp/never-execute); `id`</html>';
  await execute(['call', 'patent_query_search', '--stdin'], { env, fetchImpl, stdin: Readable.from([JSON.stringify({ html })]) });
  assert.deepEqual(seen.find(value => value.method === 'tools/call').params.arguments, { html });
});

test('all 100 reviewed operations route to their fixed MCP name', async t => {
  const { fetchImpl, seen } = await fixture(t);
  for (const [command, operation] of Object.entries(OPERATIONS)) {
    await execute([...command.split(' '), '--json', '{}'], { env, fetchImpl });
    assert.equal(seen.filter(value => value.method === 'tools/call').at(-1).params.name, operation.tool);
  }
});

test('discovery filters unreviewed tools and schema never invokes a business tool', async t => {
  const { fetchImpl, seen } = await fixture(t);
  const tools = await execute(['tools'], { env, fetchImpl });
  assert.equal(tools.length, 100);
  assert(!tools.some(value => value.name === 'unreviewed_write'));
  const schema = await execute(['schema', 'call', 'amz_review_query'], { env, fetchImpl });
  assert.equal(schema.name, 'amz_review_query');
  assert.equal(seen.filter(value => value.method === 'tools/call').length, 0);
});

test('raw calls, command tails, endpoint overrides and non-object input fail before network', async () => {
  for (const args of [
    ['call', 'unreviewed_write'], ['call', 'account_order_create', '--json', '{}'], ['schema', 'unreviewed', 'write'], ['tools', '--url', 'https://evil.invalid'],
    ['account', 'status', '--json', '{}', 'call', 'amz_sku_query'], ['call', 'amz_sku_query', '--json', '[]'],
    ['call', 'amz_sku_query', '--json', 'null'], ['call', 'amz_sku_query', '--json', '{}', '--stdin'],
    ['call', 'amz_sku_query', '--json', '{}\n{}'], ['call', 'amz_sku_query', '--token', TOKEN],
  ]) {
    const value = await run(args, { env, fetchImpl: () => assert.fail('network must not start') });
    assert.equal(JSON.parse(value.text).error.type, 'invalid_request');
  }
});

test('platform credentials are strict and cannot fall back to an ambient token', async () => {
  for (const value of ['{', '[]', 'null', '{"openboost_secret_key":" token "}', '{"openboost_secret_key":"x","url":"https://evil.invalid"}', '{"openboost_secret_key":42}']) {
    assert.throws(() => credential({ CONNECTOR_CREDENTIALS_JSON: value, OPENBOOST_SECRET_KEY: TOKEN }));
  }
  assert.equal(credential({ CONNECTOR_CREDENTIALS_JSON: '{}', OPENBOOST_SECRET_KEY: TOKEN }), undefined);
  assert.equal(credential({ OPENBOOST_SECRET_KEY: TOKEN }), TOKEN);
  const status = await execute(['status'], { env: {} });
  assert.equal(status.authenticated, false);
  const failure = await run(['account', 'status'], { env: {} });
  assert.equal(JSON.parse(failure.text).error.type, 'authorization_required');
});

test('status checks a real read while lifecycle help and unauth are offline', async t => {
  const { fetchImpl, seen } = await fixture(t);
  const status = await execute(['status'], { env, fetchImpl });
  assert.equal(status.authenticated, true);
  assert.equal(seen.find(value => value.method === 'tools/call').params.name, 'account_info');
  for (const command of [['--help'], ['version'], ['init'], ['unauth'], ['call', 'amz_product_selection', '--help']]) {
    await execute(command, { env, fetchImpl: () => assert.fail('offline command') });
  }
  assert.equal((await execute(['unauth'])).upstream_revoked, false);
});

for (const status of [401, 403, 429, 500]) {
  test(`HTTP ${status} has no credential leakage or automatic retry`, async t => {
    const { fetchImpl, seen } = await fixture(t, { status });
    const value = await run(['call', 'amz_product_selection', '--json', '{}'], { env, fetchImpl });
    assert.equal(value.exitCode, 1);
    assert(!value.text.includes(TOKEN));
    assert.equal(JSON.parse(value.text).error.retryable, false);
    assert.equal(seen.length, 1);
  });
}

test('tool failure is non-retryable and is not reported as success', async t => {
  const { fetchImpl, seen } = await fixture(t, { toolError: true });
  const value = await run(['call', 'amz_sku_query', '--json', '{}'], { env, fetchImpl });
  assert.equal(JSON.parse(value.text).error.type, 'tool_error');
  assert.equal(seen.filter(value => value.method === 'tools/call').length, 1);
});

test('stateless upstream handshake still checks business authorization after HTTP 200', async t => {
  const { fetchImpl, seen } = await fixture(t, { stateless: true, authorizationError: true });
  const value = await run(['status'], { env, fetchImpl });
  assert.equal(value.exitCode, 1);
  assert.equal(JSON.parse(value.text).error.type, 'authorization_required');
  assert(!seen.some(value => value.method === 'DELETE'));
});

test('missing upstream capability fails closed without calling a tool', async t => {
  const { fetchImpl, seen } = await fixture(t, { missing: true });
  const value = await run(['call', 'amz_sku_query', '--json', '{}'], { env, fetchImpl });
  assert.equal(JSON.parse(value.text).error.type, 'upstream_unsupported');
  assert.equal(seen.filter(value => value.method === 'tools/call').length, 0);
});

test('a mismatched RPC ID is rejected', async t => {
  const { fetchImpl } = await fixture(t, { mismatch: true });
  const value = await run(['tools'], { env, fetchImpl });
  assert.equal(JSON.parse(value.text).error.type, 'protocol_error');
});

test('redirects and stalled responses fail without retry', async t => {
  for (const options of [{ redirect: true }, { hang: true }]) {
    const { fetchImpl, seen } = await fixture(t, options);
    const value = await run(['call', 'tt_expert_info_list', '--json', '{}'], { env, fetchImpl, timeoutMs: 100 });
    assert.equal(JSON.parse(value.text).error.type, 'transport_error');
    assert.equal(seen.length, 1);
  }
});

test('input and response limits fail closed', async t => {
  assert.throws(() => parseCommand(['call', 'amz_sku_query', '--json', JSON.stringify({ text: 'x'.repeat(2 * 1024 * 1024) })]));
  const oversized = await run(['call', 'amz_sku_query', '--stdin'], { env, stdin: Readable.from(['x'.repeat(2 * 1024 * 1024 + 1)]), fetchImpl: () => assert.fail('no network') });
  assert.equal(JSON.parse(oversized.text).error.type, 'invalid_request');
  const value = await run(['tools'], { env, fetchImpl: async () => new Response('x'.repeat(8 * 1024 * 1024 + 1), { headers: { 'Content-Type': 'application/json' } }) });
  assert.equal(JSON.parse(value.text).error.type, 'output_limit');
});

test('tool arguments cannot override the selected MCP name', async t => {
  const { fetchImpl, seen } = await fixture(t);
  await execute(['account', 'status', '--json', '{"name":"amz_sku_query","method":"tools/call"}'], { env, fetchImpl });
  assert.equal(seen.find(value => value.method === 'tools/call').params.name, 'account_info');
});
