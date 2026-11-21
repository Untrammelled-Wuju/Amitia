import { describe, expect, it } from "vitest";
import { buildTeamMemberPayload } from "@/runtime/team-member-payload";

describe("团队成员提交", () => {
  it("统筹中心角色不带设备归属", () => {
    const payload = buildTeamMemberPayload({
      source: "center",
      deviceId: "",
      roleId: "role-a",
      role: "member",
    });
    expect(payload).toEqual({
      memberType: "agent",
      memberId: "role-a",
      role: "member",
      responderMode: "mention_or_lead",
      capabilities: [],
    });
  });

  it("设备角色携带设备归属、调用模式与角色版本", () => {
    const payload = buildTeamMemberPayload({
      source: "device",
      deviceId: "device-b",
      roleId: "role-b",
      role: "lead",
      roleRevision: 7,
    });
    expect(payload).toEqual({
      memberType: "agent",
      memberId: "role-b",
      role: "lead",
      responderMode: "lead",
      capabilities: ["*"],
      ownerType: "device",
      ownerId: "device-b",
      invocationMode: "role",
      roleRevision: 7,
    });
  });

  it("缺少角色版本时不下发版本字段", () => {
    const payload = buildTeamMemberPayload({
      source: "device",
      deviceId: "device-b",
      roleId: "role-b",
      role: "member",
    });
    expect(payload.roleRevision).toBeUndefined();
    expect(payload.ownerId).toBe("device-b");
  });
});
