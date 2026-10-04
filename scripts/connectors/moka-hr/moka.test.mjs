import { test } from 'node:test';
import assert from 'node:assert/strict';
import { Readable } from 'node:stream';
import { credentials, execute, OPERATIONS, requestSpec, run } from './moka.mjs';

const env = { CONNECTOR_CREDENTIALS_JSON: JSON.stringify({ moka_api_key: 'fixture-key', moka_org_id: 'fixtureOrg' }) };
const response = body => new Response(JSON.stringify(body), { headers: { 'Content-Type': 'application/json; charset=utf-8' } });
const query = (command, args, fetchImpl) => execute([...command.split(' '), '--json', JSON.stringify(args)], { env, fetchImpl });
const cases = [
  ['jobs list', { mode: 'social', keyword: '工程师' }, '/api-platform/v1/jobs/fixtureOrg', { jobs: [], total: 0 }],
  ['jobs get', { jobId: 'job-1' }, '/api-platform/v1/jobs/fixtureOrg/job-1', { title: 'Engineer' }],
  ['jobs fields', { jobId: 'job-1' }, '/api-platform/v1/jobs/fixtureOrg/job-1', { customFields: [{ id: 1 }] }],
  ['candidates search', { stage: 'all', limit: 20, next: 'opaque+/cursor=' }, '/api-platform/v1/data/ehrApplications', { code: 0, data: [], next: 'next-token' }],
  ['candidates get', { applicationId: '1,2' }, '/api-platform/v1/data/ehrApplications', { code: 0, data: [] }],
  ['candidates stage', { applicationId: '1' }, '/api-platform/v1/data/ehrApplications', { data: [{ applicationId: 1, candidateId: 2, stageName: 'Offer', email: 'private' }] }],
  ['applications list', { candidateId: 2 }, '/api-platform/candidate/v1/getApplicationStates', { code: 0, data: [] }],
  ['pipelines list', {}, '/api-platform/v2/pipelines/getPipelinesList', { code: 200, data: [] }],
  ['stages list', {}, '/api-platform/v2/stage/getStagesList', { code: 200, data: [] }],
  ['departments list', {}, '/api-platform/v1/departments', { success: true, departments: [] }],
  ['offers fields', {}, '/api-platform/v1/offers/custom_fields', { social: [], campus: [] }],
  ['pools list', {}, '/api-platform/v1/talentPool/list', []],
  ['pools candidates', { talentPoolIds: [1, 2], archivedAtStart: '2026-09-01', archivedAtEnd: '2026-10-01' }, '/api-platform/v1/talentPool/candidates', { data: { candidates: [] } }],
  ['interviews list', { startDate: '2026-10-01T00:00:00Z', endDate: '2026-10-08T00:00:00Z', hireMode: 1 }, '/api-platform/v1/interviews', { data: [] }],
];

test('every reviewed operation sends the documented route, Basic Auth and bounded request', async () => {
  assert.deepEqual(cases.map(item => item[0]), Object.keys(OPERATIONS));
  for (const [command, args, path, body] of cases) {
    const value = await query(command, args, async (url, options) => {
      const parsed = new URL(url);
      assert.equal(parsed.origin, 'https://api.mokahr.com');
      assert.equal(parsed.pathname, path);
      assert.equal(options.headers.Authorization, 'Basic ' + Buffer.from('fixture-key:').toString('base64'));
      assert.equal(options.redirect, 'error');
      assert.ok(options.signal instanceof AbortSignal);
      if (command === 'applications list') {
        assert.equal(options.method, 'POST');
        assert.equal(options.headers['Content-Type'], 'application/json');
        assert.deepEqual(JSON.parse(options.body), args);
      } else {
        assert.equal(options.method, 'GET');
        assert.equal(options.body, undefined);
        for (const [name, value] of Object.entries(args)) if (name !== 'jobId') assert.equal(parsed.searchParams.get(name), Array.isArray(value) ? JSON.stringify(value) : String(value));
      }
      return response(body);
    });
    if (command === 'candidates stage') assert.deepEqual(value, { data: [{ applicationId: 1, candidateId: 2, stageName: 'Offer' }] });
    else if (command === 'jobs fields') assert.deepEqual(value, { jobId: 'job-1', customFields: body.customFields });
    else assert.deepEqual(value, body);
  }
});

