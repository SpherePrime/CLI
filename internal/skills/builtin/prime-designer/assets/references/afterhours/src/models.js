import * as THREE from 'three';

const copper=()=>new THREE.MeshStandardMaterial({color:'#ae6945',metalness:.78,roughness:.3});
const black=()=>new THREE.MeshStandardMaterial({color:'#151313',metalness:.35,roughness:.43});
const cream=()=>new THREE.MeshStandardMaterial({color:'#e8c6a0',metalness:.15,roughness:.45});
function mesh(geometry,material,parent,position=[0,0,0]){const object=new THREE.Mesh(geometry,material);object.position.set(...position);object.castShadow=true;object.receiveShadow=true;parent.add(object);return object;}
function box(parent,size,material,position){return mesh(new THREE.BoxGeometry(...size),material,parent,position);}
function cylinder(parent,radius,height,material,position,segments=64){return mesh(new THREE.CylinderGeometry(radius,radius,height,segments),material,parent,position);}
function rod(parent,a,b,radius,material){const start=new THREE.Vector3(...a),end=new THREE.Vector3(...b);const part=mesh(new THREE.CylinderGeometry(radius,radius,start.distanceTo(end),16),material,parent);part.position.copy(start).add(end).multiplyScalar(.5);part.quaternion.setFromUnitVectors(new THREE.Vector3(0,1,0),end.sub(start).normalize());return part;}
function typeTexture(title,subtitle,color='#e8cfac',background='#261814',width=1024,height=256){const canvas=document.createElement('canvas');canvas.width=width;canvas.height=height;const c=canvas.getContext('2d');c.fillStyle=background;c.fillRect(0,0,width,height);c.fillStyle=color;c.textAlign='center';let fontSize=height*.4;c.font=`italic ${fontSize}px Georgia`;while(c.measureText(title).width>width*.82){fontSize*=.9;c.font=`italic ${fontSize}px Georgia`;}c.fillText(title,width/2,height*.49);c.font=`${height*.065}px monospace`;c.fillText(subtitle,width/2,height*.72,width*.82);const texture=new THREE.CanvasTexture(canvas);texture.colorSpace=THREE.SRGBColorSpace;texture.anisotropy=4;return texture;}
function plate(parent,title,subtitle,size,position,color,background){const texture=typeTexture(title,subtitle,color,background);const object=mesh(new THREE.PlaneGeometry(...size),new THREE.MeshBasicMaterial({map:texture,side:THREE.DoubleSide}),parent,position);return object;}

