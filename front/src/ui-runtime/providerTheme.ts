import type { UIProviderDefinition } from "./types";

const appliedKeys = new Set<string>();

const SEMANTIC_TOKEN_MAP: Record<string, string> = {
  "colors.primary": "--ac-color-primary",
  "colors.accent": "--ac-color-primary",
  "colors.background": "--ac-color-bg",
  "colors.surface": "--ac-color-surface",
  "colors.textPrimary": "--ac-color-text-primary",
  "colors.textSecondary": "--ac-color-text-secondary",
  "colors.textMuted": "--ac-color-text-muted",
  "colors.border": "--ac-color-border",
  "colors.danger": "--ac-color-danger",
  "colors.success": "--ac-color-success",
  "spacing.xs": "--ui-spacing-xs",
  "spacing.sm": "--ui-spacing-sm",
  "spacing.md": "--ui-spacing-md",
  "spacing.lg": "--ui-spacing-lg",
  "spacing.xl": "--ui-spacing-xl",
  "spacing.page": "--ui-spacing-page",
  "radius.xs": "--ui-radius-xs",
  "radius.sm": "--ui-radius-sm",
  "radius.md": "--ui-radius-md",
  "radius.lg": "--ui-radius-lg",
  "radius.pill": "--ui-radius-pill",
  "typography.fontFamily": "--ui-font-family",
  "typography.bodySize": "--ui-font-size-body",
  "typography.captionSize": "--ui-font-size-caption",
  "typography.titleSize": "--ui-font-size-title",
  "typography.weightRegular": "--ui-font-weight-regular",
  "typography.weightMedium": "--ui-font-weight-medium",
  "typography.weightBold": "--ui-font-weight-bold",
  "icons.size": "--ui-icon-size",
  "icons.navigationSize": "--ui-icon-size-navigation",
  "components.toolbarHeight": "--ui-toolbar-height",
  "components.drawerWidth": "--ui-drawer-width",
  "components.controlHeight": "--ui-control-height",
  "components.borderWidth": "--ui-border-width",
};

function clearApplied(root: HTMLElement) {
  for (const key of appliedKeys) root.style.removeProperty(key);
  appliedKeys.clear();
}

function setVariable(root: HTMLElement, key: string, value: unknown) {
  if (value === null || value === undefined) return;
  const cssKey = key.startsWith("--") ? key : SEMANTIC_TOKEN_MAP[key];
  if (!cssKey) return;
  const cssValue = typeof value === "number" ? `${value}px` : String(value);
  root.style.setProperty(cssKey, cssValue);
  appliedKeys.add(cssKey);
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
    ? selected as Record<string, unknown>
    : record;
}

function applyLayer(root: HTMLElement, provider: UIProviderDefinition, mode: "light" | "dark") {
  if (provider.builtin || !provider.enabled) return;
  const selected = selectTokenSource(provider, mode);
  if (!selected) return;
  for (const [key, value] of Object.entries(selected)) {
    if (key.startsWith("--")) setVariable(root, key, value);
  }
  for (const [key, value] of flatten(selected)) setVariable(root, key, value);
  root.dataset.uiThemeProvider = provider.providerId;
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
