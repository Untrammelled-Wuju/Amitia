import type { TaskResult, TaskArtifact } from "@amitia/plugin-sdk";
import type { RpcClient } from "./rpc.js";

const INLINE_JSON_LIMIT = 64 * 1024;

export interface TaskFinishedPayload {
  task_run_id: string;
  status: "succeeded" | "failed";
  result?: {
    mode: "inline_json" | "artifact";
    data?: unknown;
    artifact_id?: string;
    handle?: string;
    artifacts?: TaskArtifact[];
    metadata?: Record<string, unknown>;
  };
  error?: { code: string; message: string };
}

export async function processResult(
  result: TaskResult,
  rpc: RpcClient,
  taskRunId: string,
): Promise<TaskFinishedPayload> {
  if (!result.success) {
    return {
      task_run_id: taskRunId,
      status: "failed",
      error: result.error ?? { code: "unknown", message: "unknown error" },
    };
  }

  const output = result.output ?? null;
  const artifacts = result.artifacts;
  const metadata = result.metadata;

  let serialized: string | undefined;
  try {
    serialized = JSON.stringify(output);
  } catch {
    return { task_run_id: taskRunId, status: "failed", error: { code: "task_output_not_serializable", message: "任务输出无法序列化为 JSON" } };
  }

  if (typeof serialized !== "string") {
    return { task_run_id: taskRunId, status: "failed", error: { code: "task_output_not_serializable", message: "任务输出无法序列化为 JSON" } };
  }

  if (Buffer.byteLength(serialized, "utf8") <= INLINE_JSON_LIMIT) {
    return {
      task_run_id: taskRunId,
      status: "succeeded",
      result: {
        mode: "inline_json",
        data: output,
        artifacts,
        metadata,
      },
    };
  }

  const artifactResponse = await rpc.call<{ artifactId: string; handle: string }>(
    "task.artifact.saveData",
    {
      task_run_id: taskRunId,
      name: "result.json",
      data: output,
      options: { kind: "data", mimeType: "application/json" },
    },
  );

  return {
    task_run_id: taskRunId,
    status: "succeeded",
    result: {
      mode: "artifact",
      artifact_id: artifactResponse.artifactId,
      handle: artifactResponse.handle,
      artifacts,
      metadata,
    },
  };
}