export function createTurntable(){
  const group=new THREE.Group(),metal=copper(),dark=black(),light=cream();
  box(group,[4.9,.48,3.85],new THREE.MeshStandardMaterial({color:'#713b28',roughness:.56,metalness:.15}),[0,-.3,0]);
  box(group,[4.85,.11,3.8],metal,[0,-.005,0]);box(group,[4.54,.045,3.48],dark,[0,.072,0]);
  for(const x of [-1.95,1.95])for(const z of [-1.4,1.4])cylinder(group,.23,.25,dark,[x,-.64,z],24);
  const platter=cylinder(group,1.49,.16,metal,[-.6,.16,.1]);
  cylinder(group,1.43,.035,dark,[-.6,.26,.1]);
  const record=new THREE.Group();record.position.set(-.6,.3,.1);group.add(record);cylinder(record,1.4,.045,dark,[0,0,0]);
  const grooves=new THREE.MeshStandardMaterial({color:'#34302e',roughness:.29,metalness:.7});
  for(let i=0;i<44;i++){const ring=mesh(new THREE.TorusGeometry(.49+i*.0203,.0018,4,128),grooves,record,[0,.034,0]);ring.rotation.x=Math.PI/2;}
  const labelTexture=typeTexture('afterhours','SIDE A   /   MIDNIGHT JAZZ   /   33 RPM','#2b1610','#d88350',512,512);
  const label=mesh(new THREE.CircleGeometry(.445,64),new THREE.MeshStandardMaterial({map:labelTexture,roughness:.75}),record,[0,.032,0]);label.rotation.x=-Math.PI/2;
  cylinder(record,.035,.12,metal,[0,.045,0],16);
  const tonearm=new THREE.Group();tonearm.position.set(1.72,.22,-1.12);group.add(tonearm);
  cylinder(group,.25,.19,dark,[1.72,.15,-1.12],32);cylinder(tonearm,.11,.4,metal,[0,.1,0],24);
  rod(tonearm,[0,.24,0],[-.47,.24,1.48],.045,light);rod(tonearm,[-.47,.24,1.48],[-.73,.16,1.95],.045,light);
  box(tonearm,[.23,.105,.37],dark,[-.72,.115,1.94]);box(tonearm,[.18,.04,.24],metal,[-.72,.055,2.01]);tonearm.rotation.y=.25;
  const counter=cylinder(tonearm,.115,.4,dark,[.04,.24,-.35],32);counter.rotation.x=Math.PI/2;
  const controls=new THREE.Group();group.add(controls);
  for(const z of [.5,1.22]){cylinder(controls,.18,.14,metal,[1.85,.18,z],32);const tick=box(controls,[.02,.008,.11],light,[1.85,.255,z+.04]);}
  const led=cylinder(controls,.045,.022,new THREE.MeshStandardMaterial({color:'#e36442',emissive:'#d73c1c',emissiveIntensity:1.2}),[1.75,.13,1.68],16);
  const logo=plate(group,'afterhours','THE NIGHT PLAYER · AH 001',[1.75,.32],[0,-.25,1.936],'#e8c6a0','#713b28');
  const lid=new THREE.Group();lid.position.set(0,.08,-1.93);lid.rotation.x=-1.17;group.add(lid);
  const glass=new THREE.MeshPhysicalMaterial({color:'#ad7565',transparent:true,opacity:.13,roughness:.12,metalness:.08,clearcoat:1,side:THREE.DoubleSide,depthWrite:false});
  box(lid,[4.96,.035,3.88],glass,[0,.65,1.94]);box(lid,[.035,.65,3.88],glass,[-2.48,.325,1.94]);box(lid,[.035,.65,3.88],glass,[2.48,.325,1.94]);box(lid,[4.96,.65,.035],glass,[0,.325,3.88]);
  const border=new THREE.MeshStandardMaterial({color:'#bd8b70',metalness:.9,roughness:.25,transparent:true,opacity:.32});
  for(const x of [-2.48,2.48])rod(lid,[x,.65,0],[x,.65,3.88],.012,border);rod(lid,[-2.48,.65,3.88],[2.48,.65,3.88],.012,border);
  return {group,record,tonearm,lid,led,speed:{value:0},kind:'turntable'};
}

export function createCassette(){
  const group=new THREE.Group(),shell=cream(),dark=black(),metal=copper();
  const back=box(group,[3.9,2.48,.24],shell,[0,0,-.05]);
  const face=new THREE.Group();face.position.z=.14;group.add(face);
  const faceMaterial=new THREE.MeshStandardMaterial({color:'#a64833',roughness:.5,metalness:.1});
  box(face,[3.78,.64,.055],faceMaterial,[0,.845,0]);box(face,[3.78,.64,.055],faceMaterial,[0,-.845,0]);box(face,[.24,1.05,.055],faceMaterial,[-1.77,0,0]);box(face,[.24,1.05,.055],faceMaterial,[1.77,0,0]);
  plate(face,'afterhours','MIDNIGHT MIXTAPE — SIDE A',[3.45,.66],[0,.78,.035],'#ead0a9','#a64833');
  box(group,[3.29,.84,.02],dark,[0,-.05,.18]);
  const reels=[];
  for(const x of [-.96,.96]){
    const reel=new THREE.Group();reel.position.set(x,-.08,.24);group.add(reel);reels.push(reel);
    const tape=cylinder(reel,.46,.035,new THREE.MeshStandardMaterial({color:'#2b211a',roughness:.6}),[0,0,0]);tape.rotation.x=Math.PI/2;
    const rim=mesh(new THREE.TorusGeometry(.41,.025,8,64),shell,reel);cylinder(reel,.095,.055,metal,[0,0,.03],24).rotation.x=Math.PI/2;
    for(let i=0;i<8;i++){const a=i/8*Math.PI*2;const spoke=box(reel,[.29,.047,.025],shell,[Math.cos(a)*.23,Math.sin(a)*.23,.045]);spoke.rotation.z=a;}
    const hub=mesh(new THREE.TorusGeometry(.145,.025,8,32),shell,reel,[0,0,.05]);
  }
  for(const x of [-1.67,1.67])for(const y of [-.98,.98]){const screw=cylinder(face,.042,.026,metal,[x,y,.052],16);screw.rotation.x=Math.PI/2;box(face,[.05,.008,.012],dark,[x,y,.07]);}
  box(face,[1.4,.27,.02],shell,[0,-.85,.044]);plate(face,'90','CHROME / TYPE II',[1.2,.21],[0,-.83,.06],'#45271a','#e8c6a0');
  for(const x of [-1.27,-.7,.7,1.27]){const pin=cylinder(face,.052,.025,dark,[x,-.93,.05],20);pin.rotation.x=Math.PI/2;}
  const tapeLine=box(group,[1.1,.019,.025],new THREE.MeshStandardMaterial({color:'#61401e',roughness:.7}),[0,.32,.23]);
  const explosion={value:0};group.rotation.set(-.18,-.3,-.14);
  return {group,back,face,reels,explosion,kind:'cassette'};
}

