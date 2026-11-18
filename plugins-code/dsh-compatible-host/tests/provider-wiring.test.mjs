import assert from "node:assert/strict";
import { mkdtemp, stat } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import {
  pluginDataRootFromPackageDir,
  providerConfig,
  providerDataDirectory,
} from "../src/runtime/provider-wiring.mjs";

test("seam packages receive their required config without touching provider directories", async () => {
  const root = await mkdtemp(join(tmpdir(), "dsh-provider-wiring-"));
  assert.deepEqual(await providerConfig("@deepseek-ai/dsh-storage-domain", root), { backend: "json" });
  assert.deepEqual(await providerConfig("@deepseek-ai/dsh-session-projection-cache", root), {
    writeEveryEvents: 200,
    writeIntervalMs: 5000,
  });
  await assert.rejects(stat(join(root, "dsh-data")));
});

test("provider packages receive a created data root scoped to the plugin data directory", async () => {
  const root = await mkdtemp(join(tmpdir(), "dsh-provider-wiring-"));
  const config = await providerConfig("@deepseek-ai/dsh-spill-local", root);
  assert.deepEqual(config, { root: join(root, "dsh-data", "spill") });
  assert.equal((await stat(config.root)).isDirectory(), true);
  assert.equal(providerDataDirectory("@deepseek-ai/dsh-session-projection-cache", root), "");
});

test("unknown packages receive an empty config", async () => {
  const root = await mkdtemp(join(tmpdir(), "dsh-provider-wiring-"));
  assert.deepEqual(await providerConfig("@deepseek-ai/dsh-unknown", root), {});
});

test("plugin data root is resolved from the dsh-tree directory", () => {
  const packageDir = "/data/plugin-data/com.amitia__dsh-compatible-host/dsh-tree/node_modules/@deepseek-ai/dsh-spill-local";
  assert.equal(pluginDataRootFromPackageDir(packageDir), "/data/plugin-data/com.amitia__dsh-compatible-host");
  assert.equal(pluginDataRootFromPackageDir("/data/plugin-data/com.amitia__dsh-compatible-host"), "");
});
