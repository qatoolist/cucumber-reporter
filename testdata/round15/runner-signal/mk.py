import json,sys
W=sys.argv[1]
def load(f): return [json.loads(l) for l in open(f)]
def filt(env, keep_names, mutate=None):
    pick={}
    for e in env:
        if 'pickle' in e: pick[e['pickle']['id']]=e['pickle']['name']
    keepP={i for i,n in pick.items() if n in keep_names}
    tc={}; tcs={}
    for e in env:
        if 'testCase' in e: tc[e['testCase']['id']]=e['testCase']['pickleId']
        if 'testCaseStarted' in e: tcs[e['testCaseStarted']['id']]=tc[e['testCaseStarted']['testCaseId']]
    out=[]
    for e in env:
        k=list(e)[0]; v=e[k]
        if k=='pickle' and v['id'] not in keepP: continue
        if k=='testCase' and v['pickleId'] not in keepP: continue
        if k=='testCaseStarted' and tc[v['testCaseId']] not in keepP: continue
        if k in('testStepStarted','testStepFinished','testCaseFinished','attachment') and v.get('testCaseStartedId') and tcs[v['testCaseStartedId']] not in keepP: continue
        if mutate: mutate(e, pick, tc, tcs)
        out.append(e)
    return out
def pend(e,pick,tc,tcs):
    if 'testStepFinished' in e and pick[tcs[e['testStepFinished']['testCaseStartedId']]]=='Pass first try':
        e['testStepFinished']['testStepResult']={'status':'PENDING','duration':{'seconds':0,'nanos':1},'message':'TODO'}
r1=filt(load(W+'/xin/msg/run1.ndjson'),{'Fail fail pass','Pass first try'},pend)
r2=filt(load(W+'/xin/msg/run2pass.ndjson'),{'Fail fail pass'})
for n,r in (('r1',r1),('r2',r2)):
    open(W+'/sp/'+n+'.ndjson','w').write(''.join(json.dumps(e)+'\n' for e in r))
