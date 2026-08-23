import type { SlotSnapshot, UIContributionSummary } from "@/stores/extensionUI";
import type { ClientSlotContribution, ClientSlotDefinition, ClientSlotKind } from "@/ui-runtime/clientPluginRuntime";

export type UnifiedSlotItem =
  | {
      source: "server";
      key: string;
      ordering: number;
      priority: number;
      contributionId: string;
      entryKey: string;
      cellId: string;
      matched?: unknown;
      server: UIContributionSummary;
    }
  | {
      source: "client";
      key: string;
      ordering: number;
      priority: number;
      contributionId: string;
      entryKey: string;
      cellId: string;
      matched?: unknown;
      client: ClientSlotContribution;
    };

export type SlotContractLike =
  | Pick<SlotSnapshot, "kind" | "multiplicity">
  | Pick<ClientSlotDefinition, "kind" | "multiplicity">
  | null
  | undefined;

export interface UnifiedSlotDispatchOptions {
  dispatchKey?: string;
  listOnly?: string;
  /** Optional matched payloads already produced by executable client chain selectors. */
  clientMatched?: ReadonlyMap<string, unknown>;
}

/**
 * Merge server/declarative and trusted-client entries under one dispatch model.
 * `kind` is authoritative. `multiplicity` is only consulted for snapshots from
 * pre-kind runtimes, which keeps old plugins usable without weakening the new
 * single/list/keyed/chain semantics.
 */
export function buildUnifiedSlotItems(
  contract: SlotContractLike,
  server: readonly UIContributionSummary[],
  client: readonly ClientSlotContribution[],
  options: UnifiedSlotDispatchOptions = {},
): UnifiedSlotItem[] {
  const items = toItems(server, client, options.clientMatched);
  const explicitKind = contract?.kind;
  if (!explicitKind) return dispatchLegacyMultiplicity(contract?.multiplicity, items);

  switch (explicitKind) {
    case "single":
      return pickShadowWinner(items);
    case "list":
      return dispatchList(items, options.listOnly);
    case "keyed":
      return dispatchKeyed(items, options.dispatchKey);
    case "chain":
      // Server contributions have already passed visibility/condition matching
      // in extensionUI.getVisibleContributions(). Trusted client entries arrive
      // here only after their executable select(owner) matched. We can therefore
      // merge both families by shadow priority without a second selector system.
      return pickChainWinner(items);
    default:
      return dispatchLegacyMultiplicity(contract?.multiplicity, items);
  }
}

export function hasUnifiedSlotItem(
  contract: SlotContractLike,
  server: readonly UIContributionSummary[],
  client: readonly ClientSlotContribution[],
  options: UnifiedSlotDispatchOptions = {},
): boolean {
  return buildUnifiedSlotItems(contract, server, client, options).length > 0;
}

function toItems(
  server: readonly UIContributionSummary[],
  client: readonly ClientSlotContribution[],
  clientMatched?: ReadonlyMap<string, unknown>,
): UnifiedSlotItem[] {
  return [
    ...server.map((entry) => ({
      source: "server" as const,
      key: `server:${entry.contributionId}`,
      contributionId: entry.contributionId,
      ordering: finite(entry.ordering),
      priority: finite(entry.priority ?? entry.ordering),
      entryKey: normalizeIdentity(entry.entryKey, entry.contributionId),
      cellId: normalizeIdentity(entry.cellId, entry.contributionId),
      matched: entry.matched,
      server: entry,
    })),
    ...client.map((entry) => ({
      source: "client" as const,
      key: `client:${entry.contributionId}`,
      contributionId: entry.contributionId,
      ordering: finite(entry.ordering),
      priority: finite(entry.priority),
      entryKey: normalizeIdentity(entry.entryKey, entry.key),
      cellId: normalizeIdentity(entry.cellId, entry.key),
      matched: clientMatched?.get(entry.contributionId),
      client: entry,
    })),
  ];
}

function dispatchList(items: UnifiedSlotItem[], listOnly?: string): UnifiedSlotItem[] {
  const cells = new Map<string, UnifiedSlotItem[]>();
  for (const item of items) {
    if (listOnly && item.cellId !== listOnly) continue;
    const group = cells.get(item.cellId) ?? [];
    group.push(item);
    cells.set(item.cellId, group);
  }
  const winners = Array.from(cells.values()).map((group) => shadowWinner(group, `list cell ${group[0]?.cellId ?? "unknown"}`));
  return winners.filter((item): item is UnifiedSlotItem => !!item).sort(compareListDisplay);
}

