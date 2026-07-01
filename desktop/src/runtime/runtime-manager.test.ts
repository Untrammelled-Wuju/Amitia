import { describe, expect, it } from "vitest"
import { DesktopRuntimeManager } from "./runtime-manager"

describe("DesktopRuntimeManager", () => {
  it("returns local desktop connection", async () => {
    const manager = new DesktopRuntimeManager({ mode: "local" })
    await manager.initialize()
    await manager.start()
    await expect(manager.getConnection()).resolves.toEqual({
      mode: "desktop-local",
      apiBaseURL: "",
      websocketBaseURL: "",
    })
    expect(manager.getStatus().state).toBe("not-installed")
  })

  it("returns cloud connection and ready status", async () => {
    const manager = new DesktopRuntimeManager({ mode: "cloud" })
    await manager.initialize()
    await manager.start()
    await expect(manager.getConnection()).resolves.toEqual({
      mode: "desktop-cloud",
      apiBaseURL: "https://api.amitia.cn",
      websocketBaseURL: "wss://api.amitia.cn",
    })
    expect(manager.getStatus().state).toBe("ready")
  })

  it("switches status after config update", async () => {
    const manager = new DesktopRuntimeManager({ mode: "cloud" })
    manager.setDeploymentConfig({ mode: "self-hosted", serverURL: "https://demo.amitia.cn" })
    await expect(manager.getConnection()).resolves.toEqual({
      mode: "desktop-self-hosted",
      apiBaseURL: "https://demo.amitia.cn",
      websocketBaseURL: "wss://demo.amitia.cn",
    })
    expect(manager.getStatus().mode).toBe("self-hosted")
  })
})