test('capability and parameter bypass attempts never reach the network', async () => {
  for (const argv of [
    ['raw', '--url', 'https://evil.test'], ['jobs', 'create'], ['jobs', 'get', '--json', '{"jobId":"../departments"}'],
    ['jobs', 'list', '--json', '{"mode":"social","orgId":"other"}'], ['jobs', 'list', '--json', '{"mode":"social","url":"https://evil.test"}'],
    ['departments', 'list', '--json', '{"Authorization":"stolen"}'], ['candidates', 'search', '--json', '{"stage":"all","limit":21}'],
    ['candidates', 'get', '--json', '{"applicationId":"1/../jobs"}'], ['applications', 'list', '--json', '{"candidateId":9007199254740992}'],
    ['pools', 'candidates', '--json', '{"talentPoolIds":["1"],"archivedAtStart":"2026-01-01","archivedAtEnd":"2026-02-01"}'],
    ['stages', 'list', '--json', '{"pipelineId":1}'], ['jobs', 'list', '--json', '[]'], ['jobs', 'get', '--json', '{"jobId":"x"}', '--token', 'x'],
  ]) {
    let called = false;
    const result = await run(argv, { env, fetchImpl: async () => { called = true; throw Error(); } });
    assert.equal(result.exitCode, 1);
    assert.equal(called, false);
  }
});

test('enterprise credential validation and platform precedence fail closed', async () => {
  for (const value of ['not-json', '[]', '{"moka_api_key":"key:password"}', '{"moka_api_key":"key\n"}', '{"moka_api_key":"key","moka_org_id":"../"}', '{"moka_api_key":1}', '{"url":"https://evil.test"}']) {
    assert.throws(() => credentials({ CONNECTOR_CREDENTIALS_JSON: value }));
  }
  assert.deepEqual(credentials({ CONNECTOR_CREDENTIALS_JSON: '{}', MOKA_API_KEY: 'other' }), { key: undefined, orgId: undefined });
  assert.deepEqual(await execute(['status'], { env: {} }), { authenticated: false, next_action: 'connect' });
  assert.equal(JSON.parse((await run(['departments', 'list'], { env: {} })).text).error.type, 'authorization_required');
  assert.equal(JSON.parse((await run(['jobs', 'list', '--json', '{"mode":"social"}'], { env: { MOKA_API_KEY: 'key' } })).text).error.type, 'configuration_required');
});

test('auth checks actual authorized response and never returns enterprise records', async () => {
  const result = await execute(['status'], { env, fetchImpl: async () => response({ departments: [{ name: 'Private' }] }) });
  assert.deepEqual(result, { authenticated: true, identity: 'enterprise_api_key', organization_configured: true });
  await assert.rejects(execute(['auth'], { env, fetchImpl: async () => response({ data: [] }) }), /unexpected response/);
  assert.equal((await execute(['unauth'], { env })).upstream_revoked, false);
});

test('date constraints reject absent, partial, reversed, invalid and excessive interview ranges', () => {
  for (const args of [{}, { startDate: '2026-01-01' }, { startDate: '2026-02-30', endDate: '2026-03-01' },
    { startDate: '2026-01-01', endDate: '2026-02-02' }, { startDate: '2026-01-02', endDate: '2026-01-01' },
    { createStartDate: '2026-01-01', createEndDate: '2026-01-02', hireMode: 3 }]) assert.throws(() => requestSpec('interviews list', args, 'org'));
  assert.ok(requestSpec('interviews list', { createStartDate: '2026-01-01', createEndDate: '2026-01-31' }, 'org'));
});

