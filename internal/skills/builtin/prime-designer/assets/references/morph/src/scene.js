import * as THREE from 'three';
import {RoomEnvironment} from 'three/addons/environments/RoomEnvironment.js';
import {OrbitControls} from 'three/addons/controls/OrbitControls.js';
import gsap from 'gsap';
import {createSculpture,animateSculpture,applyMaterial,disposeSculpture} from './sculptures.js';

const reduced=window.matchMedia('(prefers-reduced-motion: reduce)');

export class SculptureScene {
  constructor(container,{index=0,interactive=false,dark=false,hero=false}={}){
    this.container=container;this.hero=hero;this.dark=dark;this.interactive=interactive;this.visible=true;this.active=true;this.pointer={x:0,y:0};this.time=0;this.last=0;this.style='chrome';this.transitioning=false;this.scrollExplosion={value:0};
    this.renderer=new THREE.WebGLRenderer({alpha:true,antialias:true,powerPreference:'high-performance'});
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio,1.6));this.renderer.outputColorSpace=THREE.SRGBColorSpace;this.renderer.toneMapping=THREE.ACESFilmicToneMapping;this.renderer.toneMappingExposure=1.15;
    this.renderer.shadowMap.enabled=true;this.renderer.shadowMap.type=THREE.PCFSoftShadowMap;
    container.appendChild(this.renderer.domElement);
    this.scene=new THREE.Scene();this.camera=new THREE.PerspectiveCamera(hero?34:38,1,.1,80);this.camera.position.set(0,.3,hero?10.7:9);
    const pmrem=new THREE.PMREMGenerator(this.renderer);const environment=new RoomEnvironment();this.envTarget=pmrem.fromScene(environment,.04);this.scene.environment=this.envTarget.texture;environment.dispose();pmrem.dispose();
    this.scene.add(new THREE.HemisphereLight('#f7f1ff',dark?'#303929':'#8d8786',2));
    const key=new THREE.DirectionalLight('#ffffff',5);key.position.set(-3,7,5);key.castShadow=true;key.shadow.mapSize.set(1024,1024);key.shadow.camera.left=-8;key.shadow.camera.right=8;key.shadow.camera.top=8;key.shadow.camera.bottom=-8;key.shadow.normalBias=.035;key.shadow.bias=-.0002;key.shadow.radius=4;this.scene.add(key);
    const rim=new THREE.DirectionalLight('#d1b9ff',5);rim.position.set(5,1,-3);this.scene.add(rim);
    const lime=new THREE.DirectionalLight('#d7ff83',2);lime.position.set(-4,-1,2);this.scene.add(lime);
    this.anchor=new THREE.Group();this.scene.add(this.anchor);this.model=createSculpture(index);this.anchor.add(this.model.group);
    if(!dark){const floor=new THREE.Mesh(new THREE.PlaneGeometry(100,100),new THREE.ShadowMaterial({opacity:.13}));floor.rotation.x=-Math.PI/2;floor.position.y=-2.6;floor.receiveShadow=true;this.scene.add(floor);}
    if(interactive){this.controls=new OrbitControls(this.camera,this.renderer.domElement);this.controls.enableDamping=true;this.controls.enablePan=false;this.controls.minDistance=5.5;this.controls.maxDistance=15;this.controls.autoRotate=!reduced.matches;this.controls.autoRotateSpeed=.7;}
    this.resize();this.resizeObserver=new ResizeObserver(()=>this.resize());this.resizeObserver.observe(container);
    this.observer=new IntersectionObserver(([entry])=>{this.visible=entry.isIntersecting;},{rootMargin:'100px'});this.observer.observe(container);
    container.addEventListener('pointermove',event=>{const r=container.getBoundingClientRect();this.pointer.x=(event.clientX-r.left)/r.width-.5;this.pointer.y=(event.clientY-r.top)/r.height-.5;});container.addEventListener('pointerleave',()=>{this.pointer.x=0;this.pointer.y=0;});
    this.frame=this.frame.bind(this);this.raf=requestAnimationFrame(this.frame);
    if(hero&&!reduced.matches){gsap.from(this.anchor.scale,{x:.55,y:.55,z:.55,duration:2.2,ease:'power3.out'});gsap.from(this.model.group.rotation,{y:this.model.group.rotation.y+1.7,duration:2.6,ease:'power3.out'});}
  }
  resize(){const width=this.container.clientWidth,height=this.container.clientHeight;if(!width||!height)return;this.renderer.setSize(width,height);this.camera.aspect=width/height;this.camera.updateProjectionMatrix();
    if(this.hero){if(width<700){this.anchor.position.set(.05,-1.05,0);this.anchor.scale.setScalar(.67);}else{this.anchor.position.set(2.08,.05,0);this.anchor.scale.setScalar(width<1050?1:1.17);}}
    else if(this.interactive){const mobile=width<700;this.anchor.position.set(mobile?0:1.25,mobile?1.5:.4,0);this.anchor.scale.setScalar(mobile?.65:1.15);}
    else{this.anchor.position.set(0,.05,0);this.anchor.scale.setScalar(.95);}
  }
  frame(timestamp){this.raf=requestAnimationFrame(this.frame);const dt=Math.min((timestamp-this.last)/1000,.04);this.last=timestamp;if(document.hidden||!this.visible||!this.active)return;this.time+=dt;
    if(this.controls){this.controls.autoRotate=!reduced.matches;this.controls.update(dt);}
    else if(!reduced.matches&&!this.transitioning){this.anchor.rotation.y+=(this.pointer.x*.32-this.anchor.rotation.y)*.035;this.anchor.rotation.x+=(-this.pointer.y*.14-this.anchor.rotation.x)*.035;this.model.group.rotation.y+=dt*.12;this.anchor.position.z=Math.sin(this.time*.7)*.08;}
    if(this.hero)this.model.explosion.value=reduced.matches?0:this.scrollExplosion.value;
    animateSculpture(this.model,this.time,!reduced.matches);this.renderer.render(this.scene,this.camera);
  }
  setObject(index){
    if(this.model.index===index)return;
    if(this.transitioning){gsap.killTweensOf(this.model.group.scale);this.anchor.remove(this.model.group);disposeSculpture(this.model);this.model=createSculpture(index);applyMaterial(this.model,this.style);this.anchor.add(this.model.group);this.transitioning=false;return;}
    this.transitioning=true;const old=this.model;const duration=reduced.matches?0:.45;
    gsap.to(old.group.scale,{x:.001,y:.001,z:.001,duration,ease:'power3.in',onComplete:()=>{this.anchor.remove(old.group);disposeSculpture(old);this.model=createSculpture(index);applyMaterial(this.model,this.style);this.anchor.add(this.model.group);gsap.fromTo(this.model.group.scale,{x:.001,y:.001,z:.001},{x:1,y:1,z:1,duration:reduced.matches?0:1.1,ease:'elastic.out(1,.8)',onComplete:()=>{this.transitioning=false;}});}});
  }
  setMaterial(style){this.style=style;applyMaterial(this.model,style);}
  explode(value){gsap.to(this.model.explosion,{value,duration:reduced.matches?0:1.7,ease:'power3.inOut'});if(this.controls){const goal=this.camera.position.clone().normalize().multiplyScalar(value?12:9);gsap.to(this.camera.position,{x:goal.x,y:goal.y,z:goal.z,duration:reduced.matches?0:1.7,ease:'power3.inOut'});}}
  reset(){if(this.controls){this.controls.reset();gsap.to(this.camera.position,{x:0,y:.3,z:9,duration:reduced.matches?0:1.2,ease:'power3.out'});}this.explode(0);}
}

