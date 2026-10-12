import { applyStyleTokens } from "./providerTheme";
import { applyStyleRulesToDocument } from "./styleEngine";
import type { StylePatch, StyleRule } from "./styleProfile";

function mapValue(source: StylePatch, key: string): Record<string, unknown> {
  const value = source[key];
  return value && typeof value === "object" && !Array.isArray(value)
    ? value as Record<string, unknown>
    : {};
}

export function applyStylePatchToDocument(patch: StylePatch) {
  const tokens: Record<string, unknown> = {};
  const palette = mapValue(patch, "palette");
  if (Object.keys(palette).length > 0) tokens.colors = palette;
  const typography = mapValue(patch, "typography");
  if (Object.keys(typography).length > 0) tokens.typography = typography;
  const layout = mapValue(patch, "layout");
  const spacing = mapValue(layout, "spacing");
  if (Object.keys(spacing).length > 0) tokens.spacing = spacing;
  const radius = mapValue(layout, "radius");
  if (Object.keys(radius).length > 0) tokens.radius = radius;
  for (const key of [
    "density",
    "listItemMinHeight",
    "listItemMaxHeight",
    "buttonHeight",
    "inputMinHeight",
  ]) {
    if (layout[key] !== undefined) tokens[key] = layout[key];
  }
  const icons = mapValue(patch, "icons");
  if (Object.keys(icons).length > 0) tokens.icons = icons;
  const components = mapValue(patch, "components");
  if (Object.keys(components).length > 0) tokens.components = components;
  applyStyleTokens(tokens, mapValue(patch, "componentVariants"));
}

export function applyStyleRules(rules: StyleRule[]) {
  applyStyleRulesToDocument(rules);
}

export function applyStyleRuntime(patch: StylePatch, rules: StyleRule[]) {
  applyStylePatchToDocument(patch);
  applyStyleRules(rules);
}
