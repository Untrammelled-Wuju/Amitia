import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import * as icons from "@element-plus/icons-vue";
import AmitiaIcon from "@/components/icons/AmitiaIcon.vue";
import {
  amitiaIconShapes,
  isAmitiaIconName,
  type AmitiaIconShape,
} from "@/components/icons/amitiaIconRegistry";

describe("Amitia desktop icon system", () => {
  it("keeps the complete Element Plus compatibility surface on Amitia icons", () => {
    const components = Object.values(icons);
    expect(components.length).toBeGreaterThanOrEqual(95);
    for (const component of components) {
      const wrapper = mount(component);
      const svg = wrapper.get("svg");
      expect(svg.attributes("viewBox")).toBe("0 0 24 24");
      expect(svg.attributes("stroke-width")).toBe("1.8");
      expect(svg.attributes("stroke-linecap")).toBe("round");
      expect(svg.attributes("stroke-linejoin")).toBe("round");
    }
  });

  it("uses valid semantic names and geometry within the shared view box", () => {
    expect(isAmitiaIconName("messages")).toBe(true);
    expect(isAmitiaIconName("missing")).toBe(false);
    for (const [name, shape] of Object.entries(amitiaIconShapes)) {
      const geometry: AmitiaIconShape = shape;
      expect(name.length).toBeGreaterThan(0);
      for (const circle of geometry.circles ?? []) {
        expect(circle.cx).toBeGreaterThanOrEqual(0);
        expect(circle.cx).toBeLessThanOrEqual(24);
        expect(circle.cy).toBeGreaterThanOrEqual(0);
        expect(circle.cy).toBeLessThanOrEqual(24);
      }
      for (const rect of geometry.rects ?? []) {
        expect(rect.x).toBeGreaterThanOrEqual(0);
        expect(rect.y).toBeGreaterThanOrEqual(0);
        expect(rect.x + rect.width).toBeLessThanOrEqual(24);
        expect(rect.y + rect.height).toBeLessThanOrEqual(24);
      }
    }
  });

  it("renders semantic icons at arbitrary inherited sizes", () => {
    const wrapper = mount(AmitiaIcon, {
      props: { name: "extensions" },
      attrs: { style: "font-size: 24px" },
    });
    expect(wrapper.get("svg").classes()).toContain("amitia-icon");
    expect(wrapper.get("svg").attributes("aria-hidden")).toBe("true");
  });
});
