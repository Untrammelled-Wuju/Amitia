import assert from "node:assert/strict";
import test from "node:test";
import { createDshClientRuntimeServices } from "../src/client-abi/runtime-services.mjs";

function fixture(options = {}) {
  const commands = [];
  const navigations = [];
  const artifactUploads = [];
  let sidebar = false;
  let workspaceListCalls = 0;
  let toolResultWatchCalls = 0;
  const registeredWorkspaces = Array.isArray(options.projectWorkspaces)
    ? options.projectWorkspaces.map((item) => ({ ...item }))
    : [];
  const services = createDshClientRuntimeServices({
    workspaceChangesIntervalMs: 5,
    command: async (input) => {
      commands.push(input);
      if (input.action === "conversation-create") return { conversationId: "conversation-42" };
      if (input.action === "agent-preset-write") return { key: input.targetId, preset: input.preset, scope: input.scope };
      if (input.action === "conversation-list") {
        return {
          items: [{
            sessionId: "conversation-1",
            title: "Workspace chat",
            displayTitle: "Workspace chat",
            updatedAt: 1_760_000_000_000,
            running: false,
            blank: false,
            projectId: "project-1",
            workspaceId: "project-1:workspace-1",
            cwd: "/workspace",
            formatStatus: "current",
          }],
          workspaces: [{
            workspaceId: "project-1:workspace-1",
            projectId: "project-1",
            path: "/workspace",
            title: "Workspace chat",
            createdAt: "2026-11-14T00:00:00Z",
            updatedAt: "2026-11-14T00:00:00Z",
            sessionIds: ["conversation-1"],
          }],
          archivedSessionIds: [],
          pinnedSessionIds: [],
          hasMore: false,
        };
      }
      if (input.action === "conversation-search") {
        return {
          items: [{ sessionId: "conversation-1", snippet: "Workspace chat" }],
          hasMore: false,
        };
      }
      if (input.action === "conversation-archive") {
        return { conversationId: input.conversationId, archived: input.archived };
      }
      if (input.action === "conversation-reorder") {
        return { conversationIds: [input.conversationId, input.beforeConversationId].filter(Boolean), projectId: input.projectId };
      }
      if (input.action === "project-reorder") {
        return { projectIds: [input.projectId] };
      }
      if (input.action === "project-catalog") return { items: [], nextCursor: "" };
      if (input.action === "project-workspaces") {
        return { items: registeredWorkspaces.map((item) => ({ ...item })) };
      }
      if (input.action === "project-create") {
        const sequence = registeredWorkspaces.length + 1;
        const entry = {
          workspaceId: `registered-workspace-${sequence}`,
          projectId: `registered-project-${sequence}`,
          path: input.directoryPath,
          title: input.title,
          createdAt: "2026-11-15T00:00:00Z",
          updatedAt: "2026-11-15T00:00:00Z",
        };
        registeredWorkspaces.push(entry);
        return {
          workspace: entry,
          project: {
            projectId: entry.projectId,
            title: entry.title,
            workspaces: [{ id: entry.workspaceId, name: entry.title, kind: "local", rootUri: `amitia://workspace/@${entry.workspaceId}/` }],
            createdAt: entry.createdAt,
            updatedAt: entry.updatedAt,
          },
        };
      }
      if (input.action === "project-update") {
        const entry = registeredWorkspaces.find((item) => item.projectId === input.projectId);
        if (entry && typeof input.title === "string") entry.title = input.title;
        return { projectId: input.projectId, revision: 2 };
      }
      if (input.action === "project-delete") {
        const index = registeredWorkspaces.findIndex((item) => item.projectId === input.projectId);
        if (index >= 0) registeredWorkspaces.splice(index, 1);
        return { projectId: input.projectId, deleted: true };
      }
      if (input.action === "schedule-catalog") return [];
      if (input.action === "schedule-create") {
        return {
          id: "schedule-1",
          kind: "after",
          title: input.request.title,
          prompt: input.request.prompt,
          afterSeconds: input.request.timing.after_seconds,
          scheduledAt: "2026-11-14T01:00:00.000Z",
          target: input.request.target,
          involvedSessionIds: [input.request.target.sessionId],
          status: "active",
        };
      }
      if (input.action === "schedule-list") {
        return { items: [], shown: 0, total: 0, limit: input.limit || 100, offset: input.offset || 0 };
      }
      if (input.action === "schedule-view") return { id: input.scheduleId, code: "schedule_not_found" };
      if (input.action === "schedule-cancel") return { id: input.scheduleId, deleted: true };
      if (input.action === "schedule-run") {
        return {
          id: input.scheduleId,
          kind: "after",
          title: "Review",
          prompt: "Review the task",
          afterSeconds: 60,
          scheduledAt: "2026-11-14T01:00:00.000Z",
          target: { kind: "session", sessionId: "conversation-1" },
          involvedSessionIds: ["conversation-1"],
          status: "active",
          executionActive: true,
        };
      }
      if (input.action === "schedule-delete-inactive") return { deleted: 0 };
      if (input.action === "schedule-update") {
        return {
          id: input.scheduleId,
          kind: input.patch.timing.kind,
          title: input.patch.title,
          prompt: input.patch.prompt,
          everySeconds: input.patch.timing.every_seconds,
          scheduledAt: "2026-11-14T01:00:00.000Z",
          target: { kind: "session", sessionId: "conversation-1" },
          involvedSessionIds: ["conversation-1"],
          status: input.patch.active === false ? "paused" : "active",
        };
      }
      if (input.action === "schedule-history") {
        return {
          id: input.scheduleId,
          records: [{
            scheduledAt: "2026-11-14T01:00:00.000Z",
            deliveredAt: "2026-11-14T01:00:01.000Z",
            messageId: "message-1",
            targetSessionId: "conversation-1",
            prompt: "Review the task",
          }],
          earlierRecordsUnavailable: false,
          earlierRecordsPruned: false,
          retention: { days: 30, records: 200 },
        };
      }
      if (input.action === "workspace-files-stat") {
        if (!input.path) throw new Error("workspace path is a directory");
        return { absolutePath: `/workspace/${input.path}`, version: "v1", bytes: 9 };
      }
      if (input.action === "workspace-files-read") {
        return {
          absolutePath: `/workspace/${input.path}`,
          version: "v1",
          bytes: 9,
          offset: input.offset || 1,
          text: "one\ntwo",
          lines: 2,
          eof: false,
        };
      }
      if (input.action === "workspace-files-read-bytes") {
        return {
          absolutePath: `/workspace/${input.path}`,
          version: "v1",
          bytes: 9,
          offset: input.offset || 0,
          dataBase64: Buffer.from("one").toString("base64"),
          eof: false,
        };
      }
      if (input.action === "workspace-files-list") {
        workspaceListCalls += 1;
        return {
          path: input.path || "",
          entries: [{ name: "notes.txt", type: "file", size: 9 + workspaceListCalls }],
          truncated: false,
        };
      }
      if (input.action === "workspace-files-search") {
        return {
          matches: [
            { path: "src", type: "directory" },
            { path: "src/notes.txt", type: "file" },
          ],
          truncated: false,
          incomplete: false,
        };
      }
      if (input.action === "tool-result-watch") {
        if (options.watchToolResult) return options.watchToolResult(input);
        toolResultWatchCalls += 1;
        return toolResultWatchCalls === 1
          ? { available: false }
          : { available: true, value: { output: "persisted" } };
      }
      if (input.action === "status") {
        return {
          packages: [{
            pluginId: "dsh-plugin-fixture",
            name: "Fixture Plugin",
            version: "1.0.0",
            enabled: true,
            source: "npm:dsh-plugin-fixture",
            entries: [{ id: "root", entry: "./index.js" }],
          }],
        };
      }
      if (input.action === "set-enabled") {
        return {
          package: {
            pluginId: input.pluginId,
            enabled: input.enabled,
          },
        };
      }
      if (input.action === "install") {
        return {
          package: {
            pluginId: "dsh-plugin-installed",
            enabled: false,
          },
        };
      }
      if (input.action === "remove") return { packages: [] };
      if (input.action === "conversation-pin") return { conversationId: input.conversationId, pinned: input.pinned };
      throw new Error(`unexpected command ${input.action}`);
    },
    navigate: async (target) => {
      navigations.push(target);
      return target;
    },
    hostActions: {
      toggleSidebar() {
        sidebar = !sidebar;
        return sidebar;
      },
      async selectDirectory() {
        return "/tmp/project";
      },
      async listDirectory(path) {
        return {
          path: path || "/tmp",
          home: "/home/user",
          crumbs: [],
          entries: [{ name: "project", path: "/tmp/project", hidden: false }],
          truncated: false,
        };
      },
      async createDirectory(path, name) {
        return `${path}/${name}`;
      },
    },
    uploadArtifact: async (input) => {
      artifactUploads.push(input);
      input.onProgress?.({ loaded: 5, total: 5 });
      return {
        artifact: {
          artifactId: input.kind === "image" ? "artifact-image-1" : "artifact-file-1",
          filename: input.filename,
          mimeType: input.mediaType || input.file.type,
          sizeBytes: input.file.size,
        },
      };
    },
    ...options,
  });
  return { services, commands, navigations, artifactUploads, sidebar: () => sidebar };
}

