const state = {
  config: null,
  clerk: null,
  me: null,
  route: location.hash.slice(1) || 'overview',
  dashboard: null,
  models: [],
  settings: null,
  cache: {},
  modal: null,
  studioTab: 'stt',
  sttMode: 'file',
  recorder: null,
  recording: null,
  stream: null,
  audioURL: '',
};

const routes = [
  ['overview', 'Overview', 'grid'],
  ['studio', 'Speech studio', 'mic'],
  ['vocabulary', 'Dictionary', 'book'],
  ['snippets', 'Snippets', 'zap'],
  ['profiles', 'Profiles', 'users'],
  ['history', 'History', 'clock'],
  ['settings', 'Providers & settings', 'sliders'],
  ['keys', 'Device keys', 'key'],
];

const icons = {
  grid: '<rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/>',
  mic: '<path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z"/><path d="M19 10v2a7 7 0 0 1-14 0v-2M12 19v3"/>',
  book: '<path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20V3H6.5A2.5 2.5 0 0 0 4 5.5v14Z"/><path d="M8 7h8M8 11h6"/>',
  zap: '<path d="m13 2-9 12h8l-1 8 9-12h-8l1-8Z"/>',
  users: '<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8ZM22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  sliders: '<path d="M4 21v-7M4 10V3M12 21v-9M12 8V3M20 21v-5M20 12V3M1 14h6M9 8h6M17 16h6"/>',
  key: '<circle cx="7.5" cy="15.5" r="5.5"/><path d="m12 12 9-9M18 6l3 3M15 9l3 3"/>',
  menu: '<path d="M4 6h16M4 12h16M4 18h16"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  upload: '<path d="M12 16V4M7 9l5-5 5 5M4 20h16"/>',
  copy: '<rect x="9" y="9" width="11" height="11" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>',
  download: '<path d="M12 3v12M7 10l5 5 5-5M4 21h16"/>',
  play: '<circle cx="12" cy="12" r="9"/><path d="m10 8 6 4-6 4V8Z"/>',
  trash: '<path d="M3 6h18M8 6V4h8v2M19 6l-1 15H6L5 6M10 11v6M14 11v6"/>',
  edit: '<path d="M12 20h9M16.5 3.5a2.12 2.12 0 0 1 3 3L8 18l-4 1 1-4Z"/>',
  search: '<circle cx="11" cy="11" r="7"/><path d="m20 20-4-4"/>',
  activity: '<path d="M3 12h4l2-7 4 14 2-7h6"/>',
  dollar: '<circle cx="12" cy="12" r="9"/><path d="M16 8.5c-.7-.9-1.8-1.5-3.2-1.5-1.8 0-3.3.9-3.3 2.4 0 3.6 6.9 1.8 6.9 5.2 0 1.5-1.5 2.4-3.5 2.4-1.6 0-3-.6-3.8-1.6M13 5v14"/>',
  alert: '<circle cx="12" cy="12" r="9"/><path d="M12 8v5M12 16.5v.5"/>',
  arrow: '<path d="M5 12h14M13 6l6 6-6 6"/>',
  x: '<path d="m6 6 12 12M18 6 6 18"/>',
  eye: '<path d="M2 12s3.5-6 10-6 10 6 10 6-3.5 6-10 6S2 12 2 12Z"/><circle cx="12" cy="12" r="2.5"/>',
  logout: '<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4M16 17l5-5-5-5M21 12H9"/>',
  chevron: '<path d="m9 18 6-6-6-6"/>',
  file: '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8Z"/><path d="M14 2v6h6"/>',
  phone: '<rect x="5" y="2" width="14" height="20" rx="3"/><path d="M10 18h4"/>',
  wifi: '<path d="M5 12.6a10 10 0 0 1 14 0M8.5 16.1a5 5 0 0 1 7 0M12 20h.01"/>',
  check: '<path d="m5 12 4 4L19 6"/>',
  external: '<path d="M15 3h6v6M10 14 21 3M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>',
};

function icon(name, cls = '') {
  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  svg.setAttribute('viewBox', '0 0 24 24');
  svg.setAttribute('fill', 'none');
  svg.setAttribute('stroke', 'currentColor');
  svg.setAttribute('stroke-linecap', 'round');
  svg.setAttribute('stroke-linejoin', 'round');
  svg.setAttribute('aria-hidden', 'true');
  if (cls) svg.setAttribute('class', cls);
  svg.innerHTML = icons[name] || icons.activity;
  return svg;
}

function el(tag, attrs = {}, ...children) {
  const node = document.createElement(tag);
  for (const [key, value] of Object.entries(attrs)) {
    if (value == null || value === false) continue;
    if (key === 'class') node.className = value;
    else if (key === 'on') Object.entries(value).forEach(([event, fn]) => node.addEventListener(event, fn));
    else if (key === 'text') node.textContent = value;
    else if (key === 'html') node.innerHTML = value;
    else if (key in node) { try { node[key] = value; } catch { node.setAttribute(key, value); } }
    else node.setAttribute(key, value === true ? '' : value);
  }
  for (const child of children.flat(Infinity)) {
    if (child == null || child === false) continue;
    node.append(child.nodeType ? child : document.createTextNode(String(child)));
  }
  return node;
}

function button(label, opts = {}) {
  const b = el('button', {
    type: opts.type || 'button',
    class: `button ${opts.kind || ''} ${opts.small ? 'small' : ''}`.trim(),
    disabled: opts.disabled,
    on: opts.onClick ? { click: opts.onClick } : {},
  });
  if (opts.icon) b.append(icon(opts.icon));
  b.append(label);
  return b;
}

function toast(message, type = '') {
  const box = el('div', { class: `toast ${type}`, role: type === 'error' ? 'alert' : 'status' }, message);
  document.getElementById('toast-region').append(box);
  setTimeout(() => box.remove(), 4200);
}

function formatNumber(value, digits = 0) {
  const n = Number(value);
  if (!Number.isFinite(n)) return '—';
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: digits }).format(n);
}

function money(value) {
  const n = Number(value);
  return Number.isFinite(n) ? new Intl.NumberFormat(undefined, { style: 'currency', currency: 'USD', minimumFractionDigits: n < 1 ? 3 : 2, maximumFractionDigits: n < 1 ? 6 : 2 }).format(n) : '—';
}

function dateTime(value) {
  if (!value) return '—';
  const d = new Date(value);
  return Number.isNaN(d.valueOf()) ? '—' : new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(d);
}

function relativeTime(value) {
  if (!value) return 'Never';
  const ms = new Date(value).getTime();
  if (!Number.isFinite(ms)) return 'Never';
  const diff = Date.now() - ms;
  const min = Math.round(diff / 60000);
  if (min < 1) return 'Just now';
  if (min < 60) return `${min}m ago`;
  const hr = Math.round(min / 60);
  if (hr < 24) return `${hr}h ago`;
  const day = Math.round(hr / 24);
  return `${day}d ago`;
}

function statusBadge(text, tone = '') { return el('span', { class: `badge ${tone}` }, text); }

function emptyState(title, copy, iconName = 'file') {
  return el('div', { class: 'empty' },
    el('div', { class: 'empty-icon' }, icon(iconName)),
    el('h3', {}, title), el('p', {}, copy));
}

function loading() { return el('div', { class: 'loading' }, el('div', { class: 'spinner', 'aria-label': 'Loading' })); }

function pageHead(kicker, title, copy, actions = []) {
  return el('div', { class: 'page-head' },
    el('div', {}, el('p', { class: 'eyebrow' }, kicker), el('h1', {}, title), el('p', {}, copy)),
    actions.length ? el('div', { class: 'head-actions' }, actions) : null);
}

function cardSection(title, subtitle, content, action = null, extra = '') {
  return el('section', { class: `card ${extra}` },
    el('div', { class: 'section-head' }, el('div', {}, el('h2', {}, title), subtitle ? el('p', {}, subtitle) : null), action),
    el('div', { class: 'section-body' }, content));
}

async function parseResponse(response) {
  const type = response.headers.get('content-type') || '';
  if (type.includes('application/json')) return response.json();
  return response.text();
}

async function api(path, options = {}) {
  const response = await authFetch(path, options);
  const data = await parseResponse(response).catch(() => null);
  if (!response.ok) {
    const message = responseError(data, `Request failed (${response.status})`);
    const error = new Error(message);
    error.status = response.status;
    error.data = data;
    throw error;
  }
  return { data, response };
}

function responseError(data, fallback) {
  if (typeof data === 'string' && data.trim()) return data;
  if (data && typeof data === 'object') {
    if (typeof data.message === 'string') return data.message;
    if (typeof data.error === 'string') return data.error;
    if (data.error && typeof data.error.message === 'string') return data.error.message;
  }
  return fallback;
}

async function authFetch(path, options = {}) {
  const token = state.clerk?.session ? await state.clerk.session.getToken() : null;
  if (!token) throw new Error('Your session has expired. Please sign in again.');
  const headers = new Headers(options.headers || {});
  headers.set('Authorization', `Bearer ${token}`);
  let body = options.body;
  if (body && !(body instanceof FormData) && typeof body !== 'string' && !(body instanceof Blob)) {
    headers.set('Content-Type', 'application/json');
    body = JSON.stringify(body);
  }
  return fetch(path, { ...options, headers, body, credentials: 'same-origin' });
}

async function loadScript(src, attrs = {}) {
  return new Promise((resolve, reject) => {
    const script = document.createElement('script');
    script.src = src;
    script.crossOrigin = 'anonymous';
    script.async = false;
    Object.entries(attrs).forEach(([k, v]) => script.setAttribute(k, v));
    const timer = setTimeout(() => reject(new Error('Sign-in service timed out.')), 20000);
    script.onload = () => { clearTimeout(timer); resolve(); };
    script.onerror = () => { clearTimeout(timer); reject(new Error('Could not load the sign-in service.')); };
    document.head.append(script);
  });
}

