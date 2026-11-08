// Browser-free regression checks execute the actual model/context source sections.
const {test} = require('node:test');
const assert = require('node:assert/strict');
const {readFileSync} = require('node:fs');
const {join} = require('node:path');
const vm = require('node:vm');
const source = readFileSync(join(__dirname,'../../report/assets/app.js'),'utf8');
const section = (start,end) => {const a=source.indexOf(start),b=source.indexOf(end,a);assert(a>=0&&b>a);return source.slice(a,b);};

test('20,000 scenarios associate analysis with linear data access',()=>{
 const n=20000;let reads=0;
 const analyses=Array.from({length:n},(_,i)=>({id:'s'+(n-i-1),marker:n-i-1}));
 const indexed=new Proxy(analyses,{get(target,key,receiver){if(/^\d+$/.test(String(key))){reads++;assert(reads<=n*4,'analysis lookup became quadratic');}return Reflect.get(target,key,receiver);}});
 const D={analysis:{scenarios:indexed},features:[{id:'f',name:'Checks',elements:Array.from({length:n},(_,i)=>({id:'s'+i,name:'scenario '+i,status:'passed',steps:[]}))}]};
 const ctx={D};vm.createContext(ctx);
 const started=performance.now();vm.runInContext(section('  const SEV =','  const T =')+'globalThis.result=S;',ctx);
 assert.equal(ctx.result.length,n);
 for(let i=0;i<n;i++)assert.equal(ctx.result[i].analysis.marker,i);
 console.log(`20,000 scenario model: ${(performance.now()-started).toFixed(1)} ms; ${reads} analysis element reads`);
});

test('scroll persistence is delayed and does not serialize scenario IDs',()=>{
 const handlers={},timers=new Map(),writes=[];let id=0;
 const ctx={main:{querySelector:()=>true},CTX:{ids:Array.from({length:20000},(_,i)=>'long-scenario-id-'+i),href:'#/scenarios?status=failed',label:'filtered',scroll:0},location:{pathname:'/report/index.html',hash:'#/scenarios?status=failed'},W:{scrollY:10,addEventListener:(name,fn)=>handlers[name]=fn},sessionStorage:{getItem:()=>null,setItem:(key,value)=>writes.push({key,value}),removeItem:()=>{}},setTimeout:(fn)=>{timers.set(++id,fn);return id;},clearTimeout:(key)=>timers.delete(key)};
 vm.createContext(ctx);vm.runInContext(section('  const contextKey =','  const onCleanup ='),ctx);
 for(let i=0;i<100;i++){ctx.W.scrollY=i;handlers.scroll();}
 assert.equal(writes.length,0);assert.equal(timers.size,1);
 for(const fn of timers.values())fn();
 assert.equal(writes.length,1);assert(writes[0].key.length<200);assert(writes[0].value.length<1024);
 const saved=JSON.parse(writes[0].value);assert.equal(saved.scroll,99);assert(!('ids' in saved));
});

test('shared table ordering respects direction and numeric durations',()=>{
 const ctx={};vm.createContext(ctx);vm.runInContext(section('  const sortTableList =','  const tableRows =')+'globalThis.sort=sortTableList;',ctx);
 const items=[{name:'B',dur:2},{name:'A',dur:10},{name:'C',dur:1}];
 assert.deepEqual(Array.from(ctx.sort(items,'name',true),x=>x.name),['A','B','C']);
 assert.deepEqual(Array.from(ctx.sort(items,'duration',false),x=>x.dur),[10,2,1]);
 assert.deepEqual(items.map(x=>x.name),['B','A','C']);
});

