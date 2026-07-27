export type SchemaUINodeType =
  | "page"
  | "section"
  | "stack"
  | "row"
  | "grid"
  | "tabs"
  | "card"
  | "text"
  | "markdown"
  | "badge"
  | "divider"
  | "icon"
  | "image"
  | "field"
  | "select"
  | "switch"
  | "slider"
  | "button"
  | "button_group"
  | "list"
  | "table"
  | "empty_state"
  | "alert"
  | "progress"
  | "code"
  | "key_value"
  | "resource_link"
  | "permission_summary"
  | "runtime_status"
  | "tab_item"
  | "column"
  | "input";

export interface UICondition {
  field: string;
  operator: "==" | "!=" | "in" | "not_null" | "is_null" | string;
  value?: unknown;
}

export interface SchemaUIBinding {
  path: string;
  source: string;
  format?: string;
  default?: unknown;
}

export interface SchemaUIActionBinding {
  action_id: string;
  target?: string;
  target_type?: string;
  input?: Record<string, unknown>;
  confirmation?: string;
}

export interface SchemaUINode {
  id: string;
  type: string;
  props?: Record<string, unknown>;
  bindings?: SchemaUIBinding[];
  actions?: SchemaUIActionBinding[];
  visibility?: UICondition[];
  visibleWhen?: UICondition[];
  disabledWhen?: UICondition[];
  dataSource?: SchemaUIBinding;
  children?: SchemaUINode[];
}

export type UITheme = "light" | "dark" | "auto";

export interface ThemeConfig {
  mode: UITheme;
  overrides?: Record<string, string>;
}

export interface LocaleConfig {
  current: string;
  available?: string[];
}

export interface AccessibilityConfig {
  enabled: boolean;
  highContrast?: boolean;
  reducedMotion?: boolean;
  screenReader?: boolean;
  keyboardNav?: boolean;
}

export interface PerformanceBudget {
  maxRenderTimeMs: number;
  maxLayoutCount: number;
  maxNodeCount: number;
  maxDataFetchCount: number;
  maxActionCount: number;
}

export interface SchemaUIDocument {
  schemaVersion?: string;
  version?: string;
  type?: string;
  title?: string;
  root?: SchemaUINode;
  children?: SchemaUINode[];
  dataSources?: unknown[];
  actions?: Array<{ actionId: string; target?: string; inputSchema?: unknown }>;
  theme?: ThemeConfig;
  locale?: LocaleConfig;
  accessibility?: AccessibilityConfig;
  performanceBudget?: PerformanceBudget;
}

export const SCHEMA_UI_VERSION = "schema-ui/1";
export const SCHEMA_UI_MAX_DEPTH = 12;
export const SCHEMA_UI_MAX_NODES = 500;

export const ALLOWED_NODE_TYPES = new Set<string>([
  "page",
  "section",
  "stack",
  "row",
  "grid",
  "tabs",
  "card",
  "text",
  "markdown",
  "badge",
  "divider",
  "icon",
  "image",
  "field",
  "select",
  "switch",
  "slider",
  "button",
  "button_group",
  "list",
  "table",
  "empty_state",
  "alert",
  "progress",
  "code",
  "key_value",
  "resource_link",
  "permission_summary",
  "runtime_status",
  "tab_item",
  "column",
  "input",
]);

export const FORBIDDEN_NODE_TYPES = new Set<string>([
  "html",
  "script",
  "style",
  "iframe",
  "webview",
  "canvas",
  "template",
]);

export function isAllowedNodeType(t: string): boolean {
  return ALLOWED_NODE_TYPES.has(t) && !FORBIDDEN_NODE_TYPES.has(t);
}

export function lookupPath(data: unknown, path: string): unknown {
  if (!path || data == null) return undefined;
  const parts = path.split(".");
  let current: unknown = data;
  for (const p of parts) {
    if (current == null) return undefined;
    if (Array.isArray(current)) {
      const idx = Number(p);
      if (!Number.isFinite(idx)) return undefined;
      current = current[idx];
    } else if (typeof current === "object") {
      current = (current as Record<string, unknown>)[p];
    } else {
      return undefined;
    }
  }
  return current;
}

