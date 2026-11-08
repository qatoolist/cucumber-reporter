import json,base64,sys
out=sys.argv[1]; n=int(sys.argv[2])
def step(name,status,err=None,emb=None,rows=None,doc=None,kw='Given '):
    s={'keyword':kw,'name':name,'line':5,'match':{'location':'steps.rb:1'},'result':{'status':status,'duration':1000000}}
    if err: s['result']['error_message']=err
    if emb: s['embeddings']=emb
    if rows: s['rows']=[{'cells':r} for r in rows]
    if doc: s['doc_string']={'value':doc,'content_type':'text/plain','line':6}
    return s
j={'deep':{'a':{'b':{'c':{'d':[1,2,{'e':'f'}]}}}},'list':list(range(5))}
emb=[{'data':base64.b64encode(json.dumps(j).encode()).decode(),'mime_type':'application/json','name':'payload.json'},
     {'data':base64.b64encode(b'line one\nline two\nbilling ok\n').decode(),'mime_type':'text/plain','name':'billing-run.log'}]
els=[]
els.append({'id':'f;rich','keyword':'Scenario','name':'rich scenario','line':3,'type':'scenario','tags':[{'name':'@x'}],'steps':[
  step('a table',"passed",rows=[['a','b'],['1','2']]),
  step('a doc',"passed",doc='\n'.join('doc line %d'%i for i in range(20))),
  step('short failure',"failed",err='boom (RuntimeError)',emb=emb),
  step('later',"skipped")]})
els.append({'id':'f;long','keyword':'Scenario','name':'long error','line':9,'type':'scenario','steps':[step('huge',"failed",err='\n'.join('frame %d'%i for i in range(1500)))]})
for i in range(n):
    st='failed' if i%7==0 else 'passed'
    els.append({'id':'f;s%d'%i,'keyword':'Scenario','name':'bulk scenario %d'%i,'line':20+i,'type':'scenario','steps':[step('bulk %d'%i,st,err='e%d'%i if st=='failed' else None)]})
json.dump([{'id':'f','uri':'features/f.feature','keyword':'Feature','name':'F','line':1,'elements':els}],open(out,'w'))
