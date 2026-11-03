import { describe, expect, it } from "vitest";
import {
  OwnedChatError,
  consumeOwnedChatStream,
} from "../runtime/device-owned-chat";

describe("device owned chat errors", () => {
  it("preserves every raw error field and exposes them on the thrown error", () => {
    const payload = {
      code: "mesh.wrong_owner",
      errorCode: "task_execution_failed",
      message: "禁止将设备专属数据保存到其他设备",
      userMessage: "禁止将设备专属数据保存到其他设备",
      internalMessage: "resource owner was device-b",
      detail: "owner=device-b request=request-1 role=role-1",
      cause: "resource owner was device-b",
      rawError: "resource owner was device-b: 禁止将设备专属数据保存到其他设备",
      stage: "execution",
      retryable: true,
    };
    const error = new OwnedChatError(payload, "fallback", 403);
    expect(error.message).toContain(payload.message);
    expect(error.message).toContain(payload.detail);
    expect(error.message).toContain(payload.rawError);
    expect(error.code).toBe(payload.code);
    expect(error.internalMessage).toBe(payload.internalMessage);
    expect(error.causeDetail).toBe(payload.cause);
    expect(error.rawPayload).toBe(payload);
    expect(error.status).toBe(403);
  });

  it("does not reduce an SSE failed event to event.message", async () => {
    const event = {
      type: "failed",
      code: "mesh.wrong_owner",
      errorCode: "task_execution_failed",
      message: "禁止将设备专属数据保存到其他设备",
      internalMessage: "resource owner was device-b",
      detail: "owner=device-b request=request-1",
      cause: "owner validation failed",
      rawError: "owner validation failed: 禁止将设备专属数据保存到其他设备",
      stage: "execution",
      retryable: true,
    };
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(new TextEncoder().encode(`data: ${JSON.stringify(event)}\n\n`));
        controller.close();
      },
    });
    try {
      await consumeOwnedChatStream(body, "request-1", () => undefined);
      throw new Error("stream should fail");
    } catch (error) {
      expect(error).toBeInstanceOf(OwnedChatError);
      const owned = error as OwnedChatError;
      expect(owned.message).toContain(event.internalMessage);
      expect(owned.message).toContain(event.detail);
      expect(owned.message).toContain(event.rawError);
      expect(owned.code).toBe("mesh.wrong_owner");
    }
  });
});
