// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import fs from "node:fs"
import path from "node:path"
import { fileURLToPath } from "node:url"

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const CONFIG_FILE = path.resolve(__dirname, "..", "data", "qqbot-config.json")

export function loadQQBotConfig(): { appId: string; token: string; sandbox: boolean } | null {
  try {
    if (fs.existsSync(CONFIG_FILE)) {
      const raw = fs.readFileSync(CONFIG_FILE, "utf-8")
      const cfg = JSON.parse(raw)
      if (cfg.appId && cfg.token) {
        console.log("[QQBot] 已从磁盘加载持久化凭证 appId=" + cfg.appId)
        return cfg
      }
    }
  } catch (err: any) {
    console.error("[QQBot] 加载持久化凭证失败:", err.message)
  }
  return null
}

export function saveQQBotConfig(config: { appId: string; token: string; sandbox: boolean }): void {
  try {
    const dir = path.dirname(CONFIG_FILE)
    if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true })
    fs.writeFileSync(CONFIG_FILE, JSON.stringify(config, null, 2), "utf-8")
    console.log("[QQBot] 凭证已持久化到磁盘")
  } catch (err: any) {
    console.error("[QQBot] 持久化凭证失败:", err.message)
  }
}
