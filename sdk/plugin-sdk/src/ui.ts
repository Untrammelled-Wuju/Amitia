import { AmitiaError, ValidationError, PermissionDeniedError } from "./errors";
import type { UIProviderDefinition, UIProviderEntry } from "./manifest";

export interface UIActionRequest {
  readonly actionId: string;
  readonly payload?: unknown;
}

export interface UIActionResponse {
  readonly ok: boolean;
  readonly result?: unknown;
  readonly error?: { code: string; message: string };
}

export interface UIReadyEvent {
  readonly extensionId: string;
  readonly contributionId: string;
  readonly contractVersion: number;
}

export interface UIBridge {
  ready(): Promise<UIReadyEvent>;
  actions: UIActionClient;
  state: UIStateClient;
  host: UIHostClient;
  slots?: UISlotClient;
}

export interface UISlotDefinition {
  readonly slotId: string;
  readonly contractVersion?: number;
  readonly supportedKinds: string[];
  readonly multiplicity?: "single" | "multiple" | "ordered_multiple" | "replaceable_single" | "exclusive";
  readonly layout?: "inline" | "stack" | "row" | "grid" | "tabs" | "panel" | "drawer" | "modal" | "hidden";
  readonly fallbackPolicy?: "none" | "skeleton" | "empty" | "default";
  readonly parentSlotId?: string;
  readonly description?: string;
}

export interface UISlotRegistration {
  readonly definition: UISlotDefinition;
  dispose(): void | Promise<void>;
}

export interface UISlotClient {
  declare(definition: UISlotDefinition): Promise<UISlotRegistration>;
  list(): Promise<UISlotDefinition[]>;
  inject(slotId: string, callback: (definition: UISlotDefinition) => void): Promise<() => void>;
}

export interface UIActionClient {
  invoke(request: UIActionRequest): Promise<UIActionResponse>;
  list(): Promise<UIActionDescriptor[]>;
}

export interface UIActionDescriptor {
  readonly actionId: string;
  readonly title: string;
  readonly description?: string;
  readonly deprecated?: boolean;
}

export interface UIStateClient {
  get<T>(key: string): Promise<T | null>;
  set<T>(key: string, value: T): Promise<void>;
  subscribe(key: string, callback: (value: unknown) => void): Promise<() => void>;
}

export interface UIHostClient {
  openExternal(url: string): Promise<void>;
  copyToClipboard(text: string): Promise<void>;
  setTitle(title: string): Promise<void>;
  notify(message: string, kind?: "info" | "success" | "warning" | "error"): Promise<void>;
}

export type UIActionHandler = (
  payload: unknown,
  context: UIActionContext,
) => Promise<UIActionResponse> | UIActionResponse;

export interface UIActionContext {
  readonly actionId: string;
  readonly traceId: string;
  readonly signal?: AbortSignal;
  readonly logger: UILogger;
}

export interface UILogger {
  debug(message: string, fields?: Record<string, unknown>): void;
  info(message: string, fields?: Record<string, unknown>): void;
  warn(message: string, fields?: Record<string, unknown>): void;
  error(message: string, fields?: Record<string, unknown>): void;
}

const uiActionRegistry = new Map<string, { handler: UIActionHandler; descriptor: UIActionDescriptor }>();

export interface UIActionRegistration {
  readonly actionId: string;
  dispose(): void;
}

export function defineUIAction(
  actionId: string,
  handler: UIActionHandler,
  descriptor?: Partial<UIActionDescriptor>,
): UIActionRegistration {
  if (!actionId) {
    throw new ValidationError("actionId is required");
  }
  if (typeof handler !== "function") {
    throw new ValidationError("handler must be a function");
  }
  uiActionRegistry.set(actionId, {
    handler,
    descriptor: {
      actionId,
      title: descriptor?.title ?? actionId,
      description: descriptor?.description,
      deprecated: descriptor?.deprecated,
    },
  });
  return {
    actionId,
    dispose: () => {
      uiActionRegistry.delete(actionId);
    },
  };
}

export function unregisterUIAction(actionId: string): boolean {
  return uiActionRegistry.delete(actionId);
}

export function listUIActions(): UIActionDescriptor[] {
  return Array.from(uiActionRegistry.values()).map((r) => r.descriptor);
}

export function clearUIActions(): void {
  uiActionRegistry.clear();
}

export function createAmitiaUI(bridge: UIBridge): UIBridge {
  return bridge;
}

