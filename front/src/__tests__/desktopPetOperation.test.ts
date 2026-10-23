import { describe, expect, it, vi } from "vitest";
import { trackInstallationOperation } from "@/composables/installation-operation";

const sleep = () => Promise.resolve();

describe("桌宠安装操作跟踪", () => {
  it("等待运行端 ACK 后继续跟踪到 completed", async () => {
    const load = vi
      .fn()
      .mockResolvedValueOnce({
        operationId: "operation-one",
        status: "waiting_runtime_ack",
      })
      .mockResolvedValueOnce({
        operationId: "operation-one",
        status: "completed",
      });

    const result = await trackInstallationOperation(
      {
        operationId: "operation-one",
        status: "waiting_runtime_ack",
      },
      load,
      { attempts: 3, sleep },
    );

    expect(load).toHaveBeenCalledTimes(2);
    expect(result.status).toBe("completed");
  });

  it("运行端失败会返回明确错误而不是伪装成功", async () => {
    const load = vi.fn().mockResolvedValue({
      operationId: "operation-two",
      status: "failed_terminal",
      errorCode: "RUNTIME_COMMAND_FAILED",
      errorMessage: "桌宠运行端拒绝应用",
    });

    await expect(
      trackInstallationOperation(
        {
          operationId: "operation-two",
          status: "waiting_runtime_ack",
        },
        load,
        { attempts: 3, sleep },
      ),
    ).rejects.toThrow("桌宠运行端拒绝应用");
  });

  it("离线超时保留 waiting_runtime_ack 供界面展示真实等待状态", async () => {
    const load = vi.fn().mockResolvedValue({
      operationId: "operation-three",
      status: "waiting_runtime_ack",
    });

    const result = await trackInstallationOperation(
      {
        operationId: "operation-three",
        status: "created",
      },
      load,
      { attempts: 2, sleep },
    );

    expect(result.status).toBe("waiting_runtime_ack");
  });
});
