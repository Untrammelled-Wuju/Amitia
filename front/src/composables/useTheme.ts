// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, watch } from "vue";
import { apiClient } from "./useApi";

export type ThemePreset = "system" | "dark" | "light";
export type CornerStyle = 0 | 1 | 2;

export interface ThemeState {
  preset: ThemePreset;
  accentColor: string;
  fontScale: number;
  cornerStyle: CornerStyle;
  dynamicEffect: boolean;
  reduceAnimation: boolean;
}

interface StoredAppearance {
  accentColor?: unknown;
  fontScale?: unknown;
  cornerStyle?: unknown;
  dynamicEffect?: unknown;
  reduceAnimation?: unknown;
}

const STORAGE_KEY = "ai-companion-theme";
const APPEARANCE_STORAGE_KEY = "ai-companion-appearance";
const VALID_PRESETS: ThemePreset[] = ["system", "light", "dark"];
const DEFAULT_ACCENT = "#8A5728";

export const FONT_SCALE_OPTIONS = [
  { value: 0.9, label: "小" },
  { value: 1, label: "标准" },
  { value: 1.15, label: "大" },
  { value: 1.3, label: "超大" },
] as const;

export const ACCENT_COLOR_OPTIONS = [
  { value: "#8A5728", label: "暖棕" },
  { value: "#6C8FEA", label: "静谧蓝" },
  { value: "#52B788", label: "薄荷绿" },
  { value: "#E9A23B", label: "琥珀" },
] as const;

export const CORNER_STYLE_OPTIONS = [
  { value: 0 as CornerStyle, label: "克制" },
  { value: 1 as CornerStyle, label: "标准" },
  { value: 2 as CornerStyle, label: "圆润" },
] as const;

export const THEME_PRESETS: {
  id: ThemePreset;
  name: string;
  description: string;
}[] = [
  { id: "system", name: "跟随系统", description: "自动跟随操作系统主题设置" },
  { id: "light", name: "亮色", description: "明亮浅色模式" },
  { id: "dark", name: "暗色", description: "深色控制台模式" },
];

function normalizePreset(preset: unknown): ThemePreset {
  return VALID_PRESETS.includes(preset as ThemePreset)
    ? (preset as ThemePreset)
    : "system";
}

function normalizeFontScale(value: unknown): number {
  const parsed = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(parsed)) return 1;
  return Math.min(1.4, Math.max(0.8, parsed));
}

function normalizeCornerStyle(value: unknown): CornerStyle {
  const parsed = Math.round(Number(value));
  if (parsed <= 0) return 0;
  if (parsed >= 2) return 2;
  return 1;
}

