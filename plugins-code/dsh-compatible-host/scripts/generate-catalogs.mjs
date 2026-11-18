import { createHash } from "node:crypto";
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { spawnSync } from "node:child_process";
import {
  CLIENT_SERVICE_POLICY,
  DSH_FACTORY_DEFINITIONS,
  DSH_SLOT_MAPPINGS,
  HOST_SERVICE_POLICY,
  LIFECYCLE_EVENT_POLICY,
  PLATFORM_SUPPORT,
  pendingSlotReason,
} from "./catalog-policy.mjs";
import { HOST_SERVICE_PROVIDER_STATUS, HOST_SERVICE_ROUTES } from "./host-service-routes.mjs";

const packageRoot = resolve(fileURLToPath(new URL("..", import.meta.url)));
const repositoryRoot = resolve(packageRoot, "..", "..");
const catalogRoot = join(packageRoot, "catalog");
const runtimeCatalogPath = join(packageRoot, "src", "runtime", "catalog-summary.generated.mjs");
const clientSlotCatalogPath = join(packageRoot, "src", "client-abi", "slot-catalog.generated.mjs");
const lifecycleEventsPath = join(packageRoot, "src", "runtime", "lifecycle-events.generated.mjs");
const officialReferencesPath = join(catalogRoot, "official-references.v1.json");
const targetVersion = "0.2.2-rc.2";
const targetPackages = Object.freeze({
  dsh: `@deepseek-ai/dsh@${targetVersion}`,
  clientRunner: `@deepseek-ai/dsh-cordis-client-runner@${targetVersion}`,
});

function npmCliPath() {
  const candidates = [
    process.env.AMITIA_NPM_CLI,
    "/usr/local/lib/node_modules/npm/bin/npm-cli.js",
    "/opt/homebrew/lib/node_modules/npm/bin/npm-cli.js",
    "C:\\Program Files\\nodejs\\node_modules\\npm\\bin\\npm-cli.js",
  ].filter(Boolean);
  for (const candidate of candidates) {
    try {
      readFileSync(candidate);
      return candidate;
    } catch {}
  }
  throw new Error("找不到 npm CLI，请通过 AMITIA_NPM_CLI 指定 npm-cli.js");
}

function runNpmPack(spec, destination) {
  const result = spawnSync(process.execPath, [
    npmCliPath(),
    "pack",
    spec,
    "--pack-destination",
    destination,
    "--ignore-scripts",
    "--silent",
  ], {
    cwd: packageRoot,
    encoding: "utf8",
    maxBuffer: 8 * 1024 * 1024,
  });
  if (result.status !== 0) throw new Error(`npm pack ${spec} 失败\n${result.stderr || result.stdout}`);
  return result.stdout.trim().split(/\r?\n/).filter(Boolean).at(-1);
}

function extractPackage(spec, destination, directory) {
  const archive = runNpmPack(spec, destination);
  const target = join(destination, directory);
  mkdirSync(target, { recursive: true });
  const result = spawnSync("tar", ["-xzf", join(destination, archive), "-C", target], {
    encoding: "utf8",
    maxBuffer: 8 * 1024 * 1024,
  });
  if (result.status !== 0) throw new Error(`解压 ${spec} 失败\n${result.stderr || result.stdout}`);
  return join(target, "package");
}

function writeJson(path, value) {
  mkdirSync(dirname(path), { recursive: true });
  writeFileSync(path, `${JSON.stringify(value, null, 2)}\n`);
}

function stripComments(value) {
  return value.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");
}

