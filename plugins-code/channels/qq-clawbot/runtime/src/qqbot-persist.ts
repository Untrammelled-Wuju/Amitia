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
      const cfg = JSON.parse(fs.readFileSync(CONFIG_FILE, "utf-8"))
      if (cfg.appId && cfg.token) {
        return cfg
      }
    }
  } catch {}
  return null
}

export function saveQQBotConfig(config: { appId: string; token: string; sandbox: boolean }): void {
  try {
    const dir = path.dirname(CONFIG_FILE)
    if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true })
    fs.writeFileSync(CONFIG_FILE, JSON.stringify(config, null, 2), "utf-8")
  } catch {}
}

export function clearQQBotConfig(): void {
  try {
    if (fs.existsSync(CONFIG_FILE)) fs.unlinkSync(CONFIG_FILE)
  } catch {}
}
