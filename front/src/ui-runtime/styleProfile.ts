import { ref } from "vue";
import { normalizeStylePageId } from "./stylePage";

const PATCH_KEY = "amitia.ui.style.patch.v1";
const REVISION_KEY = "amitia.ui.style.revision.v1";
const HISTORY_KEY = "amitia.ui.style.history.v1";
const DOCUMENT_KEY = "amitia.ui.style.document.v1";
const MAX_HISTORY = 30;
const MAX_TRANSACTIONS = 100;

export type StylePlatform = "desktop" | "mobile" | "web";
export type StyleSource = "ai" | "user";

export interface StyleScope {
  platform?: StylePlatform;
  page?: string;
  surface?: string;
}

export interface StyleRule {
  id: string;
  source: StyleSource;
  scope: StyleScope;
  target: string;
  states: string[];
  properties: Record<string, unknown>;
  transactionId?: string;
  createdAt: string;
}

export interface StyleTransaction {
  id: string;
  source: StyleSource;
  ruleIds: string[];
  changeKeys: string[];
  revision: number;
  createdAt: string;
}

export interface StyleDocument {
  version: 1;
  patch: Record<string, unknown>;
  rules: StyleRule[];
  transactions: StyleTransaction[];
  revision: number;
}

export interface StyleRuleFilters {
  source?: StyleSource;
  scope?: StyleScope;
  target?: string;
  states?: string[];
  properties?: string[];
  transactionId?: string;
  ruleIds?: string[];
}

export type StylePatch = Record<string, unknown>;

export const stylePatch = ref<StylePatch>({});
export const styleRevision = ref(0);
export const styleRules = ref<StyleRule[]>([]);
export const styleTransactions = ref<StyleTransaction[]>([]);

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === "object" && !Array.isArray(value);
}

function readJSON<T>(key: string, fallback: T): T {
  if (typeof localStorage === "undefined") return fallback;
  try {
    const raw = localStorage.getItem(key);
    return raw ? JSON.parse(raw) as T : fallback;
  } catch (error) {
  console.error("[original-error]", error);
return fallback;
  }
}

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

function normalizeStates(value: unknown): string[] {
  if (!Array.isArray(value)) return ["default"];
  const states = value
    .map((item) => String(item ?? "").trim().toLowerCase())
    .filter(Boolean);
  return states.length > 0 ? Array.from(new Set(states)) : ["default"];
}

function normalizeScope(value: unknown): StyleScope {
  if (!isRecord(value)) return {};
  const platform = String(value.platform ?? "").trim().toLowerCase();
  const scope: StyleScope = {};
  if (platform === "desktop" || platform === "mobile" || platform === "web") {
    scope.platform = platform;
  }
  const page = String(value.page ?? "").trim();
  if (page) scope.page = normalizeStylePageId(page);
  const surface = String(value.surface ?? "").trim();
  if (surface) scope.surface = surface;
  return scope;
}

function normalizeRule(value: unknown): StyleRule | null {
  if (!isRecord(value)) return null;
  const target = String(value.target ?? value.selector ?? value.component ?? "").trim();
  if (!target) return null;
  const properties = isRecord(value.properties)
    ? value.properties
    : isRecord(value.set)
      ? value.set
      : isRecord(value.styles)
        ? value.styles
        : {};
  if (Object.keys(properties).length === 0) return null;
  const source = value.source === "user" ? "user" : "ai";
  const createdAt = typeof value.createdAt === "string" && value.createdAt
    ? value.createdAt
    : new Date().toISOString();
  return {
    id: String(value.id ?? `rule_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 10)}`),
    source,
    scope: normalizeScope(value.scope ?? {
      platform: value.platform,
      page: value.page,
      surface: value.surface,
    }),
    target,
    states: normalizeStates(value.states ?? value.state),
    properties: clone(properties),
    transactionId: typeof value.transactionId === "string" && value.transactionId
      ? value.transactionId
      : undefined,
    createdAt,
  };
}

function normalizeTransaction(value: unknown): StyleTransaction | null {
  if (!isRecord(value)) return null;
  const id = String(value.id ?? "").trim();
  if (!id) return null;
  const ruleIds = Array.isArray(value.ruleIds)
    ? value.ruleIds.map((item) => String(item ?? "").trim()).filter(Boolean)
    : [];
  const changeKeys = Array.isArray(value.changeKeys)
    ? value.changeKeys.map((item) => String(item ?? "").trim()).filter(Boolean)
    : [];
  return {
    id,
    source: value.source === "user" ? "user" : "ai",
    ruleIds,
    changeKeys,
    revision: Math.max(0, Number(value.revision ?? 0) || 0),
    createdAt: typeof value.createdAt === "string" && value.createdAt
      ? value.createdAt
      : new Date().toISOString(),
  };
}