export function defineUISlot(definition: UISlotDefinition): UISlotDefinition {
  if (!definition.slotId?.trim()) throw new ValidationError("ui slot slotId is required");
  if (!Array.isArray(definition.supportedKinds) || definition.supportedKinds.length === 0) {
    throw new ValidationError(`ui slot ${definition.slotId} requires supportedKinds`);
  }
  if (definition.parentSlotId === definition.slotId) {
    throw new ValidationError(`ui slot ${definition.slotId} cannot parent itself`);
  }
  return {
    ...definition,
    slotId: definition.slotId.trim(),
    contractVersion: definition.contractVersion ?? 1,
    multiplicity: definition.multiplicity ?? "ordered_multiple",
    layout: definition.layout ?? "stack",
    fallbackPolicy: definition.fallbackPolicy ?? "empty",
  };
}

export class InMemoryUIBridge implements UIBridge {
  private readonly stateValues = new Map<string, unknown>();
  private readonly subscribers = new Map<string, Set<(value: unknown) => void>>();
  private readonly slotValues = new Map<string, UISlotDefinition>();
  private readonly slotSubscribers = new Map<string, Set<(definition: UISlotDefinition) => void>>();

  async ready(): Promise<UIReadyEvent> {
    return {
      extensionId: "in-memory",
      contributionId: "in-memory",
      contractVersion: 1,
    };
  }

  readonly actions: UIActionClient = {
    invoke: async (request: UIActionRequest): Promise<UIActionResponse> => {
      const entry = uiActionRegistry.get(request.actionId);
      if (!entry) {
        return { ok: false, error: { code: "action_not_found", message: `action ${request.actionId} not found` } };
      }
      const ctx: UIActionContext = {
        actionId: request.actionId,
        traceId: `ui-${Date.now()}`,
        logger: noopUILogger,
      };
      return entry.handler(request.payload, ctx);
    },
    list: async (): Promise<UIActionDescriptor[]> => listUIActions(),
  };

  readonly state: UIStateClient = {
    get: async <T>(key: string): Promise<T | null> => {
      return (this.stateValues.get(key) as T) ?? null;
    },
    set: async <T>(key: string, value: T): Promise<void> => {
      this.stateValues.set(key, value);
      const subs = this.subscribers.get(key);
      if (subs) {
        for (const cb of subs) cb(value);
      }
    },
    subscribe: async (key: string, callback: (value: unknown) => void): Promise<(() => void)> => {
      let subs = this.subscribers.get(key);
      if (!subs) {
        subs = new Set();
        this.subscribers.set(key, subs);
      }
      subs.add(callback);
      return () => {
        subs?.delete(callback);
      };
    },
  };

  readonly host: UIHostClient = {
    openExternal: async (_url: string): Promise<void> => {
      // no-op in memory
    },
    copyToClipboard: async (_text: string): Promise<void> => {
      // no-op in memory
    },
    setTitle: async (_title: string): Promise<void> => {},
    notify: async (_message: string, _kind?: "info" | "success" | "warning" | "error"): Promise<void> => {
      // no-op in memory
    },
  };

  readonly slots: UISlotClient = {
    declare: async (input: UISlotDefinition): Promise<UISlotRegistration> => {
      const definition = defineUISlot(input);
      this.slotValues.set(definition.slotId, definition);
      for (const callback of this.slotSubscribers.get(definition.slotId) ?? []) callback(definition);
      return {
        definition,
        dispose: () => {
          this.slotValues.delete(definition.slotId);
        },
      };
    },
    list: async (): Promise<UISlotDefinition[]> => Array.from(this.slotValues.values()),
    inject: async (slotId: string, callback: (definition: UISlotDefinition) => void): Promise<() => void> => {
      let subscribers = this.slotSubscribers.get(slotId);
      if (!subscribers) {
        subscribers = new Set();
        this.slotSubscribers.set(slotId, subscribers);
      }
      subscribers.add(callback);
      const current = this.slotValues.get(slotId);
      if (current) callback(current);
      return () => subscribers?.delete(callback);
    },
  };
}

const noopUILogger: UILogger = {
  debug: () => {},
  info: () => {},
  warn: () => {},
  error: () => {},
};

export function mapUIError(cause: unknown): AmitiaError {
  if (cause instanceof AmitiaError) return cause;
  const message = cause instanceof Error ? cause.message : String(cause);
  if (/permission|denied|forbidden/i.test(message)) {
    return new PermissionDeniedError(message);
  }
  return new ValidationError(message);
}

export function assertUIActionAllowed(actionId: string, allowed: string[]): void {
  if (!allowed.includes(actionId)) {
    throw new PermissionDeniedError(`UI action ${actionId} not allowed`);
  }
}


