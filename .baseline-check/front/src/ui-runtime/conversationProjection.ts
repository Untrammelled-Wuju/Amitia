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

export interface DurableConversationUIEventRecord {
  eventId: string;
  sequence?: number;
  occurredAt: string;
  payload: Record<string, unknown>;
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

export type ProgrammaticConversationNodePhase = "start" | "update" | "end";

export interface ProgrammaticConversationNodeMatch {
  contextId: string;
  phase: ProgrammaticConversationNodePhase;
  data?: unknown;
}

export interface ProgrammaticConversationNodeView {
  key?: string;
  target?: string;
  visibility?: "visible" | "hidden";
  nodeType?: string;
  title?: string;
  payload?: Record<string, unknown>;
}

export interface ProgrammaticConversationNodeContext<State = unknown> {
  readonly contextId: string;
  readonly state: State;
  readonly status: "active" | "completed";
  readonly events: readonly RuntimeConversationEvent[];
  readonly firstEvent: RuntimeConversationEvent;
  readonly lastEvent: RuntimeConversationEvent;
  readonly previous?: ProgrammaticConversationNodeContext<State>;
}

export interface ProgrammaticConversationNodePublication {
  visibility?: "conversation" | "turn" | "step" | "private";
  target?: string;
  scopeId?: string;
}

export interface ProgrammaticConversationLocationEntry {
  definitionId: string;
  contextId: string;
  visibility: "conversation" | "turn" | "step" | "private";
  scopeId: string;
  target: string;
  data: Record<string, unknown>;
  sequence?: number;
  timestamp: string;
}

export interface ProgrammaticConversationNodeReader {
  previous(contextId?: string): ProgrammaticConversationNodeContext<unknown> | undefined;
  location(
    target: string,
    visibility?: ProgrammaticConversationNodePublication["visibility"],
    scopeId?: string,
  ): ProgrammaticConversationLocationEntry | undefined;
}

/**
 * Trusted programmable projection definition. This mirrors the important DSH
 * ConversationNodeDefinition semantics without evaluating arbitrary source
 * strings: a trusted client plugin matches durable events, starts one context,
 * incrementally folds updates, and projects the current state into a node.
 */
export interface ProgrammaticConversationNodeDefinition<State = unknown> {
  id: string;
  contributionId: string;
  extensionId: string;
  maxEvents?: number;
  match(event: RuntimeConversationEvent): ProgrammaticConversationNodeMatch | null;
  create(event: RuntimeConversationEvent, match: ProgrammaticConversationNodeMatch, reader?: ProgrammaticConversationNodeReader): State;
  update(state: State, event: RuntimeConversationEvent, match: ProgrammaticConversationNodeMatch, reader?: ProgrammaticConversationNodeReader): State;
  buildViewNode?(context: ProgrammaticConversationNodeContext<State>): ProgrammaticConversationNodeView;
  project?(context: ProgrammaticConversationNodeContext<State>): ProgrammaticConversationNodeView;
  publication?(context: ProgrammaticConversationNodeContext<State>): ProgrammaticConversationNodePublication | undefined;
  buildLocationData?(context: ProgrammaticConversationNodeContext<State>): Record<string, unknown> | undefined;
}

export interface ConversationNode {
  nodeId: string;
  contributionId: string;
  extensionId: string;
  nodeType: string;
  visibility?: "visible" | "hidden";
  viewKey?: string;
  target?: string;
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
  publication?: ProgrammaticConversationNodePublication;
  locationData?: Record<string, unknown>;
}

export interface ConversationAssemblerDiagnostic {
  code: "duplicate_start" | "projection_error";
  definitionId: string;
  contextId: string;
  eventId: string;
  message: string;
}

interface ProjectionLifecycle {
  events: RuntimeConversationEvent[];
  completed: boolean;
}

interface ProjectionGroupState {
  finalized: ProjectionLifecycle[];
  current?: ProjectionLifecycle;
}

interface ProjectionRuntimeState {
  contribution: UIContributionSummary;
  spec: ConversationProjectionSpec;
  groups: Map<string, ProjectionGroupState>;
}

interface ProgrammaticProjectionLifecycle {
  events: RuntimeConversationEvent[];
  state: unknown;
  completed: boolean;
}

interface ProgrammaticProjectionGroupState {
  finalized: ProgrammaticProjectionLifecycle[];
  current?: ProgrammaticProjectionLifecycle;
}

interface ProgrammaticProjectionRuntimeState {
  definition: ProgrammaticConversationNodeDefinition<unknown>;
  groups: Map<string, ProgrammaticProjectionGroupState>;
}

const EVENT_NAME = "amitia:conversation-runtime-event";
const JOURNAL_PREFIX = "amitia:conversation-event-journal:";
const JOURNAL_LIMIT = 1000;
const programmaticDefinitions = new Map<string, ProgrammaticConversationNodeDefinition<unknown>>();
const programmaticDefinitionSubscribers = new Set<() => void>();

export function registerProgrammaticConversationNodeDefinition<State>(
  definition: ProgrammaticConversationNodeDefinition<State>,
): () => void {
  const id = definition.id?.trim();
  const contributionId = definition.contributionId?.trim();
  const extensionId = definition.extensionId?.trim();
  if (!id) throw new Error("conversation node definition id is required");
  if (!contributionId) throw new Error(`conversation node definition ${id} requires contributionId`);
  if (!extensionId) throw new Error(`conversation node definition ${id} requires extensionId`);
  if (typeof definition.match !== "function" || typeof definition.create !== "function"
    || typeof definition.update !== "function"
    || (typeof definition.buildViewNode !== "function" && typeof definition.project !== "function")) {
    throw new Error(`conversation node definition ${id} requires match/create/update and buildViewNode/project callbacks`);
  }
  if (programmaticDefinitions.has(id)) throw new Error(`conversation node definition ${id} already registered`);
  const normalized = { ...definition, id, contributionId, extensionId } as ProgrammaticConversationNodeDefinition<unknown>;
  programmaticDefinitions.set(id, normalized);
  notifyProgrammaticDefinitionSubscribers();
  let active = true;
  return () => {
    if (!active) return;
    active = false;
    if (programmaticDefinitions.get(id) !== normalized) return;
    programmaticDefinitions.delete(id);
    notifyProgrammaticDefinitionSubscribers();
  };
}

export function listProgrammaticConversationNodeDefinitions(): ProgrammaticConversationNodeDefinition<unknown>[] {
  return Array.from(programmaticDefinitions.values()).sort((a, b) => a.id.localeCompare(b.id));
}

export function subscribeProgrammaticConversationNodeDefinitions(handler: () => void): () => void {
  programmaticDefinitionSubscribers.add(handler);
  return () => programmaticDefinitionSubscribers.delete(handler);
}

function notifyProgrammaticDefinitionSubscribers(): void {
  for (const handler of Array.from(programmaticDefinitionSubscribers)) {
    try { handler(); } catch { /* one plugin observer must not break the registry */ }
  }
}

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

/**
 * Session-style deterministic assembler for declarative Amitia conversation
 * projections. Live appends only touch definitions that match the new event;
 * historical prepend/resync uses replaceEvents() and deterministically replays
 * the durable window in timeline order.
 */
export class ConversationNodeAssembler {
  private readonly events = new Map<string, RuntimeConversationEvent>();
  private runtimes: ProjectionRuntimeState[] = [];
  private programmaticRuntimes: ProgrammaticProjectionRuntimeState[] = [];
  private contributions: UIContributionSummary[] = [];
  private programmatic: ProgrammaticConversationNodeDefinition<unknown>[] = [];
  private lastEvent?: RuntimeConversationEvent;
  private readonly diagnosticEntries: ConversationAssemblerDiagnostic[] = [];
  private readonly locationIndex = new Map<string, ProgrammaticConversationLocationEntry>();

