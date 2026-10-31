import type { StyleRule, StyleScope } from "./styleProfile";
import { normalizeStylePageId, stylePageMatches } from "./stylePage";

const STYLE_ELEMENT_ID = "amitia-ui-style-rules-v1";
const MAX_SCAN_ELEMENTS = 1000;
const MAX_COMPUTED_STYLES = 24;
const STATE_NAMES = new Set([
  "default",
  "hover",
  "focus",
  "focus-visible",
  "active",
  "disabled",
  "selected",
  "expanded",
  "dark",
  "light",
]);
const EXCLUDED_TAGS = new Set(["SCRIPT", "STYLE", "META", "LINK", "NOSCRIPT", "TEMPLATE"]);
const UNITLESS_PROPERTIES = new Set([
  "opacity",
  "font-weight",
  "line-height",
  "z-index",
  "flex",
  "flex-grow",
  "flex-shrink",
  "order",
  "zoom",
  "animation-iteration-count",
  "column-count",
]);
const PLACEHOLDER_PROPERTIES: Record<string, string> = {
  placeholderColor: "color",
  placeholderOpacity: "opacity",
  placeholderFontSize: "font-size",
  placeholderFontWeight: "font-weight",
};
const PROPERTY_ALIASES: Record<string, string> = {
  foregroundColor: "color",
  textColor: "color",
  iconColor: "color",
  backgroundColor: "background-color",
  borderColor: "border-color",
  borderWidth: "border-width",
  borderRadius: "border-radius",
  borderStyle: "border-style",
  boxShadow: "box-shadow",
  fontFamily: "font-family",
  fontSize: "font-size",
  fontWeight: "font-weight",
  lineHeight: "line-height",
  letterSpacing: "letter-spacing",
  textAlign: "text-align",
  textDecoration: "text-decoration",
  textTransform: "text-transform",
  minWidth: "min-width",
  maxWidth: "max-width",
  minHeight: "min-height",
  maxHeight: "max-height",
  flexDirection: "flex-direction",
  justifyContent: "justify-content",
  alignItems: "align-items",
  alignContent: "align-content",
  backgroundImage: "background-image",
  backgroundSize: "background-size",
  backgroundPosition: "background-position",
  backgroundRepeat: "background-repeat",
  backdropFilter: "backdrop-filter",
  transformOrigin: "transform-origin",
  pointerEvents: "pointer-events",
  userSelect: "user-select",
  whiteSpace: "white-space",
  overflowX: "overflow-x",
  overflowY: "overflow-y",
};

export interface DomInspectionOptions {
  scope?: StyleScope;
  target?: string;
  limit?: number;
  includeComputed?: boolean;
}

export interface DomTargetInspection {
  target: string | null;
  selector: string;
  page: string;
  surface?: string;
  tag: string;
  role: string | null;
  text: string;
  classes: string[];
  bounds: {
    x: number;
    y: number;
    width: number;
    height: number;
  };
  computed: Record<string, string>;
}

export interface DomInspectionResult {
  page: string;
  platform: string;
  count: number;
  targets: DomTargetInspection[];
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === "object" && !Array.isArray(value);
}

function escapeAttribute(value: string): string {
  return value.replace(/\\/g, "\\\\").replace(/"/g, "\\\"");
}

function escapeIdentifier(value: string): string {
  if (typeof CSS !== "undefined" && typeof CSS.escape === "function") return CSS.escape(value);
  return value.replace(/[^a-zA-Z0-9_-]/g, (character) => `\\${character}`);
}

function kebabCase(value: string): string {
  if (value.startsWith("--")) return value;
  return value
    .replace(/([a-z0-9])([A-Z])/g, "$1-$2")
    .replace(/_/g, "-")
    .toLowerCase();
}

function cssPropertyName(property: string): string | null {
  if (property.startsWith("--")) {
    return /^--[a-zA-Z0-9_-]+$/.test(property) ? property : null;
  }
  const mapped = PROPERTY_ALIASES[property] ?? kebabCase(property);
  if (!/^-?[a-zA-Z][a-zA-Z0-9-]*$/.test(mapped)) return null;
  if (mapped.startsWith("behavior") || mapped.includes("binding")) return null;
  return mapped;
}

function serializeShorthand(value: Record<string, unknown>): string | null {
  const top = value.top ?? value.y ?? 0;
  const right = value.right ?? value.x ?? 0;
  const bottom = value.bottom ?? value.y ?? top;
  const left = value.left ?? value.x ?? right;
  const values = [top, right, bottom, left].map((item) => serializeValue(item, "padding"));
  if (values.some((item) => !item)) return null;
  return values.join(" ");
}

