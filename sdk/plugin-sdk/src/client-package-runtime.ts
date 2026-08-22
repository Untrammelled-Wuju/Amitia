import { ClientPluginRuntime, type ClientPluginDefinition, type ClientPluginInspection } from "./client-plugin-runtime";
import type { Disposer } from "./fiber";
import type { UISlotClient } from "./ui";
import { ValidationError } from "./errors";

export interface ClientPluginPackage extends ClientPluginDefinition {
  version: string;
  description?: string;
}

export interface ClientPackageInspection extends ClientPluginInspection {
  packages: Array<{ id: string; versions: string[]; activeVersion?: string }>;
}

/**
 * Immutable trusted client-package catalog layered over ClientPluginRuntime.
 * Package versions are never mutated in place; activation, rollback and stop
 * therefore have deterministic lifecycle boundaries and fiber cleanup.
 */
export class ClientPackageRuntime {
  readonly runtime: ClientPluginRuntime;
  private readonly packages = new Map<string, Map<string, Readonly<ClientPluginPackage>>>();
  private readonly order = new Map<string, string[]>();
  private readonly active = new Map<string, string>();

  constructor(slotClient?: UISlotClient) {
    this.runtime = new ClientPluginRuntime(slotClient);
  }

  define(pkg: ClientPluginPackage): Disposer {
    const id = pkg.id?.trim();
    const version = pkg.version?.trim();
    if (!id || !version) throw new ValidationError("client package id and version are required");
    let versions = this.packages.get(id);
    if (!versions) {
      versions = new Map();
      this.packages.set(id, versions);
    }
    if (versions.has(version)) throw new ValidationError(`client package ${id}@${version} already defined`);
    versions.set(version, Object.freeze({ ...pkg, id, version }));
    const history = this.order.get(id) ?? [];
    history.push(version);
    this.order.set(id, history);
    return () => { void this.undefine(id, version); };
  }

  async run(id: string, version?: string): Promise<void> {
    const versions = this.packages.get(id);
    const selected = version ?? this.order.get(id)?.at(-1);
    const pkg = selected ? versions?.get(selected) : undefined;
    if (!pkg || !selected) throw new ValidationError(`client package ${id}@${selected ?? "?"} is not defined`);
    await this.runtime.undefine(id);
    this.runtime.define(pkg);
    this.active.set(id, selected);
    try {
      await this.runtime.run(id);
    } catch (error) {
      this.active.delete(id);
      await this.runtime.undefine(id);
      throw error;
    }
  }

  async stop(id: string): Promise<void> {
    await this.runtime.stop(id);
  }

  async rollback(id: string): Promise<string> {
    const history = this.order.get(id) ?? [];
    const active = this.active.get(id);
    const index = active ? history.lastIndexOf(active) : history.length;
    if (index <= 0) throw new ValidationError(`client package ${id} has no rollback version`);
    const previous = history[index - 1]!;
    await this.run(id, previous);
    return previous;
  }

  async undefine(id: string, version: string): Promise<void> {
    if (this.active.get(id) === version) {
      await this.runtime.undefine(id);
      this.active.delete(id);
    }
    const versions = this.packages.get(id);
    versions?.delete(version);
    if (versions?.size === 0) this.packages.delete(id);
    const history = (this.order.get(id) ?? []).filter((item) => item !== version);
    if (history.length) this.order.set(id, history); else this.order.delete(id);
  }

  async inspect(): Promise<ClientPackageInspection> {
    const base = await this.runtime.inspect();
    return {
      ...base,
      packages: Array.from(this.packages.entries()).map(([id]) => ({
        id,
        versions: [...(this.order.get(id) ?? [])],
        activeVersion: this.active.get(id),
      })),
    };
  }
}
