// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { flushPromises, mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { beforeEach, describe, expect, it, vi } from "vitest";
import BugReportDialog from "@/components/feedback/BugReportDialog.vue";
import NotFoundView from "@/views/NotFoundView.vue";

const requests = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));

vi.mock("@/composables/useApi", () => ({
  useApi: () => ({
    get: requests.get,
    post: requests.post,
    put: vi.fn(),
    del: vi.fn(),
    postUpload: vi.fn(),
  }),
}));

vi.mock("@/runtime/runtime-capabilities", () => ({
  isDesktopShell: () => false,
}));

vi.mock("@/runtime/runtime-adapter", () => ({
  getDeploymentConfig: vi.fn(async () => ({ mode: "cloud", serverURL: "https://core.example.com" })),
}));

vi.mock("@/ui-runtime/deviceIdentity", () => ({
  resolveUIHostDeviceId: vi.fn(async () => "device-test-1"),
}));

vi.mock("vue-router", () => ({
  useRouter: () => ({ replace: vi.fn() }),
  useRoute: () => ({ fullPath: "/missing-page" }),
}));

function mountDialog() {
  return mount(BugReportDialog, {
    props: { modelValue: true, pagePath: "/missing-page" },
    attachTo: document.body,
    global: { plugins: [ElementPlus] },
  });
}

function buttonByText(text: string) {
  return Array.from(document.querySelectorAll("button")).find((button) => button.textContent?.includes(text));
}

describe("404 报错信息弹窗", () => {
  beforeEach(() => {
    requests.get.mockReset();
    requests.post.mockReset();
    document.body.innerHTML = "";
    requests.get.mockImplementation(async (url: string) => {
      if (url === "/api/user-feedback/settings") {
        return {
          uploadUrl: "https://amitia.example.com/api/v1/feedbacks",
          defaultUploadUrl: "https://amitia.example.com/api/v1/feedbacks",
          logLineLimit: 120,
          errorLineLimit: 40,
          maxScreenshotMB: 6,
        };
      }
      if (url === "/api/version") return { version: "26.2.0-beta.7" };
      return {};
    });
  });

  it("只展示功能异常反馈与报错日志选项", async () => {
    const wrapper = mountDialog();
    await flushPromises();
    const text = document.body.textContent || "";
    expect(text).toContain("反馈类型");
    expect(text).toContain("功能异常");
    expect(text).toContain("上传报错日志文件");
    expect(text).toContain("查看将要上传的信息");
    expect(text).not.toContain("体验问题");
    expect(text).not.toContain("功能建议");
    wrapper.unmount();
  });

  it("提交时固定使用 bug 类型并携带页面上下文", async () => {
    requests.post.mockResolvedValue({});
    const wrapper = mountDialog();
    await flushPromises();
    const title = document.querySelector(".report-form input") as HTMLInputElement;
    const description = document.querySelector(".report-form textarea") as HTMLTextAreaElement;
    title.value = "404 页面无法打开";
    title.dispatchEvent(new Event("input"));
    description.value = "进入不存在的页面后只看到 404";
    description.dispatchEvent(new Event("input"));
    await flushPromises();
    buttonByText("提交报错信息")?.click();
    await flushPromises();
    expect(requests.post).toHaveBeenCalledWith(
      "/api/user-feedback/tickets",
      expect.objectContaining({
        category: "bug",
        title: "404 页面无法打开",
        description: "进入不存在的页面后只看到 404",
        uploadUrl: "https://amitia.example.com/api/v1/feedbacks",
      }),
    );
    wrapper.unmount();
  });

  it("404 页面点击提交报错信息后打开反馈弹窗", async () => {
    const wrapper = mount(NotFoundView, { global: { plugins: [ElementPlus] } });
    const button = wrapper
      .findAll("button")
      .find((item) => item.text().includes("提交报错信息"));
    expect(button).toBeTruthy();
    await button!.trigger("click");
    await flushPromises();
    expect(document.body.textContent || "").toContain("反馈类型");
    expect(document.body.textContent || "").toContain("功能异常");
    wrapper.unmount();
  });
});
