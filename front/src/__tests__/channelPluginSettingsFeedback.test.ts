import { flushPromises, mount } from "@vue/test-utils";
import ElementPlus, { ElMessage } from "element-plus";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ChannelPluginSettingsPanel from "../views/channels/ChannelPluginSettingsPanel.vue";

const api = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn() }));
vi.mock("@/composables/useApi", () => ({ apiClient: api }));

describe("channel plugin settings", () => {
  beforeEach(() => {
    api.get.mockReset().mockResolvedValue({ data: { code: 200, data: { channelId: "demo", enabled: false, allowNetwork: true, workspaceId: "", sandboxRoot: "", revision: 1 } } });
    api.put.mockReset();
  });
  it("读取策略后禁用未开启沙箱的联网开关", async () => {
    const warning = vi.spyOn(ElMessage, "warning").mockImplementation(() => undefined as any);
    const wrapper = mount(ChannelPluginSettingsPanel, { props: { channelId: "demo" }, global: { plugins: [ElementPlus] } });
    try {
      await flushPromises();
      expect(api.get).toHaveBeenCalledWith("/api/channels/sandbox-policies", { params: { channelId: "demo" } });
      expect(wrapper.findAll(".el-switch")[1].classes()).toContain("is-disabled");
      await wrapper.findAll(".settings-control")[2].trigger("click");
      expect(warning).toHaveBeenCalledWith("请先开启沙箱隔离。");
      expect(api.put).not.toHaveBeenCalled();
    } finally { wrapper.unmount(); warning.mockRestore(); }
  });
  it("读取失败保留原始错误并禁止用默认值写入，重试后恢复", async () => {
    const warning = vi.spyOn(ElMessage, "warning").mockImplementation(() => undefined as any);
    api.get.mockRejectedValueOnce(new Error("sandbox-original-error"));
    const wrapper = mount(ChannelPluginSettingsPanel, { props: { channelId: "demo" }, global: { plugins: [ElementPlus] } });
    try {
      await flushPromises();
      expect(wrapper.get("details").text()).toContain("sandbox-original-error");
      expect(wrapper.findAll(".el-switch")[0].classes()).toContain("is-disabled");
      await wrapper.findAll(".settings-control")[1].trigger("click");
      expect(warning).toHaveBeenCalledWith("无法读取渠道设置，请查看原始错误并重试。");
      expect(api.put).not.toHaveBeenCalled();
      await wrapper.get(".settings-heading button").trigger("click");
      await flushPromises();
      expect(wrapper.findAll(".el-switch")[0].classes()).not.toContain("is-disabled");
    } finally { wrapper.unmount(); warning.mockRestore(); }
  });
  it("切换渠道后旧读取结果不会覆盖当前策略", async () => {
    let resolveOld!: (value: unknown) => void;
    api.get.mockReset().mockImplementationOnce(() => new Promise((resolve) => { resolveOld = resolve; })).mockResolvedValueOnce({ data: { code: 200, data: { channelId: "new", enabled: false, allowNetwork: true, workspaceId: "new-space", sandboxRoot: "", revision: 1 } } });
    const wrapper = mount(ChannelPluginSettingsPanel, { props: { channelId: "old" }, global: { plugins: [ElementPlus] } });
    try {
      await wrapper.setProps({ channelId: "new" });
      await flushPromises();
      resolveOld({ data: { code: 200, data: { channelId: "old", enabled: true, allowNetwork: true, workspaceId: "old-space", sandboxRoot: "", revision: 1 } } });
      await flushPromises();
      expect(wrapper.text()).toContain("new-space");
      expect(wrapper.text()).not.toContain("old-space");
    } finally { wrapper.unmount(); }
  });
  it("服务返回失败响应时保留完整内容并禁止写入", async () => {
    api.get.mockResolvedValueOnce({ data: { code: 503, msg: "backend-failure", diagnostic: "full-detail" } });
    const wrapper = mount(ChannelPluginSettingsPanel, { props: { channelId: "demo" }, global: { plugins: [ElementPlus] } });
    try {
      await flushPromises();
      expect(wrapper.get("details").text()).toContain("backend-failure");
      expect(wrapper.get("details").text()).toContain("full-detail");
      expect(wrapper.findAll(".el-switch")[0].classes()).toContain("is-disabled");
      expect(api.put).not.toHaveBeenCalled();
    } finally { wrapper.unmount(); }
  });
  it("保存失败保留错误信息和原策略", async () => {
    api.put.mockRejectedValueOnce(new Error("sandbox-save-original"));
    const wrapper = mount(ChannelPluginSettingsPanel, { props: { channelId: "demo" }, global: { plugins: [ElementPlus] } });
    try {
      await flushPromises();
      await wrapper.findAll(".el-switch")[0].trigger("click");
      await flushPromises();
      expect(api.put).toHaveBeenCalledOnce();
      expect(wrapper.get("details").text()).toContain("sandbox-save-original");
      expect(wrapper.text()).toContain("未隔离，可访问本机文件");
    } finally { wrapper.unmount(); }
  });
  it("保存中不重复提交，切换渠道后旧保存结果不会覆盖新策略", async () => {
    let resolveSave!: (value: unknown) => void;
    const warning = vi.spyOn(ElMessage, "warning").mockImplementation(() => undefined as any);
    api.get.mockImplementation(async (_path, options) => ({ data: { code: 200, data: { channelId: options.params.channelId, enabled: false, allowNetwork: true, workspaceId: `${options.params.channelId}-space`, sandboxRoot: "", revision: 1 } } }));
    api.put.mockImplementationOnce(() => new Promise((resolve) => { resolveSave = resolve; }));
    const wrapper = mount(ChannelPluginSettingsPanel, { props: { channelId: "old" }, global: { plugins: [ElementPlus] } });
    try {
      await flushPromises();
      await wrapper.findAll(".el-switch")[0].trigger("click");
      expect(api.put).toHaveBeenCalledOnce();
      await wrapper.findAll(".settings-control")[1].trigger("click");
      expect(warning).toHaveBeenCalledWith("正在保存渠道设置，请稍后重试。");
      expect(api.put).toHaveBeenCalledOnce();
      await wrapper.setProps({ channelId: "new" });
      await flushPromises();
      resolveSave({ data: { code: 200, data: { channelId: "old", enabled: true, allowNetwork: true, workspaceId: "old-saved", sandboxRoot: "", revision: 2 } } });
      await flushPromises();
      expect(wrapper.text()).toContain("new-space");
      expect(wrapper.text()).not.toContain("old-saved");
    } finally { wrapper.unmount(); warning.mockRestore(); }
  });
});