export function renderPreviews(){
  const renderer=new THREE.WebGLRenderer({antialias:true,alpha:true,preserveDrawingBuffer:true});renderer.setSize(640,480);renderer.setPixelRatio(1);renderer.outputColorSpace=THREE.SRGBColorSpace;renderer.toneMapping=THREE.ACESFilmicToneMapping;renderer.toneMappingExposure=1.15;
  const scene=new THREE.Scene();const pmrem=new THREE.PMREMGenerator(renderer),environment=new RoomEnvironment(),target=pmrem.fromScene(environment,.04);scene.environment=target.texture;environment.dispose();pmrem.dispose();
  scene.add(new THREE.HemisphereLight('#faf6ff','#8e8b7c',2));const light=new THREE.DirectionalLight('#ffffff',4);light.position.set(-3,5,5);scene.add(light);const rim=new THREE.DirectionalLight('#ceb0ff',4);rim.position.set(5,2,-2);scene.add(rim);
  const camera=new THREE.PerspectiveCamera(35,640/480,.1,50);camera.position.set(0,.15,8.5);
  for(let i=0;i<3;i++){const model=createSculpture(i);model.group.scale.setScalar(.88);scene.add(model.group);renderer.render(scene,camera);const image=new Image();image.src=renderer.domElement.toDataURL('image/webp');image.alt=['Объёмная хромированная лента Мёбиуса','Конструкция из острых металлических кристаллов','Закрученная башня из восемнадцати пластин'][i];document.querySelector(`#preview-${i}`).appendChild(image);scene.remove(model.group);disposeSculpture(model);}
  target.dispose();renderer.dispose();renderer.forceContextLoss();
}
