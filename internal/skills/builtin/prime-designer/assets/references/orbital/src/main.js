import './style.css';
import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import gsap from 'gsap';
import { ScrollTrigger } from 'gsap/ScrollTrigger';
gsap.registerPlugin(ScrollTrigger);

const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)');
const worlds = [
  { name: 'Aurelia', palette: ['#f4d4a0', '#b98558', '#40382b'], temp: '+28°C', radius: '6 840 км', day: '38 часов', coord: '28° 42′ N · 16° 08′ E', description: 'Мир золотых дюн и бесконечного рассвета. Тёплый свет пронизывает атмосферу, а медные кольца хранят историю миллионов лет.', short: 'Мир золотых дюн и бесконечного рассвета.', seed: 0.4 },
  { name: 'Nereia', palette: ['#94e9e1', '#246f81', '#102940'], temp: '−12°C', radius: '9 210 км', day: '26 часов', coord: '62° 18′ N · 04° 37′ E', description: 'Бескрайний океан под ледяным небом. В его глубинах рождается мягкое сияние, а каждый прилив звучит как дыхание планеты.', short: 'Бескрайний океан и холодное биолюминесцентное сияние.', seed: 3.8 },
  { name: 'Obsidian', palette: ['#ff8242', '#893923', '#1e1719'], temp: '+840°C', radius: '4 370 км', day: '17 часов', coord: '09° 51′ S · 78° 24′ E', description: 'Тёмный камень, живое пламя и необузданная энергия. Светящаяся лава рисует новые континенты на поверхности молодого мира.', short: 'Вулканический мир тёмного камня и живого пламени.', seed: 8.2 },
];
let selectedWorld = 0;
const vertexShader = `varying vec3 vNormal; varying vec3 vPosition; varying vec3 vWorld;
void main(){vNormal=normalize(normalMatrix*normal);vPosition=position;vWorld=(modelMatrix*vec4(position,1.)).xyz;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}`;
const fragmentShader = `precision highp float;
varying vec3 vNormal; varying vec3 vPosition; varying vec3 vWorld;
uniform vec3 uLight; uniform vec3 uMid; uniform vec3 uDark; uniform float uSeed; uniform float uTime;
float hash(vec3 p){p=fract(p*.3183099+vec3(.1,.2,.3));p*=17.;return fract(p.x*p.y*p.z*(p.x+p.y+p.z));}
float noise(vec3 p){vec3 i=floor(p);vec3 f=fract(p);f=f*f*(3.-2.*f);return mix(mix(mix(hash(i),hash(i+vec3(1,0,0)),f.x),mix(hash(i+vec3(0,1,0)),hash(i+vec3(1,1,0)),f.x),f.y),mix(mix(hash(i+vec3(0,0,1)),hash(i+vec3(1,0,1)),f.x),mix(hash(i+vec3(0,1,1)),hash(i+vec3(1,1,1)),f.x),f.y),f.z);}
float fbm(vec3 p){float s=0.;float a=.5;for(int i=0;i<6;i++){s+=a*noise(p);p=p*2.06+1.7;a*=.5;}return s;}
void main(){vec3 p=vPosition*2.2+uSeed;float n=fbm(p*2.7);float bands=sin(vPosition.y*23.+n*16.+uSeed)*.5+.5;float fine=fbm(p*34.);float terrain=mix(n,bands,.23);vec3 col=mix(uDark,uMid,smoothstep(.22,.55,terrain));col=mix(col,uLight,smoothstep(.42,.78,terrain));col*=.80+fine*.38;float diffuse=max(dot(normalize(vNormal),normalize(vec3(-.65,.65,1.))),0.);float edge=pow(1.-max(dot(normalize(vNormal),vec3(0,0,1)),0.),3.);col*=.14+diffuse*1.13;col+=uLight*edge*.09;gl_FragColor=vec4(col,1.);}`;

