import type { UIPluginContext } from "@/ui-runtime/uiContext";
import type { InjectionKey } from "vue";

export interface ForeignComponentEvent {
  type: string;
  payload?: unknown;
}

export interface ForeignComponentMountInput {
  element: HTMLElement;
  props: Readonly<Record<string, unknown>>;
  context: UIPluginContext;
  emit(event: ForeignComponentEvent): void;
}

export interface ForeignComponentInstance {
  update?(input: ForeignComponentMountInput): void | Promise<void>;
  unmount(): void | Promise<void>;
}

export interface ForeignComponentAdapter {
  id: string;
  mount(input: ForeignComponentMountInput): ForeignComponentInstance | Promise<ForeignComponentInstance>;
}

export class ForeignComponentRegistry {
  private readonly adapters = new Map<string, ForeignComponentAdapter>();

  register(adapter: ForeignComponentAdapter): () => void {
    const id = adapter.id.trim();
    if (!id) throw new Error("foreign component adapter id is required");
    if (typeof adapter.mount !== "function") {
      throw new Error(`foreign component adapter ${id} mount() is required`);
    }
    if (this.adapters.has(id)) {
      throw new Error(`foreign component adapter ${id} is already registered`);
    }
    const normalized = { ...adapter, id };
    this.adapters.set(id, normalized);
    let active = true;
    return () => {
      if (!active) return;
      active = false;
      if (this.adapters.get(id) === normalized) this.adapters.delete(id);
    };
  }

  get(id: string): ForeignComponentAdapter | undefined {
    return this.adapters.get(id.trim());
  }

  async mount(id: string, input: ForeignComponentMountInput): Promise<ForeignComponentInstance> {
    const adapter = this.get(id);
    if (!adapter) throw new Error(`foreign component adapter ${id} is not registered`);
    return await adapter.mount(input);
  }

  list(): ForeignComponentAdapter[] {
    return Array.from(this.adapters.values()).sort((a, b) => a.id.localeCompare(b.id));
  }
}

export const foreignComponentRegistry = new ForeignComponentRegistry();
export const FOREIGN_COMPONENT_REGISTRY_KEY = Symbol("amitia-foreign-component-registry") as InjectionKey<ForeignComponentRegistry>;
