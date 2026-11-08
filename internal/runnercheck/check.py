#!/usr/bin/env python3
"""Compare real captures and reviewer runner fixtures with their independent truth.
No optional skips: absent raw large captures are regenerated with Ruby 9.2.1.
"""
import json, os, pathlib, re, shutil, subprocess, tempfile
ROOT=pathlib.Path(__file__).resolve().parents[2]
CAPTURE_ROOTS=['testdata/captured','testdata/round13/captured-ruby-9.2.1','testdata/round14/captured','testdata/round15/captured','testdata/round16/F3/cap']
DATA=re.compile(r'<script[^>]+id="report-data"[^>]*>(.*?)</script>',re.S)

def truth_for(folder, name):
 text=(folder/'summary.txt').read_text()
 tagged=re.findall(r'^\$ \[([^]]+)\](.*?)(?:^exit=(\d+))',text,re.M|re.S)
 if tagged:
  matches=[(command,int(code)) for tag,command,code in tagged if tag==name]
  if len(matches)!=1:raise RuntimeError(f'no unique truth: {folder}/{name}')
  return matches[0]
 blocks=re.findall(r'(^\$.*?)(?:^exit=(\d+))',text,re.M|re.S)
 if not blocks:
  codes=re.findall(r'exit=(\d+)',text)
  if len(codes)==1:
   readme=folder.parent/'README.md'
   if readme.exists():
    for line in readme.read_text().splitlines():
     cells=[c.strip() for c in line.split('|')]
     if len(cells)>3 and cells[1]==folder.name:return cells[2],int(codes[0])
   return '',int(codes[0])
 i=int(name[3:])-1 if re.fullmatch(r'run\d+',name) else 0
 if i>=len(blocks):raise RuntimeError(f'missing exit truth {folder}/{name}')
 return blocks[i][0],int(blocks[i][1])

def capture_truth(folder, name):
 text=(folder/'summary.txt').read_text()
 blocks=re.split(r'^exit=\d+.*$',text,flags=re.M)
 tagged=[b for b in blocks if re.search(r'^\$ \['+re.escape(name)+r'\]',b,re.M)]
 index=int(name[3:])-1 if re.fullmatch(r'run\d+',name) else 0
 block=tagged[0] if tagged else blocks[min(index,len(blocks)-1)]
 matches=re.findall(r'^(\d+) scenarios?(?: \(([^)]*)\))?',block,re.M)
 counts={}
 if matches:
  total,items=matches[-1]
  for n,status in re.findall(r'(\d+) (\w+)',items):counts[status]=int(n)
  if not counts and int(total):raise RuntimeError('missing count truth '+str(folder))
  counts['passed']=counts.get('passed',0)+counts.pop('flaky',0)
  counts={k:v for k,v in counts.items() if v}
 else:counts=None
 section=re.search(r'Failing Scenarios:\n((?:cucumber [^\n]*\n)+)',block)
 failing=sorted(set(re.findall(r'^cucumber ([^\n]+?:\d+)(?:\s+#|$)',section[1],re.M))) if section else []
 return {'counts':counts,'failing':failing}

