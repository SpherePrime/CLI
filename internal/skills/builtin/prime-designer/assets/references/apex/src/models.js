import * as THREE from 'three';
import {RoundedBoxGeometry} from 'three/addons/geometries/RoundedBoxGeometry.js';

const standard=(color,metalness=0,roughness=.4)=>new THREE.MeshStandardMaterial({color,metalness,roughness});
const paint=new THREE.MeshPhysicalMaterial({color:'#a1a8a3',metalness:.82,roughness:.24,clearcoat:1,clearcoatRoughness:.13});
const metal=standard('#aeb3b4',.9,.25),dark=standard('#252827',.65,.35),rubber=standard('#171918',0,.86),orange=standard('#ff5727',.35,.34),black=standard('#111413',.2,.55),seat=standard('#95422e',0,.65);
const glass=new THREE.MeshPhysicalMaterial({color:'#172926',metalness:.3,roughness:.13,transparent:true,opacity:.74,side:THREE.DoubleSide,clearcoat:1,depthWrite:false});
const led=new THREE.MeshStandardMaterial({color:'#e8faff',emissive:'#e0f9ff',emissiveIntensity:3});
const tailLed=new THREE.MeshStandardMaterial({color:'#ff3b1e',emissive:'#ff230c',emissiveIntensity:2});

function mesh(geometry,material,parent,position=[0,0,0],rotation=[0,0,0]){const object=new THREE.Mesh(geometry,material);object.position.set(...position);object.rotation.set(...rotation);object.castShadow=true;object.receiveShadow=true;parent.add(object);return object;}
function box(parent,size,position,material=metal,radius=.03){return mesh(new RoundedBoxGeometry(...size,2,Math.min(radius,...size.map(v=>v*.24))),material,parent,position);}
function cylinder(parent,radius,length,position,material=metal,rotation=[0,0,0],segments=32){return mesh(new THREE.CylinderGeometry(radius,radius,length,segments),material,parent,position,rotation);}
function tube(parent,points,radius,material=metal){const curve=new THREE.CatmullRomCurve3(points.map(p=>new THREE.Vector3(...p)));return mesh(new THREE.TubeGeometry(curve,32,radius,8,false),material,parent);}
function surface(parent,vertices,indices,material){const geometry=new THREE.BufferGeometry();geometry.setAttribute('position',new THREE.Float32BufferAttribute(vertices,3));geometry.setIndex(indices);geometry.computeVertexNormals();return mesh(geometry,material,parent);}
function quad(parent,a,b,c,d,material){return surface(parent,[...a,...b,...c,...d],[0,1,2,0,2,3],material);}
function screw(parent,position,r=.035){cylinder(parent,r,.024,position,metal,[Math.PI/2,0,0],6);}
function assembly(){return {group:new THREE.Group(),parts:[],kind:''};}
function part(model,name,position,offset,rotation=[0,0,0]){const group=new THREE.Group();group.position.set(...position);group.name=name;model.group.add(group);model.parts.push({group,origin:group.position.clone(),offset:new THREE.Vector3(...offset),rotation:new THREE.Vector3(...rotation)});return group;}

export function createWheel(){
  const model=assembly();model.kind='wheel';
  const tire=part(model,'Performance tire',[0,0,0],[0,0,1.65]);
  mesh(new THREE.TorusGeometry(.47,.145,14,64),rubber,tire);mesh(new THREE.TorusGeometry(.445,.009,6,64),dark,tire,[0,0,.128]);mesh(new THREE.TorusGeometry(.445,.009,6,64),dark,tire,[0,0,-.128]);
  for(let i=0;i<48;i++){const a=i/48*Math.PI*2;const tread=box(tire,[.055,.017,.235],[Math.cos(a)*.611,Math.sin(a)*.611,0],rubber,.004);tread.rotation.z=a-Math.PI/2;}
  const rim=part(model,'Forged five-spoke rim',[0,0,.065],[0,0,.65]);
  mesh(new THREE.TorusGeometry(.38,.035,10,64),metal,rim,[0,0,.11]);mesh(new THREE.TorusGeometry(.38,.025,8,48),dark,rim,[0,0,-.09]);
  cylinder(rim,.12,.18,[0,0,0],metal,[Math.PI/2,0,0]);
  for(let i=0;i<5;i++){const a=i/5*Math.PI*2;const spoke=box(rim,[.105,.31,.085],[Math.sin(a)*.235,Math.cos(a)*.235,.07],metal,.018);spoke.rotation.z=-a;const cut=box(rim,[.025,.27,.09],[Math.sin(a+.085)*.235,Math.cos(a+.085)*.235,.065],dark,.006);cut.rotation.z=-a;const boltA=a+.32;screw(rim,[Math.sin(boltA)*.079,Math.cos(boltA)*.079,.105],.018);}
  cylinder(rim,.05,.01,[0,0,.11],orange,[Math.PI/2,0,0],32);
  const rotor=part(model,'Ventilated brake rotor',[0,0,-.055],[0,0,-.65]);
  cylinder(rotor,.327,.043,[0,0,0],metal,[Math.PI/2,0,0],64);cylinder(rotor,.12,.055,[0,0,0],dark,[Math.PI/2,0,0]);
  for(let i=0;i<28;i++){const a=i/28*Math.PI*2;for(const radius of [.235,.282])cylinder(rotor,.009,.045,[Math.cos(a)*radius,Math.sin(a)*radius,0],black,[Math.PI/2,0,0],6);}
  const caliper=part(model,'Six-piston caliper',[.28,.03,-.04],[.85,.3,-.45]);box(caliper,[.11,.33,.18],[0,0,0],orange,.04);box(caliper,[.12,.23,.045],[.006,0,.095],orange,.025);for(const y of [-.085,0,.085])cylinder(caliper,.035,.045,[.005,y,.08],dark,[Math.PI/2,0,0]);
  const hub=part(model,'Hub and bearing',[0,0,-.19],[0,0,-1.5]);cylinder(hub,.11,.25,[0,0,0],dark,[Math.PI/2,0,0]);cylinder(hub,.19,.045,[0,0,-.1],metal,[Math.PI/2,0,0]);for(let i=0;i<5;i++){const a=i/5*Math.PI*2;cylinder(hub,.018,.12,[Math.sin(a)*.145,Math.cos(a)*.145,-.02],metal,[Math.PI/2,0,0],8);}
  return model;
}

export function setExplosion(model,value){model.parts.forEach(({group,origin,offset,rotation},i)=>{const delay=model.kind==='car'?i*.008:i*.018;const t=THREE.MathUtils.clamp((value-delay)/(1-delay),0,1);group.position.copy(origin).addScaledVector(offset,t);const base=group.userData.baseRotation??(group.userData.baseRotation=group.rotation.clone());group.rotation.set(base.x+rotation.x*t,base.y+rotation.y*t,base.z+rotation.z*t);});}

export {assembly,part,box,cylinder,mesh,tube,metal,dark,rubber,orange};


