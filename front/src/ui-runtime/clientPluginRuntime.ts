import type { UIContributionSnapshot, SlotSnapshot } from "@/stores/extensionUI";

export type ClientDisposer = () => void | Promise<void>;
export type ClientSlotEffect = void | ClientDisposer | Promise<void | ClientDisposer>;

export interface ClientSlotDefinition {
  slotId: string;
  contractVersion: number;
  supportedKinds: string[];
  multiplicity?: SlotSnapshot["multiplicity"];
  layout?: SlotSnapshot["layout"];
  fallbackPolicy?: SlotSnapshot["fallbackPolicy"];
  parentSlotId?: string;
  description?: string;
}

export interface ClientSlotRegistration {
  definition: ClientSlotDefinition;
  dispose(): void | Promise<void>;
}

interface SlotSubscriber {
  callback: (definition: ClientSlotDefinition) => ClientSlotEffect;
  cleanup?: ClientDisposer;
}

class BrowserSlotRuntime {
  private readonly local = new Map<string, ClientSlotDefinition>();
  private readonly server = new Map<string, ClientSlotDefinition>();
  private readonly epochs = new Map<string, number>();
  private readonly subscribers = new Map<string, Set<SlotSubscriber>>();

  async syncSnapshot(snapshot: UIContributionSnapshot | null): Promise<void> {
    const next = new Map<string, ClientSlotDefinition>();
    for (const slot of snapshot?.slots ?? []) next.set(slot.slotId, fromSnapshot(slot));
    const ids = new Set([...this.server.keys(), ...next.keys()]);
    for (const id of ids) {
      const previous = this.server.get(id);
      const incoming = next.get(id);
      if (sameDefinition(previous, incoming)) continue;
      if (previous && !this.local.has(id)) await this.deactivate(id);
      if (incoming) this.server.set(id, incoming); else this.server.delete(id);
      if (incoming && !this.local.has(id)) await this.activate(id, incoming);
    }
  }

  async declare(input: ClientSlotDefinition): Promise<ClientSlotRegistration> {
    const definition = normalizeDefinition(input);
    const id = definition.slotId;
    if (this.local.has(id)) throw new Error(`client slot ${id} is already declared locally`);
    if (this.current(id)) await this.deactivate(id);
    this.local.set(id, definition);
    const epoch = (this.epochs.get(id) ?? 0) + 1;
    this.epochs.set(id, epoch);
    await this.activate(id, definition);
    return {
      definition,
      dispose: async () => {
        if (this.epochs.get(id) !== epoch || !this.local.has(id)) return;
        await this.deactivate(id);
        this.local.delete(id);
        const fallback = this.server.get(id);
        if (fallback) {
          this.epochs.set(id, epoch + 1);
          await this.activate(id, fallback);
        }
      },
    };
  }

  async inject(slotId: string, callback: (definition: ClientSlotDefinition) => ClientSlotEffect): Promise<ClientDisposer> {
    const id = slotId.trim();
    if (!id) throw new Error("slotId is required");
    let subscribers = this.subscribers.get(id);
    if (!subscribers) {
      subscribers = new Set();
      this.subscribers.set(id, subscribers);
    }
    const subscriber: SlotSubscriber = { callback };
    subscribers.add(subscriber);
    const current = this.current(id);
    if (current) subscriber.cleanup = await callback(current) || undefined;
    return async () => {
      subscribers?.delete(subscriber);
      if (subscriber.cleanup) await subscriber.cleanup();
      subscriber.cleanup = undefined;
      if (subscribers?.size === 0) this.subscribers.delete(id);
    };
  }

  list(): ClientSlotDefinition[] {
    const values = new Map(this.server);
    for (const [id, definition] of this.local) values.set(id, definition);
    return Array.from(values.values()).sort((a, b) => a.slotId.localeCompare(b.slotId));
  }

  private current(id: string): ClientSlotDefinition | undefined {
    return this.local.get(id) ?? this.server.get(id);
  }

  private async deactivate(id: string): Promise<void> {
    for (const subscriber of this.subscribers.get(id) ?? []) {
      if (subscriber.cleanup) await subscriber.cleanup();
      subscriber.cleanup = undefined;
    }
  }

  private async activate(id: string, definition: ClientSlotDefinition): Promise<void> {
    for (const subscriber of this.subscribers.get(id) ?? []) {
      if (subscriber.cleanup) await subscriber.cleanup();
      subscriber.cleanup = await subscriber.callback(definition) || undefined;
    }
  }
}

