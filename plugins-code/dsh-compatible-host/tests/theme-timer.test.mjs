import assert from "node:assert/strict";
import test from "node:test";
import { JSDOM } from "jsdom";
import { createDshClientContext } from "../src/client-abi/module-loader.mjs";
import {
  DEFAULT_FONT_SIZES,
  DSH_BASE_TOKEN_SOURCES,
  createDshThemeRuntime,
} from "../src/client-abi/theme-runtime.mjs";
import { createDshTimerService } from "../src/client-abi/timer-runtime.mjs";

function delay(milliseconds) {
  return new Promise((resolve) => setTimeout(resolve, milliseconds));
}

function themeFixture(options = {}) {
  const dom = new JSDOM("<!doctype html><html><body></body></html>");
  const events = [];
  const storage = options.storage ?? { getItem: () => null };
  const theme = createDshThemeRuntime({
    document: dom.window.document,
    storage,
    matchMedia: options.matchMedia,
    hostActions: options.hostActions,
    emit: (type, payload) => {
      events.push({ type, payload });
    },
  });
  return { dom, theme, events };
}

test("theme 快照按宿主调色板解析默认状态", () => {
  const { dom, theme } = themeFixture();
  const snapshot = theme.getTheme();
  assert.equal(snapshot.preference, "system");
  assert.equal(snapshot.active.id, "light");
  assert.equal(snapshot.active.colorScheme, "light");
  assert.deepEqual(snapshot.fontSizes, DEFAULT_FONT_SIZES);
  assert.deepEqual(snapshot.themes.map((entry) => entry.id), ["light", "dark"]);
  assert.equal(snapshot.revision, 0);
  assert.equal(dom.window.document.documentElement.getAttribute("data-dsh-theme"), "light");
  theme.dispose();
});

test("theme 把 Amitia 调色板映射到 DSH 别名 token", () => {
  const { dom, theme } = themeFixture();
  const style = dom.window.document.documentElement.style;
  for (const [token, value] of Object.entries(DSH_BASE_TOKEN_SOURCES)) {
    assert.equal(style.getPropertyValue(token), value, token);
  }
  assert.equal(style.getPropertyValue("--dsh-content-font-size"), "14px");
  const inspected = theme.exportInspectTokens();
  assert.ok(inspected.some((token) => token.name === "--dsw-alias-bg-base" && token.cssVariable === "--dsw-alias-bg-base"));
  theme.dispose();
});

test("theme 读取宿主暗色状态与持久化偏好", async () => {
  const dom = new JSDOM("<!doctype html><html class=\"dark\" data-theme=\"dark\"><body></body></html>");
  const events = [];
  const theme = createDshThemeRuntime({
    document: dom.window.document,
    storage: { getItem: () => "dark" },
    matchMedia: () => ({ addEventListener() {}, removeEventListener() {}, matches: false }),
    emit: (type, payload) => events.push({ type, payload }),
  });
  assert.equal(theme.getTheme().preference, "dark");
  assert.equal(theme.getTheme().active.id, "dark");
  assert.equal(theme.getTheme().active.colorScheme, "dark");
  assert.equal(dom.window.document.documentElement.getAttribute("data-dsh-color-scheme"), "dark");
  dom.window.document.documentElement.classList.remove("dark");
  dom.window.document.documentElement.setAttribute("data-theme", "light");
  await delay(0);
  assert.equal(theme.getTheme().active.id, "light");
  assert.equal(theme.getTheme().preference, "dark");
  assert.equal(events.at(-1).type, "theme/change");
  theme.dispose();
});

test("theme 切换宿主内置主题缺少宿主能力时原样失败", () => {
  const { theme } = themeFixture();
  assert.throws(() => theme.setTheme("dark"), (error) => {
    assert.equal(error.name, "DshOperationUnavailableError");
    assert.equal(error.code, "DSH_OPERATION_UNAVAILABLE");
    assert.equal(error.operation, "theme.setTheme");
    assert.equal(error.phase, "P13");
    assert.match(error.reason, /主题切换/);
    return true;
  });
  theme.setTheme("light");
  assert.equal(theme.getTheme().active.id, "light");
  theme.dispose();
});

