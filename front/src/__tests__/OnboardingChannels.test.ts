import { afterEach, describe, expect, it } from "vitest"
import boundarySource from "../views/onboarding/components/StageBoundary.vue?raw"
import wechatSource from "../views/wechat-connect/WechatConnectView.vue?raw"
import qqSource from "../views/qq-connect/QqConnectView.vue?raw"
import { getQQApiBaseURL, resetRuntimeConnectionCache } from "../runtime/runtime-adapter"

describe("引导页渠道连接", () => {
  afterEach(() => {
    delete (window as any).amitiaDesktop
    resetRuntimeConnectionCache()
  })

  it("微信和 QQ 按钮打开真实连接弹窗", () => {
    expect(boundarySource).toContain("openChannelDialog('wechat')")
    expect(boundarySource).toContain("openChannelDialog('qq')")
    expect(boundarySource).toContain("WechatConnectView")
    expect(boundarySource).toContain("QqConnectView")
    expect(boundarySource).toContain("@connectionChanged=\"handleConnectionChanged\"")
    expect(boundarySource).not.toContain("permissions.wechat = !permissions.wechat")
    expect(boundarySource).not.toContain("permissions.qq = !permissions.qq")
  })

  it("连接页向引导步骤回写真实状态", () => {
    expect(wechatSource).toContain('emit("connectionChanged", detail.value?.status === "connected")')
    expect(qqSource).toContain('emit("connectionChanged", qqOnline.value)')
  })

  it("本地 QQ 连接使用侧车 API 路径", async () => {
    ;(window as any).amitiaDesktop = {
      getDeploymentConfig: async () => ({ mode: "local" }),
    }
    resetRuntimeConnectionCache()
    await expect(getQQApiBaseURL()).resolves.toBe("http://127.0.0.1:9877/api")
  })
})
