import { dshUnavailableError } from "./operation-error.mjs";

function text(value) {
  return value == null ? "" : String(value).trim();
}

export const THEME_PREFERENCES = Object.freeze(["light", "dark", "system"]);

export const DEFAULT_PREFERENCE = "system";

export const FONT_ROLES = Object.freeze(["text", "code", "terminal"]);

export const FONT_SIZE_SPECS = Object.freeze({
  text: Object.freeze({ field: "fontSize", variable: "--dsh-content-font-size", min: 10, max: 22, default: 14 }),
  code: Object.freeze({ field: "codeFontSize", variable: "--dsh-code-font-size", min: 10, max: 16, default: 11 }),
  terminal: Object.freeze({ field: "terminalFontSize", variable: "--dsh-terminal-font-size", min: 10, max: 20, default: 13 }),
});

export const DEFAULT_FONT_SIZES = Object.freeze({
  text: FONT_SIZE_SPECS.text.default,
  code: FONT_SIZE_SPECS.code.default,
  terminal: FONT_SIZE_SPECS.terminal.default,
});

export const DEFAULT_FONT_FAMILIES = Object.freeze({ text: "", code: "", terminal: "" });

export const DSH_BASE_TOKEN_SOURCES = Object.freeze({
  "--dsw-alias-bg-base": "var(--tp-page)",
  "--dsw-alias-bg-layer-1": "var(--tp-panel)",
  "--dsw-alias-bg-layer-2": "var(--tp-panel-soft)",
  "--dsw-alias-bg-overlay": "var(--tp-float)",
  "--dsw-alias-border-l1": "var(--tp-border)",
  "--dsw-alias-border-l2": "var(--tp-border-strong)",
  "--dsw-alias-brand-primary": "var(--tp-primary)",
  "--dsw-alias-label-primary": "var(--tp-text)",
  "--dsw-alias-label-secondary": "var(--tp-text-secondary)",
  "--dsw-alias-state-error-primary": "var(--tp-danger)",
  "--dsw-alias-state-idle-primary": "var(--tp-text-muted)",
  "--dsw-alias-state-success-primary": "var(--tp-success)",
  "--dsw-alias-state-warn-primary": "var(--tp-warning)",
  "--dsw-specific-sidebar-fill": "var(--tp-shell)",
});

export const DSH_THEME_TOKEN_INSPECTION = Object.freeze([
  { name: "--dsw-alias-bg-base", description: "Application base background.", valueType: "CSS color", requiresLightAndDark: true },
  { name: "--dsw-alias-bg-layer-1", description: "Primary raised surface background.", valueType: "CSS color", requiresLightAndDark: true },
  { name: "--dsw-alias-bg-layer-2", description: "Secondary nested surface background.", valueType: "CSS color", requiresLightAndDark: true },
  { name: "--dsw-alias-bg-overlay", description: "Overlay and popover background.", valueType: "CSS color", requiresLightAndDark: true },
  { name: "--dsw-alias-border-l1", description: "Primary subtle border.", valueType: "CSS color", requiresLightAndDark: true },
  { name: "--dsw-alias-border-l2", description: "Secondary stronger border.", valueType: "CSS color", requiresLightAndDark: true },
  { name: "--dsw-alias-brand-primary", description: "Primary brand accent.", valueType: "CSS color", requiresLightAndDark: true },
  { name: "--dsw-alias-label-primary", description: "Primary text color.", valueType: "CSS color", requiresLightAndDark: true },
  { name: "--dsw-alias-label-secondary", description: "Secondary text color.", valueType: "CSS color", requiresLightAndDark: true },
  { name: "--dsw-alias-state-error-primary", description: "Primary error state color.", valueType: "CSS color", requiresLightAndDark: true },
  { name: "--dsw-alias-state-idle-primary", description: "Primary inactive state color.", valueType: "CSS color", requiresLightAndDark: true },
  { name: "--dsw-alias-state-success-primary", description: "Primary success state color.", valueType: "CSS color", requiresLightAndDark: true },
  { name: "--dsw-alias-state-warn-primary", description: "Primary warning state color.", valueType: "CSS color", requiresLightAndDark: true },
  { name: "--dsw-specific-sidebar-fill", description: "Sidebar column and title-row background.", valueType: "CSS color", requiresLightAndDark: true },
]);

const HOST_STORAGE_KEY = "ai-companion-theme";

const BUILTIN_THEMES = Object.freeze([
  Object.freeze({ id: "light", colorScheme: "light", tokens: Object.freeze({}) }),
  Object.freeze({ id: "dark", colorScheme: "dark", tokens: Object.freeze({}) }),
]);

