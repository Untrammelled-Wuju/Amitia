import type { LocalizedText, PermissionRequirement, ScopeRule } from "./types";

export type ModuleKind =
  | "tool"
  | "skill"
  | "workflow"
  | "mcp_server"
  | "javascript_main"
  | "task_runtime"
  | "trusted_service"
  | "wasm_runtime"
  | "ui_contribution"
  | "ui_provider";

export interface AmitiaxManifestV2 {
  manifestVersion: 2;
  extension: {
    id: string;
    name: LocalizedText;
    description: LocalizedText;
    version: string;
    license?: string;
    homepage?: string;
    repository?: string;
    categories?: string[];
    keywords?: string[];
    icon?: string;
  };
  publisher: {
    id: string;
    displayName: string;
    trustLevel?: string;
    contact?: string;
    website?: string;
  };
  compatibility?: {
    minHostVersion?: string;
    maxHostVersion?: string;
    platforms?: string[];
    featureFlags?: string[];
  };
  modules: ModuleDeclaration[];
  dependencies?: ManifestDependency[];
  permissions?: ManifestPermission[];
  resources?: ManifestResource[];
  lifecycle?: Record<string, unknown>;
  integrity: {
    algorithm: "sha256";
    contentTreeHash: string;
    fileHashes?: Record<string, string>;
  };
  development?: Record<string, unknown>;
}

export interface ModuleDeclaration {
  id: string;
  name: LocalizedText;
  description?: LocalizedText;
  type: "builtin" | "native" | "javascript" | "wasm" | "service" | "data_only";
  version?: string;
  runtime?: {
    type: string;
    entryPoint?: string;
    workerCount?: number;
    timeout?: string;
    memory?: number;
    permissions?: string[];
    capabilities?: Record<string, boolean>;
    env?: Record<string, string>;
  };
  contributions?: ManifestContribution[];
  dependencies?: ManifestDependency[];
}

export interface ManifestContribution {
  id: string;
  kind: string;
  name: LocalizedText;
  description?: LocalizedText;
  version?: string;
  spec?: Record<string, unknown>;
  requiredPermissions?: string[];
  requiredScope?: string[];
}

export interface ManifestDependency {
  type: string;
  id: string;
  version?: string;
  optional?: boolean;
  reason?: string;
}

export interface ManifestPermission {
  name: string;
  reason?: string;
  required?: boolean;
  scope?: string;
}

export interface ManifestResource {
  id: string;
  type: string;
  path: string;
  hash?: string;
  size?: number;
}

export interface ToolContributionDefinition {
  toolId: string;
  modelToolName?: string;
  title: LocalizedText;
  description: LocalizedText;
  parameters?: JSONSchema;
  permissions?: PermissionRequirement[];
  riskLevel?: "low" | "medium" | "high" | "critical";
  runtimeBinding: string;
  idempotent?: boolean;
  deprecated?: boolean;
}

export interface SkillContributionDefinition {
  skillId: string;
  title: LocalizedText;
  description: LocalizedText;
  category: string;
  triggerPatterns?: string[];
  inputSchema?: JSONSchema;
  outputSchema?: JSONSchema;
  requiredTools?: string[];
  permissions?: PermissionRequirement[];
  riskLevel?: "low" | "medium" | "high";
  runtimeBinding: string;
}

export interface WorkflowContributionDefinition {
  workflowId: string;
  title: LocalizedText;
  description: LocalizedText;
  category: string;
  steps: WorkflowStepDefinition[];
  inputSchema?: JSONSchema;
  outputSchema?: JSONSchema;
  requiredTools?: string[];
  permissions?: PermissionRequirement[];
  riskLevel?: "low" | "medium" | "high";
  maxConcurrency?: number;
  timeoutMs?: number;
}

export interface WorkflowStepDefinition {
  stepId: string;
  type: "tool_call" | "condition" | "transform" | "delay" | "parallel" | "sequential";
  toolId?: string;
  input?: unknown;
  condition?: string;
  onError?: "continue" | "abort" | "retry";
}

export type UIProviderCapability =
  | "app.shell" | "app.navigation" | "app.workspace"
  | "route.registry" | "page.provider"
  | "conversation.shell" | "conversation.header" | "conversation.messages"
  | "conversation.message_renderer" | "conversation.sidebar" | "conversation.composer" | "conversation.overlay"
  | "character.shell" | "character.detail" | "memory.shell" | "memory.detail"
  | "settings.shell" | "settings.section" | "extension.center" | "extension.page"
  | "ui.theme" | "ui.tokens" | "ui.icons" | "ui.components";

