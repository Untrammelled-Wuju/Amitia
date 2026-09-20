import type { Component } from "vue";

const extensionRenderers = new Map<string, Component>();

export interface ExtensionRendererRegistration {
  rendererId: string;
  component: Component;
  capability?: string;
}

export function registerExtensionRenderer(registration: ExtensionRendererRegistration): void {
  const rendererId = registration.rendererId.trim();
  if (!rendererId) return;
  extensionRenderers.set(rendererId, registration.component);
}

export function resolveExtensionRenderer(rendererId: string): Component | null {
  return extensionRenderers.get(rendererId.trim()) ?? null;
}

export function listExtensionRenderers(): string[] {
  return [...extensionRenderers.keys()].sort();
}