class ClientFiber {
  private readonly disposers: ClientDisposer[] = [];
  private disposed = false;
  constructor(readonly id: string) {}
  own(disposer: ClientDisposer): ClientDisposer {
    if (this.disposed) void disposer(); else this.disposers.push(disposer);
    return disposer;
  }
  async dispose(): Promise<void> {
    if (this.disposed) return;
    this.disposed = true;
    for (let index = this.disposers.length - 1; index >= 0; index--) await this.disposers[index]?.();
    this.disposers.length = 0;
  }
}

export interface BrowserClientPluginContext {
  pluginId: string;
  services: {
    provide<T>(id: string, value: T): ClientDisposer;
    get<T>(id: string): T | undefined;
    list(): string[];
  };
  events: {
    on<T>(type: string, handler: (payload: T) => void | Promise<void>): ClientDisposer;
    emit<T>(type: string, payload: T): Promise<void>;
    list(): string[];
  };
  slots: {
    declare(definition: ClientSlotDefinition): Promise<ClientSlotRegistration>;
    inject(slotId: string, callback: (definition: ClientSlotDefinition) => ClientSlotEffect): Promise<ClientDisposer>;
    list(): ClientSlotDefinition[];
  };
}

export interface BrowserClientPluginDefinition {
  id: string;
  version?: string;
  setup(context: BrowserClientPluginContext): void | ClientDisposer | Promise<void | ClientDisposer>;
}

export class BrowserClientPluginRuntime {
  private readonly definitions = new Map<string, BrowserClientPluginDefinition>();
  private readonly packages = new Map<string, Map<string, BrowserClientPluginDefinition>>();
  private readonly packageOrder = new Map<string, string[]>();
  private readonly activePackageVersion = new Map<string, string>();
  private readonly running = new Map<string, ClientFiber>();
  private readonly services = new Map<string, unknown>();
  private readonly events = new Map<string, Set<(payload: unknown) => void | Promise<void>>>();
  readonly slots = new BrowserSlotRuntime();

  define(definition: BrowserClientPluginDefinition): ClientDisposer {
    const id = definition.id?.trim();
    if (!id) throw new Error("client plugin id is required");
    if (this.definitions.has(id)) throw new Error(`client plugin ${id} already defined`);
    this.definitions.set(id, { ...definition, id });
    return () => this.undefine(id);
  }


  definePackage(definition: BrowserClientPluginDefinition & { version: string }): ClientDisposer {
    const id = definition.id?.trim();
    const version = definition.version?.trim();
    if (!id || !version) throw new Error("client package id and version are required");
    let versions = this.packages.get(id);
    if (!versions) { versions = new Map(); this.packages.set(id, versions); }
    if (versions.has(version)) throw new Error(`client package ${id}@${version} already exists`);
    versions.set(version, Object.freeze({ ...definition, id, version }));
    const order = this.packageOrder.get(id) ?? [];
    order.push(version);
    this.packageOrder.set(id, order);
    return () => this.undefinePackage(id, version);
  }

  async runPackage(id: string, version?: string): Promise<void> {
    const versions = this.packages.get(id);
    if (!versions || versions.size === 0) throw new Error(`client package ${id} is not defined`);
    const selected = version || this.packageOrder.get(id)?.at(-1);
    const definition = selected ? versions.get(selected) : undefined;
    if (!definition || !selected) throw new Error(`client package ${id}@${selected ?? "?"} is not defined`);
    const current = this.definitions.get(id);
    if (current) await this.undefine(id);
    this.define(definition);
    this.activePackageVersion.set(id, selected);
    try {
      await this.run(id);
    } catch (error) {
      this.activePackageVersion.delete(id);
      await this.undefine(id);
      throw error;
    }
  }

  async rollbackPackage(id: string): Promise<string> {
    const order = this.packageOrder.get(id) ?? [];
    const active = this.activePackageVersion.get(id);
    const currentIndex = active ? order.lastIndexOf(active) : order.length;
    if (currentIndex <= 0) throw new Error(`client package ${id} has no rollback version`);
    const previous = order[currentIndex - 1]!;
    await this.runPackage(id, previous);
    return previous;
  }

  async stopPackage(id: string): Promise<void> {
    await this.stop(id);
  }

  async undefinePackage(id: string, version: string): Promise<void> {
    if (this.activePackageVersion.get(id) === version) {
      await this.undefine(id);
      this.activePackageVersion.delete(id);
    }
    const versions = this.packages.get(id);
    versions?.delete(version);
    if (versions?.size === 0) this.packages.delete(id);
    const order = (this.packageOrder.get(id) ?? []).filter((item) => item !== version);
    if (order.length) this.packageOrder.set(id, order); else this.packageOrder.delete(id);
  }

