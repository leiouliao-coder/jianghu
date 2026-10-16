#!/usr/bin/env python3
"""Verify imported design/art bytes, not game acceptance."""
from pathlib import Path
import hashlib,json
root=Path(__file__).resolve().parents[1]
manifest=json.loads((root/'docs/import-manifest.json').read_text())
for item in manifest['files']:
    path=root/item['file']
    assert hashlib.sha256(path.read_bytes()).hexdigest()==item['repository_sha256'],item['file']
print(f"IMPORT_MANIFEST_PASS {len(manifest['files'])} files")