test("maps official DSH panels to verified Amitia routes", async () => {
  const { services, navigations, sidebar } = fixture();
  await services.layout.selectPanel(null);
  await services.layout.selectPanel("plugins");
  await services.layout.selectPanel("schedules");
  assert.deepEqual(navigations, ["/chat", "/extensions", "/tasks"]);
  assert.equal(services.layout.toggleSidebar(), true);
  assert.equal(sidebar(), true);
  const firstNavigation = services.layout.beginNavigation();
  const secondNavigation = services.layout.beginNavigation();
  assert.equal(firstNavigation.aborted, true);
  assert.equal(secondNavigation.aborted, false);
  await services.layout.selectPanel("custom-panel");
  assert.deepEqual(navigations, ["/chat", "/extensions", "/tasks", "/main-panel/custom-panel"]);
});

test("creates and opens an Amitia conversation through generic host APIs", async () => {
  const { services, commands, navigations } = fixture();
  const id = await services.uiWorkspace.startSession(undefined, {});
  assert.equal(id, "conversation-42");
  assert.equal(commands[0].action, "conversation-create");
  assert.ok(commands[0].requestId);
  assert.deepEqual(navigations, ["/chat?conversationId=conversation-42"]);
  await services.uiWorkspace.openSession({ sessionId: "conversation 9" });
  assert.equal(navigations[1], "/chat?conversationId=conversation%209");
});