export type UIProviderEntryType =
  | "builtin_native" | "declarative" | "web_module" | "schema_renderer" | "web_restricted" | "web_isolated";

export interface UIProviderEntry {
  contributionId?: string;
  type: UIProviderEntryType;
  path?: string;
  schemaPath?: string;
  exportName?: string;
  contentHash?: string;
}

export interface UINavigationItemDefinition {
  id: string;
  label: string;
  route: string;
  icon?: string;
  group?: string;
  groupLabel?: string;
  groupIcon?: string;
  panel?: "main" | "more";
  order?: number;
  match?: string[];
  routePrefixes?: string[];
  mobile?: boolean;
}

export interface UIRouteDefinition {
  id: string;
  path: string;
  providerId: string;
  capability?: UIProviderCapability;
  title?: string;
  priority?: number;
}

export interface UIMessageRendererSelector {
  messageTypes?: string[];
  roles?: string[];
  mimeTypes?: string[];
  extensionTypes?: string[];
}

export interface UIDesignColorTokens {
  primary?: string; accent?: string; background?: string; surface?: string;
  backgroundPrimary?: string; backgroundSecondary?: string;
  surfacePrimary?: string; surfaceSecondary?: string;
  accentPrimary?: string; accentSecondary?: string; accentSoft?: string; accentPressed?: string;
  textPrimary?: string; textSecondary?: string; textTertiary?: string; textMuted?: string; textDisabled?: string;
  border?: string; borderPrimary?: string; borderSecondary?: string;
  success?: string; warning?: string; error?: string; danger?: string; info?: string; scrim?: string; overlay?: string;
}

export interface UIDesignTokenSet {
  colors?: UIDesignColorTokens;
  spacing?: Partial<Record<"xs" | "sm" | "md" | "lg" | "xl" | "xxl" | "xxxl" | "page" | "card" | "section" | "component" | "tight", string | number>>;
  radius?: Partial<Record<"xs" | "sm" | "md" | "lg" | "tag" | "pill", string | number>>;
  typography?: Partial<Record<
    "fontFamily" | "pageTitleSize" | "pageLargeTitleSize" | "sectionTitleSize" | "cardTitleSize" | "titleSize" |
    "bodySize" | "bodySmallSize" | "captionSize" | "labelSize" | "statusLabelSize" | "buttonSize" |
    "weightRegular" | "weightMedium" | "weightBold" | "pageTitleWeight" | "sectionTitleWeight" | "cardTitleWeight" |
    "bodyWeight" | "labelWeight" | "buttonWeight",
    string | number
  >>;
  icons?: Partial<Record<"extraSmall" | "small" | "medium" | "size" | "large" | "navigation" | "navigationSize", string | number>>;
  components?: Partial<Record<"toolbarHeight" | "drawerWidth" | "drawerMaxWidth" | "controlHeight" | "compactControlHeight" | "borderWidth", string | number>>;
  [key: `--${string}`]: unknown;
}

export interface UIComponentVariantDefinition {
  minHeight?: number; height?: number; paddingX?: number; paddingY?: number;
  outerPaddingX?: number; outerPaddingY?: number; radius?: number; gap?: number;
  fontSize?: number; fontWeight?: number; iconSize?: number; borderWidth?: number; opacity?: number;
  [key: string]: string | number | boolean | undefined;
}

export interface UIIconGlyphDefinition {
  codePoint: number;
  fontFamily?: string;
  fontPackage?: string;
  matchTextDirection?: boolean;
}

export interface UIProviderMetadata extends UIMessageRendererSelector {
  routes?: Array<string | UIRouteDefinition>;
  routePatterns?: string[];
  navigationItems?: UINavigationItemDefinition[];
  tokens?: UIDesignTokenSet | { light?: UIDesignTokenSet; dark?: UIDesignTokenSet };
  cssVariables?: Record<string, string>;
  iconAliases?: Record<string, string>;
  /** Trusted Web module named exports for semantic icons. */
  iconExports?: Record<string, string>;
  /** Flutter-safe glyph definitions for semantic icons. */
  iconGlyphs?: Record<string, number | UIIconGlyphDefinition>;
  /** Declarative primitive variants consumed by both Web and Flutter host components. */
  componentVariants?: Record<string, UIComponentVariantDefinition>;
  components?: Record<string, unknown>;
  [key: string]: unknown;
}

