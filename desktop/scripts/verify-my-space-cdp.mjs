const host = process.env.AMITIA_CDP_URL || "http://127.0.0.1:9229";
const targets = await fetch(host + "/json/list").then((response) => response.json());
const target = targets.find((item) => item.type === "page");
if (!target) throw new Error("Electron main page not found");
const socket = new WebSocket(target.webSocketDebuggerUrl);
await new Promise((resolve, reject) => {
  socket.addEventListener("open", resolve, { once: true });
  socket.addEventListener("error", reject, { once: true });
});
let sequence = 0;
const pending = new Map();
socket.addEventListener("message", (event) => {
  const reply = JSON.parse(String(event.data));
  if (!reply.id) return;
  const callback = pending.get(reply.id);
  if (!callback) return;
  pending.delete(reply.id);
  callback(reply);
});
function command(method, params = {}) {
  const id = ++sequence;
  return new Promise((resolve) => {
    pending.set(id, resolve);
    socket.send(JSON.stringify({ id, method, params }));
  });
}
async function inspect() {
  const result = await command("Runtime.evaluate", {
    expression: "JSON.stringify({url:location.href, hash:location.hash, body:document.body?.innerText.slice(0,650), space:!!document.querySelector('.my-space-page'), notFound:!!document.querySelector('.nf-code'), sections:[...document.querySelectorAll('.my-space-page [id]')].map(e=>e.id)})",
    returnByValue: true,
    awaitPromise: true,
  });
  return result.result?.result?.value ?? JSON.stringify(result.result?.exceptionDetails ?? result.error);
}
const diagnostics = [];
socket.addEventListener("message", (event) => {
  const item = JSON.parse(String(event.data));
  if (item.method === "Runtime.exceptionThrown") diagnostics.push(item.params);
  if (item.method === "Runtime.consoleAPICalled" && item.params.type === "error") diagnostics.push(item.params);
});
await command("Runtime.enable");
await command("Page.navigate", { url: "about:blank" });
await new Promise((resolve) => setTimeout(resolve, 800));
await command("Page.navigate", { url: "http://127.0.0.1:15178/#/my-space/profile" });
await new Promise((resolve) => setTimeout(resolve, 8000));
const sections = ["runtime"];
for (const section of sections) {
  const navigation = await command("Runtime.evaluate", { expression: "document.querySelectorAll('.account-navigation-item')[2]?.click()", returnByValue: true });
  await new Promise((resolve) => setTimeout(resolve, 8000));
  await command("Runtime.evaluate", { expression: "document.querySelector('.runtime-advanced').open=true", returnByValue: true });
  await new Promise((resolve) => setTimeout(resolve, 600));
  const state = await command("Runtime.evaluate", {
    expression: `JSON.stringify({url:location.hash,notFound:!!document.querySelector('.nf-code'),nav:[...document.querySelectorAll('.account-navigation-item')].map(x=>({text:x.textContent.trim(),active:x.getAttribute('aria-current')})),body:document.querySelector('.my-space-page')?.innerText.slice(0,500),bodyText:document.body?.innerText.slice(0,450),ready:document.readyState,connectionOptions:[...document.querySelectorAll('.mode-radio-card .mode-label')].map(x=>x.textContent.trim()),coreOptions:[...document.querySelectorAll('.mode-option .mo-label')].map(x=>x.textContent.trim()),scopeNote:document.querySelector('.mode-scope-note')?.textContent.trim(),collapsed:[...document.querySelectorAll('.my-space-page details')].map(x=>({name:x.className,open:x.open})),error:document.body.innerText.includes('页面加载失败')})`,
    returnByValue: true,
  });
  console.log("SECTION_" + section.toUpperCase() + "=" + JSON.stringify({
    navigationError: navigation.result?.errorText || null,
    state: state.result?.result?.value,
    exception: state.result?.exceptionDetails?.text,
  }));
}
const url = "http://127.0.0.1:15178/#/settings/system";
await command("Page.navigate", {url});
await new Promise((resolve) => setTimeout(resolve, 5000));
const settings = await command("Runtime.evaluate", {
  expression: "JSON.stringify({route:location.hash,hasDeploymentLink:!!document.querySelector('a[href*=\"/settings/deployment\"]'),settingsNav:!!document.querySelector('.settings-navigation')})",
  returnByValue: true,
});
console.log("SETTINGS=" + settings.result?.result?.value);
console.log("ERROR_COUNT=" + diagnostics.filter(x => x.type === "error" && JSON.stringify(x.args).includes("[App] render error")).length);
socket.close();