test("projects the real Session and Workspace catalogs into DSH snapshots", async () => {
  const { services } = fixture();
  await services.sessions.refresh();
  await services.workspaces.refresh();
  assert.deepEqual(services.sessions.list.getSnapshot().ids, ["conversation-1"]);
  assert.equal(services.sessions.list.getSnapshot().byId["conversation-1"].displayTitle, "Workspace chat");
  assert.equal(services.sessions.list.getSnapshot().byId["conversation-1"].cwd, "/workspace");
  assert.equal(services.workspaces.list.getSnapshot().items[0].workspaceId, "project-1:workspace-1");
  assert.equal(services.workspaces.list.getSnapshot().items[0].sessionIds[0], "conversation-1");
  const id = await services.sessions.create({
    workspaceId: "project-1:workspace-1",
    sessionId: "dsh-session-reserved",
  });
  assert.equal(id, "conversation-42");
});

test("sessions.create 落 agentPreset 到新会话作用域", async () => {
  const { services, commands } = fixture();
  await services.sessions.refresh();
  await services.workspaces.refresh();
  const id = await services.sessions.create({
    workspaceId: "project-1:workspace-1",
    agentPreset: "reviewer",
  });
  assert.equal(id, "conversation-42");
  assert.deepEqual(
    commands.find((entry) => entry.action === "agent-preset-write"),
    { action: "agent-preset-write", targetId: "conversation-42", preset: "reviewer", scope: "session" },
  );
});

test("searches visible Amitia conversations through the generic host API", async () => {
  const { services, commands } = fixture();
  assert.deepEqual(
    await services.sessions.search("workspace", new AbortController().signal),
    {
      ok: true,
      value: {
        items: [{ sessionId: "conversation-1", snippet: "Workspace chat" }],
        hasMore: false,
      },
    },
  );
  assert.equal(commands.some((entry) => entry.action === "conversation-search" && entry.query === "workspace"), true);
});

test("uploads durable DSH file and image receipts through the Amitia artifact service", async () => {
  const { services, artifactUploads } = fixture();
  const progress = [];
  assert.deepEqual(
    await services.fileUpload.upload("conversation-1", new Blob(["hello"], { type: "text/plain" }), "hello.txt", undefined, (value) => progress.push(value)),
    { ok: true, value: { receiptId: "artifact-file-1" } },
  );
  assert.deepEqual(
    await services.fileUpload.uploadImage("conversation-1", new Blob(["image"], { type: "image/png" }), "image/png", "image.png"),
    { ok: true, value: { receiptId: "artifact-image-1" } },
  );
  assert.equal(artifactUploads.length, 2);
  assert.equal(artifactUploads[0].kind, "file");
  assert.equal(artifactUploads[0].sessionId, "conversation-1");
  assert.equal(artifactUploads[0].filename, "hello.txt");
  assert.equal(artifactUploads[1].kind, "image");
  assert.equal(artifactUploads[1].mediaType, "image/png");
  assert.deepEqual(progress, [{ loaded: 5, total: 5 }]);
});

