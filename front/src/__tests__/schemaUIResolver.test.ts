import { describe, expect, it } from "vitest";
import {
  resolveActionInput,
  resolveBinding,
  setPath,
  type SchemaUIActionBinding,
} from "@/components/extension/schema-ui-utils";

describe("schema UI resolver", () => {
  it("resolves a target property from a different source path", () => {
    expect(
      resolveBinding(
        {
          path: "text",
          source: "state",
          sourcePath: "dashboard.status.schedulerRunning",
        },
        {},
        { localState: { dashboard: { status: { schedulerRunning: true } } } },
      ),
    ).toBe(true);
  });

  it("resolves action templates from form, context and item values", () => {
    const input = {
      peerId: "$item.id",
      enabled: "$form.peerEnabled",
      conversationId: "$context.conversationId",
    };
    expect(
      resolveActionInput(
        input,
        { peerEnabled: true },
        { conversationId: "conversation-1" },
        { id: "wxid-1" },
      ),
    ).toEqual({
      peerId: "wxid-1",
      enabled: true,
      conversationId: "conversation-1",
    });
  });

  it("writes action state to a nested path", () => {
    const state: Record<string, unknown> = {};
    setPath(state, "dashboard.status", { running: true });
    expect(state).toEqual({ dashboard: { status: { running: true } } });
  });

  it("accepts action statePath as part of the binding contract", () => {
    const action: SchemaUIActionBinding = {
      action_id: "command",
      statePath: "dashboard",
    };
    expect(action.statePath).toBe("dashboard");
  });
});
