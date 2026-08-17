export interface AmitiaUIContext {
  theme: { mode: string; density: string; tokens: Record<string, string> };
  locale: string;
  platform: "windows" | "macos" | "linux" | "web";
  host: "web" | "desktop";
  os: "windows" | "macos" | "linux" | "unknown";
  surface: string;
  slotId: string;
  scope: { extensionId: string; moduleId: string };
  characterId: string;
  conversationId: string;
  capabilities: string[];
  generation: number;
}

export interface AmitiaUIReadyPayload {
  sessionId: string;
  contributionId: string;
  slotId: string;
  theme: { mode: string; density: string; tokens: Record<string, string> };
  locale: string;
  actions: string[];
  dataSources: string[];
  generation: number;
}

export interface AmitiaUI {
  ready(): Promise<void>;
  onReady(cb: () => void): void;
  getContext(): Promise<AmitiaUIContext>;
  invokeAction(actionId: string, input?: unknown): Promise<unknown>;
  queryData(sourceId: string, params?: unknown): Promise<unknown>;
  subscribeData(sourceId: string, params: unknown, rate?: number): Promise<{ subscriptionId: string; sourceId: string }>;
  navigate(target: string, type?: string): Promise<void>;
  requestResize(width: number, height: number): Promise<void>;
  openResource(handleId: string): Promise<{ handleId: string; mimeType: string; size: number }>;
  readResource(handleId: string): Promise<{ handleId: string; path: string; mimeType: string; size: number; readOnly: boolean }>;
  createArtifact(contentType: string, data: unknown, filename?: string): Promise<{ artifactId: string; contentType: string; size: number; filename: string }>;
  log(level: string, message: string): Promise<void>;
  ping(): Promise<{ ok: boolean; timestamp: string; expiresAt: string }>;
  onHostContextChange(cb: (context: AmitiaUIContext) => void): () => void;
  onThemeChange(cb: (theme: AmitiaUIContext["theme"]) => void): () => void;
  onResize(cb: (payload: { width: number; height: number; breakpoint: string; surfaceRole: string }) => void): () => void;
  readonly sessionId: string;
  readonly origin: string;
  readonly generation: number;
}

declare global {
  interface Window {
    amitiaUI?: AmitiaUI;
  }
}

export function getAmitiaUI(): AmitiaUI {
  const ui = window.amitiaUI;
  if (!ui) throw new Error('amitiaUI is not available. Ensure the preload script is loaded.');
  return ui;
}

export function isAmitiaUIAvailable(): boolean {
  return typeof window !== 'undefined' && !!window.amitiaUI;
}

export async function waitUntilReady(timeout = 10000): Promise<AmitiaUI> {
  const start = Date.now();
  while (!isAmitiaUIAvailable()) {
    if (Date.now() - start > timeout) throw new Error('amitiaUI timeout');
    await new Promise(r => setTimeout(r, 100));
  }
  const ui = getAmitiaUI();
  await ui.ready();
  return ui;
}