test("theme 通过宿主动作切换内置主题", () => {
  const applied = [];
  const { theme } = themeFixture({ hostActions: { setTheme: (id) => applied.push(id) } });
  theme.setTheme("dark");
  assert.deepEqual(applied, ["dark"]);
  theme.dispose();
});

test("theme 注册第三方主题并应用其 token", () => {
  const { dom, theme, events } = themeFixture();
  const disposeTheme = theme.register({
    id: "midnight",
    colorScheme: "dark",
    tokens: { "--dsw-alias-bg-base": "#000000" },
  });
  assert.deepEqual(theme.getTheme().themes.map((entry) => entry.id), ["light", "dark", "midnight"]);
  theme.setTheme("midnight");
  assert.equal(theme.getTheme().preference, "midnight");
  assert.equal(theme.getTheme().active.id, "midnight");
  assert.equal(dom.window.document.documentElement.style.getPropertyValue("--dsw-alias-bg-base"), "#000000");
  assert.equal(dom.window.document.documentElement.getAttribute("data-dsh-theme"), "midnight");
  assert.equal(events.at(-1).payload.active.id, "midnight");
  disposeTheme();
  assert.equal(theme.getTheme().preference, "system");
  assert.equal(theme.getTheme().themes.length, 2);
  assert.equal(dom.window.document.documentElement.style.getPropertyValue("--dsw-alias-bg-base"), DSH_BASE_TOKEN_SOURCES["--dsw-alias-bg-base"]);
  theme.dispose();
});

test("theme 拒绝重复注册与保留标识", () => {
  const { theme } = themeFixture();
  assert.throws(() => theme.register({ id: "system", colorScheme: "light" }), /preference/);
  assert.throws(() => theme.register({ id: "light", colorScheme: "light" }), /already registered/);
  assert.throws(() => theme.register({ id: "warm", colorScheme: "sepia" }), /colorScheme/);
  assert.throws(() => theme.register({ id: "warm", colorScheme: "light", tokens: { "--dsw-alias-bg-base": 1 } }), /CSS value string/);
  theme.dispose();
});

test("theme 覆盖层按注册顺序叠加并可逐层移除", () => {
  const { dom, theme } = themeFixture();
  const removeFirst = theme.overrideTokens("plugin-a", {
    "--dsw-alias-bg-base": { light: "#111111", dark: "#222222" },
  });
  const removeSecond = theme.overrideTokens("plugin-b", {
    "--dsw-alias-bg-base": { light: "#333333", dark: "#444444" },
  });
  assert.equal(theme.getTheme().active.tokens["--dsw-alias-bg-base"], "#333333");
  assert.equal(dom.window.document.documentElement.style.getPropertyValue("--dsw-alias-bg-base"), "#333333");
  removeSecond();
  assert.equal(theme.getTheme().active.tokens["--dsw-alias-bg-base"], "#111111");
  removeFirst();
  assert.equal(theme.getTheme().active.tokens["--dsw-alias-bg-base"], undefined);
  assert.throws(() => theme.overrideTokens("plugin-c", { "--dsw-alias-bg-base": "#000000" }), /bare string/);
  assert.throws(() => theme.overrideTokens("plugin-c", { "--dsw-alias-bg-base": { light: "#000000" } }), /light, dark/);
  theme.dispose();
});

test("theme 字号与字体族按角色写入并做范围校验", () => {
  const { dom, theme, events } = themeFixture();
  theme.setFontSize("text", 16);
  assert.equal(theme.getTheme().fontSizes.text, 16);
  assert.equal(dom.window.document.documentElement.style.getPropertyValue("--dsh-content-font-size"), "16px");
  assert.equal(events.at(-1).type, "theme/change");
  assert.throws(() => theme.setFontSize("text", 40), /outside 10\.\.22/);
  assert.throws(() => theme.setFontSize("text", 14.5), /outside 10\.\.22/);
  assert.throws(() => theme.setFontSize("sidebar", 14), /role/);
  theme.setFontFamily("code", " Fira Code , Menlo ");
  assert.equal(theme.getTheme().fontFamilies.code, "Fira Code, Menlo");
  assert.equal(dom.window.document.documentElement.style.getPropertyValue("--dsh-font-family-code"), "Fira Code, Menlo");
  theme.setFontFamily("code", "");
  assert.equal(dom.window.document.documentElement.style.getPropertyValue("--dsh-font-family-code"), "");
  theme.dispose();
});

