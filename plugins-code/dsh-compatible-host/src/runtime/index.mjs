import {
  STATE_KEY,
  defaultState,
  normalizeState,
  removePackage,
  setPackageEnabled,
  upsertPackage,
} from "./state.mjs";
import {
  installDshPackage,
  restoreDshTreeLock,
  listClientDependencyPackages,
  readDshClientSource,
  readInstalledClientSource,
  removeDshPackageInstallation,
  resolvePluginDataRoot,
} from "./package-manager.mjs";
import { assertDshHostPluginActive, createDshHostContext, invokeDshTool, loadDshHostPlugin } from "./host-abi.mjs";
import { mapDshTool } from "./tool-mapping.mjs";
import { createDshScheduleDefinition, mapDshScheduleCatalog, updateDshScheduleDefinition } from "./schedule-adapter.mjs";
import {
  createAmitiaDshHostServices,
  createAmitiaDshPromptState,
  snapshotAmitiaDshPromptState,
} from "./host-services.mjs";
import { compatibilitySummary, TARGET_DSH_VERSION } from "./abi.mjs";
import { assertSessionStateVersion, restoreSessionEvents } from "./session-events.mjs";
import { pluginDataRootFromPackageDir, providerConfig } from "./provider-wiring.mjs";
import { runModelDiagnose } from "./diagnose.mjs";
import * as modelAdapterModule from "./model-adapter.mjs";

const activePlugins = new Map();
const activationFailures = new Map();
const containerRuntimes = new Map();
let detachedContainer = null;
let activeContext = null;
let promptProviderRegistration = null;
const promptState = createAmitiaDshPromptState();

const modelAdapterPlugin = modelAdapterModule.plugin;

function ensureModelAdapter(container, host) {
  if (container.modelAdapter) return container.modelAdapter;
  const state = { provider: "amitia", state: "pending", error: null };
  container.modelAdapter = state;
  const fiber = container.context.plugin(modelAdapterPlugin, {
    host,
    logger: {
      warn: (message) => console.warn(message),
      error: (message) => console.error(message),
    },
  });
  Promise.resolve(fiber).then(
    () => {
      state.state = "registered";
      console.error("[dsh-model-adapter] provider=amitia state=registered");
    },
    (error) => {
      state.state = "failed";
      state.error = {
        name: error?.name ?? "Error",
        message: error?.message ?? String(error),
        stack: error?.stack ?? null,
      };
      console.error(`[original-error] amitia model adapter 注册失败: ${state.error.stack ?? state.error.message}`);
    },
  );
  return state;
}

function requireContainer(host) {
  const container = containerRuntimes.get(host) ?? detachedContainer;
  if (!container) throw new Error("DSH 容器尚未创建：请先启用任意 DSH 子插件");
  return container;
}

function modelAdapterStatus() {
  const containers = [...containerRuntimes.values()];
  if (detachedContainer) containers.push(detachedContainer);
  return containers.map((container) => container.modelAdapter ?? { provider: "amitia", state: "absent", error: null });
}

function resolveContainer(context, services) {
  const host = context?.host ?? null;
  if (!host) {
    if (!detachedContainer) {
      detachedContainer = createDshHostContext({ pluginId: "dsh-compatible-host", services });
    }
    return detachedContainer.context;
  }
  let container = containerRuntimes.get(host);
  if (!container) {
    container = createDshHostContext({ pluginId: "dsh-compatible-host", services });
    ensureModelAdapter(container, host);
    containerRuntimes.set(host, container);
  }
  return container.context;
}

async function disposeContainers() {
  const containers = [...containerRuntimes.values()];
  containerRuntimes.clear();
  if (detachedContainer) containers.push(detachedContainer);
  detachedContainer = null;
  for (const container of containers) await container.dispose();
}

async function activatePromptProvider(context) {
  context.handlers.bindTool("dsh_prompt_context", async () => {
    const snapshot = snapshotAmitiaDshPromptState(promptState);
    return {
      sections: snapshot.sections.map((section) => ({
        id: section.name,
        content: section.text,
        priority: -section.order,
      })),
      contexts: snapshot.contexts,
      variables: snapshot.variables,
      tools: snapshot.tools,
    };
  });
  promptProviderRegistration = await context.contributions.register({
    id: "dsh.prompt-context",
    kind: "tool",
    adapter: "tool",
    contractVersion: 1,
    name: { default: "DSH system prompt context" },
    description: { default: "Provides DSH system-prompt sections to the Amitia prompt pipeline." },
    metadata: {
      "amitia.context.provides": ["chat.realtime.prompt"],
      "amitia.context.priority": 250,
      "amitia.context.key": "dsh-system-prompt",
    },
    spec: {
      toolId: "dsh/prompt-context",
      modelName: "dsh_prompt_context",
      handlerName: "dsh_prompt_context",
      inputSchema: { type: "object", properties: {}, additionalProperties: false },
      outputSchema: { type: "object", additionalProperties: true },
      riskLevel: "low",
      sideEffect: "read_only",
      internal: true,
      metadata: {
        "amitia.context.provides": ["chat.realtime.prompt"],
        "amitia.context.priority": 250,
        "amitia.context.key": "dsh-system-prompt",
      },
      runtime: {
        runtimeType: "javascript",
        handlerName: "dsh_prompt_context",
      },
    },
  });
}

async function deactivatePromptProvider() {
  if (promptProviderRegistration) await promptProviderRegistration.dispose();
  promptProviderRegistration = null;
  promptState.sections.length = 0;
  promptState.contexts.length = 0;
  promptState.variables.clear();
  promptState.toolProviders.length = 0;
}

