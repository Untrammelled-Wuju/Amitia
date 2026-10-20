import { describe, expect, it } from "vitest";
import { projectCitationSnapshot } from "@/conversation/runtime/citationProjection";

const oldCitation = [{ marker: "[K1]", kind: "knowledge", index: 1 }];
const newCitation = [{ marker: "[K2]", kind: "knowledge", index: 2 }];

describe("Phase 46 authoritative citation projection", () => {
  it("replaces historical evidence when a newer completed block contains citations", () => {
    expect(projectCitationSnapshot(oldCitation, { citationSnapshot: newCitation })).toEqual(newCitation);
  });
  it("clears stale historical citations when the completed block explicitly has none", () => {
    expect(projectCitationSnapshot(oldCitation, { citationSnapshot: [] })).toEqual([]);
  });
  it("preserves persisted citations when the block omits the field", () => {
    expect(projectCitationSnapshot(oldCitation, {})).toEqual(oldCitation);
    expect(projectCitationSnapshot(oldCitation, null)).toEqual(oldCitation);
    expect(projectCitationSnapshot(oldCitation, undefined)).toEqual(oldCitation);
  });
  it("permits an empty historical message to receive the final citation set", () => {
    expect(projectCitationSnapshot([], { citationSnapshot: newCitation })).toEqual(newCitation);
  });
  it("does not manufacture citations from an absent final message", () => {
    expect(projectCitationSnapshot(undefined, undefined)).toBeUndefined();
  });
});
