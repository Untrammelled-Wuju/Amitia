import type { LocalizedText, PermissionRequirement, ScopeRule, IntegrityReference } from "./types";

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
  extensionId: string;
  publisher: string;
  displayName: LocalizedText;
  description: LocalizedText;
  version: string;
  contractVersion: number;
  modules: ModuleDeclaration[];
  permissions?: PermissionRequirement[];
  scopeRule?: ScopeRule;
  integrity?: IntegrityReference;
  icon?: string;
  homepage?: string;
  repository?: string;
  license?: string;
  category?: string;
  keywords?: string[];
  platforms?: Array<"windows" | "macos" | "linux" | "web">;
  minHostVersion?: string;
}

export interface ModuleDeclaration {
  moduleId: string;
  kind: ModuleKind;
  entry: string;
  runtime?: string;
  displayName?: LocalizedText;
  description?: LocalizedText;
  tools?: string[];
  skills?: string[];
  workflows?: string[];
  mcpServers?: string[];
  uiContributions?: string[];
  permissions?: PermissionRequirement[];
  scopeRule?: ScopeRule;
  integrity?: IntegrityReference;
  deprecated?: boolean;
  deprecationNote?: string;
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
  if (!manifest.extensionId) {
    errors.push("extensionId is required");
  }
  if (!manifest.publisher) {
    errors.push("publisher is required");
  }
  if (!manifest.displayName?.default) {
    errors.push("displayName.default is required");
  }
  if (!manifest.version) {
    errors.push("version is required");
  }
  if (!manifest.modules || manifest.modules.length === 0) {
    errors.push("at least one module is required");
  }
  for (const module of manifest.modules ?? []) {
    if (!module.moduleId) {
      errors.push("module.moduleId is required");
    }
    if (!module.kind) {
      errors.push(`module ${module.moduleId}: kind is required`);
    }
    if (!module.entry) {
      errors.push(`module ${module.moduleId}: entry is required`);
    }
  }
  return errors;
}