async function activatePackage(record, context = null) {
  await deactivatePackage(record.pluginId);
  const services = context?.host ? createAmitiaDshHostServices(context.host, promptState.sections, promptState) : {};
  const containerContext = resolveContainer(context, services);
  const config = await providerConfig(record.pluginId, pluginDataRootFromPackageDir(record.packageDir));
  const runtimes = [];
  const registrations = [];
  try {
    for (const entry of record.entries) {
      const runtimePluginId = entry.id === "root" ? record.pluginId : `${record.pluginId}#${entry.id}`;
      const loaded = await loadDshHostPlugin({
        packageDir: record.packageDir,
        entry: entry.entry,
        pluginId: runtimePluginId,
        services,
        context: containerContext,
        config,
        deferInjectCheck: true,
      });
      runtimes.push({ id: entry.id, runtime: loaded });
      if (context) {
        for (const tool of loaded.tools) {
          const mapped = mapDshTool({ ...record, pluginId: runtimePluginId }, tool);
          context.handlers.bindTool(mapped.handlerName, (input, invocation) => {
            const conversationId = invocation?.runtime?.scope?.conversationId;
            const agent = conversationId && services.agents
              ? services.agents.get(conversationId) ?? services.agents.agentForTool(conversationId)
              : undefined;
            return invokeDshTool(loaded, tool.name, input, {
              callId: invocation?.toolCallId,
              agent,
            });
          });
          const registration = await context.contributions.register(mapped.descriptor);
          registrations.push(registration);
        }
      }
    }
    for (const item of runtimes) await assertDshHostPluginActive(item.runtime);
  } catch (error) {
    if (services.jobs?.cancelAll) services.jobs.cancelAll("DSH plugin activation failed");
    for (let index = registrations.length - 1; index >= 0; index -= 1) await registrations[index]?.dispose();
    for (let index = runtimes.length - 1; index >= 0; index -= 1) await runtimes[index]?.runtime.dispose();
    throw error;
  }
  const active = { runtimes, registrations, services };
  activePlugins.set(record.pluginId, active);
  activationFailures.delete(record.pluginId);
  return active;
}

async function deactivatePackage(pluginId) {
  activationFailures.delete(pluginId);
  const current = activePlugins.get(pluginId);
  if (!current) return;
  activePlugins.delete(pluginId);
  if (current.services?.jobs?.cancelAll) current.services.jobs.cancelAll("DSH plugin deactivated");
  for (let index = current.registrations.length - 1; index >= 0; index -= 1) {
    await current.registrations[index]?.dispose();
  }
  for (let index = current.runtimes.length - 1; index >= 0; index -= 1) {
    await current.runtimes[index]?.runtime.dispose();
  }
}

async function retryFailedActivations(host) {
  if (!activeContext) return;
  const snapshot = await readState(host);
  for (let pass = 0; pass < 4; pass += 1) {
    let progressed = false;
    for (const record of snapshot.state.packages) {
      if (!record.enabled || activePlugins.has(record.pluginId)) continue;
      try {
        await activatePackage(record, activeContext);
        progressed = true;
      } catch (error) {
        activationFailures.set(record.pluginId, {
          name: error?.name ?? "Error",
          message: error?.message ?? String(error),
          stack: error?.stack ?? null,
        });
      }
    }
    if (!progressed) break;
  }
}

async function readState(host) {
  const response = await host.call("host.state.get", { key: STATE_KEY });
  return {
    state: normalizeState(response && response.found ? response.value : defaultState()),
    version: response && Number.isFinite(Number(response.version)) ? Number(response.version) : 0,
  };
}

async function mutateState(host, mutate) {
  for (let attempt = 0; attempt < 8; attempt += 1) {
    const snapshot = await readState(host);
    const next = mutate(snapshot.state);
    next.updatedAt = new Date().toISOString();
    const response = await host.call("host.state.cas", {
      key: STATE_KEY,
      expectedVersion: snapshot.version,
      newValue: next,
    });
    if (response && response.swapped) return next;
  }
  throw new Error("DSH 兼容宿主状态写入冲突");
}

function summary(state) {
  return {
    targetDshVersion: state.targetDshVersion || TARGET_DSH_VERSION,
    compatibility: compatibilitySummary(),
    packageCount: state.packages.length,
    enabledCount: state.packages.filter((item) => item.enabled).length,
    packages: state.packages.map((item) => ({
      pluginId: item.pluginId,
      name: item.name,
      version: item.version,
      enabled: item.enabled,
      entry: item.entry,
      entries: item.entries,
      clientEntry: item.clientEntry,
      clientEntries: item.clientEntries,
      entryCount: item.entries.length,
      source: item.source,
      active: activePlugins.has(item.pluginId),
      activationError: activationFailures.get(item.pluginId) ?? null,
      mappedContributionCount: activePlugins.get(item.pluginId)?.registrations.length ?? 0,
    })),
    updatedAt: state.updatedAt,
  };
}

