import { describe, expect, it } from "vitest";
import {
  ROLE_MANAGEMENT_TARGET_HEADER,
  withRoleTarget,
} from "@/runtime/role-management-target";
import { isDeviceRoleManagementPath } from "@/runtime/runtime-adapter";
import { isDeviceReminderPath } from "@/runtime/runtime-adapter";

describe("角色来源路由", () => {
  it("写入来源请求头且不改动原有请求头", () => {
    const headers = { "X-Amitia-Role-Authority": "a".repeat(64) };
    const next = withRoleTarget(headers, "local");
    expect(next).toEqual({
      "X-Amitia-Role-Authority": "a".repeat(64),
      [ROLE_MANAGEMENT_TARGET_HEADER]: "local",
    });
    expect(headers).toEqual({ "X-Amitia-Role-Authority": "a".repeat(64) });
  });

  it("未指定来源时不注入请求头", () => {
    expect(withRoleTarget(undefined, null)).toBeUndefined();
    expect(withRoleTarget({ a: "1" }, null)).toEqual({ a: "1" });
  });

  it("角色与团队接口都属于按来源路由的管理路径", () => {
    expect(isDeviceRoleManagementPath("/api/characters")).toBe(true);
    expect(isDeviceRoleManagementPath("/api/characters/role-1")).toBe(true);
    expect(isDeviceRoleManagementPath("/api/roles/teams")).toBe(true);
    expect(isDeviceRoleManagementPath("/api/roles")).toBe(true);
    expect(isDeviceRoleManagementPath("/api/characters/generate-card")).toBe(false);
    expect(isDeviceRoleManagementPath("/api/chat/messages")).toBe(false);
  });

  it("主动任务接口属于按来源路由的任务路径", () => {
    expect(isDeviceReminderPath("/api/reminders")).toBe(true);
    expect(isDeviceReminderPath("/api/reminders?kind=proactive_task")).toBe(true);
    expect(isDeviceReminderPath("/api/reminders/task-1/toggle")).toBe(true);
    expect(isDeviceReminderPath("/api/reminders-archive")).toBe(false);
    expect(isDeviceReminderPath("/api/mcp")).toBe(false);
  });
});
