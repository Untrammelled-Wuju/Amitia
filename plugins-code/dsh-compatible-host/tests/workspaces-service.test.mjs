import assert from "node:assert/strict";
import test from "node:test";
import { createDshClientRuntimeServices } from "../src/client-abi/runtime-services.mjs";

const NOW = "2026-11-17T00:00:00Z";

function fixture(options = {}) {
  const commands = [];
  const handlers = options.handlers ?? {};
  const state = {
    conversations: options.conversations ?? [{
      sessionId: "session-1",
      title: "Session one",
      displayTitle: "Session one",
      updatedAt: 1_760_000_000_000,
      running: false,
      blank: false,
      projectId: "project-1",
      workspaceId: "workspace-1",
      cwd: "/tmp/one",
      formatStatus: "current",
    }],
    workspaces: options.workspaces ?? [{
      workspaceId: "workspace-1",
      projectId: "project-1",
      path: "/tmp/one",
      title: "One",
      sessionIds: ["session-1"],
      createdAt: NOW,
      updatedAt: NOW,
    }],
    projects: [],
  };
  const services = createDshClientRuntimeServices({
    workspaceChangesIntervalMs: 25,
    command: async (input) => {
      commands.push({ ...input });
      const handler = handlers[input.action];
      if (typeof handler === "function") return await handler(input, state);
      if (input.action === "conversation-list") {
        return {
          items: state.conversations,
          workspaces: state.workspaces,
          archivedSessionIds: [],
          pinnedSessionIds: [],
          hasMore: false,
        };
      }
      if (input.action === "project-workspaces") {
        return { items: state.projects.map((item) => ({ ...item })) };
      }
      if (input.action === "project-create") {
        const sequence = state.projects.length + 1;
        const workspace = {
          workspaceId: `workspace-created-${sequence}`,
          projectId: `project-created-${sequence}`,
          path: input.directoryPath,
          title: input.title,
          createdAt: NOW,
          updatedAt: NOW,
        };
        state.projects.push(workspace);
        return {
          workspace,
          project: {
            id: workspace.projectId,
            title: workspace.title,
            workspaces: [{ id: workspace.workspaceId, name: workspace.title, kind: "local" }],
            createdAt: NOW,
            updatedAt: NOW,
          },
        };
      }
      if (input.action === "project-update") {
        const target = [...state.workspaces, ...state.projects].find((item) => item.projectId === input.projectId);
        if (target && typeof input.title === "string") target.title = input.title;
        return { projectId: input.projectId, revision: 2 };
      }
      if (input.action === "conversation-reorder") {
        const target = state.workspaces.find((item) => item.projectId === input.projectId);
        const moving = String(input.conversationId || "");
        if (target) {
          const rest = target.sessionIds.filter((id) => id !== moving);
          const index = input.beforeConversationId ? rest.indexOf(input.beforeConversationId) : rest.length;
          rest.splice(index < 0 ? rest.length : index, 0, moving);
          target.sessionIds = rest;
        }
        return { conversationIds: target ? [...target.sessionIds] : [moving], projectId: input.projectId };
      }
      if (input.action === "project-reorder") {
        const moving = state.projects.find((item) => item.projectId === input.projectId);
        if (moving) {
          const rest = state.projects.filter((item) => item !== moving);
          const index = input.beforeProjectId ? rest.findIndex((item) => item.projectId === input.beforeProjectId) : rest.length;
          rest.splice(index < 0 ? rest.length : index, 0, moving);
          state.projects.splice(0, state.projects.length, ...rest);
        }
        return { projectIds: state.projects.map((item) => item.projectId) };
      }
      if (input.action === "project-delete") {
        const projectIndex = state.projects.findIndex((item) => item.projectId === input.projectId);
        if (projectIndex >= 0) state.projects.splice(projectIndex, 1);
        const workspaceIndex = state.workspaces.findIndex((item) => item.projectId === input.projectId);
        if (workspaceIndex >= 0) state.workspaces.splice(workspaceIndex, 1);
        return { projectId: input.projectId, revision: 3 };
      }
      if (input.action === "conversation-archive") {
        return { conversationId: input.conversationId, archived: input.archived };
      }
      throw new Error(`unexpected command ${input.action}`);
    },
    navigate: async (target) => target,
    hostActions: {},
  });
  return { services, commands, state };
}

