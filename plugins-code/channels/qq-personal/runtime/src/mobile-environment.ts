import { execFile, spawn } from "node:child_process"
import { randomUUID } from "node:crypto"
import fs from "node:fs/promises"
import path from "node:path"
import { downloadMobileQQ } from "./mobile-qq-package.js"

export type InstallProgress = (phase: string, detail: string) => void
export type RunCommand = (command: string, args: string[]) => Promise<void>
export type ExtractDebPackage = (archive: string, destination: string) => Promise<void>

export const runInstallCommand: RunCommand = (command, args) => new Promise((resolve, reject) => {
  execFile(command, args, {
    timeout: 20 * 60 * 1000,
    maxBuffer: 4 * 1024 * 1024,
    env: { ...process.env, DEBIAN_FRONTEND: "noninteractive" },
  }, (error, stdout, stderr) => {
    if (error) reject(new Error(`${path.basename(command)} 执行失败：${String(stderr || stdout || error.message).slice(-4000)}`))
    else resolve()
  })
})

function collectProcessError(stream: NodeJS.ReadableStream): () => string {
  let value = ""
  stream.on("data", chunk => {
    value = `${value}${String(chunk)}`.slice(-4000)
  })
  return () => value
}

export function runWithReceipt(command: string, args: string[], receipt: string, stdoutFile = ""): Promise<void> {
  const script = stdoutFile
    ? 'output="$1"; status="$2"; shift 2; "$@" > "$output" && printf 0 > "$status"'
    : 'status="$1"; shift; "$@" && printf 0 > "$status"'
  const values = stdoutFile
    ? ["-c", script, "amitia-extract", stdoutFile, receipt, command, ...args]
    : ["-c", script, "amitia-extract", receipt, command, ...args]
  const child = spawn("/bin/sh", values, { stdio: ["ignore", "ignore", "pipe"] })
  const errors = child.stderr ? collectProcessError(child.stderr) : () => ""
  const deadline = Date.now() + 20 * 60 * 1000
  return new Promise((resolve, reject) => {
    const probe = async () => {
      try {
        const value = await fs.readFile(receipt, "utf8")
        if (value.trim() === "0") {
          resolve()
          return
        }
      } catch (error) {
        if ((error as NodeJS.ErrnoException).code !== "ENOENT") {
          reject(error)
          return
        }
      }
      if (Date.now() >= deadline) {
        child.kill("SIGKILL")
        reject(new Error(`${path.basename(command)} 执行超时`))
        return
      }
      if (child.pid) {
        try {
          process.kill(child.pid, 0)
        } catch (error) {
          if ((error as NodeJS.ErrnoException).code === "ESRCH") {
            reject(new Error(`${path.basename(command)} 执行失败：${errors() || "进程未生成成功回执"}`))
            return
          }
        }
      }
      setTimeout(() => { void probe() }, 100)
    }
    void probe()
  })
}

export const extractDebPackage: ExtractDebPackage = async (archive, destination) => {
  const stagedTar = path.join(path.dirname(destination), `.extract-${randomUUID()}.tar`)
  const sourceReceipt = path.join(path.dirname(destination), `.extract-${randomUUID()}.status`)
  const sinkReceipt = path.join(path.dirname(destination), `.extract-${randomUUID()}.status`)
  try {
    await runWithReceipt("/usr/bin/dpkg-deb", ["--fsys-tarfile", archive], sourceReceipt, stagedTar)
    await runWithReceipt("/bin/tar", ["-xf", stagedTar, "--no-same-owner", "--no-same-permissions", "-C", destination], sinkReceipt)
  } finally {
    await fs.rm(stagedTar, { force: true })
    await fs.rm(sourceReceipt, { force: true })
    await fs.rm(sinkReceipt, { force: true })
  }
}

export function mobileAptOptions(root: string): string[] {
  return [
    "-c", path.join(root, "apt", "config"),
    "-o", "Debug::NoLocking=1",
    "-o", `Dir::State::lists=${path.join(root, "apt", "lists")}`,
    "-o", `Dir::Cache::archives=${path.join(root, "apt", "archives")}`,
    "-o", "APT::Get::List-Cleanup=0",
    "-o", "Acquire::http::Timeout=30",
    "-o", "Acquire::https::Timeout=30",
    "-o", "Acquire::Retries=2",
  ]
}

export const mobileRuntimePackages = [
  "xvfb",
  "xkb-data",
  "x11-xkb-utils",
  "fluxbox",
  "x11vnc",
  "novnc",
  "websockify",
  "fonts-noto-cjk",
  "libatomic1",
  "libgbm1",
  "libnss3",
  "libasound2t64",
  "libgtk-3-0t64",
  "libxshmfence1",
  "libatspi2.0-0t64",
  "libnotify4",
  "libsecret-1-0",
] as const

export async function prepareMobileEnvironment(
  directory: string,
  progress: InstallProgress,
  run: RunCommand = runInstallCommand,
  download: typeof downloadMobileQQ = downloadMobileQQ,
  extract: ExtractDebPackage = extractDebPackage,
  platform = process.platform,
  arch = process.arch,
): Promise<string> {
  if (platform !== "linux" || arch !== "arm64") throw new Error("手机运行环境需要 Linux ARM64")
  const root = path.resolve(directory)
  const prefix = path.join(root, "root")
  await fs.mkdir(path.join(root, "apt", "lists", "partial"), { recursive: true, mode: 0o700 })
  await fs.mkdir(path.join(root, "apt", "archives", "partial"), { recursive: true, mode: 0o700 })
  await fs.mkdir(prefix, { recursive: true, mode: 0o700 })
  await fs.writeFile(path.join(root, "apt", "config"), [
    "#clear APT::Update::Post-Invoke;",
    "#clear APT::Update::Post-Invoke-Success;",
    "#clear DPkg::Post-Invoke;",
    "",
  ].join("\n"), { mode: 0o600 })
  const options = mobileAptOptions(root)
  progress("dependencies", "更新图形运行依赖索引")
  await run("/usr/bin/apt-get", [...options, "update"])
  progress("dependencies", "下载图形运行依赖")
  await run("/usr/bin/apt-get", [
    ...options,
    "--download-only",
    "--assume-yes",
    "--no-install-recommends",
    "install",
    ...mobileRuntimePackages,
  ])
  const archives = path.join(root, "apt", "archives")
  const packages = (await fs.readdir(archives)).filter(file => file.endsWith(".deb")).sort()
  for (const [index, file] of packages.entries()) {
    progress("dependencies", `解包图形运行依赖 ${index + 1}/${packages.length}：${file}`)
    await extract(path.join(archives, file), prefix)
  }
  progress("qq", "下载并校验 QQ ARM64 安装包")
  const qq = await download(path.join(root, "downloads"))
  progress("qq", "解包 QQ")
  await extract(qq, prefix)
  await Promise.all([
    fs.access(path.join(prefix, "opt", "QQ", "qq"), fs.constants.X_OK),
    fs.access(path.join(prefix, "usr", "bin", "Xvfb"), fs.constants.X_OK),
    fs.access(path.join(prefix, "usr", "bin", "x11vnc"), fs.constants.X_OK),
    fs.access(path.join(prefix, "usr", "lib", "aarch64-linux-gnu", "libatomic.so.1"), fs.constants.R_OK),
  ])
  progress("prepared", "QQ 与图形运行文件已准备")
  return prefix
}
