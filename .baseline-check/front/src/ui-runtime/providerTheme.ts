import type { UIProviderDefinition } from "./types";

const appliedKeys = new Set<string>();
type TokenUnit = "raw" | "px" | "number" | "ms";
interface TokenSpec { css: string; unit: TokenUnit }

/** Canonical cross-platform token schema shared with Flutter ui_theme.dart. */
const TOKEN_SPECS: Record<string, TokenSpec> = {
  "colors.primary": { css: "--ac-color-primary", unit: "raw" },
  "colors.accent": { css: "--ac-color-primary", unit: "raw" },
  "colors.background": { css: "--ac-color-bg", unit: "raw" },
  "colors.backgroundPrimary": { css: "--ac-color-bg", unit: "raw" },
  "colors.backgroundSecondary": { css: "--ac-color-bg-secondary", unit: "raw" },
  "colors.surface": { css: "--ac-color-surface", unit: "raw" },
  "colors.surfacePrimary": { css: "--ac-color-surface", unit: "raw" },
  "colors.surfaceSecondary": { css: "--ac-color-surface-secondary", unit: "raw" },
  "colors.accentPrimary": { css: "--ac-color-primary", unit: "raw" },
  "colors.accentSecondary": { css: "--ac-color-accent-secondary", unit: "raw" },
  "colors.accentSoft": { css: "--ac-color-accent-soft", unit: "raw" },
  "colors.accentPressed": { css: "--ac-color-accent-pressed", unit: "raw" },
  "colors.textPrimary": { css: "--ac-color-text-primary", unit: "raw" },
  "colors.textSecondary": { css: "--ac-color-text-secondary", unit: "raw" },
  "colors.textMuted": { css: "--ac-color-text-muted", unit: "raw" },
  "colors.textTertiary": { css: "--ac-color-text-muted", unit: "raw" },
  "colors.textDisabled": { css: "--ac-color-text-disabled", unit: "raw" },
  "colors.border": { css: "--ac-color-border", unit: "raw" },
  "colors.borderPrimary": { css: "--ac-color-border", unit: "raw" },
  "colors.borderSecondary": { css: "--ac-color-border-secondary", unit: "raw" },
  "colors.danger": { css: "--ac-color-danger", unit: "raw" },
  "colors.error": { css: "--ac-color-danger", unit: "raw" },
  "colors.success": { css: "--ac-color-success", unit: "raw" },
  "colors.warning": { css: "--ac-color-warning", unit: "raw" },
  "colors.info": { css: "--ac-color-info", unit: "raw" },
  "colors.scrim": { css: "--ac-color-scrim", unit: "raw" },
  "colors.overlay": { css: "--ac-color-overlay", unit: "raw" },

  "spacing.xs": { css: "--ui-spacing-xs", unit: "px" },
  "spacing.sm": { css: "--ui-spacing-sm", unit: "px" },
  "spacing.md": { css: "--ui-spacing-md", unit: "px" },
  "spacing.lg": { css: "--ui-spacing-lg", unit: "px" },
  "spacing.xl": { css: "--ui-spacing-xl", unit: "px" },
  "spacing.xxl": { css: "--ui-spacing-xxl", unit: "px" },
  "spacing.xxxl": { css: "--ui-spacing-xxxl", unit: "px" },
  "spacing.page": { css: "--ui-spacing-page", unit: "px" },
  "spacing.card": { css: "--ui-spacing-card", unit: "px" },
  "spacing.section": { css: "--ui-spacing-section", unit: "px" },
  "spacing.component": { css: "--ui-spacing-component", unit: "px" },
  "spacing.tight": { css: "--ui-spacing-tight", unit: "px" },

  "radius.xs": { css: "--ui-radius-xs", unit: "px" },
  "radius.sm": { css: "--ui-radius-sm", unit: "px" },
  "radius.md": { css: "--ui-radius-md", unit: "px" },
  "radius.lg": { css: "--ui-radius-lg", unit: "px" },
  "radius.tag": { css: "--ui-radius-tag", unit: "px" },
  "radius.pill": { css: "--ui-radius-pill", unit: "px" },

  "typography.fontFamily": { css: "--ui-font-family", unit: "raw" },
  "typography.pageTitleSize": { css: "--ui-font-size-page-title", unit: "px" },
  "typography.pageLargeTitleSize": { css: "--ui-font-size-page-large-title", unit: "px" },
  "typography.sectionTitleSize": { css: "--ui-font-size-section-title", unit: "px" },
  "typography.cardTitleSize": { css: "--ui-font-size-card-title", unit: "px" },
  "typography.titleSize": { css: "--ui-font-size-page-title", unit: "px" },
  "typography.bodySize": { css: "--ui-font-size-body", unit: "px" },
  "typography.bodySmallSize": { css: "--ui-font-size-body-small", unit: "px" },
  "typography.captionSize": { css: "--ui-font-size-caption", unit: "px" },
  "typography.labelSize": { css: "--ui-font-size-label", unit: "px" },
  "typography.statusLabelSize": { css: "--ui-font-size-status", unit: "px" },
  "typography.buttonSize": { css: "--ui-font-size-button", unit: "px" },
  "typography.weightRegular": { css: "--ui-font-weight-regular", unit: "number" },
  "typography.weightMedium": { css: "--ui-font-weight-medium", unit: "number" },
  "typography.weightBold": { css: "--ui-font-weight-bold", unit: "number" },
  "typography.pageTitleWeight": { css: "--ui-font-weight-page-title", unit: "number" },
  "typography.sectionTitleWeight": { css: "--ui-font-weight-section-title", unit: "number" },
  "typography.cardTitleWeight": { css: "--ui-font-weight-card-title", unit: "number" },
  "typography.bodyWeight": { css: "--ui-font-weight-body", unit: "number" },
  "typography.labelWeight": { css: "--ui-font-weight-label", unit: "number" },
  "typography.buttonWeight": { css: "--ui-font-weight-button", unit: "number" },

  "icons.extraSmall": { css: "--ui-icon-size-xs", unit: "px" },
  "icons.small": { css: "--ui-icon-size-sm", unit: "px" },
  "icons.medium": { css: "--ui-icon-size", unit: "px" },
  "icons.size": { css: "--ui-icon-size", unit: "px" },
  "icons.large": { css: "--ui-icon-size-lg", unit: "px" },
  "icons.navigation": { css: "--ui-icon-size-navigation", unit: "px" },
  "icons.navigationSize": { css: "--ui-icon-size-navigation", unit: "px" },

  "components.toolbarHeight": { css: "--ui-toolbar-height", unit: "px" },
  "components.drawerWidth": { css: "--ui-drawer-width", unit: "px" },
  "components.drawerMaxWidth": { css: "--ui-drawer-width", unit: "px" },
  "components.controlHeight": { css: "--ui-control-height", unit: "px" },
  "components.compactControlHeight": { css: "--ui-control-height-compact", unit: "px" },
  "components.borderWidth": { css: "--ui-border-width", unit: "px" },
};

