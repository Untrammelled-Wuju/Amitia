import assert from "node:assert/strict";
import test from "node:test";
import {
  conversationIdOf,
  conversationReference,
  normalizeConversationRequest,
  parseConversationReference,
} from "../src/runtime/conversation-id.mjs";

const OWNER_ID = "dev_5afec0d2-6c80-4230-b18d-88a102cfae9a";
const CONVERSATION_ID = "4883a2f0-9df5-56c4-b8b1-6099bd108817";

test("parses an owned conversation reference into its origin", () => {
  const reference = conversationReference({ ownerId: OWNER_ID, id: CONVERSATION_ID });
  assert.equal(reference, `meshconv1:${encodeURIComponent(OWNER_ID)}:${CONVERSATION_ID}`);
  assert.deepEqual(parseConversationReference(reference), { ownerId: OWNER_ID, id: CONVERSATION_ID });
  assert.equal(conversationIdOf(reference), CONVERSATION_ID);
});

test("keeps plain conversation ids untouched", () => {
  assert.equal(conversationIdOf(CONVERSATION_ID), CONVERSATION_ID);
  assert.equal(parseConversationReference(CONVERSATION_ID), undefined);
});

test("decodes escaped characters inside the reference segments", () => {
  const reference = `meshconv1:${encodeURIComponent("owner:with:colon")}:${encodeURIComponent("id/with: colon")}`;
  assert.deepEqual(parseConversationReference(reference), { ownerId: "owner:with:colon", id: "id/with: colon" });
  assert.equal(conversationIdOf(reference), "id/with: colon");
});

test("rejects malformed references instead of guessing", () => {
  assert.throws(() => conversationIdOf("meshconv1:only-owner"), /会话来源无效/);
  assert.throws(() => conversationIdOf(`meshconv1:${OWNER_ID}:${CONVERSATION_ID}:extra`), /会话来源无效/);
  assert.throws(() => conversationIdOf("meshconv1:%E0%A4%A:x"), /会话来源无效/);
  assert.throws(() => conversationIdOf(`meshconv1::${CONVERSATION_ID}`), /会话来源无效/);
  assert.throws(() => conversationIdOf("meshconv1:owner:"), /会话来源无效/);
});

test("normalizes request conversation and session ids in one pass", () => {
  const reference = conversationReference({ ownerId: OWNER_ID, id: CONVERSATION_ID });
  const request = { action: "conversation-archive", conversationId: reference, sessionId: reference, archived: true };
  const normalized = normalizeConversationRequest(request);
  assert.equal(normalized.conversationId, CONVERSATION_ID);
  assert.equal(normalized.sessionId, CONVERSATION_ID);
  assert.equal(normalized.archived, true);
  assert.equal(request.conversationId, reference);
});

test("returns the original request when nothing needs normalization", () => {
  const request = { action: "conversation-read", conversationId: CONVERSATION_ID };
  assert.equal(normalizeConversationRequest(request), request);
  assert.equal(normalizeConversationRequest(null).action, undefined);
});
