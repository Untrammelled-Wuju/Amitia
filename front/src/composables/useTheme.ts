// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, watch } from "vue"
import { apiClient } from "./useApi"

export type ThemePreset = "system" | "dark" | "light"

export interface ThemeState {
  preset: ThemePreset
  accentColor: string
}

const STORAGE_KEY = "ai-companion-theme"
const VALID_PRESETS: ThemePreset[] = ["system", "light", "dark"]

function normalizePreset(preset: unknown): ThemePreset {
  return VALID_PRESETS.includes(preset as ThemePreset) ? preset as ThemePreset : "system"
}

const state = ref<ThemeState>({
  preset: normalizePreset(localStorage.getItem(STORAGE_KEY)),
  accentColor: "",
})

const resolvedMode = ref<"light" | "dark">("light")
const themeLoaded = ref(false)
const preferredLight = ref<ThemePreset>("light")

// 立即应用已保存的主题，避免刷新闪烁
applyTheme(state.value.preset)

export const THEME_PRESETS: { id: ThemePreset; name: string; description: string }[] = [
  { id: "system", name: "跟随系统", description: "自动跟随操作系统主题设置" },
  { id: "light", name: "亮色", description: "明亮浅色模式" },
  { id: "dark", name: "暗色", description: "深色控制台模式" },
]

function getSystemPreference(): "light" | "dark" {
  if (typeof window === "undefined") return "light"
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light"
}

function resolveEffectivePreset(preset: ThemePreset): "light" | "dark" {
  if (preset === "system") {
    return getSystemPreference()
  }
  return preset
}

function applyTheme(preset: ThemePreset) {
  const html = document.documentElement
  const effective = resolveEffectivePreset(preset)
  html.style.removeProperty("--tp-primary")
  html.style.removeProperty("--el-color-primary")

  // Determine light/dark for class toggling
  if (effective === "dark") {
    html.classList.add("dark")
    resolvedMode.value = "dark"
  } else {
    html.classList.remove("dark")
    resolvedMode.value = "light"
  }

  // Set data-theme attribute for CSS variable selection
  html.setAttribute("data-theme", effective)
  window.amitiaDesktop?.setTheme(effective)

}

// Watch for preset changes
watch(() => state.value.preset, (val) => {
  applyTheme(val)
  localStorage.setItem(STORAGE_KEY, val)
})

// Listen for system theme changes
if (typeof window !== "undefined" && window.matchMedia) {
  window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", () => {
    if (state.value.preset === "system") {
      applyTheme("system")
    }
  })
}

async function loadFromServer() {
  try {
    const res = await apiClient.get("/api/theme")
    const d = (res.data as any)?.data || res.data
    if (d?.preset) {
      state.value.preset = normalizePreset(d.preset)
      state.value.accentColor = ""
      applyTheme(state.value.preset)
    }
    themeLoaded.value = true
  } catch {
    // Server not available, use localStorage
    applyTheme(state.value.preset)
    themeLoaded.value = true
  }
}

async function saveToServer(preset: ThemePreset) {
  try {
    await apiClient.put("/api/theme", { preset, accentColor: "" })
  } catch {
    // Silently fail - localStorage is the source of truth
  }
}

export function useTheme() {
  function setPreset(preset: ThemePreset) {
    state.value.preset = normalizePreset(preset)
    saveToServer(preset)
    if (state.value.preset === "light") {
      preferredLight.value = preset
    }
  }

  function toggleLightDark() {
    if (state.value.preset === "dark") {
      setPreset(preferredLight.value)
    } else {
      setPreset("dark")
    }
  }

  return {
    state,
    resolvedMode,
    themeLoaded,
    presets: THEME_PRESETS,
    setPreset,
    toggleLightDark,
    preferredLight,
    loadFromServer,
  }
}
