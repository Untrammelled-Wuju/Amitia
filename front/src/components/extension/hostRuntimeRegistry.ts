import { defineAsyncComponent, type Component } from "vue";

const registry = new Map<string, Component>();

export const HOST_RUNTIME_CHARACTER_PSYCHE = "host.character.psyche";
export const HOST_RUNTIME_CHARACTER_ACTIVE_TASK = "host.character.active-task";

export function registerHostRuntimeComponent(runtimeId: string, component: Component) {
  const key = runtimeId.trim();
  if (!key) return;
  registry.set(key, component);
}

export function resolveHostRuntimeComponent(runtimeId: string): Component | null {
  const key = runtimeId.trim();
  if (!key) return null;
  return registry.get(key) ?? null;
}

registerHostRuntimeComponent(
  HOST_RUNTIME_CHARACTER_PSYCHE,
  defineAsyncComponent(() => import("@/views/character/CharacterPsycheView.vue")),
);

registerHostRuntimeComponent(
  HOST_RUNTIME_CHARACTER_ACTIVE_TASK,
  defineAsyncComponent(() => import("@/views/character/CharacterActiveTaskView.vue")),
);
