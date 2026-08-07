import { createHash } from "node:crypto";
import { spawnSync } from "node:child_process";
import { mkdtempSync, rmSync, writeFileSync, mkdirSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const assert = (cond, msg) => {
  if (!cond) throw new Error("断言失败: " + msg);
};

assert(process.platform === "linux", `platform 期望 linux, 实际 ${process.platform}`);
assert(process.arch === "arm64", `arch 期望 arm64, 实际 ${process.arch}`);
assert(process.version === "v24.19.0", `version 期望 v24.19.0, 实际 ${process.version}`);
console.log(`[PASS] platform=${process.platform} arch=${process.arch} version=${process.version}`);

assert(process.versions.napi === "10", `N-API 期望 10, 实际 ${process.versions.napi}`);
console.log(`[PASS] N-API=${process.versions.napi}`);

const baseDir = process.env.TMPDIR || tmpdir();
mkdirSync(baseDir, { recursive: true });
const dir = mkdtempSync(join(baseDir, "node-test-"));
try {
  const file = join(dir, "测试.txt");
  const content = "Amitia 中文测试 UTF-8";
  writeFileSync(file, content, "utf-8");
  const read = readFileSync(file, "utf-8");
  assert(read === content, "读写不一致");
  console.log("[PASS] fs UTF-8 中文读写");
} finally {
  rmSync(dir, { recursive: true, force: true });
}

const h = createHash("sha256").update("amitia").digest("hex");
assert(h.length === 64, "sha256 结果异常");
console.log("[PASS] crypto SHA-256");

const r1 = spawnSync(process.execPath, ["--eval", "process.exit(0)"], { encoding: "utf-8" });
assert(r1.status === 0, `退出码期望 0, 实际 ${r1.status}`);
const r2 = spawnSync(process.execPath, ["--eval", "process.exit(7)"], { encoding: "utf-8" });
assert(r2.status === 7, `退出码期望 7, 实际 ${r2.status}`);
console.log("[PASS] child_process 退出码");

const m = await import("node:assert/strict");
assert(typeof m.default.equal === "function", "ESM import 失败");
console.log("[PASS] ESM import");

const path = await import("node:path");
assert(typeof path.join === "function", "CommonJS 兼容失败");
console.log("[PASS] CommonJS 兼容");

process.stdout.write("[PASS] stdout 输出正常\n");
process.stderr.write("[PASS] stderr 输出正常\n");

console.log("\n[test-runtime-simple.mjs] 全部测试通过");
