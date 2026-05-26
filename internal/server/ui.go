package server

const uiHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Vessel</title>
<style>
*,*::before,*::after{box-sizing:border-box;margin:0;padding:0}
:root{
  --bg:#0a0b0f;--surface:#111318;--surface2:#1a1d27;--surface3:#22263a;
  --border:#1e2235;--border2:#2a2f47;
  --text:#e2e8f0;--muted:#64748b;--muted2:#94a3b8;
  --accent:#6366f1;--accent2:#818cf8;--accent-bg:#6366f115;
  --green:#22c55e;--green-bg:#22c55e15;
  --red:#ef4444;--red-bg:#ef444415;
  --yellow:#f59e0b;--yellow-bg:#f59e0b15;
  --blue:#3b82f6;--blue-bg:#3b82f615;
  --r:8px;--r2:12px;
  --font:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;
  --mono:'SF Mono','Fira Code',monospace;
}
body{background:var(--bg);color:var(--text);font-family:var(--font);font-size:14px;line-height:1.6;min-height:100vh}
a{color:var(--accent);text-decoration:none}
a:hover{color:var(--accent2)}
button{cursor:pointer;font-family:var(--font);font-size:13px;border:none;border-radius:var(--r);padding:7px 14px;transition:all .15s;font-weight:500}
.btn{background:var(--surface2);color:var(--muted2);border:1px solid var(--border2)}
.btn:hover{color:var(--text);border-color:var(--muted)}
.btn-primary{background:var(--accent);color:#fff;border:1px solid transparent}
.btn-primary:hover{background:var(--accent2)}
.btn-danger{background:transparent;color:var(--red);border:1px solid var(--red-bg)}
.btn-danger:hover{background:var(--red-bg)}
.btn-sm{padding:4px 10px;font-size:12px}
.btn-xs{padding:3px 8px;font-size:11px}
input,select,textarea{background:var(--surface2);border:1px solid var(--border2);border-radius:var(--r);color:var(--text);font-family:var(--font);font-size:13px;padding:8px 12px;width:100%;outline:none;transition:border-color .15s}
input:focus,select:focus,textarea:focus{border-color:var(--accent)}
label{display:block;font-size:11px;color:var(--muted);margin-bottom:5px;font-weight:600;text-transform:uppercase;letter-spacing:.06em}
.fg{margin-bottom:16px}
.grid2{display:grid;grid-template-columns:1fr 1fr;gap:16px}
.card{background:var(--surface);border:1px solid var(--border);border-radius:var(--r2);padding:20px}
.card:hover{border-color:var(--border2)}
.tag{display:inline-flex;align-items:center;gap:4px;font-size:11px;font-weight:600;padding:2px 8px;border-radius:20px;text-transform:uppercase;letter-spacing:.04em}
.tag-running{background:var(--green-bg);color:var(--green)}
.tag-stopped{background:var(--surface3);color:var(--muted)}
.tag-error{background:var(--red-bg);color:var(--red)}
.tag-deploying,.tag-updating{background:var(--yellow-bg);color:var(--yellow)}
.tag-imported{background:var(--blue-bg);color:var(--blue)}
.dot{width:6px;height:6px;border-radius:50%;background:currentColor;display:inline-block}
.dot-pulse{animation:pulse 2s infinite}
@keyframes pulse{0%,100%{opacity:1}50%{opacity:.4}}
</style>
</head>
<body>
<div id="app"></div>
<script>
const API='/api/v1';
let S={
  page:'containers',
  deployments:[],apps:[],containers:[],
  logs:'',logsTarget:null,logsType:null,logsEs:null,
  deploying:false,error:null,loading:false,
  selectedApp:null
};

function set(patch){Object.assign(S,patch);render()}

// ── API ──────────────────────────────────────────────────────────────────────
async function api(method,path,body){
  const o={method,headers:{'Content-Type':'application/json'}};
  if(body)o.body=JSON.stringify(body);
  const r=await fetch(API+path,o);
  const d=await r.json();
  if(!r.ok)throw new Error(d.error||'Request failed');
  return d;
}
async function load(){
  try{
    const[dc,dd]=await Promise.all([
      api('GET','/docker/containers'),
      api('GET','/deployments')
    ]);
    set({containers:dc.containers||[],deployments:dd.deployments||[],loading:false});
  }catch(e){set({error:e.message,loading:false})}
}
async function loadApps(){
  const d=await api('GET','/apps');
  set({apps:d.apps||[]});
}

// ── Actions ───────────────────────────────────────────────────────────────────
async function act(id,action){
  try{await api('POST','/deployments/'+id+'/'+action);await load()}
  catch(e){set({error:e.message})}
}
async function actContainer(id,action){
  try{await api('POST','/docker/containers/'+id+'/'+action);await load()}
  catch(e){set({error:e.message})}
}
async function importContainer(id,name){
  try{
    await api('POST','/docker/import',{container_id:id,name});
    await load();
  }catch(e){set({error:e.message})}
}
async function remove(id,name){
  if(!confirm('Remove "'+name+'"? Containers and volumes will be deleted.'))return;
  try{await api('DELETE','/deployments/'+id);await load()}
  catch(e){set({error:e.message})}
}
async function deploy(e){
  e.preventDefault();
  const f=e.target;
  const env={};
  (f.env.value||'').trim().split('\n').forEach(l=>{
    const i=l.indexOf('=');
    if(i>0)env[l.slice(0,i).trim()]=l.slice(i+1).trim();
  });
  set({deploying:true,error:null});
  try{
    await api('POST','/deployments',{app_id:f.app_id.value,name:f.dname.value,domain:f.domain.value,env});
    await load();
    set({page:'containers',deploying:false});
  }catch(e){set({deploying:false,error:e.message})}
}

// ── Logs ──────────────────────────────────────────────────────────────────────
function openLogs(id,type,name){
  if(S.logsEs)S.logsEs.close();
  const path=type==='container'
    ?'/docker/containers/'+id+'/logs'
    :'/deployments/'+id+'/logs';
  set({page:'logs',logsTarget:{id,name,type},logs:''});
  const es=new EventSource(API+path);
  es.onmessage=e=>{
    S.logs+=e.data+'\n';
    const el=document.getElementById('logbox');
    if(el){el.textContent=S.logs;el.scrollTop=el.scrollHeight}
  };
  es.onerror=()=>es.close();
  S.logsEs=es;
}

// ── Nav ───────────────────────────────────────────────────────────────────────
function nav(p){
  if(S.logsEs&&p!=='logs'){S.logsEs.close();S.logsEs=null}
  set({page:p,error:null});
  if(p==='containers')load();
  if(p==='deploy')loadApps();
}

// ── Helpers ───────────────────────────────────────────────────────────────────
function appIcon(id){
  return{metabase:'📊',n8n:'🔄',umami:'📈',plausible:'📉','open-webui':'🤖',plane:'✈️',
    mysql:'🐬',postgres:'🐘',redis:'⚡',mongodb:'🍃',nginx:'🌐',custom:'📦'}[id]||'📦';
}
function statusTag(s){
  return'<span class="tag tag-'+s+'"><span class="dot'+(s==='running'?' dot-pulse':'')+'"></span>'+s+'</span>';
}
function ago(ts){
  if(!ts)return'';
  const d=new Date(ts),now=new Date(),diff=Math.floor((now-d)/1000);
  if(diff<60)return diff+'s ago';
  if(diff<3600)return Math.floor(diff/60)+'m ago';
  if(diff<86400)return Math.floor(diff/3600)+'h ago';
  return Math.floor(diff/86400)+'d ago';
}
</script>
`
<parameter name="text">
<script>
// ── Layout ────────────────────────────────────────────────────────────────────
function render(){
  document.getElementById('app').innerHTML=
    '<div style="display:flex;min-height:100vh">'+sidebar()+'<main style="flex:1;overflow:auto">'+
    '<div style="max-width:1000px;margin:0 auto;padding:32px 24px">'+
    (S.error?err():'')+pages[S.page]()+'</div></main></div>';
}

function sidebar(){
  const items=[
    {id:'containers',icon:'<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="3" width="20" height="14" rx="2"/><path d="M8 21h8M12 17v4"/></svg>',label:'Containers'},
    {id:'deploy',icon:'<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="M12 8v8M8 12h8"/></svg>',label:'Deploy App'},
    {id:'settings',icon:'<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>',label:'Settings'},
  ];
  return '<nav style="width:200px;background:var(--surface);border-right:1px solid var(--border);display:flex;flex-direction:column;position:sticky;top:0;height:100vh">'+
    '<div style="padding:20px 16px 16px;border-bottom:1px solid var(--border)">'+
      '<div style="display:flex;align-items:center;gap:8px">'+
        '<span style="font-size:20px">⚓</span>'+
        '<div><div style="font-weight:700;font-size:15px;letter-spacing:-.3px">Vessel</div>'+
        '<div style="font-size:10px;color:var(--muted);margin-top:1px">v0.1.0</div></div>'+
      '</div>'+
    '</div>'+
    '<div style="padding:8px">'+
    items.map(i=>{
      const a=S.page===i.id;
      return '<a href="#" onclick="nav(\''+i.id+'\');return false" style="display:flex;align-items:center;gap:10px;padding:9px 12px;border-radius:var(--r);color:'+(a?'var(--text)':'var(--muted)')+';background:'+(a?'var(--surface2)':'transparent')+';font-weight:'+(a?'500':'400')+';margin-bottom:2px;transition:all .15s">'+
        '<span style="color:'+(a?'var(--accent)':'var(--muted)')+'">'+i.icon+'</span>'+i.label+'</a>';
    }).join('')+
    '</div>'+
    '<div style="flex:1"></div>'+
    '<div style="padding:16px;border-top:1px solid var(--border)">'+
      '<div style="font-size:11px;color:var(--muted)">'+
        '<a href="https://github.com/Yash121l/Vessel" target="_blank" style="color:var(--muted)">GitHub</a>'+
      '</div>'+
    '</div>'+
  '</nav>';
}

function err(){
  return '<div style="background:var(--red-bg);border:1px solid #ef444430;border-radius:var(--r);padding:12px 16px;margin-bottom:20px;display:flex;justify-content:space-between;align-items:center">'+
    '<span style="color:var(--red);font-size:13px">'+S.error+'</span>'+
    '<button class="btn btn-sm" onclick="set({error:null})">✕</button>'+
  '</div>';
}

// ── Pages ─────────────────────────────────────────────────────────────────────
const pages={
  containers: containersPage,
  deploy: deployPage,
  logs: logsPage,
  settings: settingsPage,
};

function containersPage(){
  const managed=S.deployments.filter(d=>!d.imported);
  const imported=S.deployments.filter(d=>d.imported);
  // Containers not yet tracked by Vessel
  const trackedIds=new Set(S.deployments.map(d=>d.container_id).filter(Boolean));
  const untracked=S.containers.filter(c=>!c.managed_by_vessel&&!trackedIds.has(c.id));

  return '<div>'+
    '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:28px">'+
      '<div>'+
        '<h1 style="font-size:20px;font-weight:700;letter-spacing:-.4px">Containers</h1>'+
        '<p style="color:var(--muted);margin-top:3px;font-size:13px">'+
          (S.containers.length)+' running on this host</p>'+
      '</div>'+
      '<div style="display:flex;gap:8px">'+
        '<button class="btn btn-sm" onclick="load()">↺ Refresh</button>'+
        '<button class="btn-primary btn-sm" onclick="nav(\'deploy\')">+ Deploy App</button>'+
      '</div>'+
    '</div>'+

    // Vessel-managed
    (managed.length>0?
      '<div style="margin-bottom:32px">'+
        '<div style="font-size:11px;font-weight:700;color:var(--muted);text-transform:uppercase;letter-spacing:.08em;margin-bottom:12px">Vessel Managed</div>'+
        managed.map(managedCard).join('')+
      '</div>':'')+

    // Imported / monitored
    (imported.length>0?
      '<div style="margin-bottom:32px">'+
        '<div style="font-size:11px;font-weight:700;color:var(--muted);text-transform:uppercase;letter-spacing:.08em;margin-bottom:12px">Monitored</div>'+
        imported.map(importedCard).join('')+
      '</div>':'')+

    // Discovered but untracked
    (untracked.length>0?
      '<div>'+
        '<div style="font-size:11px;font-weight:700;color:var(--muted);text-transform:uppercase;letter-spacing:.08em;margin-bottom:12px">'+
          'Discovered — not yet monitored</div>'+
        untracked.map(discoveredCard).join('')+
      '</div>':'')+

    (S.deployments.length===0&&S.containers.length===0?emptyState():'');
}

function emptyState(){
  return '<div style="text-align:center;padding:80px 20px">'+
    '<div style="font-size:48px;margin-bottom:16px">🚀</div>'+
    '<div style="font-size:16px;font-weight:600;margin-bottom:8px">No containers yet</div>'+
    '<div style="color:var(--muted);margin-bottom:24px">Deploy your first self-hosted app</div>'+
    '<button class="btn-primary" onclick="nav(\'deploy\')">Deploy an app</button>'+
  '</div>';
}

function managedCard(d){
  const running=d.status==='running';
  return '<div class="card" style="margin-bottom:10px;transition:border-color .15s">'+
    '<div style="display:flex;align-items:center;gap:14px">'+
      '<div style="font-size:26px;width:40px;text-align:center">'+appIcon(d.app_id)+'</div>'+
      '<div style="flex:1;min-width:0">'+
        '<div style="display:flex;align-items:center;gap:10px;flex-wrap:wrap">'+
          '<span style="font-weight:600;font-size:15px">'+d.name+'</span>'+
          statusTag(d.status)+
        '</div>'+
        '<div style="color:var(--muted);font-size:12px;margin-top:3px">'+
          d.app_id+(d.domain?' · <a href="https://'+d.domain+'" target="_blank">'+d.domain+'</a>':'')+
        '</div>'+
      '</div>'+
      '<div style="display:flex;gap:6px;flex-shrink:0">'+
        (running
          ?'<button class="btn btn-sm" onclick="act(\''+d.id+'\',\'stop\')">Stop</button>'
          :'<button class="btn btn-sm" onclick="act(\''+d.id+'\',\'start\')">Start</button>')+
        '<button class="btn btn-sm" onclick="act(\''+d.id+'\',\'restart\')">Restart</button>'+
        '<button class="btn btn-sm" onclick="openLogs(\''+d.id+'\',\'deployment\',\''+d.name+'\')">Logs</button>'+
        '<button class="btn btn-sm" onclick="act(\''+d.id+'\',\'update\')">Update</button>'+
        '<button class="btn btn-danger btn-sm" onclick="remove(\''+d.id+'\',\''+d.name+'\')">Remove</button>'+
      '</div>'+
    '</div>'+
  '</div>';
}

function importedCard(d){
  const running=d.status==='running';
  const ports=d.ports?d.ports.split(', ').filter(Boolean):[];
  return '<div class="card" style="margin-bottom:10px">'+
    '<div style="display:flex;align-items:center;gap:14px">'+
      '<div style="font-size:26px;width:40px;text-align:center">'+appIcon(d.app_id)+'</div>'+
      '<div style="flex:1;min-width:0">'+
        '<div style="display:flex;align-items:center;gap:10px;flex-wrap:wrap">'+
          '<span style="font-weight:600;font-size:15px">'+d.name+'</span>'+
          statusTag(d.status)+
          '<span class="tag tag-imported">imported</span>'+
        '</div>'+
        '<div style="color:var(--muted);font-size:12px;margin-top:3px">'+
          (d.image||d.app_id)+
          (ports.length?' · '+ports.slice(0,2).join(', '):'')+
        '</div>'+
      '</div>'+
      '<div style="display:flex;gap:6px;flex-shrink:0">'+
        (running
          ?'<button class="btn btn-sm" onclick="actContainer(\''+d.container_id+'\',\'stop\')">Stop</button>'
          :'<button class="btn btn-sm" onclick="actContainer(\''+d.container_id+'\',\'start\')">Start</button>')+
        '<button class="btn btn-sm" onclick="actContainer(\''+d.container_id+'\',\'restart\')">Restart</button>'+
        '<button class="btn btn-sm" onclick="openLogs(\''+d.container_id+'\',\'container\',\''+d.name+'\')">Logs</button>'+
      '</div>'+
    '</div>'+
  '</div>';
}

function discoveredCard(c){
  const ports=c.ports&&c.ports.length?c.ports.slice(0,3).join(', '):'no ports exposed';
  return '<div class="card" style="margin-bottom:10px;border-style:dashed">'+
    '<div style="display:flex;align-items:center;gap:14px">'+
      '<div style="font-size:26px;width:40px;text-align:center">'+appIcon(c.name)+'</div>'+
      '<div style="flex:1;min-width:0">'+
        '<div style="display:flex;align-items:center;gap:10px;flex-wrap:wrap">'+
          '<span style="font-weight:600;font-size:15px">'+c.name+'</span>'+
          statusTag(c.state)+
        '</div>'+
        '<div style="color:var(--muted);font-size:12px;margin-top:3px">'+
          c.image+' · '+ports+
        '</div>'+
      '</div>'+
      '<div style="display:flex;gap:6px;flex-shrink:0">'+
        '<button class="btn btn-sm" onclick="openLogs(\''+c.id+'\',\'container\',\''+c.name+'\')">Logs</button>'+
        '<button class="btn btn-primary btn-sm" onclick="importContainer(\''+c.id+'\',\''+c.name+'\')">Monitor</button>'+
      '</div>'+
    '</div>'+
  '</div>';
}
</script>

<script>
function deployPage(){
  const apps=S.apps;
  return '<div>'+
    '<div style="margin-bottom:24px">'+
      '<h1 style="font-size:20px;font-weight:700;letter-spacing:-.4px">Deploy App</h1>'+
      '<p style="color:var(--muted);margin-top:3px;font-size:13px">Choose from curated self-hosted applications</p>'+
    '</div>'+
    '<form onsubmit="deploy(event)">'+
      '<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(140px,1fr));gap:10px;margin-bottom:24px">'+
        apps.map(a=>'<label style="cursor:pointer">'+
          '<input type="radio" name="app_id" value="'+a.id+'" style="display:none" onchange="selectApp(\''+a.id+'\')" required>'+
          '<div id="ac-'+a.id+'" style="background:var(--surface);border:2px solid var(--border);border-radius:var(--r2);padding:16px 12px;text-align:center;transition:all .15s">'+
            '<div style="font-size:28px;margin-bottom:8px">'+appIcon(a.id)+'</div>'+
            '<div style="font-weight:600;font-size:13px">'+a.name+'</div>'+
            '<div style="font-size:11px;color:var(--muted);margin-top:3px">'+a.category+'</div>'+
          '</div>'+
        '</label>').join('')+
      '</div>'+
      '<div class="card">'+
        '<div class="grid2">'+
          '<div class="fg"><label>Deployment Name *</label>'+
            '<input name="dname" placeholder="my-metabase" pattern="[a-z0-9-]+" title="Lowercase, numbers, hyphens" required></div>'+
          '<div class="fg"><label>Custom Domain (optional)</label>'+
            '<input name="domain" placeholder="app.example.com"></div>'+
        '</div>'+
        '<div class="fg"><label>Environment Variables</label>'+
          '<textarea name="env" rows="5" placeholder="SECRET_KEY=your-secret\nADMIN_EMAIL=you@example.com" style="font-family:var(--mono);font-size:12px"></textarea>'+
        '</div>'+
        '<div id="env-hints" style="margin-bottom:16px"></div>'+
        '<div style="display:flex;gap:10px;justify-content:flex-end">'+
          '<button type="button" class="btn" onclick="nav(\'containers\')">Cancel</button>'+
          '<button type="submit" class="btn-primary"'+(S.deploying?' disabled':'')+'>'+
            (S.deploying?'⏳ Deploying…':'🚀 Deploy')+'</button>'+
        '</div>'+
      '</div>'+
    '</form>'+
  '</div>';
}

function selectApp(id){
  S.apps.forEach(a=>{
    const el=document.getElementById('ac-'+a.id);
    if(el)el.style.borderColor=a.id===id?'var(--accent)':'var(--border)';
  });
  const app=S.apps.find(a=>a.id===id);
  if(!app||!app.env_vars)return;
  const req=app.env_vars.filter(e=>e.required);
  const hints=document.getElementById('env-hints');
  if(!hints)return;
  if(!req.length){hints.innerHTML='';return}
  hints.innerHTML='<div style="background:var(--surface2);border-radius:var(--r);padding:12px 14px">'+
    '<div style="font-size:11px;font-weight:700;color:var(--muted);text-transform:uppercase;letter-spacing:.06em;margin-bottom:8px">Required variables</div>'+
    req.map(e=>'<div style="display:flex;gap:12px;margin-bottom:5px;font-size:12px">'+
      '<code style="color:var(--accent);min-width:200px;font-family:var(--mono)">'+e.key+'</code>'+
      '<span style="color:var(--muted)">'+e.description+'</span>'+
    '</div>').join('')+
  '</div>';
}

function logsPage(){
  const t=S.logsTarget;
  return '<div>'+
    '<div style="display:flex;align-items:center;gap:12px;margin-bottom:24px">'+
      '<button class="btn btn-sm" onclick="nav(\'containers\')">← Back</button>'+
      '<h1 style="font-size:18px;font-weight:700">Logs'+(t?' — '+t.name:'')+'</h1>'+
      '<span style="margin-left:auto;font-size:11px;color:var(--green);display:flex;align-items:center;gap:5px">'+
        '<span class="dot dot-pulse" style="background:var(--green)"></span> live'+
      '</span>'+
    '</div>'+
    '<div style="background:var(--surface);border:1px solid var(--border);border-radius:var(--r2);overflow:hidden">'+
      '<div style="padding:10px 16px;border-bottom:1px solid var(--border);display:flex;justify-content:space-between;align-items:center">'+
        '<span style="font-size:11px;color:var(--muted);font-family:var(--mono)">stdout / stderr</span>'+
        '<button class="btn btn-xs" onclick="S.logs=\'\';const el=document.getElementById(\'logbox\');if(el)el.textContent=\'\'">Clear</button>'+
      '</div>'+
      '<pre id="logbox" style="padding:16px;font-size:12px;font-family:var(--mono);height:520px;overflow-y:auto;white-space:pre-wrap;word-break:break-all;color:#a8b4c8;line-height:1.7">'+
        (S.logs||'Connecting…')+
      '</pre>'+
    '</div>'+
  '</div>';
}

function settingsPage(){
  return '<div>'+
    '<div style="margin-bottom:24px">'+
      '<h1 style="font-size:20px;font-weight:700;letter-spacing:-.4px">Settings</h1>'+
    '</div>'+
    '<div class="card" style="max-width:480px">'+
      row('Version','0.1.0')+
      row('Data directory','/var/lib/vessel')+
      row('Config file','/etc/vessel/config.yaml')+
      row('UI port','4800')+
      '<div style="margin-top:20px;padding-top:20px;border-top:1px solid var(--border)">'+
        '<a href="https://github.com/Yash121l/Vessel" target="_blank" class="btn btn-sm">View on GitHub</a>'+
      '</div>'+
    '</div>'+
  '</div>';
}

function row(label,value){
  return '<div style="display:flex;justify-content:space-between;align-items:center;padding:10px 0;border-bottom:1px solid var(--border)">'+
    '<span style="color:var(--muted);font-size:13px">'+label+'</span>'+
    '<code style="font-family:var(--mono);font-size:12px;color:var(--accent2)">'+value+'</code>'+
  '</div>';
}

// ── Boot ──────────────────────────────────────────────────────────────────────
set({loading:true});
load();
</script>
</body>
</html>
`
