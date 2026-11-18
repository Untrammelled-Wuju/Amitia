import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { DSH_SLOT_ALIASES } from "../src/client-abi/slot-map.mjs";
import { DSH_UI_SLOT_RESOLUTIONS } from "../src/client-abi/slot-catalog.generated.mjs";
import { DSH_CATALOG_SUMMARY } from "../src/runtime/catalog-summary.generated.mjs";

const catalogRoot = new URL("../catalog/", import.meta.url);
const readCatalog = (name) => JSON.parse(readFileSync(new URL(name, catalogRoot), "utf8"));
const abi = readCatalog("dsh-abi.v1.json");
const ui = readCatalog("dsh-ui-slots.v1.json");
const gaps = readCatalog("amitia-gap.v1.json");
const platforms = readCatalog("platform-support.v1.json");
const compatibility = readCatalog("compatibility.v1.json");
const frontSlotContract = readFileSync(
  new URL("../../../front/src/ui-runtime/slotContract.ts", import.meta.url),
  "utf8",
);
const amitiaSlots = new Set(
  [...frontSlotContract.matchAll(/^  (?:"([^"]+)"|([A-Za-z_$][\w$.-]*)):/gm)]
    .map((match) => match[1] || match[2]),
);

test("freezes every catalog to the target DSH version", () => {
  for (const catalog of [abi, ui, gaps, platforms, compatibility]) {
    assert.equal(catalog.targetDshVersion, "0.2.2-rc.2");
    assert.equal(catalog.catalogVersion, "v1");
    assert.ok(catalog.sourcePackages.some((entry) => entry.spec === "@deepseek-ai/dsh@0.2.2-rc.2"));
    assert.ok(catalog.sourcePackages.some((entry) => entry.spec === "@deepseek-ai/dsh-cordis-client-runner@0.2.2-rc.2"));
  }
});

test("catalogs every official public surface with a unique identifier", () => {
  const entries = [
    ...abi.host.services,
    ...abi.host.services.flatMap((entry) => entry.methods),
    ...abi.host.events,
    ...abi.host.tools,
    ...abi.host.configurations,
    ...abi.host.lifecycleEvents,
    ...abi.client.services,
    ...abi.client.services.flatMap((entry) => entry.methods),
    ...abi.client.events,
    ...ui.slots,
  ];
  const ids = entries.map((entry) => entry.id);
  assert.equal(new Set(ids).size, ids.length);
  assert.equal(abi.host.services.length, 69);
  assert.equal(abi.host.events.length, 74);
  assert.equal(abi.host.tools.length, 77);
  assert.equal(abi.host.configurations.length, 163);
  assert.ok(abi.host.lifecycleEvents.length > 0);
  const tools = abi.host.services.find((entry) => entry.key === "tools");
  assert.deepEqual(
    tools.methods.map((entry) => entry.name),
    ["register", "restrict", "get", "executable", "resolve", "definitions", "schemas", "setExecutor", "execute", "guard", "checkGuards"],
  );
  assert.equal(abi.client.services.length, 8);
  assert.equal(abi.client.events.length, 4);
  assert.equal(ui.slots.length, 120);
});

test("classifies every UI slot as mapped or explicitly pending", () => {
  for (const slot of ui.slots) {
    assert.ok(slot.id);
    assert.ok(["P10", "P12"].includes(slot.phase));
    assert.ok(["direct", "composite", "pending"].includes(slot.status));
    if (slot.status === "pending") {
      assert.ok(slot.reason.length > 0);
      assert.deepEqual(slot.targets, []);
    } else {
      assert.ok(slot.targets.length > 0);
      for (const target of slot.targets) assert.ok(amitiaSlots.has(target), `${slot.key} targets unknown ${target}`);
    }
  }
  assert.equal(Object.keys(DSH_UI_SLOT_RESOLUTIONS).length, ui.slots.length);
});

test("keeps production aliases inside the official target catalog", () => {
  const officialKeys = new Set(ui.slots.map((entry) => entry.key));
  for (const key of Object.keys(DSH_SLOT_ALIASES)) assert.ok(officialKeys.has(key), key);
  for (const target of Object.values(DSH_SLOT_ALIASES)) assert.ok(amitiaSlots.has(target), target);
});

test("routes every host service gap to an explicit provider decision", () => {
  assert.equal(gaps.host.length, 48);
  const counts = { delegated: 0, pending: 0, unavailable: 0 };
  const abiServices = new Map(abi.host.services.map((entry) => [entry.id, entry]));
  for (const gap of gaps.host) {
    assert.ok(["official-package", "amitia-service"].includes(gap.route), `${gap.capability} route=${gap.route}`);
    assert.ok(
      ["none", "amitia", "amitia-landed", "amitia-wired", "unavailable", "container"].includes(gap.provider),
      `${gap.capability} provider=${gap.provider}`,
    );
    assert.ok(gap.reason.length > 0, `${gap.capability} 缺少原因`);
    assert.ok(gap.consumerCount > 0, `${gap.capability} 缺少官方消费者证据`);
    const service = abiServices.get(gap.id);
    assert.ok(service, `${gap.id} 未出现在 dsh-abi`);
    assert.equal(service.status, gap.status);
    assert.equal(service.route, gap.route);
    assert.equal(service.provider, gap.provider);
    counts[gap.status] += 1;
  }
  assert.deepEqual(counts, { delegated: 27, pending: 8, unavailable: 13 });
  assert.equal(DSH_CATALOG_SUMMARY.hostServiceDelegatedCount, counts.delegated);
  assert.equal(DSH_CATALOG_SUMMARY.hostServiceProviderPendingCount, counts.pending);
  assert.equal(DSH_CATALOG_SUMMARY.hostServiceUnavailableCount, counts.unavailable);
  const remaining = gaps.host.filter((entry) => entry.status !== "delegated");
  assert.equal(remaining.length, counts.pending + counts.unavailable);
  assert.equal(
    abi.host.services.filter((entry) => entry.status === "pending" || entry.status === "unavailable").length,
    remaining.length,
  );
});

test("records landed and wired Amitia providers instead of hiding them as gaps", () => {
  assert.equal(
    gaps.resolved.length,
    DSH_CATALOG_SUMMARY.hostServiceImplementedCount + DSH_CATALOG_SUMMARY.hostServicePartialCount,
  );
  const pendingIds = new Set(gaps.host.map((entry) => entry.id));
  const resolvedCapabilities = new Map(gaps.resolved.map((entry) => [entry.capability, entry]));
  for (const capability of [
    "ctx.storage",
    "ctx.storageDomain",
    "ctx.workspaceRegistry",
    "ctx.sessionPersistence",
    "ctx.sessionProjectionCache",
    "ctx.sessionProjections",
    "ctx.spillStore",
  ]) {
    const entry = resolvedCapabilities.get(capability);
    assert.ok(entry, `${capability} 未出现在已落地清单`);
    assert.ok(["amitia-landed", "amitia-wired"].includes(entry.provider), `${capability} provider=${entry.provider}`);
    assert.equal(pendingIds.has(entry.id), false, `${capability} 同时出现在缺口清单`);
    assert.ok(entry.reason.length > 0, `${capability} 缺少落地说明`);
    assert.ok(entry.consumerCount > 0, `${capability} 缺少官方消费者证据`);
  }
  for (const [capability, expected] of [
    ["ctx.storage", "implemented"],
    ["ctx.storageDomain", "implemented"],
    ["ctx.workspaceRegistry", "implemented"],
    ["ctx.sessionPersistence", "implemented"],
    ["ctx.sessionProjectionCache", "implemented"],
    ["ctx.sessionProjections", "implemented"],
    ["ctx.spillStore", "implemented"],
  ]) {
    assert.equal(resolvedCapabilities.get(capability).status, expected, capability);
  }
});

test("summarizes gaps and platform claims without hiding pending work", () => {
  assert.equal(DSH_CATALOG_SUMMARY.hostServiceCount, abi.host.services.length);
  assert.equal(DSH_CATALOG_SUMMARY.uiSlotCount, ui.slots.length);
  assert.equal(
    DSH_CATALOG_SUMMARY.uiSlotMappedCount,
    ui.slots.filter((entry) => entry.status !== "pending").length,
  );
  assert.equal(gaps.ui.length, DSH_CATALOG_SUMMARY.uiSlotPendingCount);
  assert.equal(gaps.configurations.length, abi.host.configurations.length);
  assert.equal(
    gaps.lifecycleEvents.length,
    abi.host.lifecycleEvents.filter((entry) => entry.status === "pending").length,
  );
  assert.equal(
    abi.host.lifecycleEvents.filter((entry) => entry.status === "implemented").length,
    abi.host.lifecycleEvents.length,
  );
  assert.equal(abi.host.lifecycleEvents.length, DSH_CATALOG_SUMMARY.lifecycleEventCount);
  assert.equal(platforms.platforms.find((entry) => entry.platform === "ios").declaration, "not-declared");
  assert.equal(compatibility.summary.uiSlotPendingCount, gaps.ui.length);
});
