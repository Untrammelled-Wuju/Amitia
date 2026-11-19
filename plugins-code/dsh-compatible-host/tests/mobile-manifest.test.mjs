import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { applyMobileContributionActions } from "../scripts/mobile-contributions.mjs";
import { DSH_MOBILE_UNAVAILABLE_SLOTS } from "../scripts/catalog-policy.mjs";
import { DSH_SLOT_ALIASES } from "../src/client-abi/slot-map.mjs";

const manifest = JSON.parse(readFileSync(new URL("../amitia-extension.json", import.meta.url), "utf8"));
const contributions = manifest.modules.flatMap((module) => module.contributions ?? []);

test("declares desktop, Android and iOS page entries in one Amitiax package", () => {
  const provider = contributions.find((item) => item.id === "dsh-compatible-page-provider");
  assert.ok(provider);
  for (const platform of ["electron_windows", "electron_macos", "electron_linux"]) {
    assert.equal(provider.spec.entries[platform].type, "web_module");
    assert.equal(provider.spec.entries[platform].path, "modules/dsh-compatible-ui/desktop/index.mjs");
  }
  for (const platform of ["android", "ios", "mobile"]) {
    assert.equal(provider.spec.entries[platform].type, "mobile_web_runtime");
    assert.equal(provider.spec.entries[platform].contributionId, "dsh-compatible-mobile-page");
    assert.equal(provider.spec.entries[platform].path, "modules/dsh-compatible-ui/mobile/index.mjs");
    assert.equal(provider.spec.entries[platform].stylePath, "modules/dsh-compatible-ui/mobile/index.css");
  }
});

test("registers the shared container route through generic UI providers", () => {
  const routeProvider = contributions.find((item) => item.id === "dsh-compatible-route");
  assert.ok(routeProvider);
  assert.equal(routeProvider.spec.capability, "route.registry");
  const route = routeProvider.spec.metadata.routes[0];
  assert.equal(route.path, "/dsh-compatible-host");
  assert.equal(route.providerId, "dsh-compatible-page-provider");
  assert.equal(routeProvider.spec.metadata.navigationItems[0].route, route.path);
});

test("derives the command action for every mobile DSH slot contribution", () => {
  const generated = applyMobileContributionActions(structuredClone(manifest));
  const mobile = generated.modules
    .flatMap((module) => module.contributions ?? [])
    .filter((item) => item.id.startsWith("dsh-mobile-"));
  assert.equal(mobile.length, 40);
  for (const contribution of mobile) {
    assert.equal(contribution.kind, "ui_panel");
    assert.ok(contribution.spec.kind);
    assert.equal(contribution.spec.actions[0].action_id, "command");
    assert.equal(contribution.spec.actions[0].target.tool_id, "com.amitia/dsh-compatible-host/command");
    assert.deepEqual(contribution.requiredPermissions, ["tool.invoke"]);
  }
});

test("covers every DSH slot alias with a mobile host contribution", () => {
  const covered = new Set(
    manifest.modules
      .flatMap((module) => module.contributions ?? [])
      .filter((item) => item.id.startsWith("dsh-mobile-") || item.id === "dsh-compatible-mobile-page")
      .map((item) => item.spec.slot.slot_id),
  );
  const unavailable = new Set(DSH_MOBILE_UNAVAILABLE_SLOTS);
  const missing = [...new Set(Object.entries(DSH_SLOT_ALIASES)
    .filter(([key]) => !unavailable.has(key))
    .map(([, slotId]) => slotId))]
    .filter((slotId) => !covered.has(slotId));
  assert.deepEqual(missing, []);
});

test("uses the panel contract for the complete conversation region", () => {
  const contribution = contributions.find((item) => item.id === "dsh-mobile-conversation-messages");
  assert.ok(contribution);
  assert.equal(contribution.kind, "panel");
  assert.equal(contribution.spec.kind, "panel");
  assert.equal(contribution.spec.slot.slot_id, "provider.conversation.messages");
});

test("registers the DSH management page in the mobile extension entry group", () => {
  const navigation = contributions.find((item) => item.id === "dsh-compatible-mobile-navigation");
  assert.ok(navigation);
  assert.equal(navigation.kind, "ui_provider");
  assert.equal(navigation.spec.capability, "app.navigation");
  for (const platform of ["android", "ios", "mobile"]) {
    assert.equal(navigation.spec.entries[platform].type, "declarative");
  }
  const item = navigation.spec.metadata.navigationItems[0];
  assert.equal(item.label, "DSH 插件管理");
  assert.equal(item.icon, "extension");
  assert.equal(
    item.route,
    "/extension/page/dsh-compatible-mobile-page?extensionId=com.amitia%2Fdsh-compatible-host",
  );
  const page = contributions.find((entry) => entry.id === "dsh-compatible-mobile-page");
  assert.ok(page);
  assert.ok(page.spec.visibility.platforms.includes("android"));
  assert.ok(page.spec.visibility.platforms.includes("ios"));
});