  constructor(contributions: UIContributionSummary[] = []) {
    this.setContributions(contributions);
  }

  setContributions(contributions: UIContributionSummary[]): void {
    this.contributions = contributions.filter((item) => projectionSpec(item) != null);
    this.rebuild();
  }

  setProgrammaticDefinitions(definitions: ProgrammaticConversationNodeDefinition<unknown>[]): void {
    this.programmatic = [...definitions];
    this.rebuild();
  }

  replaceEvents(events: RuntimeConversationEvent[]): void {
    this.events.clear();
    for (const event of events) {
      const normalized = normalizeEvent(event);
      const key = eventKey(normalized);
      const existing = this.events.get(key);
      this.events.set(key, preferCanonicalEvent(existing, normalized));
    }
    this.rebuild();
  }

  prepend(events: RuntimeConversationEvent[]): void {
    let changed = false;
    for (const event of events) {
      const normalized = normalizeEvent(event);
      const key = eventKey(normalized);
      const existing = this.events.get(key);
      const selected = preferCanonicalEvent(existing, normalized);
      if (!existing || selected !== existing) changed = true;
      this.events.set(key, selected);
    }
    if (changed) this.rebuild();
  }

  repairGap(events: RuntimeConversationEvent[]): void {
    this.prepend(events);
  }

