export type HookPhase = "before" | "filter" | "transform" | "observe" | "after";

export type HookDecision = "continue" | "deny" | "replace" | "reject";

export interface MutationOperation {
  op: "add" | "remove" | "replace" | "move" | "copy" | "test";
  path: string;
  value?: unknown;
  from?: string;
}

export interface HookResult {
  decision: HookDecision;
  patch?: MutationOperation[];
  metadata?: Record<string, unknown>;
}

export interface HookContext {
  invocationId: string;
  operationId: string;
  extensionId: string;
  userId: string;
  characterId: string;
  conversationId: string;
  sessionId: string;
  channel: string;
  timestamp: string;
}

export interface HookHandler {
  (payload: unknown, ctx: HookContext): HookResult | Promise<HookResult>;
}

export interface HookContributionConfig {
  contributionId: string;
  hookPointId: string;
  contractVersion: number;
  phase: HookPhase;
  priority: number;
  entry: string;
  before?: string[];
  after?: string[];
  timeout?: number;
  failurePolicy?: {
    onRuntimeError?: string;
    onTimeout?: string;
    onInvalidResult?: string;
    onPermissionDenied?: string;
    disableAfterConsecutiveFailures?: number;
  };
  mutationClaims?: string[];
  permissionRequirements?: Array<{
    permissionID: string;
    reason: string;
    required: boolean;
  }>;
  scopeRule?: {
    scopeType: string;
    scopeID: string;
    reason: string;
  };
  dependencyRequirements?: Array<{
    type: string;
    id: string;
    version?: string;
    optional: boolean;
    reason?: string;
  }>;
}

export function defineHook(config: HookContributionConfig) {
  return config;
}

export function continueResult(metadata?: Record<string, unknown>): HookResult {
  return { decision: "continue", metadata };
}

export function denyResult(reason: string): HookResult {
  return { decision: "deny", metadata: { reason } };
}

export function replaceResult(patch: MutationOperation[], metadata?: Record<string, unknown>): HookResult {
  return { decision: "replace", patch, metadata };
}

export function rejectResult(reason: string): HookResult {
  return { decision: "reject", metadata: { reason } };
}

export function addOp(path: string, value: unknown): MutationOperation {
  return { op: "add", path, value };
}

export function removeOp(path: string): MutationOperation {
  return { op: "remove", path };
}

export function replaceOp(path: string, value: unknown): MutationOperation {
  return { op: "replace", path, value };
}

export function testOp(path: string, value: unknown): MutationOperation {
  return { op: "test", path, value };
}

export const HookPoints = {
  MESSAGE_BEFORE_SEND: "message.before_send/1",
  MESSAGE_BEFORE_PERSIST: "message.before_persist/1",
  MESSAGE_AFTER_PERSIST: "message.after_persist/1",
  MODEL_BEFORE_REQUEST: "model.before_request/1",
  MODEL_AFTER_RESPONSE: "model.after_response/1",
  PROMPT_BEFORE_ASSEMBLE: "prompt.before_assemble/1",
  PROMPT_AFTER_ASSEMBLE: "prompt.after_assemble/1",
  TOOL_BEFORE_EXECUTE: "tool.before_execute/1",
  TOOL_AFTER_EXECUTE: "tool.after_execute/1",
  WORKFLOW_BEFORE_START: "workflow.before_start/1",
  WORKFLOW_AFTER_FINISH: "workflow.after_finish/1",
} as const;

export type HookPointID = typeof HookPoints[keyof typeof HookPoints];

export function createHookRegistry() {
  const handlers = new Map<string, HookHandler>();
  const contributions = new Map<string, HookContributionConfig>();

  function register(config: HookContributionConfig, handler: HookHandler) {
    contributions.set(config.contributionId, config);
    handlers.set(config.contributionId, handler);
    return () => {
      handlers.delete(config.contributionId);
      contributions.delete(config.contributionId);
    };
  }

  function getHandler(id: string): HookHandler | undefined {
    return handlers.get(id);
  }

  function list(): HookContributionConfig[] {
    return Array.from(contributions.values());
  }

  return { register, getHandler, list };
}

export type HookRegistry = ReturnType<typeof createHookRegistry>;
