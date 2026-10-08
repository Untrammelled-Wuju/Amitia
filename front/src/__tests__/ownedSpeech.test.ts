import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { defineComponent } from "vue";
import { flushPromises, mount } from "@vue/test-utils";
import { useOwnedSpeech } from "../composables/useOwnedSpeech";
import VoicePlayBar from "../components/chat-bubble/VoicePlayBar.vue";
import { useDeviceOwnedConversation } from "../composables/useDeviceOwnedConversation";
const cryptoModule = "node:crypto";
const { webcrypto } = await import(cryptoModule);

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), legacy: vi.fn(), deployment: vi.fn(), error: vi.fn(), warning: vi.fn() }));
vi.mock("../composables/useApi", () => ({ apiClient: { get: mocks.get, post: mocks.post }, useApi: () => ({ post: mocks.legacy }) }));
vi.mock("../runtime/runtime-adapter", () => ({ getDeploymentConfig: mocks.deployment, getNativeProviderTransition: vi.fn().mockResolvedValue(null) }));
vi.mock("element-plus", () => ({ ElMessage: { error: mocks.error, warning: mocks.warning } }));

describe("Core 角色语音保存与播放", () => {
  const audio = new TextEncoder().encode("实际响应音频字节");
  const audioBase64 = btoa(String.fromCharCode(...audio));
  let audioHash: string;
  let scope: any;
  let resultChange: (value: any) => void;
  const wrappers: ReturnType<typeof mount>[] = [];
  const open = () => {
    let speech!: ReturnType<typeof useOwnedSpeech>;
    wrappers.push(mount(defineComponent({ setup() { speech = useOwnedSpeech(); return () => null; } })));
    return speech;
  };
  beforeEach(async () => {
    vi.stubGlobal("crypto", webcrypto);
    audioHash = Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256", audio)), (value) => value.toString(16).padStart(2, "0")).join("");
    Object.values(mocks).forEach((mock) => mock.mockReset());
    mocks.deployment.mockResolvedValue({ mode: "cloud" });
    resultChange = () => {};
    scope = { authorizationRealm: "mesh", spaceId: "core-b", initiatorDeviceId: "source-a", targetDeviceId: "source-a", coreId: "core-b", providerEpoch: 1, coordinated: false, modeRevision: 1, permissionRevision: 1, targetPermissionRevision: 1, targetProviderEpoch: 1, roleId: "source-role", roleRevision: 1, roleOwnerId: "source-a", resourceOwnerId: "source-a", requestId: "query", turnId: "query-turn", executionId: "query-execution" };
    mocks.get.mockImplementation(async (path) => ({ data: path.endsWith("/roles") ? { roles: [{ id: "source-role" }] } : { executionScope: structuredClone(scope) } }));
    mocks.post.mockImplementation(async (_path, payload) => {
      const hash = Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(`${scope.coreId}\0${scope.initiatorDeviceId}\0${payload.requestId}`))), (value) => value.toString(16).padStart(2, "0")).join("");
      const id = `speech/${hash.slice(0, 32)}`;
      const value = { requestId: payload.requestId, executionScope: { ...payload.expectedExecutionScope, requestId: `speech/${payload.requestId}`, turnId: "speech-turn", executionId: "speech-execution" }, saved: true, acknowledgement: { requestId: `speech/${payload.requestId}`, ownerId: "source-a", versions: { [`tool-result/${id}`]: 1, [`checkpoint/${id}`]: 2 } }, audio: { mime: "audio/mpeg", data: audioBase64, sha256: audioHash } };
      resultChange(value);
      return { data: value };
    });
  });
  afterEach(() => { wrappers.splice(0).forEach((wrapper) => wrapper.unmount()); vi.clearAllTimers(); vi.useRealTimers(); vi.unstubAllGlobals(); });

  it("OFF 使用 Source 完整角色归属，ACK及完整性校验后提供音频", async () => {
    const speech = open();
    expect(await speech.synthesizeIfBound("语音文本", "source-role")).toBe(`data:audio/mpeg;base64,${audioBase64}`);
    expect(mocks.post).toHaveBeenCalledWith("/api/device-mesh/v1/business/speech", expect.objectContaining({ characterId: "source-role", expectedExecutionScope: scope, text: "语音文本" }), expect.any(Object));
    expect(mocks.post.mock.calls[0][1]).not.toHaveProperty("voiceConfigId");
    expect(mocks.get).toHaveBeenCalledTimes(2);
  });
  it.each(["saved", "owner", "hash", "role", "realm", "checkpoint", "foreign-ack"])("缺失或错误 %s 不返回可播放音频", async (failure) => {
    resultChange = (value) => {
      if (failure === "saved") value.saved = false;
      if (failure === "owner") value.acknowledgement.ownerId = "foreign-source";
      if (failure === "hash") value.audio.sha256 = "0".repeat(64);
      if (failure === "role") value.executionScope.roleOwnerId = "core-b";
      if (failure === "realm") value.executionScope.authorizationRealm = "another-realm";
      if (failure === "checkpoint") { const key = Object.keys(value.acknowledgement.versions).find((key) => key.startsWith("checkpoint/"))!; value.acknowledgement.versions[key] = 1; }
      if (failure === "foreign-ack") value.acknowledgement.versions = { "tool-result/unrelated": 1, "checkpoint/unrelated": 2 };
    };
    await expect(open().synthesizeIfBound("语音文本", "source-role")).rejects.toThrow();
  });
  it("响应完成后角色版本已变化，旧声音丢弃", async () => {
    resultChange = () => { scope.roleRevision = 2; };
    await expect(open().synthesizeIfBound("语音文本", "source-role")).rejects.toThrow("旧语音已丢弃");
  });
  it("Core 切换中断在途请求并拦截迟到音频", async () => {
    let finish!: (response: any) => void;
    mocks.post.mockImplementation(() => new Promise((resolve) => { finish = resolve; }));
    const pending = open().synthesizeIfBound("语音文本", "source-role");
    await vi.waitFor(() => expect(mocks.post).toHaveBeenCalledTimes(1));
    window.dispatchEvent(new CustomEvent("amitia:execution-scope-changed"));
    finish({ data: {} });
    await expect(pending).rejects.toThrow("旧语音已丢弃");
  });
  it("本地保持旧路径，绑定多角色未选择时拒绝调用", async () => {
    const speech = open();
    mocks.deployment.mockResolvedValue({ mode: "local" });
    expect(await speech.synthesizeIfBound("测试")).toBeNull();
    expect(mocks.post).not.toHaveBeenCalled();
    mocks.deployment.mockResolvedValue({ mode: "cloud" });
    mocks.get.mockResolvedValue({ data: { roles: [{ id: "a" }, { id: "b" }] } });
    await expect(speech.synthesizeIfBound("测试")).rejects.toThrow("选择一个已保存的角色");
  });
  it("聊天播放使用 OwnedSpeech，服务变化立即停止已播放音频", async () => {
    const pause = vi.fn();
    const play = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("Audio", class { pause = pause; play = play; onended: any; onerror: any; constructor(public src: string) {} });
    const wrapper = mount(VoicePlayBar, { props: { messageRole: "assistant", messageContent: "回复文本", characterId: "source-role" } });
    wrappers.push(wrapper);
    await wrapper.trigger("click");
    await vi.waitFor(() => expect(play).toHaveBeenCalledTimes(1));
    expect(mocks.legacy).not.toHaveBeenCalled();
    window.dispatchEvent(new CustomEvent("amitia:runtime-connection-changed"));
    expect(pause).toHaveBeenCalledTimes(1);
  });

  it("自动发现 B→C 提供者切换时中断实际播放，无需设备页面人工事件", async () => {
    mocks.get.mockImplementation(async (path) => ({ data: path.endsWith("/coordination/me") ? { coreId: scope.coreId, coordinationAvailable: true, policy: { coordinated: scope.coordinated, providerEpoch: 1, modeRevision: 1, permissionRevision: 1 } } : path.endsWith("/roles") ? { roles: [{ id: "source-role" }], roleOwnerId: "source-a" } : { executionScope: structuredClone(scope) } }));
    const mesh = useDeviceOwnedConversation();
    await mesh.refresh();
    const pause = vi.fn();
    const play = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("Audio", class { pause = pause; play = play; onended: any; onerror: any; constructor(public src: string) {} });
    const wrapper = mount(VoicePlayBar, { props: { messageRole: "assistant", messageContent: "回复文本", characterId: "source-role" } });
    wrappers.push(wrapper);
    await wrapper.trigger("click");
    await vi.waitFor(() => expect(play).toHaveBeenCalledTimes(1));
    scope.coreId = "core-c";
    scope.spaceId = "core-c";
    await mesh.refresh();
    expect(pause).toHaveBeenCalledTimes(1);
    expect(mesh.notice.value).toContain("core-c");
  });

  it.each(["updated", "deleted"])("播放期间角色%s，元数据轮询立即停播", async (change) => {
    vi.useFakeTimers();
    let deleted = false;
    mocks.get.mockImplementation(async (path) => ({ data: path.endsWith("/coordination/me") ? { coreId: scope.coreId, policy: { coordinated: scope.coordinated, providerEpoch: 1, modeRevision: 1, permissionRevision: 1 } } : path.endsWith("/roles") ? { roles: deleted ? [] : [{ id: "source-role", revision: scope.roleRevision }], roleOwnerId: scope.roleOwnerId } : { executionScope: structuredClone(scope) } }));
    const pause = vi.fn();
    const play = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("Audio", class { pause = pause; play = play; onended: any; onerror: any; constructor(public src: string) {} });
    const wrapper = mount(VoicePlayBar, { props: { messageRole: "assistant", messageContent: "回复文本", characterId: "source-role" } });
    wrappers.push(wrapper);
    await wrapper.trigger("click");
    await vi.waitFor(() => expect(play).toHaveBeenCalledTimes(1));
    if (change === "deleted") deleted = true;
    else scope.roleRevision++;
    await vi.advanceTimersByTimeAsync(1000);
    expect(pause).toHaveBeenCalledTimes(1);
    expect(mocks.get.mock.calls.filter(([path]) => path.endsWith("/business/data"))).toHaveLength(2);
  });
});
