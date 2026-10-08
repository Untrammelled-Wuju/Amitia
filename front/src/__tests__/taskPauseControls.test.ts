import { describe, expect, it } from "vitest";
import { taskPauseControls } from "../views/kernel/tasks/control";
import type { TaskDefinition, TaskRun, TaskRunStatus } from "../views/kernel/tasks/types";

const definition = { checkpoint: true } as TaskDefinition;
const task = { generation: 2, checkpointId: "owner-checkpoint" } as TaskRun;

describe("task pause controls", () => {
  it.each(["local", "device"] as const)("permits confirmed %s execution controls", (executionPlacement) => {
    expect(taskPauseControls({ ...task, executionPlacement, status: "running" }, definition)).toEqual({ pause: true, resume: false });
    expect(taskPauseControls({ ...task, executionPlacement, status: "checkpointing" }, definition).pause).toBe(true);
    expect(taskPauseControls({ ...task, executionPlacement, status: "paused" }, definition)).toEqual({ pause: false, resume: true });
  });
  it.each(["pausing", "resuming", "cancelling", "recovery_required", "succeeded"] as TaskRunStatus[])("blocks premature controls in %s", (status) => {
    expect(taskPauseControls({ ...task, executionPlacement: "device", status }, definition)).toEqual({ pause: false, resume: false });
  });
  it("requires a definition, generation and owner checkpoint", () => {
    const paused = { ...task, executionPlacement: "device", status: "paused" } as TaskRun;
    expect(taskPauseControls(paused).resume).toBe(false);
    expect(taskPauseControls(paused, { ...definition, checkpoint: false }).resume).toBe(false);
    expect(taskPauseControls({ ...paused, generation: 0 }, definition).resume).toBe(false);
    expect(taskPauseControls({ ...paused, checkpointId: undefined }, definition).resume).toBe(false);
    expect(taskPauseControls({ ...paused, executionPlacement: "cloud" }, definition).resume).toBe(false);
  });
});
