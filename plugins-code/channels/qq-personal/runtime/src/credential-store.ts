import crypto from "node:crypto"
import fs from "node:fs"
import path from "node:path"

const AAD = Buffer.from("amitia:qq-personal:webui-credential:v1", "utf8")
const KEY_BYTES = 32
const IV_BYTES = 12

interface StoredRecord {
  version: 1
  iv: string
  tag: string
  data: string
  updatedAt: string
}

function readKeyFile(file: string): Buffer | null {
  try {
    const raw = fs.readFileSync(file)
    if (raw.length === KEY_BYTES) return raw
    const hex = raw.toString("utf8").trim()
    if (/^[0-9a-f]{64}$/i.test(hex)) return Buffer.from(hex, "hex")
    return null
  } catch {
    return null
  }
}

function deriveLeaseKey(lease: string): Buffer {
  return crypto.createHash("sha256").update(`amitia:qq-personal:${lease}`).digest()
}

export class CredentialStore {
  private readonly secretPath: string
  private readonly keyPath: string
  private key: Buffer | null = null

  constructor(directory: string) {
    this.secretPath = path.join(directory, "webui-credential.json")
    this.keyPath = path.join(directory, "webui-credential.key")
  }

  private resolvedKey(): Buffer {
    if (this.key) return this.key
    const lease = String(process.env.AMITIA_SECRET_LEASE || "").trim()
    if (lease) {
      this.key = deriveLeaseKey(lease)
      return this.key
    }
    const existing = readKeyFile(this.keyPath)
    if (existing) {
      this.key = existing
      return existing
    }
    const created = crypto.randomBytes(KEY_BYTES)
    fs.mkdirSync(path.dirname(this.keyPath), { recursive: true, mode: 0o700 })
    fs.writeFileSync(this.keyPath, created, { mode: 0o600 })
    fs.chmodSync(this.keyPath, 0o600)
    this.key = created
    return created
  }

  read(): string {
    let parsed: StoredRecord
    try {
      parsed = JSON.parse(fs.readFileSync(this.secretPath, "utf8")) as StoredRecord
    } catch {
      return ""
    }
    if (!parsed || parsed.version !== 1 || !parsed.data || !parsed.iv || !parsed.tag) return ""
    try {
      const decipher = crypto.createDecipheriv("aes-256-gcm", this.resolvedKey(), Buffer.from(parsed.iv, "base64"))
      decipher.setAAD(AAD)
      decipher.setAuthTag(Buffer.from(parsed.tag, "base64"))
      const plain = Buffer.concat([decipher.update(Buffer.from(parsed.data, "base64")), decipher.final()])
      return plain.toString("utf8")
    } catch {
      return ""
    }
  }

  write(password: string): void {
    const value = String(password || "")
    if (!value) return
    const iv = crypto.randomBytes(IV_BYTES)
    const cipher = crypto.createCipheriv("aes-256-gcm", this.resolvedKey(), iv)
    cipher.setAAD(AAD)
    const encrypted = Buffer.concat([cipher.update(value, "utf8"), cipher.final()])
    const record: StoredRecord = {
      version: 1,
      iv: iv.toString("base64"),
      tag: cipher.getAuthTag().toString("base64"),
      data: encrypted.toString("base64"),
      updatedAt: new Date().toISOString(),
    }
    fs.mkdirSync(path.dirname(this.secretPath), { recursive: true, mode: 0o700 })
    const temporary = `${this.secretPath}.tmp`
    fs.writeFileSync(temporary, `${JSON.stringify(record)}\n`, { mode: 0o600 })
    fs.renameSync(temporary, this.secretPath)
    fs.chmodSync(this.secretPath, 0o600)
  }

  clear(): void {
    try {
      fs.rmSync(this.secretPath, { force: true })
    } catch {
      return
    }
  }
}