function hostCallError(message, body) {
  const error = new Error(message);
  error.code = "HOST_CALL_FAILED";
  error.response = body;
  return error;
}

test("workspaces.create 通过真实 project-create 建立项目工作区", async () => {
  const { services, commands } = fixture();
  await services.workspaces.refresh();
  const created = await services.workspaces.create({ path: "/tmp/vault", title: "Vault" });
  assert.deepEqual(
    commands.find((entry) => entry.action === "project-create"),
    { action: "project-create", title: "Vault", directoryPath: "/tmp/vault" },
  );
  assert.equal(created.workspaceId, "workspace-created-1");
  assert.equal(created.projectId, "project-created-1");
  assert.equal(created.path, "/tmp/vault");
  assert.equal(created.title, "Vault");
  assert.deepEqual(
    services.workspaces.list.getSnapshot().items.map((item) => item.workspaceId).sort(),
    ["workspace-1", "workspace-created-1"],
  );
});

test("workspaces.create 失败时原样透传宿主返回体与 cause", async () => {
  const body = { status: "failed", error: { code: "PERMISSION_DENIED", message: "project.manage denied" } };
  const thrown = hostCallError("host.project.create failed: project.manage denied", body);
  const { services } = fixture({
    handlers: {
      "project-create": () => {
        throw thrown;
      },
    },
  });
  await assert.rejects(() => services.workspaces.create({ path: "/tmp/vault" }), (error) => {
    assert.equal(error.name, "DshHostCommandError");
    assert.equal(error.code, "HOST_CALL_FAILED");
    assert.equal(error.operation, "workspaces.create");
    assert.equal(error.action, "project-create");
    assert.deepEqual(error.request, { action: "project-create", title: "", directoryPath: "/tmp/vault" });
    assert.match(error.message, /DSH workspaces\.create 调用 Amitia 宿主 action project-create 失败/);
    assert.match(error.message, /host\.project\.create failed: project\.manage denied/);
    assert.ok(error.message.includes(JSON.stringify(body)));
    assert.deepEqual(error.response, body);
    assert.equal(error.cause, thrown);
    return true;
  });
});

test("workspaces.rename 通过真实 project-update 返回工作区视图", async () => {
  const { services, commands } = fixture();
  await services.workspaces.refresh();
  const view = await services.workspaces.rename("workspace-1", "Renamed");
  assert.deepEqual(
    commands.find((entry) => entry.action === "project-update"),
    { action: "project-update", projectId: "project-1", title: "Renamed" },
  );
  assert.equal(view.workspaceId, "workspace-1");
  assert.equal(view.projectId, "project-1");
  assert.equal(view.title, "Renamed");
  assert.equal(services.workspaces.list.getSnapshot().items[0].title, "Renamed");
});

test("workspaces.rename 拒绝空标题且不调用宿主", async () => {
  const { services, commands } = fixture();
  await services.workspaces.refresh();
  await assert.rejects(() => services.workspaces.rename("workspace-1", "   "), (error) => {
    assert.match(error.message, /DSH workspaces\.rename 需要非空 title/);
    assert.match(error.message, /"workspaceId":"workspace-1"/);
    return true;
  });
  assert.equal(commands.some((entry) => entry.action === "project-update"), false);
});

