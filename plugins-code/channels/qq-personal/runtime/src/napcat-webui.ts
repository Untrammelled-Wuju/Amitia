import fs from "node:fs"
import os from "node:os"
import path from "node:path"

export interface NapCatWebUIInfo {
  token: string
  error: string
}

function expandHome(value: string, home: string): string {
  if (value === "~") return home
  if (value.startsWith("~/")) return path.join(home, value.slice(2))
  return path.resolve(value)
}

export function napcatWebUIConfigCandidates(
  env: NodeJS.ProcessEnv = process.env,
  home = os.homedir(),
): string[] {
  const candidates = [
    env.QQ_PERSONAL_NAPCAT_WEBUI_CONFIG,
    env.NAPCAT_WEBUI_CONFIG,
    "~/napcat-docker/data/config/webui.json",
    "~/napcat/config/webui.json",
    "~/napcat-docker/napcat/config/webui.json",
  ]
    .map((item) => String(item || "").trim())
    .filter(Boolean)
    .map((item) => expandHome(item, home))
  return [...new Set(candidates)]
}

export function readNapCatWebUIInfo(candidates = napcatWebUIConfigCandidates()): NapCatWebUIInfo {
  for (const candidate of candidates) {
    try {
      const parsed = JSON.parse(fs.readFileSync(candidate, "utf8")) as Record<string, unknown>
      const token = String(parsed.token || "").trim()
      if (token) return { token, error: "" }
    } catch {
      continue
    }
  }
  return {
    token: "",
    error: "未找到 NapCat WebUI Token，请确认 webui.json 路径或通过 QQ_PERSONAL_NAPCAT_WEBUI_CONFIG 指定。",
  }
}
