const POLL_MS = 30 * 1000;

fitStrip();
window.addEventListener('resize', fitStrip);

function fitStrip() {
  const strip = document.querySelector('.strip');
  if (!strip) return;
  const s = Math.min(window.innerWidth / 2560, window.innerHeight / 720);
  strip.style.transform = `scale(${s})`;
  strip.style.left = ((window.innerWidth - 2560 * s) / 2) + 'px';
  strip.style.top = ((window.innerHeight - 720 * s) / 2) + 'px';
}

runBootSplash();

let dashCfg = {
  primary:   { label: 'Hamilton', tz: 'America/Toronto' },
  secondary: { label: 'Adelaide', tz: 'Australia/Adelaide' },
};

loadConfigThenStart();

async function loadConfigThenStart() {
  try {
    const res = await fetch('/api/config', { cache: 'no-store' });
    if (res.ok) {
      const cfg = await res.json();
      if (cfg && cfg.primary && cfg.secondary) dashCfg = cfg;
    }
  } catch (_) { /* keep defaults */ }
  applyLabels();
  tickClocks();
  tickSecondaryHUD();
}

function applyLabels() {
  const set = (sel, text) => {
    const e = document.querySelector(sel);
    if (e) e.textContent = text;
  };
  set('.tile-primary .label', dashCfg.primary.label);
  set('.tile-secondary .label', dashCfg.secondary.label);
  set('.col-weather > .label', dashCfg.primary.label + ' weather');
  set('.col-astro > .label', 'Astro . ' + dashCfg.primary.label);
}

setInterval(tickClocks, 1000);
setInterval(tickSecondaryHUD, 60 * 1000);

setInterval(tickSunsetCountdown, 60 * 1000);
setInterval(updateSunDot, 60 * 1000);
setInterval(updateMoonDot, 60 * 1000);
setInterval(checkDailyReload, 60 * 1000);

function tickClocks() {
  const now = new Date();

  setText('clock-primary-time', fmt12hHM(now, dashCfg.primary.tz));
  setText('clock-primary-sec', ':' + String(now.getSeconds()).padStart(2, '0'));
  setText('clock-primary-ampm', ampm(now, dashCfg.primary.tz));

  const secHM = fmt12hHM(now, dashCfg.secondary.tz);
  const secAP = ampm(now, dashCfg.secondary.tz);
  const secDay = new Intl.DateTimeFormat('en-US', { timeZone: dashCfg.secondary.tz, weekday: 'short' })
    .format(now).toUpperCase();
  setText('clock-secondary-time', secHM + ' ' + secAP);
  setText('clock-secondary-day', secDay);

  const diffH = (tzOffsetMinutes(now, dashCfg.secondary.tz) - tzOffsetMinutes(now, dashCfg.primary.tz)) / 60;
  const sign = diffH >= 0 ? '+' : '';
  const diffStr = Number.isInteger(diffH) ? `${diffH}` : diffH.toFixed(1);
  setText('clock-secondary-offset', `${sign}${diffStr}h`);

  const dateStr = new Intl.DateTimeFormat('en-US', {
    timeZone: dashCfg.primary.tz, weekday: 'short', day: '2-digit', month: 'short', year: 'numeric',
  }).format(now).toUpperCase().replace(',', '');
  setText('topbar-date', dateStr);
  setText('topbar-weekday', ` . W${isoWeek(now)}`);

  setText('topbar-utc', fmtTime(now, 'UTC', { hour: '2-digit', minute: '2-digit', second: '2-digit' }));
}

function fmtTime(d, tz, opts) {
  return new Intl.DateTimeFormat('en-GB', { timeZone: tz, hour12: false, ...opts }).format(d);
}

function fmt12hHM(d, tz) {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: tz, hour: 'numeric', minute: '2-digit', hour12: true,
  }).formatToParts(d);
  const m = {};
  for (const p of parts) m[p.type] = p.value;
  return m.hour + ':' + m.minute;
}

function ampm(d, tz) {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: tz, hour: 'numeric', hour12: true,
  }).formatToParts(d);
  for (const p of parts) if (p.type === 'dayPeriod') return p.value.toUpperCase();
  return '';
}

function hh24To12(h24) {
  const n = parseInt(h24, 10);
  if (isNaN(n)) return h24;
  if (n === 0) return '12 AM';
  if (n === 12) return '12 PM';
  if (n < 12) return n + ' AM';
  return (n - 12) + ' PM';
}