/**
 * Define a replaceable UI provider contribution with SDK-side fail-fast validation.
 * The manifest contribution id remains the authority for extension/module identity;
 * the host normalizes and verifies those fields again during installation.
 */
export function defineUIProvider(definition: UIProviderDefinition): UIProviderDefinition {
  if (!definition.providerId?.trim()) {
    throw new ValidationError("ui provider providerId is required");
  }
  if (!definition.capability) {
    throw new ValidationError("ui provider capability is required");
  }
  const entries = definition.entries ?? {};
  if (Object.keys(entries).length === 0) {
    throw new ValidationError("ui provider requires at least one platform entry");
  }
  for (const [platform, entry] of Object.entries(entries)) {
    assertUIProviderEntry(platform, entry, definition.capability);
  }
  assertUIProviderMetadata(definition);
  return {
    ...definition,
    providerId: definition.providerId.trim(),
    mode: definition.mode ?? "replace",
    entries: { ...entries },
  };
}

function assertUIProviderMetadata(definition: UIProviderDefinition): void {
  const metadata = definition.metadata;
  if (!metadata) return;
  if (definition.capability === "route.registry" && metadata.routes !== undefined) {
    if (!Array.isArray(metadata.routes)) throw new ValidationError("route.registry metadata.routes must be an array");
    for (const route of metadata.routes) {
      if (!route || typeof route !== "object" || !("id" in route) || !("path" in route) || !("providerId" in route)) {
        throw new ValidationError("route.registry routes require object entries with id, path and providerId");
      }
    }
  }
  if (["app.navigation", "route.registry"].includes(definition.capability) && metadata.navigationItems !== undefined) {
    if (!Array.isArray(metadata.navigationItems)) throw new ValidationError("app.navigation metadata.navigationItems must be an array");
    for (const item of metadata.navigationItems) {
      if (!item?.id || !item.label || !item.route?.startsWith("/")) {
        throw new ValidationError("navigation items require id, label and an absolute route");
      }
    }
  }
  if (definition.capability === "conversation.message_renderer") {
    for (const key of ["messageTypes", "roles", "mimeTypes", "extensionTypes"] as const) {
      const value = metadata[key];
      if (value !== undefined && !Array.isArray(value)) {
        throw new ValidationError(`conversation.message_renderer metadata.${key} must be an array`);
      }
    }
  }
  if (definition.capability === "ui.components" && metadata.componentVariants !== undefined) {
    if (!metadata.componentVariants || typeof metadata.componentVariants !== "object" || Array.isArray(metadata.componentVariants)) {
      throw new ValidationError("ui.components metadata.componentVariants must be an object");
    }
  }
  if (definition.capability === "ui.icons") {
    for (const key of ["iconAliases", "iconExports", "iconGlyphs"] as const) {
      const value = metadata[key];
      if (value !== undefined && (!value || typeof value !== "object" || Array.isArray(value))) {
        throw new ValidationError(`ui.icons metadata.${key} must be an object`);
      }
    }
  }
}

function assertUIProviderEntry(
  platform: string,
  entry: UIProviderEntry,
  capability: UIProviderDefinition["capability"],
): void {
  if (!platform.trim()) throw new ValidationError("ui provider platform key is required");
  switch (entry.type) {
    case "builtin_native":
      throw new ValidationError("builtin_native is reserved for host built-in providers");
    case "declarative": {
      const allowed = new Set<UIProviderDefinition["capability"]>([
        "app.navigation",
        "route.registry",
        "ui.theme",
        "ui.tokens",
        "ui.icons",
        "ui.components",
      ]);
      if (!allowed.has(capability)) {
        throw new ValidationError(`declarative entry is not supported for ${capability}`);
      }
      return;
    }
    case "web_module":
      if (["android", "ios", "mobile"].includes(platform.trim().toLowerCase())) {
        throw new ValidationError(`web_module cannot be used for Flutter AOT platform ${platform}; use schema_renderer or sandbox web`);
      }
      if (!entry.path?.trim()) throw new ValidationError(`web_module path required for ${platform}`);
      return;
    case "schema_renderer":
      if (!entry.contributionId?.trim()) {
        throw new ValidationError(`schema_renderer contributionId required for ${platform}`);
      }
      return;
    case "web_restricted":
    case "web_isolated":
      if (!entry.contributionId?.trim()) {
        throw new ValidationError(`${entry.type} contributionId required for ${platform}`);
      }
      return;
    default:
      throw new ValidationError(`unsupported UI provider entry type for ${platform}`);
  }
}