function clientSourceChunk(source, request) {
  const executableSource = source.code.replace(/\/\/# sourceMappingURL=.*$/gm, "");
  const bytes = Buffer.from(executableSource, "utf8");
  const offset = Math.min(bytes.length, Math.max(0, Number(request.offset) || 0));
  const length = Math.min(256 * 1024, Math.max(1, Number(request.length) || 256 * 1024));
  const end = Math.min(bytes.length, offset + length);
  return {
    pluginId: source.pluginId,
    path: source.path,
    dataBase64: bytes.subarray(offset, end).toString("base64"),
    offset,
    nextOffset: end,
    totalBytes: bytes.length,
    done: end >= bytes.length,
    ...(source.installedOnly ? { installedOnly: true } : {}),
  };
}

const clientDependencyCache = new Map();

async function clientDependenciesFor(record) {
  if (!record) return [];
  const clientEntries = Array.isArray(record.clientEntries) && record.clientEntries.length > 0
    ? record.clientEntries
    : record.clientEntry ? [{ id: "root", entry: record.clientEntry }] : [];
  if (clientEntries.length === 0) return [];
  const cacheKey = `${record.packageDir || ""}::${record.clientEntry || ""}`;
  if (!cacheKey) return [];
  if (clientDependencyCache.has(cacheKey)) return clientDependencyCache.get(cacheKey);
  const value = await listClientDependencyPackages(record);
  clientDependencyCache.set(cacheKey, value);
  return value;
}

function jobView(job, pluginId) {
  const output = job.output && typeof job.output === "object" && !Array.isArray(job.output) ? job.output : null;
  const text = output ? String(job.text ?? "") : Array.isArray(job.output) ? job.output.join("") : "";
  const total = output ? Number(output.total) || 0 : new TextEncoder().encode(text).length;
  return {
    id: job.id,
    pluginId,
    owner: String(job.owner || job.sessionId || "").trim(),
    kind: job.kind,
    label: job.label,
    status: job.status === "cancelled" ? "killed" : job.status,
    progress: job.progress,
    detail: job.detail,
    startedAt: job.startedAt,
    finishedAt: job.finishedAt,
    output: {
      total,
      earliest: output ? Number(output.earliest) || 0 : 0,
    },
    text,
  };
}

function activeJobRecords(pluginId = "") {
  const requested = String(pluginId || "").trim();
  const items = [];
  for (const [activePluginId, active] of activePlugins) {
    if (requested && requested !== activePluginId) continue;
    for (const job of active.services?.jobs?.list?.() ?? []) {
      items.push(jobView(job, activePluginId));
    }
  }
  return items;
}

async function scheduleCatalog(host) {
  return mapDshScheduleCatalog(await host.call("host.schedule.list", {}));
}

function projectScopeInput(request) {
  const scope = {};
  const spaceId = String(request.spaceId || "").trim();
  const characterId = String(request.characterId || "").trim();
  if (spaceId) scope.spaceId = spaceId;
  if (characterId) scope.characterId = characterId;
  return scope;
}

const WORKSPACE_INDEX_KEY = "dsh-workspace-index";

async function conversationScopeSnapshotId(host, conversationId) {
  const response = await host.call("host.state.get", { key: `dsh-session:${conversationId}` });
  if (response && response.found) {
    const value = response.value && typeof response.value === "object" ? response.value : {};
    const persisted = typeof value.scopeSnapshotId === "string" ? value.scopeSnapshotId.trim() : "";
    if (persisted) return persisted;
  }
  const created = await host.call("host.conversation.scope", { conversationId });
  const minted = typeof created?.scopeSnapshotId === "string" ? created.scopeSnapshotId.trim() : "";
  if (!minted) throw new Error(`Amitia host.conversation.scope returned no scope snapshot id: ${JSON.stringify(created)}`);
  return minted;
}

async function ownedConversation(host, conversationId) {
  const target = String(conversationId || "").trim();
  if (!target) return null;
  let offset = 0;
  for (;;) {
    const page = await host.call("host.conversation.list", { limit: 128, offset });
    const items = Array.isArray(page?.items) ? page.items : [];
    const found = items.find((item) => String(item?.sessionId || item?.conversationId || item?.id || "").trim() === target);
    if (found) return found;
    if (items.length === 0 || page?.hasMore !== true) return null;
    offset += items.length;
  }
}

function workspaceIndexEntry(value) {
  if (!value || typeof value !== "object") return null;
  const workspaceId = String(value.workspaceId || "").trim();
  if (!workspaceId) return null;
  return {
    workspaceId,
    projectId: String(value.projectId || "").trim(),
    path: String(value.path || "").trim(),
    title: String(value.title || "").trim() || workspaceId,
    createdAt: String(value.createdAt || "").trim(),
    updatedAt: String(value.updatedAt || "").trim(),
  };
}

async function readWorkspaceIndex(host) {
  const response = await host.call("host.state.get", { key: WORKSPACE_INDEX_KEY });
  const value = response && response.found && response.value && typeof response.value === "object" ? response.value : {};
  const items = Array.isArray(value.items) ? value.items.map(workspaceIndexEntry).filter(Boolean) : [];
  return { items, version: Number(response?.version) || 0 };
}

async function mutateWorkspaceIndex(host, mutate) {
  for (let attempt = 0; attempt < 8; attempt += 1) {
    const snapshot = await readWorkspaceIndex(host);
    const next = { items: mutate(snapshot.items.slice()), updatedAt: new Date().toISOString() };
    const response = await host.call("host.state.cas", {
      key: WORKSPACE_INDEX_KEY,
      expectedVersion: snapshot.version,
      newValue: next,
    });
    if (response && response.swapped) return next;
  }
  throw new Error("DSH workspace 索引写入冲突");
}

export async function command(host, input) {
  const request = input && typeof input === "object" ? input : {};
  const action = String(request.action || "").trim();
  switch (action) {
    case "status": {
      const snapshot = await readState(host);
      console.error(`[dsh-status] packages=${snapshot.state.packages.length} version=${snapshot.version}`);
      console.error(`[dsh-model-adapter-status] ${JSON.stringify(modelAdapterStatus())}`);
      if (request.toolInput?.diagnostic != null) {
        console.error(`[dsh-ui-diagnostic] ${JSON.stringify(request.toolInput.diagnostic).slice(0, 24000)}`);
      }
      const view = { ...summary(snapshot.state), modelAdapter: modelAdapterStatus() };
      for (const item of view.packages) {
        const record = snapshot.state.packages.find((entry) => entry.pluginId === item.pluginId);
        item.clientDependencies = await clientDependenciesFor(record);
      }
      return view;
    }
    case "list": {
      const snapshot = await readState(host);
      return { packages: summary(snapshot.state).packages };
    }
    case "register": {
      const next = await mutateState(host, (state) => upsertPackage(state, request.plugin));
      return { package: summary(next).packages.find((item) => item.pluginId === String(request.plugin?.pluginId || "").trim()) };
    }
    case "restore-tree": {
      return restoreDshTreeLock({ pluginDataRoot: request.pluginDataRoot });
    }
    case "install": {
      try {
        const installed = await installDshPackage(request.spec, {
          pluginDataRoot: request.pluginDataRoot,
        });
        console.error(`[dsh-install] plugin=${installed.pluginId} version=${installed.version} packageDir=${installed.packageDir}`);
        const next = await mutateState(host, (state) => upsertPackage(state, installed));
        console.error(`[dsh-install-store] packages=${next.packages.length}`);
        return {
          package: summary(next).packages.find((item) => item.pluginId === installed.pluginId),
          packageDir: installed.packageDir,
        };
      } catch (error) {
        console.error(`[dsh-install-error] ${error instanceof Error ? error.stack ?? error.message : String(error)}`);
        throw error;
      }
    }
    case "set-enabled": {
      const pluginId = String(request.pluginId || "").trim();
      const snapshot = await readState(host);
      const existing = snapshot.state.packages.find((item) => item.pluginId === pluginId);
      if (!existing) throw new Error(`DSH 插件 ${pluginId} 未登记`);
      if (!request.enabled) {
        await deactivatePackage(pluginId);
        activationFailures.delete(pluginId);
        const next = await mutateState(host, (state) => setPackageEnabled(state, pluginId, false));
        return { package: summary(next).packages.find((item) => item.pluginId === pluginId) };
      }
      const next = await mutateState(host, (state) => setPackageEnabled(state, pluginId, true));
      activationFailures.delete(pluginId);
      try {
        await activatePackage(existing, activeContext);
      } catch (error) {
        activationFailures.set(pluginId, {
          name: error?.name ?? "Error",
          message: error?.message ?? String(error),
          stack: error?.stack ?? null,
        });
        console.error(`[dsh-activate-error] ${pluginId} ${error?.stack ?? error?.message ?? String(error)}`);
        await retryFailedActivations(host);
        throw error;
      }
      await retryFailedActivations(host);
      return { package: summary(next).packages.find((item) => item.pluginId === pluginId) };
    }
    case "remove": {
      const pluginId = String(request.pluginId || "").trim();
      const snapshot = await readState(host);
      const record = snapshot.state.packages.find((item) => item.pluginId === pluginId);
      if (!record) throw new Error(`DSH 插件 ${pluginId} 未登记`);
      await deactivatePackage(pluginId);
      const next = await mutateState(host, (state) => removePackage(state, request.pluginId));
      await removeDshPackageInstallation(record);
      return summary(next);
    }
    case "reset": {
      for (const pluginId of Array.from(activePlugins.keys())) await deactivatePackage(pluginId);
      const next = await mutateState(host, () => defaultState());
      return summary(next);
    }
    case "diagnose": {
      const container = requireContainer(host);
      const sessionId = String(request.conversationId || "").trim() || `dsh-diagnose-${Date.now().toString(36)}`;
      let approvalScopeSnapshotId = "";
      let approvalConversationId = "";
      {
        const created = await host.call("host.conversation.create", {
          spaceId: String(request.spaceId || "").trim(),
          characterId: String(request.characterId || "").trim(),
          ...(String(request.approvalConversationId || "").trim()
            ? { conversationId: String(request.approvalConversationId).trim() }
            : {}),
        });
        approvalConversationId = String(created?.conversationId || "").trim();
        approvalScopeSnapshotId = String(created?.scopeSnapshotId || "").trim();
        if (!approvalConversationId || !approvalScopeSnapshotId) {
          throw new Error(`host.conversation.create 未返回会话与作用域快照: ${JSON.stringify(created)}`);
        }
      }
      const approvalDisposers = [];
      try {
      const report = await runModelDiagnose({
        host,
        llm: container.context.get("llm"),
        agents: container.context.get("agents"),
        spillStore: container.context.get("spillStore"),
        model: String(request.model || "").trim(),
        prompt: String(request.prompt || "").trim(),
        sessionId,
        cwd: String(request.cwd || "").trim() || process.cwd(),
        dispose: request.dispose === true,
        commandLine: String(request.commandLine || "").trim(),
        toolProbe: request.toolProbe === true,
        approvalProbe: request.approvalProbe === true,
        approvalTimeoutMs: Number(request.approvalTimeoutMs) > 0 ? Number(request.approvalTimeoutMs) : undefined,
        approvalScopeSnapshotId,
        approvalConversationId,
        installApprovalAnswerer: (answerer) => {
          const off = container.context.on("approval/request", answerer);
          if (typeof off === "function") approvalDisposers.push(off);
          return off;
        },
        maxTokens: Number(request.limit) > 0 ? Number(request.limit) : undefined,
      });
      console.error(
        `[dsh-diagnose] provider=${report.provider} model=${report.model} llmChunks=${JSON.stringify(report.llm.chunkTypes)} turnEnd=${report.turn.turnEnd} assistantMessages=${report.turn.assistantMessages} approvals=${report.turn.approvalDecided} spill=${report.spill ? report.spill.bytes : 0}`,
      );
      return report;
      } finally {
        for (const off of approvalDisposers) {
          try {
            off();
          } catch (error) {
            console.error(`[dsh-diagnose] 移除审批监听器失败: ${error?.stack ?? error}`);
          }
        }
      }
    }
    case "invoke": {
      const pluginId = String(request.pluginId || "").trim();
      const active = activePlugins.get(pluginId);
      if (!active) throw new Error(`DSH 插件 ${pluginId} 未启用`);
      const entryId = String(request.entryId || "").trim();
      const candidates = active.runtimes
        .filter((item) => !entryId || item.id === entryId)
        .filter((item) => item.runtime.tools.some((tool) => tool.name === request.toolName));
      if (candidates.length === 0) throw new Error(`DSH 工具 ${request.toolName} 不存在或未启用`);
      if (candidates.length > 1) throw new Error(`DSH 工具 ${request.toolName} 在多个子入口中存在，请指定 entryId`);
      return {
        value: await invokeDshTool(candidates[0].runtime, request.toolName, request.toolInput, {
          callId: request.callId,
        }),
      };
    }
    case "client-source": {
      const snapshot = await readState(host);
      const requestedPluginId = String(request.pluginId || "").trim();
      const record = snapshot.state.packages.find((item) => item.pluginId === requestedPluginId);
      if (!record) {
        if (!requestedPluginId) throw new Error("DSH client-source 需要 pluginId");
        return clientSourceChunk(await readInstalledClientSource(requestedPluginId, request.entryId, resolvePluginDataRoot()), request);
      }
      return clientSourceChunk(await readDshClientSource(record, request.entryId), request);
    }
    case "conversation-create": {
      const createInput = {
        title: String(request.title || "").trim(),
        projectId: String(request.projectId || "").trim(),
        channel: String(request.channel || "").trim(),
        peerId: String(request.peerId || "").trim(),
        requestId: String(request.requestId || "").trim(),
      };
      const requestedConversationId = String(request.conversationId || "").trim();
      if (requestedConversationId) {
        const existing = await ownedConversation(host, requestedConversationId);
        if (existing) {
          return {
            conversationId: requestedConversationId,
            scopeSnapshotId: await conversationScopeSnapshotId(host, requestedConversationId),
          };
        }
        createInput.conversationId = requestedConversationId;
      }
      const result = await host.call("host.conversation.create", createInput);
      const conversationId = String(result?.conversationId || result?.sessionId || result?.id || "");
      if (!conversationId) throw new Error(`Amitia host.conversation.create returned no conversation id: ${JSON.stringify(result)}`);
      const scopeSnapshotId = String(result?.scopeSnapshotId || "").trim();
      if (!scopeSnapshotId) throw new Error(`Amitia host.conversation.create returned no scope snapshot id: ${JSON.stringify(result)}`);
      return { conversationId, scopeSnapshotId };
    }
    case "conversation-list": {
      return host.call("host.conversation.list", {
        limit: request.limit,
        offset: request.offset,
        archived: request.archived === true,
      });
    }
    case "directory-list": {
      const input = { limit: Number(request.limit) || 200 };
      const path = String(request.path || "").trim();
      if (path) input.path = path;
      return host.call("host.directory.list", input);
    }
    case "directory-create": {
      const path = String(request.path || "").trim();
      if (!path) throw new Error("directory-create 需要绝对路径 path");
      const input = { path };
      const name = String(request.name || "").trim();
      if (name) input.name = name;
      return host.call("host.directory.create", input);
    }
    case "model-defaults": {
      return host.call("host.model.defaults", {});
    }
    case "skill-list": {
      const input = { limit: Number(request.limit) || 50 };
      const query = String(request.query || "").trim();
      if (query) input.query = query;
      if (typeof request.enabled === "boolean") input.enabled = request.enabled;
      return host.call("host.skill.list", input);
    }
    case "conversation-pin": {
      const conversationId = String(request.conversationId || "").trim();
      if (!conversationId) throw new Error("conversationId 不能为空");
      if (typeof request.pinned !== "boolean") throw new Error("pinned 必须为布尔值");
      const input = { conversationId, pinned: request.pinned };
      const requestId = String(request.requestId || "").trim();
      if (requestId) input.requestId = requestId;
      return host.call("host.conversation.pin", input);
    }
    case "conversation-scope": {
      const conversationId = String(request.conversationId || "").trim();
      if (!conversationId) throw new Error("conversationId 不能为空");
      const scopeSnapshotId = await conversationScopeSnapshotId(host, conversationId);
      return { conversationId, scopeSnapshotId };
    }
    case "conversation-read": {
      const conversationId = String(request.conversationId || "").trim();
      if (!conversationId) throw new Error("conversationId 不能为空");
      const input = {
        conversationId,
        limit: Number(request.limit) || 100,
        offset: Number(request.offset) || 0,
      };
      const scopeSnapshotId = String(request.scopeSnapshotId || "").trim() || await conversationScopeSnapshotId(host, conversationId);
      const result = scopeSnapshotId && typeof host.callTargeted === "function"
        ? host.callTargeted("host.conversation.read", input, scopeSnapshotId)
        : host.call("host.conversation.read", input);
      return await result;
    }
    case "conversation-message-send": {
      const conversationId = String(request.conversationId || "").trim();
      if (!conversationId) throw new Error("conversationId 不能为空");
      const content = String(request.content || "");
      if (!content.trim()) throw new Error("发送内容不能为空");
      const input = {
        conversationId,
        content,
        channel: String(request.channel || "").trim(),
        requestId: String(request.requestId || "").trim(),
      };
      const spaceId = String(request.spaceId || "").trim();
      const characterId = String(request.characterId || "").trim();
      if (spaceId) input.spaceId = spaceId;
      if (characterId) input.characterId = characterId;
      if (!input.requestId) input.requestId = `dsh-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
      const scopeSnapshotId = String(request.scopeSnapshotId || "").trim() || await conversationScopeSnapshotId(host, conversationId);
      const result = scopeSnapshotId && typeof host.callTargeted === "function"
        ? host.callTargeted("host.conversation.message.send", input, scopeSnapshotId)
        : host.call("host.conversation.message.send", input);
      return await result;
    }
    case "conversation-search": {
      const query = String(request.query || "").trim();
      if (!query) throw new Error("conversation search query 不能为空");
      return host.call("host.conversation.search", {
        query,
        limit: request.limit,
      });
    }
    case "conversation-delete": {
      const conversationId = String(request.conversationId || "").trim();
      if (!conversationId) throw new Error("conversationId 不能为空");
      const input = {
        conversationId,
        requestId: String(request.requestId || "").trim(),
      };
      const scopeSnapshotId = String(request.scopeSnapshotId || "").trim();
      const result = scopeSnapshotId && typeof host.callTargeted === "function"
        ? host.callTargeted("host.conversation.delete", input, scopeSnapshotId)
        : host.call("host.conversation.delete", input);
      const deleted = await result;
      await host.call("host.state.delete", { key: `dsh-session:${conversationId}` });
      return deleted;
    }
    case "conversation-prepare": {
      const prepareInput = {
        title: String(request.title || "").trim(),
        requestId: String(request.requestId || "").trim(),
      };
      const preparedConversationId = String(request.conversationId || "").trim();
      if (preparedConversationId) prepareInput.conversationId = preparedConversationId;
      if (!prepareInput.requestId) prepareInput.requestId = `dsh-prepare-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
      const result = await host.call("host.conversation.prepare", prepareInput);
      const conversationId = String(result?.conversationId || result?.sessionId || result?.id || "");
      if (!conversationId) throw new Error(`Amitia host.conversation.prepare returned no conversation id: ${JSON.stringify(result)}`);
      return {
        conversationId,
        draft: result?.draft !== false,
        preparedAt: String(result?.preparedAt || ""),
        title: String(result?.title || ""),
      };
    }
    case "conversation-fork": {
      const conversationId = String(request.conversationId || "").trim();
      if (!conversationId) throw new Error("conversation-fork 需要 conversationId");
      const input = { conversationId };
      const boundary = Number(request.boundarySequence);
      if (Number.isFinite(boundary) && boundary > 0) input.boundarySequence = Math.trunc(boundary);
      const title = String(request.title || "").trim();
      if (title) input.title = title;
      return host.call("host.conversation.fork", input);
    }
    case "conversation-archive": {
      const conversationId = String(request.conversationId || "").trim();
      if (!conversationId) throw new Error("conversationId 不能为空");
      const input = {
        conversationId,
        archived: request.archived === true,
        stopActivity: request.stopActivity === true,
        requestId: String(request.requestId || "").trim(),
      };
      const scopeSnapshotId = String(request.scopeSnapshotId || "").trim() || await conversationScopeSnapshotId(host, conversationId);
      const result = scopeSnapshotId && typeof host.callTargeted === "function"
        ? host.callTargeted("host.conversation.archive", input, scopeSnapshotId)
        : host.call("host.conversation.archive", input);
      return await result;
    }
    case "project-catalog": {
      return host.call("host.project.list", {
        ...projectScopeInput(request),
        limit: request.limit,
        cursor: String(request.cursor || "").trim(),
      });
    }
    case "project-workspaces": {
      return { items: (await readWorkspaceIndex(host)).items };
    }
    case "project-create": {
      const directoryPath = String(request.directoryPath || "").trim();
      if (!directoryPath) throw new Error("project-create directoryPath 不能为空");
      const fallbackTitle = directoryPath.split(/[\\/]+/).filter(Boolean).pop() || "Workspace";
      const result = await host.call("host.project.create", {
        ...projectScopeInput(request),
        title: String(request.title || "").trim() || fallbackTitle,
        directoryPath,
        requestId: String(request.requestId || "").trim(),
      });
      const project = result && typeof result.project === "object" ? result.project : null;
      if (!project) throw new Error(`Amitia host.project.create returned no project: ${JSON.stringify(result)}`);
      const created = Array.isArray(project.workspaces) ? project.workspaces[0] : null;
      const workspaceId = created ? String(created.id || "").trim() : "";
      if (!workspaceId) throw new Error(`Amitia host.project.create returned no workspace: ${JSON.stringify(project)}`);
      const entry = {
        workspaceId,
        projectId: String(project.id || "").trim(),
        path: directoryPath,
        title: String(project.title || "").trim() || String(created.name || "").trim(),
        createdAt: String(project.createdAt || "").trim(),
        updatedAt: String(project.updatedAt || "").trim(),
      };
      await mutateWorkspaceIndex(host, (items) => [...items.filter((item) => item.workspaceId !== workspaceId), entry]);
      return { workspace: entry, project };
    }
    case "project-update": {
      const projectId = String(request.projectId || "").trim();
      if (!projectId) throw new Error("project-update projectId 不能为空");
      const input = { ...projectScopeInput(request), projectId, requestId: String(request.requestId || "").trim() };
      if (Object.prototype.hasOwnProperty.call(request, "title")) input.title = String(request.title ?? "").trim();
      if (Object.prototype.hasOwnProperty.call(request, "pinned")) input.pinned = Boolean(request.pinned);
      if (Object.prototype.hasOwnProperty.call(request, "hidden")) input.hidden = Boolean(request.hidden);
      const result = await host.call("host.project.update", input);
      if (Object.prototype.hasOwnProperty.call(input, "title")) {
        await mutateWorkspaceIndex(host, (items) => items.map((item) => item.projectId === projectId
          ? { ...item, title: input.title || item.title, updatedAt: new Date().toISOString() }
          : item));
      }
      return result;
    }
    case "project-delete": {
      const projectId = String(request.projectId || "").trim();
      if (!projectId) throw new Error("project-delete projectId 不能为空");
      const result = await host.call("host.project.delete", {
        ...projectScopeInput(request),
        projectId,
        requestId: String(request.requestId || "").trim(),
      });
      await mutateWorkspaceIndex(host, (items) => items.filter((item) => item.projectId !== projectId));
      return result;
    }
    case "workspace-files-stat": {
      return host.call("host.workspace.read", {
        operation: "file_stat",
        payload: {
          sessionId: String(request.sessionId || "").trim(),
          path: String(request.path || "").trim(),
        },
      });
    }
    case "workspace-files-read": {
      return host.call("host.workspace.read", {
        operation: "file_read_text",
        payload: {
          sessionId: String(request.sessionId || "").trim(),
          path: String(request.path || "").trim(),
          offset: request.offset,
          limit: request.limit,
        },
      });
    }
    case "workspace-files-read-bytes": {
      return host.call("host.workspace.read", {
        operation: "file_read_bytes",
        payload: {
          sessionId: String(request.sessionId || "").trim(),
          path: String(request.path || "").trim(),
          baseFile: String(request.baseFile || "").trim(),
          offset: request.offset,
          length: request.length,
        },
      });
    }
    case "workspace-files-list": {
      return host.call("host.workspace.read", {
        operation: "file_list",
        payload: {
          sessionId: String(request.sessionId || "").trim(),
          path: String(request.path || "").trim(),
        },
      });
    }
    case "workspace-files-search": {
      const searchRequest = request.request && typeof request.request === "object" ? request.request : {};
      return host.call("host.workspace.read", {
        operation: "file_search",
        payload: {
          sessionId: String(request.sessionId || "").trim(),
          query: String(searchRequest.query || "").trim(),
          excludedPaths: Array.isArray(searchRequest.excludedPaths) ? searchRequest.excludedPaths : [],
          includeHidden: searchRequest.includeHidden === true,
          respectGitignore: searchRequest.respectGitignore === true,
        },
      });
    }
    case "tool-result-watch": {
      const sessionId = String(request.sessionId || "").trim();
      const seq = Number(request.seq);
      if (!sessionId) throw new Error("tool-result-watch sessionId 不能为空");
      if (!Number.isSafeInteger(seq) || seq < 0) throw new Error("tool-result-watch seq 必须是非负安全整数");
      const response = await host.call("host.state.get", { key: `dsh-session:${sessionId}` });
      let events = [];
      if (response?.found) {
        assertSessionStateVersion(response.value, sessionId);
        events = restoreSessionEvents(response.value.events, sessionId).events;
      }
      const event = events.find((item) => Number(item?.seq) === seq && item?.type === "tool/result");
      if (!event || !Object.prototype.hasOwnProperty.call(event.data || {}, "liveResult")) {
        return { available: false };
      }
      return { available: true, value: event.data.liveResult };
    }
    case "schedule-create": {
      const definition = createDshScheduleDefinition(request.request);
      await host.call("host.schedule.create", definition);
      const items = await scheduleCatalog(host);
      const entry = items.find((item) => item.id === definition.scheduleId);
      if (!entry) throw new Error(`Amitia host.schedule.create did not return schedule ${definition.scheduleId}`);
      return entry;
    }
    case "schedule-catalog": {
      return scheduleCatalog(host);
    }
    case "schedule-list": {
      const all = await scheduleCatalog(host);
      const filtered = request.active === undefined
        ? all
        : all.filter((item) => (item.status === "active") === Boolean(request.active));
      const limit = Math.min(500, Math.max(1, Number(request.limit) || 100));
      const offset = Math.max(0, Number(request.offset) || 0);
      const items = filtered.slice(offset, offset + limit);
      return { items, shown: items.length, total: filtered.length, limit, offset };
    }
    case "schedule-view": {
      const scheduleId = String(request.scheduleId || "").trim();
      if (!scheduleId) throw new Error("scheduleId 不能为空");
      return (await scheduleCatalog(host)).find((item) => item.id === scheduleId)
        || { id: scheduleId, code: "schedule_not_found" };
    }
    case "schedule-cancel": {
      const scheduleId = String(request.scheduleId || "").trim();
      if (!scheduleId) throw new Error("scheduleId 不能为空");
      try {
        await host.call("host.schedule.cancel", {
          scheduleId,
          expectedUpdatedAt: request.expectedUpdatedAt,
        });
      } catch (error) {
        if (error?.code === "resource_not_found") return { id: scheduleId, deleted: false, code: "schedule_not_found" };
        if (error?.code === "state_conflict") return { id: scheduleId, code: "schedule_conflict" };
        throw error;
      }
      return { id: scheduleId, deleted: true };
    }
    case "schedule-run": {
      const scheduleId = String(request.scheduleId || "").trim();
      if (!scheduleId) throw new Error("scheduleId 不能为空");
      await host.call("host.schedule.run", { scheduleId });
      return (await scheduleCatalog(host)).find((item) => item.id === scheduleId)
        || { id: scheduleId, code: "schedule_not_found" };
    }
    case "schedule-delete-inactive": {
      const ended = (await scheduleCatalog(host)).filter((item) => item.status === "ended");
      for (const item of ended) await host.call("host.schedule.cancel", { scheduleId: item.id });
      return { deleted: ended.length };
    }
    case "schedule-update": {
      const scheduleId = String(request.scheduleId || "").trim();
      if (!scheduleId) throw new Error("scheduleId 不能为空");
      const raw = await host.call("host.schedule.list", {});
      const definition = (raw?.schedules || []).find((item) => item.scheduleId === scheduleId);
      const state = raw?.states?.[scheduleId];
      if (!definition || !state) return { id: scheduleId, code: "schedule_not_found" };
      const expected = request.expectedUpdatedAt;
      if (expected !== undefined && expected !== state.updatedAt) return { id: scheduleId, code: "schedule_conflict" };
      const updated = updateDshScheduleDefinition(definition, request.patch);
      try {
        await host.call("host.schedule.update", {
          scheduleId,
          expectedUpdatedAt: expected ?? state.updatedAt,
          definition: updated.definition,
          enabled: updated.enabled,
        });
      } catch (error) {
        if (error?.code === "state_conflict") return { id: scheduleId, code: "schedule_conflict" };
        throw error;
      }
      return (await scheduleCatalog(host)).find((item) => item.id === scheduleId)
        || { id: scheduleId, code: "schedule_not_found" };
    }
    case "schedule-history": {
      const scheduleId = String(request.scheduleId || "").trim();
      if (!scheduleId) throw new Error("scheduleId 不能为空");
      try {
        return await host.call("host.schedule.history", {
          scheduleId,
          limit: request.limit,
          before: String(request.before || "").trim(),
        });
      } catch (error) {
        if (error?.code === "resource_not_found") return { id: scheduleId, code: "schedule_not_found" };
        throw error;
      }
    }
    case "jobs-list": {
      return { jobs: activeJobRecords(request.pluginId) };
    }
    case "jobs-kill": {
      const jobId = String(request.jobId || "").trim();
      if (!jobId) throw new Error("jobId 不能为空");
      const pluginId = String(request.pluginId || "").trim();
      const candidates = [];
      for (const [activePluginId, active] of activePlugins) {
        if (pluginId && pluginId !== activePluginId) continue;
        const job = active.services?.jobs?.get?.(jobId);
        if (job) candidates.push({ activePluginId, active, job });
      }
      if (candidates.length === 0) throw new Error(`DSH 后台任务 ${jobId} 不存在或插件未启用`);
      if (candidates.length > 1) throw new Error(`DSH 后台任务 ${jobId} 在多个子插件中存在，请指定 pluginId`);
      const candidate = candidates[0];
      const alreadyFinished = candidate.job.status !== "running";
      if (!alreadyFinished) {
        candidate.active.services.jobs.cancel(jobId, request.reason || "用户停止");
      }
      return {
        ok: true,
        alreadyFinished,
        pluginId: candidate.activePluginId,
        job: jobView(candidate.job, candidate.activePluginId),
      };
    }
    default:
      throw new Error(`未知 DSH 兼容宿主操作: ${action}`);
  }
}

export const dshCompatibleExtension = {
  async activate(context) {
    activeContext = context;
    context.handlers.bindTool("command", (input) => command(context.host, input));
    await activatePromptProvider(context);
    const snapshot = await readState(context.host);
    const pending = snapshot.state.packages.filter((item) => item.enabled);
    let progressed = true;
    while (pending.length && progressed) {
      progressed = false;
      for (let index = pending.length - 1; index >= 0; index -= 1) {
        const record = pending[index];
        try {
          await activatePackage(record, context);
          pending.splice(index, 1);
          progressed = true;
        } catch (error) {
          activationFailures.set(record.pluginId, {
            name: error?.name ?? "Error",
            message: error?.message ?? String(error),
            stack: error?.stack ?? null,
          });
          console.error(`[dsh-activate-failed] plugin=${record.pluginId} ${error?.stack ?? error}`);
        }
      }
    }
  },
  async deactivate() {
    for (const pluginId of Array.from(activePlugins.keys())) await deactivatePackage(pluginId);
    await deactivatePromptProvider();
    await disposeContainers();
    activeContext = null;
  },
};

if (typeof module !== "undefined" && module.exports) {
  module.exports = dshCompatibleExtension;
}

if (typeof globalThis.defineExtension === "function") {
  globalThis.defineExtension(dshCompatibleExtension);
}
