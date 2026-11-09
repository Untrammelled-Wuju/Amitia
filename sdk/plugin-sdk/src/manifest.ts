import type { LocalizedText, PermissionRequirement, ScopeRule } from "./types";

export const PLUGIN_SCHEMA_VERSION = 1;

export interface PluginManifest {
  schemaVersion: typeof PLUGIN_SCHEMA_VERSION;
  plugin: PluginMeta;
  compatibility: PluginCompatibility;
  modules: PluginModule[];
  dependencies?: PluginDependency[];
  permissions?: ManifestPermission[];
  resources?: ManifestResource[];
  secretRefs?: SecretRef[];
  adapterProviders?: AdapterProviderDeclaration[];
  data?: DataContract;
  integrity: PluginIntegrity;
  development?: PluginDevelopment;
}

export interface PluginMeta {
  id: string;
  name: LocalizedText;
  description?: LocalizedText;
  version: string;
  license?: string;
  homepage?: string;
  repository?: string;
  publisher: PluginPublisher;
  categories?: string[];
  keywords?: string[];
  icon?: string;
  metadata?: Record<string, unknown>;
}

export interface PluginPublisher {
  id: string;
  displayName: string;
  trustLevel?: string;
  contact?: string;
  website?: string;
}

export interface PluginCompatibility {
  minHostVersion: string;
  maxHostVersion?: string;
  platforms?: string[];
  featureFlags?: string[];
}

export interface PluginModule {
  id: string;
  name: LocalizedText;
  description?: LocalizedText;
  version?: string;
  runtime?: PluginRuntime;
  services?: ServiceDeclaration;
  dynamic?: DynamicCapability;
  contributions?: ManifestContribution[];
  permissions?: ManifestPermission[];
  resources?: ManifestResource[];
  dependencies?: PluginDependency[];
  placement?: ServicePlacement;
  compatibility?: ModuleCompatibility;
  policies?: ModulePolicies;
  deviceRequirements?: DeviceRequirements;
  providedCapabilities?: ProvidedCapability[];
  provider?: ProviderMetadata;
}

export interface PluginRuntime {
  adapter: AdapterID;
  serviceId?: string;
  entry?: string;
  workerCount?: number;
  timeout?: string;
  memoryBytes?: number;
  cpuPercent?: number;
  maxProcesses?: number;
  permissions?: string[];
  capabilities?: Record<string, boolean>;
  environment?: Record<string, string>;
  hostAccess?: RuntimeHostAccess;
  nativeArtifacts?: NativeArtifact[];
}

export interface RuntimeHostAccess {
  filesystem?: {
    mode?: "none" | "read" | "read_write";
    roots?: Array<"package" | "plugin_data">;
  };
  network?: {
    mode?: "none" | "host";
  };
  process?: {
    mode?: "none" | "direct";
    maxProcesses?: number;
  };
}

export interface NativeArtifact {
  id: string;
  platform: string;
  architecture?: string;
  path: string;
  sha256: string;
  executable: boolean;
  arguments?: string[];
}

export interface ModuleCompatibility {
  minHostVersion?: string;
  platforms?: string[];
}

export interface ModulePolicies {
  isolation?: "none" | "process" | "sandbox";
  networkAccess?: boolean;
  fileSystemAccess?: boolean;
}

export interface DeviceRequirements {
  platforms?: string[];
  architectures?: string[];
  minAppVersion?: string;
  minRuntimeVersion?: string;
  requiredFeatures?: string[];
}

export interface ProvidedCapability {
  id: string;
  version?: string;
  metadata?: Record<string, unknown>;
}

export interface ProviderMetadata {
  id?: string;
  priority?: number;
  labels?: Record<string, string>;
  metadata?: Record<string, unknown>;
}

export interface ServiceDeclaration {
  provides?: ServiceOffer[];
  requires?: ServiceRequirement[];
}

