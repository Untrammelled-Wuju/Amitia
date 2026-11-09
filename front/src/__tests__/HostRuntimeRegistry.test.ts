import { describe, expect, it } from "vitest";
import {
  HOST_RUNTIME_CHARACTER_PSYCHE,
  HOST_RUNTIME_CHARACTER_DETAIL,
  HOST_RUNTIME_CHARACTER_TEAMS,
  HOST_RUNTIME_CHARACTER_WORLDBOOK,
  HOST_RUNTIME_MEMORY_OVERVIEW,
  HOST_RUNTIME_MEMORY_TIMELINE,
  HOST_RUNTIME_MEMORY_EPISODIC,
  HOST_RUNTIME_MEMORY_GRAPH,
  HOST_RUNTIME_MEMORY_KNOWLEDGE,
  HOST_RUNTIME_PROFILE_SUMMARY,
  resolveHostRuntimeComponent,
} from "@/components/extension/hostRuntimeRegistry";

describe("host runtime registry", () => {
  it("resolves registered host runtimes without binding an extension id", () => {
    expect(resolveHostRuntimeComponent(HOST_RUNTIME_CHARACTER_PSYCHE)).not.toBeNull();
    expect(resolveHostRuntimeComponent(HOST_RUNTIME_CHARACTER_DETAIL)).not.toBeNull();
    expect(resolveHostRuntimeComponent(HOST_RUNTIME_CHARACTER_TEAMS)).not.toBeNull();
    expect(resolveHostRuntimeComponent(HOST_RUNTIME_CHARACTER_WORLDBOOK)).not.toBeNull();
    for (const runtimeId of [
      HOST_RUNTIME_MEMORY_OVERVIEW,
      HOST_RUNTIME_MEMORY_TIMELINE,
      HOST_RUNTIME_MEMORY_EPISODIC,
      HOST_RUNTIME_MEMORY_GRAPH,
      HOST_RUNTIME_MEMORY_KNOWLEDGE,
      HOST_RUNTIME_PROFILE_SUMMARY,
    ]) {
      expect(resolveHostRuntimeComponent(runtimeId)).not.toBeNull();
    }
  });

  it("rejects unknown host runtimes", () => {
    expect(resolveHostRuntimeComponent("host.character.unknown")).toBeNull();
    expect(resolveHostRuntimeComponent("")).toBeNull();
  });
});
