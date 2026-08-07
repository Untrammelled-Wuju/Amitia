import { createHash } from "node:crypto";
import { spawnSync } from "node:child_process";
import { mkdtempSync, rmSync, writeFileSync, mkdirSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { Worker, isMainThread, parentPort, workerData } from "node:worker_threads";

const __filenameSelf = fileURLToPath(import.meta.url);

const assert = (cond, msg) => {
  if (!cond) {
    throw new Error("断言失败: " + msg);
  }
};

const ensureTmpDir = () => {
  const dir = process.env.TMPDIR || tmpdir();
  mkdirSync(dir, { recursive: true });
  return dir;
};

const checkPlatform = () => {
  assert(process.platform === "linux", `platform 期望 linux, 实际 ${process.platform}`);
  assert(process.arch === "arm64", `arch 期望 arm64, 实际 ${process.arch}`);
  assert(process.version === "v24.19.0", `version 期望 v24.19.0, 实际 ${process.version}`);
  console.log(`[PASS] platform=${process.platform} arch=${process.arch} version=${process.version}`);
};

const checkNapi = () => {
  const napi = process.versions.napi;
  assert(napi !== undefined, "N-API 版本未定义");
  assert(napi === "10", `N-API 版本期望 10 (Node-API for Node 24), 实际 ${napi}`);
  console.log(`[PASS] N-API=${napi}`);
};

const checkFs = async () => {
  const { readFile, writeFile } = await import("node:fs/promises");
  const baseDir = ensureTmpDir();
  const dir = mkdtempSync(join(baseDir, "node-test-"));
  try {
    const file = join(dir, "测试-test.txt");
    const content = "你好世界 Amitia 测试 中文 UTF-8";
    await writeFile(file, content, "utf-8");
    const read = await readFile(file, "utf-8");
    assert(read === content, "读写内容不一致");
    console.log("[PASS] fs/promises UTF-8 读写");
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
};

const checkCrypto = () => {
  const h = createHash("sha256").update("amitia").digest("hex");
  assert(typeof h === "string" && h.length === 64, "crypto sha256 结果异常");
  console.log("[PASS] crypto SHA-256");
};

const checkWorkerThreads = () => {
  if (!isMainThread) {
    parentPort.postMessage(workerData * 2);
    return;
  }
  return new Promise((resolve, reject) => {
    const w = new Worker(__filenameSelf, { workerData: 21 });
    w.on("message", (msg) => {
      assert(msg === 42, `Worker 期望 42, 实际 ${msg}`);
      console.log("[PASS] worker_threads");
      resolve();
    });
    w.on("error", reject);
  });
};

const checkChildProcess = () => {
  const result = spawnSync(process.execPath, ["--eval", "process.exit(0)"], { encoding: "utf-8" });
  assert(result.status === 0, `子进程退出码期望 0, 实际 ${result.status}`);
  const result2 = spawnSync(process.execPath, ["--eval", "process.exit(7)"], { encoding: "utf-8" });
  assert(result2.status === 7, `子进程退出码期望 7, 实际 ${result2.status}`);
  console.log("[PASS] child_process");
};

const checkEsm = async () => {
  const m = await import("node:assert/strict");
  assert(typeof m.default.equal === "function", "ESM import 失败");
  console.log("[PASS] ESM import");
};

const checkCjs = async () => {
  const path = await import("node:path");
  assert(typeof path.join === "function", "CommonJS 兼容模块加载失败");
  console.log("[PASS] CommonJS require 兼容");
};

const checkStdout = () => {
  process.stdout.write("[PASS] stdout 输出正常\n");
  process.stderr.write("[PASS] stderr 输出正常\n");
};

const main = async () => {
  checkPlatform();
  checkNapi();
  await checkFs();
  checkCrypto();
  await checkWorkerThreads();
  checkChildProcess();
  await checkEsm();
  await checkCjs();
  checkStdout();
  console.log("\n[test-runtime.mjs] 全部测试通过");
};

if (isMainThread) {
  main().catch((err) => {
    console.error("[test-runtime.mjs] 测试失败:", err);
    process.exit(1);
  });
}
