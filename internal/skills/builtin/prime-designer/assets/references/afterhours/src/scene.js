import * as THREE from 'three';
import {RoomEnvironment} from 'three/addons/environments/RoomEnvironment.js';
import gsap from 'gsap';
import {createTurntable,createCassette,createSpeakers,animateModel} from './models.js';
const reduced=window.matchMedia('(prefers-reduced-motion: reduce)');
export class AudioScene {
  constructor(container,kind,state){
    this.container=container;this.state=state;this.kind=kind;this.time=0;this.last=0;this.pointer={x:0,y:0};this.visible=true;
    this.renderer=new THREE.WebGLRenderer({alpha:true,antialias:true,powerPreference:'high-performance'});this.renderer.setPixelRatio(Math.min(devicePixelRatio,1.6));this.renderer.outputColorSpace=THREE.SRGBColorSpace;this.renderer.toneMapping=THREE.ACESFilmicToneMapping;this.renderer.toneMappingExposure=1.35;this.renderer.shadowMap.enabled=true;this.renderer.shadowMap.type=THREE.PCFSoftShadowMap;container.appendChild(this.renderer.domElement);
    this.scene=new THREE.Scene();this.camera=new THREE.PerspectiveCamera(kind==='turntable'?33:36,1,.1,60);
    const pmrem=new THREE.PMREMGenerator(this.renderer),room=new RoomEnvironment();this.environment=pmrem.fromScene(room,.03);this.scene.environment=this.environment.texture;this.scene.environmentIntensity=.8;room.dispose();pmrem.dispose();
    this.scene.add(new THREE.HemisphereLight('#f6bf85','#28130f',2));
    const key=new THREE.DirectionalLight('#ffc194',5);key.position.set(-4,6,6);key.castShadow=true;key.shadow.mapSize.set(1024,1024);key.shadow.camera.left=-9;key.shadow.camera.right=9;key.shadow.camera.top=9;key.shadow.camera.bottom=-9;key.shadow.normalBias=.05;this.scene.add(key);
    const red=new THREE.DirectionalLight('#e7663d',5);red.position.set(4,2,-3);this.scene.add(red);const fill=new THREE.DirectionalLight('#d6cecd',2);fill.position.set(-2,1,-2);this.scene.add(fill);
    this.anchor=new THREE.Group();this.scene.add(this.anchor);this.model=kind==='turntable'?createTurntable():kind==='cassette'?createCassette():createSpeakers();this.anchor.add(this.model.group);
    const floor=new THREE.Mesh(new THREE.PlaneGeometry(100,100),new THREE.ShadowMaterial({opacity:.18}));floor.rotation.x=-Math.PI/2;floor.position.y=kind==='turntable'?-.85:-1.85;floor.receiveShadow=true;this.scene.add(floor);
    this.scroll={value:0};this.resize();new ResizeObserver(()=>this.resize()).observe(container);
    new IntersectionObserver(([entry])=>{this.visible=entry.isIntersecting;},{rootMargin:'100px'}).observe(container);
    container.addEventListener('pointermove',event=>{const r=container.getBoundingClientRect();this.pointer.x=(event.clientX-r.left)/r.width-.5;this.pointer.y=(event.clientY-r.top)/r.height-.5;});container.addEventListener('pointerleave',()=>{this.pointer.x=0;this.pointer.y=0;});
    this.frame=this.frame.bind(this);requestAnimationFrame(this.frame);
    if(kind==='turntable'&&!reduced.matches){gsap.from(this.anchor.position,{y:this.anchor.position.y-1.1,duration:2,ease:'power3.out'});gsap.from(this.anchor.rotation,{y:.7,duration:2.3,ease:'power3.out'});gsap.from(this.model.lid.rotation,{x:0,duration:2.1,delay:.5,ease:'power3.inOut'});}
  }
  resize(){const w=this.container.clientWidth,h=this.container.clientHeight;if(!w||!h)return;this.renderer.setSize(w,h);this.camera.aspect=w/h;this.camera.updateProjectionMatrix();
    if(this.kind==='turntable'){this.camera.position.set(0,5.7,10.8);this.camera.lookAt(0,0,0);if(w<700){this.anchor.position.set(0,-1.7,.65);this.anchor.scale.setScalar(.5);}else{this.anchor.position.set(2.15,-.35,0);this.anchor.scale.setScalar(w<1050?.92:1.08);}}
    else{this.camera.position.set(0,.25,this.kind==='cassette'?6.9:7.8);this.camera.lookAt(0,0,0);this.anchor.scale.setScalar(this.kind==='cassette'?1.1:.97);}
  }
  setPlaying(playing){if(this.kind==='turntable'){gsap.to(this.model.tonearm.rotation,{y:playing?-.36:.25,duration:reduced.matches?0:1.8,ease:'power2.inOut'});gsap.to(this.model.speed,{value:playing?3.49:0,duration:reduced.matches?0:2.6,ease:'power2.inOut'});}}
  openLid(open){if(this.kind==='turntable')gsap.to(this.model.lid.rotation,{x:open?-1.17:0,duration:reduced.matches?0:1.7,ease:'power3.inOut'});}
  explode(value){if(this.kind==='cassette'){gsap.to(this.model.explosion,{value,duration:reduced.matches?0:1.8,ease:'power3.inOut'});gsap.to(this.camera.position,{z:value?8.1:6.9,duration:reduced.matches?0:1.8,ease:'power3.inOut'});}}
  frame(timestamp){requestAnimationFrame(this.frame);const dt=Math.min((timestamp-this.last)/1000,.04);this.last=timestamp;if(document.hidden||!this.visible)return;this.time+=dt;
    if(!reduced.matches){this.anchor.rotation.y+=(this.pointer.x*.22+(this.kind==='turntable'?this.scroll.value*.2:Math.sin(this.time*.28)*.12)-this.anchor.rotation.y)*.035;this.anchor.rotation.x+=(-this.pointer.y*.12-this.anchor.rotation.x)*.035;}
    const pulse=this.state.playing?Math.exp(-((this.state.beatPhase??0)*8)):0;animateModel(this.model,this.time,this.state.playing,pulse,!reduced.matches,dt);this.renderer.render(this.scene,this.camera);
  }
}
