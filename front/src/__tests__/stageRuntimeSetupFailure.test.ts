import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, expect, it, vi } from "vitest";
import StageRuntimeSetup from "../views/onboarding/components/StageRuntimeSetup.vue";

vi.mock("@/runtime/web-access", () => ({
  getWebAccessStatus: vi.fn(),
  setupWebAccess: vi.fn(),
}));
vi.mock("@/runtime/runtime-adapter", () => ({
  getApiBaseURL: vi.fn().mockResolvedValue("http://127.0.0.1:9"),
  getBackendAuthHeaders: vi.fn().mockResolvedValue({}),
  getCurrentDevicePairingStatus: vi.fn(),
  isCurrentDevicePaired: vi.fn().mockResolvedValue(false),
  provisionCurrentDeviceMesh: vi.fn(),
}));

afterEach(() => {
  vi.unstubAllGlobals();
});

it("shows the original remote check error and offers retry", async () => {
  const fetchMock = vi.fn().mockRejectedValue(new TypeError("fetch failed: ECONNREFUSED 127.0.0.1:9"));
  vi.stubGlobal("fetch", fetchMock);
  const wrapper = mount(StageRuntimeSetup, {
    props: { deployMode: "remote", serverURL: "http://127.0.0.1:9" },
  });

  await flushPromises();

  expect(wrapper.text()).toContain("连接远程统筹中心失败");
  expect(wrapper.text()).toContain("fetch failed: ECONNREFUSED 127.0.0.1:9");
  const retry = wrapper.findAll("button").find((button) => button.text() === "重新检查");
  expect(retry).toBeTruthy();
  await retry!.trigger("click");
  await flushPromises();
  expect(fetchMock).toHaveBeenCalledTimes(2);
});
