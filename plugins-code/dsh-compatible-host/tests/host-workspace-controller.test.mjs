import assert from "node:assert/strict";
import test from "node:test";
import { createAmitiaDshHostServices } from "../src/runtime/host-services.mjs";

function createWorkspaceHost(initial = [], conversations = [{ sessionId: "session-1", projectId: "project-1" }]) {
  const projects = initial.map((project) => ({ ...project }));
  const sessions = conversations.map((entry) => ({ ...entry }));
  const targeted = [];
  const calls = [];
  return {
    calls,
    projects,
    sessions,
    targeted,
    async callTargeted(method, input, scopeSnapshotId) {
      targeted.push({ method, input, scopeSnapshotId });
      if (method === "host.conversation.archive") {
        const target = sessions.find((entry) => entry.sessionId === input.conversationId);
        if (!target) throw new Error(`session ${input.conversationId} not found`);
        target.archivedAt = input.archived ? "2026-11-18T02:00:00Z" : "";
        return { conversationId: input.conversationId, archived: input.archived === true };
      }
      if (method === "host.conversation.reorder") {
        const members = sessions.filter((entry) => (entry.projectId || "") === (input.projectId || ""));
        const moving = members.find((entry) => entry.sessionId === input.conversationId);
        if (!moving) throw new Error(`session ${input.conversationId} not in workspace ${input.projectId}`);
        const rest = members.filter((entry) => entry !== moving);
        let index = rest.length;
        if (input.beforeConversationId) {
          index = rest.findIndex((entry) => entry.sessionId === input.beforeConversationId);
          if (index < 0) throw new Error(`anchor ${input.beforeConversationId} not in workspace ${input.projectId}`);
        }
        rest.splice(index, 0, moving);
        rest.forEach((entry, position) => { entry.order = (position + 1) * 1024; });
        return { conversationIds: rest.map((entry) => entry.sessionId), projectId: input.projectId };
      }
      if (method === "host.conversation.pin") {
        const target = sessions.find((entry) => entry.sessionId === input.conversationId);
        if (!target) throw new Error(`session ${input.conversationId} not found`);
        target.pinnedAt = input.pinned ? "2026-11-18T03:00:00Z" : "";
        return { conversationId: input.conversationId, pinned: input.pinned === true };
      }
      return {};
    },
    async call(method, input = {}) {
      calls.push({ method, input });
      if (method === "host.project.list") return { projects: projects.map((project) => ({ ...project })) };
      if (method === "host.project.create") {
        const id = "project-created";
        projects.push({
          id,
          title: input.title,
          primaryWorkspaceId: "ws-created",
          workspaces: [{ id: "ws-created", rootUri: `file://${input.directoryPath}` }],
          createdAt: "2026-11-18T00:00:00Z",
          updatedAt: "2026-11-18T00:00:00Z",
        });
        return { projectId: id };
      }
      if (method === "host.project.update") {
        const target = projects.find((project) => project.id === input.projectId);
        if (!target) throw new Error(`project ${input.projectId} not found`);
        if (input.title !== undefined) target.title = input.title;
        target.updatedAt = "2026-11-18T01:00:00Z";
        return { projectId: input.projectId };
      }
      if (method === "host.project.reorder") {
        const moving = projects.find((project) => project.id === input.projectId);
        if (!moving) throw new Error(`project ${input.projectId} not found`);
        const rest = projects.filter((project) => project !== moving);
        let index = rest.length;
        if (input.beforeProjectId) {
          index = rest.findIndex((project) => project.id === input.beforeProjectId);
          if (index < 0) throw new Error(`anchor ${input.beforeProjectId} not found`);
        }
        rest.splice(index, 0, moving);
        projects.splice(0, projects.length, ...rest);
        return { projectIds: projects.map((project) => project.id) };
      }
      if (method === "host.project.delete") {
        const index = projects.findIndex((project) => project.id === input.projectId);
        if (index >= 0) projects.splice(index, 1);
        return { projectId: input.projectId, deleted: true };
      }
      if (method === "host.conversation.list") {
        const items = input.archived === true
          ? sessions.filter((entry) => entry.archivedAt)
          : sessions.filter((entry) => !entry.archivedAt);
        return { items: items.map((entry) => ({ ...entry })), hasMore: false };
      }
      if (method === "host.state.get") {
        return { found: true, value: { scopeSnapshotId: "snapshot-session-1" } };
      }
      if (method === "host.directory.list") {
        return { path: "/Users/untrammelled", entries: [{ name: "untrammelled", path: "/Users/untrammelled" }] };
      }
      return {};
    },
  };
}

const BASE_PROJECT = {
  id: "project-1",
  title: "U-AI",
  primaryWorkspaceId: "ws-1",
  workspaces: [
    { id: "ws-1", rootUri: "file:///Users/untrammelled/Desktop/Project/U-AI" },
    { id: "ws-2", rootUri: "file:///Users/untrammelled/Desktop/Project/other" },
  ],
  createdAt: "2026-11-01T00:00:00Z",
  updatedAt: "2026-11-02T00:00:00Z",
};

