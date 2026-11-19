import assert from "node:assert/strict";
import test from "node:test";
import { createDshClientRuntimeServices } from "../src/client-abi/runtime-services.mjs";

function createUiWorkspaceFixture(options = {}) {
  const commands = [];
  const navigations = [];
  const command = async (input) => {
    commands.push({ ...input });
    if (input.action === "conversation-list") {
      const items = input.archived === true ? [] : (options.conversations ?? []);
      return {
        items,
        workspaces: options.workspaces ?? [],
        archivedSessionIds: [],
        pinnedSessionIds: [],
        hasMore: false,
      };
    }
    if (input.action === "project-workspaces") return { items: [] };
    if (input.action === "conversation-create") return { conversationId: options.createdId ?? "new-session" };
    if (input.action === "conversation-message-send") return { messageId: "message-1" };
    if (input.action === "agent-preset-write") return { key: input.targetId, preset: input.preset, scope: input.scope };
    if (input.action === "conversation-fork") {
      if (options.forkError) throw options.forkError;
      return { conversationId: "forked-session", forkedFrom: input.conversationId, messageCount: 3 };
    }
    throw new Error(`unexpected command ${input.action}`);
  };
  return {
    services: createDshClientRuntimeServices({ command, navigate: (target) => navigations.push(target), hostActions: {} }),
    commands,
    navigations,
  };
}

test("uiWorkspace.forkSession 走 conversation-fork 并回调新会话 id", async () => {
  const { services, commands } = createUiWorkspaceFixture();
  const created = [];
  const forkedId = await services.uiWorkspace.forkSession("session-1", (childId) => created.push(childId));
  assert.equal(forkedId, "forked-session");
  assert.deepEqual(created, ["forked-session"]);
  const forkCall = commands.find((entry) => entry.action === "conversation-fork");
  assert.deepEqual(forkCall, { action: "conversation-fork", conversationId: "session-1" });
});

test("uiWorkspace.forkSession 在没有回调时也能返回派生会话", async () => {
  const { services } = createUiWorkspaceFixture();
  assert.equal(await services.uiWorkspace.forkSession("session-2"), "forked-session");
});

test("uiWorkspace.forkSession 原样抛出宿主失败", async () => {
  const original = new Error("host.conversation.fork failed: record not found");
  const { services } = createUiWorkspaceFixture({ forkError: original });
  await assert.rejects(services.uiWorkspace.forkSession("session-3"), (error) => {
    assert.equal(error.message, original.message);
    return true;
  });
});

test("uiWorkspace.startSession 落 agentPreset 并发送首条 prompt", async (t) => {
  const storage = installLocalStorage();
  t.after(() => storage.restore());
  const { services, commands, navigations } = createUiWorkspaceFixture();
  const id = await services.uiWorkspace.startSession(undefined, { prompt: "先读一下 README", agentPreset: "reviewer" });
  assert.equal(id, "new-session");
  const created = commands.find((entry) => entry.action === "conversation-create");
  assert.equal(created.agentPreset, undefined);
  assert.equal(created.prompt, undefined);
  const presetWrite = commands.find((entry) => entry.action === "agent-preset-write");
  assert.deepEqual(presetWrite, { action: "agent-preset-write", targetId: "new-session", preset: "reviewer", scope: "session" });
  const send = commands.find((entry) => entry.action === "conversation-message-send");
  assert.equal(send.conversationId, "new-session");
  assert.equal(send.content, "先读一下 README");
  assert.deepEqual(navigations, ["/chat?conversationId=new-session"]);
});

function installLocalStorage(initial = {}) {
  const store = new Map(Object.entries(initial));
  const stub = {
    getItem: (key) => (store.has(key) ? store.get(key) : null),
    setItem: (key, value) => { store.set(key, String(value)); },
    removeItem: (key) => { store.delete(key); },
  };
  const previous = Object.getOwnPropertyDescriptor(globalThis, "localStorage");
  Object.defineProperty(globalThis, "localStorage", { configurable: true, value: stub });
  return {
    store,
    restore: () => {
      if (previous) Object.defineProperty(globalThis, "localStorage", previous);
      else delete globalThis.localStorage;
    },
  };
}

