import assert from "node:assert/strict";
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { DSH_HOST_ERROR_CODES } from "../src/runtime/errors.mjs";
import {
  assertDshHostPluginActive,
  createDshHostContext,
  dshCompatSpecifiers,
  loadDshHostPlugin,
} from "../src/runtime/host-abi.mjs";
import { appliedCount as missingServiceApplied } from "./fixtures/native-missing-plugin/index.js";

const fixtureUrl = (name) => fileURLToPath(new URL(`./fixtures/${name}`, import.meta.url));
const officialRoot = fileURLToPath(new URL("../node_modules/@deepseek-ai/", import.meta.url));

function officialPackage(name) {
  return {
    packageDir: join(officialRoot, name),
    entry: "./lib/index.js",
  };
}

test("keeps an unsaturated inject inactive and reports the missing service", async () => {
  const before = missingServiceApplied();
  await assert.rejects(
    loadDshHostPlugin({
      packageDir: fixtureUrl("native-missing-plugin"),
      entry: "./index.js",
      pluginId: "@amitia-test/native-missing",
      services: {},
    }),
    (error) => {
      assert.equal(error.code, DSH_HOST_ERROR_CODES.SERVICE_MISSING);
      assert.match(error.message, /nativeMissingService/);
      return true;
    },
  );
  assert.equal(missingServiceApplied(), before);
});

test("preserves the original cordis conflict error when a plugin re-registers an occupied service", async () => {
  const host = createDshHostContext({ services: { workspace: { stale: true } } });
  await assert.rejects(
    loadDshHostPlugin({
      packageDir: fixtureUrl("native-shadow-plugin"),
      entry: "./index.js",
      pluginId: "@amitia-test/native-shadow",
      context: host.context,
    }),
    (error) => {
      assert.equal(error.code, DSH_HOST_ERROR_CODES.SERVICE_CONFLICT);
      assert.equal(error.message, 'service "workspace" has been registered at <root>');
      return true;
    },
  );
  assert.deepEqual(host.context.workspace, { stale: true });
  await host.dispose();
});

test("preserves the original cordis error for an undeclared service read", async () => {
  await assert.rejects(
    loadDshHostPlugin({
      packageDir: fixtureUrl("native-uninjected-plugin"),
      entry: "./index.js",
      pluginId: "@amitia-test/native-uninjected",
      services: {},
    }),
    (error) => {
      assert.equal(error.code, DSH_HOST_ERROR_CODES.SERVICE_UNINJECTED);
      assert.equal(error.message, 'cannot get property "undeclaredService" without inject');
      return true;
    },
  );
});

test("deactivates dependents when their provider stops and reports the service missing afterwards", async () => {
  const host = createDshHostContext({});
  const provider = host.context.plugin({
    name: "native-provider",
    apply(ctx) {
      ctx.provide("nativeService", { value: "provider" });
    },
  });
  await provider.await();
  assert.equal(provider.state, 2);

  let seen = "";
  const consumer = host.context.plugin({
    name: "native-consumer",
    inject: ["nativeService"],
    apply(ctx) {
      seen = ctx.nativeService.value;
    },
  });
  await consumer.await();
  assert.equal(consumer.state, 2);
  assert.equal(seen, "provider");

  await provider.dispose();
  assert.equal(provider.state, 4);
  assert.equal(consumer.state, 0);
  assert.equal(host.context.get("nativeService"), undefined);
  await assert.rejects(
    loadDshHostPlugin({
      packageDir: fixtureUrl("native-missing-plugin"),
      entry: "./index.js",
      pluginId: "@amitia-test/native-missing-after-stop",
      context: host.context,
      deferInjectCheck: true,
    }).then(assertDshHostPluginActive),
    (error) => {
      assert.equal(error.code, DSH_HOST_ERROR_CODES.SERVICE_MISSING);
      assert.match(error.message, /nativeMissingService/);
      return true;
    },
  );

  await consumer.dispose();
  await host.dispose();
});

test("reactivates a deferred plugin once another entry provides its service", async () => {
  const host = createDshHostContext({});
  const before = missingServiceApplied();
  const runtime = await loadDshHostPlugin({
    packageDir: fixtureUrl("native-missing-plugin"),
    entry: "./index.js",
    pluginId: "@amitia-test/native-deferred",
    context: host.context,
    deferInjectCheck: true,
  });
  assert.equal(runtime.plugin.state, 0);
  await assert.rejects(assertDshHostPluginActive(runtime), error => {
    assert.equal(error.code, DSH_HOST_ERROR_CODES.SERVICE_MISSING);
    return true;
  });

  host.context.provide("nativeMissingService", { value: "late" });
  await assertDshHostPluginActive(runtime);
  assert.equal(runtime.plugin.state, 2);
  assert.equal(missingServiceApplied(), before + 1);

  await runtime.dispose();
  await host.dispose();
});

