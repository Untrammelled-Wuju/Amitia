import { describe, expect, it } from "vitest";
import { trustedServicePath } from "../views/kernel/trusted-service-path";

describe("trusted service identity routing", () => {
  it("keeps compound identifiers out of path segments for every operation", () => {
    const id = "com.amitia/channel-qq/service?+%中文";
    for (const operation of [undefined, "start", "stop", "health", "status", "invoke", "quarantine/release"] as const) {
      const url = new URL(trustedServicePath(id, operation), "https://core.test");
      expect(url.pathname).toBe(`/api/extensions/services${operation ? `/${operation}` : ""}`);
      expect(url.searchParams.getAll("service_id")).toEqual([id]);
    }
  });
  it("preserves service names that match static operations and rejects empty identity", () => {
    expect(new URL(trustedServicePath("health"), "https://core.test").pathname).toBe("/api/extensions/services");
    expect(() => trustedServicePath(" ")).toThrow();
  });
});
