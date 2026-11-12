import {writeFile} from 'node:fs/promises';
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
let target;
for (let i=0;i<40;i++) {
  try {target=(await (await fetch('http://127.0.0.1:9222/json')).json()).find(t=>t.type==='page');if(target)break;}catch{}
  await pause(100);
}
if(!target)throw Error('Browser did not start');
const socket=new WebSocket(target.webSocketDebuggerUrl);
await new Promise((resolve,reject)=>{socket.onopen=resolve;socket.onerror=reject});
let seq=0;const waiting=new Map();
socket.onmessage=e=>{const m=JSON.parse(e.data);if(m.id){const p=waiting.get(m.id);waiting.delete(m.id);m.error?p.reject(Error(JSON.stringify(m.error))):p.resolve(m.result)}};
function send(method,params={}){const id=++seq;return new Promise((resolve,reject)=>{waiting.set(id,{resolve,reject});socket.send(JSON.stringify({id,method,params}))})}
await send('Page.enable');
await send('Runtime.enable');
await send('Emulation.setDeviceMetricsOverride',{width:1340,height:1250,deviceScaleFactor:1,mobile:false});
await send('Page.navigate',{url:'file:///out/index.html'});
for(let i=0;i<50;i++) {
  const {result}=await send('Runtime.evaluate',{expression:'document.readyState === "complete" && !!document.getElementById("portrait")?.querySelector(".touch")',returnByValue:true});
  if(result.value)break;
  await pause(100);
}
await send('Runtime.evaluate',{expression:'document.fonts.ready',awaitPromise:true});
const {result,exceptionDetails}=await send('Runtime.evaluate',{returnByValue:true,expression:`(()=>{
 const reports=[];
 for(const id of ['land','portrait']) {
  const el=document.getElementById(id);setMode(el,'explore');
  const buttons=[...el.querySelectorAll('.touch,.member')];
  const small=buttons.filter(b=>{const r=b.getBoundingClientRect();return r.width<48||r.height<48}).map(b=>b.textContent);
  if(small.length)throw Error('Small targets: '+small.join(','));
  el.querySelector('[data-action="前進"]').click();
  if(!el.querySelector('.log').textContent.includes('前進'))throw Error('Movement feedback missing');
  el.querySelector('[data-member="3"]').click();
  if(!el.querySelector('.notice').textContent.includes('Gene Eric'))throw Error('Party selection missing');
  el.querySelector('[data-close]').click();
  if(el.querySelector('.notice').classList.contains('open'))throw Error('Return failed');
  setMode(el,'battle');
  if(!el.querySelector('[data-action="攻擊"]')||[...el.querySelectorAll('.dpad button')].some(b=>!b.disabled))throw Error('Battle controls wrong');
  setMode(el,'input');
  el.querySelector('input').value='测试姓名';el.querySelector('[data-action="送出"]').click();
  if(!el.querySelector('.log').textContent.includes('测试姓名'))throw Error('Unicode input preview failed');
  setMode(el,'explore');
  reports.push({view:id,targets:buttons.length,minimumTarget:48,movementFeedback:true,partySelection:true,return:true,battleButtons:true,unicodeInputPreview:true});
 }
 return {status:'prototype only; no game engine or native mobile validation',reports};
})()`});
if(exceptionDetails)throw Error(JSON.stringify(exceptionDetails));
await writeFile('/out/checks.json',JSON.stringify(result.value,null,2)+'\n');
async function screenshot(path){await pause(200);const r=await send('Page.captureScreenshot',{format:'png',captureBeyondViewport:false});await writeFile(path,Buffer.from(r.data,'base64'));}
await screenshot('/out/comparison.png');
await send('Runtime.evaluate',{expression:'setMode(document.getElementById("land"),"battle");setMode(document.getElementById("portrait"),"input")'});
await screenshot('/out/context-controls.png');
socket.close();
console.log(JSON.stringify(result.value));