  window(): { firstSequence: number; lastSequence: number; size: number } {
    const ordered = this.listEvents();
    return {
      firstSequence: ordered.length ? Number(ordered[0]!.sequence ?? 0) : 0,
      lastSequence: ordered.length ? Number(ordered[ordered.length - 1]!.sequence ?? 0) : 0,
      size: ordered.length,
    };
  }

  append(event: RuntimeConversationEvent): void {
    const normalized = normalizeEvent(event);
    const key = eventKey(normalized);
    const existing = this.events.get(key);
    const selected = preferCanonicalEvent(existing, normalized);
    if (existing && selected === existing) return;
    this.events.set(key, selected);
    if (existing || (this.lastEvent && compareEvents(selected, this.lastEvent) < 0)) {
      this.rebuild();
      return;
    }
    this.processEvent(selected);
    this.lastEvent = selected;
  }

  listEvents(): RuntimeConversationEvent[] {
    return Array.from(this.events.values()).sort(compareEvents);
  }

  diagnostics(): ConversationAssemblerDiagnostic[] {
    return this.diagnosticEntries.map((entry) => ({ ...entry }));
  }

  nodes(): ConversationNode[] {
    const nodes: ConversationNode[] = [];
    for (const runtime of this.runtimes) {
      for (const [groupKey, state] of runtime.groups) {
        for (const lifecycle of state.finalized) {
          const node = buildNode(runtime.contribution, runtime.spec, groupKey, lifecycle);
          if (node) nodes.push(node);
        }
        if (state.current) {
          const node = buildNode(runtime.contribution, runtime.spec, groupKey, state.current);
          if (node) nodes.push(node);
        }
      }
    }
    for (const runtime of this.programmaticRuntimes) {
      for (const [contextId, state] of runtime.groups) {
        let previous: ProgrammaticProjectionLifecycle | undefined;
        for (const lifecycle of state.finalized) {
          const node = buildProgrammaticNode(runtime.definition, contextId, lifecycle, previous);
          if (node && node.visibility !== "hidden") nodes.push(node);
          previous = lifecycle;
        }
        if (state.current) {
          const node = buildProgrammaticNode(runtime.definition, contextId, state.current, previous);
          if (node && node.visibility !== "hidden") nodes.push(node);
        }
      }
    }
    return nodes.sort((a, b) => compareTimeline(a.anchorSeq, a.anchorTimestamp, b.anchorSeq, b.anchorTimestamp)
      || a.nodeId.localeCompare(b.nodeId));
  }

  private rebuild(): void {
    this.diagnosticEntries.length = 0;
    this.locationIndex.clear();
    this.runtimes = this.contributions.flatMap((contribution) => {
      const spec = projectionSpec(contribution);
      return spec ? [{ contribution, spec, groups: new Map<string, ProjectionGroupState>() }] : [];
    });
    this.programmaticRuntimes = this.programmatic.map((definition) => ({
      definition,
      groups: new Map<string, ProgrammaticProjectionGroupState>(),
    }));
    const ordered = this.listEvents();
    this.lastEvent = undefined;
    for (const event of ordered) {
      this.processEvent(event);
      this.lastEvent = event;
    }
  }

