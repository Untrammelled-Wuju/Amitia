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
  | "ui_contribution";

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
  }
  if (!manifest.integrity?.algorithm) {
    errors.push("integrity.algorithm is required");
  }
  return errors;
}
