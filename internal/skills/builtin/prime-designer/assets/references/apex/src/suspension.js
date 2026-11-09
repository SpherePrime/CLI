import * as THREE from 'three';
import {assembly,part,box,cylinder,mesh,tube,metal,dark,rubber,orange} from './models.js';

const blue=new THREE.MeshStandardMaterial({color:'#3478b2',metalness:.45,roughness:.32});
const cast=new THREE.MeshStandardMaterial({color:'#919c9d',metalness:.75,roughness:.45});

function bolt(parent,position,rotation=[0,0,0]){cylinder(parent,.046,.042,position,metal,rotation,6);}
function ring(parent,radius,tubeRadius,position){mesh(new THREE.TorusGeometry(radius,tubeRadius,8,48),metal,parent,position,[Math.PI/2,0,0]);}
function ballJoint(parent,position){mesh(new THREE.SphereGeometry(.09,20,12),metal,parent,position);cylinder(parent,.052,.14,[position[0],position[1]+.075,position[2]],metal);}

export function createSuspension(){
  const model=assembly();model.kind='suspension';
  const anchor=[.35,-.58,0],tilt=.16;
  const strut=(name,offset)=>{const group=part(model,name,anchor,offset);group.rotation.z=tilt;return group;};

  const lower=part(model,'Cast aluminium lower control arm',[0,-.90,0],[0,-.72,-.32]);
  const shape=new THREE.Shape();shape.moveTo(-1.03,-.36);shape.quadraticCurveTo(-1.12,-.42,-1.18,-.27);shape.lineTo(-1.05,.37);shape.quadraticCurveTo(-.93,.44,-.77,.29);shape.lineTo(.50,.07);shape.quadraticCurveTo(.63,0,.50,-.07);shape.lineTo(-.85,-.30);shape.closePath();
  const hole=new THREE.Path();hole.moveTo(-.74,-.16);hole.lineTo(.16,0);hole.lineTo(-.73,.17);hole.closePath();shape.holes.push(hole);
  const geometry=new THREE.ExtrudeGeometry(shape,{depth:.072,bevelEnabled:true,bevelSize:.017,bevelThickness:.014,bevelSegments:3,steps:1});geometry.rotateX(-Math.PI/2);mesh(geometry,cast,lower,[0,-.035,0]);
  for(const [x,z] of [[-1.05,-.30],[-1.02,.30]]){cylinder(lower,.13,.22,[x,0,z],dark,[0,0,Math.PI/2]);cylinder(lower,.077,.235,[x,0,z],rubber,[0,0,Math.PI/2]);cylinder(lower,.035,.26,[x,0,z],metal,[0,0,Math.PI/2]);}
  ballJoint(lower,[.47,.035,0]);

  const knuckle=part(model,'Steering knuckle and hub',[.47,-.51,0],[1.2,-.08,0]);
  box(knuckle,[.18,.67,.23],[-.09,-.015,0],cast,.055);box(knuckle,[.25,.12,.30],[-.13,.29,0],cast,.033);
  cylinder(knuckle,.14,.20,[.01,-.025,0],cast,[0,0,Math.PI/2]);cylinder(knuckle,.205,.085,[.16,-.025,0],metal,[0,0,Math.PI/2]);cylinder(knuckle,.075,.19,[.21,-.025,0],dark,[0,0,Math.PI/2]);
  for(let i=0;i<5;i++){const angle=i/5*Math.PI*2;cylinder(knuckle,.021,.105,[.24,Math.sin(angle)*.142-.025,Math.cos(angle)*.142],metal,[0,0,Math.PI/2],12);}
  ring(knuckle,.058,.015,[-.10,-.30,0]);box(knuckle,[.12,.11,.33],[-.14,.02,.20],cast,.025);bolt(knuckle,[-.13,.15,.10]);bolt(knuckle,[-.13,.15,-.10]);

  const body=strut('Damper body and lower spring seat',[-.50,-.10,.72]);
  cylinder(body,.112,1.04,[0,.52,0],dark);cylinder(body,.122,.12,[0,.12,0],blue);cylinder(body,.145,.075,[0,1.045,0],metal);
  cylinder(body,.273,.045,[0,.86,0],dark);mesh(new THREE.TorusGeometry(.24,.018,8,48),blue,body,[0,.90,0],[Math.PI/2,0,0]);
  box(body,[.085,.34,.26],[0,.17,0],dark,.025);for(const y of [.07,.26]){cylinder(body,.041,.34,[0,y,0],metal,[Math.PI/2,0,0],12);bolt(body,[0,y,.18],[Math.PI/2,0,0]);}
  box(body,[.011,.31,.083],[.115,.56,0],orange,.004);for(let i=0;i<6;i++)ring(body,.112,.006,[0,.29+i*.055,0]);

  const rod=strut('Polished piston rod',[0,1.0,0]);cylinder(rod,.042,.95,[0,1.44,0],metal);cylinder(rod,.065,.035,[0,1.045,0],dark);cylinder(rod,.032,.16,[0,1.96,0],metal);

  const spring=strut('Coil spring',[-.62,.45,-.65]);
  const points=[];for(let i=0;i<=256;i++){const t=i/256,angle=t*Math.PI*2*7.25;points.push([Math.cos(angle)*.213,.91+t*.82,Math.sin(angle)*.213]);}
  const curve=new THREE.CatmullRomCurve3(points.map(p=>new THREE.Vector3(...p)));mesh(new THREE.TubeGeometry(curve,256,.035,10,false),blue,spring);
  mesh(new THREE.TorusGeometry(.213,.035,10,64),blue,spring,[0,.905,0],[Math.PI/2,0,0]);mesh(new THREE.TorusGeometry(.213,.035,10,64),blue,spring,[0,1.733,0],[Math.PI/2,0,0]);

  const top=strut('Top spring seat and bearing',[0,1.33,.15]);cylinder(top,.265,.054,[0,1.79,0],metal);cylinder(top,.23,.07,[0,1.845,0],rubber);cylinder(top,.146,.10,[0,1.91,0],dark);ring(top,.175,.022,[0,1.92,0]);

  const mount=strut('Three-bolt upper mount',[0,1.83,0]);cylinder(mount,.29,.066,[0,1.995,0],cast,[0,0,0],3);cylinder(mount,.10,.08,[0,2.055,0],dark);cylinder(mount,.043,.075,[0,2.095,0],metal,[0,0,0],6);
  for(let i=0;i<3;i++){const a=i/3*Math.PI*2;cylinder(mount,.025,.13,[Math.cos(a)*.22,2.085,Math.sin(a)*.22],metal,[0,0,0],12);bolt(mount,[Math.cos(a)*.22,2.13,Math.sin(a)*.22]);}

  const tie=part(model,'Steering tie rod',[0,0,0],[-.35,-.30,1.10]);tube(tie,[[-1.05,-.63,.55],[-.50,-.60,.43],[.33,-.51,.28]],.026,metal);for(const p of [[-1.05,-.63,.55],[.33,-.51,.28]])ballJoint(tie,p);cylinder(tie,.043,.26,[-.7,-.61,.48],dark,[0,0,Math.PI/2]);

  const link=part(model,'Anti-roll bar drop link',[0,0,0],[-.82,.1,.64]);tube(link,[[.33,.06,.19],[.20,-.31,.28],[-.24,-.73,.53]],.022,metal);for(const p of [[.33,.06,.19],[-.24,-.73,.53]])mesh(new THREE.SphereGeometry(.057,16,10),dark,link,p);
  return model;
}
