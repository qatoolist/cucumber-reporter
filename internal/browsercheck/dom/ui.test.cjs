// Reviewer D2's independent real-DOM checks, promoted from console probes to assertions.
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('fs');
const {open,tabbable,sleep,click,key}=require('./lib.cjs');
const reports=JSON.parse(process.env.CR_DOM_REPORTS||'{}');
const data=file=>JSON.parse(fs.readFileSync(file,'utf8').match(/id="report-data">([\s\S]*?)<\/script>/)[1]);
const scenario=(file,failed=true)=>data(file).features.flatMap(f=>f.elements).find(s=>!failed||s.status==='failed');
const button=(d,text)=>[...d.querySelectorAll('button')].find(b=>b.textContent.trim()===text);
const evidenceLink=d=>[...d.querySelectorAll('a.btn')].find(a=>a.textContent.startsWith('View evidence'));
async function load(t,name,hash,opts){const r=await open(reports[name],hash,opts);t.after(()=>{assert.deepEqual(r.log.errors,[]);r.dom.window.close();});return r;}
for(const name of ['retry','enhanced'])test(name+': evidence counted once, image target, repeated reveals',async t=>{
 const s=scenario(reports[name]),{w,d,log}=await load(t,name,'#/scenarios/'+s.id);
 const a=evidenceLink(d);assert(a);assert.match(a.textContent,/View evidence \(15\)/);
 assert.match(a.textContent,/png|screenshot|image/i);
 for(let n=0;n<3;n++){
  click(w,button(d,'Collapse all'));await sleep(25);log.scroll.length=0;click(w,evidenceLink(d));await sleep(70);
  assert(d.activeElement.matches('.att-h'));assert.equal(d.activeElement.getAttribute('aria-expanded'),'true');assert(log.scroll.length>0);
 }
});
test('all-details controls expand lazy descendants and collapse keyboard access',async t=>{
 const s=scenario(reports.attachments,false),{w,d}=await load(t,'attachments','#/scenarios/'+s.id);
 assert(d.querySelectorAll('.att-h').length>0);for(const a of d.querySelectorAll('.att-h'))assert(d.getElementById(a.getAttribute('aria-controls')));
 button(d,'Expand all').focus();click(w,button(d,'Expand all'));await sleep(60);
 assert([...d.querySelectorAll('main details')].every(x=>x.open));assert([...d.querySelectorAll('main .att-h')].every(x=>x.getAttribute('aria-expanded')==='true'));
 assert.equal(button(d,'Expand all').disabled,true);assert.notEqual(d.activeElement,d.body);
 button(d,'Collapse all').focus();click(w,button(d,'Collapse all'));await sleep(40);
 assert([...d.querySelectorAll('main details')].every(x=>!x.open));assert([...d.querySelectorAll('main .att-h')].every(x=>x.getAttribute('aria-expanded')==='false'));
 assert(!tabbable(d,d.querySelector('main')).some(x=>x.closest('.att-tools')));assert.notEqual(d.activeElement,d.body);
 for(const e of d.querySelectorAll('.error-lines'))if(e.textContent.split('\n').length<=12)assert(!e.classList.contains('clip'));
});
test('filter chips, clear and sort retain keyboard focus',async t=>{
 const {w,d}=await load(t,'showcase','#/scenarios');
 const chip=d.querySelector('.fchip.s-failed');chip.focus();click(w,chip);assert.equal(d.activeElement.dataset.focusKey,'status-failed');
 const clear=button(d,'Clear filters');assert(clear);clear.focus();click(w,clear);assert.notEqual(d.activeElement,d.body);
 click(w,button(d,'Table'));const sort=d.querySelector('[data-focus-key="sort-duration"]');sort.focus();click(w,sort);assert.equal(d.activeElement.dataset.focusKey,'sort-duration');
});
test('legend and navigation collapse keep focus and correct labels',async t=>{
 const {w,d}=await load(t,'showcase','#/dashboard');
 for(const selector of ['.c-status .leg','.c-feat .chipleg .leg','.mini-btn']){
  const b=d.querySelector(selector);assert(b,selector);b.focus();click(w,b);assert.notEqual(d.activeElement,d.body);assert(d.activeElement.isConnected);
 }
 assert.equal(d.querySelector('.mini-btn').title,'Expand navigation');
});
test('mobile drawer is inert when closed and returns focus',async t=>{
 const {w,d}=await load(t,'showcase','#/dashboard',{before(w){w.matchMedia=q=>({matches:q.includes('1024')||q.includes('reduce'),addEventListener(){},removeEventListener(){}});}});
 const side=d.querySelector('#side'),menu=d.querySelector('.menu-btn');assert(side.inert);assert.equal(tabbable(d,side).length,0);
 menu.focus();click(w,menu);assert(!side.inert);assert(side.contains(d.activeElement));key(w,d.activeElement,'Escape');assert(side.inert);assert.equal(d.activeElement,menu);
});
test('palette Enter repeats the current step navigation',async t=>{
 const s=scenario(reports.attachments,false),{w,d,log}=await load(t,'attachments','#/scenarios/'+s.id+'?step=0');
 for(let n=0;n<2;n++){
  key(w,d.body,'/');const pin=d.querySelector('.pal-in');pin.value=d.querySelector('#step-0 .txt').textContent.trim();pin.dispatchEvent(new w.Event('input'));
  const opts=[...d.querySelectorAll('.pal-o')],target=opts.find(x=>x.getAttribute('href')===w.location.hash);assert(target);
  for(let tries=0;d.querySelector('.pal-o[aria-selected=true]')!==target&&tries<opts.length;tries++)key(w,pin,'ArrowDown');
  log.scroll.length=0;key(w,pin,'Enter');await sleep(70);assert(log.scroll.length>0);assert.equal(d.activeElement.id,'step-0');
 }
});
test('JSON Find selects Raw and maintains radio semantics',async t=>{
 const s=scenario(reports.attachments,false),{w,d}=await load(t,'attachments','#/scenarios/'+s.id);
 click(w,button(d,'Expand all'));await sleep(40);
 const tools=[...d.querySelectorAll('.att-tools')].find(x=>x.querySelector('[role=radio]'));assert(tools);
 const search=tools.querySelector('input');search.value='a';search.dispatchEvent(new w.Event('input'));await sleep(220);
 assert.equal([...tools.querySelectorAll('[role=radio]')].find(x=>x.getAttribute('aria-checked')==='true').textContent,'Raw');
});
test('deep lazy JSON opens completely on the first Expand all',async t=>{
 const s=scenario(reports.deep,false),{w,d}=await load(t,'deep','#/scenarios/'+s.id);
 click(w,button(d,'Expand all'));await sleep(70);
 const details=[...d.querySelectorAll('main details')];assert(details.length>8);assert(details.every(x=>x.open));assert.equal(button(d,'Expand all').disabled,true);
});
test('attachment collapse alone removes descendants from the tab order',async t=>{
 const s=scenario(reports.attachments,false),{w,d}=await load(t,'attachments','#/scenarios/'+s.id);
 const header=[...d.querySelectorAll('.att-h')].find(x=>x.textContent.includes('application/json'));assert(header);
 for(let i=0;i<2;i++){
  click(w,header);await sleep(25);assert.equal(header.getAttribute('aria-expanded'),'true');
  const panel=d.getElementById(header.getAttribute('aria-controls'));assert(panel);assert(tabbable(d,panel).length>0);
  click(w,header);await sleep(25);assert.equal(header.getAttribute('aria-expanded'),'false');assert(panel.hidden);assert(panel.inert);assert.equal(tabbable(d,panel).length,0);
 }
});
test('long error bulk controls and final batch disablement',async t=>{
 const s=scenario(reports.deep),{w,d}=await load(t,'deep','#/scenarios/'+s.id);
 const error=d.querySelector('.steps .error-lines');assert(error.classList.contains('clip'));
 click(w,button(d,'Expand all'));await sleep(25);assert(!error.classList.contains('clip'));
 click(w,button(d,'Collapse all'));await sleep(25);assert(error.classList.contains('clip'));
 click(w,button(d,'Expand all'));await sleep(25);
 const more=[...d.querySelectorAll('.steps button')].find(x=>x.textContent.includes('Show next 1,000'));assert(more);more.focus();click(w,more);
 assert.equal(error.querySelectorAll('.error-line').length,1501);assert(more.disabled);assert.equal(d.activeElement.getAttribute('aria-label'),'Find in error');
 assert.equal(button(d,'Line numbers').getAttribute('aria-pressed'),'false');
});
test('new-page navigation focuses the heading rather than a recycled link',async t=>{
 const s=scenario(reports.attachments,false),{w,d}=await load(t,'attachments','#/scenarios/'+s.id);
 const feature=[...d.querySelectorAll('main a')].find(x=>x.getAttribute('href')?.startsWith('#/features/'));feature.focus();click(w,feature);await sleep(70);
 assert.equal(d.activeElement,d.querySelector('.page-t'));
});
test('empty filtered lists and Clear filters retain usable focus',async t=>{
 const {w,d}=await load(t,'bulk','#/scenarios');const search=d.querySelector('[aria-label="Filter scenarios"]');
 search.value='"bulk scenario 0"';search.dispatchEvent(new w.Event('input'));await sleep(90);
 const failed=d.querySelector('.fchip.s-failed');failed.focus();click(w,failed);await sleep(25);
 assert(d.querySelector('main').textContent.includes('No scenarios match'));assert.equal(d.activeElement.dataset.focusKey,'status-failed');
 const clear=button(d,'Clear filters');assert(clear);clear.focus();click(w,clear);assert.notEqual(d.activeElement,d.body);assert(d.activeElement.isConnected);
});
test('large lists keep constant listener counts and dispose on navigation',async t=>{
 const {w,d,log}=await load(t,'bulk','#/scenarios');assert(d.querySelectorAll('.rows .row').length<100);
 const initial={...log.listeners};assert.equal(initial.scroll,2);assert.equal(initial.resize,1);
 for(let i=0;i<10;i++){click(w,d.querySelector('.fchip.s-failed'));await sleep(20);assert.deepEqual(log.listeners,initial);}
 const search=d.querySelector('[aria-label="Filter scenarios"]');
 for(const text of ['b','bu','bul','bulk','bulk s','bulk sc']){search.value=text;search.dispatchEvent(new w.Event('input'));await sleep(90);assert.deepEqual(log.listeners,initial);}
 const row=d.querySelector('.rows .row');click(w,row);await sleep(60);assert.equal(log.listeners.scroll,1);assert.equal(log.listeners.resize,0);
});

