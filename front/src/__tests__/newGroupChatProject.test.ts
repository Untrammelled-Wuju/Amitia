import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { defineComponent } from "vue";
import NewGroupChatDialog from "@/components/NewGroupChatDialog.vue";
import type { OwnedGroupState } from "@/runtime/device-owned-chat";

const mocks = vi.hoisted(() => ({
  projects: vi.fn(),
  createGroupConversation: vi.fn(),
}));

vi.mock("@/composables/useDeviceOwnedConversation", () => ({
  useDeviceOwnedConversation: () => mocks,
}));

const project = {
  id: "project-a",
  title: "项目 A",
  workspaces: [
    {
      id: "workspace-a",
      deviceId: "device-a",
      name: "主目录",
      kind: "local",
      rootUri: "amitia://workspace/@workspace-a/",
    },
  ],
  primaryWorkspaceId: "workspace-a",
  revision: 1,
  ownerId: "device-a",
  roleId: "role-a",
  readOnly: false,
  createdAt: "",
  updatedAt: "",
  executionScope: {},
};

function mountDialog() {
  return mount(NewGroupChatDialog, {
    props: {
      visible: false,
      characters: [
        { id: "role-a", name: "角色 A" },
        { id: "role-b", name: "角色 B" },
      ],
      defaultRoleId: "role-a",
    },
    global: {
      stubs: {
        ElDialog: defineComponent({
          props: { modelValue: Boolean },
          template: "<section><slot /><slot name=\"footer\" /></section>",
        }),
        ElSelect: defineComponent({
          props: {
            modelValue: String,
            disabled: Boolean,
            loading: Boolean,
            class: String,
          },
          emits: ["update:modelValue"],
          template:
            "<select :data-amitia-ui=\"$attrs['data-amitia-ui']\" :value=\"modelValue\" :disabled=\"disabled\" @change=\"$emit('update:modelValue', $event.target.value)\"><slot /></select>",
        }),
        ElOption: defineComponent({
          props: { value: String, label: String },
          template: "<option :value=\"value\">{{ label }}</option>",
        }),
        ElButton: defineComponent({
          props: { disabled: Boolean, loading: Boolean },
          emits: ["click"],
          template: "<button :disabled=\"disabled\" @click=\"$emit('click', $event)\"><slot /></button>",
        }),
        ElCheckbox: defineComponent({
          props: { modelValue: Boolean },
          emits: ["update:modelValue"],
          template:
            "<label><input type=\"checkbox\" :checked=\"modelValue\" @change=\"$emit('update:modelValue', $event.target.checked)\" /><slot /></label>",
        }),
        ElAvatar: defineComponent({
          template: "<span><slot /></span>",
        }),
        ElIcon: defineComponent({
          template: "<span><slot /></span>",
        }),
      },
    },
  });
}

async function openDialog(wrapper: VueWrapper) {
  await wrapper.setProps({ visible: true });
  await flushPromises();
}

async function selectRole(wrapper: VueWrapper, label: string) {
  const button = wrapper
    .findAll("button.role-option")
    .filter((item) => item.text().includes(label))
    .at(-1);
  expect(button).toBeTruthy();
  await button!.trigger("click");
  await flushPromises();
  expect(button!.classes()).toContain("is-selected");
}

async function selectBaseRole(wrapper: VueWrapper, label: string) {
  const button = wrapper
    .findAll("button.role-option")
    .find((item) => item.text().includes(label));
  expect(button).toBeTruthy();
  await button!.trigger("click");
  await flushPromises();
}

async function selectProject(wrapper: VueWrapper, value: string) {
  const select = wrapper.find('[data-amitia-ui="new-group.project"]');
  expect(select.exists()).toBe(true);
  await select.setValue(value);
  await flushPromises();
}

describe("桌面端新建群聊项目选择", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    document.body.innerHTML = "";
    mocks.projects.mockResolvedValue({
      projects: [project],
      historicalProjects: [],
      historicalRoles: [],
      selectedHistoricalRole: "",
      executionScope: {},
    });
    mocks.createGroupConversation.mockResolvedValue({
      conversationId: "conversation-group",
      conversationRoleId: "role-a",
      roleOwnerId: "device-a",
      title: "新群聊",
      conversationType: "group",
      membershipRevision: 1,
      settingsRevision: 1,
      lastMessageSequence: 0,
      groupSettings: {
        allowCharacterMentions: false,
        maxAutoReplies: 6,
      },
      members: [],
    } satisfies OwnedGroupState);
  });

  it("不选择项目时按默认消息创建群聊", async () => {
    const wrapper = mountDialog();
    await openDialog(wrapper);
    expect(mocks.projects).toHaveBeenCalledWith("role-a");
    expect(wrapper.text()).toContain("不加入项目（默认消息）");

    await selectRole(wrapper, "角色 B");
    await flushPromises();
    await (wrapper.vm as unknown as { create: () => Promise<void> }).create();
    await flushPromises();

    expect(mocks.createGroupConversation).toHaveBeenCalledWith(
      "role-a",
      [{ characterId: "role-b", shareHistory: false }],
      "",
    );
    wrapper.unmount();
  });

  it("选择项目时把项目 ID 传给群聊创建接口", async () => {
    const wrapper = mountDialog();
    await openDialog(wrapper);
    await selectRole(wrapper, "角色 B");
    await selectProject(wrapper, "project-a");
    await flushPromises();
    await (wrapper.vm as unknown as { create: () => Promise<void> }).create();
    await flushPromises();

    expect(mocks.createGroupConversation).toHaveBeenCalledWith(
      "role-a",
      [{ characterId: "role-b", shareHistory: false }],
      "project-a",
    );
    wrapper.unmount();
  });

  it("切换默认发言角色后重新加载该角色的项目", async () => {
    const wrapper = mountDialog();
    await openDialog(wrapper);
    mocks.projects.mockResolvedValueOnce({
      projects: [],
      historicalProjects: [],
      historicalRoles: [],
      selectedHistoricalRole: "",
      executionScope: {},
    });

    await selectBaseRole(wrapper, "角色 B");
    await flushPromises();

    expect(mocks.projects).toHaveBeenLastCalledWith("role-b");
    wrapper.unmount();
  });
});
