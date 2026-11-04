import { describe, expect, it } from "vitest";
import { taskPauseControls } from "../views/kernel/tasks/control";
import type { TaskRun, TaskRunStatus } from "../views/kernel/tasks/types";

const task = { generation: 2, checkpointId: "owner-checkpoint", checkpointSupported: true } as TaskRun;

describe("task pause controls", () => {
  it.each(["local", "device"] as const)("permits confirmed %s execution controls", (executionPlacement) => {
    expect(taskPauseControls({ ...task, executionPlacement, status: "running" })).toEqual({ pause: true, resume: false });
    expect(taskPauseControls({ ...task, executionPlacement, status: "checkpointing" }).pause).toBe(true);
    expect(taskPauseControls({ ...task, executionPlacement, status: "paused" })).toEqual({ pause: false, resume: true });
  });
  it.each(["pausing", "resuming", "cancelling", "recovery_required", "succeeded"] as TaskRunStatus[])("blocks premature controls in %s", (status) => {
    expect(taskPauseControls({ ...task, executionPlacement: "device", status })).toEqual({ pause: false, resume: false });
  });
  it("requires run capability, generation and owner checkpoint", () => {
    const paused = { ...task, executionPlacement: "device", status: "paused" } as TaskRun;
    expect(taskPauseControls(paused).resume).toBe(true);
    expect(taskPauseControls({ ...paused, checkpointSupported: false }).resume).toBe(false);
    expect(taskPauseControls({ ...paused, generation: 0 }).resume).toBe(false);
    expect(taskPauseControls({ ...paused, checkpointId: undefined }).resume).toBe(false);
    expect(taskPauseControls({ ...paused, executionPlacement: "cloud" }).resume).toBe(false);
  });
  it("requires current run capabilities", () => {
    const paused = { ...task, executionPlacement: "device", status: "paused", checkpointSupported: true } as TaskRun;
    expect(taskPauseControls(paused).resume).toBe(true);
    expect(taskPauseControls({ ...paused, checkpointSupported: false }).resume).toBe(false);
  });
});
