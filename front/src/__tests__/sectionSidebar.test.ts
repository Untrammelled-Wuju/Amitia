import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory, createRouter } from "vue-router";
import ElementPlus from "element-plus";
import { describe, expect, it, vi } from "vitest";
import SectionSidebar from "../components/SectionSidebar.vue";

const mocks = vi.hoisted(() => ({ get: vi.fn() }));
vi.mock("../composables/useApi", () => ({
  useApi: () => ({ get: mocks.get }),
}));

async function render(sectionId: "character" | "memory" | "tasks" | "continuity" | "game" | "workshop" | "extensions") {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: "/game-plugin", name: "gamePlugin", component: { template: "<div />" } },
      { path: "/:pathMatch(.*)*", component: { template: "<div />" } },
    ],
  });
  await router.push("/chat");
  await router.isReady();
  const wrapper = mount(SectionSidebar, {
    props: { sectionId },
    global: { plugins: [router, ElementPlus] },
  });
  await flushPromises();
  return { wrapper, router };
}

describe("一级导航对应的第二栏", () => {
  it("角色和记忆显示各自的二级进入入口", async () => {
    const character = await render("character");
    expect(character.wrapper.findAll(".section-entry strong").map(node => node.text())).toEqual([
      "角色管理",
      "世界书",
    ]);
    character.wrapper.unmount();

    const memory = await render("memory");
    expect(memory.wrapper.findAll(".section-entry strong").map(node => node.text())).toEqual([
      "知识库",
      "记忆总览",
      "情景记忆",
      "记忆图谱",
      "时间线",
      "用户画像",
    ]);
    memory.wrapper.unmount();
  });

  it("主动任务和持续事项仅提供新建入口与不可点击的已有标题", async () => {
    const tasks = await render("tasks");
    expect(tasks.wrapper.get(".subsection-heading").text()).toBe("已有任务");
    expect(tasks.wrapper.find(".section-action-list button:not(.primary)").exists()).toBe(false);
    await tasks.wrapper.get("button.section-action.primary").trigger("click");
    await vi.waitFor(() => expect(tasks.router.currentRoute.value.fullPath).toBe("/reminders?create=1"));
    tasks.wrapper.unmount();

    const continuity = await render("continuity");
    expect(continuity.wrapper.get(".subsection-heading").text()).toBe("已有事项");
    expect(continuity.wrapper.find(".section-action-list button:not(.primary)").exists()).toBe(false);
    await continuity.wrapper.get("button.section-action.primary").trigger("click");
    await vi.waitFor(() => expect(continuity.router.currentRoute.value.fullPath).toBe("/continuity?create=1"));
    continuity.wrapper.unmount();
  });

  it("游戏模式显示添加入口与已安装插件列表", async () => {
    mocks.get.mockResolvedValue({
      items: [{
        extensionId: "com.example/game",
        pluginId: "game-plugin",
        name: "示例游戏",
        version: "1.0.0",
        enabled: true,
      }],
    });
    const game = await render("game");
    expect(mocks.get).toHaveBeenCalledWith("/api/game-center/plugins", { page: 1, pageSize: 100 });
    expect(game.wrapper.findAll(".game-plugin-row strong").map(node => node.text())).toEqual(["示例游戏"]);
    await game.wrapper.get(".game-plugin-row").trigger("click");
    await vi.waitFor(() => expect(game.router.currentRoute.value).toMatchObject({
      name: "gamePlugin",
      query: { pluginId: "game-plugin", extensionId: "com.example/game" },
    }));
    await game.router.push("/chat");
    await flushPromises();
    await game.wrapper.findAll("button.section-action.primary")[0].trigger("click");
    await vi.waitFor(() => expect(game.router.currentRoute.value.fullPath).toBe("/game-center?install=1"));
    game.wrapper.unmount();
  });
});
