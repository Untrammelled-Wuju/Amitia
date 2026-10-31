import fs from "node:fs"
import { dataFilePath } from "./config.js"

type PeerKind = "group" | "private"
type PeerIndex = Record<string, PeerKind>

export class PeerStore {
  private filePath = dataFilePath("qq-personal-peers.json")
  private peers: PeerIndex = this.load()

  get(peerId: string): PeerKind | undefined {
    return this.peers[peerId]
  }

  list(limit = 200, offset = 0): Array<{ id: string; kind: PeerKind }> {
    const start = Math.max(0, Math.trunc(offset))
    const size = Math.max(0, Math.min(500, Math.trunc(limit)))
    return Object.entries(this.peers)
      .slice(start, start + size)
      .map(([id, kind]) => ({ id, kind }))
  }

  count(): number {
    return Object.keys(this.peers).length
  }

  set(peerId: string, kind: PeerKind): void {
    const id = peerId.trim()
    if (!id || this.peers[id] === kind) return
    this.peers[id] = kind
    const entries = Object.entries(this.peers)
    if (entries.length > 2000) {
      this.peers = Object.fromEntries(entries.slice(-2000))
    }
    this.save()
  }

  private load(): PeerIndex {
    try {
      const parsed = JSON.parse(fs.readFileSync(this.filePath, "utf8"))
      return parsed && typeof parsed === "object" ? parsed : {}
    } catch (error) {
  console.error("[original-error]", error);
return {}
    }
  }

  private save(): void {
    const temporary = `${this.filePath}.tmp`
    fs.writeFileSync(temporary, `${JSON.stringify(this.peers, null, 2)}\n`, { mode: 0o600 })
    fs.renameSync(temporary, this.filePath)
    fs.chmodSync(this.filePath, 0o600)
  }
}