function normalizeAccentColor(value: unknown): string {
  const raw = String(value ?? "").trim();
  if (!raw) return DEFAULT_ACCENT;
  const candidate = raw.startsWith("#") ? raw : `#${raw}`;
  if (/^#[0-9a-fA-F]{6}$/.test(candidate)) return candidate.toUpperCase();
  return DEFAULT_ACCENT;
}

function loadStoredAppearance(): StoredAppearance {
  try {
    const raw = localStorage.getItem(APPEARANCE_STORAGE_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch {
    return {};
  }
}

const storedAppearance = loadStoredAppearance();

const state = ref<ThemeState>({
  preset: normalizePreset(localStorage.getItem(STORAGE_KEY)),
  accentColor: normalizeAccentColor(storedAppearance.accentColor),
  fontScale: normalizeFontScale(storedAppearance.fontScale),
  cornerStyle: normalizeCornerStyle(storedAppearance.cornerStyle),
  dynamicEffect:
    typeof storedAppearance.dynamicEffect === "boolean"
      ? storedAppearance.dynamicEffect
      : true,
  reduceAnimation:
    typeof storedAppearance.reduceAnimation === "boolean"
      ? storedAppearance.reduceAnimation
      : false,
});

const resolvedMode = ref<"light" | "dark">("light");
const themeLoaded = ref(false);
const preferredLight = ref<ThemePreset>("light");

function getSystemPreference(): "light" | "dark" {
  if (typeof window === "undefined") return "light";
  return window.matchMedia("(prefers-color-scheme: dark)").matches
    ? "dark"
    : "light";
}

function resolveEffectivePreset(preset: ThemePreset): "light" | "dark" {
  if (preset === "system") return getSystemPreference();
  return preset;
}

function hexToRgb(hex: string): [number, number, number] {
  const normalized = normalizeAccentColor(hex).slice(1);
  return [
    Number.parseInt(normalized.slice(0, 2), 16),
    Number.parseInt(normalized.slice(2, 4), 16),
    Number.parseInt(normalized.slice(4, 6), 16),
  ];
}

function mixHex(hex: string, target: [number, number, number], ratio: number) {
  const source = hexToRgb(hex);
  const p = Math.min(1, Math.max(0, ratio));
  const values = source.map((channel, index) =>
    Math.round(channel + (target[index] - channel) * p),
  );
  return `#${values.map((value) => value.toString(16).padStart(2, "0")).join("")}`;
}

function rgba(hex: string, alpha: number) {
  const [r, g, b] = hexToRgb(hex);
  return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}

function persistAppearance() {
  localStorage.setItem(
    APPEARANCE_STORAGE_KEY,
    JSON.stringify({
      accentColor: state.value.accentColor,
      fontScale: state.value.fontScale,
      cornerStyle: state.value.cornerStyle,
      dynamicEffect: state.value.dynamicEffect,
      reduceAnimation: state.value.reduceAnimation,
    }),
  );
}

function applyAccent(html: HTMLElement, accent: string) {
  const normalized = normalizeAccentColor(accent);
  html.style.setProperty("--tp-primary", normalized);
  html.style.setProperty("--tp-primary-hover", mixHex(normalized, [0, 0, 0], 0.12));
  html.style.setProperty("--tp-primary-active", mixHex(normalized, [0, 0, 0], 0.2));
  html.style.setProperty("--tp-primary-soft", rgba(normalized, 0.14));
  html.style.setProperty("--tp-primary-bg", rgba(normalized, 0.12));
  html.style.setProperty("--tp-primary-border", rgba(normalized, 0.3));
  html.style.setProperty("--tp-primary-light-3", mixHex(normalized, [255, 255, 255], 0.3));
  html.style.setProperty("--tp-primary-light-5", mixHex(normalized, [255, 255, 255], 0.5));
  html.style.setProperty("--tp-primary-light-7", mixHex(normalized, [255, 255, 255], 0.7));
  html.style.setProperty("--tp-primary-light-8", mixHex(normalized, [255, 255, 255], 0.8));
  html.style.setProperty("--tp-primary-light-9", mixHex(normalized, [255, 255, 255], 0.9));
  html.style.setProperty("--el-color-primary", normalized);
}

function applyFontScale(html: HTMLElement, scale: number) {
  const normalized = normalizeFontScale(scale);
  const px = (base: number) => `${Math.round(base * normalized * 100) / 100}px`;
  html.style.setProperty("--ac-font-size-xs", px(12));
  html.style.setProperty("--ac-font-size-sm", px(13));
  html.style.setProperty("--ac-font-size-base", px(14));
  html.style.setProperty("--ac-font-size-lg", px(16));
  html.style.setProperty("--ac-font-size-xl", px(18));
}

function applyCornerStyle(html: HTMLElement, style: CornerStyle) {
  const radii = [
    [4, 6, 10, 12, 12],
    [6, 10, 14, 18, 16],
    [10, 14, 18, 22, 22],
  ][normalizeCornerStyle(style)];
  const [sm, md, lg, xl, composer] = radii;
  html.style.setProperty("--ac-radius-sm", `${sm}px`);
  html.style.setProperty("--ac-radius-md", `${md}px`);
  html.style.setProperty("--ac-radius-lg", `${lg}px`);
  html.style.setProperty("--ac-radius-xl", `${xl}px`);
  html.style.setProperty("--radius-sm", `${sm}px`);
  html.style.setProperty("--radius-md", `${md}px`);
  html.style.setProperty("--radius-lg", `${lg}px`);
  html.style.setProperty("--radius-composer", `${composer}px`);
}

function applyMotion(html: HTMLElement) {
  const reduce = !state.value.dynamicEffect || state.value.reduceAnimation;
  html.dataset.reduceMotion = reduce ? "true" : "false";
  if (reduce) {
    html.style.setProperty("--ac-transition-fast", "0ms");
    html.style.setProperty("--ac-transition-normal", "0ms");
  } else {
    html.style.removeProperty("--ac-transition-fast");
    html.style.removeProperty("--ac-transition-normal");
  }
}

function applyTheme(preset: ThemePreset) {
  const html = document.documentElement;
  const effective = resolveEffectivePreset(preset);

  if (effective === "dark") {
    html.classList.add("dark");
    resolvedMode.value = "dark";
  } else {
    html.classList.remove("dark");
    resolvedMode.value = "light";
  }
  html.setAttribute("data-theme", effective);

  applyAccent(html, state.value.accentColor);
  applyFontScale(html, state.value.fontScale);
  applyCornerStyle(html, state.value.cornerStyle);
  applyMotion(html);
}

function applyAppearance() {
  applyTheme(state.value.preset);
  persistAppearance();
}

// 立即应用已保存的外观，避免刷新闪烁。
applyTheme(state.value.preset);

watch(
  () => state.value.preset,
  (val) => {
    applyTheme(val);
    localStorage.setItem(STORAGE_KEY, val);
  },
);

if (typeof window !== "undefined" && window.matchMedia) {
  window
    .matchMedia("(prefers-color-scheme: dark)")
    .addEventListener("change", () => {
      if (state.value.preset === "system") applyTheme("system");
    });
}

async function loadFromServer() {
  try {
    const res = await apiClient.get("/api/theme");
    const d = (res.data as any)?.data || res.data;
    if (d?.preset) state.value.preset = normalizePreset(d.preset);
    if (typeof d?.accentColor === "string" && d.accentColor.trim()) {
      state.value.accentColor = normalizeAccentColor(d.accentColor);
    }
    applyAppearance();
  } catch {
    applyTheme(state.value.preset);
  } finally {
    themeLoaded.value = true;
  }
}

async function saveToServer() {
  try {
    await apiClient.put("/api/theme", {
      preset: state.value.preset,
      accentColor: state.value.accentColor,
    });
  } catch {
    // 外观仍保留在本地；服务恢复后下一次修改会重新同步。
  }
}

export function useTheme() {
  function setPreset(preset: ThemePreset) {
    state.value.preset = normalizePreset(preset);
    if (state.value.preset === "light") preferredLight.value = "light";
    void saveToServer();
  }

  function setFontScale(value: number) {
    state.value.fontScale = normalizeFontScale(value);
    applyAppearance();
  }

  function setAccentColor(value: string) {
    state.value.accentColor = normalizeAccentColor(value);
    applyAppearance();
    void saveToServer();
  }

  function setCornerStyle(value: number) {
    state.value.cornerStyle = normalizeCornerStyle(value);
    applyAppearance();
  }

  function setDynamicEffect(value: boolean) {
    state.value.dynamicEffect = Boolean(value);
    applyAppearance();
  }

  function setReduceAnimation(value: boolean) {
    state.value.reduceAnimation = Boolean(value);
    applyAppearance();
  }

  function toggleLightDark() {
    if (state.value.preset === "dark") {
      setPreset(preferredLight.value);
    } else {
      setPreset("dark");
    }
  }

  return {
    state,
    resolvedMode,
    themeLoaded,
    presets: THEME_PRESETS,
    fontScaleOptions: FONT_SCALE_OPTIONS,
    accentColorOptions: ACCENT_COLOR_OPTIONS,
    cornerStyleOptions: CORNER_STYLE_OPTIONS,
    setPreset,
    setFontScale,
    setAccentColor,
    setCornerStyle,
    setDynamicEffect,
    setReduceAnimation,
    toggleLightDark,
    preferredLight,
    loadFromServer,
  };
}
