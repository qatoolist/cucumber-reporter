const { open, desc, sleep, click } = require('./lib.cjs');
(async () => {
  let t0 = Date.now();
  const { w, d, log } = await open(process.argv[2], '#/scenarios'); await sleep(50);
  console.log('load+render ms', Date.now() - t0, '| rows in DOM', d.querySelectorAll('.rows .row').length, '| window scroll listeners', log.listeners.scroll, 'resize', log.listeners.resize);
  const chips = () => [...d.querySelectorAll('.fchip')];
  for (let i = 0; i < 10; i++) { const c = chips()[i % 2 ? 0 : chips().length - 1]; click(w, c); await sleep(30); }
  console.log('after 10 status-chip clicks: window scroll listeners', log.listeners.scroll, 'resize', log.listeners.resize);
  const s = d.querySelector('input[aria-label="Filter scenarios"]');
  for (const q of ['b', 'bu', 'bul', 'bulk', 'bulk ', 'bulk s', 'bulk sc']) { s.value = q; s.dispatchEvent(new w.Event('input')); await sleep(90); }
  console.log('after typing 7 chars: window scroll listeners', log.listeners.scroll, 'resize', log.listeners.resize);
  t0 = Date.now(); w.location.hash = '#/scenarios/f-L20'; await sleep(60);
  console.log('navigate to scenario: listeners now', log.listeners.scroll, log.listeners.resize, '| prev/next label', d.querySelector('.pn-c').textContent);
  t0 = Date.now(); w.location.hash = '#/print'; await sleep(100); console.log('print preview of 5002 (blocked?)', (d.querySelector('.pr-scope') || {}).textContent, 'ms', Date.now() - t0);
  console.log('errors', log.errors);
})();
