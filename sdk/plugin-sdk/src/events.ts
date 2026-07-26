import type { RuntimeScope } from "./runtime";
import type { JSONSchema } from "./manifest";

export interface EventEnvelope<T = unknown> {
  readonly eventId: string;
  readonly type: string;
  readonly payload: T;
  readonly timestamp: string;
  readonly source: string;
  readonly version: number;
  readonly traceId: string;
  readonly scope: RuntimeScope;
  readonly deliveryId: string;
  readonly idempotencyKey: string;
  readonly retryAttempt: number;
}

export interface EventDeliveryContext {
  readonly deadline?: number;
  readonly signal?: AbortSignal;
  readonly logger: EventLogger;
  ack(): Promise<void>;
  nack(reason?: string): Promise<void>;
}

export interface EventLogger {
  debug(message: string, fields?: Record<string, unknown>): void;
  info(message: string, fields?: Record<string, unknown>): void;
  warn(message: string, fields?: Record<string, unknown>): void;
  error(message: string, fields?: Record<string, unknown>): void;
}

export type EventHandler<T = unknown> = (
  event: EventEnvelope<T>,
  context: EventDeliveryContext,
) => Promise<void> | void;

export interface EventSubscriptionSpec {
  readonly eventType: string;
  readonly handler: EventHandler;
  readonly filter?: EventFilter;
  readonly maxRetries?: number;
  readonly timeoutMs?: number;
  readonly deadLetterPolicy?: "drop" | "park" | "requeue";
}

export interface EventFilter {
  readonly expression?: string;
  readonly payloadSchema?: JSONSchema;
  readonly sourcePrefix?: string;
  readonly versionRange?: { min?: number; max?: number };
}

export interface EventEmitRequest<T = unknown> {
  readonly type: string;
  readonly payload: T;
  readonly scope?: RuntimeScope;
  readonly idempotencyKey?: string;
  readonly metadata?: Record<string, unknown>;
}

export interface EventEmitResult {
  readonly eventId: string;
  readonly accepted: boolean;
  readonly duplicate: boolean;
}

export interface EventPublisher {
  emit<T>(request: EventEmitRequest<T>): Promise<EventEmitResult>;
}

const eventHandlers = new Map<string, EventSubscriptionSpec>();

export function defineEvent(spec: EventSubscriptionSpec): EventSubscriptionSpec {
  if (!spec.eventType) {
    throw new Error("event type is required");
  }
  if (typeof spec.handler !== "function") {
    throw new Error("event handler must be a function");
  }
  eventHandlers.set(spec.eventType, spec);
  return spec;
}

export function getEventSpec(eventType: string): EventSubscriptionSpec | undefined {
  return eventHandlers.get(eventType);
}

export function listEventSpecs(): EventSubscriptionSpec[] {
  return Array.from(eventHandlers.values());
}

export function clearEventSpecs(): void {
  eventHandlers.clear();
}

export function matchEvent(filter: EventFilter | undefined, event: EventEnvelope): boolean {
  if (!filter) return true;
  if (filter.sourcePrefix && !event.source.startsWith(filter.sourcePrefix)) {
    return false;
  }
  if (filter.versionRange) {
    if (filter.versionRange.min !== undefined && event.version < filter.versionRange.min) {
      return false;
    }
    if (filter.versionRange.max !== undefined && event.version > filter.versionRange.max) {
      return false;
    }
  }
  if (filter.payloadSchema) {
    const errors = validateSchema(event.payload, filter.payloadSchema);
    if (errors.length > 0) return false;
  }
  return true;
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
