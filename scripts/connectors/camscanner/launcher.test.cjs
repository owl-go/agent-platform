const test=require('node:test');const assert=require('node:assert/strict');const fs=require('node:fs');const os=require('node:os');const path=require('node:path');const crypto=require('node:crypto');const {spawnSync}=require('node:child_process');
const {credential,validateArguments,restoreKeychain}=require('./launcher.cjs');
const good={access_token:'synthetic-token',user_id:'test-owner',is_domestic:'1',access_expires_at:new Date(Date.now()+3600000).toISOString()};
test('credentials reject expiry, extra secrets and header injection',()=>{
 assert.equal(credential({CONNECTOR_CREDENTIALS_JSON:JSON.stringify(good)}).user_id,'test-owner');
 for(const v of [{...good,refresh_token:'must-stay-platform'},{...good,access_token:'token\nheader'},{...good,access_expires_at:'2000-01-01'},{...good,access_expires_at:new Date(Date.now()+120000).toISOString()},{}])assert.throws(()=>credential({CONNECTOR_CREDENTIALS_JSON:JSON.stringify(v)}));
});
test('reviewed policy rejects login, arbitrary help and diagnostics with business arguments',()=>{
 for(const args of [['auth','login'],['completion','bash'],['help','auth','status'],['--help','doc','move'],['info','image','ocr'],['doc','delete'],['help','image','ocr','--save'],['doc','search','x\n']])assert.throws(()=>validateArguments(args));
 assert.equal(validateArguments(['doc','search','合同']).risk,'low');assert.equal(validateArguments(['image','ocr','scan.jpg']).risk,'high');assert.equal(validateArguments(['help','image','ocr']).risk,'low');
});
test('native AES-GCM credential store uses owner-only files and no plaintext token',()=>{
 const home=fs.mkdtempSync(path.join(os.tmpdir(),'cs-keychain-test-'));
 try{const grant=credential({CONNECTOR_CREDENTIALS_JSON:JSON.stringify(good)});restoreKeychain(home,grant);const directory=path.join(home,'data','camscanner-cli');const filename=path.join(directory,Buffer.from('camscanner-cli:default').toString('hex')+'.enc');const key=fs.readFileSync(path.join(directory,'master.key'));const encrypted=fs.readFileSync(filename);assert.equal(encrypted.includes(Buffer.from(good.access_token)),false);assert.equal(fs.statSync(filename).mode&0o777,0o600);const d=crypto.createDecipheriv('aes-256-gcm',key,encrypted.subarray(0,12));d.setAuthTag(encrypted.subarray(-16));assert.deepEqual(JSON.parse(Buffer.concat([d.update(encrypted.subarray(12,-16)),d.final()]).toString()),grant);
 }finally{fs.rmSync(home,{recursive:true,force:true});}
});
test('pinned native Linux binary reads the temporary encrypted credential store',{skip:!process.env.CAMSCANNER_TEST_NATIVE},()=>{
 const home=fs.mkdtempSync(path.join(os.tmpdir(),'cs-native-test-'));
 try{restoreKeychain(home,credential({CONNECTOR_CREDENTIALS_JSON:JSON.stringify(good)}));const p=spawnSync(process.env.CAMSCANNER_TEST_NATIVE,['auth','status','--json'],{env:{HOME:home,XDG_DATA_HOME:path.join(home,'data')},encoding:'utf8'});assert.equal(p.status,0,p.stderr);assert.equal(JSON.parse(p.stdout).authenticated,true);}finally{fs.rmSync(home,{recursive:true,force:true});}
});
