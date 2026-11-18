import { describe, expect, it } from "vitest";
import {
  extensionPageActionContext,
  extensionPageActionResult,
} from "@/components/extension/pageActionContext";

describe("extension page action context", () => {
  it("drops the page session while preserving host authority fields", () => {
    const context = extensionPageActionContext({
      extensionId: "com.amitia/dsh-compatible-host",
      pageId: "dsh-compatible-page",
      sessionId: "pgs_page_session",
      characterId: "character-1",
      conversationId: "conversation-1",
      scope: "global",
    });

    expect(context).toEqual({
      extensionId: "com.amitia/dsh-compatible-host",
      pageId: "dsh-compatible-page",
      characterId: "character-1",
      conversationId: "conversation-1",
      scope: "global",
    });
    expect(context).not.toHaveProperty("sessionId");
  });

  it("unwraps structured tool results and preserves the original error", () => {
    expect(extensionPageActionResult({
      ok: true,
      result: {
        structured: { packages: [{ pluginId: "plugin-1" }] },
        error: null,
      },
    })).toEqual({ packages: [{ pluginId: "plugin-1" }] });

    expect(() => extensionPageActionResult({
      ok: true,
      result: {
        structured: null,
        error: { message: "original package failure", code: "failed" },
      },
    })).toThrow("original package failure");
  });
});
