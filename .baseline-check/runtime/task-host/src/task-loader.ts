import { pathToFileURL } from "node:url";
import type { TaskHandler } from "@amitia/plugin-sdk";

export async function loadTaskHandler(entryPath: string): Promise<TaskHandler> {
  const fileUrl = pathToFileURL(entryPath).href;
  const mod = await import(fileUrl);
  const handler = extractHandler(mod);
  if (!handler) {
    throw new Error(`task entry module does not export a valid handler: ${entryPath}`);
  }
  return handler;
}

function extractHandler(mod: unknown): TaskHandler | null {
  if (!mod || typeof mod !== "object") {
    return null;
  }

  const record = mod as Record<string, unknown>;

  if (typeof record.default === "function") {
    return record.default as TaskHandler;
  }

  if (typeof record.handler === "function") {
    return record.handler as TaskHandler;
  }

  if (typeof record.run === "function") {
    return record.run as TaskHandler;
  }

  if (record.default && typeof record.default === "object") {
    const def = record.default as Record<string, unknown>;
    if (typeof def.handler === "function") {
      return def.handler as TaskHandler;
    }
  }

  return null;
}
