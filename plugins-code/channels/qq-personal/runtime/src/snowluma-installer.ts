import { createHash, randomUUID } from "node:crypto"
import { once } from "node:events"
import fs from "node:fs"
import path from "node:path"
import { unzipSync } from "fflate"
import * as tar from "tar"

export const SNOWLUMA_VERSION = "2.2.0"
export const SNOWLUMA_RELEASE_BASE_URL = "https://api.github.com/repos/SnowLuma/SnowLuma/releases/tags"

export type SnowLumaInstallPhase =
  | "idle"
  | "checking"
  | "downloading"
  | "verifying"
  | "extracting"
  | "validating"
  | "migrating"
  | "starting"
  | "completed"
  | "failed"

export interface SnowLumaInstallStatus {
  jobId: string
  phase: SnowLumaInstallPhase
  message: string
  percent: number
  downloadedBytes: number
  totalBytes: number
  version: string
  platform: string
  arch: string
  assetName: string
  installedVersion: string
  installed: boolean
  running: boolean
  error: string
  startedAt: string
  completedAt: string
}

export interface SnowLumaReleaseAsset {
  name: string
  size: number
  digest: string
  browser_download_url: string
}

interface SnowLumaRelease {
  tag_name: string
  assets: SnowLumaReleaseAsset[]
}

interface SnowLumaInstallPointer {
  schemaVersion: number
  version: string
  platform: string
  arch: string
  installDir: string
  archiveSha256: string
  installedAt: string
  previousVersion: string
  previousInstallDir: string
}

interface InstallOfficialOptions {
  allowedRoot: string
  legacyRoot?: string
  version?: string
  platform?: NodeJS.Platform
  arch?: NodeJS.Architecture
  nodeVersion?: string
  fetchImpl?: typeof fetch
  onProgress?: (status: SnowLumaInstallStatus) => void
}

const RELEASE_ASSET_PATTERNS: Record<string, RegExp> = {
  "win32-x64": /^SnowLuma-v.+-win-x64-lite\.zip$/,
  "linux-x64": /^SnowLuma-v.+-linux-x64-lite\.tar\.gz$/,
  "linux-arm64": /^SnowLuma-v.+-linux-arm64-lite\.tar\.gz$/,
}

function parseVersion(value: string): [number, number, number] {
  const match = String(value || "").replace(/^v/, "").match(/^(\d+)\.(\d+)\.(\d+)/)
  return match ? [Number(match[1]), Number(match[2]), Number(match[3])] : [0, 0, 0]
}

export function isSupportedProjectNode(version = process.versions.node): boolean {
  const [major, minor, patch] = parseVersion(version)
  if (major === 24) return minor > 22 || minor === 22 && patch >= 1
  return major >= 26 && (major > 26 || minor > 12 || minor === 12 && patch >= 1)
}

export function assertSupportedProjectNode(version = process.versions.node): void {
  if (!isSupportedProjectNode(version)) {
    throw new Error(`项目 Node ${version} 不满足 SnowLuma ${SNOWLUMA_VERSION} 的最低版本要求`)
  }
}

export function snowLumaInstallRoot(allowedRoot: string): string {
  return path.join(path.resolve(allowedRoot), "snowluma")
}

export function readInstallPointer(allowedRoot: string): SnowLumaInstallPointer | null {
  const file = path.join(snowLumaInstallRoot(allowedRoot), "install.json")
  try {
    const parsed = JSON.parse(fs.readFileSync(file, "utf8")) as SnowLumaInstallPointer
    if (!parsed || parsed.schemaVersion !== 1 || !path.isAbsolute(parsed.installDir)) return null
    return parsed
  } catch {
    return null
  }
}

export function currentSnowLumaInstallDir(allowedRoot: string): string {
  const pointer = readInstallPointer(allowedRoot)
  if (!pointer) return ""
  const root = snowLumaInstallRoot(allowedRoot)
  return isInside(root, pointer.installDir) ? pointer.installDir : ""
}

