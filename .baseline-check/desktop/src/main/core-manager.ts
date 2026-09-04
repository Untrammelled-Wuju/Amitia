import { app } from "electron";
import { spawn, execSync, ChildProcessWithoutNullStreams } from "child_process";
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

export type { CorePrerequisiteResult };

export type BundledCoreProfile = "local" | "device-agent";

let coreProcess: ChildProcessWithoutNullStreams | null = null;
let runningCoreProfile: BundledCoreProfile | null = null;
let coreGeneration = 0;

export function getCorePath(): string {
  if (!isDevMode()) {
    return path.join(process.resourcesPath, "core", "AmitiaCore.exe");
  }
  return path.join(getInstallDir(), "resources", "core", "AmitiaCore.exe");
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

const KNOWN_INSECURE_JWT_SECRETS = new Set([
  "IJ8ffa4-WAmfBfTFnmEdwdRx1k2kooXHgFQpYMVMUjs",
  "gIWcNHCKHdZWQyOanUhLvhLOVFgz1Z64G0xDYsUNWGA",
  "zTMPXMQGsKBp0WuYlEWHZNLaUOd2lPbFeRSu1fRNrBU",
]);

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

    const jwtPath = ["jwt", "secret"];
    const jwt = findYamlScalar(lines, jwtPath);
    const jwtSecret = jwt?.value.trim() ?? "";
    if (jwtSecret.length < 32 || KNOWN_INSECURE_JWT_SECRETS.has(jwtSecret)) {
      setYamlScalar(lines, jwtPath, randomBytes(48).toString("base64url"));
      changed = true;
      console.warn("[CoreManager] 检测到缺失/不安全的 JWT Secret，已完成本机随机轮换；现有登录会话需要重新登录");
    }

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
  const binaries = [
    {
      src: path.join(resourcesPath, "qdrant", "qdrant.zip"),
      dest: path.join(dataDir, "qdrant", "qdrant.zip"),
    },
    {
      src: path.join(resourcesPath, "surrealdb", "surreal.zip"),
      dest: path.join(dataDir, "surrealdb", "surreal.zip"),
    },
  ];
  for (const { src, dest } of binaries) {
    if (fs.existsSync(dest)) {
      console.log("[CoreManager] 二进制已存在, 跳过复制:", dest);
      continue;
    }
    if (fs.existsSync(src)) {
      fs.copyFileSync(src, dest);
      console.log("[CoreManager] 二进制已复制:", dest);
    } else {
      console.error("[CoreManager] 二进制资源不存在:", src);
    }
  }
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
  ensureHealthPolling();

  const corePath = getCorePath();
  const dataDir = getAmitiaDataDir();

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
  const env: NodeJS.ProcessEnv = {
    ...process.env,
    CONFIG_PATH: configPath,
    AMITIA_RUN_MODE: "desktop",
    AMITIA_DATA_DIR: dataDir,
  };

  console.log("[CoreManager] CONFIG_PATH:", configPath);
  console.log("[CoreManager] cwd:", dataDir);

  const currentGeneration = ++coreGeneration;
  coreProcess = spawn(corePath, [`--runtime-profile=${profile}`], {
    cwd: dataDir,
    env,
    windowsHide: true,
    detached: process.platform !== "win32",
  });
  runningCoreProfile = profile;

  console.log(`[CoreManager] 核心进程已启动, PID: ${coreProcess.pid}, generation: ${currentGeneration}`);

  coreProcess.stdout.on("data", (data) => {
    console.log(`[AmitiaCore] ${data.toString().trim()}`);
  });

  coreProcess.stderr.on("data", (data) => {
    console.error(`[AmitiaCore Error] ${data.toString().trim()}`);
  });

  coreProcess.on("exit", (code, signal) => {
    console.log(`[AmitiaCore] 进程退出, code=${code}, signal=${signal}, generation=${currentGeneration}`);
    if (currentGeneration === coreGeneration) {
      coreProcess = null;
      runningCoreProfile = null;
    }
  });

  coreProcess.on("error", (error) => {
    console.error("[AmitiaCore] 启动失败:", error.message);
    if (currentGeneration === coreGeneration) {
      coreProcess = null;
      runningCoreProfile = null;
    }
  });
}

export async function stopCore(): Promise<void> {
  if (coreProcess && !coreProcess.killed) {
    const currentGeneration = coreGeneration;
    console.log("[CoreManager] 正在停止核心进程, PID:", coreProcess.pid);
    const pid = coreProcess.pid ?? 0;

    try {
      const success = await gracefulShutdown(pid);
      if (!success) {
        console.warn("[CoreManager] 优雅关闭超时，回退到强制终止");
        forceKillProcessTree(pid);
      }
    } catch (err) {
      console.error("[CoreManager] 优雅关闭异常，强制终止:", err);
      forceKillProcessTree(pid);
    }
    await waitForChildExit(coreProcess, 5000);
    if (currentGeneration === coreGeneration) {
      coreProcess = null;
      runningCoreProfile = null;
    }
    return;
  }
  coreProcess = null;
  runningCoreProfile = null;
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
    const healthUrl = "http://127.0.0.1:18899/livez";
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

function forceKillProcessTree(pid: number): void {
  if (!pid) return;
  if (process.platform === "win32") {
    try {
      execSync(`taskkill /PID ${pid} /T /F`, {
        windowsHide: true,
        stdio: "pipe",
      });
      console.log("[CoreManager] 进程树已强制终止");
    } catch (e) {
      console.error("[CoreManager] taskkill异常:", e);
    }
  } else {
    try {
      process.kill(-pid, "SIGKILL");
      console.log("[CoreManager] 进程组已强制终止");
    } catch (e) {
      console.error("[CoreManager] kill进程组异常:", e);
      try {
        process.kill(pid, "SIGKILL");
      } catch (_) {}
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

async function gracefulShutdown(pid: number): Promise<boolean> {
  const healthUrl = "http://127.0.0.1:18899/livez";
  const isAlive = await httpHealthCheck(healthUrl, 2000);
  if (!isAlive) {
    console.log("[CoreManager] 核心服务不可达，跳过优雅关闭");
    return false;
  }

  console.log("[CoreManager] 尝试优雅关闭核心服务...");
  const shutdownOk = await httpShutdown("http://127.0.0.1:18899/api/local/admin/shutdown", 3000, getLocalAdminHeaders());
  if (!shutdownOk) {
    console.warn("[CoreManager] 优雅关闭请求失败");
    return false;
  }

  const deadline = Date.now() + 8000;
  while (Date.now() < deadline) {
    if (coreProcess === null || coreProcess.killed || coreProcess.exitCode !== null) {
      console.log("[CoreManager] 核心进程已优雅退出");
      return true;
    }
    const stillAlive = await httpHealthCheck(healthUrl, 1000);
    if (!stillAlive) {
      console.log("[CoreManager] 核心服务已停止响应，等待进程退出");
      await new Promise((r) => setTimeout(r, 500));
      if (coreProcess === null || coreProcess.killed || coreProcess.exitCode !== null) {
        console.log("[CoreManager] 核心进程已优雅退出");
        return true;
      }
      return false;
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
  const healthUrl = "http://127.0.0.1:18899/livez";

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

    const ok = await httpHealthCheck(healthUrl, 2000);
    if (ok) {
      console.log(`[CoreManager] 健康检查通过, Profile: ${profile}`);
      return;
    }

    await new Promise((resolve) => setTimeout(resolve, 500));
  }

  console.error("[CoreManager] 健康检查超时, 已等待", timeoutMs, "ms");
  throw new Error("Amitia Core启动超时");
}
