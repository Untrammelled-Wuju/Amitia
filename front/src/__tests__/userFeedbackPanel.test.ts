// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { flushPromises, mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { beforeEach, describe, expect, it, vi } from "vitest";
import UserFeedbackPanel from "@/views/settings/UserFeedbackPanel.vue";

const requests = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn() }));

vi.mock("@/composables/useApi", () => ({
  useApi: () => ({
    get: requests.get,
    post: requests.post,
    put: requests.put,
    del: vi.fn(),
    postUpload: vi.fn(),
  }),
}));

vi.mock("@/runtime/runtime-adapter", () => ({
  getDeploymentConfig: vi.fn(async () => ({ mode: "cloud", serverURL: "https://core.example.com" })),
}));

vi.mock("@/ui-runtime/deviceIdentity", () => ({
  resolveUIHostDeviceId: vi.fn(async () => "device-test-1"),
}));

function mountPanel() {
  return mount(UserFeedbackPanel, { global: { plugins: [ElementPlus] } });
}

describe("问题反馈设置页", () => {
  beforeEach(() => {
    requests.get.mockReset();
    requests.post.mockReset();
    requests.put.mockReset();
    requests.get.mockImplementation(async (url: string) => {
      if (url === "/api/user-feedback/settings") {
        return {
          uploadUrl: "https://amitia.example.com/api/v1/feedbacks",
          defaultUploadUrl: "https://amitia.example.com/api/v1/feedbacks",
          logLineLimit: 500,
          errorLineLimit: 500,
          maxScreenshotMB: 6,
        };
      }
      if (url === "/api/version") return { version: "26.2.0-beta.2" };
      if (url === "/api/user-feedback/tickets") {
        return { tickets: [{ id: "t1", title: "历史反馈", category: "bug", status: "failed", lastError: "网络不可用", createdAt: "2026-10-16T08:00:00Z" }] };
      }
      return {};
    });
  });

  it("展示可编辑的上报地址并明确标注会上传报错日志", async () => {
    const wrapper = mountPanel();
    await flushPromises();
    const urlInput = wrapper.get("input[placeholder='https://example.com/api/v1/feedbacks']");
    expect((urlInput.element as HTMLInputElement).value).toBe("https://amitia.example.com/api/v1/feedbacks");
    expect(wrapper.text()).toContain("提交时会一起上传本机与插件的运行日志");
    expect(wrapper.text()).toContain("查看将要上传的信息");
    expect(wrapper.text()).toContain("上传失败");
    wrapper.unmount();
  });

  it("按需拉取将要上传的信息并展示预览", async () => {
    requests.post.mockImplementation(async (url: string) => {
      if (url === "/api/user-feedback/diagnostics/preview") {
        return {
          uploadUrl: "https://amitia.example.com/api/v1/feedbacks",
          text: "【提交内容】\n标题：聊天窗口空白\n\n【日志文件】\n是否上传报错日志：是",
          payload: { title: "聊天窗口空白", includeLogs: true },
        };
      }
      return {};
    });
    const wrapper = mountPanel();
    await flushPromises();
    const previewButton = wrapper
      .findAll("button")
      .find((button) => button.text().includes("查看将要上传的信息"));
    expect(previewButton).toBeTruthy();
    await previewButton!.trigger("click");
    await flushPromises();
    await new Promise((resolve) => setTimeout(resolve, 0));
    await flushPromises();
    expect(requests.post).toHaveBeenCalledWith(
      "/api/user-feedback/diagnostics/preview",
      expect.objectContaining({ platform: "web", runtimeMode: "cloud", deviceId: "device-test-1" }),
    );
    const previewText = `${wrapper.html()}${document.body.textContent || ""}`;
    expect(previewText).toContain("标题：聊天窗口空白");
    wrapper.unmount();
  });
});