function readLegacyPatch(): StylePatch {
  const value = readJSON<unknown>(PATCH_KEY, {});
  return isRecord(value) ? clone(value) : {};
}

function readLegacyRevision(): number {
  if (typeof localStorage === "undefined") return 0;
  return Math.max(0, Number(localStorage.getItem(REVISION_KEY) ?? 0) || 0);
}

function persistStyleDocument() {
  if (typeof localStorage === "undefined") return;
  const documentValue: StyleDocument = {
    version: 1,
    patch: clone(stylePatch.value),
    rules: clone(styleRules.value),
    transactions: clone(styleTransactions.value),
    revision: styleRevision.value,
  };
  localStorage.setItem(DOCUMENT_KEY, JSON.stringify(documentValue));
  localStorage.setItem(PATCH_KEY, JSON.stringify(stylePatch.value));
  localStorage.setItem(REVISION_KEY, String(styleRevision.value));
}

export function initializeStyleProfile() {
  const stored = readJSON<unknown>(DOCUMENT_KEY, null);
  if (isRecord(stored)) {
    stylePatch.value = isRecord(stored.patch) ? clone(stored.patch) : readLegacyPatch();
    styleRules.value = Array.isArray(stored.rules)
      ? stored.rules.map(normalizeRule).filter((rule): rule is StyleRule => !!rule)
      : [];
    styleTransactions.value = Array.isArray(stored.transactions)
      ? stored.transactions.map(normalizeTransaction).filter((item): item is StyleTransaction => !!item)
      : [];
    styleRevision.value = Math.max(0, Number(stored.revision ?? readLegacyRevision()) || 0);
    persistStyleDocument();
    return;
  }
  stylePatch.value = readLegacyPatch();
  styleRules.value = [];
  styleTransactions.value = [];
  styleRevision.value = readLegacyRevision();
  persistStyleDocument();
}

export function replaceStylePatch(patch: StylePatch, revision: number) {
  stylePatch.value = clone(patch);
  styleRevision.value = Math.max(0, revision);
  persistStyleDocument();
}

export function clearStylePatch(revision: number) {
  replaceStylePatch({}, revision);
}

export function replaceStyleDocument(documentValue: StyleDocument) {
  stylePatch.value = isRecord(documentValue.patch) ? clone(documentValue.patch) : {};
  styleRules.value = Array.isArray(documentValue.rules)
    ? documentValue.rules.map(normalizeRule).filter((rule): rule is StyleRule => !!rule)
    : [];
  styleTransactions.value = Array.isArray(documentValue.transactions)
    ? documentValue.transactions.map(normalizeTransaction).filter((item): item is StyleTransaction => !!item)
    : [];
  styleRevision.value = Math.max(0, Number(documentValue.revision ?? 0) || 0);
  persistStyleDocument();
}

export function captureStyleDocument(): StyleDocument {
  return {
    version: 1,
    patch: clone(stylePatch.value),
    rules: clone(styleRules.value),
    transactions: clone(styleTransactions.value),
    revision: styleRevision.value,
  };
}

export function createStyleRule(value: unknown): StyleRule | null {
  return normalizeRule(value);
}

export function addStyleRules(values: unknown[]): StyleRule[] {
  const rules = values
    .map(normalizeRule)
    .filter((rule): rule is StyleRule => !!rule);
  if (rules.length === 0) return [];
  styleRules.value = [...styleRules.value, ...rules];
  persistStyleDocument();
  return rules;
}

export function recordStyleTransaction(value: StyleTransaction) {
  const transaction = normalizeTransaction(value);
  if (!transaction) return;
  styleTransactions.value = [...styleTransactions.value, transaction].slice(-MAX_TRANSACTIONS);
  persistStyleDocument();
}

export function popStyleTransaction(): StyleTransaction | null {
  const transactions = [...styleTransactions.value];
  const transaction = transactions.pop() ?? null;
  styleTransactions.value = transactions;
  persistStyleDocument();
  return transaction ? clone(transaction) : null;
}

function scopeMatches(ruleScope: StyleScope, filter: StyleScope): boolean {
  if (filter.platform && ruleScope.platform !== filter.platform) return false;
  if (filter.page && ruleScope.page !== filter.page) return false;
  if (filter.surface && ruleScope.surface !== filter.surface) return false;
  return true;
}

