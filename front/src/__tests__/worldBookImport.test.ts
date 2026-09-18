import { describe, expect, it } from "vitest";
import {
  importExampleJson,
  parseWorldBookImport,
} from "@/views/world-book/worldBookImport";

describe("world book json import", () => {
  it("parses the built-in example as valid import rows", () => {
    const rows = parseWorldBookImport(importExampleJson);

    expect(rows).toHaveLength(1);
    expect(rows.every((row) => row.valid)).toBe(true);
    expect(rows[0].item?.matchType).toBe("keyword");
  });

  it("defaults optional fields", () => {
    const rows = parseWorldBookImport(
      JSON.stringify([
        {
          matchType: "exact",
          matchPattern: "世界树",
          injectContent: "世界树位于大陆中央。",
        },
      ]),
    );

    expect(rows[0].valid).toBe(true);
    expect(rows[0].item).toMatchObject({
      matchScope: "full_context",
      priority: 0,
      characterId: "",
    });
  });

  it("rejects invalid regex patterns", () => {
    const rows = parseWorldBookImport(
      JSON.stringify([
        {
          matchType: "regex",
          matchPattern: "[",
          injectContent: "无效正则",
        },
      ]),
    );

    expect(rows[0].valid).toBe(false);
    expect(rows[0].message).toContain("正则");
  });

  it("requires a top-level array", () => {
    expect(() => parseWorldBookImport("{}")).toThrow("顶层必须是数组");
  });

  it("embeds every supported field description in the example json", () => {
    const example = JSON.parse(importExampleJson);

    expect(Object.keys(example[0].__说明)).toEqual([
      "matchType",
      "matchPattern",
      "matchScope",
      "injectContent",
      "priority",
      "characterId",
    ]);
  });
});
