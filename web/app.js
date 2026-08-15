/*
 * Togather dashboard — frontend.
 *
 * Talks only to this app's own BFF (same origin). It never holds a SEL
 * credential and never calls a SEL node directly for admin data.
 *
 * Every panel renders independently: an unavailable admin panel must not
 * prevent the public coverage panel from working.
 */

const $ = (id) => document.getElementById(id);

async function getPanel(path) {
  const res = await fetch(path, { headers: { Accept: 'application/json' } });
  // The BFF answers 200 with ok:false for expected unavailability, and a 5xx
  // only for genuine faults. Both carry the same envelope.
  try {
    return await res.json();
  } catch {
    return { ok: false, error: `The dashboard service returned ${res.status}.` };
  }
}

/* ---------- shared rendering ---------- */

function unavailable(payload) {
  const wrap = document.createElement('div');
  wrap.className = 'unavailable';

  const p = document.createElement('p');
  p.className = 'unavailable-msg';
  p.textContent = payload.error || 'Unavailable.';
  wrap.append(p);

  if (payload.reason === 'blocked' && payload.upstream) {
    const a = document.createElement('a');
    a.href = payload.upstream;
    a.rel = 'noopener';
    a.className = 'unavailable-link';
    a.textContent = 'Tracked upstream →';
    wrap.append(a);
  } else if (payload.upstream) {
    const d = document.createElement('details');
    const s = document.createElement('summary');
    s.textContent = 'Details';
    const pre = document.createElement('pre');
    pre.textContent = payload.upstream;
    d.append(s, pre);
    wrap.append(d);
  }
  return wrap;
}

function statTile(label, value, tone) {
  const d = document.createElement('div');
  d.className = 'stat' + (tone ? ` stat-${tone}` : '');
  const v = document.createElement('span');
  v.className = 'stat-value';
  v.textContent = value;
  const l = document.createElement('span');
  l.className = 'stat-label';
  l.textContent = label;
  d.append(v, l);
  return d;
}

/*
 * Inline SVG bar series. No charting dependency: the only chart this MVP needs
 * is "count per day, including the zeros", and the zeros are the whole point —
 * so the bars are drawn from a dense series rather than a sparse one.
 */
function barSeries(days, { height = 90 } = {}) {
  const max = Math.max(1, ...days.map((d) => d.count));
  const w = 100 / days.length;

  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  svg.setAttribute('viewBox', `0 0 100 ${height}`);
  svg.setAttribute('preserveAspectRatio', 'none');
  svg.setAttribute('class', 'bars');
  svg.setAttribute('role', 'img');
  const quiet = days.filter((d) => d.count === 0).length;
  svg.setAttribute('aria-label',
    `Events per day over ${days.length} days. Peak ${max}. ${quiet} days with no events.`);

  days.forEach((d, i) => {
    const h = d.count === 0 ? 1.5 : Math.max(2, (d.count / max) * (height - 4));
    const r = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
    r.setAttribute('x', (i * w + w * 0.14).toFixed(3));
    r.setAttribute('y', (height - h).toFixed(2));
    r.setAttribute('width', (w * 0.72).toFixed(3));
    r.setAttribute('height', h.toFixed(2));
    r.setAttribute('class', d.count === 0 ? 'bar bar-zero' : 'bar');
    const t = document.createElementNS('http://www.w3.org/2000/svg', 'title');
    t.textContent = `${d.date}: ${d.count} event${d.count === 1 ? '' : 's'}`;
    r.append(t);
    svg.append(r);
  });

  return svg;
}

function table(headers, rows) {
  const t = document.createElement('table');
  const thead = document.createElement('thead');
  const htr = document.createElement('tr');
  headers.forEach((h) => {
    const th = document.createElement('th');
    th.textContent = h;
    htr.append(th);
  });
  thead.append(htr);
  const tb = document.createElement('tbody');
  rows.forEach((cells) => {
    const tr = document.createElement('tr');
    cells.forEach((c) => {
      const td = document.createElement('td');
      if (c instanceof Node) td.append(c);
      else td.textContent = c;
      tr.append(td);
    });
    tb.append(tr);
  });
  t.append(thead, tb);
  const scroller = document.createElement('div');
  scroller.className = 'table-scroll';
  scroller.append(t);
  return scroller;
}

/* ---------- coverage ---------- */