export function setPath(data: Record<string, unknown>, path: string, value: unknown): void {
  if (!path) return;
  const parts = path.split(".");
  let current: Record<string, unknown> = data;
  for (let i = 0; i < parts.length - 1; i++) {
    const p = parts[i];
    const next = current[p];
    if (next == null || typeof next !== "object" || Array.isArray(next)) {
      const newObj: Record<string, unknown> = {};
      current[p] = newObj;
      current = newObj;
    } else {
      current = next as Record<string, unknown>;
    }
  }
  current[parts[parts.length - 1]] = value;
}

export function evaluateCondition(value: unknown, op: string, expected: unknown): boolean {
  switch (op) {
    case "==":
    case "eq":
      return value === expected;
    case "!=":
    case "ne":
      return value !== expected;
    case "in": {
      if (Array.isArray(expected)) return expected.includes(value);
      if (typeof expected === "string" && typeof value === "string") {
        return expected.split(",").map((s) => s.trim()).includes(value);
      }
      return false;
    }
    case "not_null":
      return value !== null && value !== undefined;
    case "is_null":
      return value === null || value === undefined;
    default:
      return false;
  }
}

export function evaluateVisibility(
  conditions: UICondition[] | undefined,
  context: Record<string, unknown>
): boolean {
  if (!conditions || conditions.length === 0) return true;
  for (const c of conditions) {
    const val = lookupPath(context, c.field);
    if (!evaluateCondition(val, c.operator, c.value)) return false;
  }
  return true;
}

export function resolveBinding(
  binding: SchemaUIBinding | undefined,
  formState: Record<string, unknown>,
  context: Record<string, unknown>
): unknown {
  if (!binding) return undefined;
  const { source, path, default: def } = binding;
  let resolved: unknown;
  switch (source) {
    case "static":
      resolved = def;
      break;
    case "form_state":
    case "form":
      resolved = lookupPath(formState, path);
      break;
    case "state":
    case "query":
    case "input":
    case "runtime":
    case "host":
      resolved = lookupPath(context, path);
      break;
    case "storage": {
      try {
        const raw = window.localStorage.getItem(path);
        resolved = raw == null ? undefined : safeJsonParse(raw);
      } catch {
        resolved = undefined;
      }
      break;
    }
    case "runtime_status":
      resolved = lookupPath(
        (context as Record<string, unknown>).runtimeStatus ??
          (context as Record<string, unknown>).runtime_status,
        path
      );
      break;
    case "resource_list":
      resolved = lookupPath(
        (context as Record<string, unknown>).resourceList ??
          (context as Record<string, unknown>).resource_list,
        path
      );
      break;
    default:
      resolved = lookupPath(context, path);
  }
  if (resolved === undefined && def !== undefined) return def;
  return resolved;
}

function safeJsonParse(raw: string): unknown {
  try {
    return JSON.parse(raw);
  } catch {
    return raw;
  }
}

export function getFormBinding(node: SchemaUINode): SchemaUIBinding | undefined {
  if (node.dataSource) return node.dataSource;
  if (node.bindings && node.bindings.length > 0) {
    const formBinding = node.bindings.find(
      (b) => b.source === "form_state" || b.source === "form"
    );
    if (formBinding) return formBinding;
    return node.bindings[0];
  }
  return undefined;
}

export function countNodes(node: SchemaUINode): number {
  let count = 1;
  if (node.children) {
    for (const c of node.children) count += countNodes(c);
  }
  return count;
}

export function maxDepth(node: SchemaUINode, depth = 1): number {
  if (!node.children || node.children.length === 0) return depth;
  let m = depth;
  for (const c of node.children) {
    m = Math.max(m, maxDepth(c, depth + 1));
  }
  return m;
}

