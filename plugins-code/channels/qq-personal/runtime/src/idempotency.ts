import { createHash } from "node:crypto"

interface Entry {
  fingerprint: string
  result: Promise<unknown>
  timer: ReturnType<typeof setTimeout>
}

export class IdempotencyStore {
  private entries = new Map<string, Entry>()
  private ttlMs: number

  constructor(ttlMs = 10 * 60 * 1000) {
    this.ttlMs = ttlMs
  }

  fingerprint(value: unknown): string {
    return createHash("sha256").update(JSON.stringify(value)).digest("hex")
  }

  execute<T>(key: string, fingerprint: string, operation: () => Promise<T>): Promise<T> {
    if (!key) return operation()
    const existing = this.entries.get(key)
    if (existing) {
      if (existing.fingerprint !== fingerprint) {
        return Promise.reject(new Error("相同幂等键对应了不同的发送内容"))
      }
      return existing.result as Promise<T>
    }
    const timer = setTimeout(() => this.entries.delete(key), this.ttlMs)
    timer.unref?.()
    const result = operation().catch((error) => {
      clearTimeout(timer)
      this.entries.delete(key)
      throw error
    })
    this.entries.set(key, { fingerprint, result, timer })
    return result
  }
}
