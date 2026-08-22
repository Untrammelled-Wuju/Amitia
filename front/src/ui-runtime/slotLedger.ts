import type { SlotSnapshot, UIContributionSummary } from "@/stores/extensionUI";
import type { ClientSlotContribution, ClientSlotDefinition } from "@/ui-runtime/clientPluginRuntime";

export type UnifiedSlotItem =
  | { source: "server"; key: string; ordering: number; priority: number; contributionId: string; server: UIContributionSummary }
  | { source: "client"; key: string; ordering: number; priority: number; contributionId: string; client: ClientSlotContribution };

export type SlotContractLike = Pick<SlotSnapshot, "multiplicity"> | Pick<ClientSlotDefinition, "multiplicity"> | null | undefined;

export function buildUnifiedSlotItems(
  contract: SlotContractLike,
  server: readonly UIContributionSummary[],
  client: readonly ClientSlotContribution[],
): UnifiedSlotItem[] {
  const items: UnifiedSlotItem[] = [
    ...server.map((entry) => ({
      source: "server" as const,
      key: `server:${entry.contributionId}`,
      contributionId: entry.contributionId,
      ordering: finite(entry.ordering),
      priority: finite(entry.priority ?? entry.ordering),
      server: entry,
    })),
    ...client.map((entry) => ({
      source: "client" as const,
      key: `client:${entry.contributionId}`,
      contributionId: entry.contributionId,
      ordering: finite(entry.ordering),
      priority: finite(entry.priority),
      client: entry,
    })),
  ];

  const multiplicity = contract?.multiplicity ?? "ordered_multiple";
  if (multiplicity === "single" || multiplicity === "exclusive") {
    return items.sort(compareStable).slice(0, 1);
  }
  if (multiplicity === "replaceable_single") {
    return items.sort(compareReplacement).slice(0, 1);
  }
  return items.sort(compareStable);
}

export function hasUnifiedSlotItem(
  contract: SlotContractLike,
  server: readonly UIContributionSummary[],
  client: readonly ClientSlotContribution[],
): boolean {
  return buildUnifiedSlotItems(contract, server, client).length > 0;
}

function compareStable(a: UnifiedSlotItem, b: UnifiedSlotItem): number {
  const ordering = a.ordering - b.ordering;
  if (ordering !== 0) return ordering;
  // Strict DSH list dispatch already resolved shadow priority inside each cell.
  // Across different cells, preserve registration assembly order instead of
  // incorrectly re-sorting the visible list by priority.
  if (a.source === "client" && b.source === "client" && a.client.strict && b.client.strict) {
    return a.client.sequence - b.client.sequence || a.key.localeCompare(b.key);
  }
  return b.priority - a.priority || a.key.localeCompare(b.key);
}

function compareReplacement(a: UnifiedSlotItem, b: UnifiedSlotItem): number {
  return b.priority - a.priority || a.ordering - b.ordering || a.key.localeCompare(b.key);
}

function finite(value: unknown): number {
  const number = Number(value);
  return Number.isFinite(number) ? number : 0;
}
