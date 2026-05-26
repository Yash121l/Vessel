package server

const uiHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Vessel</title>
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/lucide-static@0.441.0/font/lucide.min.css">
<style>
*,*::before,*::after{box-sizing:border-box;margin:0;padding:0}
:root{
  --bg:#080a0f;--surface:#0f1117;--surface2:#161b27;--surface3:#1e2438;
  --border:#1e2438;--border2:#2a3050;
  --text:#e2e8f0;--muted:#64748b;--muted2:#94a3b8;
  --accent:#6366f1;--accent2:#818cf8;--accent-dim:#6366f120;
  --green:#22c55e;--green-dim:#22c55e18;
  --red:#ef4444;--red-dim:#ef444418;
  --yellow:#f59e0b;--yellow-dim:#f59e0b18;
  --blue:#3b82f6;--blue-dim:#3b82f618;
  --purple:#a855f7;--purple-dim:#a855f718;
  --r:8px;--r2:12px;
  --font:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;
  --mono:'SF Mono','Fira Code','Cascadia Code',monospace;
  --sidebar:220px;
}
html,body{height:100%;background:var(--bg);color:var(--text);font-family:var(--font);font-size:14px;line-height:1.5}
a{color:var(--accent);text-decoration:none}a:hover{color:var(--accent2)}
button{cursor:pointer;font-family:var(--font);font-size:13px;border:none;border-radius:var(--r);padding:7px 14px;transition:all .15s;font-weight:500;white-space:nowrap}
.btn{background:var(--surface2);color:var(--muted2);border:1px solid var(--border2)}.btn:hover{color:var(--text);border-color:var(--muted)}
.btn-primary{background:var(--accent);color:#fff;border:1px solid transparent}.btn-primary:hover{background:var(--accent2)}
.btn-success{background:#16a34a;color:#fff;border:1px solid transparent}.btn-success:hover{background:#15803d}
.btn-danger{background:transparent;color:var(--red);border:1px solid #ef444430}.btn-danger:hover{background:var(--red-dim)}
.btn-sm{padding:4px 10px;font-size:12px}.btn-xs{padding:3px 8px;font-size:11px}
input,select,textarea{background:var(--surface2);border:1px solid var(--border2);border-radius:var(--r);color:var(--text);font-family:var(--font);font-size:13px;padding:8px 12px;width:100%;outline:none;transition:border-color .15s}
input:focus,select:focus,textarea:focus{border-color:var(--accent)}
label{display:block;font-size:11px;color:var(--muted);margin-bottom:5px;font-weight:600;text-transform:uppercase;letter-spacing:.06em}
.fg{margin-bottom:16px}.grid2{display:grid;grid-template-columns:1fr 1fr;gap:16px}.grid3{display:grid;grid-template-columns:1fr 1fr 1fr;gap:16px}.grid4{display:grid;grid-template-columns:repeat(4,1fr);gap:16px}
.card{background:var(--surface);border:1px solid var(--border);border-radius:var(--r2);padding:20px}
.tag{display:inline-flex;align-items:center;gap:4px;font-size:11px;font-weight:600;padding:2px 8px;border-radius:20px;text-transform:uppercase;letter-spacing:.04em}
.tag-running,.tag-active{background:var(--green-dim);color:var(--green)}
.tag-stopped,.tag-exited,.tag-inactive{background:var(--surface3);color:var(--muted)}
.tag-error{background:var(--red-dim);color:var(--red)}
.tag-deploying,.tag-updating{background:var(--yellow-dim);color:var(--yellow)}
.tag-imported{background:var(--blue-dim);color:var(--blue)}
.dot{width:6px;height:6px;border-radius:50%;background:currentColor;display:inline-block;flex-shrink:0}
.pulse{animation:pulse 2s infinite}
@keyframes pulse{0%,100%{opacity:1}50%{opacity:.35}}
</style>
</head>
<body>
<div id="app" style="display:flex;height:100vh;overflow:hidden"></div>
<script>
const API='/api/v1';
let S={
  page:'containers',nginxTab:'overview',deployTab:'templates',
  deployments:[],apps:[],containers:[],
  nginxStatus:null,nginxSites:[],nginxMainConfig:'',nginxLogs:[],nginxStats:null,
  editingSite:null,editingContent:'',newSiteMode:false,
  logs:'',logsTarget:null,logsEs:null,
  deploying:false,error:null,
  hubQuery:'',hubResults:[],hubSearching:false,hubSelected:null,
  customPorts:[{internal:'',external:'',protocol:'tcp'}],
  customVolumes:[{name:'',mount:''}]
};
function set(p){Object.assign(S,p);render()}
async function api(method,path,body){
  const o={method,headers:{'Content-Type':'application/json'}};
  if(body)o.body=JSON.stringify(body);
  const r=await fetch(API+path,o);
  const d=await r.json().catch(()=>({}));
  if(!r.ok)throw new Error(d.error||'Request failed');
  return d;
}
async function load(){
  try{
    const[dc,dd]=await Promise.all([api('GET','/docker/containers'),api('GET','/deployments')]);
    set({containers:dc.containers||[],deployments:dd.deployments||[]});
  }catch(e){set({error:e.message})}
}
async function loadApps(){try{const d=await api('GET','/apps');set({apps:d.apps||[]})}catch(e){set({error:e.message})}}
async function loadNginx(){
  try{
    const[st,si,stats]=await Promise.all([api('GET','/nginx/status'),api('GET','/nginx/sites'),api('GET','/nginx/stats')]);
    set({nginxStatus:st,nginxSites:si.sites||[],nginxStats:stats});
  }catch(e){set({error:'nginx: '+e.message})}
}
async function loadNginxConfig(){try{const d=await api('GET','/nginx/config');set({nginxMainConfig:d.content})}catch(e){set({error:e.message})}}
async function loadNginxLogs(t){try{const d=await api('GET','/nginx/logs/'+t);set({nginxLogs:d.lines||[]})}catch(e){set({error:e.message})}}
async function act(id,a){
  if(a==='stop'&&!confirm('Stop this deployment?'))return;
  if(a==='restart'&&!confirm('Restart this deployment?'))return;
  try{await api('POST','/deployments/'+id+'/'+a);await load()}catch(e){set({error:e.message})}
}
async function actC(id,a,name){
  if(a==='stop'&&!confirm('Stop "'+name+'"?'))return;
  if(a==='restart'&&!confirm('Restart "'+name+'"?'))return;
  try{await api('POST','/docker/containers/'+id+'/'+a);await load()}catch(e){set({error:e.message})}
}
async function monitor(id,name){try{await api('POST','/docker/import',{container_id:id,name});await load()}catch(e){set({error:e.message})}}
async function remove(id,name){
  if(!confirm('Remove "'+name+'"? Containers and volumes will be deleted.'))return;
  try{await api('DELETE','/deployments/'+id);await load()}catch(e){set({error:e.message})}
}
async function deploy(e){
  e.preventDefault();const f=e.target,env={};
  (f.env.value||'').trim().split('\n').forEach(l=>{const i=l.indexOf('=');if(i>0)env[l.slice(0,i).trim()]=l.slice(i+1).trim()});
  set({deploying:true,error:null});
  try{
    await api('POST','/deployments',{app_id:f.app_id.value,name:f.dname.value,domain:f.domain.value,env});
    await load();set({page:'containers',deploying:false});
  }catch(e){set({deploying:false,error:e.message})}
}
async function deployCustom(e){
  e.preventDefault();const f=e.target,env={};
  (f.cenv.value||'').trim().split('\n').forEach(l=>{const i=l.indexOf('=');if(i>0)env[l.slice(0,i).trim()]=l.slice(i+1).trim()});
  const ports=S.customPorts.filter(p=>p.internal).map(p=>({internal:parseInt(p.internal),external:parseInt(p.external||p.internal),protocol:p.protocol||'tcp'}));
  const volumes=S.customVolumes.filter(v=>v.mount).map(v=>({name:v.name||v.mount.replace(/\//g,'-').replace(/^-/,''),mount_path:v.mount}));
  const image=S.hubSelected?S.hubSelected.slug:f.cimage.value.trim();
  if(!image){set({error:'Image is required'});return}
  set({deploying:true,error:null});
  try{
    await api('POST','/docker/deploy',{image,name:f.cname.value,domain:f.cdomain.value,ports,volumes,env});
    await load();set({page:'containers',deploying:false,hubSelected:null,hubResults:[],hubQuery:''});
  }catch(e){set({deploying:false,error:e.message})}
}
let hubTimer=null;
function hubSearch(q){
  S.hubQuery=q;
  clearTimeout(hubTimer);
  if(!q.trim()){set({hubResults:[],hubSearching:false});return}
  set({hubSearching:true});
  hubTimer=setTimeout(async()=>{
    try{
      const r=await fetch(API+'/docker/search?q='+encodeURIComponent(q));
      const d=await r.json();
      set({hubResults:d.results||[],hubSearching:false});
    }catch{set({hubSearching:false})}
  },400);
}
function selectHub(item){
  set({hubSelected:item,hubResults:[]});
  const el=document.getElementById('cimage');
  if(el)el.value=item.slug;
}
function ngxAction(action){api('POST','/nginx/'+action).then(loadNginx).catch(e=>set({error:e.message}))}
async function ngxTest(){try{const d=await api('GET','/nginx/test');alert(d.ok?'Config OK\n\n'+d.output:'Config Error\n\n'+d.output)}catch(e){set({error:e.message})}}
async function ngxSaveMainConfig(){try{await api('PUT','/nginx/config',{content:S.nginxMainConfig});alert('Saved.')}catch(e){set({error:e.message})}}
async function ngxEditSite(name){try{const d=await api('GET','/nginx/sites/'+name);set({editingSite:name,editingContent:d.content,newSiteMode:false})}catch(e){set({error:e.message})}}
async function ngxSaveSite(){try{await api('PUT','/nginx/sites/'+S.editingSite,{content:S.editingContent});set({editingSite:null,editingContent:''});await loadNginx()}catch(e){set({error:e.message})}}
async function ngxToggleSite(name,enabled){try{await api('POST','/nginx/sites/'+name+'/'+(enabled?'disable':'enable'));await loadNginx()}catch(e){set({error:e.message})}}
async function ngxDeleteSite(name){if(!confirm('Delete site "'+name+'"?'))return;try{await api('DELETE','/nginx/sites/'+name);await loadNginx()}catch(e){set({error:e.message})}}
async function ngxCreateSite(e){
  e.preventDefault();const f=e.target;
  try{await api('POST','/nginx/sites',{name:f.sitename.value,server_name:f.server_name.value,port:parseInt(f.port.value)||80,upstream:f.upstream.value});set({newSiteMode:false});await loadNginx()}catch(e){set({error:e.message})}
}
function openLogs(id,type,name){
  if(S.logsEs)S.logsEs.close();
  const path=type==='c'?'/docker/containers/'+id+'/logs':'/deployments/'+id+'/logs';
  set({page:'logs',logsTarget:{id,name},logs:''});
  const es=new EventSource(API+path);
  es.onmessage=e=>{S.logs+=e.data+'\n';const el=document.getElementById('logbox');if(el){el.textContent=S.logs;el.scrollTop=el.scrollHeight}};
  es.onerror=()=>es.close();S.logsEs=es;
}
function openNginxLogs(type){
  if(S.logsEs)S.logsEs.close();
  set({page:'logs',logsTarget:{id:type,name:'nginx '+type+' log'},logs:''});
  const es=new EventSource(API+'/nginx/logs/'+type+'/stream');
  es.onmessage=e=>{S.logs+=e.data+'\n';const el=document.getElementById('logbox');if(el){el.textContent=S.logs;el.scrollTop=el.scrollHeight}};
  es.onerror=()=>es.close();S.logsEs=es;
}
function nav(p){
  if(S.logsEs&&p!=='logs'){S.logsEs.close();S.logsEs=null}
  set({page:p,error:null,editingSite:null,newSiteMode:false});
  if(p==='containers')load();
  if(p==='deploy')loadApps();
  if(p==='nginx'){loadNginx();loadNginxConfig();}
}
// ── Icon helpers ──────────────────────────────────────────────────────────────
// dockerIcon returns an <img> tag using Docker Hub logo, with SVG fallback.
// For official images (no slash) namespace is "library".
function dockerIcon(image,size){
  size=size||36;
  if(!image)return lucideIcon('box',size);
  const clean=image.split(':')[0];
  const parts=clean.split('/');
  let ns,repo;
  if(parts.length===1){ns='library';repo=parts[0]}
  else if(parts.length===2){ns=parts[0];repo=parts[1]}
  else{ns=parts[0];repo=parts[1]}
  // Docker Hub logo endpoint
  const src='https://hub.docker.com/api/content/v1/products/images/'+ns+'/'+repo+'/logo';
  return '<img src="'+src+'" width="'+size+'" height="'+size+'" style="border-radius:6px;object-fit:contain;background:var(--surface2)" '+
    'onerror="this.replaceWith(lucideIconEl(\'box\','+size+'))">';
}
function lucideIconEl(name,size){
  const el=document.createElement('span');
  el.innerHTML=lucideIcon(name,size);
  return el.firstChild;
}
function lucideIcon(name,size){
  size=size||16;
  return '<i class="icon-'+name+'" style="font-size:'+size+'px;color:var(--muted2)"></i>';
}
// appIcon: for template apps use Docker Hub logo based on image field; fallback to lucide
function appIcon(app,size){
  if(app&&app.image)return dockerIcon(app.image,size||36);
  return lucideIcon('box',size||36);
}
// containerIcon: use image from container data
function containerIcon(image,size){
  if(image)return dockerIcon(image,size||36);
  return lucideIcon('box',size||36);
}
function badge(s){
  return'<span class="tag tag-'+(s||'stopped')+'"><span class="dot'+(s==='running'||s==='active'?' pulse':'')+'"></span>'+(s||'unknown')+'</span>';
}
function fmtBytes(b){
  if(!b)return'0 B';if(b<1024)return b+' B';if(b<1048576)return(b/1024).toFixed(1)+' KB';
  if(b<1073741824)return(b/1048576).toFixed(1)+' MB';return(b/1073741824).toFixed(2)+' GB';
}
function statusColor(code){
  const c=parseInt(code);
  if(c>=500)return'var(--red)';if(c>=400)return'var(--yellow)';if(c>=300)return'var(--blue)';return'var(--green)';
}
function escHtml(s){return(s||'').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;')}
// ── Layout ────────────────────────────────────────────────────────────────────
function render(){
  const navItems=[
    {id:'containers',label:'Containers',icon:'layout-dashboard'},
    {id:'nginx',label:'Nginx',icon:'server'},
    {id:'deploy',label:'Deploy',icon:'rocket'},
    {id:'settings',label:'Settings',icon:'settings'},
  ];
  const sidebar='<nav style="width:var(--sidebar);min-width:var(--sidebar);background:var(--surface);border-right:1px solid var(--border);display:flex;flex-direction:column;height:100vh;overflow:hidden">'+
    '<div style="padding:16px;border-bottom:1px solid var(--border)">'+
      '<div style="display:flex;align-items:center;gap:10px">'+
        '<div style="width:32px;height:32px;background:var(--accent);border-radius:8px;display:flex;align-items:center;justify-content:center;flex-shrink:0">'+
          '<i class="icon-anchor" style="font-size:16px;color:#fff"></i></div>'+
        '<div><div style="font-weight:700;font-size:14px;letter-spacing:-.3px">Vessel</div>'+
        '<div style="font-size:10px;color:var(--muted)">v0.1.0</div></div>'+
      '</div>'+
    '</div>'+
    '<div style="padding:8px;flex:1">'+
    navItems.map(n=>{
      const a=S.page===n.id||(S.page==='logs'&&n.id==='containers');
      return'<a href="#" onclick="nav(\''+n.id+'\');return false" style="display:flex;align-items:center;gap:10px;padding:9px 12px;border-radius:var(--r);color:'+(a?'var(--text)':'var(--muted)')+';background:'+(a?'var(--surface2)':'transparent')+';font-weight:'+(a?'600':'400')+';margin-bottom:2px;font-size:13px;transition:all .15s;text-decoration:none">'+
        '<i class="icon-'+n.icon+'" style="font-size:16px;color:'+(a?'var(--accent)':'currentColor')+'"></i>'+
        n.label+'</a>';
    }).join('')+
    '</div>'+
    '<div style="padding:12px 16px;border-top:1px solid var(--border)">'+
      '<a href="https://github.com/Yash121l/Vessel" target="_blank" style="font-size:11px;color:var(--muted);display:flex;align-items:center;gap:6px">'+
        '<i class="icon-github" style="font-size:13px"></i>GitHub</a>'+
    '</div></nav>';
  const errBanner=S.error?'<div style="background:var(--red-dim);border:1px solid #ef444430;border-radius:var(--r);padding:10px 16px;margin-bottom:20px;display:flex;justify-content:space-between;align-items:center;gap:12px"><span style="color:var(--red);font-size:13px">'+S.error+'</span><button class="btn btn-xs" onclick="set({error:null})">✕</button></div>':'';
  const pages={containers:pageContainers,nginx:pageNginx,deploy:pageDeploy,logs:pageLogs,settings:pageSettings};
  const content=(pages[S.page]||pageContainers)();
  document.getElementById('app').innerHTML=
    sidebar+
    '<div style="flex:1;overflow-y:auto;height:100vh">'+
      '<div style="padding:28px 32px;max-width:1400px;margin:0 auto">'+errBanner+content+'</div>'+
    '</div>';
}
// ── Containers page ───────────────────────────────────────────────────────────
function pageContainers(){
  const managed=S.deployments.filter(d=>!d.imported);
  const imported=S.deployments.filter(d=>d.imported);
  const trackedIds=new Set();const trackedNames=new Set();
  S.deployments.forEach(d=>{
    if(d.container_id){trackedIds.add(d.container_id);trackedIds.add(d.container_id.slice(0,12))}
    if(d.name)trackedNames.add(d.name);
  });
  const untracked=S.containers.filter(c=>!c.managed_by_vessel&&!trackedIds.has(c.id)&&!trackedIds.has(c.id.slice(0,12))&&!trackedNames.has(c.name));
  const running=S.containers.filter(c=>c.state==='running').length;
  let h='<div style="display:flex;justify-content:space-between;align-items:flex-start;margin-bottom:28px">'+
    '<div><h1 style="font-size:22px;font-weight:700;letter-spacing:-.5px">Containers</h1>'+
      '<p style="color:var(--muted);font-size:13px;margin-top:4px"><span style="color:var(--green);font-weight:600">'+running+' running</span> · '+S.containers.length+' total</p></div>'+
    '<div style="display:flex;gap:8px">'+
      '<button class="btn btn-sm" onclick="load()"><i class="icon-refresh-cw" style="font-size:12px"></i> Refresh</button>'+
      '<button class="btn-primary btn-sm" onclick="nav(\'deploy\')"><i class="icon-plus" style="font-size:12px"></i> Deploy</button>'+
    '</div></div>';
  function section(title,count,cards){
    return'<div style="font-size:10px;font-weight:700;color:var(--muted);text-transform:uppercase;letter-spacing:.1em;margin-bottom:12px;display:flex;align-items:center;gap:8px">'+
      '<span>'+title+'</span><span style="background:var(--surface2);border-radius:10px;padding:1px 7px;font-size:10px">'+count+'</span></div>'+
      '<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(440px,1fr));gap:10px;margin-bottom:28px">'+cards+'</div>';
  }
  if(managed.length)h+=section('Vessel Managed',managed.length,managed.map(cardManaged).join(''));
  if(imported.length)h+=section('Monitored',imported.length,imported.map(cardImported).join(''));
  const uRun=untracked.filter(c=>c.state==='running');
  const uStop=untracked.filter(c=>c.state!=='running');
  if(uRun.length)h+=section('Running — click Monitor to track',uRun.length,uRun.map(cardDiscovered).join(''));
  if(uStop.length)h+=section('Stopped / Exited',uStop.length,uStop.map(cardDiscovered).join(''));
  if(!managed.length&&!imported.length&&!untracked.length){
    h+='<div style="text-align:center;padding:80px 20px">'+
      '<i class="icon-rocket" style="font-size:48px;color:var(--muted);display:block;margin-bottom:16px"></i>'+
      '<div style="font-size:18px;font-weight:600;margin-bottom:8px">No containers yet</div>'+
      '<div style="color:var(--muted);margin-bottom:24px">Deploy your first self-hosted app in seconds</div>'+
      '<button class="btn-primary" onclick="nav(\'deploy\')">Deploy an app</button></div>';
  }
  return h;
}
function cardManaged(d){
  const r=d.status==='running';
  const app=S.apps.find(a=>a.id===d.app_id);
  const img=app?app.image:d.app_id;
  return'<div class="card" style="display:flex;align-items:center;gap:14px;padding:16px 18px">'+
    '<div style="flex-shrink:0">'+containerIcon(img,36)+'</div>'+
    '<div style="flex:1;min-width:0">'+
      '<div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap">'+
        '<span style="font-weight:600;font-size:14px">'+d.name+'</span>'+badge(d.status)+'</div>'+
      '<div style="color:var(--muted);font-size:11px;margin-top:3px">'+d.app_id+
        (d.domain?' · <a href="https://'+d.domain+'" target="_blank" style="color:var(--accent2)">'+d.domain+'</a>':'')+
      '</div></div>'+
    '<div style="display:flex;gap:5px;flex-shrink:0">'+
      (r?'<button class="btn btn-sm" onclick="act(\''+d.id+'\',\'stop\')">Stop</button>':'<button class="btn btn-sm" onclick="act(\''+d.id+'\',\'start\')">Start</button>')+
      '<button class="btn btn-sm" onclick="act(\''+d.id+'\',\'restart\')">Restart</button>'+
      '<button class="btn btn-sm" onclick="openLogs(\''+d.id+'\',\'d\',\''+d.name+'\')">Logs</button>'+
      '<button class="btn btn-sm" onclick="act(\''+d.id+'\',\'update\')">Update</button>'+
      '<button class="btn-danger btn-sm" onclick="remove(\''+d.id+'\',\''+d.name+'\')">✕</button>'+
    '</div></div>';
}
function cardImported(d){
  const r=d.status==='running';
  const ports=d.ports?d.ports.split(', ').filter(Boolean):[];
  return'<div class="card" style="display:flex;align-items:center;gap:14px;padding:16px 18px">'+
    '<div style="flex-shrink:0">'+containerIcon(d.image,36)+'</div>'+
    '<div style="flex:1;min-width:0">'+
      '<div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap">'+
        '<span style="font-weight:600;font-size:14px">'+d.name+'</span>'+badge(d.status)+'<span class="tag tag-imported">monitored</span></div>'+
      '<div style="color:var(--muted);font-size:11px;margin-top:3px">'+(d.image||d.app_id)+(ports.length?' · <span style="color:var(--accent2)">'+ports.slice(0,2).join(' · ')+'</span>':'')+
      '</div></div>'+
    '<div style="display:flex;gap:5px;flex-shrink:0">'+
      (r?'<button class="btn btn-sm" onclick="actC(\''+d.container_id+'\',\'stop\',\''+d.name+'\')">Stop</button>':'<button class="btn btn-sm" onclick="actC(\''+d.container_id+'\',\'start\',\''+d.name+'\')">Start</button>')+
      '<button class="btn btn-sm" onclick="actC(\''+d.container_id+'\',\'restart\',\''+d.name+'\')">Restart</button>'+
      '<button class="btn btn-sm" onclick="openLogs(\''+d.name+'\',\'c\',\''+d.name+'\')">Logs</button>'+
    '</div></div>';
}
function cardDiscovered(c){
  const r=c.state==='running';
  const ports=c.ports&&c.ports.length?c.ports.slice(0,2).join(' · '):'';
  return'<div class="card" style="display:flex;align-items:center;gap:14px;padding:16px 18px;'+(r?'':'opacity:.65;')+'">'+
    '<div style="flex-shrink:0">'+containerIcon(c.image,36)+'</div>'+
    '<div style="flex:1;min-width:0">'+
      '<div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap">'+
        '<span style="font-weight:600;font-size:14px">'+c.name+'</span>'+badge(c.state)+'</div>'+
      '<div style="color:var(--muted);font-size:11px;margin-top:3px">'+c.image+(ports?' · <span style="color:var(--accent2)">'+ports+'</span>':'')+
      '</div></div>'+
    '<div style="display:flex;gap:5px;flex-shrink:0">'+
      (r?'<button class="btn btn-sm" onclick="openLogs(\''+c.id+'\',\'c\',\''+c.name+'\')">Logs</button>':'')+
      (r?'<button class="btn-primary btn-sm" onclick="monitor(\''+c.id+'\',\''+c.name+'\')">Monitor</button>':
         '<button class="btn btn-sm" onclick="actC(\''+c.id+'\',\'start\',\''+c.name+'\')">Start</button>')+
    '</div></div>';
}
// ── Deploy page ───────────────────────────────────────────────────────────────
function pageDeploy(){
  const tabs='<div class="tabs">'+
    '<span class="tab'+(S.deployTab==='templates'?' on':'')+'" onclick="set({deployTab:\'templates\'})">Templates</span>'+
    '<span class="tab'+(S.deployTab==='custom'?' on':'')+'" onclick="set({deployTab:\'custom\'})">Custom Container</span>'+
  '</div>';
  return'<div>'+
    '<div style="margin-bottom:24px">'+
      '<h1 style="font-size:22px;font-weight:700;letter-spacing:-.5px">Deploy</h1>'+
      '<p style="color:var(--muted);font-size:13px;margin-top:4px">Deploy from curated templates or any Docker Hub image</p>'+
    '</div>'+
    tabs+
    (S.deployTab==='templates'?deployTemplates():deployCustom())+
  '</div>';
}
function deployTemplates(){
  const apps=S.apps;
  return'<form onsubmit="deploy(event)">'+
    '<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(160px,1fr));gap:10px;margin-bottom:24px">'+
    apps.map(a=>'<label style="cursor:pointer">'+
      '<input type="radio" name="app_id" value="'+a.id+'" style="display:none" onchange="selectApp(\''+a.id+'\')" required>'+
      '<div id="ac-'+a.id+'" style="background:var(--surface);border:2px solid var(--border);border-radius:var(--r2);padding:18px 12px;text-align:center;transition:all .15s">'+
        '<div style="width:48px;height:48px;margin:0 auto 10px;border-radius:10px;overflow:hidden;background:var(--surface2);display:flex;align-items:center;justify-content:center">'+
          appIcon(a,40)+
        '</div>'+
        '<div style="font-weight:600;font-size:13px">'+(a.name||a.id)+'</div>'+
        '<div style="font-size:11px;color:var(--muted);margin-top:3px">'+(a.category||'')+'</div>'+
      '</div></label>').join('')+
    '</div>'+
    '<div class="card" style="max-width:700px">'+
      '<div class="grid2">'+
        '<div class="fg"><label>Deployment Name *</label><input name="dname" placeholder="my-app" pattern="[a-z0-9-]+" required></div>'+
        '<div class="fg"><label>Custom Domain (optional)</label><input name="domain" placeholder="app.example.com"></div>'+
      '</div>'+
      '<div class="fg"><label>Environment Variables (KEY=VALUE, one per line)</label>'+
        '<textarea name="env" rows="5" placeholder="SECRET_KEY=abc123" style="font-family:var(--mono);font-size:12px"></textarea></div>'+
      '<div id="env-hints" style="margin-bottom:16px"></div>'+
      '<div style="display:flex;gap:8px;justify-content:flex-end">'+
        '<button type="button" class="btn" onclick="nav(\'containers\')">Cancel</button>'+
        '<button type="submit" class="btn-primary"'+(S.deploying?' disabled':'')+'>'+
          (S.deploying?'<i class="icon-loader-2" style="font-size:13px"></i> Deploying…':'<i class="icon-rocket" style="font-size:13px"></i> Deploy')+'</button>'+
      '</div>'+
    '</div></form>';
}
function selectApp(id){
  S.apps.forEach(a=>{const el=document.getElementById('ac-'+a.id);if(el)el.style.borderColor=a.id===id?'var(--accent)':'var(--border)'});
  const app=S.apps.find(a=>a.id===id);
  const hints=document.getElementById('env-hints');
  if(!hints||!app||!app.env_vars)return;
  const req=app.env_vars.filter(e=>e.required);
  if(!req.length){hints.innerHTML='';return}
  hints.innerHTML='<div style="background:var(--surface2);border-radius:var(--r);padding:12px 16px">'+
    '<div style="font-size:10px;font-weight:700;color:var(--muted);text-transform:uppercase;letter-spacing:.06em;margin-bottom:10px">Required variables</div>'+
    req.map(e=>'<div style="display:flex;gap:12px;margin-bottom:6px;font-size:12px;align-items:baseline">'+
      '<code style="color:var(--accent);min-width:200px;font-family:var(--mono);flex-shrink:0">'+e.key+'</code>'+
      '<span style="color:var(--muted)">'+e.description+'</span></div>').join('')+'</div>';
}
function deployCustom(){
  const sel=S.hubSelected;
  const searchBox='<div style="position:relative;margin-bottom:16px">'+
    '<div style="display:flex;gap:8px;align-items:center">'+
      '<i class="icon-search" style="font-size:14px;color:var(--muted);position:absolute;left:10px;top:50%;transform:translateY(-50%)"></i>'+
      '<input id="hubsearch" placeholder="Search Docker Hub (e.g. nginx, postgres, redis…)" style="padding-left:34px" '+
        'value="'+escHtml(S.hubQuery)+'" oninput="hubSearch(this.value)">'+
      (S.hubSearching?'<i class="icon-loader-2" style="font-size:14px;color:var(--muted);position:absolute;right:10px;top:50%;transform:translateY(-50%)"></i>':'')+
    '</div>'+
    (S.hubResults.length?
      '<div style="position:absolute;z-index:100;width:100%;background:var(--surface);border:1px solid var(--border2);border-radius:var(--r);margin-top:4px;max-height:280px;overflow-y:auto;box-shadow:0 8px 24px #0006">'+
        S.hubResults.map(r=>{
          const logo=r.logo_url&&r.logo_url.large?r.logo_url.large:'';
          return'<div class="hub-result" onclick="selectHub('+JSON.stringify({slug:r.slug,name:r.name,description:r.short_description,logo:logo}).replace(/"/g,'&quot;')+')">'+
            (logo?'<img src="'+logo+'" width="32" height="32" style="border-radius:6px;object-fit:contain;background:var(--surface2)" onerror="this.style.display=\'none\'">':
              '<i class="icon-box" style="font-size:24px;color:var(--muted2)"></i>')+
            '<div style="min-width:0">'+
              '<div style="font-weight:600;font-size:13px;font-family:var(--mono)">'+escHtml(r.slug)+'</div>'+
              '<div style="font-size:11px;color:var(--muted);overflow:hidden;text-overflow:ellipsis;white-space:nowrap">'+escHtml(r.short_description||'')+'</div>'+
            '</div>'+
            '<div style="margin-left:auto;font-size:11px;color:var(--muted);white-space:nowrap">'+
              (r.pull_count?fmtPulls(r.pull_count)+' pulls':'')+
            '</div>'+
          '</div>';
        }).join('')+
      '</div>':'')+
  '</div>';
  const selectedBanner=sel?
    '<div style="display:flex;align-items:center;gap:12px;background:var(--accent-dim);border:1px solid var(--accent);border-radius:var(--r);padding:12px 16px;margin-bottom:16px">'+
      (sel.logo?'<img src="'+sel.logo+'" width="36" height="36" style="border-radius:6px;object-fit:contain;background:var(--surface2)">':
        '<i class="icon-box" style="font-size:28px;color:var(--accent2)"></i>')+
      '<div style="flex:1;min-width:0">'+
        '<div style="font-weight:600;font-family:var(--mono)">'+escHtml(sel.slug)+'</div>'+
        '<div style="font-size:11px;color:var(--muted2)">'+escHtml(sel.description||'')+'</div>'+
      '</div>'+
      '<button class="btn btn-xs" onclick="set({hubSelected:null,hubQuery:\'\'})">Change</button>'+
    '</div>':'';
  return'<form onsubmit="deployCustom(event)">'+
    '<div class="card" style="max-width:760px">'+
      '<div style="font-weight:600;font-size:13px;margin-bottom:14px;display:flex;align-items:center;gap:8px">'+
        '<i class="icon-search" style="font-size:14px;color:var(--accent)"></i> Find image on Docker Hub</div>'+
      searchBox+
      selectedBanner+
      '<div class="fg"><label>Image *</label>'+
        '<input id="cimage" name="cimage" placeholder="nginx:latest or myorg/myapp:1.0" '+(sel?'value="'+escHtml(sel.slug)+'"':'')+' required></div>'+
      '<div class="grid2">'+
        '<div class="fg"><label>Deployment Name *</label><input name="cname" placeholder="my-nginx" pattern="[a-z0-9-]+" required></div>'+
        '<div class="fg"><label>Custom Domain (optional)</label><input name="cdomain" placeholder="app.example.com"></div>'+
      '</div>'+
      '<div class="fg">'+
        '<label style="display:flex;justify-content:space-between">Ports <button type="button" class="btn btn-xs" onclick="S.customPorts.push({internal:\'\',external:\'\',protocol:\'tcp\'});render()">+ Add</button></label>'+
        '<div style="display:flex;flex-direction:column;gap:6px">'+
        S.customPorts.map((p,i)=>
          '<div style="display:flex;gap:6px;align-items:center">'+
            '<input placeholder="Container port" value="'+p.internal+'" oninput="S.customPorts['+i+'].internal=this.value" style="width:140px">'+
            '<span style="color:var(--muted);flex-shrink:0">→</span>'+
            '<input placeholder="Host port" value="'+p.external+'" oninput="S.customPorts['+i+'].external=this.value" style="width:140px">'+
            '<select oninput="S.customPorts['+i+'].protocol=this.value" style="width:80px">'+
              '<option value="tcp"'+(p.protocol==='tcp'?' selected':'')+'>tcp</option>'+
              '<option value="udp"'+(p.protocol==='udp'?' selected':'')+'>udp</option>'+
            '</select>'+
            (S.customPorts.length>1?'<button type="button" class="btn btn-xs btn-danger" onclick="S.customPorts.splice('+i+',1);render()">✕</button>':'')+
          '</div>'
        ).join('')+
        '</div></div>'+
      '<div class="fg">'+
        '<label style="display:flex;justify-content:space-between">Volumes <button type="button" class="btn btn-xs" onclick="S.customVolumes.push({name:\'\',mount:\'\'});render()">+ Add</button></label>'+
        '<div style="display:flex;flex-direction:column;gap:6px">'+
        S.customVolumes.map((v,i)=>
          '<div style="display:flex;gap:6px;align-items:center">'+
            '<input placeholder="Volume name (optional)" value="'+v.name+'" oninput="S.customVolumes['+i+'].name=this.value" style="width:180px">'+
            '<input placeholder="Mount path e.g. /data" value="'+v.mount+'" oninput="S.customVolumes['+i+'].mount=this.value" style="flex:1">'+
            (S.customVolumes.length>1?'<button type="button" class="btn btn-xs btn-danger" onclick="S.customVolumes.splice('+i+',1);render()">✕</button>':'')+
          '</div>'
        ).join('')+
        '</div></div>'+
      '<div class="fg"><label>Environment Variables (KEY=VALUE, one per line)</label>'+
        '<textarea name="cenv" rows="4" placeholder="MY_VAR=value" style="font-family:var(--mono);font-size:12px"></textarea></div>'+
      '<div style="display:flex;gap:8px;justify-content:flex-end">'+
        '<button type="button" class="btn" onclick="nav(\'containers\')">Cancel</button>'+
        '<button type="submit" class="btn-primary"'+(S.deploying?' disabled':'')+'>'+
          (S.deploying?'<i class="icon-loader-2" style="font-size:13px"></i> Deploying…':'<i class="icon-rocket" style="font-size:13px"></i> Deploy')+'</button>'+
      '</div>'+
    '</div></form>';
}
function fmtPulls(n){if(n>=1e9)return(n/1e9).toFixed(1)+'B';if(n>=1e6)return(n/1e6).toFixed(1)+'M';if(n>=1e3)return(n/1e3).toFixed(0)+'K';return n}
// ── Nginx page ────────────────────────────────────────────────────────────────
function pageNginx(){
  const st=S.nginxStatus;const stats=S.nginxStats;
  const header='<div style="display:flex;justify-content:space-between;align-items:flex-start;margin-bottom:24px">'+
    '<div><h1 style="font-size:22px;font-weight:700;letter-spacing:-.5px">Nginx</h1>'+
      '<p style="color:var(--muted);font-size:13px;margin-top:4px">'+(st?st.version||'nginx':'Loading…')+'</p></div>'+
    '<div style="display:flex;gap:8px;align-items:center">'+
      (st&&st.running?'<span class="tag tag-active"><span class="dot pulse"></span>Active</span>':'<span class="tag tag-stopped">Inactive</span>')+
      '<button class="btn btn-sm" onclick="ngxTest()">Test Config</button>'+
      '<button class="btn btn-sm" onclick="ngxAction(\'reload\')">Reload</button>'+
      '<button class="btn btn-sm" onclick="ngxAction(\'restart\')">Restart</button>'+
      (st&&st.running?'<button class="btn-danger btn-sm" onclick="ngxAction(\'stop\')">Stop</button>':'<button class="btn-success btn-sm" onclick="ngxAction(\'start\')">Start</button>')+
      '<button class="btn btn-sm" onclick="loadNginx()"><i class="icon-refresh-cw" style="font-size:12px"></i></button>'+
    '</div></div>';
  const tabs=['overview','sites','config','logs'].map(t=>
    '<span class="tab'+(S.nginxTab===t?' on':'')+'" onclick="set({nginxTab:\''+t+'\'})">'+
      {overview:'Overview',sites:'Sites',config:'nginx.conf',logs:'Logs'}[t]+'</span>').join('');
  let content='';
  if(S.nginxTab==='overview')content=nginxOverview(stats,st);
  else if(S.nginxTab==='sites')content=nginxSitesTab();
  else if(S.nginxTab==='config')content=nginxConfigTab();
  else if(S.nginxTab==='logs')content=nginxLogsTab();
  return header+'<div class="tabs">'+tabs+'</div>'+content;
}
function statCard(val,label,sub,color,iconName){
  return'<div class="stat-card">'+
    '<div style="display:flex;justify-content:space-between;align-items:flex-start">'+
      '<div><div class="stat-val" style="color:'+color+'">'+val+'</div>'+
        '<div class="stat-lbl">'+label+'</div><div class="stat-sub">'+sub+'</div></div>'+
      '<div style="width:36px;height:36px;background:'+color+'18;border-radius:8px;display:flex;align-items:center;justify-content:center;flex-shrink:0">'+
        '<i class="icon-'+iconName+'" style="font-size:18px;color:'+color+'"></i>'+
      '</div>'+
    '</div></div>';
}
function nginxOverview(stats,st){
  if(!stats)return'<div style="color:var(--muted);padding:40px;text-align:center">Loading stats…</div>';
  const total=stats.total_requests||0;
  const pct=(n)=>total?Math.round(n/total*100):0;
  const cards='<div class="grid4" style="margin-bottom:24px">'+
    statCard(total.toLocaleString(),'Total Requests','Last 5000 log lines','var(--accent)','bar-chart-2')+
    statCard(stats.requests_2xx.toLocaleString(),'2xx Success',pct(stats.requests_2xx)+'% of traffic','var(--green)','check-circle')+
    statCard(stats.requests_4xx.toLocaleString(),'4xx Errors',pct(stats.requests_4xx)+'% of traffic','var(--yellow)','alert-triangle')+
    statCard(stats.requests_5xx.toLocaleString(),'5xx Errors',pct(stats.requests_5xx)+'% of traffic','var(--red)','x-circle')+
  '</div>';
  const maxPath=stats.top_paths&&stats.top_paths.length?stats.top_paths[0].count:1;
  const pathsHtml=stats.top_paths&&stats.top_paths.length?
    stats.top_paths.map(p=>'<div class="bar-row">'+
      '<span style="min-width:200px;color:var(--muted2);font-family:var(--mono);font-size:11px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">'+escHtml(p.path)+'</span>'+
      '<div class="bar-track"><div class="bar-fill" style="width:'+Math.round(p.count/maxPath*100)+'%;background:var(--accent)"></div></div>'+
      '<span style="min-width:40px;text-align:right;color:var(--muted2)">'+p.count+'</span></div>').join(''):
    '<div style="color:var(--muted);font-size:13px;padding:20px 0">No data yet</div>';
  const maxIP=stats.top_ips&&stats.top_ips.length?stats.top_ips[0].count:1;
  const ipsHtml=stats.top_ips&&stats.top_ips.length?
    stats.top_ips.map(ip=>'<div class="bar-row">'+
      '<span style="min-width:140px;color:var(--muted2);font-family:var(--mono);font-size:11px">'+ip.ip+'</span>'+
      '<div class="bar-track"><div class="bar-fill" style="width:'+Math.round(ip.count/maxIP*100)+'%;background:var(--purple)"></div></div>'+
      '<span style="min-width:40px;text-align:right;color:var(--muted2)">'+ip.count+'</span></div>').join(''):
    '<div style="color:var(--muted);font-size:13px;padding:20px 0">No data yet</div>';
  const codes=stats.status_codes||{};
  const codeRows=Object.entries(codes).sort((a,b)=>b[1]-a[1]).slice(0,8).map(([code,count])=>
    '<tr><td><span style="color:'+statusColor(code)+';font-family:var(--mono);font-weight:600">'+code+'</span></td>'+
    '<td style="text-align:right">'+count+'</td><td style="text-align:right;color:var(--muted)">'+pct(count)+'%</td></tr>').join('');
  const activeSites=stats.active_sites||[];
  const sitesRows=activeSites.length?activeSites.map(s=>
    '<tr><td><span style="font-family:var(--mono);font-size:12px">'+s.name+'</span></td>'+
    '<td><span style="color:var(--accent2)">'+escHtml(s.server_name||'—')+'</span></td>'+
    '<td>'+escHtml(s.upstream||'—')+'</td><td>'+escHtml(s.port||'80')+'</td>'+
    '<td>'+(s.enabled?'<span class="tag tag-active">enabled</span>':'<span class="tag tag-stopped">disabled</span>')+'</td></tr>').join(''):
    '<tr><td colspan="5" style="text-align:center;color:var(--muted);padding:20px">No sites configured</td></tr>';
  const recentRows=(stats.recent_requests||[]).slice(0,15).map(r=>
    '<tr><td style="font-family:var(--mono);font-size:11px;color:var(--muted)">'+r.ip+'</td>'+
    '<td><span style="color:var(--blue);font-weight:600;font-size:11px">'+r.method+'</span></td>'+
    '<td style="font-family:var(--mono);font-size:11px;max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">'+escHtml(r.path)+'</td>'+
    '<td><span style="color:'+statusColor(r.status)+';font-weight:700;font-family:var(--mono)">'+r.status+'</span></td>'+
    '<td style="color:var(--muted);font-size:11px">'+r.size+' B</td></tr>').join('');
  return cards+
    '<div class="grid2" style="margin-bottom:24px">'+
      '<div class="card"><div style="font-weight:600;margin-bottom:16px;font-size:13px">Top Paths</div>'+pathsHtml+'</div>'+
      '<div class="card"><div style="font-weight:600;margin-bottom:16px;font-size:13px">Top Client IPs</div>'+ipsHtml+
        '<div style="margin-top:20px;padding-top:16px;border-top:1px solid var(--border)">'+
          '<div style="font-weight:600;margin-bottom:12px;font-size:13px">Status Codes</div>'+
          '<table class="tbl"><thead><tr><th>Code</th><th style="text-align:right">Count</th><th style="text-align:right">%</th></tr></thead><tbody>'+codeRows+'</tbody></table>'+
        '</div></div>'+
    '</div>'+
    '<div class="card" style="margin-bottom:24px">'+
      '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">'+
        '<div style="font-weight:600;font-size:13px">Active Sites</div>'+
        '<button class="btn btn-sm" onclick="set({nginxTab:\'sites\'})">Manage →</button></div>'+
      '<table class="tbl"><thead><tr><th>Config</th><th>Domain</th><th>Upstream</th><th>Port</th><th>Status</th></tr></thead><tbody>'+sitesRows+'</tbody></table>'+
    '</div>'+
    '<div class="card">'+
      '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">'+
        '<div style="font-weight:600;font-size:13px">Recent Requests</div>'+
        '<button class="btn btn-sm" onclick="set({nginxTab:\'logs\'})">View Logs →</button></div>'+
      (recentRows?'<table class="tbl"><thead><tr><th>IP</th><th>Method</th><th>Path</th><th>Status</th><th>Size</th></tr></thead><tbody>'+recentRows+'</tbody></table>':
        '<div style="color:var(--muted);font-size:13px;padding:20px 0;text-align:center">No recent requests in log</div>')+
    '</div>';
}
function nginxSitesTab(){
  if(S.editingSite){
    return'<div>'+
      '<div style="display:flex;align-items:center;gap:10px;margin-bottom:16px">'+
        '<button class="btn btn-sm" onclick="set({editingSite:null,editingContent:\'\'})">← Back</button>'+
        '<span style="font-weight:600;font-family:var(--mono)">'+S.editingSite+'</span>'+
        '<div style="flex:1"></div>'+
        '<button class="btn btn-sm" onclick="ngxTest()">Test</button>'+
        '<button class="btn-primary btn-sm" onclick="ngxSaveSite()">Save</button>'+
      '</div>'+
      '<textarea class="editor" style="min-height:500px;height:calc(100vh - 280px)" oninput="S.editingContent=this.value">'+escHtml(S.editingContent)+'</textarea>'+
    '</div>';
  }
  if(S.newSiteMode){
    return'<div>'+
      '<div style="display:flex;align-items:center;gap:10px;margin-bottom:20px">'+
        '<button class="btn btn-sm" onclick="set({newSiteMode:false})">← Back</button>'+
        '<span style="font-weight:600">New Site Config</span>'+
      '</div>'+
      '<form onsubmit="ngxCreateSite(event)" class="card" style="max-width:600px">'+
        '<div class="grid2">'+
          '<div class="fg"><label>Filename *</label><input name="sitename" placeholder="myapp.conf" required></div>'+
          '<div class="fg"><label>Server Name *</label><input name="server_name" placeholder="app.example.com" required></div>'+
        '</div>'+
        '<div class="grid2">'+
          '<div class="fg"><label>Listen Port</label><input name="port" type="number" value="80"></div>'+
          '<div class="fg"><label>Upstream (host:port)</label><input name="upstream" placeholder="localhost:3000"></div>'+
        '</div>'+
        '<div style="display:flex;gap:8px;justify-content:flex-end">'+
          '<button type="button" class="btn" onclick="set({newSiteMode:false})">Cancel</button>'+
          '<button type="submit" class="btn-primary">Create Site</button>'+
        '</div>'+
      '</form></div>';
  }
  const sites=S.nginxSites;
  return'<div>'+
    '<div style="display:flex;justify-content:flex-end;margin-bottom:16px">'+
      '<button class="btn-primary btn-sm" onclick="set({newSiteMode:true})">+ New Site</button>'+
    '</div>'+
    (sites.length===0?'<div class="card" style="text-align:center;padding:40px;color:var(--muted)">No site configs found</div>':
      '<div style="display:grid;gap:8px">'+
      sites.map(s=>'<div class="card" style="padding:14px 18px">'+
        '<div style="display:flex;align-items:center;gap:12px">'+
          '<div style="flex:1;min-width:0">'+
            '<div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap">'+
              '<span style="font-weight:600;font-size:13px;font-family:var(--mono)">'+s.name+'</span>'+
              (s.enabled?'<span class="tag tag-active">enabled</span>':'<span class="tag tag-stopped">disabled</span>')+
            '</div>'+
            '<div style="color:var(--muted);font-size:11px;margin-top:3px">'+s.path+'</div>'+
          '</div>'+
          '<div style="display:flex;gap:5px;flex-shrink:0">'+
            '<button class="btn btn-sm" onclick="ngxEditSite(\''+s.name+'\')">Edit</button>'+
            (s.enabled?
              '<button class="btn btn-sm" onclick="ngxToggleSite(\''+s.name+'\',true)">Disable</button>':
              '<button class="btn-success btn-sm" onclick="ngxToggleSite(\''+s.name+'\',false)">Enable</button>')+
            '<button class="btn-danger btn-sm" onclick="ngxDeleteSite(\''+s.name+'\')">Delete</button>'+
          '</div>'+
        '</div></div>').join('')+
      '</div>')+
  '</div>';
}
function nginxConfigTab(){
  return'<div>'+
    '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:12px">'+
      '<span style="font-size:12px;color:var(--muted);font-family:var(--mono)">/etc/nginx/nginx.conf</span>'+
      '<div style="display:flex;gap:8px">'+
        '<button class="btn btn-sm" onclick="ngxTest()">Test Config</button>'+
        '<button class="btn-primary btn-sm" onclick="ngxSaveMainConfig()">Save</button>'+
      '</div></div>'+
    '<textarea class="editor" style="height:calc(100vh - 280px)" oninput="S.nginxMainConfig=this.value">'+escHtml(S.nginxMainConfig)+'</textarea>'+
  '</div>';
}
function nginxLogsTab(){
  return'<div>'+
    '<div style="display:flex;gap:8px;margin-bottom:16px;flex-wrap:wrap">'+
      '<button class="btn btn-sm" onclick="openNginxLogs(\'access\')"><i class="icon-play" style="font-size:11px"></i> Stream Access</button>'+
      '<button class="btn btn-sm" onclick="openNginxLogs(\'error\')"><i class="icon-play" style="font-size:11px"></i> Stream Error</button>'+
      '<button class="btn btn-sm" onclick="loadNginxLogs(\'access\')">Load Access (200 lines)</button>'+
      '<button class="btn btn-sm" onclick="loadNginxLogs(\'error\')">Load Error (200 lines)</button>'+
    '</div>'+
    '<div style="background:var(--surface);border:1px solid var(--border);border-radius:var(--r2);overflow:hidden">'+
      '<pre style="padding:16px;font-size:12px;font-family:var(--mono);height:calc(100vh - 320px);overflow-y:auto;white-space:pre-wrap;word-break:break-all;color:#a8b4c8;line-height:1.6">'+
        (S.nginxLogs.length?escHtml(S.nginxLogs.join('\n')):'Click a button above to load logs')+
      '</pre></div></div>';
}
// ── Logs page ─────────────────────────────────────────────────────────────────
function pageLogs(){
  const t=S.logsTarget;
  return'<div style="height:calc(100vh - 56px);display:flex;flex-direction:column">'+
    '<div style="display:flex;align-items:center;gap:12px;margin-bottom:20px;flex-shrink:0">'+
      '<button class="btn btn-sm" onclick="nav(\'containers\')">← Back</button>'+
      '<h1 style="font-size:18px;font-weight:700">'+(t?t.name:'Logs')+'</h1>'+
      '<span style="margin-left:auto;font-size:11px;color:var(--green);display:flex;align-items:center;gap:5px">'+
        '<span class="dot pulse" style="background:var(--green)"></span>live</span>'+
    '</div>'+
    '<div style="background:var(--surface);border:1px solid var(--border);border-radius:var(--r2);overflow:hidden;flex:1;display:flex;flex-direction:column">'+
      '<div style="padding:10px 16px;border-bottom:1px solid var(--border);display:flex;justify-content:space-between;align-items:center;flex-shrink:0">'+
        '<span style="font-size:11px;color:var(--muted);font-family:var(--mono)">stdout / stderr</span>'+
        '<button class="btn btn-xs" onclick="S.logs=\'\';const el=document.getElementById(\'logbox\');if(el)el.textContent=\'\'">Clear</button>'+
      '</div>'+
      '<pre id="logbox" style="padding:16px;font-size:12px;font-family:var(--mono);flex:1;overflow-y:auto;white-space:pre-wrap;word-break:break-all;color:#a8b4c8;line-height:1.7;margin:0">'+
        (S.logs||'Connecting…')+'</pre>'+
    '</div></div>';
}
// ── Settings page ─────────────────────────────────────────────────────────────
function pageSettings(){
  function row(l,v){return'<div style="display:flex;justify-content:space-between;align-items:center;padding:12px 0;border-bottom:1px solid var(--border)"><span style="color:var(--muted);font-size:13px">'+l+'</span><code style="font-family:var(--mono);font-size:12px;color:var(--accent2)">'+v+'</code></div>'}
  return'<div>'+
    '<div style="margin-bottom:24px"><h1 style="font-size:22px;font-weight:700;letter-spacing:-.5px">Settings</h1></div>'+
    '<div class="card" style="max-width:520px">'+
      row('Version','0.1.0')+row('Data directory','/var/lib/vessel')+
      row('Config file','/etc/vessel/config.yaml')+row('UI port','4800')+
      '<div style="margin-top:20px;padding-top:20px;border-top:1px solid var(--border);display:flex;gap:10px">'+
        '<a href="https://github.com/Yash121l/Vessel" target="_blank" class="btn btn-sm"><i class="icon-github" style="font-size:13px"></i> View on GitHub</a>'+
      '</div>'+
    '</div></div>';
}
// ── Boot ──────────────────────────────────────────────────────────────────────
set({});
load();
</script>
</body>
</html>`
