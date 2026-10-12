import { ownedMemoryTitle } from "./owned-memory-format";

export interface MemoryGraphNode { id: string; name: string; symbolSize: number }
export interface MemoryGraphLink { source: string; target: string; name: string }

export function buildOwnedMemoryGraph(resources: any[]) {
  const nodes = new Map<string, MemoryGraphNode>();
  const links = new Map<string, MemoryGraphLink>();
  const text = (value: any): string => {
    if (typeof value === "string" || typeof value === "number") return String(value);
    if (value && typeof value === "object") return text(value.id ?? value.name ?? value.label ?? value.value);
    return "";
  };
  for (const row of resources) {
    if (row.kind !== "graph" || row.deleted) continue;
    const prefix = JSON.stringify([row.ownerId, row.roleId]);
    const node = (value: any, label?: any) => {
      const identity = text(value);
      if (!identity) return "";
      const id = JSON.stringify([prefix, identity]);
      const name = text(label) || text(value?.label ?? value?.name) || identity;
      if (!nodes.has(id)) nodes.set(id, { id, name, symbolSize: 36 });
      return id;
    };
    const edge = (from: any, to: any, relation: any) => {
      if (!text(from) || !text(to)) return;
      const source = node(from);
      const target = node(to);
      if (!source || !target) return;
      const name = text(relation);
      links.set(JSON.stringify([source, target, name]), { source, target, name });
    };
    const content = row.body?.content;
    if (content && typeof content === "object") {
      for (const item of Array.isArray(content.nodes) ? content.nodes : []) {
        if (item != null) node(item.id ?? item, item.label ?? item.name);
      }
      const edges = content.edges ?? content.links ?? content.relations;
      if (Array.isArray(edges)) {
        for (const item of edges) {
          if (item && typeof item === "object") edge(item.source ?? item.subject ?? item.from ?? item.in, item.target ?? item.object ?? item.to ?? item.out, item.relation ?? item.predicate ?? item.label ?? item.type);
        }
      }
      edge(content.source ?? content.subject ?? content.from ?? row.body.key, content.target ?? content.object ?? content.to, content.relation ?? content.predicate ?? content.type);
    }
    if (!content || typeof content !== "object" || (!content.target && !content.object && !content.to && !Array.isArray(content.nodes) && !Array.isArray(content.edges) && !Array.isArray(content.links) && !Array.isArray(content.relations))) {
      node(row.id, ownedMemoryTitle(row));
    }
  }
  return { nodes: Array.from(nodes.values()), links: Array.from(links.values()) };
}
