import { describe, expect, it } from "vitest"
import {
  DEFAULT_CLOUD_SERVER_URL,
  configToLabel,
  normalizeServerURL,
  validateDeploymentConfig,
  validateSelfHostedURL,
} from "./deployment"

describe("deployment", () => {
  it("normalizes server url", () => {
    expect(normalizeServerURL(" https://example.com/api/?a=1#hash ")).toBe("https://example.com/api")
  })

  it("validates cloud config", () => {
    expect(validateDeploymentConfig({ mode: "cloud" })).toEqual({
      mode: "cloud",
      serverURL: DEFAULT_CLOUD_SERVER_URL,
    })
  })

  it("validates self hosted config", () => {
    expect(validateDeploymentConfig({ mode: "self-hosted", serverURL: "https://demo.amitia.cn/" })).toEqual({
      mode: "self-hosted",
      serverURL: "https://demo.amitia.cn",
    })
  })

  it("rejects blocked self hosted host", () => {
    expect(() => validateSelfHostedURL("http://0.0.0.0:18899")).toThrow("不可路由地址")
  })

  it("maps config to label", () => {
    expect(configToLabel({ mode: "local" })).toBe("本地模式")
    expect(configToLabel({ mode: "cloud" })).toBe("云端模式")
    expect(configToLabel({ mode: "self-hosted", serverURL: "https://demo.amitia.cn" })).toBe("自建服务器")
  })
})