export function removeStyleRules(filters: StyleRuleFilters): { removedRules: number; changedRules: number; removedProperties: string[] } {
  const rules: StyleRule[] = [];
  let removedRules = 0;
  let changedRules = 0;
  const removedProperties = new Set<string>();
  const stateFilter = filters.states && filters.states.length > 0
    ? new Set(filters.states.map((item) => String(item).trim().toLowerCase()).filter(Boolean))
    : null;
  const propertyFilter = filters.properties && filters.properties.length > 0
    ? new Set(filters.properties.map((item) => String(item).trim()).filter(Boolean))
    : null;
  const ruleIdFilter = filters.ruleIds && filters.ruleIds.length > 0
    ? new Set(filters.ruleIds)
    : null;

  for (const rule of styleRules.value) {
    if (ruleIdFilter && !ruleIdFilter.has(rule.id)) {
      rules.push(rule);
      continue;
    }
    if (filters.source && rule.source !== filters.source) {
      rules.push(rule);
      continue;
    }
    if (filters.transactionId && rule.transactionId !== filters.transactionId) {
      rules.push(rule);
      continue;
    }
    if (filters.scope && !scopeMatches(rule.scope, filters.scope)) {
      rules.push(rule);
      continue;
    }
    if (filters.target && rule.target !== filters.target) {
      rules.push(rule);
      continue;
    }
    const stateMatch = !stateFilter || rule.states.some((state) => stateFilter.has(state));
    const propertyMatch = !propertyFilter || Object.keys(rule.properties).some((property) => propertyFilter.has(property));
    if (!stateMatch || !propertyMatch) {
      rules.push(rule);
      continue;
    }
    if (!stateFilter && !propertyFilter) {
      removedRules += 1;
      continue;
    }
    const nextStates = stateFilter
      ? rule.states.filter((state) => !stateFilter.has(state))
      : [...rule.states];
    const nextProperties = { ...rule.properties };
    if (propertyFilter) {
      for (const property of propertyFilter) {
        if (Object.prototype.hasOwnProperty.call(nextProperties, property)) {
          delete nextProperties[property];
          removedProperties.add(property);
        }
      }
    }
    if (nextStates.length === 0 || Object.keys(nextProperties).length === 0) {
      removedRules += 1;
      continue;
    }
    changedRules += 1;
    rules.push({ ...rule, states: nextStates, properties: nextProperties });
  }
  styleRules.value = rules;
  persistStyleDocument();
  return { removedRules, changedRules, removedProperties: Array.from(removedProperties) };
}

export function listStyleRules(filters: StyleRuleFilters = {}): StyleRule[] {
  const ruleIdFilter = filters.ruleIds && filters.ruleIds.length > 0
    ? new Set(filters.ruleIds)
    : null;
  const stateFilter = filters.states && filters.states.length > 0
    ? new Set(filters.states.map((item) => String(item).trim().toLowerCase()).filter(Boolean))
    : null;
  const propertyFilter = filters.properties && filters.properties.length > 0
    ? new Set(filters.properties.map((item) => String(item).trim()).filter(Boolean))
    : null;
  return styleRules.value.filter((rule) => {
    if (ruleIdFilter && !ruleIdFilter.has(rule.id)) return false;
    if (filters.source && rule.source !== filters.source) return false;
    if (filters.transactionId && rule.transactionId !== filters.transactionId) return false;
    if (filters.scope && !scopeMatches(rule.scope, filters.scope)) return false;
    if (filters.target && rule.target !== filters.target) return false;
    if (stateFilter && !rule.states.some((state) => stateFilter.has(state))) return false;
    if (propertyFilter && !Object.keys(rule.properties).some((property) => propertyFilter.has(property))) return false;
    return true;
  }).map((rule) => clone(rule));
}

export function loadStyleHistory(): Array<Record<string, unknown>> {
  const history = readJSON<unknown>(HISTORY_KEY, []);
  return Array.isArray(history)
    ? history.filter(isRecord)
    : [];
}

export function pushStyleHistory(snapshot: Record<string, unknown>) {
  const history = loadStyleHistory();
  history.push(snapshot);
  while (history.length > MAX_HISTORY) history.shift();
  if (typeof localStorage !== "undefined") {
    localStorage.setItem(HISTORY_KEY, JSON.stringify(history));
  }
}

export function popStyleHistory(): Record<string, unknown> | null {
  const history = loadStyleHistory();
  const snapshot = history.pop() ?? null;
  if (typeof localStorage !== "undefined") {
    localStorage.setItem(HISTORY_KEY, JSON.stringify(history));
  }
  return snapshot;
}
