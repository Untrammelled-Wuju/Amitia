import { defineAsyncComponent, type Component } from "vue";

const registry = new Map<string, Component>();

export const HOST_RUNTIME_CHARACTER_PSYCHE = "host.character.psyche";
export const HOST_RUNTIME_CHARACTER_DETAIL = "host.character.detail";
export const HOST_RUNTIME_CHARACTER_TEAMS = "host.character.teams";
export const HOST_RUNTIME_CHARACTER_WORLDBOOK = "host.character.worldbook";
export const HOST_RUNTIME_CREATIVE_CARDS = "host.creative.character-cards";
export const HOST_RUNTIME_CREATIVE_PET_INSTALLATIONS = "host.creative.pet-installations";
export const HOST_RUNTIME_GAME_PLUGIN = "host.game.plugin";
export const HOST_RUNTIME_MEMORY_OVERVIEW = "host.memory.overview";
export const HOST_RUNTIME_MEMORY_TIMELINE = "host.memory.timeline";
export const HOST_RUNTIME_MEMORY_EPISODIC = "host.memory.episodic";
export const HOST_RUNTIME_MEMORY_GRAPH = "host.memory.graph";
export const HOST_RUNTIME_MEMORY_KNOWLEDGE = "host.memory.knowledge";
export const HOST_RUNTIME_PROFILE_SUMMARY = "host.profile.summary";

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
  HOST_RUNTIME_CHARACTER_DETAIL,
  defineAsyncComponent(() => import("@/views/character/CharacterView.vue")),
);
registerHostRuntimeComponent(
  HOST_RUNTIME_CHARACTER_TEAMS,
  defineAsyncComponent(() => import("@/views/character/CharacterTeamsView.vue")),
);
registerHostRuntimeComponent(
  HOST_RUNTIME_CHARACTER_WORLDBOOK,
  defineAsyncComponent(() => import("@/views/world-book/WorldBookView.vue")),
);
registerHostRuntimeComponent(
  HOST_RUNTIME_CREATIVE_CARDS,
  defineAsyncComponent(() => import("@/views/character-config/CharacterConfigView.vue")),
);
registerHostRuntimeComponent(
  HOST_RUNTIME_CREATIVE_PET_INSTALLATIONS,
  defineAsyncComponent(() => import("@/views/creative-workshop/PetInstallationsView.vue")),
);
registerHostRuntimeComponent(
  HOST_RUNTIME_GAME_PLUGIN,
  defineAsyncComponent(() => import("@/views/game-center/GamePluginView.vue")),
);
registerHostRuntimeComponent(
  HOST_RUNTIME_MEMORY_OVERVIEW,
  defineAsyncComponent(() => import("@/views/memory-manager/MemoryManagerView.vue")),
);
registerHostRuntimeComponent(
  HOST_RUNTIME_MEMORY_TIMELINE,
  defineAsyncComponent(() => import("@/views/memory-timeline/MemoryTimeline.vue")),
);
registerHostRuntimeComponent(
  HOST_RUNTIME_MEMORY_EPISODIC,
  defineAsyncComponent(() => import("@/views/episodic/EpisodicView.vue")),
);
registerHostRuntimeComponent(
  HOST_RUNTIME_MEMORY_GRAPH,
  defineAsyncComponent(() => import("@/views/graph/GraphView.vue")),
);
registerHostRuntimeComponent(
  HOST_RUNTIME_MEMORY_KNOWLEDGE,
  defineAsyncComponent(() => import("@/views/knowledge/KnowledgeView.vue")),
);
registerHostRuntimeComponent(
  HOST_RUNTIME_PROFILE_SUMMARY,
  defineAsyncComponent(() => import("@/views/profile/ProfileView.vue")),
);