async function initialiseClerk(config) {
  const clerkConfig = config?.clerk;
  if (!clerkConfig?.publishable_key || !clerkConfig?.frontend_api) throw new Error('Google sign-in is not configured.');
  const host = String(clerkConfig.frontend_api).replace(/^https?:\/\//, '').replace(/\/+$/, '');
  if (!/^[a-z0-9.-]+(:\d+)?$/i.test(host)) throw new Error('The sign-in host is invalid.');
  await Promise.all([
    loadScript(`https://${host}/npm/@clerk/ui@1/dist/ui.browser.js`),
    loadScript(`https://${host}/npm/@clerk/clerk-js@6/dist/clerk.browser.js`, { 'data-clerk-publishable-key': clerkConfig.publishable_key }),
  ]);
  let clerk = window.Clerk;
  if (typeof clerk === 'function') clerk = new clerk(clerkConfig.publishable_key);
  if (!clerk?.load) throw new Error('Google sign-in did not initialise.');
  if (!window.__internal_ClerkUICtor) throw new Error('Google sign-in components did not initialise.');
  await clerk.load({ telemetry: false, ui: { ClerkUI: window.__internal_ClerkUICtor } });
  state.clerk = clerk;
}

function renderAuth(error = '') {
  const app = document.getElementById('app');
  const mount = el('div', { class: 'clerk-mount', id: 'clerk-sign-in' });
  const panel = el('main', { class: 'auth-page' },
    el('section', { class: 'auth-panel' },
      el('div', { class: 'auth-brand' }, waveMark(), el('span', {}, 'Hypnos Speech')),
      el('h1', {}, 'Your voice, composed.'),
      el('p', {}, 'A private speech workspace for transcription, voices, and the words that make them yours.'),
      error ? el('div', { class: 'auth-error', role: 'alert' }, error) : mount),
    el('aside', { class: 'auth-art', 'aria-hidden': 'true' },
      el('div', { class: 'auth-quote' }, authWave(),
        el('p', {}, 'Fast when it should be. Precise when it matters.'),
        el('small', {}, 'Running privately on Olympus'))));
  app.replaceChildren(panel);
  if (!error && state.clerk) {
    const ownURL = location.origin + location.pathname;
    try {
      state.clerk.mountSignIn(mount, { forceRedirectUrl: ownURL, signUpForceRedirectUrl: ownURL });
      if (!state.authListener) {
        state.authListener = state.clerk.addListener(({ user }) => {
          if (user) location.replace(ownURL);
        }, { skipInitialEmit: true });
      }
    } catch (e) { renderAuth(e.message || 'Could not show Google sign-in.'); }
  }
}

function waveMark() {
  return el('div', { class: 'brand-mark', 'aria-hidden': 'true' }, ...Array.from({ length: 5 }, () => el('span')));
}

function authWave() { return el('div', { class: 'auth-wave' }, ...Array.from({ length: 12 }, () => el('i'))); }

async function boot() {
  try {
    const response = await fetch('/api/config', { credentials: 'same-origin' });
    if (!response.ok) throw new Error(`Configuration unavailable (${response.status}).`);
    state.config = await response.json();
    await initialiseClerk(state.config);
    if (!state.clerk.user || !state.clerk.session) { renderAuth(); return; }
    const [{ data: me }, { data: models }] = await Promise.all([api('/api/me'), api('/api/models')]);
    state.me = me;
    state.models = models?.data || [];
    renderShell();
    await navigate(state.route, false);
  } catch (error) {
    if (state.clerk && !state.clerk.user) renderAuth();
    else renderAuth(error.message || 'Hypnos Speech could not start.');
  }
}

function navButton([id, label, iconName]) {
  return el('button', { class: state.route === id ? 'active' : '', on: { click: () => navigate(id) } }, icon(iconName), label);
}

function renderShell() {
  const shell = el('div', { class: 'shell', id: 'shell' },
    el('aside', { class: 'sidebar' },
      el('div', { class: 'sidebar-head' }, waveMark(), el('span', {}, 'Hypnos Speech')),
      el('p', { class: 'sidebar-section' }, 'Workspace'),
      el('nav', { class: 'nav', 'aria-label': 'Main navigation', id: 'main-nav' }, routes.slice(0, 6).map(navButton)),
      el('p', { class: 'sidebar-section' }, 'System'),
      el('nav', { class: 'nav', 'aria-label': 'System navigation', id: 'system-nav' }, routes.slice(6).map(navButton)),
      el('div', { class: 'sidebar-foot' },
        el('button', { class: 'account', title: 'Sign out', on: { click: signOut } },
          el('span', { class: 'avatar' }, (state.me?.name || state.me?.email || 'T')[0].toUpperCase()),
          el('span', {}, el('strong', {}, state.me?.name || 'Tushar'), el('small', {}, state.me?.email || 'Owner')),
          icon('logout')))),
    el('div', { class: 'workspace' },
      el('header', { class: 'topbar' },
        el('div', { class: 'top-actions' },
          el('button', { class: 'mobile-menu', 'aria-label': 'Open menu', on: { click: toggleMenu } }, icon('menu')),
          el('div', { class: 'breadcrumb' }, 'Workspace / ', el('strong', { id: 'route-label' }, routeLabel(state.route)))),
        el('div', { class: 'top-actions' },
          el('span', { class: 'system-pill' }, el('i', { class: 'dot' }), el('span', {}, 'Gateway online')))),
      el('main', { class: 'page', id: 'page', tabindex: '-1' })));
  document.getElementById('app').replaceChildren(shell);
  shell.addEventListener('click', (event) => {
    if (shell.classList.contains('menu-open') && event.target === shell) toggleMenu();
  });
}

function routeLabel(id) { return routes.find((route) => route[0] === id)?.[1] || 'Overview'; }
function toggleMenu() { document.getElementById('shell')?.classList.toggle('menu-open'); }

async function signOut() {
  try { await state.clerk.signOut({ redirectUrl: location.origin + location.pathname }); }
  catch { location.reload(); }
}

async function navigate(route, push = true) {
  if (!routes.some(([id]) => id === route)) route = 'overview';
  stopAllRecording();
  state.route = route;
  if (push) history.pushState(null, '', `#${route}`);
  document.querySelectorAll('.nav button').forEach((b) => b.classList.remove('active'));
  const tuple = routes.find(([id]) => id === route);
  const nav = [...document.querySelectorAll('.nav button')].find((b) => b.textContent.trim() === tuple[1]);
  nav?.classList.add('active');
  const label = document.getElementById('route-label');
  if (label) label.textContent = tuple[1];
  document.getElementById('shell')?.classList.remove('menu-open');
  const page = document.getElementById('page');
  page.replaceChildren(loading());
  try {
    await ({ overview: renderOverview, studio: renderStudio, vocabulary: () => renderCollection('dictionary'), snippets: () => renderCollection('snippets'), profiles: () => renderCollection('profiles'), history: renderHistory, settings: renderSettings, keys: renderKeys })[route]();
    page.focus({ preventScroll: true });
  } catch (error) {
    renderPageError(error);
  }
}

function renderPageError(error) {
  const page = document.getElementById('page');
  page.replaceChildren(pageHead('Something went quiet', 'This view could not load', error.message || 'Please try again.'),
    cardSection('Connection interrupted', '', emptyState('Try the request again', 'The gateway may be restarting or your session may need refreshing.', 'alert'), button('Retry', { kind: 'primary', onClick: () => navigate(state.route, false) })));
}

window.addEventListener('popstate', () => navigate(location.hash.slice(1) || 'overview', false));

async function renderOverview() {
  const { data } = await api('/api/dashboard');
  state.dashboard = data;
  const stats = data?.stats || {};
  const page = document.getElementById('page');
  const recent = data?.recent || [];
  const providers = data?.providers || [];
  page.replaceChildren(
    pageHead('Command centre', `Good ${greeting()}, ${firstName()}`, 'A clear view of every request, provider, and result moving through your speech gateway.', [
      button('Open studio', { kind: 'primary', icon: 'mic', onClick: () => navigate('studio') }),
    ]),
    el('div', { class: 'grid stats-grid' },
      statCard('Total requests', formatNumber(stats.requests), 'All speech activity', 'activity'),
      statCard('Average latency', stats.avg_latency_ms != null ? `${formatNumber(stats.avg_latency_ms)} ms` : '—', 'Across completed requests', 'clock'),
      statCard('Estimated cost', money(stats.estimated_cost_usd), 'Provider estimate', 'dollar'),
      statCard('Errors', formatNumber(stats.errors), stats.requests ? `${formatNumber((Number(stats.errors || 0) / Number(stats.requests)) * 100, 1)}% of requests` : 'No requests yet', 'alert')),
    el('div', { class: 'grid two-col section' },
      cardSection('Recent activity', 'The latest speech through the gateway', recent.length ? historyTable(recent, true) : emptyState('No speech yet', 'Your first transcription or voice generation will appear here.', 'activity'), button('View history', { small: true, onClick: () => navigate('history') })),
      cardSection('Providers', 'Live configuration status', providers.length ? providerList(providers) : emptyState('No providers found', 'Configure a provider to begin.', 'sliders'), button('Configure', { small: true, onClick: () => navigate('settings') }))),
    el('div', { class: 'grid equal-col section' },
      overviewSplitCard('Speech to text', stats.stt_requests, stats.requests, 'Transcriptions'),
      overviewSplitCard('Text to speech', stats.tts_requests, stats.requests, 'Voice generations')),
    systemFoot(data?.system));
}

function greeting() {
  const hour = new Date().getHours();
  return hour < 12 ? 'morning' : hour < 18 ? 'afternoon' : 'evening';
}

function firstName() { return (state.me?.name || 'Tushar').trim().split(/\s+/)[0]; }

function statCard(label, value, note, iconName) {
  return el('article', { class: 'card stat-card' },
    el('div', { class: 'stat-top' }, el('span', {}, label), el('span', { class: 'stat-icon' }, icon(iconName))),
    el('div', { class: 'stat-value' }, value), el('div', { class: 'stat-note' }, note));
}

function overviewSplitCard(title, value, total, label) {
  const number = Number(value || 0);
  const percentage = Number(total) ? Math.round(number / Number(total) * 100) : 0;
  return el('article', { class: 'card pad' },
    el('div', { class: 'stat-top' }, el('span', {}, title), statusBadge(`${percentage}%`, 'info')),
    el('div', { class: 'stat-value' }, formatNumber(number)),
    el('div', { class: 'stat-note' }, label));
}

function providerList(providers) {
  return el('div', { class: 'provider-list' }, providers.map((provider) => {
    const ok = provider.configured;
    const mark = provider.id === 'microsoft' ? 'M' : provider.id === 'openrouter' ? 'O' : 'G';
    return el('div', { class: 'provider-row' },
      el('span', { class: 'provider-icon' }, mark),
      el('span', {}, el('strong', {}, provider.name || titleCase(provider.id)), el('small', {}, provider.verified_at ? `Checked ${relativeTime(provider.verified_at)}` : ok ? 'Configured' : 'Needs credentials')),
      statusBadge(ok ? 'Ready' : 'Setup', ok ? 'good' : 'warn'));
  }));
}

function systemFoot(system = {}) {
  if (!system?.version && system?.uptime_seconds == null) return document.createDocumentFragment();
  return el('p', { class: 'muted', style: 'margin:20px 2px 0;font-size:10px' },
    `Gateway ${system.version || ''}${system.uptime_seconds != null ? ` · up ${formatDuration(system.uptime_seconds)}` : ''}`);
}

function titleCase(value = '') { return String(value).replace(/[-_]/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase()); }

function formatDuration(seconds) {
  const n = Number(seconds);
  if (!Number.isFinite(n)) return '—';
  if (n < 60) return `${Math.round(n)}s`;
  if (n < 3600) return `${Math.floor(n / 60)}m`;
  if (n < 86400) return `${Math.floor(n / 3600)}h`;
  return `${Math.floor(n / 86400)}d ${Math.floor((n % 86400) / 3600)}h`;
}

function historyTable(items, compact = false) {
  const table = el('table', {},
    el('thead', {}, el('tr', {},
      el('th', {}, 'Result'), el('th', {}, 'Model'), compact ? null : el('th', {}, 'Latency'), el('th', { class: 'hide-mobile' }, 'When'), el('th', { class: 'align-right' }, ''))),
    el('tbody'));
  const body = table.tBodies[0];
  items.forEach((item) => {
    const tone = item.status === 'error' || item.error ? 'bad' : item.status === 'processing' ? 'warn' : 'good';
    const text = item.text || item.error || (item.kind === 'tts' ? `${formatNumber(item.input_chars)} characters` : 'No transcript text saved');
    body.append(el('tr', {},
      el('td', { class: 'main-cell' }, el('strong', {}, item.kind === 'tts' ? `Voice · ${item.voice || 'default'}` : text), item.kind === 'tts' ? el('span', {}, text) : null),
      el('td', {}, statusBadge(shortModel(item.model || item.provider || item.kind), tone)),
      compact ? null : el('td', { class: 'nowrap muted' }, item.latency_ms != null ? `${formatNumber(item.latency_ms)} ms` : '—'),
      el('td', { class: 'nowrap muted hide-mobile' }, relativeTime(item.created_at)),
      el('td', { class: 'align-right' }, el('div', { class: 'row-actions' },
        item.audio_available ? iconAction('play', 'Play audio', () => playHistoryAudio(item)) : null,
        iconAction('chevron', 'View details', () => showHistoryDetail(item))))));
  });
  return el('div', { class: 'table-wrap' }, table);
}

function shortModel(value = '') {
  const clean = String(value).replace(/^models\//, '');
  return clean.length > 25 ? `${clean.slice(0, 22)}…` : clean || 'Unknown';
}

function iconAction(iconName, label, action, danger = false) {
  return el('button', { type: 'button', class: `icon-action ${danger ? 'danger' : ''}`, title: label, 'aria-label': label, on: { click: action } }, icon(iconName));
}

async function renderHistory() {
  const page = document.getElementById('page');
  page.replaceChildren(
    pageHead('Archive', 'History', 'Search every transcription and generated voice that the gateway has retained.'),
    el('div', { class: 'filter-bar' }, searchBox('Search transcript or model…', loadHistorySearch), el('span', { class: 'muted', id: 'history-count', style: 'font-size:11px' })),
    el('section', { class: 'card', id: 'history-card' }, loading()));
  await loadHistorySearch('');
}

function searchBox(placeholder, onSearch) {
  let timer;
  const input = el('input', { class: 'input', type: 'search', placeholder, 'aria-label': placeholder, on: { input: (event) => {
    clearTimeout(timer);
    timer = setTimeout(() => onSearch(event.target.value.trim()), 260);
  } } });
  return el('label', { class: 'search' }, icon('search'), input);
}

async function loadHistorySearch(query) {
  const card = document.getElementById('history-card');
  if (!card) return;
  card.replaceChildren(loading());
  try {
    const { data } = await api(`/api/history?limit=100&q=${encodeURIComponent(query)}`);
    const items = data?.data || [];
    document.getElementById('history-count').textContent = `${items.length} ${items.length === 1 ? 'result' : 'results'}`;
    card.replaceChildren(items.length ? historyTable(items) : emptyState(query ? 'No matching history' : 'No history yet', query ? 'Try a broader phrase or model name.' : 'Speech results will collect here as you use the studio.', 'clock'));
  } catch (error) { card.replaceChildren(emptyState('History unavailable', error.message, 'alert')); }
}

async function fetchHistoryAudio(item) {
  const response = await authFetch(`/api/history/${encodeURIComponent(item.id)}/audio`);
  if (!response.ok) throw new Error(`Audio unavailable (${response.status}).`);
  return response.blob();
}

async function playHistoryAudio(item) {
  try {
    const blob = await fetchHistoryAudio(item);
    const url = URL.createObjectURL(blob);
    const audio = new Audio(url);
    audio.addEventListener('ended', () => URL.revokeObjectURL(url), { once: true });
    await audio.play();
  } catch (error) { toast(error.message, 'error'); }
}

function cleanupStatusMeta(status) {
  return ({
    applied: ['Applied', 'good', 'OpenRouter used your cleanup prompt and dictionary context to produce the final transcript.'],
    fallback: ['Fallback', 'warn', 'Cleanup was unavailable, so Hypnos applied the exact dictionary replacement rules instead.'],
    skipped_snippet: ['Snippet', 'info', 'Cleanup was skipped because an exact snippet expansion produced the final text.'],
    off: ['Off', '', 'Sentence cleanup was off, so Hypnos used the exact dictionary replacement rules.'],
  })[status] || [titleCase(status || 'off'), '', 'No cleanup details were recorded.'];
}

function historyDetailItem(label, value) {
  return el('div', { class: 'history-fact' }, el('span', {}, label), el('strong', {}, value == null || value === '' ? '—' : value));
}

function showHistoryDetail(item) {
  const isSTT = item.kind !== 'tts';
  const cleanup = cleanupStatusMeta(item.cleanup_status || 'off');
  const facts = [
    historyDetailItem('Provider', item.provider || '—'),
    historyDetailItem('Model', item.model || '—'),
    item.voice ? historyDetailItem('Voice', item.voice) : null,
    historyDetailItem('Created', dateTime(item.created_at)),
    historyDetailItem('Request latency', item.latency_ms != null ? `${formatNumber(item.latency_ms)} ms` : '—'),
    historyDetailItem('Audio length', item.duration_seconds != null ? formatDuration(item.duration_seconds) : '—'),
    historyDetailItem('Estimated cost', item.estimated_cost_usd != null ? money(item.estimated_cost_usd) : '—'),
    historyDetailItem('Status', titleCase(item.status || (item.error ? 'error' : 'complete'))),
  ];
  const cleanupFacts = isSTT ? [
    historyDetailItem('Cleanup model', item.cleanup_model || '—'),
    historyDetailItem('Cleanup latency', item.cleanup_latency_ms != null ? `${formatNumber(item.cleanup_latency_ms)} ms` : '—'),
    historyDetailItem('Cleanup cost', item.cleanup_cost_usd != null ? money(item.cleanup_cost_usd) : 'Unavailable'),
    item.cleanup_input_tokens != null ? historyDetailItem('Input tokens', formatNumber(item.cleanup_input_tokens)) : null,
    item.cleanup_output_tokens != null ? historyDetailItem('Output tokens', formatNumber(item.cleanup_output_tokens)) : null,
  ] : [];
  const content = el('div', { class: 'history-detail' },
    isSTT ? el('section', { class: 'transcript-compare' },
      el('div', {}, el('span', { class: 'detail-label' }, 'Final transcript'), el('p', {}, item.text || 'Transcript text was not retained.')),
      el('div', {}, el('span', { class: 'detail-label' }, 'Original transcript'), el('p', {}, item.raw_text || 'Original transcript was not retained.'))) :
      el('section', { class: 'transcript-compare single' }, el('div', {}, el('span', { class: 'detail-label' }, 'Generated speech'), el('p', {}, item.text || `${formatNumber(item.input_chars)} characters`))),
    isSTT ? el('section', { class: `cleanup-summary ${cleanup[1]}` },
      el('div', { class: 'cleanup-summary-head' }, el('span', { class: 'cleanup-icon' }, icon('activity')), el('strong', {}, 'Sentence cleanup'), statusBadge(cleanup[0], cleanup[1])),
      el('p', {}, cleanup[2]),
      item.cleanup_error ? el('p', { class: 'cleanup-error' }, item.cleanup_error) : null,
      el('div', { class: 'history-facts compact' }, cleanupFacts)) : null,
    el('section', {}, el('h3', { class: 'detail-section-title' }, 'Request details'), el('div', { class: 'history-facts' }, facts)),
    item.error ? el('div', { class: 'test-result bad' }, item.error) : null,
    item.audio_available ? el('div', { class: 'form-actions' }, button('Play audio', { icon: 'play', kind: 'primary', onClick: () => playHistoryAudio(item) })) : null);
  showModal(isSTT ? 'Transcription details' : 'Speech details', content);
}

async function renderStudio() {
  const [settingsResult, profilesResult] = await Promise.all([
    state.settings ? Promise.resolve({ data: state.settings }) : api('/api/settings'),
    api('/api/profiles'),
  ]);
  state.settings = settingsResult.data;
  state.cache.profiles = profilesResult.data?.data || [];
  drawStudio();
}

function drawStudio() {
  if (state.sttMode === 'stream' && !configuredModels('stt', { streaming: true }).length) state.sttMode = 'file';
  const page = document.getElementById('page');
  const tabs = el('div', { class: 'tabs', role: 'tablist', 'aria-label': 'Speech studio mode' },
    el('button', { class: state.studioTab === 'stt' ? 'active' : '', role: 'tab', 'aria-selected': state.studioTab === 'stt', on: { click: () => { state.studioTab = 'stt'; drawStudio(); } } }, 'Transcribe'),
    el('button', { class: state.studioTab === 'tts' ? 'active' : '', role: 'tab', 'aria-selected': state.studioTab === 'tts', on: { click: () => { state.studioTab = 'tts'; drawStudio(); } } }, 'Create voice'));
  page.replaceChildren(
    pageHead('Workbench', 'Speech studio', 'Try every configured model with your real audio and routing profiles.', [tabs]),
    state.studioTab === 'stt' ? renderSTTStudio() : renderTTSStudio());
}

function configuredModels(kind, options = {}) {
  return state.models.filter((model) => model.kind === kind && (options.all || model.configured) && (options.streaming == null || !!model.streaming === options.streaming));
}

function modelSelect(kind, id, selected, options = {}) {
  const models = configuredModels(kind, options);
  const select = el('select', { class: 'select', id, name: id });
  if (!models.length) select.append(el('option', { value: '' }, 'No configured models'));
  models.forEach((model) => select.append(el('option', { value: model.id, selected: model.id === selected }, `${model.name} · ${titleCase(model.provider)}`)));
  return select;
}

function profileSelect() {
  const select = el('select', { class: 'select', id: 'studio-profile', name: 'profile_id', on: { change: () => applyStudioProfile(select.value, true) } }, el('option', { value: '' }, 'Default routing'));
  (state.cache.profiles || []).forEach((profile) => select.append(el('option', { value: profile.id, selected: profile.id === state.settings?.default_profile_id }, profile.name)));
  return select;
}

function defaultStudioProfile() {
  return (state.cache.profiles || []).find((profile) => profile.id === state.settings?.default_profile_id) || null;
}

function applicableModel(kind, requested, streaming = null) {
  const candidates = configuredModels(kind, streaming == null ? {} : { streaming });
  return candidates.some((model) => model.id === requested) ? requested : '';
}

function applyStudioProfile(profileID, notify = false) {
  const profile = (state.cache.profiles || []).find((item) => item.id === profileID);
  const defaults = state.settings || {};
  let fellBack = false;
  if (state.studioTab === 'stt') {
    const model = document.getElementById('stt-model');
    const language = document.getElementById('studio-language');
    const wanted = profile?.stt_model || defaults.default_stt_model;
    const supported = applicableModel('stt', wanted, state.sttMode === 'stream');
    if (model) {
      if (supported && [...model.options].some((option) => option.value === supported)) model.value = supported;
      else if (wanted) fellBack = true;
    }
    if (language) language.value = profile?.language || defaults.language || 'auto';
  } else {
    const model = document.getElementById('tts-model');
    const wanted = profile?.tts_model || defaults.default_tts_model;
    const supported = applicableModel('tts', wanted);
    if (model && supported && [...model.options].some((option) => option.value === supported)) {
      model.value = supported;
      model.dispatchEvent(new Event('change'));
    } else if (wanted) fellBack = true;
    const voice = document.getElementById('tts-voice');
    const wantedVoice = profile?.voice || defaults.default_voice;
    if (voice && [...voice.options].some((option) => option.value === wantedVoice)) voice.value = wantedVoice;
    else if (wantedVoice) fellBack = true;
  }
  if (notify) toast(fellBack ? 'Profile applied with the closest configured model.' : profile ? `${profile.name} applied` : 'Workspace defaults applied');
}

function renderSTTStudio() {
  const inputPanel = el('section', { class: 'card studio-panel' },
    el('h2', {}, 'Transcribe audio'),
    el('p', {}, 'Upload a recording, capture a batch, or watch Microsoft transcribe live.'),
    el('div', { class: 'mode-choice' },
      modeOption('file', 'Upload file', 'WAV, MP3, M4A, or WebM'),
      modeOption('record', 'Record batch', 'Capture first, then transcribe'),
      modeOption('stream', 'Live stream', 'Incremental Microsoft results')),
    el('div', { id: 'stt-capture' }),
    el('div', { class: 'field-grid', style: 'margin-top:20px' },
      field('Model', modelSelect('stt', 'stt-model', selectedSTTModel())),
      field('Language', languageSelect()),
      field('Profile', profileSelect(), '', true)),
    el('div', { class: 'form-actions' }, button('Transcribe', { kind: 'accent', icon: 'arrow', disabled: true, onClick: submitCurrentSTT })));
  const resultPanel = el('section', { class: 'card result-panel' },
    el('div', { class: 'result-top' }, el('h2', {}, 'Transcript'), el('div', { class: 'inline-actions', id: 'transcript-actions' })),
    el('div', { class: 'transcript placeholder', id: 'transcript-result' }, 'Your transcript will appear here.'),
    el('div', { class: 'result-meta', id: 'transcript-meta', hidden: true }));
  queueMicrotask(renderCaptureMode);
  return el('div', { class: 'grid studio-grid' }, inputPanel, resultPanel);
}

function selectedSTTModel() {
  const profileModel = defaultStudioProfile()?.stt_model;
  if (state.sttMode === 'stream') {
    return applicableModel('stt', profileModel, true) || configuredModels('stt', { streaming: true })[0]?.id || '';
  }
  const current = applicableModel('stt', profileModel, false) || state.settings?.default_stt_model;
  const allowed = configuredModels('stt', { streaming: false });
  return allowed.some((m) => m.id === current) ? current : allowed[0]?.id || '';
}

function modeOption(value, title, help) {
  const unavailable = value === 'stream' && !configuredModels('stt', { streaming: true }).length;
  return el('label', { class: 'mode-option' },
    el('input', { type: 'radio', name: 'stt-mode', value, checked: state.sttMode === value, disabled: unavailable, on: { change: () => {
      stopAllRecording();
      state.sttMode = value;
      const model = document.getElementById('stt-model');
      if (model) model.replaceWith(modelSelect('stt', 'stt-model', selectedSTTModel(), value === 'stream' ? { streaming: true } : { streaming: false }));
      renderCaptureMode();
    } } }),
    el('span', { class: 'mode-box' }, el('strong', {}, title), el('small', {}, unavailable ? 'Set up Microsoft to enable' : help)));
}

function languageSelect(value = defaultStudioProfile()?.language || state.settings?.language || 'auto') {
  const select = el('select', { class: 'select', id: 'studio-language', name: 'language' });
  [['auto', 'Auto detect'], ['en', 'English'], ['hi', 'Hindi / Hinglish']].forEach(([id, label]) => select.append(el('option', { value: id, selected: id === value }, label)));
  return select;
}

function field(label, control, help = '', full = false) {
  const id = control.id || `field-${Math.random().toString(36).slice(2)}`;
  control.id = id;
  return el('div', { class: `field ${full ? 'full' : ''}` }, el('label', { htmlFor: id }, label), control, help ? el('small', {}, help) : null);
}

function renderCaptureMode() {
  const host = document.getElementById('stt-capture');
  if (!host) return;
  const submit = host.closest('.studio-panel')?.querySelector('.form-actions .button');
  submit.hidden = false;
  if (state.sttMode === 'file') {
    const input = el('input', { type: 'file', id: 'stt-file', accept: 'audio/*,.wav,.mp3,.m4a,.webm', on: { change: () => {
      const file = input.files?.[0];
      name.textContent = file ? file.name : 'Choose an audio file';
      sub.textContent = file ? `${formatBytes(file.size)} · ready to transcribe` : 'or drag one here';
      submit.disabled = !file;
    } } });
    const name = el('strong', {}, 'Choose an audio file');
    const sub = el('p', {}, 'or drag one here');
    const zone = el('label', { class: 'dropzone', htmlFor: 'stt-file' }, input, el('span', { class: 'dropzone-icon' }, icon('upload')), name, sub);
    ['dragenter', 'dragover'].forEach((event) => zone.addEventListener(event, (e) => { e.preventDefault(); zone.classList.add('dragging'); }));
    ['dragleave', 'drop'].forEach((event) => zone.addEventListener(event, (e) => { e.preventDefault(); zone.classList.remove('dragging'); }));
    zone.addEventListener('drop', (event) => {
      const file = event.dataTransfer?.files?.[0];
      if (!file) return;
      const transfer = new DataTransfer(); transfer.items.add(file); input.files = transfer.files; input.dispatchEvent(new Event('change'));
    });
    host.replaceChildren(zone);
    submit.disabled = true;
  } else {
    const recButton = el('button', { type: 'button', class: 'record-button', 'aria-label': state.sttMode === 'stream' ? 'Start live transcription' : 'Start recording' });
    const timer = el('span', { class: 'timer', id: 'record-timer' }, '00:00');
    const status = el('p', { id: 'record-status' }, state.sttMode === 'stream' ? 'Tap to begin live transcription' : 'Tap to start recording');
    const wave = el('div', { class: 'wave-live', id: 'live-wave', 'aria-hidden': 'true' }, ...Array.from({ length: 22 }, () => el('i')));
    recButton.addEventListener('click', () => state.sttMode === 'stream' ? toggleStream(recButton, status) : toggleBatchRecording(recButton, status, submit));
    host.replaceChildren(el('div', { class: 'record-zone' }, recButton, wave, timer, status));
    submit.hidden = state.sttMode === 'stream';
    submit.disabled = true;
  }
}

function formatBytes(bytes) {
  if (!Number.isFinite(bytes)) return '';
  if (bytes < 1024 * 1024) return `${Math.max(1, Math.round(bytes / 1024))} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

async function submitCurrentSTT(event) {
  const submit = event.currentTarget;
  const file = state.sttMode === 'record' ? state.recording?.blob : document.getElementById('stt-file')?.files?.[0];
  if (!file) return;
  submit.disabled = true;
  submit.textContent = 'Transcribing…';
  setTranscript('Listening closely…', true);
  try {
    const form = new FormData();
    form.append('file', file, file.name || `recording-${Date.now()}.webm`);
    form.append('model', document.getElementById('stt-model').value);
    form.append('language', document.getElementById('studio-language').value);
    form.append('profile_id', document.getElementById('studio-profile').value);
    form.append('response_format', 'json');
    const { data } = await api('/v1/audio/transcriptions', { method: 'POST', body: form });
    showTranscriptResult(data);
    toast('Transcription complete');
  } catch (error) {
    setTranscript(error.message, false);
    toast(error.message, 'error');
  } finally {
    submit.disabled = false;
    submit.replaceChildren(icon('arrow'), 'Transcribe');
  }
}

async function toggleBatchRecording(buttonNode, status, submit) {
  if (state.recorder?.state === 'recording') { state.recorder.stop(); return; }
  try {
    const media = await navigator.mediaDevices.getUserMedia({ audio: { echoCancellation: true, noiseSuppression: true, channelCount: 1 } });
    const chunks = [];
    const recorder = new MediaRecorder(media);
    state.recorder = recorder;
    state.recording = { media, chunks, started: Date.now() };
    recorder.ondataavailable = (event) => { if (event.data.size) chunks.push(event.data); };
    recorder.onstop = () => {
      media.getTracks().forEach((track) => track.stop());
      const blob = new Blob(chunks, { type: recorder.mimeType || 'audio/webm' });
      state.recording = { blob, duration: (Date.now() - state.recording.started) / 1000 };
      buttonNode.classList.remove('recording');
      status.textContent = `${formatDuration(state.recording.duration)} recorded · ready to transcribe`;
      submit.disabled = !blob.size;
      stopTimer();
    };
    recorder.start(250);
    buttonNode.classList.add('recording');
    status.textContent = 'Recording · tap to stop';
    submit.disabled = true;
    startTimer();
    animateWave(media);
  } catch (error) { toast(microphoneError(error), 'error'); }
}

function microphoneError(error) {
  return error?.name === 'NotAllowedError' ? 'Microphone access was denied. Allow it in your browser settings and try again.' : (error?.message || 'Could not access the microphone.');
}

function setTranscript(text, placeholder = false) {
  const result = document.getElementById('transcript-result');
  if (!result) return;
  result.className = `transcript ${placeholder ? 'placeholder' : ''}`;
  result.textContent = text;
}

function showTranscriptResult(data) {
  setTranscript(data?.text || 'No speech was detected.');
  const actions = document.getElementById('transcript-actions');
  actions.replaceChildren(button('Copy', { small: true, icon: 'copy', onClick: () => copyText(data?.text || '') }));
  const meta = document.getElementById('transcript-meta');
  meta.hidden = false;
  meta.replaceChildren(el('span', {}, shortModel(data?.model)), el('span', {}, titleCase(data?.provider || '')), data?.latency_ms != null ? el('span', {}, `${formatNumber(data.latency_ms)} ms`) : null);
}

async function copyText(text) {
  try { await navigator.clipboard.writeText(text); toast('Copied to clipboard'); }
  catch { toast('Could not copy to the clipboard', 'error'); }
}

function startTimer() {
  stopTimer();
  const start = Date.now();
  state.recordingTimer = setInterval(() => {
    const timer = document.getElementById('record-timer');
    if (!timer) return;
    const sec = Math.floor((Date.now() - start) / 1000);
    timer.textContent = `${String(Math.floor(sec / 60)).padStart(2, '0')}:${String(sec % 60).padStart(2, '0')}`;
  }, 250);
}

function stopTimer() { clearInterval(state.recordingTimer); state.recordingTimer = null; }

async function animateWave(media) {
  let context;
  try {
    context = new AudioContext();
    const analyser = context.createAnalyser();
    analyser.fftSize = 64;
    context.createMediaStreamSource(media).connect(analyser);
    const data = new Uint8Array(analyser.frequencyBinCount);
    const draw = () => {
      if (!media.active || !document.getElementById('live-wave')) { context.close(); return; }
      analyser.getByteFrequencyData(data);
      document.querySelectorAll('#live-wave i').forEach((bar, index) => { bar.style.height = `${4 + (data[index % data.length] / 255) * 24}px`; });
      requestAnimationFrame(draw);
    };
    draw();
  } catch { /* Decorative meter is allowed to fail without interrupting recording. */ }
}

async function toggleStream(buttonNode, status) {
  if (state.stream) { finishStream(); return; }
  try {
    const token = await state.clerk.session.getToken();
    const media = await navigator.mediaDevices.getUserMedia({ audio: { echoCancellation: true, noiseSuppression: true, channelCount: 1 } });
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(`${protocol}//${location.host}/v1/audio/stream`);
    ws.binaryType = 'arraybuffer';
    state.stream = { ws, media, buttonNode, status, finished: false };
    buttonNode.classList.add('recording');
    status.textContent = 'Connecting to Microsoft…';
    setTranscript('Listening…', true);
    startTimer();
    ws.onopen = () => {
      ws.send(JSON.stringify({ type: 'auth', token }));
      ws.send(JSON.stringify({ type: 'start', model: document.getElementById('stt-model').value, language: document.getElementById('studio-language').value, profile_id: document.getElementById('studio-profile').value }));
    };
    ws.onmessage = (event) => handleStreamMessage(event, state.stream);
    ws.onerror = () => streamFailure('The live connection was interrupted.');
    ws.onclose = () => { if (state.stream && !state.stream.finished) streamFailure('The live connection closed before a final transcript.'); };
  } catch (error) { state.stream = null; stopTimer(); toast(microphoneError(error), 'error'); }
}

async function handleStreamMessage(event, stream) {
  let message;
  try { message = JSON.parse(event.data); } catch { return; }
  if (message.type === 'ready') {
    stream.status.textContent = message.mode === 'batch' ? 'Recording for batch transcription · tap to finish' : 'Live transcription · tap to finish';
    await startPCMStream(stream);
    animateWave(stream.media);
  } else if (message.type === 'partial') {
    setTranscript(message.text || 'Listening…');
    document.getElementById('transcript-result')?.classList.add('partial');
  } else if (message.type === 'final') {
    stream.finished = true;
    showTranscriptResult(message);
    cleanupStream();
    toast('Live transcription complete');
  } else if (message.type === 'error') streamFailure(message.message || 'Live transcription failed.');
}

async function startPCMStream(stream) {
  if (stream.context) return;
  const context = new AudioContext();
  await context.resume();
  const source = context.createMediaStreamSource(stream.media);
  const processor = context.createScriptProcessor(2048, 1, 1);
  const mute = context.createGain();
  mute.gain.value = 0;
  processor.onaudioprocess = (event) => {
    if (stream.ws.readyState !== WebSocket.OPEN || stream.finished) return;
    const samples = event.inputBuffer.getChannelData(0);
    const pcm = downsamplePCM16(samples, context.sampleRate, 16000);
    if (pcm.byteLength) stream.ws.send(pcm.buffer);
  };
  source.connect(processor); processor.connect(mute); mute.connect(context.destination);
  Object.assign(stream, { context, source, processor, mute });
}

function downsamplePCM16(input, inputRate, outputRate) {
  if (outputRate >= inputRate) {
    const pcm = new Int16Array(input.length);
    input.forEach((sample, i) => { const x = Math.max(-1, Math.min(1, sample)); pcm[i] = x < 0 ? x * 0x8000 : x * 0x7fff; });
    return pcm;
  }
  const ratio = inputRate / outputRate;
  const length = Math.floor(input.length / ratio);
  const output = new Int16Array(length);
  for (let i = 0; i < length; i++) {
    const start = Math.floor(i * ratio);
    const end = Math.min(input.length, Math.floor((i + 1) * ratio));
    let total = 0;
    for (let j = start; j < end; j++) total += input[j];
    const sample = Math.max(-1, Math.min(1, total / Math.max(1, end - start)));
    output[i] = sample < 0 ? sample * 0x8000 : sample * 0x7fff;
  }
  return output;
}

function finishStream() {
  const stream = state.stream;
  if (!stream) return;
  stopPCMCapture(stream);
  stream.status.textContent = 'Finalising transcript…';
  stream.buttonNode.classList.remove('recording');
  if (stream.ws.readyState === WebSocket.OPEN) stream.ws.send(JSON.stringify({ type: 'commit' }));
}

function stopPCMCapture(stream) {
  stream.processor?.disconnect(); stream.source?.disconnect(); stream.mute?.disconnect();
  stream.media?.getTracks().forEach((track) => track.stop());
  stream.context?.close().catch(() => {});
  stream.processor = null; stream.source = null;
  stopTimer();
}

function cleanupStream() {
  const stream = state.stream;
  if (!stream) return;
  stopPCMCapture(stream);
  if (stream.ws.readyState === WebSocket.OPEN) stream.ws.close(1000, 'complete');
  stream.buttonNode?.classList.remove('recording');
  state.stream = null;
}

function streamFailure(message) {
  setTranscript(message);
  toast(message, 'error');
  cleanupStream();
}

function stopAllRecording() {
  if (state.recorder?.state === 'recording') state.recorder.stop();
  state.recorder = null;
  if (state.stream) { state.stream.finished = true; cleanupStream(); }
  stopTimer();
}

function renderTTSStudio() {
  const profile = defaultStudioProfile();
  const wantedModel = profile?.tts_model || state.settings?.default_tts_model;
  const defaultModel = configuredModels('tts').some((m) => m.id === wantedModel) ? wantedModel : configuredModels('tts')[0]?.id || '';
  const text = el('textarea', { class: 'tts-input', id: 'tts-input', maxlength: 4000, placeholder: 'Write what Hypnos should say…', on: { input: (e) => { document.getElementById('tts-count').textContent = `${formatNumber(e.target.value.length)} / 4,000`; } } });
  let voice = voiceSelect(profile?.voice || state.settings?.default_voice || 'troy', defaultModel);
  const ttsModel = modelSelect('tts', 'tts-model', defaultModel);
  ttsModel.addEventListener('change', () => {
    const replacement = voiceSelect('', ttsModel.value);
    voice.replaceWith(replacement);
    voice = replacement;
  });
  const panel = el('section', { class: 'card studio-panel' },
    el('h2', {}, 'Create voice'), el('p', {}, 'Turn text into natural speech with Groq or Microsoft.'), text,
    el('div', { class: 'char-count', id: 'tts-count' }, '0 / 4,000'),
    el('div', { class: 'field-grid' },
      field('Model', ttsModel),
      field('Voice', voice),
      field('Profile', profileSelect(), '', true)),
    el('div', { class: 'form-actions' }, button('Generate speech', { kind: 'accent', icon: 'activity', onClick: generateSpeech })));
  const result = el('section', { class: 'card result-panel' },
    el('div', { class: 'result-top' }, el('h2', {}, 'Voice preview'), el('div', { id: 'audio-actions', class: 'inline-actions' })),
    el('div', { class: 'audio-result', id: 'audio-result' },
      el('div', { class: 'audio-orb' }, icon('activity')),
      el('strong', {}, 'Ready when you are'),
      el('p', { class: 'muted', style: 'font-size:12px;max-width:280px;line-height:1.5' }, 'Generated audio can be played here or downloaded as a WAV file.')),
    el('div', { class: 'result-meta', id: 'audio-meta', hidden: true }));
  return el('div', { class: 'grid studio-grid' }, panel, result);
}

function voiceSelect(selected, modelID = '') {
  const select = el('select', { class: 'select', id: 'tts-voice', name: 'voice' });
  const groups = [
    ['Groq · Orpheus', [['troy', 'Troy'], ['austin', 'Austin'], ['daniel', 'Daniel'], ['diana', 'Diana'], ['hannah', 'Hannah']]],
    ['Microsoft · MAI', [['en-IN-Dhruv', 'Dhruv · Indian English'], ['hi-IN-Dhruv', 'Dhruv · Hindi'], ['en-US-Harper', 'Harper · US English']]],
  ];
  const provider = state.models.find((model) => model.id === modelID)?.provider;
  groups.filter(([label]) => !provider || (provider === 'groq' ? label.startsWith('Groq') : label.startsWith('Microsoft'))).forEach(([label, entries]) => {
    const group = el('optgroup', { label });
    entries.forEach(([id, name]) => group.append(el('option', { value: id, selected: String(selected).toLowerCase() === id.toLowerCase() }, name)));
    select.append(group);
  });
  return select;
}

async function generateSpeech(event) {
  const trigger = event.currentTarget;
  const input = document.getElementById('tts-input').value.trim();
  if (!input) { toast('Write something to turn into speech.', 'error'); return; }
  trigger.disabled = true; trigger.textContent = 'Generating…';
  try {
    const response = await authFetch('/v1/audio/speech', { method: 'POST', body: {
      model: document.getElementById('tts-model').value,
      input,
      voice: document.getElementById('tts-voice').value,
      response_format: 'wav',
      profile_id: document.getElementById('studio-profile').value,
    } });
    if (!response.ok) {
      const data = await parseResponse(response).catch(() => null);
      throw new Error(responseError(data, `Generation failed (${response.status}).`));
    }
    const blob = await response.blob();
    if (state.audioURL) URL.revokeObjectURL(state.audioURL);
    state.audioURL = URL.createObjectURL(blob);
    const audio = el('audio', { controls: true, src: state.audioURL, autoplay: true });
    document.getElementById('audio-result').replaceChildren(el('div', { class: 'audio-orb' }, icon('play')), el('strong', {}, 'Your voice is ready'), audio);
    document.getElementById('audio-actions').replaceChildren(button('Download', { small: true, icon: 'download', onClick: () => downloadBlob(state.audioURL, `hypnos-speech-${Date.now()}.wav`) }));
    const meta = document.getElementById('audio-meta');
    meta.hidden = false;
    meta.replaceChildren(el('span', {}, shortModel(document.getElementById('tts-model').value)), el('span', {}, document.getElementById('tts-voice').selectedOptions[0]?.textContent || ''), response.headers.get('X-Speech-Latency-Ms') ? el('span', {}, `${response.headers.get('X-Speech-Latency-Ms')} ms`) : null);
    toast('Speech generated');
  } catch (error) { toast(error.message, 'error'); }
  finally { trigger.disabled = false; trigger.replaceChildren(icon('activity'), 'Generate speech'); }
}

function downloadBlob(url, filename) {
  const link = el('a', { href: url, download: filename });
  document.body.append(link); link.click(); link.remove();
}

const collectionMeta = {
  dictionary: {
    kicker: 'Personal vocabulary', title: 'Dictionary', singular: 'dictionary entry',
    copy: 'Give cleanup the names and technical terms it should recognise, their preferred spelling, and useful context.',
    empty: ['Your dictionary is empty', 'Add names, aliases, and technical terms to guide cleanup across every client.'],
  },
  snippets: {
    kicker: 'Shortcuts', title: 'Snippets', singular: 'snippet',
    copy: 'Expand an exact spoken trigger into text you use often.',
    empty: ['No snippets yet', 'Create a precise trigger for an address, sign-off, or longer passage.'],
  },
  profiles: {
    kicker: 'Routing', title: 'Profiles', singular: 'profile',
    copy: 'Bundle language, models, and voice choices for each way you use speech.',
    empty: ['No profiles yet', 'Create a profile for dictation, Hinglish, meetings, or a specific device.'],
  },
};

async function renderCollection(name) {
  const endpoint = name === 'dictionary' ? 'dictionary' : name;
  const { data } = await api(`/api/${endpoint}`);
  state.cache[name] = data?.data || [];
  drawCollection(name);
}

function drawCollection(name, query = '') {
  const meta = collectionMeta[name];
  const items = state.cache[name] || [];
  const needle = query.toLowerCase();
  const filtered = needle ? items.filter((item) => Object.values(item).some((value) => String(value || '').toLowerCase().includes(needle))) : items;
  const page = document.getElementById('page');
  const add = button(`Add ${meta.singular}`, { kind: 'primary', icon: 'plus', onClick: () => openCollectionModal(name) });
  const content = filtered.length ? (name === 'profiles' ? profileCards(filtered) : collectionTable(name, filtered)) : emptyState(needle ? `No matching ${meta.title.toLowerCase()}` : meta.empty[0], needle ? 'Try another search.' : meta.empty[1], name === 'snippets' ? 'zap' : name === 'profiles' ? 'users' : 'book');
  page.replaceChildren(
    pageHead(meta.kicker, meta.title, meta.copy, [add]),
    el('div', { class: 'filter-bar' }, searchBox(`Search ${meta.title.toLowerCase()}…`, (value) => drawCollection(name, value)), el('span', { class: 'muted', style: 'font-size:11px' }, `${filtered.length} ${filtered.length === 1 ? 'item' : 'items'}`)),
    el('section', { class: 'card' }, content));
}

function collectionTable(name, items) {
  const dictionary = name === 'dictionary';
  const table = el('table', {},
    el('thead', {}, el('tr', {},
      el('th', {}, dictionary ? 'Alias or term' : 'Trigger'),
      el('th', {}, dictionary ? 'Preferred spelling' : 'Expansion'),
      dictionary ? el('th', { class: 'hide-mobile' }, 'Context notes') : null,
      el('th', { class: 'align-right' }, ''))), el('tbody'));
  items.forEach((item) => table.tBodies[0].append(el('tr', {},
    el('td', {}, dictionary ? el('strong', {}, item.word) : el('span', { class: 'trigger' }, item.trigger)),
    el('td', { class: 'main-cell' }, el('span', { style: 'color:var(--ink);margin:0' }, dictionary ? item.replacement : item.text)),
    dictionary ? el('td', { class: 'muted hide-mobile' }, item.notes || '—') : null,
    el('td', {}, el('div', { class: 'row-actions' },
      iconAction('edit', `Edit ${collectionMeta[name].singular}`, () => openCollectionModal(name, item)),
      iconAction('trash', `Delete ${collectionMeta[name].singular}`, () => deleteCollectionItem(name, item), true))))));
  return el('div', { class: 'table-wrap' }, table);
}

function profileCards(items) {
  const cards = items.map((profile) =>
    el('article', { class: 'card item-card' },
      el('div', { class: 'item-card-top' }, el('div', {}, el('h3', {}, profile.name), el('span', { class: 'badge info', style: 'margin-top:7px' }, profile.language || 'auto')), el('div', { class: 'row-actions' },
        iconAction('edit', 'Edit profile', () => openCollectionModal('profiles', profile)),
        iconAction('trash', 'Delete profile', () => deleteCollectionItem('profiles', profile), true))),
      el('p', {}, profile.description || 'No description'),
      el('div', { class: 'item-card-foot' }, el('span', {}, shortModel(profile.stt_model || 'Default STT')), el('span', {}, profile.voice || 'Default voice'))));
  return el('div', { class: 'grid item-grid', style: 'padding:18px' }, cards);
}

function openCollectionModal(name, item = null) {
  const meta = collectionMeta[name];
  const form = el('form', { on: { submit: (event) => saveCollectionItem(event, name, item) } });
  const fields = el('div', { class: 'field-grid' });
  if (name === 'dictionary') {
    fields.append(
      field('Alias or term', textInput('word', item?.word, 'e.g. tushar', true), 'What you may say, or the canonical term when spelling guidance is enough.'),
      field('Preferred spelling', textInput('replacement', item?.replacement, 'e.g. Tushar', true), 'How cleanup should write it. Using the same canonical term in both fields is valid.'),
      field('Context notes', textArea('notes', item?.notes, 'e.g. Person’s name; preserve this spelling'), 'Sent to the cleanup model to clarify names, technical terms, and ambiguous aliases.', true));
  } else if (name === 'snippets') {
    fields.append(
      field('Exact spoken trigger', textInput('trigger', item?.trigger, 'e.g. my home address', true), 'Expansion happens only when the normalised transcript matches exactly.', true),
      field('Expanded text', textArea('text', item?.text, 'The full text to insert', true), '', true));
  } else {
    fields.append(
      field('Profile name', textInput('name', item?.name, 'e.g. Hinglish notes', true)),
      field('Language', languageNamedSelect('language', item?.language || 'auto')),
      field('Description', textArea('description', item?.description, 'When should this profile be used?'), '', true),
      field('STT model', modelNamedSelect('stt_model', 'stt', item?.stt_model)),
      field('TTS model', modelNamedSelect('tts_model', 'tts', item?.tts_model)),
      field('Voice', voiceNamedSelect('voice', item?.voice || state.settings?.default_voice)));
  }
  const save = button(item ? 'Save changes' : `Create ${meta.singular}`, { type: 'submit', kind: 'primary' });
  form.append(fields, el('div', { class: 'form-actions' }, button('Cancel', { onClick: closeModal }), save));
  showModal(`${item ? 'Edit' : 'Add'} ${meta.singular}`, form);
  setTimeout(() => form.querySelector('input,textarea,select')?.focus(), 50);
}

function textInput(name, value = '', placeholder = '', required = false, type = 'text') {
  return el('input', { class: 'input', type, name, value: value || '', placeholder, required, autocomplete: 'off' });
}

function textArea(name, value = '', placeholder = '', required = false) {
  return el('textarea', { class: 'textarea', name, placeholder, required, value: value || '' });
}

function languageNamedSelect(name, value) {
  const select = languageSelect(value); select.name = name; select.removeAttribute('id'); return select;
}

function modelNamedSelect(name, kind, value) {
  const select = el('select', { class: 'select', name }, el('option', { value: '' }, 'Use workspace default'));
  configuredModels(kind, { all: true }).forEach((model) => select.append(el('option', { value: model.id, selected: model.id === value }, `${model.name}${model.configured ? '' : ' · not configured'}`)));
  return select;
}

function voiceNamedSelect(name, selected, modelID = '') { const select = voiceSelect(selected, modelID); select.name = name; select.removeAttribute('id'); return select; }

function showModal(title, content) {
  closeModal();
  const backdrop = el('div', { class: 'modal-backdrop', role: 'presentation', on: { mousedown: (event) => { if (event.target === backdrop) closeModal(); } } },
    el('section', { class: 'modal', role: 'dialog', 'aria-modal': 'true', 'aria-labelledby': 'modal-title' },
      el('div', { class: 'modal-head' }, el('h2', { id: 'modal-title' }, title), iconAction('x', 'Close', closeModal)),
      el('div', { class: 'modal-body' }, content)));
  document.body.append(backdrop);
  state.modal = backdrop;
  state.modalKey = (event) => { if (event.key === 'Escape') closeModal(); };
  document.addEventListener('keydown', state.modalKey);
}

function closeModal() {
  state.modal?.remove(); state.modal = null;
  if (state.modalKey) document.removeEventListener('keydown', state.modalKey);
  state.modalKey = null;
}

async function saveCollectionItem(event, name, item) {
  event.preventDefault();
  const form = event.currentTarget;
  const submit = form.querySelector('[type="submit"]');
  submit.disabled = true; submit.textContent = 'Saving…';
  const body = Object.fromEntries(new FormData(form));
  try {
    const endpoint = name === 'dictionary' ? 'dictionary' : name;
    await api(`/api/${endpoint}${item ? `/${encodeURIComponent(item.id)}` : ''}`, { method: item ? 'PUT' : 'POST', body });
    closeModal();
    toast(`${collectionMeta[name].singular.replace(/^./, (c) => c.toUpperCase())} saved`);
    await renderCollection(name);
  } catch (error) { toast(error.message, 'error'); submit.disabled = false; submit.textContent = item ? 'Save changes' : `Create ${collectionMeta[name].singular}`; }
}

async function deleteCollectionItem(name, item) {
  const label = item.name || item.word || item.trigger || collectionMeta[name].singular;
  if (!confirm(`Delete “${label}”? This only removes this configuration item.`)) return;
  try {
    const endpoint = name === 'dictionary' ? 'dictionary' : name;
    await api(`/api/${endpoint}/${encodeURIComponent(item.id)}`, { method: 'DELETE' });
    toast(`${titleCase(collectionMeta[name].singular)} deleted`);
    await renderCollection(name);
  } catch (error) { toast(error.message, 'error'); }
}

async function renderSettings() {
  const [{ data: providers }, { data: settings }, { data: profiles }] = await Promise.all([
    api('/api/providers'), api('/api/settings'), api('/api/profiles'),
  ]);
  state.cache.providers = providers?.data || [];
  state.settings = settings;
  state.cache.profiles = profiles?.data || [];
  const page = document.getElementById('page');
  page.replaceChildren(
    pageHead('System', 'Providers & settings', 'Connect speech providers and choose the defaults every client inherits.'),
    el('div', { class: 'grid settings-layout' },
      el('div', { class: 'settings-stack' }, state.cache.providers.map(providerCard)),
      el('div', { class: 'settings-stack' }, defaultSettingsCard(), cleanupSettingsCard())));
}

function providerCard(provider) {
  const microsoft = provider.id === 'microsoft';
  const openrouter = provider.id === 'openrouter';
  const form = el('form', { class: `provider-card card ${microsoft && !provider.configured ? 'is-optional' : ''}`, on: { submit: (event) => saveProvider(event, provider) } });
  const fields = el('div', { class: 'field-grid' },
    field('API key', el('div', { class: 'secret-row' }, textInput('api_key', '', provider.has_api_key ? 'Saved · enter to replace' : 'Paste provider API key', false, 'password')), provider.has_api_key ? 'A key is securely stored. It is never returned to this browser.' : 'Stored encrypted after saving.', true));
  if (microsoft) {
    fields.append(
      field('Azure resource endpoint', textInput('endpoint', provider.endpoint, 'https://your-resource.services.ai.azure.com'), 'The public HTTPS URL for your Azure AI resource.', true),
      field('Speech region', textInput('region', provider.region, 'southindia'), 'Azure region used for MAI Voice.'),
      field('Streaming deployment', textInput('stream_deployment', provider.stream_deployment, 'mai-transcribe-2-streaming'), 'Deployment name for live dictation.'),
      el('input', { type: 'hidden', name: 'batch_deployment', value: provider.batch_deployment || '' }));
  } else if (openrouter) {
    fields.append(
      field('API endpoint', el('input', { class: 'input', value: 'https://openrouter.ai/api/v1', readOnly: true, 'aria-readonly': 'true' }), 'OpenRouter API endpoint is managed by Hypnos.', true),
      el('input', { type: 'hidden', name: 'endpoint', value: 'https://openrouter.ai/api/v1' }));
  } else {
    fields.append(field('API endpoint', textInput('endpoint', provider.endpoint || 'https://api.groq.com/openai/v1', 'https://api.groq.com/openai/v1'), 'Groq OpenAI-compatible API base URL.', true));
  }
  const test = button('Test connection', { small: true, onClick: (event) => testProvider(provider.id, event.currentTarget, form) });
  const save = button('Save provider', { type: 'submit', kind: 'primary', small: true });
  const remove = provider.has_api_key ? button('Remove key', { kind: 'danger', small: true, onClick: () => clearProviderKey(provider) }) : null;
  form.append(
    el('div', { class: 'provider-card-head' },
      el('span', { class: 'provider-icon' }, microsoft ? 'M' : openrouter ? 'O' : 'G'),
      el('div', {}, el('h3', {}, provider.name || titleCase(provider.id)), el('p', {}, microsoft ? 'MAI transcription and voices' : openrouter ? 'Post-transcription cleanup' : 'Whisper and Orpheus')),
      statusBadge(provider.configured ? 'Configured' : microsoft ? 'Available later' : 'Setup needed', provider.configured ? 'good' : microsoft ? 'info' : 'warn')),
    el('div', { class: 'provider-card-body' }, fields,
      microsoft && !provider.configured ? el('div', { class: 'optional-note' }, 'Microsoft is optional. Groq remains active while you configure MAI later.') : null,
      provider.last_error ? el('div', { class: 'test-result bad' }, provider.last_error) : provider.verified_at ? el('div', { class: 'test-result good' }, `Verified ${dateTime(provider.verified_at)}`) : null,
      el('div', { class: 'form-actions' }, remove, test, save)));
  return form;
}

async function saveProvider(event, provider) {
  event.preventDefault();
  const form = event.currentTarget;
  const submit = form.querySelector('[type="submit"]');
  submit.disabled = true; submit.textContent = 'Saving…';
  const values = Object.fromEntries(new FormData(form));
  if (!values.api_key) delete values.api_key;
  if (provider.id !== 'microsoft') Object.assign(values, { region: '', stream_deployment: '', batch_deployment: '' });
  if (provider.id === 'openrouter') values.endpoint = 'https://openrouter.ai/api/v1';
  try {
    await api(`/api/providers/${provider.id}`, { method: 'PUT', body: values });
    toast(`${provider.name || titleCase(provider.id)} saved`);
    state.models = (await api('/api/models')).data?.data || state.models;
    await renderSettings();
  } catch (error) { toast(error.message, 'error'); submit.disabled = false; submit.textContent = 'Save provider'; }
}

async function clearProviderKey(provider) {
  if (!confirm(`Remove the saved ${provider.name || provider.id} API key? Requests using this provider will stop until a new key is saved.`)) return;
  try {
    await api(`/api/providers/${provider.id}`, { method: 'PUT', body: {
      clear_api_key: true,
      endpoint: provider.endpoint || '', region: provider.region || '',
      stream_deployment: provider.stream_deployment || '', batch_deployment: provider.batch_deployment || '',
    } });
    toast('Provider key removed');
    state.models = (await api('/api/models')).data?.data || state.models;
    await renderSettings();
  } catch (error) { toast(error.message, 'error'); }
}

async function testProvider(id, trigger, form) {
  trigger.disabled = true; trigger.textContent = 'Testing…';
  let result = form.querySelector('.test-result');
  if (!result) { result = el('div', { class: 'test-result' }); form.querySelector('.provider-card-body').insertBefore(result, form.querySelector('.form-actions')); }
  try {
    const { data } = await api(`/api/providers/${id}/test`, { method: 'POST' });
    result.className = `test-result ${data?.ok ? 'good' : 'bad'}`;
    result.textContent = data?.message || (data?.ok ? 'Connection verified.' : 'Provider could not be verified.');
  } catch (error) { result.className = 'test-result bad'; result.textContent = error.message; }
  finally { trigger.disabled = false; trigger.textContent = 'Test connection'; }
}

function defaultSettingsCard() {
  const form = el('form', { class: 'card', on: { submit: saveDefaultSettings } });
  const profile = el('select', { class: 'select', name: 'default_profile_id' }, el('option', { value: '' }, 'None'));
  (state.cache.profiles || []).forEach((item) => profile.append(el('option', { value: item.id, selected: item.id === state.settings?.default_profile_id }, item.name)));
  const stt = settingsModelSelect('default_stt_model', 'stt', state.settings?.default_stt_model);
  const tts = settingsModelSelect('default_tts_model', 'tts', state.settings?.default_tts_model);
  let voice = voiceNamedSelect('default_voice', state.settings?.default_voice || 'troy', state.settings?.default_tts_model);
  tts.addEventListener('change', () => {
    const replacement = voiceNamedSelect('default_voice', '', tts.value);
    voice.replaceWith(replacement);
    voice = replacement;
  });
  const language = languageNamedSelect('language', state.settings?.language || 'auto');
  form.append(
    el('div', { class: 'section-head' }, el('div', {}, el('h2', {}, 'Workspace defaults'), el('p', {}, 'Applied when a client does not choose a profile'))),
    el('div', { class: 'section-body' },
      el('div', { class: 'field-grid' },
        field('STT model', stt, '', true), field('TTS model', tts, '', true),
        field('Voice', voice), field('Language', language),
        field('Default profile', profile, '', true),
        el('label', { class: 'checkbox field full' },
          el('input', { type: 'checkbox', name: 'save_history_text', checked: !!state.settings?.save_history_text }),
          el('span', {}, el('strong', {}, 'Save transcript text'), el('small', {}, 'Keep transcript text in encrypted history for search and review.')))),
      el('div', { class: 'form-actions' }, button('Save defaults', { type: 'submit', kind: 'primary' }))));
  return form;
}

function settingsModelSelect(name, kind, selected) {
  const select = el('select', { class: 'select', name });
  configuredModels(kind, { all: true }).forEach((model) => select.append(el('option', { value: model.id, selected: model.id === selected }, `${model.name}${model.configured ? '' : ' · not configured'}`)));
  return select;
}

async function saveDefaultSettings(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const submit = form.querySelector('[type="submit"]');
  submit.disabled = true; submit.textContent = 'Saving…';
  const values = { ...state.settings, ...Object.fromEntries(new FormData(form)) };
  values.save_history_text = form.elements.save_history_text.checked;
  try {
    const { data } = await api('/api/settings', { method: 'PUT', body: values });
    state.settings = data;
    toast('Workspace defaults saved');
  } catch (error) { toast(error.message, 'error'); }
  finally { submit.disabled = false; submit.textContent = 'Save defaults'; }
}

function cleanupSettingsCard() {
  const provider = (state.cache.providers || []).find((item) => item.id === 'openrouter');
  const configured = !!provider?.configured;
  const form = el('form', { class: 'card cleanup-card', on: { submit: saveCleanupSettings } });
  const enabled = el('input', { type: 'checkbox', name: 'cleanup_enabled', 'aria-label': 'Enable sentence cleanup', checked: configured && !!state.settings?.cleanup_enabled, disabled: !configured });
  const model = el('input', {
    class: 'input', type: 'text', name: 'cleanup_model', value: state.settings?.cleanup_model || 'openai/gpt-6-sol',
    placeholder: 'openai/gpt-6-sol', list: 'cleanup-model-catalog', maxlength: 160, autocomplete: 'off',
  });
  const suggestions = state.models.filter((item) => item.provider === 'openrouter');
  const catalog = el('datalist', { id: 'cleanup-model-catalog' }, suggestions.map((item) => el('option', { value: item.id }, item.name || item.id)));
  const prompt = el('textarea', {
    class: 'textarea cleanup-prompt', name: 'cleanup_prompt', maxlength: 32000,
    placeholder: 'Tell the model how to clean and format transcripts while preserving the speaker’s meaning.',
    value: state.settings?.cleanup_prompt || '',
  });
  const timeout = el('input', { class: 'input', type: 'number', name: 'cleanup_timeout_seconds', min: 2, max: 30, step: 1, required: true, value: Number(state.settings?.cleanup_timeout_seconds) || 10 });
  form.append(
    el('div', { class: 'section-head cleanup-head' },
      el('div', {}, el('p', { class: 'eyebrow' }, 'After transcription'), el('h2', {}, 'Sentence cleanup'),
        el('p', {}, 'Polish punctuation, grammar, and formatting with your personal dictionary while preserving meaning and natural Hinglish.')),
      el('label', { class: `switch ${configured ? '' : 'is-disabled'}`, title: configured ? 'Enable sentence cleanup' : 'Configure OpenRouter first' }, enabled, el('span', { 'aria-hidden': 'true' })) ),
    el('div', { class: 'section-body' },
      !configured ? el('div', { class: 'cleanup-unavailable' }, icon('key'), el('div', {}, el('strong', {}, 'OpenRouter key required'), el('p', {}, 'Save and test an OpenRouter key to enable cleanup.'))) : null,
      el('div', { class: 'cleanup-disclosure' }, icon('alert'), el('p', {}, 'Transcript text and your complete saved dictionary, including notes, are sent to the selected OpenRouter model. Matching entries guide corrections without forcing unrelated changes. If cleanup is off or unavailable, exact dictionary replacements still apply; exact snippets bypass cleanup.')),
      el('div', { class: 'field-grid cleanup-fields' },
        field('Model ID', model, suggestions.length ? 'Choose a listed model or enter any valid OpenRouter model ID.' : 'Enter a valid OpenRouter model ID.', true),
        catalog,
        field('System prompt', prompt, 'Used alongside your dictionary context. Markdown paragraphs, lists, inline code, and Handy’s `${output}` reference are supported. Maximum 32 KB as UTF-8.', true),
        field('Timeout', timeout, 'Between 2 and 30 seconds.', true)),
      el('div', { class: 'form-actions' }, button('Save cleanup', { type: 'submit', kind: 'primary' }))));
  return form;
}

async function saveCleanupSettings(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const submit = form.querySelector('[type="submit"]');
  const configured = !!(state.cache.providers || []).find((item) => item.id === 'openrouter')?.configured;
  const cleanupEnabled = configured && form.elements.cleanup_enabled.checked;
  const cleanupModel = form.elements.cleanup_model.value.trim();
  const cleanupPrompt = form.elements.cleanup_prompt.value;
  if (cleanupEnabled && !cleanupModel) { toast('Choose an OpenRouter model before enabling cleanup.', 'error'); form.elements.cleanup_model.focus(); return; }
  if (cleanupEnabled && !cleanupPrompt.trim()) { toast('Add a system prompt before enabling cleanup.', 'error'); form.elements.cleanup_prompt.focus(); return; }
  if (new TextEncoder().encode(cleanupPrompt).length > 32000) { toast('The cleanup prompt must be 32 KB or smaller when encoded as UTF-8.', 'error'); form.elements.cleanup_prompt.focus(); return; }
  const values = {
    ...state.settings,
    cleanup_enabled: cleanupEnabled,
    cleanup_model: cleanupModel,
    cleanup_prompt: cleanupPrompt,
    cleanup_timeout_seconds: Number(form.elements.cleanup_timeout_seconds.value),
  };
  submit.disabled = true; submit.textContent = 'Saving…';
  try {
    const { data } = await api('/api/settings', { method: 'PUT', body: values });
    state.settings = data;
    toast('Sentence cleanup settings saved');
  } catch (error) { toast(error.message, 'error'); }
  finally { submit.disabled = false; submit.textContent = 'Save cleanup'; }
}

async function renderKeys() {
  const { data } = await api('/api/keys');
  state.cache.keys = data?.data || [];
  drawKeys();
}

function drawKeys(revealed = null) {
  const keys = state.cache.keys || [];
  const page = document.getElementById('page');
  const content = keys.length ? keysTable(keys) : emptyState('No device keys', 'Create one for a trusted Mac, iPhone, or another private speech client.', 'key');
  page.replaceChildren(
    pageHead('Access', 'Device keys', 'Give trusted capture clients speech-only access. Device keys cannot open this admin panel.', [button('Create device key', { kind: 'primary', icon: 'plus', onClick: () => openKeyModal() })]),
    revealed ? keyReveal(revealed) : document.createDocumentFragment(),
    dictionSetupCard(),
    el('section', { class: 'card section' },
      el('div', { class: 'section-head' }, el('div', {}, el('h2', {}, 'Issued keys'), el('p', {}, 'Revoked keys remain visible for audit history'))),
      el('div', { class: 'section-body' }, content)));
}

function keysTable(keys) {
  const table = el('table', {}, el('thead', {}, el('tr', {}, el('th', {}, 'Device'), el('th', {}, 'Key prefix'), el('th', { class: 'hide-mobile' }, 'Last used'), el('th', {}, 'Status'), el('th', { class: 'align-right' }, ''))), el('tbody'));
  keys.forEach((key) => table.tBodies[0].append(el('tr', {},
    el('td', { class: 'main-cell' }, el('strong', {}, key.name), el('span', {}, `Created ${dateTime(key.created_at)}`)),
    el('td', { class: 'mono' }, `${key.prefix || ''}…`),
    el('td', { class: 'muted hide-mobile' }, key.last_used_at ? relativeTime(key.last_used_at) : 'Never'),
    el('td', {}, statusBadge(key.revoked ? 'Revoked' : 'Active', key.revoked ? 'bad' : 'good')),
    el('td', {}, el('div', { class: 'row-actions' }, key.revoked ? null : iconAction('trash', 'Revoke key', () => revokeKey(key), true))))));
  return el('div', { class: 'table-wrap' }, table);
}

function dictionSetupCard() {
  const endpoint = state.config?.diction?.endpoint || '';
  const endpointRow = endpoint
    ? el('div', { class: 'diction-endpoint' }, el('code', {}, endpoint), button('Copy', { small: true, icon: 'copy', onClick: () => copyText(endpoint) }))
    : el('div', { class: 'diction-unavailable' }, 'The Diction endpoint is not available yet.');
  return el('section', { class: 'card diction-card section' },
    el('div', { class: 'diction-intro' },
      el('span', { class: 'diction-app-icon', 'aria-hidden': 'true' }, 'D'),
      el('div', {}, el('p', { class: 'eyebrow' }, 'iPhone capture'), el('h2', {}, 'Connect Diction'),
        el('p', {}, 'Use the Diction keyboard anywhere on your iPhone, with speech routed privately through Hypnos.')),
      button('Create iPhone key', { kind: 'accent', icon: 'phone', disabled: !endpoint, onClick: () => openKeyModal('Tushar’s iPhone') })),
    el('div', { class: 'diction-body' },
      el('div', { class: 'diction-steps' },
        setupStep('1', 'Connect Tailscale', 'Make sure Tailscale is connected on your iPhone before pairing.', 'wifi'),
        setupStep('2', 'Choose Self-Hosted', 'In Diction, open Preferences → Mode and select Self-Hosted.', 'sliders'),
        setupStep('3', 'Create and pair', 'Create an iPhone key above, then scan its QR in Diction → Self-Hosted → Scan to pair.', 'key')),
      el('aside', { class: 'diction-details' },
        el('span', { class: 'detail-label' }, 'Gateway endpoint'), endpointRow,
        el('div', { class: 'keyboard-note' }, icon('phone'), el('div', {}, el('strong', {}, 'Add the keyboard'),
          el('p', {}, 'iOS Settings → General → Keyboard → Keyboards → Add New Keyboard → Diction. Then allow Full Access and microphone access.'))),
        el('div', { class: 'keyboard-note' }, icon('activity'), el('div', {}, el('strong', {}, 'Sentence cleanup'),
          el('p', {}, 'Keep AI editing off in Diction. Hypnos applies the cleanup prompt and dictionary context configured here.'))))));
}

function setupStep(number, title, copy, iconName) {
  return el('div', { class: 'setup-step' },
    el('span', { class: 'setup-number' }, number),
    el('span', { class: 'setup-icon' }, icon(iconName)),
    el('div', {}, el('strong', {}, title), el('p', {}, copy)));
}

function openKeyModal(prefill = '') {
  const form = el('form', { on: { submit: createKey } },
    field('Device name', textInput('name', prefill, 'e.g. Tushar’s MacBook', true), 'Choose a name you will recognise later.', true),
    el('div', { class: 'form-actions' }, button('Cancel', { onClick: closeModal }), button('Create key', { type: 'submit', kind: 'primary' })));
  showModal('Create device key', form);
  setTimeout(() => form.elements.name.focus(), 50);
}

async function createKey(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const submit = form.querySelector('[type="submit"]'); submit.disabled = true; submit.textContent = 'Creating…';
  try {
    const { data } = await api('/api/keys', { method: 'POST', body: { name: form.elements.name.value.trim() } });
    const list = (await api('/api/keys')).data?.data || [];
    state.cache.keys = list;
    closeModal(); drawKeys(data); toast('Device key created');
  } catch (error) { toast(error.message, 'error'); submit.disabled = false; submit.textContent = 'Create key'; }
}

function keyReveal(key) {
  const pairingURI = typeof key.diction?.pairing_uri === 'string' && key.diction.pairing_uri.startsWith('diction://pair?') ? key.diction.pairing_uri : '';
  const qrDataURL = typeof key.diction?.qr_data_url === 'string' && key.diction.qr_data_url.startsWith('data:image/png;base64,') ? key.diction.qr_data_url : '';
  const pairing = pairingURI || qrDataURL ? el('div', { class: 'pairing-layout' },
    qrDataURL ? el('div', { class: 'pairing-qr' }, el('img', { src: qrDataURL, alt: 'Diction pairing QR code' }), el('small', {}, 'Scan in Diction on another device')) : null,
    el('div', { class: 'pairing-actions' },
      el('strong', {}, 'Pair Diction now'),
      el('p', {}, 'On this iPhone, open Diction directly. On another device, scan the QR in Diction → Self-Hosted → Scan to pair.'),
      pairingURI ? el('a', { class: 'button accent', href: pairingURI }, icon('external'), 'Open Diction on this iPhone') : null)) : null;
  return el('section', { class: 'key-reveal section' },
    el('div', { class: 'reveal-heading' }, el('span', { class: 'reveal-check' }, icon('check')), el('div', {},
      el('h3', {}, `${key.name} is ready`),
      el('p', {}, 'Pair or copy this key now. For your security, the complete key is shown only once.'))),
    pairing,
    el('span', { class: 'detail-label' }, 'Manual device key'),
    el('div', { class: 'token' }, el('code', {}, key.token), button('Copy', { small: true, icon: 'copy', onClick: () => copyText(key.token) })));
}

async function revokeKey(key) {
  if (!confirm(`Revoke the key for “${key.name}”? That device will immediately lose speech access.`)) return;
  try {
    await api(`/api/keys/${encodeURIComponent(key.id)}`, { method: 'DELETE' });
    toast('Device key revoked');
    await renderKeys();
  } catch (error) { toast(error.message, 'error'); }
}

boot();
