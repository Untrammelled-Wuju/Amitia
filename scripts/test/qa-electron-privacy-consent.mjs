import assert from "node:assert/strict";
import { existsSync, readFileSync, realpathSync } from "node:fs";
import { setTimeout as delay } from "node:timers/promises";

const [logPath, dataDir] = process.argv.slice(2);
const root = realpathSync(dataDir);
assert(existsSync(root + "/.qa-pet-isolated"), root);
const log = readFileSync(logPath, "utf8");
assert(log.includes("数据目录: " + root), "需要隔离 Electron 实例");
const port = [...log.matchAll(/DevTools listening on ws:\/\/127\.0\.0\.1:(\d+)/g)].at(-1)?.[1];
assert(port, "找不到隔离 Electron 调试端口");
const tabs = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json();
const page = tabs.find(tab => tab.type === "page" && /http:\/\/(localhost|127\.0\.0\.1):5178\/#\//.test(tab.url));
assert(page, "找不到主窗口");

const socket = new WebSocket(page.webSocketDebuggerUrl);
const pending = new Map();
let sequence = 0;
socket.addEventListener("message", ({ data }) => {
  const reply = JSON.parse(data);
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

function nextPageLoad() {
  return new Promise((resolve, reject) => {
    const onMessage = ({ data }) => {
      if (JSON.parse(data).method !== "Page.loadEventFired") return;
      clearTimeout(timeout);
      socket.removeEventListener("message", onMessage);
      resolve();
    };
    const timeout = setTimeout(() => {
      socket.removeEventListener("message", onMessage);
      reject(new Error("刷新后等待页面加载超时"));
    }, 10000);
    socket.addEventListener("message", onMessage);
  });
}

async function evaluate(expression) {
  const reply = await command("Runtime.evaluate", { expression, returnByValue: true, awaitPromise: true });
  return reply.result.value;
}

function snapshot() {
  return evaluate(`({
    hash: location.hash,
    visibility: document.visibilityState,
    timeOrigin: performance.timeOrigin,
    saved: localStorage.getItem('ai-companion-privacy-consent'),
    consent: Boolean(document.querySelector('.consent-footer')?.getClientRects().length),
    consentDisabled: document.querySelector('.consent-footer button')?.disabled,
    publicPage: Boolean(document.querySelector('.public-root')),
    chat: Boolean(document.querySelector('textarea.input-field')),
  })`);
}

async function waitFor(label, condition) {
  const deadline = Date.now() + 10000;
  let state;
  do {
    state = await snapshot();
    if (condition(state)) {
      console.log(label, state);
      return state;
    }
    await delay(150);
  } while (Date.now() < deadline);
  throw new Error(`${label}未达到预期 ${JSON.stringify(state)}`);
}

try {
  await evaluate("localStorage.removeItem('ai-companion-privacy-consent'); location.hash = '/privacy'");
  await waitFor("进入公开隐私页", state => state.hash === "#/privacy" && state.publicPage && !state.consent);
  await evaluate("location.hash = '/chat'");
  await waitFor("首次进入聊天", state => state.chat && state.consent && state.consentDisabled && state.saved === null);
  await evaluate("document.querySelector('.consent-links a').click()");
  await waitFor("从弹层查看隐私说明", state => state.hash === "#/privacy" && state.publicPage && !state.consent && state.saved === null);
  await evaluate("location.hash = '/chat'");
  await waitFor("返回聊天仍需同意", state => state.chat && state.consent && state.saved === null);
  await evaluate("document.querySelector('.consent-footer .el-checkbox input').click()");
  await waitFor("勾选后可同意", state => state.consent && !state.consentDisabled && state.saved === null);
  await evaluate("document.querySelector('.consent-footer button').click()");
  const accepted = await waitFor("同意后弹层卸载", state => state.chat && !state.consent && state.saved === "true");
  await command("Page.enable");
  await Promise.all([nextPageLoad(), command("Page.reload")]);
  await waitFor("刷新后保留同意状态", state => state.timeOrigin !== accepted.timeOrigin && state.chat && !state.consent && state.saved === "true");
} finally {
  socket.close();
}
