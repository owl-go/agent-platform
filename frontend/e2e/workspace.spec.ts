import { test, expect, request, type BrowserContext, type Page, type APIRequestContext } from '@playwright/test';
import { readFileSync } from 'node:fs';
import { randomUUID, createHash } from 'node:crypto';
let admin: BrowserContext, user: BrowserContext, adminAPI: APIRequestContext, userAPI: APIRequestContext;
const baseURL = process.env.E2E_BASE_URL!;
const prefix = `e2e-${Date.now()}`;
const expertInput = (name: string) => ({ name, icon: 'sparkles', icon_background: 'sage', introduction: 'E2E expert', core_capability: 'Analyze input', operating_procedure: 'Read, analyze, report', output_standard: 'Plain text report', cautions: '', mcp_server_ids: [], skill_ids: [], cli_connector_definition_ids: [] });
const workflowInput = (name: string) => ({ name, goal: 'Validate current workspace', environment: [] });
async function signIn(page: Page, username: string, password: string) {
  await page.goto('/');
  await page.getByRole('button', { name: /^(登录|Sign in)/ }).click();
  await page.locator('#username').fill(username);
  await page.locator('#password').fill(password);
  await page.locator('#kc-login').click();
  await expect(page.locator('.new-session')).toBeVisible();
  expect(new URL(page.url()).search).toBe('');
}
async function apiFor(page: Page) {
  const token = await page.evaluate(() => {
    const key = Object.keys(localStorage).find(k => k.startsWith('oidc.user:'));
    return key ? JSON.parse(localStorage.getItem(key)!).access_token : '';
  });
  if (!token) throw new Error('OIDC callback did not store an access token');
  return request.newContext({ baseURL, extraHTTPHeaders: { Authorization: `Bearer ${token}` } });
}
async function ok(api: APIRequestContext, method: string, path: string, data?: unknown) {
  const response = await api.fetch(`/api/v1${path}`, { method, data });
  expect(response.status(), `${method} ${path}`).toBeGreaterThanOrEqual(200);
  expect(response.status(), `${method} ${path}`).toBeLessThan(300);
  return response.status() === 204 ? {} : response.json();
}
async function newExpert(api = userAPI) { return ok(api, 'POST', '/experts', { expert: expertInput(`${prefix}-${randomUUID()}`) }); }
async function newWorkflow(api = userAPI) { return ok(api, 'POST', '/workflows', { workflow: workflowInput(`${prefix}-${randomUUID()}`) }); }

test.beforeAll(async ({ browser }) => {
  if (!baseURL || !process.env.E2E_PASSWORD) throw new Error('Use node scripts/e2e/run-local.mjs to provision an isolated environment');
  admin = await browser.newContext({ baseURL, locale: 'zh-CN' });
  const a = await admin.newPage();
  await signIn(a, 'platform-admin', process.env.E2E_PASSWORD!);
  adminAPI = await apiFor(a);
  const created = await ok(adminAPI, 'POST', '/admin/users', { username: `${prefix}-user`, email: `${prefix}@example.test`, display_name: 'E2E User' });
  expect(created.temporary_password).toBeTruthy();
  // The product creates a temporary password; fixture setup assigns a permanent test password.
  const identity = process.env.E2E_IDENTITY!, realm = process.env.E2E_REALM!;
  const tokenResponse = await fetch(`${identity}/realms/${realm}/protocol/openid-connect/token`, { method: 'POST', body: new URLSearchParams({ grant_type: 'client_credentials', client_id: 'agent-workspace-admin', client_secret: process.env.E2E_CLIENT_SECRET! }) });
  if (!tokenResponse.ok) throw new Error(`Identity fixture authentication failed: ${tokenResponse.status}`);
  const token = (await tokenResponse.json()).access_token;
  const headers = { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };
  const identities = await (await fetch(`${identity}/admin/realms/${realm}/users?username=${prefix}-user&exact=true`, { headers })).json();
  const passwordResponse = await fetch(`${identity}/admin/realms/${realm}/users/${identities[0].id}/reset-password`, { method: 'PUT', headers, body: JSON.stringify({ type: 'password', value: process.env.E2E_PASSWORD, temporary: false }) });
  if (!passwordResponse.ok) throw new Error(`Identity fixture password setup failed: ${passwordResponse.status}`);
  const updateResponse = await fetch(`${identity}/admin/realms/${realm}/users/${identities[0].id}`, { method: 'PUT', headers, body: JSON.stringify({ requiredActions: [] }) });
  if (!updateResponse.ok) throw new Error(`Identity fixture update failed: ${updateResponse.status}`);
  user = await browser.newContext({ baseURL, locale: 'zh-CN' });
  const u = await user.newPage();
  await signIn(u, `${prefix}-user`, process.env.E2E_PASSWORD!);
  userAPI = await apiFor(u);
  await a.close(); await u.close();
});
test.afterAll(async () => { await adminAPI?.dispose(); await userAPI?.dispose(); await admin?.close(); await user?.close(); });

