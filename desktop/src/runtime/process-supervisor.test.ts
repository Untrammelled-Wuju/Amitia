import { describe, expect, it } from "vitest";
import { InMemoryProcessSupervisor } from "./process-supervisor";

describe("InMemoryProcessSupervisor", () => {
  it("starts process into running state", async () => {
    const supervisor = new InMemoryProcessSupervisor();
    const status = await supervisor.start({
      id: "core",
      executable: "server.exe",
      args: [],
      cwd: "D:/runtime",
      env: {},
      logFile: "D:/runtime/server.log",
    });
    expect(status).toEqual({
      id: "core",
      state: "running",
      pid: 0,
    });
  });

  it("stops process into stopped state", async () => {
    const supervisor = new InMemoryProcessSupervisor();
    await supervisor.start({
      id: "core",
      executable: "server.exe",
      args: [],
      cwd: "D:/runtime",
      env: {},
      logFile: "D:/runtime/server.log",
    });
    await expect(supervisor.stop("core")).resolves.toEqual({
      id: "core",
      state: "stopped",
      exitCode: 0,
    });
  });

  it("marks failure state", () => {
    const supervisor = new InMemoryProcessSupervisor();
    expect(supervisor.fail("core", "boom", 2)).toEqual({
      id: "core",
      state: "failed",
      error: "boom",
      exitCode: 2,
    });
  });
});
