import { apiClient } from "@/composables/useApi";

const BASE = "/api/extensions/events";

export type RiskLevel = "low" | "medium" | "high" | "critical";
export type EventOrderingRequirement = "none" | "per_partition" | "per_aggregate";
export type CircuitState = "closed" | "open" | "half_open";
export type DeliveryStatus =
  | "pending"
  | "leased"
  | "delivering"
  | "succeeded"
  | "retry_wait"
  | "failed"
  | "dead_letter"
  | "cancelled"
  | "skipped";
export type DeadLetterReason =
  | "max_attempts_exceeded"
  | "permanent_error"
  | "handler_not_found"
  | "subscription_invalid"
  | "permission_revoked"
  | "scope_invalid"
  | "extension_disabled"
  | "circuit_open"
  | "manual_discard";
export type DeadLetterStatus = "pending" | "replayed" | "discarded";
export type OutboxStatus = "pending" | "dispatching" | "dispatched" | "failed" | "dead_letter" | "cancelled";
export type ReplayStrategy =
  | "replay_same_subscription"
  | "replay_after_repair"
  | "replay_to_new_generation"
  | "discard";

export interface EventProducerPolicy {
  AllowedProducers: string[];
  RequireSystemTrust: boolean;
  RequireNamespaceMatch: boolean;
  MaxPayloadBytes: number;
  MaxMetadataBytes: number;
  RateLimitPerSecond: number;
}

export interface EventSubscriberPolicy {
  AllowThirdParty: boolean;
  MaxSubscribers: number;
  RequireApproval: boolean;
  AllowedFilterFields: string[];
  RequiredPermissions: string[];
  RequiredScope: string;
}

export interface EventDeliveryPolicy {
  Timeout: number;
  MaxAttempts: number;
  InitialBackoff: number;
  MaxBackoff: number;
  BackoffMultiplier: number;
  JitterFactor: number;
  OrderingRequirement: EventOrderingRequirement;
  MaxInFlight: number;
  RetryableErrorCodes: string[];
  NonRetryableErrorCodes: string[];
}

export interface EventRetentionPolicy {
  MaxAge: number;
  MaxDeliveryCount: number;
  DeleteAfterSuccess: boolean;
  DeleteAfterDeadLetter: boolean;
  ArchiveDeadLetters: boolean;
}

export interface SensitiveFieldRule {
  Path: string;
  Classification: string;
  DefaultAction: string;
  RequiredPermission: { Permission: string; Scope: string; Reason: string }[];
}

export interface EventProjectionRule {
  SourcePath: string;
  TargetPath: string;
  RequiredPermission: string;
  RequiresScope: string;
}

export interface EventTypeDefinition {
  EventTypeID: string;
  Version: number;
  Description: string;
  PayloadSchema: unknown;
  MetadataSchema: unknown;
  ProducerPolicy: EventProducerPolicy;
  SubscriberPolicy: EventSubscriberPolicy;
  DeliveryPolicy: EventDeliveryPolicy;
  OrderingPolicy: EventOrderingRequirement;
  RetentionPolicy: EventRetentionPolicy;
  SensitiveFields: SensitiveFieldRule[];
  ProjectionRules: EventProjectionRule[];
  MaxPayloadBytes: number;
  MaxMetadataBytes: number;
  RiskLevel: RiskLevel;
  DefinitionHash: string;
}

export interface CircuitStats {
  State: CircuitState;
  ConsecutiveFails: number;
  ConsecutiveSuccess: number;
  TotalFails: number;
  TotalSuccess: number;
  LastFailCode: string;
  LastFailTime: string;
  OpenedAt: string;
}

export interface ServiceStats {
  pendingOutbox: number;
  dispatchingOutbox: number;
  dispatchedOutbox: number;
  deadLetterOutbox: number;
  pendingDeliveries: number;
  leasedDeliveries: number;
  succeededDeliveries: number;
  failedDeliveries: number;
  retryWaitDeliveries: number;
  deadLetterDeliveries: number;
  cancelledDeliveries: number;
  skippedDeliveries: number;
  activeSubscriptions: number;
  circuits: Record<string, CircuitStats>;
}

