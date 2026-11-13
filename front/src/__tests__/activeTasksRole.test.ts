import { shallowMount } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ref } from "vue";
import ActiveTasksView from "../views/tasks/ActiveTasksView.vue";

const api = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
const refresh = vi.hoisted(() => vi.fn());
const roles = ref<Array<{ id: string; name: string }>>([]);
const coreId = ref("core-a");

interface TaskViewState {
  form: { title: string; remindAt: string; characterId: string };
  openCreate(): Promise<void>;
  createTask(): Promise<void>;
}

vi.mock("@/composables/useApi", () => ({ apiClient: api }));
vi.mock("@/composables/useDeviceOwnedConversation", () => ({
  useDeviceOwnedConversation: () => ({ roles, coreId, refresh }),
}));

function openView() {
  const wrapper = shallowMount(ActiveTasksView, {
    global: {
      directives: { loading: {} },
      stubs: { ElDialog: true, ElForm: true, ElFormItem: true, ElInput: true, ElSelect: true, ElOption: true, ElButton: true, ElEmpty: true },
    },
  });
  const instance = wrapper.vm.$ as unknown as { setupState: TaskViewState };
  return { wrapper, view: instance.setupState };
}

describe("桌面主动任务所属角色", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    coreId.value = "core-a";
    roles.value = [{ id: "role-a", name: "测试角色" }];
    api.get.mockResolvedValue({ data: { data: [] } });
    api.post.mockResolvedValue({ data: { data: {} } });
    refresh.mockResolvedValue(true);
  });

  it("预选唯一角色并提交角色 ID", async () => {
    const { wrapper, view } = openView();
    await view.openCreate();
    view.form.title = "提醒";
    view.form.remindAt = "2030-12-31T20:00";
    await view.createTask();
    expect(api.post).toHaveBeenCalledWith("/api/reminders", expect.objectContaining({
      title: "提醒", characterId: "role-a", kind: "user_reminder",
    }));
    wrapper.unmount();
  });

  it("有多个角色时要求选择，并拒绝在切换统筹中心后提交旧角色", async () => {
    roles.value.push({ id: "role-b", name: "第二角色" });
    const { wrapper, view } = openView();
    await view.openCreate();
    expect(view.form.characterId).toBe("");
    view.form.title = "提醒";
    view.form.remindAt = "2030-12-31T20:00";
    await view.createTask();
    view.form.characterId = "role-b";
    coreId.value = "core-b";
    await view.createTask();
    expect(api.post).not.toHaveBeenCalled();
    wrapper.unmount();
  });
});
