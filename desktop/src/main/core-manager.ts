import { app } from "electron";
import { spawn, execSync, ChildProcessWithoutNullStreams } from "child_process";
import http from "http";
import path from "path";
import fs from "fs";
import {
  getAmitiaDataDir,
  ensureAmitiaDataDir,
  getInstallDir,
} from "./path-manager";
import { validateCorePrerequisites } from "./core-prereq";
import type { CorePrerequisiteResult } from "./core-prereq";

export type { CorePrerequisiteResult };

let coreProcess: ChildProcessWithoutNullStreams | null = null;

export function getCorePath(): string {
  if (app.isPackaged) {
    return path.join(process.resourcesPath, "core", "AmitiaCore.exe");
  }
  return path.join(getInstallDir(), "resources", "core", "AmitiaCore.exe");
}

export function getCoreResourcesPath(): string {
  if (app.isPackaged) {
    return process.resourcesPath;
  }
  return path.join(getInstallDir(), "resources");
}

export function ensureDataAndConfig(): { ok: boolean; errors: string[] } {
  const dataDir = ensureAmitiaDataDir();
  ensureDefaultConfig(dataDir);
  ensureInitialSQL(dataDir);
  ensureCoreBinaries(dataDir);
  const validation = validateCorePrerequisites(dataDir, getCorePath());
  return { ok: validation.ok, errors: validation.missing };
}

function ensureDefaultConfig(dataDir: string): void {
  const destConfig = path.join(dataDir, "config", "config.yml");
  const resourcesPath = getCoreResourcesPath();
  const templatePath = path.join(
    resourcesPath,
    "config-template",
    "config.yaml",
  );
  console.log("[CoreManager] 配置模板路径:", templatePath);
  if (fs.existsSync(templatePath)) {
    fs.copyFileSync(templatePath, destConfig);
    console.log("[CoreManager] 配置文件已更新:", destConfig);
  } else {
    console.error("[CoreManager] 配置模板不存在:", templatePath);
  }
}

function ensureInitialSQL(dataDir: string): void {
  const destSQL = path.join(dataDir, "data", "sql.sql");
  if (fs.existsSync(destSQL)) {
    console.log("[CoreManager] sql.sql已存在, 跳过复制:", destSQL);
    return;
  }
  const resourcesPath = getCoreResourcesPath();
  const sourceSQL = path.join(resourcesPath, "data", "sql.sql");
  console.log("[CoreManager] sql.sql资源路径:", sourceSQL);
  if (fs.existsSync(sourceSQL)) {
    fs.copyFileSync(sourceSQL, destSQL);
    console.log("[CoreManager] sql.sql已复制到:", destSQL);
  } else {
    console.error("[CoreManager] sql.sql资源不存在:", sourceSQL);
  }
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

export function startCore(): void {
  if (coreProcess) {
    console.log("[CoreManager] 核心进程已在运行, 跳过");
    return;
  }

  const corePath = getCorePath();
  const dataDir = getAmitiaDataDir();

  console.log("[CoreManager] 核心路径:", corePath);
  console.log("[CoreManager] 数据目录:", dataDir);
  console.log("[CoreManager] 核心文件存在:", fs.existsSync(corePath));
  console.log(
    "[CoreManager] 核心文件大小:",
    fs.existsSync(corePath) ? fs.statSync(corePath).size : 0,
  );

  const prereq = validateCorePrerequisites(dataDir, corePath);
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

  coreProcess = spawn(corePath, [], {
    cwd: dataDir,
    env,
    windowsHide: true,
  });

  console.log("[CoreManager] 核心进程已启动, PID:", coreProcess.pid);

  coreProcess.stdout.on("data", (data) => {
    console.log(`[AmitiaCore] ${data.toString().trim()}`);
  });

  coreProcess.stderr.on("data", (data) => {
    console.error(`[AmitiaCore Error] ${data.toString().trim()}`);
  });

  coreProcess.on("exit", (code, signal) => {
    console.log(`[AmitiaCore] 进程退出, code=${code}, signal=${signal}`);
    coreProcess = null;
  });

  coreProcess.on("error", (error) => {
    console.error("[AmitiaCore] 启动失败:", error.message);
    coreProcess = null;
  });
}

export function stopCore(): void {
  if (coreProcess && !coreProcess.killed) {
    console.log("[CoreManager] 正在停止核心进程, PID:", coreProcess.pid);
    const pid = coreProcess.pid;
    if (process.platform === "win32" && pid) {
      try {
        execSync(`taskkill /PID ${pid} /T /F`, {
          windowsHide: true,
          stdio: "pipe",
        });
        console.log("[CoreManager] 进程树已终止");
        execSync(`taskkill /F /IM qdrant.exe`, {
          windowsHide: true,
          stdio: "pipe",
        });
      } catch (e) {
        console.error("[CoreManager] taskkill异常:", e);
      }
      try {
        execSync(`taskkill /F /IM surreal.exe`, {
          windowsHide: true,
          stdio: "pipe",
        });
      } catch (_) {}
    } else {
      coreProcess.kill("SIGTERM");
      try {
        execSync("pkill -9 qdrant", { stdio: "pipe" });
      } catch (_) {}
      try {
        execSync("pkill -9 surreal", { stdio: "pipe" });
      } catch (_) {}
    }
    coreProcess = null;
  }
}

export function isCoreRunning(): boolean {
  return (
    coreProcess !== null && !coreProcess.killed && coreProcess.exitCode === null
  );
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

export async function waitForCoreReady(timeoutMs = 60000): Promise<void> {
  const startedAt = Date.now();
  const healthUrl = "http://127.0.0.1:18899/api/health";

  while (Date.now() - startedAt < timeoutMs) {
    if (!isCoreRunning() && Date.now() - startedAt > 2000) {
      console.error(
        "[CoreManager] 核心进程已退出, 退出码:",
        coreProcess?.exitCode,
      );
      throw new Error("Amitia Core进程意外退出");
    }

    const ok = await httpHealthCheck(healthUrl, 2000);
    if (ok) {
      console.log("[CoreManager] 健康检查通过");
      return;
    }

    await new Promise((resolve) => setTimeout(resolve, 500));
  }

  console.error("[CoreManager] 健康检查超时, 已等待", timeoutMs, "ms");
  throw new Error("Amitia Core启动超时");
}
