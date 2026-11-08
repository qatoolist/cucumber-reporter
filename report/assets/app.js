/* cucumber-reporter page script.
 * Data enters only through the application/json block that the Go renderer writes; every node is built
 * with createElement and textContent. No innerHTML, no eval, no network, no inline handlers. Styles
 * change through CSSOM only. Statuses, including known failures and the known-failure step marker, are
 * computed by the renderer (report.Annotation documents the rules); this script only displays them. */
(() => {
  'use strict';
  const doc = document, root = doc.documentElement, W = window;
  const dataEl = doc.getElementById('report-data');
  let D;
  try { D = JSON.parse(dataEl.textContent); if (!D || !Array.isArray(D.features)) throw new Error('Invalid report'); } catch (e) { const message=doc.createElement('p');message.setAttribute('role','alert');message.textContent='This report could not be read. Regenerate it from the original results.';doc.body.prepend(message);return; }
  root.classList.add('js');
  const reduced = W.matchMedia('(prefers-reduced-motion: reduce)');
  const isMac = /Mac|iP(hone|ad)/.test(navigator.platform || navigator.userAgent);

  /* ---------- DOM helpers ---------- */
  const add = (el, kids) => {
    for (const k of kids) {
      if (k == null || k === false) continue;
      if (Array.isArray(k)) add(el, k); else el.append(k.nodeType ? k : String(k));
    }
    return el;
  };
  const set = (el, a) => {
    if (a) for (const k in a) {
      const v = a[k];
      if (v == null || v === false) continue;
      if (k === 'class') el.setAttribute('class', v);
      else if (k === 'text') el.textContent = v;
      else if (k.slice(0, 2) === 'on') el.addEventListener(k.slice(2), v);
      else el.setAttribute(k, v === true ? '' : v);
    }
    return el;
  };
  const h = (t, a, ...k) => add(set(doc.createElement(t), a), k);
  // Like el.replaceChildren(...k), but skipping null and false as h() does (the native call would insert
  // the text "null").
  const fill = (el, ...k) => { el.replaceChildren(); return add(el, k); };
  const NS = 'http://www.w3.org/2000/svg';
  const sv = (t, a, ...k) => add(set(doc.createElementNS(NS, t), a), k);
  const $ = (s, r) => (r || doc).querySelector(s);
  const css = (el, p, v) => { el.style.setProperty(p, v); return el; };

  const IC = {
    check: 'M5 12.5l4.5 4.5L19 7.5', x: 'M7 7l10 10M17 7L7 17', flag: 'M6 21V4h11l-2.5 4 2.5 4H6',
    skip: 'M6 6l8 6-8 6zM18 6v12', pending: 'M20 12a8 8 0 1 1-16 0 8 8 0 0 1 16 0zM8.5 12h.01M12 12h.01M15.5 12h.01',
    undef: 'M9.2 9a3 3 0 1 1 4.3 2.7c-.9.5-1.5 1.2-1.5 2.3M12 17.5v.01', ambig: 'M5 9.5h14M5 14.5h14M15.5 5l-7 14',
    dashboard: 'M4 4h7v7H4zM13 4h7v4h-7zM13 10h7v10h-7zM4 13h7v7H4z', features: 'M12 3l9 5-9 5-9-5zM3 12.5l9 5 9-5M3 17l9 5 9-5',
    list: 'M9 6h11M9 12h11M9 18h11M4.5 6h.01M4.5 12h.01M4.5 18h.01', alert: 'M12 3.5L2.5 20h19zM12 10v4.5M12 17.5v.01',
    timeline: 'M3 5h7M7 10h9M12 15h6M16 20h5', info: 'M12 21a9 9 0 1 1 0-18 9 9 0 0 1 0 18zM12 11v5M12 8v.01',
    search: 'M11 18a7 7 0 1 1 0-14 7 7 0 0 1 0 14zM20 20l-4-4',
    sun: 'M12 16a4 4 0 1 1 0-8 4 4 0 0 1 0 8zM12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4',
    moon: 'M20 14.5A8 8 0 0 1 9.5 4a8 8 0 1 0 10.5 10.5z', auto: 'M12 21a9 9 0 1 1 0-18 9 9 0 0 1 0 18zM12 3v18',
    sliders: 'M4 7h9M17 7h3M4 17h3M11 17h9M15 4.5v5M9 14.5v5', print: 'M7 9V3h10v6M7 17H4v-7h16v7h-3M7 14h10v7H7z',
    menu: 'M4 6h16M4 12h16M4 18h16', right: 'M9 6l6 6-6 6', left: 'M15 6l-6 6 6 6', down: 'M6 9l6 6 6-6', up: 'M6 15l6-6 6 6', image: 'M3 3h18v18H3zM3 17l6-6 4 4 3-3 5 5M8 7h.01',
    copy: 'M9 9h11v11H9zM5 15H4V4h11v1', clock: 'M12 21a9 9 0 1 1 0-18 9 9 0 0 1 0 18zM12 7v5l3 2',
    tag: 'M3 12V4h8l10 10-8 8zM7.5 7.5h.01', file: 'M14 3H6v18h12V7zM14 3v4h4',
    clip: 'M20 11.5l-8.5 8.5a5 5 0 0 1-7-7L13 4.5a3.3 3.3 0 0 1 4.7 4.7L9.2 17.7a1.7 1.7 0 0 1-2.4-2.4L15 7',
    arrow: 'M5 12h14M13 6l6 6-6 6', save: 'M12 4v11M7 10l5 5 5-5M5 20h14',
    eyeoff: 'M3 3l18 18M10.6 6.1L12 6c6 0 9.5 6 9.5 6a17 17 0 0 1-2.6 3.3M6.6 6.6C3.9 8.4 2.5 12 2.5 12S6 18 12 18c1.8 0 3.3-.5 4.6-1.2M9.9 9.9a3 3 0 0 0 4.2 4.2',
    table: 'M3 5h18v14H3zM3 10h18M3 15h18M9 5v14', doc: 'M7 4h10v16H7zM10 8h4M10 12h4M10 16h2',
    collapse: 'M15 6l-6 6 6 6M20 4v16', keyboard: 'M3 6h18v12H3zM7 10h.01M11 10h.01M15 10h.01M7 14h10',
    pulse: 'M3 12.5h4l2.5-6 5 11 2.5-5H21', close: 'M6 6l12 12M18 6L6 18'
  };
  const icon = (n, c) => {
    const s = sv('svg', { class: 'i' + (c ? ' ' + c : ''), viewBox: '0 0 24 24', 'aria-hidden': 'true', focusable: 'false' });
    for (const d of IC[n].split('|')) s.append(sv('path', { d }));
    return s;
  };

  /* ---------- formatting ---------- */
  const nf = new Intl.NumberFormat('en');
  const num = (n) => nf.format(n);
  const pct = (a, b) => (b ? (a * 100 / b) : 0);
  const fpct = (v) => (v === 100 || v === 0 ? v.toFixed(0) : v.toFixed(1)) + '%';
  // Each branch is taken only when its rounded value stays below the next unit, so 59.96 s reads
  // "1m 00s", never "60.0 s"; report/model.go formatNS mirrors this function.
  const dur = (ns) => {
    if (ns == null) return '—';
    if (ns < 1e3) return ns + ' ns';
    if (Math.round(ns / 1e3) < 1000) return Math.round(ns / 1e3) + ' µs';
    const ms = ns / 1e6;
    if (+ms.toFixed(1) < 10) return ms.toFixed(1) + ' ms';
    if (Math.round(ms) < 1000) return Math.round(ms) + ' ms';
    const s = ms / 1000;
    if (+s.toFixed(2) < 10) return s.toFixed(2) + ' s';
    if (+s.toFixed(1) < 60) return s.toFixed(1) + ' s';
    const t = Math.round(s);
    if (t < 3600) return Math.floor(t / 60) + 'm ' + String(t % 60).padStart(2, '0') + 's';
    const m = Math.round(s / 60);
    return Math.floor(m / 60) + 'h ' + String(m % 60).padStart(2, '0') + 'm';
  };
  const firstLine = (s) => (s || '').split('\n')[0];
  const plural = (n, w, p) => num(n) + ' ' + (n === 1 ? w : (p || w + 's'));
  const bytes = (n) => n < 1024 ? n + ' B' : n < 1048576 ? (n / 1024).toFixed(1) + ' KB' : (n / 1048576).toFixed(1) + ' MB';

  /* ---------- model ---------- */
  const SEV = ['failed', 'ambiguous', 'undefined', 'known', 'pending', 'skipped', 'passed'];
  const ORDER = ['passed', 'failed', 'known', 'ambiguous', 'undefined', 'pending', 'skipped'];
  // Lookup tables keyed by data or by the URL have no prototype, so names such as "constructor" or
  // "__proto__" in a link match nothing.
  const ST = Object.assign(Object.create(null), {
    passed: ['Passed', 'check', 'All steps passed.'], failed: ['Failed', 'x', 'A step failed and no known issue covers it.'],
    known: ['Known failure', 'flag', 'A step failed and a known issue covers it.'], ambiguous: ['Ambiguous', 'ambig', 'More than one step definition matches a step.'],
    undefined: ['Undefined', 'undef', 'No step definition matches a step.'], pending: ['Pending', 'pending', 'A step is marked as not yet implemented.'],
    skipped: ['Skipped', 'skip', 'The scenario did not run.']
  });
  const zero = () => Object.fromEntries(ORDER.map((k) => [k, 0]));
  const ANN = Object.create(null);
  for (const a of D.annotations || []) (ANN[a.scenario] = ANN[a.scenario] || []).push(a);
  const analysisByID = new Map((D.analysis && D.analysis.scenarios || []).map((x) => [x.id,x]));
  const F = [], S = [], STEPS = [], FB = Object.create(null), SB = Object.create(null), TAGS = new Map();
  for (const f of D.features || []) {
    const fm = { id: f.id, name: f.name || '(unnamed feature)', uri: f.uri || '', desc: f.description || '', tags: f.tags || [], sc: [], n: zero(), dur: 0, steps: 0, status: ST[f.status] ? f.status : 'passed' };
    for (const e of f.elements || []) {
      const steps = e.steps || [];
      let d = 0;
      for (const st of steps) if (st.duration != null) d += st.duration;
      const status = ST[e.status] ? e.status : 'undefined';
      const sm = { runHook: !!e.runHook, analysis: analysisByID.get(e.id), id: e.id, f: fm, name: e.name || '(unnamed scenario)', kw: e.keyword || 'Scenario', desc: e.description || '', line: e.line, tags: e.tags || [], steps, status, dur: d, ann: ANN[e.id] || [], i: S.length };
      sm.bad = steps.find((st) => st.status !== 'passed' && st.status !== 'skipped') || steps.find((st) => st.known);
      steps.forEach((st, k) => {
        st.sc = sm; st.k = k;
        st.vs = st.known ? 'known' : st.status;
        STEPS.push(st);
      });
      for (const t of sm.tags) TAGS.set(t, (TAGS.get(t) || 0) + 1);
      fm.sc.push(sm); fm.n[status]++; fm.dur += d; fm.steps += steps.length;
      S.push(sm); SB[sm.id] = sm;
    }
    fm.runHooks = fm.sc.length > 0 && fm.sc.every(s=>s.runHook);
    F.push(fm); FB[fm.id] = fm;
  }
  const T = { sc: zero(), st: zero(), fe: zero(), dur: 0 };
  for (const s of S) { if(!s.runHook)T.sc[s.status]++; T.dur += s.dur; }
  for (const st of STEPS) T.st[st.vs]++;
  for (const f of F) T.fe[f.status]++;
  const executionLabel=(list)=>{const hooks=list.filter(s=>s.runHook).length;return plural(list.length-hooks,'scenario')+(hooks?' · '+plural(hooks,'run hook'):'');};
  const scenarioCount=S.filter(s=>!s.runHook).length;
  const runHookCount=S.length-scenarioCount;
  const failedExecutionCount=T.sc.failed+S.filter(s=>s.runHook&&s.status==='failed').length;
  const executed = scenarioCount - T.sc.skipped;
  const passRate = pct(T.sc.passed, executed);
  // With known failures the headline metric is "as expected" (passed + known), matching the caller's verdict; the raw pass rate stays secondary.
  const asExpected = pct(T.sc.passed + T.sc.known, executed);
  const HEAD = T.sc.known ? { label: 'As expected', value: asExpected, sub: num(T.sc.passed) + ' passed · ' + num(T.sc.known) + ' known · pass rate ' + fpct(passRate), href: '#/scenarios?status=passed,known' } : { label: 'Pass rate', value: passRate, sub: num(T.sc.passed) + ' of ' + num(executed) + ' executed', href: '#/scenarios?status=passed' };
  if (T.sc.skipped) HEAD.sub += ' · ' + num(T.sc.skipped) + ' skipped';
  const blocking = T.sc.ambiguous + T.sc.undefined;
  // The renderer decides each known-issue record's state once (applied, passed, not-applied, invalid);
  // every view, the print document and the fallback use that state.
  const KNOWN = (D.annotations || []).filter((a) => a.kind === 'known' && (a.state === 'applied' || a.state === 'passed' || a.state === 'not-applicable'));
  const OOS = (D.annotations || []).filter((a) => a.kind === 'known' && a.state === 'not-applicable' && SB[a.scenario]);
  const NOTAPPLIED = (D.annotations || []).filter((a) => a.kind === 'known' && (a.state === 'not-applied' || a.state === 'invalid') && SB[a.scenario]);
  const LIMITS = (D.annotations || []).filter((a) => a.kind === 'limit');
  const tone = (D.verdict && D.verdict.tone) || (T.sc.failed || blocking ? 'fail' : T.sc.known ? 'warn' : 'ok');
  const verdict = (D.verdict && D.verdict.label) || (tone === 'fail' ? 'Failed' : tone === 'warn' ? 'Passed with known failures' : 'Passed');

  /* ---------- preferences ---------- */
  const KEY = 'cucumber-reporter:v2:prefs';
  const WIDGETS = [['hero', 'Verdict summary'], ['kpis', 'Key figures'], ['status', 'Status breakdown'], ['features', 'Results per feature'], ['attention', 'Needs attention'], ['known', 'Known and unexpected failures'], ['duration', 'Duration profile'], ['sections', 'Report sections']];
  const NAV = [['dashboard', 'Dashboard', 'dashboard'], ['features', 'Features', 'features'], ['scenarios', 'Scenarios', 'list'], ['failures', 'Failures', 'alert'], ['known', 'Known issues', 'flag'], ['skipped', 'Skipped', 'skip'], ['timeline', 'Steps and timeline', 'timeline'], ['run', 'Run details', 'info']];
  const CHOICE = { theme: ['auto', 'light', 'dark'], density: ['comfortable', 'compact'], expand: ['failed', 'all', 'none'], view: ['dashboard', 'features', 'scenarios', 'failures'], side: ['open', 'mini'] };
  const dflt = D.defaults || {};
  let storeOK = true;
  const base = () => {
    const p = { widgets: {}, nav: {} };
    for (const k in CHOICE) p[k] = CHOICE[k].includes(dflt[k]) ? dflt[k] : CHOICE[k][0];
    for (const [k] of WIDGETS) p.widgets[k] = !(dflt.widgets && dflt.widgets[k] === false);
    for (const [k] of NAV) p.nav[k] = !(dflt.nav && dflt.nav[k] === false);
    return p;
  };
  const load = () => {
    const p = base();
    let s = null;
    try { const raw = W.localStorage.getItem(KEY); s = raw ? JSON.parse(raw) : null; } catch (e) { storeOK = false; }
    if (!s || typeof s !== 'object' || s.v !== 2) return p;
    for (const k in CHOICE) if (CHOICE[k].includes(s[k])) p[k] = s[k];
    for (const g of ['widgets', 'nav']) if (s[g] && typeof s[g] === 'object') for (const k in p[g]) if (typeof s[g][k] === 'boolean') p[g][k] = s[g][k];
    p.nav.dashboard = true;
    return p;
  };
  let P = load();
  const save = () => {
    try { W.localStorage.setItem(KEY, JSON.stringify(Object.assign({ v: 2 }, P))); storeOK = true; } catch (e) { storeOK = false; }
    const n = $('#store-note'); if (n) n.hidden = storeOK;
  };
  const applyPrefs = () => {
    if (P.theme === 'auto') delete root.dataset.theme; else root.dataset.theme = P.theme;
    root.dataset.density = P.density;
    root.dataset.side = P.side;
  };
  applyPrefs();

  /* ---------- small components ---------- */
  const chip = (s, small) => h('span', { class: 'chip s-' + s + (small ? ' sm' : '') }, icon(ST[s][1]), h('span', { class: 'chip-l' }, ST[s][0]));
  const glyph = (s) => h('span', { class: 'glyph s-' + s, role: 'img', 'aria-label': ST[s][0], title: ST[s][0] }, icon(ST[s][1]));
  const tagEl = (t) => h('a', { class: 'tag', href: '#/scenarios?tag=' + encodeURIComponent(t) }, t);
  const bdi = (t, c) => h('bdi', { class: c }, t);
  const card = (cls, title, sub, actions, ...body) => h('section', { class: 'card ' + (cls || '') },
    title && h('header', { class: 'card-h' }, h('div', null, h('h2', { class: 'card-t' }, title), sub && h('p', { class: 'card-s' }, sub)), actions && h('div', { class: 'card-a' }, actions)), ...body);
  const btn = (label, ic, on, cls, extra) => h('button', Object.assign({ type: 'button', class: 'btn ' + (cls || ''), onclick: on }, extra), ic && icon(ic), label && h('span', null, label));
  const ibtn = (label, ic, on, extra) => h('button', Object.assign({ type: 'button', class: 'ibtn', 'aria-label': label, title: label, onclick: on }, extra), icon(ic));
  const empty = (ic, title, text, toneCls) => h('div', { class: 'empty ' + (toneCls || '') }, h('div', { class: 'empty-i' }, icon(ic)), h('p', { class: 'empty-t' }, title), text && h('p', { class: 'empty-x' }, text));
  const seg = (name, opts, cur, on) => {
    const g = h('div', { class: 'seg', role: 'radiogroup', 'aria-label': name });
    for (const [v, l] of opts) {
      g.append(h('button', { type: 'button', role: 'radio', 'aria-checked': String(v === cur), onclick: (e) => {
        for (const b of g.children) b.setAttribute('aria-checked', String(b === e.currentTarget));
        on(v);
      } }, l));
    }
    g.addEventListener('keydown', (e) => {
      const b = [...g.children], i = b.indexOf(doc.activeElement);
      const d = e.key === 'ArrowRight' || e.key === 'ArrowDown' ? 1 : e.key === 'ArrowLeft' || e.key === 'ArrowUp' ? -1 : 0;
      if (d && i >= 0) { e.preventDefault(); const n = b[(i + d + b.length) % b.length]; n.focus(); n.click(); }
    });
    return g;
  };
  const argText = (t) => {
    const out = [];
    let last = 0;
    String(t).replace(/"[^"]*"|<[^>\s]+>|-?\b\d+(?:[.,]\d+)*\b/g, (m, i) => { out.push(t.slice(last, i), h('span', { class: 'arg' }, m)); last = i + m.length; });
    out.push(t.slice(last));
    return out;
  };
  const tokens = (q) => q.toLowerCase().split(/\s+/).filter(Boolean);
  const hl = (text, toks) => {
    text = String(text);
    if (!toks || !toks.length) return [text];
    let low='',offsets=[],ends=[],position=0;for(const ch of text){const folded=ch.toLowerCase();for(let i=0;i<folded.length;i++){offsets.push(position);ends.push(position+ch.length);}low+=folded;position+=ch.length;}low=text.toLowerCase();const r=[];
    for (const t of toks) { let i = low.indexOf(t); while (i >= 0) { r.push([offsets[i], ends[i+t.length-1]]); i = low.indexOf(t, i + t.length); } }
    if (!r.length) return [text];
    r.sort((a, b) => a[0] - b[0]);
    const m = [r[0]];
    for (const x of r.slice(1)) { const l = m[m.length - 1]; if (x[0] <= l[1]) l[1] = Math.max(l[1], x[1]); else m.push(x); }
    const out = [];
    let p = 0;
    for (const [a, b] of m) { out.push(text.slice(p, a), h('mark', null, text.slice(a, b))); p = b; }
    out.push(text.slice(p));
    return out;
  };

  /* tooltip */
  const tip = h('div', { class: 'tip', role: 'tooltip', id: 'tip', hidden: true });
  const showTip = (lines, x, y) => {
    tip.replaceChildren(...lines.map((l, i) => h('div', { class: i ? 'tip-l' : 'tip-t' }, l)));
    tip.hidden = false;
    const r = tip.getBoundingClientRect();
    const left = Math.min(Math.max(8, x - r.width / 2), W.innerWidth - r.width - 8);
    const top = y - r.height - 12 < 8 ? y + 18 : y - r.height - 12;
    css(css(tip, 'left', left + 'px'), 'top', top + 'px');
  };
  const hideTip = () => { tip.hidden = true; };
  const bindTip = (el, fn) => {
    el.addEventListener('pointermove', (e) => showTip(fn(), e.clientX, e.clientY));
    el.addEventListener('pointerleave', hideTip);
    el.addEventListener('focus', () => { const r = el.getBoundingClientRect(); showTip(fn(), r.left + r.width / 2, r.top); });
    el.addEventListener('blur', hideTip);
    return el;
  };

  /* count-up */
  const countUp = (el, to, fmt) => {
    if (reduced.matches) { el.textContent = fmt(to); return el; }
    const t0 = performance.now(), d = 700;
    el.textContent = fmt(0);
    const step = (t) => {
      const k = Math.min(1, (t - t0) / d), e = 1 - Math.pow(1 - k, 3);
      el.textContent = fmt(k === 1 ? to : to * e);
      if (k < 1) requestAnimationFrame(step);
    };
    requestAnimationFrame(step);
    return el;
  };

  /* toast */
  const toast = h('div', { class: 'toast', role: 'status', 'aria-live': 'polite' });
  let toastT = 0;
  const say = (m) => { toast.textContent = m; toast.classList.add('on'); clearTimeout(toastT); toastT = setTimeout(() => toast.classList.remove('on'), 1800); };
  const copy = (text, what) => {
    const done = () => say((what || 'Text') + ' copied');
    const fallback = () => {
      const t = h('textarea', { class: 'sr', readonly: true }); t.value = text; doc.body.append(t); t.select();
      try { doc.execCommand('copy'); done(); } catch (e) { say('Copy is blocked in this browser'); }
      t.remove();
    };
    if (navigator.clipboard && W.isSecureContext !== false) navigator.clipboard.writeText(text).then(done, fallback); else fallback();
  };

  /* ---------- charts ---------- */
  // Stacked horizontal bar as SVG (percent geometry). Each segment may link somewhere.
  const sbar = (counts, opts) => {
    opts = opts || {};
    const tot = ORDER.reduce((a, k) => a + (opts.hide && opts.hide.has(k) ? 0 : counts[k] || 0), 0);
    const s = sv('svg', { class: 'sbar ' + (opts.cls || ''), viewBox: '0 0 100 10', preserveAspectRatio: 'none', role: opts.link ? null : 'img', 'aria-label': opts.label || ORDER.filter((k) => counts[k]).map((k) => counts[k] + ' ' + ST[k][0].toLowerCase()).join(', ') });
    const g = sv('g', { class: 'grow' });
    if (opts.delay) css(g, '--d', opts.delay + 'ms');
    let x = 0;
    for (const k of ORDER) {
      const n = counts[k] || 0;
      if (!n || (opts.hide && opts.hide.has(k))) continue;
      const w = n * 100 / (tot || 1);
      const r = sv('rect', { class: 'f-' + k, x: x.toFixed(3), y: 0, width: Math.max(w, 0.4).toFixed(3), height: 10 });
      if (opts.link) {
        const a = sv('a', { href: opts.link(k), 'aria-label': n + ' ' + ST[k][0].toLowerCase() + (opts.of ? ' in ' + opts.of : '') }, r);
        bindTip(a, () => [ST[k][0], plural(n, opts.unit || 'scenario') + ' · ' + fpct(pct(n, tot)), opts.of || '']);
        g.append(a);
      } else g.append(r);
      x += w;
    }
    s.append(g);
    return s;
  };

  // Donut with drill-down segments.
  const donut = (counts, unit, hide, pick, still) => {
    const R = 70, C = 2 * Math.PI * R;
    const keys = ORDER.filter((k) => counts[k] && !hide.has(k));
    const tot = keys.reduce((a, k) => a + counts[k], 0);
    const s = sv('svg', { class: 'donut', viewBox: '0 0 180 180', role: 'group', 'aria-label': 'Status of ' + unit + 's' });
    s.append(sv('circle', { class: 'track', cx: 90, cy: 90, r: R }));
    let off = 0;
    const gap = keys.length > 1 ? 1.6 : 0;
    for (const k of keys) {
      const len = counts[k] / tot * C;
      const c = sv('circle', { class: 'seg f-' + k, cx: 90, cy: 90, r: R, transform: 'rotate(-90 90 90)', 'stroke-dashoffset': (-off).toFixed(2), 'stroke-dasharray': still || reduced.matches ? Math.max(len - gap, 0.5).toFixed(2) + ' ' + C.toFixed(2) : '0 ' + C.toFixed(2) });
      c.dataset.len = Math.max(len - gap, 0.5).toFixed(2);
      if (pick) {
        set(c, { tabindex: 0, role: 'link', 'aria-label': ST[k][0] + ': ' + plural(counts[k], unit) + ', ' + fpct(pct(counts[k], tot)) + '. Show them.' });
        c.addEventListener('click', () => pick(k));
        c.addEventListener('keydown', (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); pick(k); } });
        bindTip(c, () => [ST[k][0], plural(counts[k], unit) + ' · ' + fpct(pct(counts[k], tot)), 'Click to view']);
      }
      s.append(c);
      off += len;
    }
    if (!still && !reduced.matches) requestAnimationFrame(() => requestAnimationFrame(() => { for (const c of s.querySelectorAll('.seg')) c.setAttribute('stroke-dasharray', c.dataset.len + ' ' + C.toFixed(2)); }));
    return s;
  };

  // The ring covers all scenarios; the centre rate still excludes skipped scenarios.
  const ringSegments = (counts) => {
    const total = ORDER.reduce((n, k) => n + (counts[k] || 0), 0);
    let offset = 0;
    return ORDER.filter(k => counts[k] > 0).map(k => {
      const part = { status: k, count: counts[k], share: counts[k] / total, offset };
      offset += part.share;
      return part;
    });
  };
  const ring = (counts, still) => {
    const R = 52, C = 2 * Math.PI * R, now = still || reduced.matches;
    const arcs = ringSegments(counts).map(part => sv('circle', {
      class: 'ring-v f-' + part.status, cx: 60, cy: 60, r: R,
      transform: 'rotate(-90 60 60)', 'stroke-dashoffset': -part.offset * C,
      'stroke-dasharray': (now ? part.share * C : 0) + ' ' + C, 'data-len': part.share * C
    }));
    if (!now) requestAnimationFrame(() => requestAnimationFrame(() => { for (const c of arcs) c.setAttribute('stroke-dasharray', c.getAttribute('data-len') + ' ' + C); }));
    return sv('svg', { class: 'ring', viewBox: '0 0 120 120', 'aria-hidden': 'true' }, sv('circle', { class: 'ring-t', cx: 60, cy: 60, r: R }), ...arcs);
  };
  const outcomeRing = () => {
    const parts = ringSegments(T.sc);
    const description = parts.map(p => ST[p.status][0] + ': ' + num(p.count) + ' (' + fpct(p.share * 100) + ')').join('; ') || 'No scenarios';
    return h('div', { class: 'hero-chart' },
      h('a', { class: 'hero-r', href: '#/scenarios', 'aria-label': HEAD.label + ' ' + fpct(HEAD.value) + ' of executed scenarios. All scenarios: ' + description + '. Show all scenarios', title: description },
        ring(T.sc), h('span', { class: 'hero-rv' }, countUp(h('b'), HEAD.value, fpct)), h('span', { class: 'hero-rl' }, HEAD.label.toLowerCase())),
      h('ul', { class: 'hero-legend', 'aria-label': 'Scenario outcomes' }, parts.map(p => h('li', null,
        h('a', { href: '#/scenarios?status=' + p.status }, h('span', { class: 'hero-swatch f-' + p.status, 'aria-hidden': 'true' }), ST[p.status][0] + ' ' + num(p.count) + ' · ' + fpct(p.share * 100))))),
      T.sc.skipped ? h('p', { class: 'hero-rate-note' }, 'Ring: all scenarios. Centre rate excludes skipped.') : null);
  };

  const retainFocus = (scope, fallback) => {
    const old = doc.activeElement;
    if (!old || !scope.contains(old)) return () => {};
    const key = old.dataset.focusKey, label = old.getAttribute('aria-label'), text = old.textContent;
    return () => {
      if (old.isConnected && !old.disabled) return;
      const next = [...scope.querySelectorAll('button,input,select,a[href],summary')].find(el => !el.disabled && (key ? el.dataset.focusKey === key : label ? el.getAttribute('aria-label') === label : el.tagName === old.tagName && el.textContent === text));
      (next || fallback || scope.querySelector('input,button,a[href]'))?.focus({preventScroll:true});
    };
  };

  /* ---------- shell ---------- */
  const main = h('main', { id: 'main', class: 'main', tabindex: '-1' });
  const side = h('nav', { id: 'side', class: 'side', 'aria-label': 'Report sections' });
  const kbd = isMac ? '⌘K' : 'Ctrl K';
  const themeBtn = ibtn('Theme', 'auto', () => { P.theme = CHOICE.theme[(CHOICE.theme.indexOf(P.theme) + 1) % 3]; applyPrefs(); save(); syncTheme(); say('Theme: ' + P.theme); });
  const syncTheme = () => {
    const n = { auto: 'auto', light: 'sun', dark: 'moon' }[P.theme];
    themeBtn.replaceChildren(icon(n));
    set(themeBtn, { 'aria-label': 'Theme: ' + P.theme + '. Change theme', title: 'Theme: ' + P.theme });
  };
  const menuBtn = ibtn('Show navigation', 'menu', () => toggleSide(), { class: 'ibtn menu-btn', 'aria-controls': 'side', 'aria-expanded': 'false' });
  const mobileNav = W.matchMedia('(max-width: 1024px)');
  const syncDrawer = () => { const closed = mobileNav.matches && !root.classList.contains('side-open'); side.inert = closed; side.setAttribute('aria-hidden', String(closed)); };
  mobileNav.addEventListener('change', syncDrawer);
  const toggleSide = (force) => {
    const open = force != null ? force : !root.classList.contains('side-open');
    root.classList.toggle('side-open', open);
    menuBtn.setAttribute('aria-expanded', String(open));
    syncDrawer();
    if (mobileNav.matches && open) side.querySelector('a[href]')?.focus();
    else if (mobileNav.matches && side.contains(doc.activeElement)) menuBtn.focus();
  };
  const facts3 = (D.top || []).filter(Boolean);
  const top = h('header', { class: 'top' },
    menuBtn,
    h('a', { class: 'brand', href: '#/', 'aria-label': 'Dashboard' }, h('span', { class: 'logo' }, icon('pulse'))),
    h('div', { class: 'run-id' }, h('div', { class: 'run-t' }, bdi(D.name || D.title || 'Test report')), h('div', { class: 'run-s' }, facts3.map((f, i) => [i ? h('span', { class: 'dot', 'aria-hidden': 'true' }, '·') : null, bdi(f)]))),
    h('a', { class: 'verdict-pill t-' + tone, href: '#/' + (T.sc.failed + blocking ? 'failures' : 'dashboard') }, icon(tone === 'fail' ? 'x' : tone === 'warn' ? 'flag' : 'check'), h('span', null, verdict)),
    h('div', { class: 'grow-x' }),
    h('button', { type: 'button', class: 'search-btn', onclick: () => openPalette(), 'aria-label': 'Search (' + kbd + ')' }, icon('search'), h('span', { class: 'sb-l' }, 'Search'), h('kbd', null, kbd)),
    themeBtn,
    ibtn('Settings', 'sliders', () => openSettings()),
    btn('Print', 'print', () => doPrint(), 'btn-print', { 'aria-label': 'Print or save as PDF' }));
  syncTheme();

  const navCount = { features: [F.length], scenarios: [scenarioCount], failures: [failedExecutionCount + blocking, 'f'], known: [T.sc.known, 'k'], skipped: [T.sc.skipped + T.sc.pending + LIMITS.length] };
  const buildSide = () => {
    const ul = h('ul', { class: 'nav' });
    for (const [k, l, ic] of NAV) {
      if (!P.nav[k]) continue;
      const c = navCount[k];
      ul.append(h('li', null, h('a', { class: 'nav-i', href: '#/' + k, 'data-k': k, title: k === 'failures' ? 'Unexpected failures plus undefined and ambiguous scenarios' : l }, icon(ic), h('span', { class: 'nav-l' }, l), c && c[0] ? h('span', { class: 'nav-c ' + (c[1] || '') }, num(c[0])) : null)));
    }
    const fl = h('ul', { class: 'nav nav-f' });
    for (const f of F) fl.append(h('li', null, h('a', { class: 'nav-i', href: '#/features/' + encodeURIComponent(f.id), 'data-k': 'f:' + f.id, title: f.name }, glyph(f.status), h('span', { class: 'nav-l' }, bdi(f.name)))));
    const featureSearch = h('input', { class: 'input nav-search', type: 'search', placeholder: 'Find feature…', 'aria-label': 'Find feature' });
    const problemsOnly = h('input', { type: 'checkbox', 'aria-label': 'Only features needing attention' });
    const filterFeatures = () => { for (const li of fl.children) { const f = FB[li.firstChild.dataset.k.slice(2)]; li.hidden = !f.name.toLowerCase().includes(featureSearch.value.toLowerCase()) || (problemsOnly.checked && !['failed','undefined','ambiguous'].includes(f.status)); } };
    featureSearch.addEventListener('input', filterFeatures); problemsOnly.addEventListener('change', filterFeatures);
    side.replaceChildren(ul, h('details', { class: 'feature-nav', open: true }, h('summary', { class: 'nav-h' }, 'Features'), featureSearch, h('label', { class: 'nav-l' }, problemsOnly, ' Problems only'), fl),
      h('div', { class: 'side-f' }, h('button', { type: 'button', class: 'nav-i mini-btn', 'aria-pressed': String(P.side === 'mini'), title: P.side === 'mini' ? 'Expand navigation' : 'Collapse navigation', 'data-focus-key':'collapse-nav', onclick: () => { const restore = retainFocus(side); P.side = P.side === 'mini' ? 'open' : 'mini'; applyPrefs(); save(); buildSide(); markNav(); restore(); } }, icon('collapse'), h('span', { class: 'nav-l' }, P.side === 'mini' ? 'Expand' : 'Collapse'))));
  };
  const scrim = h('div', { class: 'scrim', onclick: () => toggleSide(false) });
  doc.body.append(h('a', { class: 'skip', href: '#main', onclick: (e) => { e.preventDefault(); main.focus(); } }, 'Skip to content'), top, h('div', { class: 'body' }, side, main), scrim, tip, toast);
  buildSide(); syncDrawer();

  /* ---------- router ---------- */
  const parse = () => {
    const raw = location.hash.replace(/^#\/?/, '');
    const qi = raw.indexOf('?');
    const parts = (qi < 0 ? raw : raw.slice(0, qi)).split('/').filter(Boolean).map((p) => { try { return decodeURIComponent(p); } catch (e) { return p; } });
    return { parts, q: new URLSearchParams(qi < 0 ? '' : raw.slice(qi + 1)) };
  };
  const qs = (q) => { const s = q.toString().replace(/%40/g, '@').replace(/%2C/g, ','); return s ? '?' + s : ''; };
  const setQ = (q) => { const { parts } = parse(); history.replaceState(null, '', '#/' + parts.map(encodeURIComponent).join('/') + qs(q)); };
  let cleanups = [], mounts = [], lastPath = null, CTX = null, applyOpenAttachments = null;
  const contextKey = 'cucumber-reporter:context:v2:' + location.pathname;
  try { const saved = JSON.parse(sessionStorage.getItem(contextKey) || 'null'); if (saved && typeof saved.href==='string' && saved.href.length<8192 && /^#\/(scenarios|features)([/?]|$)/.test(saved.href)) CTX = {ids:[], href:saved.href, label:'filtered scenarios', scroll:Number.isFinite(saved.scroll)?saved.scroll:0}; } catch (e) { /* optional storage */ }
  let contextTimer=0, restoringList=false;
  const rememberContext = () => { clearTimeout(contextTimer); if(!CTX)return; try { if(!CTX.href || CTX.href.length>8192){sessionStorage.removeItem(contextKey);return;} sessionStorage.setItem(contextKey, JSON.stringify({href:CTX.href,label:CTX.label,scroll:CTX.scroll})); } catch (e) { /* optional storage */ } };
  W.addEventListener('scroll', () => { if (!restoringList && CTX && CTX.href===location.hash && main.querySelector('.slist')) { CTX.scroll = W.scrollY; CTX.anchor = null; clearTimeout(contextTimer); contextTimer=setTimeout(rememberContext,180); } }, { passive: true });
  W.addEventListener('pagehide',rememberContext);
  const onCleanup = (f) => cleanups.push(f);
  const onMount = (f) => mounts.push(f); // runs once the view is in the document
  const resolveScenarioID = (id) => SB[id] ? id : (D.aliases || {})[id] || id;
  const markNav = () => {
    const { parts } = parse();
    const k = parts[0] === 'features' && parts[1] ? 'f:' + parts[1] : parts[0] === 'scenarios' && parts[1] && SB[resolveScenarioID(parts[1])] ? 'f:' + SB[resolveScenarioID(parts[1])].f.id : (parts[0] || P.view);
    for (const a of side.querySelectorAll('.nav-i[data-k]')) {
      const on = a.dataset.k === k;
      if (on) a.setAttribute('aria-current', 'page'); else a.removeAttribute('aria-current');
    }
  };
  const crumbs = (items) => h('nav', { class: 'crumbs', 'aria-label': 'Breadcrumb' }, h('ol', null, items.map((it, i) => h('li', null,
    i < items.length - 1 ? h('a', { href: it[1] }, bdi(it[0])) : h('span', { 'aria-current': 'page' }, bdi(it[0]))))));
  const head = (title, sub, extra, pre) => h('header', { class: 'page-h' }, h('div', { class: 'page-ht' }, pre, h('h1', { class: 'page-t', tabindex: '-1' }, bdi(title)), sub && h('p', { class: 'page-s' }, sub)), extra && h('div', { class: 'page-a' }, extra));

  const render = (event) => {
    const { parts, q } = parse();
    const name = parts[0] || P.view;
    const path = parts.join('/');
    const samePath = path === lastPath;
    // A bookmark within the currently mounted scenario changes its reveal target,
    // not the user's filters, expansion state or attempt selection.
    if (event && event.type === 'hashchange' && samePath && name === 'scenarios' && parts.length > 1) {
      toggleSide(false);
      if(q.has('step')||q.has('attempt')) focusStepTarget(q); else main.querySelector('.page-t')?.focus();
      return;
    }
    const returnToList = CTX && CTX.href === location.hash && !q.has('step') ? { href: CTX.href, scroll: CTX.scroll || 0, anchor: CTX.anchor } : null;
    restoringList = !!returnToList;
    const restoreFocus=retainFocus(main);
    for (const f of cleanups) f();
    cleanups = [];
    hideTip();
    toggleSide(false);
    if (printRoot.parentNode !== doc.body) doc.body.append(printRoot);
    doc.body.classList.toggle('show-print', name === 'print');
    let node;
    try { node = (VIEWS[name] || VIEWS.missing)(parts.slice(1), q); } catch (e) { node = h('div', { class: 'view' }, empty('alert', 'This view could not be shown', String(e && e.message || e))); console.error(e); }
    main.replaceChildren(node);
    for (const f of mounts.splice(0)) f();
    markNav();
    const t = $('.page-t', node);
    doc.title = (t ? t.textContent + ' · ' : '') + (D.name || D.title || 'Test report');
    if (path !== lastPath) {
      if (lastPath !== null) { if (!returnToList) W.scrollTo(0, 0); if (t) t.focus({ preventScroll: true }); }
      lastPath = path;
    }
    if(samePath)restoreFocus();
    if (returnToList) requestAnimationFrame(() => {
      if (location.hash !== returnToList.href) return;
      const box = main.querySelector('.rows'), restore = box && listRestorers.get(box);
      if (restore) restore(returnToList); else {
        W.scrollTo(0, returnToList.scroll);
        const anchor = returnToList.anchor && main.querySelector('.row[data-id="' + CSS.escape(returnToList.anchor.id) + '"]');
        if (anchor) { W.scrollTo(0, W.scrollY + anchor.getBoundingClientRect().top - returnToList.anchor.top); anchor.focus({preventScroll:true}); }
      }
      requestAnimationFrame(() => { if (location.hash === returnToList.href) { restoringList = false; CTX.scroll = W.scrollY; rememberContext(); } });
    });
    // Reveal only on a path change or a hashchange; Settings, print and reset re-renders must not move focus.
    if (name === 'scenarios' && (event || !samePath)) focusStepTarget(q);
  };
  const focusStepTarget = (q) => {
    const step = q.get('step'), attempt = q.get('attempt');
    if (attempt !== null && !/^\d+$/.test(attempt)) return;
    if (attempt !== null) {
      const option = doc.querySelector('.attempts [data-attempt-index="' + attempt + '"]');
      if (option && option.getAttribute('aria-checked') !== 'true') option.click();
    }
    if (step === null || !/^\d+$/.test(step)) return;
    const row = (attempt !== null && doc.getElementById('attempt-' + attempt + '-step-' + step)) || doc.getElementById('step-' + step);
    if (!row) return;
    if(row.hidden) doc.querySelector('.step-controls [role=radio]')?.click();
    row.hidden = false;
    for (let parent = row.parentElement; parent; parent = parent.parentElement) if (parent.tagName === 'DETAILS') parent.open = true;
    for (const detail of row.querySelectorAll('details')) detail.open = true;
    let target = row;
    const evidence = q.get('evidence');
    if (evidence !== null && /^\d+$/.test(evidence)) {
      const button = row.querySelectorAll('.att-h')[Number(evidence)];
      if (button) { if (button.getAttribute('aria-expanded') !== 'true') button.click(); target = button; }
    }
    if (target === row) target.setAttribute('tabindex', '-1');
    requestAnimationFrame(() => { target.focus({preventScroll:true}); target.scrollIntoView({block:'start',behavior:reduced.matches?'auto':'smooth'}); target.classList.remove('flash'); requestAnimationFrame(()=>target.classList.add('flash')); });
  };
  const navigate = (href, source) => {
    if (!href || !href.startsWith('#/')) return;
    toggleSide(false);
    if (href !== location.hash) {
      if (CTX && CTX.href === location.hash && main.querySelector('.slist')) {
        // Read synchronously: scrollIntoView can precede its scroll event.
        CTX.scroll = W.scrollY;
        const row = source && source.closest('.rows .row');
        CTX.anchor = row ? {id:row.dataset.id, top:row.getBoundingClientRect().top} : null;
        rememberContext();
      }
      location.hash = href;return;
    }
    const q = new URLSearchParams(href.split('?')[1] || '');
    if (q.has('step')) focusStepTarget(q); else main.querySelector('.page-t')?.focus();
  };
  doc.addEventListener('click', (e) => {
    const a = e.target.closest && e.target.closest('a[href]');
    if (!a || e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    const href = a.getAttribute('href');
    if (href && href.startsWith('#/')) {e.preventDefault();navigate(href,a);}
  });
  W.addEventListener('hashchange', render);

  /* ---------- virtual list ---------- */
  const listDisposers = new WeakMap(), listRestorers = new WeakMap(), registeredLists = new WeakSet();
  const disposeList = (box) => { const dispose = listDisposers.get(box); if(dispose)dispose();listDisposers.delete(box);listRestorers.delete(box); };
  const vlist = (box, items, row) => {
    disposeList(box);
    if(!registeredLists.has(box)){registeredLists.add(box);onCleanup(()=>disposeList(box));}
    if (items.length <= 150) { box.replaceChildren(...items.map(row)); return; }
    const pad = h('div', { class: 'vl' }), win = h('div', { class: 'vl-w' });
    pad.append(win); box.replaceChildren(pad);
    // The row height is measured on a row in the document (a detached row measures 0); it is measured
    // again on resize, since density and width change it.
    let rh = 56, a0 = -1, b0 = -1, raf = 0;
    const measure = () => {
      if (!pad.isConnected) return;
      const probe = win.firstElementChild || win.appendChild(row(items[0]));
      const m = probe.getBoundingClientRect().height;
      if (m > 0 && m !== rh) { rh = m; a0 = b0 = -1; }
      css(pad, 'height', items.length * rh + 'px');
    };
    const upd = () => {
      raf = 0;
      const r = pad.getBoundingClientRect();
      const a = Math.max(0, Math.floor(-r.top / rh) - 8), b = Math.min(items.length, Math.ceil((-r.top + W.innerHeight) / rh) + 8);
      if (a === a0 && b === b0) return;
      a0 = a; b0 = b;
      const f = doc.activeElement && win.contains(doc.activeElement) ? doc.activeElement.dataset.id : null;
      css(win, 'transform', 'translateY(' + a * rh + 'px)');
      win.replaceChildren(...items.slice(a,b).map((item,i)=>{const node=row(item);node.setAttribute('aria-posinset',a+i+1);node.setAttribute('aria-setsize',items.length);return node;}));
      if (f) { const el = win.querySelector('[data-id="' + CSS.escape(f) + '"]'); if (el) el.focus({ preventScroll: true }); }
    };
    const on = () => { if (!raf) raf = requestAnimationFrame(upd); };
    const resized = () => { measure(); on(); };
    W.addEventListener('scroll', on, { passive: true });
    W.addEventListener('resize', resized);
    listRestorers.set(box, (position) => {
      measure();
      const index = position.anchor ? items.findIndex(s => s.id === position.anchor.id) : -1;
      const y = index < 0 ? position.scroll : pad.getBoundingClientRect().top + W.scrollY + index * rh - position.anchor.top;
      W.scrollTo(0, y); upd();
      const target = index < 0 ? null : win.querySelector('.row[data-id="' + CSS.escape(position.anchor.id) + '"]');
      if (target) target.focus({preventScroll:true});
    });
    listDisposers.set(box, () => { W.removeEventListener('scroll', on); W.removeEventListener('resize', resized); if(raf)cancelAnimationFrame(raf); });
    if (pad.isConnected) { measure(); upd(); } else onMount(() => { measure(); upd(); });
  };

  /* ---------- scenario list with filters ---------- */
  const SORTS = [['order', 'Run order'], ['severity', 'Severity'], ['slowest', 'Slowest first'], ['name', 'Name']];
  const filteredScenarios = (pool,st) => {
    const toks=tokens(st.q),tagTest=tagPredicate(st.expression||'');
    const hay=(s)=>s.hay||(s.hay=[s.name,s.f.name,s.tags.join(' '),...s.steps.map((x)=>(x.keyword||'')+x.name+' '+(x.error||''))].join('\n').toLowerCase());
      const [lo, hi] = st.dur ? st.dur.split('-').map((x) => (x === '' ? Infinity : +x * 1e6)) : [0, Infinity];
      let list = pool.filter((s) => (!st.status.size || st.status.has(s.status)) && (!st.tag || s.tags.includes(st.tag)) && (!st.feature || st.feature.split(',').includes(s.f.id)) && (!st.dur || (s.dur >= lo && s.dur < hi)) && (!toks.length || toks.every((t) => hay(s).includes(t))));
      list=list.filter((s)=>tagTest([...s.f.tags,...s.tags])&&(!st.issue||s.ann.some((a)=>(a.ref||'').toLowerCase().includes(st.issue.toLowerCase())))&&(!st.evidence||(st.evidence==='none'?!s.steps.some((x)=>(x.att||[]).length):s.steps.some((x)=>(x.att||[]).length)))&&(!st.change||(s.analysis&&st.change.split(',').includes(s.analysis.change)))&&(!st.retry||!!(s.analysis&&s.analysis.retried)===(st.retry==='yes')));
      if (st.sort === 'severity') list = list.slice().sort((a, b) => SEV.indexOf(a.status) - SEV.indexOf(b.status) || a.i - b.i);
      else if (st.sort === 'slowest') list = list.slice().sort((a, b) => b.dur - a.dur);
      else if (st.sort === 'name') list = list.slice().sort((a, b) => a.name.localeCompare(b.name));
    return list;
  };
  const scenarioList = (pool, q, fixedFeature) => {
    const wrap = h('div', { class: 'slist' });
    const st = { q: q.get('q') || '', status: new Set((q.get('status') || '').split(',').filter((s) => ST[s])), tag: q.get('tag') || '', feature: fixedFeature ? '' : q.get('feature') || '', sort: q.get('sort') || 'order', dur: q.get('dur') || '', expression: q.get('tags') || '', issue: q.get('issue') || '', evidence: q.get('evidence') || '', change: q.get('change') || '', retry: q.get('retry') || '', layout: q.get('layout') || 'list' };
    const search = h('input', { class: 'input', type: 'search', placeholder: 'Filter scenarios, steps, errors…', 'aria-label': 'Filter scenarios', value: st.q });
    const counts = zero();
    for (const s of pool) counts[s.status]++;
    const chips = h('div', { class: 'fchips', role: 'group', 'aria-label': 'Status filter' });
    const drawChips = () => chips.replaceChildren(h('button', { type: 'button', class: 'fchip', 'data-focus-key':'status-all', 'aria-pressed': String(!st.status.size), onclick: () => { st.status.clear(); upd(); } }, 'All ', h('span', { class: 'fc-n' }, num(pool.length))),
      ...ORDER.filter((k) => counts[k]).map((k) => h('button', { type: 'button', class: 'fchip s-' + k, 'data-focus-key':'status-'+k, 'aria-pressed': String(st.status.has(k)), onclick: () => { st.status.has(k) ? st.status.delete(k) : st.status.add(k); upd(); } }, icon(ST[k][1]), ST[k][0] + ' ', h('span', { class: 'fc-n' }, num(counts[k])))));
    const sel = (label, opts, cur, on) => {
      const s = h('select', { class: 'input select', 'aria-label': label, onchange: (e) => on(e.target.value) }, opts.map(([v, l]) => h('option', { value: v, selected: v === cur }, l)));
      return h('label', { class: 'sel' }, h('span', { class: 'sel-l' }, label), s);
    };
    const tagsHere = [...new Set(pool.flatMap((s) => s.tags))].sort();
    const tools = h('div', { class: 'toolbar' }, h('div', { class: 'tb-search' }, icon('search'), search),
      fixedFeature ? null : sel('Feature', [['', 'All features']].concat(F.map((f) => [f.id, f.name])), st.feature, (v) => { st.feature = v; upd(); }),
      sel('Tag', [['', 'All tags']].concat(tagsHere.map((t) => [t, t])), st.tag, (v) => { st.tag = v; upd(); }),
      sel('Sort', SORTS, st.sort, (v) => { st.sort = v; upd(); }));
    let currentList = [], tableSort = q.get('tableSort') || '', tableAsc = q.get('tableDir') !== 'desc';
    const advancedError = h('p', { class: 's-failed', role: 'status' });
    const advanced = h('details', { class: 'advanced-filters' }, h('summary', null, 'More filters'));
    const extraInputs = {};
    for (const [key,label,placeholder] of [['expression','Tag expression','@smoke and not @slow'],['issue','Issue reference','Issue key'],['evidence','Evidence','any or none'],['change','Change','new-failure, fixed, added, newly-skipped'],['retry','Retries','yes or no']]) {
      if (['change','retry'].includes(key) && !D.analysis) continue;
      const input = h('input', { class:'input', type:'search', value:st[key], placeholder, 'aria-label':label }); extraInputs[key]=input;
      input.addEventListener('input',()=>{st[key]=input.value.trim();upd();});advanced.append(h('label',null,label,input));
    }
    if(!fixedFeature){const featureChoices=h('div',{class:'feature-choices'});const searchFeatures=h('input',{class:'input',type:'search','aria-label':'Search feature filters',placeholder:'Search feature filters…'});for(const f of F){const input=h('input',{type:'checkbox',checked:st.feature.split(',').includes(f.id),onchange:()=>{const selected=[...featureChoices.querySelectorAll('input:checked')].map((x)=>x.value);st.feature=selected.join(',');upd();},value:f.id});featureChoices.append(h('label',null,input,bdi(f.name)));}searchFeatures.addEventListener('input',()=>{for(const label of featureChoices.children)label.hidden=!label.textContent.toLowerCase().includes(searchFeatures.value.toLowerCase());});advanced.append(foldContent('Select multiple features',h('div',null,searchFeatures,featureChoices),false));}
    advanced.append(advancedError);
    const viewName = h('input', {class:'input',placeholder:'View name','aria-label':'Saved view name'});
    const savedSelect = h('select',{class:'input','aria-label':'Saved view'},h('option',{value:''},'Saved views'));
    const viewsKey='cucumber-reporter:saved-views:v1';let savedViews=[];
    const loadViews=()=>{try{const x=JSON.parse(localStorage.getItem(viewsKey)||'[]');savedViews=Array.isArray(x)?x.filter((v)=>v&&typeof v.name==='string'&&typeof v.query==='string').slice(0,30):[];}catch(e){savedViews=[];}savedSelect.replaceChildren(h('option',{value:''},'Saved views'),...savedViews.map((v,i)=>h('option',{value:i},v.name)));};loadViews();
    savedSelect.addEventListener('change',()=>{if(savedSelect.value!=='')navigate('#/scenarios?'+savedViews[+savedSelect.value].query);});
    const importView=h('input',{type:'file',accept:'.json',class:'sr','aria-label':'Import saved views'});
    importView.addEventListener('change',async()=>{try{const file=importView.files[0];if(!file||file.size>65536)throw new Error('View file exceeds limit');const data=JSON.parse(await file.text());if(!Array.isArray(data)||!data.every((v)=>v&&typeof v.name==='string'&&typeof v.query==='string'))throw new Error('Invalid view file');localStorage.setItem(viewsKey,JSON.stringify(data.slice(0,30)));loadViews();say('Views imported');}catch(e){say('Could not import views');}});
    const listActions=h('div',{class:'list-actions'},seg('Scenario layout',[['list','List'],['table','Table']],st.layout,(v)=>{st.layout=v;upd();}),
      btn('Export rerun list','save',()=>downloadText('rerun.txt',rerunText(currentList)),'sm'),
      btn('Export selection CSV','save',()=>{const cell=(v)=>'"'+String(v).replace(/"/g,'""')+'"';const safe=(v)=>/^[\s]*[=+@-]/.test(String(v))?"'"+v:v;downloadText('scenarios.csv',[['Scenario','Feature','Status','Duration ns'],...currentList.map((x)=>[safe(x.name),safe(x.f.uri),x.status,x.dur])].map((r)=>r.map(cell).join(',')).join('\r\n'),'text/csv');},'sm'),
      h('details',null,h('summary',null,'Reusable views'),viewName,btn('Save view','save',()=>{if(!viewName.value.trim())return;try{savedViews=savedViews.filter((v)=>v.name!==viewName.value.trim());const savedQuery=parse().q;if(fixedFeature)savedQuery.set('feature',fixedFeature.id);savedViews.push({name:viewName.value.trim(),query:savedQuery.toString()});localStorage.setItem(viewsKey,JSON.stringify(savedViews.slice(-30)));loadViews();say('View saved');}catch(e){say('Storage unavailable');}},'sm'),savedSelect,btn('Export views','save',()=>downloadText('views.json',JSON.stringify(savedViews,null,2),'application/json'),'sm'),btn('Import views','file',()=>importView.click(),'sm'),importView));
    const active = h('div', { class: 'active-f', 'aria-live': 'polite' });
    const box = h('div', { class: 'rows', role: 'list' });
    let tq = 0;
    search.addEventListener('input', () => { clearTimeout(tq); tq = setTimeout(() => { st.q = search.value.trim(); upd(); }, 60); });
    const upd = () => {
      const restore = retainFocus(wrap, search);
      disposeList(box);
      const toks = tokens(st.q);
      let tagTest;try{tagTest=tagPredicate(st.expression);advancedError.textContent='';}catch(e){advancedError.textContent=e.message;currentList=[];CTX={ids:[],label:'invalid filter',href:location.hash,scroll:0};active.textContent='No selection: invalid tag expression';box.replaceChildren(empty('search','Invalid tag expression','Correct the expression to see results.'));return;}
      let list=filteredScenarios(pool,st);
      if(st.layout==='table' && tableSort)list=sortTableList(list,tableSort,tableAsc);
      const nq = new URLSearchParams();
      if (st.q) nq.set('q', st.q);
      if (st.status.size) nq.set('status', [...st.status].join(','));
      if (st.feature) nq.set('feature', st.feature);
      if (st.tag) nq.set('tag', st.tag);
      if (st.dur) nq.set('dur', st.dur);
      if (st.sort !== 'order') nq.set('sort', st.sort);
      for(const [key,param] of [['expression','tags'],['issue','issue'],['evidence','evidence'],['change','change'],['retry','retry']])if(st[key])nq.set(param,st[key]);
      if(st.layout==='table'){nq.set('layout','table');if(tableSort){nq.set('tableSort',tableSort);nq.set('tableDir',tableAsc?'asc':'desc');}}
      setQ(nq);
      for(const input of advanced.querySelectorAll('.feature-choices input'))input.checked=st.feature.split(',').includes(input.value);
      drawChips();
      const af = [];
      const rm = (label, fn) => af.push(h('button', { type: 'button', class: 'achip', onclick: () => { fn(); upd(); }, 'aria-label': 'Remove filter ' + label }, label, icon('close')));
      if (st.feature) rm('Feature: ' + (FB[st.feature] ? FB[st.feature].name : st.feature), () => { st.feature = ''; tools.querySelector('[aria-label=Feature]').value = ''; });
      if (st.tag) rm('Tag: ' + st.tag, () => { st.tag = ''; tools.querySelector('[aria-label=Tag]').value = ''; });
      if (st.dur) rm('Duration: ' + st.dur.split('-').map((x) => (x === '' ? '∞' : dur(+x * 1e6))).join(' – '), () => { st.dur = ''; });
      if (st.q) rm('Text: ' + st.q, () => { st.q = ''; search.value = ''; });
      for(const key of ['expression','issue','evidence','change','retry'])if(st[key])rm(key+': '+st[key],()=>{st[key]='';if(extraInputs[key])extraInputs[key].value='';upd();});
      const filtered = af.length || st.status.size;
      fill(active, h('span', { class: 'count' }, pool.some(s=>s.runHook) ? executionLabel(list) : list.length === pool.length ? plural(pool.length, 'scenario') : num(list.length) + ' of ' + plural(pool.length, 'scenario')), ...af,
        filtered ? h('button', { type: 'button', class: 'linkbtn', onclick: () => { st.q = ''; search.value = ''; st.status.clear(); st.tag = ''; st.feature = ''; st.dur = ''; for(const key of ['expression','issue','evidence','change','retry']){st[key]='';if(extraInputs[key])extraInputs[key].value='';} for (const s of tools.querySelectorAll('select')) if (s.getAttribute('aria-label') !== 'Sort') s.value = ''; upd(); } }, 'Clear filters') : null);
      CTX = { ids: list.map((s) => s.id), label: filtered ? 'filtered list' : fixedFeature ? fixedFeature.name : 'all scenarios', href: location.hash, scroll: CTX && CTX.href === location.hash ? CTX.scroll : 0, anchor: CTX && CTX.href === location.hash ? CTX.anchor : null }; rememberContext();
      currentList=list;
      if (!list.length) { box.replaceChildren(empty('search', 'No scenarios match these filters', 'Remove a filter or clear the search to see more.')); restore(); return; }
      if(st.layout==='table'){
        let page=0;const rows=h('div'),pageLabel=h('span',{role:'status'});
        const previous=btn('Previous','left',()=>{page--;draw();if(previous.disabled)next.focus();},'sm'),next=btn('Next','right',()=>{page++;draw();if(next.disabled)previous.focus();},'sm');
        const draw=()=>{rows.replaceChildren(tableRows(list.slice(page*200,(page+1)*200),(key)=>{tableAsc=key===tableSort?!tableAsc:true;tableSort=key;upd();},tableSort,tableAsc));pageLabel.textContent='Page '+(page+1)+' of '+Math.ceil(list.length/200);previous.disabled=page===0;next.disabled=(page+1)*200>=list.length;};
        box.replaceChildren(rows,h('div',{class:'toolbar'},previous,pageLabel,next));draw();
      }else vlist(box, list, (s) => srow(s, toks, !fixedFeature));
      restore();
    };
    wrap.append(h('div',{class:'filter-sticky'},tools,chips,active),advanced,listActions,box);
    upd();
    return wrap;
  };
  const strip = (s) => {
    const g = h('span', { class: 'strip', 'aria-hidden': 'true' });
    for (const st of s.steps.slice(0, 14)) g.append(h('i', { class: 'f-' + st.vs }));
    if (s.steps.length > 14) g.append(h('em', null, '+' + (s.steps.length - 14)));
    return g;
  };
  const srow = (s, toks, showF) => h('div', {role:'listitem'}, h('a', { class: 'row', href: scenarioAddress(s.id), 'data-id': s.id },
    chip(s.status, true),
    h('span', { class: 'row-m' }, h('span', { class: 'row-t' }, bdi(hl(s.name, toks))),
      h('span', { class: 'row-s' }, showF ? bdi(s.f.name, 'row-f') : null, s.bad && s.bad.error ? h('span', { class: 'row-e' }, hl(firstLine(s.bad.error), toks)) : h('span', { class: 'row-tags' }, s.tags.slice(0, 3).join('  ')))),
    strip(s), h('span', { class: 'row-d' }, dur(s.dur)), icon('right', 'row-go')));

  /* ---------- views ---------- */
  const VIEWS = Object.create(null);
  const view = (cls, ...k) => h('div', { class: 'view ' + (cls || '') }, ...k);
  const go = (path) => navigate('#/' + path);

  const dashboardView = () => {
    const w = P.widgets;
    const g = h('div', { class: 'dash' });
    const sum = [];
    sum.push(T.sc.known ? num(T.sc.passed + T.sc.known) + ' of ' + num(executed) + ' executed scenarios ended as expected (' + num(T.sc.passed) + ' passed)' : num(T.sc.passed) + ' of ' + num(executed) + ' executed scenarios passed');
    if (T.sc.failed) sum.push(plural(T.sc.failed, 'unexpected failure'));
    if (T.sc.known) sum.push(plural(T.sc.known, 'known failure'));
    if (blocking) sum.push(num(blocking) + ' undefined or ambiguous');
    if (T.sc.pending) sum.push(num(T.sc.pending) + ' pending');
    if (T.sc.skipped) sum.push(num(T.sc.skipped) + ' skipped');
    if (w.hero) {
      g.append(h('section', { class: 'hero t-' + tone, 'aria-label': 'Run verdict' },
        h('div', { class: 'hero-i' }, icon(tone === 'fail' ? 'x' : tone === 'warn' ? 'flag' : 'check')),
        h('div', { class: 'hero-m' }, h('p', { class: 'eyebrow' }, 'Run verdict'), h('h1', { class: 'hero-t page-t', tabindex: '-1' }, bdi(verdict)), h('p', { class: 'hero-s' }, sum.join(' · ') + '.'),
          h('dl', { class: 'hero-f' }, (D.facts || []).slice(0, 6).map((f) => h('div', null, h('dt', null, f.label), h('dd', null, bdi(f.value)))))),
        outcomeRing()));
    } else g.append(h('h1', { class: 'page-t sr', tabindex: '-1' }, 'Dashboard'));
    if (w.kpis) {
      const kpi = (href, label, val, fmt, sub, bar, ic) => h('a', { class: 'kpi', href }, h('span', { class: 'kpi-l' }, icon(ic), label), countUp(h('span', { class: 'kpi-v' }), val, fmt), bar || null, h('span', { class: 'kpi-s' }, sub));
      const SHORT = { passed: 'passed', failed: 'failed', known: 'known', ambiguous: 'ambiguous', undefined: 'undefined', pending: 'pending', skipped: 'skipped' };
      const top2 = (c) => ORDER.filter((k) => c[k] && k !== 'passed').sort((a, b) => SEV.indexOf(a) - SEV.indexOf(b)).slice(0, 2).map((k) => num(c[k]) + ' ' + SHORT[k]).concat(c.passed ? [num(c.passed) + ' passed'] : []).slice(0, 3).join(' · ');
      const slow = S.reduce((a, s) => (s.dur > (a ? a.dur : -1) ? s : a), null);
      g.append(h('div', { class: 'kpis' },
        kpi('#/features', 'Features', F.length, (v) => num(Math.round(v)), top2(T.fe), sbar(T.fe, { cls: 'micro', delay: 100 }), 'features'),
        kpi('#/scenarios', 'Scenarios', scenarioCount, (v) => num(Math.round(v)), top2(T.sc), sbar(T.sc, { cls: 'micro', delay: 160 }), 'list'),
        kpi('#/timeline', 'Steps', STEPS.length, (v) => num(Math.round(v)), top2(T.st), sbar(T.st, { cls: 'micro', delay: 220 }), 'timeline'),
        kpi(HEAD.href, HEAD.label, HEAD.value, fpct, HEAD.sub, null, 'check'),
        kpi(D.timingSection ? '#/run?section=' + encodeURIComponent(D.timingSection) : '#/scenarios?sort=slowest', D.wallClock ? 'Total elapsed time' : 'Summed step time', T.dur, (v) => D.wallClock || dur(Math.round(v)), [D.wallClock ? 'summed steps ' + dur(T.dur) : '', slow ? 'slowest ' + dur(slow.dur) : ''].filter(Boolean).join(' · '), null, 'clock')));
    }
    // Two cards that share a row are fitted once laid out: the first card's list takes as many rows
    // (from most down to fewest) as fit beside the second, so neither column ends in a large empty area.
    // Narrow layouts stack the cards and keep the most rows.
    const fitted = (build, other, most, fewest, builtFirst, slack) => {
      let cur = build(most);
      const row = h('div', { class: 'pair' }, builtFirst ? [cur, other] : [other, cur]);
      const fit = () => {
        if (!row.isConnected || getComputedStyle(row).gridTemplateColumns.split(' ').length < 2) return;
        row.classList.add('fit');
        for (let n = most; ; n--) {
          const c = build(n);
          row.replaceChild(c, cur);
          cur = c;
          if (n <= fewest || c.offsetHeight <= other.offsetHeight + slack) break;
        }
        row.classList.remove('fit');
      };
      onMount(fit);
      let rt = 0;
      const resized = () => { clearTimeout(rt); rt = setTimeout(fit, 120); };
      W.addEventListener('resize', resized);
      onCleanup(() => W.removeEventListener('resize', resized));
      return row;
    };
    const sc = w.status && statusCard();
    if (w.features && sc) {
      // The feature list keeps its rows unless it would leave a large gap beside the status card.
      g.append(fitted(featureCard, sc, 12, 1, false, 96));
    } else if (sc || w.features) g.append(sc || featureCard());
    const kc = w.known && knownCard();
    const secs = w.sections && sectionsCard();
    if (w.attention && kc) {
      // Needs attention beside a column of the known-failures card and, while they fit, the first
      // section cards: the column takes sections until it is about as tall as the full attention list,
      // then the list sheds rows (down to one per group) to end with the column.
      const side = h('div', { class: 'stack' }, kc);
      let att = attentionCard(12);
      const row = h('div', { class: 'pair r2' }, att, side);
      const fit = () => {
        if (!row.isConnected) return;
        if (secs) secs.prepend(...[...side.children].slice(1));
        if (getComputedStyle(row).gridTemplateColumns.split(' ').length < 2) return;
        row.classList.add('fit');
        const swap = (n) => { const c = attentionCard(n); row.replaceChild(c, att); att = c; };
        // Try 0, 1, 2 … section cards beside the known card; for each, shed attention rows until the list
        // ends with the column; keep the arrangement whose columns end closest together.
        const pool = secs ? [...secs.children] : [];
        let best = null;
        for (let k = 0; k <= pool.length; k++) {
          side.append(...pool.slice(0, k));
          swap(12);
          let n = 12;
          while (n > 1 && att.offsetHeight > side.offsetHeight + 8) swap(--n);
          const d = Math.abs(att.offsetHeight - side.offsetHeight);
          if (!best || d < best.d) best = { k, n, d };
          if (secs) secs.prepend(...pool.slice(0, k));
          if (side.offsetHeight > att.offsetHeight + 200) break;
        }
        side.append(...pool.slice(0, best.k));
        swap(best.n);
        row.classList.remove('fit');
      };
      onMount(fit);
      let rt = 0;
      const resized = () => { clearTimeout(rt); rt = setTimeout(fit, 120); };
      W.addEventListener('resize', resized);
      onCleanup(() => W.removeEventListener('resize', resized));
      g.append(row);
    } else if (w.attention) g.append(attentionCard()); else if (kc) g.append(kc);
    if (secs) g.append(secs);
    if (w.duration) g.append(durationCard());
    if (!g.children.length || (g.children.length === 1 && !w.hero)) g.append(empty('sliders', 'Every dashboard widget is hidden', 'Open Settings to choose what the dashboard shows.'));
    return view('v-dash', g);
  };

  const statusCard = () => {
    let mode = 'sc';
    const hide = new Set();
    const body = h('div', { class: 'donut-w' });
    const draw = () => {
      const restore = retainFocus(body);
      const c = mode === 'sc' ? T.sc : T.st, unit = mode === 'sc' ? 'scenario' : 'step';
      const tot = ORDER.reduce((a, k) => a + (hide.has(k) ? 0 : c[k]), 0);
      const legend = h('ul', { class: 'legend' }, ORDER.filter((k) => c[k]).map((k) => h('li', null, h('button', { type: 'button', class: 'leg s-' + k, 'data-focus-key':'legend-'+k, 'aria-pressed': String(!hide.has(k)), 'aria-label': (hide.has(k) ? 'Show ' : 'Hide ') + ST[k][0], onclick: () => { hide.has(k) ? hide.delete(k) : hide.add(k); draw(); } },
        h('span', { class: 'sw f-' + k }), icon(ST[k][1]), h('span', { class: 'leg-l' }, ST[k][0]), h('span', { class: 'leg-n' }, num(c[k])), h('span', { class: 'leg-p' }, hide.has(k) ? '—' : fpct(pct(c[k], tot)))))));
      const pick = (k) => go(mode === 'sc' ? 'scenarios?status=' + k : 'scenarios?status=' + (k === 'known' ? 'known' : k));
      body.replaceChildren(h('div', { class: 'donut-c' }, donut(c, unit, hide, pick), h('div', { class: 'donut-n' }, h('b', null, num(tot)), h('span', null, unit + 's'))), legend); restore();
    };
    draw();
    return card('c-status', 'Status breakdown', 'Select a segment to list those scenarios', seg('Count', [['sc', 'Scenarios'], ['st', 'Steps']], mode, (v) => { mode = v; draw(); }), body);
  };

  const featureCard = (rows) => {
    rows = rows || 12;
    let mode = 'share';
    const hide = new Set();
    const list = h('div', { class: 'fbars' });
    const max = F.reduce((max, f) => Math.max(max, f.sc.length), 1);
    const keys = ORDER.filter((k) => T.sc[k]);
    const legend = h('div', { class: 'chipleg', role: 'group', 'aria-label': 'Statuses shown' });
    const draw = () => {
      const restore = retainFocus(legend);
      legend.replaceChildren(...keys.map((k) => h('button', { type: 'button', class: 'leg mini s-' + k, 'data-focus-key':'legend-'+k, 'aria-pressed': String(!hide.has(k)), onclick: () => { hide.has(k) ? hide.delete(k) : hide.add(k); draw(); } }, h('span', { class: 'sw f-' + k }), ST[k][0])));
      const sorted = F.slice().sort((a, b) => SEV.indexOf(a.status) - SEV.indexOf(b.status) || a.name.localeCompare(b.name));
      list.replaceChildren(...sorted.slice(0, rows).map((f, i) => {
        const bar = sbar(f.n, { hide, link: (k) => '#/features/' + encodeURIComponent(f.id) + '?status=' + k, of: f.name, delay: 80 + i * 40 });
        const holder = h('div', { class: 'fb-bar' }, bar);
        if (mode === 'count') css(bar, 'width', (f.sc.length / max * 100) + '%');
        const bad = f.n.failed + f.n.ambiguous + f.n.undefined;
        return h('div', { class: 'fb-row' }, h('a', { class: 'fb-n', href: '#/features/' + encodeURIComponent(f.id) }, glyph(f.status), bdi(f.name)), holder,
          h('span', { class: 'fb-m' }, h('b', null, num(f.n.passed)), '/' + num(f.sc.length), bad ? h('span', { class: 'bad' }, ' · ' + bad + ' failing') : f.n.known ? h('span', { class: 'kn' }, ' · ' + f.n.known + ' known') : null));
      }));
      if (F.length > rows) list.append(h('a', { class: 'more', href: '#/features' }, 'All ' + F.length + ' features', icon('arrow')));
      restore();
    };
    draw();
    return card('c-feat', 'Results per feature', 'Scenarios by outcome, most affected first', seg('Scale', [['share', 'Share'], ['count', 'Count']], mode, (v) => { mode = v; draw(); }), legend, list);
  };

  const attnList = (limit) => {
    const groups = [['failed', 'Unexpected failures'], ['ambiguous', 'Ambiguous steps'], ['undefined', 'Undefined steps'], ['known', 'Known failures'], ['pending', 'Pending']];
    const out = [];
    // Every non-empty group keeps its header, its count and at least one row; the remaining rows up to
    // the limit go to the groups in order, and the rest of a group is one "+N more" link.
    const present = groups.map(([k, l]) => [k, l, S.filter((s) => s.status === k)]).filter((g) => g[2].length);
    let spare = Math.max(0, limit - present.length);
    for (const [k, l, xs] of present) {
      const shown = Math.min(xs.length, 1 + spare);
      spare -= shown - 1;
      out.push(h('li', { class: 'att-g' }, l, h('span', null, num(xs.length))));
      for (const s of xs.slice(0, shown)) {
        out.push(h('li', null, h('a', { class: 'att', href: '#/scenarios/' + encodeURIComponent(s.id) + (s.bad ? '?step=' + s.bad.k : '') }, glyph(s.status),
          h('span', { class: 'att-m' }, h('span', { class: 'att-t' }, bdi(s.name)), h('span', { class: 'att-s' }, bdi(s.f.name), s.bad ? [' · ', h('span', { class: 'mono' }, firstLine(s.bad.error) || s.bad.keyword + s.bad.name)] : null)),
          h('span', { class: 'att-d' }, dur(s.dur)))));
      }
      if (xs.length > shown) out.push(h('li', { class: 'att-more' }, h('a', { class: 'link', href: '#/scenarios?status=' + k }, '+' + num(xs.length - shown) + ' more', icon('arrow'))));
    }
    return out;
  };
  const attentionCard = (limit) => {
    const items = attnList(limit || 8);
    const total = T.sc.failed + blocking + T.sc.known + T.sc.pending;
    return card('c-attn', 'Needs attention', total ? 'Unexpected failures first' : null, total ? h('a', { class: 'link', href: '#/failures' }, 'All failures', icon('arrow')) : null,
      items.length ? h('ul', { class: 'attn' }, items) : empty('check', 'Nothing needs attention', 'Every executed scenario passed.', 'ok'));
  };

  const knownCard = () => {
    const u = T.sc.failed, k = T.sc.known, tot = u + k;
    const refs = new Map();
    for (const a of KNOWN) { const s = SB[a.scenario]; if (!s) continue; const r = refs.get(a.ref) || { a, ids: new Set() }; if (!r.a.observed && a.observed) r.a = a; r.ids.add(s.id); r.n = r.ids.size; refs.set(a.ref, r); }
    const bar = sv('svg', { class: 'sbar big', viewBox: '0 0 100 10', preserveAspectRatio: 'none', 'aria-hidden': 'true' }, sv('g', { class: 'grow' },
      tot ? [sv('rect', { class: 'f-failed', x: 0, y: 0, width: u / tot * 100, height: 10 }), sv('rect', { class: 'f-known', x: u / tot * 100, y: 0, width: k / tot * 100, height: 10 })] : sv('rect', { class: 'f-passed', x: 0, y: 0, width: 100, height: 10 })));
    return card('c-known', 'Known and unexpected failures', 'Failures covered by a known issue are expected', null,
      h('div', { class: 'ku' },
        h('a', { class: 'ku-n s-failed', href: '#/failures' }, countUp(h('b'), u, (v) => num(Math.round(v))), h('span', null, icon('x'), 'Unexpected')),
        h('a', { class: 'ku-n s-known', href: '#/known' }, countUp(h('b'), k, (v) => num(Math.round(v))), h('span', null, icon('flag'), 'Known'))),
      bar,
      refs.size ? h('ul', { class: 'refs' }, [...refs].map(([ref, r]) => h('li', null, h('a', { href: '#/known' }, h('span', { class: 'ref' }, ref || '—'), h('span', { class: 'refs-t' }, bdi(r.a.title || '')), h('span', { class: 'refs-n' }, plural(r.n, 'scenario')))))) : h('p', { class: 'muted' }, 'No known issues are recorded for this run.'),
      NOTAPPLIED.length ? h('a', { class: 'note na', href: '#/known' }, icon('alert'), plural(NOTAPPLIED.length, 'known-issue record') + ' not applied; ' + (NOTAPPLIED.length > 1 ? 'their scenarios keep their own results.' : 'its scenario keeps its own result.')) : null,
      blocking ? h('p', { class: 'note' }, icon('alert'), num(blocking) + ' scenario' + (blocking > 1 ? 's' : '') + ' also stopped on undefined or ambiguous steps.') : null);
  };
  const notApplied = (a) => h('section', { class: 'card grp na-c' }, h('header', { class: 'grp-h' }, h('span', { class: 'ref big' }, a.ref || '—'),
    h('div', null, h('p', { class: 'banner-t' }, icon('alert'), 'Known issue, not applied'), h('p', { class: 'grp-t' }, bdi(a.title || '')), h('p', { class: 'banner-w' }, a.reason || ''))),
    h('div', { class: 'mrows' }, mini(SB[a.scenario])));

  const BUCKETS = [[0, 1, '< 1 ms'], [1, 10, '1–10 ms'], [10, 100, '10–100 ms'], [100, 1e3, '0.1–1 s'], [1e3, 1e4, '1–10 s'], [1e4, 6e4, '10 s–1 min'], [6e4, 6e5, '1–10 min'], [6e5, Infinity, '> 10 min']];
  const quant = (xs, p) => xs.length ? xs[Math.min(xs.length - 1, Math.floor(p * xs.length))] : 0;
  const durationCard = () => {
    const ran = S.filter((s) => s.status !== 'skipped');
    const xs = ran.map((s) => s.dur).sort((a, b) => a - b);
    const stats = h('dl', { class: 'stats' }, [['Median', quant(xs, 0.5)], ['95th percentile', quant(xs, 0.95)], ['Slowest', xs[xs.length - 1] || 0], ['Total', T.dur]].map(([l, v]) => h('div', null, h('dt', null, l), h('dd', null, dur(v)))));
    if (!xs.length || xs[xs.length - 1] < 1e6) return card('c-dur', 'Duration profile', null, null, stats, empty('clock', 'Every scenario finished in under 1 ms', 'Timing differences at this scale are noise, so no distribution is drawn.'));
    const n = BUCKETS.map(() => 0), bad = BUCKETS.map(() => 0), kn = BUCKETS.map(() => 0);
    for (const s of ran) { const ms = s.dur / 1e6; const i = BUCKETS.findIndex((b) => ms >= b[0] && ms < b[1]); n[i]++; if (s.status === 'known') kn[i]++; else if (s.status === 'failed') bad[i]++; }
    let a = n.findIndex((x) => x), b = n.length - 1 - [...n].reverse().findIndex((x) => x);
    while (b - a < 3) { if (a > 0) a--; else if (b < n.length - 1) b++; else break; }
    const mx = n.reduce((max, x) => Math.max(max, x), 0);
    const bars = sv('svg', { class: 'hist', viewBox: '0 0 ' + (b - a + 1) * 10 + ' 100', preserveAspectRatio: 'none', 'aria-hidden': 'true' });
    const cols = h('div', { class: 'hist-x' });
    for (let i = a; i <= b; i++) {
      const hgt = n[i] ? Math.max(3, n[i] / mx * 92) : 0, x = (i - a) * 10 + 1.5;
      bars.append(sv('g', { class: 'grow-y' }, sv('rect', { class: 'hb', x: x + 1.5, y: 100 - hgt, width: 4, height: hgt }), kn[i] ? sv('rect', { class: 'hb-kn', x: x + 1.5, y: 100 - (bad[i] + kn[i]) / mx * 92, width: 4, height: kn[i] / mx * 92 }) : null, bad[i] ? sv('rect', { class: 'hb-bad', x: x + 1.5, y: 100 - bad[i] / mx * 92, width: 4, height: bad[i] / mx * 92 }) : null));
      css(bars.lastChild, '--d', (i - a) * 60 + 'ms');
      const lo = BUCKETS[i][0], hi = BUCKETS[i][1];
      const col = h('a', { class: 'hist-c', href: '#/scenarios?sort=slowest&dur=' + lo + '-' + (hi === Infinity ? '' : hi), 'aria-label': plural(n[i], 'scenario') + ' took ' + BUCKETS[i][2] + (bad[i] ? ', ' + bad[i] + ' failed' : '') + (kn[i] ? ', ' + kn[i] + ' known failures' : '') }, h('b', null, num(n[i])), h('span', null, BUCKETS[i][2]));
      bindTip(col, () => [BUCKETS[i][2], plural(n[i], 'scenario'), [bad[i] ? bad[i] + ' failed' : '', kn[i] ? kn[i] + ' known' : ''].filter(Boolean).join(' · ') || 'no failures']);
      cols.append(col);
    }
    const slow = ran.slice().sort((x, y) => y.dur - x.dur).slice(0, 6);
    const top = slow[0] ? slow[0].dur : 1;
    const slowList = h('ol', { class: 'slow' }, slow.map((s, i) => h('li', null, h('a', { href: '#/scenarios/' + encodeURIComponent(s.id) },
      h('span', { class: 'slow-t' }, glyph(s.status), bdi(s.name)), h('span', { class: 'slow-d' }, dur(s.dur), h('small', null, fpct(pct(s.dur, T.dur)) + ' of test time')),
      sv('svg', { class: 'sbar thin', viewBox: '0 0 100 10', preserveAspectRatio: 'none', 'aria-hidden': 'true' }, css(sv('g', { class: 'grow' }, sv('rect', { class: 'f-accent', x: 0, y: 0, width: Math.max(0.6, s.dur / top * 100), height: 10 })), '--d', 200 + i * 50 + 'ms'))))));
    return card('c-dur', 'Duration profile', 'Scenario durations on a logarithmic scale; red marks failures, amber known failures', h('a', { class: 'link', href: '#/timeline' }, 'Timeline', icon('arrow')),
      stats, h('div', { class: 'dur-g' }, h('div', { class: 'hist-w' }, h('div', { class: 'hist-plot' }, bars), cols), h('div', null, h('h3', { class: 'sub-t' }, 'Slowest scenarios'), slowList)));
  };

  const sectionsCard = () => {
    const secs = (D.sections || []).filter((s) => s.role !== 'known');
    if (!secs.length) return null;
    return h('div', { class: 'secs' }, secs.map((s) => h('a', { class: 'sec t-' + (s.tone || 'info'), href: '#/run?section=' + encodeURIComponent(s.id) },
      h('h2', { class: 'sec-t' }, bdi(s.title)),
      (s.rows || []).length ? h('p', { class: 'sec-n' }, plural(s.rows.length, 'entry', 'entries')) : null,
      !(s.rows || []).length && !(s.summary || []).length && s.note ? h('p', { class: 'sec-x' }, bdi(s.note)) : null,
      h('ul', { class: 'sec-l' }, (s.summary || []).slice(0, 3).map((f) => h('li', null, h('span', null, f.label), h('b', null, bdi(f.value)))), (s.rows || []).slice(0, 2).map((f) => h('li', { class: 'sec-r' }, bdi(f.label)))))));
  };

  VIEWS.features = (p, q) => {
    if(p[0])return FB[p[0]]?featureView(FB[p[0]],q):VIEWS.missing();
    const st = { q: q.get('q') || '', sort: q.get('sort') || 'order' };
    const grid = h('div', { class: 'fgrid' });
    const search = h('input', { class: 'input', type: 'search', placeholder: 'Filter features…', 'aria-label': 'Filter features', value: st.q });
    const draw = () => {
      const toks = tokens(st.q);
      let list = F.filter((f) => toks.every((t) => (f.name + ' ' + f.uri + ' ' + f.tags.join(' ')).toLowerCase().includes(t)));
      if (st.sort === 'severity') list = list.slice().sort((a, b) => SEV.indexOf(a.status) - SEV.indexOf(b.status));
      else if (st.sort === 'slowest') list = list.slice().sort((a, b) => b.dur - a.dur);
      else if (st.sort === 'name') list = list.slice().sort((a, b) => a.name.localeCompare(b.name));
      const nq = new URLSearchParams(); if (st.q) nq.set('q', st.q); if (st.sort !== 'order') nq.set('sort', st.sort); setQ(nq);
      grid.replaceChildren(...(list.length ? list.map((f, i) => h('a', { class: 'fcard', href: '#/features/' + encodeURIComponent(f.id) },
        h('div', { class: 'fc-h' }, chip(f.status, true), h('span', { class: 'fc-d' }, icon('clock'), dur(f.dur))),
        h('h2', { class: 'fc-t' }, bdi(hl(f.name, toks))), h('p', { class: 'fc-u mono' }, bdi(f.uri)),
        sbar(f.n, { delay: i * 30 }),
        h('ul', { class: 'fc-n' }, ORDER.filter((k) => f.n[k]).map((k) => h('li', { class: 's-' + k }, icon(ST[k][1]), num(f.n[k]) + ' ' + ST[k][0].toLowerCase()))),
        h('p', { class: 'fc-f' }, executionLabel(f.sc) + ' · ' + plural(f.steps, 'step'), f.tags.length ? h('span', { class: 'fc-tags' }, f.tags.join(' ')) : null))) : [empty('search', 'No features match', 'Try a different search.')]));
    };
    let tq = 0;
    search.addEventListener('input', () => { clearTimeout(tq); tq = setTimeout(() => { st.q = search.value.trim(); draw(); }, 60); });
    const sort = h('select', { class: 'input select', 'aria-label': 'Sort', onchange: (e) => { st.sort = e.target.value; draw(); } }, SORTS.map(([v, l]) => h('option', { value: v, selected: v === st.sort }, l)));
    draw();
    return view('', crumbs([['Dashboard', '#/'], ['Features']]), head('Features', plural(F.length, 'feature') + ' · ' + plural(scenarioCount, 'scenario') + (runHookCount ? ' · '+plural(runHookCount,'run hook') : '') + ' · ' + dur(T.dur)),
      h('div', { class: 'toolbar' }, h('div', { class: 'tb-search' }, icon('search'), search), h('label', { class: 'sel' }, h('span', { class: 'sel-l' }, 'Sort'), sort)), grid);
  };

  const featureView = (f, q) => {
    const bad = f.n.failed + f.n.ambiguous + f.n.undefined;
    return view('', crumbs([['Dashboard', '#/'], ['Features', '#/features'], [f.name]]),
      head(f.name, null, null, chip(f.status)),
      h('div', { class: 'meta' }, h('span', { class: 'mono' }, icon('file'), bdi(f.uri)), h('span', null, icon('clock'), dur(f.dur)), f.tags.map(tagEl)),
      f.desc ? h('p', { class: 'desc' }, bdi(f.desc)) : null,
      h('div', { class: 'minis' }, [[f.runHooks?'Run hooks':'Scenarios', num(f.sc.length)], ['Passed', num(f.n.passed)], ['Failing', num(bad)], ['Known', num(f.n.known)], ['Steps', num(f.steps)]].map(([l, v]) => h('div', { class: 'mini' }, h('span', null, l), h('b', null, v))),
        h('div', { class: 'mini wide' }, h('span', null, 'Outcome'), sbar(f.n, { link: (k) => '#/features/' + encodeURIComponent(f.id) + '?status=' + k, of: f.name }))),
      h('h2', { class: 'sec-h' }, 'Scenarios'),
      scenarioList(f.sc, q, f));
  };


  VIEWS.scenarios = (p, q) => {
    if (p[0]) { const id = resolveScenarioID(p[0]); return SB[id] ? scenarioView(SB[id], q) : VIEWS.missing(); }
    return view('', crumbs([['Dashboard', '#/'], ['Scenarios']]), head('Scenarios', plural(scenarioCount, 'scenario') + (runHookCount ? ' · '+plural(runHookCount,'run hook') : '') + ' in ' + plural(F.length, 'feature')), scenarioList(S, q, null));
  };

  /* attachments and JSON tree */
  const decode = (b64) => { const bin = atob(b64); const u = new Uint8Array(bin.length); for (let i = 0; i < bin.length; i++) u[i] = bin.charCodeAt(i); return u; };
  const jnode = (k, v, depth) => {
    const key = k != null ? [h('span', { class: 'jk' }, typeof k === 'number' ? String(k) : JSON.stringify(k)), h('span', { class: 'jp' }, ': ')] : null;
    if (v && typeof v === 'object') {
      const arr = Array.isArray(v), ks = arr ? v.map((_, i) => i) : Object.keys(v);
      const d = h('details', { open: depth < 2 });
      const ul = h('ul', { class: 'jt' });
      let filled = false;
      const fill = () => { if (filled) return; filled = true; for (const kk of ks.slice(0, 500)) ul.append(jnode(kk, v[kk], depth + 1)); if (ks.length > 500) ul.append(h('li', { class: 'muted' }, '… ' + (ks.length - 500) + ' more')); };
      d.addEventListener('toggle', fill);
      if (depth < 2) fill();
      d.append(h('summary', null, key, h('span', { class: 'jb' }, arr ? '[' : '{'), h('span', { class: 'jn' }, ks.length + (arr ? ' items' : ' keys')), h('span', { class: 'jb' }, arr ? ']' : '}')), ul);
      return h('li', null, d);
    }
    const t = v === null ? 'null' : typeof v;
    return h('li', { class: 'jl' }, key, h('span', { class: 'jv j-' + t }, t === 'string' ? JSON.stringify(v) : String(v)));
  };
  const treeNote = () => h('p', { class: 'muted att-note' }, 'Tree shows values as JavaScript reads them; Raw and Copy keep the original text.');
  const attachmentBody = (a, open) => {
    let u8 = null, text = null, json;
    // a.data is absent when the content is not carried, and "" for an empty attachment.
    if (typeof a.data === 'string') try { u8 = decode(a.data); } catch (e) { u8 = null; }
    const mime = (a.mime || 'application/octet-stream').toLowerCase();
    const isImg = /^image\/(png|jpeg|gif|webp)$/.test(mime);
    if (u8 && !isImg) { try { text = new TextDecoder('utf-8', { fatal: true, ignoreBOM: true }).decode(u8); } catch (e) { text = null; } }
    if (text != null && /json/.test(mime)) { try { json = JSON.parse(text); } catch (e) { json = undefined; } }
    const body = h('div', { class: 'att-b' });
    const tools = h('div', { class: 'att-tools' });
    const content = h('div', { class: 'att-c' });
    // Raw and Copy always use the original text: re-serialising parsed JSON would round large numbers,
    // turn 1e400 into null and drop repeated keys. Only the tree view shows parsed values.
    const raw = () => h('pre', { class: 'code' }, text);
    if (json !== undefined) tools.append(seg('View', [['tree', 'Tree'], ['raw', 'Raw']], 'tree', (v) => content.replaceChildren(...(v === 'tree' ? [treeNote(), h('ul', { class: 'jt root' }, jnode(null, json, 0))] : [raw()]))));
    if (text != null) tools.append(btn('Copy', 'copy', () => copy(text, 'Attachment'), 'sm'));
    if (text != null) {
      const find = h('input', { class: 'input', type: 'search', placeholder: 'Find in attachment…', 'aria-label': 'Find in attachment' });
      const result = h('span', { role: 'status', class: 'muted' });
      let searchTimer=0;const lines=text.split('\n');
      onCleanup(()=>clearTimeout(searchTimer));
      find.addEventListener('input', () => {clearTimeout(searchTimer);searchTimer=setTimeout(()=>{
        if(!find.isConnected)return;
        const q = find.value.toLowerCase(), pre = raw();
        let matches=0;
        if(q){const fragment=doc.createDocumentFragment();let plain='';for(let i=0;i<lines.length;i++){const line=lines[i],ending=i<lines.length-1?'\n':'';const hit=line.toLowerCase().includes(q);if(hit)matches++;if(hit&&matches<=1000){if(plain){fragment.append(doc.createTextNode(plain));plain='';}fragment.append(h('span',{class:'matched-line'},line,ending));}else plain+=line+ending;}if(plain)fragment.append(doc.createTextNode(plain));pre.replaceChildren(fragment);}
        content.replaceChildren(pre);
        for (const button of tools.querySelectorAll('[role=radio]')) button.setAttribute('aria-checked', String(button.textContent === 'Raw'));
        result.textContent = q ? matches + ' matching lines in original text' + (matches>1000?' (first 1,000 highlighted)':'') : '';
      },180);});
      tools.append(find, result);
    }
    if (u8) tools.append(btn('Save', 'save', () => { const url = URL.createObjectURL(new Blob([u8], { type: mime })); const l = h('a', { href: url, download: a.name || 'attachment' }); doc.body.append(l); l.click(); l.remove(); setTimeout(() => URL.revokeObjectURL(url), 2000); }, 'sm'));
    if (u8 && u8.length === 0) content.append(h('p', { class: 'muted' }, 'Empty attachment (0 bytes).'));
    else if (json !== undefined) content.append(treeNote(), h('ul', { class: 'jt root' }, jnode(null, json, 0)));
    else if (text != null) content.append(raw());
    else if (isImg && u8) content.append(h('img', { class: 'att-img', alt: a.name || 'attachment', src: 'data:' + mime + ';base64,' + a.data }));
    else if (u8) content.append(h('p', { class: 'muted' }, 'Binary content is not shown. Use Save to keep the file.'));
    else if (a.href && /^attachment-[\w-]+\.(txt|json|bin|png|jpg|gif|webp)$/.test(a.href)) content.append(h('p', null, 'The content is saved beside this report: ', h('a', { href: a.href }, a.href)));
    else content.append(h('p', { class: 'muted' }, a.note || 'This attachment could not be decoded.'));
    body.append(tools, content);
    const id = 'a' + Math.random().toString(36).slice(2, 8);
    const b = h('button', { type: 'button', class: 'att-h', 'aria-expanded': String(!!open), 'aria-controls': id },
      icon('clip'), h('span', { class: 'att-n' }, bdi(a.name || 'attachment')), h('span', { class: 'att-k' }, mime + (a.size || u8 ? ' · ' + bytes(a.size || 0) : '')), icon('down', 'chev'));
    const wrap = h('div', { class: 'acc', id, hidden: !open, inert: !open }, h('div', { class: 'acc-i' }, body));
    b.addEventListener('click', () => { const o = b.getAttribute('aria-expanded') !== 'true'; b.setAttribute('aria-expanded', String(o)); wrap.hidden = !o; wrap.inert = !o; });
    return h('div', { class: 'att-x' }, b, wrap);
  };
  const errPanel = (err, label) => errorViewer(err, label);
  const dataTable = (rows) => h('div', { class: 'tbl-w' }, h('table', { class: 'tbl' }, h('thead', null, h('tr', null, rows[0].map((c) => h('th', { scope: 'col' }, bdi(c))))), h('tbody', null, rows.slice(1).map((r) => h('tr', null, r.map((c) => h('td', null, bdi(c))))))));
  const docString = (d) => {
    const lines = d.value.split('\n');
    const pre = h('pre', { class: 'code' + (lines.length > 14 ? ' clip' : '') }, d.value);
    return h('div', { class: 'docs' }, h('div', { class: 'docs-h' }, icon('doc'), h('span', null, 'Doc string'), d.type ? h('span', { class: 'att-k' }, d.type) : null, h('span', { class: 'grow-x' }), btn('Copy', 'copy', () => copy(d.value, 'Doc string'), 'sm ghost')), pre);
  };

  const baseScenarioView = (s, q) => {
    const ctx = CTX && CTX.ids.includes(s.id) ? CTX : { ids: S.map((x) => x.id), label: 'all scenarios' };
    const idx = ctx.ids.indexOf(s.id);
    const prev = SB[ctx.ids[idx - 1]], next = SB[ctx.ids[idx + 1]];
    const nav = h('div', { class: 'pn' },
      h('a', { class: 'btn' + (prev ? '' : ' off'), href: prev ? scenarioAddress(prev.id) : null, 'aria-disabled': prev ? null : 'true', 'aria-label': prev ? 'Previous scenario: ' + prev.name : 'No previous scenario', title: 'Previous ( [ )' }, icon('left')),
      h('span', { class: 'pn-c' }, num(idx + 1) + ' / ' + num(ctx.ids.length), h('small', null, ctx.label)),
      h('a', { class: 'btn' + (next ? '' : ' off'), href: next ? scenarioAddress(next.id) : null, 'aria-disabled': next ? null : 'true', 'aria-label': next ? 'Next scenario: ' + next.name : 'No next scenario', title: 'Next ( ] )' }, icon('right')));
    const grp = new Map();
    for (const a of s.ann) { const k = a.kind + '\u0000' + (a.ref || a.title); grp.set(k, (grp.get(k) || []).concat([a])); }
    const banners = [...grp.values()].map((as) => annotationBanner(as, s));
    const maxD = s.steps.reduce((max, x) => Math.max(max, x.duration || 0), 1);
    const exp = P.expand;
    const steps = h('ol', { class: 'steps' }, s.steps.map((st, k) => {
      const hasErr = st.error && st.status !== 'passed';
      const atts = (st.att || []).map((a, i) => attachment(a, exp === 'all' || (exp === 'failed' && i === 0 && hasErr && st.status === 'failed')));
      return h('li', { class: 'step s-' + st.vs, id: 'step-' + k },
        h('span', { class: 'node', role: 'img', 'aria-label': ST[st.vs][0] }, icon(ST[st.vs][1])),
        h('div', { class: 'step-b' },
          h('div', { class: 'step-h' }, h('span', { class: 'kw' }, (st.keyword || '').trim()), h('span', { class: 'txt' }, bdi(argText(st.name || ''))),
            h('span', { class: 'step-d' }, st.duration != null ? sv('svg', { class: 'dbar', viewBox: '0 0 100 10', preserveAspectRatio: 'none', 'aria-hidden': 'true' }, sv('rect', { class: 'f-' + st.vs, x: 0, y: 0, width: Math.max(1, st.duration / maxD * 100), height: 10 })) : null, h('span', null, st.duration != null ? dur(st.duration) : ST[st.vs][0].toLowerCase()))),
          st.match ? h('p', { class: 'loc mono' }, bdi(st.match)) : null,
          st.rows && st.rows.length ? dataTable(st.rows) : null,
          st.doc ? docString(st.doc) : null,
          hasErr ? errPanel(st.error, st.status === 'pending' ? 'Pending' : st.status === 'ambiguous' ? 'Ambiguous match' : 'Error') : null,
          atts.length ? h('div', { class: 'atts' }, atts) : null));
    }));
    // Open attachments changed in Settings: open or close the mounted toggles in place through their own click path.
    applyOpenAttachments = () => s.steps.forEach((st, k) => (st.att || []).forEach((a, i) => {
      const hasErr = st.error && st.status !== 'passed';
      const want = !!(P.expand === 'all' || (P.expand === 'failed' && i === 0 && hasErr && st.status === 'failed'));
      const b = steps.querySelectorAll('#step-' + k + ' .atts > .att-x')[i]?.querySelector('.att-h');
      if (b && (b.getAttribute('aria-expanded') === 'true') !== want) b.click();
    }));
    cleanups.push(() => { applyOpenAttachments = null; });
    const n = zero();
    for (const st of s.steps) n[st.vs]++;
    const allAtt = s.steps.flatMap((st) => (st.att || []).map((a) => [a, st]));
    const aside = h('aside', { class: 'sc-aside' },
      h('section', { class: 'card' }, h('h2', { class: 'card-t' }, 'Steps'), sbar(n), h('ul', { class: 'fc-n' }, ORDER.filter((k) => n[k]).map((k) => h('li', { class: 's-' + k }, icon(ST[k][1]), num(n[k]) + ' ' + ST[k][0].toLowerCase())))),
      allAtt.length ? h('section', { class: 'card' }, h('h2', { class: 'card-t' }, 'Attachments'), h('ul', { class: 'jump' }, allAtt.map(([a, st]) => h('li', null, h('a', { href: '#/scenarios/' + encodeURIComponent(s.id) + '?step=' + st.k + '&evidence=' + st.att.indexOf(a) }, icon('clip'), bdi(a.name || 'attachment')))))) : null,
      s.tags.length ? h('section', { class: 'card' }, h('h2', { class: 'card-t' }, 'Tags'), h('div', { class: 'tags' }, s.tags.map(tagEl))) : null);
    return view('v-sc', crumbs([['Dashboard', '#/'], ['Features', '#/features'], [s.f.name, '#/features/' + encodeURIComponent(s.f.id)], [s.name]]),
      head(s.name, null, nav, h('div', { class: 'pre-t' }, chip(s.status), h('span', { class: 'kwl' }, s.kw))),
      h('div', { class: 'meta' }, h('a', { href: '#/features/' + encodeURIComponent(s.f.id) }, icon('features'), bdi(s.f.name)), h('span', { class: 'mono' }, icon('file'), bdi(s.f.uri + (s.line ? ':' + s.line : ''))), h('span', null, icon('clock'), dur(s.dur)), h('span', null, icon('list'), plural(s.steps.length, 'step'))),
      s.desc ? h('p', { class: 'desc' }, bdi(s.desc)) : null,
      banners,
      h('div', { class: 'sc-g' }, h('section', { class: 'card steps-c', 'aria-label': 'Steps' }, steps), aside));
  };

  /* failures */
  // Error signature for grouping: first line with identifiers, quoted values and numbers masked.
  const sig = (e) => firstLine(e).replace(/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/gi, '‹id›').replace(/\b[0-9a-f]{12,}\b/gi, '‹id›')
    .replace(/\b[a-z]{2,4}_[A-Za-z0-9]{4,}\b/g, '‹id›').replace(/"[^"]*"/g, '"…"').replace(/(^|[^\w‹])-?\d+(?:[.,]\d+)*/g, '$1N').trim();
  const mini = (s, extra) => h('a', { class: 'mrow', href: '#/scenarios/' + encodeURIComponent(s.id) + (s.bad ? '?step=' + s.bad.k : '') }, glyph(s.status), h('span', { class: 'mrow-m' }, h('span', { class: 'mrow-t' }, bdi(s.name)), h('span', { class: 'mrow-s' }, bdi(s.f.name), s.bad ? [' · ', bdi((s.bad.keyword || '') + s.bad.name)] : null, extra ? [' · ', extra] : null)), h('span', { class: 'row-d' }, dur(s.dur)), icon('right', 'row-go'));
  const failuresView = () => {
    const fails = S.filter((s) => s.status === 'failed');
    const groups = new Map();
    for (const s of fails) { const k = s.analysis && s.analysis.fingerprint || sig(s.bad && s.bad.error || ''); const g = groups.get(k) || []; g.push(s); groups.set(k, g); }
    const blockers = S.filter((s) => s.status === 'ambiguous' || s.status === 'undefined');
    const body = [];
    body.push(h('h2', { class: 'sec-h' }, 'Unexpected failures', h('span', { class: 'cnt s-failed' }, num(fails.length))));
    if (!fails.length) body.push(card('', null, null, null, empty('check', 'No unexpected failures', T.sc.known ? 'Every failure in this run is covered by a known issue.' : 'No scenario failed in this run.', 'ok')));
    [...groups].sort((a, b) => b[1].length - a[1].length).forEach(([k, xs], gi) => {
      const fs = new Set(xs.map((s) => s.f.name));
      body.push(h('section', { class: 'card grp' }, h('header', { class: 'grp-h' }, h('span', { class: 'grp-n' }, '×' + xs.length), h('div', null, h('p', { class: 'grp-t mono' }, k || '(no error message)'), h('p', { class: 'card-s' }, plural(xs.length, 'scenario') + ' in ' + plural(fs.size, 'feature') + (xs.length > 1 ? ' share this error' : '')))),
        h('p', { class: 'muted group-reason' }, xs[0].analysis && xs[0].analysis.groupReason || 'First error line; identifiers, quoted values and numbers masked.'), h('div', { class: 'mrows' }, xs.map((s) => mini(s))), xs[0].bad && xs[0].bad.error ? errPanel(xs[0].bad.error, xs.length > 1 ? 'First occurrence' : 'Error') : null));
    });
    if (blockers.length) body.push(h('h2', { class: 'sec-h' }, 'Undefined and ambiguous steps', h('span', { class: 'cnt s-ambiguous' }, num(blockers.length))), card('', null, 'These scenarios stopped before their assertions ran.', null, h('div', { class: 'mrows' }, blockers.map((s) => mini(s, ST[s.status][0].toLowerCase())))));
    if (T.sc.known) body.push(h('a', { class: 'callout', href: '#/known' }, icon('flag'), h('span', null, plural(T.sc.known, 'known failure') + ' are listed under Known issues'), icon('arrow')));
    return view('', crumbs([['Dashboard', '#/'], ['Failures']]), head('Failures', plural(fails.length, 'unexpected failure') + ', grouped by error' + (groups.size ? ' into ' + plural(groups.size, 'group') : '') + (blockers.length ? ' · ' + num(blockers.length) + ' undefined or ambiguous' : '')), body);
  };

  const knownView = () => {
    const sec = (D.sections || []).find((s) => s.role === 'known');
    const refs = new Map();
    for (const a of KNOWN) { const s = SB[a.scenario]; if (!s) continue; const r = refs.get(a.ref) || { a, xs: [], fixed: new Set(), oos: new Set() }; if (!r.a.observed && a.observed) r.a = a; if (!r.xs.includes(s)) r.xs.push(s); if (a.state === 'passed') r.fixed.add(s); if (a.state === 'not-applicable') r.oos.add(s); refs.set(a.ref, r); }
    const body = [];
    if (sec && sec.summary) body.push(h('dl', { class: 'facts-row' }, sec.summary.map((f) => h('div', null, h('dt', null, f.label), h('dd', null, bdi(f.value))))));
    if (!refs.size) body.push(card('', null, null, null, empty('flag', 'No known issues', 'This run records no known failures.')));
    for (const [ref, r] of refs) {
      const fixed = r.fixed.size;
      body.push(h('section', { class: 'card grp' }, h('header', { class: 'grp-h' }, h('span', { class: 'ref big' }, ref || '—'), h('div', null, h('p', { class: 'grp-t' }, bdi(r.a.title || '')), r.a.note ? h('p', { class: 'card-s' }, bdi(r.a.note)) : null)),
        fixed ? h('p', { class: 'fixed' }, icon('check'), plural(fixed, 'scenario') + ' passed although the issue is open — consider closing it.') : null,
        r.oos.size ? h('p', { class: 'oos' }, plural(r.oos.size, 'scenario') + ' passed; the issue was not applicable in this run.') : null,
        h('div', { class: 'mrows' }, r.xs.map((s) => mini(s)))));
    }
    if (NOTAPPLIED.length) body.push(h('h2', { class: 'sec-h' }, 'Records not applied', h('span', { class: 'cnt s-failed' }, num(NOTAPPLIED.length))), h('p', { class: 'muted' }, 'These records cover no known failure; their scenarios keep their own result.'), NOTAPPLIED.map(notApplied));
    if (sec && sec.note) body.push(h('p', { class: 'muted' }, bdi(sec.note)));
    return view('', crumbs([['Dashboard', '#/'], ['Known issues']]), head('Known issues', plural(T.sc.known, 'known failure') + ' across ' + plural(refs.size, 'issue') + '. They are expected and kept apart from unexpected failures.'), body);
  };

  VIEWS.skipped = () => {
    const sk = S.filter((s) => s.status === 'skipped'), pe = S.filter((s) => s.status === 'pending');
    const why = (s, kind) => { const a = s.ann.find((x) => x.kind === kind); return a ? a.title : null; };
    const body = [
      h('h2', { class: 'sec-h' }, 'Skipped', h('span', { class: 'cnt s-skipped' }, num(sk.length))),
      card('', null, null, null, sk.length ? h('div', { class: 'mrows' }, sk.map((s) => mini(s, why(s, 'skip') || 'no reason recorded'))) : empty('check', 'Nothing was skipped', null)),
      h('h2', { class: 'sec-h' }, 'Pending', h('span', { class: 'cnt s-pending' }, num(pe.length))),
      card('', null, pe.length ? 'Steps marked as not yet implemented.' : null, null, pe.length ? h('div', { class: 'mrows' }, pe.map((s) => mini(s, s.bad && s.bad.error ? firstLine(s.bad.error) : null))) : empty('check', 'No pending steps', null)),
      h('h2', { class: 'sec-h' }, 'Not verified', h('span', { class: 'cnt s-undefined' }, num(LIMITS.length))),
      card('', null, LIMITS.length ? 'These scenarios ran, but part of what they claim cannot be observed in this environment.' : null, null, LIMITS.length ? h('div', { class: 'mrows' }, LIMITS.filter((a) => SB[a.scenario]).map((a) => mini(SB[a.scenario], a.title))) : empty('check', 'No observation limits recorded', null))];
    return view('', crumbs([['Dashboard', '#/'], ['Skipped']]), head('Skipped and not verified', plural(sk.length, 'skipped scenario') + ' · ' + num(pe.length) + ' pending · ' + plural(LIMITS.length, 'observation limit')), body);
  };

  const timelineView = (_, q) => {
    // Gantt: scenarios placed back to back in run order (Cucumber JSON carries durations, not start times).
    const total = Math.max(1, T.dur);
    const lanes = h('div', { class: 'gantt-l' });
    const g = sv('svg', { class: 'gantt', viewBox: '0 0 1000 ' + F.length * 10, preserveAspectRatio: 'none', role: 'group', 'aria-label': 'Run timeline' });
    css(g, '--lanes', F.length);
    let t = 0;
    F.forEach((f, li) => {
      lanes.append(h('a', { class: 'gl', href: '#/features/' + encodeURIComponent(f.id) }, glyph(f.status), bdi(f.name)));
      g.append(sv('rect', { class: 'lane' + (li % 2 ? ' odd' : ''), x: 0, y: li * 10, width: 1000, height: 10 }));
      for (const s of f.sc) {
        const a = sv('a', { href: '#/scenarios/' + encodeURIComponent(s.id), 'aria-label': s.name + ', ' + ST[s.status][0] + ', ' + dur(s.dur) },
          sv('rect', { class: 'gb f-' + s.status, x: (t / total * 1000).toFixed(2), y: li * 10 + 2, width: Math.max(1.2, s.dur / total * 1000).toFixed(2), height: 6 }));
        bindTip(a, () => [s.name, ST[s.status][0] + ' · ' + dur(s.dur), s.f.name]);
        g.append(a);
        t += s.dur;
      }
    });
    const axis = h('div', { class: 'gantt-x' });
    // Axis ticks: the smallest "nice" interval (1, 2 or 5 of a unit, or a clock unit) that needs at most
    // n intervals, n from the axis width (one label per 64 px, 2 to 10), never more than eleven labels.
    // Drawn once the axis is laid out and again on resize; if labels still touch, n shrinks.
    const NICE = [1e6, 2e6, 5e6, 1e7, 2e7, 5e7, 1e8, 2e8, 5e8, 1e9, 2e9, 5e9, 1e10, 2e10, 3e10, 6e10, 1.2e11, 3e11, 6e11, 9e11, 1.8e12, 3.6e12, 7.2e12, 1.08e13, 2.16e13, 4.32e13, 8.64e13];
    const ticks = (n) => {
      let stepNs = NICE.find((x) => total / x <= n);
      if (!stepNs) { const d = 8.64e13, k = Math.pow(10, Math.max(0, Math.floor(Math.log10(total / d / n)))); stepNs = d * k * [1, 2, 5, 10].find((m) => total / (d * k * m) <= n); }
      axis.replaceChildren();
      for (let i = 0; i * stepNs <= total && i <= 10; i++) { const x = i * stepNs; axis.append(css(h('span', null, x ? dur(x) : '0'), 'left', (x / total * 100) + '%')); }
    };
    const touching = () => { const b = [...axis.children].map((e) => e.getBoundingClientRect()); return b.some((r, i) => i && r.left < b[i - 1].right + 4); };
    const drawAxis = () => {
      if (!axis.isConnected) return;
      for (let n = Math.min(10, Math.max(2, Math.floor(axis.clientWidth / 64))); ; n--) { ticks(n); if (n <= 2 || !touching()) break; }
    };
    ticks(10);
    onMount(drawAxis);
    let axisT = 0;
    const axisResize = () => { clearTimeout(axisT); axisT = setTimeout(drawAxis, 120); };
    W.addEventListener('resize', axisResize);
    onCleanup(() => W.removeEventListener('resize', axisResize));
    // Step-definition profile.
    const defs = new Map();
    for (const st of STEPS) {
      const k = st.match || '(no definition) ' + st.name;
      const d = defs.get(k) || { k, text: (st.keyword || '').trim() + ' ' + st.name, n: 0, fail: 0, tot: 0, max: 0, ex: st };
      d.n++; if (st.status !== 'passed' && st.status !== 'skipped') d.fail++;
      if (st.duration != null) { d.tot += st.duration; if (st.duration > d.max) { d.max = st.duration; d.ex = st; } }
      defs.set(k, d);
    }
    const cols = [['text', 'Step definition'], ['n', 'Calls'], ['fail', 'Not passed'], ['tot', 'Total'], ['mean', 'Mean'], ['max', 'Max']];
    let sortK = q.get('by') || 'tot', asc = false, limit = 15;
    const moreRow = h('div', { class: 'tbl-more' });
    const fsearch = h('input', { class: 'input', type: 'search', placeholder: 'Filter step definitions…', 'aria-label': 'Filter step definitions' });
    const tbody = h('tbody');
    const thead = h('thead');
    const all = [...defs.values()].map((d) => Object.assign(d, { mean: d.n ? d.tot / d.n : 0 }));
    const maxTot = all.reduce((max, d) => Math.max(max, d.tot), 1);
    const draw = () => {
      const toks = tokens(fsearch.value);
      let rows = all.filter((d) => toks.every((t) => (d.text + ' ' + d.k).toLowerCase().includes(t)));
      rows.sort((a, b) => (sortK === 'text' ? a.text.localeCompare(b.text) : a[sortK] - b[sortK]) * (asc ? 1 : -1));
      thead.replaceChildren(h('tr', null, cols.map(([k, l]) => h('th', { scope: 'col', 'aria-sort': k === sortK ? (asc ? 'ascending' : 'descending') : null, class: k === 'text' ? '' : 'num' }, h('button', { type: 'button', class: 'th-b', onclick: () => { if (sortK === k) asc = !asc; else { sortK = k; asc = false; } draw(); } }, l, k === sortK ? icon(asc ? 'down' : 'down', asc ? 'up' : '') : null)))));
      const lim = toks.length ? 300 : limit;
      fill(moreRow, rows.length > lim && lim < 300 ? btn('Show all ' + num(rows.length) + ' step definitions', 'down', () => { limit = 300; draw(); }, 'sm ghost') : null);
      tbody.replaceChildren(...rows.slice(0, lim).map((d) => h('tr', null,
        h('td', null, h('a', { href: '#/scenarios/' + encodeURIComponent(d.ex.sc.id) + '?step=' + d.ex.k }, bdi(hl(d.text, toks))), h('div', { class: 'loc mono' }, bdi(d.k)),
          sv('svg', { class: 'sbar thin', viewBox: '0 0 100 10', preserveAspectRatio: 'none', 'aria-hidden': 'true' }, sv('rect', { class: 'f-accent', x: 0, y: 0, width: Math.max(0.5, d.tot / maxTot * 100), height: 10 }))),
        h('td', { class: 'num' }, num(d.n)), h('td', { class: 'num' + (d.fail ? ' bad' : '') }, num(d.fail)), h('td', { class: 'num' }, dur(d.tot)), h('td', { class: 'num' }, dur(Math.round(d.mean))), h('td', { class: 'num' }, dur(d.max)))));
      if (rows.length > 300 && lim === 300) tbody.append(h('tr', null, h('td', { colspan: 6, class: 'muted' }, 'Showing 300 of ' + rows.length + '. Filter to narrow the list.')));
    };
    fsearch.addEventListener('input', draw);
    draw();
    return view('', crumbs([['Dashboard', '#/'], ['Steps and timeline']]), head('Steps and timeline', plural(STEPS.length, 'step') + ' · ' + plural(defs.size, 'step definition') + ' · ' + dur(T.dur) + ' of test time'),
      card('c-gantt', 'Run timeline', 'Scenarios back to back in run order, one lane per feature. Cucumber JSON records durations, not start times, so parallel runs appear sequential.', null,
        h('div', { class: 'gantt-w' }, lanes, h('div', { class: 'gantt-p' }, g, axis))),
      card('', 'Step definitions', 'Where the time goes: calls, failures and duration per step definition', h('div', { class: 'tb-search' }, icon('search'), fsearch),
        h('div', { class: 'tbl-w' }, h('table', { class: 'tbl prof' }, thead, tbody)), moreRow));
  };

  const sectionView = (s) => h('section', { class: 'card sec-card t-' + (s.tone || 'info'), id: 'section-' + s.id },
    h('h2', { class: 'card-t' }, bdi(s.title)),
    (s.summary || []).length ? h('dl', { class: 'facts-row' }, s.summary.map((f) => h('div', null, h('dt', null, f.label), h('dd', null, bdi(f.value))))) : null,
    (s.rows || []).length ? h('div', { class: 'tbl-w' }, h('table', { class: 'tbl' }, h('tbody', null, s.rows.map((r) => h('tr', null, h('th', { scope: 'row' }, bdi(r.label)), h('td', null, bdi(r.value))))))) : null,
    s.note ? h('p', { class: 'muted' }, bdi(s.note)) : null);
  const runView = (_, q) => {
    const v = view('', crumbs([['Dashboard', '#/'], ['Run details']]), head('Run details', 'Facts recorded by the runner and sections supplied by the caller'),
      card('', 'Run facts', null, null, h('dl', { class: 'facts' }, (D.facts || []).map((f) => h('div', null, h('dt', null, f.label), h('dd', null, bdi(f.value)))))),
      (D.sections || []).map(sectionView),
      h('div', { class: 'pair' },
        card('', 'Status legend', 'Status is shown by shape and label, never by colour alone', null, h('ul', { class: 'legend-x' }, ORDER.map((k) => h('li', null, chip(k), h('span', null, ST[k][2]))))),
        card('', 'Keyboard shortcuts', null, null, keysList())),
      card('', 'About this report', null, null, h('p', null, 'One self-contained HTML file generated from Cucumber JSON. It makes no network requests. Preferences are stored only in this browser under ', h('code', null, KEY), '.'),
        h('p', { id: 'store-note', class: 'note', hidden: storeOK }, icon('alert'), 'This browser blocks local storage, so preference changes last until the page is closed.')));
    const sid = q.get('section');
    if (sid) requestAnimationFrame(() => { const el = doc.getElementById('section-' + sid); if (el) { el.scrollIntoView({ block: 'start' }); el.classList.add('flash'); } });
    return v;
  };
  const keysList = () => h('dl', { class: 'keys' }, [[kbd, 'Search everything'], ['/', 'Search everything'], ['[  ]', 'Previous or next scenario'], ['g d', 'Go to dashboard'], ['g f', 'Go to failures'], ['?', 'Show shortcuts'], ['Esc', 'Close a dialog']].map(([k, l]) => h('div', null, h('dt', null, k.split(' ').filter(Boolean).map((x) => h('kbd', null, x))), h('dd', null, l))));

  const printView = () => {
    buildPrint();
    onCleanup(()=>{if(!location.hash.startsWith('#/print')){printSelection=null;printed=false;printRoot.replaceChildren();}});
    // The preview shows the print document inside the view; printing moves it back to the body.
    const v = view('', crumbs([['Dashboard', '#/'], ['Print preview']]), head('Print preview', 'The printed report: cover summary, failures first, then every feature. Interactive controls are left out.', btn('Print / Save as PDF', 'print', () => W.print(), 'primary')));
    v.append(printRoot);
    return v;
  };
  VIEWS.missing = () => view('', crumbs([['Dashboard', '#/'], ['Not found']]), head('Not found'), card('', null, null, null, empty('search', 'This link does not match anything in this report', 'It may come from a different run.')), h('a', { class: 'btn', href: '#/' }, icon('left'), 'Back to the dashboard'));

  /* ---------- print document ---------- */
  const printRoot = h('div', { id: 'print-root', class: 'pr' });
  doc.body.append(printRoot);
  let printed = false;
  const cssStr = (s) => '"' + String(s).replace(/[\u0000-\u001f\u007f]/g, ' ').replace(/[\\"]/g, '\\$&') + '"';
  const buildPrint = () => {
    if (printed) return;
    printed = true;
    const plan=printPlan(S,printSelection),printScenarios=plan.items,selectedIDs=new Set(printScenarios.map((s)=>s.id));
    const printFeatures=plan.summary?[]:F.map((f)=>({...f,sc:f.sc.filter((s)=>selectedIDs.has(s.id))})).filter((f)=>f.sc.length);
    const showErrors=!printSelection||printSelection.errors,showAttachments=!printSelection||printSelection.attachments;
    try {
      const sheet = [...doc.styleSheets].find((x) => { try { return x.cssRules; } catch (e) { return false; } });
      if (sheet) sheet.insertRule('@page { @top-left { content: ' + cssStr(D.name || D.title || 'Test report') + '; } @top-right { content: ' + cssStr(verdict) + '; } }', sheet.cssRules.length);
    } catch (e) { /* margin boxes unsupported: browser defaults apply */ }
    const P1 = h('section', { class: 'pr-cover' },
      h('p', { class: 'pr-brand' }, bdi(D.title || 'Test report'), ' · Test execution report'),
      h('h1', null, bdi(D.name || '')),
      h('div', { class: 'pr-verdict t-' + tone }, h('span', { class: 'pr-vi' }, icon(tone === 'fail' ? 'x' : tone === 'warn' ? 'flag' : 'check')), h('div', null, h('b', null, verdict), h('p', null, (T.sc.known ? num(T.sc.passed + T.sc.known) + ' of ' + num(executed) + ' executed scenarios ended as expected (' + num(T.sc.passed) + ' passed, pass rate ' + fpct(passRate) + ')' : num(T.sc.passed) + ' of ' + num(executed) + ' executed scenarios passed (' + fpct(passRate) + ')') + ' · ' + plural(T.sc.failed, 'unexpected failure') + ' · ' + plural(T.sc.known, 'known failure') + ' · ' + num(blocking) + ' undefined or ambiguous · ' + num(T.sc.skipped) + ' skipped'))),
      h('dl', { class: 'pr-facts' }, (D.facts || []).map((f) => h('div', null, h('dt', null, f.label), h('dd', null, bdi(f.value))))),
      h('div', { class: 'pr-kpis' }, [['Features', num(F.length)], ['Scenarios', num(scenarioCount)], ['Steps', num(STEPS.length)], [HEAD.label, fpct(HEAD.value)], [D.wallClock ? 'Total elapsed time' : 'Summed step time', D.wallClock || dur(T.dur), D.wallClock ? 'Summed step time: ' + dur(T.dur) : '']].map(([l, v, sub]) => h('div', null, h('span', null, l), h('b', null, v), sub ? h('span', null, sub) : null))),
      h('div', { class: 'pr-charts' },
        h('div', { class: 'pr-donut' }, h('h2', null, 'Scenarios by status'), h('div', { class: 'pr-dw' }, donut(T.sc, 'scenario', new Set(), null, true), h('table', { class: 'pr-t' }, h('tbody', null, ORDER.filter((k) => T.sc[k]).map((k) => h('tr', null, h('td', null, h('span', { class: 'sw f-' + k }), ' ', glyphTxt(k), ' ', ST[k][0]), h('td', { class: 'num' }, num(T.sc[k])), h('td', { class: 'num' }, fpct(pct(T.sc[k], S.length))))))))),
        h('div', { class: 'pr-feat' }, h('h2', null, F.length>1000?'Results per feature (first 1,000; full totals above)':'Results per feature'), h('table', { class: 'pr-t' }, h('thead', null, h('tr', null, h('th', null, 'Feature'), h('th', null, 'Outcome'), h('th', { class: 'num' }, 'Passed'), h('th', { class: 'num' }, 'Time'))),
          h('tbody', null, F.slice(0,1000).map((f) => h('tr', null, h('td', null, glyphTxt(f.status), ' ', bdi(f.name)), h('td', { class: 'pr-bar' }, sbar(f.n, { cls: 'still' })), h('td', { class: 'num' }, num(f.n.passed) + '/' + num(f.sc.length)), h('td', { class: 'num' }, dur(f.dur)))))))));
    const notApplied=NOTAPPLIED.filter((a)=>selectedIDs.has(a.scenario)),outOfScope=OOS.filter((a)=>selectedIDs.has(a.scenario)),limits=LIMITS.filter((a)=>selectedIDs.has(a.scenario));
    const prob = printScenarios.filter((s) => s.status === 'failed' || s.status === 'ambiguous' || s.status === 'undefined');
    const known = printScenarios.filter((s) => s.status === 'known');
    const sk = printScenarios.filter((s) => s.status === 'skipped' || s.status === 'pending');
    const excerpt = (e, n) => { const l = e.split('\n'); return l.slice(0, n).join('\n') + (l.length > n ? '\n… ' + (l.length - n) + ' more lines in the interactive report' : ''); };
    const pcard = (s, note) => h('div', { 'data-scenario': s.id, class: 'pr-card s-' + s.status }, h('p', { class: 'pr-ct' }, h('b', null, glyphTxt(s.status), ' ', ST[s.status][0]), ' ', bdi(s.name), h('span', { class: 'pr-m' }, ' · ' + s.f.name + ' · ' + dur(s.dur))),
      note ? h('p', { class: 'pr-note' }, note) : null,
      s.bad ? h('p', { class: 'pr-step' }, bdi((s.bad.keyword || '') + s.bad.name)) : null,
      showErrors && s.bad && s.bad.error ? h('pre', { class: 'pr-pre err' }, excerpt(s.bad.error, 12)) : null);
    const P2 = plan.summary ? null : h('section', { class: 'pr-sec' }, h('h2', null, 'Failures and known issues'),
      h('div',{class:'pr-unexpected'},h('h3', null, 'Unexpected failures, undefined and ambiguous (' + prob.length + ')'), prob.length ? prob.map((s) => pcard(s)) : h('p', null, 'None.')),
      known.length ? h('h3', null, 'Known failures (' + known.length + ')') : null, known.length ? known.map((s) => { const a = s.ann.find((x) => x.kind === 'known' && x.state === 'applied'); return pcard(s, a ? (a.ref ? a.ref + ': ' : '') + (a.title || '') + (a.step ? ' (step ' + a.step + ')' : '') : null); }) : null,
      notApplied.length ? [h('h3', null, 'Known-issue records not applied (' + notApplied.length + ')'), notApplied.map((a) => pcard(SB[a.scenario], 'Known issue, not applied: ' + (a.ref ? a.ref + ': ' : '') + (a.title || '') + '. ' + (a.reason || '')))] : null,
      outOfScope.length ? [h('h3', null, 'Known issues not applicable in this run (' + outOfScope.length + ')'), outOfScope.map((a) => pcard(SB[a.scenario], 'Known issue ' + (a.ref ? a.ref + ' ' : '') + '· not applicable in this run' + (a.title ? ': ' + a.title : '')))] : null,
      sk.length+limits.length ? h('h3', null, 'Skipped, pending and not verified (' + (sk.length + limits.length) + ')') : null,
      !sk.length && !limits.length ? null : h('table', { class: 'pr-t full' }, h('tbody', null, sk.map((s) => h('tr', null, h('td', null, glyphTxt(s.status), ' ', ST[s.status][0]), h('td', null, bdi(s.f.name + ' / ' + s.name)), h('td', null, bdi((s.ann.find((a) => a.kind === 'skip') || {}).title || (showErrors && s.bad && s.bad.error ? firstLine(s.bad.error) : ''))))),
        limits.filter((a) => SB[a.scenario]).map((a) => h('tr', null, h('td', null, '◌ Not verified'), h('td', null, bdi(SB[a.scenario].f.name + ' / ' + SB[a.scenario].name)), h('td', null, bdi(a.title)))))),
      (D.sections || []).filter((s) => !s.role).map((s) => h('div', { class: 'pr-secbox' }, h('h3', null, bdi(s.title)), (s.summary || []).length ? h('p', null, s.summary.map((f) => f.label + ': ' + f.value).join(' · ')) : null,
        (s.rows || []).length ? h('table', { class: 'pr-t' }, h('tbody', null, s.rows.map((r) => h('tr', null, h('th', null, bdi(r.label)), h('td', null, bdi(r.value)))))) : null, s.note ? h('p', null, bdi(s.note)) : null)));
    const P3 = plan.summary ? null : h('section', { class: 'pr-sec' }, h('h2', null, 'Features and scenarios'), printFeatures.map((f) => h('div', { class: 'pr-f' },
      h('h3', null, glyphTxt(f.status), ' ', bdi(f.name), h('span', { class: 'pr-m' }, '  ' + f.uri + ' · ' + executionLabel(f.sc) + ' · ' + dur(f.dur))),
      f.desc ? h('p', { class: 'pr-note' }, bdi(f.desc)) : null,
      null,
      f.sc.map((s) => h('div', { 'data-scenario': s.id, class: 'pr-sc s-' + s.status + (s.steps.length <= 6 ? ' short' : '') }, h('p', { class: 'pr-ct' }, h('b', null, glyphTxt(s.status), ' ', ST[s.status][0]), ' ', h('span', { class: 'pr-kw' }, s.kw + ':'), ' ', bdi(s.name), h('span', { class: 'pr-m' }, ' · ' + dur(s.dur) + (s.tags.length ? ' · ' + s.tags.join(' ') : ''))),
        h('ol', { class: 'pr-steps' }, s.steps.map((st) => h('li', { class: 's-' + st.vs }, h('span', { class: 'pr-g' }, glyphTxt(st.vs)), h('span', { class: 'pr-kw' }, (st.keyword || '').trim()), ' ', bdi(st.name), h('span', { class: 'pr-m' }, st.duration != null ? '  ' + dur(st.duration) : '  ' + ST[st.vs][0].toLowerCase()),
          st.rows && st.rows.length ? h('table', { class: 'pr-dt' }, h('tbody', null, st.rows.map((r, i) => h('tr', null, r.map((c) => h(i ? 'td' : 'th', null, bdi(c))))))) : null,
          st.doc ? h('pre', { class: 'pr-pre' }, excerpt(st.doc.value, 20)) : null,
          showErrors && st.error && st.status !== 'passed' ? h('pre', { class: 'pr-pre err' }, excerpt(st.error, 8)) : null,
          showAttachments && (st.att || []).length ? h('p', { class: 'pr-att' }, 'Attachments: ' + st.att.map((a) => (a.name || 'attachment') + ' (' + (a.mime || '?') + ', ' + bytes(a.size || 0) + ')').join('; ')) : null))))))));
    // Keep each feature heading on the same page as its first scenario: wrap them when that scenario is short
    // enough to stay whole; a long first scenario may split under its heading instead.
    for (const fd of (P3 ? P3.querySelectorAll('.pr-f') : [])) {
      const first = fd.querySelector('.pr-sc');
      if (!first || !first.classList.contains('short')) continue;
      const keep = h('div', { class: 'pr-keep' });
      while (fd.firstChild !== first) keep.append(fd.firstChild);
      keep.append(first);
      fd.prepend(keep);
    }
    printRoot.replaceChildren(...[P1, plan.blocked ? h('p',{class:'pr-scope'},'Detailed printing was not prepared: '+plan.count+' scenarios exceed the 1,000-scenario confirmation threshold. Open Print / Save as PDF to choose a smaller scope or confirm the full report.') : null, P2, P3, h('p', { class: 'pr-end' }, 'End of report · generated from Cucumber JSON by cucumber-reporter')].filter(Boolean));
  };
  const glyphTxt = (k) => ({ passed: '✓', failed: '✕', known: '⚑', ambiguous: '≠', undefined: '?', pending: '…', skipped: '↷' }[k]);
  const doPrint = () => printDialog();
  W.addEventListener('beforeprint', () => { buildPrint(); doc.body.append(printRoot); });
  W.addEventListener('afterprint', () => { if (doc.body.classList.contains('show-print')) { render(); } else {printSelection=null;printed=false;printRoot.replaceChildren();} });

  /* ---------- search palette ---------- */
  const pal = h('dialog', { class: 'dlg palette', 'aria-label': 'Search' });
  const pin = h('input', { class: 'pal-in', type: 'text', role: 'combobox', 'aria-expanded': 'true', 'aria-controls': 'pal-l', 'aria-autocomplete': 'list', placeholder: 'Search features, scenarios, steps, tags and errors', autocomplete: 'off', spellcheck: 'false' });
  const plist = h('div', { class: 'pal-l', id: 'pal-l', role: 'listbox', 'aria-label': 'Results' });
  pal.append(h('div', { class: 'pal-h' }, icon('search'), pin, h('kbd', null, 'Esc')), plist, h('div', { class: 'pal-f' }, h('span', null, h('kbd', null, '↑'), h('kbd', null, '↓'), ' move'), h('span', null, h('kbd', null, '↵'), ' open'), h('span', null, h('kbd', null, 'Esc'), ' close')));
  doc.body.append(pal);
  let IDX = null, sel = 0, opts = [];
  const index = () => {
    if (IDX) return IDX;
    IDX = { f: F.map((f) => ({ t: f.name, s: f.uri, l: (f.name + ' ' + f.uri + ' ' + f.tags.join(' ')).toLowerCase(), href: '#/features/' + encodeURIComponent(f.id), st: f.status, ic: 'features' })),
      s: S.map((s) => ({ t: s.name, s: s.f.name, l: (s.name + ' ' + s.tags.join(' ')).toLowerCase(), href: '#/scenarios/' + encodeURIComponent(s.id), st: s.status, ic: 'list' })),
      st: STEPS.map((st) => ({ t: (st.keyword || '').trim() + ' ' + st.name, s: st.sc.name, l: (st.keyword + st.name).toLowerCase(), href: '#/scenarios/' + encodeURIComponent(st.sc.id) + '?step=' + st.k, st: st.vs, ic: 'right' })),
      tg: [...TAGS].map(([t, n]) => ({ t, s: plural(n, 'scenario'), l: t.toLowerCase(), href: '#/scenarios?tag=' + encodeURIComponent(t), ic: 'tag' })),
      er: STEPS.filter((st) => st.error && st.status !== 'passed').map((st) => ({ t: firstLine(st.error), s: st.sc.name, l: st.error.toLowerCase(), href: '#/scenarios/' + encodeURIComponent(st.sc.id) + '?step=' + st.k, st: st.vs, ic: 'alert' })) };
    return IDX;
  };
  const drawPal = () => {
    const toks = tokens(pin.value);
    const I = index();
    const groups = [];
    if (!toks.length) {
      groups.push(['Go to', NAV.filter(([k]) => P.nav[k]).map(([k, l, ic]) => ({ t: l, s: '', href: '#/' + k, ic }))]);
      const at = S.filter((s) => s.status === 'failed' || s.status === 'ambiguous' || s.status === 'undefined').slice(0, 5).map((s) => ({ t: s.name, s: s.f.name, href: '#/scenarios/' + encodeURIComponent(s.id), st: s.status, ic: 'list' }));
      if (at.length) groups.push(['Needs attention', at]);
    } else {
      const find = (arr, lim) => {
        const r = [];
        for (const x of arr) { if (!toks.every((t) => x.l.includes(t))) continue; const tl = x.t.toLowerCase(); r.push([x, (tl.startsWith(toks[0]) ? 0 : tl.includes(toks[0]) ? 1 : 2) * 1e4 + x.t.length]); }
        r.sort((a, b) => a[1] - b[1]);
        return [r.slice(0, lim).map((x) => x[0]), r.length];
      };
      for (const [k, l, lim] of [['f', 'Features', 5], ['s', 'Scenarios', 7], ['st', 'Steps', 7], ['tg', 'Tags', 5], ['er', 'Errors', 5]]) {
        const [xs, n] = find(I[k], lim);
        if (xs.length) groups.push([l + (n > xs.length ? ' · ' + num(n) + ' matches' : ''), xs.concat(k === 's' && n > xs.length ? [{ t: 'Show all ' + num(n) + ' matching scenarios', s: '', href: '#/scenarios?q=' + encodeURIComponent(pin.value.trim()), ic: 'arrow' }] : [])]);
      }
    }
    opts = [];
    plist.replaceChildren(...(groups.length ? groups.map(([l, xs]) => h('div', { role: 'group', 'aria-label': l }, h('div', { class: 'pal-g', 'aria-hidden': 'true' }, l), xs.map((x) => {
      const id = 'po' + opts.length;
      const o = h('a', { class: 'pal-o', role: 'option', id, href: x.href, 'aria-selected': 'false', tabindex: '-1', onclick: () => closeDlg(pal) }, x.st ? glyph(x.st) : h('span', { class: 'glyph' }, icon(x.ic)), h('span', { class: 'pal-m' }, h('span', { class: 'pal-t' }, bdi(hl(x.t, toks))), x.s ? h('span', { class: 'pal-s' }, bdi(x.s)) : null), icon('right', 'row-go'));
      o.addEventListener('pointermove', () => pick(opts.indexOf(o), true));
      opts.push(o);
      return o;
    }))) : [empty('search', 'No results for “' + pin.value.trim() + '”', 'Search matches names, step text, tags and error messages.')]));
    pick(0);
  };
  const pick = (i, noScroll) => {
    if (!opts.length) { pin.removeAttribute('aria-activedescendant'); return; }
    sel = (i + opts.length) % opts.length;
    opts.forEach((o, j) => o.setAttribute('aria-selected', String(j === sel)));
    pin.setAttribute('aria-activedescendant', opts[sel].id);
    if (!noScroll) opts[sel].scrollIntoView({ block: 'nearest' });
  };
  pin.addEventListener('input', drawPal);
  pin.addEventListener('keydown', (e) => {
    if (e.key === 'ArrowDown') { e.preventDefault(); pick(sel + 1); } else if (e.key === 'ArrowUp') { e.preventDefault(); pick(sel - 1); } else if (e.key === 'Enter' && opts[sel]) { e.preventDefault(); navigate(opts[sel].getAttribute('href')); closeDlg(pal); }
  });
  const openDlg = (d) => { if (d.open) return; d.showModal(); d.classList.add('in'); };
  const closeDlg = (d) => { if (!d.open) return; d.classList.remove('in'); d.close(); };
  for (const d of [pal]) d.addEventListener('click', (e) => { if (e.target === d) closeDlg(d); });
  const openPalette = (text) => { pin.value = text || ''; drawPal(); openDlg(pal); pin.focus(); };

  /* ---------- settings ---------- */
  const setD = h('dialog', { class: 'dlg sheet', 'aria-labelledby': 'set-t' });
  setD.addEventListener('click', (e) => { if (e.target === setD) closeDlg(setD); });
  const sw = (label, on, fn, dis) => {
    const id = 'sw' + Math.random().toString(36).slice(2, 7);
    return h('div', { class: 'sw-r' }, h('label', { for: id }, label), h('input', { type: 'checkbox', role: 'switch', class: 'switch', id, checked: on, disabled: dis, onchange: (e) => fn(e.target.checked) }));
  };
  const onScenario = () => { const { parts } = parse(); return parts[0] === 'scenarios' && parts.length > 1; };
  const buildSettings = () => {
    // A mounted scenario keeps its Collapse all, Problems only and attempt choice: only other views re-render.
    const rerender = () => { save(); if (!onScenario()) render(); };
    const reexpand = () => { save(); if (applyOpenAttachments) applyOpenAttachments(); else render(); };
    setD.replaceChildren(
      h('header', { class: 'sheet-h' }, h('h2', { id: 'set-t' }, 'Settings'), ibtn('Close settings', 'close', () => closeDlg(setD))),
      h('div', { class: 'sheet-b' },
        h('h3', null, 'Appearance'),
        h('div', { class: 'set-r' }, h('span', null, 'Theme'), seg('Theme', [['auto', 'Auto'], ['light', 'Light'], ['dark', 'Dark']], P.theme, (v) => { P.theme = v; applyPrefs(); syncTheme(); save(); })),
        h('div', { class: 'set-r' }, h('span', null, 'Density'), seg('Density', [['comfortable', 'Comfortable'], ['compact', 'Compact']], P.density, (v) => { P.density = v; applyPrefs(); rerender(); })),
        h('div', { class: 'set-r' }, h('span', null, 'Start page'), h('select', { class: 'input select', 'aria-label': 'Start page', onchange: (e) => { P.view = e.target.value; save(); } }, CHOICE.view.map((v) => h('option', { value: v, selected: v === P.view }, NAV.find((n) => n[0] === v)[1])))),
        h('div', { class: 'set-r' }, h('span', null, 'Open attachments'), seg('Open attachments', [['failed', 'Failed steps'], ['all', 'All'], ['none', 'None']], P.expand, (v) => { P.expand = v; reexpand(); })),
        h('h3', null, 'Dashboard'),
        WIDGETS.map(([k, l]) => sw(l, P.widgets[k], (v) => { P.widgets[k] = v; rerender(); })),
        h('h3', null, 'Navigation'),
        NAV.map(([k, l]) => sw(l, P.nav[k], (v) => { P.nav[k] = v; save(); buildSide(); markNav(); }, k === 'dashboard'))),
      h('footer', { class: 'sheet-f' }, h('p', { class: 'muted' }, storeOK ? 'Saved in this browser only.' : 'This browser blocks storage; changes last until the page closes.'),
        btn('Reset to report defaults', null, () => { const restore = retainFocus(setD); P = base(); try { W.localStorage.removeItem(KEY); } catch (e) { storeOK = false; } applyPrefs(); syncTheme(); buildSide(); if (applyOpenAttachments) applyOpenAttachments(); else render(); buildSettings(); restore(); say('Settings reset'); }, 'ghost')));
  };
  doc.body.append(setD);
  const openSettings = () => { buildSettings(); openDlg(setD); };

  const keysD = h('dialog', { class: 'dlg small', 'aria-label': 'Keyboard shortcuts' });
  keysD.addEventListener('click', (e) => { if (e.target === keysD) closeDlg(keysD); });
  doc.body.append(keysD);

  // Native modal focus containment can still hand Tab to browser chrome at a
  // boundary. Wrap explicitly, while leaving interior navigation to the browser.
  const dialogTabStops = (dialog) => [...dialog.querySelectorAll('a[href],button,input,select,textarea,summary,[tabindex],[contenteditable="true"]')].filter((el) => {
    if (el.matches(':disabled') || el.getAttribute('type') === 'hidden' || (el.hasAttribute('tabindex') && el.tabIndex < 0)) return false;
    for (let n = el; n && n !== dialog; n = n.parentElement) {
      if (n.hidden || n.inert || n.getAttribute('aria-hidden') === 'true') return false;
      const style = W.getComputedStyle(n);
      if (style.display === 'none' || style.visibility === 'hidden') return false;
      const p = n.parentElement;
      if (p && p.tagName === 'DETAILS' && !p.open && n !== p.querySelector(':scope > summary')) return false;
    }
    return true;
  }).sort((a,b) => (a.tabIndex > 0 ? a.tabIndex : Infinity) - (b.tabIndex > 0 ? b.tabIndex : Infinity));
  doc.addEventListener('keydown', (e) => {
    if (e.key !== 'Tab') return;
    const dialog = e.target.closest('dialog[open]');
    if (!dialog) return;
    const stops = dialogTabStops(dialog), first = stops[0], last = stops[stops.length - 1];
    if (!first) {
      e.preventDefault(); dialog.setAttribute('tabindex','-1'); dialog.focus(); return;
    }
    if (!stops.includes(doc.activeElement) || (e.shiftKey ? doc.activeElement === first : doc.activeElement === last)) {
      e.preventDefault(); (e.shiftKey ? last : first).focus();
    }
  });

  /* ---------- keyboard ---------- */
  let gPending = 0;
  doc.addEventListener('keydown', (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') { e.preventDefault(); pal.open ? closeDlg(pal) : openPalette(); return; }
    const t = e.target;
    if (e.metaKey || e.ctrlKey || e.altKey || /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName) || t.isContentEditable || doc.querySelector('dialog[open]')) return;
    if (gPending) { gPending = 0; const m = { d: '', f: 'failures', s: 'scenarios', k: 'known', t: 'timeline', r: 'run' }[e.key]; if (m != null) { e.preventDefault(); go(m); } return; }
    if (e.key === '/') { e.preventDefault(); openPalette(); } else if (e.key === '?') { keysD.replaceChildren(h('header', { class: 'sheet-h' }, h('h2', null, 'Keyboard shortcuts'), ibtn('Close', 'close', () => closeDlg(keysD))), h('div', { class: 'sheet-b' }, keysList())); openDlg(keysD); } else if (e.key === 'g') { gPending = 1; setTimeout(() => { gPending = 0; }, 1200); } else if (e.key === '[' || e.key === ']') {
      const a = $(e.key === '[' ? '.pn a:first-child' : '.pn a:last-child', main);
      if (a && a.getAttribute('href')) navigate(a.getAttribute('href'));
    }
  });
  W.addEventListener('keydown', (e) => { if (e.key === 'Escape' && root.classList.contains('side-open')) toggleSide(false); });

  /* ---------- investigation tools (0.3.0) ---------- */
  const downloadText = (name, text, type) => {
    const url = URL.createObjectURL(new Blob([text], { type: type || 'text/plain;charset=utf-8' }));
    const a = h('a', { href: url, download: name }); doc.body.append(a); a.click(); a.remove(); setTimeout(() => URL.revokeObjectURL(url), 2000);
  };
  const temporaryDialog = (title, content, cls) => {
    const previous = doc.activeElement;
    const d = h('dialog', { class: 'dlg investigation-dialog ' + (cls || ''), 'aria-label': title },
      h('header', { class: 'dialog-head' }, h('h2', null, title), btn('Close', 'close', () => d.close(), 'sm')), content);
    d.addEventListener('close', () => { d.remove(); if (previous && previous.isConnected) previous.focus(); });
    doc.body.append(d); d.showModal(); return d;
  };
  const foldContent = (title, content, open) => h('details', { class: 'detail-block', open: !!open }, h('summary', null, title), content);
  const annotationBanner = (as, s) => {
    const a = as.find((x) => x.observed) || as[0];
    const invalid = a.kind === 'known' && ['not-applied','invalid'].includes(a.state);
    const state = invalid ? 'Known issue, not applied' : a.kind === 'known' ? a.state === 'passed' ? 'Passed — review issue' : a.state === 'not-applicable' ? 'Not applicable in this run' : 'Known failure' : a.kind === 'skip' ? 'Skipped' : 'Not verified';
    const detailRows = [...(a.details || [])];
    if (a.owner) detailRows.push({ label: 'Owner', value: a.owner });
    if (a.expires) detailRows.push({ label: 'Expires', value: a.expires });
    if (a.reviewDate) detailRows.push({ label: D.analysis && D.analysis.recordedAt && D.analysis.recordedAt.slice(0,10) > a.reviewDate ? 'Review overdue' : 'Review date', value: a.reviewDate });
    const evidence = as.flatMap((x) => x.outcomes || []);
    return h('section', { class: 'banner clean-banner b-' + (invalid ? 'fail' : a.kind), 'aria-label': state },
      icon(invalid ? 'alert' : a.kind === 'known' ? 'flag' : a.kind === 'skip' ? 'skip' : 'eyeoff'),
      h('div', { class: 'banner-content' },
        h('div', { class: 'banner-header' }, h('span', { class: 'banner-t' }, state), a.ref ? h('span', { class: 'ref' }, bdi(a.ref)) : null,
          (a.step || a.backgroundStep) ? h('a', { class: 'banner-step', href: '#/scenarios/' + encodeURIComponent(s.id) + '?step=' + ((a.executionStep || a.step) - 1) }, a.backgroundStep ? 'Background step ' + a.backgroundStep : 'Step ' + a.step) : null),
        a.title ? h('p', { class: 'banner-summary' }, bdi(a.title)) : null,
        invalid ? h('p', { class: 'banner-w' }, bdi(a.reason || 'Record covers no failure')) : null,
        a.state === 'passed' ? h('p', { class: 'fixed' }, 'Passed in this run — the issue may be fixed') : null,
        detailRows.length ? h('dl', { class: 'banner-facts' }, detailRows.map((f) => h('div', null, h('dt', null, bdi(f.label)), h('dd', null, bdi(f.value))))) : null,
        evidence.length ? foldContent('Evidence · ' + evidence.length + ' observations', h('ul', { class: 'banner-evidence' }, evidence.map((o) => h('li', null,
          h('b', null, o.step ? 'Step ' + o.step + (o.step === a.step ? ' (declared)' : '') : 'Run observation'), ' · ', bdi(o.outcome || ''), o.text ? h('p', null, bdi(o.text)) : null))), false) : null,
        as.length > 1 ? foldContent('Additional records · ' + (as.length - 1), h('ul', { class: 'banner-evidence' }, as.filter((x) => x !== a).map((x) => h('li', null, bdi(x.title || ''), x.note ? h('p', null, bdi(x.note)) : null, (x.details || []).map((f) => h('p', null, bdi(f.label), ': ', bdi(f.value)))))), false) : null,
        a.note && a.note !== a.title ? foldContent('Notes and context', h('p', { class: 'banner-note' }, bdi(a.note)), a.note.length < 180) : null));
  };
  const errorViewer = (err, label, fullscreen) => {
    const lines = err.split('\n'); const pre = h('pre', { class: 'code err-c error-lines' + (lines.length > 12 ? ' clip' : '') });
    let wrap = true, numbered = false, limit=1000, more;
    pre.setExpanded = (open) => { pre.classList.toggle('clip', !open && lines.length > 12); if (more && lines.length <= 1000) { more.textContent = open ? 'Collapse error' : 'Expand error'; more.setAttribute('aria-expanded', String(open)); } };
    const search = h('input', { type: 'search', class: 'input', placeholder: 'Find in error…', 'aria-label': 'Find in error' });
    const matches = h('span', { class: 'muted', role: 'status' });
    const paint = () => {
      const q = search.value.toLowerCase(); let count = 0;
      pre.replaceChildren();let shown=0;
      for(let i=0;i<lines.length;i++){const line=lines[i],hit=!!q&&line.toLowerCase().includes(q);if(hit)count++;if(shown>=limit||(q&&!hit))continue;shown++;pre.append(h('span',{class:'error-line'+(hit?' matched-line':'')},numbered?h('span',{class:'line-number','aria-hidden':'true'},i+1):null,i===0?h('b',null,line):line,i<lines.length-1?'\n':''));}

      if(more && lines.length>1000){more.disabled=limit>=lines.length;if(more.disabled&&doc.activeElement===more)search.focus({preventScroll:true});}
      if (q) pre.setExpanded(true); matches.textContent = q ? count + ' matching lines (showing '+shown+')' : 'Showing '+shown+' of '+lines.length+' lines';
    };
    search.addEventListener('input', paint); paint();
    const tools = h('div', { class: 'error-tools' }, search, matches,
      btn('Wrap', 'list', (e) => { wrap = !wrap; pre.classList.toggle('nowrap', !wrap); e.currentTarget.setAttribute('aria-pressed', String(wrap)); }, 'sm ghost', { 'aria-pressed': 'true' }),
      btn('Line numbers', 'list', (e) => { numbered = !numbered; e.currentTarget.setAttribute('aria-pressed', String(numbered)); paint(); }, 'sm ghost', {'aria-pressed':'false'}),
      !fullscreen ? btn('Full screen', 'search', () => temporaryDialog('Error details', errorViewer(err, label, true), 'wide-dialog'), 'sm ghost') : null);
    more = lines.length>1000 ? btn('Show next 1,000 lines','down',()=>{limit+=1000;pre.setExpanded(true);paint();},'sm ghost more-b') : lines.length>12 ? btn('Expand error','down',()=>pre.setExpanded(pre.classList.contains('clip')),'sm ghost more-b',{'aria-expanded':'false'}):null;
    return h('div', { class: 'err' }, h('div', { class: 'err-h' }, icon('alert'), h('span', null, label || 'Error'), h('span', { class: 'grow-x' }), btn('Copy', 'copy', () => copy(err, 'Error'), 'sm ghost')),
      tools, pre, more);

  };
  const evidenceGallery = (current) => {
    const images = STEPS.flatMap((st) => (st.att || []).filter((a) => /^image\/(png|jpeg|gif|webp)$/i.test(a.mime || '') && typeof a.data === 'string').map((a) => ({ a, st })));
    if (!images.some((x) => x.a === current)) images.push({ a: current, st: { sc: { name: 'Execution attempt' }, k: 0 } });
    let index = images.findIndex((x) => x.a === current), zoom = 1;
    const stage = h('div', { class: 'image-stage' }), caption = h('p', { class: 'muted' });
    const draw = () => { const x = images[index]; if (!x) return; const img = h('img', { src: 'data:' + x.a.mime.toLowerCase() + ';base64,' + x.a.data, alt: x.a.name || 'Evidence' }); css(img, 'width', (zoom * 100) + '%'); stage.replaceChildren(img); caption.textContent = (index + 1) + ' / ' + images.length + ' · ' + x.st.sc.name + ' · Step ' + (x.st.k + 1) + ' · ' + (x.a.name || 'Image'); };
    temporaryDialog('Evidence viewer', h('div', null, h('div', { class: 'toolbar' }, btn('Previous', 'left', () => { index = (index + images.length - 1) % images.length; zoom = 1; draw(); }), btn('Next', 'right', () => { index = (index + 1) % images.length; zoom = 1; draw(); }), btn('Zoom in', 'search', () => { zoom = Math.min(4, zoom + .25); draw(); }), btn('Fit', 'image', () => { zoom = 1; draw(); })), caption, stage), 'wide-dialog'); draw();
  };
  const attachment = (a, open) => {
    const lazyID='lazy-'+Math.random().toString(36).slice(2);const placeholder=h('div',{id:lazyID,hidden:true,inert:true});
    const shell = h('div', { class: 'att-x lazy-attachment' }); let loaded = false;
    const load = () => { if (loaded) return; loaded = true; const node = attachmentBody(a, true);
      const img = node.querySelector('.att-img'); if (img) { const b = h('button', { type: 'button', class: 'image-open', 'aria-label': 'Open evidence viewer', onclick: () => evidenceGallery(a) }); img.replaceWith(b); b.append(img); }

      const oldButton=shell.querySelector('.att-h');if(oldButton){const newButton=node.querySelector('.att-h');oldButton.setAttribute('aria-expanded','true');oldButton.setAttribute('aria-controls',newButton.getAttribute('aria-controls'));newButton.replaceWith(oldButton);oldButton.addEventListener('click',()=>{const expanded=oldButton.getAttribute('aria-expanded')!=='true';oldButton.setAttribute('aria-expanded',String(expanded));const wrap=shell.querySelector('.acc');wrap.hidden=!expanded;wrap.inert=!expanded;});}
      shell.replaceChildren(...node.childNodes);
    };
    if (open) load(); else shell.append(h('button', { class: 'att-h', type: 'button', 'aria-expanded': 'false', 'aria-controls':lazyID, onclick: load }, icon('clip'), h('span', { class: 'att-n' }, bdi(a.name || 'attachment')), h('span', { class: 'att-k' }, (a.mime || '') + ' · ' + bytes(a.size || 0)), icon('down')),placeholder);
    return shell;
  };
  const attemptSteps = (scenario, attempt) => attempt.useScenarioSteps ? (scenario.steps || []) : (attempt.steps || []);
  // Runner timestamps are shown as recorded (time of day and zone, full value in datetime); never localised or estimated.
  const clockOf = (iso, digits) => {
    const m = /T(\d\d:\d\d:\d\d)(?:[.,](\d+))?(Z|[+-]\d\d:?\d\d)?$/i.exec(iso || '');
    if (!m) return null;
    // A fixed precision pads whole seconds too, so labels of one scenario line up.
    const frac = digits == null ? (m[2] || '') : (m[2] || '').slice(0, digits).padEnd(digits, '0');
    return m[1] + (frac ? '.' + frac : '') + (m[3] ? (/z/i.test(m[3]) ? ' UTC' : ' UTC' + m[3]) : '');
  };
  const isoMs = (iso) => Date.parse(String(iso).replace(/[.,](\d{1,3})\d*/, '.$1'));
  const subMsNs = (iso) => { const m = /[.,](\d+)/.exec(String(iso).split('T')[1] || ''); return m ? Number((m[1] + '000000000').slice(3, 9)) : 0; };
  const attemptTiming = (scenario, a) => {
    const started = clockOf(a.started), finished = clockOf(a.finished);
    if (started || finished) {
      const out = [];
      if (started) out.push(' · Started ', h('time', { datetime: a.started }, started));
      if (finished) out.push(' · Finished ', h('time', { datetime: a.finished }, finished));
      const ns = started && finished ? (isoMs(a.finished) - isoMs(a.started)) * 1e6 + subMsNs(a.finished) - subMsNs(a.started) : NaN;
      if (Number.isFinite(ns) && ns >= 0) out.push(' · ' + dur(ns) + ' elapsed');
      return out;
    }
    const timed = attemptSteps(scenario, a).filter((st) => typeof st.duration === 'number');
    return timed.length ? [' · Step time ' + dur(timed.reduce((n, st) => n + st.duration, 0)) + ' (no start or finish time recorded)'] : [];
  };
  // Evidence can belong to a hook or an earlier execution, not the failing step.
  const scenarioEvidence = (s, attempts) => {
    let count = 0, target = null, best = -1;
    const consider = (steps, attempt) => {
      for (let step = 0; step < steps.length; step++) for (let evidence = 0; evidence < (steps[step].att || []).length; evidence++) {
        const a = steps[step].att[evidence]; count++;
        const score = (/^image\//i.test(a.mime || '') ? 1000 : 0) + (steps[step] === s.bad ? 100 : 0) + (attempt == null ? 50 : 0);
        if (score > best) { best = score; target = { step, evidence, name: a.name || 'attachment', ...(attempt == null ? {} : { attempt }) }; }
      }
    };
    consider(s.steps, null);
    for (let i = attempts.length - 2; i >= 0; i--) if (!attempts[i].useScenarioSteps) consider(attemptSteps(s, attempts[i]), i);
    return { count, target };
  };
  const stepLabel = (st) => st.label || (st.ownIndex ? 'Step '+st.ownIndex : st.origin==='hook'?'Hook':st.origin==='background'?'Background step':'Step '+(st.k+1));
  const investigationText = (s) => [s.name, 'Feature: ' + s.f.uri + ':' + s.line, 'Outcome: ' + ST[s.status][0], ...((D.facts || []).map((f) => f.label + ': ' + f.value)), s.bad ? stepLabel(s.bad) + ': ' + (s.bad.keyword || '') + s.bad.name : '', s.bad && s.bad.error || ''].filter(Boolean).join('\n');
  const scenarioAddress = (id) => '#/scenarios/' + encodeURIComponent(id) + (CTX && CTX.href ? '?from=' + encodeURIComponent(CTX.href) : '');
  const restoreListContext = (from) => {
    if(!from || !/^#\/(scenarios|features)([/?]|$)/.test(from))return;
    const split=from.indexOf('?'), path=(split<0?from:from.slice(0,split)).slice(2).split('/'), q=new URLSearchParams(split<0?'':from.slice(split+1));
    if(path[0]==='scenarios'&&path.length>1)return;let featureID;try{featureID=decodeURIComponent(path[1]||'');}catch(e){return;}const feature=path[0]==='features'?FB[featureID]:null;
    let list;
    try{list=filteredScenarios(feature?feature.sc:S,{q:q.get('q')||'',status:new Set((q.get('status')||'').split(',').filter(Boolean)),feature:q.get('feature')||'',tag:q.get('tag')||'',dur:q.get('dur')||'',expression:q.get('tags')||'',issue:q.get('issue')||'',evidence:q.get('evidence')||'',change:q.get('change')||'',retry:q.get('retry')||'',sort:q.get('sort')||'order'});}catch(e){return;}
    if(q.get('layout')==='table')list=sortTableList(list,q.get('tableSort'),q.get('tableDir')!=='desc');
    CTX={ids:list.map((x)=>x.id),href:from,label:'filtered scenarios',scroll:CTX&&CTX.href===from?CTX.scroll:0,anchor:CTX&&CTX.href===from?CTX.anchor:null};rememberContext();
  };
  const scenarioView = (s, q) => {
    if(q.get('from'))restoreListContext(q.get('from'));else if(CTX&&!CTX.ids.length)restoreListContext(CTX.href);
    const node = baseScenarioView(s, q);
    const actions = h('div', { class: 'investigation-actions' },
      CTX && CTX.ids.includes(s.id) ? h('a', { class: 'btn', href: CTX.href || '#/scenarios' }, icon('left'), 'Back to ' + CTX.ids.length + ' ' + CTX.label) : null,
      btn('Copy investigation summary', 'copy', () => copy(investigationText(s), 'Investigation summary')),
      btn('Copy report address', 'copy', () => { copy(location.href, 'Report address'); if (location.protocol === 'file:') say('Local file address: recipients need their own copy of this report.'); }));
    node.querySelector('.page-h').after(actions);
    if(s.analysis){const a=s.analysis;const facts=[];if(a.change)facts.push('Change: '+a.change);if(a.retried)facts.push('Passed first attempt: '+(a.firstStatus==='passed'?'yes':'no')+' · '+a.attempts.length+' attempts');if(a.samples)facts.push(a.failures+' unexpected failures across '+a.samples+' observed runs in this environment and branch');if(a.durationDelta)facts.push('Duration change: '+(a.durationDelta>0?'+':'−')+dur(Math.abs(a.durationDelta)));if(facts.length)actions.after(h('p',{class:'scenario-analysis'},facts.join(' · ')));}
    const attempts = D.execution && D.execution[s.id] || [];
    const evidence = scenarioEvidence(s, attempts);
    const evidenceHref = evidence.target ? '#/scenarios/' + encodeURIComponent(s.id) + '?step=' + evidence.target.step + '&evidence=' + evidence.target.evidence + (evidence.target.attempt == null ? '' : '&attempt=' + evidence.target.attempt) : null;
    if (s.bad) { const summary = h('section', { class: 'failure-summary card s-' + s.status, 'aria-label': 'Problem summary' },
      h('h2', { class: 'card-t' }, stepLabel(s.bad) + ' · ' + ST[s.bad.vs][0]), h('p', null, bdi(s.bad.name)),
      s.bad.error ? h('p', { class: 'failure-excerpt mono' }, bdi(firstLine(s.bad.error))) : null,
      h('div', { class: 'investigation-actions' }, h('a', { class: 'btn sm', href: '#/scenarios/' + encodeURIComponent(s.id) + '?step=' + s.bad.k }, 'Jump to step'), evidence.count ? h('a', { class: 'btn sm', href: evidenceHref, title: 'Attachments from this scenario, including hooks and earlier attempts' }, 'View evidence (' + evidence.count + ')', h('span', {class:'evidence-target'}, ' · ' + evidence.target.name)) : null));
      node.querySelector('.sc-g').before(summary);
    }
    const stepList = node.querySelector('.steps');
    if (stepList) {
      for (const body of stepList.querySelectorAll('.step-b')) {
        const children = [...body.children].filter((x)=>!x.classList.contains('step-h'));
        if (children.length) body.append(foldContent('Step details', children, true));
      }
      const bulky = [...stepList.querySelectorAll('.tbl-w,.docs')];
      for (const el of bulky) { const d = foldContent(el.classList.contains('docs') ? 'Doc string' : 'Data table', [], false); el.replaceWith(d); d.append(el); }
      const setAll = (open) => {
        // Attachments can materialize lazy JSON trees. Load them before walking
        // details, so one action also reaches the newly created descendants.
        for (const b of node.querySelectorAll('.att-h[aria-expanded="' + !open + '"]')) b.click();
        for (let pass=0;pass<66;pass++){const closed=[...node.querySelectorAll('details')].filter(d=>d.open!==open);if(!closed.length)break;for(const d of closed){d.open=open;if(open)d.dispatchEvent(new Event('toggle'));}}
        for (const error of node.querySelectorAll('.error-lines')) error.setExpanded(open);
        updateBulk();
      };
      const expand = btn('Expand all', 'down', () => setAll(true), 'sm');
      const collapse = btn('Collapse all', 'up', () => setAll(false), 'sm');
      const updateBulk = () => {
        const states = [...node.querySelectorAll('details')].map((d) => d.open).concat([...node.querySelectorAll('.att-h')].map((b) => b.getAttribute('aria-expanded') === 'true'));
        expand.disabled = !states.includes(false) && !node.querySelector('.error-lines.clip');
        collapse.disabled = !states.includes(true);
        if(doc.activeElement===expand && expand.disabled && !collapse.disabled)collapse.focus();
        else if(doc.activeElement===collapse && collapse.disabled && !expand.disabled)expand.focus();
      };
      node.addEventListener('toggle', updateBulk, true);
      node.addEventListener('click', () => queueMicrotask(updateBulk));
      queueMicrotask(updateBulk);
      const controls = h('div', { class: 'step-controls', 'aria-label': 'Scenario details' }, seg('Steps shown', [['all','All steps'],['problems','Problems only']], 'all', (value) => { [...stepList.children].forEach((li,i) => { li.hidden = value === 'problems' && ['passed','skipped'].includes(s.steps[i].vs); }); }), expand, collapse);
      stepList.before(controls);
    }
    if (attempts.length > 1) {
      const body = h('div', { class: 'attempt-body' });
      const show = (i) => { const a = attempts[i]; fill(body, h('p', null, 'Attempt ' + (a.number + 1) + ': ' + (ST[a.status] ? ST[a.status][0] : a.status) + (a.worker ? ' · Worker ' + a.worker : ''), attemptTiming(s, a)), attemptSteps(s,a).map((st,j) => h('section', { class: 'detail-block', id: 'attempt-' + i + '-step-' + j }, h('b', null, bdi((st.keyword || '') + st.name)), st.error ? errPanel(st.error) : null, (st.att || []).map((x) => attachment(x, false))))); };
      const requested = q.get('attempt');
      const selected = requested !== null && /^\d+$/.test(requested) && Number(requested) < attempts.length ? Number(requested) : attempts.length - 1;
      const choose = seg('Attempt', attempts.map((a,i) => [String(i), 'Attempt ' + (a.number + 1) + ' · ' + a.status + (clockOf(a.started, 3) ? ' · ' + clockOf(a.started, 3) : '')]), String(selected), (v) => show(+v));
      [...choose.querySelectorAll('[role="radio"]')].forEach((button,i) => { button.dataset.attemptIndex = String(i); });
      const section = h('section', { class: 'card attempts' }, h('h2', { class: 'card-t' }, 'Execution attempts'), foldContent('Attempt details', [choose, body], true)); show(selected); node.append(section);
    }
    return node;
  };
  const tagPredicate = (expression) => {
    if (!expression.trim()) return () => true;
    const ts = expression.match(/\(|\)|\b(?:and|or|not)\b|[^\s()]+/g) || []; let i = 0;
    const atom = () => { if (ts[i] === 'not') { i++; const p = atom(); return (tags) => !p(tags); } if (ts[i] === '(') { i++; const p = or(); if (ts[i++] !== ')') throw new Error('Closing parenthesis required'); return p; } const tag = ts[i++]; if (!tag || ['and','or',')'].includes(tag)) throw new Error('Expected tag'); return (tags) => tags.includes(tag); };
    const and = () => { let p = atom(); while (ts[i] === 'and') { i++; const a = p, b = atom(); p = (tags) => a(tags) && b(tags); } return p; };
    const or = () => { let p = and(); while (ts[i] === 'or') { i++; const a = p, b = and(); p = (tags) => a(tags) || b(tags); } return p; };
    const result = or(); if (i !== ts.length) throw new Error('Use and, or, not and parentheses'); return result;
  };
  const rerunText = (list) => [...new Set(list.filter((s) => !(s.analysis && (s.analysis.runHook || s.analysis.rerunUnavailable)) && s.line > 0 && !/[\r\n]/.test(s.f.uri)).map((s) => s.f.uri + ':' + s.line))].join('\n') + '\n';
  const sortTableList = (list,key,ascending) => {
    if(!['status','name','feature','duration','tags','attachments'].includes(key))return list;
    const value=(s)=>key==='feature'?s.f.name:key==='duration'?s.dur:key==='attachments'?s.steps.reduce((n,x)=>n+(x.att||[]).length,0):key==='tags'?s.tags.join(' '):key==='status'?SEV.indexOf(s.status):s[key];
    return list.slice().sort((a,b)=>{const av=value(a),bv=value(b);return (typeof av==='number'?av-bv:String(av).localeCompare(String(bv)))*(ascending?1:-1);});
  };
  const hiddenColumns=new Set();
  const tableRows = (list, sortBy, sortKey, ascending) => {
    const columns = [['status','Status'],['name','Scenario'],['feature','Feature'],['duration','Duration'],['tags','Tags'],['attachments','Evidence']];
    const table = h('table', { class: 'tbl scenario-table' });
    const heads = h('tr', null, columns.map(([key,label]) => h('th', { scope: 'col', 'aria-sort':sortKey===key?(ascending?'ascending':'descending'):'none' }, h('button', { type: 'button', class: 'linkbtn', 'data-focus-key':'sort-'+key, onclick: () => sortBy(key) }, label))));
    const rows = list.map((s) => h('tr', null, h('td', null, chip(s.status)), h('td', null, h('a', { href: scenarioAddress(s.id) }, bdi(s.name))), h('td', null, bdi(s.f.name)), h('td', { class: 'num' }, dur(s.dur)), h('td', null, s.tags.join(' ')), h('td', { class: 'num' }, s.steps.reduce((n,st) => n + (st.att || []).length,0))));
    table.append(h('thead', null, heads), h('tbody', null, rows));
    const choose = h('details', { class: 'column-picker' }, h('summary', null, 'Columns'));
    columns.forEach(([key,label],i) => { if (key === 'name') return; const input = h('input', { type: 'checkbox', checked: !hiddenColumns.has(key), onchange: () => { if(input.checked)hiddenColumns.delete(key);else hiddenColumns.add(key);heads.children[i].hidden = !input.checked; rows.forEach((r) => { r.children[i].hidden = !input.checked; }); } }); heads.children[i].hidden=hiddenColumns.has(key);rows.forEach((r)=>r.children[i].hidden=hiddenColumns.has(key));choose.append(h('label', null, input, label)); });
    return h('div', null, choose, h('div', { class: 'tbl-w' }, table));
  };
  const attentionStrip = () => h('section', { class: 'attention-strip', 'aria-label': 'Run actions' },
    h('h2', { class: 'sr' }, 'Needs attention'),
    [[failedExecutionCount, 'unexpected failures', '#/failures'], [blocking, 'undefined / ambiguous', '#/scenarios?status=undefined,ambiguous'], [NOTAPPLIED.length, 'issue records not applied', '#/known'], [LIMITS.length, 'observation limits', '#/skipped']].filter(([n]) => n).map(([n,l,url]) => h('a', { class: 'attention-action', href: url }, h('b', null, n), h('span', null, l), h('span', { class: 'action-label' }, 'Review →'))),
    !failedExecutionCount && !blocking && !NOTAPPLIED.length && !LIMITS.length && !(D.analysis&&D.analysis.runnerFailed) ? h('p', null, icon('check'), 'No unexpected problems in this run') : null);
  const analysisPanel = () => {
    const a = D.analysis; if (!a) return null;
    const c = a.comparison;
    return h('section', { class: 'card analysis-panel' }, h('h2', { class: 'card-t' }, 'Run analysis'),
      h('p', null, 'Completeness: ' + a.completeness + ' · Verdict: ' + a.verdictSource + (a.expectedScenarios ? ' · Expected ' + a.expectedScenarios + ', observed ' + (a.observedScenarios || 0) + (a.runHooks ? ' · Run hooks: ' + a.runHooks : '') : '')),
      (a.missingShards || []).length ? h('p', { class: 's-failed' }, 'Missing shards: ', bdi(a.missingShards.join(', '))) : null,
      (a.issueRecords || []).some((x)=>x.consecutiveUnobserved>1) ? foldContent('Known issues repeatedly unobserved',h('ul',null,a.issueRecords.filter((x)=>x.consecutiveUnobserved>1).map((x)=>h('li',null,bdi(x.ref),': unobserved in '+x.consecutiveUnobserved+' consecutive scoped runs'))),false) : null,
      c ? [h('p', { class: 'muted' }, 'Baseline: ', bdi(c.baseline)), h('div', { class: 'comparison-strip' }, [['newFailures','New failures'],['fixed','Fixed'],['added','Added'],['newlySkipped','Newly skipped'],['slower','Slower']].map(([k,l]) => h('button', { class: 'btn', type: 'button', onclick: () => temporaryDialog(l, h('div', null, (c[k] || []).map((id) => SB[id] ? mini(SB[id]) : h('p', null, bdi(id))))) }, l + ': ' + (c[k] || []).length))), h('p', null, (c.missing || []).length + ' scenarios missing from this run (not assumed deleted)')] : null,
      (a.scenarios || []).some((x)=>x.rerunUnavailable) ? h('p',{class:'muted'},'Rerun exports omit redacted feature paths because they no longer identify source files.') : null,
      (a.gateFailures || []).length ? h('p', { class: 's-failed' }, 'Quality gate: ' + a.gateFailures.join('; ')) : null,
      (a.history || []).length ? foldContent('History · ' + a.history.length + ' previous runs', h('div', { class: 'tbl-w' }, h('table', { class: 'tbl' }, h('thead', null, h('tr', null, ['Run','Recorded','Passed','Failed','Summed duration'].map((x) => h('th', { scope:'col' },x)))), h('tbody', null, a.history.map((x) => h('tr', null, h('td',null,bdi(x.id)),h('td',null,x.recordedAt || 'Not supplied'),h('td',null,x.counts.passed || 0),h('td',null,(x.counts.failed || 0)+(x.counts.undefined || 0)+(x.counts.ambiguous || 0)),h('td',null,dur(x.duration))))))), false) : null);
  };
  VIEWS.dashboard = (...args) => {
    const node = dashboardView(...args), grid = node.querySelector('.dash');
    const attention = attentionStrip(); const hero = grid.querySelector('.hero'); if (hero) hero.after(attention); else grid.prepend(attention);
    const analysis = analysisPanel(); if (analysis) attention.after(analysis);
    const compact = h('input', { type: 'checkbox', 'aria-label': 'Compact dashboard' });
    try { compact.checked = localStorage.getItem('cucumber-reporter:compact') === 'true'; } catch(e) { /* optional */ }
    const apply = () => { grid.classList.toggle('compact-dashboard',compact.checked); try { localStorage.setItem('cucumber-reporter:compact',String(compact.checked)); } catch(e) { /* optional */ } }; compact.addEventListener('change',apply); apply();
    grid.prepend(h('div', { class: 'dashboard-controls' }, h('label',null,compact,' Compact overview'), foldContent('How are rates calculated?', h('p',null,'Pass rate = passed / (all scenarios − skipped). As expected = (passed + known failures) / the same denominator. Pending, undefined and ambiguous scenarios remain in the denominator. Counts in Failures include undefined and ambiguous scenarios.'),false)));
    return node;
  };
  VIEWS.failures = (p,q) => {
    const node = failuresView(p,q);
    for (const group of node.querySelectorAll('.grp')) { const header = group.querySelector('.grp-h'); const details = h('details',{open:true,class:'failure-group'}); const summary = h('summary',null); header.replaceWith(details); summary.append(header); details.append(summary); while (details.nextSibling) details.append(details.nextSibling); }
    const active = q && SB[q.get('scenario')];
    const split = q && q.get('layout') === 'split';
    const toggle = h('a',{class:'btn',href:split?'#/failures':'#/failures?layout=split'},split?'Full-width groups':'Open split investigation'); node.querySelector('.page-h').append(toggle);
    if (!split) return node;
    for (const a of node.querySelectorAll('.mrow')) { const id=(a.getAttribute('href')||'').split('/')[2].split('?')[0]; a.href='#/failures?layout=split&scenario='+id; }
    const selected=active||S.find((s)=>['failed','undefined','ambiguous'].includes(s.status));
    return view('failure-split',h('div',{class:'failure-master'},node),h('div',{class:'failure-detail'},selected?scenarioView(selected,new URLSearchParams()):empty('check','No failures to inspect',null)));
  };
  VIEWS.known = (...args) => {
    const node = knownView(...args); const filter=h('select',{class:'input','aria-label':'Known issue state'},[['','All states'],['applied','Observed known failure'],['passed','Passed — review issue'],['not-applicable','Not applicable'],['not-applied','Record not applied']].map(([v,l])=>h('option',{value:v},l)));
    filter.addEventListener('change',()=>{for(const group of node.querySelectorAll('.grp')){const ref=group.querySelector('.ref');const anns=(D.annotations||[]).filter((a)=>ref&&a.ref===ref.textContent);group.hidden=!!filter.value&&!anns.some((a)=>a.state===filter.value);}for(const row of node.querySelectorAll('.na-c'))row.hidden=!!filter.value&&filter.value!=='not-applied';});node.querySelector('.page-h').append(filter);return node;
  };

  VIEWS.run=(...args)=>{const node=runView(...args);node.append(h('section',{class:'card'},h('h2',{class:'card-t'},'Verdict provenance'),h('p',null,D.verdictSource==='caller'?'The headline was supplied by the caller. Scenario outcomes are computed from test results and known-issue rules.':'The headline was computed from test results and known-issue rules.')));const a=analysisPanel();if(a)node.append(a);return node;};

  VIEWS.timeline=(...args)=>{
    const node=timelineView(...args);const a=D.analysis;if(!a)return node;
    const intervals=[...(a.scenarios||[]).filter((s)=>s.started&&s.finished).map((s)=>({name:s.name,worker:s.worker||'Unspecified worker',started:s.started,finished:s.finished,id:s.id})),...(a.phases||[])];if(!intervals.length)return node;
    const {start,end}=intervalBounds(intervals),span=Math.max(1,end-start);
    const workers = new Map();
    for (const x of intervals) { const worker = x.worker || 'Phase'; if (!workers.has(worker)) workers.set(worker, []); workers.get(worker).push(x); }
    const utilization = h('table', { class: 'tbl' }, h('thead', null, h('tr', null, ['Worker', 'Measured activity', 'Unoccupied time in measured span'].map((label) => h('th', { scope: 'col' }, label)))));
    const workerRows = h('tbody');
    for (const [worker, items] of workers) {
      let busy = 0, until = start;
      for (const x of items.slice().sort((a,b) => Date.parse(a.started)-Date.parse(b.started))) { const begin = Date.parse(x.started), finish = Date.parse(x.finished); busy += Math.max(0, finish - Math.max(until, begin)); until = Math.max(until, finish); }
      workerRows.append(h('tr', null, h('th', { scope: 'row' }, bdi(worker)), h('td', null, dur(busy * 1e6)), h('td', null, dur(Math.max(0, end-start-busy) * 1e6))));
    }
    utilization.append(workerRows);
    const lanes=h('div',{class:'measured-lanes'});for(const x of intervals){const bar=h('span',{class:'measured-bar'});css(bar,'margin-left',((Date.parse(x.started)-start)/span*100)+'%');css(bar,'width',Math.max(.2,(Date.parse(x.finished)-Date.parse(x.started))/span*100)+'%');lanes.append(h('div',{class:'measured-row'},h('span',null,bdi(x.worker||'Phase'),': ',x.id?h('a',{href:'#/scenarios/'+encodeURIComponent(x.id)},bdi(x.name)):bdi(x.name)),h('div',{class:'measured-track'},bar),h('span',{class:'mono'},dur((Date.parse(x.finished)-Date.parse(x.started))*1e6))));}
    node.querySelector('.page-h').after(h('section',{class:'card'},h('h2',{class:'card-t'},'Measured execution intervals'),h('p',{class:'muted'},'Real timestamps; overlapping bars represent concurrent execution. '+new Date(start).toISOString()+' → '+new Date(end).toISOString()),h('p',{class:'muted'},'Unoccupied time is the gap between supplied intervals within this measured span; activity outside these intervals is unknown.'),h('div',{class:'tbl-w'},utilization),lanes));return node;
  };
  const intervalBounds = (intervals) => {
    let start=Infinity,end=-Infinity;
    for(const x of intervals){start=Math.min(start,Date.parse(x.started));end=Math.max(end,Date.parse(x.finished));}
    return {start,end};
  };
  const printPlan = (scenarios, selection) => {
    const scope=selection?selection.scope:'full';
    const ids=new Set(selection&&selection.ids||[]);
    const items=scope==='summary'?[]:scenarios.filter((s)=>scope==='failures'?['failed','undefined','ambiguous'].includes(s.status):scope==='filtered'?ids.has(s.id):true);
    const blocked=items.length>1000 && !(selection&&selection.confirmed);
    return {scope,blocked,count:items.length,items:blocked?[]:items,summary:scope==='summary'||blocked};
  };
  let printSelection = null;
  const printDialog = () => {
    const select=h('select',{class:'input','aria-label':'Print scope'},[['full','Full report'],['summary','Executive summary'],['failures','Failures only'],['filtered','Current filtered scenarios']].map(([v,l])=>h('option',{value:v},l)));
    const errors=h('input',{type:'checkbox',checked:true}), attachments=h('input',{type:'checkbox',checked:true});
    const count=h('p',{role:'status'});const update=()=>{const n=select.value==='filtered'?(CTX?CTX.ids.length:0):select.value==='failures'?S.filter((s)=>['failed','undefined','ambiguous'].includes(s.status)).length:S.length;count.textContent=select.value==='summary'?'Run overview; scenario details excluded.':n+' scenarios selected. No per-feature truncation.';};select.addEventListener('change',update);update();
    const quality = h('select', { class: 'input', 'aria-label': 'Saved PDF compression quality' }, [['balanced','Balanced · 150 dpi images'],['small','Small file · 96 dpi images'],['high','High quality · original image resolution']].map(([v,l]) => h('option', { value:v },l)));
    const command = h('code');
    const updateCommand = () => { command.textContent = 'cucumber-report compress-pdf -quality ' + quality.value + ' -o report-shared.pdf report.pdf'; };
    quality.addEventListener('change',updateCommand); updateCommand();
    const compression = foldContent('Compress a saved PDF for sharing', [h('p',null,'Save the PDF first, then run this local command. It keeps all saved pages and reports the size reduction. Ghostscript is required. These presets apply to the saved file, not the browser print dialog.'), h('label',null,'Compression quality ',quality),h('pre',{class:'code'},command),btn('Copy compression command','copy',()=>copy(command.textContent,'Compression command'),'sm')],false);
    const d=temporaryDialog('Print or save as PDF',h('div',{class:'print-options'},select,count,h('label',null,errors,' Include error detail'),h('label',null,attachments,' Include attachment metadata'),compression,btn('Preview','print',()=>{const selection={scope:select.value,ids:CTX?CTX.ids.slice():[],errors:errors.checked,attachments:attachments.checked};const plan=printPlan(S,selection);if(plan.blocked&&!W.confirm('Prepare '+plan.count+' scenarios for printing? This may take substantial time and memory. Choose a smaller scope to reduce it.'))return;selection.confirmed=true;printSelection=selection;printed=false;d.close();navigate('#/print');applyPrintSelection();},'primary')));
  };
  const applyPrintSelection = () => {
    if(!printSelection)return;buildPrint();for(const old of printRoot.querySelectorAll('.pr-scope'))old.remove();
    const plan=printPlan(S,printSelection);
    printRoot.prepend(h('p',{class:'pr-scope'},'Print scope: '+plan.scope+' · '+plan.count+' selected scenarios. Run-level totals on the cover describe the complete input run.'));

  };
VIEWS.print=(...args)=>{const node=printView(...args);applyPrintSelection();return node;};
  W.addEventListener('beforeprint',applyPrintSelection);

  render();
})();
