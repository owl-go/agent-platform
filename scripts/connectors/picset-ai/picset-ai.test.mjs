import test from 'node:test';
import assert from 'node:assert/strict';
import { Readable } from 'node:stream';
import { OPERATIONS, BASE, credential, parseCommand, validateArguments, execute, run } from './picset-ai.mjs';
const key = 'sk_live_fixture_secret', env = {CONNECTOR_CREDENTIALS_JSON:JSON.stringify({picset_api_key:key})};
const id = '550e8400-e29b-41d4-a716-446655440000';
function sample(operation) {
  const spec=OPERATIONS[operation], args={};
  for (const field of spec.required) {
    if (field==='request_id') args[field]=id;
    else if (field==='scene') args[field]='product_detail';
    else if (field==='oss_path') args[field]='temp/user/agent-uploads/product_detail/asset';
    else if (field==='rectangles') args[field]=[[0,0,999,999]];
    else if (spec.image_scenes[field]) { const v={oss_path:`temp/user/agent-uploads/${spec.image_scenes[field]}/asset`}; args[field]=field.endsWith('images')?[v]:v; }
    else args[field]='fixture prompt';
  }
  return args;
}
test('credentials reject JWTs, unknown fields, malformed JSON and line breaks without fallback',()=>{
  assert.equal(credential(env),key);
  for (const value of ['jwt','sk_live_x\n','sk_live_','sk_live_x space']) assert.throws(()=>credential({PICSET_API_KEY:value}));
  for (const value of ['{',JSON.stringify({picset_api_key:key,other:'x'})]) assert.throws(()=>credential({CONNECTOR_CREDENTIALS_JSON:value,PICSET_API_KEY:key}));
});
test('all 15 operations use official paths, Bearer only, reviewed fields and paid idempotency',async()=>{
  for (const operation of Object.keys(OPERATIONS)) {
    const args=sample(operation), paid=OPERATIONS[operation].paid;
    let calls=0;
    const fetchImpl=async(url,options)=>{
      calls++;
      assert.equal(url,BASE+(operation==='request'?`requests/${id}`:operation));
      assert.equal(options.method,OPERATIONS[operation].method);
      assert.equal(options.headers.Authorization,`Bearer ${key}`);
      assert.equal(options.redirect,'error');
      assert.equal(options.headers['Idempotency-Key'],paid?id:undefined);
      assert.equal(options.headers.apikey,undefined);
      if (options.method==='POST') {assert.equal(options.headers['Content-Type'],'application/json');assert.deepEqual(JSON.parse(options.body),args);}
      else assert.equal(options.body,undefined);
      return Response.json(paid?{request_id:id,idempotency_key:id,status:'generating'}:operation==='request'?{request_id:id,status:'completed'}:{success:true},{status:paid?202:200});
    };
    await execute([operation,'--json',JSON.stringify(args),...(paid?['--idempotency-key',id]:[])],{env,fetchImpl});
    assert.equal(calls,1);
  }
});
test('rejects arbitrary endpoints, credential flags, missing saved key and unsafe schemas before network',async()=>{
  for (const argv of [['raw','--json','{}'],['canvas-image','--json','{}'],['canvas-image','--json','{}','--idempotency-key','unsafe key'],['request','--json','{}','--key',key]]) assert.throws(()=>parseCommand(argv));
  for (const args of [{...sample('canvas-image'),api_key:key},{...sample('canvas-image'),reference_images:[{url:'https://evil.example'}]},{...sample('canvas-image'),reference_images:[{oss_path:'temp/user/agent-uploads/image_layer/a'}]},{...sample('canvas-image'),prompt:'x'.repeat(4001)}]) assert.throws(()=>validateArguments('canvas-image',args));
  assert.throws(()=>validateArguments('request',{request_id:'../../evil'}));
  assert.throws(()=>validateArguments('image-layer-2-0-region',{...sample('image-layer-2-0-region'),rectangles:[[5,0,4,999]]}));
  assert.throws(()=>validateArguments('canvas-image',{prompt:'x',model:'nova-img-2-vip',gpt_quality:'high'}));
  assert.throws(()=>validateArguments('image-audit',{scene:'product_main',oss_path:'temp/user/agent-uploads/product_detail/a'}));
});
test('status declares local configuration, diagnostics do not access network or spend credits',async()=>{
  const fetchImpl=()=>{throw new Error('unexpected network');};
  assert.deepEqual(await execute(['status'],{env,fetchImpl}),{authenticated:true,upstream_verified:false,next_action:'query_owned_request'});
  assert.equal((await execute(['status'],{env:{},fetchImpl})).authenticated,false);
  await execute(['schema','product-detail'],{fetchImpl});
  await execute(['--help'],{fetchImpl});
});
test('HTTP denials map stable error codes and never echo provider message secrets',async()=>{
  for (const [status,code,retryable] of [[401,'UNAUTHORIZED',false],[409,'IDEMPOTENCY_CONFLICT',false],[429,'RATE_LIMITED',true],[503,'ASSET_VALIDATION_UNAVAILABLE',true]]) {
    const result=await run(['request','--json',JSON.stringify({request_id:id})],{env,fetchImpl:async()=>Response.json({error:code,message:key},{status})});
    assert.equal(result.exitCode,1); const data=JSON.parse(result.text);assert.equal(data.error.type,code);assert.equal(data.error.retryable,retryable);assert.ok(!result.text.includes(key));
  }
});
test('response interruption and redirects never retry a paid request automatically',async()=>{
  let calls=0;
  const argv=['canvas-image','--json','{"prompt":"fixture"}','--idempotency-key',id];
  const result=await run(argv,{env,fetchImpl:async()=>{calls++;throw new Error('redirect or timeout '+key);}});
  assert.equal(calls,1);assert.equal(JSON.parse(result.text).error.next_action,'retry_same_request');assert.ok(!result.text.includes(key));
  const wrong=await run(argv,{env,fetchImpl:async()=>Response.json({request_id:id,idempotency_key:'wrong'},{status:202})});
  assert.equal(JSON.parse(wrong.text).error.type,'protocol_error');
});
test('bounds stdin/response and redacts exact credential bytes from success results',async()=>{
  const response=await run(['request','--stdin'],{env,stdin:Readable.from([JSON.stringify({request_id:id})]),fetchImpl:async()=>Response.json({request_id:id,status:'failed',error:{code:'UPSTREAM_FAILURE',message:key}})});
  assert.ok(response.text.includes('[REDACTED]'));assert.ok(!response.text.includes(key));
  const large=await run(['request','--stdin'],{env,stdin:Readable.from(['x'.repeat(65537)])});assert.equal(JSON.parse(large.text).error.type,'invalid_request');
  const oversized=await run(['request','--json',JSON.stringify({request_id:id})],{env,fetchImpl:async()=>Response.json({text:'x'.repeat(8*1024*1024)})});assert.equal(JSON.parse(oversized.text).error.type,'output_limit');
});