test("hosts the official LSP seam and routes provider queries through the container", async () => {
  const host = createDshHostContext({});
  const runtime = await loadDshHostPlugin({
    ...officialPackage("dsh-lsp"),
    pluginId: "@deepseek-ai/dsh-lsp",
    context: host.context,
  });
  assert.equal(runtime.plugin.state, 2);
  assert.equal(host.context.lsp.providerIds.size, 0);

  const provider = {
    id: "amitia-probe",
    extensionToLanguage: { ".ts": "typescript" },
    query: async (request) => ({ languageId: "typescript", filePath: request.filePath }),
  };
  const release = host.context.lsp.registerProvider(provider);
  assert.deepEqual(await host.context.lsp.query({ filePath: "/tmp/probe.ts" }), {
    languageId: "typescript",
    filePath: "/tmp/probe.ts",
  });
  assert.throws(() => host.context.lsp.registerProvider({ ...provider, id: "amitia-other" }), (error) => {
    assert.equal(error.code, "LSP_CONFLICT");
    return true;
  });

  release();
  await assert.rejects(host.context.lsp.query({ filePath: "/tmp/probe.ts" }), (error) => {
    assert.equal(error.code, "LSP_UNAVAILABLE");
    return true;
  });

  await runtime.dispose();
  assert.equal(host.context.get("lsp"), undefined);
  await host.dispose();
});

test("hosts the official shell environment seam with registering and collecting values", async () => {
  const host = createDshHostContext({});
  const dshHome = join(tmpdir(), "amitia-dsh-home");
  const runtime = await loadDshHostPlugin({
    ...officialPackage("dsh-shell-env"),
    pluginId: "@deepseek-ai/dsh-shell-env",
    context: host.context,
    config: { dshHome },
  });
  assert.equal(runtime.plugin.state, 2);
  assert.deepEqual(host.context.shellEnv.list(), []);

  const release = host.context.shellEnv.register({
    name: "amitia-probe",
    variables: { DSH_AMITIA_PROBE: { description: "probe value" } },
    resolve: () => ({ DSH_AMITIA_PROBE: "/tmp/probe" }),
  });
  assert.deepEqual(host.context.shellEnv.list(), [
    { contributor: "amitia-probe", description: "probe value", key: "DSH_AMITIA_PROBE" },
  ]);
  const collected = host.context.shellEnv.collect({});
  assert.equal(collected.DSH_AMITIA_PROBE, "/tmp/probe");
  assert.equal(collected.DSH_SHELL, "1");
  assert.equal(collected.DSH_HOME, dshHome);

  release();
  assert.deepEqual(host.context.shellEnv.list(), []);

  await runtime.dispose();
  assert.equal(host.context.get("shellEnv"), undefined);
  await host.dispose();
});

test("prefers installed official packages and covers the imports they take from compat shims", async () => {
  for (const specifier of dshCompatSpecifiers()) {
    const name = specifier.slice("@deepseek-ai/".length);
    const installed = existsSync(join(officialRoot, name, "package.json"));
    const resolved = import.meta.resolve(specifier);
    if (installed) {
      assert.match(resolved, /\/node_modules\//, `${specifier} 应解析到已安装的官方包`);
    } else {
      assert.match(resolved, /\/src\/runtime\/(compat|vendor)\//, `${specifier} 应解析到容器兼容层`);
    }
  }

  const namedImports = new Map();
  for (const entry of readdirSync(officialRoot, { withFileTypes: true })) {
    if (!entry.name.startsWith("dsh-")) continue;
    const file = join(officialRoot, entry.name, "lib/index.js");
    if (!existsSync(file)) continue;
    const source = readFileSync(file, "utf8");
    for (const match of source.matchAll(/import\s*\{([^}]+)\}\s*from\s*"(@deepseek-ai\/[^"]+)"/g)) {
      const names = match[1].split(",").map(part => part.trim().split(/\s+as\s+/)[0].trim()).filter(Boolean);
      const specifier = match[2];
      const known = namedImports.get(specifier) ?? new Set();
      for (const name of names) known.add(name);
      namedImports.set(specifier, known);
    }
    for (const match of source.matchAll(/import\s+([A-Za-z_$][\w$]*)\s*(?:,\s*\{[^}]*\}\s*)?from\s*"(@deepseek-ai\/[^"]+)"/g)) {
      const known = namedImports.get(match[2]) ?? new Set();
      known.add("default");
      namedImports.set(match[2], known);
    }
  }

  const mapped = new Set(dshCompatSpecifiers());
  let checked = 0;
  for (const [specifier, names] of namedImports) {
    if (!mapped.has(specifier)) continue;
    const module = await import(specifier);
    const exported = new Set(Object.keys(module));
    const missing = Array.from(names).filter(name => !exported.has(name));
    assert.deepEqual(missing, [], `${specifier} 缺少导出名 ${missing.join(", ")}`);
    checked += 1;
  }
  assert.ok(checked > 0, "官方包未从容器映射的 specifier 导入任何名称，导出面守卫失去意义");
});
