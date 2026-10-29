import { createHash } from "node:crypto"
import fs from "node:fs"
import path from "node:path"
import { Readable } from "node:stream"
import { pipeline } from "node:stream/promises"

export const MOBILE_QQ_VERSION = "3.2.32_260812"
export const MOBILE_QQ_SHA256 = "8796ccfd66acc025ef18db37185532d40bc8c58921e17da6d28c295acbcf8f92"
export const MOBILE_QQ_URL = `https://qqdl.gtimg.cn/qqfile/QQNT/9.9.33/release/3f89efc5/QQ_${MOBILE_QQ_VERSION}_arm64_01.deb`

export async function downloadMobileQQ(directory: string, signal?: AbortSignal): Promise<string> {
  await fs.promises.mkdir(directory, { recursive: true, mode: 0o700 })
  const destination = path.join(directory, `QQ_${MOBILE_QQ_VERSION}_arm64.deb`)
  const temporary = `${destination}.download`
  await fs.promises.rm(temporary, { force: true })
  try {
    const timeout = AbortSignal.timeout(15 * 60 * 1000)
    const effectiveSignal = signal ? AbortSignal.any([signal, timeout]) : timeout
    const response = await fetch(MOBILE_QQ_URL, { signal: effectiveSignal, redirect: "error" })
    if (!response.ok || !response.body) throw new Error(`QQ 安装包下载失败：HTTP ${response.status}`)
    await pipeline(Readable.fromWeb(response.body as import("node:stream/web").ReadableStream), fs.createWriteStream(temporary, { mode: 0o600 }), { signal: effectiveSignal })
    const hash = createHash("sha256")
    for await (const chunk of fs.createReadStream(temporary)) hash.update(chunk)
    if (hash.digest("hex") !== MOBILE_QQ_SHA256) throw new Error("QQ 安装包 SHA-256 校验失败")
    await fs.promises.rename(temporary, destination)
    return destination
  } catch (error) {
    await fs.promises.rm(temporary, { force: true })
    throw error
  }
}
