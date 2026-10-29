import fs from "node:fs"
import type { InboundPayload } from "./inbound.js"
import { dataFilePath } from "./config.js"

export interface ChannelMessageRecord {
  id: string
  messageId: string
  conversationId: string
  peerId: string
  peerKind: "group" | "private"
  direction: "inbound" | "outbound"
  contentType: string
  text: string
  senderId: string
  senderName: string
  createdAt: string
}

export interface ChannelLogRecord {
  id: string
  level: "info" | "warn" | "error"
  event: string
  message: string
  createdAt: string
}

interface EventState {
  messages: ChannelMessageRecord[]
  logs: ChannelLogRecord[]
}

const MAX_MESSAGES = 2000
const MAX_LOGS = 1000

export class ChannelEventStore {
  private readonly filePath = dataFilePath("qq-personal-events.json")
  private state: EventState = this.load()

  recordInbound(payload: InboundPayload): ChannelMessageRecord {
    const record: ChannelMessageRecord = {
      id: payload.messageId || `${payload.conversationId}:${Date.now()}`,
      messageId: payload.messageId,
      conversationId: payload.conversationId,
      peerId: payload.peerId,
      peerKind: payload.peerKind,
      direction: "inbound",
      contentType: payload.contentType,
      text: payload.text,
      senderId: payload.senderId,
      senderName: payload.senderName || "",
      createdAt: new Date().toISOString(),
    }
    this.state.messages.push(record)
    this.trim()
    this.save()
    return record
  }

  recordOutbound(input: {
    conversationId: string
    peerId: string
    peerKind: "group" | "private"
    contentType: string
    text: string
    messageId?: string
  }): ChannelMessageRecord {
    const record: ChannelMessageRecord = {
      id: input.messageId || `${input.conversationId}:${Date.now()}`,
      messageId: input.messageId || "",
      conversationId: input.conversationId,
      peerId: input.peerId,
      peerKind: input.peerKind,
      direction: "outbound",
      contentType: input.contentType,
      text: input.text,
      senderId: "",
      senderName: "Amitia",
      createdAt: new Date().toISOString(),
    }
    this.state.messages.push(record)
    this.trim()
    this.save()
    return record
  }

  log(level: ChannelLogRecord["level"], event: string, message: string): void {
    this.state.logs.push({
      id: `${Date.now()}:${this.state.logs.length}`,
      level,
      event,
      message: message.trim().slice(0, 4000),
      createdAt: new Date().toISOString(),
    })
    if (this.state.logs.length > MAX_LOGS) this.state.logs.splice(0, this.state.logs.length - MAX_LOGS)
    this.save()
  }

  messages(conversationId = "", limit = 200, offset = 0): { messages: ChannelMessageRecord[]; total: number } {
    const filtered = conversationId
      ? this.state.messages.filter(item => item.conversationId === conversationId)
      : this.state.messages
    const start = Math.max(0, Math.trunc(offset))
    const size = Math.max(0, Math.min(500, Math.trunc(limit)))
    return {
      messages: filtered.slice(start, start + size).reverse(),
      total: filtered.length,
    }
  }

  logs(limit = 200, offset = 0): { items: ChannelLogRecord[]; total: number } {
    const start = Math.max(0, Math.trunc(offset))
    const size = Math.max(0, Math.min(500, Math.trunc(limit)))
    return {
      items: this.state.logs.slice(start, start + size).reverse(),
      total: this.state.logs.length,
    }
  }

  counts(): { inbound: number; outbound: number } {
    return this.state.messages.reduce((count, item) => {
      count[item.direction]++
      return count
    }, { inbound: 0, outbound: 0 })
  }

  private load(): EventState {
    try {
      const parsed = JSON.parse(fs.readFileSync(this.filePath, "utf8")) as Partial<EventState>
      return {
        messages: Array.isArray(parsed.messages) ? parsed.messages : [],
        logs: Array.isArray(parsed.logs) ? parsed.logs : [],
      }
    } catch {
      return { messages: [], logs: [] }
    }
  }

  private trim(): void {
    if (this.state.messages.length <= MAX_MESSAGES) return
    this.state.messages.splice(0, this.state.messages.length - MAX_MESSAGES)
  }

  private save(): void {
    const temporary = `${this.filePath}.tmp`
    fs.writeFileSync(temporary, `${JSON.stringify(this.state, null, 2)}\n`, { mode: 0o600 })
    fs.renameSync(temporary, this.filePath)
    fs.chmodSync(this.filePath, 0o600)
  }
}