test("workspaces.rename 在宿主返回失败体时抛错并带上原始返回体", async () => {
  const body = { ok: false, error: { code: "REVISION_CONFLICT", message: "revision conflict" } };
  const { services } = fixture({ handlers: { "project-update": () => body } });
  await services.workspaces.refresh();
  await assert.rejects(() => services.workspaces.rename("workspace-1", "Renamed"), (error) => {
    assert.equal(error.name, "DshHostCommandError");
    assert.equal(error.code, "REVISION_CONFLICT");
    assert.equal(error.operation, "workspaces.rename");
    assert.match(error.message, /DSH workspaces\.rename 的 Amitia 宿主 action project-update 返回失败/);
    assert.ok(error.message.includes(JSON.stringify(body)));
    assert.deepEqual(error.response, body);
    return true;
  });
});

test("workspaces.rename 在目录未确认工作区时报错并带上宿主返回体", async () => {
  const updateBody = { projectId: "project-1", revision: 2 };
  const { services } = fixture({
    handlers: {
      "project-update": (input, state) => {
        state.workspaces.length = 0;
        return updateBody;
      },
    },
  });
  await services.workspaces.refresh();
  await assert.rejects(() => services.workspaces.rename("workspace-1", "Renamed"), (error) => {
    assert.match(error.message, /DSH workspaces\.rename 后无法在 Amitia 工作区目录确认工作区 workspace-1/);
    assert.ok(error.message.includes(JSON.stringify(updateBody)));
    return true;
  });
});

test("workspaces.delete 通过真实 project-delete 移除项目并返回 true", async () => {
  const { services, commands } = fixture();
  await services.workspaces.refresh();
  assert.deepEqual(
    services.workspaces.list.getSnapshot().items.map((item) => item.workspaceId),
    ["workspace-1"],
  );
  assert.equal(await services.workspaces.delete("workspace-1"), true);
  assert.deepEqual(
    commands.find((entry) => entry.action === "project-delete"),
    { action: "project-delete", projectId: "project-1" },
  );
  assert.deepEqual(services.workspaces.list.getSnapshot().items, []);
});

test("workspaces.delete 在宿主明确未删除时报错并带上原始返回体", async () => {
  const body = { projectId: "project-1", deleted: false };
  const { services } = fixture({ handlers: { "project-delete": () => body } });
  await services.workspaces.refresh();
  await assert.rejects(() => services.workspaces.delete("workspace-1"), (error) => {
    assert.equal(error.name, "DshHostCommandError");
    assert.match(error.message, /DSH workspaces\.delete 的 Amitia 宿主 action project-delete 返回失败/);
    assert.ok(error.message.includes(JSON.stringify(body)));
    assert.deepEqual(error.response, body);
    return true;
  });
});

test("workspaces.delete 无项目映射时拒绝且不调用宿主", async () => {
  const { services, commands } = fixture();
  await services.workspaces.refresh();
  await assert.rejects(() => services.workspaces.delete("workspace-missing"), /DSH WorkspaceId workspace-missing has no verified Amitia ProjectId mapping/);
  assert.equal(commands.some((entry) => entry.action === "project-delete"), false);
});

test("workspaces.archiveSession 与 unarchiveSession 通过真实 conversation-archive 落库", async () => {
  const { services, commands } = fixture();
  await services.workspaces.archiveSession("session-1", { stopActivity: true });
  await services.workspaces.unarchiveSession("session-1");
  assert.deepEqual(
    commands.filter((entry) => entry.action === "conversation-archive"),
    [
      { action: "conversation-archive", conversationId: "session-1", archived: true, stopActivity: true },
      { action: "conversation-archive", conversationId: "session-1", archived: false },
    ],
  );
});

test("workspaces.archiveSession 在宿主未归档时报错并带上原始返回体", async () => {
  const body = { conversationId: "session-1", archived: false, reason: "archive rejected" };
  const { services } = fixture({ handlers: { "conversation-archive": () => body } });
  await assert.rejects(() => services.workspaces.archiveSession("session-1"), (error) => {
    assert.equal(error.name, "DshHostCommandError");
    assert.equal(error.operation, "workspaces.archiveSession");
    assert.match(error.message, /DSH workspaces\.archiveSession 的 Amitia 宿主 action conversation-archive 返回失败/);
    assert.ok(error.message.includes(JSON.stringify(body)));
    assert.deepEqual(error.response, body);
    return true;
  });
});