test('single-page pagination preserves opaque cursor without following upstream URLs', async () => {
  let calls = 0;
  const result = await query('candidates search', { stage: 'offer' }, async url => {
    calls++;
    assert.equal(new URL(url).searchParams.get('limit'), '20');
    return response({ data: [], next: 'https://evil.test/next' });
  });
  assert.equal(calls, 1);
  assert.equal(result.next, 'https://evil.test/next');
});

test('PII masking and raw/base64 credential reflection redaction work recursively', async () => {
  const result = await run(['departments', 'list'], { env, fetchImpl: async () => response({
    departments: [{ phone: '13812345678', citizenId: '123456789012345678', nested: { mobile: '123', name: 'fixture-key' } }],
    echoed: env.CONNECTOR_CREDENTIALS_JSON, auth: 'Basic ' + Buffer.from('fixture-key:').toString('base64'),
  }) });
  assert.equal(result.exitCode, 0);
  assert.ok(!result.text.includes('fixture-key'));
  assert.ok(!result.text.includes(Buffer.from('fixture-key:').toString('base64')));
  const body = JSON.parse(result.text);
  assert.equal(body.data.departments[0].phone, '138****5678');
  assert.equal(body.data.departments[0].nested.mobile, '[MASKED]');
});

test('HTTP and business errors remain errors, including stage HTTP 500', async () => {
  for (const [status, type, retryable] of [[401, 'authorization_required', false], [403, 'permission_denied', false], [404, 'not_found', false], [429, 'upstream_error', true], [500, 'upstream_error', true]]) {
    const result = JSON.parse((await run(['candidates', 'search', '--json', '{"stage":"all"}'], { env, fetchImpl: async () => new Response('fixture-key', { status }) })).text);
    assert.equal(result.ok, false); assert.equal(result.error.type, type); assert.equal(result.error.retryable, retryable);
  }
  for (const body of [{ code: 101, data: [] }, { code: 101, success: true }, { success: false, code: 200 }]) {
    const result = JSON.parse((await run(['departments', 'list'], { env, fetchImpl: async () => response(body) })).text);
    assert.equal(result.error.type, 'upstream_error');
  }
});

test('non-JSON, malformed JSON, interrupted, oversized and transport responses fail safely', async () => {
  for (const fetchImpl of [async () => new Response('<html>login</html>', { headers: { 'Content-Type': 'text/html' } }),
    async () => new Response('{', { headers: { 'Content-Type': 'application/json' } }),
    async () => response(null), async () => { throw Error('fixture-key'); },
    async () => new Response(' '.repeat(8 * 1024 * 1024 + 1), { headers: { 'Content-Type': 'application/json' } }),
    async () => new Response(new ReadableStream({ start(controller) { controller.error(Error('fixture-key')); } }), { headers: { 'Content-Type': 'application/json' } }),
  ]) {
    const result = await run(['departments', 'list'], { env, fetchImpl });
    assert.equal(result.exitCode, 1); assert.ok(!result.text.includes('fixture-key'));
  }
});

test('stdin is bounded and parsed before making the documented call', async () => {
  const value = await execute(['applications', 'list', '--stdin'], { env, stdin: Readable.from(['{"candidate', 'Id":1}']), fetchImpl: async (_, options) => {
    assert.deepEqual(JSON.parse(options.body), { candidateId: 1 }); return response({ code: 0, data: [] });
  } });
  assert.deepEqual(value.data, []);
  await assert.rejects(execute(['departments', 'list', '--stdin'], { env, stdin: Readable.from(['x'.repeat(65537)]) }), /64 KiB/);
});

test('diagnostics and lifecycle do not need credentials or network', async () => {
  for (const argv of [['--help'], ['init'], ['version'], ['tools'], ['schema', 'jobs', 'list'], ['jobs', 'list', '--help'], ['unauth']]) {
    assert.equal((await run(argv, { env: {}, fetchImpl: async () => { throw Error('network forbidden'); } })).exitCode, 0);
  }
});