function createPlanet(index = 0) {
  const world = worlds[index];
  const group = new THREE.Group();
  const material = new THREE.ShaderMaterial({vertexShader,fragmentShader,uniforms:{uLight:{value:new THREE.Color(world.palette[0])},uMid:{value:new THREE.Color(world.palette[1])},uDark:{value:new THREE.Color(world.palette[2])},uSeed:{value:world.seed},uTime:{value:0}}});
  const sphere = new THREE.Mesh(new THREE.SphereGeometry(1.7,96,64),material);
  sphere.rotation.z = -.13;
  group.add(sphere);
  const atmosphere = new THREE.Mesh(new THREE.SphereGeometry(1.735,64,48),new THREE.ShaderMaterial({vertexShader,fragmentShader:`varying vec3 vNormal;uniform vec3 color;void main(){float rim=pow(1.-abs(dot(normalize(vNormal),vec3(0.,0.,1.))),3.);gl_FragColor=vec4(color,rim*.17);}`,uniforms:{color:{value:new THREE.Color(world.palette[0])}},transparent:true,side:THREE.BackSide,depthWrite:false,blending:THREE.AdditiveBlending}));
  group.add(atmosphere);
  const rings = new THREE.Group();
  rings.rotation.set(1.12,.25,-.32);
  const ringMaterial = new THREE.ShaderMaterial({side:THREE.DoubleSide,transparent:true,depthWrite:false,uniforms:{color:{value:new THREE.Color('#c5a476')}},vertexShader:`varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}`,fragmentShader:`varying vec2 vUv;uniform vec3 color;void main(){float stripe=sin(vUv.x*900.)*.5+.5;float fade=smoothstep(0.,.08,vUv.x)*(1.-smoothstep(.85,1.,vUv.x));gl_FragColor=vec4(color,fade*(.025+stripe*.1));}`});
  const geometry = new THREE.RingGeometry(2.05,3.5,200,1);
  const uv=geometry.attributes.uv;const positions=geometry.attributes.position;
  for(let i=0;i<uv.count;i++){const radius=Math.hypot(positions.getX(i),positions.getY(i));uv.setXY(i,(radius-2.05)/1.45,0);}
  rings.add(new THREE.Mesh(geometry,ringMaterial));
  for(const radius of [2.04,2.3,2.73,3.17,3.51]){const pts=[];for(let i=0;i<=256;i++){const angle=i/256*Math.PI*2;pts.push(new THREE.Vector3(Math.cos(angle)*radius,Math.sin(angle)*radius,0));}rings.add(new THREE.Line(new THREE.BufferGeometry().setFromPoints(pts),new THREE.LineBasicMaterial({color:'#a5906d',transparent:true,opacity:radius===3.51?.25:.14})));}
  group.add(rings);
  const moon = new THREE.Mesh(new THREE.SphereGeometry(.12,24,16),new THREE.MeshStandardMaterial({color:'#a6977c',roughness:1}));
  moon.position.set(2.65,.6,.7);group.add(moon);
  return {group,sphere,material,rings,atmosphere,moon,ringMaterial};
}

function makeRenderer(container, preview = false) {
  const renderer = new THREE.WebGLRenderer({antialias:true,alpha:true,powerPreference:'high-performance',preserveDrawingBuffer:preview});
  renderer.setPixelRatio(Math.min(window.devicePixelRatio,preview?1.5:1.8));
  renderer.setSize(container.clientWidth,container.clientHeight);
  renderer.outputColorSpace=THREE.SRGBColorSpace;
  container.appendChild(renderer.domElement);
  return renderer;
}

