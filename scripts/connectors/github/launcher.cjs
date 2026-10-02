#!/usr/bin/env node
'use strict';
const {spawnSync}=require('node:child_process');
const {mkdtempSync,rmSync,readFileSync}=require('node:fs');
const {tmpdir}=require('node:os');
const {join}=require('node:path');
const VERSION='2.102.0';
const policy=JSON.parse(readFileSync(join(__dirname,'capabilities.json'),'utf8'));
function command(argv){
 if(argv.length===1 && ['--help','--version'].includes(argv[0]))return {lifecycle:argv[0]};
 if(argv[0]==='platform' && argv.length===2 && ['init','auth','status','unauth'].includes(argv[1]))return {lifecycle:argv[1]};
 const capability=policy.find(c=>c.argv_prefix.every((v,i)=>argv[i]===v));
 if(!capability)throw Error('Unreviewed command. Read the bundled coverage and command reference.');
 const args=argv.slice(capability.argv_prefix.length);const positionals=[];let repository;
 for(let i=0;i<args.length;i++){
  const arg=args[i];if(typeof arg!=='string'||arg.includes('\0'))throw Error('Invalid argument');
  if(!arg.startsWith('-')){positionals.push(arg);continue;}
  const split=arg.indexOf('=');const flag=split<0?arg:arg.slice(0,split);let value=split<0?undefined:arg.slice(split+1);
  const arity=capability.options[flag];if(arity===undefined)throw Error('Unreviewed option: '+flag);
  if(arity){if(value===undefined)value=args[++i];if(value===undefined)throw Error('Missing option value');}
  else if(value!==undefined)throw Error('Boolean options use their flag without a value');
  if(capability.stdin_flags.includes(flag)&&value!=='-')throw Error('File input must use stdin (-)');
  if(flag==='--url' && !/^https:\/\/github\.com\/[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+\/(issues|pull)\/\d+$/.test(value))throw Error('Project item URL must be a github.com issue or pull request');
  if(['--repo','-R'].includes(flag)){if(!/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(value))throw Error('Repository must be OWNER/REPO on github.com');repository=value;}
 }
 if(capability.repo_required&&!repository)throw Error('Use --repo OWNER/REPO; local Git context is not used');
 if(positionals.some(v=>/^https?:/i.test(v)&&!/^https:\/\/(?:github\.com|gist\.github\.com)\//.test(v)))throw Error('Only GitHub.com URLs are allowed');
 if(['repo view','repo archive','repo unarchive','repo edit'].includes(capability.argv_prefix.join(' ')) && (positionals.length!==1 || !/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(positionals[0])))throw Error('Use an explicit OWNER/REPO');
 if(capability.argv_prefix.join(' ')==='label clone' && (positionals.length!==1 || !/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(positionals[0])))throw Error('Label source must be OWNER/REPO');
 if(capability.argv_prefix.join(' ')==='release create' && positionals.length!==1)throw Error('Use one release tag without asset files');
 return {capability};
}
function credential(env){
 if(!env.CONNECTOR_CREDENTIALS_JSON)return undefined;
 let fields;try{fields=JSON.parse(env.CONNECTOR_CREDENTIALS_JSON);}catch{throw Error('Invalid platform credential');}
 if(!fields || Array.isArray(fields)||Object.keys(fields).length!==1||typeof fields.access_token!=='string'||!/^[A-Za-z0-9_]{1,32768}$/.test(fields.access_token))throw Error('Invalid platform access credential');
 return fields.access_token;
}
function main(argv=process.argv.slice(2),env=process.env){
 const parsed=command(argv);const token=credential(env);
 if(parsed.lifecycle && !['--help','--version'].includes(parsed.lifecycle)){
  if(parsed.lifecycle==='auth'){console.log(JSON.stringify({ok:true,authorization_mode:'browser_device',next_action:'Connect through the platform and enter the displayed code on GitHub'}));return;}
  console.log(JSON.stringify({ok:true,configured:parsed.lifecycle==='status'&&!!token,verification:'credential_shape_only'}));return;
 }
 const arch={x64:'amd64',arm64:'arm64'}[process.arch];if(process.platform!=='linux'||!arch)throw Error('This immutable bundle requires Linux amd64 or arm64');
 if(!parsed.lifecycle&&!token)throw Error('Connect your GitHub account through the platform first');
 const home=mkdtempSync(join(tmpdir(),'github-connector-'));
 const childEnv={PATH:'/usr/local/bin:/usr/bin:/bin',HOME:home,TMPDIR:home,GH_CONFIG_DIR:home,GH_HOST:'github.com',GH_PROMPT_DISABLED:'1',GH_PAGER:'cat',PAGER:'cat',GH_EDITOR:'/bin/false',VISUAL:'/bin/false',EDITOR:'/bin/false',GH_BROWSER:'/bin/false',BROWSER:'/bin/false',GH_NO_UPDATE_NOTIFIER:'1',GH_NO_EXTENSION_UPDATE_NOTIFIER:'1',GH_ACCESSIBLE_PROMPTER:'disabled',DO_NOT_TRACK:'1',GIT_CONFIG_GLOBAL:'/dev/null',GIT_CONFIG_NOSYSTEM:'1'};
 if(token)childEnv.GH_TOKEN=token;
 try{
  const result=spawnSync(join(__dirname,'gh-linux-'+arch),argv,{shell:false,cwd:home,env:childEnv,stdio:'inherit',timeout:120000});
  if(result.error)throw Error('GitHub CLI process failed');process.exitCode=result.status??1;
 }finally{rmSync(home,{recursive:true,force:true});}
}
module.exports={command,credential,main};
if(require.main===module){try{main();}catch(error){console.error(JSON.stringify({ok:false,error:{type:'invalid_request',message:error.message,retryable:false,next_action:'read_connector_skill'}}));process.exitCode=1;}}
