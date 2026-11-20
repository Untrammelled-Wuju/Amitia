import { mount } from "@vue/test-utils";
import { defineComponent } from "vue";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useAppClientModules } from "@/composables/useAppClientModules";
import { apiClient } from "@/composables/useApi";

vi.mock("@/composables/useApi", () => ({
  apiClient: {
    get: vi.fn(),
    post: vi.fn(),
  },
}));

const rejection = new Error("核心服务不可用");

const Host = defineComponent({
  setup() {
    const { modules, error } = useAppClientModules({ enabled: true });
    return { modules, error };
  },
  template: "<div />",
});

const unhandled: unknown[] = [];
const onUnhandled = (reason: unknown) => unhandled.push(reason);

afterEach(() => {
  process.off("unhandledRejection", onUnhandled);
  unhandled.length = 0;
  vi.restoreAllMocks();
  vi.clearAllMocks();
});

async function mountFailingHost() {
  process.on("unhandledRejection", onUnhandled);
  const logged = vi.spyOn(console, "error").mockImplementation(() => {});
  const wrapper = mount(Host);
  await vi.waitFor(() => expect(wrapper.vm.error).toBeInstanceOf(Error));
  await new Promise((resolve) => setTimeout(resolve, 0));
  return { wrapper, logged };
}

describe("app client module loading", () => {
  it("exposes the original failure instead of rejecting without a handler", async () => {
    vi.mocked(apiClient.get).mockRejectedValue(rejection);
    const { wrapper, logged } = await mountFailingHost();
    expect(wrapper.vm.error).toBe(rejection);
    expect(wrapper.vm.modules).toEqual([]);
    expect(unhandled).toEqual([]);
    const output = String(logged.mock.calls[0]?.[0] ?? "");
    expect(output).toContain("客户端模块清单加载失败");
    expect(output).toContain("核心服务不可用");
    wrapper.unmount();
  });

  it("keeps the normalized service error message and original payload", async () => {
    const normalized = { code: 10001, message: "本地凭据无效", detail: "ERR_NETWORK", severity: "toast" };
    vi.mocked(apiClient.get).mockRejectedValue(normalized);
    const { wrapper, logged } = await mountFailingHost();
    expect(wrapper.vm.error?.message).toBe("本地凭据无效");
    expect((wrapper.vm.error as Error & { cause?: unknown }).cause).toBe(normalized);
    expect(String(logged.mock.calls[0]?.[0] ?? "")).toContain("本地凭据无效");
    expect(unhandled).toEqual([]);
    wrapper.unmount();
  });
});
