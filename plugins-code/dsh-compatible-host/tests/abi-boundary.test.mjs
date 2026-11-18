import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import {
  DSH_ABI_CAPABILITIES,
  DSH_CAPABILITY_STATUS,
  TARGET_DSH_CLIENT_UI_PRIMITIVES_VERSION,
  TARGET_DSH_VERSION,
  compatibilitySummary,
} from "../src/runtime/abi.mjs";

const packageRoot = fileURLToPath(new URL("..", import.meta.url));

test("keeps the frozen DSH target and Cordis dependency boundary explicit", async () => {
  const packageJson = JSON.parse(await readFile(join(packageRoot, "package.json"), "utf8"));
  assert.equal(TARGET_DSH_VERSION, "0.2.2-rc.2");
  assert.equal(TARGET_DSH_CLIENT_UI_PRIMITIVES_VERSION, "0.2.2-rc.2");
  assert.equal(packageJson.dependencies["@deepseek-ai/dsh-client-ui-primitives"], "0.2.2-rc.2");
  assert.equal(Object.hasOwn(packageJson.dependencies, "@deepseek-ai/cordis"), false);
  assert.equal(Object.hasOwn(packageJson.devDependencies, "@deepseek-ai/cordis"), false);
  assert.equal(DSH_ABI_CAPABILITIES.includes("host.subagents"), true);
  assert.equal(DSH_ABI_CAPABILITIES.includes("host.mcp-resources"), true);
  assert.equal(DSH_ABI_CAPABILITIES.every((id) => DSH_CAPABILITY_STATUS[id]), true);
  const summary = compatibilitySummary();
  assert.equal(summary.capabilityCount, DSH_ABI_CAPABILITIES.length);
  assert.equal(summary.complete, false);
});
