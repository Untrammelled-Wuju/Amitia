import { describe, expect, it } from "vitest";
import { ownedMemoryDecayLabel, ownedMemoryScoreSummary, ownedMemoryVerificationLabel } from "@/runtime/owned-memory-format";

describe("owned memory lifecycle labels", () => {
  it.each([
    [{ effectiveDecayState: "active" }, "活跃"],
    [{ effectiveDecayState: "fading" }, "淡化"],
    [{ effectiveDecayState: "dormant" }, "休眠 · 可检索"],
    [{ effectiveDecayState: "archived" }, "自动归档 · 可检索"],
    [{ effectiveDecayState: "dormant", pinned: true }, "已固定"],
  ])("maps lifecycle state %#", (body, expected) => {
    expect(ownedMemoryDecayLabel(body)).toBe(expected);
  });

  it("explains verification and retrieval scores", () => {
    expect(ownedMemoryVerificationLabel({ verifiedStatus: "user_corrected" })).toBe("用户已更正");
    expect(ownedMemoryVerificationLabel({ verifiedStatus: "disputed" })).toBe("有争议");
    expect(ownedMemoryScoreSummary({ finalScore: .82, semanticScore: .73, keywordMatch: true, importance: .8, confidence: 1, strength: .68, validity: 1, verification: 1 })).toContain("关键词 命中");
    expect(ownedMemoryScoreSummary({ finalScore: .82, semanticScore: .73, keywordMatch: true, importance: .8, confidence: 1, strength: .68, validity: 1, verification: 1 })).toContain("时效 100");
  });
});
