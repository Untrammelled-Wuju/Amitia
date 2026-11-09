import { readFileSync, writeFileSync } from "node:fs";
import { relative, resolve } from "node:path";

const root = resolve(import.meta.dirname, "..");
const files = process.argv.slice(2);

const contributionAdapters = {
  tool: "tool",
  agent_skill: "agent_skill",
  workflow: "workflow",
  mcp_server: "mcp_server",
  hook: "hook",
  event_subscription: "event",
  event: "event",
  schedule: "schedule",
  ui_page: "ui",
  ui_panel: "ui",
  ui_chat: "ui",
  ui_context_action: "ui",
  ui_desktop: "ui",
  ui_provider: "ui_provider",
  ui_slot: "ui_slot",
  background_task: "background",
  background: "background",
  provider: "provider",
  gamex: "game",
  petx: "desktop_pet",
};

const runtimeAdapters = {
  javascript: "javascript",
  service: "service",
  wasm: "wasm",
  native: "native",
  builtin: "builtin",
  static: "static",
  mcp: "mcp",
  workflow: "workflow",
};

for (const file of files) {
  const absolute = resolve(root, file);
  const source = JSON.parse(readFileSync(absolute, "utf8"));
  if (!source.manifestVersion && !source.extension) continue;
  const extension = source.extension ?? {};
  const publisher = source.publisher ?? extension.publisher ?? {};
  const placement = source.placement ?? "";
  const modules = (source.modules ?? []).map((module) => migrateModule(module, placement));
  const migrated = {
    schemaVersion: 1,
    plugin: {
      id: extension.id ?? source.id,
      name: extension.name ?? { default: extension.id ?? source.id },
      description: extension.description ?? { default: "" },
      version: extension.version ?? source.version,
      ...(extension.license ? { license: extension.license } : {}),
      ...(extension.homepage ? { homepage: extension.homepage } : {}),
      ...(extension.repository ? { repository: extension.repository } : {}),
      ...(extension.categories ? { categories: extension.categories } : {}),
      ...(extension.keywords ? { keywords: extension.keywords } : {}),
      ...(extension.icon ? { icon: extension.icon } : {}),
      ...(extension.metadata ? { metadata: extension.metadata } : {}),
      publisher: {
        id: publisher.id ?? "unknown",
        displayName: publisher.displayName ?? publisher.id ?? "unknown",
        ...(publisher.trustLevel ? { trustLevel: publisher.trustLevel } : {}),
        ...(publisher.contact ? { contact: publisher.contact } : {}),
        ...(publisher.website ? { website: publisher.website } : {}),
      },
    },
    compatibility: {
      minHostVersion: source.compatibility?.minHostVersion || "0.1.0",
      ...(source.compatibility?.maxHostVersion ? { maxHostVersion: source.compatibility.maxHostVersion } : {}),
      ...(source.compatibility?.platforms ? { platforms: source.compatibility.platforms } : {}),
      ...(source.compatibility?.featureFlags ? { featureFlags: source.compatibility.featureFlags } : {}),
    },
    modules,
    ...(source.dependencies?.length ? { dependencies: source.dependencies } : {}),
    ...(source.permissions?.length ? { permissions: source.permissions } : {}),
    ...(source.resources?.length ? { resources: source.resources } : {}),
    ...(source.secretRefs?.length ? { secretRefs: source.secretRefs } : {}),
    data: {
      schemaVersion: source.data?.schemaVersion ?? 1,
      backup: source.data?.backup ?? true,
      export: source.data?.export ?? true,
      uninstall: source.data?.uninstall ?? "retain",
    },
    integrity: {
      algorithm: source.integrity?.algorithm ?? "sha256",
      contentTreeHash: source.integrity?.contentTreeHash ?? "",
      ...(source.integrity?.fileHashes ? { fileHashes: source.integrity.fileHashes } : {}),
    },
    ...(source.development
      ? {
          development: {
            ...(source.development.hotReload !== undefined ? { hotReload: source.development.hotReload } : {}),
            ...(source.development.sourceMaps !== undefined ? { sourceMaps: source.development.sourceMaps } : {}),
            ...(source.development.testEntry ? { testEntry: source.development.testEntry } : {}),
            ...(source.development.watchPaths ? { watchPaths: source.development.watchPaths } : {}),
          },
        }
      : {}),
  };
  writeFileSync(absolute, `${JSON.stringify(migrated, null, 2)}\n`);
  process.stdout.write(`${relative(root, absolute)}\n`);
}

