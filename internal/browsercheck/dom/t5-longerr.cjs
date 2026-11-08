const { open, desc, sleep, click } = require('./lib.cjs');
(async () => {
  const { w, d } = await open(process.argv[2], '#/scenarios/f-L9'); await sleep(20);
  const more = d.querySelector('.more-b'); const pre = d.querySelector('.steps .error-lines');
  console.log('button', more.textContent, '| shown', pre.querySelectorAll('.error-line').length);
  for (let i = 1; i <= 3; i++) { click(w, more); await sleep(5); console.log('press', i, 'shown', pre.querySelectorAll('.error-line').length, 'disabled', more.disabled, 'text', more.textContent); }
  const ln = [...d.querySelectorAll('button')].find((b) => b.textContent.trim() === 'Line numbers'); console.log('Line numbers aria-pressed initially:', ln.getAttribute('aria-pressed'));
  const coll = [...d.querySelectorAll('button')].find((b) => b.textContent.trim() === 'Collapse all'); click(w, coll); await sleep(10);
  console.log('after Collapse all: clip', pre.classList.contains('clip'), 'shown lines still', pre.querySelectorAll('.error-line').length);
})();