export interface PublishEventRequest {
  eventTypeId: string;
  version: number;
  payload: unknown;
  producerId: string;
  producerType: string;
  aggregateType?: string;
  aggregateId?: string;
  partitionKey?: string;
  orderingKey?: string;
  traceId?: string;
  operationId?: string;
  parentEventId?: string;
  parentDepth?: number;
  metadata?: unknown;
}

export interface PublishResult {
  EventID: string;
  OutboxID: string;
  Accepted: boolean;
  RejectReason: string;
}

export interface Delivery {
  DeliveryID: string;
  EventID: string;
  SubscriptionID: string;
  ExtensionID: string;
  ModuleID: string;
  Status: DeliveryStatus;
  PartitionKey: string;
  OrderingKey: string;
  Sequence: number;
  Attempt: number;
  MaxAttempts: number;
  AvailableAt: string;
  LeaseOwner: string;
  LeaseExpiresAt: string | null;
  RuntimeInstanceID: string;
  ScopeSnapshotID: string;
  PermissionSnapshotID: string;
  ProjectedPayloadHash: string;
  StartedAt: string | null;
  FinishedAt: string | null;
  ErrorCode: string;
  ErrorMessage: string;
  CreatedAt: string;
  UpdatedAt: string;
}

export interface DeadLetterRecord {
  DeadLetterID: string;
  EventID: string;
  DeliveryID: string;
  SubscriptionID: string;
  ExtensionID: string;
  ModuleID: string;
  EventTypeID: string;
  EventVersion: number;
  Reason: DeadLetterReason;
  ErrorCode: string;
  ErrorMessage: string;
  Attempts: number;
  PartitionKey: string;
  OrderingKey: string;
  PayloadHash: string;
  ProjectedPayloadHash: string;
  DefinitionHash: string;
  ScopeSnapshotID: string;
  PermissionSnapshotID: string;
  RuntimeInstanceID: string;
  TraceID: string;
  OperationID: string;
  OriginEvent: unknown;
  SubscriptionSnapshot: unknown;
  CreatedAt: string;
  ReplayCount: number;
  LastReplayAt: string | null;
  Status: DeadLetterStatus;
}

export interface OutboxRecord {
  OutboxID: string;
  EventID: string;
  EventTypeID: string;
  EventVersion: number;
  ProducerID: string;
  ProducerType: string;
  ProducerGeneration: number;
  AggregateType: string;
  AggregateID: string;
  AggregateVersion: number | null;
  PartitionKey: string;
  OrderingKey: string;
  IdempotencyKey: string;
  ScopeSnapshotID: string;
  PermissionSnapshotID: string;
  TraceID: string;
  OperationID: string;
  ParentEventID: string | null;
  Depth: number;
  OccurredAt: string;
  PublishedAt: string | null;
  Payload: unknown;
  Metadata: unknown;
  PayloadHash: string;
  DefinitionHash: string;
  Status: OutboxStatus;
  AvailableAt: string;
  CreatedAt: string;
  UpdatedAt: string;
  ErrorCode: string;
  ErrorMessage: string;
  LeaseOwner: string;
  LeaseExpiresAt: string | null;
  DispatchedAt: string | null;
}

export interface EventAuditEntry {
  OperationID: string;
  InvocationID: string;
  EventID: string;
  DeliveryID: string;
  Action: string;
  Actor: string;
  ExtensionID: string;
  Timestamp: string;
  PayloadHash: string;
  ErrorCode: string;
  Success: boolean;
  Detail: unknown;
}

export interface ListResult<T> {
  items: T[];
  total: number;
}

function buildQuery(params: Record<string, unknown>): string {
  const sp = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== "") {
      sp.set(key, String(value));
    }
  }
  const qs = sp.toString();
  return qs ? `?${qs}` : "";
}

export async function listEventTypes(): Promise<ListResult<EventTypeDefinition>> {
  const res = await apiClient.get(`${BASE}/types`);
  return res.data;
}

export async function getEventType(typeId: string, version: number): Promise<EventTypeDefinition> {
  const res = await apiClient.get(`${BASE}/types/${encodeURIComponent(typeId)}/${encodeURIComponent(version)}`);
  return res.data;
}

