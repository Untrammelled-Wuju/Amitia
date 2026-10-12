import type { StyleRule, StyleScope } from "./styleProfile";
import { normalizeStylePageId, stylePageMatches } from "./stylePage";

interface AmitiaPluginStyleBridgeOptions {
  rules: StyleRule[];
  scope: StyleScope;
  target: string;
  additionalTargets?: string[];
  states?: string[];
}

const DANGEROUS_STYLE_VALUE = /javascript\s*:|expression\s*\(|url\s*\(|@import|behavior\s*:|[;{}<>]/i;

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === "object" && !Array.isArray(value);
}

function cloneValue(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(cloneValue).filter((item) => item !== undefined);
  }
  if (!isRecord(value)) {
    if (typeof value === "string" && DANGEROUS_STYLE_VALUE.test(value)) return undefined;
    return value;
  }
  const result: Record<string, unknown> = {};
  for (const [key, item] of Object.entries(value)) {
    if (!/^[a-zA-Z_][a-zA-Z0-9_-]*$/.test(key) && !/^--[a-zA-Z0-9_-]+$/.test(key)) continue;
    const next = cloneValue(item);
    if (next !== undefined) result[key] = next;
  }
  return result;
}

function cloneProperties(value: Record<string, unknown>): Record<string, unknown> {
  const next = cloneValue(value);
  return isRecord(next) ? next : {};
}

function scopeValueMatches(expected: string | undefined, current: string | undefined): boolean {
  if (!expected || expected === "*" || expected === "all") return true;
  if (!current) return false;
  if (expected === current) return true;
  if (expected === "desktop" || expected === "mobile" || expected === "web") return false;
  return stylePageMatches(expected, current);
}

function scopeMatches(ruleScope: StyleScope, currentScope: StyleScope): boolean {
  if (!scopeValueMatches(ruleScope.platform, currentScope.platform)) return false;
  if (!scopeValueMatches(ruleScope.page, currentScope.page)) return false;
  if (!scopeValueMatches(ruleScope.surface, currentScope.surface)) return false;
  return true;
}

function targetMatches(ruleTarget: string, requestedTarget: string): boolean {
  if (ruleTarget === "*" || ruleTarget === requestedTarget) return true;
  if (ruleTarget.endsWith(".*")) {
    return requestedTarget.startsWith(ruleTarget.slice(0, -1));
  }
  return false;
}

function ruleScore(
  rule: StyleRule,
  currentScope: StyleScope,
  requestedTarget: string,
  requestedStates: Set<string>,
  index: number,
): number {
  let score = rule.source === "ai" ? 200 : 100;
  if (rule.scope.platform && rule.scope.platform === currentScope.platform) score += 20;
  if (rule.scope.page && currentScope.page && stylePageMatches(rule.scope.page, currentScope.page)) score += 40;
  if (rule.scope.surface && rule.scope.surface === currentScope.surface) score += 60;
  if (rule.target === requestedTarget) score += 120;
  if (rule.states.some((state) => requestedStates.has(state) && state !== "default")) score += 24;
  return score + index;
}

function mergeInto(
  target: Record<string, unknown>,
  source: Record<string, unknown>,
): void {
  for (const [key, value] of Object.entries(source)) {
    const current = target[key];
    if (isRecord(current) && isRecord(value)) {
      const merged = { ...current };
      mergeInto(merged, value);
      target[key] = merged;
    } else {
      target[key] = value;
    }
  }
}

function resolveProperties(
  options: AmitiaPluginStyleBridgeOptions,
  target: string,
): Record<string, unknown> {
  const states = new Set(["default", ...(options.states ?? [])]);
  const selected = options.rules
    .map((rule, index) => ({ rule, index }))
    .filter(({ rule }) => scopeMatches(rule.scope, options.scope))
    .filter(({ rule }) => targetMatches(rule.target, target))
    .filter(({ rule }) => rule.states.some((state) => states.has(state)))
    .sort((left, right) =>
      ruleScore(left.rule, options.scope, target, states, left.index) -
      ruleScore(right.rule, options.scope, target, states, right.index)
    );
  const properties: Record<string, unknown> = {};
  for (const { rule } of selected) {
    mergeInto(properties, cloneProperties(rule.properties));
  }
  return properties;
}

function cssVariableName(value: string): string {
  return value.replace(/([a-z0-9])([A-Z])/g, "$1-$2").replace(/_/g, "-").toLowerCase();
}

function cssVariableValue(value: unknown): string | null {
  if (typeof value === "number" && Number.isFinite(value)) return String(value);
  if (typeof value === "boolean") return value ? "1" : "0";
  if (typeof value === "string") return value;
  if (Array.isArray(value)) {
    const parts = value.map(cssVariableValue).filter((item): item is string => item !== null);
    return parts.length > 0 ? parts.join(" ") : null;
  }
  return null;
}

export function amitiaStyleVars(properties: Record<string, unknown>): Record<string, string> {
  const result: Record<string, string> = {};
  for (const [property, value] of Object.entries(properties)) {
    const serialized = cssVariableValue(value);
    if (serialized === null) continue;
    result[`--amitia-${cssVariableName(property)}`] = serialized;
  }
  return result;
}

export function amitiaStyleBridgePayload(
  options: AmitiaPluginStyleBridgeOptions,
): Record<string, unknown> {
  const states = Array.from(new Set(["default", ...(options.states ?? [])]));
  const targetNames = Array.from(new Set([options.target, ...(options.additionalTargets ?? [])]));
  const targetPayloads: Record<string, unknown> = {};
  for (const target of targetNames) {
    const properties = resolveProperties({ ...options, states }, target);
    targetPayloads[target] = {
      target,
      properties,
      vars: amitiaStyleVars(properties),
      warnings: [],
    };
  }
  const primary = targetPayloads[options.target] as Record<string, unknown>;
  const bridgeRules = options.rules
    .filter((rule) => scopeMatches(rule.scope, options.scope))
    .filter((rule) => {
      const target = rule.target.trim().toLowerCase();
      return target === "*" ||
        target.startsWith("extension.") ||
        target.startsWith("plugin.") ||
        target.startsWith("component.") ||
        target.startsWith("page.");
    })
    .slice(0, 80)
    .map((rule) => ({
      id: rule.id,
      source: rule.source,
      scope: { ...rule.scope },
      target: rule.target,
      states: [...rule.states],
      properties: cloneProperties(rule.properties),
    }));
  return {
    schemaVersion: 1,
    target: options.target,
    states,
    scope: {
      ...options.scope,
      page: options.scope.page ? normalizeStylePageId(options.scope.page) : undefined,
    },
    properties: primary.properties,
    vars: primary.vars,
    warnings: primary.warnings,
    targets: targetPayloads,
    rules: bridgeRules,
  };
}
