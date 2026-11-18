import assert from "node:assert/strict";
import { access, lstat, mkdtemp, mkdir, readFile, realpath, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";
import test from "node:test";
import {
  describeDshPackage,
  installDshPackage,
  restoreDshTreeLock,
  readInstalledClientSource,
  readDshClientSource,
  removeDshPackageInstallation,
  resolvePluginDataRoot,
} from "../src/runtime/package-manager.mjs";

test("resolves plugin_data from the runtime filesystem roots", () => {
  assert.equal(resolvePluginDataRoot({
    AMITIA_HOST_FILESYSTEM_ROOTS_JSON: JSON.stringify({
      package: "/tmp/package",
      plugin_data: "/tmp/plugin-data",
    }),
  }), "/tmp/plugin-data");
});

test("describes host and client entries from a DSH package", async () => {
  const root = await mkdtemp(join(tmpdir(), "amitia-dsh-package-"));
  await mkdir(root, { recursive: true });
  await writeFile(join(root, "package.json"), `${JSON.stringify({
    name: "@example/dsh-ui",
    displayName: "Example DSH UI",
    version: "1.2.3",
    type: "module",
    exports: {
      ".": { import: "./index.js" },
      "./client": { import: "./client.js" },
    },
    dsh: {
      manifestVersion: 1,
      client: { platform: "web" },
    },
  }, null, 2)}\n`);
  const described = await describeDshPackage(root, "npm:@example/dsh-ui@1.2.3");
  assert.equal(described.pluginId, "@example/dsh-ui");
  assert.equal(described.entry, "./index.js");
  assert.equal(described.clientEntry, "./client.js");
  assert.equal(described.enabled, false);
});

test("rejects packages without a DSH manifest", async () => {
  const root = await mkdtemp(join(tmpdir(), "amitia-non-dsh-package-"));
  await writeFile(join(root, "package.json"), `${JSON.stringify({
    name: "not-dsh",
    version: "1.0.0",
    main: "./index.js",
  }, null, 2)}\n`);
  await assert.rejects(describeDshPackage(root), /不是 DSH 插件/);
});

test("treats only the package root export as the host plugin entry", async () => {
  const fixture = new URL("./fixtures/multi-entry-plugin", import.meta.url);
  const described = await describeDshPackage(fileURLToPath(fixture));
  assert.deepEqual(described.entries, [{ id: "root", entry: "./index.js" }]);
  assert.deepEqual(described.clientEntries, [
    { id: "search", entry: "./search-client.js" },
    { id: "root", entry: "./client.js" },
  ]);
});

test("accepts a client-only DSH package without a host entry", async () => {
  const root = await mkdtemp(join(tmpdir(), "amitia-client-only-dsh-package-"));
  await writeFile(join(root, "package.json"), `${JSON.stringify({
    name: "@amitia-test/client-only-plugin",
    version: "1.0.0",
    type: "module",
    exports: { "./client": { import: "./client.js" } },
    dsh: { manifestVersion: 1, client: { platform: "web" } },
  }, null, 2)}\n`);
  const described = await describeDshPackage(root);
  assert.deepEqual(described.entries, []);
  assert.equal(described.entry, "");
  assert.deepEqual(described.clientEntries, [{ id: "root", entry: "./client.js" }]);
});

test("accepts an official DSH package whose package.json has no dsh field", async () => {
  const root = await mkdtemp(join(tmpdir(), "amitia-official-dsh-package-"));
  await writeFile(join(root, "package.json"), `${JSON.stringify({
    name: "@deepseek-ai/dsh-tool-fixture",
    version: "0.2.2-rc.2",
    type: "module",
    exports: { ".": "./lib/index.js" },
  }, null, 2)}\n`);
  await mkdir(join(root, "lib"));
  await writeFile(join(root, "lib", "index.js"), "export function apply() {}\n");
  const described = await describeDshPackage(root);
  assert.equal(described.pluginId, "@deepseek-ai/dsh-tool-fixture");
  assert.equal(described.dsh.official, true);
  assert.equal(described.entry, "./lib/index.js");
});

test("installs a local DSH package with the project npm", async () => {
  const pluginDataRoot = await mkdtemp(join(tmpdir(), "amitia-dsh-data-"));
  const fixture = new URL("./fixtures/sample-dsh-plugin", import.meta.url);
  const installed = await installDshPackage(fileURLToPath(fixture), { pluginDataRoot });
  assert.equal(installed.pluginId, "@amitia-test/dsh-fixture");
  assert.equal(installed.entry, "./index.js");
  assert.equal(installed.clientEntry, "./client.js");
  assert.equal(installed.enabled, false);
  assert.equal(installed.installRoot, join(pluginDataRoot, "dsh-tree"));
  assert.equal(installed.packageDir, join(installed.installRoot, "node_modules", "@amitia-test", "dsh-fixture"));
  assert.equal((await lstat(installed.packageDir)).isSymbolicLink(), false);
  assert.equal((await realpath(installed.packageDir)).startsWith(await realpath(installed.installRoot)), true);
  const client = await readDshClientSource(installed);
  assert.match(client.code, /__ModuleLoader__/);
  await removeDshPackageInstallation(installed, { AMITIA_PLUGIN_DATA_DIR: pluginDataRoot });
  await assert.rejects(access(installed.packageDir), /ENOENT/);
  const manifest = JSON.parse(await readFile(join(installed.installRoot, "package.json"), "utf8"));
  assert.deepEqual(manifest.dependencies, {});
});

test("serves the bundled React ESM vendor for react packages", async () => {
  const pluginDataRoot = await mkdtemp(join(tmpdir(), "amitia-react-vendor-"));
  const source = await readInstalledClientSource("react", "root", pluginDataRoot);
  assert.equal(source.vendor, true);
  assert.match(source.code, /createElement/);
  assert.match(source.code, /__AMITIA_REACT_VENDOR__/);
  const reactDom = await readInstalledClientSource("react-dom/client", "root", pluginDataRoot);
  assert.match(reactDom.code, /createRoot/);
});

test("records the installed DSH subpackage versions into plugin data", async () => {
  const pluginDataRoot = await mkdtemp(join(tmpdir(), "amitia-dsh-lock-"));
  const installed = await installDshPackage("npm:@example/dsh-atomic@2.3.4", {
    pluginDataRoot,
    runInstall: async (_, workspace) => {
      await writeFakePackage(workspace, "2.3.4");
      await writeFile(join(workspace, "package-lock.json"), `${JSON.stringify({
        lockfileVersion: 3,
        packages: {
          "": { name: "amitia-dsh-tree", version: "0.0.0" },
          "node_modules/@example/dsh-atomic": { version: "2.3.4" },
          "node_modules/@alpha/dsh-peer": { version: "2.0.0" },
        },
      }, null, 2)}\n`);
      return { log: "", packageName: "@example/dsh-atomic" };
    },
  });
  assert.equal(installed.lockPath, join(pluginDataRoot, "dsh-data", "dsh-tree-lock.json"));
  const lock = JSON.parse(await readFile(installed.lockPath, "utf8"));
  assert.deepEqual(lock.packages, [
    { name: "@alpha/dsh-peer", version: "2.0.0" },
    { name: "@example/dsh-atomic", version: "2.3.4" },
  ]);
  assert.equal(typeof lock.generatedAt, "string");
});

test("restores the DSH tree from the recorded subpackage versions", async () => {
  const pluginDataRoot = await mkdtemp(join(tmpdir(), "amitia-dsh-restore-"));
  const workspace = join(pluginDataRoot, "dsh-tree");
  await mkdir(join(workspace, "node_modules", "@example", "dsh-atomic"), { recursive: true });
  await writeFile(join(workspace, "package.json"), `${JSON.stringify({
    name: "amitia-dsh-tree",
    private: true,
    dependencies: { "@example/dsh-atomic": "1.0.0" },
  }, null, 2)}\n`);
  await mkdir(join(pluginDataRoot, "dsh-data"), { recursive: true });
  await writeFile(join(pluginDataRoot, "dsh-data", "dsh-tree-lock.json"), `${JSON.stringify({
    generatedAt: "2026-01-01T00:00:00.000Z",
    packages: [
      { name: "@alpha/dsh-peer", version: "2.0.0" },
      { name: "@example/dsh-atomic", version: "1.0.0" },
    ],
  }, null, 2)}\n`);
  const pinsSeen = [];
  const result = await restoreDshTreeLock({
    pluginDataRoot,
    runInstall: async (pins, target) => {
      pinsSeen.push(pins);
      for (const [name, version] of Object.entries(pins)) {
        const packageDir = join(target, "node_modules", ...name.split("/"));
        await mkdir(packageDir, { recursive: true });
        await writeFile(join(packageDir, "package.json"), `${JSON.stringify({ name, version }, null, 2)}\n`);
      }
      const peerDir = join(target, "node_modules", "@alpha", "dsh-peer");
      await mkdir(peerDir, { recursive: true });
      await writeFile(join(peerDir, "package.json"), `${JSON.stringify({ name: "@alpha/dsh-peer", version: "2.0.0" }, null, 2)}\n`);
      await writeFile(join(target, "package-lock.json"), `${JSON.stringify({
        lockfileVersion: 3,
        packages: {
          "node_modules/@alpha/dsh-peer": { version: "2.0.0" },
          "node_modules/@example/dsh-atomic": { version: "1.0.0" },
        },
      }, null, 2)}\n`);
    },
  });
  assert.deepEqual(pinsSeen, [{ "@example/dsh-atomic": "1.0.0" }]);
  assert.deepEqual(result.mismatched, []);
  assert.equal(result.lockPath, join(pluginDataRoot, "dsh-data", "dsh-tree-lock.json"));
});

test("atomically replaces an existing DSH installation", async () => {
  const pluginDataRoot = await mkdtemp(join(tmpdir(), "amitia-dsh-atomic-"));
  const source = "npm:@example/dsh-atomic@1.0.0";
  const install = async (workspace, version) => writeFakePackage(workspace, version);
  const first = await installDshPackage(source, {
    pluginDataRoot,
    runInstall: (_, workspace) => install(workspace, "1.0.0"),
  });
  await writeFile(join(first.packageDir, "old-version-marker"), "old");
  const second = await installDshPackage(source, {
    pluginDataRoot,
    runInstall: (_, workspace) => install(workspace, "1.0.1"),
  });
  assert.equal(second.version, "1.0.1");
  assert.equal(second.packageDir, first.packageDir);
  assert.equal(second.installRoot, first.installRoot);
  await assert.rejects(access(join(second.packageDir, "old-version-marker")), /ENOENT/);
  const manifest = JSON.parse(await readFile(join(second.installRoot, "package.json"), "utf8"));
  assert.deepEqual(manifest.dependencies, { "@example/dsh-atomic": "1.0.1" });
});

test("resolves the installed package from the install result instead of the first dependency", async () => {
  const pluginDataRoot = await mkdtemp(join(tmpdir(), "amitia-dsh-peer-"));
  const source = "npm:@example/dsh-atomic@1.0.0";
  const installed = await installDshPackage(source, {
    pluginDataRoot,
    runInstall: async (_, workspace) => {
      await writeFakePackage(workspace, "1.0.0");
      const packageJsonPath = join(workspace, "package.json");
      const packageJson = JSON.parse(await readFile(packageJsonPath, "utf8"));
      packageJson.dependencies = { "@alpha/dsh-peer": "2.0.0", "@example/dsh-atomic": "1.0.0" };
      await writeFile(packageJsonPath, `${JSON.stringify(packageJson, null, 2)}\n`);
      return { log: "", packageName: "@example/dsh-atomic" };
    },
  });
  assert.equal(installed.pluginId, "@example/dsh-atomic");
  assert.equal(installed.version, "1.0.0");
});

test("keeps the current DSH installation when install fails", async () => {
  const pluginDataRoot = await mkdtemp(join(tmpdir(), "amitia-dsh-rollback-"));
  const source = "npm:@example/dsh-atomic@1.0.0";
  const installed = await installDshPackage(source, {
    pluginDataRoot,
    runInstall: (_, workspace) => writeFakePackage(workspace, "1.0.0"),
  });
  const originalError = new Error("fixture install failed");
  await assert.rejects(installDshPackage("npm:@example/dsh-atomic@2.0.0", {
    pluginDataRoot,
    runInstall: async () => {
      throw originalError;
    },
  }), (error) => error === originalError);
  const packageJson = JSON.parse(await readFile(join(installed.packageDir, "package.json"), "utf8"));
  assert.equal(packageJson.version, "1.0.0");
  const manifest = JSON.parse(await readFile(join(installed.installRoot, "package.json"), "utf8"));
  assert.deepEqual(manifest.dependencies, { "@example/dsh-atomic": "1.0.0" });
});

async function writeFakePackage(workspace, version) {
  const packageDir = join(workspace, "node_modules", "@example", "dsh-atomic");
  await rm(packageDir, { recursive: true, force: true });
  await mkdir(packageDir, { recursive: true });
  await writeFile(join(workspace, "package.json"), `${JSON.stringify({
    name: "amitia-dsh-runtime",
    private: true,
    version: "0.0.0",
    type: "module",
    dependencies: { "@example/dsh-atomic": version },
  }, null, 2)}\n`);
  await writeFile(join(packageDir, "package.json"), `${JSON.stringify({
    name: "@example/dsh-atomic",
    version,
    type: "module",
    exports: { ".": "./index.js" },
    dsh: { manifestVersion: 1 },
  }, null, 2)}\n`);
  await writeFile(join(packageDir, "index.js"), "export function apply() {}\n");
  return { packageName: "@example/dsh-atomic" };
}
