import * as THREE from 'three';
import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js';
import {MeshoptDecoder} from 'three/addons/libs/meshopt_decoder.module.js';

// Offsets use the prepared asset's common frame: X longitudinal, Y up, Z across.
const modules=[
  ['chassis',[0,-.25,0],[0,0,0]],
  ['shell',[0,.75,0],[0,0,0]],
  ['hood',[1.1,1.55,0],[0,0,-.11]],
  ['trunk',[-.9,1.35,0],[0,0,.09]],
  ['cabin',[0,2.35,0],[0,0,0]],
  ['door-left',[.08,.23,-1.65],[0,-.18,0]],
  ['door-right',[.08,.23,1.65],[0,.18,0]],
  ['front',[1.25,.08,0],[0,0,0]],
  ['rear',[-1.25,.08,0],[0,0,0]],
  ['interior',[0,1.38,0],[0,0,0]],
  ['engine',[1.15,.9,0],[0,0,0]],
  ['wheel-front-left',[.45,.12,-1.30],[0,0,0]],
  ['wheel-front-right',[.45,.12,1.30],[0,0,0]],
  ['wheel-rear-left',[-.45,.12,-1.30],[0,0,0]],
  ['wheel-rear-right',[-.45,.12,1.30],[0,0,0]],
];

function prepareMaterial(source){
  const material=source.clone(),name=material.name;
  material.side=THREE.DoubleSide;
  if(/zx1Mtl/.test(name)){
    const lacquer=new THREE.MeshPhysicalMaterial();
    THREE.MeshStandardMaterial.prototype.copy.call(lacquer,material);
    lacquer.color.set('#2f6696');lacquer.metalness=.6;lacquer.roughness=.25;lacquer.clearcoat=1;lacquer.clearcoatRoughness=.16;return lacquer;
  }
  if(/windows|headlight51/.test(name)){
    material.color.set('#9caeb5');material.transparent=true;material.opacity=/windows/.test(name)?.18:.25;material.roughness=.12;material.metalness=.08;material.depthWrite=false;material.side=THREE.FrontSide;
  }else{material.transparent=false;material.opacity=1;material.depthWrite=true;}
  if(/interior|dash|white41|pillars/.test(name)){material.roughness=.67;material.metalness=.05;}
  if(/m8rim001/.test(name)){material.roughness=.86;material.metalness=0;}
  else if(/m8rim1/.test(name)){material.roughness=.28;material.metalness=.9;}
  if(/Caliper/.test(name)){material.color.set('#dd382b');material.roughness=.35;material.metalness=.25;}
  if(/led111|led11/.test(name)){material.emissive.copy(material.color);material.emissiveIntensity=.35;}
  if(/redlight/.test(name)){material.emissive.set('#d32a19');material.emissiveIntensity=.3;}
  return material;
}

export function createBMW(){
  const model={group:new THREE.Group(),parts:[],kind:'car',ready:null};
  const loader=new GLTFLoader().setMeshoptDecoder(MeshoptDecoder);
  model.ready=loader.loadAsync('/models/bmw-m4.glb').then(gltf=>{
    const materials=new Map();
    gltf.scene.traverse(object=>{if(!object.isMesh)return;
      if(!materials.has(object.material))materials.set(object.material,prepareMaterial(object.material));
      object.material=materials.get(object.material);object.castShadow=!object.material.transparent;object.receiveShadow=!object.material.transparent;
      if(object.material.transparent)object.renderOrder=5;
      object.frustumCulled=false;
    });
    model.group.add(gltf.scene);
    for(const [name,offset,rotation] of modules){
      const group=gltf.scene.getObjectByName(name);if(!group)throw new Error(`BMW assembly module missing: ${name}`);
      model.parts.push({group,origin:group.position.clone(),offset:new THREE.Vector3(...offset),rotation:new THREE.Vector3(...rotation)});
    }
    return model;
  });
  return model;
}
