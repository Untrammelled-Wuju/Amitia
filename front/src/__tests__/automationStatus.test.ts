import { describe, expect, it } from "vitest";
import { mount } from "@vue/test-utils";
import type { AssistantTurnData } from "@/conversation/rendering/types";
import { automationAction, projectAutomationStatus } from "@/conversation/runtime/automationStatus";
import AutomationStatusIndicator from "@/components/AutomationStatusIndicator.vue";

function turn(status = "running", name = "browser_interact_click", itemStatus = "running"): AssistantTurnData {
  return { id: "root", conversationId: "chat", sequence: 1, status,
    items: [{ id: "call", turnId: "root", conversationId: "chat", sequence: 1, type: "tool_call", status: itemStatus, toolName: name }] };
}

describe("automation status", () => {
  it("maps supported browser and Android operations without showing arguments", () => {
    expect(automationAction("android_interaction_visual_locate")).toBe("正在观察画面");
    expect(automationAction("android.interaction.input_text")).toBe("正在输入");
    expect(automationAction("browser_dom_scroll_to_element")).toBe("正在滚动");
    expect(automationAction("android_virtual_display_capture")).toBe("正在观察画面");
    expect(automationAction("browser.agent.run")).toBe("正在自动化操作");
    expect(automationAction("exec_command")).toBeNull();
    expect(automationAction("android_interaction_status")).toBeNull();
    expect(automationAction("browser_tab_list")).toBeNull();
  });
  it("hides terminal turns and finished calls", () => {
    for (const status of ["completed", "failed", "interrupted"]) {
      expect(projectAutomationStatus([turn(status)])).toBeNull();
      expect(projectAutomationStatus([turn("running", "browser_interact_click", status)])).toBeNull();
    }
    expect(projectAutomationStatus([])).toBeNull();
  });
  it("distinguishes queued, approval, and cancellation states", () => {
    expect(projectAutomationStatus([turn("waiting_approval")])?.label).toBe("等待授权");
    expect(projectAutomationStatus([turn("running", "browser_interact_click", "queued")])?.label).toBe("等待自动化执行");
    expect(projectAutomationStatus([turn("cancelling")])?.phase).toBe("cancelling");
  });
  it("counts parallel calls and follows the newest operation", () => {
    const root = turn();
    root.items.push({ id: "second", turnId: "root", conversationId: "chat", sequence: 2, type: "tool_call", status: "running", toolName: "browser_interact_input" });
    expect(projectAutomationStatus([root])).toMatchObject({ count: 2, label: "正在输入" });
  });
  it("does not revive stale roots or orphaned child turns", () => {
    const child = { ...turn(), id: "child", parentTurnId: "root", sequence: 2 };
    expect(projectAutomationStatus([turn("completed"), child])).toBeNull();
    expect(projectAutomationStatus([child])).toBeNull();
    expect(projectAutomationStatus([turn(), { ...turn("completed"), id: "new", sequence: 3 }])).toBeNull();
    expect(projectAutomationStatus([turn(), child])?.count).toBe(2);
  });
  it("renders an accessible status and removes it after completion", async () => {
    const wrapper = mount(AutomationStatusIndicator, { props: { status: projectAutomationStatus([turn()]) } });
    expect(wrapper.get('[role="status"]').text()).toContain("正在点击");
    expect(wrapper.findAll("button")).toHaveLength(0);
    await wrapper.setProps({ status: null });
    expect(wrapper.find('[role="status"]').exists()).toBe(false);
    wrapper.unmount();
  });
});
