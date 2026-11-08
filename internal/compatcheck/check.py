#!/usr/bin/env python3
"""Compare plain data against a locally built v0.2.0, without network or checkout changes.
Only explicit per-fixture contract exceptions in expectations.json are admitted.
Runner truth and annotation tests independently verify changed execution outcomes.
"""
import io, json, os, re, shutil, subprocess, tarfile, tempfile
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
EXPECT=Path(__file__).with_name('expectations.json')
def command(args,**kw):
    return subprocess.run(args,text=True,capture_output=True,**kw)
def data(binary,raw,work):
    work.mkdir();(work/'cucumber.json').write_bytes(raw)
    p=command([str(binary),str(work)])
    if p.returncode:return None,p.stderr.strip()
    html=next(work.glob('html-*/index.html')).read_text()
    return json.loads(re.search(r'id="report-data">(.*?)</script>',html,re.S)[1]),None
def compare(old,new,changes,raw):
    new=dict(new);new.pop('verdictSource',None)
    aliases=new.pop('aliases',{})
    if aliases:
        assert any(c in changes for c in ('background','retries')), 'unexpected aliases'
        old_ids={s['id'] for f in old['features'] for s in (f.get('elements') or [])}
        new_ids={s['id'] for f in new['features'] for s in (f.get('elements') or [])}
        assert all(k in old_ids and v in new_ids for k,v in aliases.items()), 'invalid folded bookmark alias'
    def green(value):return value.get('verdict',{}).get('tone') in ('ok',) or (value.get('verdict',{}).get('tone')=='warn' and value.get('verdict',{}).get('label','').startswith('Passed with known failures'))
    if not green(old) and green(new):
        assert 'retries' in changes,'verdict became greener without a confirmed retry exception' 
    if not changes:
        assert old==new,'unexpected data difference with no contract exception'
        return
    old=dict(old);of=old.pop('features');nf=new.pop('features')
    if any(c in changes for c in ('hooks','background','retries')):
        old.pop('verdict',None);new.pop('verdict',None)
    execution=new.get('execution',{})
    if 'retries' in changes:new.pop('execution',None)
    if 'background' in changes:new['sections']=[s for s in (new.get('sections') or []) if s.get('title')!='Input diagnostics'] or old.get('sections')
    assert old==new,'unexpected run metadata/annotation difference'
    assert len(of)==len(nf),'feature count changed'
    for feature_index,(before,after) in enumerate(zip(of,nf)):
        before=dict(before);after=dict(after);bs=before.pop('elements') or [];ns=after.pop('elements') or []
        if any(c in changes for c in ('hooks','background','retries')):
            before.pop('status');after.pop('status')
        assert before==after,'feature metadata/ID changed'
        byid={s['id']:s for s in bs}
        for s in ns:
            assert s['id'] in byid,'scenario ID changed: '+s['id']
            previous=dict(byid[s['id']]);current=dict(s)
            previous.pop('steps');current.pop('steps')
            if any(c in changes for c in ('hooks','background','retries')):
                if previous['status'] in ('failed','undefined','ambiguous','pending','skipped') and current['status'] in ('passed','known'):
                    assert 'retries' in changes,'scenario became greener without confirmed retry: '+s['id']
                previous.pop('status');current.pop('status')
            assert previous==current,'scenario metadata changed: '+s['id']
        remaining={s['id'] for s in ns}
        all_steps=[step for s in ns for step in (s.get('steps') or [])]
        for source_index,source in enumerate(bs):
            if source['id'] in remaining:continue
            if 'retries' in changes:
                # A removed execution must be retained in attempt history.
                # execution is stripped from the run comparison above; use its saved copy.
                histories=[attempt for attempts in execution.values() for attempt in attempts]
                assert any(a.get('steps') for a in histories),'removed retry has no attempt history: '+source['id']
            else:
                assert 'background' in changes and json.loads(raw)[feature_index]['elements'][source_index].get('type')=='background','old ID disappeared: '+source['id']
                for step in source.get('steps') or []:
                    assert step['status']=='passed' or any(x['status']==step['status'] and x['name']==step['name'] for x in all_steps),'nonpassing Background disappeared: '+source['id']
        if not any(c in changes for c in ('background','retries')):
            assert len(bs)==len(ns),'scenario count changed'
        # Every source step must survive, in order, except prior retry attempts.
        if 'retries' not in changes:
            for s in ns:
                previous=byid[s['id']].get('steps') or [];current=s.get('steps') or [];at=0
                for step in previous:
                    while at<len(current) and (current[at]['name'],current[at]['keyword'])!=(step['name'],step['keyword']):at+=1
                    assert at<len(current),'source step disappeared: '+s['id']
                    a=dict(step);b=dict(current[at]);b.pop('label',None)
                    if 'evidence' in changes:
                        for key in ('att','doc','rows'):a.pop(key,None);b.pop(key,None)
                    if 'hooks' in changes:
                        # Passing hook logs may be associated with the next own step.
                        a.pop('att',None);b.pop('att',None)
                    assert a==b,'source step result/content changed: '+s['id']
                    at+=1

def self_test():
    for tone in ('fail','info'):
        try: compare({'verdict':{'tone':tone}}, {'verdict':{'tone':'ok'}}, ['hooks'], '[]')
        except AssertionError as e: assert 'greener' in str(e)
        else: raise AssertionError('greener-verdict detector disabled')

    for status in ('pending','skipped'):
        scenario={'id':'s','status':status,'steps':[]}
        old={'verdict':{'tone':'ok'},'features':[{'status':'passed','elements':[scenario]}]}
        new={'verdict':{'tone':'ok'},'features':[{'status':'passed','elements':[dict(scenario,status='passed')]}]}
        try: compare(old,new,['hooks'],'[]')
        except AssertionError as e: assert 'scenario became greener' in str(e)
        else: raise AssertionError('per-scenario greener detector disabled')

def main():
    self_test()
    expectations=json.loads(EXPECT.read_text());errors=[];accepted=0
    with tempfile.TemporaryDirectory(prefix='report-compat-') as tmp:
        temp=Path(tmp);oldsrc=temp/'v020';oldsrc.mkdir()
        archive=subprocess.check_output(['git','archive','v0.2.0'],cwd=ROOT)
        with tarfile.open(fileobj=io.BytesIO(archive)) as t:t.extractall(oldsrc,filter='data')
        old=temp/'old';new=temp/'new'
        for source,binary in ((oldsrc,old),(ROOT,new)):
            p=command(['go','build','-o',str(binary),'./cmd/cucumber-report'],cwd=source)
            if p.returncode:raise RuntimeError(p.stderr)
        for i,(path,changes) in enumerate(expectations.items()):
            raw=(ROOT/path).read_bytes();a,ae=data(old,raw,temp/f'old-{i}');b,be=data(new,raw,temp/f'new-{i}')
            if a is None:continue # Acceptance extensions are covered by the runner tests.
            accepted+=1
            if b is None:errors.append(path+': newly refused: '+be);continue
            try:compare(a,b,changes,raw)
            except AssertionError as e:errors.append(path+': '+str(e))
    print(f'{accepted} v0.2.0-accepted fixtures checked; {len(errors)} unexplained differences')
    for e in errors:print(e)
    return bool(errors)
if __name__=='__main__':raise SystemExit(main())