  private processEvent(event: RuntimeConversationEvent): void {
    for (const runtime of this.runtimes) {
      const { spec } = runtime;
      if (!spec.eventTypes.includes(event.eventType)) continue;
      const groupValue = spec.keyPath ? lookup(event.payload, spec.keyPath) : undefined;
      const groupKey = text(groupValue) || event.id;
      let group = runtime.groups.get(groupKey);
      if (!group) {
        group = { finalized: [] };
        runtime.groups.set(groupKey, group);
      }

      const startEvents = spec.startEvents ?? [];
      const endEvents = spec.endEvents ?? [];
      const isStart = startEvents.includes(event.eventType);
      const isEnd = endEvents.includes(event.eventType);

      if (startEvents.length > 0) {
        if (isStart) {
          if (group.current || group.finalized.length > 0) {
            if (group.current) {
              group.current.completed = true;
              group.finalized.push(group.current);
              group.current = undefined;
            }
            this.diagnosticEntries.push({
              code: "duplicate_start",
              definitionId: runtime.contribution.contributionId,
              contextId: groupKey,
              eventId: event.id,
              message: `duplicate start for ${runtime.contribution.contributionId}/${groupKey}; use a new stable context id`,
            });
            continue;
          }
          group.current = { events: [event], completed: isEnd };
          if (isEnd) {
            group.finalized.push(group.current);
            group.current = undefined;
          }
          continue;
        }
        if (!group.current) continue;
        group.current.events.push(event);
        if (isEnd) {
          group.current.completed = true;
          group.finalized.push(group.current);
          group.current = undefined;
        }
        continue;
      }

      if (!group.current) group.current = { events: [], completed: false };
      group.current.events.push(event);
      if (isEnd) {
        group.current.completed = true;
        group.finalized.push(group.current);
        group.current = undefined;
      }
    }

    for (const runtime of this.programmaticRuntimes) {
      let match: ProgrammaticConversationNodeMatch | null = null;
      try {
        match = runtime.definition.match(event);
      } catch {
        continue;
      }
      if (!match?.contextId?.trim()) continue;
      const contextId = match.contextId.trim();
      let group = runtime.groups.get(contextId);
      if (!group) {
        group = { finalized: [] };
        runtime.groups.set(contextId, group);
      }

      if (match.phase === "start") {
        if (group.current || group.finalized.length > 0) {
          if (group.current) {
            group.current.completed = true;
            group.finalized.push(group.current);
            group.current = undefined;
          }
          this.diagnosticEntries.push({
            code: "duplicate_start",
            definitionId: runtime.definition.id,
            contextId,
            eventId: event.id,
            message: `duplicate start for ${runtime.definition.id}/${contextId}; use a new stable context id`,
          });
          continue;
        }
        try {
          group.current = {
            events: [event],
            state: runtime.definition.create(event, match, this.readerFor(runtime, contextId, event)),
            completed: false,
          };
          this.refreshProgrammaticLocation(runtime, contextId, group.current);
        } catch {
          group.current = undefined;
        }
        continue;
      }

      if (!group.current) continue;
      try {
        group.current.state = runtime.definition.update(group.current.state, event, match, this.readerFor(runtime, contextId, event));
      } catch {
        continue;
      }
      group.current.events.push(event);
      this.refreshProgrammaticLocation(runtime, contextId, group.current);
      if (match.phase === "end") {
        group.current.completed = true;
        this.refreshProgrammaticLocation(runtime, contextId, group.current);
        group.finalized.push(group.current);
        group.current = undefined;
      }
    }
  }

