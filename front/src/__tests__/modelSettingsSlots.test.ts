import { mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { describe, expect, it } from "vitest";
import ConfigCardList from "../views/model-config/components/ConfigCardList.vue";

describe("model settings extension slots", () => {
  it("mounts provider-card and footer surfaces with provider context", () => {
    const wrapper = mount(ConfigCardList, {
      props: {
        configs: [
          {
            id: 7,
            name: "主模型",
            apiType: "openai",
            modelName: "gpt-test",
            baseUrl: "https://api.example.test",
            hasApiKey: true,
            isActive: true,
            disabled: false,
            lastTestStatus: "success",
          },
        ],
        testingId: null,
        providers: [{ id: "openai", name: "OpenAI" }],
      },
      global: {
        plugins: [ElementPlus],
        stubs: {
          ExtensionSlot: {
            props: ["slotId", "dispatchKey", "context"],
            template:
              '<div :data-slot-id="slotId" :data-dispatch-key="dispatchKey" :data-slot-context="JSON.stringify(context)"></div>',
          },
        },
      },
    });

    const provider = wrapper.get('[data-slot-id="settings.model.provider"]');
    expect(provider.attributes("data-dispatch-key")).toBe("openai");
    expect(JSON.parse(provider.attributes("data-slot-context") || "{}")).toMatchObject({
      settingsNs: "openai",
      provider: "openai",
      providerName: "OpenAI",
      mode: "saved",
      configId: 7,
      modelName: "gpt-test",
      hasApiKey: true,
      active: true,
      status: "success",
    });
    expect(
      JSON.parse(
        wrapper
          .get('[data-slot-id="settings.model.footer"]')
          .attributes("data-slot-context") || "{}",
      ),
    ).toMatchObject({ settingsNs: "models", surface: "models-footer" });
    wrapper.unmount();
  });
});