test("preserves the original artifact upload failure", async () => {
  const cause = new Error("disk full");
  cause.code = "ARTIFACT_WRITE_FAILED";
  const { services } = fixture({
    uploadArtifact: async () => {
      throw new Error("artifact upload rejected", { cause });
    },
  });
  const result = await services.fileUpload.upload(
    "conversation-1",
    new Blob(["hello"], { type: "text/plain" }),
    "hello.txt",
  );
  assert.equal(result.ok, false);
  assert.equal(result.error.code, "FILE_UPLOAD_FAILED");
  assert.equal(result.error.message, "artifact upload rejected");
  assert.equal(result.error.cause.message, "disk full");
  assert.equal(result.error.cause.code, "ARTIFACT_WRITE_FAILED");
});

test("fails explicitly instead of faking unsupported remote semantics", async () => {
  const { services } = fixture();
  assert.deepEqual(
    await services.remote.commands.execute(),
    {
      ok: false,
      error: {
        code: "REMOTE_UNAVAILABLE",
        message: "DSH remote.commands.execute is unavailable (P13): generated Remote endpoint mapping is unavailable",
        detail: {
          operation: "remote.commands.execute",
          phase: "P13",
          reason: "generated Remote endpoint mapping is unavailable",
        },
      },
    },
  );
});

test("projects the DSH compatibility package store through Plugin Manager remotes", async () => {
  const { services, commands } = fixture();
  const inventory = await services.pluginInventory.list();
  assert.equal(inventory.ok, true);
  assert.equal(inventory.value.managementAvailable, true);
  assert.equal(inventory.value.entries[0].entryId, "dsh-plugin-fixture#root");
  assert.equal(inventory.value.entries[0].fiberPhase, "active");

  const bundles = await services.remote.pluginManager.listBundles();
  assert.equal(bundles.ok, true);
  assert.equal(bundles.value[0].name, "dsh-plugin-fixture");
  assert.equal(bundles.value[0].rows[0].entryId, "dsh-plugin-fixture#root");

  const disabled = await services.remote.pluginManager.setBundleEnabled("dsh-plugin-fixture", false);
  assert.equal(disabled.ok, true);
  assert.equal(disabled.value.application, "applied");
  assert.equal(disabled.value.changed, true);
  assert.equal(commands.some((entry) => entry.action === "set-enabled" && entry.enabled === false), true);

  const installed = await services.remote.pluginManager.installBundle("dsh-plugin-installed@1.0.0", { enabled: true });
  assert.equal(installed.ok, true);
  assert.equal(installed.value.stage, "enable");
  assert.equal(installed.value.target, "dsh-plugin-installed");
  assert.equal(commands.some((entry) => entry.action === "install" && entry.spec === "dsh-plugin-installed@1.0.0"), true);

  const removed = await services.remote.pluginManager.removeBundle("dsh-plugin-fixture");
  assert.deepEqual(removed, {
    ok: true,
    value: {
      changed: true,
      application: "applied",
      stage: "remove",
      target: "dsh-plugin-fixture",
      enabled: false,
    },
  });

  assert.deepEqual(await services.remote.pluginRegistryProbe.fastest(), { ok: true, value: null });
  assert.equal((await services.remote.directoryPicker.pick()).value, "/tmp/project");
  assert.equal((await services.remote.directoryPicker.list("/tmp")).value.entries[0].name, "project");
  assert.equal((await services.remote.directoryPicker.createDirectory("/tmp", "new")).value, "/tmp/new");
  assert.deepEqual(await services.remote.pluginManager.cancelInstall("request-1"), {
    ok: true,
    value: { status: "not-running" },
  });
  assert.equal((await services.remote.pluginManager.listVersions("dsh-plugin-fixture")).value.problem, "not-updatable");
});