  private readerFor(
    runtime: ProgrammaticProjectionRuntimeState,
    contextId: string,
    currentEvent?: RuntimeConversationEvent,
  ): ProgrammaticConversationNodeReader {
    return {
      previous: (requestedContextId?: string) => {
        const requested = requestedContextId?.trim();
        if (requested) {
          const target = runtime.groups.get(requested);
          const lifecycle = target?.finalized.at(-1);
          if (!lifecycle?.events.length) return undefined;
          const previousLifecycle = target!.finalized.length > 1 ? target!.finalized[target!.finalized.length - 2] : undefined;
          const previousContext = previousLifecycle?.events.length
            ? programmaticContext(requested, previousLifecycle, undefined, positiveInt(runtime.definition.maxEvents, 100))
            : undefined;
          return programmaticContext(requested, lifecycle, previousContext, positiveInt(runtime.definition.maxEvents, 100));
        }

        let bestContextId = "";
        let bestLifecycle: ProgrammaticProjectionLifecycle | undefined;
        for (const [candidateId, group] of runtime.groups) {
          if (candidateId === contextId) continue;
          const candidate = group.finalized.at(-1);
          if (!candidate?.events.length) continue;
          if (!bestLifecycle) {
            bestContextId = candidateId;
            bestLifecycle = candidate;
            continue;
          }
          const candidateLast = candidate.events[candidate.events.length - 1]!;
          const bestLast = bestLifecycle.events[bestLifecycle.events.length - 1]!;
          if (compareEvents(candidateLast, bestLast) > 0) {
            bestContextId = candidateId;
            bestLifecycle = candidate;
          }
        }
        return bestLifecycle
          ? programmaticContext(bestContextId, bestLifecycle, undefined, positiveInt(runtime.definition.maxEvents, 100))
          : undefined;
      },
      location: (
        target: string,
        visibility: ProgrammaticConversationNodePublication["visibility"] = "conversation",
        scopeId?: string,
      ) => {
        const normalizedTarget = target.trim();
        if (!normalizedTarget) return undefined;
        const group = runtime.groups.get(contextId);
        const lifecycle = group?.current ?? group?.finalized.at(-1);
        const event = currentEvent ?? lifecycle?.events.at(-1);
        if (!event) return undefined;
        const normalizedVisibility = visibility ?? "conversation";
        const resolvedScope = locationScopeId(
          normalizedVisibility,
          event,
          runtime.definition.id,
          contextId,
          scopeId,
        );
        return this.locationIndex.get(locationKey(normalizedVisibility, resolvedScope, normalizedTarget));
      },
    };
  }

  private refreshProgrammaticLocation(
    runtime: ProgrammaticProjectionRuntimeState,
    contextId: string,
    lifecycle: ProgrammaticProjectionLifecycle,
  ): void {
    if (!lifecycle.events.length || !runtime.definition.buildLocationData) return;
    const group = runtime.groups.get(contextId);
    const previousLifecycle = group?.finalized.at(-1);
    const previousContext = previousLifecycle?.events.length && previousLifecycle !== lifecycle
      ? programmaticContext(contextId, previousLifecycle, undefined, positiveInt(runtime.definition.maxEvents, 100))
      : undefined;
    const context = programmaticContext(contextId, lifecycle, previousContext, positiveInt(runtime.definition.maxEvents, 100));
    let publication: ProgrammaticConversationNodePublication | undefined;
    let data: Record<string, unknown> | undefined;
    try {
      publication = runtime.definition.publication?.(context);
      data = runtime.definition.buildLocationData(context);
    } catch {
      return;
    }
    if (!data) return;
    const visibility = publication?.visibility ?? "conversation";
    const target = publication?.target?.trim() || runtime.definition.id;
    const last = lifecycle.events[lifecycle.events.length - 1]!;
    const scopeId = locationScopeId(visibility, last, runtime.definition.id, contextId, publication?.scopeId);
    this.locationIndex.set(locationKey(visibility, scopeId, target), {
      definitionId: runtime.definition.id,
      contextId,
      visibility,
      scopeId,
      target,
      data: { ...data },
      sequence: last.sequence,
      timestamp: last.timestamp,
    });
  }
}

export function assembleConversationNodes(
  events: RuntimeConversationEvent[],
  contributions: UIContributionSummary[],
): ConversationNode[] {
  const assembler = new ConversationNodeAssembler(contributions);
  assembler.replaceEvents(events);
  return assembler.nodes();
}

export function messageHistoryEvents(messages: any[], fallbackConversationId: string): RuntimeConversationEvent[] {
  return messages.map((message, index) => normalizeEvent({
    id: String(message?.id ?? `message-${index}`),
    eventType: "message_created",
    conversationId: String(message?.conversationId ?? fallbackConversationId ?? ""),
    timestamp: String(message?.createdAt ?? message?.timestamp ?? new Date(0).toISOString()),
    sequence: numeric(message?.seq ?? message?.sequence),
    source: "history",
    payload: message && typeof message === "object"
      ? { ...message, messageId: message.id ?? message.messageId }
      : { value: message },
  })).filter((event) => !!event.conversationId);
}

export function durableConversationEvents(records: DurableConversationUIEventRecord[], fallbackConversationId: string): RuntimeConversationEvent[] {
  return records.map((record, index) => {
    const payload = record.payload && typeof record.payload === "object" ? record.payload : {};
    return normalizeEvent({
      id: String(record.eventId || `durable-${index}`),
      eventType: text(payload.type) || "conversation.ui_event",
      conversationId: text(payload.conversationId) || fallbackConversationId,
      timestamp: text(payload.createdAt) || record.occurredAt || new Date(0).toISOString(),
      sequence: numeric(record.sequence ?? payload.seq ?? payload.sequence ?? payload.sequenceId),
      source: "durable",
      payload,
    });
  }).filter((event) => !!event.conversationId);
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
    // Storage is only an offline replay optimization. Durable server history is
    // the cross-device source of truth and chat delivery must never depend on it.
  }
}

