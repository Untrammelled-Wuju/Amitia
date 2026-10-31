import { describe, expect, it } from "vitest";
import { originalErrorDetail, originalErrorMessage } from "@/runtime/original-error";
import { classifyError } from "@/composables/request";

describe("原始错误信息", () => {
  it("优先返回后端响应中的原始 message 和 internalMessage", () => {
    const error = {
      message: "Request failed with status code 409",
      response: {
        status: 409,
        data: {
          message: "项目数据版本冲突",
          internalMessage: "revision 3 != 4",
          code: 409,
        },
      },
    };
    expect(originalErrorMessage(error)).toBe("项目数据版本冲突\nrevision 3 != 4");
    expect(originalErrorDetail(error)).toBe("revision 3 != 4");
  });

  it("Error 包装对象也不会隐藏 response 中的后端原文", () => {
    const error = new Error("Request failed with status code 409");
    Object.assign(error, {
      response: {
        status: 409,
        data: {
          message: "项目数据版本冲突",
          detail: "revision 3 != 4",
        },
      },
    });
    expect(originalErrorMessage(error)).toBe("项目数据版本冲突\nrevision 3 != 4");
  });

  it("请求层不再把网络和时间错误改写为固定文案", () => {
    const network = classifyError(null, {
      code: "ERR_NETWORK",
      message: "Network Error: connect ECONNREFUSED 127.0.0.1:18899",
    } as any);
    const timeout = classifyError(null, {
      code: "ECONNABORTED",
      message: "timeout of 30000ms exceeded",
    } as any);
    expect(network.message).toBe("Network Error: connect ECONNREFUSED 127.0.0.1:18899");
    expect(timeout.message).toBe("timeout of 30000ms exceeded");
  });
});
