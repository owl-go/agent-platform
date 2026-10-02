const test=require('node:test');const assert=require('node:assert/strict');const https=require('node:https');const http=require('node:http');const net=require('node:net');const fs=require('node:fs');const os=require('node:os');const path=require('node:path');const {spawn,spawnSync}=require('node:child_process');const {restoreKeychain,restoreManagedKeychain}=require('./launcher.cjs');
test('pinned native CLI uses reviewed login, polling, renewal and business protocol',{skip:!process.env.CAMSCANNER_TEST_NATIVE},async()=>{
 const home=fs.mkdtempSync(path.join(os.tmpdir(),'cs-protocol-'));let proxy,server;const sockets=new Set();
 try{
  const cert=path.join(home,'cert.pem'),key=path.join(home,'key.pem');
  const generated=spawnSync('openssl',['req','-x509','-newkey','rsa:2048','-nodes','-days','1','-subj','/CN=ai-tools.camscanner.com','-addext','subjectAltName=DNS:ai-tools.camscanner.com','-keyout',key,'-out',cert],{stdio:'ignore'});assert.equal(generated.status,0);
  const expiry=Math.floor(Date.now()/1000)+172800;const requests=[];
  server=https.createServer({key:fs.readFileSync(key),cert:fs.readFileSync(cert)},async(req,res)=>{
   const chunks=[];for await(const b of req)chunks.push(b);const body=Buffer.concat(chunks).toString();requests.push({method:req.method,url:req.url,body,headers:req.headers});
   if(req.headers['x-is-agent']!=='general'||!req.headers['user-agent']?.startsWith('camscanner-cli/')){res.writeHead(400).end();return;}
   res.setHeader('Content-Type','application/json');
   if(req.url==='/auth/user/code')res.end(JSON.stringify({code:'synthetic-code',expires_in:300}));
   else if(req.url==='/auth/user/status?code=synthetic-code')res.end(JSON.stringify({status:'authorized',token:'synthetic-token',user_id:'synthetic-user',is_domestic:'1',expires_at:expiry}));
   else if(req.url==='/auth/user/refresh')res.end(JSON.stringify({token:'renewed-token',expires_at:expiry}));
   else if(req.url==='/v1/tools/query_cloud_dir/execute?channel=camscanner-cli'){if(!['Bearer synthetic-token','Bearer renewed-token'].includes(req.headers.authorization)){res.writeHead(401).end();return;}res.end(JSON.stringify({code:200,tool:'query_cloud_dir',tool_result:{success:true,data:{dirs:[]}},traceId:'fixture'}));}
   else {console.log('Unhandled native route:',req.url);res.writeHead(404).end();}
  });await new Promise(r=>server.listen(0,'127.0.0.1',r));
  proxy=http.createServer();proxy.on('connection',s=>{sockets.add(s);s.on('close',()=>sockets.delete(s));});proxy.on('connect',(req,socket,head)=>{
   if(req.url!=='ai-tools.camscanner.com:443'){socket.destroy();return;}
   const upstream=net.connect(server.address().port,'127.0.0.1',()=>{socket.write('HTTP/1.1 200 Connection Established\r\n\r\n');if(head.length)upstream.write(head);socket.pipe(upstream);upstream.pipe(socket);});upstream.on('error',()=>socket.destroy());socket.on('error',()=>upstream.destroy());socket.on('close',()=>upstream.destroy());
  });await new Promise(r=>proxy.listen(0,'127.0.0.1',r));
  const env={PATH:process.env.PATH,HOME:home,XDG_DATA_HOME:path.join(home,'data'),SSL_CERT_FILE:cert,HTTPS_PROXY:'http://127.0.0.1:'+proxy.address().port};
  const run=async args=>await new Promise((resolve,reject)=>{const child=spawn(process.env.CAMSCANNER_TEST_NATIVE,args,{env});let stdout='',stderr='';child.stdout.on('data',b=>stdout+=b);child.stderr.on('data',b=>stderr+=b);const timer=setTimeout(()=>child.kill('SIGKILL'),15000);child.once('error',reject);child.once('close',code=>{clearTimeout(timer);if(code!==0){reject(new Error(args.join(" ")+": "+stderr));return;}resolve(stdout);});});
  const code=JSON.parse(await run(['auth','login-code','--json']));assert.equal(code.code,'synthetic-code');const url=new URL(code.login_url);const from=new URL(url.searchParams.get('from'));assert.equal(url.origin+url.pathname,'https://www.camscanner.com/agent-auth');assert.equal(from.origin+from.pathname,'https://ai-tools.camscanner.com/auth/user/callback');assert.equal(from.searchParams.get('code'),'synthetic-code');
  await run(['auth','login-poll','--code','synthetic-code','--timeout','5','--json']);
  assert.equal(JSON.parse(await run(['auth','status','--json'])).authenticated,true);
  restoreKeychain(home,{token:'synthetic-token',user_id:'synthetic-user',is_domestic:'1',expires_at:expiry});await run(['doc','dirs','--json']);
  restoreKeychain(home,{token:'synthetic-token',user_id:'synthetic-user',is_domestic:'1',expires_at:Math.floor(Date.now()/1000)+30});await run(['doc','dirs','--json']);
  restoreManagedKeychain(home,{token:'synthetic-token',user_id:'synthetic-user',is_domestic:'1',expires_at:Math.floor(Date.now()/1000)+3600});await run(['doc','dirs','--json']);
  assert.equal(requests.filter(r=>r.url==='/auth/user/refresh').length,1);
  assert.deepEqual(JSON.parse(requests.find(r=>r.url==='/auth/user/code').body),{client_id:'camscanner-cli'});
  assert.deepEqual(JSON.parse(requests.find(r=>r.url==='/auth/user/refresh').body),{token:'synthetic-token'});
  const business=requests.filter(r=>r.url==='/v1/tools/query_cloud_dir/execute?channel=camscanner-cli');assert.equal(business.length,3);assert.equal(business[0].headers.authorization,'Bearer synthetic-token');assert.equal(business[1].headers.authorization,'Bearer renewed-token');assert.equal(business[2].headers.authorization,'Bearer synthetic-token');assert.deepEqual(JSON.parse(business[0].body),{});
 }finally{for(const s of sockets)s.destroy();if(proxy){proxy.closeAllConnections();proxy.close();}if(server){server.closeAllConnections();server.close();}fs.rmSync(home,{recursive:true,force:true});}
});