test("exposes DSH Schedule Remote with session prompt operations", async () => {
  const { services, commands } = fixture();
  assert.deepEqual(
    await services.remote.schedule.deliveryLimits(),
    { ok: true, value: { newSessionIntervalSeconds: 60 } },
  );
  const request = {
    title: "Review",
    prompt: "Review the task",
    timing: { kind: "after", after_seconds: 60 },
    target: { kind: "session", sessionId: "conversation-1" },
  };
  assert.deepEqual(
    await services.remote.schedule.create(request),
    {
      ok: true,
      value: {
        id: "schedule-1",
        kind: "after",
        title: "Review",
        prompt: "Review the task",
        afterSeconds: 60,
        scheduledAt: "2026-11-14T01:00:00.000Z",
        target: request.target,
        involvedSessionIds: ["conversation-1"],
        status: "active",
      },
    },
  );
  assert.deepEqual(await services.remote.schedule.catalog(), { ok: true, value: [] });
  assert.deepEqual(await services.remote.schedule.edit({ id: "schedule-1", action: "delete" }), {
    ok: true,
    value: { id: "schedule-1", deleted: true },
  });
  assert.equal(commands.some((entry) => entry.action === "schedule-create"), true);
  assert.equal((await services.remote.schedule.run({ id: "schedule-1" })).value.executionActive, true);
  assert.equal((await services.remote.schedule.edit({
    id: "schedule-1",
    action: "update",
    patch: {
      title: "Updated",
      prompt: "Updated prompt",
      timing: { kind: "every", every_seconds: 120 },
      active: true,
    },
  })).value.title, "Updated");
  assert.equal((await services.remote.schedule.history({
    id: "schedule-1",
    limit: 20,
  })).value.records[0].messageId, "message-1");
  assert.deepEqual(
    await services.remote.schedule.deleteInactive(),
    { ok: true, value: { deleted: 0 } },
  );
  assert.equal(typeof services.remote.$on("schedule/changed", () => {}), "function");
});

test("maps DSH Schedule created-session cwd through the verified workspace catalog", async () => {
  const { services, commands } = fixture();
  const request = {
    title: "Workspace schedule",
    prompt: "Run in the workspace",
    timing: { kind: "after", after_seconds: 60 },
    target: {
      kind: "new",
      create: { cwd: "/workspace" },
    },
  };
  await services.remote.schedule.create(request);
  const call = commands.find((entry) => entry.action === "schedule-create");
  assert.deepEqual(call.request.target.create, {
    cwd: "/workspace",
    projectId: "project-1",
  });
  assert.deepEqual(request.target.create, { cwd: "/workspace" });
});

test("runs and cleans remote access effects without losing failures", async () => {
  const { services } = fixture();
  let cleaned = 0;
  const dispose = services.remoteAccess.whenAccess("schedule/catalog", function* () {
    yield () => {
      cleaned += 1;
    };
    yield Promise.resolve(() => {
      cleaned += 1;
    });
  });
  await dispose();
  await services.dispose();
  assert.equal(cleaned, 2);
});

test("bridges DSH workspaceFiles calls to verified workspace operations", async () => {
  const { services, commands } = fixture();
  assert.deepEqual(
    await services.remote.workspaceFiles.read("conversation-1", "notes.txt", { offset: 2 }, new AbortController().signal),
    {
      ok: true,
      value: {
        absolutePath: "/workspace/notes.txt",
        version: "v1",
        bytes: 9,
        offset: 2,
        text: "one\ntwo",
        lines: 2,
        eof: false,
      },
    },
  );
  const bytes = await services.remote.workspaceFiles.readBytes(
    "conversation-1",
    "notes.txt",
    { range: { offset: 4, length: 3 } },
    new AbortController().signal,
  );
  assert.equal(bytes.ok, true);
  assert.equal(new TextDecoder().decode(bytes.value.data), "one");
  assert.deepEqual(
    await services.remote.workspaceFiles.stat("conversation-1", "notes.txt"),
    {
      ok: true,
      value: { absolutePath: "/workspace/notes.txt", version: "v1", bytes: 9 },
    },
  );
  assert.deepEqual(
    await services.remote.workspaceFiles.list("conversation-1", "", new AbortController().signal),
    {
      ok: true,
      value: {
        path: "",
        entries: [{ name: "notes.txt", type: "file", size: 10 }],
        truncated: false,
      },
    },
  );
  assert.equal(commands.some((entry) => entry.action === "workspace-files-read-bytes"), true);
});

