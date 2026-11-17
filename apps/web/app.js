import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';

// All cell-space coordinates are schematic renderer units, never PDB coordinates.
// Atomic structure inspection uses external experimental PDB entries in Mol*.
const el = (id) => document.getElementById(id);
const KIND_ICON = {
  ribosome: '◉', rna_polymerase: '◈', atp_synthase: '✳', nucleoid: '⌁', membrane: '⬡',
};
const MEDIUM_DESCRIPTIONS = {
  rich: 'Substrate availability supports active metabolism and cell growth.',
  limited: 'A small substrate feed slows translation and growth as glucose is consumed.',
  starved: 'No new glucose is supplied. ATP and translation activity gradually decline.',
};
const CHARTS = [
  ['atp-chart', 'atp_mM', '#f7be78'],
  ['glucose-chart', 'glucose_mM', '#53d7c3'],
  ['biomass-chart', 'biomass_fg', '#c494ee'],
  ['translation-chart', 'translation_activity', '#91aff5'],
];

let catalog = [];
let manifest = null;
let selected = 'ribosome';
let activeSession = null;
let simulationMode = 'live';
let replayLabel = 'Illustrative trace';
let events = null;
let state = null;
let history = [];
let three = null;
let rotating = false;

async function getJSON(url, options) {
  const res = await fetch(url, options);
  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`;
    try { message = (await res.json()).error || message; } catch { /* preserve HTTP error */ }
    throw new Error(message);
  }
  if (res.status === 204) return null;
  return res.json();
}

function connection(text, on) {
  el('connection').textContent = text;
  el('conn-dot').classList.toggle('on', !!on);
}
function errorMessage(err) {
  console.error(err);
  connection('API OFFLINE', false);
  el('sim-title').textContent = err.message || 'Connection failed';
}

function buildStructureCards() {
  const list = el('structure-list');
  list.replaceChildren();
  for (const structure of catalog) {
    const b = document.createElement('button');
    b.className = 'structure-card';
    b.type = 'button';
    b.dataset.kind = structure.kind;
    b.setAttribute('role', 'option');
    const icon = document.createElement('span');
    icon.className = 'structure-icon';
    icon.textContent = KIND_ICON[structure.kind] || '◉';
    icon.style.color = structure.color;
    const stack = document.createElement('span');
    stack.className = 'text-stack';
    const title = document.createElement('strong');
    title.textContent = structure.name;
    const sub = document.createElement('small');
    sub.textContent = structure.category;
    stack.append(title, sub);
    const arrow = document.createElement('span');
    arrow.className = 'structure-arrow'; arrow.textContent = '›';
    b.append(icon, stack, arrow);
    b.addEventListener('click', () => selectStructure(structure.kind));
    list.append(b);
  }
  selectStructure(selected);
}
function selectStructure(kind) {
  selected = kind;
  document.querySelectorAll('.structure-card').forEach(b => {
    const active = b.dataset.kind === kind;
    b.classList.toggle('active', active);
    b.setAttribute('aria-selected', String(active));
  });
  const info = catalog.find(x => x.kind === kind);
  if (!info) return;
  const panel = el('structure-detail');
  panel.replaceChildren();
  function node(tag, cls, text) {
    const e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text !== undefined) e.textContent = text;
    panel.append(e);
    return e;
  }
  node('span', 'detail-chip', info.category.toUpperCase());
  node('h4', '', info.name);
  node('p', '', info.description);
  for (const [label, value] of [
    ['Localization', info.location],
    ['PDB accession', info.pdb_id || 'Schematic only'],
    ['Structure strain', info.pdb_strain],
    ['Cell placement', 'Illustrative / procedural'],
  ]) {
    const row = node('div', 'detail-row');
    const left = document.createElement('span'); left.textContent = label;
    const right = document.createElement('strong'); right.textContent = value;
    row.append(left, right);
  }
  const inspect = node('button', 'inspect-btn', info.pdb_id ? 'Inspect atomic structure ↗' : 'No atomistic structure in this scene');
  inspect.type = 'button';
  inspect.disabled = !info.pdb_id;
  inspect.addEventListener('click', () => openInspector(info));
  three?.highlight(kind);
}

function createViewer(data) {
  const canvas = el('scene-canvas');
  const container = el('scene-wrap');
  const renderer = new THREE.WebGLRenderer({canvas, antialias: true, alpha: true, powerPreference: 'high-performance'});
  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
  renderer.outputColorSpace = THREE.SRGBColorSpace;
  renderer.toneMapping = THREE.ACESFilmicToneMapping;
  renderer.toneMappingExposure = 1.32;
  const world = new THREE.Scene();
  world.fog = new THREE.FogExp2('#091924', 0.022);
  const camera = new THREE.PerspectiveCamera(44, 1, 0.1, 130);
  const initialCamera = new THREE.Vector3(9.1, 5.0, 10.6);
  camera.position.copy(initialCamera);
  const controls = new OrbitControls(camera, canvas);
  controls.enableDamping = true;
  controls.dampingFactor = 0.07;
  controls.minDistance = 5.3;
  controls.maxDistance = 28;
  controls.enablePan = true;
  controls.autoRotateSpeed = 0.7;
  controls.target.set(0, 0, 0);
  controls.update();
  world.add(new THREE.HemisphereLight('#8cdff5', '#081621', 2.4));
  const key = new THREE.DirectionalLight('#e4fff8', 3.5); key.position.set(1, 6, 8); world.add(key);
  const rim = new THREE.PointLight('#53d9c0', 50, 25); rim.position.set(-1, -3, -6); world.add(rim);
  const warm = new THREE.PointLight('#e3a5e9', 35, 22); warm.position.set(6, 2, -4); world.add(warm);

  const grid = new THREE.GridHelper(42, 42, '#224e60', '#16404e');
  grid.position.y = -2.28;
  grid.material.opacity = 0.23; grid.material.transparent = true;
  world.add(grid);
  const starsGeometry = new THREE.BufferGeometry();
  const starLocations = new Float32Array(450 * 3);
  for (let i=0; i<450; i++) {
    // Deterministic decorative backdrop; unrelated to molecular data.
    const phi = i * 2.3999632, z = 1-2*(i+0.5)/450, r = Math.sqrt(1-z*z);
    starLocations[i*3]=r*Math.cos(phi)*24;
    starLocations[i*3+1]=z*19;
    starLocations[i*3+2]=r*Math.sin(phi)*24;
  }
  starsGeometry.setAttribute('position', new THREE.BufferAttribute(starLocations, 3));
  world.add(new THREE.Points(starsGeometry, new THREE.PointsMaterial({color:'#6c9fad', size:0.023, transparent:true, opacity:0.4, depthWrite:false})));

  const cell = new THREE.Group();
  world.add(cell);
  const layerGroups = {};
  for (const kind of ['membrane','nucleoid','ribosome','rna_polymerase','atp_synthase','flagella']) {
    const g = new THREE.Group(); g.userData.kind=kind; cell.add(g); layerGroups[kind]=g;
  }
  const clickable = [];
  const outerMat = new THREE.MeshPhysicalMaterial({color:'#36b8b1',transparent:true,opacity:.14,roughness:.25,metalness:.12,side:THREE.DoubleSide,depthWrite:false,clearcoat:1});
  const middleMat = new THREE.MeshBasicMaterial({color:'#7ce6d9',transparent:true,opacity:.085,wireframe:true,depthWrite:false});
  const innerMat = new THREE.MeshPhysicalMaterial({color:'#4cb9d2',transparent:true,opacity:.09,roughness:.38,side:THREE.DoubleSide,depthWrite:false});
  function envelope(radius,mat) {
    const mesh = new THREE.Mesh(new THREE.CapsuleGeometry(radius, 2*data.cell.body_half_length_units, 12, 48),mat);
    mesh.rotation.z = Math.PI/2;
    mesh.userData.kind='membrane';
    layerGroups.membrane.add(mesh); clickable.push(mesh);
    return mesh;
  }
  envelope(1.5,outerMat); envelope(1.43,middleMat); envelope(1.34,innerMat);
  // Small envelope rings lend 3D depth; not resolved peptidoglycan architecture.
  for(let i=-10;i<=10;i++) {
    const x=i*.24;
    const ring=new THREE.Mesh(new THREE.TorusGeometry(1.495,.004,4,70),new THREE.MeshBasicMaterial({color:'#72f1dc',transparent:true,opacity:.12,depthWrite:false}));
    ring.position.x=x;ring.rotation.y=Math.PI/2;layerGroups.membrane.add(ring);
  }

  // Looped stylized chromosome filaments. Not DNA sequence or measured fold.
  for(let strand=0;strand<13;strand++) {
    const points=[];
    for(let j=0;j<=150;j++) {
      const t=j/150, a=t*Math.PI*15+strand*.79;
      const x=-1.85+3.7*t;
      const radius=.34 + .14*Math.sin(2*Math.PI*t*2 + strand);
      points.push(new THREE.Vector3(x,Math.cos(a)*radius+.15*Math.sin(t*18+strand),Math.sin(a)*radius+.15*Math.cos(t*20+strand)));
    }
    const curve=new THREE.CatmullRomCurve3(points);
    const mesh=new THREE.Mesh(new THREE.TubeGeometry(curve,440,.009+strand%3*.002,5,false),new THREE.MeshStandardMaterial({color:strand%3===0?'#e1beec':'#95b9ff',emissive:'#4d6bbd',emissiveIntensity:.3,transparent:true,opacity:.72,roughness:.4}));
    mesh.userData.kind='nucleoid';layerGroups.nucleoid.add(mesh);clickable.push(mesh);
  }

  // Appendages: representative flagella and pili geometry.
  for(let strand=0;strand<5;strand++) {
    const points=[];const anchor=(strand%2===0?1:-1);
    for(let j=0;j<=190;j++) {
      const t=j/190, coil=t*19+strand*2.2;
      points.push(new THREE.Vector3(anchor*(3.8+t*3.3),.31*Math.sin(coil)+(.35*strand-.7)*(1-t*.35),.32*Math.cos(coil)+Math.sin(strand*2)*.6));
    }
    layerGroups.flagella.add(new THREE.Mesh(new THREE.TubeGeometry(new THREE.CatmullRomCurve3(points),240,.018,6,false),new THREE.MeshStandardMaterial({color:'#77cfcd',emissive:'#194d53',emissiveIntensity:.4,transparent:true,opacity:.78})));
  }
  for(let i=0;i<38;i++) {
    const x=-2.3 + i*.124;
    const a=i*2.39996;
    const v1=new THREE.Vector3(x,Math.sin(a)*1.49,Math.cos(a)*1.49);
    const v2=new THREE.Vector3(x + Math.sin(i*3)*.12,Math.sin(a)*1.91,Math.cos(a)*1.91);
    layerGroups.flagella.add(new THREE.Line(new THREE.BufferGeometry().setFromPoints([v1,v2]),new THREE.LineBasicMaterial({color:'#6baeb4',transparent:true,opacity:.37})));
  }

  const instanced = new Map();
  const kinds = ['ribosome','rna_polymerase','atp_synthase'];
  for (const kind of kinds) {
    const instances=data.instances.filter(m=>m.kind===kind);
    const info=catalog.find(x=>x.kind===kind);
    const geometry=new THREE.IcosahedronGeometry(1,kind==='ribosome'?1:2);
    const material=new THREE.MeshStandardMaterial({color:info?.color||'#aaf6e4',metalness:.14,roughness:.35,emissive:info?.color||'#64caba',emissiveIntensity:.12,transparent:true,opacity:.94});
    const mesh=new THREE.InstancedMesh(geometry,material,instances.length);
    mesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
    mesh.userData.kind=kind;
    layerGroups[kind].add(mesh);clickable.push(mesh);
    const dummy=new THREE.Object3D();
    instances.forEach((item,i)=>{
      dummy.position.set(...item.position);dummy.scale.setScalar(item.radius*1.2);
      dummy.rotation.set(0,item.phase,item.phase/2);dummy.updateMatrix();mesh.setMatrixAt(i,dummy.matrix);
    });
    mesh.instanceMatrix.needsUpdate=true;
    instanced.set(kind,{instances,mesh,dummy});
  }
  const cytoplasmPositions=new Float32Array(350*3);
  for(let i=0;i<350;i++) {
    let x,y,z;
    let radius;
    do {
      x=Math.sin(i*39.89)*3.7;
      y=Math.sin(i*13.37)*1.30;
      z=Math.cos(i*17.13)*1.30;
      radius=Math.abs(x)>2.45?Math.sqrt(Math.max(0,1.34**2-(Math.abs(x)-2.45)**2)):1.34;
    } while (Math.sqrt(y*y+z*z)>radius);
    cytoplasmPositions[3*i]=x;cytoplasmPositions[3*i+1]=y;cytoplasmPositions[3*i+2]=z;
  }
  const cytoGeo=new THREE.BufferGeometry();cytoGeo.setAttribute('position',new THREE.BufferAttribute(cytoplasmPositions,3));
  const cyto=new THREE.Points(cytoGeo,new THREE.PointsMaterial({size:.019,color:'#b9f7e4',transparent:true,opacity:.38,depthWrite:false}));
  layerGroups.membrane.add(cyto);

  function resize() {
    const rect=container.getBoundingClientRect();
    if(rect.width<10||rect.height<10)return;
    renderer.setSize(rect.width,rect.height,false);
    camera.aspect=rect.width/rect.height;camera.updateProjectionMatrix();
  }
  const observer = new ResizeObserver(resize); observer.observe(container);resize();
  const raycaster=new THREE.Raycaster();
  const mouse=new THREE.Vector2();
  let pointerDown=null;
  canvas.addEventListener('pointerdown',event=>{pointerDown={x:event.clientX,y:event.clientY};});
  canvas.addEventListener('pointerup',event=>{
    if(!pointerDown || Math.hypot(event.clientX-pointerDown.x,event.clientY-pointerDown.y)>6)return;
    const rect=canvas.getBoundingClientRect();
    mouse.set((event.clientX-rect.left)/rect.width*2-1,-(event.clientY-rect.top)/rect.height*2+1);
    raycaster.setFromCamera(mouse,camera);
    const hits=raycaster.intersectObjects(clickable.filter(o=>o.parent?.visible),false);
    // Pick nearest opaque molecule over the broad transparent membrane surface.
    const hit=hits.find(h=>h.object.userData.kind!=='membrane') || hits[0];
    if(hit?.object.userData.kind)selectStructure(hit.object.userData.kind);
  });
  canvas.addEventListener('pointerleave',()=>{pointerDown=null;});

  const dummy=new THREE.Object3D();
  const v3=new THREE.Vector3();
  let tickValue=0;
  function updateState(snapshot) {
    tickValue=snapshot.time_min;
    const baseScale = Math.max(0.93,Math.min(1.13,Math.cbrt(snapshot.biomass_fg/280)));
    cell.scale.set(baseScale,1,1);
    const active = snapshot.translation_activity;
    const atp = snapshot.energy_index;
    instanced.get('ribosome').mesh.material.emissiveIntensity=.09+.65*active;
    instanced.get('atp_synthase').mesh.material.emissiveIntensity=.10+.95*atp;
    instanced.get('rna_polymerase').mesh.material.emissiveIntensity=.09+.30*active;
    outerMat.opacity=.11 + .04*baseScale;
  }
  function highlight(kind) {
    instanced.forEach(({mesh},key)=>{
      mesh.material.opacity=key===kind?1:.78;
      mesh.material.emissiveIntensity=key===kind?.65:.14;
    });
  }
  let frame=0;
  function animate(t) {
    requestAnimationFrame(animate);
    const time=t*.001;
    controls.autoRotate=rotating;
    controls.update();
    // Animated motions are illustrative, correlated with the simulated state.
    const translation=state?.translation_activity ?? .45;
    const atp=state?.energy_index ?? .7;
    if(frame++%2===0){
      for(const [kind,{instances,mesh}] of instanced){
        const activity=kind==='ribosome'?translation:kind==='atp_synthase'?atp:translation*.65;
        for(let i=0;i<instances.length;i++){
          const item=instances[i];
          v3.set(...item.position);
          const pulse=1+0.15*activity*Math.sin(time*3.6+item.phase);
          dummy.position.copy(v3);
          dummy.rotation.set(0,item.phase+time*.05,item.phase/2);
          dummy.scale.setScalar(item.radius*1.2*pulse*(selected===kind?1.16:1));
          dummy.updateMatrix();mesh.setMatrixAt(i,dummy.matrix);
        }
        mesh.instanceMatrix.needsUpdate=true;
      }
    }
    layerGroups.flagella.rotation.x=Math.sin(time*(.22+atp*.6))*.055;
    layerGroups.nucleoid.rotation.x=Math.sin(tickValue*.025+time*.3)*.025;
    renderer.render(world,camera);
  }
  requestAnimationFrame(animate);
  el('reset-camera').addEventListener('click',()=>{
    camera.position.copy(initialCamera);controls.target.set(0,0,0);controls.update();
  });
  el('auto-rotate').addEventListener('click',()=>{
    rotating=!rotating;el('auto-rotate').classList.toggle('active',rotating);
  });
  document.querySelectorAll('[data-layer]').forEach(box=>{
    box.addEventListener('change',()=>{layerGroups[box.dataset.layer].visible=box.checked;});
  });
  return {highlight,updateState};
}

function plot(canvas, numbers, color) {
  const c=el(canvas);
  const ctx=c.getContext('2d');
  const w=c.width,h=c.height;
  ctx.clearRect(0,0,w,h);
  ctx.strokeStyle='#315361';ctx.lineWidth=1;
  for(const y of [12,36,60]){ctx.beginPath();ctx.moveTo(0,y);ctx.lineTo(w,y);ctx.stroke();}
  if(!numbers.length) return;
  let min=Math.min(...numbers),max=Math.max(...numbers);
  if(max-min<.01){max+=.02;min-=.02;}
  const gap=(max-min)*.18;min-=gap;max+=gap;
  const pts = numbers.map((v,i)=>[(i/Math.max(1,numbers.length-1))*w,h-6-(v-min)/(max-min)*(h-13)]);
  ctx.beginPath();
  pts.forEach(([x,y],i)=>{if(i===0)ctx.moveTo(x,y);else ctx.lineTo(x,y);});
  ctx.strokeStyle=color;ctx.lineWidth=2.7;ctx.stroke();
  ctx.lineTo(pts.at(-1)[0],h);ctx.lineTo(0,h);ctx.closePath();
  ctx.fillStyle=color+'21';ctx.fill();
  const [x,y]=pts.at(-1);
  ctx.beginPath();ctx.arc(x,y,3.4,0,Math.PI*2);ctx.fillStyle=color;ctx.fill();
}
function updateTelemetry(snapshot) {
  state=snapshot;
  el('time-value').textContent=String(Math.floor(snapshot.time_min/60)).padStart(2,'0')+':'+String(Math.floor(snapshot.time_min%60)).padStart(2,'0');
  el('atp-value').textContent=snapshot.atp_mM.toFixed(2);
  el('glucose-value').textContent=snapshot.glucose_mM.toFixed(2);
  el('biomass-value').textContent=snapshot.biomass_fg.toFixed(1);
  el('translation-value').textContent=snapshot.translation_activity.toFixed(2);
  el('sim-title').textContent=snapshot.running?(simulationMode==='replay'?'Replaying trajectory':'Simulation running'):(simulationMode==='replay'?'Replay paused':'Simulation paused');
  el('sim-indicator').classList.toggle('running',snapshot.running);
  el('play-icon').textContent=snapshot.running?'Ⅱ':'▶';
  el('play-label').textContent=snapshot.running?'Pause simulation':'Start simulation';
  el('medium-select').value=snapshot.medium;
  el('medium-description').textContent=MEDIUM_DESCRIPTIONS[snapshot.medium];
  if (!history.length || snapshot.time_min < history.at(-1).time_min)history=[];
  if (!history.length || history.at(-1).time_min!==snapshot.time_min){
    history.push(snapshot);
    if(history.length>100)history.shift();
  }
  for(const [canvas,key,color] of CHARTS)plot(canvas,history.map(x=>x[key]),color);
  three?.updateState(snapshot);
}
async function createSession(mode = 'live') {
  if(events)events.close();
  if(activeSession){
    try{await fetch('/api/simulations/'+activeSession,{method:'DELETE'});}catch{/* TTL removes abandoned sessions */}
  }
  const result=await getJSON('/api/simulations',{
    method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({medium:el('medium-select').value,mode})
  });
  activeSession=result.id;
  simulationMode=result.mode;
  el('medium-select').disabled=simulationMode==='replay';
  el('toggle-replay').textContent=simulationMode==='replay'?'↩ Return to live model':'↪ Load sample trajectory';
  el('toggle-replay').classList.toggle('active',simulationMode==='replay');
  el('model-hint').textContent=simulationMode==='replay' ? `Recorded trajectory · ${replayLabel}` : '2 simulated minutes per frame update (~0.45 sec real-time)';
  history=[];
  updateTelemetry(result.snapshot);
  el('toggle-sim').disabled=false;
  el('reset-sim').disabled=false;
  el('toggle-replay').disabled=false;
  events=new EventSource('/api/simulations/'+activeSession+'/events');
  events.addEventListener('state',ev=>{try{updateTelemetry(JSON.parse(ev.data));connection('API CONNECTED',true);}catch(e){console.error(e);}});
  events.onopen=()=>connection('API CONNECTED',true);
  events.onerror=()=>connection('RECONNECTING',false);
}
async function action(data) {
  if(!activeSession)return;
  try{
    const snapshot=await getJSON('/api/simulations/'+activeSession+'/actions',{
      method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(data)
    });
    updateTelemetry(snapshot);
  }catch(e){errorMessage(e);}
}

let molstarPromise;
let molstarViewer;
let inspectedPdb;
function loadMolstarScript() {
  if(molstarPromise)return molstarPromise;
  const css=document.createElement('link');css.rel='stylesheet';css.href='https://cdn.jsdelivr.net/npm/molstar@5.4.2/build/viewer/molstar.css';document.head.append(css);
  molstarPromise=new Promise((resolve,reject)=>{
    if(window.molstar?.Viewer){resolve();return;}
    const script=document.createElement('script');
    script.src='https://cdn.jsdelivr.net/npm/molstar@5.4.2/build/viewer/molstar.js';
    script.onload=resolve;script.onerror=()=>reject(new Error('Mol* download failed: network or CDN unavailable'));
    document.head.append(script);
  });
  return molstarPromise;
}
async function openInspector(info) {
  if(!info.pdb_id)return;
  el('inspector').hidden=false;
  el('inspector-title').textContent=`${info.name} · ${info.pdb_id}`;
  el('inspector-caption').textContent=`Experimental structure: ${info.pdb_strain}`;
  el('rcsb-link').href='https://www.rcsb.org/structure/'+encodeURIComponent(info.pdb_id);
  const loading=el('molstar-loading');loading.hidden=false;
  loading.textContent='Loading Mol* and the experimental structure…';
  try {
    await loadMolstarScript();
    if(!molstarViewer){
      molstarViewer=await window.molstar.Viewer.create('molstar-container',{
        layoutIsExpanded:false,layoutShowControls:true,layoutShowSequence:true,
        layoutShowLog:false,layoutShowLeftPanel:false,pdbProvider:'rcsb',
        viewportShowExpand:false,viewportShowSelectionMode:true
      });
    }
    if(info.pdb_id!==inspectedPdb){
      if(molstarViewer.plugin && typeof molstarViewer.plugin.clear==='function') await molstarViewer.plugin.clear();
      await molstarViewer.loadPdb(info.pdb_id.toLowerCase());
      inspectedPdb=info.pdb_id;
    }
    loading.hidden=true;
  } catch(err) {
    loading.hidden=false;
    loading.textContent=`Could not load Mol*: ${err.message}. You can still open the PDB entry using the link below.`;
  }
}
function bindControls(){
  el('toggle-sim').addEventListener('click',()=>action({action:state?.running?'pause':'start'}));
  el('reset-sim').addEventListener('click',()=>action({action:'reset'}));
  el('toggle-replay').addEventListener('click',async()=>{
    el('toggle-replay').disabled=true;
    try {await createSession(simulationMode==='replay'?'live':'replay');}
    catch(e){errorMessage(e);el('toggle-replay').disabled=false;}
  });
  el('medium-select').addEventListener('change',ev=>{
    el('medium-description').textContent=MEDIUM_DESCRIPTIONS[ev.target.value];
    action({action:'set_medium',medium:ev.target.value});
  });
  el('close-inspector').addEventListener('click',()=>{el('inspector').hidden=true;});
  el('inspector').addEventListener('click',ev=>{if(ev.target===el('inspector'))el('inspector').hidden=true;});
  document.addEventListener('keydown',ev=>{if(ev.key==='Escape')el('inspector').hidden=true;});
  window.addEventListener('pagehide',()=>{events?.close();});
}
async function bootstrap() {
  bindControls();
  try {
    const [cat,scene]=await Promise.all([getJSON('/api/catalog'),getJSON('/api/scene')]);
    catalog=cat.structures;manifest=scene;
    if(manifest.schema_version!==1)throw new Error('Unsupported scene version');
    buildStructureCards();
    try{three=createViewer(scene);three.highlight(selected);}catch(err){
      console.error('WebGL viewer failed:',err);el('three-error').hidden=false;
    }
    try {const info=await getJSON('/api/replay/info');if(info.available)replayLabel=info.source;}catch{ /* live mode still works */ }
    await createSession();
  }catch(err){errorMessage(err);}
}
bootstrap();