function clearApplied(root: HTMLElement) {
  for (const key of appliedKeys) root.style.removeProperty(key);
  appliedKeys.clear();
}

function formatToken(value: unknown, unit: TokenUnit): string {
  if (typeof value !== "number") return String(value);
  switch (unit) {
    case "px": return `${value}px`;
    case "ms": return `${value}ms`;
    case "number": return String(value);
    case "raw": return String(value);
  }
}

function kebab(value: string): string {
  return value.replace(/([a-z0-9])([A-Z])/g, "$1-$2").replace(/[_\s]+/g, "-").toLowerCase();
}

function componentVariantVariable(key: string, value: unknown): [string, string] | null {
  if (!key.startsWith("componentVariants.")) return null;
  const parts = key.split(".").filter(Boolean);
  if (parts.length !== 3) return null;
  const [, component, property] = parts;
  if (!component || !property) return null;
  const unit: TokenUnit = ["fontWeight", "opacity", "lineHeight", "flex"].includes(property)
    ? "number"
    : property.toLowerCase().includes("duration")
      ? "ms"
      : "px";
  return [`--ui-component-${kebab(component)}-${kebab(property)}`, formatToken(value, unit)];
}

function setVariable(root: HTMLElement, key: string, value: unknown) {
  if (value === null || value === undefined) return;
  if (key.startsWith("--")) {
    root.style.setProperty(key, String(value));
    appliedKeys.add(key);
    return;
  }
  const componentVariant = componentVariantVariable(key, value);
  if (componentVariant) {
    root.style.setProperty(componentVariant[0], componentVariant[1]);
    appliedKeys.add(componentVariant[0]);
    return;
  }
  const spec = TOKEN_SPECS[key];
  if (!spec) return;
  root.style.setProperty(spec.css, formatToken(value, spec.unit));
  appliedKeys.add(spec.css);
}

function flatten(source: Record<string, unknown>, prefix = ""): Array<[string, unknown]> {
  const rows: Array<[string, unknown]> = [];
  for (const [key, value] of Object.entries(source)) {
    const path = prefix ? `${prefix}.${key}` : key;
    if (value && typeof value === "object" && !Array.isArray(value)) rows.push(...flatten(value as Record<string, unknown>, path));
    else rows.push([path, value]);
  }
  return rows;
}

function selectTokenSource(provider: UIProviderDefinition, mode: "light" | "dark"): Record<string, unknown> | null {
  const metadata = provider.metadata ?? {};
  const source = (metadata.tokens ?? metadata.cssVariables ?? metadata.theme ?? metadata) as unknown;
  if (!source || typeof source !== "object" || Array.isArray(source)) return null;
  const record = source as Record<string, unknown>;
  const selected = record[mode];
  return selected && typeof selected === "object" && !Array.isArray(selected)
    ? { ...record, ...(selected as Record<string, unknown>) }
    : record;
}

function applyLayer(root: HTMLElement, provider: UIProviderDefinition, mode: "light" | "dark") {
  if (provider.builtin || !provider.enabled) return;
  const selected = selectTokenSource(provider, mode);
  if (selected) {
    for (const [key, value] of Object.entries(selected)) {
      if (key.startsWith("--")) setVariable(root, key, value);
    }
    for (const [key, value] of flatten(selected)) setVariable(root, key, value);
  }
  // componentVariants is an independent design-system layer and must not be
  // hidden when the same provider also declares metadata.tokens/theme.
  const variants = provider.metadata?.componentVariants;
  if (variants && typeof variants === "object" && !Array.isArray(variants)) {
    for (const [key, value] of flatten({ componentVariants: variants as Record<string, unknown> })) {
      setVariable(root, key, value);
    }
  }
  if (selected || variants) root.dataset.uiThemeProvider = provider.providerId;
}

/** Theme, token, icon and component providers share one semantic token cascade. Later layers win. */
export function applyProviderTheme(
  providers: UIProviderDefinition | Array<UIProviderDefinition | null> | null,
  mode: "light" | "dark",
) {
  if (typeof document === "undefined") return;
  const root = document.documentElement;
  clearApplied(root);
  delete root.dataset.uiThemeProvider;
  const layers = Array.isArray(providers) ? providers : providers ? [providers] : [];
  for (const provider of layers) if (provider) applyLayer(root, provider, mode);
}
