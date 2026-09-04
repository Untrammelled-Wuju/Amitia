import { AmitiaError, ValidationError } from "./errors";
import type { JSONSchema } from "./manifest";

export type HookPhase = "before" | "filter" | "transform" | "observe";

export interface HookContext {
  readonly phase: HookPhase;
  readonly pipeline: string;
  readonly stage: string;
  readonly traceId: string;
  readonly deadline?: number;
  readonly signal?: AbortSignal;
  readonly logger: HookLogger;
  abort(): void;
}

export interface HookLogger {
  debug(message: string, fields?: Record<string, unknown>): void;
  info(message: string, fields?: Record<string, unknown>): void;
  warn(message: string, fields?: Record<string, unknown>): void;
  error(message: string, fields?: Record<string, unknown>): void;
}

export interface BeforeHookResult<I> {
  proceed: boolean;
  input?: I;
  vetoReason?: string;
  metadata?: Record<string, unknown>;
}

export interface FilterHookResult {
  allowed: boolean;
  reason?: string;
  sanitized?: unknown;
  metadata?: Record<string, unknown>;
}

export type BeforeHook<I> = (input: I, ctx: HookContext) => Promise<BeforeHookResult<I>> | BeforeHookResult<I>;
export type FilterHook<I> = (input: I, ctx: HookContext) => Promise<FilterHookResult> | FilterHookResult;
export type TransformHook<I, O> = (input: I, ctx: HookContext) => Promise<O> | O;
export type ObserveHook<I> = (input: I, ctx: HookContext) => Promise<void> | void;

export interface HookRegistration<I = unknown, O = unknown> {
  readonly pipeline: string;
  readonly stage: string;
  readonly phase: HookPhase;
  readonly priority: number;
  readonly handler: HookHandler<I, O>;
  readonly inputSchema?: JSONSchema;
  readonly outputSchema?: JSONSchema;
  readonly timeoutMs?: number;
  readonly deprecated?: boolean;
  readonly deprecationNote?: string;
}

export type HookHandler<I, O> =
  | BeforeHook<I>
  | FilterHook<I>
  | TransformHook<I, O>
  | ObserveHook<I>;

const hookRegistry = new Map<string, HookRegistration>();

function hookKey(pipeline: string, stage: string, phase: HookPhase, index: number): string {
  return `${pipeline}:${stage}:${phase}:${index}`;
}

export function defineBeforeHook<I>(
  pipeline: string,
  stage: string,
  handler: BeforeHook<I>,
  options?: HookOptions,
): HookRegistration<I, void> {
  return registerHook<I, void>({
    pipeline,
    stage,
    phase: "before",
    priority: options?.priority ?? 0,
    handler: handler as HookHandler<I, void>,
    inputSchema: options?.inputSchema,
    outputSchema: options?.outputSchema,
    timeoutMs: options?.timeoutMs,
    deprecated: options?.deprecated,
    deprecationNote: options?.deprecationNote,
  });
}

export function defineFilterHook<I>(
  pipeline: string,
  stage: string,
  handler: FilterHook<I>,
  options?: HookOptions,
): HookRegistration<I, void> {
  return registerHook<I, void>({
    pipeline,
    stage,
    phase: "filter",
    priority: options?.priority ?? 0,
    handler: handler as HookHandler<I, void>,
    inputSchema: options?.inputSchema,
    outputSchema: options?.outputSchema,
    timeoutMs: options?.timeoutMs,
    deprecated: options?.deprecated,
    deprecationNote: options?.deprecationNote,
  });
}

export function defineTransformHook<I, O>(
  pipeline: string,
  stage: string,
  handler: TransformHook<I, O>,
  options?: HookOptions,
): HookRegistration<I, O> {
  return registerHook<I, O>({
    pipeline,
    stage,
    phase: "transform",
    priority: options?.priority ?? 0,
    handler: handler as HookHandler<I, O>,
    inputSchema: options?.inputSchema,
    outputSchema: options?.outputSchema,
    timeoutMs: options?.timeoutMs,
    deprecated: options?.deprecated,
    deprecationNote: options?.deprecationNote,
  });
}

export function defineObserveHook<I>(
  pipeline: string,
  stage: string,
  handler: ObserveHook<I>,
  options?: HookOptions,
): HookRegistration<I, void> {
  return registerHook<I, void>({
    pipeline,
    stage,
    phase: "observe",
    priority: options?.priority ?? 0,
    handler: handler as HookHandler<I, void>,
    inputSchema: options?.inputSchema,
    outputSchema: options?.outputSchema,
    timeoutMs: options?.timeoutMs,
    deprecated: options?.deprecated,
    deprecationNote: options?.deprecationNote,
  });
}

export interface HookOptions {
  priority?: number;
  inputSchema?: JSONSchema;
  outputSchema?: JSONSchema;
  timeoutMs?: number;
  deprecated?: boolean;
  deprecationNote?: string;
}

function registerHook<I, O>(reg: Omit<HookRegistration<I, O>, never>): HookRegistration<I, O> {
  if (!reg.pipeline || !reg.stage) {
    throw new ValidationError("pipeline and stage are required");
  }
  if (reg.priority < -100 || reg.priority > 100) {
    throw new ValidationError("hook priority must be between -100 and 100");
  }
  const phaseIndex = nextPhaseIndex(reg.pipeline, reg.stage, reg.phase);
  const key = hookKey(reg.pipeline, reg.stage, reg.phase, phaseIndex);
  hookRegistry.set(key, reg as HookRegistration);
  return reg;
}

function nextPhaseIndex(pipeline: string, stage: string, phase: HookPhase): number {
  let count = 0;
  for (const k of hookRegistry.keys()) {
    if (k.startsWith(`${pipeline}:${stage}:${phase}:`)) count++;
  }
  return count;
}

export function listHooks(pipeline?: string, stage?: string, phase?: HookPhase): HookRegistration[] {
  const all = Array.from(hookRegistry.values());
  return all.filter((h) => {
    if (pipeline && h.pipeline !== pipeline) return false;
    if (stage && h.stage !== stage) return false;
    if (phase && h.phase !== phase) return false;
    return true;
  });
}

export function unregisterHook<I = unknown, O = unknown>(registration: HookRegistration<I, O>): boolean {
  for (const [key, value] of hookRegistry.entries()) {
    if (value === registration) {
      hookRegistry.delete(key);
      return true;
    }
  }
  return false;
}

export function clearHooks(): void {
  hookRegistry.clear();
}

export function isDeprecated(reg: HookRegistration): boolean {
  return reg.deprecated === true;
}

export function assertTransformOutput<O>(value: unknown, schema?: JSONSchema): O {
  if (!schema) return value as O;
  const errors = validateSchema(value, schema);
  if (errors.length > 0) {
    throw new ValidationError("transform output schema mismatch", {
      details: { errors },
    });
  }
  return value as O;
}

function validateSchema(value: unknown, schema: JSONSchema): string[] {
  const errors: string[] = [];
  if (schema.type) {
    const actual = Array.isArray(value) ? "array" : typeof value;
    if (actual !== schema.type) {
      errors.push(`expected ${schema.type}, got ${actual}`);
    }
  }
  return errors;
}

export function wrapHookError(cause: unknown, reg: HookRegistration): AmitiaError {
  if (cause instanceof AmitiaError) return cause;
  return new ValidationError(`hook ${reg.pipeline}:${reg.stage}:${reg.phase} failed`, {
    details: { error: cause instanceof Error ? cause.message : String(cause) },
  });
}
