import fs from "node:fs/promises"
import path from "node:path"

interface OwnedLink { target: string; source: string }

export async function linkMobileEnvironment(directory: string, guestRoot = "/"): Promise<void> {
  const root = path.resolve(directory)
  const prefix = path.join(root, "root")
  const receipt = path.join(root, "links.json")
  let links: OwnedLink[] = []
  try { links = JSON.parse(await fs.readFile(receipt, "utf8")) as OwnedLink[] } catch (error) {
    if ((error as NodeJS.ErrnoException).code !== "ENOENT") throw error
  }
  const visit = async (relative: string): Promise<void> => {
    const source = path.join(prefix, relative)
    const target = path.join(guestRoot, relative)
    let sourceInfo
    try { sourceInfo = await fs.lstat(source) } catch (error) {
      if ((error as NodeJS.ErrnoException).code === "ENOENT") return
      throw error
    }
    try {
      const targetInfo = await fs.lstat(target)
      if (sourceInfo.isDirectory() && targetInfo.isDirectory()) {
        for (const name of await fs.readdir(source)) await visit(path.join(relative, name))
      }
      return
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code !== "ENOENT") throw error
    }
    links.push({ target, source })
    await fs.writeFile(`${receipt}.tmp`, JSON.stringify(links), { mode: 0o600 })
    await fs.rename(`${receipt}.tmp`, receipt)
    await fs.symlink(source, target)
  }
  for (const relative of ["usr/bin", "usr/lib", "usr/share", "opt/QQ"]) await visit(relative)
}

export async function unlinkMobileEnvironment(directory: string): Promise<void> {
  const root = path.resolve(directory)
  const prefix = path.join(root, "root")
  const receipt = path.join(root, "links.json")
  let links: OwnedLink[]
  try { links = JSON.parse(await fs.readFile(receipt, "utf8")) as OwnedLink[] } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ENOENT") return
    throw error
  }
  if (!Array.isArray(links)) throw new Error("手机 QQ 安装归属记录无效")
  for (const link of [...links].reverse()) {
    if (typeof link.source !== "string" || typeof link.target !== "string" || !link.source.startsWith(prefix + path.sep)) throw new Error("手机 QQ 安装归属路径无效")
    try {
      if (await fs.readlink(link.target) === link.source) await fs.unlink(link.target)
    } catch (error) {
      if (!["ENOENT", "EINVAL"].includes((error as NodeJS.ErrnoException).code || "")) throw error
    }
  }
  await fs.rm(receipt, { force: true })
}
