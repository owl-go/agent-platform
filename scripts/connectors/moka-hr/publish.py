#!/usr/bin/env python3
"""Publish the reviewed Moka HR package using the shared build/Conformance lifecycle."""
import importlib.util
from pathlib import Path
import sys

if '--connector-source' in sys.argv:
    raise SystemExit('Moka HR publisher selects its own package source')
spec = importlib.util.spec_from_file_location('cli_package_publish', Path(__file__).resolve().parents[1] / 'teambition' / 'publish.py')
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
sys.argv.extend(['--connector-source', 'moka-hr'])
try:
    module.main()
except Exception as exc:
    print('publication_failed', type(exc).__name__, str(exc) if isinstance(exc, RuntimeError) else '')
    raise SystemExit(1)
