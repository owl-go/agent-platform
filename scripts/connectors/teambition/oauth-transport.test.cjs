'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const http = require('node:http');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn } = require('node:child_process');
const { startOAuthTransport } = require('./oauth-transport.cjs');

async function fixture(handler) {
  const upstream = http.createServer(handler);
  await new Promise(resolve => upstream.listen(0, '127.0.0.1', resolve));
  const calls = [];
  const transport = await startOAuthTransport('real-oauth-canary', (options, done) => {
    calls.push(options);
    return http.request({ ...options, hostname: '127.0.0.1', port: upstream.address().port }, done);
  });
  return { calls, transport, close: async () => { await transport.close(); await new Promise(resolve => upstream.close(resolve)); upstream.closeAllConnections(); } };
}
function request(origin, options = {}, body = '{}') {
  return new Promise((resolve, reject) => {
    const req = http.request(origin + (options.path || '/api/mcp/v2'), { method: options.method || 'POST', headers: options.headers || {} }, res => {
      let data = ''; res.on('data', chunk => data += chunk); res.on('end', () => resolve({status: res.statusCode, headers: res.headers, body: data})); res.on('error', reject);
    });
    req.on('error', reject); req.end(body);
  });
}
async function runNative(transport, home, argv) {
  const child = spawn(process.env.TEAMBITION_TEST_NATIVE, argv, {env:{PATH:process.env.PATH, HOME:home, TEAMBITION_MCP_HOST:transport.origin, TEAMBITION_MCP_TOKEN:transport.routingToken, TEAMBITION_LEGACY_TOKEN_TRUSTED_ORIGINS:transport.origin}, stdio:['ignore','pipe','pipe']});
  let output = ''; child.stdout.on('data', chunk => output += chunk); child.stderr.on('data', chunk => output += chunk);
  const timeout = setTimeout(() => child.kill('SIGKILL'), 10000);
  const code = await new Promise(resolve => child.once('close', resolve)); clearTimeout(timeout);
  assert.equal(code, 0, output); assert.ok(!output.includes('real-oauth-canary'));
  return output;
}
test('loopback adapter replaces its routing nonce with OAuth and fixes upstream origin', async () => {
  const f = await fixture((req,res) => { req.resume(); res.writeHead(200, {'content-type':'application/json','mcp-session-id':'session-1','set-cookie':'never-forward'}); res.end('{"ok":true}'); });
  try {
    const result = await request(f.transport.origin, {headers:{authorization:'Bearer '+f.transport.routingToken, 'content-type':'application/json', cookie:'never-send', host:'attacker.test'}});
    assert.equal(result.status,200); assert.equal(result.headers['mcp-session-id'],'session-1'); assert.equal(result.headers['set-cookie'],undefined);
    assert.equal(f.calls.length,1); assert.equal(f.calls[0].hostname,'open.teambition.com'); assert.equal(f.calls[0].port,443); assert.equal(f.calls[0].path,'/api/mcp/v2');
    assert.equal(f.calls[0].headers.authorization,'Bearer real-oauth-canary'); assert.equal(f.calls[0].headers.cookie,undefined); assert.equal(f.calls[0].headers.host,undefined);
    assert.ok(!JSON.stringify(f.calls).includes(f.transport.routingToken));
  } finally { await f.close(); }
});
test('wrong nonce, methods and paths never reach the OAuth upstream', async () => {
  const f = await fixture((req,res)=>res.end());
  try {
    for (const options of [{headers:{authorization:'Bearer real-oauth-canary'}},{path:'/api/mcp/v2?host=attacker.test',headers:{authorization:'Bearer '+f.transport.routingToken}},{method:'PUT',headers:{authorization:'Bearer '+f.transport.routingToken}}]) {
      assert.equal((await request(f.transport.origin,options)).status,403);
    }
    assert.equal(f.calls.length,0);
  } finally { await f.close(); }
});
test('MCP2 method and name survive OAuth replacement while unrelated headers remain blocked', async () => {
  const f = await fixture((req,res) => { req.resume(); res.end('{}'); });
  try {
    for (const [method, name] of [['tools/list', undefined], ['resources/read', 'teambition://docs'], ['tools/call', 'teambition.docs.get']]) {
      const headers = {authorization:'Bearer '+f.transport.routingToken, 'mcp-method':method, 'mcp-protocol-version':'2026-07-28', 'x-unreviewed-header':'blocked'};
      if (name) headers['mcp-name'] = name;
      assert.equal((await request(f.transport.origin, {headers})).status, 200);
      const forwarded = f.calls.at(-1).headers;
      assert.equal(forwarded['mcp-method'], method); assert.equal(forwarded['mcp-name'], name);
      assert.equal(forwarded['mcp-protocol-version'], '2026-07-28');
      assert.equal(forwarded['x-unreviewed-header'], undefined);
    }
  } finally { await f.close(); }
});
test('redirects and transport errors cannot expose tokens or move credentials to another origin', async () => {
  const f = await fixture((req,res)=>{res.writeHead(302,{location:'https://attacker.test','set-cookie':'secret'});res.end();});
  try {
    const result = await request(f.transport.origin,{headers:{authorization:'Bearer '+f.transport.routingToken}});
    assert.equal(result.status,302);assert.equal(result.headers.location,undefined);assert.equal(f.calls.length,1);assert.equal(result.body,'');
  } finally { await f.close(); }
  const failing = await fixture((req,res)=>req.socket.destroy());
  try { const result=await request(failing.transport.origin,{headers:{authorization:'Bearer '+failing.transport.routingToken}});assert.equal(result.status,502);assert.ok(!result.body.includes('canary')); }
  finally { await failing.close(); }
});
test('closing transport cancels active requests and releases the listener', async () => {
  let arrived;
  const arrival = new Promise(resolve => arrived=resolve);
  const f=await fixture(req => {req.resume();arrived();});
  const pending=request(f.transport.origin,{headers:{authorization:'Bearer '+f.transport.routingToken}}).catch(()=>null);
  await arrival; await f.transport.close(); await pending;
  await assert.rejects(request(f.transport.origin));
  await f.close();
});
test('pinned native CLI discovers and calls tools through the OAuth adapter without a keyring', {skip: !process.env.TEAMBITION_TEST_NATIVE}, async () => {
  const methods = [];
  const f=await fixture((req,res)=>{
    let body='';req.on('data',chunk=>body+=chunk);req.on('end',()=>{
      const message=JSON.parse(body);res.setHeader('content-type','application/json');
      if (!req.headers['mcp-method']) {
        res.end(JSON.stringify({jsonrpc:'2.0',id:message.id,error:{code:-32020,message:'缺少必需的 Mcp-Method 请求头。'}}));return;
      }
      assert.equal(req.headers['mcp-method'], message.method);
      methods.push(message.method);
      if (message.method === 'resources/read') assert.equal(req.headers['mcp-name'], message.params.uri);
      if (message.method === 'tools/call') assert.equal(req.headers['mcp-name'], message.params.name);
      if(message.method==='notifications/initialized'){res.writeHead(202).end();return;}
      const result=message.method==='initialize'?{protocolVersion:'2024-11-05',capabilities:{tools:{}},serverInfo:{name:'test-fixture',version:'1'}}:message.method==='tools/call'?{content:[{type:'text',text:'{"verified":true}'}],isError:false}:{tools:[]};
      res.end(JSON.stringify({jsonrpc:'2.0',id:message.id,result}));
    });
  });
  const home = fs.mkdtempSync(path.join(os.tmpdir(), 'teambition-protocol-test-'));
  try {
    for (const argv of [['tools','list','--json'], ['tools','call','teambition.docs.get','--arguments-json','{}','--json']]) {
      await runNative(f.transport, home, argv); assert.ok(f.calls.length>=2);
    }
    assert.ok(methods.includes('tools/list')); assert.ok(methods.includes('resources/read')); assert.ok(methods.includes('tools/call'));
  } finally { await f.close(); fs.rmSync(home, {recursive: true, force: true}); }
});

