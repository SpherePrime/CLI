import './style.css';
import gsap from 'gsap';
import {ScrollTrigger} from 'gsap/ScrollTrigger';
import {SculptureScene,renderPreviews} from './scene.js';
import {objects} from './sculptures.js';
gsap.registerPlugin(ScrollTrigger);
const reduced=window.matchMedia('(prefers-reduced-motion: reduce)');
let heroScene,labScene,playgroundScene,selected=0,exploded=false,material='chrome';
const dialog=document.querySelector('#playground');

function notify(text){const notice=document.querySelector('.notice');notice.textContent=text;notice.classList.add('visible');setTimeout(()=>notice.classList.remove('visible'),5000);}
try{heroScene=new SculptureScene(document.querySelector('#hero-canvas'),{hero:true});renderPreviews();}catch(error){console.warn('WebGL unavailable',error);document.querySelector('#hero-canvas').innerHTML='<div class="fallback-object" aria-hidden="true"></div>';notify('3D недоступно в этом браузере. Содержание галереи остаётся доступным.');}

// Secondary live scene starts only when it approaches the viewport.
const labObserver=new IntersectionObserver(([entry])=>{if(entry.isIntersecting){if(heroScene&&!labScene){try{labScene=new SculptureScene(document.querySelector('#lab-art'),{index:2,dark:true});}catch{notify('Не удалось открыть вторую 3D-сцену.');}}labObserver.disconnect();}},{rootMargin:'300px'});labObserver.observe(document.querySelector('#lab-art'));

function updatePlayground(){const object=objects[selected];document.querySelector('#playground-title').textContent=object.name;document.querySelector('#playground-description').textContent=object.description;document.querySelector('#playground-number').textContent=`OBJ—00${selected+1}`;document.querySelector('#playground-index').textContent=`0${selected+1} / 03`;}
function selectObject(index){selected=(index+objects.length)%objects.length;const object=objects[selected];document.querySelectorAll('[data-select]').forEach(button=>{const active=Number(button.dataset.select)===selected;button.classList.toggle('active',active);button.classList.toggle('selected',active);button.setAttribute('aria-pressed',String(active));});document.querySelector('#hero-object-index').textContent=`00${selected+1}`;document.querySelector('#hero-object-name').textContent=object.label;document.querySelector('#hero-object-material').textContent=object.material;document.querySelector('#selection-note').textContent=`ВЫБРАН ${object.label} — ${object.note}`;heroScene?.setObject(selected);playgroundScene?.setObject(selected);exploded=false;updateExplodeButton();updatePlayground();}
document.querySelectorAll('[data-select]').forEach(button=>button.addEventListener('click',()=>selectObject(Number(button.dataset.select))));

function updateExplodeButton(){const button=document.querySelector('#playground-explode');button.setAttribute('aria-pressed',String(exploded));button.innerHTML=`${exploded?'Собрать форму':'Разобрать форму'} <span>${exploded?'⊟':'⊞'}</span>`;}
function openPlayground(){updatePlayground();dialog.showModal();document.body.style.overflow='hidden';if(heroScene&&!playgroundScene){try{playgroundScene=new SculptureScene(document.querySelector('#playground-canvas'),{index:selected,interactive:true});}catch{notify('Не удалось открыть интерактивную 3D-сцену.');}}if(playgroundScene){playgroundScene.active=true;playgroundScene.resize();playgroundScene.setObject(selected);playgroundScene.setMaterial(material);if(!reduced.matches)gsap.fromTo(playgroundScene.camera.position,{z:13},{z:9,duration:1.8,ease:'power3.out'});}}
document.querySelectorAll('[data-open-lab]').forEach(button=>button.addEventListener('click',openPlayground));document.querySelector('.close').addEventListener('click',()=>dialog.close());dialog.addEventListener('close',()=>{document.body.style.overflow='';if(playgroundScene)playgroundScene.active=false;});
dialog.addEventListener('click',event=>{if(event.target===dialog){const r=dialog.getBoundingClientRect();if(event.clientX<r.left||event.clientX>r.right||event.clientY<r.top||event.clientY>r.bottom)dialog.close();}});
document.querySelector('#previous-object').addEventListener('click',()=>selectObject(selected-1));document.querySelector('#next-object').addEventListener('click',()=>selectObject(selected+1));document.querySelector('#playground-explode').addEventListener('click',()=>{if(playgroundScene?.transitioning)return;exploded=!exploded;playgroundScene?.explode(exploded?1:0);updateExplodeButton();});
document.querySelector('#reset-view').addEventListener('click',()=>{playgroundScene?.reset();exploded=false;updateExplodeButton();});
document.querySelectorAll('[data-material]').forEach(button=>button.addEventListener('click',()=>{material=button.dataset.material;document.querySelectorAll('[data-material]').forEach(item=>{const active=item===button;item.classList.toggle('active',active);item.setAttribute('aria-pressed',String(active));});playgroundScene?.setMaterial(material);}));
document.querySelector('#lab-explode').addEventListener('click',event=>{if(!labScene)return;const button=event.currentTarget,expanded=button.getAttribute('aria-pressed')!=='true';button.setAttribute('aria-pressed',String(expanded));button.innerHTML=`<span>${expanded?'⊟':'⊞'}</span> ${expanded?'Собрать форму':'Разобрать форму'}`;labScene.explode(expanded?1:0);});

const menu=document.querySelector('.menu-toggle'),mobileNav=document.querySelector('.mobile-nav');menu.addEventListener('click',()=>{const open=menu.getAttribute('aria-expanded')!=='true';menu.setAttribute('aria-expanded',String(open));menu.setAttribute('aria-label',open?'Закрыть меню':'Открыть меню');mobileNav.hidden=!open;});mobileNav.querySelectorAll('a').forEach(link=>link.addEventListener('click',()=>{mobileNav.hidden=true;menu.setAttribute('aria-expanded','false');menu.setAttribute('aria-label','Открыть меню');}));

gsap.matchMedia().add('(prefers-reduced-motion: no-preference)',()=>{
  gsap.from('.intro',{y:35,opacity:0,duration:1.3,stagger:.11,ease:'power3.out',delay:.12});
  document.querySelectorAll('.reveal').forEach(element=>gsap.from(element,{y:55,opacity:0,duration:1,ease:'power3.out',scrollTrigger:{trigger:element,start:'top 92%',once:true}}));
  gsap.to('.page-progress',{scaleX:1,ease:'none',scrollTrigger:{trigger:document.body,start:'top top',end:'bottom bottom',scrub:true}});
  if(heroScene){gsap.to(heroScene.scrollExplosion,{value:.38,ease:'none',scrollTrigger:{trigger:'.hero',start:'top top',end:'bottom top',scrub:1}});}
  gsap.from('.big-word',{yPercent:23,ease:'none',scrollTrigger:{trigger:'.big-word',start:'top bottom',end:'bottom bottom',scrub:1}});
});
document.querySelectorAll('.object-card').forEach(card=>{card.addEventListener('pointermove',event=>{if(reduced.matches||event.pointerType!=='mouse')return;const r=card.getBoundingClientRect();gsap.to(card,{rotationY:(event.clientX-r.left-r.width/2)/r.width*8,rotationX:-(event.clientY-r.top-r.height/2)/r.height*6,y:-8,transformPerspective:1000,duration:.45,ease:'power2.out'});});card.addEventListener('pointerleave',()=>gsap.to(card,{rotationX:0,rotationY:0,y:0,duration:reduced.matches?0:.6,ease:'power3.out'}));});
