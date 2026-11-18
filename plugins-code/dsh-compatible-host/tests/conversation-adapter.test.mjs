import assert from "node:assert/strict";
import test from "node:test";
import {
  adaptDshConversationDefinition,
  registerDshConversationDefinition,
} from "../src/client-abi/conversation-adapter.mjs";

test("maps DSH conversation definitions to Amitia node projections", () => {
  const definition = {
    kind: "plan",
    target: "chat",
    match(event) {
      if (event.type !== "tool/call" || event.data.name !== "exit_plan_mode") return null;
      return { id: event.data.callId, role: "start" };
    },
    start(_context, match) {
      return { plan: match.event.data.arguments.plan };
    },
    update(context) {
      return context.state;
    },
    buildViewNode(context) {
      return {
        key: context.key,
        kind: context.kind,
        id: context.id,
        target: context.start.location.kind,
        data: context.state,
      };
    },
  };
  const adapted = adaptDshConversationDefinition("@example/conversation", definition, 0);
  const runtimeEvent = {
    id: "event-1",
    eventType: "tool.call",
    sequence: 3,
    timestamp: "2026-11-13T00:00:00Z",
    payload: { name: "exit_plan_mode", callId: "call-1", arguments: { plan: "# Plan" } },
  };
  const matched = adapted.match(runtimeEvent);
  assert.equal(matched.contextId, "call-1");
  const state = adapted.create(runtimeEvent, matched);
  assert.deepEqual(state, { plan: "# Plan" });
  const node = adapted.buildViewNode({
    contextId: "call-1",
    state,
    events: [runtimeEvent],
    firstEvent: runtimeEvent,
  });
  assert.equal(node.key, "call-1");
  assert.equal(node.target, "unresolved");
  assert.deepEqual(node.payload.data, { plan: "# Plan" });
});

test("registers a DSH definition on the Amitia conversation runtime", () => {
  let registered;
  const runtime = {
    register(key, component, definition) {
      registered = { key, component, definition };
      return () => {};
    },
  };
  const component = {};
  const dispose = registerDshConversationDefinition(runtime, "@example/conversation", {
    kind: "fixture",
    match: { "fixture/start": () => ({ id: "one", role: "start" }) },
    start: () => "state",
    update: (context) => context.state,
    buildViewNode: () => null,
  }, 2, component);
  assert.equal(registered.key, "dsh:@example/conversation:fixture:2");
  assert.equal(registered.component, component);
  assert.equal(registered.definition.match({
    id: "event",
    eventType: "fixture.start",
    payload: {},
  }).contextId, "one");
  assert.equal(typeof dispose, "function");
});
