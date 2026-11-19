import assert from "node:assert/strict";
import test from "node:test";
import { createAmitiaDshHostServices } from "../src/runtime/host-services.mjs";

const PLAN_EVENTS = [
  { type: "turn/start", seq: 0, time: 1762000000000, data: { turn: 1 } },
  {
    type: "user/message",
    seq: 1,
    time: 1762000000001,
    data: { message: { id: "m1" }, content: [{ type: "text", text: "plan the quarterly launch" }] },
    surfaceOp: "append",
  },
  {
    type: "assistant/message",
    seq: 2,
    time: 1762000000002,
    data: { message: { id: "m2", content: [{ type: "text", text: "plan draft ready" }] } },
    surfaceOp: "append",
  },
  {
    type: "assistant/message",
    seq: 3,
    time: 1762000000003,
    data: { message: { id: "m3", content: [{ type: "text", text: "unrelated summary" }] } },
    surfaceOp: "append",
  },
];

const BUDGET_EVENTS = [
  { type: "turn/start", seq: 0, time: 1762000000499, data: { turn: 1 } },
  {
    type: "user/message",
    seq: 1,
    time: 1762000000500,
    data: { message: { id: "b1" }, content: [{ type: "text", text: "budget review" }] },
    surfaceOp: "append",
  },
  {
    type: "assistant/message",
    seq: 2,
    time: 1762000000600,
    data: { message: { id: "b2", content: [{ type: "text", text: "plan and budget plan" }] } },
    surfaceOp: "append",
  },
];

function createSearchHost() {
  const conversations = [
    { sessionId: "session-plan", title: "Quarterly plan", createdAt: "2026-11-01T10:00:00Z", cwd: "/workspace/plan" },
    { sessionId: "session-budget", title: "Budget", createdAt: "2026-11-02T11:00:00Z", cwd: "/workspace/budget" },
    { sessionId: "session-empty", title: "Empty", createdAt: "2026-11-03T12:00:00Z" },
  ];
  const state = {
    "dsh-session:session-plan": {
      version: 2,
      conversationId: "session-plan",
      scopeSnapshotId: "",
      events: PLAN_EVENTS,
      updatedAt: "2026-11-01T10:00:00.000Z",
    },
    "dsh-session:session-budget": {
      version: 2,
      conversationId: "session-budget",
      scopeSnapshotId: "",
      events: BUDGET_EVENTS,
      updatedAt: "2026-11-02T11:00:00.000Z",
    },
  };
  const host = {
    async call(method, input = {}) {
      if (method === "host.conversation.list") return { items: conversations, workspaces: [], hasMore: false };
      if (method === "host.state.get") {
        const value = state[input.key];
        return value === undefined ? { found: false } : { found: true, value };
      }
      if (method === "host.conversation.read") return { messages: [], hasMore: false };
      if (method === "host.conversation.turns") return { turns: [] };
      if (method === "host.conversation.scope") return { scopeSnapshotId: "scope-" + String(input.conversationId || "") };
      return {};
    },
  };
  return createAmitiaDshHostServices(host);
}

test("searchEvents 按命中次数排序并给出片段与目标会话头", async () => {
  const services = createSearchHost();
  const page = await services.sessionQuery.searchEvents({ sessionId: "session-plan", query: "plan" });
  assert.equal(page.session.id, "session-plan");
  assert.deepEqual(page.items.map((item) => item.seq), [2, 1]);
  assert.deepEqual(page.items.map((item) => item.type), ["assistant/message", "user/message"]);
  assert.equal(page.items[0].surface, "current");
  assert.match(page.items[0].snippet, /plan/);
  assert.equal(page.nextCursor, undefined);
});

