import { RuntimeFiber, type ExtensionFiber, type Disposer } from "./fiber";
import type { UISlotClient, UISlotDefinition, UISlotRegistration } from "./ui";
import { ValidationError } from "./errors";

export interface ClientPluginServiceRegistry {
  provide<T>(serviceId: string, value: T): Disposer;
  get<T>(serviceId: string): T | undefined;
  has(serviceId: string): boolean;
  list(): string[];
}

export interface ClientPluginEventBus {
  on<T = unknown>(eventType: string, handler: (payload: T) => void | Promise<void>): Disposer;
  emit<T = unknown>(eventType: string, payload: T): Promise<void>;
  list(): string[];
}

export interface ClientPluginSlotContext {
  declare(definition: UISlotDefinition): Promise<UISlotRegistration>;
  inject(slotId: string, callback: (definition: UISlotDefinition) => void): Promise<Disposer>;
  list(): Promise<UISlotDefinition[]>;
}

export interface ClientPluginContext {
  readonly pluginId: string;
  readonly fiber: ExtensionFiber;
  readonly services: ClientPluginServiceRegistry;
  readonly events: ClientPluginEventBus;
  readonly slots?: ClientPluginSlotContext;
}

export interface ClientPluginDefinition {
  id: string;
  setup(context: ClientPluginContext): void | Disposer | Promise<void | Disposer>;
}

export interface ClientPluginInspection {
  plugins: Array<{ id: string; state: "defined" | "running" }>;
  services: string[];
  events: string[];
  slots: UISlotDefinition[];
}

interface RunningPlugin {
  definition: ClientPluginDefinition;
  fiber: RuntimeFiber;
}

/**
 * Trusted client-plugin runtime. It accepts factories rather than eval strings:
 * host/agent code can define plugins dynamically while untrusted extension code
 * remains inside the existing restricted/isolated UI sandboxes.
 */
export class ClientPluginRuntime {
  private readonly definitions = new Map<string, ClientPluginDefinition>();
  private readonly running = new Map<string, RunningPlugin>();
  private readonly serviceValues = new Map<string, unknown>();
  private readonly eventHandlers = new Map<string, Set<(payload: unknown) => void | Promise<void>>>();

  constructor(private readonly slotClient?: UISlotClient) {}

  define(definition: ClientPluginDefinition): Disposer {
    const id = definition.id?.trim();
    if (!id) throw new ValidationError("client plugin id is required");
    if (typeof definition.setup !== "function") throw new ValidationError(`client plugin ${id} requires setup()`);
    if (this.definitions.has(id)) throw new ValidationError(`client plugin ${id} already defined`);
    this.definitions.set(id, { ...definition, id });
    return () => { void this.undefine(id); };
  }

  async run(pluginId: string): Promise<void> {
    const definition = this.definitions.get(pluginId);
    if (!definition) throw new ValidationError(`client plugin ${pluginId} is not defined`);
    if (this.running.has(pluginId)) return;
    const fiber = new RuntimeFiber(`client-plugin:${pluginId}`);
    const context: ClientPluginContext = {
      pluginId,
      fiber,
      services: this.scopedServices(fiber),
      events: this.scopedEvents(fiber),
      slots: this.slotClient ? this.scopedSlots(fiber, this.slotClient) : undefined,
    };
    this.running.set(pluginId, { definition, fiber });
    try {
      const cleanup = await definition.setup(context);
      if (typeof cleanup === "function") fiber.own(cleanup);
    } catch (error) {
      this.running.delete(pluginId);
      await fiber.dispose();
      throw error;
    }
  }

  async stop(pluginId: string): Promise<void> {
    const active = this.running.get(pluginId);
    if (!active) return;
    this.running.delete(pluginId);
    await active.fiber.dispose();
  }

  async undefine(pluginId: string): Promise<void> {
    await this.stop(pluginId);
    this.definitions.delete(pluginId);
  }

  async stopAll(): Promise<void> {
    for (const id of Array.from(this.running.keys()).reverse()) await this.stop(id);
  }

  async inspect(): Promise<ClientPluginInspection> {
    const slots = this.slotClient ? await this.slotClient.list() : [];
    return {
      plugins: Array.from(this.definitions.keys()).sort().map((id) => ({
        id,
        state: this.running.has(id) ? "running" as const : "defined" as const,
      })),
      services: Array.from(this.serviceValues.keys()).sort(),
      events: Array.from(this.eventHandlers.keys()).filter((key) => (this.eventHandlers.get(key)?.size ?? 0) > 0).sort(),
      slots,
    };
  }

  private scopedServices(fiber: RuntimeFiber): ClientPluginServiceRegistry {
    return {
      provide: <T>(serviceId: string, value: T) => {
        const id = serviceId.trim();
        if (!id) throw new ValidationError("service id is required");
        if (this.serviceValues.has(id)) throw new ValidationError(`service ${id} already provided`);
        this.serviceValues.set(id, value);
        const dispose = () => {
          if (this.serviceValues.get(id) === value) this.serviceValues.delete(id);
        };
        fiber.own(dispose);
        return dispose;
      },
      get: <T>(serviceId: string) => this.serviceValues.get(serviceId) as T | undefined,
      has: (serviceId: string) => this.serviceValues.has(serviceId),
      list: () => Array.from(this.serviceValues.keys()).sort(),
    };
  }

  private scopedEvents(fiber: RuntimeFiber): ClientPluginEventBus {
    return {
      on: <T = unknown>(eventType: string, handler: (payload: T) => void | Promise<void>) => {
        const type = eventType.trim();
        if (!type) throw new ValidationError("event type is required");
        let handlers = this.eventHandlers.get(type);
        if (!handlers) {
          handlers = new Set();
          this.eventHandlers.set(type, handlers);
        }
        const wrapped = handler as (payload: unknown) => void | Promise<void>;
        handlers.add(wrapped);
        const dispose = () => {
          handlers?.delete(wrapped);
          if (handlers?.size === 0) this.eventHandlers.delete(type);
        };
        fiber.own(dispose);
        return dispose;
      },
      emit: async <T = unknown>(eventType: string, payload: T) => {
        const handlers = Array.from(this.eventHandlers.get(eventType) ?? []);
        for (const handler of handlers) await handler(payload);
      },
      list: () => Array.from(this.eventHandlers.keys()).sort(),
    };
  }

  private scopedSlots(fiber: RuntimeFiber, slots: UISlotClient): ClientPluginSlotContext {
    return {
      declare: async (definition) => {
        const registration = await slots.declare(definition);
        fiber.own(registration.dispose);
        return registration;
      },
      inject: async (slotId, callback) => {
        const dispose = await slots.inject(slotId, callback);
        fiber.own(dispose);
        return dispose;
      },
      list: () => slots.list(),
    };
  }
}

export const defaultClientPluginRuntime = new ClientPluginRuntime();
