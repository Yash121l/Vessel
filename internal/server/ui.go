package server

// uiHTML is the embedded single-page application.
// In production builds, this is replaced by the compiled frontend via go:embed.
const uiHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Vessel</title>
<style>
  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
  :root {
    --bg: #0f1117;
    --surface: #1a1d27;
    --surface2: #22263a;
    --border: #2e3347;
    --text: #e2e8f0;
    --text-muted: #64748b;
    --accent: #6366f1;
    --accent-hover: #818cf8;
    --green: #22c55e;
    --red: #ef4444;
    --yellow: #f59e0b;
    --blue: #3b82f6;
    --radius: 8px;
    --font: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
  }
  body { background: var(--bg); color: var(--text); font-family: var(--font); font-size: 14px; line-height: 1.5; }
  a { color: var(--accent); text-decoration: none; }
  button { cursor: pointer; font-family: var(--font); font-size: 14px; border: none; border-radius: var(--radius); padding: 8px 16px; transition: all 0.15s; }
  .btn-primary { background: var(--accent); color: #fff; }
  .btn-primary:hover { background: var(--accent-hover); }
  .btn-ghost { background: transparent; color: var(--text-muted); border: 1px solid var(--border); }
  .btn-ghost:hover { color: var(--text); border-color: var(--text-muted); }
  .btn-danger { background: transparent; color: var(--red); border: 1px solid var(--red); }
  .btn-danger:hover { background: var(--red); color: #fff; }
  .btn-sm { padding: 4px 10px; font-size: 12px; }
  input, select, textarea {
    background: var(--surface2); border: 1px solid var(--border); border-radius: var(--radius);
    color: var(--text); font-family: var(--font); font-size: 14px; padding: 8px 12px; width: 100%;
    outline: none; transition: border-color 0.15s;
  }
  input:focus, select:focus, textarea:focus { border-color: var(--accent); }
  label { display: block; font-size: 12px; color: var(--text-muted); margin-bottom: 4px; font-weight: 500; text-transform: uppercase; letter-spacing: 0.05em; }
  .form-group { margin-bottom: 16px; }
</style>
</head>
<body>
<div id="app"></div>
<script>
// ── State ──────────────────────────────────────────────────────────────────
const API = '/api/v1';
let state = { page: 'deployments', deployments: [], apps: [], logs: '', logsId: null, logsEs: null, deploying: false, error: null };

function setState(patch) {
  Object.assign(state, patch);
  render();
}

// ── API helpers ────────────────────────────────────────────────────────────
async function api(method, path, body) {
  const opts = { method, headers: { 'Content-Type': 'application/json' } };
  if (body) opts.body = JSON.stringify(body);
  const r = await fetch(API + path, opts);
  const data = await r.json();
  if (!r.ok) throw new Error(data.error || 'Request failed');
  return data;
}

async function loadDeployments() {
  const data = await api('GET', '/deployments');
  setState({ deployments: data.deployments || [] });
}

async function loadApps() {
  const data = await api('GET', '/apps');
  setState({ apps: data.apps || [] });
}

// ── Actions ────────────────────────────────────────────────────────────────
async function action(id, act) {
  try {
    await api('POST', '/deployments/' + id + '/' + act);
    await loadDeployments();
  } catch(e) { setState({ error: e.message }); }
}

async function remove(id, name) {
  if (!confirm('Remove deployment "' + name + '"? This will delete all containers and volumes.')) return;
  try {
    await api('DELETE', '/deployments/' + id);
    await loadDeployments();
  } catch(e) { setState({ error: e.message }); }
}

function openLogs(id) {
  if (state.logsEs) state.logsEs.close();
  setState({ page: 'logs', logsId: id, logs: '' });
  const es = new EventSource(API + '/deployments/' + id + '/logs');
  es.onmessage = e => {
    state.logs += e.data + '\n';
    const el = document.getElementById('log-output');
    if (el) { el.textContent = state.logs; el.scrollTop = el.scrollHeight; }
  };
  es.onerror = () => es.close();
  state.logsEs = es;
}

async function deploy(e) {
  e.preventDefault();
  const form = e.target;
  const envRaw = form.env.value.trim();
  const env = {};
  if (envRaw) {
    envRaw.split('\n').forEach(line => {
      const idx = line.indexOf('=');
      if (idx > 0) env[line.slice(0,idx).trim()] = line.slice(idx+1).trim();
    });
  }
  setState({ deploying: true, error: null });
  try {
    await api('POST', '/deployments', {
      app_id: form.app_id.value,
      name: form.name.value,
      domain: form.domain.value,
      env
    });
    await loadDeployments();
    setState({ page: 'deployments', deploying: false });
  } catch(e) {
    setState({ deploying: false, error: e.message });
  }
}

// ── Status badge ───────────────────────────────────────────────────────────
function statusBadge(status) {
  const colors = { running: '#22c55e', stopped: '#64748b', error: '#ef4444', deploying: '#f59e0b', updating: '#3b82f6' };
  const color = colors[status] || '#64748b';
  return '<span style="display:inline-flex;align-items:center;gap:5px;font-size:12px;font-weight:500;color:' + color + '">' +
    '<span style="width:7px;height:7px;border-radius:50%;background:' + color + ';' + (status==='running'?'box-shadow:0 0 0 2px '+color+'33':'') + '"></span>' +
    status + '</span>';
}

// ── App icon map ───────────────────────────────────────────────────────────
function appIcon(appId) {
  const icons = { metabase:'📊', n8n:'🔄', umami:'📈', plausible:'📉', 'open-webui':'🤖', plane:'✈️' };
  return icons[appId] || '📦';
}

// ── Render ─────────────────────────────────────────────────────────────────
function render() {
  const app = document.getElementById('app');
  app.innerHTML = layout();
}

function layout() {
  return '<div style="display:flex;min-height:100vh">' +
    sidebar() +
    '<main style="flex:1;padding:32px;max-width:960px">' +
      (state.error ? errorBanner() : '') +
      page() +
    '</main>' +
  '</div>';
}

function sidebar() {
  const nav = [
    { id: 'deployments', label: 'Installed Apps', icon: '🚀' },
    { id: 'deploy', label: 'Deploy New App', icon: '➕' },
    { id: 'settings', label: 'Settings', icon: '⚙️' },
  ];
  return '<nav style="width:220px;background:var(--surface);border-right:1px solid var(--border);padding:24px 0;display:flex;flex-direction:column">' +
    '<div style="padding:0 20px 24px;border-bottom:1px solid var(--border);margin-bottom:16px">' +
      '<div style="font-size:18px;font-weight:700;color:var(--text)">⚓ Vessel</div>' +
      '<div style="font-size:11px;color:var(--text-muted);margin-top:2px">Self-hosted app manager</div>' +
    '</div>' +
    nav.map(n => {
      const active = state.page === n.id;
      return '<a href="#" onclick="navigate(\'' + n.id + '\');return false" style="display:flex;align-items:center;gap:10px;padding:10px 20px;color:' +
        (active ? 'var(--text)' : 'var(--text-muted)') +
        ';background:' + (active ? 'var(--surface2)' : 'transparent') +
        ';border-left:2px solid ' + (active ? 'var(--accent)' : 'transparent') +
        ';font-weight:' + (active ? '500' : '400') + ';transition:all 0.15s">' +
        '<span>' + n.icon + '</span><span>' + n.label + '</span></a>';
    }).join('') +
    '<div style="flex:1"></div>' +
    '<div style="padding:16px 20px;border-top:1px solid var(--border);font-size:11px;color:var(--text-muted)">v0.1.0</div>' +
  '</nav>';
}

function errorBanner() {
  return '<div style="background:#ef444420;border:1px solid #ef4444;border-radius:var(--radius);padding:12px 16px;margin-bottom:20px;display:flex;justify-content:space-between;align-items:center">' +
    '<span style="color:#ef4444">' + state.error + '</span>' +
    '<button class="btn-ghost btn-sm" onclick="setState({error:null})">✕</button>' +
  '</div>';
}

function page() {
  if (state.page === 'deployments') return deploymentsPage();
  if (state.page === 'deploy') return deployPage();
  if (state.page === 'logs') return logsPage();
  if (state.page === 'settings') return settingsPage();
  return '';
}

function navigate(p) {
  if (state.logsEs && p !== 'logs') { state.logsEs.close(); state.logsEs = null; }
  setState({ page: p, error: null });
  if (p === 'deployments') loadDeployments();
  if (p === 'deploy') loadApps();
}

// ── Pages ──────────────────────────────────────────────────────────────────
function deploymentsPage() {
  const ds = state.deployments;
  return '<div>' +
    '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:24px">' +
      '<div><h1 style="font-size:20px;font-weight:600">Installed Apps</h1>' +
      '<p style="color:var(--text-muted);margin-top:2px">' + ds.length + ' deployment' + (ds.length!==1?'s':'') + '</p></div>' +
      '<button class="btn-primary" onclick="navigate(\'deploy\')">+ Deploy App</button>' +
    '</div>' +
    (ds.length === 0 ? emptyState() : ds.map(deploymentCard).join('')) +
  '</div>';
}

function emptyState() {
  return '<div style="text-align:center;padding:80px 20px;color:var(--text-muted)">' +
    '<div style="font-size:48px;margin-bottom:16px">🚀</div>' +
    '<div style="font-size:16px;font-weight:500;color:var(--text);margin-bottom:8px">No apps deployed yet</div>' +
    '<div style="margin-bottom:24px">Deploy your first self-hosted app in seconds</div>' +
    '<button class="btn-primary" onclick="navigate(\'deploy\')">Deploy your first app</button>' +
  '</div>';
}

function deploymentCard(d) {
  const isRunning = d.status === 'running';
  return '<div style="background:var(--surface);border:1px solid var(--border);border-radius:var(--radius);padding:20px;margin-bottom:12px">' +
    '<div style="display:flex;justify-content:space-between;align-items:flex-start">' +
      '<div style="display:flex;align-items:center;gap:12px">' +
        '<div style="font-size:28px">' + appIcon(d.app_id) + '</div>' +
        '<div>' +
          '<div style="font-weight:600;font-size:15px">' + d.name + '</div>' +
          '<div style="color:var(--text-muted);font-size:12px;margin-top:2px">' + d.app_id +
            (d.domain ? ' · <a href="https://' + d.domain + '" target="_blank" style="color:var(--accent)">' + d.domain + '</a>' : '') +
          '</div>' +
        '</div>' +
      '</div>' +
      '<div style="display:flex;align-items:center;gap:8px">' +
        statusBadge(d.status) +
      '</div>' +
    '</div>' +
    '<div style="display:flex;gap:8px;margin-top:16px;padding-top:16px;border-top:1px solid var(--border)">' +
      (isRunning
        ? '<button class="btn-ghost btn-sm" onclick="action(\'' + d.id + '\',\'stop\')">⏹ Stop</button>'
        : '<button class="btn-ghost btn-sm" onclick="action(\'' + d.id + '\',\'start\')">▶ Start</button>') +
      '<button class="btn-ghost btn-sm" onclick="action(\'' + d.id + '\',\'restart\')">↺ Restart</button>' +
      '<button class="btn-ghost btn-sm" onclick="action(\'' + d.id + '\',\'update\')">⬆ Update</button>' +
      '<button class="btn-ghost btn-sm" onclick="openLogs(\'' + d.id + '\')">📋 Logs</button>' +
      '<div style="flex:1"></div>' +
      '<button class="btn-danger btn-sm" onclick="remove(\'' + d.id + '\',\'' + d.name + '\')">Remove</button>' +
    '</div>' +
  '</div>';
}

function logsPage() {
  const d = state.deployments.find(x => x.id === state.logsId);
  return '<div>' +
    '<div style="display:flex;align-items:center;gap:12px;margin-bottom:24px">' +
      '<button class="btn-ghost btn-sm" onclick="navigate(\'deployments\')">← Back</button>' +
      '<h1 style="font-size:20px;font-weight:600">Logs' + (d ? ': ' + d.name : '') + '</h1>' +
    '</div>' +
    '<div style="background:var(--surface);border:1px solid var(--border);border-radius:var(--radius);overflow:hidden">' +
      '<div style="padding:12px 16px;border-bottom:1px solid var(--border);display:flex;justify-content:space-between;align-items:center">' +
        '<span style="font-size:12px;color:var(--text-muted)">Live log stream</span>' +
        '<span style="font-size:11px;color:var(--green)">● streaming</span>' +
      '</div>' +
      '<pre id="log-output" style="padding:16px;font-size:12px;font-family:monospace;height:500px;overflow-y:auto;white-space:pre-wrap;word-break:break-all;color:var(--text)">' +
        (state.logs || 'Connecting to log stream...') +
      '</pre>' +
    '</div>' +
  '</div>';
}

function deployPage() {
  const apps = state.apps;
  return '<div>' +
    '<div style="margin-bottom:24px">' +
      '<h1 style="font-size:20px;font-weight:600">Deploy New App</h1>' +
      '<p style="color:var(--text-muted);margin-top:2px">Choose from curated self-hosted applications</p>' +
    '</div>' +
    '<form onsubmit="deploy(event)">' +
      '<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(160px,1fr));gap:12px;margin-bottom:24px">' +
        apps.map(a =>
          '<label style="cursor:pointer">' +
            '<input type="radio" name="app_id" value="' + a.id + '" style="display:none" onchange="selectApp(\'' + a.id + '\')" required>' +
            '<div id="app-card-' + a.id + '" style="background:var(--surface);border:2px solid var(--border);border-radius:var(--radius);padding:16px;text-align:center;transition:all 0.15s">' +
              '<div style="font-size:32px;margin-bottom:8px">' + (a.icon||'📦') + '</div>' +
              '<div style="font-weight:600;font-size:13px">' + a.name + '</div>' +
              '<div style="font-size:11px;color:var(--text-muted);margin-top:4px">' + (a.category||'') + '</div>' +
            '</div>' +
          '</label>'
        ).join('') +
      '</div>' +
      '<div style="background:var(--surface);border:1px solid var(--border);border-radius:var(--radius);padding:24px">' +
        '<div style="display:grid;grid-template-columns:1fr 1fr;gap:16px">' +
          '<div class="form-group">' +
            '<label>Deployment Name *</label>' +
            '<input name="name" placeholder="my-metabase" pattern="[a-z0-9-]+" title="Lowercase letters, numbers, hyphens only" required>' +
          '</div>' +
          '<div class="form-group">' +
            '<label>Custom Domain (optional)</label>' +
            '<input name="domain" placeholder="analytics.example.com">' +
          '</div>' +
        '</div>' +
        '<div class="form-group">' +
          '<label>Environment Variables (KEY=VALUE, one per line)</label>' +
          '<textarea name="env" rows="6" placeholder="SECRET_KEY=your-secret-key\nADMIN_EMAIL=admin@example.com" style="font-family:monospace;font-size:13px"></textarea>' +
        '</div>' +
        '<div id="app-env-hints" style="margin-bottom:16px"></div>' +
        '<div style="display:flex;gap:12px;justify-content:flex-end">' +
          '<button type="button" class="btn-ghost" onclick="navigate(\'deployments\')">Cancel</button>' +
          '<button type="submit" class="btn-primary" ' + (state.deploying ? 'disabled' : '') + '>' +
            (state.deploying ? '⏳ Deploying...' : '🚀 Deploy') +
          '</button>' +
        '</div>' +
      '</div>' +
    '</form>' +
  '</div>';
}

function selectApp(id) {
  // Highlight selected card
  state.apps.forEach(a => {
    const el = document.getElementById('app-card-' + a.id);
    if (el) el.style.borderColor = a.id === id ? 'var(--accent)' : 'var(--border)';
  });
  // Show env hints
  const app = state.apps.find(a => a.id === id);
  if (!app || !app.env_vars) return;
  const required = app.env_vars.filter(e => e.required);
  if (required.length === 0) {
    document.getElementById('app-env-hints').innerHTML = '';
    return;
  }
  document.getElementById('app-env-hints').innerHTML =
    '<div style="background:var(--surface2);border-radius:var(--radius);padding:12px 16px">' +
    '<div style="font-size:12px;color:var(--text-muted);margin-bottom:8px;font-weight:500;text-transform:uppercase;letter-spacing:0.05em">Required variables</div>' +
    required.map(e =>
      '<div style="display:flex;gap:8px;margin-bottom:4px;font-size:12px">' +
        '<code style="color:var(--accent);min-width:180px">' + e.key + '</code>' +
        '<span style="color:var(--text-muted)">' + e.description + '</span>' +
      '</div>'
    ).join('') +
    '</div>';
}

function settingsPage() {
  return '<div>' +
    '<div style="margin-bottom:24px">' +
      '<h1 style="font-size:20px;font-weight:600">Settings</h1>' +
      '<p style="color:var(--text-muted);margin-top:2px">Vessel configuration</p>' +
    '</div>' +
    '<div style="background:var(--surface);border:1px solid var(--border);border-radius:var(--radius);padding:24px;max-width:480px">' +
      '<div style="margin-bottom:20px">' +
        '<div style="font-size:12px;color:var(--text-muted);margin-bottom:4px;text-transform:uppercase;letter-spacing:0.05em">Version</div>' +
        '<div>0.1.0</div>' +
      '</div>' +
      '<div style="margin-bottom:20px">' +
        '<div style="font-size:12px;color:var(--text-muted);margin-bottom:4px;text-transform:uppercase;letter-spacing:0.05em">Data Directory</div>' +
        '<code style="font-size:13px;color:var(--accent)">/var/lib/vessel</code>' +
      '</div>' +
      '<div style="margin-bottom:20px">' +
        '<div style="font-size:12px;color:var(--text-muted);margin-bottom:4px;text-transform:uppercase;letter-spacing:0.05em">Documentation</div>' +
        '<a href="https://github.com/vessel-app/vessel" target="_blank">github.com/vessel-app/vessel</a>' +
      '</div>' +
    '</div>' +
  '</div>';
}

// ── Boot ───────────────────────────────────────────────────────────────────
render();
loadDeployments();
</script>
</body>
</html>
`
