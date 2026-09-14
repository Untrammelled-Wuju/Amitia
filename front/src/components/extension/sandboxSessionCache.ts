import { apiClient } from "@/composables/useApi";
import { resolveHostEnvironment } from "@/composables/useHostEnvironment";
import type { UIContributionSummary } from "@/stores/extensionUI";

export interface SandboxSessionRecord {
  sessionId: string;
  nonce: string;
  token: string;
  origin: string;
  csp: string;
  resourceUrl: string;
  capabilities: string[];
  grantedPerms: string[];
  grantedScopes: string[];
}

export interface SandboxSessionOptions {
  contribution: UIContributionSummary;
  context?: Record<string, unknown>;
  slotId: string;
}

interface CachedSandboxSession extends SandboxSessionRecord {
  expiresAt: number;
  timer: ReturnType<typeof setTimeout> | null;
}

const SESSION_CACHE_TTL = 5 * 60 * 1000;
const sandboxSessionCache = new Map<string, CachedSandboxSession>();
const sandboxSessionInflight = new Map<string, Promise<SandboxSessionRecord>>();
const sandboxSessionClaims = new Set<string>();

function surfaceRoleOf(context?: Record<string, unknown>): string {
  const surface = context?.surface as Record<string, unknown> | undefined;
  return String(surface?.role ?? "main");
}

export function buildSandboxThemeTokens(): Record<string, string> {
  if (typeof document === "undefined") {
    return {
      "--amitia-bg-surface": "transparent",
      "--amitia-text-primary": "inherit",
      "--amitia-text-secondary": "inherit",
      "--amitia-border": "transparent",
      "--amitia-control-hover": "transparent",
      "--amitia-control-active": "transparent",
      "--amitia-radius-sm": "8px",
      "--amitia-radius-lg": "12px",
      "--amitia-font-ui": "system-ui",
      "--amitia-font-size-sm": "13px",
      "--amitia-color-accent": "#c99557",
      "--amitia-color-success": "#75a184",
      "--amitia-color-warning": "#c99a56",
      "--amitia-color-danger": "#c96e6a",
    };
  }
  const cs = getComputedStyle(document.documentElement);
  return {
    "--amitia-bg-surface": cs.getPropertyValue("--amitia-bg-surface").trim() || "transparent",
    "--amitia-text-primary": cs.getPropertyValue("--amitia-text-primary").trim() || "inherit",
    "--amitia-text-secondary": cs.getPropertyValue("--amitia-text-secondary").trim() || "inherit",
    "--amitia-border": cs.getPropertyValue("--amitia-border").trim() || "transparent",
    "--amitia-control-hover": cs.getPropertyValue("--amitia-control-hover").trim() || "transparent",
    "--amitia-control-active": cs.getPropertyValue("--amitia-control-active").trim() || "transparent",
    "--amitia-radius-sm": cs.getPropertyValue("--amitia-radius-sm").trim() || "8px",
    "--amitia-radius-lg": cs.getPropertyValue("--amitia-radius-lg").trim() || "12px",
    "--amitia-font-ui": cs.getPropertyValue("--amitia-font-ui").trim() || "system-ui",
    "--amitia-font-size-sm": cs.getPropertyValue("--amitia-font-size-sm").trim() || "13px",
    "--amitia-color-accent": cs.getPropertyValue("--amitia-color-accent").trim() || "#c99557",
    "--amitia-color-success": cs.getPropertyValue("--amitia-color-success").trim() || "#75a184",
    "--amitia-color-warning": cs.getPropertyValue("--amitia-color-warning").trim() || "#c99a56",
    "--amitia-color-danger": cs.getPropertyValue("--amitia-color-danger").trim() || "#c96e6a",
  };
}

export function buildSandboxThemeSnapshot(context?: Record<string, unknown>): {
  mode: string;
  density: string;
  tokens: Record<string, string>;
} {
  const themeData = (context?.theme as Record<string, unknown> | undefined) ?? {};
  return {
    mode: String(themeData.mode || context?.hostTheme || "light"),
    density: String(themeData.density || "default"),
    tokens: buildSandboxThemeTokens(),
  };
}

export function sandboxSessionKey(options: SandboxSessionOptions): string {
  const context = options.context ?? {};
  const env = resolveHostEnvironment();
  return [
    options.contribution.contributionId,
    options.contribution.generation,
    context.characterId || "",
    context.conversationId || "",
    context.messageId || "",
    options.slotId,
    surfaceRoleOf(context),
    env.platform,
    env.host,
    env.os,
  ].join(":");
}

function revokeCachedSession(entry: CachedSandboxSession): void {
  if (entry.timer) clearTimeout(entry.timer);
  if (entry.sessionId) {
    apiClient.delete(`/api/extension/webui/session/${entry.sessionId}`).catch(() => {});
  }
}

function cacheSessionEntry(key: string, entry: CachedSandboxSession): void {
  entry.expiresAt = Date.now() + SESSION_CACHE_TTL;
  if (entry.timer) clearTimeout(entry.timer);
  entry.timer = setTimeout(() => {
    const current = sandboxSessionCache.get(key);
    if (current !== entry) return;
    sandboxSessionCache.delete(key);
    revokeCachedSession(entry);
  }, SESSION_CACHE_TTL);
}

