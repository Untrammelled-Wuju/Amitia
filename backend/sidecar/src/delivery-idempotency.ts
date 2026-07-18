import crypto from "node:crypto"
import fs from "node:fs"
import path from "node:path"

export type DeliveryExecutionResult = {
  duplicate: boolean
  clientId: string
}

interface LedgerEntry {
  deliveryKey: string
  clientId: string
  status: "sending" | "sent"
  updatedAt: number
}

interface LedgerData {
  entries: LedgerEntry[]
}

export class DeliveryIdempotencyStore {
  private filePath: string
  private ledger: Map<string, LedgerEntry> = new Map()
  private inflight: Map<string, Promise<DeliveryExecutionResult>> = new Map()

  constructor(filePath: string) {
    this.filePath = filePath
    this.load()
  }

  private resolveClientId(deliveryKey: string): string {
    const hash = crypto.createHash("sha256").update(deliveryKey).digest("hex")
    return `amitia:${hash.slice(0, 16)}`
  }

  private load(): void {
    try {
      if (!fs.existsSync(this.filePath)) {
        return
      }
      const raw = fs.readFileSync(this.filePath, "utf-8")
      if (raw.trim() === "") {
        return
      }
      const data = JSON.parse(raw) as LedgerData
      if (!data.entries || !Array.isArray(data.entries)) {
        this.backupCorrupt()
        return
      }
      for (const entry of data.entries) {
        if (entry.deliveryKey && entry.clientId && entry.status) {
          this.ledger.set(entry.deliveryKey, entry)
        }
      }
    } catch {
      this.backupCorrupt()
    }
  }

  private backupCorrupt(): void {
    try {
      const ts = Date.now()
      const corruptPath = this.filePath.replace(/\.json$/, `.corrupt.${ts}.json`)
      if (fs.existsSync(this.filePath)) {
        fs.renameSync(this.filePath, corruptPath)
      }
    } catch {
    }
    this.ledger = new Map()
  }

  private persist(): void {
    const entries: LedgerEntry[] = []
    const cutoff = Date.now() - 7 * 24 * 60 * 60 * 1000
    for (const entry of this.ledger.values()) {
      if (entry.status === "sent" && entry.updatedAt < cutoff) {
        continue
      }
      entries.push(entry)
    }
    if (entries.length > 5000) {
      entries.sort((a, b) => b.updatedAt - a.updatedAt)
      entries.splice(5000)
    }
    const data: LedgerData = { entries }
    const dir = path.dirname(this.filePath)
    if (!fs.existsSync(dir)) {
      fs.mkdirSync(dir, { recursive: true })
    }
    const tmpPath = this.filePath + ".tmp." + crypto.randomBytes(4).toString("hex")
    fs.writeFileSync(tmpPath, JSON.stringify(data), "utf-8")
    fs.renameSync(tmpPath, this.filePath)
  }

  async execute(
    deliveryKey: string,
    sender: (clientId: string) => Promise<void>
  ): Promise<DeliveryExecutionResult> {
    const clientId = this.resolveClientId(deliveryKey)

    const inflight = this.inflight.get(deliveryKey)
    if (inflight) {
      const result = await inflight
      return { duplicate: true, clientId }
    }

    const existing = this.ledger.get(deliveryKey)
    if (existing) {
      if (existing.status === "sent") {
        return { duplicate: true, clientId }
      }
      const promise = this.executeSender(clientId, deliveryKey, sender)
      this.inflight.set(deliveryKey, promise)
      try {
        return await promise
      } finally {
        this.inflight.delete(deliveryKey)
      }
    }

    const entry: LedgerEntry = {
      deliveryKey,
      clientId,
      status: "sending",
      updatedAt: Date.now(),
    }
    this.ledger.set(deliveryKey, entry)
    this.persist()

    const promise = this.executeSender(clientId, deliveryKey, sender)
    this.inflight.set(deliveryKey, promise)
    try {
      return await promise
    } finally {
      this.inflight.delete(deliveryKey)
    }
  }

  private async executeSender(
    clientId: string,
    deliveryKey: string,
    sender: (clientId: string) => Promise<void>
  ): Promise<DeliveryExecutionResult> {
    try {
      await sender(clientId)
      const entry = this.ledger.get(deliveryKey)
      if (entry) {
        entry.status = "sent"
        entry.updatedAt = Date.now()
        this.persist()
      }
      return { duplicate: false, clientId }
    } catch {
      return { duplicate: false, clientId }
    }
  }
}
