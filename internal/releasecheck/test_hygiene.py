#!/usr/bin/env python3
"""The path-hygiene scan must fail on scratch, reviewer and home paths and pass on clean trees."""
from pathlib import Path
import subprocess, sys, tempfile, unittest
HYGIENE = Path(__file__).with_name('hygiene.py')

def scan(files):
    with tempfile.TemporaryDirectory() as directory:
        for name, text in files.items():
            path = Path(directory) / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(text)
        return subprocess.run([sys.executable, str(HYGIENE), '--scan-paths', directory], capture_output=True, text=True)

class PathHygiene(unittest.TestCase):
    def test_clean_tree_passes(self):
        self.assertEqual(scan({'a.txt': b'plain ~/.rbenv/versions/3.2 and /work/x'}).returncode, 0)
    def test_forbidden_strings_fail(self):
        for token in ('/private' + '/tmp/x', 'scratch' + 'pad', 'perfect' + '-society', '/Us' + 'ers/someone/.rbenv'):
            with self.subTest(token=token):
                result = scan({'testdata/d/run.stderr.txt': b'at ' + token.encode() + b' line'})
                self.assertEqual(result.returncode, 1)
                self.assertIn('run.stderr.txt', result.stdout)
    def test_binary_files_are_ignored(self):
        self.assertEqual(scan({'b.bin': b'\0\1/private' + b'/tmp'}).returncode, 0)

class GateStages(unittest.TestCase):
    def test_each_make_target_is_its_own_stage(self):
        import ast, re
        source = Path(__file__).with_name('run.py').read_text()
        match = re.search(r'^commands=(\[.*\])$', source, re.M)
        commands = ast.literal_eval(match.group(1))
        for command in commands:
            if command[0] == 'make':
                targets = [a for a in command[1:] if '=' not in a]
                self.assertEqual(len(targets), 1, command)
        names = [c[1] for c in commands if c[0] == 'make']
        for required in ('fmt-check', 'vet', 'lint', 'test', 'compat-check'):
            self.assertIn(required, names)

if __name__ == '__main__':
    unittest.main()