test("workspaceController.list 把 Amitia project 映射成 WorkspaceView", async () => {
  const host = createWorkspaceHost([BASE_PROJECT]);
  const services = createAmitiaDshHostServices(host);
  const items = await services.workspaceController.list();
  assert.equal(items.length, 1);
  assert.deepEqual(items[0], {
    workspaceId: "project-1",
    path: "/Users/untrammelled/Desktop/Project/U-AI",
    title: "U-AI",
    sessionIds: ["session-1"],
    createdAt: "2026-11-01T00:00:00Z",
    updatedAt: "2026-11-02T00:00:00Z",
  });
});

test("workspaceController.create 复用已注册路径，新路径走 project-create", async () => {
  const host = createWorkspaceHost([BASE_PROJECT]);
  const services = createAmitiaDshHostServices(host);
  const existing = await services.workspaceController.create({ path: "/Users/untrammelled/Desktop/Project/U-AI" });
  assert.equal(existing.created, false);
  assert.equal(existing.workspace.workspaceId, "project-1");
  assert.equal(host.calls.filter((call) => call.method === "host.project.create").length, 0);
  const created = await services.workspaceController.create({ path: "/Users/untrammelled/Desktop/Project/new-repo" });
  assert.equal(created.created, true);
  assert.equal(created.workspace.title, "new-repo");
  assert.equal(created.workspace.path, "/Users/untrammelled/Desktop/Project/new-repo");
  const createCall = host.calls.find((call) => call.method === "host.project.create");
  assert.equal(createCall.input.directoryPath, "/Users/untrammelled/Desktop/Project/new-repo");
  await assert.rejects(services.workspaceController.create({}), /需要绝对路径/);
});

test("workspaceController.rename 拒绝空标题与重名并写回 project-update", async () => {
  const host = createWorkspaceHost([BASE_PROJECT, { ...BASE_PROJECT, id: "project-2", title: "between", primaryWorkspaceId: "ws-3", workspaces: [{ id: "ws-3", rootUri: "file:///Volumes/UTD/Project/AI-Community" }] }]);
  const services = createAmitiaDshHostServices(host);
  await assert.rejects(services.workspaceController.rename({ workspaceId: "project-1", title: "  " }), (error) => {
    assert.equal(error.code, "gateway/bad-request");
    return true;
  });
  await assert.rejects(services.workspaceController.rename({ workspaceId: "project-1", title: "between" }), (error) => {
    assert.equal(error.code, "workspace/name-conflict");
    return true;
  });
  const renamed = await services.workspaceController.rename({ workspaceId: "project-1", title: "Amitia" });
  assert.equal(renamed.workspace.title, "Amitia");
  assert.equal(host.calls.filter((call) => call.method === "host.project.update").length, 1);
  await assert.rejects(services.workspaceController.rename({ workspaceId: "project-missing", title: "x" }), (error) => {
    assert.equal(error.code, "session/not-found");
    return true;
  });
});

test("workspaceController.delete 删除 project 注册并返回 deleted", async () => {
  const host = createWorkspaceHost([BASE_PROJECT]);
  const services = createAmitiaDshHostServices(host);
  assert.deepEqual(await services.workspaceController.delete({ workspaceId: "project-1" }), { deleted: true });
  assert.equal(host.projects.length, 0);
  await assert.rejects(services.workspaceController.delete({}), /需要 workspaceId/);
});

test("workspaceController.initializeDefault 复用现有工作区，缺失时才按主目录创建", async () => {
  const existingHost = createWorkspaceHost([BASE_PROJECT]);
  const existingServices = createAmitiaDshHostServices(existingHost);
  const reused = await existingServices.workspaceController.initializeDefault();
  assert.equal(reused.workspaceId, "project-1");
  assert.equal(existingHost.calls.filter((call) => call.method === "host.project.create").length, 0);
  const emptyHost = createWorkspaceHost([]);
  const emptyServices = createAmitiaDshHostServices(emptyHost);
  const created = await emptyServices.workspaceController.initializeDefault();
  assert.equal(created.path, "/Users/untrammelled");
  assert.equal(created.title, "untrammelled");
});