export async function registerEventType(def: Partial<EventTypeDefinition>): Promise<{ message: string }> {
  const res = await apiClient.post(`${BASE}/types`, def);
  return res.data;
}

export async function publishEvent(body: PublishEventRequest): Promise<PublishResult> {
  const res = await apiClient.post(`${BASE}/publish`, body);
  return res.data;
}

export async function publishEventTx(body: PublishEventRequest): Promise<PublishResult> {
  const res = await apiClient.post(`${BASE}/publish-tx`, body);
  return res.data;
}

export async function listSubscriptions(
  extensionId?: string,
  eventTypeId?: string,
): Promise<ListResult<unknown>> {
  const qs = buildQuery({ extensionId, eventTypeId });
  const res = await apiClient.get(`${BASE}/subscriptions${qs}`);
  return res.data;
}

export async function getSubscription(contributionId: string): Promise<unknown> {
  const res = await apiClient.get(`${BASE}/subscriptions/${encodeURIComponent(contributionId)}`);
  return res.data;
}

export async function unregisterSubscription(contributionId: string): Promise<void> {
  await apiClient.delete(`${BASE}/subscriptions/${encodeURIComponent(contributionId)}`);
}

export async function listDeliveries(filter: {
  extensionId?: string;
  subscriptionId?: string;
  eventId?: string;
  status?: DeliveryStatus;
  limit?: number;
  offset?: number;
}): Promise<ListResult<Delivery>> {
  const qs = buildQuery(filter);
  const res = await apiClient.get(`${BASE}/deliveries${qs}`);
  return res.data;
}

export async function getDelivery(deliveryId: string): Promise<Delivery> {
  const res = await apiClient.get(`${BASE}/deliveries/${encodeURIComponent(deliveryId)}`);
  return res.data;
}

export async function listDeadLetters(filter: {
  extensionId?: string;
  subscriptionId?: string;
  reason?: DeadLetterReason;
  status?: DeadLetterStatus;
  limit?: number;
  offset?: number;
}): Promise<ListResult<DeadLetterRecord>> {
  const qs = buildQuery(filter);
  const res = await apiClient.get(`${BASE}/dead-letters${qs}`);
  return res.data;
}

export async function getDeadLetter(deadLetterId: string): Promise<DeadLetterRecord> {
  const res = await apiClient.get(`${BASE}/dead-letters/${encodeURIComponent(deadLetterId)}`);
  return res.data;
}

export async function replayDeadLetter(
  deadLetterId: string,
  body: { strategy?: ReplayStrategy; newSubscriptionId?: string; requestedBy?: string; reason?: string } = {},
): Promise<{ message: string }> {
  const payload = { strategy: "replay_same_subscription", ...body };
  const res = await apiClient.post(`${BASE}/dead-letters/${encodeURIComponent(deadLetterId)}/replay`, payload);
  return res.data;
}

export async function discardDeadLetter(deadLetterId: string): Promise<void> {
  await apiClient.post(`${BASE}/dead-letters/${encodeURIComponent(deadLetterId)}/discard`);
}

export async function getEventStats(): Promise<ServiceStats> {
  const res = await apiClient.get(`${BASE}/stats`);
  return res.data;
}

export async function listOutbox(filter: {
  extensionId?: string;
  status?: OutboxStatus;
  limit?: number;
  offset?: number;
}): Promise<ListResult<OutboxRecord>> {
  const qs = buildQuery(filter);
  const res = await apiClient.get(`${BASE}/outbox${qs}`);
  return res.data;
}

export async function listAudit(filter: {
  eventId?: string;
  deliveryId?: string;
  extensionId?: string;
  action?: string;
}): Promise<ListResult<EventAuditEntry>> {
  const qs = buildQuery(filter);
  const res = await apiClient.get(`${BASE}/audit${qs}`);
  return res.data;
}

export async function resetCircuit(subscriptionId: string): Promise<{ message: string }> {
  const res = await apiClient.post(`${BASE}/circuits/${encodeURIComponent(subscriptionId)}/reset`);
  return res.data;
}
