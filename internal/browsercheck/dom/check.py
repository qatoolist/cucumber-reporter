#!/usr/bin/env python3
"""Run reviewer D2's real-DOM assertions, optionally killing UI mutants as well."""
import argparse
import base64
import json
import os
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[3]


def fixtures(tmp):
    binary = tmp / 'report'
    subprocess.run(['go', 'build', '-o', str(binary), './cmd/cucumber-report'], cwd=ROOT, check=True)
    # Independent synthetic edge cases supplement the real runner capture. Keep
    # these generated instead of committing another large report or image blob.
    deep = json.loads((ROOT / 'testdata/round15/ui/attachments.json').read_text())
    nested = {'value': 42}
    for i in range(8):
        nested = {'level' + str(i): nested}
    deep[0]['elements'][0]['steps'][0]['embeddings'][2]['data'] = base64.b64encode(json.dumps(nested).encode()).decode()
    deep[0]['elements'][1]['steps'][0]['result']['error_message'] = '\n'.join('failure line ' + str(i) for i in range(1501))
    (tmp / 'deep.json').write_text(json.dumps(deep))
    deep[0]['elements'][0]['steps'][0]['result']={'status':'failed','error_message':'eager attachment probe'}
    (tmp / 'eager.json').write_text(json.dumps(deep))
    bulk = [{'uri': 'bulk.feature', 'name': 'Bulk', 'elements': [
        {'type': 'scenario', 'name': 'bulk scenario ' + str(i), 'line': i + 2,
         'steps': [{'name': 'step', 'result': {'status': 'failed' if i % 2 else 'passed'}}]}
        for i in range(5000)]}]
    (tmp / 'bulk.json').write_text(json.dumps(bulk))
    (tmp / 'empty.json').write_text('[]')
    reports = {}
    for name, source, enhanced in [
        ('retry', ROOT / 'testdata/captured/cucumber-ruby-9.2.1-retry/retry.json', False),
        ('enhanced', ROOT / 'testdata/captured/cucumber-ruby-9.2.1-retry/retry.json', True),
        ('attachments', ROOT / 'testdata/round15/ui/attachments.json', True),
        ('showcase', ROOT / 'testdata/synthetic/showcase/cucumber.json', True),
        ('empty', tmp / 'empty.json', True),
        ('emptyMessages', ROOT/'testdata/round16/H3/in/empty.ndjson', True),
        ('eager', tmp / 'eager.json', True),
        ('earlier', ROOT / 'testdata/round16/H3/in/rj2/cucumber.json', True),
        ('messagesRetry', ROOT / 'testdata/round15/captured/s3-retry2/run.ndjson', True),
        ('deep', tmp / 'deep.json', True), ('bulk', tmp / 'bulk.json', True),
    ]:
        parent = tmp / name
        parent.mkdir()
        args = [str(binary), '-o', str(parent)]
        if source.suffix=='.ndjson':args+=['-format','messages']
        if enhanced:
            args += ['-enhanced', str(source)]
        else:
            inp = parent / 'input'
            inp.mkdir()
            (inp / 'cucumber.json').write_bytes(source.read_bytes())
            args.append(str(inp))
        result = subprocess.run(args, cwd=ROOT, capture_output=True, text=True)
        if result.returncode:
            raise RuntimeError(result.stderr)
        reports[name] = str(next(parent.glob('html-*/index.html')))
    return reports


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--mutants', action='store_true')
    args = parser.parse_args()
    with tempfile.TemporaryDirectory(prefix='cr-dom-') as directory:
        tmp = Path(directory)
        reports = fixtures(tmp)
        env = dict(os.environ, CR_DOM_REPORTS=json.dumps(reports))
        subprocess.run(['node', '--test', 'internal/browsercheck/dom/ui.test.cjs'], cwd=ROOT, env=env, check=True)
        if args.mutants:
            from mutations import run
            run(ROOT, tmp, reports)


if __name__ == '__main__':
    main()
