import type { AIMessageData, AssistantTurnData, AssistantTurnItem, RichBlock } from "./types";

export const bubbleSeparator = "[AMITIA_BR]";

export function isBubbleContentComplete(status: string): boolean {
  return ["completed", "complete", "sent", "delivered", "success", "succeeded", "failed", "error", "interrupted", "cancelled", "canceled"].includes(status.toLowerCase());
}

export function splitBubbleText(content: string, complete: boolean): string[] {
  const result: string[] = [];
  let start = 0;
  let code = "";
  for (let i = 0; i < content.length;) {
    const char = content[i];
    if (char === "`" || char === "~") {
      let end = i + 1;
      while (content[end] === char) end++;
      const delimiter = content.slice(i, end);
      if (char === "`" || delimiter.length >= 3) {
        if (!code) code = delimiter;
        else if (code[0] === char && delimiter.length >= code.length) code = "";
      }
      i = end;
      continue;
    }
    if (!code && content.startsWith(bubbleSeparator, i)) {
      const text = content.slice(start, i).trim();
      if (text) result.push(text);
      i += bubbleSeparator.length;
      start = i;
      continue;
    }
    i++;
  }
  if (complete) {
    let tail = content.slice(start).trim();
    if (!code) {
      for (let size = bubbleSeparator.length - 1; size >= 4; size--) {
        if (tail.endsWith(bubbleSeparator.slice(0, size))) {
          tail = tail.slice(0, -size).trim();
          break;
        }
      }
    }
    if (tail) result.push(tail);
  }
  return result;
}

export function flowBubbleText(content: string): string {
  return content.includes(bubbleSeparator) ? splitBubbleText(content, true).join("\n\n") : content;
}

export type BubbleContentItem =
  | { key: string; kind: "text"; content: string }
  | { key: string; kind: "turn"; turn: AssistantTurnData }
  | { key: string; kind: "thinking"; content: string; complete: boolean; duration?: number }
  | { key: string; kind: "block"; block: RichBlock };

export function projectBubbleContent(message: AIMessageData, turn?: AssistantTurnData | null): BubbleContentItem[] {
  const output: BubbleContentItem[] = [];
  const turnComplete = !!turn && isBubbleContentComplete(turn.status);
  if (turn?.items.length) {
    const ordered = [...turn.items].sort((a, b) => a.sequence - b.sequence);
    const tools = new Map<string, AssistantTurnItem[]>();
    for (const item of ordered) {
      if (item.type === "tool_call" || item.type === "tool_result") {
        const key = item.callId || item.id;
        const group = tools.get(key) || [];
        group.push(item);
        tools.set(key, group);
      }
    }
    const emittedTools = new Set<string>();
    for (const item of ordered) {
      const complete = turnComplete || isBubbleContentComplete(item.status);
      if (item.type === "text") {
        splitBubbleText(item.content || "", complete).forEach((content, index) => {
          output.push({ key: `${item.id}:text:${index}`, kind: "text", content });
        });
      } else if (item.type === "reasoning") {
        output.push({ key: item.id, kind: "thinking", content: complete ? item.content || "" : "", complete, duration: (item.durationMs || 0) / 1000 });
      } else if (item.type === "tool_call" || item.type === "tool_result") {
        const key = item.callId || item.id;
        if (emittedTools.has(key)) continue;
        emittedTools.add(key);
        output.push({ key: `tool:${key}`, kind: "turn", turn: { ...turn, items: tools.get(key)! } });
      }
    }
  } else {
    const complete = message.role !== "assistant" || isBubbleContentComplete(message.state) || turnComplete;
    if (message.thinking) {
      output.push({ key: `${message.id}:thinking`, kind: "thinking", content: complete ? message.thinking.content : "", complete, duration: message.thinking.duration });
    }
    const content = message.blocks.length && /^\[(语音|图片|视频|文件)\]$/.test(message.markdown.trim()) ? "" : message.markdown;
    const texts = message.role === "user" ? (content.trim() ? [content] : []) : splitBubbleText(content, complete);
    texts.forEach((text, index) => output.push({ key: `${message.id}:text:${index}`, kind: "text", content: text }));
  }
  for (const block of message.blocks) {
    if (turn?.items.length && block.kind === "tool") continue;
    output.push({ key: `${message.id}:block:${block.id}`, kind: "block", block });
  }
  return output;
}