function isPreference(value) {
  return THEME_PREFERENCES.includes(text(value));
}

function isFontSize(role, px) {
  const spec = FONT_SIZE_SPECS[role];
  return Number.isInteger(px) && px >= spec.min && px <= spec.max;
}

function normalizeFontFamily(value) {
  return text(value)
    .split(",")
    .map((part) => part.trim())
    .filter(Boolean)
    .join(", ");
}

function validateThemeTokens(themeId, tokens) {
  if (tokens == null) return Object.freeze({});
  if (typeof tokens !== "object" || Array.isArray(tokens)) {
    throw new TypeError(`theme "${themeId}" tokens must be a token-name to CSS-value dictionary`);
  }
  const validated = {};
  for (const [name, value] of Object.entries(tokens)) {
    if (typeof value !== "string") {
      throw new TypeError(`theme "${themeId}" token "${name}" must be a CSS value string`);
    }
    validated[text(name)] = value;
  }
  return Object.freeze(validated);
}

function validateTokenOverrides(source, tokens) {
  if (tokens == null) return Object.freeze({});
  if (typeof tokens !== "object" || Array.isArray(tokens)) {
    throw new TypeError(`theme override "${source}" must be a token-name to { light, dark } dictionary`);
  }
  const validated = {};
  for (const [name, value] of Object.entries(tokens)) {
    if (typeof value === "string") {
      throw new TypeError(
        `theme override "${name}" from "${source}" is a bare string — pass { light: ${JSON.stringify(value)}, dark: ${JSON.stringify(value)} } `
        + "(repeat the value when it is the same in both palettes); a single value goes illegible when the user switches color scheme",
      );
    }
    if (typeof value !== "object" || value === null
      || typeof value.light !== "string"
      || typeof value.dark !== "string") {
      throw new TypeError(
        `theme override "${name}" from "${source}" must map to a { light, dark } pair of strings — one value per color scheme`,
      );
    }
    validated[text(name)] = Object.freeze({ light: value.light, dark: value.dark });
  }
  return Object.freeze(validated);
}

function validateIcons(themeId, icons) {
  if (icons == null) return undefined;
  if (typeof icons !== "object" || Array.isArray(icons)) {
    throw new TypeError(`theme "${themeId}" icons must be an icon-id to SVG document dictionary`);
  }
  const validated = {};
  for (const [name, markup] of Object.entries(icons)) {
    if (typeof markup !== "string") {
      throw new TypeError(`theme "${themeId}" icon "${name}" must be an SVG document string`);
    }
    validated[text(name)] = markup;
  }
  return Object.freeze(validated);
}

function hostStorage(documentValue) {
  try {
    return documentValue?.defaultView?.localStorage ?? null;
  } catch (error) {
    return null;
  }
}

