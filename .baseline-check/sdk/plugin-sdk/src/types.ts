export interface ExtensionID {
  readonly publisher: string;
  readonly name: string;
  toString(): string;
}

export interface ModuleID {
  readonly extension: ExtensionID;
  readonly name: string;
  toString(): string;
}

export interface ContributionID {
  readonly module: ModuleID;
  readonly name: string;
  toString(): string;
}

export interface RuntimeDefinitionID {
  readonly module: ModuleID;
  readonly runtime: string;
  toString(): string;
}

export interface LocalizedText {
  default: string;
  translations?: Record<string, string>;
}

export interface PermissionRequirement {
  permission: string;
  reason?: string;
  required?: boolean;
}

export interface ScopeRule {
  scope: "global" | "character" | "conversation";
  requiresCharacter?: boolean;
  requiresConversation?: boolean;
}

export interface IntegrityReference {
  algorithm: "sha256" | "sha512";
  hash: string;
}

export interface ContributionIntegrity {
  contentHash: IntegrityReference;
  signature?: string;
  signAlgorithm?: string;
}

export function makeExtensionID(publisher: string, name: string): ExtensionID {
  return {
    publisher,
    name,
    toString: () => `${publisher}/${name}`,
  };
}

export function makeModuleID(extension: ExtensionID, name: string): ModuleID {
  return {
    extension,
    name,
    toString: () => `${extension.toString()}#${name}`,
  };
}

export function makeContributionID(module: ModuleID, name: string): ContributionID {
  return {
    module,
    name,
    toString: () => `${module.toString()}/contribution/${name}`,
  };
}
