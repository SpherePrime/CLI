import * as THREE from 'three';
import {mergeGeometries} from 'three/addons/utils/BufferGeometryUtils.js';
import {assembly,part,box,cylinder,mesh} from './models.js';

// Original illustrative geometry: crankshaft along X, vertical axis Y.
const TAU=Math.PI*2, X_AXIS=[0,0,Math.PI/2], Z_AXIS=[Math.PI/2,0,0];
const stations=Array.from({length:6},(_,i)=>-1.025+i*.41);
const material=(color,metalness,roughness)=>new THREE.MeshStandardMaterial({color,metalness,roughness});
const cast=material('#485158',.62,.58),machined=material('#abb6c1',.92,.28);
const steel=material('#666c75',.95,.28),iron=material('#433e3a',.74,.67);
const polymer=material('#13171c',.08,.55),rubber=material('#090c10',.02,.86);
const gasket=material('#303640',.68,.43),brass=material('#b39761',.82,.36);

function castFinish(){
  let seed=19;const pixels=new Uint8Array(128*128*4);
  for(let i=0;i<pixels.length;i+=4){seed=(seed*1664525+1013904223)>>>0;const value=80+(seed>>>24)*.46;pixels[i]=pixels[i+1]=pixels[i+2]=value;pixels[i+3]=255;}
  const texture=new THREE.DataTexture(pixels,128,128,THREE.RGBAFormat);
  texture.wrapS=texture.wrapT=THREE.RepeatWrapping;texture.repeat.set(7,7);texture.magFilter=THREE.LinearFilter;texture.minFilter=THREE.LinearMipmapLinearFilter;texture.generateMipmaps=true;texture.needsUpdate=true;
  cast.bumpMap=texture;cast.bumpScale=.008;
}
castFinish();

function outline(points){
  const shape=new THREE.Shape();shape.moveTo(...points[0]);
  points.slice(1).forEach(p=>shape.lineTo(...p));shape.closePath();return shape;
}

function hole(shape,x,z,r){const path=new THREE.Path();path.absarc(x,-z,r,0,TAU,true);shape.holes.push(path);}

function casting(parent,points,depth,y,mat,holes=[],bevel=.018){
  const shape=outline(points);holes.forEach(([x,z,r])=>hole(shape,x,z,r));
  const geometry=new THREE.ExtrudeGeometry(shape,{depth,steps:1,curveSegments:32,bevelEnabled:bevel>0,bevelThickness:bevel,bevelSize:bevel,bevelSegments:3});
  return mesh(geometry,mat,parent,[0,y,0],[-Math.PI/2,0,0]);
}

function crankcaseShell(parent){
  const shape=new THREE.Shape();shape.moveTo(-.43,.25);shape.lineTo(.43,.25);
  shape.quadraticCurveTo(.51,.1,.48,-.10);shape.quadraticCurveTo(.44,-.26,.24,-.27);
  shape.lineTo(-.24,-.27);shape.quadraticCurveTo(-.44,-.26,-.48,-.10);shape.quadraticCurveTo(-.51,.10,-.43,.25);
  const inside=new THREE.Path();inside.moveTo(-.32,.18);inside.lineTo(-.32,-.13);inside.quadraticCurveTo(0,-.25,.32,-.13);inside.lineTo(.32,.18);inside.closePath();shape.holes.push(inside);
  const geometry=new THREE.ExtrudeGeometry(shape,{depth:2.61,curveSegments:24,bevelEnabled:true,bevelSegments:3,bevelSize:.015,bevelThickness:.015});
  mesh(geometry,cast,parent,[-1.305,0,0],[0,Math.PI/2,0]);
}

