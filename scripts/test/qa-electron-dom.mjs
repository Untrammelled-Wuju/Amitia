import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { resolve as resolvePath } from "node:path";
import { setTimeout as delay } from "node:timers/promises";

const [logPath, ...steps] = process.argv.slice(2);
const log = await readFile(logPath, "utf8");
const port = [...log.matchAll(/^DevTools listening on ws:\/\/127\.0\.0\.1:(\d+)\//gm)].at(-1)?.[1];
assert(port, "找不到隔离 Electron 的调试端口");
const tabs = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json();
const page = tabs.find(tab => {
  if (tab.type !== "page" || !tab.url) return false;
  const url = new URL(tab.url);
  return ["http://127.0.0.1:5178", "http://localhost:5178"].includes(url.origin) && ["/", "/index.html"].includes(url.pathname);
});
assert(page, "找不到 Amitia 页面");

const socket = new WebSocket(page.webSocketDebuggerUrl);
let sequence = 0;
const pending = new Map();
let interceptRequest;
let interceptFailure;
socket.addEventListener("message", ({ data }) => {
  const reply = JSON.parse(data);
  if (reply.method === "Fetch.requestPaused" && interceptRequest) {
    interceptRequest(reply.params).catch(error => { interceptFailure = error; });
  }
  const request = pending.get(reply.id);
  if (!request) return;
  pending.delete(reply.id);
  clearTimeout(request.timeout);
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
    }, 5000);
    pending.set(id, { resolve, reject, timeout });
    socket.send(JSON.stringify({ id, method, params }));
  });
}

async function evaluate(expression) {
  const result = await command("Runtime.evaluate", { expression, returnByValue: true, awaitPromise: true });
  return result.result.value;
}

async function until(expression) {
  const deadline = Date.now() + 10000;
  while (Date.now() < deadline) {
    if (interceptFailure) throw interceptFailure;
    const value = await evaluate(expression);
    if (value) return value;
    await delay(150);
  }
  throw new Error(`页面断言超时 ${expression}\n${await evaluate("document.body.innerText.slice(0, 1200)")}`);
}

async function verifyUninstallPolling() {
  const path = "/creative-workshop/pet/installations";
  const resource = "/api/desktop-pets/installations";
  const requests = new Map();
  const failures = new Map();
  const observe = ({ data }) => {
    const { method, params } = JSON.parse(data);
    if (method === "Network.requestWillBeSent" && params.request.method === "GET" && new URL(params.request.url).pathname === resource) {
      requests.set(params.requestId, Date.now());
    }
    if (method === "Network.loadingFailed") failures.set(params.requestId, params);
  };
  const goto = value => evaluate(`location.hash = ${JSON.stringify(value)}`);
  const expectPending = () => until("[...document.querySelectorAll('.installation-card.uninstalled .el-tag')].some(node => node.textContent.trim() === '卸载中')");
  const waitFor = async condition => {
    const deadline = Date.now() + 10000;
    while (Date.now() < deadline) {
      if (interceptFailure) throw interceptFailure;
      if (condition()) return;
      await delay(100);
    }
    throw new Error("等待桌宠安装列表的网络事件超时");
  };

  socket.addEventListener("message", observe);
  await command("Network.enable");
  try {
    await goto("/chat");
    await until("Boolean(document.querySelector('textarea.input-field'))");
    await goto(path);
    await expectPending();
    let inFlight;
    interceptRequest = async ({ requestId, request, networkId }) => {
      if (request.method === "GET" && networkId && !inFlight) inFlight = networkId;
      else await command("Fetch.continueRequest", { requestId });
    };
    await command("Fetch.enable", { patterns: [{ urlPattern: `*${resource}` }] });
    await waitFor(() => inFlight);
    await goto("/chat");
    await until("Boolean(document.querySelector('textarea.input-field'))");
    await waitFor(() => failures.has(inFlight));
    assert.equal(failures.get(inFlight).canceled, true, JSON.stringify(failures.get(inFlight)));
    await command("Fetch.disable");
    const leftCount = requests.size;
    await delay(7000);
    assert.equal(requests.size, leftCount, "离开页面后仍查询安装列表");
    console.log("页面离开时在途 GET 被取消，之后 7 秒无查询");

    requests.clear();
    const entered = Date.now();
    await goto(path);
    await expectPending();
    await delay(Math.max(0, 66000 - (Date.now() - entered)));
    await expectPending();
    const times = [...requests.values()].map(time => time - entered);
    assert(times.some(time => time > 48000 && time < 62000), JSON.stringify(times));
    assert(times.every(time => time < 63000), JSON.stringify(times));
    const deadlineCount = requests.size;
    await delay(7000);
    assert.equal(requests.size, deadlineCount, "达到时间上限后仍查询安装列表");
    console.log(`等待上限验证通过，状态仍为卸载中，最后一次 GET 在 ${times.at(-1)}ms`);
    await goto("/chat");
  } finally {
    socket.removeEventListener("message", observe);
  }
}

