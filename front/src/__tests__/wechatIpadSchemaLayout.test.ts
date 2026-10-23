import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { flushPromises, mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { createPinia } from "pinia";
import { reactive, nextTick } from "vue";
import { describe, expect, it } from "vitest";
import SchemaUINode from "../components/extension/SchemaUINode.vue";
import { validateDocument, type SchemaUIDocument } from "../components/extension/schema-ui-utils";

const document = JSON.parse(readFileSync(resolve(__dirname, "../../../plugins-code/channels/wechat-ipad860/ui/schema/desktop.json"), "utf8")) as SchemaUIDocument;

describe("个人微信 Schema 页面", () => {
  it("直接使用通道响应展示联系人和日志，并隐藏内部状态字段", async () => {
    expect(validateDocument(document).valid).toBe(true);
    const state = reactive({
      status: { connected: false, nickname: "", peerCount: 1, aiReplyEnabledCount: 1, messageCount: 0, sessionKey: "internal-session-key", engine: { port: 12345 }, qrImageUrl: "", lastError: "" },
      peers: { items: [{ id: "peer-one", displayName: "测试联系人", isGroup: false, aiReplyEnabled: true }] },
      logs: { items: [{ createdAt: "2026-10-12", level: "info", message: "通道已启动" }] },
    });
    const wrapper = mount(SchemaUINode, {
      props: { node: document.children![0], depth: 1, formState: { groupMentionOnly: true, peerEnabled: true, peerId: "", sendPeerId: "", sendText: "" }, context: { localState: state }, sessionId: "test", extensionId: "com.amitia/channel-wechat-ipad860", contributionId: "wechat-ipad860-page" },
      global: { plugins: [createPinia(), ElementPlus] },
    });
    try {
      await flushPromises();
      expect(wrapper.text()).toContain("未连接");
      expect(wrapper.text()).toContain("测试联系人");
      expect(wrapper.text()).not.toContain("internal-session-key");
      expect(wrapper.text()).not.toContain("12345");
      expect(wrapper.find('[data-node-id="wechat-login-qr"]').exists()).toBe(false);
      await wrapper.find('[data-node-id="wechat-refresh"]').trigger("click");
      expect(wrapper.emitted("action")).toHaveLength(1);
      state.status.qrImageUrl = "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg'/%3E";
      await nextTick();
      expect(wrapper.findAll('[data-node-id="wechat-login-qr"]')).toHaveLength(1);
      expect(wrapper.find('[data-node-id="wechat-login-qr"]').attributes("style")).toContain("height: 260px");
      const logsTab = wrapper.findAll(".el-tabs__item").find((tab) => tab.text() === "运行日志")!;
      await logsTab.trigger("click");
      await flushPromises();
      expect(wrapper.find('[data-node-id="wechat-log-table"]').text()).toContain("通道已启动");
    } finally {
      wrapper.unmount();
    }
  });
});
