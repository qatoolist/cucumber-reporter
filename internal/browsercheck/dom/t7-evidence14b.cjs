const { open, desc, sleep, click } = require('./lib.cjs');
(async () => {
  const [file, id] = process.argv.slice(2);
  const { w, d, log } = await open(file, '#/scenarios/' + encodeURIComponent(id)); await sleep(30);
  const find = (l) => [...d.querySelectorAll('a.btn')].find((b) => b.textContent.startsWith(l));
  const ev = find('View evidence');
  console.log(id, '| label', ev && ev.textContent, '| href', ev && ev.getAttribute('href'), '| attempts seg', [...d.querySelectorAll('.attempts [role=radio]')].map((b) => b.textContent + (b.getAttribute('aria-checked') === 'true' ? '*' : '')).join(', '));
  if (!ev) return;
  for (let i = 1; i <= 3; i++) {
    log.scroll.length = 0;
    // between clicks: user picks another attempt and collapses Attempt details
    const other = d.querySelector('.attempts [role=radio][aria-checked=false]'); if (i > 1 && other) click(w, other);
    const det = d.querySelector('.attempts details'); if (i > 1 && det) det.open = false;
    for (const b of d.querySelectorAll('.att-h[aria-expanded=true]')) click(w, b);
    const a = find('View evidence'); click(w, a); await sleep(80);
    const f = d.activeElement;
    console.log(' click', i, 'hash', w.location.hash, '| focus', desc(f), '| expanded', f.getAttribute && f.getAttribute('aria-expanded'), '| in attempt row', f.closest && (f.closest('[id^=attempt-]') || {}).id, '| checked attempt', (d.querySelector('.attempts [role=radio][aria-checked=true]') || {}).textContent, '| scrolled', log.scroll.length);
  }
})();
