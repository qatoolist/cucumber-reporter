const { open, tabbable, desc, sleep, click } = require('./lib.cjs');
const file = process.argv[2], id = process.argv[3];
(async () => {
  const { w, d, log } = await open(file, '#/scenarios/' + encodeURIComponent(id));
  const B = (l) => [...d.querySelectorAll('button')].find((b) => b.textContent.trim() === l);
  const state = (tag) => {
    const det = [...d.querySelectorAll('main details')], att = [...d.querySelectorAll('main .att-h')], err = [...d.querySelectorAll('main .error-lines')];
    const more = [...d.querySelectorAll('main .more-b')];
    console.log(tag, '| details open', det.filter((x) => x.open).length + '/' + det.length, '| att expanded', att.filter((b) => b.getAttribute('aria-expanded') === 'true').length + '/' + att.length,
      '| att w/o aria-controls', att.filter((b) => !b.hasAttribute('aria-controls')).length, '| errors clipped', err.filter((e) => e.classList.contains('clip')).length + '/' + err.length,
      '(short errors clipped: ' + err.filter((e) => e.classList.contains('clip') && e.textContent.split('\n').length <= 12).length + ')',
      '| Expand disabled', B('Expand all').disabled, 'Collapse disabled', B('Collapse all').disabled, '| focus', desc(d.activeElement));
  };
  await sleep(20); state('initial');
  const tabsIn = () => tabbable(d, d.querySelector('main')).filter((el) => el.closest('.steps,.attempts'));
  console.log('tabbable in steps/attempts initially', tabsIn().length);
  B('Expand all').focus(); click(w, B('Expand all')); await sleep(30); state('after Expand all'); console.log('tabbable', tabsIn().length);
  click(w, B('Expand all')); await sleep(30); state('after 2nd Expand all');
  B('Collapse all').focus(); click(w, B('Collapse all')); await sleep(30); state('after Collapse all');
  const t = tabsIn(); console.log('tabbable in steps/attempts after collapse', t.length, t.map(desc).slice(0, 12));
  // hidden-but-tabbable: anything tabbable whose ancestor .acc is hidden or closed details (sanity using our own rules + aria-hidden)
  const leaks = [...d.querySelectorAll('main .acc[hidden] *, main details:not([open]) > :not(summary) *')].filter((el) => tabbable(d, el.parentElement).includes(el));
  console.log('controls inside collapsed regions still tabbable:', leaks.length);
})();