let renderer,scene,camera,planet,heroVisible=true;
const pointer={x:0,y:0};
const sceneContainer=document.querySelector('#space-scene');
try {
  renderer=makeRenderer(sceneContainer);
  scene=new THREE.Scene();
  camera=new THREE.PerspectiveCamera(35,sceneContainer.clientWidth/sceneContainer.clientHeight,.1,100);
  camera.position.z=12;
  scene.add(new THREE.AmbientLight('#ffe5c4',1.4));
  const light=new THREE.DirectionalLight('#ffd9ae',3);light.position.set(-4,5,6);scene.add(light);
  planet=createPlanet();scene.add(planet.group);
  const starPositions=new Float32Array(1300*3);const starColors=new Float32Array(1300*3);
  for(let i=0;i<1300;i++){starPositions[i*3]=(Math.random()-.5)*45;starPositions[i*3+1]=(Math.random()-.5)*26;starPositions[i*3+2]=-Math.random()*20-3;const brightness=.3+Math.random()*.5;starColors.set([brightness,brightness*.95,brightness*.8],i*3);}
  const starGeometry=new THREE.BufferGeometry();starGeometry.setAttribute('position',new THREE.BufferAttribute(starPositions,3));starGeometry.setAttribute('color',new THREE.BufferAttribute(starColors,3));
  const stars=new THREE.Points(starGeometry,new THREE.PointsMaterial({size:.024,vertexColors:true,transparent:true,opacity:.65,sizeAttenuation:true}));scene.add(stars);
  const resize=()=>{const w=sceneContainer.clientWidth,h=sceneContainer.clientHeight;renderer.setSize(w,h);camera.aspect=w/h;camera.updateProjectionMatrix();if(w<700){planet.group.position.set(.25,-1.45,0);planet.group.scale.setScalar(.82);}else{planet.group.position.set(2.05,.05,0);planet.group.scale.setScalar(w<1050?.92:1.12);}};
  resize();new ResizeObserver(resize).observe(sceneContainer);
  sceneContainer.addEventListener('pointermove',(event)=>{pointer.x=(event.clientX/window.innerWidth-.5)*2;pointer.y=(event.clientY/window.innerHeight-.5)*2;});
  sceneContainer.addEventListener('pointerleave',()=>{pointer.x=0;pointer.y=0;});
  new IntersectionObserver(([entry])=>{heroVisible=entry.isIntersecting;},{rootMargin:'100px'}).observe(sceneContainer);
  let previous=0;
  const animate=(time)=>{requestAnimationFrame(animate);if(document.hidden||!heroVisible)return;const delta=Math.min((time-previous)/1000,.05);previous=time;if(!reducedMotion.matches){planet.sphere.rotation.y+=delta*.075;planet.group.rotation.y+=(pointer.x*.09-planet.group.rotation.y)*.025;planet.group.rotation.x+=(-pointer.y*.055-planet.group.rotation.x)*.025;stars.rotation.y=Math.sin(time*.00002)*.015;planet.moon.position.y=.6+Math.sin(time*.0003)*.18;}renderer.render(scene,camera);};requestAnimationFrame(animate);
  // Render thumbnails with the same actual 3D materials, then release their WebGL context.
  const previewContainer=document.createElement('div');previewContainer.style.cssText='width:640px;height:460px;position:fixed;left:-2000px';document.body.appendChild(previewContainer);
  const previewRenderer=makeRenderer(previewContainer,true);const previewScene=new THREE.Scene();const previewCamera=new THREE.PerspectiveCamera(35,640/460,.1,100);previewCamera.position.z=10.5;previewScene.add(new THREE.AmbientLight('#ffedca',2));
  const previewLight=new THREE.DirectionalLight('#ffedca',4);previewLight.position.set(-4,4,6);previewScene.add(previewLight);
  worlds.forEach((world,index)=>{const p=createPlanet(index);p.group.scale.setScalar(.85);p.group.rotation.z=-.1;previewScene.add(p.group);previewRenderer.render(previewScene,previewCamera);const img=new Image();img.src=previewRenderer.domElement.toDataURL('image/webp');img.alt=`Объёмная планета ${world.name} с орбитальными кольцами`;document.querySelector(`#planet-preview-${index}`).appendChild(img);previewScene.remove(p.group);p.group.traverse(obj=>{obj.geometry?.dispose();if(obj.material) obj.material.dispose();});});
  previewRenderer.dispose();previewRenderer.forceContextLoss();previewContainer.remove();
}catch(error){console.warn('WebGL unavailable; displaying fallback.',error);sceneContainer.innerHTML='<div class="webgl-fallback" aria-hidden="true"></div>';showNotice('3D недоступно в этом браузере. Содержание и выбор миров доступны.');}