test('pinned native MCP2 preserves task comment text and activity arguments through OAuth', {skip: !process.env.TEAMBITION_TEST_NATIVE}, async () => {
  const calls = [];
  const f = await fixture((req,res) => {
    let body='';req.on('data',chunk=>body+=chunk);req.on('end',()=>{
      const message=JSON.parse(body);
      assert.equal(message.method,'tools/call');assert.equal(req.headers['mcp-method'],message.method);
      assert.equal(req.headers['mcp-name'],message.params.name);
      calls.push(message.params);
      res.setHeader('content-type','application/json');
      res.end(JSON.stringify({jsonrpc:'2.0',id:message.id,result:{content:[{type:'text',text:'{"id":"fixture-comment"}'}],isError:false}}));
    });
  });
  const home = fs.mkdtempSync(path.join(os.tmpdir(),'teambition-comment-test-'));
  const comment = {taskId:'fixture-task',content:'首页布局已完成，“明天完成”保留原文。'};
  try {
    // Raw tools calls are fixture-only; the package exposes public task argv.
    await runNative(f.transport,home,['tools','call','teambition.task.comment','--arguments-json',JSON.stringify(comment),'--json']);
    await runNative(f.transport,home,['tools','call','teambition.task.activity','--arguments-json','{"taskId":"fixture-task"}','--json']);
    assert.deepEqual(calls.map(({name,arguments:args})=>({name,arguments:args})),[{name:'teambition.task.comment',arguments:comment},{name:'teambition.task.activity',arguments:{taskId:'fixture-task'}}]);
  } finally { await f.close();fs.rmSync(home,{recursive:true,force:true}); }
});
