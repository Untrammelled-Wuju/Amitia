import { describe, expect, it } from "vitest";
import { parseRuntimeCapabilitiesPayload } from "@/runtime/runtime-capabilities";

describe("runtime capabilities", () => {
  it("parses the wrapped public capability response", () => {
    const snapshot = parseRuntimeCapabilitiesPayload({
      code: 200,
      data: {
        runtimeProfile: "local",
        capabilities: {
          gameMode: true,
          devicePluginRuntime: true,
          deviceExecutionPlane: true,
          localUIEndpoints: true,
        },
      },
    });

    expect(snapshot.runtimeProfile).toBe("local");
    expect(snapshot.capabilities.gameMode).toBe(true);
    expect(snapshot.loaded).toBe(true);
  });

  it("fails closed when fields are missing or non-boolean", () => {
    const snapshot = parseRuntimeCapabilitiesPayload({
      data: {
        runtimeProfile: "cloud-core",
        capabilities: {
          gameMode: "true",
          devicePluginRuntime: 1,
        },
      },
    });

    expect(snapshot.runtimeProfile).toBe("cloud-core");
    expect(snapshot.capabilities).toEqual({
      gameMode: false,
      devicePluginRuntime: false,
      deviceExecutionPlane: false,
      localUIEndpoints: false,
    });
  });

  it("never enables Game Mode for cloud-core even if a malformed server advertises it", () => {
    const snapshot = parseRuntimeCapabilitiesPayload({
      data: {
        runtimeProfile: "cloud-core",
        capabilities: {
          gameMode: true,
          devicePluginRuntime: true,
          deviceExecutionPlane: true,
          localUIEndpoints: true,
        },
      },
    });

    expect(snapshot.capabilities.gameMode).toBe(false);
  });

  it("fails closed for an unknown runtime profile", () => {
    const snapshot = parseRuntimeCapabilitiesPayload({
      data: {
        runtimeProfile: "future-profile",
        capabilities: {
          gameMode: true,
          devicePluginRuntime: true,
          deviceExecutionPlane: true,
          localUIEndpoints: true,
        },
      },
    });

    expect(snapshot.capabilities).toEqual({
      gameMode: false,
      devicePluginRuntime: false,
      deviceExecutionPlane: false,
      localUIEndpoints: false,
    });
  });

});