function methodName(signature) {
  const normalized = signature.replace(/^declare\s+readonly\s+/, "").trim();
  const match = /^(?:async\s+)?([A-Za-z_$][\w$]*)\s*</.exec(normalized)
    || /^(?:async\s+)?([A-Za-z_$][\w$]*)\s*\(/.exec(normalized);
  return match?.[1] || "";
}

function parseServiceMethods(block) {
  const code = stripComments(block);
  const lines = code.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
  const starts = [];
  for (let index = 0; index < lines.length; index += 1) {
    if (/^(?:declare\s+readonly\s+|async\s+|get\s+|set\s+)?[A-Za-z_$][\w$]*(?:\s*<|\s*\()/.test(lines[index])) {
      starts.push(index);
    }
  }
  const signatures = starts.map((start, index) => {
    const end = starts[index + 1] ?? lines.length;
    return lines.slice(start, end).join(" ").replace(/;$/, "").replace(/\s+/g, " ").trim();
  });
  return [...new Set(signatures.filter((signature) => signature.includes("(") && methodName(signature)))];
}

function readOfficialReferences() {
  const payload = JSON.parse(readFileSync(officialReferencesPath, "utf8"));
  const byService = new Map(Object.entries(payload.services || {}));
  return { scannedPackageCount: payload.scannedPackageCount || 0, byService };
}

function hostServiceRoute(key, references) {
  const route = HOST_SERVICE_ROUTES[key];
  if (!route) return null;
  const evidence = references.byService.get(key);
  const provider = HOST_SERVICE_PROVIDER_STATUS[route.provider];
  if (!provider) throw new Error(`unknown host service provider: ${key} -> ${route.provider}`);
  return {
    status: provider.status,
    route: route.route,
    provider: route.provider,
    reason: route.note,
    officialPackages: evidence ? [...evidence.definitionPackages] : [],
    consumerCount: evidence ? evidence.consumerCount : 0,
  };
}

function parseHostServices(root, references) {
  const docsRoot = join(root, "docs", "subsystems");
  const paths = ["approval.md", "client-modules.md", "commands.md", "compaction.md", "core.md", "credentials.md", "filesystem.md", "goal.md", "jobs.md", "llm-streaming.md", "lsp.md", "permission-presets.md", "persistence.md", "plan.md", "ptc-runtime.md", "sandbox.md", "schedule.md", "session-projection.md", "session-query.md", "session-reference.md", "session-telemetry.md", "session-title.md", "session.md", "settings.md", "shell.md", "skills.md", "spill.md", "storage.md", "subagent.md", "subprocess.md", "system-prompt.md", "terminal.md", "token-meter.md", "tools.md", "typert.md", "user-questions.md", "web-server.md", "web.md", "workflow.md", "workspace.md"];
  const entries = [];
  for (const file of paths) {
    const source = join(docsRoot, file);
    const markdown = readFileSync(source, "utf8");
    const sections = markdown.split(/^### /m).slice(1);
    for (const section of sections) {
      const heading = section.split(/\r?\n/, 1)[0].trim();
      const match = /^`ctx\.([^`]+)` — `([^`]+)`/.exec(heading);
      if (!match) continue;
      const key = match[1];
      const service = match[2];
      const block = [...section.matchAll(/```ts cordis-catalog\r?\n([\s\S]*?)```/g)].map((item) => item[1]).join("\n");
      const methods = parseServiceMethods(block);
      const routed = hostServiceRoute(key, references);
      const policy = routed
        ? { status: routed.status, phase: "P11", mode: routed.provider === "none" ? "delegated" : routed.status, reason: routed.reason }
        : HOST_SERVICE_POLICY[key] || {
          status: "pending",
          phase: "P11",
          mode: "pending",
        };
      entries.push({
        key,
        service,
        status: policy.status,
        phase: policy.phase,
        mapping: policy.mode,
        route: routed ? routed.route : "amitia-service",
        provider: routed ? routed.provider : "container",
        reason: policy.reason || (policy.mode === "pending" ? "P11 尚未建立该 Host Service 的 Amitia 通用语义" : ""),
        officialPackages: routed ? routed.officialPackages : [],
        consumerCount: routed ? routed.consumerCount : 0,
        source: `@deepseek-ai/dsh@${targetVersion}:docs/subsystems/${file}`,
        methods,
      });
    }
  }
  const unique = [...new Map(entries.map((entry) => [entry.key, entry])).values()].sort((a, b) => a.key.localeCompare(b.key));
  return unique.map((entry, index) => ({
    id: `DSH-HOST-SERVICE-${String(index + 1).padStart(3, "0")}`,
    ...entry,
    methods: entry.methods.map((signature, methodIndex) => ({
      id: `DSH-HOST-SERVICE-${String(index + 1).padStart(3, "0")}-M${String(methodIndex + 1).padStart(3, "0")}`,
      name: methodName(signature),
      signature,
    })),
  }));
}

function parseHostEvents(root) {
  const docsRoot = join(root, "docs", "subsystems");
  const entries = [];
  for (const file of ["approval.md", "commands.md", "compaction.md", "core.md", "credentials.md", "filesystem.md", "goal.md", "jobs.md", "llm-streaming.md", "permission-presets.md", "sandbox.md", "schedule.md", "session-projection.md", "session-telemetry.md", "session.md", "settings.md", "skills.md", "storage.md", "subagent.md", "system-prompt.md", "tools.md", "user-questions.md", "web-server.md", "web.md", "workflow.md", "workspace.md"]) {
    const source = join(docsRoot, file);
    const markdown = readFileSync(source, "utf8");
    for (const match of markdown.matchAll(/^#### `([^`]+)` — ([^\r\n]+)/gm)) {
      entries.push({
        name: match[1],
        mode: match[2].trim(),
        status: "partial",
        phase: "P9",
        mapping: "composite",
        source: `@deepseek-ai/dsh@${targetVersion}:docs/subsystems/${file}`,
      });
    }
  }
  const unique = [...new Map(entries.map((entry) => [entry.name, entry])).values()].sort((a, b) => a.name.localeCompare(b.name));
  return unique.map((entry, index) => ({
    id: `DSH-HOST-EVENT-${String(index + 1).padStart(3, "0")}`,
    ...entry,
  }));
}

function parseHostTools(root) {
  const markdown = readFileSync(join(root, "docs", "tool-catalog.md"), "utf8");
  const section = markdown.split("## Tool Package Map")[1]?.split("\n## ")[0] || "";
  const entries = [];
  for (const line of section.split(/\r?\n/)) {
    const cells = line.split("|").map((cell) => cell.trim());
    if (cells.length < 5 || !/^`@deepseek-ai\//.test(cells[1])) continue;
    const packageName = cells[1].replaceAll("`", "");
    const names = [...cells[2].matchAll(/`([^`]+)`/g)].map((item) => item[1]);
    for (const name of names) {
      entries.push({
        packageName,
        name,
        status: "container",
        phase: "P9",
        mapping: "container",
        reason: "由已安装 DSH 子插件提供，容器通过 host.tools 动态映射",
        source: `@deepseek-ai/dsh@${targetVersion}:docs/tool-catalog.md`,
      });
    }
  }
  const unique = [...new Map(entries.map((entry) => [`${entry.packageName}:${entry.name}`, entry])).values()]
    .sort((a, b) => a.packageName.localeCompare(b.packageName) || a.name.localeCompare(b.name));
  return unique.map((entry, index) => ({
    id: `DSH-HOST-TOOL-${String(index + 1).padStart(3, "0")}`,
    ...entry,
  }));
}

function parseHostConfigurations(root) {
  const markdown = readFileSync(join(root, "docs", "config-catalog.md"), "utf8");
  const sections = markdown.split(/^## /m).slice(1);
  const entries = [];
  for (const section of sections) {
    const heading = section.split(/\r?\n/, 1)[0].trim();
    const match = /^`(@deepseek-ai\/[^`]+)`$/.exec(heading);
    if (!match) continue;
    const configBlock = /```ts config-catalog\r?\n([\s\S]*?)```/.exec(section)?.[1] || "";
    const source = /- `source`: \[[^\]]+\]\(([^)]+)\)/.exec(section)?.[1] || "";
    entries.push({
      packageName: match[1],
      status: "pending",
      phase: "P11",
      mapping: "pending",
      reason: "P11 尚未完成该配置声明的 Amitia 兼容映射",
      source: `@deepseek-ai/dsh@${targetVersion}:docs/config-catalog.md#${source}`,
      configSha256: createHash("sha256").update(configBlock).digest("hex"),
    });
  }
  return entries
    .sort((a, b) => a.packageName.localeCompare(b.packageName))
    .map((entry, index) => ({
      id: `DSH-HOST-CONFIG-${String(index + 1).padStart(3, "0")}`,
      ...entry,
    }));
}

function parseLifecycleEvents(root) {
  const source = join(root, "docs", "persistence-catalog.md");
  const markdown = readFileSync(source, "utf8");
  const entries = [...markdown.matchAll(/^#### `([^`]+)` — (surface|log-only)$/gm)].map((match) => ({
    name: match[1],
    lifecycle: match[2],
    ...(LIFECYCLE_EVENT_POLICY[match[2]] || {
      status: "pending",
      phase: "P14",
      mapping: "pending",
    }),
    reason: LIFECYCLE_EVENT_POLICY[match[2]] ? "" : "P14 尚未完成该持久化会话事件的事务恢复与清理映射",
    source: `@deepseek-ai/dsh@${targetVersion}:docs/persistence-catalog.md`,
  }));
  return entries
    .sort((a, b) => a.name.localeCompare(b.name))
    .map((entry, index) => ({
      id: `DSH-HOST-LIFECYCLE-${String(index + 1).padStart(3, "0")}`,
      ...entry,
    }));
}

function parseUiSlots(clientSlotApi) {
  const ordinary = [...clientSlotApi]
    .sort((a, b) => a.key.localeCompare(b.key))
    .map((entry, index) => {
      const policy = DSH_SLOT_MAPPINGS[entry.key] || {
        mode: "pending",
        targets: [],
      };
      return {
        id: `DSH-UI-SLOT-${String(index + 1).padStart(3, "0")}`,
        key: entry.key,
        kind: entry.kind,
        scope: entry.scope,
        declaredBy: entry.declaredBy,
        replaceRisk: entry.replaceRisk,
        summary: entry.summary,
        status: policy.mode === "pending" ? "pending" : policy.mode,
        phase: policy.mode === "pending" ? "P12" : (policy.phase || "P10"),
        targets: policy.targets,
        reason: policy.mode === "pending" ? pendingSlotReason(entry.key) : "",
        source: `@deepseek-ai/dsh-cordis-client-runner@${targetVersion}:${entry.source}`,
      };
    });
  const factories = DSH_FACTORY_DEFINITIONS
    .map((entry) => {
      const policy = DSH_SLOT_MAPPINGS[entry.key] || { mode: "pending", targets: [] };
      return {
        key: entry.key,
        kind: "factory",
        scope: entry.scope,
        declaredBy: entry.declaredBy,
        replaceRisk: "none",
        summary: `Official DSH Slot Factory ${entry.key}.`,
        status: policy.mode === "pending" ? "pending" : policy.mode,
        phase: policy.mode === "pending" ? "P12" : (policy.phase || "P10"),
        targets: policy.targets,
        reason: policy.mode === "pending" ? pendingSlotReason(entry.key) : "",
        source: `@deepseek-ai/dsh@${targetVersion}:docs/subsystems/slots.md`,
      };
    })
    .sort((a, b) => a.key.localeCompare(b.key));
  return [...ordinary, ...factories].map((entry, index) => ({
    id: `DSH-UI-SLOT-${String(index + 1).padStart(3, "0")}`,
    ...entry,
  }));
}

function parseClientApi(apiCatalog) {
  const services = apiCatalog.SERVICE_API.map((entry, index) => {
    const policy = CLIENT_SERVICE_POLICY[entry.key] || {
      status: "pending",
      phase: "P13",
      mode: "pending",
      reason: "P13 尚未建立该 Client Service 的 Amitia 通用语义",
    };
    return {
      id: `DSH-CLIENT-SERVICE-${String(index + 1).padStart(3, "0")}`,
      key: entry.key,
      status: policy.status,
      phase: policy.phase,
      mapping: policy.mode,
      reason: policy.reason || "",
      methods: entry.methods.map((method, methodIndex) => ({
        id: `DSH-CLIENT-SERVICE-${String(index + 1).padStart(3, "0")}-M${String(methodIndex + 1).padStart(3, "0")}`,
        name: methodName(method.signature),
        signature: method.signature,
      })),
    };
  });
  const events = apiCatalog.EVENT_API.map((entry, index) => ({
    id: `DSH-CLIENT-EVENT-${String(index + 1).padStart(3, "0")}`,
    name: entry.name,
    mode: entry.mode,
    signature: entry.signature,
    status: "implemented",
    phase: "P10",
    mapping: "composite",
  }));
  return { services, events };
}

function packageIntegrity(path) {
  return `sha256-${createHash("sha256").update(readFileSync(path)).digest("hex")}`;
}

const temporaryRoot = mkdtempSync(join(tmpdir(), "amitia-dsh-catalog-"));
try {
  const dshPackage = extractPackage(targetPackages.dsh, temporaryRoot, "dsh");
  const clientRunnerPackage = extractPackage(targetPackages.clientRunner, temporaryRoot, "client-runner");
  const apiCatalog = await import(pathToFileURL(join(clientRunnerPackage, "src", "client", "api-catalog.ts")).href);
  const slotCatalog = await import(pathToFileURL(join(clientRunnerPackage, "src", "client", "slot-catalog.ts")).href);
  const references = readOfficialReferences();
  const hostServices = parseHostServices(dshPackage, references);
  const hostEvents = parseHostEvents(dshPackage);
  const hostTools = parseHostTools(dshPackage);
  const hostConfigurations = parseHostConfigurations(dshPackage);
  const lifecycleEvents = parseLifecycleEvents(dshPackage);
  const uiSlots = parseUiSlots(slotCatalog.CLIENT_SLOT_API);
  const client = parseClientApi(apiCatalog);
  const meta = {
    schemaVersion: 1,
    catalogVersion: "v1",
    targetDshVersion: targetVersion,
    targetClientUiPrimitivesVersion: targetVersion,
    sourcePackages: [
      {
        spec: targetPackages.dsh,
        packageJsonSha256: packageIntegrity(join(dshPackage, "package.json")),
      },
      {
        spec: targetPackages.clientRunner,
        packageJsonSha256: packageIntegrity(join(clientRunnerPackage, "package.json")),
      },
    ],
  };
  const abi = {
    ...meta,
    host: {
      services: hostServices,
      events: hostEvents,
      tools: hostTools,
      configurations: hostConfigurations,
      lifecycleEvents,
    },
    client,
    inherited: apiCatalog.INHERITED_CTX_API,
    types: apiCatalog.TYPE_API,
  };
  const ui = {
    ...meta,
    slots: uiSlots,
  };
  const mappedSlots = uiSlots.filter((entry) => entry.status !== "pending");
  const summary = {
    targetDshVersion: targetVersion,
    hostServiceCount: hostServices.length,
    hostEventCount: hostEvents.length,
    hostToolCount: hostTools.length,
    hostConfigurationCount: hostConfigurations.length,
    lifecycleEventCount: lifecycleEvents.length,
    clientServiceCount: client.services.length,
    clientEventCount: client.events.length,
    uiSlotCount: uiSlots.length,
    uiSlotMappedCount: mappedSlots.length,
    uiSlotPendingCount: uiSlots.length - mappedSlots.length,
    hostServiceDelegatedCount: hostServices.filter((entry) => entry.status === "delegated").length,
    hostServiceImplementedCount: hostServices.filter((entry) => entry.status === "implemented").length,
    hostServicePartialCount: hostServices.filter((entry) => entry.status === "partial").length,
    hostServiceProviderPendingCount: hostServices.filter((entry) => entry.status === "pending").length,
    hostServiceUnavailableCount: hostServices.filter((entry) => entry.status === "unavailable").length,
    officialReferencePackageCount: references.scannedPackageCount,
  };
  const gaps = {
    ...meta,
    host: hostServices.filter((entry) => entry.status !== "implemented" && entry.status !== "partial").map((entry) => ({
      id: entry.id,
      capability: `ctx.${entry.key}`,
      status: entry.status,
      route: entry.route,
      provider: entry.provider,
      phase: entry.phase,
      reason: entry.reason,
      officialPackages: entry.officialPackages,
      consumerCount: entry.consumerCount,
    })),
    resolved: hostServices.filter((entry) => entry.status === "implemented" || entry.status === "partial").map((entry) => ({
      id: entry.id,
      capability: `ctx.${entry.key}`,
      status: entry.status,
      route: entry.route,
      provider: entry.provider,
      phase: entry.phase,
      reason: entry.reason,
      officialPackages: entry.officialPackages,
      consumerCount: entry.consumerCount,
    })),
    configurations: hostConfigurations.map((entry) => ({
      id: entry.id,
      capability: entry.packageName,
      phase: entry.phase,
      reason: entry.reason,
    })),
    lifecycleEvents: lifecycleEvents.filter((entry) => entry.status === "pending").map((entry) => ({
      id: entry.id,
      capability: entry.name,
      phase: entry.phase,
      reason: entry.reason,
    })),
    client: client.services.filter((entry) => entry.status === "pending" || entry.status === "unavailable").map((entry) => ({
      id: entry.id,
      capability: entry.key,
      phase: entry.phase,
      reason: entry.reason || entry.status,
    })),
    ui: uiSlots.filter((entry) => entry.status === "pending").map((entry) => ({
      id: entry.id,
      capability: entry.key,
      phase: entry.phase,
      reason: entry.reason,
    })),
  };
  const platform = {
    ...meta,
    platforms: PLATFORM_SUPPORT,
  };
  const compatibility = {
    ...meta,
    summary,
    hostServices: hostServices.map((entry) => ({
      id: entry.id,
      capability: entry.key,
      status: entry.status,
      phase: entry.phase,
      mapping: entry.mapping,
      route: entry.route,
      provider: entry.provider,
      reason: entry.reason,
    })),
    clientServices: client.services.map((entry) => ({
      id: entry.id,
      capability: entry.key,
      status: entry.status,
      phase: entry.phase,
      mapping: entry.mapping,
    })),
    uiSlots: uiSlots.map((entry) => ({
      id: entry.id,
      capability: entry.key,
      status: entry.status,
      phase: entry.phase,
      targets: entry.targets,
      reason: entry.reason,
    })),
  };
  writeJson(join(catalogRoot, "dsh-abi.v1.json"), abi);
  writeJson(join(catalogRoot, "dsh-ui-slots.v1.json"), ui);
  writeJson(join(catalogRoot, "amitia-gap.v1.json"), gaps);
  writeJson(join(catalogRoot, "platform-support.v1.json"), platform);
  writeJson(join(catalogRoot, "compatibility.v1.json"), compatibility);
  const resolutions = Object.fromEntries(uiSlots.map((entry) => [entry.key, {
    id: entry.id,
    key: entry.key,
    kind: entry.kind,
    scope: entry.scope,
    status: entry.status,
    phase: entry.phase,
    targets: entry.targets,
    reason: entry.reason,
  }]));
  writeFileSync(clientSlotCatalogPath, `export const DSH_UI_SLOT_RESOLUTIONS = Object.freeze(${JSON.stringify(resolutions, null, 2)});\n`);
  writeFileSync(runtimeCatalogPath, `export const DSH_CATALOG_SUMMARY = Object.freeze(${JSON.stringify(summary, null, 2)});\n`);
  const lifecycleEntries = lifecycleEvents.map((entry) => ({
    id: entry.id,
    name: entry.name,
    lifecycle: entry.lifecycle,
  }));
  const surfaceTypes = lifecycleEvents.filter((entry) => entry.lifecycle === "surface").map((entry) => entry.name);
  writeFileSync(
    lifecycleEventsPath,
    `export const LIFECYCLE_EVENTS = Object.freeze(${JSON.stringify(lifecycleEntries, null, 2)});\n\nexport const SURFACE_EVENT_TYPES = Object.freeze(${JSON.stringify(surfaceTypes, null, 2)});\n`,
  );
  console.log(JSON.stringify({ catalogRoot, runtimeCatalogPath, clientSlotCatalogPath, lifecycleEventsPath, summary }, null, 2));
} finally {
  rmSync(temporaryRoot, { recursive: true, force: true });
}