test('E2E-001 | ACC | Browser: OIDC login persists across reload', async () => {
  const page = await user.newPage(); await page.goto('/sessions'); await page.reload();
  await expect(page.locator('.new-session')).toBeVisible();
  expect(new URL(page.url()).searchParams.has('code')).toBe(false); await page.close();
});
test('E2E-002 | ACC | Anonymous protected API is rejected', async ({ request }) => {
  expect((await request.get('/api/v1/sessions')).status()).toBe(401);
});
test('E2E-003 | ACC | Invalid bearer token is rejected', async ({ request }) => {
  expect((await request.get('/api/v1/sessions', { headers: { Authorization: 'Bearer invalid' } })).status()).toBe(401);
});
test('E2E-004 | ACC | Ordinary user cannot administer accounts', async () => {
  expect((await userAPI.get('/api/v1/admin/users')).status()).toBe(403);
  expect((await userAPI.post('/api/v1/admin/users', { data: { username: 'forbidden' } })).status()).toBe(403);
});
test('E2E-005 | ACC | Duplicate username is rejected', async () => {
  expect((await adminAPI.post('/api/v1/admin/users', { data: { username: `${prefix}-user`, email: 'duplicate@example.test', display_name: 'Duplicate' } })).status()).toBe(412);
});
test('E2E-006 | SES | Session create, rename, archive, restore and delete persist', async () => {
  let item = await ok(userAPI, 'POST', '/sessions', {});
  item = await ok(userAPI, 'PATCH', `/sessions/${item.id}`, { title: `${prefix}-session`, expected_version: item.version });
  expect(((await ok(userAPI, 'GET', '/sessions')).items ?? []).some((s: any) => s.title === item.title)).toBe(true);
  item = await ok(userAPI, 'PATCH', `/sessions/${item.id}/archived`, { archived: true, expected_version: item.version });
  expect((await ok(userAPI, 'GET', '/sessions?archived=true')).items.some((s: any) => s.id === item.id)).toBe(true);
  item = await ok(userAPI, 'PATCH', `/sessions/${item.id}/archived`, { archived: false, expected_version: item.version });
  await ok(userAPI, 'DELETE', `/sessions/${item.id}`);
  expect(((await ok(userAPI, 'GET', '/sessions')).items ?? []).some((s: any) => s.id === item.id)).toBe(false);
});
test('E2E-007 | SES | Stale session version is rejected', async () => {
  const item = await ok(userAPI, 'POST', '/sessions', {});
  await ok(userAPI, 'PATCH', `/sessions/${item.id}`, { title: 'fresh', expected_version: item.version });
  expect((await userAPI.patch(`/api/v1/sessions/${item.id}`, { data: { title: 'stale', expected_version: item.version } })).status()).toBe(412);
});
test('E2E-008 | SEC | Session owner isolation includes Administrator', async () => {
  const item = await ok(userAPI, 'POST', '/sessions', {});
  expect(((await ok(adminAPI, 'GET', '/sessions')).items ?? []).some((s: any) => s.id === item.id)).toBe(false);
  expect((await adminAPI.get(`/api/v1/sessions/${item.id}/messages`)).status()).toBe(404);
  expect((await adminAPI.delete(`/api/v1/sessions/${item.id}`)).status()).toBe(404);
});
test('E2E-009 | EXP | Expert CRUD and structured fields persist', async () => {
  let item = await newExpert(); expect(item.complete).toBe(true);
  const input = expertInput(`${prefix}-edited`);
  item = await ok(userAPI, 'PATCH', `/experts/${item.id}`, { expert: input, expected_version: item.version });
  expect((await ok(userAPI, 'GET', `/experts/${item.id}`)).core_capability).toBe(input.core_capability);
  await ok(userAPI, 'DELETE', `/experts/${item.id}`);
  expect((await userAPI.get(`/api/v1/experts/${item.id}`)).status()).toBe(404);
});
test('E2E-010 | SEC | Expert private data cannot be read or changed by another owner', async () => {
  const item = await newExpert();
  expect((await adminAPI.get(`/api/v1/experts/${item.id}`)).status()).toBe(404);
  expect((await adminAPI.delete(`/api/v1/experts/${item.id}`)).status()).toBe(404);
});
test('E2E-011 | EXP | Empty expert name is rejected', async () => {
  expect((await userAPI.post('/api/v1/experts', { data: { expert: expertInput('') } })).status()).toBe(422);
});
test('E2E-012 | TEAM | Ordered team members persist', async () => {
  const one = await newExpert(), two = await newExpert();
  const team = await ok(userAPI, 'POST', '/expert-teams', { expert_team: { name: `${prefix}-team`, icon: 'sparkles', icon_background: 'sage', introduction: 'Test team', core_capability: 'Review', members: [{ id: randomUUID(), name: 'Author', expert_id: one.id, labels: [] }, { id: randomUUID(), name: 'Reviewer', expert_id: two.id, labels: [] }] } });
  const saved = await ok(userAPI, 'GET', `/expert-teams/${team.id}`);
  expect(saved.members.map((m: any) => m.expert.id)).toEqual([one.id, two.id]);
  await ok(userAPI, 'DELETE', `/expert-teams/${team.id}`);
});
test('E2E-013 | WF | Workflow create/update/delete persist', async () => {
  let item = await newWorkflow();
  item = await ok(userAPI, 'PATCH', `/workflows/${item.id}`, { workflow: workflowInput(`${prefix}-changed`), expected_version: item.version });
  expect((await ok(userAPI, 'GET', `/workflows/${item.id}`)).name).toBe(`${prefix}-changed`);
  await ok(userAPI, 'DELETE', `/workflows/${item.id}`);
  expect(((await ok(userAPI, 'GET', '/workflows')).items ?? []).some((w: any) => w.id === item.id)).toBe(false);
});
test('E2E-014 | SEC | Workflow owner isolation', async () => {
  const item = await newWorkflow();
  expect((await adminAPI.get(`/api/v1/workflows/${item.id}`)).status()).toBe(404);
  expect((await adminAPI.get(`/api/v1/workflows/${item.id}/workspace`)).status()).toBe(404);
});
test('E2E-015 | WF | Secret environment value is omitted from reads', async () => {
  const secret = `private-${randomUUID()}`;
  const item = await ok(userAPI, 'POST', '/workflows', { workflow: { ...workflowInput(`${prefix}-secret`), environment: [{ name: 'E2E_SECRET', value: secret, secret: true }] } });
  const saved = await ok(userAPI, 'GET', `/workflows/${item.id}`);
  expect(JSON.stringify(saved).includes(secret)).toBe(false);
  expect(saved.environment[0].configured).toBe(true);
});
test('E2E-016 | WSP | Empty workspace is bounded; traversal is rejected', async () => {
  const item = await newWorkflow();
  const workspace = await ok(userAPI, 'GET', `/workflows/${item.id}/workspace`);
  expect(workspace.items ?? []).toHaveLength(0);
  expect((await userAPI.get(`/api/v1/workflows/${item.id}/workspace/file?path=..%2F..%2Fetc%2Fpasswd`)).status()).toBe(422);
});
test('E2E-017 | SET | Settings persist and stale writes fail', async () => {
  const settings = await ok(userAPI, 'GET', '/settings');
  const { version, ...values } = settings;
  const input = { ...values, personality: 'custom', personality_instructions: 'E2E test preference', expected_version: version };
  await ok(userAPI, 'PATCH', '/settings', input);
  expect((await ok(userAPI, 'GET', '/settings')).personality_instructions).toBe(input.personality_instructions);
  expect((await userAPI.patch('/api/v1/settings', { data: input })).status()).toBe(412);
});
test('E2E-018 | SET | Ordinary user cannot create global model connection', async () => {
  expect((await userAPI.post('/api/v1/model-provider-connections', { data: { name: 'forbidden', provider_type: 'openai', api_key: 'test' } })).status()).toBe(403);
});
test('E2E-019 | RUN | Unavailable Runtime is reported honestly', async () => {
  const items = (await ok(userAPI, 'GET', '/runtime-engines')).items;
  expect(items).toHaveLength(5); expect(items.every((r: any) => !r.available && !r.native_resume)).toBe(true);
});
test('E2E-020 | FIL | Upload, hash and signed download round trip', async () => {
  const content = Buffer.from('End-to-end attachment test\n');
  const response = await userAPI.post('/api/v1/attachments/upload?name=e2e.txt', { data: content, headers: { 'Content-Type': 'text/plain', 'Idempotency-Key': randomUUID() } });
  expect(response.status()).toBeLessThan(300);
  const item = await response.json(); expect(item.sha256).toBe(createHash('sha256').update(content).digest('hex'));
  const download = await userAPI.get(`/api/v1/attachments/${item.id}/download`);
  expect(download.ok()).toBe(true); expect(await download.body()).toEqual(content);
  expect((await adminAPI.get(`/api/v1/attachments/${item.id}/download`)).status()).toBe(404);
});
test('E2E-021 | CRD | Balance and ledger are readable; admin endpoints denied', async () => {
  const balance = await ok(userAPI, 'GET', '/credits/balance'); expect(balance.total_hundredths).toBeGreaterThanOrEqual(0);
  await ok(userAPI, 'GET', '/credits/ledger?limit=50');
  expect((await userAPI.post('/api/v1/admin/redemption-code-batches', { data: { count: 1, value_hundredths: 100 } })).status()).toBe(403);
});
test('E2E-022 | CRD | Redemption applies exactly once', async () => {
  const batch = await ok(adminAPI, 'POST', '/admin/redemption-code-batches', { count: 1, value_hundredths: 1234 });
  const before = await ok(userAPI, 'GET', '/credits/balance');
  const after = await ok(userAPI, 'POST', '/credits/redemptions', { code: batch.codes[0].plaintext });
  expect(after.total_hundredths - before.total_hundredths).toBe(1234);
  expect((await userAPI.post('/api/v1/credits/redemptions', { data: { code: batch.codes[0].plaintext } })).status()).toBe(422);
  expect((await ok(userAPI, 'GET', '/credits/balance')).total_hundredths).toBe(after.total_hundredths);
});
test('E2E-023 | CLI | Ordinary user cannot manage CLI definitions', async () => {
  expect((await userAPI.post('/api/v1/admin/connectors/cli', { data: { definition: {} } })).status()).toBe(403);
  expect((await userAPI.get('/api/v1/admin/connectors/cli-health')).status()).toBe(403);
});
test('E2E-024 | APR | Fresh account has no command approvals', async () => {
  expect((await ok(userAPI, 'GET', '/command-approvals')).items ?? []).toHaveLength(0);
});
for (const [index, path, heading] of [[25, '/workflows', '工作流'], [26, '/experts', '专家'], [27, '/resources', '技能·连接器'], [28, '/settings', '设置']] as const) {
  test(`E2E-0${index} | UI | Browser page ${path} loads with real API`, async () => {
    const page = await user.newPage(); const errors: string[] = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.goto(path);
    if (path === '/experts') await expect(page.getByRole('tab', { name: '专家', exact: true })).toBeVisible();
    else await expect(page.locator('h1').first()).toContainText(heading);
    await expect(page.locator('.new-session')).toBeVisible(); expect(errors).toEqual([]); await page.close();
  });
}
test('E2E-029 | EXP | Browser creates an expert and sees it after reload', async () => {
  const page = await user.newPage(); await page.goto('/experts/new');
  await expect(page.locator('.editor-form')).toBeVisible();
  await page.locator('.editor-form input').first().fill(`${prefix}-browser-expert`);
  const text = page.locator('.editor-form textarea');
  for (const [i, value] of ['Browser introduction', 'Analyze requirements', 'Read and report', 'Markdown output'].entries()) await text.nth(i).fill(value);
  await page.getByRole('button', { name: '保存', exact: true }).click(); await expect(page).toHaveURL(/\/experts$/);
  await page.reload(); await expect(page.getByText(`${prefix}-browser-expert`, { exact: true })).toBeVisible(); await page.close();
});
test('E2E-030 | UI | Mobile navigation and no horizontal overflow', async () => {
  const page = await user.newPage(); await page.setViewportSize({ width: 390, height: 844 }); await page.goto('/sessions');
  await expect(page.locator('.mobile-header')).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.screenshot({ path: '../output/playwright/mobile-sessions.png', fullPage: true }); await page.close();
});
for (const [id, feature] of [['031', 'Runtime real model execution and SSE terminal event'], ['032', 'Scheduled Worker trigger and concurrent run locking'], ['033', 'CLI real authorization, sandbox command approval and recovery'], ['034', 'Linux gVisor sandbox isolation and production conformance']] as const) {
  test(`E2E-${id} | BLOCKED | ${feature}`, async () => { test.skip(true, 'Local macOS fixture has no Linux runsc Worker, verified Runtime RepoDigest or real provider credentials.'); });
}

