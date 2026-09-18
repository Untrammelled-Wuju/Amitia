import http from "node:http";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { fileURLToPath } from "node:url";
import { dirname, join, resolve } from "node:path";
import assert from "node:assert/strict";

const root=resolve(dirname(fileURLToPath(import.meta.url)),"..");
const SERVICE_TOKEN = "amitia-test-service-token";
const AUTH_HEADERS = { authorization: `Bearer ${SERVICE_TOKEN}` };
const temp=fs.mkdtempSync(path.join(os.tmpdir(),"amitia-wechat-version-gate-"));
const fake=path.join(temp,"fake-agent.mjs");
fs.writeFileSync(fake,`#!/usr/bin/env node\nimport readline from "node:readline";\nconst client={found:true,running:true,pid:5,path:"/mock",version:"4.1.13.56",strategy:"native",windowManaged:true};\nconst driver={available:true,attached:true,kind:"bad-version-driver",version:"v1",clientVersion:"4.1.12.55",versionVerified:false,capabilities:{attached:true,qr:true,loginStatus:true,selfProfile:true,receiveText:true,sendText:true},message:"intentional mismatch"};\nconst rl=readline.createInterface({input:process.stdin,crlfDelay:Infinity});\nconst out=v=>process.stdout.write(JSON.stringify(v)+"\\n");\nrl.on("line",line=>{let r;try{r=JSON.parse(line)}catch{return};const b={id:r.id,ok:true,platform:"linux",architecture:"amd64",client,driver};if(["probe","start","driver.probe","hide"].includes(r.op))return out(b);if(r.op==="driver.call"){let data={};if(r.driverOp==="login.status")data={logged:true};else if(r.driverOp==="account.self")data={wxid:"wxid_bad"};else if(r.driverOp==="login.qr")data={imageDataUrl:"data:image/png;base64,UE5H"};else if(r.driverOp==="events.poll")data={events:[{messageId:"should-not-poll",peerId:"x",senderId:"x",type:1,text:"bad"}]};else if(r.driverOp==="messages.send_text")data={ok:true};return out({...b,data})}out({...b,ok:false,error:"unsupported"})});\n`);
fs.chmodSync(fake,0o755);
const sha=createHash("sha256").update(fs.readFileSync(fake)).digest("hex");
const core=http.createServer((req,res)=>{res.writeHead(200,{"content-type":"application/json"});res.end('{"success":true}')});
await new Promise((ok,fail)=>{core.once("error",fail);core.listen(18899,"127.0.0.1",ok)});
const child=spawn(process.execPath,[join(root,"runtime","service.mjs")],{cwd:root,env:{...process.env,AMITIA_SERVICE_AUTH_TOKEN:SERVICE_TOKEN,AMITIA_SERVICE_AUTH_VERSION:"1",AMITIA_NATIVE_COMPANIONS_VERSION:"1",AMITIA_NATIVE_COMPANIONS:JSON.stringify([{id:"wechat-agent-linux-x64",platform:"linux",architecture:"amd64",path:fake,sha256:sha,executable:true}])},stdio:["ignore","ignore","pipe"]});
async function wait(fn,timeout=6000){const st=Date.now();while(Date.now()-st<timeout){try{const v=await fn();if(v)return v}catch{}await new Promise(r=>setTimeout(r,100))}throw new Error("timeout")}
try{
 await wait(async()=> (await fetch("http://127.0.0.1:19878/api/health", { headers: AUTH_HEADERS })).ok);
 const c=await fetch("http://127.0.0.1:19878/api/connect",{method:"POST",headers:{...AUTH_HEADERS,"content-type":"application/json"},body:"{}"}).then(r=>r.json());
 assert.equal(c.success,true);
 const st=await fetch("http://127.0.0.1:19878/api/status", { headers: AUTH_HEADERS }).then(r=>r.json());
 assert.equal(st.data.nativeDriverVersionVerified,false);
 assert.equal(st.data.messageTransportReady,false);
 const resp=await fetch("http://127.0.0.1:19878/api/send",{method:"POST",headers:{...AUTH_HEADERS,"content-type":"application/json"},body:JSON.stringify({toUserId:"x",text:"should fail"})});
 assert.equal(resp.status,500);
 const body=await resp.json();
 assert.match(body.message,/版本验证|发送能力/);
 console.log("wechat-personal native version gate e2e: PASS");
} finally { child.kill("SIGTERM"); await new Promise(r=>setTimeout(r,120)); core.close(); fs.rmSync(temp,{recursive:true,force:true}); }
