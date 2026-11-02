import { flushPromises, mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { afterEach, describe, expect, it, vi } from "vitest";
import UpdateDialog from "@/components/UpdateDialog.vue";

vi.mock("@/composables/useBrandLogo", () => ({
  useBrandLogo: () => ({ logoUrl: "/test-logo.svg" }),
}));

function updateListener() {
  return vi.fn(() => () => {});
}

describe("UpdateDialog", () => {
  afterEach(() => {
    delete (window as any).amitiaDesktop;
    document.body.innerHTML = "";
    vi.restoreAllMocks();
  });

  it("renders the configured update title, highlights and sanitized markdown", async () => {
    (window as any).amitiaDesktop = {
      getPendingUpdate: vi.fn().mockResolvedValue({
        currentVersion: "26.2.0-beta.19",
        version: "26.2.0-beta.20",
        releaseDate: "2026-11-02T00:00:00Z",
        size: 1024,
        channel: "stable",
        updateTitle: "本次更新集中完善更新体验",
        highlights: ["新增本地模型库", "完善后台下载"],
        releaseNotes: "## 主要更新\n\n- **新增**后台下载",
      }),
      onUpdateAvailable: updateListener(),
      onUpdateDownloadProgress: updateListener(),
      onUpdateDownloaded: updateListener(),
      onUpdateError: updateListener(),
      onUpdateNotAvailable: updateListener(),
      startDownload: vi.fn().mockResolvedValue(undefined),
      cancelAndEnter: vi.fn().mockResolvedValue(undefined),
      skipVersion: vi.fn().mockResolvedValue(undefined),
      restartNow: vi.fn().mockResolvedValue(undefined),
    };

    const wrapper = mount(UpdateDialog, {
      attachTo: document.body,
      global: { plugins: [ElementPlus] },
    });
    await flushPromises();

    const text = document.body.textContent || "";
    expect(text).toContain("本次更新集中完善更新体验");
    expect(text).toContain("新增本地模型库");
    expect(text).toContain("完善后台下载");
    expect(text).toContain("完整更新内容");
    expect(document.querySelector(".release-markdown strong")?.textContent).toBe("新增");
    expect(document.querySelectorAll(".new-version br")).toHaveLength(1);
    expect(document.querySelector(".new-version")?.textContent).toContain(
      "26.2.0beta.20",
    );
    expect(text).toContain("2026-11-02");

    wrapper.unmount();
  });
});