  async run(id: string): Promise<void> {
    if (this.running.has(id)) return;
    const definition = this.definitions.get(id);
    if (!definition) throw new Error(`client plugin ${id} is not defined`);
    const fiber = new ClientFiber(`client-plugin:${id}`);
    this.running.set(id, fiber);
    try {
      const cleanup = await definition.setup(this.context(id, fiber));
      if (typeof cleanup === "function") fiber.own(cleanup);
    } catch (error) {
      this.running.delete(id);
      await fiber.dispose();
      throw error;
    }
  }

  async stop(id: string): Promise<void> {
    const fiber = this.running.get(id);
    if (!fiber) return;
    this.running.delete(id);
    await fiber.dispose();
  }

  async undefine(id: string): Promise<void> {
    await this.stop(id);
    this.definitions.delete(id);
  }

  inspect() {
    return {
      plugins: Array.from(this.definitions.values()).map((definition) => ({
        id: definition.id,
        version: definition.version ?? "0",
        state: this.running.has(definition.id) ? "running" as const : "defined" as const,
      })),
      services: Array.from(this.services.keys()).sort(),
      events: Array.from(this.events.keys()).filter((type) => (this.events.get(type)?.size ?? 0) > 0).sort(),
      slots: this.slots.list(),
      packages: Array.from(this.packages.entries()).map(([id, versions]) => ({
        id,
        versions: this.packageOrder.get(id) ?? Array.from(versions.keys()),
        activeVersion: this.activePackageVersion.get(id),
      })),
    };
  }

  private context(pluginId: string, fiber: ClientFiber): BrowserClientPluginContext {
    return {
      pluginId,
      services: {
        provide: <T>(id: string, value: T) => {
          if (!id.trim()) throw new Error("service id is required");
          if (this.services.has(id)) throw new Error(`service ${id} already provided`);
          this.services.set(id, value);
          return fiber.own(() => { if (this.services.get(id) === value) this.services.delete(id); });
        },
        get: <T>(id: string) => this.services.get(id) as T | undefined,
        list: () => Array.from(this.services.keys()).sort(),
      },
      events: {
        on: <T>(type: string, handler: (payload: T) => void | Promise<void>) => {
          let handlers = this.events.get(type);
          if (!handlers) { handlers = new Set(); this.events.set(type, handlers); }
          const wrapped = handler as (payload: unknown) => void | Promise<void>;
          handlers.add(wrapped);
          return fiber.own(() => {
            handlers?.delete(wrapped);
            if (handlers?.size === 0) this.events.delete(type);
          });
        },
        emit: async <T>(type: string, payload: T) => {
          for (const handler of Array.from(this.events.get(type) ?? [])) await handler(payload);
        },
        list: () => Array.from(this.events.keys()).sort(),
      },
      slots: {
        declare: async (definition) => {
          const registration = await this.slots.declare(definition);
          fiber.own(registration.dispose);
          return registration;
        },
        inject: async (slotId, callback) => {
          const dispose = await this.slots.inject(slotId, callback);
          fiber.own(dispose);
          return dispose;
        },
        list: () => this.slots.list(),
      },
    };
  }
}

export const browserClientPluginRuntime = new BrowserClientPluginRuntime();

export function syncBrowserClientSlots(snapshot: UIContributionSnapshot | null): Promise<void> {
  return browserClientPluginRuntime.slots.syncSnapshot(snapshot);
}

function fromSnapshot(slot: SlotSnapshot): ClientSlotDefinition {
  return normalizeDefinition({
    slotId: slot.slotId,
    contractVersion: slot.contractVersion,
    supportedKinds: slot.supportedKinds ?? [],
    multiplicity: slot.multiplicity,
    layout: slot.layout,
    fallbackPolicy: slot.fallbackPolicy,
    parentSlotId: slot.parentSlotId,
    description: slot.description,
  });
}

function normalizeDefinition(definition: ClientSlotDefinition): ClientSlotDefinition {
  const slotId = definition.slotId?.trim();
  if (!slotId) throw new Error("slotId is required");
  return { ...definition, slotId, contractVersion: definition.contractVersion || 1, supportedKinds: [...definition.supportedKinds] };
}

function sameDefinition(a?: ClientSlotDefinition, b?: ClientSlotDefinition): boolean {
  return JSON.stringify(a ?? null) === JSON.stringify(b ?? null);
}

declare global {
  interface Window {
    amitiaClientPlugins?: BrowserClientPluginRuntime;
  }
}
