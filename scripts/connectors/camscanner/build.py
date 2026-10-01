#!/usr/bin/env python3
"""Build a pinned CamScanner CLI bundle and Connector ZIP; does not claim Conformance."""
import argparse
import base64
import hashlib
import io
import json
import re
import subprocess
import tempfile
import tarfile
import zipfile
from pathlib import Path
ROOT=Path(__file__).parent
VERSION='1.1.8'
INTEGRITIES={
 'x64':'ahXLKpHTTAE7ORzaxk/2H3IL7CS7Zg2YXfbpEVjucfdLbg+pwNH8XFyyi29ob7zMOSLCiqVQSbY4XnbqG82U5A==',
 'arm64':'Sltrazp7p/TPlRZ3hlKqga2GINOAIOWKYUPbKhi5LCrGschTl1KZ6f8BJVj9xZ5jmbpCtOQxaxtZlulSGpm4FQ==',
}
SKILL_SHA256='f15c1889b2c0baded18da909c6dde139bc57865b709cefe2a55cbc9f8c07e182'
HOSTS=['ai-tools.camscanner.com']
def sha256(b):return hashlib.sha256(b).hexdigest()
def json_bytes(v):return (json.dumps(v,ensure_ascii=False,sort_keys=True,indent=2)+'\n').encode()
def capabilities():
 reviewed=json.loads((ROOT/'capabilities.json').read_text())
 prefixes=[tuple(c['argv_prefix']) for c in reviewed]
 if len(prefixes)!=len(set(prefixes)) or any(a!=b and b[:len(a)]==a for a in prefixes for b in prefixes):raise ValueError('overlapping policy')
 return [{**c,'id':'cs_'+sha256(' '.join(c['argv_prefix']).encode())[:16],'identities':['user'],'scopes':[],'egress_hosts':HOSTS,'timeout_seconds':120} for c in reviewed]
def native(package,arch):
 if base64.b64encode(hashlib.sha512(package).digest()).decode()!=INTEGRITIES[arch]:raise ValueError('native npm integrity differs from pinned 1.1.8')
 with tarfile.open(fileobj=io.BytesIO(package),mode='r:gz') as z:
  members=z.getmembers()
  if {m.name for m in members} != {'package/package.json','package/bin/camscanner-cli'} or any(not m.isfile() for m in members):raise ValueError('unexpected native npm content')
  meta=json.load(z.extractfile('package/package.json'))
  if meta['name']!='@camscanner-cli/linux-'+arch or meta['version']!=VERSION:raise ValueError('native package identity differs')
  b=z.extractfile('package/bin/camscanner-cli').read()
 if not b.startswith(b'\x7fELF') or int.from_bytes(b[18:20],'little')!=(62 if arch=='x64' else 183):raise ValueError('wrong ELF architecture')
 return b

def archive(files):
 output=io.BytesIO()
 with zipfile.ZipFile(output,'w',zipfile.ZIP_DEFLATED) as z:
  for name,(body,mode) in sorted(files.items()):
   info=zipfile.ZipInfo(name,date_time=(1980,1,1,0,0,0));info.compress_type=zipfile.ZIP_DEFLATED;info.external_attr=(0o100000|mode)<<16;z.writestr(info,body)
 return output.getvalue()