export function mergeConversationEvents(...groups: RuntimeConversationEvent[][]): RuntimeConversationEvent[] {
  const map = new Map<string, RuntimeConversationEvent>();
  for (const group of groups) {
    for (const event of group) {
      const normalized = normalizeEvent(event);
      const key = eventKey(normalized);
      map.set(key, preferCanonicalEvent(map.get(key), normalized));
    }
  }
  return Array.from(map.values()).sort(compareEvents);
}

function preferCanonicalEvent(
  existing: RuntimeConversationEvent | undefined,
  candidate: RuntimeConversationEvent,
): RuntimeConversationEvent {
  if (!existing) return candidate;
  const existingRank = eventSourceRank(existing.source);
  const candidateRank = eventSourceRank(candidate.source);
  if (candidateRank !== existingRank) return candidateRank > existingRank ? candidate : existing;
  const existingHasSequence = Number.isFinite(existing.sequence);
  const candidateHasSequence = Number.isFinite(candidate.sequence);
  if (candidateHasSequence !== existingHasSequence) return candidateHasSequence ? candidate : existing;
  return candidate;
}

function eventSourceRank(source: string | undefined): number {
  switch ((source ?? "").toLowerCase()) {
    case "durable": return 40;
    case "server": return 35;
    case "live": return 30;
    case "journal": return 20;
    case "history": return 10;
    default: return 25;
  }
}

export function compareTimeline(aSeq: number | undefined, aTime: string, bSeq: number | undefined, bTime: string): number {
  if (aSeq != null && bSeq != null && aSeq !== bSeq) return aSeq - bSeq;
  const timeDelta = Date.parse(aTime || "") - Date.parse(bTime || "");
  if (Number.isFinite(timeDelta) && timeDelta !== 0) return timeDelta;
  if (aSeq != null && bSeq == null) return -1;
  if (aSeq == null && bSeq != null) return 1;
  return 0;
}

function buildNode(
  contribution: UIContributionSummary,
  spec: ConversationProjectionSpec,
  groupKey: string,
  lifecycle: ProjectionLifecycle,
): ConversationNode | null {
  if (lifecycle.events.length === 0) return null;
  const first = lifecycle.events[0]!;
  const last = lifecycle.events[lifecycle.events.length - 1]!;
  const maxEvents = spec.maxEvents ?? 100;
  const retained = lifecycle.events.slice(-maxEvents);
  let title = "";
  if (spec.titlePath) {
    for (let index = lifecycle.events.length - 1; index >= 0; index--) {
      title = text(lookup(lifecycle.events[index]!.payload, spec.titlePath));
      if (title) break;
    }
  }
  return {
    nodeId: `${contribution.contributionId}:${groupKey}:${first.id}`,
    contributionId: contribution.contributionId,
    extensionId: contribution.extensionId,
    nodeType: spec.nodeType || contribution.kind || "conversation_node",
    conversationId: first.conversationId,
    groupKey,
    status: lifecycle.completed ? "completed" : "active",
    title: title || undefined,
    createdAt: first.timestamp,
    updatedAt: last.timestamp,
    anchorTimestamp: first.timestamp,
    anchorSeq: first.sequence,
    eventType: last.eventType,
    payload: last.payload,
    events: retained,
  };
}

