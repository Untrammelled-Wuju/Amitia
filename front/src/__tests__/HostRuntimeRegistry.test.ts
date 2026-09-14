import { describe, expect, it } from "vitest";
import {
  HOST_RUNTIME_CHARACTER_PSYCHE,
  resolveHostRuntimeComponent,
} from "@/components/extension/hostRuntimeRegistry";

describe("host runtime registry", () => {
  it("resolves registered host runtimes without binding an extension id", () => {
    expect(resolveHostRuntimeComponent(HOST_RUNTIME_CHARACTER_PSYCHE)).not.toBeNull();
  });

  it("rejects unknown host runtimes", () => {
    expect(resolveHostRuntimeComponent("host.character.unknown")).toBeNull();
    expect(resolveHostRuntimeComponent("")).toBeNull();
  });
});
