const { open, tabbable, desc, sleep, click, key } = require('./lib.cjs');
const file = process.argv[2], id = process.argv[3];
const B = (d, l, r) => [...(r || d).querySelectorAll('button')].find((b) => b.textContent.trim().startsWith(l));
(async () => {
  const { w, d, log } = await open(file, '#/scenarios/' + encodeURIComponent(id));
  await sleep(20);
  const det = () => [...d.querySelectorAll('main details')];
  console.log('details initially', det().map((x) => (x.querySelector('summary').textContent.trim().slice(0, 18)) + ':' + x.open).join(' | '));
  for (let i = 1; i <= 5; i++) { const e = B(d, 'Expand all'); if (e.disabled) { console.log('Expand all disabled after press', i - 1); break; } click(w, e); await sleep(30); console.log('Expand all press', i, 'details open', det().filter((x) => x.open).length + '/' + det().length); }
  // JSON tree / Raw desync
  const find = d.querySelector('input[aria-label="Find in attachment"]');
  const tools = find.closest('.att-tools'), seg = tools.querySelector('.seg');
  console.log('seg present', !!seg);
  find.value = 'deep'; find.dispatchEvent(new w.Event('input')); await sleep(250);
  const c = find.closest('.att-b').querySelector('.att-c');
  console.log('after Find "deep": content', c.firstElementChild.tagName + '.' + c.firstElementChild.className, '| radios', [...seg.children].map((b) => b.textContent + '=' + b.getAttribute('aria-checked')).join(' '));
  find.value = ''; find.dispatchEvent(new w.Event('input')); await sleep(250);
  console.log('after clearing Find: content', c.firstElementChild.tagName + '.' + c.firstElementChild.className, '| radios', [...seg.children].map((b) => b.textContent + '=' + b.getAttribute('aria-checked')).join(' '));
  // Collapse all: then is the doc-string Copy / table tabbable?
  click(w, B(d, 'Collapse all')); await sleep(30);
  console.log('after Collapse all: tabbable inside .steps', tabbable(d, d.querySelector('.steps')).map(desc).join(' ; '));
  // Long error: "Show next 1,000 lines" never disables
  const more = [...d.querySelectorAll('.more-b')].find((b) => /next/.test(b.textContent));
  if (more) { for (let i = 0; i < 4; i++) click(w, more); await sleep(10); const pre = more.parentElement.querySelector('.error-lines');
    console.log('long error: after 4 presses lines shown', pre.querySelectorAll('.error-line').length, '| button still enabled', !more.disabled, 'hidden', more.hidden, 'text', more.textContent); }
  // Expand error (13..1000 lines) / Collapse all interplay done; "Line numbers" toggle has no initial aria-pressed
  const ln = B(d, 'Line numbers'); console.log('Line numbers aria-pressed initially', ln.getAttribute('aria-pressed'));
  // Problems-only + jump to passed step via aside/banner -> seg desync
  const po = [...d.querySelectorAll('.step-controls .seg button')].find((b) => b.textContent === 'Problems only'); click(w, po); await sleep(10);
  console.log('Problems only: hidden steps', [...d.querySelectorAll('.steps > li')].filter((l) => l.hidden).map((l) => l.id).join(','));
  const pal = d.querySelector('.pal-in'); key(w, d.body, '/'); pal.value = 'a table'; pal.dispatchEvent(new w.Event('input'));
  const o = [...d.querySelectorAll('.pal-o')].find((x) => /step=0$/.test(x.getAttribute('href'))); click(w, o); await sleep(80);
  console.log('palette -> step 0 (passed) while Problems only: hash', w.location.hash, '| rebuilt (seg reset)?', [...d.querySelectorAll('.step-controls .seg button')].map((b) => b.textContent + '=' + b.getAttribute('aria-checked')).join(' '), '| step-0 hidden', d.getElementById('step-0').hidden, '| focus', desc(d.activeElement));
  console.log('errors', log.errors);
})();
