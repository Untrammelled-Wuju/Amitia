import { app } from "electron";
import { spawn, ChildProcessWithoutNullStreams } from "child_process";
import { createHash } from "node:crypto";
import http from "http";
import path from "path";
import fs from "fs";
import {
  getAmitiaDataDir,
  ensureAmitiaDataDir,
  getInstallDir,
  isDevMode,
} from "./path-manager";
import { validateCorePrerequisites, validateDeviceAgentPrerequisites } from "./core-prereq";
import type { CorePrerequisiteResult } from "./core-prereq";
import { getLocalAdminHeaders } from "./backend-session-client";
import { buildCoreSpawnArgs, parseCoreChildPid } from "./core-supervisor";
import {
  databaseSourceCandidates,
  firstExisting,
  resolveCorePath,
} from "./platform-artifacts";
import {
  getCoreBaseURL,
  invalidateCoreEndpoint,
  isCoreEndpointOwnedByProcess,
  legacyCoreEndpoint,
  readCoreEndpoint,
  removeCoreEndpoint,
  setCoreEndpoint,
} from "./core-endpoint";
import { resolveWindowsPowerShellPath } from "./windows-elevation";

export type { CorePrerequisiteResult };

export type BundledCoreProfile = "local" | "device-agent";

let coreProcess: ChildProcessWithoutNullStreams | null = null;
let runningCoreProfile: BundledCoreProfile | null = null;
let coreGeneration = 0;
let coreSpawnedAt = 0;
let coreChildPid = 0;
const CORE_SHUTDOWN_TIMEOUT_MS = 30000;
const CORE_SHUTDOWN_PROCESS_WAIT_MS = 35000;

process.on("exit", () => {
  const pid = coreProcess?.pid;
  if (!pid) {
    return;
  }
  try {
    removeCoreEndpoint(getAmitiaDataDir(), coreChildPid || undefined);
  } catch {
    console.error("[CoreManager] 退出时清理核心端点失败");
  }
  try {
    if (coreProcess && coreProcess.exitCode === null && !coreProcess.killed) {
      try {
        coreProcess.stdin?.write("shutdown\n");
      } catch {
      }
      coreProcess.kill();
    }
  } catch {
    return;
  }
});

const CORE_ERROR_PATTERNS = [
  /\[(error|fatal|panic)\]/i,
  /\bpanic[: ]/i,
  /"@level":"(error|fatal)"/i,
  /\berrorcode=\S+/i,
  /\bresult=(failed|error)\b/i,
];

export function isCoreErrorLine(line: string): boolean {
  if (/deprecationwarning/i.test(line)) return false;
  return CORE_ERROR_PATTERNS.some((pattern) => pattern.test(line));
}

export function getCorePath(): string {
  return resolveCorePath(getCoreResourcesPath());
}

export function getCoreResourcesPath(): string {
  if (!isDevMode()) {
    return process.resourcesPath;
  }
  return path.join(getInstallDir(), "resources");
}

export function getRunningCoreProfile(): BundledCoreProfile | null {
  return runningCoreProfile;
}

export function ensureDataAndConfig(): { ok: boolean; errors: string[] } {
  const dataDir = ensureAmitiaDataDir();
  ensureDefaultConfig(dataDir);
  ensureLocalToken(dataDir);
  ensureCoreBinaries(dataDir);
  const validation = validateCorePrerequisites(dataDir, getCorePath());
  return { ok: validation.ok, errors: validation.missing };
}

export function ensureCorePrerequisites(profile: BundledCoreProfile): { ok: boolean; errors: string[] } {
  const dataDir = ensureAmitiaDataDir();
  ensureDefaultConfig(dataDir);
  ensureLocalToken(dataDir);

  if (profile === "local") {
    ensureCoreBinaries(dataDir);
    const validation = validateCorePrerequisites(dataDir, getCorePath());
    return { ok: validation.ok, errors: validation.missing };
  }

  const validation = validateDeviceAgentPrerequisites(dataDir, getCorePath());
  return { ok: validation.ok, errors: validation.missing };
}

