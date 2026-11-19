import assert from "node:assert/strict";
import test from "node:test";
import { createDshClientRuntimeServices } from "../src/client-abi/runtime-services.mjs";

function conversationMessages(total, conversationId = "session-1") {
  return Array.from({ length: total }, (_, index) => ({
    messageId: `message-${index + 1}`,
    conversationId,
    role: index % 2 === 0 ? "user" : "assistant",
    timestamp: new Date(Date.UTC(2026, 0, 1, 0, 0, index)).toISOString(),
    contentSummary: `第 ${index + 1} 条消息`,
  }));
}

function stateCommand(state, calls = []) {
  return async (input) => {
    calls.push({ ...input });
    if (input.action === "conversation-list") {
      const items = input.archived === true ? (state.archived ?? []) : state.conversations;
      return {
        items,
        workspaces: input.archived === true ? [] : (state.workspaces ?? []),
        archivedSessionIds: [],
        pinnedSessionIds: [],
        hasMore: false,
      };
    }
    if (input.action === "project-workspaces") return { items: state.projects ?? [] };
    if (input.action === "conversation-read") {
      const messages = state.messages ?? [];
      const limit = Number(input.limit) || 100;
      const start = Math.max(0, messages.length - limit);
      return { messages: messages.slice(start), hasMore: start > 0 };
    }
    if (input.action === "conversation-create") return { conversationId: "conversation-created" };
    if (input.action === "conversation-archive") {
      return { conversationId: input.conversationId, archived: input.archived };
    }
    if (input.action === "conversation-message-send") {
      if (state.sendError) throw state.sendError;
      return { accepted: true, messageId: "message-sent" };
    }
    if (input.action === "attachment-read") {
      if (String(input.path).startsWith("/etc/")) throw new Error("attachment-read 只允许读取附件落盘目录");
      return { path: input.path, size: 4, mediaType: "image/png", bytesBase64: "AAAA" };
    }
    if (input.action === "conversation-update") {
      return { conversationId: input.conversationId, title: input.title };
    }
    if (input.action === "conversation-interrupt") {
      if (state.interruptError) throw state.interruptError;
      return { interrupted: true, conversationId: input.conversationId };
    }
    throw new Error(`unexpected command ${input.action}`);
  };
}

