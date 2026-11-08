// Browser-free DOM harness: real report HTML + real inline app script in jsdom 20.0.3 (no browser).
const {JSDOM, VirtualConsole} = require('jsdom');
const fs = require('fs');
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
async function open(file, hash, opts = {}) {
  const html = fs.readFileSync(file, 'utf8');
  const log = { scroll: [], listeners: { scroll: 0, resize: 0 }, errors: [] };
  const vc = new VirtualConsole(); vc.on('jsdomError', (e) => { if (!/CSS stylesheet/.test(e.message)) log.errors.push('jsdom: ' + e.message); }); vc.on('error', (...a) => log.errors.push(a.join(' ')));
  const dom = new JSDOM(html, { virtualConsole: vc,
    url: 'file:///tmp/report/index.html' + (hash || ''), runScripts: 'dangerously', pretendToBeVisual: true,
    beforeParse(w) {
      w.matchMedia = () => ({ matches: true, addEventListener() {}, removeEventListener() {} });
      w.Element.prototype.scrollIntoView = function () { log.scroll.push(this); };
      w.scrollTo = () => {}; w.TextDecoder = TextDecoder; w.TextEncoder = TextEncoder;
      if (!('inert' in w.HTMLElement.prototype)) Object.defineProperty(w.HTMLElement.prototype, 'inert', { get() { return this.hasAttribute('inert'); }, set(v) { v ? this.setAttribute('inert', '') : this.removeAttribute('inert'); } });
      w.queueMicrotask = w.queueMicrotask || queueMicrotask;
      w.CSS = { escape: (s) => String(s).replace(/["\\]/g, '\\$&') };
      const D = w.HTMLDialogElement && w.HTMLDialogElement.prototype;
      if (D && !D.__patched) { D.__patched = 1;
        D.showModal = function () { this.setAttribute('open', ''); this.__prev = w.document.activeElement; const f = this.querySelector('input,button,[tabindex]'); if (f) f.focus(); };
        D.close = function () { if (!this.hasAttribute('open')) return; this.removeAttribute('open'); const p = this.__prev; setTimeout(() => { this.dispatchEvent(new w.Event('close')); if (p && p.isConnected && w.document.activeElement === w.document.body) p.focus(); }, 0); };
        Object.defineProperty(D, 'open', { get() { return this.hasAttribute('open'); }, set(v) { v ? this.setAttribute('open', '') : this.removeAttribute('open'); } });
      }
      const ae = w.addEventListener.bind(w);
      w.addEventListener = (t, f, o) => { if (t in log.listeners) log.listeners[t]++; return ae(t, f, o); };
      const re = w.removeEventListener.bind(w);
      w.removeEventListener = (t, f, o) => { if (t in log.listeners) log.listeners[t]--; return re(t, f, o); };
      w.addEventListener('error', (e) => log.errors.push(e.message));
      Object.defineProperty(w.navigator, 'clipboard', { value: { writeText: async () => {} } });
      if (opts.before) opts.before(w);
    } });
  await sleep(50);
  return { dom, w: dom.window, d: dom.window.document, log };
}
// Tabbable per HTML rules that matter here: hidden/inert ancestors, closed <details> content, disabled.
function tabbable(d, scope) {
  const sel = 'a[href],button,input,select,textarea,summary,[tabindex]';
  return [...(scope || d).querySelectorAll(sel)].filter((el) => {
    if (el.disabled) return false;
    if (el.getAttribute('tabindex') === '-1') return false;
    if (el.tagName === 'INPUT' && el.type === 'hidden') return false;
    for (let n = el; n && n.nodeType === 1; n = n.parentElement) {
      if (n.hidden || n.hasAttribute('inert')) return false;
      if (n.tagName === 'DIALOG' && !n.hasAttribute('open')) return false;
      const p = n.parentElement;
      if (p && p.tagName === 'DETAILS' && !p.open && !(n.tagName === 'SUMMARY' && p.querySelector(':scope > summary') === n)) return false;
    }
    return true;
  });
}
const desc = (el) => !el ? 'null' : el.tagName.toLowerCase() + (el.id ? '#' + el.id : '') + (el.className && typeof el.className === 'string' ? '.' + el.className.split(' ').join('.') : '') + ' "' + (el.getAttribute('aria-label') || el.textContent || '').trim().slice(0, 40) + '"';
const click = (w, el) => el.dispatchEvent(new w.MouseEvent('click', { bubbles: true, cancelable: true, button: 0 }));
const key = (w, el, k) => el.dispatchEvent(new w.KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true }));
module.exports = { open, tabbable, desc, sleep, click, key };