function migrateModule(module, extensionPlacement) {
  const contributions = (module.contributions ?? []).map(migrateContribution);
  const adapters = [...new Set(contributions.map((contribution) => contribution.adapter))];
  const requires = (module.dependencies ?? [])
    .filter((dependency) => dependency.type === "provider")
    .map((dependency) => ({
      serviceId: dependency.id,
      ...(dependency.version ? { versionRange: dependency.version } : {}),
      ...(dependency.optional ? { optional: true } : {}),
      ...(dependency.reason ? { reason: dependency.reason } : {}),
    }));
  const provides = (module.providedCapabilities ?? []).map((capability) => ({
    serviceId: capability.id,
    version: capability.version || "1.0.0",
    scopes: ["global"],
    placement: module.placement || extensionPlacement || "device",
  }));
  const runtime = module.runtime
    ? {
        adapter: runtimeAdapters[module.runtime.type] ?? module.runtime.type,
        ...(module.runtime.serviceId ? { serviceId: module.runtime.serviceId } : {}),
        ...(module.runtime.entryPoint ? { entry: module.runtime.entryPoint } : {}),
        ...(module.runtime.workerCount ? { workerCount: module.runtime.workerCount } : {}),
        ...(module.runtime.timeout ? { timeout: module.runtime.timeout } : {}),
        ...(module.runtime.memory ? { memoryBytes: module.runtime.memory } : {}),
        ...(module.runtime.cpuPercent ? { cpuPercent: module.runtime.cpuPercent } : {}),
        ...(module.runtime.maxSubprocesses ? { maxProcesses: module.runtime.maxSubprocesses } : {}),
        ...(module.runtime.permissions ? { permissions: module.runtime.permissions } : {}),
        ...(module.runtime.capabilities ? { capabilities: module.runtime.capabilities } : {}),
        ...(module.runtime.env ? { environment: module.runtime.env } : {}),
        ...(module.runtime.hostAccess ? { hostAccess: module.runtime.hostAccess } : {}),
        ...(module.runtime.nativeCompanions
          ? {
              nativeArtifacts: module.runtime.nativeCompanions.map((artifact) => ({
                id: artifact.id,
                platform: artifact.platform,
                ...(artifact.architecture ? { architecture: artifact.architecture } : {}),
                path: artifact.path,
                sha256: artifact.sha256,
                executable: artifact.executable,
                ...(artifact.args ? { arguments: artifact.args } : {}),
              })),
            }
          : {}),
      }
    : undefined;
  return {
    id: module.id,
    name: module.name,
    ...(module.description ? { description: module.description } : {}),
    ...(module.version ? { version: module.version } : {}),
    ...(runtime ? { runtime } : {}),
    ...(provides.length || requires.length ? { services: { provides, requires } } : {}),
    ...(adapters.length
      ? {
          dynamic: {
            allowedAdapters: adapters,
            maxContributions: contributions.length,
            maxTools: contributions.filter((contribution) => contribution.adapter === "tool").length,
            maxRoutes: contributions.filter((contribution) => contribution.adapter === "ui" || contribution.adapter === "ui_provider").length,
            maxServices: provides.length,
            allowUnregister: true,
          },
        }
      : {}),
    ...(contributions.length ? { contributions } : {}),
    ...(module.permissions?.length ? { permissions: module.permissions } : {}),
    ...(module.resources?.length ? { resources: module.resources } : {}),
    ...(module.dependencies?.length ? { dependencies: module.dependencies } : {}),
    placement: module.placement || extensionPlacement || "device",
    ...(module.compatibility ? { compatibility: module.compatibility } : {}),
    ...(module.policies ? { policies: module.policies } : {}),
    ...(module.deviceRequirements ? { deviceRequirements: module.deviceRequirements } : {}),
    ...(module.providedCapabilities ? { providedCapabilities: module.providedCapabilities } : {}),
    ...(module.provider ? { provider: module.provider } : {}),
  };
}

function migrateContribution(contribution) {
  return {
    id: contribution.id,
    kind: contribution.kind,
    adapter: contribution.adapter ?? contributionAdapters[contribution.kind],
    contractVersion: contribution.contractVersion ?? 1,
    ...(contribution.schema ? { schema: contribution.schema } : {}),
    name: contribution.name,
    ...(contribution.description ? { description: contribution.description } : {}),
    ...(contribution.version ? { version: contribution.version } : {}),
    ...(contribution.spec ? { spec: contribution.spec } : {}),
    ...(contribution.requiredPermissions ? { requiredPermissions: contribution.requiredPermissions } : {}),
    ...(contribution.requiredScope ? { requiredScope: contribution.requiredScope } : {}),
    ...(contribution.exposure ? { exposure: contribution.exposure } : {}),
    ...(contribution.runtimeBinding ? { runtimeBinding: contribution.runtimeBinding } : {}),
    ...(contribution.dependencies ? { dependencies: contribution.dependencies } : {}),
  };
}
