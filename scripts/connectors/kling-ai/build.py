#!/usr/bin/env python3
"""Build and validate the reviewed Kling AI MCP Connector Package."""
import argparse
import json
from pathlib import Path
import subprocess
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parent
REPO = ROOT.parents[2]
VALIDATOR = '''package main
import("encoding/json";"os";"agent-platform/backend/internal/connectorpackage")
func main(){b,e:=os.ReadFile(os.Args[1]);if e!=nil{panic(e)};p,e:=connectorpackage.Parse(b);if e!=nil{panic(e)}
if e=os.WriteFile(os.Args[1],p.NormalizedArchive,0644);e!=nil{panic(e)}
json.NewEncoder(os.Stdout).Encode(map[string]any{"source":p.Metadata.Source,"version":p.Metadata.Version,"mode":p.Metadata.Type,"sha256":p.SHA256,"skills":len(p.Skills)})}
'''


def build(output):
    output.mkdir(parents=True, exist_ok=True)
    meta = json.loads((ROOT / 'connector-meta.json').read_text())
    destination = output / f"{meta['source']}-{meta['version']}.zip"
    with zipfile.ZipFile(destination, 'w', compression=zipfile.ZIP_DEFLATED) as archive:
        for name, source in [('connector-meta.json', 'connector-meta.json'), ('icon.svg', 'icon.svg'),
                             ('mcp.json', 'mcp.json'), ('skills/kling-ai/SKILL.md', 'SKILL.md')]:
            info = zipfile.ZipInfo(name, (2026, 1, 1, 0, 0, 0))
            info.external_attr = 0o100644 << 16
            info.compress_type = zipfile.ZIP_DEFLATED
            archive.writestr(info, (ROOT / source).read_bytes())
    # Go's internal import rule requires the temporary validator inside backend.
    with tempfile.TemporaryDirectory(prefix='.kling-package-parse-', dir=REPO / 'backend') as temp:
        source = Path(temp) / 'main.go'
        source.write_text(VALIDATOR)
        result = subprocess.run(['go', 'run', str(source), str(destination.resolve())], cwd=REPO / 'backend',
                                check=True, capture_output=True, text=True)
    evidence = json.loads(result.stdout)
    (output / 'package-validation.json').write_text(json.dumps(evidence, indent=2) + '\n')
    print(json.dumps({'package': str(destination.resolve()), **evidence}, ensure_ascii=False))
    return destination


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, default=REPO / 'outputs/connectors/kling-ai')
    build(parser.parse_args().output)
