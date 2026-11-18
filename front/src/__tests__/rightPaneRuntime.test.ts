import { describe, expect, it, vi } from "vitest";
import { UIRightPaneRuntime } from "@/ui-runtime/rightPaneRuntime";

describe("right pane runtime", () => {
  it("publishes open state, params, and close lifecycle", () => {
    const runtime = new UIRightPaneRuntime();
    const listener = vi.fn();
    runtime.subscribe(listener);
    runtime.open("dsh-resource://plan/session/call", {
      title: "Plan",
      params: { callId: "call" },
      sessionId: "session",
    });
    expect(runtime.getSnapshot()).toEqual({
      open: true,
      address: "dsh-resource://plan/session/call",
      title: "Plan",
      params: { callId: "call" },
      sessionId: "session",
    });
    runtime.close();
    expect(runtime.getSnapshot()).toEqual({ open: false });
    expect(listener).toHaveBeenCalledTimes(2);
  });
});
