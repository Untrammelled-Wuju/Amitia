import { describe, expect, it } from "vitest";

import {
  DeploymentConfigError,
  configToLabel,
  validateDeploymentConfigForSave,
} from "./deployment";

describe("deployment endpoint normalization", () => {
  it("uses http for private endpoints without an explicit scheme", () => {
    expect(
      validateDeploymentConfigForSave({
        mode: "cloud",
        serverURL: "192.168.1.10:18899",
      }),
    ).toEqual({ mode: "cloud", serverURL: "http://192.168.1.10:18899" });
  });

  it("uses https for public endpoints without an explicit scheme", () => {
    expect(
      validateDeploymentConfigForSave({
        mode: "cloud",
        serverURL: "cloud.example.com",
      }),
    ).toEqual({ mode: "cloud", serverURL: "https://cloud.example.com" });
  });

  it("keeps public hostnames beginning with fc/fd on https", () => {
    expect(
      validateDeploymentConfigForSave({ mode: "cloud", serverURL: "fcloud.example.com" }),
    ).toEqual({ mode: "cloud", serverURL: "https://fcloud.example.com" });
    expect(
      validateDeploymentConfigForSave({ mode: "cloud", serverURL: "fdomain.example.com" }),
    ).toEqual({ mode: "cloud", serverURL: "https://fdomain.example.com" });
  });

  it("canonicalizes default ports", () => {
    expect(
      validateDeploymentConfigForSave({ mode: "cloud", serverURL: "https://cloud.example.com:443" }),
    ).toEqual({ mode: "cloud", serverURL: "https://cloud.example.com" });
    expect(
      validateDeploymentConfigForSave({ mode: "cloud", serverURL: "http://192.168.1.10:80" }),
    ).toEqual({ mode: "cloud", serverURL: "http://192.168.1.10" });
  });

  it("rejects non-root URLs instead of rewriting them", () => {
    expect(() =>
      validateDeploymentConfigForSave({
        mode: "cloud",
        serverURL: "https://cloud.example.com/core",
      }),
    ).toThrow(DeploymentConfigError);
  });

  it("exposes only the two user-facing deployment modes", () => {
    expect(configToLabel({ mode: "local" })).toBe("本地模式");
    expect(configToLabel({ mode: "cloud", serverURL: "https://cloud.example.com" })).toBe(
      "云端统筹 (https://cloud.example.com)",
    );
  });

  it.each([
    "https://cloud.example.com?tenant=a",
    "https://cloud.example.com#fragment",
    "https://user:pass@cloud.example.com",
  ])("rejects unsafe endpoint metadata: %s", (serverURL) => {
    expect(() =>
      validateDeploymentConfigForSave({ mode: "cloud", serverURL }),
    ).toThrow(DeploymentConfigError);
  });
});
