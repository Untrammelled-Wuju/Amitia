import { beforeEach, describe, expect, it } from "vitest";
import {
  applyStyleRulePreview,
  applyStyleRulesToDocument,
  inspectDomTargets,
  resolveStyleTargetSelector,
} from "../ui-runtime/styleEngine";
import { normalizeStylePageId } from "../ui-runtime/stylePage";
import type { StyleRule } from "../ui-runtime/styleProfile";

function rule(overrides: Partial<StyleRule> = {}): StyleRule {
  return {
    id: "rule_test",
    source: "ai",
    scope: { platform: "desktop", page: "/chat" },
    target: "chat.composer.input",
    states: ["default"],
    properties: { backgroundColor: "#000000", foregroundColor: "#FFFFFF" },
    createdAt: "2026-10-10T00:00:00.000Z",
    ...overrides,
  };
}

describe("runtime style engine", () => {
  beforeEach(() => {
    document.head.innerHTML = "";
    document.body.innerHTML = "";
    document.documentElement.removeAttribute("style");
    document.documentElement.dataset.amitiaPlatform = "desktop";
    document.documentElement.dataset.amitiaPage = "chat";
  });

  it("inspects semantic targets first and falls back to stable DOM selectors", () => {
    document.body.innerHTML = `
      <main data-amitia-ui="page.chat" data-amitia-surface="chat.main">
        <section>
          <textarea data-amitia-ui="chat.composer.input"></textarea>
          <button id="send-button"><span>发送</span></button>
        </section>
      </main>
    `;
    const result = inspectDomTargets({ limit: 20, includeComputed: true });
    const input = result.targets.find((item) => item.target === "chat.composer.input");
    const button = result.targets.find((item) => item.tag === "button");
    const label = result.targets.find((item) => item.tag === "span");
    expect(input).toMatchObject({
      target: "chat.composer.input",
      selector: 'body > [data-amitia-ui="chat.composer.input"]',
      page: "chat",
      surface: "chat.main",
      tag: "textarea",
    });
    expect(input?.computed).toHaveProperty("background-color");
    expect(button?.selector).toBe("body > #send-button");
    expect(label?.selector).toBe("body > #send-button > span");
  });

  it("applies page scoped states and arbitrary safe CSS properties", () => {
    applyStyleRulesToDocument([
      rule({
        states: ["default", "hover", "focus-visible", "dark"],
        properties: {
          backgroundColor: "#000000",
          foregroundColor: "#FFFFFF",
          borderRadius: 16,
          scrollMarginTop: 8,
          placeholderColor: "#9CA3AF",
        },
      }),
    ]);
    const css = document.getElementById("amitia-ui-style-rules-v1")?.textContent ?? "";
    expect(css).toContain('html[data-amitia-platform="desktop"][data-amitia-page="chat"] body');
    expect(css).toContain('background-color: #000000');
    expect(css).toContain('color: #FFFFFF');
    expect(css).toContain("border-radius: 16px");
    expect(css).toContain("scroll-margin-top: 8px");
    expect(css).toContain(":hover");
    expect(css).toContain(":focus-visible");
    expect(css).toContain(".dark body");
    expect(css).toContain("::placeholder");
  });

  it("blocks scripts and raw url values while preserving safe unknown properties", () => {
    expect(resolveStyleTargetSelector("chat.composer.input")).toBe('[data-amitia-ui="chat.composer.input"]');
    expect(resolveStyleTargetSelector("dom:#chat-input")).toBe("#chat-input");
    expect(applyStyleRulePreview(rule({
      properties: {
        color: "javascript:alert(1)",
        backgroundImage: "url(https://example.com/a.png)",
        scrollPaddingTop: 12,
      },
    }))).toContain("scroll-padding-top: 12px");
    expect(applyStyleRulePreview(rule({
      target: 'body[data-amitia-ui="x"] {}',
      properties: { color: "red" },
    }))).toBe("");
  });

  it("prefers unique extension node targets while preserving semantic aliases", () => {
    document.body.innerHTML = `
      <button
        data-amitia-ui="component.button"
        data-amitia-node-target="plugin.schema.demo.node_save"
      >保存</button>
    `;
    expect(resolveStyleTargetSelector("plugin.schema.demo.node_save"))
      .toBe('[data-amitia-node-target="plugin.schema.demo.node_save"]');
    const result = inspectDomTargets({
      target: "plugin.schema.demo.node_save",
      limit: 5,
      includeComputed: false,
    });
    expect(result.targets[0]).toMatchObject({
      target: "plugin.schema.demo.node_save",
      selector: 'body > [data-amitia-node-target="plugin.schema.demo.node_save"]',
    });
  });

  it("normalizes page identifiers consistently across platforms", () => {
    expect(normalizeStylePageId("/chat")).toBe("chat");
    expect(normalizeStylePageId("/settings/appearance")).toBe("settings.appearance");
    expect(normalizeStylePageId("settings.appearance")).toBe("settings.appearance");
    expect(normalizeStylePageId("extension/page/emote.main")).toBe("plugin.emote-main");
    expect(normalizeStylePageId("/extension/page/main?extensionId=emote")).toBe("plugin.emote.main");
    expect(normalizeStylePageId("/")).toBe("root");
  });
});
