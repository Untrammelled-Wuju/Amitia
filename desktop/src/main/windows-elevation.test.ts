import path from "node:path";
import { describe, expect, it } from "vitest";
import {
  quoteWindowsArguments,
  resolveWindowsPowerShellPath,
  shouldRequestAdministrator,
} from "./windows-elevation";

describe("quoteWindowsArguments", () => {
  it("quotes paths and arguments containing spaces", () => {
    expect(
      quoteWindowsArguments([
        "C:\\Program Files\\Amitia\\Amitia.exe",
        "--profile",
        "D:\\User Data\\AmitiaData",
      ]),
    ).toBe(
      '"C:\\Program Files\\Amitia\\Amitia.exe" --profile "D:\\User Data\\AmitiaData"',
    );
  });

  it("escapes embedded quotes and trailing backslashes", () => {
    expect(quoteWindowsArguments(['a"b', "C:\\path with spaces\\"])).toBe(
      '"a\\"b" "C:\\path with spaces\\\\"',
    );
  });

  it("requests elevation only for a non-elevated system-drive install", () => {
    expect(shouldRequestAdministrator("C:\\Amitia", "win32", false)).toBe(true);
    expect(shouldRequestAdministrator("C:\\Amitia", "win32", true)).toBe(false);
    expect(shouldRequestAdministrator("D:\\Amitia", "win32", false)).toBe(false);
    expect(shouldRequestAdministrator("C:\\Amitia", "darwin", false)).toBe(false);
  });

  it("prefers the packaged PowerShell runtime", () => {
    const resourcesPath = "C:\\Program Files\\Amitia\\resources";
    const bundledPath = path.win32.join(resourcesPath, "powershell", "pwsh.exe");
    expect(
      resolveWindowsPowerShellPath({
        resourcesPath,
        systemRoot: "C:\\Windows",
        exists: (candidate) => candidate === bundledPath,
      }),
    ).toBe(bundledPath);
  });

  it("prefers an explicit configured PowerShell path", () => {
    const configuredPath = "D:\\AmitiaRuntime\\pwsh.exe";
    expect(
      resolveWindowsPowerShellPath({
        configuredPath,
        resourcesPath: "C:\\Program Files\\Amitia\\resources",
        systemRoot: "C:\\Windows",
        exists: (candidate) => candidate === configuredPath,
      }),
    ).toBe(configuredPath);
  });

  it("falls back to Windows PowerShell when bundled runtime is unavailable", () => {
    expect(
      resolveWindowsPowerShellPath({
        configuredPath: "",
        resourcesPath: "C:\\Program Files\\Amitia\\resources",
        systemRoot: "C:\\Windows",
        exists: () => false,
      }),
    ).toBe("C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe");
  });
});
