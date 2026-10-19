import { describe, expect, it } from "vitest";
import { buildCoreSpawnArgs, parseCoreChildPid } from "./core-supervisor";

describe("core supervisor protocol", () => {
  it("uses supervisor mode on windows", () => {
    expect(buildCoreSpawnArgs("win32", "local")).toEqual(["--supervise", "--runtime-profile=local"]);
    expect(buildCoreSpawnArgs("linux", "local")).toEqual(["--runtime-profile=local"]);
  });

  it("parses the child pid line and rejects unrelated output", () => {
    expect(parseCoreChildPid("AMITIA_CORE_PID=4321")).toBe(4321);
    expect(parseCoreChildPid("AMITIA_CORE_PID=0")).toBeNull();
    expect(parseCoreChildPid("[AmitiaCore] ready")).toBeNull();
  });
});
