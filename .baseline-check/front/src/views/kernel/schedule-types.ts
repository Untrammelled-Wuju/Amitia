export type TriggerType = "cron" | "interval" | "one_shot";
export type TargetType = "tool" | "workflow" | "task" | "runtime_handler";
export type ScheduleDefinitionStatus = "created" | "enabled" | "disabled" | "paused" | "expired" | "uninstalled";
export type ScheduleRunStatus = "waiting" | "due" | "leased" | "triggering" | "running" | "retry_wait" | "completed" | "failed" | "expired" | "blocked" | "skipped" | "cancelled" | "quarantined";
export type MisfirePolicyType = "skip" | "fire_once" | "catch_up_limited" | "reschedule_from_now";
export type OverlapPolicyType = "forbid" | "allow" | "replace" | "queue_one" | "skip_if_running";
export type CircuitState = "closed" | "open" | "half_open";

export interface CronTriggerDefinition { expression: string; seconds: boolean; }
export interface IntervalTriggerDefinition { interval: number; anchorAt: string; }
export interface OneShotTriggerDefinition { runAt: string; }
export interface ScheduleTriggerDefinition {
  type: TriggerType;
  cron?: CronTriggerDefinition;
  interval?: IntervalTriggerDefinition;
  oneShot?: OneShotTriggerDefinition;
}
export interface ScheduleTargetDefinition {
  type: TargetType;
  targetId: string;
  inputTemplate?: any;
  idempotencyMode: string;
}
export interface ScheduleMisfirePolicy { policy: MisfirePolicyType; maxCatchUp: number; }
export interface ScheduleOverlapPolicy { policy: OverlapPolicyType; }
export interface ScheduleRetryPolicy {
  maxAttempts: number;
  initialBackoff: number;
  maxBackoff: number;
  multiplier: number;
  jitter: number;
  retryableErrorCodes?: string[];
}
export interface ScheduleJitterPolicy { enabled: boolean; maxDelay: number; seedMode: string; }
export interface ScheduleConcurrencyPolicy { maxConcurrentRuns: number; perExtensionLimit: number; perTargetLimit: number; }
export interface PermissionRequirement { permission: string; reason?: string; required: boolean; scope?: any; }
export interface ScopeRule { scopeType: string; scopeIds?: string[]; namespaces?: string[]; }
export interface DependencyRequirement { type: string; id: string; optional: boolean; }

export interface ScheduleContributionDefinition {
  contributionId: string;
  extensionId: string;
  moduleId: string;
  scheduleId: string;
  name: string;
  description: string;
  trigger: ScheduleTriggerDefinition;
  target: ScheduleTargetDefinition;
  timezone: string;
  startAt?: string;
  endAt?: string;
  enabledByDefault: boolean;
  misfirePolicy: ScheduleMisfirePolicy;
  overlapPolicy: ScheduleOverlapPolicy;
  retryPolicy: ScheduleRetryPolicy;
  jitterPolicy: ScheduleJitterPolicy;
  concurrencyPolicy: ScheduleConcurrencyPolicy;
  permissionRequirements?: PermissionRequirement[];
  scopeRule: ScopeRule;
  dependencyRequirements?: DependencyRequirement[];
  dstSpringPolicy?: string;
  dstFallPolicy?: string;
  definitionHash: string;
  version: string;
}

export interface ScheduleState {
  scheduleId: string;
  enabled: boolean;
  paused: boolean;
  status: ScheduleDefinitionStatus;
  lastScheduledAt?: string;
  lastTriggeredAt?: string;
  lastFinishedAt?: string;
  nextScheduledAt?: string;
  nextEffectiveAt?: string;
  lastResult?: string;
  failureCount: number;
  generation: number;
  updatedAt: string;
}

export interface ScheduleDetail {
  definition: ScheduleContributionDefinition;
  state: ScheduleState;
}

export interface ScheduleTriggerRecord {
  triggerId: string;
  scheduleId: string;
  scheduledAt: string;
  effectiveAt: string;
  triggeredAt?: string;
  idempotencyKey: string;
  status: ScheduleRunStatus;
  leaseOwner?: string;
  leaseExpiresAt?: string;
  attempt: number;
  generation: number;
  manual: boolean;
  errorCode?: string;
  errorMessage?: string;
  misfireDecision?: string;
  overlapDecision?: string;
  dstDecision?: string;
  createdAt: string;
  updatedAt: string;
}

export interface ScheduleRunRecord {
  runId: string;
  triggerId: string;
  scheduleId: string;
  status: ScheduleRunStatus;
  attempt: number;
  startedAt: string;
  finishedAt?: string;
  operationId?: string;
  invocationId?: string;
  targetType: TargetType;
  targetId: string;
  errorCode?: string;
  errorMessage?: string;
  generation: number;
  createdAt: string;
  updatedAt: string;
}

export interface ScheduleMisfireRecord {
  misfireId: string;
  scheduleId: string;
  scheduledAt: string;
  detectedAt: string;
  policy: MisfirePolicyType;
  action: string;
  skippedCount: number;
  detail?: string;
}

export interface ScheduleCircuitRecord {
  scheduleId: string;
  state: CircuitState;
  consecutiveFails: number;
  totalFails: number;
  totalSuccess: number;
  lastFailCode?: string;
  lastFailTime?: string;
  openedAt?: string;
  updatedAt: string;
}

export interface ScheduleQuarantineRecord {
  quarantineId: string;
  scheduleId: string;
  reason: string;
  detail: string;
  quarantinedAt: string;
  releasedAt?: string;
}