function buildProgrammaticNode(
  definition: ProgrammaticConversationNodeDefinition<unknown>,
  contextId: string,
  lifecycle: ProgrammaticProjectionLifecycle,
  previousLifecycle?: ProgrammaticProjectionLifecycle,
): ConversationNode | null {
  if (lifecycle.events.length === 0) return null;
  const first = lifecycle.events[0]!;
  const last = lifecycle.events[lifecycle.events.length - 1]!;
  const maxEvents = positiveInt(definition.maxEvents, 100);
  const retained = lifecycle.events.slice(-maxEvents);
  const previousContext = previousLifecycle && previousLifecycle.events.length > 0
    ? programmaticContext(contextId, previousLifecycle, undefined, positiveInt(definition.maxEvents, 100))
    : undefined;
  const context = programmaticContext(contextId, lifecycle, previousContext, maxEvents);
  let view: ProgrammaticConversationNodeView;
  let publication: ProgrammaticConversationNodePublication | undefined;
  let locationData: Record<string, unknown> | undefined;
  try {
    const buildViewNode = definition.buildViewNode ?? definition.project;
    if (!buildViewNode) return null;
    view = buildViewNode(context);
    publication = definition.publication?.(context);
    locationData = definition.buildLocationData?.(context);
  } catch {
    return null;
  }
  return {
    nodeId: `${definition.id}:${contextId}:${first.id}`,
    contributionId: definition.contributionId,
    extensionId: definition.extensionId,
    nodeType: view.nodeType?.trim() || definition.id,
    visibility: view.visibility ?? "visible",
    viewKey: view.key?.trim() || contextId,
    target: view.target?.trim() || publication?.target?.trim() || undefined,
    conversationId: first.conversationId,
    groupKey: contextId,
    status: lifecycle.completed ? "completed" : "active",
    title: view.title?.trim() || undefined,
    createdAt: first.timestamp,
    updatedAt: last.timestamp,
    anchorTimestamp: first.timestamp,
    anchorSeq: first.sequence,
    eventType: last.eventType,
    payload: view.payload ?? last.payload,
    events: retained,
    publication,
    locationData,
  };
}

function programmaticContext(
  contextId: string,
  lifecycle: ProgrammaticProjectionLifecycle,
  previous: ProgrammaticConversationNodeContext<unknown> | undefined,
  maxEvents: number,
): ProgrammaticConversationNodeContext<unknown> {
  const events = lifecycle.events.slice(-maxEvents);
  return {
    contextId,
    state: lifecycle.state,
    status: lifecycle.completed ? "completed" : "active",
    events,
    firstEvent: lifecycle.events[0]!,
    lastEvent: lifecycle.events[lifecycle.events.length - 1]!,
    previous,
  };
}

function locationKey(
  visibility: NonNullable<ProgrammaticConversationNodePublication["visibility"]>,
  scopeId: string,
  target: string,
): string {
  return `${visibility}:${scopeId}:${target}`;
}

function locationScopeId(
  visibility: NonNullable<ProgrammaticConversationNodePublication["visibility"]>,
  event: RuntimeConversationEvent,
  definitionId: string,
  contextId: string,
  explicit?: string,
): string {
  const requested = explicit?.trim();
  if (requested) return requested;
  if (visibility === "conversation") return event.conversationId;
  if (visibility === "turn") {
    return locationPayloadId(event.payload, ["turnId", "turn_id", "turn.id"]) || event.conversationId;
  }
  if (visibility === "step") {
    return locationPayloadId(event.payload, [
      "stepId",
      "step_id",
      "step.id",
      "toolCallId",
      "tool_call_id",
      "taskId",
      "task_id",
    ]) || contextId;
  }
  return `${definitionId}:${contextId}`;
}

function locationPayloadId(payload: Record<string, unknown>, paths: string[]): string {
  for (const path of paths) {
    const value = text(lookup(payload, path));
    if (value) return value;
  }
  return "";
}

function normalizeEvent(event: RuntimeConversationEvent): RuntimeConversationEvent {
  const sequence = event.sequence ?? numeric(event.payload?.seq ?? event.payload?.sequence ?? event.payload?.sequenceId);
  return { ...event, sequence, timestamp: event.timestamp || new Date().toISOString(), payload: event.payload ?? {} };
}

function compareEvents(a: RuntimeConversationEvent, b: RuntimeConversationEvent): number {
  return compareTimeline(a.sequence, a.timestamp, b.sequence, b.timestamp) || eventKey(a).localeCompare(eventKey(b));
}

function eventKey(event: RuntimeConversationEvent): string {
  const messageId = text(event.payload?.messageId)
    || (event.eventType.startsWith("message_") ? text(event.payload?.id) : "");
  if (messageId) return `${event.conversationId}:${event.eventType}:message:${messageId}`;
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
