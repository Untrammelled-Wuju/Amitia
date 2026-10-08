import { beforeEach, describe, expect, it, vi } from "vitest";
import { defineComponent } from "vue";
import { mount } from "@vue/test-utils";
import { useCharacterConfig } from "../views/character-config/composables/useCharacterConfig";

const mocks = vi.hoisted(() => ({
  get: vi.fn(), post: vi.fn(), put: vi.fn(), del: vi.fn(),
  success: vi.fn(), error: vi.fn(), warning: vi.fn(),
}));
vi.mock("../composables/useApi", () => ({ useApi: () => mocks }));
vi.mock("element-plus", () => ({
  ElMessage: { success: mocks.success, error: mocks.error, warning: mocks.warning },
  ElMessageBox: { confirm: vi.fn().mockResolvedValue(undefined) },
}));
vi.mock("../runtime/runtime-adapter", () => ({
  getDeploymentConfig: vi.fn().mockResolvedValue({ mode: "cloud" }),
  resolveApiUrl: vi.fn(),
}));
vi.mock("../runtime/request-auth", () => ({ createAuthenticatedFetchInit: vi.fn() }));

describe("角色编辑数据归属", () => {
  const source = "a".repeat(64);
  const core = "b".repeat(64);
  let owner: string;
  let edited: ReturnType<typeof useCharacterConfig>;
  const row = () => ({ id: "same-role", name: "旧角色", roleAuthority: owner });
  const open = async () => {
    const wrapper = mount(defineComponent({ setup() { edited = useCharacterConfig(); return () => null; } }));
    await edited.fetchChars();
    if (edited.characters.value[0]) edited.selectChar(edited.characters.value[0]);
    return wrapper;
  };
  beforeEach(() => {
    Object.values(mocks).forEach((mock) => mock.mockReset());
    owner = source;
    mocks.get.mockImplementation(async (path: string) => {
      if (path.endsWith("/coordination/me")) return { policy: { coordinated: owner === core }, canAdminister: true };
      if (path.endsWith("/authority")) return { roleAuthority: owner };
      return [row()];
    });
  });

  it("切换 Core 后保持原编辑归属并拒绝同 ID 角色保存", async () => {
    const wrapper = await open();
    owner = core;
    await edited.fetchChars();
    mocks.put.mockImplementation(async (_path, _data, config) => {
      if (config.headers["X-Amitia-Role-Authority"] !== owner) throw new Error("角色数据归属已变化，请重新加载");
    });
    await edited.saveChar();
    expect(mocks.put).toHaveBeenCalledTimes(1);
    expect(mocks.put.mock.calls[0][2].headers["X-Amitia-Role-Authority"]).toBe(source);
    expect(mocks.success).not.toHaveBeenCalled();
    expect(mocks.error).toHaveBeenCalledWith("角色数据归属已变化，请重新加载");
    wrapper.unmount();
  });

  it("第一阶段保存后切换数据源，角色卡仍携带原归属且不报告完整成功", async () => {
    const wrapper = await open();
    mocks.put.mockImplementation(async (_path, _data, config) => {
      if (config.headers["X-Amitia-Role-Authority"] !== owner) throw new Error("归属变化");
      owner = core;
    });
    await edited.saveChar();
    expect(mocks.put).toHaveBeenCalledTimes(2);
    expect(mocks.put.mock.calls[1][2].headers["X-Amitia-Role-Authority"]).toBe(source);
    expect(mocks.success).not.toHaveBeenCalled();
    wrapper.unmount();
  });

  it("等待保存时冻结卡片快照并拦截重复提交", async () => {
    const wrapper = await open();
    let finish!: () => void;
    mocks.put.mockImplementationOnce(() => new Promise<void>((resolve) => { finish = resolve; })).mockResolvedValue(undefined);
    edited.form.scenario = "提交时场景";
    const saving = edited.saveChar();
    edited.form.scenario = "等待期间场景";
    await edited.saveChar();
    expect(mocks.put).toHaveBeenCalledTimes(1);
    expect(mocks.success).not.toHaveBeenCalled();
    finish();
    await saving;
    expect(mocks.put).toHaveBeenCalledTimes(2);
    expect(mocks.put.mock.calls[1][1].scenario).toBe("提交时场景");
    expect(mocks.success).toHaveBeenCalledTimes(1);
    wrapper.unmount();
  });

  it("空集合的新建操作保留打开时归属，归属未知时禁止提交", async () => {
    mocks.get.mockImplementation(async (path: string) => path.endsWith("/authority") ? { roleAuthority: source } : path.endsWith("/coordination/me") ? { policy: { coordinated: false } } : []);
    const wrapper = await open();
    edited.createNew();
    edited.form.name = "新角色";
    owner = core;
    mocks.post.mockImplementation(async (_path, _data, config) => {
      expect(config.headers["X-Amitia-Role-Authority"]).toBe(source);
      throw new Error("归属变化");
    });
    await edited.saveChar();
    expect(mocks.post).toHaveBeenCalledTimes(1);
    edited.selectChar({ id: "legacy", name: "无归属角色" });
    await edited.saveChar();
    expect(mocks.put).not.toHaveBeenCalled();
    wrapper.unmount();
  });

  it("删除和模板创建遭遇数据源变化时展示拒绝提示并保持原归属", async () => {
    const wrapper = await open();
    const target = edited.characters.value[0];
    owner = core;
    mocks.del.mockImplementation(async (_path, config) => {
      expect(config.headers["X-Amitia-Role-Authority"]).toBe(source);
      throw new Error("归属变化，删除已拒绝");
    });
    await edited.delChar(target);
    expect(mocks.error).toHaveBeenCalledWith("归属变化，删除已拒绝");
    expect(mocks.success).not.toHaveBeenCalled();
    mocks.post.mockImplementation(async (_path, _data, config) => {
      expect(config.headers["X-Amitia-Role-Authority"]).toBe(source);
      throw new Error("归属变化，模板创建已拒绝");
    });
    await edited.createFromTemplate({ id: "template", name: "旧模板", roleAuthority: source } as any);
    expect(mocks.error).toHaveBeenCalledWith("归属变化，模板创建已拒绝");
    wrapper.unmount();
  });
});
