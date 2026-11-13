import assert from "node:assert/strict";
import { readFile, writeFile } from "node:fs/promises";
import { setTimeout as delay } from "node:timers/promises";

const [logPath, mode, ...rest] = process.argv.slice(2);
const log = await readFile(logPath, "utf8");
const port = [...log.matchAll(/^DevTools listening on ws:\/\/127\.0\.0\.1:(\d+)\//gm)].at(-1)?.[1];
assert(port, "找不到 Electron 调试端口");
const tabs = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json();
const page = tabs.find(tab => {
  if (tab.type !== "page" || !tab.url) return false;
  try {
    return tab.url.startsWith("http://127.0.0.1:5178/") || tab.url.startsWith("http://localhost:5178/");
  } catch {
    return false;
  }
});
assert(page, `找不到 Amitia 主窗口: ${tabs.map(tab => `${tab.type}:${tab.url}`).join(", ")}`);

const socket = new WebSocket(page.webSocketDebuggerUrl);
const pending = new Map();
let sequence = 0;
socket.addEventListener("message", ({ data }) => {
  const reply = JSON.parse(data);
  const request = pending.get(reply.id);
  if (!request) return;
  pending.delete(reply.id);
  const failure = reply.error ?? reply.result?.exceptionDetails;
  if (failure) request.reject(new Error(JSON.stringify(failure)));
  else request.resolve(reply.result);
});
await new Promise((resolve, reject) => {
  socket.addEventListener("open", resolve, { once: true });
  socket.addEventListener("error", reject, { once: true });
});

function command(method, params = {}) {
  return new Promise((resolve, reject) => {
    const id = ++sequence;
    const timeout = setTimeout(() => {
      pending.delete(id);
      reject(new Error(`调试命令超时 ${method}`));
    }, 30000);
    pending.set(id, { resolve, reject, timeout });
    socket.send(JSON.stringify({ id, method, params }));
  });
}

async function evaluate(expression) {
  const result = await command("Runtime.evaluate", {
    expression,
    returnByValue: true,
    awaitPromise: true,
    userGesture: true,
  });
  if (result.exceptionDetails) throw new Error(JSON.stringify(result.exceptionDetails));
  return result.result.value;
}

try {
  if (mode === "eval") {
    const value = await evaluate(rest.join(" "));
    console.log(typeof value === "string" ? value : JSON.stringify(value, null, 2));
  } else if (mode === "shot") {
    const { data } = await command("Page.captureScreenshot", { format: "png" });
    await writeFile(rest[0], Buffer.from(data, "base64"));
    console.log(rest[0]);
  } else if (mode === "inspect") {
    console.log(JSON.stringify(await evaluate(`({
      path: location.hash,
      text: document.body.innerText.slice(0, 2500),
      controls: [...document.querySelectorAll('button, a, input, textarea, select, [role=button], [role=option], [role=tab]')]
        .filter(node => node.getClientRects().length)
        .slice(0, 80)
        .map(node => ({
          tag: node.tagName,
          label: (node.innerText || node.placeholder || node.title || node.getAttribute('aria-label') || '').trim().slice(0, 80),
          disabled: Boolean(node.disabled),
        }))
    })`), null, 2));
  } else if (mode === "wait") {
    const expression = rest.join(" ");
    const deadline = Date.now() + 30000;
    for (;;) {
      if (await evaluate(expression)) break;
      if (Date.now() > deadline) throw new Error(`等待超时: ${expression}`);
      await delay(200);
    }
    console.log("ok");
  } else if (mode === "net") {
    const [outPath, seconds] = rest;
    const lines = [];
    socket.addEventListener("message", ({ data }) => {
      const reply = JSON.parse(data);
      if (reply.method === "Network.requestWillBeSent") {
        const { request, requestId } = reply.params;
        lines.push(`REQ ${requestId} ${request.method} ${request.url}`);
        if (request.postData) lines.push(`BODY ${requestId} ${request.postData.slice(0, 600)}`);
      }
      if (reply.method === "Network.responseReceived") {
        const { response, requestId } = reply.params;
        lines.push(`RES ${requestId} ${response.status} ${response.url}`);
      }
      if (reply.method === "Network.loadingFailed") {
        lines.push(`FAIL ${reply.params.requestId} ${reply.params.errorText}`);
      }
    });
    await command("Network.enable");
    await delay(Number(seconds || "10") * 1000);
    await writeFile(outPath, lines.join("\n"));
    console.log(`${outPath} ${lines.length}`);
  } else if (mode === "key") {
    const [key, code, keyCode] = rest;
    await command("Input.dispatchKeyEvent", { type: "keyDown", key, code, windowsVirtualKeyCode: Number(keyCode) });
    await command("Input.dispatchKeyEvent", { type: "keyUp", key, code, windowsVirtualKeyCode: Number(keyCode) });
    console.log("ok");
  } else if (mode === "bp") {
    const [urlPart, line, exprJson, seconds] = rest;
    const expressions = JSON.parse(exprJson || "[]");
    let hits = 0;
    const handlers = new Map();
    socket.addEventListener("message", async ({ data }) => {
      const reply = JSON.parse(data);
      if (reply.method === "Debugger.paused") {
        hits += 1;
        const frame = reply.params.callFrames[0];
        const results = {};
        for (const expression of expressions) {
          const id = ++sequence;
          socket.send(JSON.stringify({ id, method: "Debugger.evaluateOnCallFrame", params: { callFrameId: frame.callFrameId, expression, returnByValue: true, silent: true } }));
          const answer = await new Promise(resolve => { handlers.set(id, resolve); setTimeout(() => resolve(null), 5000); });
          results[expression] = answer?.result?.value ?? answer?.result?.description ?? `error:${answer?.exceptionDetails?.text ?? "unknown"}`;
        }
        console.log(`hit#${hits} ${frame.functionName || "(anonymous)"}:${frame.location.lineNumber + 1} ${JSON.stringify(results)}`);
        socket.send(JSON.stringify({ id: ++sequence, method: "Debugger.resume", params: {} }));
      } else if (handlers.has(reply.id)) {
        handlers.get(reply.id)(reply);
      }
    });
    await command("Debugger.enable");
    const breakpoint = await command("Debugger.setBreakpointByUrl", { urlRegex: `.*${urlPart}.*`, lineNumber: Number(line) - 1, columnNumber: 0 });
    await delay(Number(seconds || "10") * 1000);
    await command("Debugger.removeBreakpoint", { breakpointId: breakpoint.breakpointId });
    console.log(`total=${hits}`);
  } else if (mode === "exceptions") {
    const seconds = Number(rest[0] || "10");
    let pausedCount = 0;
    socket.addEventListener("message", ({ data }) => {
      const reply = JSON.parse(data);
      if (reply.method !== "Debugger.paused") return;
      pausedCount += 1;
      const frames = reply.params.callFrames.slice(0, 6).map(frame => `${frame.functionName || "(anonymous)"} ${frame.url.split("/").pop()}:${frame.location.lineNumber + 1}`);
      const description = reply.params.data?.description ?? reply.params.data?.value ?? "";
      console.log(`paused#${pausedCount} reason=${reply.params.reason} ${String(description).slice(0, 400)}`);
      frames.forEach(frame => console.log(`  at ${frame}`));
      socket.send(JSON.stringify({ id: ++sequence, method: "Debugger.resume", params: {} }));
    });
    await command("Debugger.enable");
    await command("Debugger.setPauseOnExceptions", { state: "all" });
    await delay(seconds * 1000);
    await command("Debugger.setPauseOnExceptions", { state: "none" });
    console.log(`total=${pausedCount}`);
  } else if (mode === "script") {
    const [scriptPath, seconds] = rest;
    const lines = [];
    socket.addEventListener("message", ({ data }) => {
      const reply = JSON.parse(data);
      if (reply.method === "Runtime.consoleAPICalled") {
        lines.push(`${reply.params.type} ${reply.params.args.map(arg => arg.value ?? arg.description ?? "").join(" ")}`);
      }
      if (reply.method === "Runtime.exceptionThrown") {
        lines.push(`exception ${reply.params.exceptionDetails.text} ${reply.params.exceptionDetails.exception?.description ?? ""}`);
      }
    });
    await command("Runtime.enable");
    const source = await readFile(scriptPath, "utf8");
    const value = await evaluate(source);
    console.log(typeof value === "string" ? value : JSON.stringify(value, null, 2));
    await delay(Number(seconds || "5") * 1000);
    if (lines.length) console.log(`--- console (${lines.length}) ---\n${lines.join("\n")}`);
  } else if (mode === "console") {
    socket.addEventListener("message", ({ data }) => {
      const reply = JSON.parse(data);
      if (reply.method === "Runtime.consoleAPICalled") {
        console.log(reply.params.type, reply.params.args.map(arg => arg.value ?? arg.description ?? "").join(" "));
      }
      if (reply.method === "Runtime.exceptionThrown") {
        console.log("exception", reply.params.exceptionDetails.text, reply.params.exceptionDetails.exception?.description ?? "");
      }
    });
    await command("Runtime.enable");
    await delay(Number(rest[0] || "5000"));
  } else {
    throw new Error(`未知模式 ${mode}`);
  }
} finally {
  socket.close();
}
