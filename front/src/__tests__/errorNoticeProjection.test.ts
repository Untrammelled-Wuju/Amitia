import { describe, expect, it } from "vitest";
import {
  errorNoticeText,
  formatTurnErrorDetail,
} from "../conversation/rendering/errorNoticeProjection";

describe("error notice projection", () => {
  it("places the complete raw detail between both hyphens", () => {
    const turn = {
      items: [{
        type: "error",
        errorCode: "tool_execution_failed",
        resultJson: JSON.stringify({
          provider: "test",
          errorCode: "tool_execution_failed",
          message: "tool execution cancelled",
          cause: { message: "inner failure" },
          rawError: "Bad state: inner failure",
        }),
      }],
    };
    const detail = formatTurnErrorDetail(turn);
    expect(detail).toContain("provider: test");
    expect(detail).toContain("errorCode: tool_execution_failed");
    expect(detail).toContain("cause:");
    expect(detail).toContain("rawError: Bad state: inner failure");
    expect(detail).toContain("originalPayload:");
    expect(errorNoticeText(detail)).toBe(`-${detail}-`);
    expect(errorNoticeText(detail)).not.toBe("-错误-");
  });
});