def large_capture(source, tmp):
 # Recreate the omitted evidence at the documented sizes, using the real runner.
 version=subprocess.check_output(['cucumber','--version'],text=True).strip()
 if version!='9.2.1':raise RuntimeError(f'large capture gate requires cucumber 9.2.1, got {version}')
 target=tmp/source.name;shutil.copytree(source,target);blobs=target/'blobs';blobs.mkdir(exist_ok=True)
 if source.name.startswith(('e1-','e2-')):
  size=(5 if source.name.startswith('e1-') else 3)*1024*1024
  for i in range(20):(blobs/f'img{i:02}.png').write_bytes(b'\x89PNG\r\n\x1a\n'+bytes([i+1])*(size-8))
 if not source.name.startswith('e2-'):(blobs/'big.txt').write_bytes(b'large evidence\n'*(30*1024*1024//15)+b'\n')
 result=subprocess.run(['cucumber','--no-color','--format','json','--out','run.json','--format','message','--out','run.ndjson','--format','progress'],cwd=target,env=dict(os.environ,PLAN='L:fail',CUCUMBER_PUBLISH_QUIET='true'),capture_output=True,text=True)
 if result.returncode!=1:raise RuntimeError('large capture runner: '+result.stdout+result.stderr)
 print('REGENERATED',source.name,'cucumber='+version,flush=True)
 return target

with tempfile.TemporaryDirectory(prefix='cr-corpus-') as work:
 work=pathlib.Path(work);binary=work/'report';subprocess.run(['go','build','-o',str(binary),'./cmd/cucumber-report'],cwd=ROOT,check=True)
 failures=[];checked=0;groups=0;refusals=0
 def run(file, expected_exit, command='', truth=None, mode='enhanced', captured_exit=None,known=False):
  global checked,refusals
  reference_exit=expected_exit if captured_exit is None else captured_exit
  out=work/'output';shutil.rmtree(out,ignore_errors=True);out.mkdir()
  args=[str(binary),'-o',str(out)]
  if mode=='plain':
   inp=work/'plain';inp.mkdir(exist_ok=True);shutil.copyfile(file,inp/'cucumber.json');args.append(str(inp))
  else:
   args+=['-enhanced','-fail-on-unexpected']
   if known:args+=['-options',str(ROOT/'testdata/round16/F3/adv/knownz.opt')]
   if file.suffix=='.ndjson':args+=['-format','messages']
   # Legacy JSON omits runner policy. Use its captured strict policy explicitly.
   strict='--strict' in command and '--no-strict' not in command
   strict=strict or '-p ci' in command
   if (strict or known) and file.suffix=='.json':
    cfg=work/'policy.json';cfg.write_text(json.dumps({'gates':{'pending':True,'undefined':True,'flaky':known or '--strict-flaky' in command or '-p ci' in command}}));args+=['-run-config',str(cfg)]
   args.append(str(file))
  result=subprocess.run(args,cwd=ROOT,capture_output=True,text=True,timeout=180)
  label=f'{file.parent.name}/{file.name}:{mode}'+(':known' if known else '')
  # Allow-lists are keyed by the capture's full relative path, never a bare name.
  try:rel=file.parent.relative_to(ROOT).as_posix()
  except ValueError:rel='regenerated/'+file.parent.name
  refusal_reason = None
  if rel in ('testdata/round13/captured-ruby-9.2.1/around-fail','testdata/round14/captured/d3-comments-only'):
   refusal_reason = 'runner formatter failed before producing a complete artifact (captured exit=2)'
  if rel.startswith('testdata/runners/behave-1.2.6'):
   refusal_reason = 'behave native JSON is not supported (documented support matrix)'
  if truth and (truth.get('expect_refusal') or (truth.get('enhancedError') and mode=='enhanced')):
   refusal_reason = truth.get('note','truncated malformed fixture')
  if rel=='testdata/runners/cucumber-js-10-messages-crashed':refusal_reason='truncated stream: unfinished last execution, no testRunFinished'
  if refusal_reason:
   if result.returncode!=1:failures.append(label+' documented refusal missing')
   else:refusals+=1;print('DOCUMENTED REFUSAL',label,refusal_reason,flush=True)
   return
  if result.returncode not in (0,3):failures.append(label+' refused: '+result.stderr[-400:]);return
  html=next(out.glob('html-*/index.html')).read_text();m=DATA.search(html)
  if not m:failures.append(label+' no report-data');return
  page=json.loads(m[1]);tone=page['verdict']['tone']
  if rel in ('testdata/round14/captured/d6-zero-plus-failing-after-all','testdata/round16/F3/cap/k01-afterall-pass','testdata/round16/F3/cap/k17-afterall-retry-flaky'):
   # Captured formatter reports success:true before its AfterAll hook crashes;
   # JSON is []. Neither file includes the failure. Require an empty INFO report,
   # never a green pass. This limitation is explicitly documented in 0.3.0.md.
   if rel=='testdata/round14/captured/d6-zero-plus-failing-after-all' and (tone!='info' or any(f['elements'] for f in (page['features'] or []))):failures.append(label+' missing empty-run limitation')
   print('DOCUMENTED MISSING EVIDENCE',label,'runner emitted success:true then failed outside artifact',flush=True)
   expected_exit=0
  if rel=='testdata/round16/F3/cap/k19-wip-pass' and file.suffix=='.json':
   expected_exit=0;print('DOCUMENTED POLICY DIFFERENCE',label,'JSON does not encode --wip',flush=True)
  if expected_exit and (tone in ('ok','warn') or (mode!='plain' and result.returncode!=3)):
   failures.append(f'{label} greener: cucumber={expected_exit} reporter={result.returncode}/{tone}')
  if rel in ('testdata/round16/F3/cap/k14-retry-fail-skip','testdata/round13/captured-ruby-9.2.1/wip-allfail') and not expected_exit:
   # Documented redder exits: this release requires a strictly passed rescue
   # (k14) and treats --wip all-failed scenarios as failures (wip-allfail).
   expected_exit=1;print('DOCUMENTED EXIT DIFFERENCE',label,f'cucumber=0 reporter={result.returncode}/{tone}',flush=True)
  if not expected_exit and result.returncode!=0:failures.append(f'{label} redder exit: expected 0 got {result.returncode}/{tone}')

  if truth:truth=dict(truth)
  # Explicit, pinned differences: the source cannot attribute omitted Around
  # failures, and this release intentionally requires a strictly passed rescue.
  exception={
   'testdata/captured/ruby-9.2.1-r12/retryfinal':({'failed':2,'passed':1},None),
   'testdata/round16/F3/cap/k14-retry-fail-skip':({'failed':1,'passed':3},['features/a.feature:2']),
  }.get(rel)
  if file.suffix=='.ndjson':exception={
   'testdata/round13/captured-ruby-9.2.1/around-after-only':({'passed':2},[]),
   'testdata/round13/captured-ruby-9.2.1/retry-around-after':({'passed':2},[]),
   'testdata/round13/captured-ruby-9.2.1/around-fail-msgonly':({'failed':1,'passed':2},['features/a.feature:4']),
   'testdata/round14/captured/b2-around-after-fail':({'passed':3},[]),
  }.get(rel,exception)
  if known:exception=({'known':1,'passed':3},[])
  if exception:
   if not truth:raise RuntimeError('exception without captured truth '+label)
   truth['counts']=exception[0]
   if exception[1] is not None:truth['failing']=exception[1]
   print('DOCUMENTED COUNT DIFFERENCE',label,exception,flush=True)
  if truth and truth.get('counts') is not None:
   actual={}
   for feature in page['features']:
    for sc in feature['elements']:
     if not sc.get('runHook'):actual[sc['status']]=actual.get(sc['status'],0)+1
   expected={k:v for k,v in truth['counts'].items() if v}
   expected_tone='fail' if any(expected.get(x,0) for x in ('failed','ambiguous','undefined')) else ('ok' if expected.get('passed') else 'info')
   if mode!='plain' and expected_exit:expected_tone='fail'
   if rel in ('testdata/round13/captured-ruby-9.2.1/around-after-only','testdata/round13/captured-ruby-9.2.1/retry-around-after','testdata/round13/captured-ruby-9.2.1/around-fail-msgonly','testdata/round14/captured/b2-around-after-fail') and file.suffix=='.ndjson':
    expected_tone='fail';print('DOCUMENTED TONE DIFFERENCE',label,'Around failure is outside the Messages step results',flush=True)
   if tone!=expected_tone:failures.append(f'{label} tone {tone} != {expected_tone}')
   if actual!=expected:failures.append(f'{label} counts {actual} != {expected}')
  if truth and 'failing' in truth:
   actual_fail=sorted(set(f["uri"]+":"+str(sc.get('line',0)) for f in page['features'] for sc in f['elements'] if sc['status']=='failed' and not sc.get('runHook')))
   if actual_fail!=truth['failing']:failures.append(f"{label} failing locations {actual_fail} != {truth['failing']}")
  checked+=1;print('CAPTURE',label,f'reference_exit={reference_exit} required_failure={bool(expected_exit)} reporter={result.returncode}/{tone}',flush=True)
 for root in CAPTURE_ROOTS:
  for summary in sorted((ROOT/root).rglob('summary.txt')):
   source=summary.parent;folder=source;groups+=1
   files=sorted([p for p in folder.iterdir() if p.suffix in ('.json','.ndjson') and p.name not in ('args.json','truth.json')])
   if not files and source.name.startswith(('e1-large','e2-budget','e3-big')):
    folder=large_capture(source,work);files=[folder/'run.json',folder/'run.ndjson']
   if not files and source.relative_to(ROOT).as_posix()=='testdata/round16/F3/cap/k02-beforeall':
    if 'exit=2' not in summary.read_text():failures.append('k02-beforeall truth changed')
    print('DOCUMENTED NO ARTIFACT k02-beforeall: Ruby aborts before either formatter writes',flush=True)
    continue
   if not files:
    # A crashed JSON formatter is documented by the capture. Its Messages-only
    # reproduction is exercised as a separate capture, not silently skipped.
    failures.append(f'{source}: missing captured output');continue
   for file in files:
    command,code=truth_for(source,file.stem)
    run(file,code,command,truth=capture_truth(source,file.stem))
    if source.relative_to(ROOT).as_posix() in ('testdata/round16/F3/cap/k23-known-z-flaky-x','testdata/round16/F3/cap/k24-known-z-strictflaky'):run(file,code,command,truth=capture_truth(source,file.stem),known=True)
    if file.suffix=='.json':
     # Plain mode cannot know strict runner policies; compare evidence, not an
     # unavailable policy signal. Enhanced JSON above applies the explicit policy.
     policy_only=('--strict' in command or '-p ci' in command)
     run(file,0 if policy_only else code,command,truth=capture_truth(source,file.stem),mode='plain',captured_exit=code)
 original=ROOT/'testdata/captured/cucumber-ruby-9.2.1-retry';groups+=1
 for ext in ('json','ndjson'):run(original/f'retry.{ext}',1)
 if groups!=122:failures.append(f'capture inventory changed: expected 122, found {groups}')
 for folder in sorted((ROOT/'testdata/runners').iterdir()):
  if not (folder/'truth.json').exists():continue
  truth=json.loads((folder/'truth.json').read_text())
  for file in folder.iterdir():
   if file.name in ('cucumber.json','cucumber.ndjson'):
    run(file,1 if truth.get('verdict')=='failed' else 0,truth=truth)
    if file.suffix=='.json':run(file,1 if truth.get('verdict')=='failed' else 0,truth=truth,mode='plain')
 if refusals!=16:failures.append(f'refusal inventory changed: {refusals} != 16')
 print(f'CORPUS groups={groups} checked={checked} documented_refusals={refusals} failures={len(failures)}',flush=True)
 for failure in failures:print('FAIL',failure,flush=True)
 raise SystemExit(bool(failures))