test("uiWorkspace.startSession 默认保留草稿，replace 清空，append 合并，keep 不动", async (t) => {
  const cases = [
    { options: { prompt: "新内容" }, expectedContent: "新内容", expectedDraft: "旧草稿" },
    { options: { prompt: "新内容", whenDraftExists: "replace" }, expectedContent: "新内容", expectedDraft: "" },
    { options: { prompt: "新内容", whenDraftExists: "append" }, expectedContent: "旧草稿\n新内容", expectedDraft: "" },
    { options: { prompt: "新内容", whenDraftExists: "keep" }, expectedContent: "新内容", expectedDraft: "旧草稿" },
    { options: { prompt: "新内容", clearPreviousDraft: true }, expectedContent: "新内容", expectedDraft: "" },
  ];
  for (const item of cases) {
    const storage = installLocalStorage({ "webchat_draft:new:recent": "旧草稿" });
    t.after(() => storage.restore());
    const { services, commands } = createUiWorkspaceFixture();
    await services.uiWorkspace.startSession(undefined, item.options);
    const send = commands.find((entry) => entry.action === "conversation-message-send");
    assert.equal(send.content, item.expectedContent);
    assert.equal(storage.store.get("webchat_draft:new:recent") ?? "", item.expectedDraft);
    storage.restore();
  }
});

test("uiWorkspace.startSession 拒绝非法的草稿模式", async (t) => {
  const storage = installLocalStorage();
  t.after(() => storage.restore());
  const { services, commands } = createUiWorkspaceFixture();
  await assert.rejects(services.uiWorkspace.startSession(undefined, { whenDraftExists: "merge" }), /只支持 replace\/append\/keep/);
  await assert.rejects(services.uiWorkspace.startSession(undefined, { clearPreviousDraft: "yes" }), /必须是布尔值/);
  assert.equal(commands.some((entry) => entry.action === "conversation-create"), false);
});

const workspaceFixtureOptions = () => ({
  conversations: [
    { sessionId: "session-old", title: "Old", updatedAt: 100, projectId: "project-1", workspaceId: "workspace-1" },
    { sessionId: "session-new", title: "New", updatedAt: 500, projectId: "project-1", workspaceId: "workspace-1" },
    { sessionId: "session-other", title: "Other", updatedAt: 900, projectId: "project-2", workspaceId: "workspace-2" },
  ],
  workspaces: [
    { workspaceId: "workspace-1", projectId: "project-1", path: "/tmp/one", title: "One", sessionIds: ["session-old", "session-new"] },
    { workspaceId: "workspace-2", projectId: "project-2", path: "/tmp/two", title: "Two", sessionIds: ["session-other"] },
  ],
});

test("uiWorkspace.openWorkspace 用工作区最近会话投影 beforeOpen", async () => {
  const { services, navigations } = createUiWorkspaceFixture(workspaceFixtureOptions());
  await services.workspaces.refresh();
  const opened = [];
  await services.uiWorkspace.openWorkspace("workspace-1", (sessionId) => opened.push(sessionId));
  assert.deepEqual(opened, ["session-new"]);
  assert.deepEqual(navigations, ["/workspaces?projectId=project-1"]);
});

test("uiWorkspace.openWorkspace 无会话时原样说明无法投影", async () => {
  const { services } = createUiWorkspaceFixture({
    conversations: [],
    workspaces: [{ workspaceId: "workspace-empty", projectId: "project-empty", path: "/tmp/empty", title: "Empty", sessionIds: [] }],
  });
  await services.workspaces.refresh();
  await assert.rejects(
    services.uiWorkspace.openWorkspace("workspace-empty", () => {}),
    /beforeOpen 需要可投影的 SessionId：工作区 "workspace-empty" 没有会话/,
  );
});

test("uiWorkspace.openWorkspace 未映射工作区时保留原始 ProjectId 报错", async () => {
  const { services } = createUiWorkspaceFixture();
  await assert.rejects(services.uiWorkspace.openWorkspace("workspace-missing", () => {}), /has no verified Amitia ProjectId mapping/);
});

test("uiWorkspace.connectWorkspace 建会话并落 agentPreset", async () => {
  const { services, commands, navigations } = createUiWorkspaceFixture(workspaceFixtureOptions());
  await services.workspaces.refresh();
  const id = await services.uiWorkspace.connectWorkspace("workspace-2", { agentPreset: "reviewer" });
  assert.equal(id, "new-session");
  const created = commands.find((entry) => entry.action === "conversation-create");
  assert.equal(created.projectId, "project-2");
  assert.deepEqual(
    commands.find((entry) => entry.action === "agent-preset-write"),
    { action: "agent-preset-write", targetId: "new-session", preset: "reviewer", scope: "session" },
  );
  assert.deepEqual(navigations, ["/chat?conversationId=new-session"]);
});