test("workspaceController 的归档与置顶按会话作用域快照落到 conversation 写接口", async () => {
  const host = createWorkspaceHost([BASE_PROJECT]);
  const services = createAmitiaDshHostServices(host);
  const archived = await services.workspaceController.archiveSession({ sessionId: "session-1", stopActivity: true });
  assert.deepEqual(archived.archivedSessionIds, ["session-1"]);
  assert.deepEqual(host.targeted[0], {
    method: "host.conversation.archive",
    scopeSnapshotId: "snapshot-session-1",
    input: { conversationId: "session-1", archived: true, stopActivity: true, requestId: "dsh-workspace-archive:session-1" },
  });
  await assert.rejects(services.workspaceController.pinSession({ sessionId: "session-1" }), (error) => {
    assert.equal(error.code, "gateway/bad-request");
    assert.match(error.message, /mutually exclusive/);
    return true;
  });
  const unarchived = await services.workspaceController.unarchiveSession({ sessionId: "session-1" });
  assert.deepEqual(unarchived.archivedSessionIds, []);
  const pinned = await services.workspaceController.pinSession({ sessionId: "session-1" });
  assert.deepEqual(pinned.pinnedSessionIds, ["session-1"]);
  const unpinned = await services.workspaceController.unpinSession({ sessionId: "session-1" });
  assert.deepEqual(unpinned.pinnedSessionIds, []);
  assert.deepEqual(host.targeted.map((entry) => entry.input.requestId), [
    "dsh-workspace-archive:session-1",
    "dsh-workspace-unarchive:session-1",
    "dsh-workspace-pin:session-1",
    "dsh-workspace-unpin:session-1",
  ]);
  await assert.rejects(services.workspaceController.archiveSession({}), /需要 sessionId/);
});

test("workspaceController 缺少 callTargeted 时明确报错而不是静默降级", async () => {
  const host = createWorkspaceHost([BASE_PROJECT]);
  delete host.callTargeted;
  const services = createAmitiaDshHostServices(host);
  await assert.rejects(services.workspaceController.archiveSession({ sessionId: "session-1" }), /host\.callTargeted 不可用/);
});

test("workspaceController.insertBefore 调 host.project.reorder 并返回完整工作区顺序", async () => {
  const second = { ...BASE_PROJECT, id: "project-2", title: "Docs", primaryWorkspaceId: "ws-2", workspaces: [{ id: "ws-2", rootUri: "file:///tmp/docs" }] };
  const host = createWorkspaceHost([BASE_PROJECT, second]);
  const services = createAmitiaDshHostServices(host);
  const result = await services.workspaceController.insertBefore({ workspaceId: "project-2", beforeWorkspaceId: "project-1" });
  assert.deepEqual(result.workspaceIds, ["project-2", "project-1"]);
  const reorder = host.calls.find((call) => call.method === "host.project.reorder");
  assert.equal(reorder.input.projectId, "project-2");
  assert.equal(reorder.input.beforeProjectId, "project-1");
  assert.match(reorder.input.requestId, /^dsh-workspace-order:project-2:project-1:/);
  assert.deepEqual(host.projects.map((project) => project.id), ["project-2", "project-1"]);
});

test("workspaceController.insertBefore 拒绝未知工作区与未知锚点", async () => {
  const host = createWorkspaceHost([BASE_PROJECT]);
  const services = createAmitiaDshHostServices(host);
  await assert.rejects(services.workspaceController.insertBefore({ workspaceId: "missing" }), /not found/);
  await assert.rejects(services.workspaceController.insertBefore({}), /需要 workspaceId/);
  await assert.rejects(services.workspaceController.insertBefore({ workspaceId: "project-1", beforeWorkspaceId: "missing" }), /not found/);
  assert.equal(host.calls.filter((call) => call.method === "host.project.reorder").length, 0);
});

test("workspaceController.insertSessionBefore 按会话作用域快照调 host.conversation.reorder", async () => {
  const host = createWorkspaceHost([BASE_PROJECT], [
    { sessionId: "session-1", projectId: "project-1", order: 1024 },
    { sessionId: "session-2", projectId: "project-1", order: 2048 },
  ]);
  const services = createAmitiaDshHostServices(host);
  const result = await services.workspaceController.insertSessionBefore({ workspaceId: "project-1", sessionId: "session-2", beforeSessionId: "session-1" });
  assert.deepEqual(result.workspace.sessionIds, ["session-2", "session-1"]);
  const reorder = host.targeted.find((entry) => entry.method === "host.conversation.reorder");
  assert.equal(reorder.scopeSnapshotId, "snapshot-session-1");
  assert.equal(reorder.input.conversationId, "session-2");
  assert.equal(reorder.input.beforeConversationId, "session-1");
  assert.equal(reorder.input.projectId, "project-1");
  assert.match(reorder.input.requestId, /^dsh-session-order:project-1:session-2:session-1:/);
});

test("workspaceController.insertSessionBefore 拒绝跨工作区会话", async () => {
  const host = createWorkspaceHost([BASE_PROJECT], [{ sessionId: "session-1", projectId: "project-1" }]);
  const services = createAmitiaDshHostServices(host);
  await assert.rejects(services.workspaceController.insertSessionBefore({ workspaceId: "project-1", sessionId: "session-9" }), /does not belong to workspace/);
  await assert.rejects(services.workspaceController.insertSessionBefore({ workspaceId: "project-1", sessionId: "session-1", beforeSessionId: "session-9" }), /does not belong to workspace/);
  await assert.rejects(services.workspaceController.insertSessionBefore({}), /需要 workspaceId 与 sessionId/);
  assert.equal(host.targeted.filter((entry) => entry.method === "host.conversation.reorder").length, 0);
});
