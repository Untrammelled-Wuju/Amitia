import { describe, expect, it } from "vitest";
import { isServiceTransitionMessage } from "../runtime/service-transition";

describe("服务切换错误识别", () => {
  it("把服务切换期间的取消错误视为预期状态", () => {
    expect(isServiceTransitionMessage("服务已变化，旧项目列表已丢弃")).toBe(true);
    expect(isServiceTransitionMessage("云端服务连接正在变化，当前回复已中断；请等待当前服务确认")).toBe(true);
    expect(isServiceTransitionMessage("调用角色已切换，原回复已中断，将使用新角色开始对话")).toBe(true);
  });

  it("不吞掉普通业务错误", () => {
    expect(isServiceTransitionMessage("模型服务不可用")).toBe(false);
    expect(isServiceTransitionMessage(new Error("数据库连接失败"))).toBe(false);
  });
});