export function selectReleaseAsset(release: Pick<SnowLumaRelease, "assets">, platform = process.platform, arch: NodeJS.Architecture = process.arch): SnowLumaReleaseAsset {
  const key = `${platform}-${arch}`
  const pattern = RELEASE_ASSET_PATTERNS[key]
  if (!pattern) throw new Error(`SnowLuma ${SNOWLUMA_VERSION} 不支持当前平台 ${key}`)
  const asset = (release.assets || []).find((item) => pattern.test(item.name))
  if (!asset) throw new Error(`SnowLuma ${SNOWLUMA_VERSION} 发行包缺少 ${key} Lite 资产`)
  if (!/^sha256:[a-f0-9]{64}$/i.test(asset.digest || "")) throw new Error("SnowLuma 发行资产缺少 SHA-256 摘要")
  if (!Number.isSafeInteger(asset.size) || asset.size <= 0) throw new Error("SnowLuma 发行资产大小无效")
  return asset
}

function extractZipArchive(archive: string, destination: string): void {
  const root = path.resolve(destination)
  const files = unzipSync(new Uint8Array(fs.readFileSync(archive)))
  for (const [entryName, content] of Object.entries(files)) {
    const normalized = entryName.replaceAll("\\", "/")
    if (!normalized || normalized.startsWith("/") || normalized.split("/").includes("..")) {
      throw new Error(`SnowLuma ZIP 包含无效路径：${entryName}`)
    }
    const target = path.resolve(root, normalized)
    if (!isInside(root, target)) throw new Error(`SnowLuma ZIP 路径越界：${entryName}`)
    if (normalized.endsWith("/")) {
      fs.mkdirSync(target, { recursive: true, mode: 0o700 })
      continue
    }
    fs.mkdirSync(path.dirname(target), { recursive: true, mode: 0o700 })
    fs.writeFileSync(target, content, { mode: 0o600 })
  }
}

function isInside(root: string, candidate: string): boolean {
  const relative = path.relative(path.resolve(root), path.resolve(candidate))
  return relative !== ".." && !relative.startsWith(`..${path.sep}`) && !path.isAbsolute(relative)
}

function writeJsonAtomic(file: string, value: unknown): void {
  fs.mkdirSync(path.dirname(file), { recursive: true, mode: 0o700 })
  const temporary = `${file}.${randomUUID()}.tmp`
  fs.writeFileSync(temporary, `${JSON.stringify(value, null, 2)}\n`, { mode: 0o600 })
  fs.renameSync(temporary, file)
}

function statusBase(version: string, platform: NodeJS.Platform, arch: string, installedVersion: string): SnowLumaInstallStatus {
  return {
    jobId: "",
    phase: "idle",
    message: "",
    percent: 0,
    downloadedBytes: 0,
    totalBytes: 0,
    version,
    platform,
    arch,
    assetName: "",
    installedVersion,
    installed: Boolean(installedVersion),
    running: false,
    error: "",
    startedAt: "",
    completedAt: "",
  }
}

export function currentInstallStatus(allowedRoot: string, version = SNOWLUMA_VERSION, platform = process.platform, arch = process.arch): SnowLumaInstallStatus {
  const pointer = readInstallPointer(allowedRoot)
  return statusBase(version, platform, arch, pointer?.version || "")
}

async function fetchRelease(fetchImpl: typeof fetch, version: string): Promise<SnowLumaRelease> {
  const response = await fetchImpl(`${SNOWLUMA_RELEASE_BASE_URL}/v${version}`, {
    headers: {
      Accept: "application/vnd.github+json",
      "User-Agent": "Amitia-QQ-Personal-Plugin",
      "X-GitHub-Api-Version": "2022-11-28",
    },
    signal: AbortSignal.timeout(30_000),
  })
  if (!response.ok) throw new Error(`读取 SnowLuma 发行信息失败：HTTP ${response.status}`)
  return await response.json() as SnowLumaRelease
}

