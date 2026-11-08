#!/usr/bin/env python3
"""Run public surface parity over unique repository fixtures and optional review roots."""
from pathlib import Path
import hashlib
import json
import re
import os
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[2]
seen = set()
inputs = []
rounds = set()
for root in [ROOT / 'testdata', *(Path(p) for p in sys.argv[1:])]:
    for path in sorted(root.rglob('*')):
        if path.suffix not in ('.json', '.ndjson') or any(p in ('golden', 'node_modules', '.git') or p.startswith('html-') for p in path.parts):
            continue
        digest = hashlib.sha256(path.read_bytes()).digest()
        if digest not in seen:
            seen.add(digest)
            inputs.append(str(path))
        if path.name in ('run2.json', 'run2.ndjson'):
            first = path.with_name('run1' + path.suffix)
            if first.exists():
                files = [first, path]
                third = path.with_name('run3' + path.suffix)
                if third.exists():
                    files.append(third)
                rounds.add(','.join(map(str, files)))
print(f'{len(inputs)} unique inputs; {len(rounds)} cross-input refusal sets', flush=True)
with tempfile.TemporaryDirectory(prefix='report-surfaces-') as tmp:
    binary = str(Path(tmp) / 'check')
    subprocess.run(['go', 'build', '-o', binary, 'internal/surfacecheck/check.go'], cwd=ROOT, check=True)
    result = subprocess.run([binary, *inputs, *sorted(rounds)], cwd=ROOT, env=os.environ, capture_output=True,text=True)
    print(result.stdout,end='');print(result.stderr,end='',file=sys.stderr)
    refusals=[]
    for stage,label in re.findall(r'^(ENH-PARSE-REFUSED|PLAIN-REFUSED-RENDER|PLAIN-REFUSED|ENH-REFUSED) (.+?): ',result.stdout,re.M):
        refusals.append(stage+' '+label.replace(str(ROOT)+'/', ''))
    counts=re.findall(r'^MONOTONIC CHECKS (\d+)$',result.stdout,re.M)
    observed={'refusals':sorted(refusals),'monotonic_checks':int(counts[0]) if len(counts)==1 else None}
    manifest=ROOT/'internal/surfacecheck/expected-refusals.json'
    if not manifest.exists():
        (ROOT/'dist').mkdir(exist_ok=True)
        (ROOT/'dist/observed-refusals.json').write_text(json.dumps(observed,indent=2)+'\n')
        raise SystemExit('Missing reviewed refusal inventory; observations saved to dist/observed-refusals.json for review, never auto-accepted')
    expected=json.loads(manifest.read_text())
    if observed!=expected:
        print('FAIL refusal/property inventory changed',json.dumps({'expected':expected,'actual':observed},indent=2));raise SystemExit(1)
    print('PINNED refusal inventory and monotonicity count match')
raise SystemExit(result.returncode)
