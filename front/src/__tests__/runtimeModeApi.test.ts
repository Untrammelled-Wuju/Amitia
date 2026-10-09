import { describe, expect, it, vi } from "vitest";

const get = vi.hoisted(() => vi.fn());
const post = vi.hoisted(() => vi.fn());

vi.mock("../composables/useApi", () => ({
  apiClient: { get, post },
}));

import { fetchModeApi, validateModeApi } from "../views/runtime-mode/api";

describe("运行模式 API 响应解包", () => {
  it("正确提取统一 API 响应中的 data", async () => {
    const mode = { deployMode: "desktop-local", bridge: { mode: "local" } };
    get.mockResolvedValueOnce({ data: { code: 200, data: mode } });
    expect(await fetchModeApi()).toEqual(mode);
  });

  it("支持未包装响应和空响应", async () => {
    const mode = { deployMode: "cloud-web" };
    get.mockResolvedValueOnce({ data: mode });
    get.mockResolvedValueOnce({ data: { code: 200, data: null } });
    expect(await fetchModeApi()).toEqual(mode);
    expect(await fetchModeApi()).toBeNull();
  });

  it("配置校验结果提取统一响应的 data", async () => {
    const result = { valid: true, errors: [], warnings: [] };
    post.mockResolvedValueOnce({ data: { code: 200, data: result } });
    expect(await validateModeApi()).toEqual(result);
  });
});