test("searches workspace paths and polls directory changes", async () => {
  const { services, commands } = fixture();
  const request = {
    query: "notes",
    excludedPaths: [],
    includeHidden: false,
    respectGitignore: true,
  };
  assert.deepEqual(
    await services.remote.workspaceFiles.search("conversation-1", request, new AbortController().signal),
    {
      ok: true,
      value: {
        matches: [
          { path: "src", type: "directory" },
          { path: "src/notes.txt", type: "file" },
        ],
        truncated: false,
        incomplete: false,
      },
    },
  );
  assert.equal(commands.some((entry) => entry.action === "workspace-files-search" && entry.sessionId === "conversation-1"), true);

  const controller = new AbortController();
  const iterator = services.remote.workspaceFiles
    .changes("conversation-1", "", controller.signal)[Symbol.asyncIterator]();
  assert.deepEqual(await iterator.next(), { done: false, value: { kind: "ready" } });
  const changed = await iterator.next();
  assert.equal(changed.done, false);
  assert.equal(changed.value.kind, "change");
  assert.equal(changed.value.change.absolutePath, "");
  assert.match(changed.value.change.version, /^directory-/);
  controller.abort(new DOMException("done", "AbortError"));
  assert.deepEqual(await iterator.next(), { done: true, value: undefined });
});

test("watches persisted DSH tool results by session sequence", async () => {
  const { services, commands } = fixture();
  const controller = new AbortController();
  const iterator = services.remote.toolResults
    .watch({ sessionId: "conversation-1", seq: 7 }, controller.signal)
    [Symbol.asyncIterator]();
  assert.deepEqual(await iterator.next(), { done: false, value: { available: false } });
  assert.deepEqual(await iterator.next(), {
    done: false,
    value: { available: true, value: { output: "persisted" } },
  });
  assert.equal(commands.some((entry) => entry.action === "tool-result-watch" && entry.seq === 7), true);
  controller.abort(new DOMException("done", "AbortError"));
  assert.deepEqual(await iterator.next(), { done: true, value: undefined });
});

test("stops a DSH tool result watch without emitting an in-flight response after cancellation", async () => {
  const response = Promise.withResolvers();
  const started = Promise.withResolvers();
  const { services } = fixture({
    watchToolResult() {
      started.resolve();
      return response.promise;
    },
  });
  const controller = new AbortController();
  const iterator = services.remote.toolResults
    .watch({ sessionId: "conversation-1", seq: 7 }, controller.signal)
    [Symbol.asyncIterator]();
  const update = iterator.next();
  await started.promise;
  controller.abort();
  response.resolve({ available: true, value: { output: "late" } });
  assert.deepEqual(await update, { done: true, value: undefined });
  await services.dispose();
});

test("creates a DSH workspace as an Amitia project with a registered local mount", async () => {
  const { services, commands } = fixture();
  const created = await services.workspaces.create({ path: "/Users/example/vault", title: "Vault" });
  assert.equal(created.workspaceId, "registered-workspace-1");
  assert.equal(created.projectId, "registered-project-1");
  assert.equal(created.path, "/Users/example/vault");
  const create = commands.find((entry) => entry.action === "project-create");
  assert.deepEqual(
    create && { title: create.title, directoryPath: create.directoryPath },
    { title: "Vault", directoryPath: "/Users/example/vault" },
  );
  assert.deepEqual(
    services.workspaces.list.getSnapshot().items.map((item) => item.workspaceId).sort(),
    ["project-1:workspace-1", "registered-workspace-1"],
  );
});

test("rejects workspace mutations without a path or a verified Amitia project mapping", async () => {
  const { services, commands } = fixture();
  await assert.rejects(() => services.workspaces.create({}), /DSH workspaces\.create requires a path/);
  assert.equal(commands.some((entry) => entry.action === "project-create"), false);
  await assert.rejects(
    () => services.workspaces.rename("missing-workspace", "Renamed"),
    /DSH WorkspaceId missing-workspace has no verified Amitia ProjectId mapping/,
  );
  await assert.rejects(
    () => services.workspaces.delete("missing-workspace"),
    /DSH WorkspaceId missing-workspace has no verified Amitia ProjectId mapping/,
  );
  assert.equal(commands.some((entry) => entry.action === "project-update" || entry.action === "project-delete"), false);
});

