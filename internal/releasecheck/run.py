#!/usr/bin/env python3
"""Release gate bound to a clean commit, executed in a detached worktree."""
from pathlib import Path
import hashlib, os, shlex, shutil, subprocess, sys, tempfile, time
root=Path(__file__).resolve().parents[2]
# Check before opening the receipt: never overwrite a valid receipt with a dirty
# tree's partial run or call a working-directory digest a releasable revision.
status=subprocess.check_output(['git','status','--porcelain','--untracked-files=all'],cwd=root)
if status:
 print('RELEASE GATE REFUSED: working tree is dirty. Commit the reviewed changes before running the release gate.',file=sys.stderr)
 sys.exit(1)
head=subprocess.check_output(['git','rev-parse','HEAD'],cwd=root,text=True).strip()
listing=subprocess.check_output(['git','ls-tree','-r','-z',head],cwd=root)
digest=hashlib.sha256(listing).hexdigest()
receipt=root/'dist/release-gate.log';receipt.parent.mkdir(exist_ok=True)
commands=[['npm','ci','--prefix','internal/browsercheck','--ignore-scripts'],['node','-p',"'jsdom='+require('./internal/browsercheck/node_modules/jsdom/package.json').version"],['make','fmt-check'],['make','vet'],['make','lint'],['make','test'],['make','compat-check'],['go','test','-C','testdata/consumer','./...'],['python3','internal/runnercheck/check.py'],['python3','internal/surfacecheck/run.py'],['python3','internal/surfacecheck/mutations.py'],['python3','internal/browsercheck/dom/check.py','--mutants'],['node','--test','internal/browsercheck/model.test.cjs'],['python3','internal/apicheck/check.py'],['python3','internal/releasecheck/test_hygiene.py'],['python3','internal/releasecheck/hygiene.py'],['make','fuzz-smoke','FUZZTIME=10s'],['go','test','-tags','browser','./internal/cdp','./report','-run','^$']]
# Each stage runs even after an earlier failure so one failure never hides another; any failure fails the gate.
failed=False
with tempfile.TemporaryDirectory(prefix='cr-release-head-') as temporary:
 checkout=Path(temporary)/'checkout'
 subprocess.run(['git','worktree','add','--detach',str(checkout),head],cwd=root,check=True)
 try:
  with receipt.open('w') as log:
   def emit(s):print(s,flush=True);log.write(s+'\n');log.flush()
   emit('RELEASE GATE HEAD='+head+' ls-tree-sha256='+digest)
   for command in commands:
    emit('$ '+shlex.join(command));begin=time.monotonic()
    with subprocess.Popen(command,cwd=checkout,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True,bufsize=1) as proc:
     for line in proc.stdout:emit(line.rstrip('\n'))
    emit(f'EXIT {proc.returncode} elapsed={time.monotonic()-begin:.1f}s')
    if proc.returncode:failed=True
   changed=subprocess.check_output(['git','status','--porcelain','--untracked-files=all'],cwd=checkout)
   if changed:emit('FAIL: checks changed the clean release checkout');failed=True
   emit('RELEASE GATE '+('FAILED' if failed else 'PASSED')+' HEAD='+head+' ls-tree-sha256='+digest)
   emit('Browser/print/cross-browser CI and the manual Comet checklist remain additional release requirements.')
 finally:subprocess.run(['git','worktree','remove','--force',str(checkout)],cwd=root,check=True)
print('Complete receipt:',receipt)
sys.exit(bool(failed))