function hhmmTo12(hhmm) {
  const m = /^(\d{1,2}):(\d{2})$/.exec(hhmm);
  if (!m) return hhmm;
  const h = parseInt(m[1], 10);
  const ap = h >= 12 ? 'PM' : 'AM';
  const h12 = (h % 12) || 12;
  return h12 + ':' + m[2] + ' ' + ap;
}

function tzOffsetMinutes(date, tz) {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: tz, year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
  }).formatToParts(date);
  const m = {};
  for (const p of parts) m[p.type] = p.value;
  const utcMs = Date.UTC(+m.year, +m.month - 1, +m.day, +m.hour, +m.minute, +m.second);
  return Math.round((utcMs - date.getTime()) / 60000);
}

function isoWeek(d) {
  const t = new Date(Date.UTC(d.getFullYear(), d.getMonth(), d.getDate()));
  const dayNum = (t.getUTCDay() + 6) % 7;
  t.setUTCDate(t.getUTCDate() - dayNum + 3);
  const firstThursday = new Date(Date.UTC(t.getUTCFullYear(), 0, 4));
  return 1 + Math.round(((t - firstThursday) / 86400000 - 3 + (firstThursday.getUTCDay() + 6) % 7) / 7);
}

const widgets = [
  { col: 'weather', url: '/api/weather', render: renderWeather },
  { col: 'astro',   url: '/api/astro',   render: renderAstro   },
  { col: 'feed',    url: '/api/feed',    render: renderFeed    },
  { col: null,      url: '/api/aqhi',    render: renderAQHI    },
  { col: null,      url: '/api/alerts',  render: renderAlerts  },
];


for (const w of widgets) {
  poll(w);
  setInterval(() => poll(w), POLL_MS);
}


async function poll(w) {
  try {
    const res = await fetch(w.url, { cache: 'no-store' });
    if (!res.ok) throw new Error('HTTP ' + res.status);
    const data = await res.json();
    w.render(data);
    if (w.col) setStatus(w.col, 'live');
  } catch (e) {
    console.warn('[' + w.col + '] poll failed:', e);
    if (w.col) setStatus(w.col, 'stale');
  }
}

function renderWeather(d) {
  const c = d.current || {};
  setText('weather-temp', Math.round(c.temp) + '°');
  const meta = `feels ${Math.round(c.feels_like)} . wind ${c.wind_kmh} km/h ${c.wind_dir}`;
  setText('weather-meta', meta);
  setText('weather-summary', d.summary || '');
  const iconEl = el('weather-icon');
  if (iconEl) iconEl.innerHTML = wmoIcon(c.code);

  const allHourly = d.hourly || [];

  const hEl = el('weather-hourly');
  hEl.innerHTML = '';
  for (const h of allHourly.slice(0, 8)) {
    const tile = document.createElement('div');
    tile.className = 'h';
    const precip = Math.max(0, Math.min(100, h.precip_pct || 0));
    tile.innerHTML = `
      <div class="precip-fill" style="height:${precip}%"></div>
      <span class="t">${Math.round(h.temp)}°</span>
      <span class="hr">${esc(hh24To12(h.hour))}</span>
      ${precip >= 30 ? `<span class="precip-pct">${precip}%</span>` : ''}`;
    hEl.appendChild(tile);
  }

  renderTomorrow(d.tomorrow);
}

function renderTomorrow(t) {
  const tile = el('tomorrow-tile');
  if (!tile) return;
  if (!t) { tile.style.display = 'none'; return; }
  tile.style.display = '';
  setText('tomorrow-high', Math.round(t.high_c));
  setText('tomorrow-low', Math.round(t.low_c));
  setText('tomorrow-desc', t.description || '');
  if (t.precip_pct_max != null && t.precip_pct_max >= 20) {
    setText('tomorrow-precip', 'rain ' + t.precip_pct_max + '%');
  } else {
    setText('tomorrow-precip', '');
  }
  const iconEl = el('tomorrow-icon');
  if (iconEl) iconEl.innerHTML = wmoIcon(t.code);
  const wd = new Intl.DateTimeFormat('en-US', { weekday: 'short' })
    .format(new Date(Date.now() + 24 * 60 * 60 * 1000)).toUpperCase();
  setText('tomorrow-weekday', wd);
}

