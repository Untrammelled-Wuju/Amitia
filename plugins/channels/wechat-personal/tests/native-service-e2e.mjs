import http from "node:http";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { fileURLToPath } from "node:url";
import { dirname, join, resolve } from "node:path";
import assert from "node:assert/strict";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const SERVICE_TOKEN = "amitia-test-service-token";
const AUTH_HEADERS = { authorization: `Bearer ${SERVICE_TOKEN}` };
const temp = fs.mkdtempSync(path.join(os.tmpdir(), "amitia-wechat-native-e2e-"));
const sendLog = path.join(temp, "send.jsonl");
const fakeAgent = path.join(temp, "fake-agent.mjs");

fs.writeFileSync(fakeAgent, `#!/usr/bin/env node\nimport fs from "node:fs";\nimport readline from "node:readline";\nconst sendLog = process.env.AMITIA_TEST_SEND_LOG || "";\nlet eventDelivered = false;\nconst client = {found:true,running:true,pid:4242,path:"/mock/wechat",version:"4.1.13.56",strategy:"native",windowManaged:true,message:"mock official client"};\nconst driver = {available:true,attached:true,kind:"amitia-test-hook",version:"test-v1",clientVersion:"4.1.13.56",versionVerified:true,endpoint:"mock://pipe",capabilities:{attached:true,qr:true,loginStatus:true,selfProfile:true,receiveText:true,sendText:true},message:"verified test driver"};\nconst rl = readline.createInterface({input:process.stdin, crlfDelay:Infinity});\nfunction out(v){ process.stdout.write(JSON.stringify(v)+"\\n"); }\nrl.on("line", line => {\n  let req; try { req=JSON.parse(line); } catch { return; }\n  const base={id:req.id,ok:true,platform:"linux",architecture:"amd64",client,driver};\n  if(req.op==="probe" || req.op==="start" || req.op==="driver.probe") return out(base);\n  if(req.op==="hide") return out(base);\n  if(req.op==="driver.call") {\n    let data={};\n    if(req.driverOp==="login.status") data={logged:true,status:"online"};\n    else if(req.driverOp==="account.self") data={wxid:"wxid_native_ai",nickname:"Native测试号",alias:"native_test"};\n    else if(req.driverOp==="login.qr") data={imageDataUrl:"data:image/png;base64,UE5HTU9DSw=="};\n    else if(req.driverOp==="events.poll") {\n      data=eventDelivered?{events:[]}:{events:[{messageId:"native-m-1",peerId:"wxid_friend",senderId:"wxid_friend",type:1,text:"native hello",createdAt:1700000000000}]};\n      eventDelivered=true;\n    } else if(req.driverOp==="messages.send_text") {\n      if(sendLog) fs.appendFileSync(sendLog, JSON.stringify(req.payload||{})+"\\n");\n      data={ok:true};\n    } else if(req.driverOp==="driver.capabilities") data={ok:true,...driver};\n    else return out({...base,ok:false,error:"unsupported_driver_op"});\n    return out({...base,data});\n  }\n  out({...base,ok:false,error:"unsupported_op"});\n});\n`);
fs.chmodSync(fakeAgent, 0o755);
const sha = createHash("sha256").update(fs.readFileSync(fakeAgent)).digest("hex");

let lastCoreInbound = null;
function server(port, handler) {
  const s = http.createServer(handler);
  return new Promise((resolvePromise, reject) => {
    s.once("error", reject);
    s.listen(port, "127.0.0.1", () => resolvePromise(s));
  });
}
async function readBody(req) {
  const chunks=[]; for await (const c of req) chunks.push(c);
  const t=Buffer.concat(chunks).toString("utf8"); return t?JSON.parse(t):{};
}
function reply(res, value, status=200) {
  const data=JSON.stringify(value); res.writeHead(status,{"content-type":"application/json","content-length":Buffer.byteLength(data)}); res.end(data);
}
async function waitFor(fn, timeout=8000) {
  const start=Date.now();
  while(Date.now()-start<timeout){ try{const v=await fn(); if(v)return v;}catch{} await new Promise(r=>setTimeout(r,100)); }
  throw new Error("timeout");
}
const core = await server(18899, async (req,res)=>{
  if(req.url==="/api/channels/inbound" && req.method==="POST") { lastCoreInbound=await readBody(req); return reply(res,{success:true,conversationId:"conv-native"}); }
  return reply(res,{error:"not_found"},404);
});

const descriptors=[{id:"wechat-agent-linux-x64",platform:"linux",architecture:"amd64",path:fakeAgent,sha256:sha,executable:true}];
const child=spawn(process.execPath,[join(root,"runtime","service.mjs")],{
  cwd:root,
  env:{...process.env,AMITIA_SERVICE_AUTH_TOKEN:SERVICE_TOKEN,AMITIA_SERVICE_AUTH_VERSION:"1",AMITIA_NATIVE_COMPANIONS_VERSION:"1",AMITIA_NATIVE_COMPANIONS:JSON.stringify(descriptors),AMITIA_TEST_SEND_LOG:sendLog},
  stdio:["ignore","pipe","pipe"],
});
let logs=""; child.stdout.on("data",c=>logs+=c); child.stderr.on("data",c=>logs+=c);

try {
  await waitFor(async()=> (await fetch("http://127.0.0.1:19878/api/health", { headers: AUTH_HEADERS })).ok);
  const connect=await fetch("http://127.0.0.1:19878/api/connect",{method:"POST",headers:{...AUTH_HEADERS,"content-type":"application/json"},body:"{}"}).then(r=>r.json());
  assert.equal(connect.success,true);
  assert.equal(connect.data.status,"connected");
  const status=await fetch("http://127.0.0.1:19878/api/status", { headers: AUTH_HEADERS }).then(r=>r.json());
  assert.equal(status.data.driverKind,"amitia-native");
  assert.equal(status.data.nativeDriverVersionVerified,true);
  assert.equal(status.data.nativeDriverClientVersion,"4.1.13.56");
  assert.equal(status.data.messageTransportReady,true);
  assert.equal(status.data.accountId,"wxid_native_ai");

  await waitFor(()=>lastCoreInbound);
  assert.equal(lastCoreInbound.channel,"wechat_personal");
  assert.equal(lastCoreInbound.accountId,"wxid_native_ai");
  assert.equal(lastCoreInbound.senderId,"wxid_friend");
  assert.equal(lastCoreInbound.text,"native hello");

  const send=await fetch("http://127.0.0.1:19878/api/send",{method:"POST",headers:{...AUTH_HEADERS,"content-type":"application/json","idempotency-key":"native-delivery-1"},body:JSON.stringify({toUserId:"wxid_friend",text:"native reply",deliveryKey:"native-delivery-1"})}).then(r=>r.json());
  assert.equal(send.success,true);
  await waitFor(()=>fs.existsSync(sendLog) && fs.readFileSync(sendLog,"utf8").trim());
  const sent=JSON.parse(fs.readFileSync(sendLog,"utf8").trim().split(/\n/).at(-1));
  assert.deepEqual(sent,{peerId:"wxid_friend",text:"native reply",deliveryKey:"native-delivery-1"});
  console.log("wechat-personal native companion e2e: PASS");
} finally {
  child.kill("SIGTERM");
  await new Promise(r=>setTimeout(r,150));
  core.close();
  fs.rmSync(temp,{recursive:true,force:true});
  if(child.exitCode && child.exitCode!==0) console.error(logs);
}