function taperedSump(parent,points){
  const vertices=[],uv=[],indices=[],levels=[[-.37,.87,.74],[-.32,.92,.82],[-.08,1,1],[-.04,1,1]];
  for(const [y,sx,sz] of levels)for(const [x,z] of points){vertices.push(x*sx,y,-z*sz);uv.push(x*.3,z*.3);}
  const n=points.length;
  for(let layer=0;layer<levels.length-1;layer++)for(let j=0;j<n;j++){
    const a=layer*n+j,b=layer*n+(j+1)%n,c=a+n,d=b+n;indices.push(a,b,c,b,d,c);
  }
  const cap=THREE.ShapeUtils.triangulateShape(points.map(p=>new THREE.Vector2(...p)),[]);
  for(const [a,b,c] of cap){indices.push(c,b,a,a+(levels.length-1)*n,b+(levels.length-1)*n,c+(levels.length-1)*n);}
  const geometry=new THREE.BufferGeometry();geometry.setAttribute('position',new THREE.Float32BufferAttribute(vertices,3));geometry.setAttribute('uv',new THREE.Float32BufferAttribute(uv,2));geometry.setIndex(indices);geometry.computeVertexNormals();mesh(geometry,cast,parent);
}

function curvedHeatShield(parent){
  const positions=[],uv=[],indices=[],nx=48,nz=16;
  for(let i=0;i<=nx;i++)for(let j=0;j<=nz;j++){
    const x=-1.30+i/nx*2.6,t=j/nz,z=t*.65,width=1-.09*t;
    const corrugation=Math.sin(i/nx*TAU*18)*.004;
    positions.push(x*width,.10-t*.22-t*t*.13+corrugation,z);uv.push(i/nx,j/nz);
    if(i<nx&&j<nz){const a=i*(nz+1)+j,b=a+nz+1;indices.push(a,b,a+1,a+1,b,b+1);}
  }
  const geometry=new THREE.BufferGeometry();geometry.setAttribute('position',new THREE.Float32BufferAttribute(positions,3));geometry.setAttribute('uv',new THREE.Float32BufferAttribute(uv,2));geometry.setIndex(indices);geometry.computeVertexNormals();
  const shieldMaterial=machined.clone();shieldMaterial.side=THREE.DoubleSide;shieldMaterial.roughness=.43;mesh(geometry,shieldMaterial,parent);
  for(const x of [-1.19,0,1.19])bolt(parent,[x,.123,.035]);
}

function ring(parent,outer,inner,depth,position,rotation=[0,0,0],mat=machined){
  const shape=new THREE.Shape();shape.absarc(0,0,outer,0,TAU,false);
  const cut=new THREE.Path();cut.absarc(0,0,inner,0,TAU,true);shape.holes.push(cut);
  const geometry=new THREE.ExtrudeGeometry(shape,{depth,steps:1,curveSegments:36,bevelEnabled:false});
  geometry.translate(0,0,-depth/2);return mesh(geometry,mat,parent,position,rotation);
}

function bolt(parent,position,axis='y',r=.022){
  const rotation=axis==='x'?X_AXIS:axis==='z'?Z_AXIS:[0,0,0];
  cylinder(parent,r,.025,position,machined,rotation,6);
  const recess=new THREE.Vector3(...position);recess[axis]+=.014;
  cylinder(parent,r*.40,.002,recess.toArray(),polymer,rotation,6);
}

function pipe(parent,points,r,mat=machined){
  const curve=new THREE.CatmullRomCurve3(points.map(p=>new THREE.Vector3(...p)));
  return mesh(new THREE.TubeGeometry(curve,48,r,12,false),mat,parent);
}

function between(parent,a,b,r,mat=machined){
  const p=new THREE.Vector3(...a),q=new THREE.Vector3(...b),direction=q.clone().sub(p);
  const object=cylinder(parent,r,direction.length(),p.clone().add(q).multiplyScalar(.5).toArray(),mat);
  object.quaternion.setFromUnitVectors(new THREE.Vector3(0,1,0),direction.normalize());return object;
}

function flangeBolts(parent,x,y,z,r,axis='x',count=6){
  for(let i=0;i<count;i++){const a=i/count*TAU;
    bolt(parent,axis==='x'?[x,y+Math.sin(a)*r,z+Math.cos(a)*r]:[x+Math.cos(a)*r,y+Math.sin(a)*r,z],axis,.017);
  }
}

