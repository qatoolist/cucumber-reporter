const { open, tabbable, desc, sleep, click, key } = require('./lib.cjs');
const file = process.argv[2], id = process.argv[3];
const B = (d, l, r) => [...(r || d).querySelectorAll('button')].find((b) => b.textContent.trim().startsWith(l));
(async () => {
  // 1. Palette Enter on a step result whose href equals current hash
  { const { w, d, log } = await open(file, '#/scenarios/' + encodeURIComponent(id) + '?step=0');
    await sleep(50); log.scroll.length = 0; window_scroll_away: d.activeElement.blur && d.activeElement.blur();
    const pin = d.querySelector('.pal-in'); key(w, d.body, '/'); await sleep(10);
    pin.value = d.querySelector('#step-0 .txt').textContent.trim(); pin.dispatchEvent(new w.Event('input'));
    const opt = [...d.querySelectorAll('.pal-o')].find((o) => o.getAttribute('href') === w.location.hash);
    console.log('palette: option with same href exists', !!opt, opt && opt.getAttribute('href'));
    if (opt) { while (d.querySelector('.pal-o[aria-selected=true]') !== opt) key(w, pin, 'ArrowDown');
      key(w, pin, 'Enter'); await sleep(80);
      console.log('palette Enter same-hash -> scrolled', log.scroll.map(desc).join(',') || 'NOTHING', '| focus', desc(d.activeElement)); }
  }
  // 2. Attachment Find switches content to raw while Tree/Raw radio stays on Tree
  { const { w, d } = await open(file, '#/scenarios/' + encodeURIComponent(id));
    for (const b of d.querySelectorAll('.att-h[aria-expanded=false]')) click(w, b);
    const find = d.querySelector('input[aria-label="Find in attachment"]');
    const seg = find && find.closest('.att-tools').querySelector('.seg');
    console.log('attachment with Tree/Raw seg:', !!seg);
    if (seg) { find.value = 'a'; find.dispatchEvent(new w.Event('input')); await sleep(250);
      const c = find.closest('.att-b').querySelector('.att-c');
      console.log('after Find: content is', c.firstElementChild.tagName, c.firstElementChild.className, '| radio state', [...seg.children].map((b) => b.textContent + '=' + b.getAttribute('aria-checked')).join(' ')); }
  }
  // 3. Status chip on list loses focus (button replaced)
  { const { w, d } = await open(file, '#/scenarios');
    const chip = [...d.querySelectorAll('.fchip')].find((b) => /Failed/.test(b.textContent)); chip.focus(); click(w, chip); await sleep(20);
    console.log('chip click: focused element connected?', d.activeElement !== d.body && d.activeElement.isConnected, '| activeElement', desc(d.activeElement), '| old chip connected', chip.isConnected);
    const rm = d.querySelector('.achip,.linkbtn'); if (rm) { rm.focus(); click(w, rm); await sleep(20); console.log('Clear/remove filter -> focus', desc(d.activeElement), 'old connected', rm.isConnected); }
    // table sort header
    const tbl = [...d.querySelectorAll('.seg button')].find((b) => b.textContent === 'Table'); click(w, tbl); await sleep(20);
    const th = [...d.querySelectorAll('.scenario-table th button')][3]; th.focus(); click(w, th); await sleep(20);
    console.log('table sort header click -> focus', desc(d.activeElement), 'old header connected', th.isConnected);
  }
  // 4. Dashboard legend toggle / mini sidebar
  { const { w, d } = await open(file, '#/');
    const leg = d.querySelector('.c-status .leg'); leg.focus(); click(w, leg); await sleep(20);
    console.log('status legend toggle -> old button connected', leg.isConnected, '| focus', desc(d.activeElement));
    const fl = d.querySelector('.c-feat .chipleg .leg'); if (fl) { fl.focus(); click(w, fl); await sleep(20); console.log('feature legend toggle -> old connected', fl.isConnected, '| focus', desc(d.activeElement)); }
    const mini = d.querySelector('.mini-btn'); mini.focus(); click(w, mini); await sleep(20);
    console.log('Collapse navigation -> old connected', mini.isConnected, '| focus', desc(d.activeElement), '| new title', d.querySelector('.mini-btn').title, 'pressed', d.querySelector('.mini-btn').getAttribute('aria-pressed'));
    const sb = d.querySelector('#side .nav-search'); sb.value = 'zz'; sb.dispatchEvent(new w.Event('input'));
  }
  // 5. Full-screen error dialog focus restore; print dialog
  { const { w, d } = await open(file, '#/scenarios/' + encodeURIComponent(id));
    const fs = B(d, 'Full screen', d.querySelector('main')); fs.focus(); click(w, fs); await sleep(10);
    const dlg = d.querySelector('dialog.investigation-dialog[open]'); console.log('fullscreen dialog open', !!dlg, 'focus inside', dlg && dlg.contains(d.activeElement), desc(d.activeElement));
    click(w, B(d, 'Close', dlg)); await sleep(20); console.log('after close: focus', desc(d.activeElement), '=== Full screen?', d.activeElement === fs, '| dialog removed', !dlg.isConnected);
    const pb = d.querySelector('.btn-print'); pb.focus(); click(w, pb); await sleep(10);
    const pd = d.querySelector('dialog.investigation-dialog[open]'); console.log('print dialog open', !!pd, 'focus', desc(d.activeElement));
    click(w, B(d, 'Preview', pd)); await sleep(80); console.log('after Preview: hash', w.location.hash, 'focus', desc(d.activeElement), 'print-root in main', !!d.querySelector('main #print-root'));
    // re-open print dialog on #/print and choose different scope -> no hashchange path
    click(w, d.querySelector('.btn-print')); await sleep(10); const pd2 = d.querySelector('dialog.investigation-dialog[open]');
    const sel = pd2.querySelector('select'); sel.value = 'failures'; sel.dispatchEvent(new w.Event('change')); click(w, B(d, 'Preview', pd2)); await sleep(80);
    console.log('2nd preview on same hash: scope line', (d.querySelector('main .pr-scope') || {}).textContent, '| sc cards', d.querySelectorAll('main .pr-sc').length);
  }
})();