export interface ServiceOffer {
  serviceId: string;
  version: string;
  contractHash?: string;
  scopes: ServiceScope[];
  placement: ServicePlacement;
  priority?: number;
}

export interface ServiceRequirement {
  serviceId: string;
  versionRange?: string;
  contractHash?: string;
  optional?: boolean;
  selection?: ServiceSelection;
  reason?: string;
}

export type ServiceScope = "global" | "character" | "conversation" | "device" | "plugin";
export type ServiceSelection = "single" | "all" | "exclusive";
export type ServicePlacement = "core" | "device" | "cloud" | "hybrid";

export interface DynamicCapability {
  allowedAdapters?: AdapterID[];
  allowedKinds?: string[];
  maxContributions?: number;
  maxTools?: number;
  maxRoutes?: number;
  maxServices?: number;
  allowUnregister?: boolean;
}

export interface ManifestContribution {
  id: string;
  kind: string;
  adapter: AdapterID;
  contractVersion: number;
  schema?: Record<string, unknown>;
  name: LocalizedText;
  description?: LocalizedText;
  version?: string;
  spec?: Record<string, unknown>;
  requiredPermissions?: string[];
  requiredScope?: string[];
  exposure?: ContributionExposure;
  runtimeBinding?: ContributionRuntimeBinding;
  dependencies?: PluginDependency[];
  metadata?: Record<string, unknown>;
}

export interface ContributionExposure {
  visibleByDefault?: boolean;
  hiddenFromDiscovery?: boolean;
  requiredRoles?: string[];
}

export interface ContributionRuntimeBinding {
  runtimeId: string;
  generation?: number;
}

export function defineContribution<const T extends ManifestContribution>(contribution: T): T {
  return contribution;
}

export interface CustomManifestContribution<
  Kind extends string = string,
  Spec extends Record<string, unknown> = Record<string, unknown>,
> extends ManifestContribution {
  kind: Kind;
  adapter: ContributionAdapter;
  contractVersion: number;
  schema: Record<string, unknown>;
  spec: Spec;
}

export function defineCustomContribution<
  const Kind extends string,
  const Spec extends Record<string, unknown>,
>(contribution: CustomManifestContribution<Kind, Spec>): CustomManifestContribution<Kind, Spec> {
  return contribution;
}

export type ContributionAdapter =
  | "tool"
  | "agent_skill"
  | "workflow"
  | "mcp_server"
  | "hook"
  | "event"
  | "schedule"
  | "ui"
  | "ui_provider"
  | "ui_slot"
  | "background"
  | "provider"
  | "game"
  | "desktop_pet";

export type AdapterID = ContributionAdapter | (string & {});

