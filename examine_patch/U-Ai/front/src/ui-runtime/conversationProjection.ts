import type { UIContributionSummary } from "@/stores/extensionUI";

export interface RuntimeConversationEvent {
  id: string;
  eventType: string;
  conversationId: string;
  timestamp: string;
  payload: Record<string, unknown>;
  source?: string;
}

export interface ConversationProjectionSpec {
  eventTypes: string[];
  startEvents?: string[];
  endEvents?: string[];
  nodeType?: string;
  keyPath?: string;
  titlePath?: string;
  maxEvents?: number;
}

export interface ConversationNode {
  nodeId: string;
  contributionId: string;
  extensionId: string;
  nodeType: string;
  conversationId: string;
  groupKey: string;
  status: "active" | "completed";
  title?: string;
  createdAt: string;
  updatedAt: string;
  eventType: string;
  payload: Record<string, unknown>;
  events: RuntimeConversationEvent[];
}

const EVENT_NAME = "amitia:conversation-runtime-event";

export function emitConversationRuntimeEvent(event: RuntimeConversationEvent): void {
  if (typeof window === "undefined") return;
  window.dispatchEvent(new CustomEvent<RuntimeConversationEvent>(EVENT_NAME, { detail: event }));
}

export function subscribeConversationRuntimeEvents(
  handler: (event: RuntimeConversationEvent) => void,
): () => void {
  if (typeof window === "undefined") return () => undefined;
  const listener = (event: Event) => {
    const detail = (event as CustomEvent<RuntimeConversationEvent>).detail;
    if (detail) handler(detail);
  };
  window.addEventListener(EVENT_NAME, listener);
  return () => window.removeEventListener(EVENT_NAME, listener);
}

export function projectionSpec(contribution: UIContributionSummary): ConversationProjectionSpec | null {
  const projection = contribution.dataContract?.projection;
  if (!projection || typeof projection !== "object" || Array.isArray(projection)) return null;
  const raw = projection as Record<string, unknown>;
  const eventTypes = stringArray(raw.event_types ?? raw.eventTypes);
  if (eventTypes.length === 0) return null;
  return {
    eventTypes,
    startEvents: stringArray(raw.start_events ?? raw.startEvents),
    endEvents: stringArray(raw.end_events ?? raw.endEvents),
    nodeType: text(raw.node_type ?? raw.nodeType),
    keyPath: text(raw.key_path ?? raw.keyPath),
    titlePath: text(raw.title_path ?? raw.titlePath),
    maxEvents: positiveInt(raw.max_events ?? raw.maxEvents, 100),
  };
}

export function projectConversationEvent(
  event: RuntimeConversationEvent,
  contribution: UIContributionSummary,
  current?: ConversationNode,
): ConversationNode | null {
  const spec = projectionSpec(contribution);
  if (!spec || !spec.eventTypes.includes(event.eventType)) return null;
  const groupValue = spec.keyPath ? lookup(event.payload, spec.keyPath) : undefined;
  const groupKey = text(groupValue) || event.id;
  const now = event.timestamp || new Date().toISOString();
  const maxEvents = spec.maxEvents ?? 100;
  const events = [...(current?.events ?? []), event].slice(-maxEvents);
  const completed = (spec.endEvents ?? []).includes(event.eventType);
  const titleValue = spec.titlePath ? lookup(event.payload, spec.titlePath) : undefined;
  return {
    nodeId: `${contribution.contributionId}:${groupKey}`,
    contributionId: contribution.contributionId,
    extensionId: contribution.extensionId,
    nodeType: spec.nodeType || contribution.kind || "conversation_node",
    conversationId: event.conversationId,
    groupKey,
    status: completed ? "completed" : current?.status ?? "active",
    title: text(titleValue) || current?.title,
    createdAt: current?.createdAt ?? now,
    updatedAt: now,
    eventType: event.eventType,
    payload: event.payload,
    events,
  };
}

function lookup(value: unknown, path: string): unknown {
  let current = value;
  for (const segment of path.split(".")) {
    if (!current || typeof current !== "object" || Array.isArray(current)) return undefined;
    current = (current as Record<string, unknown>)[segment];
  }
  return current;
}

function text(value: unknown): string {
  return typeof value === "string" ? value.trim() : value == null ? "" : String(value);
}

function stringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string" && !!item.trim()) : [];
}

function positiveInt(value: unknown, fallback: number): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? Math.floor(parsed) : fallback;
}
