import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory, createRouter } from "vue-router";
import ElementPlus from "element-plus";
import { beforeEach, describe, expect, it, vi } from "vitest";
import CharacterTeamsView from "../views/character/CharacterTeamsView.vue";
import characterTeamsSource from "../views/character/CharacterTeamsView.vue?raw";

const mocks = vi.hoisted(() => ({
  listTeams: vi.fn(),
  listRoles: vi.fn(),
  listTemplates: vi.fn(),
  getTeam: vi.fn(),
}));

vi.mock("../api/role-runtime", async (importOriginal) => ({
  ...await importOriginal<typeof import("../api/role-runtime")>(),
  listRoleTeams: mocks.listTeams,
  listRoleRuntimeProfiles: mocks.listRoles,
  listRoleTeamTemplates: mocks.listTemplates,
  getRoleTeam: mocks.getTeam,
}));

describe("角色二级团队页", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.listTeams.mockResolvedValue([
      {
        id: "team-1",
        name: "研究团队",
        description: "共享研究上下文",
        status: "active",
        revision: 2,
        sharedMemory: true,
        sharedWorkspace: true,
      },
    ]);
    mocks.listRoles.mockResolvedValue([
      { id: "role-1", name: "资料研究员", status: "active" },
    ]);
    mocks.listTemplates.mockResolvedValue([]);
    mocks.getTeam.mockResolvedValue({
      team: {
        id: "team-1",
        name: "研究团队",
        description: "共享研究上下文",
        status: "active",
        revision: 2,
        sharedMemory: true,
        sharedWorkspace: true,
      },
      members: [
        {
          id: "member-1",
          memberType: "agent",
          memberId: "role-1",
          role: "lead",
          responderMode: "lead",
        },
      ],
      contexts: [
        { id: "context-1", kind: "decision", content: "结论必须附来源" },
      ],
      handoffs: [],
    });
  });

  it("显示团队、负责人、共享上下文和预算入口", async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: "/characters/teams", component: CharacterTeamsView }],
    });
    await router.push("/characters/teams");
    await router.isReady();
    const wrapper = mount(CharacterTeamsView, {
      global: {
        plugins: [router, ElementPlus],
        stubs: { ExtensionPageHeader: true },
      },
    });
    await flushPromises();

    expect(wrapper.text()).toContain("研究团队");
    expect(wrapper.text()).toContain("资料研究员");
    expect(wrapper.text()).toContain("结论必须附来源");
    expect(characterTeamsSource).toContain('label="团队成员"');
    expect(characterTeamsSource).toContain("选择一个或多个角色成员");
    expect(mocks.getTeam).toHaveBeenCalledWith("team-1");
    wrapper.unmount();
  });
});
