import path from "path";
import fs from "fs";

export interface CorePrerequisiteResult {
  ok: boolean;
  missing: string[];
}

export function validateCorePrerequisites(
  dataDir: string,
  corePath: string,
): CorePrerequisiteResult {
  const required = [
    { label: "Core可执行文件", file: corePath },
    { label: "配置文件", file: path.join(dataDir, "config", "config.yml") },
    { label: "数据库初始化脚本", file: path.join(dataDir, "data", "sql.sql") },
    {
      label: "Qdrant向量数据库",
      file: path.join(dataDir, "qdrant", "qdrant.zip"),
    },
    {
      label: "SurrealDB数据库",
      file: path.join(dataDir, "surrealdb", "surreal.zip"),
    },
  ];

  const missing = required
    .filter((r) => !fs.existsSync(r.file))
    .map((r) => `${r.label} (${r.file})`);

  return { ok: missing.length === 0, missing };
}

export function validateDeviceAgentPrerequisites(
  dataDir: string,
  corePath: string,
): CorePrerequisiteResult {
  const required = [
    { label: "Core可执行文件", file: corePath },
    { label: "配置文件", file: path.join(dataDir, "config", "config.yml") },
  ];

  const missing = required
    .filter((r) => !fs.existsSync(r.file))
    .map((r) => `${r.label} (${r.file})`);

  return { ok: missing.length === 0, missing };
}