test("renames and deletes DSH workspaces through the mapped Amitia project", async () => {
  const { services, commands } = fixture();
  await services.workspaces.refresh();
  await services.workspaces.rename("project-1:workspace-1", "Renamed");
  const update = commands.find((entry) => entry.action === "project-update");
  assert.deepEqual(update && { projectId: update.projectId, title: update.title }, { projectId: "project-1", title: "Renamed" });
  const created = await services.workspaces.create({ path: "/Users/example/vault", title: "Vault" });
  assert.equal(await services.workspaces.delete(created.workspaceId), true);
  const remove = commands.find((entry) => entry.action === "project-delete");
  assert.equal(remove && remove.projectId, "registered-project-1");
  assert.equal(
    services.workspaces.list.getSnapshot().items.some((item) => item.workspaceId === created.workspaceId),
    false,
  );
});

test("archives DSH sessions through Amitia archives and reorders through conversation-reorder", async () => {
  const { services, commands } = fixture();
  await services.workspaces.archiveSession("conversation-1", { stopActivity: true });
  await services.workspaces.unarchiveSession("conversation-1");
  const archives = commands.filter((entry) => entry.action === "conversation-archive");
  assert.deepEqual(
    archives.map((entry) => ({ conversationId: entry.conversationId, archived: entry.archived, stopActivity: entry.stopActivity })),
    [
      { conversationId: "conversation-1", archived: true, stopActivity: true },
      { conversationId: "conversation-1", archived: false, stopActivity: undefined },
    ],
  );
  await assert.rejects(() => services.workspaces.archiveSession(""), /DSH workspaces\.archiveSession requires a session id/);
  await assert.rejects(() => services.workspaces.unarchiveSession(""), /DSH workspaces\.unarchiveSession requires a session id/);
  await services.workspaces.refresh();
  await services.workspaces.insertSessionBefore("project-1:workspace-1", "conversation-1", "");
  const reorder = commands.find((entry) => entry.action === "conversation-reorder");
  assert.deepEqual(
    reorder && { conversationId: reorder.conversationId, before: reorder.beforeConversationId, projectId: reorder.projectId },
    { conversationId: "conversation-1", before: "", projectId: "project-1" },
  );
});

test("remote.job mirrors host jobs as rows, follow frames and kill receipts", async () => {
  const jobs = [
    { id: "job-1", kind: "bash", label: "fixture", owner: "session-1", status: "running", startedAt: 1, output: { earliest: 0, total: 5 }, text: "hello" },
  ];
  const command = async (input) => {
    if (input.action === "jobs-list") return { jobs };
    if (input.action === "jobs-kill") {
      jobs[0].status = "cancelled";
      return { ok: true, alreadyFinished: false };
    }
    throw new Error(`unexpected action ${input.action}`);
  };
  const services = createDshClientRuntimeServices({ command, navigate: () => {} });
  const job = services.remote.job;
  const listIterator = job.list({ sessionId: "session-1" })[Symbol.asyncIterator]();
  const rows = await listIterator.next();
  assert.equal(rows.value.type, "rows");
  assert.equal(rows.value.jobs[0].id, "job-1");
  assert.equal(rows.value.jobs[0].owner, "session-1");
  assert.equal(rows.value.jobs[0].output.total, 5);
  const controller = new AbortController();
  controller.abort();
  await listIterator.next();
  const followIterator = job.follow({ jobId: "job-1" })[Symbol.asyncIterator]();
  const opened = await followIterator.next();
  assert.equal(opened.value.type, "opened");
  assert.equal(opened.value.from, 0);
  const output = await followIterator.next();
  assert.equal(output.value.type, "output");
  assert.equal(output.value.chunks[0].text, "hello");
  assert.equal(output.value.next, 5);
  const receipt = await job.kill({ sessionId: "session-1", jobId: "job-1" });
  assert.deepEqual(receipt, { ok: true, value: { outcome: "requested" } });
});