function transitionPlanet(target,index){
  if(!target)return;const world=worlds[index];const duration=reducedMotion.matches?0:1.4;
  ['uLight','uMid','uDark'].forEach((key,i)=>{const color=new THREE.Color(world.palette[i]);gsap.to(target.material.uniforms[key].value,{r:color.r,g:color.g,b:color.b,duration});});
  gsap.to(target.material.uniforms.uSeed,{value:world.seed,duration});const color=new THREE.Color(world.palette[0]);gsap.to(target.atmosphere.material.uniforms.color.value,{r:color.r,g:color.g,b:color.b,duration});gsap.to(target.sphere.rotation,{y:target.sphere.rotation.y+.65,duration,ease:'power2.inOut'});
}

function selectWorld(index){
  selectedWorld=(index+worlds.length)%worlds.length;const world=worlds[selectedWorld];
  document.querySelectorAll('[data-world]').forEach(button=>{const active=Number(button.dataset.world)===selectedWorld;button.classList.toggle('active',active);button.classList.toggle('selected',active);button.setAttribute('aria-pressed',String(active));});
  document.querySelector('#hero-world-name').textContent=world.name.toUpperCase();document.querySelector('#hero-world-coord').textContent=world.coord;document.querySelector('#hero-index').textContent=String(selectedWorld+1).padStart(2,'0');document.querySelector('#selected-world-description').textContent=`На вашей орбите — ${world.name}. ${world.short}`;
  transitionPlanet(planet,selectedWorld);transitionPlanet(explorePlanet,selectedWorld);updateExplore();
}
document.querySelectorAll('[data-world]').forEach(button=>button.addEventListener('click',()=>selectWorld(Number(button.dataset.world))));

const dialog=document.querySelector('#explore-dialog');let exploreRenderer,exploreScene,exploreCamera,explorePlanet,controls;
function updateExplore(){const world=worlds[selectedWorld];document.querySelector('#explore-title').textContent=world.name;document.querySelector('#explore-description').textContent=world.description;document.querySelector('#fact-temperature').textContent=world.temp;document.querySelector('#fact-radius').textContent=world.radius;document.querySelector('#fact-day').textContent=world.day;document.querySelector('#explore-index').textContent=`${String(selectedWorld+1).padStart(2,'0')} / 03`;}
function openExplore(){
  updateExplore();dialog.showModal();document.body.style.overflow='hidden';
  if(!exploreRenderer&&renderer){
    const container=document.querySelector('#explore-scene');exploreRenderer=makeRenderer(container);exploreScene=new THREE.Scene();exploreCamera=new THREE.PerspectiveCamera(40,container.clientWidth/container.clientHeight,.1,80);exploreCamera.position.set(0,0,10);
    exploreScene.add(new THREE.AmbientLight('#ffe3bd',2));const light=new THREE.DirectionalLight('#ffd4ad',3);light.position.set(-4,5,6);exploreScene.add(light);explorePlanet=createPlanet(selectedWorld);exploreScene.add(explorePlanet.group);
    controls=new OrbitControls(exploreCamera,exploreRenderer.domElement);controls.enableDamping=true;controls.enablePan=false;controls.minDistance=6;controls.maxDistance=15;controls.autoRotate=!reducedMotion.matches;controls.autoRotateSpeed=.4;
    const resize=()=>{const w=container.clientWidth,h=container.clientHeight;if(!w||!h)return;exploreRenderer.setSize(w,h);exploreCamera.aspect=w/h;exploreCamera.updateProjectionMatrix();explorePlanet.group.position.set(w<700?0:1,w<700?1.7:.6,0);explorePlanet.group.scale.setScalar(w<700?.68:1);};resize();new ResizeObserver(resize).observe(container);
    let last=0;function frame(time){requestAnimationFrame(frame);if(!dialog.open||document.hidden)return;controls.autoRotate=!reducedMotion.matches;controls.update(Math.min((time-last)/1000,.05));last=time;exploreRenderer.render(exploreScene,exploreCamera);}requestAnimationFrame(frame);
  }
  if(explorePlanet)transitionPlanet(explorePlanet,selectedWorld);
  if(exploreCamera&&!reducedMotion.matches)gsap.fromTo(exploreCamera.position,{z:14},{z:10,duration:1.8,ease:'power3.out'});
}
document.querySelectorAll('[data-explore]').forEach(button=>button.addEventListener('click',openExplore));
document.querySelector('#close-explore').addEventListener('click',()=>dialog.close());
dialog.addEventListener('close',()=>{document.body.style.overflow='';});
dialog.addEventListener('click',event=>{if(event.target===dialog){const rect=dialog.getBoundingClientRect();if(event.clientX<rect.left||event.clientX>rect.right||event.clientY<rect.top||event.clientY>rect.bottom)dialog.close();}});
document.querySelector('#previous-world').addEventListener('click',()=>selectWorld(selectedWorld-1));document.querySelector('#next-world').addEventListener('click',()=>selectWorld(selectedWorld+1));

