import type { Component } from "vue";

export type UIProviderCapability =
  | "app.shell"
  | "app.navigation"
  | "app.workspace"
  | "route.registry"
  | "page.provider"
  | "conversation.shell"
  | "conversation.header"
  | "conversation.messages"
  | "conversation.message_renderer"
  | "conversation.sidebar"
  | "conversation.composer"
  | "conversation.overlay"
  | "character.shell"
  | "character.detail"
  | "memory.shell"
  | "memory.detail"
  | "settings.shell"
  | "settings.section"
  | "extension.center"
  | "extension.page"
  | "ui.theme"
  | "ui.tokens"
  | "ui.icons"
  | "ui.components";

export const UI_PROVIDER_CAPABILITIES: readonly UIProviderCapability[] = [
  "app.shell", "app.navigation", "app.workspace", "route.registry", "page.provider",
  "conversation.shell", "conversation.header", "conversation.messages", "conversation.message_renderer",
  "conversation.sidebar", "conversation.composer", "conversation.overlay",
  "character.shell", "character.detail", "memory.shell", "memory.detail",
  "settings.shell", "settings.section", "extension.center", "extension.page",
  "ui.theme", "ui.tokens", "ui.icons", "ui.components",
] as const;

export function isUIProviderCapability(value: string): value is UIProviderCapability {
  return (UI_PROVIDER_CAPABILITIES as readonly string[]).includes(value);
}

export type UIProviderMode = "replace" | "compose" | "augment";
export type UIProviderEntryType =
  | "builtin_native"
  | "declarative"
  | "web_module"
  | "schema_renderer"
  | "web_restricted"
  | "web_isolated";

export interface UIProviderEntry {
  contributionId?: string;
  type: UIProviderEntryType;
  path?: string;
  schemaPath?: string;
  exportName?: string;
  contentHash?: string;
}

export interface UIProviderDefinition {
  providerId: string;
  extensionId: string;
  moduleId?: string;
  capability: UIProviderCapability;
  mode: UIProviderMode;
  priority?: number;
  platforms?: string[];
  entries: Record<string, UIProviderEntry>;
  fallbackProviderId?: string;
  trustLevel?: string;
  permissions?: string[];
  generation?: number;
  enabled: boolean;
  builtin?: boolean;
  metadata?: Record<string, unknown>;
}

export interface UIProfile {
  profileId: string;
  name: string;
  selections: Partial<Record<UIProviderCapability, string>>;
  updatedAt?: number;
}

export interface UIProviderResolution {
  capability: UIProviderCapability;
  platform: string;
  provider?: UIProviderDefinition;
  fallbackChain?: string[];
  reason?: string;
}

export interface UIProviderRenderContext extends Record<string, unknown> {
  route?: string;
  platform?: string;
  host?: string;
  os?: string;
  locale?: string;
  capability?: UIProviderCapability;
}

export type BuiltinProviderComponentRegistry = Partial<Record<UIProviderCapability, Component>>;