export interface PluginDependency {
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

export interface SecretRef {
  ref: string;
  serviceId?: string;
  purpose?: string;
  description?: string;
  required?: boolean;
}

export interface DataContract {
  schemaVersion: number;
  scopes?: string[];
  backup?: boolean;
  export?: boolean;
  uninstall?: "retain" | "delete" | "export";
}

export interface PluginIntegrity {
  algorithm: "sha256";
  contentTreeHash: string;
  fileHashes?: Record<string, string>;
}

export interface PluginDevelopment {
  hotReload?: boolean;
  sourceMaps?: boolean;
  testEntry?: string;
  watchPaths?: string[];
}

export interface AdapterDescriptor {
  adapterId: AdapterID;
  contractVersion: number;
  supportedKinds: string[];
  contributionSchema: Record<string, unknown>;
  requiredHostCapabilities?: string[];
  permissionRequirements?: string[];
  platforms?: string[];
  trust: string;
  external?: boolean;
}

export interface AdapterProviderDeclaration {
  serviceId: string;
  adapterId: AdapterID;
  contractVersion: number;
  supportedKinds: string[];
  operations?: string[];
  contributionSchema: Record<string, unknown>;
  requiredHostCapabilities?: string[];
  permissionRequirements?: string[];
  platforms?: string[];
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
  metadata?: Record<string, unknown>;
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
  | "app.shell" | "app.navigation" | "app.workspace" | "app.workspace.sidebar"
  | "route.registry" | "page.provider"
  | "conversation.shell" | "conversation.header" | "conversation.messages"
  | "conversation.message_renderer" | "conversation.sidebar" | "conversation.composer" | "conversation.overlay"
  | "character.shell" | "character.detail" | "memory.shell" | "memory.detail"
  | "settings.shell" | "settings.section" | "extension.center" | "extension.page"
  | "workspace.tab"
  | "ui.theme" | "ui.tokens" | "ui.icons" | "ui.components";

export type UIProviderEntryType =
  | "builtin_native" | "declarative" | "web_module" | "schema_renderer" | "external_web_ui";

export interface UIProviderEntry {
  contributionId?: string;
  type: UIProviderEntryType;
  path?: string;
  schemaPath?: string;
  stylePath?: string;
  exportName?: string;
  contentHash?: string;
  styleHash?: string;
  url?: string;
  allowedOrigins?: string[];
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
  containerId?: string;
  sidebarProviderId?: string;
  sidebarItems?: Array<{ label: string; route: string }>;
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
  spaceId?: string;
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
  sandbox?: "host_native" | "schema_renderer" | "client_module";
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

export function validateManifest(manifest: PluginManifest): string[] {
  const errors: string[] = [];
  if (manifest.schemaVersion !== PLUGIN_SCHEMA_VERSION) {
    errors.push(`schemaVersion must be ${PLUGIN_SCHEMA_VERSION}`);
  }
  if (!manifest.plugin?.id) errors.push("plugin.id is required");
  if (!manifest.plugin?.publisher?.id) errors.push("plugin.publisher.id is required");
  if (!manifest.plugin?.publisher?.displayName) errors.push("plugin.publisher.displayName is required");
  if (!manifest.plugin?.name?.default) errors.push("plugin.name.default is required");
  if (!manifest.plugin?.version) errors.push("plugin.version is required");
  if (!manifest.compatibility?.minHostVersion) errors.push("compatibility.minHostVersion is required");
  if (!manifest.modules || manifest.modules.length === 0) {
    errors.push("at least one module is required");
  }
  const moduleIDs = new Set<string>();
  const contributionIDs = new Set<string>();
  for (const module of manifest.modules ?? []) {
    if (!module.id) {
      errors.push("module.id is required");
    } else if (moduleIDs.has(module.id)) {
      errors.push(`module ${module.id}: duplicate module id`);
    }
    moduleIDs.add(module.id);
    if (!module.name?.default) {
      errors.push(`module ${module.id}: name.default is required`);
    }
    if (!module.runtime && (module.contributions?.length ?? 0) === 0 && !module.services?.provides?.length) {
      errors.push(`module ${module.id}: runtime, contribution or service offer is required`);
    }
    if (module.runtime && !module.runtime.adapter) {
      errors.push(`module ${module.id}: runtime.adapter is required`);
    }
    if (module.runtime?.entry && isUnsafeRelativePath(module.runtime.entry)) {
      errors.push(`module ${module.id}: runtime.entry must stay inside the package`);
    }
    if (module.runtime?.workerCount !== undefined && (!Number.isInteger(module.runtime.workerCount) || module.runtime.workerCount < 0)) {
      errors.push(`module ${module.id}: runtime.workerCount must be a non-negative integer`);
    }
    for (const [index, offer] of (module.services?.provides ?? []).entries()) {
      if (!offer.serviceId) errors.push(`module ${module.id}: services.provides[${index}].serviceId is required`);
      if (!offer.version) errors.push(`module ${module.id}: services.provides[${index}].version is required`);
      if (!offer.scopes?.length) errors.push(`module ${module.id}: services.provides[${index}].scopes is required`);
      if (!offer.placement) errors.push(`module ${module.id}: services.provides[${index}].placement is required`);
    }
    for (const [index, requirement] of (module.services?.requires ?? []).entries()) {
      if (!requirement.serviceId) errors.push(`module ${module.id}: services.requires[${index}].serviceId is required`);
    }
    for (const key of ["maxContributions", "maxTools", "maxRoutes", "maxServices"] as const) {
      const value = module.dynamic?.[key];
      if (value !== undefined && (!Number.isInteger(value) || value < 0)) {
        errors.push(`module ${module.id}: dynamic.${key} must be a non-negative integer`);
      }
    }
    const capabilityIDs = new Set<string>();
    for (const [index, capability] of (module.providedCapabilities ?? []).entries()) {
      if (!capability.id) errors.push(`module ${module.id}: providedCapabilities[${index}].id is required`);
      if (capabilityIDs.has(capability.id)) {
        errors.push(`module ${module.id}: providedCapabilities[${index}].id is duplicated`);
      }
      capabilityIDs.add(capability.id);
    }
    if (module.provider && !module.provider.id) {
      errors.push(`module ${module.id}: provider.id cannot be empty when provider metadata is declared`);
    }
    for (const contribution of module.contributions ?? []) {
      if (!contribution.id) errors.push(`module ${module.id}: contribution id is required`);
      if (contributionIDs.has(contribution.id)) {
        errors.push(`module ${module.id} contribution ${contribution.id}: duplicate contribution id`);
      }
      contributionIDs.add(contribution.id);
      if (!contribution.adapter) errors.push(`module ${module.id} contribution ${contribution.id}: adapter is required`);
      if (!contribution.contractVersion || contribution.contractVersion < 1) {
        errors.push(`module ${module.id} contribution ${contribution.id}: positive contractVersion is required`);
      }
      if (!contribution.kind) errors.push(`module ${module.id} contribution ${contribution.id}: kind is required`);
      if (!contribution.name?.default) errors.push(`module ${module.id} contribution ${contribution.id}: name.default is required`);
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
  if (manifest.integrity?.algorithm !== "sha256") {
    errors.push("integrity.algorithm must be sha256");
  }
  for (const [index, secret] of (manifest.secretRefs ?? []).entries()) {
    if (!secret.ref) errors.push(`secretRefs[${index}].ref is required`);
  }
  const providerServices = new Set<string>();
  const providerAdapters = new Set<string>();
  for (const [index, provider] of (manifest.adapterProviders ?? []).entries()) {
    if (!provider.serviceId) errors.push(`adapterProviders[${index}].serviceId is required`);
    if (providerServices.has(provider.serviceId)) errors.push(`adapterProviders[${index}].serviceId is duplicated`);
    providerServices.add(provider.serviceId);
    if (!provider.adapterId) errors.push(`adapterProviders[${index}].adapterId is required`);
    if (!provider.contractVersion || provider.contractVersion < 1) {
      errors.push(`adapterProviders[${index}].contractVersion must be positive`);
    }
    const adapterKey = `${provider.adapterId}@${provider.contractVersion}`;
    if (providerAdapters.has(adapterKey)) errors.push(`adapterProviders[${index}].adapterId is duplicated`);
    providerAdapters.add(adapterKey);
    if (!provider.supportedKinds?.length) errors.push(`adapterProviders[${index}].supportedKinds is required`);
    const operations = provider.operations ?? [];
    if (operations.some((operation) => !operation.trim())) {
      errors.push(`adapterProviders[${index}].operations must not contain empty values`);
    }
    if (new Set(operations).size !== operations.length) {
      errors.push(`adapterProviders[${index}].operations is duplicated`);
    }
    if (!provider.contributionSchema || typeof provider.contributionSchema !== "object") {
      errors.push(`adapterProviders[${index}].contributionSchema must be an object`);
    }
  }
  return errors;
}

function isUnsafeRelativePath(value: string): boolean {
  const normalized = value.replaceAll("\\", "/");
  return normalized.startsWith("/") || normalized === ".." || normalized.startsWith("../") || normalized.includes("/../");
}
