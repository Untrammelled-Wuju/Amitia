import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ref } from "vue";
import { flushPromises, mount } from "@vue/test-utils";
import DeviceOwnedMemoryPanel from "../components/DeviceOwnedMemoryPanel.vue";
import DeviceOwnedDataView from "../components/DeviceOwnedDataView.vue";
import MemoryInjectPanel from "../views/web-chat/components/MemoryInjectPanel.vue";
import ProfileSummaryPanel from "../views/web-chat/components/ProfileSummaryPanel.vue";
import MemoryTimeline from "../views/memory-timeline/MemoryTimeline.vue";

const mocks = vi.hoisted(() => ({ data: vi.fn(), historicalRoles: vi.fn(), edit: vi.fn(), projections: vi.fn(), refresh: vi.fn(), legacy: vi.fn(), profiles: vi.fn(), confirm: vi.fn(), prompt: vi.fn(), deployment: vi.fn(), owned: null as any }));
vi.mock("../composables/useDeviceOwnedConversation", () => ({ useDeviceOwnedConversation: () => mocks.owned }));
vi.mock("vue-echarts", () => ({ default: { name: "VChart", props: ["option"], template: '<div data-graph-canvas />' } }));
vi.mock("../composables/useApi", () => ({ apiClient: {}, useApi: () => ({ get: mocks.legacy, post: mocks.legacy, put: mocks.legacy, del: mocks.legacy }) }));
vi.mock("../composables/useProfile", () => ({ useProfile: () => ({ profiles: ref([]), fetchProfiles: mocks.profiles, categoryLabel: (value: string) => value }) }));
vi.mock("../runtime/runtime-adapter", () => ({ getDeploymentConfig: mocks.deployment }));
vi.mock("element-plus", async () => ({ ...await vi.importActual<any>("element-plus"), ElMessageBox: { confirm: mocks.confirm, prompt: mocks.prompt } }));
const global = { directives: { loading: () => {} }, stubs: {
  DeviceOwnedMemoryManagement: true,
  ElDialog: { props: ["modelValue"], template: '<div v-if="modelValue"><slot /><slot name="footer" /></div>' },
  ElInput: { props: ["modelValue"], emits: ["update:modelValue"], template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />' },
  ElSelect: { props: ["modelValue"], emits: ["update:modelValue"], template: "<select :value='modelValue' @change='$emit(\"update:modelValue\", $event.target.value)'><slot /></select>" },
  ElOption: { props: ["value", "label"], template: "<option :value='value'>{{ label }}</option>" },
  ElAlert: { props: ["title"], template: "<div>{{ title }}<slot /></div>" }, ElButton: { props: ["disabled"], emits: ["click"], template: "<button :disabled='disabled' @click='$emit(\"click\")'><slot /></button>" }, ElTag: { template: "<span><slot /></span>" }, ElEmpty: { props: ["description"], template: "<div>{{ description }}</div>" },
  ElTooltip: { template: "<span><slot /></span>" },
} };

describe("网页六层记忆的数据归属入口", () => {
  let scope: any;
  const wrappers: ReturnType<typeof mount>[] = [];
  const open = (component: any = DeviceOwnedMemoryPanel, props: any = {}) => {
    const wrapper = mount(component, { props: { visible: true, characterId: "role", conversationId: "", embedded: true, ...props }, global });
    wrappers.push(wrapper);
    return wrapper;
  };
  beforeEach(() => {
    Object.values(mocks).filter((value) => typeof value === "function").forEach((mock: any) => mock.mockReset());
    scope = { authorizationRealm: "realm", coreId: "core", initiatorDeviceId: "source", targetDeviceId: "source", roleId: "role", roleOwnerId: "source", resourceOwnerId: "source", roleRevision: 1, providerEpoch: 1, modeRevision: 1, permissionRevision: 1 };
    mocks.owned = { enabled: ref(true), coreId: ref("core"), roles: ref([{ id: "role", name: "角色" }]), coordinated: ref(false), policy: ref({ providerEpoch: 1, modeRevision: 1, permissionRevision: 1 }), selectInitialRole: () => "role", data: mocks.data, historicalRoles: mocks.historicalRoles, edit: mocks.edit, projections: mocks.projections, refresh: mocks.refresh };
    mocks.refresh.mockResolvedValue(true);
    mocks.deployment.mockResolvedValue({ mode: "cloud" });
    mocks.historicalRoles.mockResolvedValue([{ id: "old-role", name: "旧角色" }]);
    mocks.data.mockImplementation(async (kind, role) => ({ executionScope: { ...scope, roleId: role }, snapshot: { ownerId: scope.resourceOwnerId, resources: [{ kind, id: "same-id", roleId: role, ownerId: scope.resourceOwnerId, revision: 1, body: { key: "记录", content: "原始内容", allowContextUse: true } }] } }));
    mocks.confirm.mockResolvedValue(true);
  });
  afterEach(() => { wrappers.splice(0).forEach((wrapper) => wrapper.unmount()); vi.useRealTimers(); });

  it.each(["memory", "episodic", "graph", "profile"])("%s 状态轮询结果不变时保留记录且不重新请求数据", async (kind) => {
    vi.useFakeTimers();
    mocks.projections.mockResolvedValue({ executionScope: scope, status: { layers: [] } });
    mocks.refresh.mockImplementation(async () => {
      mocks.owned.policy.value = { ...mocks.owned.policy.value };
      return true;
    });
    const wrapper = open(DeviceOwnedMemoryPanel, { initialKind: kind });
    await flushPromises();
    expect(mocks.data).toHaveBeenCalledTimes(1);
    for (let index = 0; index < 5; index++) {
      await vi.advanceTimersByTimeAsync(3000);
      await flushPromises();
      if (kind === "graph") expect(wrapper.find("[data-graph-canvas]").exists()).toBe(true);
      else expect(wrapper.text()).toContain("原始内容");
      expect(mocks.data).toHaveBeenCalledTimes(1);
    }
    await wrapper.findAll("button").find((button) => button.text() === "刷新")!.trigger("click");
    await flushPromises();
    expect(mocks.data).toHaveBeenCalledTimes(2);
  });

  it("相同状态不丢弃正在加载的数据，权限实际变化后重新加载", async () => {
    let finish!: (result: any) => void;
    mocks.data.mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
    const wrapper = open();
    await flushPromises();
    mocks.owned.policy.value = { ...mocks.owned.policy.value };
    await flushPromises();
    expect(mocks.data).toHaveBeenCalledTimes(1);
    finish({ executionScope: scope, snapshot: { ownerId: "source", resources: [{ kind: "memory", id: "pending", roleId: "role", ownerId: "source", revision: 1, body: { content: "首次加载完成" } }] } });
    await flushPromises();
    expect(wrapper.text()).toContain("首次加载完成");
    scope.permissionRevision = 2;
    mocks.owned.policy.value = { ...mocks.owned.policy.value, permissionRevision: 2 };
    await flushPromises();
    expect(mocks.data).toHaveBeenCalledTimes(2);
    expect(wrapper.text()).not.toContain("首次加载完成");
    expect(wrapper.text()).toContain("原始内容");
  });

  it("服务断线恢复且权限版本未变化时重新加载一次，后续轮询保留数据", async () => {
    vi.useFakeTimers();
    const wrapper = open();
    await flushPromises();
    mocks.refresh.mockRejectedValueOnce(new Error("offline"));
    await vi.advanceTimersByTimeAsync(3000);
    await flushPromises();
    expect(wrapper.text()).not.toContain("原始内容");
    expect(mocks.data).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(3000);
    await flushPromises();
    expect(wrapper.text()).toContain("原始内容");
    expect(mocks.data).toHaveBeenCalledTimes(2);
    await vi.advanceTimersByTimeAsync(6000);
    await flushPromises();
    expect(mocks.data).toHaveBeenCalledTimes(2);
  });

  it("当前 Owner 的记忆操作保留加载时 scope、角色和 CAS", async () => {
    const wrapper = open();
    await flushPromises();
    await wrapper.findAll("button").find((button) => button.text() === "停用")!.trigger("click");
    expect(mocks.edit).toHaveBeenCalledWith("memory", "same-id", { allowContextUse: false }, { characterId: "role", expectedExecutionScope: scope, expectedOwnerId: "source", expectedRevision: 1 });
  });
  it("独立页面只有一个范围入口，默认全部且可选择具体角色", async () => {
    mocks.owned.roles.value = [{ id: "role", name: "角色" }, { id: "other", name: "第二角色" }];
    const wrapper = open(DeviceOwnedDataView, { title: "记忆总览", initialKind: "memory" });
    await flushPromises();
    const selector = wrapper.find('select[aria-label="记忆查看范围"]');
    expect(wrapper.findAll('select[aria-label="记忆查看范围"]')).toHaveLength(1);
    expect(wrapper.find('select[aria-label="工作角色"]').exists()).toBe(false);
    expect((selector.element as HTMLSelectElement).value).toBe("all");
    await selector.setValue("role:other"); await flushPromises();
    expect(mocks.data.mock.calls.at(-1)).toEqual(["memory", "other", "", "", { legacyCursor: "", memoryView: "role:other" }]);
  });
  it("用户画像全局共享，不提供角色或范围选择", async () => {
    const wrapper = open(DeviceOwnedDataView, { title: "用户画像", initialKind: "profile" }); await flushPromises();
    expect(wrapper.find('select[aria-label="记忆查看范围"]').exists()).toBe(false);
    expect(mocks.data).toHaveBeenCalledWith("profile", "role", "", "", { legacyCursor: "", memoryView: "global" });
  });
  it("旧格式图谱仍在同一画布显示，并保留源记忆以供分区迁移", async () => {
    mocks.data.mockResolvedValue({executionScope:{...scope,memoryRoleId:'@global-memory',memoryViewAll:true},snapshot:{ownerId:'source',resources:[],legacyGraphs:[{kind:'graph',id:'legacy/old',ownerId:'source',roleId:'role',revision:0,body:{key:'旧关系',legacyMemory:{id:'old',characterId:'role',scope:'character'},content:{nodes:[{id:'a',label:'用户'},{id:'b',label:'茶'}],edges:[{source:'a',target:'b',relation:'喜欢'}]}}}]}});
    const wrapper=open(DeviceOwnedDataView,{title:'记忆图谱',initialKind:'graph'});await flushPromises();
    expect(wrapper.find('[data-graph-canvas]').exists()).toBe(true);
    expect(wrapper.text()).toContain('旧关系');
    expect(wrapper.findAll('button').some(button=>button.text()==='调整所属分区')).toBe(true);
  });
  it.each(["working", "profile", "episodic", "fact", "vector", "graph", "summary"])("%s 层只读取当前角色 Owned 数据通道", async (kind) => {
    mocks.projections.mockResolvedValue({ executionScope: scope, status: { layers: [] } });
    const wrapper = open(DeviceOwnedMemoryPanel, { initialKind: kind });
    await flushPromises();
    expect(mocks.data).toHaveBeenCalledWith(kind, "role", "", "", { legacyCursor: "" });
    expect(mocks.legacy).not.toHaveBeenCalled();
    expect(wrapper.findAll("button").some((button) => button.text() === "删除")).toBe(kind === "summary");
  });
  it.each(["memory", "summary"])("%s 内容编辑期间换角色仍保留原 row 的 Owner 与 CAS", async (kind) => {
    let finish!: (result: any) => void;
    mocks.prompt.mockImplementation(() => new Promise((resolve) => { finish = resolve; }));
    mocks.data.mockImplementation(async (_kind, role) => ({ executionScope: { ...scope, roleId: role }, snapshot: { ownerId: scope.resourceOwnerId, resources: [{ kind, id: "same-id", roleId: role, ownerId: scope.resourceOwnerId, revision: 2, body: { content: kind === "summary" ? { summary: "旧摘要", title: "元数据" } : { value: "旧记忆", category: "偏好" } } }] } }));
    const wrapper = open(DeviceOwnedMemoryPanel, { initialKind: kind }); await flushPromises();
    await wrapper.findAll("button").find((button) => button.text() === "编辑")!.trigger("click");
    await wrapper.setProps({ characterId: "new-role" });
    finish({ value: "修改内容" }); await flushPromises();
    expect(mocks.edit.mock.calls[0]).toEqual([kind, "same-id", { content: kind === "summary" ? { summary: "修改内容", title: "元数据" } : { value: "修改内容", category: "偏好" } }, { characterId: "role", expectedExecutionScope: scope, expectedOwnerId: "source", expectedRevision: 2 }]);
  });
  it("到期与归档写入原始记忆，并携带原权限及版本", async () => {
    mocks.prompt.mockResolvedValue({ value: "2026-10-08T18:00:00+08:00" });
    const wrapper = open(); await flushPromises();
    await wrapper.findAll("button").find((button) => button.text() === "到期时间")!.trigger("click"); await flushPromises();
    expect(mocks.edit.mock.calls[0]).toEqual(["memory", "same-id", { expiresAt: "2026-10-08T10:00:00.000Z" }, { characterId: "role", expectedExecutionScope: scope, expectedOwnerId: "source", expectedRevision: 1 }]);
    await wrapper.findAll("button").find((button) => button.text() === "归档")!.trigger("click"); await flushPromises();
    expect(mocks.edit.mock.calls[1][2].archivedAt).toMatch(/^\d{4}-/);
    expect(mocks.edit.mock.calls[1][3]).toMatchObject({ expectedExecutionScope: scope, expectedRevision: 1 });
  });
  it("删除确认期间切换角色仍提交原意图，不能改为新角色同 ID", async () => {
    let confirm!: () => void;
    mocks.confirm.mockImplementation(() => new Promise<void>((resolve) => { confirm = resolve; }));
    const wrapper = open();
    await flushPromises();
    await wrapper.findAll("button").find((button) => button.text() === "删除")!.trigger("click");
    await wrapper.setProps({ characterId: "another-role" });
    confirm();
    await flushPromises();
    expect(mocks.edit.mock.calls[0][3]).toMatchObject({ characterId: "role", expectedExecutionScope: { roleId: "role" }, expectedOwnerId: "source", expectedRevision: 1 });
  });
  it("Core 切换清空旧记录，拒绝迟到的历史角色目录", async () => {
    mocks.owned.coordinated.value = true;
    let oldCatalog!: (roles: any[]) => void;
    mocks.historicalRoles.mockImplementationOnce(() => new Promise((resolve) => { oldCatalog = resolve; }));
    const wrapper = open();
    await flushPromises();
    mocks.owned.coreId.value = "core-c";
    scope.coreId = "core-c";
    scope.resourceOwnerId = "core-c";
    await flushPromises();
    oldCatalog([{ id: "old-b-role", name: "旧B独有角色" }]);
    await flushPromises();
    expect(wrapper.text()).not.toContain("旧B独有角色");
    expect(wrapper.text()).toContain("core-c");
  });
  it("不同 Owner 的同 ID 记录不能伪装为当前记忆", async () => {
    mocks.data.mockResolvedValue({ executionScope: scope, snapshot: { ownerId: "foreign", resources: [{ kind: "memory", id: "same-id", ownerId: "foreign", roleId: "role", revision: 1, body: { content: "异地数据" } }] } });
    const wrapper = open();
    await flushPromises();
    expect(wrapper.text()).toContain("数据所有者已变化");
    expect(wrapper.text()).not.toContain("异地数据");
  });
  it.each([MemoryInjectPanel, ProfileSummaryPanel])("聊天嵌入面板不调用 legacy 记忆或画像 API", async (component) => {
    const wrapper = open(component, { convId: "chat" });
    await flushPromises();
    expect(mocks.data).toHaveBeenCalled();
    expect(mocks.legacy).not.toHaveBeenCalled();
    expect(mocks.profiles).not.toHaveBeenCalled();
    await wrapper.find("button[aria-label]").trigger("click");
    expect(wrapper.emitted("close")).toHaveLength(1);
  });
  it("记忆时间线使用同一 Owner 查询，cloud 服务未就绪时不落入 Legacy", async () => {
    const timeline = mount(MemoryTimeline, { global });
    wrappers.push(timeline);
    await flushPromises();
    expect(timeline.find('[aria-label="记忆层"]').exists()).toBe(false);
    expect(timeline.text()).not.toContain("记忆分类");
    expect(timeline.findAll('select[aria-label="记忆查看范围"]')).toHaveLength(1);
    expect((timeline.find('select[aria-label="记忆查看范围"]').element as HTMLSelectElement).value).toBe("all");
    expect(mocks.data).toHaveBeenCalledWith("memory", "role", "", "", { legacyCursor: "", memoryView: "all" });
    expect(mocks.legacy).not.toHaveBeenCalled();
    timeline.unmount();
    wrappers.pop();
    mocks.owned.enabled.value = false;
    mocks.refresh.mockResolvedValue(false);
    const blocked = mount(DeviceOwnedDataView, { props: { title: "记忆", initialKind: "memory" }, slots: { default: "不应显示的本地页面" }, global });
    wrappers.push(blocked);
    await flushPromises();
    expect(blocked.text()).toContain("尚未就绪");
    expect(blocked.text()).not.toContain("不应显示的本地页面");
  });
});
