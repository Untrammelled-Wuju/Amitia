import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { basename } from "node:path";

const file = process.argv[2];
if (!file) throw new Error("usage: node scripts/verify-package.mjs <package.amitiax>");
const list = execFileSync("unzip", ["-Z1", file], { encoding: "utf8" }).trim().split(/\r?\n/).filter(Boolean);
const manifest = JSON.parse(execFileSync("unzip", ["-p", file, "manifest.json"], { encoding: "utf8" }));
const channelUi = execFileSync("unzip", ["-p", file, "modules/wechat-personal-channel-ui/desktop/app.js"], { encoding: "utf8" });
if (channelUi.includes("setInterval(")) {
  throw new Error("channel UI must not poll channel status");
}
if (!manifest.compatibility?.platforms?.includes("windows") || !manifest.compatibility?.platforms?.includes("linux")) {
  throw new Error("package must declare both windows and linux");
}
const contributions = (manifest.modules || []).flatMap((mod) => mod.contributions || []);
for (const providerId of [
  "wechat-personal-routes",
  "wechat-personal-page-provider",
  "wechat-personal-messages-page-provider",
  "wechat-personal-drawer",
]) {
  const provider = contributions.find((item) => item.spec?.providerId === providerId);
  if (!provider?.spec?.entries?.mobile) {
    throw new Error(`mobile entry missing for ${providerId}`);
  }
}
for (const mod of manifest.modules || []) {
  for (const companion of mod.runtime?.nativeCompanions || []) {
    const path = `modules/${mod.id}/${companion.path}`;
    if (!list.includes(path)) throw new Error(`missing companion ${path}`);
    const bytes = execFileSync("unzip", ["-p", file, path], { encoding: null, maxBuffer: 32 * 1024 * 1024 });
    const actual = createHash("sha256").update(bytes).digest("hex");
    if (actual !== companion.sha256) throw new Error(`hash mismatch ${path}`);
  }
}
console.log(`verified=${basename(file)}`);
console.log(`version=${manifest.extension.version}`);
console.log(`platforms=${manifest.compatibility.platforms.join(",")}`);
