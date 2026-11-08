import { describe, expect, it } from "vitest";
import { canProbeRuntime, isDeviceBound, isDeviceOnline } from "../runtime/device-presence";

describe("设备在线状态", () => {
  it("将后端 ready 和在线别名都视为在线", () => {
    expect(isDeviceOnline({ trustState: "trusted", presence: "ready" })).toBe(true);
    expect(isDeviceOnline({ trustState: "trusted", presence: "online" })).toBe(true);
  });

  it("已撤销、离线、过期和未知状态均视为离线", () => {
    expect(isDeviceOnline({ trustState: "revoked", presence: "ready" })).toBe(false);
    expect(isDeviceOnline({ trustState: "trusted", presence: "disconnected" })).toBe(false);
    expect(isDeviceOnline({ trustState: "trusted", presence: "expired" })).toBe(false);
    expect(isDeviceOnline({ trustState: "trusted" })).toBe(false);
  });

  it("已撤销设备不再计入绑定设备和授权目标", () => {
    expect(isDeviceBound({ trustState: "trusted" })).toBe(true);
    expect(isDeviceBound({ trustState: "revoked" })).toBe(false);
  });

  it("只有存在活动 Mesh 会话的 Runtime 才能远程探测", () => {
    expect(canProbeRuntime({ runtimeSessionId: "rtsess_active" })).toBe(true);
    expect(canProbeRuntime({})).toBe(false);
  });
});