test("theme 释放后拒绝写入并清理 token", () => {
  const { dom, theme } = themeFixture();
  theme.dispose();
  assert.throws(() => theme.setTheme("light"), /disposed/);
  assert.equal(dom.window.document.documentElement.style.getPropertyValue("--dsw-alias-bg-base"), "");
});

test("timer.timeout 支持回调与 Promise 两种签名", async () => {
  const timer = createDshTimerService();
  let calls = 0;
  timer.timeout(() => { calls += 1; }, 5);
  await delay(30);
  assert.equal(calls, 1);
  const cancel = timer.timeout(() => { calls += 1; }, 5);
  cancel();
  await delay(30);
  assert.equal(calls, 1);
  await timer.timeout(5);
  timer.dispose();
});

test("timer.interval 支持回调与异步迭代两种签名", async () => {
  const timer = createDshTimerService();
  let ticks = 0;
  const stop = timer.interval(() => { ticks += 1; }, 5);
  await delay(30);
  stop();
  const observed = ticks;
  assert.ok(observed >= 1);
  await delay(25);
  assert.equal(ticks, observed);
  const iterator = timer.interval(5);
  const first = await iterator.next();
  assert.equal(first.done, false);
  await iterator.return();
  assert.equal((await iterator.next()).done, true);
  timer.dispose();
});

test("timer.throttle 保持前沿触发与尾随补齐", async () => {
  const timer = createDshTimerService();
  const calls = [];
  const throttled = timer.throttle((value) => calls.push(value), 20);
  throttled(1);
  throttled(2);
  throttled(3);
  assert.deepEqual(calls, [1]);
  await delay(60);
  assert.deepEqual(calls, [1, 3]);
  throttled.dispose();
  throttled(4);
  await delay(60);
  assert.deepEqual(calls, [1, 3]);
  const noTrailingCalls = [];
  const leading = timer.throttle((value) => noTrailingCalls.push(value), 20, true);
  leading(1);
  leading(2);
  await delay(60);
  assert.deepEqual(noTrailingCalls, [1]);
  leading.dispose();
  timer.dispose();
});

test("timer.debounce 合并尾随调用", async () => {
  const timer = createDshTimerService();
  const calls = [];
  const debounced = timer.debounce((value) => calls.push(value), 10);
  debounced(1);
  debounced(2);
  await delay(40);
  assert.deepEqual(calls, [2]);
  debounced.dispose();
  debounced(3);
  await delay(30);
  assert.deepEqual(calls, [2]);
  timer.dispose();
});

test("timer 释放后清空挂起任务并拒绝新任务", async () => {
  const timer = createDshTimerService();
  let calls = 0;
  timer.timeout(() => { calls += 1; }, 5);
  timer.interval(() => { calls += 1; }, 5);
  timer.dispose();
  await delay(30);
  assert.equal(calls, 0);
  assert.throws(() => timer.timeout(() => {}, 1), /disposed/);
  assert.throws(() => timer.debounce(() => {}, 1), /disposed/);
});

test("客户端上下文注入 Cordis 风格定时器方法", () => {
  const timer = createDshTimerService();
  const runtime = createDshClientContext({
    pluginId: "fixture",
    slots: {},
    locale: {},
    services: { timer },
  });
  assert.equal(runtime.context.timer, timer);
  for (const method of ["timeout", "interval", "throttle", "debounce", "sleep"]) {
    assert.equal(typeof runtime.context[method], "function", method);
  }
  timer.dispose();
});
