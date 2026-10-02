import { describe, expect, it } from "vitest";
import { mount } from "@vue/test-utils";
import ConfigCardList from "../views/model-config/components/ConfigCardList.vue";

describe("vision model takeover", () => {
  it("intercepts both card and action clicks when suspended", async () => {
    const config = { id: 1, name: "Vision", disabled: true, isActive: false };
    const wrapper = mount(ConfigCardList, {
      props: { configs: [config], providers: [], testingId: null },
      global: { stubs: { "el-button": { template: '<button><slot /></button>' }, "el-tag": true, "el-empty": true } },
    });
    await wrapper.get(".config-card").trigger("click");
    await wrapper.findAll("button")[0].trigger("click");
    expect(wrapper.emitted("edit")).toHaveLength(2);
    expect(wrapper.emitted("test")).toBeUndefined();
    expect(wrapper.get(".config-card").classes()).toContain("is-suspended");
  });

  it("restores independent actions after takeover ends", async () => {
    const wrapper = mount(ConfigCardList, {
      props: { configs: [{ id: 2, name: "Vision", disabled: false }], providers: [], testingId: null },
      global: { stubs: { "el-button": { template: '<button><slot /></button>' }, "el-tag": true, "el-empty": true } },
    });
    await wrapper.findAll("button")[0].trigger("click");
    expect(wrapper.emitted("test")).toEqual([[2]]);
    expect(wrapper.emitted("edit")).toBeUndefined();
  });
});
