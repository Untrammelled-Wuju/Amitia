import { describe, expect, it } from "vitest";
import { aimMessagePlainText, normalizeAIMessage } from "@/conversation/rendering/amrp";
import { renderMarkdownSegment, splitMarkdownSegments } from "@/conversation/rendering/markdown/markdownEngine";

describe("AMRP renderer", () => {
  it("normalizes legacy rich message fields", () => {
    const message = normalizeAIMessage({
      id: "m1",
      role: "assistant",
      content: "结果如下",
      msgType: "tool_call",
      toolName: "read_file",
      toolArguments: { path: "src/App.vue" },
      toolResult: { ok: true },
      status: "success",
      createdAt: "2026-09-20T00:00:00.000Z",
    });

    expect(message.markdown).toBe("");
    expect(message.blocks).toHaveLength(1);
    expect(message.blocks[0]).toMatchObject({
      kind: "tool",
      name: "read_file",
      arguments: { path: "src/App.vue" },
      result: { ok: true },
      status: "success",
    });
    expect(aimMessagePlainText(message)).toContain("read_file");
  });

  it("splits fences by renderer type", () => {
    const segments = splitMarkdownSegments([
      "正文",
      "```dart",
      "void main() {}",
      "```",
      "```diff",
      "- old",
      "+ new",
      "```",
      "```terminal",
      "$ flutter test",
      "```",
      "```mermaid",
      "graph TD",
      "A --> B",
      "```",
      "```html-preview",
      "<h1>Hello</h1>",
      "```",
    ].join("\n"));

    expect(segments.map((segment) => segment.type)).toEqual([
      "markdown",
      "code",
      "diff",
      "terminal",
      "mermaid",
      "html-preview",
    ]);
    expect(segments[1].language).toBe("dart");
  });

  it("normalizes persisted reasoning duration", () => {
    const message = normalizeAIMessage({
      id: "m2",
      role: "assistant",
      content: "回复",
      reasoningContent: "思考",
      reasoningDurationMs: 2850,
      createdAt: "2026-09-20T00:00:00.000Z",
    });

    expect(message.thinking?.duration).toBe(2.85);
  });

  it("没有 reasoning 内容时不伪造 Thinking Block", () => {
    const pending = normalizeAIMessage({
      id: "generating-request",
      role: "assistant",
      content: "",
      status: "streaming",
    });
    const completed = normalizeAIMessage({
      id: "assistant-1",
      role: "assistant",
      content: "回复",
      status: "sent",
    });

    expect(pending.thinking).toBeUndefined();
    expect(completed.thinking).toBeUndefined();
  });

  it("sanitizes unsafe markdown links and keeps supported structure", () => {
    const html = renderMarkdownSegment([
      "# 标题",
      "",
      "[危险](javascript:alert(1))",
      "",
      "| A | B |",
      "| - | - |",
      "| 1 | 2 |",
      "",
      "行内公式 $E=mc^2$",
    ].join("\n"));

    expect(html).toContain("<h1>");
    expect(html).toContain("amrp-table-scroll");
    expect(html).toContain("katex");
    expect(html).not.toContain('href="javascript:');
  });
  it("renders only registered citation ids as interactive references", () => {
    const withoutSources = renderMarkdownSegment("unknown [99]", []);
    expect(withoutSources).toContain("[99]");
    expect(withoutSources).not.toContain('data-citation-id="99"');

    const mixed = renderMarkdownSegment("known [1], unknown [99]", ["1"]);
    expect(mixed).toContain('data-citation-id="1"');
    expect(mixed).not.toContain('data-citation-id="99"');
  });

});