export function createDshThemeRuntime(options = {}) {
  const emit = typeof options.emit === "function" ? options.emit : () => {};
  const hostActions = options.hostActions || {};
  const documentValue = options.document ?? globalThis.document ?? null;
  const storage = options.storage ?? hostStorage(documentValue);
  const matchMediaValue = typeof options.matchMedia === "function"
    ? options.matchMedia
    : typeof globalThis.matchMedia === "function"
      ? globalThis.matchMedia.bind(globalThis)
      : null;

  let themes = [...BUILTIN_THEMES];
  let selectedThemeId = "";
  let fontSizes = readHostFontSizes();
  let fontFamilies = readHostFontFamilies();
  let overrideSeq = 0;
  const overrides = new Map();
  const appliedTokens = new Set();
  let revision = 0;
  let disposed = false;
  let hostObserver = null;
  let systemMedia = null;
  let systemListener = null;
  let snapshot = buildSnapshot();

  function readHost() {
    const root = documentValue?.documentElement;
    if (!root) return { preference: DEFAULT_PREFERENCE, colorScheme: "light" };
    const attribute = text(root.getAttribute?.("data-theme"));
    const dark = root.classList?.contains?.("dark") === true || attribute === "dark";
    let stored = "";
    try {
      stored = text(storage?.getItem?.(HOST_STORAGE_KEY));
    } catch (error) {
      stored = "";
    }
    return {
      preference: isPreference(stored) ? stored : DEFAULT_PREFERENCE,
      colorScheme: dark ? "dark" : "light",
    };
  }

  function readStyleValue(variable) {
    const root = documentValue?.documentElement;
    const body = documentValue?.body;
    const fromRoot = text(root?.style?.getPropertyValue?.(variable));
    if (fromRoot) return fromRoot;
    return text(body?.style?.getPropertyValue?.(variable));
  }

  function readHostFontSizes() {
    const sizes = { ...DEFAULT_FONT_SIZES };
    for (const role of FONT_ROLES) {
      const parsed = Number.parseInt(readStyleValue(FONT_SIZE_SPECS[role].variable), 10);
      if (isFontSize(role, parsed)) sizes[role] = parsed;
    }
    return Object.freeze(sizes);
  }

  function readHostFontFamilies() {
    const families = { ...DEFAULT_FONT_FAMILIES };
    for (const role of FONT_ROLES) families[role] = normalizeFontFamily(readStyleValue(`--dsh-font-family-${role}`));
    return Object.freeze(families);
  }

  function composeActive(theme) {
    if (overrides.size === 0) return theme;
    const tokens = { ...(theme.tokens ?? {}) };
    for (const layer of [...overrides.values()].sort((left, right) => left.seq - right.seq)) {
      for (const [name, modes] of Object.entries(layer.tokens)) tokens[name] = modes[theme.colorScheme];
    }
    return Object.freeze({ ...theme, tokens: Object.freeze(tokens) });
  }

  function buildSnapshot() {
    const host = readHost();
    const preference = selectedThemeId || host.preference;
    const theme = (selectedThemeId
      ? themes.find((entry) => entry.id === selectedThemeId)
      : themes.find((entry) => entry.id === host.colorScheme))
      ?? themes[0];
    return Object.freeze({
      preference,
      fontSizes,
      fontFamilies,
      active: composeActive(theme),
      themes: Object.freeze([...themes]),
      revision,
    });
  }

  function applyDom() {
    const root = documentValue?.documentElement;
    if (!root?.style) return;
    const next = new Map(Object.entries(DSH_BASE_TOKEN_SOURCES));
    for (const [name, value] of Object.entries(snapshot.active?.tokens ?? {})) {
      if (text(name)) next.set(text(name), text(value));
    }
    for (const role of FONT_ROLES) {
      next.set(FONT_SIZE_SPECS[role].variable, `${snapshot.fontSizes[role]}px`);
      const family = text(snapshot.fontFamilies[role]);
      if (family) next.set(`--dsh-font-family-${role}`, family);
    }
    for (const [name, value] of next) {
      if (value) root.style.setProperty(name, value);
      else root.style.removeProperty(name);
    }
    for (const name of appliedTokens) {
      if (!next.has(name)) root.style.removeProperty(name);
    }
    appliedTokens.clear();
    for (const name of next.keys()) appliedTokens.add(name);
    root.setAttribute("data-dsh-theme", text(snapshot.active?.id) || "light");
    root.setAttribute("data-dsh-color-scheme", text(snapshot.active?.colorScheme) || "light");
  }

  function reportFailure(label, result) {
    if (!result || typeof result.then !== "function") return;
    void result.catch((error) => {
      console.error(`[original-error] ${label}`, error);
    });
  }

  function publish() {
    revision += 1;
    snapshot = buildSnapshot();
    applyDom();
    reportFailure("DSH theme/change listener failed", emit("theme/change", snapshot));
  }

  function assertActive() {
    if (disposed) throw new Error("DSH theme service is disposed");
  }

  applyDom();

  const Observer = documentValue?.defaultView?.MutationObserver ?? globalThis.MutationObserver;
  if (typeof Observer === "function" && documentValue?.documentElement) {
    hostObserver = new Observer(() => {
      if (disposed) return;
      if (selectedThemeId) return;
      publish();
    });
    hostObserver.observe(documentValue.documentElement, { attributes: true, attributeFilter: ["class", "data-theme"] });
  }
  if (typeof matchMediaValue === "function") {
    systemMedia = matchMediaValue("(prefers-color-scheme: dark)");
    if (typeof systemMedia?.addEventListener === "function") {
      systemListener = () => {
        if (disposed) return;
        if (selectedThemeId || readHost().preference !== "system") return;
        publish();
      };
      systemMedia.addEventListener("change", systemListener);
    }
  }

  return Object.freeze({
    getTheme() {
      return snapshot;
    },
    exportInspectTokens() {
      const tokens = new Map(DSH_THEME_TOKEN_INSPECTION.map((token) => [token.name, token]));
      for (const theme of themes) {
        for (const name of Object.keys(theme.tokens ?? {})) {
          if (!tokens.has(name)) {
            tokens.set(name, { name, description: "Theme token registered by the current Client composition.", valueType: "CSS value", requiresLightAndDark: false });
          }
        }
      }
      for (const layer of overrides.values()) {
        for (const name of Object.keys(layer.tokens)) {
          if (!tokens.has(name)) {
            tokens.set(name, { name, description: "Theme token overridden by the current Client composition.", valueType: "CSS value", requiresLightAndDark: true });
          }
        }
      }
      return [...tokens.values()].map((token) => ({ ...token, cssVariable: token.cssVariable ?? token.name }))
        .sort((left, right) => left.name.localeCompare(right.name));
    },
    setTheme(id) {
      assertActive();
      const target = text(id);
      if (!target) throw new Error("DSH theme.setTheme requires a theme id");
      const registered = themes.some((theme) => theme.id === target);
      const builtin = THEME_PREFERENCES.includes(target);
      if (!registered && !builtin) throw new Error(`theme "${target}" is not registered`);
      const host = readHost();
      if ((selectedThemeId || host.preference) === target) return;
      if (builtin) {
        const matchesHost = target === "system" ? isPreference(host.preference) : host.colorScheme === target;
        if (!matchesHost) {
          if (typeof hostActions.setTheme !== "function") {
            throw dshUnavailableError(
              "theme.setTheme",
              "P13",
              `Amitia 宿主未向 DSH 插件暴露主题切换能力，无法把内置主题从 ${host.colorScheme} 切换为 ${target}`,
            );
          }
          reportFailure(`DSH theme.setTheme(${target}) host switch failed`, hostActions.setTheme(target));
        }
        selectedThemeId = "";
      } else {
        selectedThemeId = target;
      }
      publish();
    },
    setFontSize(role, px) {
      assertActive();
      const key = text(role);
      if (!FONT_ROLES.includes(key)) throw new Error(`DSH theme.setFontSize role ${JSON.stringify(role)} is unknown`);
      const spec = FONT_SIZE_SPECS[key];
      const size = Number(px);
      if (!isFontSize(key, size)) throw new Error(`${key} font size ${px} is outside ${spec.min}..${spec.max}`);
      if (fontSizes[key] === size) return;
      fontSizes = Object.freeze({ ...fontSizes, [key]: size });
      publish();
    },
    setFontFamily(role, value) {
      assertActive();
      const key = text(role);
      if (!FONT_ROLES.includes(key)) throw new Error(`DSH theme.setFontFamily role ${JSON.stringify(role)} is unknown`);
      const family = normalizeFontFamily(value);
      if (fontFamilies[key] === family) return;
      fontFamilies = Object.freeze({ ...fontFamilies, [key]: family });
      publish();
    },
    register(definition) {
      assertActive();
      if (!definition || typeof definition !== "object") throw new Error("DSH theme definition is required");
      const id = text(definition.id);
      if (!id) throw new Error("DSH theme definition id is required");
      if (id === "system") throw new Error('"system" is a preference, not a registrable theme id');
      if (themes.some((theme) => theme.id === id)) throw new Error(`theme "${id}" is already registered`);
      const colorScheme = text(definition.colorScheme);
      if (colorScheme !== "light" && colorScheme !== "dark") {
        throw new Error(`theme "${id}" colorScheme must be "light" or "dark", received ${JSON.stringify(definition.colorScheme)}`);
      }
      const icons = validateIcons(id, definition.icons);
      const theme = Object.freeze({
        id,
        colorScheme,
        tokens: validateThemeTokens(id, definition.tokens),
        ...(icons ? { icons } : {}),
      });
      themes = [...themes, theme];
      publish();
      let active = true;
      return () => {
        if (!active) return;
        active = false;
        if (!themes.some((entry) => entry.id === id)) return;
        themes = themes.filter((entry) => entry.id !== id);
        if (selectedThemeId === id) selectedThemeId = "";
        if (disposed) return;
        publish();
      };
    },
    overrideTokens(source, tokens) {
      assertActive();
      const origin = text(source);
      if (!origin) throw new Error("DSH theme.overrideTokens requires a source name");
      const layer = { seq: overrideSeq++, tokens: validateTokenOverrides(origin, tokens) };
      overrides.set(origin, layer);
      publish();
      let active = true;
      return () => {
        if (!active) return;
        active = false;
        if (overrides.get(origin) !== layer) return;
        overrides.delete(origin);
        if (disposed) return;
        publish();
      };
    },
    dispose() {
      if (disposed) return;
      disposed = true;
      hostObserver?.disconnect?.();
      hostObserver = null;
      if (systemMedia && systemListener) systemMedia.removeEventListener?.("change", systemListener);
      systemMedia = null;
      systemListener = null;
      overrides.clear();
      const root = documentValue?.documentElement;
      if (root?.style) {
        for (const name of appliedTokens) root.style.removeProperty(name);
        root.removeAttribute?.("data-dsh-theme");
        root.removeAttribute?.("data-dsh-color-scheme");
      }
      appliedTokens.clear();
    },
  });
}
