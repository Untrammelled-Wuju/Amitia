import { describe, expect, it } from "vitest";
import { AgentEventReducer, type AgentUIEvent } from "../conversation/runtime/agentEventReducer";
import { normalizeCitationSnapshot } from "../conversation/runtime/citationSnapshot";
import { normalizeAIMessage } from "../conversation/rendering/amrp";
import { renderMarkdownSegment } from "../conversation/rendering/markdown/markdownEngine";

const evidence = [
  { marker: "[K1]", kind: "knowledge", index: 1, title: "内部文档", sourceId: "doc-1", sourceRevision: 2, unitId: "p-37", excerpt: "第37页" },
  { marker: "[1]", kind: "web", index: 1, title: "外部网页", url: "https://example.com/evidence", excerpt: "网页证据" },
];

function event(seq: number, type: string, overrides: Partial<AgentUIEvent> = {}): AgentUIEvent {
  return {
    version: 1, eventId: `e-${seq}`, eventSequence: seq, conversationId: "conversation-1",
    turnId: "turn-1", requestId: "request-1", executionId: "execution-1", turnSequence: 1,
    type, status: "running", createdAt: `2026-10-16T01:00:0${seq}Z`, ...overrides,
  };
}

describe("real-time citation snapshot", () => {
  it("normalizes bounded independent marker spaces and rejects untrusted records", () => {
    expect(normalizeCitationSnapshot([...evidence, evidence[0],
      { marker: "[K999]", kind: "knowledge", index: 1, excerpt: "inconsistent" },
      { marker: "[K2]", kind: "knowledge", index: "2" }, null, { marker: "[3]", kind: "web", index: 3, excerpt: "x".repeat(2000) },
    ])).toMatchObject([evidence[0], evidence[1], { marker: "[3]", excerpt: "x".repeat(1200) }]);
    expect(normalizeCitationSnapshot({ wrong: true })).toEqual([]);
    expect(normalizeCitationSnapshot(Array.from({ length: 80 }, (_, index) => ({ marker: `[${index + 1}]`, kind: "web", index: index + 1 })))).toHaveLength(24);
    expect(normalizeCitationSnapshot([{ marker: "[K1]", kind: "knowledge", index: 1, excerpt: `  ${"😀".repeat(1201)}  ` }])[0].excerpt).toBe("😀".repeat(1200));
  });

  it("never forwards unsafe links, local paths or token-bearing locator metadata", () => {
    const injected = [
      { marker: "[K1]", kind: "knowledge", index: 1, title: "Private", locator: {
        page: 7, row: "12", kind: "pdf_page", path: "/home/private/secret.pdf", token: "private", field: "C:\\secret\\key", nested: { accessToken: "private" },
      }, url: "javascript:alert(1)", excerpt: "e".repeat(3000) },
      { marker: "[1]", kind: "web", index: 1, url: "https://example.com/article?query=keep&api_key=secret&access_token=private#part" },
      { marker: "[2]", kind: "web", index: 2, url: "https://user:password@example.com/private" },
    ];
    const normalized = normalizeCitationSnapshot(injected);
    expect(normalized[0].locator).toEqual({ page: 7, row: 12, kind: "pdf_page" });
    expect(normalized[0].url).toBeUndefined();
    expect(normalized[0].excerpt).toHaveLength(1200);
    expect(normalized[1].url).toBe("https://example.com/article?query=keep#part");
    expect(normalized[2].url).toBeUndefined();
    const rendered = normalizeAIMessage({ id: "malicious-message", role: "assistant", content: "[K1] [1]", citationSnapshot: injected });
    expect(rendered.sources[0].url).toBeUndefined();
    expect(rendered.sources[1].url).toBe("https://example.com/article?query=keep#part");
    expect(JSON.stringify(rendered.sources)).not.toContain("access_token");
  });

  it("retains safe spreadsheet names and parser aliases without exposing paths", () => {
    const got = normalizeCitationSnapshot([
      { marker: "[K1]", kind: "knowledge", index: 1, locator: { sheetName: "年度汇总 2026", rowStart: 146, rowEnd: 150, path: "/private" } },
      { marker: "[K2]", kind: "knowledge", index: 2, locator: { pageNumber: 37, password: "secret" } },
      { marker: "[K3]", kind: "knowledge", index: 3, locator: { paragraphIndex: 86 } },
      { marker: "[K4]", kind: "knowledge", index: 4, locator: { sheetName: "C:\\private\\file", rowStart: 12 } },
      { marker: "[K5]", kind: "knowledge", index: 5, locator: { sheetName: "2026", rowStart: 32 } },
    ]);
    expect(got.map((item) => item.locator)).toEqual([
      { sheet: "年度汇总 2026", row: 146 }, { page: 37 }, { paragraph: 86 }, { row: 12 }, { sheet: "2026", row: 32 },
    ]);
    expect(normalizeAIMessage({ role: "assistant", content: "[K1]", citationSnapshot: got }).sources).toHaveLength(5);
  });

  it("delivers same citation markers in final live block, replay and historical message", () => {
    const reducer = new AgentEventReducer();
    expect(reducer.apply(event(1, "turn.started"))).toBe("applied");
    const completed = event(2, "text.completed", {
      blockId: "text-1", blockSequence: 1, revision: 2, messageId: "message-1", status: "completed",
      payload: { blockType: "text", content: "文档 [K1] 和网站 [1]，未知 [K9]", citationSnapshot: evidence, recoveryCheckpoint: true },
    });
    expect(reducer.apply(completed)).toBe("applied");
    expect(reducer.apply(completed)).toBe("duplicate");
    expect(reducer.apply(event(1, "text.delta", { eventId: "late", blockId: "text-1", payload: { delta: "late" } }))).toBe("stale");
    const finalItem = reducer.turn("turn-1")!.items[0];
    expect(finalItem.citationSnapshot).toEqual(evidence);
    const live = normalizeAIMessage({ id: finalItem.messageId, role: "assistant", content: finalItem.content, citationSnapshot: finalItem.citationSnapshot });
    const historic = normalizeAIMessage({ id: "message-1", role: "assistant", content: finalItem.content, citationSnapshot: evidence });
    expect(live.sources).toEqual(historic.sources);
    expect(live.sources.map((source) => source.id)).toEqual(["K1", "1"]);
    const html = renderMarkdownSegment(finalItem.content || "", live.sources.map((source) => source.id));
    expect(html).toContain('data-citation-id="K1"');
    expect(html).toContain('data-citation-id="1"');
    expect(html).not.toContain('data-citation-id="K9"');
    expect(reducer.apply(event(3, "turn.completed", { messageId: "message-1", status: "completed" }))).toBe("applied");
    expect(reducer.turn("turn-1")!.items[0].citationSnapshot).toEqual(evidence);
    const replay = new AgentEventReducer();
    expect(replay.apply(event(1, "turn.started"))).toBe("applied");
    expect(replay.apply(completed)).toBe("applied");
    expect(replay.turn("turn-1")!.items[0].citationSnapshot).toEqual(reducer.turn("turn-1")!.items[0].citationSnapshot);
  });

  it("preserves final citations when newer turn history lacks the ephemeral field", () => {
    const reducer = new AgentEventReducer();
    reducer.apply(event(1, "turn.started"));
    reducer.apply(event(2, "text.completed", {
      blockId: "text-1", revision: 2, blockSequence: 1, messageId: "message-1",
      payload: { content: "[K1]", citationSnapshot: [evidence[0]] },
    }));
    const turn = reducer.turn("turn-1")!;
    reducer.mergeTurns([{ ...turn, items: turn.items.map((item) => ({ ...item, revision: 3, citationSnapshot: undefined })) }]);
    expect(reducer.turn("turn-1")!.items[0].citationSnapshot).toEqual([evidence[0]]);
    reducer.reset([], 0, { turnId: "turn-1", status: "running", lastEventSequence: 0, blocks: [{ blockId: "text-1", blockSequence: 1, type: "text", status: "completed", revision: 2, payload: { citationSnapshot: [evidence[0]] } }] });
    expect(reducer.turn("turn-1")!.items[0].citationSnapshot).toEqual([evidence[0]]);
  });
});
