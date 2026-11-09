import * as THREE from 'three';

export const objects = [
  {name:'Möbius',label:'MÖBIUS',material:'CHROME · CONTINUOUS SURFACE',description:'Лента с одной непрерывной поверхностью. Поверни её, разбери на фрагменты и найди ракурс, в котором логика перестаёт работать.',note:'ОДНА ПОВЕРХНОСТЬ. БЕСКОНЕЧНОЕ ДВИЖЕНИЕ.'},
  {name:'Fracture',label:'FRACTURE',material:'CRYSTAL · ORDER IN CHAOS',description:'Острые фрагменты собираются в равновесие. Между ними — воздух и напряжение. Разбери конструкцию, чтобы увидеть её внутренний порядок.',note:'ОСТРЫЕ ГРАНИ. НЕОЖИДАННОЕ РАВНОВЕСИЕ.'},
  {name:'Stack',label:'STACK',material:'ALLOY · KINETIC ARCHITECTURE',description:'Восемнадцать пластин и ни одного неподвижного ракурса. Архитектура, которая дышит: меняет ритм, раскрывается и снова собирается.',note:'ВОСЕМНАДЦАТЬ СЛОЁВ. ОДИН ЖИВОЙ РИТМ.'},
];

function metal(color='#c7bbdf') {
  return new THREE.MeshPhysicalMaterial({color,metalness:1,roughness:.17,clearcoat:1,clearcoatRoughness:.12,iridescence:.65,iridescenceIOR:1.4,iridescenceThicknessRange:[120,370],side:THREE.DoubleSide});
}

function ribbonPoint(u,v,offset=0) {
  const twist=u*1.5;
  const radius=1.68+v*Math.cos(twist);
  const point=new THREE.Vector3(radius*Math.cos(u),radius*Math.sin(u),v*Math.sin(twist));
  if(offset){
    const tangent=new THREE.Vector3(-radius*Math.sin(u)-v*1.5*Math.sin(twist)*Math.cos(u),radius*Math.cos(u)-v*1.5*Math.sin(twist)*Math.sin(u),v*1.5*Math.cos(twist));
    const across=new THREE.Vector3(Math.cos(twist)*Math.cos(u),Math.cos(twist)*Math.sin(u),Math.sin(twist));
    point.addScaledVector(new THREE.Vector3().crossVectors(tangent,across).normalize(),offset);
  }
  return point;
}

function ribbonSegment(start,end){
  const positions=[],indices=[];const steps=18,widthSteps=14;
  for(let side=0;side<2;side++)for(let i=0;i<=steps;i++)for(let j=0;j<=widthSteps;j++){
    const p=ribbonPoint(start+(end-start)*i/steps,(j/widthSteps-.5)*1.32,side===0?.045:-.045);positions.push(p.x,p.y,p.z);
  }
  const perSide=(steps+1)*(widthSteps+1);
  for(let side=0;side<2;side++)for(let i=0;i<steps;i++)for(let j=0;j<widthSteps;j++){
    const a=side*perSide+i*(widthSteps+1)+j,b=a+widthSteps+1;
    if(side===0)indices.push(a,b,a+1,b,b+1,a+1);else indices.push(a,a+1,b,b,a+1,b+1);
  }
  for(let i=0;i<steps;i++)for(const j of [0,widthSteps]){const a=i*(widthSteps+1)+j,b=a+widthSteps+1;indices.push(a,a+perSide,b,b,a+perSide,b+perSide);}
  for(const i of [0,steps])for(let j=0;j<widthSteps;j++){const a=i*(widthSteps+1)+j;indices.push(a,a+1,a+perSide,a+1,a+1+perSide,a+perSide);}
  const geometry=new THREE.BufferGeometry();geometry.setAttribute('position',new THREE.Float32BufferAttribute(positions,3));geometry.setIndex(indices);geometry.computeVertexNormals();return geometry;
}

function crystalGeometry(){
  const geometry=new THREE.BufferGeometry();
  const vertices=[0,1.8,0, -.48,0,-.42, .5,0,-.3, .6,0,.34, -.2,0,.6, 0,-.65,0];
  const indices=[0,2,1,0,3,2,0,4,3,0,1,4,5,1,2,5,2,3,5,3,4,5,4,1];
  geometry.setAttribute('position',new THREE.Float32BufferAttribute(vertices,3));geometry.setIndex(indices);const faceted=geometry.toNonIndexed();faceted.computeVertexNormals();geometry.dispose();return faceted;
}

