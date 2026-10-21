import { describe, expect, it } from "vitest";
import { projectOwnedRuntimeState } from "../runtime/owned-runtime-state";

describe("Owned 角色运行状态投影", () => {
  it("使用 Owner 账本状态生成现有心理页数据", () => {
    const view = projectOwnedRuntimeState({
      updatedAt: "2026-10-21T02:00:00Z",
      psyche: { valence: 0.7, arousal: 0.4, dominance: 0.6, moodValence: 0.65, moodArousal: 0.2, stress: 0.1, fatigue: 0.25 },
      relationship: { trust: 0.7, familiarity: 0.4, security: 0.6, tension: 0.1, repairConfidence: 0.8, boundary: 0.5 },
      needs: [{ kind: "connection", level: 0.55 }],
      beliefs: [{ key: "偏好", value: "茶", confidence: 0.9 }],
    });
    expect(view.emotion.positive).toBe(0.7);
    expect(view.energy).toBe(0.75);
    expect(view.needs.connection).toBe(0.55);
    expect(view.relationship.familiarity).toBe(0.4);
    expect(view.beliefs[0].key).toBe("偏好");
    expect(view.affectLabel).toBe("积极");
  });
});