// Growing spiral sections form the turbo's snail-shaped housing.
function volute(parent,r,thickness,z,mat){
  const positions=[],indices=[],steps=80,sides=16;
  for(let i=0;i<=steps;i++){
    const t=i/steps,angle=-.4+t*TAU*.96,section=thickness*(.55+t*.65),radius=r+(t-.5)*.045;
    for(let j=0;j<=sides;j++){
      const b=j/sides*TAU,radial=radius+Math.cos(b)*section;
      positions.push(Math.cos(angle)*radial,Math.sin(angle)*radial,z+Math.sin(b)*section*.85);
      if(i<steps&&j<sides){const a=i*(sides+1)+j,c=a+sides+1;indices.push(a,c,a+1,a+1,c,c+1);}
    }
  }
  const geometry=new THREE.BufferGeometry();geometry.setAttribute('position',new THREE.Float32BufferAttribute(positions,3));geometry.setIndex(indices);geometry.computeVertexNormals();
  // All geometries have UVs so they can be merged with the other cast surfaces.
  geometry.setAttribute('uv',new THREE.Float32BufferAttribute(new Float32Array(positions.length/3*2),2));
  mesh(geometry,mat,parent);
  const end=-.4+TAU*.96;
  pipe(parent,[[Math.cos(end)*r,Math.sin(end)*r,z],[r+.10,-.04,z],[r+.26,-.03,z]],thickness*.95,mat);
}

function coverTexture(){
  const canvas=document.createElement('canvas');canvas.width=1024;canvas.height=512;
  const ctx=canvas.getContext('2d');ctx.fillStyle='#171c24';ctx.fillRect(0,0,1024,512);
  ctx.fillStyle='#e5eaf1';ctx.font='700 71px Arial';ctx.fillText('M POWER',200,264);
  ['#469bd2','#263b9b','#d03849'].forEach((color,i)=>{ctx.fillStyle=color;ctx.beginPath();ctx.moveTo(94+i*31,207);ctx.lineTo(121+i*31,207);ctx.lineTo(102+i*31,274);ctx.lineTo(75+i*31,274);ctx.fill();});
  ctx.font='24px Arial';ctx.fillStyle='#9ba7b9';ctx.fillText('TWINPOWER TURBO',206,310);
  const texture=new THREE.CanvasTexture(canvas);texture.colorSpace=THREE.SRGBColorSpace;texture.anisotropy=4;return texture;
}

function pulley(parent,x,y,z,r){
  cylinder(parent,r,.09,[x,y,z],polymer,X_AXIS,48);
  cylinder(parent,r*.81,.095,[x-.014,y,z],steel,X_AXIS,48);
  ring(parent,r*.86,r*.75,.008,[x-.065,y,z],[0,Math.PI/2,0],machined);
  for(let i=0;i<6;i++){const a=i/6*TAU;cylinder(parent,r*.13,.003,[x-.064,y+Math.cos(a)*r*.46,z+Math.sin(a)*r*.46],polymer,X_AXIS,16);}
  cylinder(parent,.033,.11,[x-.028,y,z],machined,X_AXIS,6);
}

function turbo(parent){
  volute(parent,.17,.064,-.10,iron);volute(parent,.205,.068,.15,cast);
  cylinder(parent,.087,.23,[0,0,0],steel,Z_AXIS);
  ring(parent,.13,.087,.12,[0,0,.24],[0,0,0],machined);
  cylinder(parent,.087,.008,[0,0,.214],polymer,Z_AXIS);
  for(let i=0;i<11;i++){
    const a=i/11*TAU,blade=box(parent,[.021,.074,.018],[Math.sin(a)*.047,Math.cos(a)*.047,.23],machined,.002);
    blade.rotation.set(.2,.4,-a+.38);
  }
  cylinder(parent,.021,.035,[0,0,.245],steel,Z_AXIS);flangeBolts(parent,0,0,.185,.13,'z',5);
  cylinder(parent,.082,.17,[-.17,-.23,.06],cast,Z_AXIS);
  cylinder(parent,.06,.09,[-.17,-.23,.18],polymer,Z_AXIS);
  between(parent,[-.17,-.23,.20],[.12,-.15,-.06],.016,steel);
  pipe(parent,[[0,-.02,-.14],[.06,-.26,-.14],[.06,-.45,-.04]],.093,iron);
  ring(parent,.108,.078,.025,[.06,-.45,-.04],[Math.PI/2,0,0]);
  pipe(parent,[[.10,.08,.10],[.17,.28,.03],[.0,.35,-.04]],.014,brass);
}