test('E2E-035 | WF | Browser creates Workflow and reopens persisted detail', async () => {
  const page = await user.newPage(); await page.goto('/workflows');
  await page.locator('.page-header').getByRole('button', { name: /新建工作流/ }).click();
  const dialog = page.getByRole('dialog');
  await dialog.locator('input').first().fill(`${prefix}-browser-workflow`);
  await dialog.locator('textarea').fill('Generate a concise project report');
  await dialog.getByRole('button', { name: '新建工作流', exact: true }).click();
  await expect(page).toHaveURL(/\/workflows\/[^/]+$/); await page.reload();
  await expect(page.locator('h1')).toContainText(`${prefix}-browser-workflow`); await page.close();
});
test('E2E-036 | SES | Browser new session is created before sending a message', async () => {
  const before = ((await ok(userAPI, 'GET', '/sessions')).items ?? []).length;
  const page = await user.newPage(); await page.goto('/sessions');
  await page.locator('.new-session').click();
  await expect.poll(async () => ((await ok(userAPI, 'GET', '/sessions')).items ?? []).length).toBe(before + 1);
  await expect(page.locator('.composer-editor')).toBeVisible(); await page.close();
});
test('E2E-037 | SKL | ZIP Skill upload, document read, private ownership and deletion', async () => {
  const archive = readFileSync(new URL('./fixtures/skill.zip', import.meta.url)).toString('base64');
  const item = await ok(userAPI, 'POST', '/skills', { name: `${prefix}-skill`, source: 'upload', archive });
  expect(item.sha256).toMatch(/^[a-f0-9]{64}$/);
  const document = await ok(userAPI, 'GET', `/skills/${item.id}/document`);
  expect(document.content).toContain('Read input and provide a concise report.');
  expect((await adminAPI.get(`/api/v1/skills/${item.id}/document`)).status()).toBe(404);
  const impact = await ok(userAPI, 'GET', `/skills/${item.id}/deletion-impact`);
  await ok(userAPI, 'DELETE', `/skills/${item.id}?confirmation_token=${encodeURIComponent(impact.confirmation_token)}`);
  expect((await userAPI.get(`/api/v1/skills/${item.id}/document`)).status()).toBe(404);
});
test('E2E-038 | SKL | ZIP traversal is rejected', async () => {
  const archive = readFileSync(new URL('./fixtures/unsafe-skill.zip', import.meta.url)).toString('base64');
  expect((await userAPI.post('/api/v1/skills', { data: { name: `${prefix}-unsafe`, source: 'upload', archive } })).status()).toBe(422);
});
test('E2E-039 | MCP | MCP configuration persists and deletion requires impact token', async () => {
  const item = await ok(userAPI, 'POST', '/connectors/mcp', { mcp_connector: { name: `${prefix}-mcp`, transport: 'streamable_http', url: 'https://example.test/mcp', arguments: [], environment: [] } });
  expect(item.tested ?? false).toBe(false);
  expect((await ok(userAPI, 'GET', '/connectors/mcp')).items.some((m: any) => m.id === item.id)).toBe(true);
  const impact = await ok(userAPI, 'GET', `/connectors/mcp/${item.id}/deletion-impact`);
  await ok(userAPI, 'DELETE', `/connectors/mcp/${item.id}?confirmation_token=${encodeURIComponent(impact.confirmation_token)}`);
});
test('E2E-040 | SEL | Conversation draft selections remain isolated by Session', async () => {
  const a = await ok(userAPI, 'POST', '/sessions', {}), b = await ok(userAPI, 'POST', '/sessions', {}), expert = await newExpert();
  const selected = await ok(userAPI, 'POST', '/conversation-selection', { session_id: a.id, change_expert: true, expert_id: expert.id, skill_ids: [], mcp_server_ids: [], cli_connector_ids: [], disabled_connectors: [] });
  expect(selected.expert_id).toBe(expert.id);
  const other = await ok(userAPI, 'GET', `/conversation-selection?session_id=${b.id}`);
  expect(other.expert_id ?? '').toBe('');
});
test('E2E-041 | CRD | Voided redemption code cannot change balance', async () => {
  const batch = await ok(adminAPI, 'POST', '/admin/redemption-code-batches', { count: 1, value_hundredths: 500 });
  await ok(adminAPI, 'POST', `/admin/redemption-codes/${batch.codes[0].id}/void`, {});
  const before = await ok(userAPI, 'GET', '/credits/balance');
  expect((await userAPI.post('/api/v1/credits/redemptions', { data: { code: batch.codes[0].plaintext } })).status()).toBe(422);
  expect((await ok(userAPI, 'GET', '/credits/balance')).total_hundredths).toBe(before.total_hundredths);
});
test('E2E-042 | OPS | Development proxy preserves business API and health routes', async ({ request }) => {
  expect((await request.get('/api/healthz')).status()).toBe(200);
  expect((await userAPI.get('/api/v1/me')).status()).toBe(200);
  expect((await request.get('/api/readyz')).status()).toBe(200);
});