test("retains, scopes and searches DSH sessions through the verified catalogs", async () => {
  const { services, commands } = fixture();
  await services.sessions.refresh();
  const reference = services.sessions.retain({ sessionId: "conversation-1" }, { owner: "workspace" });
  assert.deepEqual(
    { sessionId: reference.sessionId, retainCount: reference.retainCount, owner: reference.owner },
    { sessionId: "conversation-1", retainCount: 1, owner: "workspace" },
  );
  assert.equal(typeof reference.release, "function");
  const second = services.sessions.retain("conversation-1");
  assert.equal(second.retainCount, 2);
  second.release();
  reference.release();
  reference.release();
  assert.throws(
    () => services.sessions.retain(undefined),
    /DSH sessions\.retain requires a resolvable session target/,
  );
  assert.equal(services.sessions.searchResultLimit, 20);
  const scope = services.sessions.scopeOf("conversation-1");
  assert.equal(scope.sessionId, "conversation-1");
  assert.equal(typeof scope.get, "function");
  assert.equal(typeof scope.effect, "function");
  assert.equal(services.sessions.scopeOf({ sessionId: "conversation-1" }), scope);
  assert.equal(services.sessions.sessionOf(scope), "conversation-1");
  assert.equal(services.sessions.sessionOf("conversation-1"), "conversation-1");
  assert.equal(services.sessions.scope(), undefined);
  assert.equal(services.sessions.scope("conversation-1"), scope);
  assert.equal(services.sessions.binding("conversation-1").sessionId, "conversation-1");
  assert.equal(await services.sessions.using("conversation-1", {}, (value) => value.sessionId), "conversation-1");
  await services.sessions.search("workspace", new AbortController().signal);
  assert.equal(commands.find((entry) => entry.action === "conversation-search").limit, 20);
});

test("initializes the DSH default workspace from the existing workspace catalog", async () => {
  const { services, commands } = fixture();
  const workspace = await services.workspaces.initializeDefault();
  assert.equal(workspace.workspaceId, "project-1:workspace-1");
  assert.equal(commands.some((entry) => entry.action === "project-create"), false);
});

test("creates the DSH default workspace from the host home directory", async () => {
  const commands = [];
  const services = createDshClientRuntimeServices({
    command: async (input) => {
      commands.push(input);
      if (input.action === "conversation-list") {
        return { items: [], workspaces: [], archivedSessionIds: [], pinnedSessionIds: [] };
      }
      if (input.action === "project-workspaces") return { items: [] };
      if (input.action === "project-create") {
        return {
          workspace: {
            workspaceId: "default-workspace",
            projectId: "default-project",
            path: input.directoryPath,
            title: input.title,
          },
        };
      }
      throw new Error(`unexpected command ${input.action}`);
    },
    navigate: () => {},
    hostActions: {
      async listDirectory() {
        return { path: "/Users/example", home: "/Users/example", crumbs: [], entries: [], truncated: false };
      },
    },
  });
  const workspace = await services.workspaces.initializeDefault();
  assert.deepEqual(
    { workspaceId: workspace.workspaceId, path: workspace.path, title: workspace.title },
    { workspaceId: "default-workspace", path: "/Users/example", title: "example" },
  );
  const create = commands.find((entry) => entry.action === "project-create");
  assert.deepEqual({ directoryPath: create.directoryPath, title: create.title }, { directoryPath: "/Users/example", title: "example" });
  await services.dispose();
});

test("DSH session pinning 通过 conversation-pin 落到宿主置顶", async () => {
  const { services, commands } = fixture();
  await services.workspaces.pinSession("conversation-1");
  await services.workspaces.unpinSession("conversation-1", { workspaceId: "project-1:workspace-1" });
  assert.deepEqual(
    commands.filter((entry) => entry.action === "conversation-pin").map((entry) => ({ id: entry.conversationId, pinned: entry.pinned })),
    [{ id: "conversation-1", pinned: true }, { id: "conversation-1", pinned: false }],
  );
  await assert.rejects(() => services.workspaces.pinSession(""), /DSH workspaces\.pinSession requires a session id/);
  await assert.rejects(() => services.workspaces.unpinSession(""), /DSH workspaces\.unpinSession requires a session id/);
  await services.dispose();
});

test("exposes DSH resource access sources and resource-scoped effects", async () => {
  const { services } = fixture();
  const source = services.remoteAccess.source("workspace", "conversation-1", "read");
  assert.deepEqual(
    source.getSnapshot(),
    { resource: "workspace", id: "conversation-1", action: "read", allowed: true },
  );
  assert.equal(source.allowed, true);
  const published = [];
  assert.equal(typeof source.subscribe(() => published.push("changed")), "function");
  assert.throws(() => services.remoteAccess.source(""), /DSH remoteAccess resource is required/);
  assert.throws(
    () => services.remoteAccess.whenResourceAccess("workspace", "conversation-1", "write"),
    /DSH remoteAccess callback is required/,
  );
  let cleaned = 0;
  const dispose = services.remoteAccess.whenResourceAccess("workspace", "conversation-1", "write", () => () => {
    cleaned += 1;
  });
  assert.equal(typeof dispose, "function");
  await dispose();
  await dispose();
  await services.dispose();
  assert.equal(cleaned, 1);
  assert.deepEqual(published, []);
});