const CURRENT_CONFIG_VERSION = 2;

const CONFIG_MIGRATIONS: Array<{
  from: number;
  to: number;
  apply: (config: Record<string, unknown>) => void;
}> = [
  {
    from: 1,
    to: 2,
    apply: (config: Record<string, unknown>) => {
      if (!config.security) {
        config.security = {};
      }
      const sec = config.security as Record<string, unknown>;
      if (!sec.localTokenStorageKey) {
        sec.localTokenStorageKey = "security/local-token";
      }
    },
  },
];

function migrateConfig(configPath: string): void {
  try {
    const content = fs.readFileSync(configPath, "utf8");
    const lines = content.split("\n");
    let configVersion = 1;
    for (const line of lines) {
      const trimmed = line.trim();
      if (trimmed.startsWith("config:") || trimmed.startsWith("version:")) {
        const match = trimmed.match(/version:\s*(\d+)/);
        if (match) {
          configVersion = parseInt(match[1], 10);
          break;
        }
      }
    }

    if (configVersion >= CURRENT_CONFIG_VERSION) {
      return;
    }

    let config: Record<string, unknown> = {};
    try {
      const yaml = require("yaml");
      config = yaml.parse(content) || {};
    } catch {
      config = {};
    }

    const applicableMigrations = CONFIG_MIGRATIONS.filter(
      (m) => m.from >= configVersion,
    );
    for (const migration of applicableMigrations) {
      migration.apply(config);
    }

    const configSection = config.config as Record<string, unknown> | undefined;
    if (configSection) {
      configSection.version = CURRENT_CONFIG_VERSION;
    } else {
      config.config = { version: CURRENT_CONFIG_VERSION };
    }

    try {
      const yaml = require("yaml");
      const backupPath = `${configPath}.bak`;
      fs.copyFileSync(configPath, backupPath);
      const tmpPath = `${configPath}.tmp`;
      fs.writeFileSync(tmpPath, yaml.stringify(config), { encoding: "utf8" });
      fs.renameSync(tmpPath, configPath);
      console.log(`[CoreManager] 配置已迁移至版本 ${CURRENT_CONFIG_VERSION}`);
    } catch (err) {
      console.error("[CoreManager] 配置迁移写入失败:", err);
    }
  } catch (err) {
    console.error("[CoreManager] 配置迁移失败:", err);
  }
}

type YamlScalarLocation = {
  index: number;
  indent: number;
  key: string;
  value: string;
};

function yamlScalarValue(raw: string): string {
  const trimmed = raw.trim();
  if (trimmed.length >= 2) {
    const first = trimmed[0];
    const last = trimmed[trimmed.length - 1];
    if ((first === '"' && last === '"') || (first === "'" && last === "'")) {
      return trimmed.slice(1, -1);
    }
  }
  return trimmed.replace(/\s+#.*$/, "").trim();
}

function findYamlScalar(lines: string[], pathParts: string[]): YamlScalarLocation | null {
  const stack: Array<{ indent: number; key: string }> = [];
  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index];
    if (!line.trim() || line.trimStart().startsWith("#")) {
      continue;
    }
    const match = line.match(/^(\s*)([A-Za-z0-9_-]+):(?:\s*(.*))?$/);
    if (!match) {
      continue;
    }

    const indent = match[1].replace(/\t/g, "  ").length;
    while (stack.length > 0 && stack[stack.length - 1].indent >= indent) {
      stack.pop();
    }

    const key = match[2];
    const rawValue = (match[3] ?? "").trim();
    const currentPath = [...stack.map((entry) => entry.key), key];
    if (currentPath.length === pathParts.length && currentPath.every((part, i) => part === pathParts[i])) {
      return { index, indent, key, value: yamlScalarValue(rawValue) };
    }

    if (rawValue === "") {
      stack.push({ indent, key });
    }
  }
  return null;
}

