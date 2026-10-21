import type { TaskDefinition, TaskRun } from "./types";

export function taskPauseControls(task: TaskRun, definition?: TaskDefinition): { pause: boolean; resume: boolean } {
  const placement = task.executionPlacement || "local";
  const checkpointSupported = typeof task.checkpointSupported === "boolean" ? task.checkpointSupported : definition?.checkpoint === true;
  const supported = task.readOnly !== true && (placement === "local" || placement === "device") && checkpointSupported && Number.isSafeInteger(task.generation) && task.generation > 0;
  return {
    pause: supported && (task.status === "running" || task.status === "checkpointing"),
    resume: supported && task.status === "paused" && !!task.checkpointId,
  };
}
