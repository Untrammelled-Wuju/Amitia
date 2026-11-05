import { describe, expect, it } from "vitest";
import { highlightWorldBookMatch } from "@/views/world-book/worldBookHighlight";

describe("world book match highlighting", () => {
  it("escapes injected html while preserving the highlight", () => {
    const text = '<img src=x onerror="document.body.dataset.xss=1">';

    expect(highlightWorldBookMatch(text, text)).toBe(
      "<mark>&lt;img src=x onerror=&quot;document.body.dataset.xss=1&quot;&gt;</mark>",
    );
  });

  it("escapes unmatched text and treats the pattern literally", () => {
    expect(highlightWorldBookMatch("a.b <tag> & value", "a.b")).toBe(
      "<mark>a.b</mark> &lt;tag&gt; &amp; value",
    );
  });

  it("returns empty output for missing text", () => {
    expect(highlightWorldBookMatch(null, "<script>")).toBe("");
  });
});
