import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../../../..");
const require = createRequire(import.meta.url);
const { chromium } = require(process.env.AMITIA_PLAYWRIGHT_MODULE || "playwright");
const frontendRequire = createRequire(path.join(root, "front/package.json"));
const zxingRequire = createRequire(frontendRequire.resolve("@zxing/browser"));
const { RGBLuminanceSource, HybridBinarizer, BinaryBitmap, QRCodeReader } = zxingRequire("@zxing/library");
const bundleRoot = path.join(root, "plugins/channels/wechat/.build/package/modules/wechat-channel-ui/desktop");
const output = path.join(root, "artifacts/wechat-ui-verification");
fs.mkdirSync(output, { recursive: true });
const browser = await chromium.launch({ headless: true, channel: "msedge" });
const errors = [];
const loginURL = "https://liteapp.weixin.qq.com/q/verification?qrcode=verification&bot_type=3";
const page = await browser.newPage({ viewport: { width: 1024, height: 900 } });
await page.clock.install();
page.on("pageerror", (error) => errors.push(error.message));
page.on("console", (message) => { if (message.type() === "error") errors.push(message.text()); });
await page.route("http://wechat-plugin.test/**", async (route) => {
  const name = new URL(route.request().url()).pathname.slice(1);
  if (!name) {
    await route.fulfill({ contentType: "text/html", body: '<html><body style="margin:0"><iframe title="微信插件" src="/index.html" sandbox="allow-scripts" style="width:100%;height:100vh;border:0"></iframe></body></html>' });
    return;
  }
  assert.ok(["index.html", "app.js", "styles.css"].includes(name));
  await route.fulfill({
    body: fs.readFileSync(path.join(bundleRoot, name)),
    contentType: name.endsWith(".css") ? "text/css" : name.endsWith(".js") ? "application/javascript" : "text/html",
    headers: { "Content-Security-Policy": "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data: blob:; font-src 'self'; connect-src 'none'; object-src 'none'; frame-src 'none'; worker-src 'none'; base-uri 'none'; form-action 'none'" },
  });
});
await page.addInitScript(({ loginURL }) => {
  window.testStatus = { status: "idle" };
  window.testCalls = [];
  window.failConnect = false;
  window.amitiaUI = {
    ready: async () => {},
    invokeAction: async (action, input) => {
      window.testCalls.push({ action, input });
      if (action === "connect") {
        if (window.failConnect) return { success: false, message: "二维码服务暂时不可用" };
        window.testStatus = { status: "qr_ready", qrCodeUrl: loginURL };
      }
      if (action === "disconnect") window.testStatus = { status: "disconnected" };
      return { success: true, data: { ...window.testStatus } };
    },
  };
}, { loginURL });

try {
  await page.goto("http://wechat-plugin.test/");
  const ui = page.frameLocator("iframe");
  await ui.getByRole("button", { name: "获取二维码", exact: true }).click();
  const image = ui.getByRole("img", { name: "微信登录二维码" });
  await image.waitFor();
  await image.evaluate((image) => new Promise((resolve) => {
    if (image.complete && image.naturalWidth > 0) resolve();
    else image.onload = resolve;
  }));
  const pixels = await image.evaluate((image) => {
    const canvas = document.createElement("canvas");
    canvas.width = canvas.height = 560;
    const context = canvas.getContext("2d");
    context.drawImage(image, 0, 0, 560, 560);
    const rgba = context.getImageData(0, 0, 560, 560).data;
    return Array.from({ length: 560 * 560 }, (_, index) => rgba[index * 4]);
  });
  const luminance = new RGBLuminanceSource(Uint8ClampedArray.from(pixels), 560, 560);
  assert.equal(new QRCodeReader().decode(new BinaryBitmap(new HybridBinarizer(luminance))).getText(), loginURL);
  await page.screenshot({ path: path.join(output, "desktop.png"), fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  assert.equal(await image.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
  await page.screenshot({ path: path.join(output, "narrow.png"), fullPage: true });
  await image.evaluate(() => { window.testStatus = { status: "connected", accountId: "verification-account", messageCount: 4, replyCount: 3 }; });
  await page.clock.runFor(5001);
  await ui.getByRole("button", { name: "刷新状态", exact: true }).click();
  await ui.getByRole("heading", { name: "微信已连接" }).waitFor();
  assert.equal(await image.count(), 0);
  await ui.getByRole("button", { name: "断开连接", exact: true }).click();
  await ui.getByRole("button", { name: "取消", exact: true }).click();
  assert.equal(await ui.getByRole("heading", { name: "微信已连接" }).evaluate(() => window.testCalls.some((call) => call.action === "disconnect")), false);
  await ui.getByRole("button", { name: "断开连接", exact: true }).click();
  await ui.getByRole("button", { name: "确认", exact: true }).click();
  const loginButton = ui.getByRole("button", { name: "获取二维码", exact: true });
  await loginButton.waitFor();
  await loginButton.evaluate(() => { window.failConnect = true; });
  await loginButton.click();
  await ui.getByRole("alert").filter({ hasText: "二维码服务暂时不可用" }).waitFor();
  assert.equal(await loginButton.isEnabled(), true);
  assert.deepEqual(errors, []);
  console.log(JSON.stringify({ result: "passed", checks: ["strict-csp", "qr-roundtrip", "desktop-layout", "narrow-layout", "connected-state", "disconnect-confirmation", "service-failure", "no-browser-errors"], output }));
} finally {
  await browser.close();
}