async function downloadAsset(asset: SnowLumaReleaseAsset, destination: string, fetchImpl: typeof fetch, progress: (downloaded: number, total: number) => void): Promise<string> {
  const response = await fetchImpl(asset.browser_download_url, {
    headers: { "User-Agent": "Amitia-QQ-Personal-Plugin" },
    redirect: "follow",
  })
  if (!response.ok || !response.body) throw new Error(`下载 SnowLuma 失败：HTTP ${response.status}`)
  fs.mkdirSync(path.dirname(destination), { recursive: true, mode: 0o700 })
  const hash = createHash("sha256")
  let downloaded = 0
  let lastEmit = 0
  const output = fs.createWriteStream(destination, { mode: 0o600 })
  const finished = new Promise<void>((resolve, reject) => {
    output.once("finish", resolve)
    output.once("error", reject)
  })
  const reader = response.body.getReader()
  try {
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      const chunk = Buffer.from(value)
      hash.update(chunk)
      downloaded += chunk.length
      if (!output.write(chunk)) await once(output, "drain")
      const now = Date.now()
      if (now - lastEmit >= 250) {
        lastEmit = now
        progress(downloaded, asset.size)
      }
    }
    progress(downloaded, asset.size)
    output.end()
    await finished
  } catch (error) {
    await reader.cancel().catch(() => undefined)
    output.destroy()
    await finished.catch(() => undefined)
    throw error
  }
  return hash.digest("hex")
}

function findPackageRoot(staging: string): string {
  const direct = path.join(staging, "index.mjs")
  if (fs.existsSync(direct)) return staging
  const children = fs.readdirSync(staging, { withFileTypes: true }).filter((entry) => entry.isDirectory())
  for (const child of children) {
    const candidate = path.join(staging, child.name)
    if (fs.existsSync(path.join(candidate, "index.mjs"))) return candidate
  }
  throw new Error("SnowLuma 发行包中缺少 index.mjs")
}

function mergeDirectory(source: string, destination: string): void {
  if (!fs.existsSync(source)) return
  fs.mkdirSync(destination, { recursive: true, mode: 0o700 })
  for (const entry of fs.readdirSync(source, { withFileTypes: true })) {
    const from = path.join(source, entry.name)
    const to = path.join(destination, entry.name)
    if (entry.isDirectory()) mergeDirectory(from, to)
    else if (!fs.existsSync(to)) fs.copyFileSync(from, to)
  }
}

function ensureSharedDirectories(packageRoot: string, sharedRoot: string, legacyRoot: string): void {
  for (const name of ["config", "data", "logs"]) {
    const shared = path.join(sharedRoot, name)
    fs.mkdirSync(shared, { recursive: true, mode: 0o700 })
    if (legacyRoot && path.resolve(legacyRoot) !== path.resolve(packageRoot)) {
      mergeDirectory(path.join(legacyRoot, name), shared)
    }
    const target = path.join(packageRoot, name)
    if (fs.existsSync(target)) mergeDirectory(target, shared)
    fs.rmSync(target, { recursive: true, force: true })
    fs.symlinkSync(shared, target, process.platform === "win32" ? "junction" : "dir")
  }
}

export class SnowLumaInstaller {
  private status: SnowLumaInstallStatus
  private running = false
  private readonly allowedRoot: string

  constructor(allowedRoot: string) {
    this.allowedRoot = path.resolve(allowedRoot)
    this.status = currentInstallStatus(this.allowedRoot)
  }

  installState(): SnowLumaInstallStatus {
    const current = currentInstallStatus(this.allowedRoot)
    return {
      ...this.status,
      installedVersion: current.installedVersion,
      installed: current.installed,
    }
  }

  installedRoot(): string {
    return currentSnowLumaInstallDir(this.allowedRoot)
  }

  rollback(): string {
    const pointer = readInstallPointer(this.allowedRoot)
    if (!pointer?.previousInstallDir || !pointer.previousVersion) return ""
    const root = snowLumaInstallRoot(this.allowedRoot)
    if (!isInside(root, pointer.previousInstallDir) || !fs.existsSync(path.join(pointer.previousInstallDir, "index.mjs"))) return ""
    writeJsonAtomic(path.join(root, "install.json"), {
      ...pointer,
      version: pointer.previousVersion,
      installDir: pointer.previousInstallDir,
      installedAt: new Date().toISOString(),
      previousVersion: "",
      previousInstallDir: "",
    } satisfies SnowLumaInstallPointer)
    return pointer.previousInstallDir
  }

