import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import SchemaUIImage from "../components/extension/SchemaUIImage.vue";

const mocks = vi.hoisted(() => ({ get: vi.fn() }));
vi.mock("@/composables/useApi", () => ({ apiClient: { get: mocks.get } }));

describe("Schema 插件图片资源", () => {
  it("通过鉴权通道读取插件图片并在移除时释放对象地址", async () => {
    const create = vi.fn(() => "blob:emote-preview");
    const revoke = vi.fn();
    vi.stubGlobal("URL", { createObjectURL: create, revokeObjectURL: revoke });
    mocks.get.mockResolvedValue({ data: new Blob(["image"], { type: "image/png" }) });
    const wrapper = mount(SchemaUIImage, { props: { src: "/api/extension/resources/ticket", alt: "微笑" } });
    try {
      await flushPromises();
      expect(mocks.get).toHaveBeenCalledWith("/api/extension/resources/ticket", expect.objectContaining({ responseType: "blob", signal: expect.any(AbortSignal) }));
      expect(wrapper.find("img").attributes("src")).toBe("blob:emote-preview");
      await wrapper.setProps({ src: "https://example.com/public.png" });
      expect(revoke).toHaveBeenCalledWith("blob:emote-preview");
      expect(wrapper.find("img").attributes("src")).toBe("https://example.com/public.png");
    } finally {
      wrapper.unmount();
      vi.unstubAllGlobals();
    }
  });

  it("资源接口返回错误正文时显示失败提示", async () => {
    mocks.get.mockResolvedValue({ data: new Blob(["{}"], { type: "application/json" }) });
    const wrapper = mount(SchemaUIImage, { props: { src: "/api/extension/resources/invalid" } });
    try {
      await flushPromises();
      expect(wrapper.text()).toBe("图片加载失败");
      expect(wrapper.find("img").exists()).toBe(false);
    } finally {
      wrapper.unmount();
    }
  });
});