export type UIProviderPlacement = "any" | "cloud" | "device" | "hybrid";

export interface UIDeviceRequirements {
  platforms?: string[];
  architectures?: string[];
  minAppVersion?: string;
  minRuntimeVersion?: string;
  requiredFeatures?: string[];
}

export interface UIProfileScope {
  userId?: string;
  deviceId?: string;
  platform?: string;
  runtimeProfile?: string;
}

export interface UIProviderResolveContext extends UIProfileScope {
  architecture?: string;
  appVersion?: string;
  runtimeVersion?: string;
  deviceOnline?: boolean;
  localRuntime?: boolean;
  deviceCapabilities?: string[];
}

export interface UIProviderDefinition {
  providerId: string;
  extensionId?: string;
  moduleId?: string;
  capability: UIProviderCapability;
  mode?: "replace" | "compose" | "augment";
  priority?: number;
  platforms?: string[];
  entries: Record<string, UIProviderEntry>;
  fallbackProviderId?: string;
  trustLevel?: string;
  permissions?: string[];
  /** Host-populated from the owning module placement. */
  placement?: UIProviderPlacement;
  /** Host-populated from the owning module deviceRequirements. */
  deviceRequirements?: UIDeviceRequirements;
  metadata?: UIProviderMetadata;
}

export interface UIProfile {
  profileId: string;
  name: string;
  selections: Partial<Record<UIProviderCapability, string>>;
  scope?: UIProfileScope;
  revision?: number;
  updatedAt?: number;
}

export interface UIContributionDefinition {
  contributionId: string;
  kind: string;
  slotId: string;
  contractVersion: number;
  title: LocalizedText;
  description?: LocalizedText;
  icon?: string;
  entryKind: "schema_page" | "web_page" | "panel" | "card" | "action" | "menu_item";
  entryPath?: string;
  schemaPath?: string;
  runtimeId?: string;
  permissions?: PermissionRequirement[];
  scopeRule?: ScopeRule;
  ordering?: number;
  group?: string;
  sandbox?: "host_native" | "schema_renderer" | "web_restricted" | "web_isolated";
}

export interface JSONSchema {
  type?: string;
  properties?: Record<string, JSONSchema>;
  required?: string[];
  items?: JSONSchema;
  enum?: unknown[];
  description?: string;
  default?: unknown;
  minimum?: number;
  maximum?: number;
  minLength?: number;
  maxLength?: number;
  pattern?: string;
  format?: string;
  additionalProperties?: boolean | JSONSchema;
  $ref?: string;
}

export function validateManifest(manifest: AmitiaxManifestV2): string[] {
  const errors: string[] = [];
  if (manifest.manifestVersion !== 2) {
    errors.push("manifestVersion must be 2");
  }
  if (!manifest.extension?.id) {
    errors.push("extension.id is required");
  }
  if (!manifest.publisher?.id) {
    errors.push("publisher.id is required");
  }
  if (!manifest.extension?.name?.default) {
    errors.push("extension.name.default is required");
  }
  if (!manifest.extension?.version) {
    errors.push("extension.version is required");
  }
  if (!manifest.modules || manifest.modules.length === 0) {
    errors.push("at least one module is required");
  }
  for (const module of manifest.modules ?? []) {
    if (!module.id) {
      errors.push("module.id is required");
    }
    if (!module.type) {
      errors.push(`module ${module.id}: type is required`);
    }
    if (!module.name?.default) {
      errors.push(`module ${module.id}: name.default is required`);
    }
    if (module.runtime && !module.runtime.type) {
      errors.push(`module ${module.id}: runtime.type is required`);
    }
    for (const contribution of module.contributions ?? []) {
      if (contribution.kind !== "ui_provider") continue;
      const spec = contribution.spec as Partial<UIProviderDefinition> | undefined;
      if (!spec?.providerId) errors.push(`module ${module.id} contribution ${contribution.id}: ui_provider.providerId is required`);
      if (!spec?.capability) errors.push(`module ${module.id} contribution ${contribution.id}: ui_provider.capability is required`);
      if (!spec?.entries || Object.keys(spec.entries).length === 0) {
        errors.push(`module ${module.id} contribution ${contribution.id}: ui_provider.entries is required`);
      }
    }
  }
  if (!manifest.integrity?.algorithm) {
    errors.push("integrity.algorithm is required");
  }
  return errors;
}
