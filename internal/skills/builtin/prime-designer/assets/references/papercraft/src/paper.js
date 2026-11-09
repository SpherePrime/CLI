import * as THREE from 'three';
export const palette={pink:'#f3a4c5',yellow:'#e8f067',mint:'#a3d8bf',white:'#f3efe5'};
export const forms=[{name:'Журавль',label:'THE CRANE',subtitle:'БОЛЬШЕ, ЧЕМ ПРОСТО БУМАГА'},{name:'Цветок',label:'THE BLOOM',subtitle:'РАСКРЫВАЕТСЯ В ТВОЁМ РИТМЕ'},{name:'Гармошка',label:'THE PLEAT',subtitle:'ОДИН ЛИСТ. БЕСКОНЕЧНЫЙ РИТМ.'}];
function paperMaterial(color){return new THREE.MeshStandardMaterial({color,roughness:.9,metalness:0,flatShading:true,side:THREE.DoubleSide});}
function surface(vertices,indices,material,parent){const g=new THREE.BufferGeometry();g.setAttribute('position',new THREE.Float32BufferAttribute(vertices,3));g.setIndex(indices);g.computeVertexNormals();const object=new THREE.Mesh(g,material);object.castShadow=true;object.receiveShadow=true;parent.add(object);return object;}
function crease(parent,points){const line=new THREE.Line(new THREE.BufferGeometry().setFromPoints(points.map(p=>new THREE.Vector3(...p))),new THREE.LineBasicMaterial({color:'#ffffff',transparent:true,opacity:.19}));parent.add(line);return line;}
function materials(color){return [paperMaterial(color),paperMaterial(new THREE.Color(color).offsetHSL(0,-.03,-.08)),paperMaterial(new THREE.Color(color).offsetHSL(0,-.03,.08))];}
export function createPaper(index=0,color='pink'){
  const group=new THREE.Group(),mats=materials(palette[color]),hinges=[];
  if(index===0){
    surface([0,.29,0,-.29,0,-.65,.29,0,-.65,.35,0,.55,-.35,0,.55,0,-.23,0],[0,1,2,0,2,3,0,3,4,0,4,1,5,2,1,5,3,2,5,4,3,5,1,4],mats[0],group);
    surface([-.12,.04,-.54,.12,.04,-.54,.07,1.25,-.98,-.07,1.25,-.98,0,.58,-.66],[0,1,4,1,2,4,2,3,4,3,0,4],mats[1],group);
    surface([-.07,1.25,-.98,.07,1.25,-.98,0,1.13,-1.62,0,1.03,-1.03],[0,1,2,0,2,3,1,3,2],mats[2],group);
    surface([-.22,.02,.5,.22,.02,.5,0,1.02,1.83,0,.24,.92],[0,1,3,0,3,2,1,2,3],mats[1],group);
    for(const sign of [-1,1]){const hinge=new THREE.Group();hinge.position.set(sign*.24,.07,0);group.add(hinge);hinges.push(hinge);const v=[0,0,-.42,sign*1.95,.07,-.61,sign*1.5,.03,.86,0,0,.52,sign*.72,.22,.07];surface(v,[0,1,4,1,2,4,2,3,4,3,0,4],mats[sign===1?0:2],hinge);crease(hinge,[[0,.009,-.42],[sign*.72,.23,.07],[sign*1.5,.04,.86]]);}
    group.rotation.set(.09,-.42,.08);
  }else if(index===1){
    const stemMaterial=paperMaterial('#6fb6a0');const stem=new THREE.Mesh(new THREE.CylinderGeometry(.045,.06,1.5,6),stemMaterial);stem.position.y=-.42;stem.castShadow=true;group.add(stem);
    surface([0,-.5,0,.85,-.16,.15,.18,-.72,.3],[0,1,2],stemMaterial,group);surface([0,-.7,0,-.74,-.32,-.2,-.2,-.86,.08],[0,1,2],stemMaterial,group);
    for(let i=0;i<9;i++){const hinge=new THREE.Group();hinge.position.y=.32;hinge.rotation.y=i/9*Math.PI*2;group.add(hinge);hinges.push(hinge);surface([0,0,0,.42,.01,.68,0,.035,1.55,-.42,.01,.68,0,.18,.66],[0,1,4,1,2,4,2,3,4,3,0,4],mats[i%3],hinge);crease(hinge,[[0,.01,0],[0,.19,.66],[0,.045,1.55]]);}
    const core=new THREE.Mesh(new THREE.ConeGeometry(.23,.35,6),paperMaterial('#ebef82'));core.position.y=.49;core.castShadow=true;group.add(core);group.rotation.set(.25,.3,-.12);
  }else{
    const chain=new THREE.Group();group.add(chain);let parent=chain;
    const geometry=new THREE.PlaneGeometry(.33,1.75);geometry.translate(.165,0,0);geometry.rotateX(Math.PI/2);
    for(let i=0;i<16;i++){const hinge=new THREE.Group();if(i>0)hinge.position.x=.33;parent.add(hinge);hinges.push(hinge);const panel=new THREE.Mesh(geometry,mats[i%3]);panel.castShadow=true;panel.receiveShadow=true;hinge.add(panel);crease(hinge,[[0,.002,-.875],[0,.002,.875]]);parent=hinge;}
    group.userData.chain=chain;group.rotation.set(-.22,.1,.04);
  }
  return {index,group,hinges,mats,color};
}
export function posePaper(model,opening,time,motion){
  const wave=motion?Math.sin(time*1.45)*.16:0;
  if(model.index===0)model.hinges.forEach((hinge,i)=>hinge.rotation.z=(i===0?-1:1)*((1-opening)*1.12+wave));
  if(model.index===1)model.hinges.forEach((hinge,i)=>{hinge.rotation.x=-(1-opening)*1.38+(motion?Math.sin(time*1.05-i*.5)*.09:0);});
  if(model.index===2){const angle=(1-opening)*1.06+.15+wave*.75;model.hinges.forEach((hinge,i)=>hinge.rotation.z=i===0?angle:(i%2?-2:2)*angle);model.group.userData.chain.position.x=-16*.33*Math.cos(angle)/2;model.group.userData.chain.position.y=-.33*Math.sin(angle)/2;}
}
export function recolor(model,color){model.color=color;model.mats.forEach((m,i)=>m.color.set(palette[color]).offsetHSL(0,i===0?0:-.03,i===1?-.08:i===2?.08:0));}
export function disposePaper(model){const geo=new Set(),mats=new Set();model.group.traverse(o=>{if(o.geometry)geo.add(o.geometry);if(o.material)mats.add(o.material);});geo.forEach(g=>g.dispose());mats.forEach(m=>m.dispose());}