export function validateDocument(doc: SchemaUIDocument | null | undefined): {
  valid: boolean;
  nodeCount: number;
  depth: number;
  errors: string[];
} {
  const errors: string[] = [];
  if (!doc) {
    return { valid: false, nodeCount: 0, depth: 0, errors: ["schema 为空"] };
  }
  const version = doc.schemaVersion ?? doc.version;
  if (version && version !== SCHEMA_UI_VERSION) {
    errors.push(`schema 版本不匹配: 期望 ${SCHEMA_UI_VERSION}, 实际 ${version}`);
  }
  const roots: SchemaUINode[] = doc.root ? [doc.root] : doc.children ?? [];
  if (roots.length === 0) {
    errors.push("schema 没有根节点");
    return { valid: false, nodeCount: 0, depth: 0, errors };
  }
  let total = 0;
  let depth = 0;
  for (const r of roots) {
    total += countNodes(r);
    depth = Math.max(depth, maxDepth(r));
  }
  if (total > SCHEMA_UI_MAX_NODES) {
    errors.push(`节点数量超限: ${total} > ${SCHEMA_UI_MAX_NODES}`);
  }
  if (depth > SCHEMA_UI_MAX_DEPTH) {
    errors.push(`递归深度超限: ${depth} > ${SCHEMA_UI_MAX_DEPTH}`);
  }
  return { valid: errors.length === 0, nodeCount: total, depth, errors };
}

const HTML_ENTITY_MAP: Record<string, string> = {
  "&": "&amp;",
  "<": "&lt;",
  ">": "&gt;",
  '"': "&quot;",
  "'": "&#39;",
};

export function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (ch) => HTML_ENTITY_MAP[ch] ?? ch);
}

export function sanitizeHtml(html: string): string {
  let out = html.replace(
    /<\s*(script|style|iframe|webview|canvas|template|html|head|body|object|embed|link|meta|base|form|input|button|textarea|select|option|applet|frame|frameset)([^>]*)>([\s\S]*?)<\/\s*\1\s*>/gi,
    ""
  );
  out = out.replace(
    /<\s*(script|style|iframe|webview|canvas|template|link|meta|base|object|embed|input|button|textarea|select|option|applet|frame|frameset|img|svg|math)([^>]*?)\/?>/gi,
    (match) => {
      if (/^<\s*(img|svg)\b/i.test(match)) return match;
      return "";
    }
  );
  out = out.replace(/\s+on[a-zA-Z]+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)/gi, "");
  out = out.replace(/(href|src|action|formaction|xlink:href)\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)/gi, (match, _attr, val) => {
    const stripped = String(val).replace(/^["']|["']$/g, "");
    if (/^\s*javascript:/i.test(stripped)) return "";
    if (/^\s*data:text\/html/i.test(stripped)) return "";
    if (/^\s*vbscript:/i.test(stripped)) return "";
    return match;
  });
  return out;
}

