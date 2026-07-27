export type EventRiskLevel = "low" | "medium" | "high" | "critical";
export type EventOrderingRequirement = "none" | "per_partition" | "per_aggregate";

export interface EventPublishOptions {
  aggregateType?: string;
  aggregateId?: string;
  partitionKey?: string;
  orderingKey?: string;
  traceId?: string;
  operationId?: string;
  parentEventId?: string;
  parentDepth?: number;
  metadata?: Record<string, unknown>;
}

export interface EventPublishResult {
  eventId: string;
  outboxId: string;
  status: string;
  occurredAt: string;
}

export interface EventSubscriptionConfig {
  contributionId: string;
  eventTypeId: string;
  eventVersionRange?: string;
  entry: string;
  filter?: Record<string, unknown>;
  projection?: Record<string, unknown>;
  deliveryPolicy?: {
    timeout?: number;
    maxAttempts?: number;
    initialBackoff?: number;
    maxBackoff?: number;
    backoffMultiplier?: number;
    jitterFactor?: number;
  };
  orderingRequirement?: EventOrderingRequirement;
  timeout?: number;
  maxInFlight?: number;
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

export interface EventTypeDefinition {
  eventTypeId: string;
  version: number;
  description?: string;
  payloadSchema?: Record<string, unknown>;
  metadataSchema?: Record<string, unknown>;
  maxPayloadBytes?: number;
  maxMetadataBytes?: number;
  riskLevel?: EventRiskLevel;
}

export function defineEventType(config: EventTypeDefinition): EventTypeDefinition {
  return config;
}

export function defineSubscription(config: EventSubscriptionConfig): EventSubscriptionConfig {
  return config;
}

export interface EventHandler {
  (payload: unknown, ctx: EventDeliveryContext): void | Promise<void>;
}

export interface EventDeliveryContext {
  eventId: string;
  eventTypeId: string;
  eventVersion: number;
  deliveryId: string;
  subscriptionId: string;
  attempt: number;
  occurredAt: string;
  traceId?: string;
  operationId?: string;
  parentEventId?: string;
  depth: number;
}
