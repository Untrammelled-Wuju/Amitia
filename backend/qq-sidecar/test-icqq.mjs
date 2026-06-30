// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { createClient } from "icqq";

const client = createClient({ platform: 3 });

client.on("system.online", () => {
    console.log("登录成功！");
    process.exit(0);
});

client.on("system.login.error", (e) => {
    console.log("登录失败:", e.message || e);
    process.exit(1);
});

client.on("system.login.qrcode", () => {
    console.log("收到二维码，请扫码...");
    // 在这个测试中我们无法扫码，只是确认流程能走到这一步
    setTimeout(() => process.exit(0), 30000);
});

client.on("system.login.device", () => {
    console.log("需要设备验证...");
});

console.log("正在连接...");
client.login();
