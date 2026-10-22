import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ElementPlus from "element-plus";
import Workspace from "../views/reminders/Reminders.vue";

const api = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() }));
vi.mock("../composables/request", () => ({ request: api }));
vi.mock("../composables/useDeviceOwnedConversation", async () => {
  const { ref } = await import("vue");
  return {
    useDeviceOwnedConversation: () => ({
      coreId: ref("core"),
      roles: ref([{ id: "role", name: "小夏" }]),
      refresh: vi.fn().mockResolvedValue(true),
      conversations: vi.fn().mockResolvedValue([]),
    }),
  };
});

let wrapper: VueWrapper;
const task = {
  saved: true,
  id: "task-1",
  generation: 4,
  ownerId: "core",
  executionMode: "local",
  source: "manual",
  state: "enabled",
  title: "每晚晚安",
  content: "记得早点休息",
  enabled: 1,
  paused: false,
  characterId: "role",
  timezone: "Asia/Shanghai",
  localRemindAt: "2099-01-01 22:00:00",
  localAnchorAt: "2099-01-01 22:00:00",
  remindAt: "2099-01-01T14:00:00Z",
  repeatRule: "daily",
  recurrence: { type: "daily" },
  channelTargets: [],
  systemNotification: true,
  endAt: "",
  maxOccurrences: 0,
};

beforeEach(() => {
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  );
  api.get.mockReset();
  api.get.mockImplementation(async (path: string) => {
    if (path === "/api/reminders") return [task];
    if (path === "/api/reminders/status")
      return { schedulerRunning: true, total: 1, enabled: 1, dueNow: 0 };
    if (path === "/api/reminders/cleanup-config") return { cleanupDays: "0" };
    if (path === "/api/reminders/channels") return ["web"];
    throw new Error(path);
  });
});
afterEach(() => {
  wrapper?.unmount();
  document.body.innerHTML = "";
  vi.unstubAllGlobals();
});

async function openWorkspace() {
  wrapper = mount(Workspace, {
    attachTo: document.body,
    global: {
      plugins: [ElementPlus],
      stubs: { ProspectivePanel: true, TriggerHistoryPanel: true },
    },
  });
  await flushPromises();
}

describe("主动任务工作台", () => {
  it("加载失败提供重试，不将失败显示成空任务列表", async () => {
    const read = api.get.getMockImplementation()!;
    api.get.mockImplementation(async (path: string, ...args: unknown[]) => {
      if (path === "/api/reminders") throw new Error("连接暂不可用");
      return read(path, ...args);
    });
    await openWorkspace();
    expect(wrapper.text()).toContain("暂时无法读取任务");
    expect(wrapper.text()).not.toContain("为下一次陪伴安排时间");
  });

  it("真实任务可搜索并打开详情，版本确认后的记录保留全部发送设置", async () => {
    await openWorkspace();
    expect(wrapper.get("h1").text()).toBe("主动任务");
    expect(wrapper.get(".task-title").text()).toBe("每晚晚安");
    const search = wrapper.get('input[aria-label="搜索任务"]');
    await search.setValue("不存在");
    expect(wrapper.find(".task-title").exists()).toBe(false);
    await search.setValue("小夏");
    await wrapper.get(".task-title").trigger("click");
    await flushPromises();
    expect(document.body.textContent).toContain("任务详情");
    expect(document.body.textContent).toContain("Asia/Shanghai");
    expect(document.body.textContent).toContain("系统通知");
    expect(api.get).toHaveBeenCalledWith("/api/reminders", { params: { kind: "user_reminder" } });
  });

  it("创建表单分组展示，结束条件可展开，取消编辑不会创建任务", async () => {
    api.post.mockClear();
    await openWorkspace();
    const create = wrapper.findAll("button").find((button) => button.text() === "新建任务");
    await create!.trigger("click");
    await flushPromises();
    const dialog = document.querySelector(".task-editor")!;
    expect(dialog.textContent).toContain("任务内容");
    expect(dialog.textContent).toContain("执行时间");
    expect(dialog.textContent).toContain("发送目标");
    (dialog.querySelector(".el-collapse-item__header") as HTMLElement).click();
    await flushPromises();
    expect(dialog.querySelector(".el-collapse-item")?.classList.contains("is-active")).toBe(true);
    const cancel = [...dialog.querySelectorAll("button")].find(
      (button) => button.textContent?.trim() === "取消",
    );
    cancel!.click();
    await flushPromises();
    expect(api.post).not.toHaveBeenCalled();
  });

  it("主动消息规则合并到主动任务工作台", async () => {
    await openWorkspace();
    const proactive = wrapper
      .findAll(".el-radio-button")
      .find((item) => item.text().includes("主动消息"));
    expect(proactive).toBeTruthy();
    await proactive!.trigger("click");
    await flushPromises();
    expect(api.get).toHaveBeenCalledWith("/api/reminders", {
      params: { kind: "proactive_task" },
    });
    const create = wrapper.findAll("button").find((button) => button.text() === "新建任务");
    await create!.trigger("click");
    await flushPromises();
    const dialog = document.querySelector(".task-editor")!;
    expect(dialog.textContent).toContain("Cron 表达式");
    expect(dialog.textContent).toContain("未回复时自动降频");
  });
});