test('all programmatic hash navigation uses the shared reveal path',()=>{
 const source=fs.readFileSync(require('node:path').join(__dirname,'../../../report/assets/app.js'),'utf8');
 const assignments=[...source.matchAll(/location\.hash\s*=(?!=)/g)].map(m=>m.index);
 const start=source.indexOf('const navigate ='),end=source.indexOf('doc.addEventListener',start);
 assert(start>=0&&end>start);assert.equal(assignments.length,1);assert(assignments.every(i=>i>start&&i<end));
});

test('modal Tab boundaries wrap over visible enabled controls and restore the opener',async t=>{
 const s=scenario(reports.deep),{w,d}=await load(t,'deep','#/scenarios/'+s.id);
 const opener=button(d,'Full screen');opener.focus();click(w,opener);await sleep(20);
 const dialog=d.querySelector('dialog[aria-label="Error details"]');assert(dialog&&dialog.open);
 // Put disallowed controls after the real last control, so a simplistic selector
 // would move focus to one of them rather than wrapping to the Close button.
 const hidden=d.createElement('div');hidden.hidden=true;hidden.append(d.createElement('button'));dialog.append(hidden);
 const inert=d.createElement('div');inert.inert=true;inert.append(d.createElement('button'));dialog.append(inert);
 const disabled=d.createElement('button');disabled.disabled=true;dialog.append(disabled);
 const negative=d.createElement('button');negative.tabIndex=-1;dialog.append(negative);
 const folded=d.createElement('details'),summary=d.createElement('summary'),foldedButton=d.createElement('button');summary.textContent='Closed content';folded.append(summary,foldedButton);dialog.append(folded);
 const first=dialog.querySelector('button');
 const press=(element,shift)=>{element.focus();const event=new w.KeyboardEvent('keydown',{key:'Tab',shiftKey:shift,bubbles:true,cancelable:true});element.dispatchEvent(event);assert(event.defaultPrevented);};
 // A closed details summary itself is tabbable; its descendants are not.
 press(first,true);assert.equal(d.activeElement,summary);
 press(summary,false);assert.equal(d.activeElement,first);
 folded.hidden=true;
 const last=[...dialog.querySelectorAll('button')].find(x=>x.textContent.includes('Show next 1,000'));assert(last);
 press(first,true);assert.equal(d.activeElement,last);press(last,false);assert.equal(d.activeElement,first);
 // jsdom has no native Escape default action: emulate cancellation, then close,
 // exactly as the browser does. The app's close handler must restore the opener.
 const escape=new w.KeyboardEvent('keydown',{key:'Escape',bubbles:true,cancelable:true});first.dispatchEvent(escape);assert(!escape.defaultPrevented);
 const cancel=new w.Event('cancel',{cancelable:true});if(dialog.dispatchEvent(cancel))dialog.close();await sleep(30);assert.equal(d.activeElement,opener);
});
test('virtual list return restores the clicked row after delayed scroll and layout change',async t=>{
 const {w,d}=await load(t,'bulk','#/scenarios',{before(w){
  let y=0,origin=420;const height=56;
  Object.defineProperty(w,'scrollY',{get:()=>y});
  Object.defineProperty(w,'innerHeight',{value:800});
  // Model viewport-relative geometry and delayed native scroll delivery.
  w.__setScroll=(next,emit=true)=>{y=next;if(emit)w.dispatchEvent(new w.Event('scroll'));};
  w.__moveListOrigin=delta=>{origin+=delta;};
  w.scrollTo=(_x,next)=>{y=next;setTimeout(()=>w.dispatchEvent(new w.Event('scroll')),0);};
  const original=w.Element.prototype.getBoundingClientRect;
  w.Element.prototype.getBoundingClientRect=function(){
   if(this.matches('.vl'))return {top:origin-y,height:parseFloat(this.style.height)||0,bottom:origin-y+(parseFloat(this.style.height)||0)};
   if(this.matches('.row,[role=listitem]')){
    const item=this.matches('.row')?this.parentElement:this;
    const offset=+(item.parentElement.style.transform.match(/translateY\(([-\d.]+)px\)/)||[])[1]||0;
    const index=[...item.parentElement.children].indexOf(item);
    return {top:origin+offset+index*height-y,bottom:origin+offset+(index+1)*height-y,height};
   }
   return original.call(this);
  };
 }});
 w.__setScroll(1500);await sleep(40);
 // A scrollIntoView immediately before navigation may not have emitted scroll.
 w.__setScroll(1612,false);
 const row=[...d.querySelectorAll('.rows .row')].find(x=>{const r=x.getBoundingClientRect();return r.top>250&&r.bottom<600;});assert(row);
 const id=row.dataset.id,top=row.getBoundingClientRect().top;row.focus();click(w,row);await sleep(70);
 assert(d.querySelector('.v-sc'));w.__moveListOrigin(35);
 const back=[...d.querySelectorAll('main a')].find(x=>x.textContent.startsWith('Back to '));assert(back);click(w,back);await sleep(120);
 const restored=d.querySelector('.row[data-id="'+id+'"]');assert(restored);assert.equal(restored.getBoundingClientRect().top,top);assert.equal(d.activeElement,restored);
 const stable=w.scrollY;await sleep(100);assert.equal(w.scrollY,stable);
 click(w,restored);await sleep(60);assert(d.querySelector('.v-sc'));w.history.back();await sleep(120);
 const again=d.querySelector('.row[data-id="'+id+'"]');assert(again);assert.equal(again.getBoundingClientRect().top,top);assert.equal(d.activeElement,again);
});

