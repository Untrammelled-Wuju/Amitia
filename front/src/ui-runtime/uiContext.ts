import { canonicalUISurfaceId, normalizeUIRoute } from "./uiSurfaceCatalog";

export type UIPluginSurfaceRole = "header" | "status" | "sidebar" | "message" | "composer" | "main" | "overlay";

export interface UIPluginSurfaceContext {
  role: UIPluginSurfaceRole;
  width: number;
  height: number;
  breakpoint: "xs" | "sm" | "md" | "lg" | "xl";
}

export interface UIPluginContext {
  schemaVersion: 1;
  slotId: string;
  contractVersion: number;
  route: string;
  surfaceId: string;
  platform: "windows" | "macos" | "linux" | "web";
  theme: { mode: "light" | "dark"; density: "comfortable" | "compact" };
  locale: string;
  data: Record<string, unknown>;
  host: "web" | "desktop";
  os: "windows" | "macos" | "linux" | "unknown";
  surface: UIPluginSurfaceContext;
  capabilities: string[];
  permissions: string[];
  scope: "root" | "session-maybe" | "session";
  sessionId?: string;
  conversationId?: string;
  characterId?: string;
  messageId?: string;
  actions?: Readonly<Record<string, (...args: any[]) => unknown>>;
  navigate?: (target: string, type?: string) => unknown;
}

export interface UIPluginContextBuildInput {
  slotId: string;
  contractVersion: number;
  route?: string;
  platform: UIPluginContext["platform"];
  theme: UIPluginContext["theme"];
  locale: string;
  data: Record<string, unknown>;
  host: UIPluginContext["host"];
  os: UIPluginContext["os"];
  surface: UIPluginSurfaceContext;
  capabilities?: readonly unknown[];
  permissions?: readonly unknown[];
  scope: UIPluginContext["scope"];
  actions?: UIPluginContext["actions"];
  navigate?: UIPluginContext["navigate"];
}

export interface UIPluginContextSource {
  getSnapshot(): UIPluginContext;
  subscribe(listener: () => void): () => void;
}

export interface UIPluginContextSelection<T> {
  getSnapshot(): T;
  subscribe(listener: () => void): () => void;
}

export interface MutableUIPluginContextSource extends UIPluginContextSource {
  setContext(context: UIPluginContext): void;
  dispose(): void;
}

function stringList(values: readonly unknown[] | undefined): string[] {
  if (!values) return [];
  return Array.from(new Set(values.filter((value): value is string => typeof value === "string").map((value) => value.trim()).filter(Boolean)));
}

function readString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() ? value.trim() : undefined;
}

function readConversationId(data: Record<string, unknown>): string | undefined {
  const direct = readString(data.conversationId);
  if (direct) return direct;
  const conversation = data.conversation;
  if (!conversation || typeof conversation !== "object") return undefined;
  return readString((conversation as Record<string, unknown>).id);
}

function detectRoute(route?: string): string {
  if (route?.trim()) return normalizeUIRoute(route);
  if (typeof globalThis.location?.pathname === "string") return normalizeUIRoute(globalThis.location.pathname);
  return "/";
}

export function buildUIPluginContext(input: UIPluginContextBuildInput): UIPluginContext {
  const route = detectRoute(input.route ?? readString(input.data.route));
  const conversationId = readConversationId(input.data);
  const sessionId = readString(input.data.sessionId) ?? conversationId;
  const characterId = readString(input.data.characterId) ?? readString(input.data.character);
  const messageId = readString(input.data.messageId);
  return {
    schemaVersion: 1,
    slotId: input.slotId,
    contractVersion: input.contractVersion,
    route,
    surfaceId: canonicalUISurfaceId(route),
    platform: input.platform,
    theme: { ...input.theme },
    locale: input.locale,
    data: { ...input.data },
    host: input.host,
    os: input.os,
    surface: { ...input.surface },
    capabilities: stringList(input.capabilities),
    permissions: stringList(input.permissions),
    scope: input.scope,
    ...(sessionId ? { sessionId } : {}),
    ...(conversationId ? { conversationId } : {}),
    ...(characterId ? { characterId } : {}),
    ...(messageId ? { messageId } : {}),
    ...(input.actions ? { actions: input.actions } : {}),
    ...(input.navigate ? { navigate: input.navigate } : {}),
  };
}

export function createUIPluginContextSource(initial: UIPluginContext): MutableUIPluginContextSource {
  let snapshot = initial;
  let active = true;
  const listeners = new Set<() => void>();
  return {
    getSnapshot: () => snapshot,
    subscribe(listener: () => void) {
      if (!active) return () => undefined;
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    setContext(context: UIPluginContext) {
      if (!active) throw new Error("UI plugin context source is disposed");
      snapshot = context;
      for (const listener of Array.from(listeners)) listener();
    },
    dispose() {
      active = false;
      listeners.clear();
    },
  };
}

export function selectUIPluginContext<T>(
  source: UIPluginContextSource,
  selector: (context: UIPluginContext) => T,
  equality: (previous: T, next: T) => boolean = Object.is,
): UIPluginContextSelection<T> {
  let selected = selector(source.getSnapshot());
  return {
    getSnapshot: () => selected,
    subscribe(listener: () => void) {
      return source.subscribe(() => {
        const next = selector(source.getSnapshot());
        if (equality(selected, next)) return;
        selected = next;
        listener();
      });
    },
  };
}