test('search-match foreground and background meet WCAG AAA for normal text',()=>{
 const css=readFileSync(join(__dirname,'../../report/assets/app.css'),'utf8');
 const rule=css.match(/\.matched-line\s*\{([^}]+)\}/)[1];
 const foreground=rule.match(/(?:^|;)\s*color:\s*(#[\da-f]{6})/i)[1];
 const background=rule.match(/background:\s*(#[\da-f]{6})/i)[1];
 const luminance=(hex)=>{const channels=hex.slice(1).match(/../g).map(v=>parseInt(v,16)/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4);return channels[0]*.2126+channels[1]*.7152+channels[2]*.0722;};
 const a=luminance(foreground),b=luminance(background);const contrast=(Math.max(a,b)+.05)/(Math.min(a,b)+.05);
 assert(contrast>=7);console.log(`Search-match text contrast: ${contrast.toFixed(2)}:1`);
});

test('final attempts reference scenario steps without duplicating evidence',()=>{
 const ctx={};vm.createContext(ctx);
 vm.runInContext(section('  const attemptSteps =','  const investigationText =')+'globalThis.getSteps=attemptSteps;',ctx);
 const scenario={steps:[{name:'final evidence'}]},earlier={steps:[{name:'earlier evidence'}]};
 assert.equal(ctx.getSteps(scenario,{useScenarioSteps:true}),scenario.steps);
 assert.equal(ctx.getSteps(scenario,earlier),earlier.steps);
});

test('annotation banner labels declared step and links execution position',()=>{
 const h=(tag,attrs,...children)=>({tag,attrs,children:children.flat(Infinity).filter(x=>x!=null)});
 const ctx={h,D:{},icon:()=>null,bdi:x=>x};vm.createContext(ctx);
 vm.runInContext(section('  const foldContent =','  const errorViewer =')+'globalThis.banner=annotationBanner;',ctx);
 const nodes=[];const walk=x=>{if(x&&typeof x==='object'){nodes.push(x);x.children.forEach(walk);}};
 walk(ctx.banner([{kind:'known',state:'applied',step:2,executionStep:7,outcomes:[{step:2,outcome:'known'}]}],{id:'scenario'}));
 const link=nodes.find(x=>x.tag==='a');assert.equal(link.children[0],'Step 2');assert.equal(link.attrs.href,'#/scenarios/scenario?step=6');
 assert(nodes.some(x=>x.tag==='b'&&x.children[0]==='Step 2 (declared)'));
});

test('large timelines and selected print scopes stay bounded',()=>{
 const ctx={};vm.createContext(ctx);
 vm.runInContext(section('  const intervalBounds =','  let printSelection =')+'globalThis.bounds=intervalBounds;globalThis.plan=printPlan;',ctx);
 const rows=Array.from({length:150000},(_,i)=>({id:'s'+i,status:i===2?'failed':'passed',started:'2026-01-01T00:00:00Z',finished:'2026-01-01T00:00:01Z'}));
 assert.equal(ctx.bounds(rows).end-ctx.bounds(rows).start,1000);
 assert.equal(ctx.plan(rows,{scope:'summary'}).items.length,0);
 assert.equal(ctx.plan(rows,{scope:'failures'}).items.length,1);
 assert.equal(ctx.plan(rows,{scope:'filtered',ids:['s4','s7']}).items.length,2);
 assert.equal(ctx.plan(rows,null).blocked,true);
 assert.equal(ctx.plan(rows,null).items.length,0);
 assert.equal(ctx.plan(rows,{scope:'full',confirmed:true}).items.length,150000);
});

test('print preview retains scope after printing and rows retain native link semantics',()=>{
 let renders=0;const handlers={};const ctx={W:{addEventListener:(name,fn)=>handlers[name]=fn},printSelection:{scope:'failures'},printed:true,printRoot:{replaceChildren:()=>{},parentNode:null},doc:{body:{classList:{contains:()=>true}}},render:()=>{renders++}};
 vm.createContext(ctx);const line=source.split('\n').find(x=>x.includes("W.addEventListener('afterprint'"));vm.runInContext(line,ctx);
 handlers.afterprint();assert.equal(renders,1);assert.equal(ctx.printSelection.scope,'failures');assert.equal(ctx.printed,true);ctx.doc.body.classList.contains=()=>false;handlers.afterprint();assert.equal(ctx.printSelection,null);assert.equal(ctx.printed,false);
 const h=(tag,attrs,...children)=>({tag,attrs,children});const rowctx={h,scenarioAddress:()=>'/scenario',chip:()=>null,bdi:x=>x,hl:x=>x,firstLine:x=>x,strip:()=>null,dur:()=>'',icon:()=>null};vm.createContext(rowctx);
 vm.runInContext(section('  const srow =','  /* ---------- views')+'globalThis.row=srow;',rowctx);
 const row=rowctx.row({id:'s',name:'scenario',status:'passed',tags:[],steps:[]},[],false);
 assert.equal(row.attrs.role,'listitem');assert.equal(row.children[0].tag,'a');assert(!row.children[0].attrs.role);
});

test('print builder creates only selected scenario nodes, including limitation records',()=>{
 for(const scope of ['summary','failures','filtered','full']){
  const created=[];
  const h=(tag,attrs,...children)=>{const node={tag,attrs:attrs||{},children:children.flat(Infinity),querySelectorAll:()=>[],replaceChildren(...x){this.children=x;}};created.push(node);return node;};
  const counts={passed:4999,failed:1,known:0,skipped:0,pending:0,undefined:0,ambiguous:0};
  const f={name:'Feature',uri:'case.feature',status:'failed',n:counts,dur:0};
  const S=Array.from({length:5000},(_,i)=>({id:'s'+i,name:'Case '+i,status:i===2?'failed':'passed',f,kw:'Scenario',dur:0,tags:[],ann:[],steps:Array.from({length:7},()=>({vs:'passed',status:'passed',name:'step'}))}));f.sc=S;
  const selection=scope==='full'?null:{scope,ids:['s4'],errors:false,attachments:false};
  const ctx={h,S,executionLabel:list=>list.length+" scenarios",scenarioCount:S.length,runHookCount:0,F:[f],SB:Object.fromEntries(S.map(s=>[s.id,s])),D:{facts:[],sections:[]},T:{sc:counts,dur:0},STEPS:[],HEAD:{label:'Pass rate',value:99},ORDER:Object.keys(counts),ST:Object.fromEntries(Object.keys(counts).map(x=>[x,[x]])),LIMITS:[{scenario:'s2',title:'Limit'}],NOTAPPLIED:[],OOS:[],printSelection:selection,doc:{body:{append:()=>{}},styleSheets:[]},verdict:'Failed',tone:'fail',executed:5000,passRate:99,blocking:0,num:String,fpct:String,pct:()=>0,plural:(n,s)=>n+' '+s,dur:()=>'',bytes:String,bdi:x=>x,icon:()=>null,donut:()=>null,sbar:()=>null,glyphTxt:()=>'',firstLine:x=>x};
  vm.createContext(ctx);vm.runInContext(section('  const intervalBounds =','  let printSelection =')+section('  const printRoot =','  const glyphTxt =')+'buildPrint();',ctx);
  const scenarios=created.filter(x=>String(x.attrs.class||'').split(' ').includes('pr-sc'));
  assert.equal(scenarios.length,scope==='failures'||scope==='filtered'?1:0,scope);
 }
});

test('Unicode expansion highlights original characters without shifting later matches',()=>{
 const ctx={h:(tag,attrs,text)=>({tag,text})};vm.createContext(ctx);
 vm.runInContext(section('  const hl =','  /* tooltip */')+'globalThis.highlight=hl;',ctx);
 const pieces=ctx.highlight('İstanbul payment',['payment']);
 assert.equal(pieces.map(x=>typeof x==='string'?x:x.text).join(''),'İstanbul payment');
 assert.equal(pieces.find(x=>x&&x.tag==='mark').text,'payment');
 const expanded=ctx.highlight('İstanbul',['i']);assert.equal(expanded.find(x=>x&&x.tag==='mark').text,'İ');
});


test('run hooks affect failures while scenario totals stay separate',()=>{
 const D={features:[{id:'f',name:'Checks',status:'failed',elements:[...Array.from({length:4},(_,i)=>({id:'s'+i,name:'scenario',status:'passed',steps:[]})),{id:'hook',name:'AfterAll',status:'failed',runHook:true,steps:[{name:'hook',status:'failed'}]}]}]};
 const ctx={D};vm.createContext(ctx);
 vm.runInContext(section('  const SEV =','  const executed =')+'globalThis.result={scenarios:scenarioCount,hooks:runHookCount,failures:failedExecutionCount,counts:T.sc,feature:F[0]};',ctx);
 assert.equal(ctx.result.scenarios,4);assert.equal(ctx.result.hooks,1);assert.equal(ctx.result.failures,1);assert.equal(ctx.result.counts.failed,0);assert.equal(ctx.result.counts.passed,4);assert.equal(ctx.result.feature.n.failed,1);
});

test('folded Background and retry bookmarks resolve to the final scenario',()=>{
 const final={id:'f-L3-2'};
 const ctx={D:{aliases:{'f-L1':'f-L3-2','f-L3':'f-L3-2'}},SB:{'f-L3-2':final},VIEWS:{missing:()=>null},scenarioView:(s,q)=>({s,q})};
 vm.createContext(ctx);vm.runInContext(section('  const resolveScenarioID =','  const markNav =')+section('  VIEWS.scenarios =','  /* attachments and JSON tree */'),ctx);
 for(const id of ['f-L1','f-L3','f-L3-2'])assert.equal(ctx.VIEWS.scenarios([id],null).s,final);
 assert.equal(ctx.VIEWS.scenarios(['missing'],null),null);
 ctx.D.aliases['f-L3-2']='missing';assert.equal(ctx.VIEWS.scenarios(['f-L3-2'],null).s,final);
});

test('repeated in-page links reveal and focus targets without removing buttons from tab order',()=>{
 let focused=null,scrolled=null,listener,drawerOpen=true;const attrs={};const details=[{open:false}];
 const button={getAttribute:()=>attrs.expanded||'false',click(){attrs.expanded='true';},setAttribute(k,v){attrs[k]=v;},focus(){focused=this;},scrollIntoView(){scrolled=this;},classList:{remove(){},add(){}}};
 const row={hidden:true,querySelectorAll:s=>s==='details'?details:[button],setAttribute(k,v){this[k]=v;},focus(){focused=this;},scrollIntoView(){scrolled=this;},classList:{remove(){},add(){}}};
 const ctx={toggleSide:open=>{drawerOpen=open;},doc:{querySelector:()=>null,getElementById:()=>row,addEventListener:(name,fn)=>{listener=fn;}},requestAnimationFrame:fn=>fn(),reduced:{matches:true},location:{hash:'#/scenarios/s?step=3&evidence=0'},URLSearchParams};vm.createContext(ctx);
 vm.runInContext(section('  const focusStepTarget =',"  W.addEventListener('hashchange', render);")+ 'globalThis.reveal=focusStepTarget;',ctx);
 for(let i=0;i<2;i++){
  attrs.expanded='false';row.hidden=true;details[0].open=false;let prevented=false;
  listener({target:{closest:()=>({getAttribute:()=>ctx.location.hash})},button:0,preventDefault(){prevented=true;}});
  assert(prevented);assert.equal(drawerOpen,false);assert.equal(row.hidden,false);assert(details[0].open);assert.equal(focused,button);assert.equal(scrolled,button);assert.equal(attrs.expanded,'true');assert.equal(attrs.tabindex,undefined);
 }
 assert(source.includes('wrap.hidden=!expanded;wrap.inert=!expanded;'));
 assert(source.includes("node.querySelectorAll('details')"));
 assert(source.includes("foldContent('Attempt details'"));
});

test('evidence action includes hooks and earlier attempts without duplicating the final execution',()=>{
 const ctx={};vm.createContext(ctx);
 vm.runInContext(section('  const attemptSteps =','  const stepLabel =')+'globalThis.evidence=scenarioEvidence;',ctx);
 const failed={att:[]},hook={att:[{},{}]},s={steps:[hook,failed],bad:failed};
 let e=ctx.evidence(s,[{steps:[{att:[{}]}]},{useScenarioSteps:true}]);
 assert.equal(e.count,3);assert.equal(e.target.step,0);assert.equal(e.target.attempt,undefined);
 failed.att=[{}];e=ctx.evidence(s,[]);assert.equal(e.target.step,1);assert.equal(e.count,3);
 hook.att=[];failed.att=[];e=ctx.evidence(s,[{steps:[{att:[{}]}]},{useScenarioSteps:true}]);
 assert.equal(e.count,1);assert.equal(e.target.attempt,0);assert.equal(e.target.step,0);
 assert.equal(ctx.evidence(s,[]).target,null);
});

test('earlier-attempt evidence reselects its execution and opens collapsed ancestors',()=>{
 const ancestor={tagName:'DETAILS',open:false};let selected=false,focused=false;
 const button={getAttribute:()=> 'false',click(){},focus(){focused=true;},scrollIntoView(){},classList:{remove(){},add(){}}};
 const row={parentElement:ancestor,hidden:false,querySelectorAll:sel=>sel==='details'?[]:[button]};
 const ctx={doc:{querySelector:()=>({getAttribute:()=> 'false',click(){selected=true;}}),getElementById:id=>selected&&id==='attempt-0-step-1'?row:null,addEventListener(){}},requestAnimationFrame:fn=>fn(),reduced:{matches:true},URLSearchParams};vm.createContext(ctx);
 vm.runInContext(section('  const focusStepTarget =',"  W.addEventListener('hashchange', render);")+'focusStepTarget(new URLSearchParams("step=1&evidence=0&attempt=0"));',ctx);
 assert(selected);assert(ancestor.open);assert(focused);
});

test('dashboard ring covers every outcome proportionally, including skipped and known',()=>{
 const ctx={};vm.createContext(ctx);
 vm.runInContext(section('  const ORDER =','  const ANN =')+section('  const ringSegments =','  const outcomeRing =')+'globalThis.parts=ringSegments;globalThis.draw=ring;',ctx);
 const counts={passed:3,failed:1,known:2,ambiguous:1,undefined:1,pending:2,skipped:10};
 const parts=ctx.parts(counts);
 assert.equal(parts.length,7);
 assert.equal(parts.reduce((n,p)=>n+p.count,0),20);
 let offset=0;
 for(const p of parts){assert.equal(p.share,counts[p.status]/20);assert.equal(p.offset,offset);offset+=p.share;}
 assert(Math.abs(offset-1)<1e-12);
 ctx.sv=(tag,attrs,...children)=>({tag,attrs,children});ctx.reduced={matches:true};
 const svg=ctx.draw(counts,true), arcs=svg.children.slice(1);
 for(let i=0;i<arcs.length;i++){
  assert.equal(arcs[i].attrs.class,'ring-v f-'+parts[i].status);
  const [length,circumference]=arcs[i].attrs['stroke-dasharray'].split(' ').map(Number);
  assert(Math.abs(length/circumference-parts[i].share)<1e-12);
  assert(Math.abs(arcs[i].attrs['stroke-dashoffset']/circumference+parts[i].offset)<1e-12);
 }
 assert.equal(ctx.parts({}).length,0);
 assert.equal(ctx.draw({},true).children.length,1);
 for(const status of Object.keys(counts)){const only=ctx.parts({[status]:4});assert.equal(only.length,1);assert.equal(only[0].share,1);}
 const simple=ctx.parts({passed:3,failed:1});assert.equal(simple[0].share,.75);assert.equal(simple[1].share,.25);
 assert(source.includes("href: '#/scenarios?status=' + p.status"));
 assert(source.includes('Centre rate excludes skipped.'));
});
