export type TriggerType = "cron" | "interval" | "one_shot";

export type TargetType = "tool" | "workflow" | "task" | "runtime_handler";

export type IdempotencyMode = "idempotent" | "conditionally_idempotent" | "non_idempotent";

export type MisfirePolicyType = "skip" | "fire_once" | "catch_up_limited" | "reschedule_from_now";

export type OverlapPolicyType = "forbid" | "allow" | "replace" | "queue_one" | "skip_if_running";

export type DSTSpringPolicyType = "skip" | "fire_once_after_gap" | "next_valid_time";

export type DSTFallPolicyType = "fire_once_first" | "fire_once_second" | "fire_twice";

export interface CronTriggerConfig {
  expression: string;
  seconds?: boolean;
}

export interface IntervalTriggerConfig {
  intervalMs: number;
  anchorAt?: string;
}

export interface OneShotTriggerConfig {
  runAt: string;
}

export interface TriggerConfig {
  type: TriggerType;
  cron?: CronTriggerConfig;
  interval?: IntervalTriggerConfig;
  oneShot?: OneShotTriggerConfig;
}

export interface TargetConfig {
  type: TargetType;
  targetId: string;
  inputTemplate?: unknown;
  idempotencyMode?: IdempotencyMode;
}

export interface MisfirePolicyConfig {
  policy?: MisfirePolicyType;
  maxCatchUp?: number;
}

export interface OverlapPolicyConfig {
  policy?: OverlapPolicyType;
}

export interface RetryPolicyConfig {
  maxAttempts?: number;
  initialBackoffMs?: number;
  maxBackoffMs?: number;
  multiplier?: number;
  jitter?: number;
  retryableErrorCodes?: string[];
}

export interface JitterPolicyConfig {
  enabled?: boolean;
  maxDelayMs?: number;
  seedMode?: string;
}

export interface ConcurrencyPolicyConfig {
  maxConcurrentRuns?: number;
  perExtensionLimit?: number;
  perTargetLimit?: number;
}

export interface PermissionRequirementConfig {
  permission: string;
  reason?: string;
  required: boolean;
  scope?: unknown;
}

export interface ScopeRuleConfig {
  scopeType: string;
  scopeIds?: string[];
  namespaces?: string[];
}

export interface DependencyRequirementConfig {
  type: string;
  id: string;
  optional: boolean;
}

export interface ScheduleContributionConfig {
  contributionId: string;
  name: string;
  description: string;
  trigger: TriggerConfig;
  target: TargetConfig;
  timezone?: string;
  startAt?: string;
  endAt?: string;
  enabledByDefault?: boolean;
  misfirePolicy?: MisfirePolicyConfig;
  overlapPolicy?: OverlapPolicyConfig;
  retryPolicy?: RetryPolicyConfig;
  jitterPolicy?: JitterPolicyConfig;
  concurrencyPolicy?: ConcurrencyPolicyConfig;
  permissionRequirements?: PermissionRequirementConfig[];
  scopeRule?: ScopeRuleConfig;
  dependencyRequirements?: DependencyRequirementConfig[];
  dstSpringPolicy?: DSTSpringPolicyType;
  dstFallPolicy?: DSTFallPolicyType;
}

export function defineSchedule(config: ScheduleContributionConfig): ScheduleContributionConfig {
  return {
    timezone: "UTC",
    enabledByDefault: false,
    ...config,
  };
}

export function cronTrigger(expression: string, seconds?: boolean): TriggerConfig {
  return { type: "cron", cron: { expression, seconds } };
}

export function intervalTrigger(intervalMs: number, anchorAt?: string): TriggerConfig {
  return { type: "interval", interval: { intervalMs, anchorAt } };
}

export function oneShotTrigger(runAt: string): TriggerConfig {
  return { type: "one_shot", oneShot: { runAt } };
}

export function toolTarget(targetId: string, inputTemplate?: unknown, idempotencyMode?: IdempotencyMode): TargetConfig {
  return { type: "tool", targetId, inputTemplate, idempotencyMode: idempotencyMode || "idempotent" };
}

export function workflowTarget(targetId: string, inputTemplate?: unknown, idempotencyMode?: IdempotencyMode): TargetConfig {
  return { type: "workflow", targetId, inputTemplate, idempotencyMode: idempotencyMode || "idempotent" };
}

export function taskTarget(targetId: string, inputTemplate?: unknown, idempotencyMode?: IdempotencyMode): TargetConfig {
  return { type: "task", targetId, inputTemplate, idempotencyMode: idempotencyMode || "idempotent" };
}

export function runtimeHandlerTarget(targetId: string, inputTemplate?: unknown, idempotencyMode?: IdempotencyMode): TargetConfig {
  return { type: "runtime_handler", targetId, inputTemplate, idempotencyMode: idempotencyMode || "idempotent" };
}

export function defaultMisfirePolicy(): MisfirePolicyConfig {
  return { policy: "fire_once", maxCatchUp: 3 };
}

export function defaultOverlapPolicy(): OverlapPolicyConfig {
  return { policy: "forbid" };
}

export function defaultRetryPolicy(): RetryPolicyConfig {
  return { maxAttempts: 3, initialBackoffMs: 5000, maxBackoffMs: 300000, multiplier: 2.0, jitter: 0.1 };
}

export function defaultJitterPolicy(): JitterPolicyConfig {
  return { enabled: true, maxDelayMs: 30000, seedMode: "schedule_id_scheduled_at" };
}

export function defaultConcurrencyPolicy(): ConcurrencyPolicyConfig {
  return { maxConcurrentRuns: 1, perExtensionLimit: 4, perTargetLimit: 1 };
}