function revealed(target,d){
 assert(target&&target!==d.body,'specific reveal target required');
 for(let n=target;n&&n!==d.body;n=n.parentElement){
  assert(!n.hidden,'hidden reveal ancestor');assert(!n.inert,'inert reveal ancestor');
  assert.notEqual(n.getAttribute('aria-hidden'),'true','aria-hidden reveal ancestor');
  if(n.tagName==='DETAILS')assert(n.open,'closed details ancestor');
 }
}
for(const name of ['enhanced','earlier'])test(name+': exact reveal target stays visible across attempts and filters',async t=>{
 const s=scenario(reports[name]),{w,d,log}=await load(t,name,'#/scenarios/'+s.id);
 const link=evidenceLink(d);assert(link);const href=link.getAttribute('href'),q=new URLSearchParams(href.split('?')[1]);
 const targetRow=(q.has('attempt')?'attempt-'+q.get('attempt')+'-step-':'step-')+q.get('step');
 for(let i=0;i<2;i++){
  const options=[...d.querySelectorAll('.attempts [data-attempt-index]')];
  const other=options.find(x=>x.dataset.attemptIndex!==q.get('attempt'));if(other)click(w,other);
  click(w,button(d,'Collapse all'));const problems=[...d.querySelectorAll('.step-controls [role=radio]')].find(x=>x.textContent==='Problems only');if(problems)click(w,problems);
  log.scroll.length=0;click(w,evidenceLink(d));await sleep(90);
  const row=d.getElementById(targetRow);assert(row);const target=row.querySelectorAll('.att-h')[Number(q.get('evidence'))];
  assert.equal(d.activeElement,target);assert.equal(log.scroll.at(-1),target);revealed(target,d);
 }
});
for(const fixture of ['empty','emptyMessages'])test(fixture+': empty execution renders an informational report',async t=>{
 const {d}=await load(t,fixture,'#/dashboard');assert.match(d.querySelector('main').textContent,/No scenarios executed/);assert(!d.querySelector('main').textContent.includes('could not be read'));
});
test('same-page mobile navigation closes and hides drawer',async t=>{
 const {w,d}=await load(t,'showcase','#/dashboard',{before(w){w.matchMedia=q=>({matches:q.includes('1024')||q.includes('reduce'),addEventListener(){},removeEventListener(){}});}});
 const side=d.querySelector('#side'),menu=d.querySelector('.menu-btn');click(w,menu);
 click(w,side.querySelector('a[href="#/dashboard"]'));await sleep(50);
 assert(side.inert);assert.equal(side.getAttribute('aria-hidden'),'true');assert.equal(menu.getAttribute('aria-expanded'),'false');assert.equal(d.activeElement,d.querySelector('.page-t'));
});
test('Settings switches do not throw and keep a live control focused',async t=>{
 const {w,d}=await load(t,'showcase','#/dashboard');
 click(w,d.querySelector('[aria-label="Settings"]'));
 for(const control of [...d.querySelectorAll('dialog[open] input[type=checkbox]:not(:disabled)')]){control.focus();click(w,control);assert(control.isConnected);assert.equal(d.activeElement,control);}
});
test('eager failed-step attachment collapses hidden and inert together',async t=>{
 const s=scenario(reports.eager),{w,d}=await load(t,'eager','#/scenarios/'+s.id);
 const header=[...d.querySelectorAll('.steps .att-h')].find(x=>x.getAttribute('aria-expanded')==='true');assert(header);
 click(w,header);const panel=d.getElementById(header.getAttribute('aria-controls'));assert(panel.hidden);assert(panel.inert);assert.equal(tabbable(d,panel).length,0);
});
test('single-attempt scenario has no duplicate execution card',async t=>{
 const s=scenario(reports.attachments,false),{d}=await load(t,'attachments','#/scenarios/'+s.id);assert.equal(d.querySelectorAll('.attempts').length,0);
});
test('passed step reveal exits Problems-only without rebuilding scenario',async t=>{
 const s=scenario(reports.attachments,false),{w,d,log}=await load(t,'attachments','#/scenarios/'+s.id);
 click(w,button(d,'Collapse all'));const controls=d.querySelector('.step-controls');
 click(w,[...controls.querySelectorAll('[role=radio]')].find(x=>x.textContent==='Problems only'));
 assert(d.getElementById('step-0').hidden);
 const a=d.createElement('a');a.href='#/scenarios/'+s.id+'?step=0';d.querySelector('main').append(a);click(w,a);await sleep(90);
 assert.equal(d.querySelector('.step-controls'),controls,'query navigation rebuilt scenario');
 assert.equal(d.activeElement,d.getElementById('step-0'));assert.equal(log.scroll.at(-1),d.activeElement);revealed(d.activeElement,d);
 assert.equal(controls.querySelector('[aria-checked=true]').textContent,'All steps');
});
test('Settings selectors and reset preserve focus inside their dialog',async t=>{
 const {w,d}=await load(t,'showcase','#/dashboard');click(w,d.querySelector('[aria-label="Settings"]'));
 let dialog=d.querySelector('dialog[open]');
 for(const label of ['Theme','Density','Open attachments']) {
  const group=dialog.querySelector('[aria-label="'+label+'"]');assert(group,label);
  for(const option of [...group.querySelectorAll('[role=radio]')]){option.focus();click(w,option);assert(dialog.contains(d.activeElement));}
 }
 const start=dialog.querySelector('select');start.focus();start.value='scenarios';start.dispatchEvent(new w.Event('change',{bubbles:true}));assert.equal(d.activeElement,start);
 const reset=[...dialog.querySelectorAll('button')].find(x=>x.textContent==='Reset to report defaults');reset.focus();click(w,reset);await sleep(30);
 assert(dialog.contains(d.activeElement));assert.equal(d.activeElement.textContent,'Reset to report defaults');
});
const mobile=w=>{w.matchMedia=q=>({matches:q.includes('1024')||q.includes('reduce'),addEventListener(){},removeEventListener(){}});};
const checked=(d)=>(d.querySelector('.attempts [aria-checked=true]')||{}).textContent;
test('H4-N1: Expand all and Collapse all reach every attempt of a 3-attempt scenario',async t=>{
 const s=scenario(reports.enhanced),{w,d}=await load(t,'enhanced','#/scenarios/'+s.id);
 assert.equal(d.querySelectorAll('.attempts [data-attempt-index]').length,3);
 const attempts=()=>[...d.querySelectorAll('.attempts details')];assert(attempts().length>0);
 click(w,button(d,'Expand all'));await sleep(60);
 assert(attempts().every(x=>x.open));assert.equal(button(d,'Expand all').disabled,true);assert.equal(button(d,'Collapse all').disabled,false);
 click(w,button(d,'Collapse all'));await sleep(60);
 assert(attempts().every(x=>!x.open));assert.equal(button(d,'Collapse all').disabled,true);assert.equal(button(d,'Expand all').disabled,false);
});
test('H4-N2: a list-view query hashchange re-renders the list',async t=>{
 const {w,d}=await load(t,'showcase','#/scenarios');
 const before=d.querySelector('main').textContent;assert(!d.querySelector('.fchip.s-failed[aria-pressed=true]'));
 w.location.hash='#/scenarios?status=failed';await sleep(60);
 assert(d.querySelector('.fchip.s-failed[aria-pressed=true]'),'list not re-rendered from the new query');
 assert.notEqual(d.querySelector('main').textContent,before);
});
test('H4-N2: Back within a mounted scenario closes an open drawer',async t=>{
 const s=scenario(reports.attachments,false),{w,d}=await load(t,'attachments','#/scenarios/'+s.id,{before:mobile});
 w.location.hash='#/scenarios/'+s.id+'?step=0';await sleep(60);
 const side=d.querySelector('#side'),menu=d.querySelector('.menu-btn');click(w,menu);assert(!side.inert);assert.equal(menu.getAttribute('aria-expanded'),'true');
 w.history.back();await sleep(60);
 assert(side.inert);assert.equal(side.getAttribute('aria-hidden'),'true');assert.equal(menu.getAttribute('aria-expanded'),'false');
});
test('H4-N2: Back to the bare scenario focuses the page heading',async t=>{
 const s=scenario(reports.attachments,false),{w,d}=await load(t,'attachments','#/scenarios/'+s.id);
 w.location.hash='#/scenarios/'+s.id+'?step=0';await sleep(60);assert.equal(d.activeElement.id,'step-0');
 w.history.back();await sleep(60);
 assert.equal(d.activeElement,d.querySelector('h1.page-t'));
});
test('H4-N3: Settings changes keep focus in the dialog and keep scenario state and position',async t=>{
 const s=scenario(reports.enhanced),{w,d,log}=await load(t,'enhanced','#/scenarios/'+s.id);
 const href=evidenceLink(d).getAttribute('href');w.location.hash=href;await sleep(90);
 const controls=d.querySelector('.step-controls'),other=[...d.querySelectorAll('.attempts [data-attempt-index]')].find(x=>x.getAttribute('aria-checked')!=='true');assert(other);
 click(w,other);click(w,button(d,'Collapse all'));
 click(w,[...controls.querySelectorAll('[role=radio]')].find(x=>x.textContent==='Problems only'));await sleep(40);
 const attempt=checked(d),scrolls=log.scroll.length,opener=d.querySelector('[aria-label="Settings"]');click(w,opener);
 const dialog=d.querySelector('dialog[open]');
 const state=()=>{assert.equal(d.querySelector('.step-controls'),controls,'scenario rebuilt');assert.equal(controls.querySelector('[aria-checked=true]').textContent,'Problems only');
  assert.equal(checked(d),attempt);assert([...d.querySelectorAll('main details')].filter(x=>!x.closest('.acc')).every(x=>!x.open),'Collapse all state lost');assert.equal(log.scroll.length,scrolls,'reveal re-ran');assert.equal(w.location.hash,href);
  assert(dialog.contains(d.activeElement),'focus left the dialog');};
 for(const c of [...dialog.querySelectorAll('input,[role=radio],button,select')].filter(x=>!/Close/.test(x.getAttribute('aria-label')||x.textContent)&&!/Reset/.test(x.textContent))){
  if(!c.isConnected)continue;c.focus();
  if(c.tagName==='SELECT'){c.selectedIndex=(c.selectedIndex+1)%c.options.length;c.dispatchEvent(new w.Event('change',{bubbles:true}));}else click(w,c);
  await sleep(30);state();
 }
 const reset=[...dialog.querySelectorAll('button')].find(x=>x.textContent==='Reset to report defaults');reset.focus();click(w,reset);await sleep(60);state();
 // Any other same-path render (here: afterprint) must not re-run the step reveal either.
 d.body.classList.add('show-print');w.dispatchEvent(new w.Event('afterprint'));await sleep(60);
 assert.equal(log.scroll.length,scrolls,'afterprint re-render re-ran the reveal');
});
test('H4-N4: a query hashchange on a mounted scenario selects the requested attempt',async t=>{
 const s=scenario(reports.enhanced),{w,d}=await load(t,'enhanced','#/scenarios/'+s.id+'?attempt=0');
 assert.equal(d.querySelector('.attempts [aria-checked=true]').dataset.attemptIndex,'0');
 w.location.hash='#/scenarios/'+s.id+'?attempt=1';await sleep(60);
 assert.equal(d.querySelector('.attempts [aria-checked=true]').dataset.attemptIndex,'1');
 w.location.hash='#/scenarios/'+s.id+'?step=0&attempt=2';await sleep(80);
 assert.equal(d.querySelector('.attempts [aria-checked=true]').dataset.attemptIndex,'2');
 assert.equal(d.activeElement.id,'attempt-2-step-0');
});
test('H4-N4: a missing attempt row falls back to the scenario step',async t=>{
 const s=scenario(reports.attachments,false),{w,d}=await load(t,'attachments','#/scenarios/'+s.id);
 assert(!d.getElementById('attempt-0-step-0'));
 w.location.hash='#/scenarios/'+s.id+'?step=0&attempt=0';await sleep(80);
 assert.equal(d.activeElement.id,'step-0');
});
test('H4-N3: Open attachments applies in place on a mounted 3-attempt scenario, also on Reset',async t=>{
 const s=scenario(reports.enhanced),{w,d}=await load(t,'enhanced','#/scenarios/'+s.id);
 const controls=d.querySelector('.step-controls'),other=[...d.querySelectorAll('.attempts [data-attempt-index]')].find(x=>x.getAttribute('aria-checked')!=='true');
 click(w,other);click(w,[...controls.querySelectorAll('[role=radio]')].find(x=>x.textContent==='Problems only'));await sleep(40);
 const attempt=checked(d);click(w,d.querySelector('[aria-label="Settings"]'));const dialog=d.querySelector('dialog[open]');
 const heads=()=>[...d.querySelectorAll('.steps .att-h')];assert(heads().length>3);
 const expected=mode=>[...d.querySelectorAll('.steps > li')].flatMap((li,k)=>{const st=s.steps[k];return [...li.querySelectorAll('.atts > .att-x')].map((x,i)=>mode==='all'||(mode==='failed'&&i===0&&st.status==='failed'&&!!st.error));});
 const open=()=>heads().map(b=>b.getAttribute('aria-expanded')==='true');
 const check=mode=>{
  const want=expected(mode);assert.deepEqual(open(),want,mode);
  for(const b of heads()){const p=d.getElementById(b.getAttribute('aria-controls')),o=b.getAttribute('aria-expanded')==='true';if(o){assert(!p.hidden&&!p.inert);}else{assert(p.hidden&&p.inert);}}
  assert.equal(d.querySelector('.step-controls'),controls);assert.equal(controls.querySelector('[aria-checked=true]').textContent,'Problems only');
  assert.equal(checked(d),attempt);assert(dialog.contains(d.activeElement));
 };
 const choose=async label=>{const o=[...dialog.querySelectorAll('[aria-label="Open attachments"] [role=radio]')].find(x=>x.textContent===label);o.focus();click(w,o);await sleep(40);};
 await choose('None');check('none');assert(!open().some(Boolean));
 await choose('All');check('all');assert(open().every(Boolean));
 await choose('Failed steps');check('failed');
 await choose('All');check('all');
 const reset=[...dialog.querySelectorAll('button')].find(x=>x.textContent==='Reset to report defaults');reset.focus();click(w,reset);await sleep(60);
 check('failed');
});
test('H4-N3: Open attachments Failed steps reopens the failed step attachment in place',async t=>{
 const s=scenario(reports.eager),{w,d}=await load(t,'eager','#/scenarios/'+s.id);
 click(w,d.querySelector('[aria-label="Settings"]'));const dialog=d.querySelector('dialog[open]');
 const choose=async label=>{const o=[...dialog.querySelectorAll('[aria-label="Open attachments"] [role=radio]')].find(x=>x.textContent===label);o.focus();click(w,o);await sleep(40);assert(dialog.contains(d.activeElement));};
 const open=()=>[...d.querySelectorAll('.steps .att-h')].map(b=>b.getAttribute('aria-expanded')==='true');
 const initial=open();assert(initial.some(Boolean));
 await choose('None');assert(!open().some(Boolean));
 await choose('Failed steps');assert.deepEqual(open(),initial);
});
const attemptLines=async(w,d)=>{const out=[];for(const b of [...d.querySelectorAll('.attempts [data-attempt-index]')]){click(w,b);await sleep(10);out.push({label:b.textContent,body:d.querySelector('.attempt-body p').textContent});}return out;};
test('attempts show measured start, finish and elapsed time when the runner records them',async t=>{
 const {w,d}=await load(t,'messagesRetry','#/scenarios/checkout-55a03ad5f369-L31');
 const lines=await attemptLines(w,d);assert.equal(lines.length,3);
 assert.equal(lines[0].body,'Attempt 1: Failed · Started 03:10:51.148364 UTC · Finished 03:10:51.154157 UTC · 5.8 ms elapsed');
 assert.equal(lines[2].body,'Attempt 3: Failed · Started 03:10:51.158285 UTC · Finished 03:10:51.160796 UTC · 2.5 ms elapsed');
 assert.deepEqual(lines.map(x=>x.label),['Attempt 1 · failed · 03:10:51.148 UTC','Attempt 2 · failed · 03:10:51.154 UTC','Attempt 3 · failed · 03:10:51.158 UTC']);
 const time=d.querySelector('.attempt-body time');assert(time);assert.equal(time.getAttribute('datetime'),'2026-11-07T03:10:51.158285Z');
});
test('attempts without timestamps show their own step time, never an invented clock',async t=>{
 const s=scenario(reports.enhanced),ex=data(reports.enhanced).execution[s.id];const {w,d}=await load(t,'enhanced','#/scenarios/'+s.id);
 const lines=await attemptLines(w,d);assert.equal(lines.length,3);
 // Each attempt's own steps: 77, 94 and 92 µs in the captured Ruby retry (the final attempt reuses the scenario's steps).
 assert.deepEqual(ex.map(a=>(a.useScenarioSteps?s.steps:a.steps).reduce((n,st)=>n+(st.duration||0),0)),[77000,94000,92000]);
 assert.deepEqual(lines.map(x=>x.body),['77 µs','94 µs','92 µs'].map((v,i)=>'Attempt '+(i+1)+': Failed · Step time '+v+' (no start or finish time recorded)'));
 assert.deepEqual(lines.map(x=>x.label),['Attempt 1 · failed','Attempt 2 · failed','Attempt 3 · failed']);
});

