#!/usr/bin/env python3
"""Check the API snapshot and prove that leakage/signature/value mutants fail."""
from pathlib import Path
import os
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[2]
ENV = dict(os.environ, GOFLAGS='-mod=readonly', GOPROXY='off')
COMMAND = ['go', 'run', 'internal/apicheck/main.go']
subprocess.run(COMMAND, cwd=ROOT, env=ENV, check=True)
documentation=subprocess.check_output(['go','doc','./cucumber.ErrNoResults'],cwd=ROOT,env=ENV,text=True)
if 'modelmeta.' in documentation or '/internal/' in documentation:
    raise SystemExit('Public sentinel godoc exposes internal initialization')
print('Public error documentation: no internal initializer')
with tempfile.TemporaryDirectory(prefix='report-api-mutants-') as directory:
    source = Path(directory) / 'source'
    shutil.copytree(ROOT, source, ignore=shutil.ignore_patterns(
        '.git', 'dist', 'node_modules', '__pycache__', 'graphify-out', 'testdata'))
    mutants = [
        ('internal-alias', 'cucumber/api_leak_mutant.go', None,
         'package cucumber\nimport input "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"\ntype LeakedError = input.ParseError\n',
         'FAIL internal API type:'),
        ('method-signature', 'report/public.go',
         'Equal(other PreparedRun) bool', 'Equal(other PreparedRun, ignored ...bool) bool',
         'FAIL exported API differs from reviewed snapshot'),
        ('constant-value', 'cucumber/model.go',
         'DefaultInputLimit            int64 = 512 << 20',
         'DefaultInputLimit            int64 = 513 << 20',
         'FAIL exported API differs from reviewed snapshot'),
    ]
    for name, filename, old, new, expected in mutants:
        target = source / filename
        original = target.read_text() if target.exists() else None
        try:
            if old is None:
                if original is not None:
                    raise RuntimeError(f'{name}: mutant file already exists')
                target.write_text(new)
            else:
                if original.count(old) != 1:
                    raise RuntimeError(f'{name}: stale mutation anchor')
                target.write_text(original.replace(old, new))
            result = subprocess.run(COMMAND, cwd=source, env=ENV, text=True,
                                    capture_output=True, timeout=120)
            if result.returncode == 0 or expected not in result.stdout:
                raise RuntimeError(f'{name} survived or failed for the wrong reason:\n'
                                   f'{result.stdout}\n{result.stderr}')
            print(f'API mutant {name}: killed', flush=True)
        finally:
            if original is None:
                target.unlink(missing_ok=True)
            else:
                target.write_text(original)
