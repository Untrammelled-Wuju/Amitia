import { describe, expect, it } from "vitest";
import { ClientDiagnosticsBuffer } from "./client-diagnostics";

describe("client diagnostics buffer", () => {
  it("keeps only the newest errors", () => {
    const buffer = new ClientDiagnosticsBuffer(2);
    buffer.record({ message: "first", source: "electron:main" });
    buffer.record({ message: "second", source: "electron:main" });
    buffer.record({ message: "third", source: "electron:main" });
    expect(buffer.list().map((item) => item.message)).toEqual(["second", "third"]);
  });

  it("stores stack and clears explicitly", () => {
    const buffer = new ClientDiagnosticsBuffer();
    buffer.record({
      message: "renderer crashed",
      stack: "Error: renderer crashed\n at main.ts:1:1",
      source: "electron:render-process-gone",
      at: "2026-10-22T22:00:00+08:00",
    });
    expect(buffer.list()[0]).toMatchObject({
      message: "renderer crashed",
      source: "electron:render-process-gone",
      at: "2026-10-22T22:00:00+08:00",
    });
    buffer.clear();
    expect(buffer.list()).toEqual([]);
  });
});
