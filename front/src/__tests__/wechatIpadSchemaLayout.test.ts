import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { flushPromises, mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { createPinia } from "pinia";
import { reactive, nextTick } from "vue";
import { describe, expect, it } from "vitest";
import SchemaUINode from "../components/extension/SchemaUINode.vue";
import { validateDocument, type SchemaUIDocument } from "../components/extension/schema-ui-utils";

const document = JSON.parse(readFileSync(resolve(__dirname, "../../../plugins-code/channels/wechat/ui/mobile/schema.json"), "utf8")) as SchemaUIDocument;

describe("微信渠道 Schema 页面", () => {
  it("使用当前通道状态控制二维码和操作入口", async () => {
    expect(validateDocument(document).valid).toBe(true);
    const state = reactive({
      status: { status: "disconnected", accountId: "", message: "", lastError: "" },
      connectResult: { data: { qrImageUrl: "" } },
    });
    const wrapper = mount(SchemaUINode, {
      props: { node: document.children![0], depth: 1, formState: {}, context: { localState: state }, sessionId: "test", extensionId: "com.amitia/channel-wechat", contributionId: "wechat-mobile-page" },
      global: { plugins: [createPinia(), ElementPlus] },
    });
    try {
      await flushPromises();
      expect(wrapper.find('[data-node-id="wechat-mobile-qr"]').exists()).toBe(false);
      await wrapper.find('[data-node-id="wechat-mobile-refresh"]').trigger("click");
      expect(wrapper.emitted("action")).toHaveLength(1);
      state.connectResult.data.qrImageUrl = "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg'/%3E";
      await nextTick();
      expect(wrapper.find('[data-node-id="wechat-mobile-qr"]').exists()).toBe(true);
      expect(wrapper.find('[data-node-id="wechat-mobile-qr"]').attributes("style")).toContain("height: 280px");
    } finally {
      wrapper.unmount();
    }
  });
});