function slabGeometry(){
  const shape=new THREE.Shape();const x=-1.35,y=-.55,w=2.7,h=1.1,r=.12;
  shape.moveTo(x+r,y);shape.lineTo(x+w-r,y);shape.quadraticCurveTo(x+w,y,x+w,y+r);shape.lineTo(x+w,y+h-r);shape.quadraticCurveTo(x+w,y+h,x+w-r,y+h);shape.lineTo(x+r,y+h);shape.quadraticCurveTo(x,y+h,x,y+h-r);shape.lineTo(x,y+r);shape.quadraticCurveTo(x,y,x+r,y);
  const geometry=new THREE.ExtrudeGeometry(shape,{depth:.085,bevelEnabled:true,bevelSegments:3,steps:1,bevelSize:.04,bevelThickness:.04});geometry.rotateX(-Math.PI/2);geometry.center();return geometry;
}

export function createSculpture(index=0){
  const group=new THREE.Group(),parts=[],materials=[];
  const primary=metal();materials.push(primary);
  if(index===0){
    for(let i=0;i<16;i++){const mesh=new THREE.Mesh(ribbonSegment(i/16*Math.PI*2,(i+1)/16*Math.PI*2),primary);group.add(mesh);parts.push(mesh);}
    group.rotation.set(.48,-.38,-.35);
  }else if(index===1){
    const geometry=crystalGeometry();
    for(let i=0;i<11;i++){
      const material=i%3===0?metal('#b8db6b'):i%3===1?metal('#b2a0de'):primary;if(material!==primary)materials.push(material);
      material.roughness=.12;
      const mesh=new THREE.Mesh(geometry,material);const a=i*2.4;
      mesh.position.set(Math.cos(a)*(.25+i*.07),(i-5)*.2,Math.sin(a)*(.2+i*.065));mesh.rotation.set(Math.sin(a)*.65,a,Math.cos(a)*.4);mesh.scale.set(.68+i%3*.12,.65+i%4*.12,.68+i%3*.12);group.add(mesh);parts.push(mesh);
    }
    group.rotation.set(.2,.3,-.12);
  }else{
    const geometry=slabGeometry();const dark=metal('#a598be'),lime=metal('#b8db69');materials.push(dark,lime);
    for(let i=0;i<18;i++){const mesh=new THREE.Mesh(geometry,i%5===0?lime:i%2?dark:primary);mesh.position.y=(i-8.5)*.205;mesh.rotation.y=i*.19;group.add(mesh);parts.push(mesh);}
    group.rotation.set(.12,.5,-.1);
  }
  parts.forEach((part,i)=>{
    part.castShadow=true;part.receiveShadow=true;
    part.userData.basePosition=part.position.clone();part.userData.baseRotation=part.rotation.clone();part.userData.phase=i*1.87;
    if(index===0){const a=(i+.5)/16*Math.PI*2;part.userData.explode=new THREE.Vector3(Math.cos(a)*1.4,Math.sin(a)*1.4,Math.sin(a*2)*.6);}
    else if(index===1){const a=i*2.4;part.userData.explode=new THREE.Vector3(Math.cos(a)*1.8,(i-5)*.2,Math.sin(a)*1.8);}
    else{const a=i*.8;part.userData.explode=new THREE.Vector3(Math.cos(a)*.5,(i-8.5)*.14,Math.sin(a)*.5);}
  });
  return {index,group,parts,materials,explosion:{value:0},style:'chrome'};
}

export function animateSculpture(model,time,motion=true){
  const amount=model.explosion.value;
  model.parts.forEach((part,i)=>{
    const {basePosition,baseRotation,explode}=part.userData;
    part.position.copy(basePosition).addScaledVector(explode,amount);
    part.rotation.copy(baseRotation);
    if(model.index===2&&motion){part.rotation.y+=Math.sin(time*.65+i*.22)*.28;part.position.y+=Math.sin(time*.65+i*.22)*.015;}
    if(model.index===1&&motion){part.position.y+=Math.sin(time*.6+i)*.06;part.rotation.y+=Math.sin(time*.3+i)*.035;}
    part.rotation.z+=amount*Math.sin(i*1.4)*.32;
  });
}

export function applyMaterial(model,style){
  model.style=style;
  model.materials.forEach(material=>{material.color.set(style==='lime'?'#caff36':style==='violet'?'#7653d6':'#c7bbdf');material.metalness=style==='chrome'?1:.72;material.roughness=style==='chrome'?.17:.24;material.iridescence=style==='chrome'?.65:.3;});
}

export function disposeSculpture(model){
  const geometries=new Set();model.parts.forEach(part=>geometries.add(part.geometry));geometries.forEach(geometry=>geometry.dispose());model.materials.forEach(material=>material.dispose());
}
