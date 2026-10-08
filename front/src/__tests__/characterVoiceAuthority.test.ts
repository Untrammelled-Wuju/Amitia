import { beforeEach, describe, expect, it, vi } from "vitest";
import { defineComponent, ref } from "vue";
import { flushPromises, mount } from "@vue/test-utils";
import { useCharacterVoice } from "../views/character-voice/composables/useCharacterVoice";

const mocks = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn(), success: vi.fn(), error: vi.fn(), warning: vi.fn() }));
vi.mock("../ui-index", () => ({ apiClient: mocks }));
vi.mock("element-plus", () => ({ ElMessage: mocks, ElMessageBox: { confirm: vi.fn() } }));

describe("角色声线编辑归属", () => {
  const source = "a".repeat(64);
  const core = "b".repeat(64);
  let owner: string;
  let editor: ReturnType<typeof useCharacterVoice>;
  const id = ref<string | null>("same-role");
  const open = () => mount(defineComponent({ setup() { editor = useCharacterVoice(); return () => null; } }), { global: { provide: { currentCharacterId: id } } });
  beforeEach(() => {
    Object.values(mocks).forEach((mock) => mock.mockReset());
    id.value = "same-role";
    owner = source;
    mocks.get.mockImplementation(async (path: string) => ({ data: path.startsWith("/api/characters/") ? { roleAuthority: owner, voiceSpeed: 1 } : [] }));
  });

  it("同 ID 切换 Core 后旧声线编辑提交原归属并被拒绝", async () => {
    const wrapper = open();
    await flushPromises();
    owner = core;
    mocks.put.mockImplementation(async (_path, _body, config) => {
      expect(config.headers["X-Amitia-Role-Authority"]).toBe(source);
      throw new Error("角色数据归属已变化，请重新加载");
    });
    await editor.saveVoice();
    expect(mocks.get.mock.calls.filter(([path]) => path.startsWith("/api/characters/"))).toHaveLength(1);
    expect(mocks.put).toHaveBeenCalledTimes(1);
    expect(mocks.success).not.toHaveBeenCalled();
    expect(mocks.error).toHaveBeenCalledWith("角色数据归属已变化，请重新加载");
    wrapper.unmount();
  });

  it("切换角色期间禁止旧表单提交到新 ID，迟到响应不能覆盖新角色", async () => {
    let finishOld!: (response: unknown) => void;
    mocks.get.mockImplementation(async (path: string) => {
      if (path === "/api/characters/same-role") return new Promise((resolve) => { finishOld = resolve; });
      return { data: path === "/api/characters/new-role" ? { roleAuthority: core, voiceSpeed: 2 } : [] };
    });
    const wrapper = open();
    id.value = "new-role";
    await editor.saveVoice();
    expect(mocks.put).not.toHaveBeenCalled();
    await flushPromises();
    finishOld({ data: { roleAuthority: source, voiceSpeed: 0.5 } });
    await flushPromises();
    expect(editor.form.voiceSpeed).toBe(2);
    await editor.saveVoice();
    expect(mocks.put).toHaveBeenCalledWith("/api/characters/new-role", expect.objectContaining({ voiceSpeed: 2 }), { headers: { "X-Amitia-Role-Authority": core } });
    wrapper.unmount();
  });

  it("保存期间快照稳定且重复操作只发送一次", async () => {
    const wrapper = open();
    await flushPromises();
    let finish!: () => void;
    mocks.put.mockImplementation(() => new Promise<void>((resolve) => { finish = resolve; }));
    editor.form.voiceSpeed = 1.5;
    const save = editor.saveVoice();
    editor.form.voiceSpeed = 2;
    await editor.saveVoice();
    expect(mocks.put).toHaveBeenCalledTimes(1);
    finish();
    await save;
    expect(editor.originalForm.voiceSpeed).toBe(1.5);
    wrapper.unmount();
  });
});
