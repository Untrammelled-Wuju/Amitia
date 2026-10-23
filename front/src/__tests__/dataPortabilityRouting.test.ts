import { describe, expect, it } from "vitest";
import { isDeviceLocalApiPath } from "@/runtime/runtime-adapter";

describe("data portability routing", () => {
  it.each([
    "/api/data-portability/backups/backup-1.amitia-backup.enc",
    "/api/data-portability/restores/restore-1",
  ])("routes physical recovery path %s to the device Runtime", (path) => {
    expect(isDeviceLocalApiPath(path)).toBe(true);
  });

  it.each([
    "/api/data-portability/exports/preview",
    "/api/data-portability/imports/import-1/commit",
  ])("routes migration path %s to the active data authority", (path) => {
    expect(isDeviceLocalApiPath(path)).toBe(false);
  });

  it("does not classify lookalike paths as local", () => {
    expect(isDeviceLocalApiPath("/api/data-portability-admin/exports")).toBe(false);
  });
});