function flush() {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

function sessionFixture(options = {}) {
  const calls = [];
  const state = {
    conversations: options.conversations ?? [{
      sessionId: "session-1",
      title: "巡检会话",
      displayTitle: "巡检会话",
      updatedAt: 1_760_000_000_000,
      running: true,
      blank: false,
      projectId: "project-1",
      workspaceId: "workspace-1",
      pinnedAt: "2026-11-17T00:00:00Z",
    }],
    workspaces: options.workspaces ?? [{
      workspaceId: "workspace-1",
      projectId: "project-1",
      path: "/tmp/one",
      title: "One",
    }],
    projects: [],
    archived: [{ sessionId: "session-archived" }],
    messages: conversationMessages(options.total ?? 250),
    sendError: options.sendError,
  };
  const services = createDshClientRuntimeServices({
    command: stateCommand(state, calls),
    navigate: () => {},
    hostActions: {},
  });
  return { services, calls, state };
}

test("sessions.binding 读取宿主会话投影、待提交队列与翻页状态", async () => {
  const { services } = sessionFixture();
  await services.sessions.refresh();
  const binding = services.sessions.binding("session-1");
  await flush();
  const snapshot = binding.session.getSnapshot();
  assert.equal(snapshot.sessionId, "session-1");
  assert.equal(snapshot.running, true);
  assert.equal(snapshot.blank, false);
  assert.equal(snapshot.removed, false);
  assert.equal(snapshot.openState, "open");
  assert.equal(snapshot.hasMore, true);
  assert.equal(snapshot.historyWasPartial, true);
  assert.equal(snapshot.historyLoadStatus, "idle");
  assert.equal(binding.session.projections.faceOf("title").getSnapshot(), "巡检会话");
  assert.deepEqual(
    binding.session.projections.faceOf("sessionListMetadata").getSnapshot(),
    { blank: false, lastPromptAt: 1_760_000_000_000 },
  );
  assert.deepEqual(binding.session.projections.keys(), ["title", "sessionListMetadata"]);
  assert.equal(binding.session.projections.faceOf("missing"), binding.session.projections.faceOf("missing"));
  assert.equal(binding.session.projections.faceOf("missing").getSnapshot(), undefined);

  let notified = 0;
  const unsubscribe = binding.session.subscribe(() => { notified += 1; });
  const submission = binding.session.beginSubmission({ mode: "steer", text: "继续" });
  const queued = binding.session.getSnapshot().pendingSubmissions;
  assert.equal(queued.length, 1);
  assert.equal(queued[0].requestId, submission.requestId);
  assert.equal(queued[0].placement, "steer");
  assert.equal(queued[0].text, "继续");
  assert.ok(notified > 0);
  submission.abandon();
  assert.equal(binding.session.getSnapshot().pendingSubmissions.length, 0);

  const window = await binding.session.loadOlder({ deferConsumption: true });
  assert.ok(window.entries.length > 100);
  assert.equal(binding.session.getSnapshot().historyLoadStatus, "pendingConsumption");
  assert.equal(binding.session.getSnapshot().loadingOlder, false);
  assert.equal(binding.session.consumeOlder(), true);
  assert.equal(binding.session.consumeOlder(), false);
  assert.equal(binding.session.getSnapshot().historyLoadStatus, "idle");
  unsubscribe();

  const attachment = await binding.session.readAttachment({ type: "file", attachment: { attachmentId: "artifact-1", name: "a.png", bytes: 4, mediaType: "image/png", uri: "/tmp/amitia-dsh-attachments/abc-a.png" } });
  assert.deepEqual(attachment, { attachmentId: "artifact-1", name: "a.png", mediaType: "image/png", size: 4, bytesBase64: "AAAA" });
  await assert.rejects(() => binding.session.readAttachment({ attachment: { attachmentId: "x" } }), /需要带可读路径的附件引用/);

  const renamed = await binding.session.rename("新标题");
  assert.deepEqual(renamed, { conversationId: "session-1", title: "新标题" });
  await assert.rejects(() => binding.session.rename("  "), /需要非空标题/);
  const cancelled = await binding.session.cancel();
  assert.deepEqual(cancelled, { interrupted: true, conversationId: "session-1" });
  assert.throws(
    () => binding.session.projections.faceOf(""),
    /projections\.faceOf requires a projection key/,
  );
  await services.dispose();
});

test("sessions.binding 在宿主发送失败时保留原始报错并写入会话状态", async () => {
  const failure = new Error("宿主发送失败");
  failure.code = "HOST_SEND_FAILED";
  const { services } = sessionFixture({ sendError: failure, total: 5 });
  await services.sessions.refresh();
  const binding = services.sessions.binding("session-1");
  await assert.rejects(
    () => binding.session.prompt([{ type: "text", text: "在吗" }], "queue", undefined, "request-1"),
    (error) => {
      assert.equal(error, failure);
      assert.equal(error.code, "HOST_SEND_FAILED");
      return true;
    },
  );
  const snapshot = binding.session.getSnapshot();
  assert.equal(snapshot.promptAttempted, true);
  assert.equal(snapshot.promptError.op, "send");
  assert.equal(snapshot.promptError.error.message, "宿主发送失败");
  assert.equal(snapshot.lastAgentError, "宿主发送失败");
  await services.dispose();
});

test("sessions.retainInfo 暴露真实引用计数与持有方并驱动订阅通知", async () => {
  const { services } = sessionFixture({ total: 5 });
  await services.sessions.refresh();
  const info = services.sessions.retainInfo("session-1");
  assert.deepEqual(info.getSnapshot(), { referenceCount: 0, retainedBy: {} });
  let notified = 0;
  const unsubscribe = info.subscribe(() => { notified += 1; });
  const reference = services.sessions.retain({ sessionId: "session-1" }, { source: "workspace" });
  assert.deepEqual(info.getSnapshot(), { referenceCount: 1, retainedBy: { workspace: 1 } });
  assert.equal(reference.binding, services.sessions.binding("session-1"));
  assert.equal((await reference.ready).sessionId, "session-1");
  reference.release();
  reference.release();
  assert.deepEqual(info.getSnapshot(), { referenceCount: 0, retainedBy: {} });
  assert.ok(notified >= 2);
  unsubscribe();
  assert.throws(() => services.sessions.retainInfo(""), /sessions\.retainInfo requires a resolvable session id/);
  await services.dispose();
});

test("layout.panelInfo 跟随真实路由与右侧面板状态", async () => {
  const navigations = [];
  const rightPane = [];
  const previousAddEventListener = Object.getOwnPropertyDescriptor(globalThis, "addEventListener");
  const previousRemoveEventListener = Object.getOwnPropertyDescriptor(globalThis, "removeEventListener");
  const registered = new Map();
  let dispatchedListeners = 0;
  Object.defineProperty(globalThis, "addEventListener", {
    configurable: true,
    value: (type, listener) => {
      registered.set(type, listener);
    },
  });
  Object.defineProperty(globalThis, "removeEventListener", {
    configurable: true,
    value: (type, listener) => {
      if (registered.get(type) === listener) registered.delete(type);
    },
  });
  const popState = () => {
    const listener = registered.get("popstate");
    if (typeof listener !== "function") {
      dispatchedListeners += 1;
      return;
    }
    listener();
  };
  const services = createDshClientRuntimeServices({
    command: stateCommand({ conversations: [], workspaces: [], projects: [] }),
    navigate: (target) => {
      navigations.push(target);
      return target;
    },
    hostActions: {
      openRightPane: (state) => {
        rightPane.push(["open", state]);
        return "opened";
      },
      closeRightPane: () => {
        rightPane.push(["close"]);
        return "closed";
      },
    },
  });
  const previousLocation = Object.getOwnPropertyDescriptor(globalThis, "location");
  Object.defineProperty(globalThis, "location", { configurable: true, value: { pathname: "/extensions" } });
  let notified = 0;
  const unsubscribe = services.layout.panelInfo.subscribe(() => { notified += 1; });
  popState();
  assert.equal(services.layout.panelInfo.getSnapshot().activePanelId, "plugins");
  assert.ok(notified > 0);

  await services.layout.selectPanel("schedules");
  assert.deepEqual(navigations, ["/tasks"]);
  assert.equal(services.layout.panelInfo.getSnapshot().activePanelId, "schedules");
  assert.equal(await services.layout.openRightbar(true, false), "opened");
  assert.deepEqual(services.layout.panelInfo.getSnapshot(), {
    activePanelId: "schedules",
    rightbar: { track: true, fullscreen: false },
  });
  assert.deepEqual(rightPane, [["open", { track: true, fullscreen: false }]]);
  await services.layout.closeRightbar();
  assert.deepEqual(rightPane.at(-1), ["close"]);
  assert.equal(services.layout.panelInfo.getSnapshot().rightbar, undefined);
  unsubscribe();

  Object.defineProperty(globalThis, "location", { configurable: true, value: { pathname: "/tasks" } });
  popState();
  assert.equal(services.layout.panelInfo.getSnapshot().activePanelId, "schedules");
  assert.equal(dispatchedListeners, 0);
  if (previousLocation) Object.defineProperty(globalThis, "location", previousLocation);
  else delete globalThis.location;
  await services.dispose();
  assert.equal(registered.size, 0);
  if (previousAddEventListener) Object.defineProperty(globalThis, "addEventListener", previousAddEventListener);
  else delete globalThis.addEventListener;
  if (previousRemoveEventListener) Object.defineProperty(globalThis, "removeEventListener", previousRemoveEventListener);
  else delete globalThis.removeEventListener;
});

test("layout 在宿主缺少右侧面板能力时抛出带方法名与入参的缺口错误", async () => {
  const navigations = [];
  const services = createDshClientRuntimeServices({
    command: stateCommand({ conversations: [], workspaces: [], projects: [] }),
    navigate: (target) => { navigations.push(target); },
    hostActions: {},
  });
  assert.throws(
    () => services.layout.openRightbar(true, true),
    (error) => {
      assert.match(error.message, /layout\.openRightbar/);
      assert.match(error.message, /P12/);
      assert.match(error.message, /fullscreen/);
      return true;
    },
  );
  assert.throws(
    () => services.layout.toggleSidebar(),
    (error) => {
      assert.match(error.message, /layout\.toggleSidebar/);
      assert.match(error.message, /调用入参/);
      return true;
    },
  );
  await services.layout.selectPanel("custom-panel");
  assert.deepEqual(navigations, ["/main-panel/custom-panel"]);
  assert.equal(services.layout.panelInfo.getSnapshot().activePanelId, "custom-panel");
  await services.dispose();
});

test("uiWorkspace 目录桥接、连接工作区与归档走真实宿主调用", async () => {
  const calls = [];
  const state = {
    conversations: [{
      sessionId: "session-1",
      title: "One",
      displayTitle: "One",
      updatedAt: 1_760_000_000_000,
      running: false,
      blank: false,
      projectId: "project-1",
      workspaceId: "workspace-1",
    }],
    workspaces: [{ workspaceId: "workspace-1", projectId: "project-1", path: "/tmp/one", title: "One" }],
    projects: [],
    messages: [],
  };
  const command = async (input) => {
    calls.push({ ...input });
    if (input.action === "conversation-create") return { conversationId: "conversation-created" };
    if (input.action === "conversation-archive") return { conversationId: input.conversationId, archived: input.archived };
    if (input.action === "directory-list") return { path: input.path, home: "/tmp", crumbs: [], entries: [], truncated: false };
    if (input.action === "directory-create") return { path: `${input.path}/${input.name}` };
    return stateCommand(state, calls)(input);
  };
  const services = createDshClientRuntimeServices({
    command,
    navigate: (target) => {
      calls.push({ action: "navigate", target });
      return target;
    },
    hostActions: {
      selectDirectory: async () => "/tmp/picked",
      listDirectory: async (path) => ({ path: path ?? "/tmp", home: "/tmp", crumbs: [], entries: [], truncated: false }),
      createDirectory: async (path, name) => `${path}/${name}`,
    },
  });
  await services.workspaces.refresh();
  assert.equal(await services.uiWorkspace.pickDirectory(), "/tmp/picked");
  assert.deepEqual(
    await services.uiWorkspace.listDirectory("/tmp/one"),
    { path: "/tmp/one", home: "/tmp", crumbs: [], entries: [], truncated: false },
  );
  assert.deepEqual(await services.uiWorkspace.createDirectory("/tmp", "vault"), { path: "/tmp/vault" });
  assert.deepEqual(
    calls.filter((entry) => entry.action === "directory-list" || entry.action === "directory-create").map((entry) => entry.action),
    ["directory-list", "directory-create"],
  );
  assert.equal(await services.uiWorkspace.connectWorkspace("workspace-1"), "conversation-created");
  assert.equal(calls.find((entry) => entry.action === "conversation-create").projectId, "project-1");
  assert.equal(calls.at(-1).target, "/chat?conversationId=conversation-created");
  await services.uiWorkspace.archiveSession("session-1");
  assert.equal(
    calls.some((entry) => entry.action === "conversation-archive" && entry.conversationId === "session-1" && entry.archived === true),
    true,
  );
  await services.dispose();
});

test("uiWorkspace 在缺少宿主目录能力时抛出带方法名与入参的缺口错误", async () => {
  const services = createDshClientRuntimeServices({
    command: stateCommand({ conversations: [], workspaces: [], projects: [] }),
    navigate: () => {},
    hostActions: {},
  });
  await assert.rejects(
    () => services.uiWorkspace.listDirectory("/tmp"),
    /unexpected command directory-list/,
  );
  await assert.rejects(
    () => services.uiWorkspace.pickDirectory(),
    /uiWorkspace\.pickDirectory/,
  );
  await assert.rejects(
    () => services.uiWorkspace.createDirectory("/tmp", ""),
    /uiWorkspace\.createDirectory requires a path and a name/,
  );
  await services.dispose();
});

test("workspaces 快照从宿主会话投影推导工作区会话归属", async () => {
  const { services } = sessionFixture({ total: 5 });
  await services.workspaces.refresh();
  const snapshot = services.workspaces.list.getSnapshot();
  assert.deepEqual(snapshot.items[0].sessionIds, ["session-1"]);
  assert.deepEqual(snapshot.pinnedSessionIds, ["session-1"]);
  assert.deepEqual(snapshot.archivedSessionIds, ["session-archived"]);
  await services.dispose();
});