function wmoIcon(code) {
  const wrap = body =>
    `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"
      stroke-linecap="round" stroke-linejoin="round" style="width:100%;height:100%">${body}</svg>`;
  if (code == null || code < 0) return '';
  // Clear / mainly clear
  if (code === 0 || code === 1) return wrap(`
    <circle cx="12" cy="12" r="4"/>
    <line x1="12" y1="2" x2="12" y2="5"/>
    <line x1="12" y1="19" x2="12" y2="22"/>
    <line x1="2" y1="12" x2="5" y2="12"/>
    <line x1="19" y1="12" x2="22" y2="12"/>
    <line x1="4.9" y1="4.9" x2="7" y2="7"/>
    <line x1="17" y1="17" x2="19.1" y2="19.1"/>
    <line x1="4.9" y1="19.1" x2="7" y2="17"/>
    <line x1="17" y1="7" x2="19.1" y2="4.9"/>`);
  // Partly cloudy
  if (code === 2) return wrap(`
    <circle cx="7" cy="8" r="3"/>
    <line x1="7" y1="2" x2="7" y2="3.5"/>
    <line x1="2" y1="8" x2="3.5" y2="8"/>
    <line x1="3.2" y1="4.2" x2="4.5" y2="5.5"/>
    <line x1="10.8" y1="4.2" x2="9.5" y2="5.5"/>
    <path d="M18 18h-9a4 4 0 1 1 .8-7.92A6 6 0 0 1 21 17a2 2 0 0 1-3 1z"/>`);
  // Overcast
  if (code === 3) return wrap(`
    <path d="M18 15h-9a4 4 0 1 1 .8-7.92A6 6 0 0 1 21 14a3 3 0 0 1-3 1z"/>
    <path d="M5 18h12" opacity="0.55"/>`);
  // Fog
  if (code === 45 || code === 48) return wrap(`
    <line x1="3" y1="9" x2="21" y2="9"/>
    <line x1="3" y1="13" x2="21" y2="13"/>
    <line x1="3" y1="17" x2="21" y2="17"/>`);
  // Drizzle (light / freezing)
  if (code === 51 || code === 53 || code === 56) return wrap(`
    <path d="M18 12h-9a4 4 0 1 1 .8-7.92A6 6 0 0 1 21 11a3 3 0 0 1-3 1z"/>
    <line x1="8" y1="16" x2="8" y2="18"/>
    <line x1="12" y1="16" x2="12" y2="18"/>
    <line x1="16" y1="16" x2="16" y2="18"/>`);
  // Rain (heavier drizzle, light rain, showers)
  if (code === 55 || code === 57 || code === 61 || code === 63 || code === 66 || code === 80 || code === 81)
    return wrap(`
      <path d="M18 12h-9a4 4 0 1 1 .8-7.92A6 6 0 0 1 21 11a3 3 0 0 1-3 1z"/>
      <line x1="8" y1="16" x2="7" y2="20"/>
      <line x1="12" y1="16" x2="11" y2="20"/>
      <line x1="16" y1="16" x2="15" y2="20"/>`);
  // Heavy rain
  if (code === 65 || code === 67 || code === 82) return wrap(`
    <path d="M18 11h-9a4 4 0 1 1 .8-7.92A6 6 0 0 1 21 10a3 3 0 0 1-3 1z"/>
    <line x1="6" y1="15" x2="5" y2="18"/>
    <line x1="9" y1="15" x2="8" y2="18"/>
    <line x1="12" y1="15" x2="11" y2="18"/>
    <line x1="15" y1="15" x2="14" y2="18"/>
    <line x1="18" y1="15" x2="17" y2="18"/>
    <line x1="9" y1="19" x2="8" y2="22"/>
    <line x1="13" y1="19" x2="12" y2="22"/>`);
  // Snow + snow showers
  if ((code >= 71 && code <= 77) || code === 85 || code === 86) return wrap(`
    <path d="M18 12h-9a4 4 0 1 1 .8-7.92A6 6 0 0 1 21 11a3 3 0 0 1-3 1z"/>
    <circle cx="8"  cy="17" r="0.8" fill="currentColor" stroke="none"/>
    <circle cx="12" cy="19" r="0.8" fill="currentColor" stroke="none"/>
    <circle cx="16" cy="17" r="0.8" fill="currentColor" stroke="none"/>
    <circle cx="9"  cy="21" r="0.8" fill="currentColor" stroke="none"/>
    <circle cx="15" cy="21" r="0.8" fill="currentColor" stroke="none"/>`);
  // Thunderstorm
  if (code === 95 || code === 96 || code === 99) return wrap(`
    <path d="M18 11h-9a4 4 0 1 1 .8-7.92A6 6 0 0 1 21 10a3 3 0 0 1-3 1z"/>
    <polyline points="11 14 9 19 13 19 11 22" stroke="#e08a3a" stroke-width="2"/>`);
  // Fallback - simple cloud
  return wrap(`<path d="M18 15h-9a4 4 0 1 1 .8-7.92A6 6 0 0 1 21 14a3 3 0 0 1-3 1z"/>`);
}