test('E2E-043 | MCP | Browser creates MCP configuration', async () => {
  const page = await user.newPage(); await page.goto('/resources');
  await page.locator('.subtabs button').nth(1).click();
  await page.getByRole('button', { name: '＋ MCP', exact: true }).click();
  const form = page.locator('.modal-card');
  await form.getByLabel('名称', { exact: true }).fill(`${prefix}-browser-mcp`);
  await form.getByLabel('URL', { exact: true }).fill('https://example.test/mcp');
  const response = page.waitForResponse(r => r.url().endsWith('/api/v1/connectors/mcp') && r.request().method() === 'POST');
  await form.getByRole('button', { name: '保存', exact: true }).click();
  expect((await response).status()).toBe(200);
  await expect(page.getByText(`${prefix}-browser-mcp`, { exact: true })).toBeVisible(); await page.close();
});
test('E2E-044 | TEAM | Browser reopens and saves an existing Expert Team', async () => {
  const one = await newExpert(), two = await newExpert();
  const team = await ok(userAPI, 'POST', '/expert-teams', { expert_team: { name: `${prefix}-browser-team`, icon: 'users', icon_background: 'sage', introduction: 'Browser team', core_capability: 'Review', members: [{ id: randomUUID(), name: 'Author', expert_id: one.id, labels: [] }, { id: randomUUID(), name: 'Reviewer', expert_id: two.id, labels: [] }] } });
  const page = await user.newPage(); await page.goto(`/expert-teams/${team.id}`);
  await expect(page.locator('.ordered-members li')).toHaveCount(2);
  const response = page.waitForResponse(r => r.url().endsWith(`/api/v1/expert-teams/${team.id}`) && r.request().method() === 'PATCH');
  await page.getByRole('button', { name: '保存', exact: true }).click();
  expect((await response).status()).toBe(200);
  await expect(page).toHaveURL(/\/experts\?tab=teams/); await page.close();
});