function serializeValue(value: unknown, property: string): string | null {
  if (typeof value === "number") {
    if (!Number.isFinite(value)) return null;
    return property.startsWith("--") || UNITLESS_PROPERTIES.has(property) ? String(value) : `${value}px`;
  }
  if (typeof value === "boolean") return value ? "1" : "0";
  if (Array.isArray(value)) {
    const values = value.map((item) => serializeValue(item, property)).filter((item): item is string => !!item);
    return values.length === value.length ? values.join(" ") : null;
  }
  if (isRecord(value)) {
    if (
      property.endsWith("padding") ||
      property.endsWith("margin") ||
      property === "inset" ||
      property === "gap"
    ) {
      return serializeShorthand(value);
    }
    return null;
  }
  if (value === null || value === undefined) return null;
  const text = String(value).trim();
  if (!text) return null;
  if (/javascript\s*:/i.test(text)) return null;
  if (/expression\s*\(/i.test(text)) return null;
  if (/url\s*\(/i.test(text)) return null;
  if (/@import/i.test(text)) return null;
  if (/behavior\s*:/i.test(text)) return null;
  if (/[;{}<>]/.test(text)) return null;
  return text;
}

function declarationBlock(properties: Record<string, unknown>) {
  const declarations: string[] = [];
  const pseudoDeclarations: Record<string, string[]> = {};
  for (const [rawProperty, rawValue] of Object.entries(properties)) {
    if (STATE_NAMES.has(rawProperty.toLowerCase())) continue;
    const placeholderProperty = PLACEHOLDER_PROPERTIES[rawProperty];
    if (placeholderProperty) {
      const value = serializeValue(rawValue, placeholderProperty);
      if (!value) continue;
      pseudoDeclarations["::placeholder"] ??= [];
      pseudoDeclarations["::placeholder"].push(`${placeholderProperty}: ${value}`);
      continue;
    }
    const property = cssPropertyName(rawProperty);
    if (!property) continue;
    const value = serializeValue(rawValue, property);
    if (!value) continue;
    declarations.push(`${property}: ${value}`);
  }
  for (const [pseudo, entries] of Object.entries(pseudoDeclarations)) {
    declarations.push(`${pseudo}|${entries.join("; ")}`);
  }
  return declarations;
}

function targetSelector(target: string): string | null {
  let value = target.trim();
  if (/^dom:/i.test(value)) value = value.slice(4).trim();
  else if (/^css:/i.test(value)) value = value.slice(4).trim();
  if (!value) return null;
  if (/[{};@<>]/.test(value)) return null;
  if (typeof document !== "undefined") {
    const nodeSelector = `[data-amitia-node-target="${escapeAttribute(value)}"]`;
    try {
      if (document.querySelector(nodeSelector)) return nodeSelector;
    } catch (error) {
  console.error("[original-error]", error);
}
  }
  if (/^(html|body|\*)$/i.test(value)) return value.toLowerCase();
  if (/^(#|\.|\[|:)/.test(value) || /[\s>+~,]/.test(value)) {
    try {
      document.querySelector(value);
      return value;
    } catch (error) {
  console.error("[original-error]", error);
return null;
    }
  }
  if (/^[a-zA-Z][a-zA-Z0-9-]*$/.test(value)) return value.toLowerCase();
  return `[data-amitia-ui="${escapeAttribute(value)}"]`;
}

function scopeAttributes(scope: StyleScope): string {
  let attributes = "";
  if (scope.platform) attributes += `[data-amitia-platform="${escapeAttribute(scope.platform)}"]`;
  if (scope.page) {
    attributes += `[data-amitia-page="${escapeAttribute(normalizeStylePageId(scope.page))}"]`;
  }
  return attributes;
}

function surfacePrefix(scope: StyleScope): string {
  return scope.surface
    ? `[data-amitia-surface="${escapeAttribute(scope.surface)}"] `
    : "";
}

function selectorForState(scope: StyleScope, selector: string, state: string): string[] {
  let root = `html${scopeAttributes(scope)}`;
  const target = selector.startsWith("body")
    ? selector
    : selector.startsWith("html")
      ? selector
      : `body ${surfacePrefix(scope)}${selector}`;
  const values = state.split(/[.:+]/).map((item) => item.trim()).filter(Boolean);
  if (values.length === 0 || values.includes("default")) return [`${root} ${target}`];
  if (values.includes("dark")) root += ".dark";
  else if (values.includes("light")) root += ":not(.dark)";
  let next = target;
  for (const value of values) {
    if (value === "hover") next += ":hover";
    else if (value === "focus") next += ":focus";
    else if (value === "focus-visible") next += ":focus-visible";
    else if (value === "active") next += ":active";
    else if (value === "selected") next += ".is-selected";
    else if (value === "expanded") next += '[aria-expanded="true"]';
    else if (value === "disabled") next += ":disabled";
    else if (value !== "dark" && value !== "light" && value !== "default") return [];
  }
  if (next === target) return [`${root} ${target}`];
  if (values.includes("disabled")) {
    const ariaSelector = next.replace(/:disabled$/, '[aria-disabled="true"]');
    return [`${root} ${next}`, `${root} ${ariaSelector}`];
  }
  return [`${root} ${next}`];
}

function expandedRules(rule: StyleRule): StyleRule[] {
  const baseProperties: Record<string, unknown> = {};
  const stateProperties: Array<{ state: string; properties: Record<string, unknown> }> = [];
  for (const [property, value] of Object.entries(rule.properties)) {
    if (STATE_NAMES.has(property.toLowerCase()) && isRecord(value)) {
      stateProperties.push({ state: property.toLowerCase(), properties: value });
    } else {
      baseProperties[property] = value;
    }
  }
  const rules: StyleRule[] = [];
  if (Object.keys(baseProperties).length > 0) rules.push({ ...rule, properties: baseProperties });
  for (const item of stateProperties) {
    rules.push({
      ...rule,
      id: `${rule.id}_${item.state}`,
      states: [item.state],
      properties: item.properties,
    });
  }
  return rules;
}

function buildRuleCss(rule: StyleRule): string {
  const selector = targetSelector(rule.target);
  if (!selector) return "";
  const declarations = declarationBlock(rule.properties);
  if (declarations.length === 0) return "";
  const normalDeclarations: string[] = [];
  const pseudoDeclarations: Array<[string, string]> = [];
  for (const declaration of declarations) {
    const separator = declaration.indexOf("|");
    if (separator > 0) {
      pseudoDeclarations.push([declaration.slice(0, separator), declaration.slice(separator + 1)]);
    } else {
      normalDeclarations.push(declaration);
    }
  }
  const lines: string[] = [];
  if (normalDeclarations.length > 0) {
    for (const state of rule.states.length > 0 ? rule.states : ["default"]) {
      const selectors = selectorForState(rule.scope, selector, state);
      if (selectors.length === 0) continue;
      lines.push(`${selectors.join(", ")} { ${normalDeclarations.join("; ")} }`);
    }
  }
  for (const [pseudo, body] of pseudoDeclarations) {
    for (const state of rule.states.length > 0 ? rule.states : ["default"]) {
      const selectors = selectorForState(rule.scope, selector, state);
      if (selectors.length === 0) continue;
      lines.push(`${selectors.map((item) => `${item}${pseudo}`).join(", ")} { ${body} }`);
    }
  }
  return lines.join("\n");
}

export function applyStyleRulesToDocument(rules: StyleRule[]) {
  if (typeof document === "undefined") return;
  let style = document.getElementById(STYLE_ELEMENT_ID) as HTMLStyleElement | null;
  if (!style) {
    style = document.createElement("style");
    style.id = STYLE_ELEMENT_ID;
    style.setAttribute("data-amitia-ui-style-runtime", "true");
    document.head.appendChild(style);
  }
  const css = rules.flatMap(expandedRules).map(buildRuleCss).filter(Boolean).join("\n");
  style.textContent = css;
}

function isElementVisible(element: Element): boolean {
  if (EXCLUDED_TAGS.has(element.tagName)) return false;
  if (element.hasAttribute("hidden")) return false;
  const style = getComputedStyle(element);
  if (style.display === "none" || style.visibility === "hidden" || style.opacity === "0") return false;
  return true;
}

function hasUniqueId(element: Element): boolean {
  const id = element.getAttribute("id");
  if (!id) return false;
  try {
    return document.querySelectorAll(`#${escapeIdentifier(id)}`).length === 1;
  } catch (error) {
  console.error("[original-error]", error);
return false;
  }
}

function stableDomSelector(element: Element): string {
  const parts: string[] = [];
  let current: Element | null = element;
  while (current && current !== document.documentElement) {
    const nodeTarget = current.getAttribute("data-amitia-node-target");
    if (nodeTarget) {
      parts.unshift(`[data-amitia-node-target="${escapeAttribute(nodeTarget)}"]`);
      break;
    }
    const semanticTarget = current.getAttribute("data-amitia-ui");
    if (semanticTarget) {
      parts.unshift(`[data-amitia-ui="${escapeAttribute(semanticTarget)}"]`);
      break;
    }
    const testId = current.getAttribute("data-testid");
    if (testId && document.querySelectorAll(`[data-testid="${escapeAttribute(testId)}"]`).length === 1) {
      parts.unshift(`[data-testid="${escapeAttribute(testId)}"]`);
      break;
    }
    if (hasUniqueId(current)) {
      parts.unshift(`#${escapeIdentifier(current.getAttribute("id") as string)}`);
      break;
    }
    const parent: Element | null = current.parentElement;
    if (!parent) break;
    const tag = current.tagName.toLowerCase();
    const siblings = Array.from(parent.children).filter((item) => item.tagName === current?.tagName);
    const segment = siblings.length > 1
      ? `${tag}:nth-of-type(${siblings.indexOf(current) + 1})`
      : tag;
    parts.unshift(segment);
    if (current === document.body) break;
    current = parent;
  }
  if (parts[0]?.startsWith("body")) return parts.join(" > ");
  return `body${parts.length > 0 ? ` > ${parts.join(" > ")}` : ""}`;
}

function nearestSurface(element: Element): string | undefined {
  const surface = element.closest("[data-amitia-surface]")?.getAttribute("data-amitia-surface");
  return surface || undefined;
}

function elementPage(): string {
  return normalizeStylePageId(document.documentElement.dataset.amitiaPage);
}

function elementPlatform(): string {
  return document.documentElement.dataset.amitiaPlatform ?? "web";
}

function selectorMatchesScope(element: Element, scope: StyleScope | undefined): boolean {
  if (!scope) return true;
  if (scope.platform && elementPlatform() !== scope.platform) return false;
  if (scope.page && !stylePageMatches(elementPage(), scope.page)) return false;
  if (scope.surface && nearestSurface(element) !== scope.surface) return false;
  return true;
}

function elementTarget(element: Element): string | null {
  return element.getAttribute("data-amitia-node-target") ?? element.getAttribute("data-amitia-ui");
}

function inspectElement(element: Element, includeComputed: boolean): DomTargetInspection {
  const rect = element.getBoundingClientRect();
  const computed: Record<string, string> = {};
  if (includeComputed) {
    const style = getComputedStyle(element);
    for (const property of Array.from(style).slice(0, MAX_COMPUTED_STYLES)) {
      computed[property] = style.getPropertyValue(property);
    }
    for (const property of [
      "background-color",
      "color",
      "border-color",
      "border-width",
      "border-radius",
      "padding",
      "margin",
      "font-family",
      "font-size",
      "font-weight",
      "opacity",
      "box-shadow",
      "width",
      "height",
      "display",
      "position",
    ]) {
      if (computed[property] === undefined) computed[property] = style.getPropertyValue(property);
    }
  }
  return {
    target: elementTarget(element),
    selector: stableDomSelector(element),
    page: elementPage(),
    surface: nearestSurface(element),
    tag: element.tagName.toLowerCase(),
    role: element.getAttribute("role"),
    text: (element.textContent ?? "").replace(/\s+/g, " ").trim().slice(0, 160),
    classes: Array.from(element.classList),
    bounds: {
      x: Math.round(rect.x * 100) / 100,
      y: Math.round(rect.y * 100) / 100,
      width: Math.round(rect.width * 100) / 100,
      height: Math.round(rect.height * 100) / 100,
    },
    computed,
  };
}

export function inspectDomTargets(options: DomInspectionOptions = {}): DomInspectionResult {
  if (typeof document === "undefined") {
    return { page: "", platform: "web", count: 0, targets: [] };
  }
  if (options.scope?.platform && options.scope.platform !== elementPlatform()) {
    return { page: elementPage(), platform: elementPlatform(), count: 0, targets: [] };
  }
  if (options.scope?.page && !stylePageMatches(options.scope.page, elementPage())) {
    return { page: elementPage(), platform: elementPlatform(), count: 0, targets: [] };
  }
  const limit = Math.min(MAX_SCAN_ELEMENTS, Math.max(1, Math.floor(Number(options.limit ?? 240) || 240)));
  let elements: Element[] = [];
  if (options.target) {
    const selector = targetSelector(options.target);
    if (selector) {
      try {
        elements = Array.from(document.querySelectorAll(selector));
      } catch (error) {
  console.error("[original-error]", error);
elements = [];
      }
    }
  } else {
    elements = [document.body, ...Array.from(document.body?.querySelectorAll("*") ?? [])];
  }
  const targets: DomTargetInspection[] = [];
  for (const element of elements) {
    if (!element || !isElementVisible(element) || !selectorMatchesScope(element, options.scope)) continue;
    targets.push(inspectElement(element, options.includeComputed !== false));
    if (targets.length >= limit) break;
  }
  return {
    page: elementPage(),
    platform: elementPlatform(),
    count: targets.length,
    targets,
  };
}

export function applyStyleRulePreview(rule: StyleRule): string {
  return expandedRules(rule).map(buildRuleCss).filter(Boolean).join("\n");
}

export function resolveStyleTargetSelector(target: string): string | null {
  return targetSelector(target);
}

export function isSafeStyleProperty(property: string): boolean {
  return cssPropertyName(property) !== null;
}

export function isSafeStyleValue(value: unknown, property: string): boolean {
  const cssProperty = cssPropertyName(property);
  return !!cssProperty && serializeValue(value, cssProperty) !== null;
}