let lastAstro = null;
function renderAstro(d) {
  lastAstro = d;
  const sun = d.sun || {};
  const moon = d.moon || {};
  setText('astro-sunrise', sun.rise ? hhmmTo12(sun.rise) : '--:--');
  setText('astro-sunset', sun.set ? hhmmTo12(sun.set) : '--:--');
  setText('astro-moon-rise', moon.rise ? hhmmTo12(moon.rise) : '--:--');
  setText('astro-moon-set', moon.set ? hhmmTo12(moon.set) : '--:--');
  setText('astro-moon-phase', moon.phase ? '. ' + moon.phase.toLowerCase() : '');
  updateMoonMarkerShape(moon);
  updateSunDot();
  updateMoonDot();
  tickSunsetCountdown();
}

function updateMoonMarkerShape(moon) {
  const el = document.getElementById('moon-marker-dark');
  if (!el) return;
  const illum = (moon.illum_pct || 0) / 100;
  const waxing = /Waxing|First|^New$/i.test(moon.phase || '');
  el.setAttribute('d', moonDarkPath(illum, waxing));
}

function moonDarkPath(illum, waxing) {
  const r = 48;
  if (illum <= 0.01) return `M ${-r},0 A ${r},${r} 0 1 1 ${r},0 A ${r},${r} 0 1 1 ${-r},0 Z`;
  if (illum >= 0.99) return '';
  const k = r * Math.abs(1 - 2 * illum);
  if (waxing) {
    const sweep = illum < 0.5 ? 0 : 1;
    return `M 0,${-r} A ${r},${r} 0 0 0 0,${r} A ${k},${r} 0 0 ${sweep} 0,${-r} Z`;
  }
  const sweep = illum < 0.5 ? 1 : 0;
  return `M 0,${-r} A ${r},${r} 0 0 1 0,${r} A ${k},${r} 0 0 ${sweep} 0,${-r} Z`;
}

function updateSunDot() {
  positionArcDot('sun-dot', 'astro-arc', (lastAstro || {}).sun);
}

function updateMoonDot() {
  positionArcDot('moon-dot', 'moon-arc', (lastAstro || {}).moon);
}

function positionArcDot(dotId, arcId, body) {
  const dot = document.getElementById(dotId);
  const arc = document.getElementById(arcId);
  if (!dot || !arc) return;
  if (!body || !body.rise_unix || !body.set_unix) { dot.classList.add('hidden'); return; }
  const nowSec = Date.now() / 1000;
  let rise = body.rise_unix;
  let set = body.set_unix;
  // Moon set may roll into the next day. If we're past rise but before set,
  // the linear (nowSec - rise) / (set - rise) gives the right fraction.
  if (nowSec < rise || nowSec > set) { dot.classList.add('hidden'); return; }
  const t = (nowSec - rise) / (set - rise);
  const w = arc.clientWidth;
  const h = arc.clientHeight;
  const x = ((1 - Math.cos(Math.PI * t)) / 2) * w;
  const y = Math.sin(Math.PI * t) * h;
  const rect = dot.getBoundingClientRect();
  const halfW = (rect.width || dot.offsetWidth || 12) / 2;
  const halfH = (rect.height || dot.offsetHeight || 12) / 2;
  dot.classList.remove('hidden');
  dot.style.left = (x - halfW) + 'px';
  dot.style.top = (h - y - halfH) + 'px';
}

function parseMin(hhmm) {
  const m = /^(\d{1,2}):(\d{2})$/.exec(hhmm || '');
  return m ? +m[1] * 60 + +m[2] : null;
}

function nowPrimaryMin() {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: dashCfg.primary.tz, hour: '2-digit', minute: '2-digit', hour12: false,
  }).formatToParts(new Date());
  const m = {};
  for (const p of parts) m[p.type] = p.value;
  return +m.hour * 60 + +m.minute;
}

let didReloadToday = false;
function checkDailyReload() {
  const m = nowPrimaryMin();
  if (m === 240 && !didReloadToday) {
    didReloadToday = true;
    location.reload();
  }
  if (m > 241) didReloadToday = false;
}


