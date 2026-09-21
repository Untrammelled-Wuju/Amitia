import { createApp, h } from "vue";
import ElementPlus from "element-plus";
import "element-plus/dist/index.css";
import "../styles/theme-light.css";
import "../styles/theme-dark.css";
import "../styles/variables.css";
import "../styles/element-overrides.css";
import AIMessageRenderer from "@/conversation/rendering/AIMessageRenderer.vue";

const params = new URLSearchParams(window.location.search);
const theme = params.get("theme") === "dark" ? "dark" : "light";
document.documentElement.setAttribute("data-theme", theme);

const markdown = [
  "# Amitia Message Rendering Engine",
  "",
  "同一条 AI 回复里包含 **正文**、*斜体*、~~删除线~~、[安全链接](https://example.com)、行内代码 `const ok = true` 和引用 [1]。",
  "",
  "## Code",
  "",
  "```ts filename=stream_message_store.ts",
  "export function applyChunk(messageId: string, delta: string) {",
  '  const current = streamBuffers.get(messageId) ?? "";',
  "  const content = current + delta;",
  "  streamBuffers.set(messageId, content);",
  "  patchMessage(messageId, { content });",
  "}",
  "```",
  "",
  "```diff filename=ChatBubble.vue",
  "- localText.value += event.delta",
  "- updateMessage(event.id, localText.value)",
  "+ streamStore.applyChunk(event.id, event.delta)",
  "  scrollToBottom()",
  "```",
  "",
  "```terminal filename=Terminal",
  "$ pnpm run build",
  "vite v7.1.2 building for production...",
  "✓ 184 modules transformed.",
  "error TS2322: Type 'null' is not assignable to type 'string'.",
  "exit code: 1",
  "```",
  "",
  "## 表格与任务",
  "",
  "| 内容 | 表现 | 是否单独成卡 |",
  "| --- | --- | --- |",
  "| 正文 / Markdown | 直接排版 | 否 |",
  "| 代码 / Diff | 深色嵌入块 | 否 |",
  "| Tool Call | 轻量状态行 | 否 |",
  "",
  "- [x] 已完成任务",
  "- [ ] 未完成任务",
  "",
  "> 逻辑上是一整条回复，视觉上没有 AI 外层大卡片。",
  "",
  "## LaTeX",
  "",
  "行内公式 $E=mc^2$，块级公式：",
  "",
  "$$",
  "\\int_0^\\infty e^{-x^2}\\,dx=\\frac{\\sqrt{\\pi}}{2}",
  "$$",
  "",
  "## Mermaid",
  "",
  "```mermaid",
  "graph LR",
  "A[LLM Stream] --> B[Renderer]",
  "B --> C[Unified Message UI]",
  "```",
  "",
  "## HTML Preview",
  "",
  "```html-preview",
  "<main><h1>Hello Amitia</h1><p>Sandboxed HTML Preview</p></main>",
  "```",
].join("\n");

const message = {
  id: "acceptance-message",
  role: "assistant",
  createdAt: "2026-09-20T20:41:00+08:00",
  content: markdown,
  status: "completed",
  reasoningContent: "先确认消息聚合入口，再重构双端 Renderer。",
  sources: [
    {
      id: "1",
      title: "project/source/chat_renderer.ts",
      url: "https://example.com",
      snippet: "RendererRegistry 根据 rendererId 选择对应组件；未知类型进入 fallback。",
    },
  ],
  blocks: [
    {
      kind: "tool",
      id: "tool-success",
      name: "读取文件",
      arguments: { path: "ChatBubble.vue" },
      result: { status: "success" },
      status: "success",
      duration: 48,
    },
    {
      kind: "tool",
      id: "tool-running",
      name: "执行命令",
      arguments: { cwd: "/workspace/app", timeout: 120 },
      status: "running",
    },
    {
      kind: "tool",
      id: "tool-failed",
      name: "读取远程资源",
      arguments: { endpoint: "API /models" },
      error: "Endpoint unavailable",
      status: "failed",
    },
    {
      kind: "agent-task",
      id: "agent",
      title: "修复消息渲染链路",
      status: "running",
      progress: 68,
      elapsed: "3.2s",
      steps: [
        { id: "1", title: "读取聊天相关源码", status: "done", meta: "完成" },
        { id: "2", title: "定位重复拼接逻辑", status: "done", meta: "完成" },
        { id: "3", title: "修改双端 Renderer", status: "running", meta: "进行中" },
        { id: "4", title: "生成最终补丁", status: "pending", meta: "等待" },
      ],
    },
    {
      kind: "artifact",
      id: "artifact",
      artifactKind: "HTML",
      title: "聊天页面重构预览",
      mimeType: "text/html",
      content: "<main><h1>Artifact</h1><p>Interactive preview</p></main>",
      size: 28600,
    },
    {
      kind: "extension",
      id: "extension",
      rendererId: "minecraft.server",
      version: 2,
      payload: {
        title: "Minecraft 服务器已连接",
        player: "Steve",
        address: "192.168.1.20:25565",
        tps: 18,
      },
    },
    {
      kind: "unknown",
      id: "fallback",
      type: "weather.radar.v2",
      payload: { rendererId: "weather.radar.v2", version: 2, payload: { location: "demo" } },
    },
  ],
};

const userMessage = {
  id: "acceptance-user",
  role: "user",
  createdAt: "2026-09-20T20:40:00+08:00",
  content: "帮我看一下这段流式消息为什么会重复渲染，然后直接给我修改后的代码。",
};

createApp({
  setup() {
    return () =>
      h("main", { class: "acceptance-page" }, [
        h(AIMessageRenderer, {
          message: userMessage,
          showAvatar: false,
          readOnly: true,
        }),
        h(AIMessageRenderer, {
          message,
          charName: "林澈",
          characterId: "lin-che",
        }),
      ]);
  },
})
  .use(ElementPlus)
  .mount("#app");

const style = document.createElement("style");
style.textContent = `
  html, body, #app { min-height: 100%; }
  body { margin: 0; background: ${theme === "dark" ? "#17181b" : "#f7f7f8"}; }
  .acceptance-page { width: min(900px, 100%); margin: 0 auto; padding: 30px 18px 90px; }
`;
document.head.appendChild(style);
