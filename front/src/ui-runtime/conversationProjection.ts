import type { UIContributionSummary } from "@/stores/extensionUI";

export interface RuntimeConversationEvent {
  id: string;
  eventType: string;
  conversationId: string;
  timestamp: string;
  payload: Record<string, unknown>;
  source?: string;
  sequence?: number;
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
  anchorTimestamp: string;
  anchorSeq?: number;
  eventType: string;
  payload: Record<string, unknown>;
  events: RuntimeConversationEvent[];
}

const EVENT_NAME = "amitia:conversation-runtime-event";
const JOURNAL_PREFIX = "amitia:conversation-event-journal:";
const JOURNAL_LIMIT = 1000;

export function emitConversationRuntimeEvent(event: RuntimeConversationEvent): void {
  if (typeof window === "undefined") return;
  const normalized = normalizeEvent(event);
  appendConversationEventJournal(normalized);
  window.dispatchEvent(new CustomEvent<RuntimeConversationEvent>(EVENT_NAME, { detail: normalized }));
}

export function subscribeConversationRuntimeEvents(handler: (event: RuntimeConversationEvent) => void): () => void {
  if (typeof window === "undefined") return () => undefined;
  const listener = (event: Event) => {
    const detail = (event as CustomEvent<RuntimeConversationEvent>).detail;
    if (detail) handler(normalizeEvent(detail));
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

/** Deterministic conversation-event assembler. A projection with startEvents
 * does not publish a node until a start event exists. Replaying the same ordered
 * event log always produces the same nodes. */
export function assembleConversationNodes(
  events: RuntimeConversationEvent[],
  contributions: UIContributionSummary[],
): ConversationNode[] {
  const orderedEvents = [...events].map(normalizeEvent).sort(compareEvents);
  const nodes: ConversationNode[] = [];

  for (const contribution of contributions) {
    const spec = projectionSpec(contribution);
    if (!spec) continue;
    const groups = new Map<string, RuntimeConversationEvent[]>();
    for (const event of orderedEvents) {
      if (!spec.eventTypes.includes(event.eventType)) continue;
      const groupValue = spec.keyPath ? lookup(event.payload, spec.keyPath) : undefined;
      const groupKey = text(groupValue) || event.id;
      const group = groups.get(groupKey) ?? [];
      group.push(event);
      groups.set(groupKey, group);
    }

    for (const [groupKey, groupEvents] of groups) {
      const startEvents = spec.startEvents ?? [];
      const startIndex = startEvents.length > 0
        ? groupEvents.findIndex((event) => startEvents.includes(event.eventType))
        : 0;
      if (startIndex < 0) continue;
      const activeEvents = groupEvents.slice(startIndex);
      if (activeEvents.length === 0) continue;
      const maxEvents = spec.maxEvents ?? 100;
      const retained = activeEvents.slice(-maxEvents);
      const first = activeEvents[0]!;
      const last = activeEvents[activeEvents.length - 1]!;
      const endEvents = spec.endEvents ?? [];
      const completed = endEvents.length > 0 && activeEvents.some((event) => endEvents.includes(event.eventType));
      let title = "";
      if (spec.titlePath) {
        for (let i = activeEvents.length - 1; i >= 0; i--) {
          title = text(lookup(activeEvents[i]!.payload, spec.titlePath));
          if (title) break;
        }
      }
      nodes.push({
        nodeId: `${contribution.contributionId}:${groupKey}`,
        contributionId: contribution.contributionId,
        extensionId: contribution.extensionId,
        nodeType: spec.nodeType || contribution.kind || "conversation_node",
        conversationId: first.conversationId,
        groupKey,
        status: completed ? "completed" : "active",
        title: title || undefined,
        createdAt: first.timestamp,
        updatedAt: last.timestamp,
        anchorTimestamp: first.timestamp,
        anchorSeq: first.sequence,
        eventType: last.eventType,
        payload: last.payload,
        events: retained,
      });
    }
  }

  return nodes.sort((a, b) => compareTimeline(a.anchorSeq, a.anchorTimestamp, b.anchorSeq, b.anchorTimestamp));
}

export function messageHistoryEvents(messages: any[], fallbackConversationId: string): RuntimeConversationEvent[] {
  return messages.map((message, index) => normalizeEvent({
    id: String(message?.id ?? `message-${index}`),
    eventType: "message_created",
    conversationId: String(message?.conversationId ?? fallbackConversationId ?? ""),
    timestamp: String(message?.createdAt ?? message?.timestamp ?? new Date(0).toISOString()),
    sequence: numeric(message?.seq ?? message?.sequence),
    source: "history",
    payload: message && typeof message === "object" ? { ...message } : { value: message },
  })).filter((event) => !!event.conversationId);
}

export function loadConversationEventJournal(conversationId: string): RuntimeConversationEvent[] {
  if (typeof window === "undefined" || !conversationId) return [];
  try {
    const raw = localStorage.getItem(`${JOURNAL_PREFIX}${conversationId}`);
    const parsed = raw ? JSON.parse(raw) : [];
    return Array.isArray(parsed) ? parsed.map(normalizeEvent).filter((event) => event.conversationId === conversationId) : [];
  } catch {
    return [];
  }
}

export function appendConversationEventJournal(event: RuntimeConversationEvent): void {
  if (typeof window === "undefined" || !event.conversationId) return;
  try {
    const current = loadConversationEventJournal(event.conversationId);
    const key = eventKey(event);
    const next = [...current.filter((item) => eventKey(item) !== key), normalizeEvent(event)]
      .sort(compareEvents)
      .slice(-JOURNAL_LIMIT);
    localStorage.setItem(`${JOURNAL_PREFIX}${event.conversationId}`, JSON.stringify(next));
  } catch {
    // Storage is a replay optimization; failure must never break chat delivery.
  }
}

export function mergeConversationEvents(...groups: RuntimeConversationEvent[][]): RuntimeConversationEvent[] {
  const map = new Map<string, RuntimeConversationEvent>();
  for (const group of groups) for (const event of group) map.set(eventKey(event), normalizeEvent(event));
  return Array.from(map.values()).sort(compareEvents);
}

export function compareTimeline(aSeq: number | undefined, aTime: string, bSeq: number | undefined, bTime: string): number {
  if (aSeq != null && bSeq != null && aSeq !== bSeq) return aSeq - bSeq;
  const timeDelta = Date.parse(aTime || "") - Date.parse(bTime || "");
  if (Number.isFinite(timeDelta) && timeDelta !== 0) return timeDelta;
  if (aSeq != null && bSeq == null) return -1;
  if (aSeq == null && bSeq != null) return 1;
  return 0;
}

function normalizeEvent(event: RuntimeConversationEvent): RuntimeConversationEvent {
  const sequence = event.sequence ?? numeric(event.payload?.seq ?? event.payload?.sequence ?? event.payload?.sequenceId);
  return { ...event, sequence, timestamp: event.timestamp || new Date().toISOString(), payload: event.payload ?? {} };
}

function compareEvents(a: RuntimeConversationEvent, b: RuntimeConversationEvent): number {
  return compareTimeline(a.sequence, a.timestamp, b.sequence, b.timestamp) || eventKey(a).localeCompare(eventKey(b));
}

function eventKey(event: RuntimeConversationEvent): string {
  return `${event.conversationId}:${event.sequence ?? ""}:${event.eventType}:${event.id}`;
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

function numeric(value: unknown): number | undefined {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : undefined;
}