function inlineFmt(s: string): string {
  let r = s;
  r = r.replace(/`([^`]+)`/g, (_m, c) => `<code>${c}</code>`);
  r = r.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
  r = r.replace(/__([^_]+)__/g, "<strong>$1</strong>");
  r = r.replace(/\*([^*]+)\*/g, "<em>$1</em>");
  r = r.replace(/_([^_]+)_/g, "<em>$1</em>");
  r = r.replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_m, text, url) => {
    if (/^https?:\/\//i.test(String(url))) {
      return `<a href="${String(url)}" target="_blank" rel="noopener noreferrer">${text}</a>`;
    }
    return text;
  });
  r = r.replace(/\b(https?:\/\/[^\s<]+)/g, (url) => {
    return `<a href="${url}" target="_blank" rel="noopener noreferrer">${url}</a>`;
  });
  return r;
}

export function markdownToHtml(md: string): string {
  const source = typeof md === "string" ? md : "";
  const lines = escapeHtml(source).split(/\r?\n/);
  const out: string[] = [];
  let inCode = false;
  let codeBuf: string[] = [];
  let inUl = false;
  let inOl = false;
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    if (/^\s*```/.test(line)) {
      if (inCode) {
        out.push(`<pre><code>${codeBuf.join("\n")}</code></pre>`);
        codeBuf = [];
        inCode = false;
      } else {
        if (inUl) { out.push("</ul>"); inUl = false; }
        if (inOl) { out.push("</ol>"); inOl = false; }
        inCode = true;
      }
      continue;
    }
    if (inCode) {
      codeBuf.push(line);
      continue;
    }
    const h = line.match(/^(#{1,6})\s+(.*)$/);
    if (h) {
      if (inUl) { out.push("</ul>"); inUl = false; }
      if (inOl) { out.push("</ol>"); inOl = false; }
      const level = h[1].length;
      out.push(`<h${level}>${inlineFmt(h[2])}</h${level}>`);
      continue;
    }
    const ul = line.match(/^[-*+]\s+(.*)$/);
    if (ul) {
      if (inOl) { out.push("</ol>"); inOl = false; }
      if (!inUl) { out.push("<ul>"); inUl = true; }
      out.push(`<li>${inlineFmt(ul[1])}</li>`);
      continue;
    }
    const ol = line.match(/^\d+\.\s+(.*)$/);
    if (ol) {
      if (inUl) { out.push("</ul>"); inUl = false; }
      if (!inOl) { out.push("<ol>"); inOl = true; }
      out.push(`<li>${inlineFmt(ol[1])}</li>`);
      continue;
    }
    if (line.trim() === "") {
      if (inUl) { out.push("</ul>"); inUl = false; }
      if (inOl) { out.push("</ol>"); inOl = false; }
      continue;
    }
    const bq = line.match(/^>\s?(.*)$/);
    if (bq) {
      if (inUl) { out.push("</ul>"); inUl = false; }
      if (inOl) { out.push("</ol>"); inOl = false; }
      out.push(`<blockquote>${inlineFmt(bq[1])}</blockquote>`);
      continue;
    }
    if (/^(-{3,}|\*{3,}|_{3,})$/.test(line.trim())) {
      if (inUl) { out.push("</ul>"); inUl = false; }
      if (inOl) { out.push("</ol>"); inOl = false; }
      out.push("<hr/>");
      continue;
    }
    if (inUl || inOl) {
      if (inUl) { out.push("</ul>"); inUl = false; }
      if (inOl) { out.push("</ol>"); inOl = false; }
    }
    out.push(`<p>${inlineFmt(line)}</p>`);
  }
  if (inCode) out.push(`<pre><code>${codeBuf.join("\n")}</code></pre>`);
  if (inUl) out.push("</ul>");
  if (inOl) out.push("</ol>");
  return sanitizeHtml(out.join("\n"));
}

export function toText(v: unknown): string {
  if (v == null) return "";
  if (typeof v === "string") return v;
  if (typeof v === "number" || typeof v === "boolean") return String(v);
  try {
    return JSON.stringify(v);
  } catch {
    return String(v);
  }
}

export function toNumber(v: unknown, fallback = 0): number {
  const n = typeof v === "number" ? v : Number(v);
  return Number.isFinite(n) ? n : fallback;
}

export function toStringArray(v: unknown): string[] {
  if (Array.isArray(v)) return v.map((x) => toText(x));
  if (typeof v === "string") return v.split(",").map((s) => s.trim()).filter(Boolean);
  return [];
}

export function toKeyValueItems(v: unknown): Array<{ key: string; value: string }> {
  if (Array.isArray(v)) {
    return v.map((item) => {
      if (item && typeof item === "object") {
        const obj = item as Record<string, unknown>;
        return {
          key: toText(obj.key ?? obj.label ?? obj.name),
          value: toText(obj.value ?? obj.content ?? ""),
        };
      }
      return { key: toText(item), value: "" };
    });
  }
  if (v && typeof v === "object") {
    return Object.entries(v as Record<string, unknown>).map(([key, value]) => ({
      key,
      value: toText(value),
    }));
  }
  return [];
}
