const { open, tabbable, desc, sleep, click } = require('./lib.cjs');
const file = process.argv[2], id = process.argv[3];
(async () => {
  const { w, d, log } = await open(file, '#/scenarios/' + encodeURIComponent(id));
  const find = (label) => [...d.querySelectorAll('a.btn,button')].find((b) => b.textContent.trim().startsWith(label));
  console.log('title', d.title, 'errors', log.errors);
  for (const label of ['Jump to step', 'View evidence']) {
    for (let i = 1; i <= 3; i++) {
      log.scroll.length = 0;
      const a = find(label); if (!a) { console.log(label, 'missing'); break; }
      const before = w.location.hash;
      // user scrolls away / collapses evidence between clicks
      for (const b of d.querySelectorAll('.att-h[aria-expanded="true"]')) click(w, b);
      click(w, a); await sleep(80);
      const att = d.activeElement.classList.contains('att-h') ? d.activeElement : null;
      console.log(label, '#' + i, 'hash', before === w.location.hash ? 'same' : 'changed->' + w.location.hash, '| focus', desc(d.activeElement), '| scrolled', log.scroll.map(desc).join(','), att ? '| expanded=' + att.getAttribute('aria-expanded') + ' panelHidden=' + (att.nextElementSibling && att.nextElementSibling.hidden) : '');
    }
  }
  // repeated sidebar attachment link + annotation step link
  const side = [...d.querySelectorAll('.sc-aside .jump a')];
  for (const a of side.slice(0, 2)) for (let i = 1; i <= 2; i++) { log.scroll.length = 0; click(w, a.isConnected ? a : [...d.querySelectorAll('.sc-aside .jump a')].find((x) => x.getAttribute('href') === a.getAttribute('href'))); await sleep(80); console.log('aside', a.getAttribute('href'), '#' + i, 'focus', desc(d.activeElement), 'scrolled', log.scroll.length); }
})();
