import { describe, expect, it } from "vitest";
import {
  coreOwnershipLabel,
  deviceOwnershipLabel,
  hasOwnershipPrefix,
  ownershipLabel,
  ownershipPrefix,
} from "../runtime/ownership-label";

describe("资源归属命名", () => {
  it("统筹中心资源使用统筹中心前缀", () => {
    expect(coreOwnershipLabel("skill", "翻译助手")).toBe("统筹中心-skill-翻译助手");
    expect(coreOwnershipLabel("角色", "小美")).toBe("统筹中心-角色-小美");
    expect(coreOwnershipLabel("mcp", "天气服务")).toBe("统筹中心-mcp-天气服务");
    expect(coreOwnershipLabel("任务", "每日巡检")).toBe("统筹中心-任务-每日巡检");
  });

  it("设备资源使用设备名前缀", () => {
    expect(deviceOwnershipLabel("红米K50", "角色", "小美")).toBe("设备红米K50-角色-小美");
    expect(deviceOwnershipLabel("MacBook Pro", "团队", "研发组")).toBe("设备MacBook Pro-团队-研发组");
  });

  it("缺少设备名时使用未命名设备而不是设备标识", () => {
    expect(deviceOwnershipLabel("  ", "对话", "周报")).toBe("设备未命名设备-对话-周报");
    expect(ownershipPrefix({ coordinated: false })).toBe("设备未命名设备");
  });

  it("已经带归属前缀的名称不会重复追加", () => {
    expect(coreOwnershipLabel("skill", "统筹中心-skill-翻译助手")).toBe("统筹中心-skill-翻译助手");
    expect(deviceOwnershipLabel("红米K50", "角色", "设备红米K50-角色-小美")).toBe(
      "设备红米K50-角色-小美",
    );
  });

  it("普通名称不会被误判为已带前缀", () => {
    expect(hasOwnershipPrefix("翻译助手")).toBe(false);
    expect(hasOwnershipPrefix("统筹中心-未知类别-东西")).toBe(false);
    expect(hasOwnershipPrefix("设备-角色-小美")).toBe(false);
    expect(ownershipLabel({ coordinated: true, category: "skill", name: "" })).toBe("统筹中心");
  });
});