function speaker(parent,x,rotation){
  const metal=copper(),dark=black(),wood=new THREE.MeshStandardMaterial({color:'#542d20',roughness:.65,metalness:.06});
  const group=new THREE.Group();group.position.set(x,0,0);group.rotation.y=rotation;parent.add(group);
  box(group,[1.62,3.3,1.22],wood,[0,0,0]);box(group,[1.43,3.1,.04],dark,[0,0,.63]);
  const cones=[];
  for(const [radius,y] of [[.57,-.58],[.31,.72]]){
    const driver=new THREE.Group();driver.position.set(0,y,.69);group.add(driver);
    mesh(new THREE.TorusGeometry(radius+.035,.027,12,64),metal,driver);
    const points=[new THREE.Vector2(.06,.03),new THREE.Vector2(radius*.25,.01),new THREE.Vector2(radius*.82,-.16),new THREE.Vector2(radius,-.02)];
    const cone=mesh(new THREE.LatheGeometry(points,64),new THREE.MeshStandardMaterial({color:'#302723',metalness:.45,roughness:.38,side:THREE.DoubleSide}),driver);cone.rotation.x=Math.PI/2;cones.push(cone);
    const center=cylinder(driver,radius*.21,.047,metal,[0,0,.03],32);center.rotation.x=Math.PI/2;
  }
  plate(group,'ah.','AFTERHOURS AUDIO',[.8,.2],[0,-1.35,.66],'#d79d70','#151313');
  for(const y of [-1.45,1.45])for(const x of [-.58,.58]){const screw=cylinder(group,.025,.02,metal,[x,y,.665],12);screw.rotation.x=Math.PI/2;}
  const port=cylinder(group,.13,.04,dark,[0,-1.06,.68],32);port.rotation.x=Math.PI/2;mesh(new THREE.TorusGeometry(.14,.012,8,32),metal,group,[0,-1.06,.7]);
  return {group,cones};
}
export function createSpeakers(){const group=new THREE.Group();const left=speaker(group,-1.13,.18),right=speaker(group,1.13,-.18);group.rotation.set(.04,-.12,0);return {group,cones:[...left.cones,...right.cones],kind:'speakers'};}

export function animateModel(model,time,playing,pulse,motion=true,delta=1/60){
  if(model.kind==='turntable'){if(motion)model.record.rotation.y+=delta*model.speed.value;model.led.material.emissiveIntensity=playing?1.2+pulse*1.8:.4;}
  if(model.kind==='cassette'){
    if(motion&&playing)model.reels.forEach((reel,i)=>reel.rotation.z-=delta*(i?1.32:1.68));
    const e=model.explosion.value;model.face.position.z=.14+e*.9;model.back.position.z=-.05-e*.65;model.reels.forEach((reel,i)=>{reel.position.z=.24+e*.2;reel.position.x=(i?1:-1)*(.96+e*.28);});
  }
  if(model.kind==='speakers')model.cones.forEach((cone,i)=>cone.position.z=motion&&playing?pulse*(i%2?.045:.12):0);
}
