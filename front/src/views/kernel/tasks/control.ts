import type { TaskRun } from "./types";

export function taskPauseControls(task: TaskRun): { pause: boolean; resume: boolean } {
  const placement = task.executionPlacement || "local";
  const checkpointSupported = task.checkpointSupported === true;
  const supported = task.readOnly !== true && (placement === "local" || placement === "device") && checkpointSupported && Number.isSafeInteger(task.generation) && task.generation > 0;
  return {
    pause: supported && (task.status === "running" || task.status === "checkpointing"),
    resume: supported && task.status === "paused" && !!task.checkpointId,
  };
}