// Merge static geometry per material, preserving independently moving modules.
function batch(model){
  for(const {group} of model.parts){
    group.updateMatrixWorld(true);const inverse=group.matrixWorld.clone().invert(),buckets=new Map();
    group.traverse(object=>{if(!object.isMesh)return;
      const geometry=object.geometry.clone();geometry.applyMatrix4(inverse.clone().multiply(object.matrixWorld));
      const data=geometry.index?geometry.toNonIndexed():geometry;if(data!==geometry)geometry.dispose();
      if(!buckets.has(object.material))buckets.set(object.material,[]);buckets.get(object.material).push(data);
    });
    group.traverse(object=>{if(object.isMesh)object.geometry.dispose();});group.clear();
    for(const [mat,geometries] of buckets){const merged=mergeGeometries(geometries,false);mesh(merged,mat,group);geometries.forEach(g=>g.dispose());}
  }
}

export function createInlineSix(){
  const model=assembly();model.kind='engine';
  const perimeter=[[-1.38,-.32],[-1.29,-.47],[-.91,-.46],[-.76,-.51],[.74,-.51],[.97,-.44],[1.31,-.44],[1.4,-.30],[1.4,.29],[1.24,.44],[.92,.44],[.77,.50],[-.79,.50],[-.94,.44],[-1.28,.44],[-1.38,.28]];
  const bores=stations.map(x=>[x,0,.153]);

  const block=part(model,'Closed-deck six-cylinder crankcase',[0,0,0],[0,0,0]);
  crankcaseShell(block);
  casting(block,perimeter,.25,.23,cast,bores,.025);casting(block,perimeter,.035,.49,machined,bores,0);
  const linerMaterial=machined.clone();linerMaterial.side=THREE.DoubleSide;
  for(const x of stations){
    mesh(new THREE.CylinderGeometry(.151,.151,.70,48,1,true),linerMaterial,block,[x,.14,0]);
    ring(block,.20,.153,.45,[x,.01,0],[Math.PI/2,0,0],cast);
    for(const z of [-.315,.315]){
      between(block,[x-.15,-.22,z*1.38],[x+.13,.22,z*1.48],.034,cast);
      cylinder(block,.062,.04,[x-.17,.51,z],cast);bolt(block,[x-.17,.542,z]);
      cylinder(block,.061,.035,[x,.12,z*1.57],cast,Z_AXIS);bolt(block,[x,.12,z*1.64],'z');
    }
  }
  for(const z of [-.51,.51])for(const y of [-.2,.23])box(block,[2.42,.043,.034],[0,y,z],cast,.006);
  for(const x of [-1.15,.99])box(block,[.20,.25,.12],[x,-.02,.53],cast,.035);
  pipe(block,[[-1.1,.26,.54],[-.64,.28,.55],[.05,.29,.55],[.74,.28,.55],[1.08,.23,.5]],.015,brass);

  const sump=part(model,'Cast oil sump and drain plug',[0,-.265,0],[0,-1.42,0]);
  casting(sump,perimeter,.057,-.04,machined,[],.009);
  const sumpProfile=[[-1.31,-.29],[-1.05,-.37],[-.46,-.37],[-.27,-.43],[.86,-.43],[1.28,-.26],[1.27,.25],[.84,.39],[-.35,.40],[-.56,.32],[-1.25,.28]];
  taperedSump(sump,sumpProfile);
  for(let i=0;i<9;i++)box(sump,[.029,.024,.66],[-.98+i*.25,-.353,0],cast,.004);
  for(const x of [-1.1,-.55,0,.55,1.1])for(const z of [-.40,.40])bolt(sump,[x,-.007,z]);
  cylinder(sump,.039,.045,[.94,-.28,.38],steel,Z_AXIS,6);

  const crank=part(model,'Forged crankshaft and counterweights',[0,-.21,0],[0,-.79,0]);
  cylinder(crank,.061,2.90,[0,0,0],steel,X_AXIS);
  for(let i=0;i<6;i++){
    const x=stations[i],angle=[0,TAU/3,-TAU/3,-TAU/3,TAU/3,0][i],cy=Math.cos(angle)*.095,cz=Math.sin(angle)*.095;
    cylinder(crank,.058,.18,[x,cy,cz],machined,X_AXIS);
    for(const dx of [-.13,.13]){
      const web=box(crank,[.063,.22,.16],[x+dx,cy*.35,cz*.35],steel,.055);web.rotation.x=angle;
      const weight=box(crank,[.067,.13,.24],[x+dx,-cy,-cz],steel,.035);weight.rotation.x=angle;
    }
  }
  cylinder(crank,.09,.13,[-1.49,0,0],machined,X_AXIS);

  const piston=part(model,'Six pistons with I-beam connecting rods',[0,0,0],[0,.85,0]);
  for(let i=0;i<6;i++){
    const x=stations[i],angle=[0,TAU/3,-TAU/3,-TAU/3,TAU/3,0][i],cy=Math.cos(angle)*.095,cz=Math.sin(angle)*.095,h=.39-(.095-cy);
    cylinder(piston,.146,.16,[x,h,0],machined);
    for(const dy of [.012,.039,.063])ring(piston,.148,.142,.010,[x,h+dy,0],[Math.PI/2,0,0],steel);
    cylinder(piston,.098,.004,[x,h+.083,0],cast);cylinder(piston,.043,.245,[x,h-.046,0],steel,Z_AXIS);
    const top=new THREE.Vector3(x,h-.06,0),bottom=new THREE.Vector3(x,cy-.21,cz),axis=top.clone().sub(bottom);
    const rod=new THREE.Group();rod.position.copy(top.clone().add(bottom).multiplyScalar(.5));rod.quaternion.setFromUnitVectors(new THREE.Vector3(0,1,0),axis.clone().normalize());piston.add(rod);
    box(rod,[.037,axis.length(),.061],[0,0,0],steel,.012);
    for(const z of [-.034,.034])box(rod,[.062,axis.length()*.94,.012],[0,0,z],machined,.006);
    ring(piston,.087,.054,.085,bottom.toArray(),[0,Math.PI/2,0],machined);
    for(const z of [-.078,.078])bolt(piston,[x,bottom.y-.05,bottom.z+z]);
  }

  const head=part(model,'Cylinder head with valve ports',[0,.54,0],[0,1.35,0]);
  casting(head,perimeter,.35,0,cast,stations.map(x=>[x,0,.128]),.012);casting(head,perimeter,.011,-.018,gasket,bores,0);
  for(const x of stations)for(const sign of [-1,1]){
    ring(head,.076,.049,.074,[x,.08,sign*.49],[0,0,0],machined);
    for(const dx of [-.095,.095])bolt(head,[x+dx,.37,sign*.35]);
  }

  const valveTrain=part(model,'Twin overhead camshafts and 24 valves',[0,.79,0],[0,1.65,0]);
  for(const z of [-.22,.22]){
    cylinder(valveTrain,.031,2.57,[0,.07,z],machined,X_AXIS);
    for(const x of stations){
      cylinder(valveTrain,.054,.085,[x,.07,z],steel,X_AXIS);
      for(const dx of [-.095,.095]){
        cylinder(valveTrain,.012,.17,[x+dx,-.044,z],machined);cylinder(valveTrain,.046,.014,[x+dx,-.129,z],steel);
        const points=Array.from({length:65},(_,i)=>{const a=i/64*TAU*5;return[x+dx+Math.cos(a)*.030,-.048+i/64*.078,z+Math.sin(a)*.030];});pipe(valveTrain,points,.006,steel);
      }
    }
    cylinder(valveTrain,.107,.052,[-1.37,.07,z],steel,X_AXIS);flangeBolts(valveTrain,-1.404,.07,z,.074);
  }

  const cover=part(model,'Sculpted polymer cylinder-head cover',[0,.96,0],[0,1.95,0]);
  const coverShape=[[-1.39,-.33],[-1.18,-.44],[-.64,-.44],[-.50,-.37],[.67,-.37],[.88,-.45],[1.19,-.45],[1.37,-.29],[1.33,.28],[1.11,.43],[.65,.43],[.52,.36],[-.72,.36],[-.83,.43],[-1.24,.43],[-1.39,.24]];
  casting(cover,coverShape,.12,-.026,polymer,[],.047);casting(cover,coverShape,.018,-.06,gasket,[],.01);
  for(let i=0;i<10;i++)box(cover,[.045,.061,.47],[-1.03+i*.223,.114,-.04],polymer,.008);
  const labelMaterial=new THREE.MeshStandardMaterial({map:coverTexture(),metalness:.16,roughness:.48});
  mesh(new THREE.PlaneGeometry(1.94,.82),labelMaterial,cover,[.02,.15,0],[-Math.PI/2,0,0]);
  for(const x of [-1.22,1.20])for(const z of [-.31,.31])bolt(cover,[x,.119,z]);
  cylinder(cover,.10,.044,[1.08,.15,.16],polymer);box(cover,[.10,.022,.036],[1.08,.181,.16],gasket,.01);
  pipe(cover,[[-1.18,.06,-.28],[-1.38,.10,-.48],[-1.42,-.08,-.59]],.033,rubber);

  const intake=part(model,'Six-runner intake manifold',[0,.58,-.49],[0,.20,-1.12]);
  for(const x of stations){pipe(intake,[[x,0,.04],[x,.09,-.10],[x,.19,-.24],[x,.16,-.41]],.077,polymer);ring(intake,.093,.065,.02,[x,0,.024],[0,0,0],machined);}
  box(intake,[2.39,.33,.39],[.01,.19,-.48],polymer,.09);
  for(let i=0;i<12;i++)box(intake,[.023,.27,.017],[-1.12+i*.203,.19,-.684],polymer,.004);
  ring(intake,.14,.107,.13,[-1.29,.19,-.48],[0,Math.PI/2,0]);box(intake,[.18,.22,.14],[-1.38,.19,-.65],polymer,.045);

  const cooler=part(model,'Charge-air cooler and coolant fittings',[0,.81,-1.09],[0,.97,-1.57]);
  box(cooler,[1.66,.33,.37],[.12,0,0],cast,.055);
  for(let i=0;i<21;i++)box(cooler,[.025,.29,.023],[-.62+i*.072,0,.198],machined,.003);
  for(const x of [-.83,1.07])box(cooler,[.18,.34,.40],[x,0,0],cast,.045);
  pipe(cooler,[[-.90,-.02,0],[-1.12,-.11,.06],[-1.23,-.04,.18]],.100,rubber);
  for(const x of [.75,.94])pipe(cooler,[[x,.04,-.18],[x,.08,-.33],[x-.12,-.11,-.37]],.026,polymer);
  for(const x of [-.68,.9])bolt(cooler,[x,.18,.09]);

  const exhaust=part(model,'Two three-cylinder exhaust collectors',[0,.55,.5],[0,.24,.66]);
  for(let i=0;i<6;i++){
    const x=stations[i],collector=i<3?-.63:.63;
    ring(exhaust,.070,.049,.028,[x,-.10,0],[0,0,0],iron);
    pipe(exhaust,[[x,-.10,0],[x,-.12,.12],[x,-.28,.23],[collector,-.43,.33]],.051,iron);
    bolt(exhaust,[x-.09,-.10,.027],'z',.018);bolt(exhaust,[x+.09,-.10,.027],'z',.018);
  }
  for(const x of [-.63,.63])ring(exhaust,.108,.078,.04,[x,-.43,.34],[0,0,0],steel);

  for(const x of [-.63,.63]){const compressor=part(model,`Turbocharger ${x<0?1:2}: volute, impeller, wastegate`,[x,.12,1.01],[x*.48,-.05,1.43]);turbo(compressor);}

  const shield=part(model,'Curved pressed exhaust heat shield',[0,.62,.58],[0,.86,1.15]);curvedHeatShield(shield);

  const drive=part(model,'Front timing housing and serpentine drive',[-1.42,0,0],[-.96,0,0]);
  box(drive,[.12,1.15,.85],[0,.27,0],cast,.09);
  for(const z of [-.28,.28])cylinder(drive,.136,.14,[-.02,.69,z],cast,X_AXIS);
  for(const [y,z,r] of [[-.18,0,.215],[.30,-.44,.16],[.53,.30,.105],[.12,.27,.083]])pulley(drive,-.13,y,z,r);
  const beltPath=[[-.17,-.395,0],[-.17,.20,-.57],[-.17,.41,-.55],[-.17,.63,.24],[-.17,.61,.39],[-.17,-.30,.19],[-.17,-.395,0]];
  for(const dx of [-.033,0,.033])pipe(drive,beltPath.map(([x,y,z])=>[x+dx,y,z]),.014,rubber);
  for(const [y,z] of [[-.10,-.32],[.40,-.27],[.83,.0],[.34,.37]])bolt(drive,[-.074,y,z],'x');

  const alternator=part(model,'Ventilated alternator and mounting bracket',[-1.05,.22,-.66],[-.74,-.15,-.64]);
  cylinder(alternator,.172,.35,[0,0,0],cast,X_AXIS,48);
  for(const x of [-.19,.19])ring(alternator,.184,.098,.042,[x,0,0],[0,Math.PI/2,0],machined);
  for(let i=0;i<16;i++){const a=i/16*TAU;between(alternator,[-.14,Math.cos(a)*.176,Math.sin(a)*.176],[.14,Math.cos(a)*.176,Math.sin(a)*.176],.018,cast);}
  cylinder(alternator,.121,.23,[0,0,0],brass,X_AXIS);box(alternator,[.17,.14,.16],[.10,.16,0],polymer,.03);
  box(alternator,[.33,.055,.24],[0,-.20,0],cast,.024);pulley(alternator,-.25,0,0,.103);

  const filter=part(model,'Oil filter housing, cooler and plumbing',[.80,.16,-.60],[.55,.05,-.67]);
  box(filter,[.35,.27,.23],[0,0,0],cast,.04);cylinder(filter,.108,.34,[0,.16,0],polymer);
  cylinder(filter,.086,.035,[0,.35,0],polymer,[0,0,0],14);box(filter,[.33,.21,.16],[0,-.02,-.17],machined,.016);
  for(let i=0;i<6;i++)box(filter,[.35,.016,.18],[0,-.105+i*.039,-.17],cast,.003);
  pipe(filter,[[-.10,0,.0],[-.30,-.17,-.02],[-.52,-.23,.07]],.022,rubber);

  const fuel=part(model,'Fuel rail, injectors and wiring',[0,.78,.33],[0,.67,.51]);
  cylinder(fuel,.028,2.41,[0,0,0],machined,X_AXIS);
  for(const x of stations){cylinder(fuel,.021,.19,[x,-.083,0],steel);box(fuel,[.062,.053,.064],[x,-.03,.051],polymer,.009);}
  pipe(fuel,[[-1.16,0,0],[-1.23,.12,0],[-1.11,.21,-.14]],.018,steel);
  pipe(fuel,[[-1.13,.06,.13],[-.51,.08,.15],[.4,.08,.16],[1.15,.03,.11]],.024,rubber);
  box(fuel,[.13,.13,.11],[1.17,.01,0],polymer,.023);

  const flywheel=part(model,'Flywheel and rear crank seal',[1.49,-.21,0],[.83,-.09,0]);
  cylinder(flywheel,.337,.052,[0,0,0],steel,X_AXIS,64);
  ring(flywheel,.343,.302,.037,[.0,0,0],[0,Math.PI/2,0],machined);cylinder(flywheel,.15,.068,[.035,0,0],machined,X_AXIS,48);
  for(let i=0;i<70;i++){const a=i/70*TAU;const tooth=box(flywheel,[.048,.021,.022],[0,Math.cos(a)*.344,Math.sin(a)*.344],steel,.002);tooth.rotation.x=a;}
  flangeBolts(flywheel,.078,0,0,.113,'x',8);

  // Covers clear the internals first; the sump and crankshaft leave last.
  const order=[cover,shield,fuel,cooler,intake,exhaust,...model.parts.filter(p=>p.group.name.startsWith('Turbocharger')).map(p=>p.group),drive,alternator,filter,head,valveTrain,piston,flywheel,crank,sump,block];
  model.parts.sort((a,b)=>order.indexOf(a.group)-order.indexOf(b.group));
  for(const p of model.parts){
    if(p.group===cover)p.rotation.set(.10,0,-.06);
    if(p.group===shield)p.rotation.set(-.16,0,.05);
    if(p.group===intake)p.rotation.set(.08,0,.03);
    if(p.group===flywheel)p.rotation.set(0,.13,0);
  }
  batch(model);return model;
}
