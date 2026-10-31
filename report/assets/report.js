// cucumber-reporter page script: never writes markup, only classes, attributes and text.
(function () {
  'use strict';
  var KEY = 'cucumber-reporter:v1:prefs';
  var d = document, body = d.body, root = d.documentElement, ds = body.dataset;
  var CHOICES = { theme: ['auto', 'light', 'dark'], density: ['comfortable', 'compact'], expand: ['failed', 'none', 'all'], pageSize: ['25', '50', '100', '0'] };
  function all(sel, el) { return Array.prototype.slice.call((el || d).querySelectorAll(sel)); }
  function byId(id) { return d.getElementById(id); }
  function obj(x) { return !!x && typeof x === 'object' && !Array.isArray(x); }
  function words(s) { return (s || '').split(' ').filter(Boolean); }

  var defaults = { theme: ds.theme, density: ds.density, expand: ds.expand, pageSize: ds.pageSize, hidden: {} };
  words(ds.hiddenCharts).forEach(function (id) { defaults.hidden['chart:' + id] = true; });
  words(ds.hiddenSections).forEach(function (id) { defaults.hidden['section:' + id] = true; });

  // Storage access is guarded; blocked storage means report defaults.
  var storageOK = true;
  function storage() { try { return window.localStorage; } catch (e) { return null; } }
  function load() {
    var raw = null, s = storage();
    try { raw = s ? s.getItem(KEY) : null; if (!s) storageOK = false; } catch (e) { storageOK = false; }
    var p = null;
    try { p = raw ? JSON.parse(raw) : null; } catch (e) { p = null; }
    if (!obj(p)) p = {};
    if (!obj(p.show)) p.show = {};
    return p;
  }
  function save() {
    var s = storage();
    try { if (!s) throw 0; s.setItem(KEY, JSON.stringify(prefs)); } catch (e) { storageOK = false; }
    byId('storage-note').hidden = storageOK;
  }
  function clear() {
    var s = storage();
    try { if (s) s.removeItem(KEY); } catch (e) { storageOK = false; }
  }
  var prefs = load();

  // Valid saved choice, else the report default.
  function pref(name) {
    var v = prefs[name];
    return CHOICES[name].indexOf(String(v)) >= 0 ? String(v) : defaults[name];
  }
  function shown(key) {
    return typeof prefs.show[key] === 'boolean' ? prefs.show[key] : !defaults.hidden[key];
  }

  function applyExpand() {
    var e = pref('expand');
    all('details.scenario').forEach(function (el) {
      el.open = e === 'all' || (e === 'failed' && el.dataset.status === 'failed');
    });
  }
  function apply() {
    root.dataset.theme = pref('theme');
    body.classList.toggle('compact', pref('density') === 'compact');
    all('[data-chart]').forEach(function (el) { el.hidden = !shown('chart:' + el.dataset.chart); });
    all('[data-section]').forEach(function (el) { el.hidden = !shown('section:' + el.dataset.section); });
    all('[data-toggle]').forEach(function (el) { el.checked = shown(el.dataset.toggle); });
    all('[data-pref]').forEach(function (el) { el.value = pref(el.dataset.pref); });
    all('[data-pager]').forEach(page);
    byId('storage-note').hidden = storageOK;
  }

  // Paging.
  function page(box) {
    var size = +pref('pageSize'), limit = box.limit || size, n = 0, rest = 0;
    all(':scope > .pg-item', box).forEach(function (el) {
      var ok = !el.classList.contains('filtered');
      if (ok) n++;
      el.hidden = !ok || (size > 0 && n > limit);
      if (ok && el.hidden) rest++;
    });
    var more = box.moreButton;
    if (!more) {
      more = box.moreButton = d.createElement('button');
      more.type = 'button';
      more.className = 'more';
      more.addEventListener('click', function () { box.limit = (box.limit || size) + size; page(box); });
      (box.tagName === 'TBODY' ? box.closest('.scroll') : box).after(more);
    }
    more.hidden = rest === 0;
    more.textContent = 'Show ' + Math.min(rest, size) + ' more of ' + rest;
  }

  // Filter table rows and feature sections.
  var search = byId('search'), status = byId('status-filter'), tbody = d.querySelector('.ftable tbody');
  function filter() {
    var q = search.value.trim().toLowerCase(), st = status.value, n = 0;
    var rows = all('tr.pg-item', tbody);
    rows.forEach(function (row) {
      var ok = (!st || row.dataset.status === st) && (!q || row.dataset.search.indexOf(q) >= 0);
      row.classList.toggle('filtered', !ok);
      byId(row.dataset.target).hidden = !ok;
      if (ok) n++;
    });
    byId('match-count').textContent = n + ' of ' + rows.length + ' features';
    tbody.limit = 0;
    page(tbody);
  }
  search.addEventListener('input', filter);
  status.addEventListener('change', filter);

  // Sorting.
  all('.ftable th[data-sort]').forEach(function (th, col) {
    var b = d.createElement('button');
    b.type = 'button';
    b.textContent = th.textContent;
    th.textContent = '';
    th.appendChild(b);
    b.addEventListener('click', function () {
      var asc = th.getAttribute('aria-sort') !== 'ascending', num = th.dataset.sort === 'num';
      all('.ftable th').forEach(function (h) { h.removeAttribute('aria-sort'); });
      th.setAttribute('aria-sort', asc ? 'ascending' : 'descending');
      var rows = all('tr.pg-item', tbody);
      rows.sort(function (a, z) {
        var x = a.cells[col], y = z.cells[col];
        var r = num ? x.dataset.v - y.dataset.v : x.textContent.localeCompare(y.textContent);
        return asc ? r : -r;
      });
      rows.forEach(function (row) { tbody.appendChild(row); });
      page(tbody);
    });
  });

  // Settings.
  var settings = byId('settings'), help = byId('help');
  all('[data-toggle]').forEach(function (el) {
    el.addEventListener('change', function () { prefs.show[el.dataset.toggle] = el.checked; save(); apply(); });
  });
  all('[data-pref]').forEach(function (el) {
    el.addEventListener('change', function () {
      prefs[el.dataset.pref] = el.value;
      save();
      if (el.dataset.pref === 'pageSize') all('[data-pager]').forEach(function (b) { b.limit = 0; });
      apply();
      if (el.dataset.pref === 'expand') applyExpand();
    });
  });
  byId('reset-prefs').addEventListener('click', function () { prefs = { show: {} }; clear(); apply(); applyExpand(); });
  byId('open-settings').addEventListener('click', function () { settings.showModal(); });
  byId('open-help').addEventListener('click', function () { help.showModal(); });
  all('[data-close]').forEach(function (b) { b.addEventListener('click', function () { b.closest('dialog').close(); }); });

  // Copy, or select when the clipboard is unavailable.
  all('button.copy').forEach(function (b) {
    b.addEventListener('click', function () {
      var pre = byId(b.dataset.copy);
      function select() {
        var r = d.createRange();
        r.selectNodeContents(pre);
        var sel = window.getSelection();
        sel.removeAllRanges();
        sel.addRange(r);
        b.textContent = 'Selected';
      }
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(pre.textContent).then(function () { b.textContent = 'Copied'; }, select);
      } else {
        select();
      }
    });
  });

  // Shortcuts: / search, f failed only, ? help.
  d.addEventListener('keydown', function (e) {
    var t = e.target.tagName;
    if (e.ctrlKey || e.metaKey || e.altKey || t === 'INPUT' || t === 'SELECT' || t === 'TEXTAREA' || d.querySelector('dialog[open]')) return;
    if (e.key === '/') { e.preventDefault(); search.focus(); }
    else if (e.key === 'f') { status.value = status.value === 'failed' ? '' : 'failed'; filter(); }
    else if (e.key === '?') { help.showModal(); }
  });

  // Show a link target that paging hid.
  function reveal() {
    var t = byId(decodeURIComponent(location.hash.slice(1))), item = t && t.closest('.pg-item'), box = item && item.parentNode;
    if (!item) return;
    if (item.hidden && !item.classList.contains('filtered')) {
      box.limit = all(':scope > .pg-item:not(.filtered)', box).indexOf(item) + 1;
      page(box);
    }
    if (t.tagName === 'DETAILS') t.open = true;
    t.scrollIntoView();
  }
  window.addEventListener('hashchange', reveal);

  all('.js').forEach(function (el) { el.hidden = false; });
  apply();
  if (pref('expand') !== ds.expand) applyExpand();
  filter();
  if (location.hash) reveal();
})();
