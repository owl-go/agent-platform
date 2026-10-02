#!/usr/bin/env python3
"""Publish CamScanner through the shared reviewed CLI build/Conformance workflow."""
import importlib.util
from pathlib import Path
import sys

if '--connector-source' in sys.argv:
    raise SystemExit('CamScanner publisher selects its own package source')
spec = importlib.util.spec_from_file_location('cli_package_publish', Path(__file__).resolve().parents[1] / 'teambition' / 'publish.py')
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
sys.argv.extend(['--connector-source', 'camscanner'])
try:
    module.main()
except module.error.HTTPError as exc:
    print('publication_http_error', exc.code)
    raise SystemExit(1)
except Exception as exc:
    print('publication_failed', type(exc).__name__, str(exc) if isinstance(exc, RuntimeError) else '')
    raise SystemExit(1)
