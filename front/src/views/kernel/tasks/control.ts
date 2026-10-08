import type { TaskDefinition, TaskRun } from "./types";

export function taskPauseControls(task: TaskRun, definition?: TaskDefinition): { pause: boolean; resume: boolean } {
  const placement = task.executionPlacement || "local";
  const supported = task.readOnly !== true && (placement === "local" || placement === "device") && definition?.checkpoint === true && Number.isSafeInteger(task.generation) && task.generation > 0;
  return {
    pause: supported && (task.status === "running" || task.status === "checkpointing"),
    resume: supported && task.status === "paused" && !!task.checkpointId,
  };
}
