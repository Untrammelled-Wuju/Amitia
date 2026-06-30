// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, watch } from "vue"
import { apiClient } from "./useApi"

export type ThemePreset = "system" | "dark" | "light" | "calm-blue" | "warm-gray" | "mint" | "navy"

export interface ThemeState {
  preset: ThemePreset
  accentColor: string
  customTheme: Record<string, string> | null
}

const STORAGE_KEY = "ai-companion-theme"

const state = ref<ThemeState>({
  preset: (localStorage.getItem(STORAGE_KEY) as ThemePreset) || "system",
  accentColor: "",
  customTheme: null,
})

const resolvedMode = ref<"light" | "dark">("light")
const themeLoaded = ref(false)
const preferredLight = ref<ThemePreset>("light")

// 立即应用已保存的主题，避免刷新闪烁
applyTheme(state.value.preset)

export const THEME_PRESETS: { id: ThemePreset; name: string; description: string }[] = [
  { id: "system", name: "跟随系统", description: "自动跟随操作系统主题设置" },
  { id: "dark", name: "深色", description: "护眼深色模式" },
  { id: "light", name: "亮色", description: "明亮浅色模式" },
  { id: "calm-blue", name: "静谧蓝", description: "克制的蓝色中性风格" },
  { id: "warm-gray", name: "暖灰", description: "温暖中性灰色调" },
  { id: "mint", name: "薄荷绿", description: "清新薄荷浅色风格" },
  { id: "navy", name: "深邃蓝", description: "深海暗色护眼风格" },
]

function getSystemPreference(): "light" | "dark" {
  if (typeof window === "undefined") return "light"
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light"
}

function resolveEffectivePreset(preset: ThemePreset): "light" | "dark" | ThemePreset {
  if (preset === "system") {
    return getSystemPreference()
  }
  return preset
}

function applyTheme(preset: ThemePreset) {
  const html = document.documentElement
  const effective = resolveEffectivePreset(preset)

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

  // If custom theme, apply overrides
  if (state.value.customTheme) {
    applyCustomTheme(state.value.customTheme)
  }

  // Apply accent color if set
  if (state.value.accentColor) {
    html.style.setProperty("--tp-primary", state.value.accentColor)
    html.style.setProperty("--el-color-primary", state.value.accentColor)
  }
}

function applyCustomTheme(custom: Record<string, string>) {
  const html = document.documentElement
  for (const [key, value] of Object.entries(custom)) {
    html.style.setProperty(key, value)
  }
}

// Watch for preset changes
watch(() => state.value.preset, (val) => {
  applyTheme(val)
  localStorage.setItem(STORAGE_KEY, val)
})

// Watch for accent color changes
watch(() => state.value.accentColor, (val) => {
  if (val) {
    document.documentElement.style.setProperty("--tp-primary", val)
    document.documentElement.style.setProperty("--el-color-primary", val)
  }
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
      state.value.preset = d.preset
      state.value.accentColor = d.accentColor || ""
      applyTheme(d.preset)
    }
    themeLoaded.value = true
  } catch {
    // Server not available, use localStorage
    applyTheme(state.value.preset)
    themeLoaded.value = true
  }
}

async function saveToServer(preset: ThemePreset, accentColor?: string) {
  try {
    await apiClient.put("/api/theme", { preset, accentColor: accentColor || "" })
  } catch {
    // Silently fail - localStorage is the source of truth
  }
}

export function useTheme() {
  function setPreset(preset: ThemePreset) {
    state.value.preset = preset
    saveToServer(preset, state.value.accentColor)
    if (preset === "light" || preset === "calm-blue" || preset === "warm-gray" || preset === "mint") {
      preferredLight.value = preset
    }
  }

  function setAccentColor(color: string) {
    state.value.accentColor = color
    document.documentElement.style.setProperty("--tp-primary", color)
    document.documentElement.style.setProperty("--el-color-primary", color)
    saveToServer(state.value.preset, color)
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
    setAccentColor,
    toggleLightDark,
    preferredLight,
    loadFromServer,
  }
}