def build(packages,skill,image,node_version):
 match=re.fullmatch(r'[^\s@]+(?:/[^\s@]+)*@sha256:([a-f0-9]{64})',image)
 if not match or not re.fullmatch(r'\d+\.\d+\.\d+',node_version):raise ValueError('require Registry RepoDigest and exact Node version')
 if sha256(skill)!=SKILL_SHA256:raise ValueError('upstream Skill checksum differs')
 reviewed=capabilities()
 meta={'name':'@agent-platform/camscanner-connector','version':VERSION,'bin':{'camscanner-cli':'launcher.cjs'},'agentWorkspace':{'executable':'camscanner-cli','authenticationDriver':'connector_package','supportedArchitectures':['linux-amd64','linux-arm64'],'capabilities':[{'id':c['id'],'argvPrefix':c['argv_prefix'],'risk':c['risk'],'identities':c['identities'],'scopes':c['scopes'],'egressHosts':c['egress_hosts'],'timeoutSeconds':c['timeout_seconds']} for c in reviewed]}}
 source=archive({'launcher.cjs':((ROOT/'launcher.cjs').read_bytes(),0o755),'capabilities.json':(json_bytes(reviewed),0o644),'package.json':(json_bytes(meta),0o644),**{f'camscanner-linux-{arch}':(native(packages[arch],arch),0o755) for arch in INTEGRITIES}})
 with tempfile.TemporaryDirectory(prefix='camscanner-build-') as tmp:
  d=Path(tmp);(d/'source.zip').write_bytes(source)
  subprocess.run(['go','-C',str(ROOT.resolve().parents[2]/'backend'),'run','./cmd/cli-connector-bundle',str(d/'source.zip'),str(d/'bundle.tgz')],check=True)
  bundle=(d/'bundle.tgz').read_bytes()
 icon=(ROOT/'camscanner.png').read_bytes()
 if sha256(icon)!='95ac2cbe24e91e2be243648b857b16c09b91dc941dd17b10c46ac79db8d46fd3':raise ValueError('official icon checksum differs')
 files={'cli-bundle.tgz':(bundle,0o644),'skills/camscanner/SKILL.md':((ROOT/'SKILL.md').read_bytes(),0o644),'skills/camscanner/commands.md':((ROOT/'commands.md').read_bytes(),0o644),'skills/camscanner/capabilities.json':(json_bytes(reviewed),0o644),'icon.svg':(('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256"><image width="256" height="256" href="data:image/png;base64,'+base64.b64encode(icon).decode()+'"/></svg>').encode(),0o644)}
 with zipfile.ZipFile(io.BytesIO(skill)) as z:
  for name in z.namelist():
   if name.startswith('references/') and not name.endswith('/'):
    if '..' in Path(name).parts or '\\' in name:raise ValueError('unsafe reference')
    files['skills/camscanner/reference/'+name.removeprefix('references/')]=(z.read(name),0o644)
 files['connector-meta.json']=(json_bytes({'source':'camscanner','version':VERSION,'type':'cli','name':'扫描全能王','description':'图片增强、OCR、PDF 与 Office 格式转换、多图合并和云文档管理；浏览器授权连接个人账号。','examples_zh':['识别扫描件中的文字','将 PDF 转换为 Word'],'examples_en':['Recognize text in a scan','Convert a PDF to Word'],'minPlatformVersion':'1.0.0','auth_mode':'oauth'}),0o644)
 files['cli.json']=(json_bytes({'runtime':{'kind':'node','version':node_version,'digest':'sha256:'+match.group(1)},'executable':'camscanner-cli','bundle_path':'node_modules/.bin/camscanner-cli','authentication_driver':'connector_package','commands':{key:{'argv':['platform',v]} for key,v in [('init','init'),('auth','auth'),('status','status'),('unAuth','unauth')]},'status_match':{'json_path':'$.configured','equals':True},'capabilities':reviewed,'auth_url_domains':['www.camscanner.com','ai-tools.camscanner.com'],'egress_hosts':HOSTS,'timeout_seconds':120,'resource_limits':{'cpu_millis':1000,'memory_mib':512,'timeout_seconds':180,'concurrency':1,'child_processes':64}}),0o644)
 return archive(files),source,bundle

def main():
 p=argparse.ArgumentParser(description=__doc__)
 for arch in INTEGRITIES:p.add_argument('--'+arch+'-npm',type=Path,required=True)
 p.add_argument('--skill-zip',type=Path,required=True);p.add_argument('--runtime-image',required=True);p.add_argument('--runtime-version',required=True);p.add_argument('--output',type=Path,required=True);a=p.parse_args()
 package,source,bundle=build({arch:getattr(a,arch+'_npm').read_bytes() for arch in INTEGRITIES},a.skill_zip.read_bytes(),a.runtime_image,a.runtime_version)
 a.output.parent.mkdir(parents=True,exist_ok=True);a.output.write_bytes(package);a.output.with_suffix('.source.zip').write_bytes(source)
 print(json.dumps({'output':str(a.output),'sha256':sha256(package),'bundle_sha256':sha256(bundle),'capabilities':len(capabilities()),'conformance':'not_run'}))
if __name__=='__main__':main()