try {
  for (const step of steps) {
    const separator = step.indexOf("=");
    const action = separator < 0 ? step : step.slice(0, separator);
    const value = separator < 0 ? "" : step.slice(separator + 1);
    if (action === "click" || action === "clickwithin" || action === "clicktwice") {
      const [container, containingText, text] = action === "clickwithin" ? JSON.parse(value) : ["body", "", value];
      const label = JSON.stringify(text);
      await until(`(() => {
        const parent = [...document.querySelectorAll(${JSON.stringify(container)})]
          .find(node => node.innerText.includes(${JSON.stringify(containingText)}));
        const control = [...(parent?.querySelectorAll('button, a, label, [role=button], [role=option], [role=tab]') ?? [])].find(node =>
          node.getClientRects().length && !node.disabled &&
          (node.innerText.trim() === ${label} || node.title === ${label} || node.getAttribute('aria-label') === ${label}));
        if (!control) return false;
        control.click();
        if (${action === "clicktwice"}) control.click();
        return true;
      })()`);
    } else if (action === "expect") {
      await until(`document.body.innerText.includes(${JSON.stringify(value)})`);
    } else if (action === "absent") {
      await until(`!document.body.innerText.includes(${JSON.stringify(value)})`);
    } else if (action === "expectall") {
      const [selector, expected] = JSON.parse(value);
      await until(`(() => {
        const nodes = [...document.querySelectorAll(${JSON.stringify(selector)})];
        return nodes.length > 0 && nodes.every(node => node.innerText.trim() === ${JSON.stringify(expected)});
      })()`);
    } else if (action === "failsecondget") {
      let queries = 0;
      interceptRequest = async ({ requestId, request }) => {
        if (request.method === "GET" && ++queries === 2) {
          await command("Fetch.failRequest", { requestId, errorReason: "ConnectionReset" });
          await command("Fetch.disable");
          console.log(`已断开第二个 GET ${value}`);
        } else {
          await command("Fetch.continueRequest", { requestId });
        }
      };
      await command("Fetch.enable", { patterns: [{ urlPattern: `*${value}` }] });
    } else if (action === "uninstallpoll") {
      await verifyUninstallPolling();
    } else if (action === "runtime") {
      await until(`window.amitiaDesktop.getRuntimeStatus().then(status => status.state === ${JSON.stringify(value)})`);
    } else if (action === "runtime-error") {
      await until(`window.amitiaDesktop.getRuntimeStatus().then(status => status.state === 'failed' && status.message?.includes(${JSON.stringify(value)}))`);
    } else if (action === "fill") {
      const [selector, text] = JSON.parse(value);
      await until(`(() => {
        const input = [...document.querySelectorAll(${JSON.stringify(selector)})]
          .find(node => node.getClientRects().length && !node.disabled);
        if (!input) return false;
        input.focus();
        input.value = ${JSON.stringify(text)};
        input.dispatchEvent(new Event('input', { bubbles: true }));
        input.dispatchEvent(new Event('change', { bubbles: true }));
        return true;
      })()`);
    } else if (action === "route") {
      await evaluate(`location.hash = ${JSON.stringify(value)}`);
      await delay(700);
    } else if (action === "enter") {
      await command("Input.dispatchKeyEvent", { type: "keyDown", key: "Enter", code: "Enter", windowsVirtualKeyCode: 13 });
      await command("Input.dispatchKeyEvent", { type: "keyUp", key: "Enter", code: "Enter", windowsVirtualKeyCode: 13 });
    } else if (action === "file") {
      const [selector, path] = JSON.parse(value);
      await until(`Boolean(document.querySelector(${JSON.stringify(selector)}))`);
      const { root } = await command("DOM.getDocument");
      const { nodeId } = await command("DOM.querySelector", { nodeId: root.nodeId, selector });
      assert(nodeId, "找不到文件选择控件");
      await command("DOM.setFileInputFiles", { nodeId, files: [resolvePath(path)] });
    } else if (action === "reload") {
      await command("Page.reload");
      await delay(900);
    } else if (action === "inspect") {
      console.log(JSON.stringify(await evaluate(`({
        path: location.hash,
        text: document.body.innerText.slice(0, 1600),
        controls: [...document.querySelectorAll('button, a, input, textarea, [role=button]')]
          .filter(node => node.getClientRects().length)
          .slice(0, 60)
          .map(node => ({label: node.innerText || node.title || node.getAttribute('aria-label') || node.placeholder, disabled: node.disabled}))
      })`)));
    } else {
      throw new Error(`未知操作 ${action}`);
    }
    console.log(action === "fill" ? "完成填表" : `完成 ${step}`);
  }
} finally {
  socket.close();
}
