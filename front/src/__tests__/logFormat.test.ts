import { describe, expect, it } from "vitest";
import { formatRecentLogText } from "@/views/logs/log-format";

describe("formatRecentLogText", () => {
  it("按日志文件分组输出原始行", () => {
    const text = formatRecentLogText([
      { file: "electron.20261101.001.log", line: "first line" },
      { file: "app.20261101.log.1", line: '{"@level":"warn"}' },
      { file: "electron.20261101.001.log", line: "second line" },
    ]);

    expect(text).toBe(
      [
        "==> electron.20261101.001.log <==",
        "first line",
        "second line",
        "",
        "==> app.20261101.log.1 <==",
        '{"@level":"warn"}',
      ].join("\n"),
    );
  });

  it("缺少文件名的日志归入 runtime.log", () => {
    expect(formatRecentLogText([{ line: "fallback" }])).toBe(
      "==> runtime.log <==\nfallback",
    );
  });
});