function setYamlScalar(lines: string[], pathParts: string[], value: string): void {
  const existing = findYamlScalar(lines, pathParts);
  const escaped = value.replace(/\\/g, "\\\\").replace(/"/g, '\\"');
  if (existing) {
    lines[existing.index] = `${" ".repeat(existing.indent)}${existing.key}: "${escaped}"`;
    return;
  }

  const parentPath = pathParts.slice(0, -1);
  const parent = findYamlScalar(lines, parentPath);
  if (!parent) {
    throw new Error(`配置缺少必要节点: ${parentPath.join(".")}`);
  }
  lines.splice(parent.index + 1, 0, `${" ".repeat(parent.indent + 2)}${pathParts[pathParts.length - 1]}: "${escaped}"`);
}

function ensureGeneratedRuntimeSecrets(configPath: string): void {
  try {
    const original = fs.readFileSync(configPath, "utf8");
    const hadTrailingNewline = /\r?\n$/.test(original);
    const lines = original.replace(/\r\n/g, "\n").split("\n");
    if (hadTrailingNewline && lines[lines.length - 1] === "") {
      lines.pop();
    }

    const { randomBytes } = require("crypto");
    let changed = false;

    const surrealPasswordPath = ["providers", "graphStore", "surrealdb", "password"];
    const surrealPassword = findYamlScalar(lines, surrealPasswordPath)?.value.trim() ?? "";
    const insecureSurreal =
      surrealPassword.length < 24 ||
      ["root", "admin", "password"].includes(surrealPassword.toLowerCase());
    if (insecureSurreal) {
      setYamlScalar(lines, surrealPasswordPath, randomBytes(32).toString("base64url"));
      changed = true;
      console.warn("[CoreManager] 检测到缺失/不安全的 SurrealDB 密码，已生成本机随机凭据");
    }

    if (!changed) {
      return;
    }

    const output = lines.join("\n") + (hadTrailingNewline ? "\n" : "");
    const tmpPath = `${configPath}.secrets.tmp`;
    fs.writeFileSync(tmpPath, output, { encoding: "utf8", mode: 0o600 });
    try {
      fs.chmodSync(tmpPath, 0o600);
    } catch {
      // Windows may ignore POSIX permission bits.
    }
    try {
      fs.renameSync(tmpPath, configPath);
    } catch {
      fs.copyFileSync(tmpPath, configPath);
      fs.unlinkSync(tmpPath);
    }
    try {
      fs.chmodSync(configPath, 0o600);
    } catch {
      // Windows may ignore POSIX permission bits.
    }
  } catch (err) {
    throw new Error(`安全凭据初始化失败: ${err instanceof Error ? err.message : String(err)}`);
  }
}

function ensureDefaultConfig(dataDir: string): void {
  const configDir = path.join(dataDir, "config");
  const destConfig = path.join(configDir, "config.yml");
  const resourcesPath = getCoreResourcesPath();
  const templatePath = path.join(
    resourcesPath,
    "config-template",
    "config.yaml",
  );
  console.log("[CoreManager] 配置模板路径:", templatePath);
  fs.mkdirSync(configDir, { recursive: true });
  if (!fs.existsSync(templatePath)) {
    console.error("[CoreManager] 配置模板不存在:", templatePath);
    return;
  }
  if (!fs.existsSync(destConfig)) {
    fs.copyFileSync(templatePath, destConfig, fs.constants.COPYFILE_EXCL);
    console.log("[CoreManager] 配置文件已创建:", destConfig);
    migrateConfig(destConfig);
    ensureGeneratedRuntimeSecrets(destConfig);
    return;
  }
  console.log("[CoreManager] 配置文件已存在, 跳过覆盖:", destConfig);
  migrateConfig(destConfig);
  ensureGeneratedRuntimeSecrets(destConfig);
}

function ensureLocalToken(dataDir: string): void {
  const tokenDir = path.join(dataDir, "security");
  const tokenFile = path.join(tokenDir, "local-token");
  fs.mkdirSync(tokenDir, { recursive: true });
  if (fs.existsSync(tokenFile)) {
    return;
  }
  const { randomBytes } = require("crypto");
  const token = randomBytes(32).toString("base64url");
  fs.writeFileSync(tokenFile, token, { mode: 0o600 });
  console.log("[CoreManager] 本地安全令牌已生成");
}

function ensureCoreBinaries(dataDir: string): void {
  const resourcesPath = getCoreResourcesPath();
  const extraRoots = isDevMode()
    ? [path.resolve(getInstallDir(), "..", "backend")]
    : [];
  const nodeArchiveNames = process.platform === "win32"
    ? ["node.exe.zip", "node.zip"]
    : ["node.zip"];
  const nodeArchive = nodeArchiveNames
    .map((name) => path.join(resourcesPath, "core", "node", name))
    .find((file) => fs.existsSync(file));
  const binaries = [
    {
      sources: databaseSourceCandidates(resourcesPath, "qdrant", process.platform, process.arch, extraRoots),
      dest: path.join(dataDir, "qdrant", "qdrant.zip"),
    },
    {
      sources: databaseSourceCandidates(resourcesPath, "surrealdb", process.platform, process.arch, extraRoots),
      dest: path.join(dataDir, "surrealdb", "surreal.zip"),
    },
  ];
  if (nodeArchive) {
    ensureBundledNodeArchive(dataDir, nodeArchive);
  }
  for (const { sources, dest } of binaries) {
    if (fs.existsSync(dest)) {
      console.log("[CoreManager] 二进制已存在, 跳过复制:", dest);
      continue;
    }
    const src = firstExisting(sources);
    if (src) {
      fs.mkdirSync(path.dirname(dest), { recursive: true });
      fs.copyFileSync(src, dest);
      console.log("[CoreManager] 二进制已复制:", src, "->", dest);
    } else {
      console.error("[CoreManager] 二进制资源不存在:", sources.join(", "));
    }
  }
  syncRuntimeHostAssets(dataDir, getBundledRuntimeRoot());
}

function getBundledRuntimeRoot(): string {
  if (isDevMode()) {
    return path.resolve(getInstallDir(), "..", "runtime");
  }
  return path.join(getCoreResourcesPath(), "runtime");
}

export function syncRuntimeHostAssets(dataDir: string, runtimeRoot: string): void {
  for (const host of ["plugin-host", "task-host"]) {
    const source = path.join(runtimeRoot, host, "dist");
    const destination = path.join(dataDir, "runtime", host, "dist");
    if (!fs.existsSync(path.join(source, "index.js"))) {
      console.error("[CoreManager] 运行时宿主资源不存在:", source);
      continue;
    }
    fs.rmSync(destination, { recursive: true, force: true });
    fs.mkdirSync(path.dirname(destination), { recursive: true });
    fs.cpSync(source, destination, { recursive: true, force: true });
    console.log("[CoreManager] 运行时宿主已同步:", destination);
  }
}

function ensureBundledNodeArchive(dataDir: string, sourceArchive: string): void {
  const nodeDir = path.join(dataDir, "node");
  const destination = path.join(nodeDir, path.basename(sourceArchive));
  const sourceHash = fileSha256(sourceArchive);
  const destinationHash = fs.existsSync(destination)
    ? fileSha256(destination)
    : "";
  if (sourceHash === destinationHash) {
    return;
  }
  fs.mkdirSync(nodeDir, { recursive: true });
  fs.copyFileSync(sourceArchive, destination);
  const staleArchives = process.platform === "win32"
    ? ["node.exe.zip", "node.zip"]
    : ["node.zip"];
  for (const archive of staleArchives) {
    if (archive === path.basename(destination)) {
      continue;
    }
    fs.rmSync(path.join(nodeDir, archive), { force: true });
  }
  const extractedNodes = process.platform === "win32"
    ? ["node.exe", path.join("bin", "node.exe")]
    : [path.join("bin", "node"), "node"];
  for (const nodePath of extractedNodes) {
    fs.rmSync(path.join(nodeDir, nodePath), { force: true });
  }
  console.log("[CoreManager] Node 运行时已更新:", sourceArchive, "->", destination);
}

function fileSha256(filePath: string): string {
  return createHash("sha256").update(fs.readFileSync(filePath)).digest("hex");
}

export function startCore(profile: BundledCoreProfile): void {
  if (coreProcess && runningCoreProfile === profile) {
    console.log("[CoreManager] 核心进程已在运行且Profile相同, 跳过");
    return;
  }

  if (coreProcess) {
    console.log("[CoreManager] 当前Profile与请求不同, 需要先停止当前进程");
    throw new Error("需要先停止当前Profile才能启动新Profile");
  }

  lastHealthyAt = 0;
  invalidateCoreEndpoint();
  ensureHealthPolling();
  coreChildPid = 0;

  const corePath = getCorePath();
  const dataDir = getAmitiaDataDir();
  removeCoreEndpoint(dataDir);

  console.log("[CoreManager] 核心路径:", corePath);
  console.log("[CoreManager] 数据目录:", dataDir);
  console.log(`[CoreManager] Runtime Profile: ${profile}`);
  console.log("[CoreManager] 核心文件存在:", fs.existsSync(corePath));
  console.log(
    "[CoreManager] 核心文件大小:",
    fs.existsSync(corePath) ? fs.statSync(corePath).size : 0,
  );

  const prereq = profile === "local"
    ? validateCorePrerequisites(dataDir, corePath)
    : validateDeviceAgentPrerequisites(dataDir, corePath);
  if (!prereq.ok) {
    const msg = `Amitia Core启动前置条件不满足，缺失以下文件:\n${prereq.missing.map((m) => `  - ${m}`).join("\n")}`;
    console.error("[CoreManager]", msg);
    throw new Error(msg);
  }

  const configPath = path.join(dataDir, "config");
  const bundledPowerShellPath =
    process.platform === "win32"
      ? resolveWindowsPowerShellPath()
      : undefined;
  const env: NodeJS.ProcessEnv = {
    ...process.env,
    CONFIG_PATH: configPath,
    AMITIA_RUN_MODE: "desktop",
    AMITIA_RUNTIME_ROOT: dataDir,
    AMITIA_DATA_DIR: dataDir,
    AMITIA_EXTENSION_DEV_MODE: isDevMode() ? "true" : "false",
    AMITIA_VERSION: app.getVersion(),
    ...(bundledPowerShellPath
      ? { AMITIA_POWERSHELL_PATH: bundledPowerShellPath }
      : {}),
  };

  console.log("[CoreManager] CONFIG_PATH:", configPath);
  console.log("[CoreManager] cwd:", dataDir);

  const currentGeneration = ++coreGeneration;
  coreSpawnedAt = Date.now();
  const coreArgs = buildCoreSpawnArgs(process.platform, profile);
  coreProcess = spawn(corePath, coreArgs, {
    cwd: dataDir,
    env,
    windowsHide: true,
    detached: process.platform !== "win32",
  });
  runningCoreProfile = profile;
  const pid = coreProcess.pid ?? 0;

  console.log(`[CoreManager] 核心进程已启动, PID: ${pid}, generation: ${currentGeneration}`);

  let stdoutBuffer = "";
  coreProcess.stdout.on("data", (data) => {
    stdoutBuffer += data.toString();
    const lines = stdoutBuffer.split(/\r?\n/);
    stdoutBuffer = lines.pop() ?? "";
    for (const raw of lines) {
      const line = raw.trim();
      if (!line) continue;
      const childPid = parseCoreChildPid(line);
      if (childPid !== null) {
        coreChildPid = childPid;
        console.log(`[CoreManager] 核心子进程 PID: ${coreChildPid}`);
        continue;
      }
      console.log(`[AmitiaCore] ${line}`);
    }
  });

  coreProcess.stderr.on("data", (data) => {
    for (const raw of data.toString().split(/\r?\n/)) {
      const line = raw.trim();
      if (!line) continue;
      if (isCoreErrorLine(line)) console.error(`[AmitiaCore] ${line}`);
      else console.log(`[AmitiaCore] ${line}`);
    }
  });

  coreProcess.on("exit", (code, signal) => {
    console.log(`[AmitiaCore] 进程退出, code=${code}, signal=${signal}, generation=${currentGeneration}`);
    if (currentGeneration === coreGeneration) {
      removeCoreEndpoint(dataDir, coreChildPid || undefined);
      coreChildPid = 0;
      coreProcess = null;
      runningCoreProfile = null;
      invalidateCoreEndpoint();
    }
  });

  coreProcess.on("error", (error) => {
    console.error("[AmitiaCore] 启动失败:", error.message);
    if (currentGeneration === coreGeneration) {
      removeCoreEndpoint(dataDir, coreChildPid || undefined);
      coreChildPid = 0;
      coreProcess = null;
      runningCoreProfile = null;
      invalidateCoreEndpoint();
    }
  });
}

export async function stopCore(): Promise<void> {
  if (coreProcess && !coreProcess.killed) {
    const currentGeneration = coreGeneration;
    const dataDir = getAmitiaDataDir();
    console.log("[CoreManager] 正在停止核心进程, PID:", coreProcess.pid);
    const pid = coreProcess.pid ?? 0;
    const shutdownBaseURL = getCoreBaseURL();
    const endpointPid = coreChildPid;
    invalidateCoreEndpoint();
    removeCoreEndpoint(dataDir, endpointPid || undefined);

    try {
      const success = await gracefulShutdown(pid, shutdownBaseURL);
      if (!success) {
        console.warn("[CoreManager] 优雅关闭超时，回退到强制终止");
        await forceStopRunner();
      }
    } catch (err) {
      console.error("[CoreManager] 优雅关闭异常，强制终止:", err);
      await forceStopRunner();
    }
    await waitForChildExit(coreProcess, CORE_SHUTDOWN_PROCESS_WAIT_MS);
    if (currentGeneration === coreGeneration) {
      removeCoreEndpoint(dataDir, endpointPid || undefined);
      coreChildPid = 0;
      coreProcess = null;
      runningCoreProfile = null;
      invalidateCoreEndpoint();
    }
    return;
  }
  coreChildPid = 0;
  coreProcess = null;
  runningCoreProfile = null;
  invalidateCoreEndpoint();
}

export async function ensureCoreProfile(profile: BundledCoreProfile): Promise<void> {
  if (coreProcess && runningCoreProfile === profile) {
    console.log("[CoreManager] 核心进程已是目标Profile, 等待就绪");
    await waitForCoreReady(profile);
    return;
  }

  if (coreProcess) {
    console.log(`[CoreManager] 切换Profile: ${runningCoreProfile} -> ${profile}`);
    await stopCore();
    await waitForProcessExit();
  }

  console.log(`[CoreManager] 启动核心进程, Profile: ${profile}`);
  startCore(profile);
  await waitForCoreReady(profile);
}

async function waitForProcessExit(): Promise<void> {
  const deadline = Date.now() + 10000;
  while (Date.now() < deadline && coreProcess !== null) {
    const healthUrl = `${getCoreBaseURL()}/livez`;
    const alive = await httpHealthCheck(healthUrl, 500);
    if (!alive) {
      console.log("[CoreManager] 核心服务已停止响应");
      break;
    }
    await new Promise((r) => setTimeout(r, 300));
  }
  if (coreProcess) {
    console.warn("[CoreManager] 等待进程退出超时");
  }
}

async function forceStopRunner(): Promise<void> {
  const child = coreProcess;
  if (!child || child.exitCode !== null || child.killed) return;
  try {
    child.stdin?.write("shutdown\n");
  } catch {
  }
  await new Promise((resolve) => setTimeout(resolve, 500));
  if (child.exitCode === null && !child.killed) {
    try {
      child.kill();
    } catch {
    }
  }
}

async function waitForChildExit(child: ChildProcessWithoutNullStreams | null, timeoutMs: number): Promise<void> {
  if (!child || child.exitCode !== null || child.killed) return;
  return new Promise((resolve) => {
    const timer = setTimeout(() => {
      child.removeAllListeners("exit");
      console.warn("[CoreManager] 等待进程退出超时");
      resolve();
    }, timeoutMs);
    child.once("exit", () => {
      clearTimeout(timer);
      resolve();
    });
  });
}

async function gracefulShutdown(pid: number, baseURL: string): Promise<boolean> {
  const healthUrl = `${baseURL}/livez`;
  const isAlive = await httpHealthCheck(healthUrl, 2000);
  if (!isAlive) {
    console.log("[CoreManager] 核心服务不可达，跳过优雅关闭");
    return false;
  }

  console.log("[CoreManager] 尝试优雅关闭核心服务...");
  const shutdownOk = await httpShutdown(`${baseURL}/api/local/admin/shutdown`, 3000, getLocalAdminHeaders());
  if (!shutdownOk) {
    console.warn("[CoreManager] 优雅关闭请求失败");
    return false;
  }

  const deadline = Date.now() + CORE_SHUTDOWN_TIMEOUT_MS;
  while (Date.now() < deadline) {
    if (coreProcess === null || coreProcess.killed || coreProcess.exitCode !== null) {
      console.log("[CoreManager] 核心进程已优雅退出");
      return true;
    }
    const stillAlive = await httpHealthCheck(healthUrl, 1000);
    if (!stillAlive) {
      console.log("[CoreManager] 核心服务已停止响应，等待进程退出");
    }
    await new Promise((r) => setTimeout(r, 500));
  }

  return false;
}

function httpShutdown(
  url: string,
  timeoutMs: number,
  headers: Record<string, string>,
): Promise<boolean> {
  return new Promise((resolve) => {
    const req = http.request(
      url,
      {
        method: "POST",
        timeout: timeoutMs,
        headers: {
          Accept: "application/json",
          ...headers,
        },
      },
      (res) => {
        res.resume();
        resolve(res.statusCode === 200 || res.statusCode === 202);
      },
    );
    req.on("error", () => resolve(false));
    req.on("timeout", () => {
      req.destroy();
      resolve(false);
    });
    req.end();
  });
}

let lastHealthyAt = 0;
let healthPollTimer: ReturnType<typeof setInterval> | null = null;

function ensureHealthPolling(): void {
  if (healthPollTimer !== null) return;
  healthPollTimer = setInterval(() => {
    const tracked =
      coreProcess !== null && !coreProcess.killed && coreProcess.exitCode === null;
    if (tracked) {
      lastHealthyAt = Date.now();
    }
  }, 2000);
}

export function isCoreRunning(): boolean {
  const tracked =
    coreProcess !== null && !coreProcess.killed && coreProcess.exitCode === null;
  if (tracked) {
    lastHealthyAt = Date.now();
    return true;
  }
  return Date.now() - lastHealthyAt < 8000;
}

function httpHealthCheck(url: string, timeoutMs: number): Promise<boolean> {
  return new Promise((resolve) => {
    const req = http.get(url, { timeout: timeoutMs }, (res) => {
      res.resume();
      resolve(res.statusCode === 200);
    });
    req.on("error", () => {
      resolve(false);
    });
    req.on("timeout", () => {
      req.destroy();
      resolve(false);
    });
  });
}

export async function waitForCoreReady(profile: BundledCoreProfile, timeoutMs = 60000): Promise<void> {
  const startedAt = Date.now();
  const expectedGeneration = coreGeneration;
  const dataDir = getAmitiaDataDir();

  while (Date.now() - startedAt < timeoutMs) {
    if (coreGeneration !== expectedGeneration) {
      throw new Error("核心进程已被替换, 等待取消");
    }
    if (runningCoreProfile !== profile) {
      throw new Error(`Profile已变更: 期望${profile}, 当前${runningCoreProfile}`);
    }
    if (!isCoreRunning() && Date.now() - startedAt > 2000) {
      console.error(
        "[CoreManager] 核心进程已退出, 退出码:",
        coreProcess?.exitCode,
      );
      throw new Error("Amitia Core进程意外退出");
    }

    const expectedPid = coreChildPid;
    const endpoint = readCoreEndpoint(dataDir);
    const owned = expectedPid > 0 && isCoreEndpointOwnedByProcess(endpoint, expectedPid, coreSpawnedAt);
    if (endpoint && !owned) {
      removeCoreEndpoint(dataDir, endpoint.pid);
    }
    const ok = endpoint && owned && await httpHealthCheck(`${endpoint.httpBaseURL}/livez`, 2000);
    if (ok && endpoint) {
      setCoreEndpoint(endpoint);
      console.log(`[CoreManager] 健康检查通过, Profile: ${profile}`);
      return;
    }
    if ((!endpoint || !owned) && Date.now() - startedAt > 3000) {
      const legacy = legacyCoreEndpoint();
      if (await httpHealthCheck(`${legacy.httpBaseURL}/livez`, 1000)) {
        setCoreEndpoint(legacy);
        console.log(`[CoreManager] 兼容旧版本核心端点: ${legacy.httpBaseURL}`);
        return;
      }
    }

    await new Promise((resolve) => setTimeout(resolve, 500));
  }

  console.error("[CoreManager] 健康检查超时, 已等待", timeoutMs, "ms");
  throw new Error("Amitia Core启动超时");
}

export async function waitForCoreReadiness(timeoutMs = 60000): Promise<boolean> {
  const startedAt = Date.now();
  const dataDir = getAmitiaDataDir();

  while (Date.now() - startedAt < timeoutMs) {
    if (!isCoreRunning() && Date.now() - startedAt > 2000) {
      console.error("[CoreManager] 核心进程已退出, 跳过就绪等待");
      return false;
    }

    const expectedPid = coreChildPid;
    const endpoint = readCoreEndpoint(dataDir);
    const owned = expectedPid > 0 && isCoreEndpointOwnedByProcess(endpoint, expectedPid, coreSpawnedAt);
    if (endpoint && !owned) {
      removeCoreEndpoint(dataDir, endpoint.pid);
    }
    if (endpoint && owned && (await httpHealthCheck(`${endpoint.httpBaseURL}/readyz`, 2000))) {
      setCoreEndpoint(endpoint);
      console.log("[CoreManager] 就绪检查通过");
      return true;
    }
    if ((!endpoint || !owned) && Date.now() - startedAt > 3000) {
      const legacy = legacyCoreEndpoint();
      if (await httpHealthCheck(`${legacy.httpBaseURL}/readyz`, 1000)) {
        setCoreEndpoint(legacy);
        console.log(`[CoreManager] 兼容旧版本核心端点: ${legacy.httpBaseURL}`);
        return true;
      }
    }

    await new Promise((resolve) => setTimeout(resolve, 500));
  }

  console.error("[CoreManager] 就绪检查超时, 已等待", timeoutMs, "ms");
  return false;
}