  async install(legacyRoot = "", options: Omit<InstallOfficialOptions, "allowedRoot" | "legacyRoot"> = {}): Promise<SnowLumaInstallStatus> {
    if (this.running) return this.installState()
    this.running = true
    const version = options.version || SNOWLUMA_VERSION
    const platform = options.platform || process.platform
    const arch = options.arch || process.arch
    const jobId = randomUUID()
    const startedAt = new Date().toISOString()
    this.status = {
      ...statusBase(version, platform, arch, this.installedRoot() ? readInstallPointer(this.allowedRoot)?.version || "" : ""),
      jobId,
      phase: "checking",
      message: "正在获取 SnowLuma 发行信息",
      startedAt,
      running: true,
    }
    options.onProgress?.(this.installState())
    try {
      assertSupportedProjectNode(options.nodeVersion)
      const fetchImpl = options.fetchImpl || fetch
      const release = await fetchRelease(fetchImpl, version)
      const asset = selectReleaseAsset(release, platform, arch)
      this.status = { ...this.status, assetName: asset.name, totalBytes: asset.size, phase: "downloading", message: "正在下载 SnowLuma" }
      options.onProgress?.(this.installState())
      const root = snowLumaInstallRoot(this.allowedRoot)
      const downloads = path.join(root, "downloads")
      const staging = path.join(root, ".staging", jobId)
      const archive = path.join(downloads, asset.name)
      const digest = await downloadAsset(asset, archive, fetchImpl, (downloaded, total) => {
        this.status = {
          ...this.status,
          downloadedBytes: downloaded,
          totalBytes: total,
          percent: total > 0 ? Math.min(89, Math.round(downloaded / total * 89)) : this.status.percent,
        }
        options.onProgress?.(this.installState())
      })
      const expected = asset.digest.replace(/^sha256:/i, "").toLowerCase()
      if (digest !== expected) throw new Error("SnowLuma 安装包 SHA-256 校验失败")
      this.status = { ...this.status, phase: "verifying", percent: 90, message: "安装包校验通过" }
      options.onProgress?.(this.installState())
      fs.rmSync(staging, { recursive: true, force: true })
      fs.mkdirSync(staging, { recursive: true, mode: 0o700 })
      this.status = { ...this.status, phase: "extracting", percent: 92, message: "正在解压 SnowLuma" }
      options.onProgress?.(this.installState())
      if (asset.name.endsWith(".zip")) extractZipArchive(archive, staging)
      else await tar.x({ file: archive, cwd: staging, strict: true, preservePaths: false })
      const packageRoot = findPackageRoot(staging)
      this.status = { ...this.status, phase: "validating", percent: 95, message: "正在校验 SnowLuma 文件" }
      options.onProgress?.(this.installState())
      const versionsRoot = path.join(root, "versions")
      const installName = `v${version}-${platform}-${arch}-${jobId.slice(0, 8)}`
      const installDir = path.join(versionsRoot, installName)
      fs.mkdirSync(versionsRoot, { recursive: true, mode: 0o700 })
      fs.renameSync(packageRoot, installDir)
      const pointerBefore = readInstallPointer(this.allowedRoot)
      this.status = { ...this.status, phase: "migrating", percent: 97, message: "正在迁移配置与账号数据" }
      options.onProgress?.(this.installState())
      ensureSharedDirectories(installDir, path.join(root, "shared"), legacyRoot)
      writeJsonAtomic(path.join(root, "install.json"), {
        schemaVersion: 1,
        version,
        platform,
        arch,
        installDir,
        archiveSha256: digest,
        installedAt: new Date().toISOString(),
        previousVersion: pointerBefore?.version || "",
        previousInstallDir: pointerBefore?.installDir || "",
      } satisfies SnowLumaInstallPointer)
      fs.rmSync(staging, { recursive: true, force: true })
      this.status = { ...this.status, phase: "completed", percent: 100, message: "SnowLuma 安装完成", installedVersion: version, installed: true, running: false, completedAt: new Date().toISOString(), error: "" }
      options.onProgress?.(this.installState())
      return this.installState()
    } catch (error) {
      this.status = {
        ...this.status,
        phase: "failed",
        message: "SnowLuma 安装失败",
        running: false,
        error: error instanceof Error ? error.message : String(error),
        completedAt: new Date().toISOString(),
      }
      options.onProgress?.(this.installState())
      return this.installState()
    } finally {
      this.running = false
    }
  }
}
