import { describe, expect, it } from "vitest";
import { buildProviderRouteContext, resolveRouteIdentity } from "@/ui-runtime/routeContext";

describe("provider route context", () => {
  it("normalizes a character route id into characterId", () => {
    expect(resolveRouteIdentity({ id: "role-1" })).toEqual({
      characterId: "role-1",
      conversationId: "",
    });
  });

  it("preserves explicit identity fields over route id", () => {
    expect(resolveRouteIdentity({ id: "route-role", characterId: "explicit-role" }).characterId)
      .toBe("explicit-role");
  });

  it("builds provider context with both route params and canonical identity", () => {
    expect(buildProviderRouteContext("/characters/role-1/active-task", { id: "role-1" }, { tab: "rules" }))
      .toMatchObject({
        route: "/characters/role-1/active-task",
        routeParams: { id: "role-1" },
        routeQuery: { tab: "rules" },
        characterId: "role-1",
      });
  });
});
