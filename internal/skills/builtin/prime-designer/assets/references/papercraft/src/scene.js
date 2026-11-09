import * as THREE from 'three';
import {OrbitControls} from 'three/addons/controls/OrbitControls.js';
import gsap from 'gsap';
import {createPaper,posePaper,recolor,disposePaper} from './paper.js';
const reduced=window.matchMedia('(prefers-reduced-motion: reduce)');
export class PaperScene{
  constructor(container,state,hero=false){this.container=container;this.state=state;this.hero=hero;this.visible=true;this.pointer={x:0,y:0};this.time=0;this.last=0;this.transitioning=false;
    this.renderer=new THREE.WebGLRenderer({alpha:true,antialias:true,powerPreference:'high-performance'});this.renderer.setPixelRatio(Math.min(devicePixelRatio,1.6));this.renderer.outputColorSpace=THREE.SRGBColorSpace;this.renderer.toneMapping=THREE.ACESFilmicToneMapping;this.renderer.toneMappingExposure=1.12;this.renderer.shadowMap.enabled=true;this.renderer.shadowMap.type=THREE.PCFSoftShadowMap;container.appendChild(this.renderer.domElement);
    this.scene=new THREE.Scene();this.camera=new THREE.PerspectiveCamera(hero?34:37,1,.1,60);this.scene.add(new THREE.HemisphereLight('#fff5ee','#6577b2',2.6));
    const key=new THREE.DirectionalLight('#fff1de',4);key.position.set(-3,7,5);key.castShadow=true;key.shadow.mapSize.set(1024,1024);key.shadow.camera.left=-8;key.shadow.camera.right=8;key.shadow.camera.top=8;key.shadow.camera.bottom=-8;key.shadow.normalBias=.04;this.scene.add(key);const rim=new THREE.DirectionalLight('#c2ddff',2);rim.position.set(5,2,-4);this.scene.add(rim);
    this.anchor=new THREE.Group();this.scene.add(this.anchor);this.model=createPaper(state.shape,state.color);this.anchor.add(this.model.group);
    const floor=new THREE.Mesh(new THREE.PlaneGeometry(80,80),new THREE.ShadowMaterial({opacity:.14}));floor.rotation.x=-Math.PI/2;floor.position.y=-1.55;floor.receiveShadow=true;this.scene.add(floor);
    if(!hero){this.controls=new OrbitControls(this.camera,this.renderer.domElement);this.controls.enablePan=false;this.controls.enableDamping=true;this.controls.minDistance=5;this.controls.maxDistance=14;this.controls.autoRotate=false;}
    this.resize();new ResizeObserver(()=>this.resize()).observe(container);new IntersectionObserver(([e])=>this.visible=e.isIntersecting,{rootMargin:'150px'}).observe(container);
    container.addEventListener('pointermove',e=>{const r=container.getBoundingClientRect();this.pointer.x=(e.clientX-r.left)/r.width-.5;this.pointer.y=(e.clientY-r.top)/r.height-.5;});container.addEventListener('pointerleave',()=>{this.pointer.x=0;this.pointer.y=0;});this.frame=this.frame.bind(this);requestAnimationFrame(this.frame);
    if(hero&&!reduced.matches){gsap.from(this.anchor.position,{y:this.anchor.position.y-1.3,duration:2.3,ease:'power3.out'});gsap.from(this.anchor.rotation,{y:1.5,duration:2.3,ease:'power3.out'});}
  }
  resize(){const w=this.container.clientWidth,h=this.container.clientHeight;if(!w||!h)return;this.renderer.setSize(w,h);this.camera.aspect=w/h;this.camera.updateProjectionMatrix();
    if(this.hero){this.camera.position.set(0,2.2,10.5);this.camera.lookAt(0,0,0);if(w<700){this.anchor.position.set(.08,-1.22,0);this.anchor.scale.setScalar(.62);}else{this.anchor.position.set(1.8,-.1,0);this.anchor.scale.setScalar(w<1050?1:1.12);}}
    else{this.camera.position.set(0,2.5,w<450?9.5:8);this.camera.lookAt(0,0,0);this.anchor.position.y=.6;this.anchor.scale.setScalar(w<450?.72:1.05);if(this.controls){this.controls.target.set(0,.6,0);this.controls.saveState();}}
  }
  setShape(index){if(this.model.index===index)return;gsap.killTweensOf(this.anchor.scale);this.anchor.remove(this.model.group);disposePaper(this.model);this.model=createPaper(index,this.state.color);this.anchor.add(this.model.group);
    if(!reduced.matches)gsap.fromTo(this.model.group.rotation,{y:this.model.group.rotation.y-.8},{y:this.model.group.rotation.y,duration:1.25,ease:'power3.out'});
  }
  reset(){if(this.controls)this.controls.reset();}
  frame(time){requestAnimationFrame(this.frame);const dt=Math.min((time-this.last)/1000,.04);this.last=time;if(document.hidden||!this.visible)return;this.time+=dt;
    if(this.controls)this.controls.update();else if(!reduced.matches){this.anchor.rotation.y+=(this.pointer.x*.35-this.anchor.rotation.y)*.03;this.anchor.rotation.x+=(-this.pointer.y*.12-this.anchor.rotation.x)*.03;this.anchor.position.z=Math.sin(this.time*.6)*.06;}
    posePaper(this.model,this.state.opening,this.time,this.state.motion&&!reduced.matches);this.renderer.render(this.scene,this.camera);
  }
  setColor(color){recolor(this.model,color);}
}
export function foldPreviews(){const r=new THREE.WebGLRenderer({alpha:true,antialias:true,preserveDrawingBuffer:true});r.setSize(640,400);r.outputColorSpace=THREE.SRGBColorSpace;r.toneMapping=THREE.ACESFilmicToneMapping;r.toneMappingExposure=1.12;const scene=new THREE.Scene(),camera=new THREE.PerspectiveCamera(35,1.6,.1,30);camera.position.set(3,3.5,6);camera.lookAt(0,0,0);scene.add(new THREE.HemisphereLight('#fff9ef','#9cadd3',3));const light=new THREE.DirectionalLight('#ffffff',3);light.position.set(-3,5,4);scene.add(light);
  for(let i=0;i<3;i++){const model=createPaper(2,'pink');posePaper(model,[1,.55,.12][i],0,false);scene.add(model.group);r.render(scene,camera);const img=new Image();img.src=r.domElement.toDataURL('image/webp');img.alt=['Почти плоский бумажный лист с линиями сгиба','Бумажная форма с частично сложенными гранями','Объёмная складчатая гармошка'][i];document.querySelector(`#fold-preview-${i}`).appendChild(img);scene.remove(model.group);disposePaper(model);}r.dispose();r.forceContextLoss();}