function renderAQHI(d) {
  const v = (d && d.value != null) ? d.value : null;
  const scale = (d && d.scale) || 'AQHI';
  const band = bandFor(v, scale);
  const node = document.getElementById('topbar-aqhi');
  node.textContent = (v != null) ? v : '-';
  node.className = `aqhi-num aqhi-${band}`;
  setText('topbar-aqhi-scale', scale);
  setText('topbar-aqhi-band', (d && d.band) || '');
}

function bandFor(v, scale) {
  if (v == null) return 'cold';
  if (scale === 'EAQI') {
    if (v <= 20) return 'low';
    if (v <= 40) return 'low';
    if (v <= 60) return 'mod';
    if (v <= 80) return 'high';
    return 'vhigh';
  }
  if (v <= 3) return 'low';
  if (v <= 6) return 'mod';
  if (v <= 10) return 'high';
  return 'vhigh';
}

const SEV_CRITICAL = new Set([
  'actively exploited', 'in the wild', 'exploitation in the wild', 'zero-day',
]);

function renderFeed(items) {
  const list = el('feed-list');
  list.innerHTML = '';
  const arr = Array.isArray(items) ? items : [];
  setText('topbar-feed-count', String(arr.length));
  for (const it of arr.slice(0, 12)) {
    const li = document.createElement('li');
    const cats = it.categories || [];
    const cves = cveSet(it);
    const sev = classifySeverity(cats);
    li.className = 'feed-line' + (sev ? ' sev-' + sev : '');
    if (sev === 'critical') {
      li.classList.add('glitch-in');
      setTimeout(() => li.classList.remove('glitch-in'), 240);
    }
    const ts = it.pub_date ? relTime(new Date(it.pub_date)) : '';
    const chips = [...cves].slice(0, 2)
      .map(cve => `<span class="cve-chip">${esc(cve)}</span>`).join('');
    li.innerHTML = `
      <span class="src">${esc(it.source || '?')}</span>
      ${chips}
      <a class="title" href="${esc(it.link || '#')}" target="_blank" rel="noopener">${esc(it.title || '')}</a>
      <span class="ts">${ts}</span>`;
    list.appendChild(li);
  }
  renderTopCVE(arr);
}

function renderTopCVE(items) {
  const counts = {};
  const cutoff = Date.now() - 24 * 60 * 60 * 1000;
  for (const it of (items || [])) {
    if (it.pub_date && new Date(it.pub_date).getTime() < cutoff) continue;
    const seen = new Set();
    for (const c of (it.categories || [])) {
      const m = /CVE-\d{4}-\d{4,7}/i.exec(c);
      if (m) seen.add(m[0].toUpperCase());
    }
    for (const cve of seen) counts[cve] = (counts[cve] || 0) + 1;
  }
  let top = null, max = 0;
  for (const [cve, n] of Object.entries(counts)) {
    if (n > max) { top = cve; max = n; }
  }
  const block = el('topbar-cve-block');
  if (!block) return;
  if (!top) { block.style.display = 'none'; return; }
  block.style.display = '';
  setText('topbar-cve', top);
  setText('topbar-cve-count', `(${max}x)`);
}

function renderAlerts(alerts) {
  const banner = el('alert-banner');
  if (!banner) return;
  const arr = Array.isArray(alerts) ? alerts : [];
  if (!arr.length) { banner.style.display = 'none'; return; }
  const top = arr[0];
  const headline = (top.description || titleCase(top.type) || 'ALERT').trim();
  const body = (top.body || '').trim();
  const extra = arr.length > 1 ? ` (+${arr.length - 1} more)` : '';
  banner.style.display = '';
  banner.href = top.url || '#';
  banner.title = body || headline;
  banner.innerHTML = `<b>${esc(headline.toUpperCase())}</b>${body ? ' . ' + esc(body) : ''}${esc(extra)}`;
}

function titleCase(s) {
  if (!s) return '';
  return s.replace(/\b\w/g, c => c.toUpperCase());
}

function cveSet(it) {
  const out = new Set();
  for (const c of (it.categories || [])) {
    const m = /CVE-\d{4}-\d{4,7}/i.exec(c);
    if (m) out.add(m[0].toUpperCase());
  }
  return out;
}

function classifySeverity(cats) {
  const lower = cats.map(c => c.toLowerCase());
  if (lower.some(c => SEV_CRITICAL.has(c))) return 'critical';
  if (lower.some(c => c.startsWith('kev:'))) return 'kev';
  return null;
}