test("searchEvents 支持 metadata 过滤与游标分页", async () => {
  const services = createSearchHost();
  const first = await services.sessionQuery.searchEvents({ sessionId: "session-plan", query: "plan", limit: 1 });
  assert.deepEqual(first.items.map((item) => item.seq), [2]);
  assert.equal(typeof first.nextCursor, "string");
  const second = await services.sessionQuery.searchEvents({ sessionId: "session-plan", query: "plan", limit: 1, cursor: first.nextCursor });
  assert.deepEqual(second.items.map((item) => item.seq), [1]);
  assert.equal(second.nextCursor, undefined);
  const filtered = await services.sessionQuery.searchEvents({ sessionId: "session-plan", query: "plan", filters: [{ kind: "type", values: ["user/message"] }] });
  assert.deepEqual(filtered.items.map((item) => item.seq), [1]);
});

test("searchEvents 拒绝空检索词、无效 limit 与越界游标", async () => {
  const services = createSearchHost();
  await assert.rejects(services.sessionQuery.searchEvents({ sessionId: "session-plan", query: "   " }), /需要非空检索词/);
  await assert.rejects(services.sessionQuery.searchEvents({ sessionId: "session-plan", query: "plan", limit: 0 }), /limit 必须是正整数/);
  await assert.rejects(services.sessionQuery.searchEvents({ sessionId: "session-plan", query: "plan", cursor: "not-a-cursor" }), /cursor 无法解析/);
  const overshoot = Buffer.from(JSON.stringify({ offset: 99 }), "utf8").toString("base64url");
  await assert.rejects(services.sessionQuery.searchEvents({ sessionId: "session-plan", query: "plan", cursor: overshoot }), /cursor 超出结果范围/);
});

test("searchEvents 透传会话缺失的原始错误", async () => {
  const services = createSearchHost();
  await assert.rejects(services.sessionQuery.searchEvents({ sessionId: "session-missing", query: "plan" }), (error) => {
    assert.match(String(error?.message ?? error), /session-missing/);
    return true;
  });
});

test("searchSessions 按会话分组并以最强命中排序", async () => {
  const services = createSearchHost();
  const page = await services.sessionQuery.searchSessions({ query: "plan" });
  assert.deepEqual(page.items.map((item) => item.header.id), ["session-budget", "session-plan"]);
  const budget = page.items[0];
  assert.equal(budget.bestMatch.seq, 2);
  assert.equal(budget.bestMatch.sessionId, "session-budget");
  assert.match(budget.bestMatch.snippet, /plan/);
  assert.equal(budget.live, false);
  assert.equal(budget.persisted, true);
});

test("searchSessions 支持会话过滤、事件过滤与游标分页", async () => {
  const services = createSearchHost();
  const filtered = await services.sessionQuery.searchSessions({ query: "plan", sessionFilters: [{ kind: "cwd", values: ["/workspace/plan"] }] });
  assert.deepEqual(filtered.items.map((item) => item.header.id), ["session-plan"]);
  const byType = await services.sessionQuery.searchSessions({ query: "plan", eventFilters: [{ kind: "type", values: ["user/message"] }] });
  assert.deepEqual(byType.items.map((item) => item.header.id), ["session-plan"]);
  assert.deepEqual(byType.items.map((item) => item.bestMatch.type), ["user/message"]);
  const first = await services.sessionQuery.searchSessions({ query: "plan", limit: 1 });
  assert.deepEqual(first.items.map((item) => item.header.id), ["session-budget"]);
  const second = await services.sessionQuery.searchSessions({ query: "plan", limit: 1, cursor: first.nextCursor });
  assert.deepEqual(second.items.map((item) => item.header.id), ["session-plan"]);
});

test("searchSessions 没有命中时返回空页", async () => {
  const services = createSearchHost();
  const page = await services.sessionQuery.searchSessions({ query: "nonexistent-token" });
  assert.deepEqual(page.items, []);
  assert.equal(page.nextCursor, undefined);
  await assert.rejects(services.sessionQuery.searchSessions({ query: "" }), /需要非空检索词/);
});
