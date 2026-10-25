import { describe, expect, it, vi } from "vitest";

vi.mock("electron", () => ({
  app: { isPackaged: false },
  BrowserWindow: class {},
  ipcMain: { handle: vi.fn() },
  session: { fromPartition: vi.fn() },
}));

import { externalWebUIProviderAuthorized, validateExternalWebUIRequest } from "./external-web-ui";

describe("external web UI request policy", () => {
  it("accepts an HTTP target with an explicit allowed origin", () => {
    expect(validateExternalWebUIRequest({
      providerId: "service-webui",
      extensionId: "com.example/service",
      url: "http://127.0.0.1:6099/webui/",
      allowedOrigins: ["http://127.0.0.1:6099"],
      storageKey: "com.example/service",
      persistent: true,
    })).toEqual({
      providerId: "service-webui",
      extensionId: "com.example/service",
      url: "http://127.0.0.1:6099/webui/",
      allowedOrigins: ["http://127.0.0.1:6099"],
      storageKey: "com.example/service",
      persistent: true,
    });
  });

  it("rejects credentials, unsupported schemes and missing target origins", () => {
    expect(() => validateExternalWebUIRequest({
      providerId: "service-webui",
      extensionId: "com.example/service",
      url: "http://user:pass@127.0.0.1:6099/webui/",
      allowedOrigins: ["http://127.0.0.1:6099"],
      storageKey: "service",
    })).toThrow("without credentials");
    expect(() => validateExternalWebUIRequest({
      providerId: "service-webui",
      extensionId: "com.example/service",
      url: "file:///tmp/webui.html",
      allowedOrigins: ["file:///"],
      storageKey: "service",
    })).toThrow("HTTP(S)");
    expect(() => validateExternalWebUIRequest({
      providerId: "service-webui",
      extensionId: "com.example/service",
      url: "http://127.0.0.1:6099/webui/",
      allowedOrigins: ["http://127.0.0.1:6100"],
      storageKey: "service",
    })).toThrow("target origin must be allowed");
  });

  it("requires an enabled, trusted provider entry with matching origins", () => {
    const request = validateExternalWebUIRequest({
      providerId: "service-webui",
      extensionId: "com.example/service",
      url: "http://127.0.0.1:6099/webui/",
      allowedOrigins: ["http://127.0.0.1:6099"],
      storageKey: "com.example/service",
      persistent: true,
    });
    const provider = {
      providerId: "service-webui",
      extensionId: "com.example/service",
      capability: "workspace.tab",
      enabled: true,
      trustLevel: "official",
      entries: {
        electron_macos: {
          type: "external_web_ui",
          url: "http://127.0.0.1:6099/webui/",
          allowedOrigins: ["http://127.0.0.1:6099"],
        },
      },
    };
    expect(externalWebUIProviderAuthorized(provider, "darwin", request)).toBe(true);
    expect(externalWebUIProviderAuthorized({ ...provider, enabled: false }, "darwin", request)).toBe(false);
    expect(externalWebUIProviderAuthorized({
      ...provider,
      entries: {
        electron_macos: {
          type: "external_web_ui",
          url: "http://127.0.0.1:6100/webui/",
          allowedOrigins: ["http://127.0.0.1:6100"],
        },
      },
    }, "darwin", request)).toBe(false);
  });
});