async function loadCoverage() {
  const body = $('coverage-body');
  body.replaceChildren(Object.assign(document.createElement('p'),
    { className: 'loading', textContent: 'Loading…' }));

  const days = $('horizon').value;
  const payload = await getPanel(`/api/coverage?days=${encodeURIComponent(days)}`);
  if (!payload.ok) {
    body.replaceChildren(unavailable(payload));
    return;
  }

  const d = payload.data;
  const frag = document.createDocumentFragment();

  const stats = document.createElement('div');
  stats.className = 'stats';
  stats.append(
    statTile('events ahead', d.totalEvents),
    statTile('days with none', d.quietDays.length, d.quietDays.length ? 'warn' : 'good'),
    statTile('venues active', d.activeVenues),
    statTile('venues quiet', d.quietVenues.length, d.quietVenues.length ? 'warn' : 'good'),
  );
  frag.append(stats);

  frag.append(barSeries(d.days));

  const axis = document.createElement('p');
  axis.className = 'axis';
  const first = document.createElement('span');
  first.textContent = d.days[0].date;
  const last = document.createElement('span');
  last.textContent = d.days[d.days.length - 1].date;
  axis.append(first, last);
  frag.append(axis);

  if (d.truncated) {
    const warn = document.createElement('p');
    warn.className = 'note note-warn';
    warn.textContent =
      'Result truncated at the pagination ceiling — totals are a floor, not a count. ' +
      'The API exposes no aggregate capability (server#24).';
    frag.append(warn);
  }

  // Quality: defects that a total would hide.
  const q = d.quality;
  const qh = document.createElement('h3');
  qh.textContent = 'Data quality';
  frag.append(qh);
  const qstats = document.createElement('div');
  qstats.className = 'stats stats-sm';
  qstats.append(
    statTile('venues without geo', q.missingGeo, q.missingGeo ? 'warn' : 'good'),
    statTile('unknown start time', q.unknownStartTime, q.unknownStartTime ? 'warn' : 'good'),
    statTile('no description', q.missingDescription, q.missingDescription ? 'warn' : 'good'),
    statTile('name defects', q.duplicatedName, q.duplicatedName ? 'bad' : 'good'),
    statTile('html-escaped names', q.encodedEntities, q.encodedEntities ? 'bad' : 'good'),
  );
  frag.append(qstats);

  if (d.topVenues.length) {
    const h = document.createElement('h3');
    h.textContent = 'Most active venues';
    frag.append(h, table(
      ['Venue', 'Events', 'Mappable'],
      d.topVenues.map((v) => [v.name, String(v.count), v.hasGeo ? 'yes' : 'no']),
    ));
  }

  if (d.quietVenues.length) {
    const h = document.createElement('h3');
    h.textContent = 'Venues with nothing upcoming';
    const note = document.createElement('p');
    note.className = 'note';
    note.textContent =
      'Known places with no events in this window. These are the gaps worth chasing — ' +
      'a venue that stops reporting never appears in the event feed at all.';
    frag.append(h, note, table(
      ['Venue', 'Mappable'],
      d.quietVenues.map((v) => [v.name, v.hasGeo ? 'yes' : 'no']),
    ));
  }

  body.replaceChildren(frag);
}

/* ---------- source health ---------- */

async function loadSources() {
  const body = $('sources-body');
  const payload = await getPanel('/api/sources');
  if (!payload.ok) {
    body.replaceChildren(unavailable(payload));
    return;
  }

  const items = payload.data || [];
  if (!items.length) {
    body.replaceChildren(Object.assign(document.createElement('p'),
      { className: 'note', textContent: 'The node reports no configured sources.' }));
    return;
  }

  const rows = items.map((s) => {
    const status = document.createElement('span');
    const ok = (s.last_run_status || '').toLowerCase() === 'success';
    status.className = `tag ${ok ? 'tag-good' : 'tag-bad'}`;
    status.textContent = s.last_run_status || 'unknown';
    if (s.last_run_error_message) status.title = s.last_run_error_message;

    return [
      s.name || `#${s.id}`,
      s.enabled ? 'on' : 'off',
      status,
      s.last_run_completed_at ? new Date(s.last_run_completed_at).toLocaleString() : '—',
      String(s.last_run_events_found ?? '—'),
      String(s.last_run_events_new ?? '—'),
      String(s.last_run_events_failed ?? '—'),
    ];
  });

  body.replaceChildren(table(
    ['Source', 'Enabled', 'Last run', 'Completed', 'Found', 'New', 'Failed'],
    rows,
  ));
}

/* ---------- provenance ---------- */

async function loadProvenance() {
  const payload = await getPanel('/api/provenance');
  $('provenance-body').replaceChildren(
    payload.ok ? document.createTextNode('') : unavailable(payload));
}

/* ---------- usage ---------- */

async function loadUsage() {
  const body = $('usage-body');
  const payload = await getPanel('/api/usage');
  if (!payload.ok) {
    body.replaceChildren(unavailable(payload));
    return;
  }

  const daily = payload.data || [];
  if (!daily.length) {
    body.replaceChildren(Object.assign(document.createElement('p'),
      { className: 'note', textContent: 'No usage recorded in the last 30 days.' }));
    return;
  }

  const total = daily.reduce((a, d) => a + (d.requests || 0), 0);
  const errors = daily.reduce((a, d) => a + (d.errors || 0), 0);

  const frag = document.createDocumentFragment();
  const stats = document.createElement('div');
  stats.className = 'stats stats-sm';
  stats.append(
    statTile('requests (30d)', total.toLocaleString()),
    statTile('errors (30d)', errors.toLocaleString(), errors ? 'warn' : 'good'),
  );
  frag.append(stats);
  frag.append(barSeries(daily.map((d) => ({ date: d.date, count: d.requests || 0 })), { height: 60 }));
  body.replaceChildren(frag);
}

/* ---------- boot ---------- */

async function loadHealth() {
  const payload = await getPanel('/api/health');
  if (!payload.ok) return;
  $('node-name').textContent = payload.data.node;
  const pill = $('admin-pill');
  pill.hidden = false;
  pill.textContent = payload.data.adminReady ? 'admin connected' : 'public data only';
  pill.className = `pill ${payload.data.adminReady ? 'pill-good' : 'pill-muted'}`;
}

function loadAll() {
  loadHealth();
  loadCoverage();
  loadSources();
  loadProvenance();
  loadUsage();
}

$('refresh').addEventListener('click', loadAll);
$('horizon').addEventListener('change', loadCoverage);
loadAll();
