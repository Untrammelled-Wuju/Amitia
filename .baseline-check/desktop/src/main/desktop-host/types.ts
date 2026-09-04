export interface DesktopContributionDef {
  contributionId: string;
  extensionId: string;
  moduleId: string;
  desktopType:
    | "app.menu.item"
    | "app.menu.submenu"
    | "app.tray.item"
    | "app.tray.submenu"
    | "app.shortcut.application"
    | "app.shortcut.global";
  contractId: string;
  contractVersion: number;
  target: string;
  order: { group?: string; priority: number; before?: string; after?: string };
  label: { default: string; translations?: Record<string, string> };
  description?: { default: string };
  iconResourceId?: string;
  visibility?: {
    platform?: string[];
    windowFocused?: boolean;
    extensionEnabled?: boolean;
    runtimeReady?: boolean;
    permissionGranted?: string[];
  };
  enabledRule?: {
    platform?: string[];
    extensionEnabled?: boolean;
    runtimeReady?: boolean;
  };
  action: { actionType: string; targetId?: string; input?: any };
  shortcut?: {
    accelerator: string;
    scope?: string;
    global: boolean;
    repeatable?: boolean;
  };
  permissionRequirements?: {
    permissionId: string;
    scope?: string;
    required: boolean;
  }[];
  definitionHash: string;
  version: string;
}

export interface ResolvedContribution {
  definition: DesktopContributionDef;
  status:
    | "declared"
    | "pending_permission"
    | "registered"
    | "conflict"
    | "unsupported"
    | "disabled"
    | "failed"
    | "quarantined";
  generation: number;
  effectiveLabel: string;
  effectiveIcon?: string;
  resolvedAt: string;
  conflictReason?: string;
}

export interface DesktopSnapshot {
  generation: number;
  contributions: ResolvedContribution[];
  hash: string;
  createdAt: string;
  menuTree: Record<string, ResolvedContribution[]>;
  trayTree: Record<string, ResolvedContribution[]>;
  shortcuts: ResolvedContribution[];
  conflicts: ConflictRecord[];
}

export interface ConflictRecord {
  conflictId: string;
  type: string;
  severity: string;
  target: string;
  existingContribId: string;
  newContribId: string;
  accelerator?: string;
  resolved: boolean;
  resolution?: string;
  createdAt: string;
}

export interface ActionInvokeRequest {
  contributionId: string;
  extensionId: string;
  scope?: {
    characterId?: string;
    conversationId?: string;
    extensionId?: string;
    global?: boolean;
  };
  input?: any;
}

export interface ActionInvokeResult {
  success: boolean;
  result?: any;
  error?: string;
}
