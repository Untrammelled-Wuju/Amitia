import { describe, expect, it } from "vitest";
import { listMainPanels, registerMainPanel, resolveMainPanel } from "@/ui-runtime/mainPanelRegistry";

describe("mainPanelRegistry", () => {
  it("注册后可解析并列出，注销后消失", () => {
    const dispose = registerMainPanel({ id: "probe-panel", title: "探测面板", component: { render: () => null } as never });
    expect(resolveMainPanel("probe-panel")?.title).toBe("探测面板");
    expect(listMainPanels().some((panel) => panel.id === "probe-panel")).toBe(true);
    dispose();
    expect(resolveMainPanel("probe-panel")).toBeUndefined();
    expect(listMainPanels().some((panel) => panel.id === "probe-panel")).toBe(false);
  });

  it("拒绝空 id、空标题与重复注册", () => {
    expect(() => registerMainPanel({ id: "  ", title: "x", component: {} as never })).toThrow(/id 不能为空/);
    expect(() => registerMainPanel({ id: "no-title", title: " ", component: {} as never })).toThrow(/需要标题/);
    const dispose = registerMainPanel({ id: "dup", title: "dup", component: {} as never });
    expect(() => registerMainPanel({ id: "dup", title: "dup", component: {} as never })).toThrow(/已注册/);
    dispose();
  });
});