let audioContext,audioGain,audioOn=false;
document.querySelector('.sound-control').addEventListener('click',async()=>{
  try{if(!audioContext){audioContext=new (window.AudioContext||window.webkitAudioContext)();audioGain=audioContext.createGain();audioGain.gain.value=0;audioGain.connect(audioContext.destination);[55,82.41,110,164.81].forEach((frequency,index)=>{const osc=audioContext.createOscillator();osc.type='sine';osc.frequency.value=frequency;const gain=audioContext.createGain();gain.gain.value=.12/(index+1);osc.connect(gain);gain.connect(audioGain);osc.start();});}await audioContext.resume();audioOn=!audioOn;audioGain.gain.setTargetAtTime(audioOn?.28:0,audioContext.currentTime,.7);document.querySelector('.sound-control').setAttribute('aria-pressed',String(audioOn));document.querySelector('.sound-text').textContent=audioOn?'ЗВУК ВКЛ.':'ЗВУК ВЫКЛ.';}catch{showNotice('Браузер не поддерживает воспроизведение атмосферы.');}
});
document.addEventListener('visibilitychange',()=>{if(audioGain)audioGain.gain.setTargetAtTime(document.hidden?0:audioOn?.28:0,audioContext.currentTime,.3);});

const menu=document.querySelector('.menu-toggle'),mobileNav=document.querySelector('.mobile-nav');menu.addEventListener('click',()=>{const open=menu.getAttribute('aria-expanded')!=='true';menu.setAttribute('aria-expanded',String(open));menu.setAttribute('aria-label',open?'Закрыть меню':'Открыть меню');mobileNav.hidden=!open;});mobileNav.querySelectorAll('a').forEach(link=>link.addEventListener('click',()=>{mobileNav.hidden=true;menu.setAttribute('aria-expanded','false');menu.setAttribute('aria-label','Открыть меню');}));
function showNotice(message){const notice=document.querySelector('.notice');notice.textContent=message;notice.classList.add('visible');setTimeout(()=>notice.classList.remove('visible'),5000);}
gsap.matchMedia().add('(prefers-reduced-motion: no-preference)',()=>{gsap.from('.intro',{y:28,opacity:0,duration:1.3,stagger:.12,ease:'power3.out',delay:.15});document.querySelectorAll('.reveal').forEach(element=>gsap.from(element,{y:45,opacity:0,duration:1,ease:'power3.out',scrollTrigger:{trigger:element,start:'top 92%',once:true}}));gsap.to('.page-progress',{scaleX:1,ease:'none',scrollTrigger:{trigger:document.body,start:'top top',end:'bottom bottom',scrub:true}});});
document.querySelector('#year').textContent=new Date().getFullYear();

// Spatial hover on precise pointers; touch remains a direct, stable selection.
document.querySelectorAll('.world-card').forEach(card=>{
  card.addEventListener('pointermove',event=>{
    if(event.pointerType!=='mouse'||reducedMotion.matches)return;
    const rect=card.getBoundingClientRect();
    gsap.to(card,{rotationY:(event.clientX-rect.left-rect.width/2)/rect.width*9,rotationX:-(event.clientY-rect.top-rect.height/2)/rect.height*7,transformPerspective:1000,y:-8,duration:.45,ease:'power2.out'});
  });
  card.addEventListener('pointerleave',()=>gsap.to(card,{rotationX:0,rotationY:0,y:0,duration:reducedMotion.matches?0:.6,ease:'power3.out'}));
});
