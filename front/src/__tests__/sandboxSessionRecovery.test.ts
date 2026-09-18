import { describe, expect, it } from "vitest";
import { isSandboxSessionMissingError } from "../components/extension/sandboxSessionCache";

describe("沙箱 WebUI 会话恢复", () => {
  it("识别未经转换的 404 响应", () => {
    expect(
      isSandboxSessionMissingError({
        response: {
          status: 404,
          data: { code: "webui_session_not_found" },
        },
      }),
    ).toBe(true);
  });

  it("识别统一错误转换后保留的原始响应体", () => {
    expect(
      isSandboxSessionMissingError({
        code: 0,
        message: "Request failed with status code 404",
        raw: {
          code: "webui_session_not_found",
          error: "sandbox_webui: session not found",
        },
      }),
    ).toBe(true);
  });

  it("从沙箱错误消息识别失效会话", () => {
    expect(
      isSandboxSessionMissingError(new Error("sandbox_webui: session not found")),
    ).toBe(true);
  });

  it("不把其他错误当成会话失效", () => {
    expect(isSandboxSessionMissingError(new Error("network error"))).toBe(false);
    expect(isSandboxSessionMissingError({ response: { status: 500 } })).toBe(false);
  });
});
