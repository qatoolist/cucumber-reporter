#!/usr/bin/env python3
"""Check reproducible generated assets, module files and release fixture size."""
from pathlib import Path
import hashlib,subprocess
root=Path(__file__).resolve().parents[2]
import os,sys
# Reviewer scratch locations, reviewer checkouts and personal home paths must
# never be committed. Written in pieces so this file does not match itself.
FORBIDDEN=('/private'+'/tmp','scratch'+'pad','perfect'+'-society','/Us'+'ers/')
def scan_paths(base,files):
 """Return 'file: token' for each tracked text file containing a forbidden string."""
 bad=[]
 for name in files:
  data=(base/name).read_bytes() if (base/name).is_file() else b''
  if b'\0' in data[:8192]:continue
  text=data.decode('utf-8','replace')
  bad+=[f'{name}: {token}' for token in FORBIDDEN if token in text]
 return bad
def walk_files(base):
 out=[]
 for directory,dirs,names in os.walk(base):
  dirs[:]=[d for d in dirs if d not in ('.git','node_modules','dist')]
  out+=[str((Path(directory)/n).relative_to(base)) for n in names]
 return out
if len(sys.argv)==3 and sys.argv[1]=='--scan-paths':
 # Test hook: scan a directory instead of the Git index.
 found=scan_paths(Path(sys.argv[2]),walk_files(Path(sys.argv[2])))
 print('\n'.join(found)) if found else print('HYGIENE no scratch, reviewer or absolute paths')
 sys.exit(1 if found else 0)
# A node_modules directory is third-party installation output. The directory
# itself must not be a symlink; source/release files may never be symlinks.
for directory,dirs,files in os.walk(root,followlinks=False):
 for name in dirs+files:
  p=Path(directory)/name
  if p.is_symlink():raise SystemExit('symlink in release tree: '+str(p.relative_to(root)))
 dirs[:]=[name for name in dirs if not (Path(directory)==root and name in ('.git','dist')) and Path(directory)/name!=root/'internal/browsercheck/node_modules']
tracked=subprocess.check_output(['git','ls-files','-s','-z'],cwd=root)
if any(row.startswith(b'120000 ') for row in tracked.split(b'\0')):raise SystemExit('tracked source symlink in Git index')
version=subprocess.check_output(['node','-p',"require('./internal/browsercheck/node_modules/jsdom/package.json').version"],cwd=root,text=True).strip()
if version!='20.0.3':raise SystemExit('unexpected jsdom version: '+version)
print('HYGIENE jsdom='+version+'; no source symlinks')
tracked_names=[n.decode() for n in subprocess.check_output(['git','ls-files','-z'],cwd=root).split(b'\0') if n]
found=scan_paths(root,tracked_names)
if found:raise SystemExit('scratch, reviewer or absolute paths in tracked files:\n  '+'\n  '.join(found))
print('HYGIENE no scratch, reviewer or absolute paths in %d tracked files'%len(tracked_names))
assets=[root/'report/assets'/p for p in ('app.min.js','app.min.css')]
before={p:p.read_bytes() for p in assets}
subprocess.run(['go','generate','./report'],cwd=root,check=True)
for p,b in before.items():
 if p.read_bytes()!=b:raise SystemExit(f'generated asset differed: {p}; inspect and regenerate before handover')
subprocess.run(['go','mod','tidy','-diff'],cwd=root,check=True)
blobs=list((root/'testdata/round14/captured/e2-budget-spill-3mb').glob('blobs/img*.png'))
if blobs:raise SystemExit('unused large source images returned; generate them in the corpus gate')
print('HYGIENE assets reproducible; module files clean; unused 60 MiB blobs absent')
# Validate the lockfile in isolation: never install through a local node_modules
# symlink used by the reviewer harness, and never depend on the registry here.
import tempfile,shutil,os
with tempfile.TemporaryDirectory(prefix='cr-lock-gate-') as directory:
 tmp=Path(directory)
 for name in ('package.json','package-lock.json'):
  shutil.copyfile(root/'internal/browsercheck'/name,tmp/name)
 subprocess.run(['npm','ci','--dry-run','--offline','--ignore-scripts'],cwd=tmp,
                env=dict(os.environ,npm_config_cache=str(tmp/'cache')),check=True)
print('HYGIENE npm lockfile resolves consistently offline')