function dispatchKeyed(items: UnifiedSlotItem[], dispatchKey?: string): UnifiedSlotItem[] {
  const key = dispatchKey?.trim();
  if (!key) return [];
  return pickShadowWinner(items.filter((item) => item.entryKey === key), `keyed cell ${key}`);
}

function pickShadowWinner(items: UnifiedSlotItem[], cell = "single cell"): UnifiedSlotItem[] {
  const winner = shadowWinner(items, cell);
  return winner ? [winner] : [];
}

function pickChainWinner(items: UnifiedSlotItem[]): UnifiedSlotItem[] {
  if (items.length === 0) return [];
  // Chain ties are legal: assembly order is the final deterministic tiebreaker.
  // Unlike shadow cells, a chain does not represent multiple contenders for the
  // same addressable cell and therefore does not fail on equal priority.
  return [[...items].sort(compareChain)[0]!];
}

function shadowWinner(items: UnifiedSlotItem[], cell: string): UnifiedSlotItem | undefined {
  if (items.length === 0) return undefined;
  const ordered = [...items].sort(compareShadow);
  const first = ordered[0]!;
  const tied = ordered.filter((item) => item.priority === first.priority);
  if (tied.length > 1) {
    throw new Error(
      `UI slot ${cell} has multiple entries at strict priority ${first.priority}: ${tied.map((item) => item.contributionId).join(", ")}`,
    );
  }
  return first;
}

function dispatchLegacyMultiplicity(
  multiplicity: SlotSnapshot["multiplicity"] | ClientSlotDefinition["multiplicity"] | undefined,
  items: UnifiedSlotItem[],
): UnifiedSlotItem[] {
  const legacy = multiplicity ?? "ordered_multiple";
  if (legacy === "single" || legacy === "exclusive") {
    return items.sort(compareLegacyStable).slice(0, 1);
  }
  if (legacy === "replaceable_single") {
    return items.sort(compareLegacyReplacement).slice(0, 1);
  }
  return items.sort(compareLegacyStable);
}

function compareShadow(a: UnifiedSlotItem, b: UnifiedSlotItem): number {
  // DSH-style shadow priority: lower number wins. Registration sequence is
  // represented by stable source ordering/key only after priority is unique.
  return a.priority - b.priority || a.ordering - b.ordering || compareAssembly(a, b);
}

function compareChain(a: UnifiedSlotItem, b: UnifiedSlotItem): number {
  return a.priority - b.priority || compareAssembly(a, b);
}

function compareListDisplay(a: UnifiedSlotItem, b: UnifiedSlotItem): number {
  return a.ordering - b.ordering || compareAssembly(a, b);
}

function compareAssembly(a: UnifiedSlotItem, b: UnifiedSlotItem): number {
  if (a.source === "client" && b.source === "client") {
    return a.client.sequence - b.client.sequence || a.key.localeCompare(b.key);
  }
  if (a.source === b.source) return a.key.localeCompare(b.key);
  // Server entries are assembled before client entries in the host snapshot.
  return a.source === "server" ? -1 : 1;
}

function compareLegacyStable(a: UnifiedSlotItem, b: UnifiedSlotItem): number {
  const ordering = a.ordering - b.ordering;
  if (ordering !== 0) return ordering;
  if (a.source === "client" && b.source === "client" && a.client.strict && b.client.strict) {
    return a.client.sequence - b.client.sequence || a.key.localeCompare(b.key);
  }
  return b.priority - a.priority || a.key.localeCompare(b.key);
}

function compareLegacyReplacement(a: UnifiedSlotItem, b: UnifiedSlotItem): number {
  return b.priority - a.priority || a.ordering - b.ordering || a.key.localeCompare(b.key);
}

function normalizeIdentity(value: unknown, fallback: string): string {
  const normalized = typeof value === "string" ? value.trim() : "";
  return normalized || fallback;
}

function finite(value: unknown): number {
  const number = Number(value);
  return Number.isFinite(number) ? number : 0;
}

// Compile-time guard that keeps the union synchronized with the client kind.
const _kindGuard: Record<ClientSlotKind, true> = { single: true, list: true, keyed: true, chain: true };
void _kindGuard;
