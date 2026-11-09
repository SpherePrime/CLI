import * as THREE from 'three';
import {OrbitControls} from 'three/addons/controls/OrbitControls.js';
import {RoomEnvironment} from 'three/addons/environments/RoomEnvironment.js';
import gsap from 'gsap';
import {createWheel,setExplosion} from './models.js';
import {createInlineSix} from './inline-six.js';
import {createBMW} from './bmw.js';
import {createSuspension} from './suspension.js';

const factories={car:createBMW,engine:createInlineSix,wheel:createWheel,suspension:createSuspension};
const reduced=window.matchMedia('(prefers-reduced-motion: reduce)');

export class MechanicalScene{
  constructor(container,kind){
    this.container=container;this.kind=kind;this.visible=true;this.exploded=false;this.progress={value:0};this.auto=false;this.last=0;this.time=0;
    this.renderer=new THREE.WebGLRenderer({antialias:true,alpha:true,powerPreference:'high-performance'});
    this.renderer.setPixelRatio(Math.min(devicePixelRatio,1.5));this.renderer.outputColorSpace=THREE.SRGBColorSpace;this.renderer.toneMapping=THREE.ACESFilmicToneMapping;this.renderer.toneMappingExposure=kind==='car'?1.12:kind==='engine'?1.04:1.30;this.renderer.shadowMap.enabled=true;this.renderer.shadowMap.type=THREE.PCFSoftShadowMap;container.appendChild(this.renderer.domElement);
    this.scene=new THREE.Scene();this.camera=new THREE.PerspectiveCamera(kind==='car'?27:34,1,.1,80);
    const pmrem=new THREE.PMREMGenerator(this.renderer),room=new RoomEnvironment();this.environment=pmrem.fromScene(room,.04);this.scene.environment=this.environment.texture;this.scene.environmentIntensity=kind==='car'?1.25:kind==='engine'?.95:1.7;room.dispose();pmrem.dispose();
    this.scene.add(new THREE.HemisphereLight(kind==='car'?'#ffffff':'#d6e2dd',kind==='car'?'#9a9f99':'#333a32',kind==='car'?2:kind==='engine'?.9:1.8));
    const key=new THREE.DirectionalLight('#fff6e5',kind==='car'?4.1:kind==='engine'?3:4.5);key.position.set(3,8,5);key.castShadow=true;key.shadow.mapSize.set(kind==='car'?2048:1024,kind==='car'?2048:1024);key.shadow.camera.left=-7;key.shadow.camera.right=7;key.shadow.camera.top=7;key.shadow.camera.bottom=-7;key.shadow.normalBias=.035;key.shadow.bias=-.0001;this.scene.add(key);
    const rim=new THREE.DirectionalLight('#d5e6ff',kind==='engine'?1.8:3);rim.position.set(-5,4,-3);this.scene.add(rim);const edge=new THREE.DirectionalLight('#ffd6b2',kind==='engine'?.8:1.5);edge.position.set(4,1,-5);this.scene.add(edge);
    this.model=factories[kind]();this.scene.add(this.model.group);this.ready=this.model.ready??Promise.resolve(this.model);
    if(kind==='wheel')this.model.group.scale.setScalar(2.15);
    if(kind==='engine')this.model.group.scale.setScalar(1.12);
    if(kind==='suspension')this.model.group.scale.setScalar(1.25);
    const floor=new THREE.Mesh(new THREE.PlaneGeometry(60,60),new THREE.ShadowMaterial({opacity:kind==='car'?.19:.24}));floor.rotation.x=-Math.PI/2;floor.position.y=kind==='car'?-.055:kind==='wheel'?-1.39:kind==='engine'?-2.6:-2.90;floor.receiveShadow=true;this.scene.add(floor);
    this.controls=new OrbitControls(this.camera,this.renderer.domElement);this.controls.enableDamping=true;this.controls.dampingFactor=.045;this.controls.enablePan=false;this.controls.autoRotateSpeed=.5;this.controls.minPolarAngle=.18;this.controls.maxPolarAngle=Math.PI*.49;
    this.controls.minDistance=kind==='car'?7:4;this.controls.maxDistance=kind==='car'?28:19;this.controls.zoomSpeed=.55;
    this.resize();this.controls.saveState();this.sizeObserver=new ResizeObserver(()=>this.resize());this.sizeObserver.observe(container);this.visibilityObserver=new IntersectionObserver(([entry])=>this.visible=entry.isIntersecting,{rootMargin:'160px'});this.visibilityObserver.observe(container);
    this.frame=this.frame.bind(this);requestAnimationFrame(this.frame);
    if(kind==='car'&&!reduced.matches){gsap.from(this.model.group.rotation,{y:-.6,duration:2.5,ease:'power3.out'});gsap.from(this.model.group.position,{y:-.35,duration:2.2,ease:'power3.out'});}
  }
  resize(){
    const width=this.container.clientWidth,height=this.container.clientHeight;if(!width||!height)return;
    this.renderer.setSize(width,height);this.camera.aspect=width/height;this.camera.updateProjectionMatrix();
    const factor=this.kind==='car'?Math.max(1,1.5/this.camera.aspect):Math.max(1,1.15/this.camera.aspect);
    if(this.kind==='car'){this.camera.position.set(6.2*factor,3.25*factor,7.6*factor);this.controls.target.set(0,.68,0);}
    if(this.kind==='engine'){this.camera.position.set(-3.8*factor,2.8*factor,5.4*factor);this.controls.target.set(0,.35,0);}
    if(this.kind==='wheel'){this.camera.position.set(4.0*factor,1.8*factor,5.3*factor);this.controls.target.set(0,0,0);}
    if(this.kind==='suspension'){this.camera.position.set(4.6*factor,3.0*factor,6.6*factor);this.controls.target.set(0,.35,0);}
    this.controls.update();this.controls.saveState();
  }
  toggle(){
    this.exploded=!this.exploded;gsap.to(this.progress,{value:this.exploded?1:0,duration:reduced.matches?0:this.kind==='engine'?3.1:2.6,ease:'power3.inOut',overwrite:true});return this.exploded;
  }
  rotate(value){this.auto=value;}
  reset(){this.controls.reset();}
  frame(now){
    requestAnimationFrame(this.frame);const delta=Math.min((now-this.last)/1000,.035);this.last=now;if(document.hidden||!this.visible)return;this.time+=delta;
    setExplosion(this.model,this.progress.value);
    this.controls.target.y=(this.kind==="car"?.68:this.kind==="engine"?.35:this.kind==="suspension"?.35:0)+this.progress.value*(this.kind==="car"?.65:this.kind==="engine"?.32:this.kind==="suspension"?.75:0);this.controls.autoRotate=this.auto&&!reduced.matches;this.controls.update(delta);
    const targetZoom=1-this.progress.value*(this.kind==='car'?.25:this.kind==='engine'?.38:this.kind==='wheel'?.34:.29);this.camera.zoom=targetZoom;this.camera.updateProjectionMatrix();
    this.renderer.render(this.scene,this.camera);
  }
}