test("workspaces.archiveSession 在宿主抛错时透传原始报错与 cause", async () => {
  const details = { status: "rejected", error: { code: "SCOPE_DENIED", message: "conversation is out of scope" } };
  const thrown = hostCallError("host.conversation.archive failed: conversation is out of scope", details);
  const { services } = fixture({
    handlers: {
      "conversation-archive": () => {
        throw thrown;
      },
    },
  });
  await assert.rejects(() => services.workspaces.archiveSession("session-1"), (error) => {
    assert.equal(error.name, "DshHostCommandError");
    assert.equal(error.code, "HOST_CALL_FAILED");
    assert.equal(error.action, "conversation-archive");
    assert.match(error.message, /host\.conversation\.archive failed: conversation is out of scope/);
    assert.ok(error.message.includes(JSON.stringify(details)));
    assert.deepEqual(error.response, details);
    assert.equal(error.cause, thrown);
    return true;
  });
});

test("workspaces.pinSession 与 unpinSession 通过 conversation-pin 落库", async () => {
  const { services, commands } = fixture({
    handlers: {
      "conversation-pin": (input) => ({ conversationId: input.conversationId, pinned: input.pinned }),
    },
  });
  await services.workspaces.pinSession("session-1", { workspaceId: "workspace-1" });
  await services.workspaces.unpinSession("session-1");
  assert.deepEqual(
    commands.filter((entry) => entry.action === "conversation-pin").map((entry) => ({ id: entry.conversationId, pinned: entry.pinned })),
    [{ id: "session-1", pinned: true }, { id: "session-1", pinned: false }],
  );
  assert.equal(commands.some((entry) => entry.action === "project-update"), false);
});

test("workspaces.insertSessionBefore 通过 conversation-reorder 落库并刷新工作区视图", async () => {
  const { services, commands } = fixture({
    conversations: [
      { sessionId: "session-1", title: "One", displayTitle: "One", updatedAt: 1_760_000_000_000, running: false, blank: false, projectId: "project-1", workspaceId: "workspace-1", cwd: "/tmp/one", formatStatus: "current" },
      { sessionId: "session-2", title: "Two", displayTitle: "Two", updatedAt: 1_760_000_000_100, running: false, blank: false, projectId: "project-1", workspaceId: "workspace-1", cwd: "/tmp/one", formatStatus: "current", order: 2048 },
    ],
    workspaces: [{ workspaceId: "workspace-1", projectId: "project-1", path: "/tmp/one", title: "One", sessionIds: ["session-1", "session-2"], createdAt: NOW, updatedAt: NOW }],
  });
  await services.workspaces.refresh();
  const view = await services.workspaces.insertSessionBefore("workspace-1", "session-2", "session-1");
  const reorder = commands.find((entry) => entry.action === "conversation-reorder");
  assert.equal(reorder.conversationId, "session-2");
  assert.equal(reorder.beforeConversationId, "session-1");
  assert.equal(reorder.projectId, "project-1");
  assert.match(reorder.requestId, /^workspaces\.insertSessionBefore:session-2:session-1:/);
  assert.deepEqual(view.sessionIds, ["session-2", "session-1"]);
});

test("workspaces.insertSessionBefore 与 insertBefore 拒绝未知工作区映射", async () => {
  const { services, commands } = fixture();
  await assert.rejects(() => services.workspaces.insertSessionBefore("workspace-missing", "session-1"), /has no verified Amitia ProjectId mapping/);
  await assert.rejects(() => services.workspaces.insertBefore("workspace-missing"), /has no verified Amitia ProjectId mapping/);
  assert.equal(commands.some((entry) => entry.action === "conversation-reorder" || entry.action === "project-reorder"), false);
});