function relTime(d) {
  const diffM = Math.max(0, Math.round((Date.now() - d.getTime()) / 60000));
  if (diffM < 60) return `${diffM}m`;
  const h = Math.round(diffM / 60);
  if (h < 24) return `${h}h`;
  return `${Math.round(h / 24)}d`;
}

function setStatus(col, status) {
  const els = document.querySelectorAll('.col-' + col);
  for (const e of els) e.dataset.status = status;
}

function el(id) { return document.getElementById(id); }
function setText(id, v) { const e = el(id); if (e) e.textContent = v; }
function esc(s) {
  return String(s).replace(/[&<>"']/g, c =>
    ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}

function tickSunsetCountdown() {
  const out = el('astro-sunset-countdown');
  if (!out) return;
  const setStr = (lastAstro && lastAstro.sun && lastAstro.sun.set) || '';
  if (!/^\d{1,2}:\d{2}$/.test(setStr)) { out.textContent = ''; return; }
  const [sh, sm] = setStr.split(':').map(Number);
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: dashCfg.primary.tz, hour: '2-digit', minute: '2-digit',
    hour12: false,
  }).formatToParts(new Date());
  const m = {};
  for (const p of parts) m[p.type] = p.value;
  const nowH = +m.hour, nowM = +m.minute;
  const diff = (sh * 60 + sm) - (nowH * 60 + nowM);
  if (diff <= 0) { out.textContent = ''; return; }
  const hh = Math.floor(diff / 60);
  const mm = diff % 60;
  out.textContent = '. sets in ' + (hh > 0 ? `${hh}h ${mm}m` : `${mm}m`);
}

function tickSecondaryHUD() {
  const hud = el('secondary-hud');
  if (!hud) return;
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: dashCfg.secondary.tz, weekday: 'short',
    hour: '2-digit', minute: '2-digit', hour12: false,
  }).formatToParts(new Date());
  const m = {};
  for (const p of parts) m[p.type] = p.value;
  const day = m.weekday;
  const total = (+m.hour) * 60 + (+m.minute);
  const start = 9 * 60, end = 17 * 60;
  const isWeekend = day === 'Sat' || day === 'Sun';
  const tag = (dashCfg.secondary.label || 'Secondary').toUpperCase();
  let status, text;
  if (isWeekend) {
    status = 'offline';
    text = `${tag} . weekend`;
  } else if (total >= start && total < end) {
    status = 'online';
    text = `${tag} . online now`;
  } else if (total < start) {
    const mins = start - total;
    const hh = Math.floor(mins / 60), mm = mins % 60;
    status = (mins <= 60) ? 'soon' : 'offline';
    text = `${tag} . online in ` + (hh ? `${hh}h ${mm}m` : `${mm}m`);
  } else {
    const mins = (24 * 60 - total) + start;
    const hh = Math.floor(mins / 60), mm = mins % 60;
    status = 'offline';
    text = `${tag} . online in ` + (hh ? `${hh}h ${mm}m` : `${mm}m`);
  }
  hud.dataset.status = status;
  setText('secondary-hud-text', text);
}

function runBootSplash() {
  const wrap = el('boot-lines');
  const splash = el('boot-splash');
  if (!wrap || !splash) return;
  const steps = [
    '[ <span class="ok">OK</span>   ] xeneon-dash bootstrapping',
    '[ <span class="ok">OK</span>   ] embed.fs mounted',
    '[ <span class="ok">OK</span>   ] cache initialized',
    '[ <span class="wait">..</span>   ] weather   .  .  .',
    '[ <span class="wait">..</span>   ] aqhi      .  .  .',
    '[ <span class="wait">..</span>   ] astro     .  .  .',
    '[ <span class="wait">..</span>   ] feed      .  .  .',
    '[ <span class="wait">..</span>   ] alerts    .  .  .',
    '[ <span class="ok">OK</span>   ] kiosk handshake',
    '[ <span class="ok">OK</span>   ] ready',
  ];
  let i = 0;
  const id = setInterval(() => {
    if (i >= steps.length) {
      clearInterval(id);
      setTimeout(() => splash.classList.add('fade'), 220);
      setTimeout(() => splash.style.display = 'none', 900);
      return;
    }
    const line = document.createElement('div');
    line.className = 'boot-line';
    line.innerHTML = steps[i];
    wrap.appendChild(line);
    i++;
  }, 110);
}