export function takeCachedSandboxSession(key: string): SandboxSessionRecord | null {
  const entry = sandboxSessionCache.get(key);
  if (!entry) return null;
  sandboxSessionCache.delete(key);
  if (entry.timer) clearTimeout(entry.timer);
  entry.timer = null;
  if (Date.now() > entry.expiresAt) {
    revokeCachedSession(entry);
    return null;
  }
  return {
    sessionId: entry.sessionId,
    nonce: entry.nonce,
    token: entry.token,
    origin: entry.origin,
    csp: entry.csp,
    resourceUrl: entry.resourceUrl,
    capabilities: [...entry.capabilities],
    grantedPerms: [...entry.grantedPerms],
    grantedScopes: [...entry.grantedScopes],
  };
}

export function claimSandboxSession(key: string): void {
  sandboxSessionClaims.add(key);
}

export function releaseSandboxSessionClaim(key: string): void {
  sandboxSessionClaims.delete(key);
}

export function putCachedSandboxSession(key: string, record: SandboxSessionRecord): void {
  if (!record.sessionId) return;
  const previous = sandboxSessionCache.get(key);
  if (previous) {
    if (previous.sessionId === record.sessionId) {
      cacheSessionEntry(key, previous);
      return;
    }
    sandboxSessionCache.delete(key);
    revokeCachedSession(previous);
  }
  const entry: CachedSandboxSession = {
    ...record,
    capabilities: [...record.capabilities],
    grantedPerms: [...record.grantedPerms],
    grantedScopes: [...record.grantedScopes],
    expiresAt: Date.now() + SESSION_CACHE_TTL,
    timer: null,
  };
  sandboxSessionCache.set(key, entry);
  cacheSessionEntry(key, entry);
}

export function revokeSandboxSession(sessionId: string): void {
  if (!sessionId) return;
  for (const [key, entry] of sandboxSessionCache.entries()) {
    if (entry.sessionId === sessionId) {
      sandboxSessionCache.delete(key);
      if (entry.timer) clearTimeout(entry.timer);
    }
  }
  apiClient.delete(`/api/extension/webui/session/${sessionId}`).catch(() => {});
}

export async function getOrCreateSandboxSession(options: SandboxSessionOptions): Promise<SandboxSessionRecord> {
  const key = sandboxSessionKey(options);
  const cached = takeCachedSandboxSession(key);
  if (cached) return cached;
  const pending = sandboxSessionInflight.get(key);
  if (pending) return pending;

  const context = options.context ?? {};
  const theme = buildSandboxThemeSnapshot(context);
  const env = resolveHostEnvironment();
  const surfaceRole = surfaceRoleOf(context);
  const request = apiClient.post<{
    sessionId: string;
    nonce: string;
    token: string;
    origin: string;
    csp: string;
    resourceUrl?: string;
    entryUrl?: string;
    capabilities?: string[];
    grantedPerms?: string[];
    grantedScopes?: string[];
  }>("/api/extension/webui/session", {
    contributionId: options.contribution.contributionId,
    extensionId: options.contribution.extensionId,
    moduleId: options.contribution.moduleId,
    slotId: options.slotId,
    generation: options.contribution.generation,
    surface: surfaceRole,
    surfaceRole,
    host: env.host,
    os: env.os,
    platform: env.platform,
    characterId: String(context.characterId ?? ""),
    conversationId: String(context.conversationId ?? ""),
    theme,
    locale: String(context.locale ?? (typeof navigator !== "undefined" ? navigator.language : "en")),
    uiContext: context,
    sandbox: options.contribution.sandbox ?? "web_restricted",
    entryPath: options.contribution.entryPath ?? "index.html",
    allowedActions: (options.contribution.actions ?? []).map((action) => action.actionId),
  }).then((response) => {
    const data = response.data;
    if (!data?.sessionId) throw new Error("session response missing sessionId");
    return {
      sessionId: data.sessionId,
      nonce: data.nonce,
      token: data.token,
      origin: data.origin,
      csp: data.csp,
      resourceUrl: data.resourceUrl || data.entryUrl || "",
      capabilities: data.capabilities ?? [],
      grantedPerms: data.grantedPerms ?? [],
      grantedScopes: data.grantedScopes ?? [],
    } satisfies SandboxSessionRecord;
  });

  sandboxSessionInflight.set(key, request);
  try {
    return await request;
  } finally {
    if (sandboxSessionInflight.get(key) === request) {
      sandboxSessionInflight.delete(key);
    }
  }
}

export function prewarmSandboxSession(options: SandboxSessionOptions): void {
  const key = sandboxSessionKey(options);
  if (sandboxSessionClaims.has(key)) return;
  void getOrCreateSandboxSession(options)
    .then((record) => {
      if (!sandboxSessionClaims.has(key)) {
        putCachedSandboxSession(key, record);
      }
    })
    .catch(() => {});
}
