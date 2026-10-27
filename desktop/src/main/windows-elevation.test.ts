import { describe, expect, it } from "vitest";
import {
  quoteWindowsArguments,
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
});
