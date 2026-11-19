import assert from "node:assert/strict";
import test from "node:test";
import { createDshClientRuntimeServices } from "../src/client-abi/runtime-services.mjs";

function prepareFixture() {
  const calls = [];
  const command = async (input) => {
    calls.push({ ...input });
    if (input.action === "conversation-list") {
      return { items: [], workspaces: [], archivedSessionIds: [], pinnedSessionIds: [], hasMore: false };
    }
    if (input.action === "project-workspaces") return { items: [] };
    if (input.action === "conversation-create") {
      const conversationId = String(input.conversationId || "conversation-prepared");
      if (input.draft === true) {
        return { conversationId, draft: true, preparedAt: "2026-11-18T00:00:00Z", title: String(input.title || "") };
      }
      return { conversationId, scopeSnapshotId: "scope-published" };
    }
    if (input.action === "conversation-delete") return { deleted: true, conversationId: input.conversationId };
    throw new Error(`unexpected command ${input.action}`);
  };
  return { services: createDshClientRuntimeServices({ command, navigate: () => {}, hostActions: {} }), calls };
}

test("sessions.prepare 用 conversation-create 的 draft 参数预分配草稿会话", async () => {
  const { services, calls } = prepareFixture();
  const prepared = await services.sessions.prepare("draft-1", { title: "草稿会话" });
  assert.equal(prepared.sessionId, "draft-1");
  assert.equal(prepared.id, "draft-1");
  assert.equal(prepared.draft, true);
  assert.equal(prepared.prepared, true);
  assert.equal(prepared.preparedAt, "2026-11-18T00:00:00Z");
  const createCall = calls.find((entry) => entry.action === "conversation-create");
  assert.equal(createCall.draft, true);
  assert.equal(createCall.conversationId, "draft-1");
  assert.equal(createCall.title, "草稿会话");
});

test("sessions.prepare 返回的 handle 用同一 id 发布并复用会话作用域", async () => {
  const { services, calls } = prepareFixture();
  const prepared = await services.sessions.prepare("draft-2", { title: "草稿会话" });
  assert.equal(prepared.scope().sessionId, "draft-2");
  const publishedId = await prepared.publish();
  assert.equal(publishedId, "draft-2");
  const publishCall = calls.find((entry) => entry.action === "conversation-create" && entry.draft !== true);
  assert.equal(publishCall.conversationId, "draft-2");
  assert.equal(publishCall.title, "草稿会话");
});

test("sessions.prepare 的 handle 可丢弃未发布的草稿会话", async () => {
  const { services, calls } = prepareFixture();
  const prepared = await services.sessions.prepare("draft-3");
  await prepared.drop();
  const deleteCall = calls.find((entry) => entry.action === "conversation-delete");
  assert.equal(deleteCall.conversationId, "draft-3");
});
