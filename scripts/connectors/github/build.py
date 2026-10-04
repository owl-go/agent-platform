#!/usr/bin/env python3
"""Build the pinned GitHub CLI bundle and package; Conformance is not claimed here."""
import argparse
import hashlib
import io
import json
import re
import subprocess
import tarfile
import tempfile
import zipfile
from pathlib import Path
ROOT = Path(__file__).resolve().parent
VERSION = '0.1.0'
CLI_VERSION = '2.102.0'
ASSETS = {'amd64':'bb766f710eef8ede859c18578c72c327597cd4c8a85b06001b1f3843c6019386',
          'arm64':'7862c86c72f43df3a2d93ddde6f473285b4e2af61b494849846827e513ef6484'}
MANUAL_SHA256 = 'a4b2f15473ae7fb6d26dbfd8f3beb0a4f28a590180b8b09f4808cab1b84cdea4'
HOSTS = ['api.github.com','github.com']
def sha(body): return hashlib.sha256(body).hexdigest()
def encoded(value): return (json.dumps(value,ensure_ascii=False,sort_keys=True,indent=2)+'\n').encode()
def zip_bytes(files):
    output=io.BytesIO()
    with zipfile.ZipFile(output,'w',zipfile.ZIP_DEFLATED) as z:
        for name,(body,mode) in sorted(files.items()):
            info=zipfile.ZipInfo(name,date_time=(1980,1,1,0,0,0));info.compress_type=zipfile.ZIP_DEFLATED;info.external_attr=(0o100000|mode)<<16;z.writestr(info,body)
    return output.getvalue()
def native(asset,arch):
    if sha(asset)!=ASSETS[arch]: raise ValueError('GitHub release asset checksum differs')
    with tarfile.open(fileobj=io.BytesIO(asset),mode='r:gz') as z:
        member=z.getmember(f'gh_{CLI_VERSION}_linux_{arch}/bin/gh')
        if not member.isfile() or member.size>50<<20: raise ValueError('invalid GitHub binary')
        body=z.extractfile(member).read()
    if body[:4]!=b'\x7fELF' or int.from_bytes(body[18:20],'little')!= {'amd64':62,'arm64':183}[arch]:raise ValueError('wrong ELF architecture')
    return body

def build(assets,image,node):
    if not re.fullmatch(r'[^\s@]+@sha256:[a-f0-9]{64}',image) or not re.fullmatch(r'\d+\.\d+\.\d+',node):raise ValueError('require Registry RepoDigest and exact Node version')
    reviewed=json.loads((ROOT/'capabilities.json').read_text())
    prefixes=[tuple(c['argv_prefix']) for c in reviewed]
    if len(prefixes)!=len(set(prefixes)) or any(a!=b and b[:len(a)]==a for a in prefixes for b in prefixes):raise ValueError('overlapping command policy')
    capabilities=[{k:v for k,v in c.items() if k not in ['options','repo_required','stdin_flags']} for c in reviewed]
    capabilities += [{'id':'gh_help' if flag=='--help' else 'gh_version','argv_prefix':[flag],'risk':'low','identities':['user'],'scopes':[],'egress_hosts':HOSTS,'timeout_seconds':30} for flag in ['--help','--version']]
    metadata={'name':'@agent-platform/github-connector','version':VERSION,'bin':{'gh':'launcher.cjs'},'agentWorkspace':{'executable':'gh','authenticationDriver':'connector_package','supportedArchitectures':['linux-amd64','linux-arm64'],'capabilities':[{'id':c['id'],'argvPrefix':c['argv_prefix'],'risk':c['risk'],'identities':c['identities'],'scopes':c['scopes'],'egressHosts':c['egress_hosts'],'timeoutSeconds':c['timeout_seconds']} for c in capabilities]}}
    source=zip_bytes({'package.json':(encoded(metadata),0o644),'launcher.cjs':((ROOT/'launcher.cjs').read_bytes(),0o755),'capabilities.json':(encoded(reviewed),0o644),'LICENSE':((ROOT/'LICENSE').read_bytes(),0o644),**{f'gh-linux-{arch}':(native(asset,arch),0o755) for arch,asset in assets.items()}})
    with tempfile.TemporaryDirectory(prefix='github-connector-build-') as directory:
        directory=Path(directory);(directory/'source.zip').write_bytes(source)
        subprocess.run(['go','-C',str(ROOT.parents[2]/'backend'),'run','./cmd/cli-connector-bundle',str(directory/'source.zip'),str(directory/'bundle.tgz')],check=True)
        bundle=(directory/'bundle.tgz').read_bytes()
    files={'cli-bundle.tgz':(bundle,0o644),'icon.svg':((ROOT/'icon.svg').read_bytes(),0o644),'skills/github/SKILL.md':((ROOT/'SKILL.md').read_bytes(),0o644),'skills/github/references/coverage.md':((ROOT/'coverage.md').read_bytes(),0o644),'skills/github/references/capabilities.json':(encoded(reviewed),0o644)}
    manual=(ROOT/'manual-v2.102.0.tar.gz').read_bytes()
    if sha(manual)!=MANUAL_SHA256:raise ValueError('manual snapshot checksum differs')
    with tarfile.open(fileobj=io.BytesIO(manual),mode='r:gz') as z:
        for m in z:
            if not m.isfile() or not re.fullmatch(r'[A-Za-z0-9_.-]+',m.name):raise ValueError('unsafe manual reference')
            files['skills/github/references/manual/'+m.name]=(z.extractfile(m).read(),0o644)
    files['connector-meta.json']=(encoded({'source':'github','version':VERSION,'type':'cli','name':'GitHub','description':'GitHub 仓库、Issue、PR 与评论、Actions、Release、Projects 和搜索；通过浏览器设备授权连接。','examples_zh':['查询指定仓库的 Issue','在指定 Issue 发表评论'],'examples_en':['List issues in a selected repository','Comment on a selected issue'],'minPlatformVersion':'1.0.0','auth_mode':'oauth'}),0o644)
    files['cli.json']=(encoded({'runtime':{'kind':'node','version':node,'digest':image.split('@')[1]},'executable':'gh','bundle_path':'node_modules/@agent-platform/github-connector/launcher.cjs','authentication_driver':'connector_package','commands':{name:{'argv':['gh','platform',value],'timeout_seconds':30} for name,value in [('init','init'),('auth','auth'),('status','status'),('unAuth','unauth')]},'status_match':{'json_path':'$.configured','equals':True},'capabilities':capabilities,'activation_scopes':['repo','read:org','gist','project','workflow'],'auth_url_domains':['github.com'],'egress_hosts':HOSTS,'timeout_seconds':120,'resource_limits':{'cpu_millis':1000,'memory_mib':512,'timeout_seconds':120,'concurrency':1,'child_processes':32}}),0o644)
    return zip_bytes(files),source,bundle

def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--assets-directory',type=Path,required=True);p.add_argument('--runtime-image',required=True);p.add_argument('--runtime-version',required=True);p.add_argument('--output',type=Path,required=True);args=p.parse_args()
    package,source,bundle=build({arch:(args.assets_directory/f'gh_{CLI_VERSION}_linux_{arch}.tar.gz').read_bytes() for arch in ASSETS},args.runtime_image,args.runtime_version)
    args.output.parent.mkdir(parents=True,exist_ok=True);args.output.write_bytes(package);args.output.with_suffix('.source.zip').write_bytes(source)
    print(json.dumps({'package':str(args.output),'package_sha256':sha(package),'bundle_sha256':sha(bundle),'conformance':'not_run'}))
if __name__=='__main__':main()
