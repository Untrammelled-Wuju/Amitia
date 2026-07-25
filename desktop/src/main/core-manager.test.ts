import { describe, expect, it, beforeEach, afterEach } from "vitest";
import fs from "fs";
import path from "path";
import os from "os";
import { validateCorePrerequisites } from "./core-prereq";

function getTestCorePath(): string {
  return path.join(os.tmpdir(), "amitia-test-core", "AmitiaCore.exe");
}

function getTestDataDir(): string {
  return path.join(os.tmpdir(), "amitia-test-data");
}

function createTestFiles(dataDir: string, corePath: string): void {
  const dirs = [
    path.join(dataDir, "config"),
    path.join(dataDir, "data"),
    path.join(dataDir, "qdrant"),
    path.join(dataDir, "surrealdb"),
    path.dirname(corePath),
  ];
  for (const dir of dirs) {
    fs.mkdirSync(dir, { recursive: true });
  }
  fs.writeFileSync(
    path.join(dataDir, "config", "config.yml"),
    "# test",
    "utf-8",
  );
  fs.writeFileSync(path.join(dataDir, "data", "sql.sql"), "-- test", "utf-8");
  fs.writeFileSync(path.join(dataDir, "qdrant", "qdrant.zip"), "test", "utf-8");
  fs.writeFileSync(
    path.join(dataDir, "surrealdb", "surreal.zip"),
    "test",
    "utf-8",
  );
  fs.writeFileSync(corePath, "test", "utf-8");
}

function cleanup(dataDir: string, coreDir: string): void {
  try {
    fs.rmSync(dataDir, { recursive: true, force: true });
  } catch {}
  try {
    fs.rmSync(coreDir, { recursive: true, force: true });
  } catch {}
}

describe("validateCorePrerequisites", () => {
  const dataDir = getTestDataDir();
  const coreDir = path.dirname(getTestCorePath());

  beforeEach(() => {
    cleanup(dataDir, coreDir);
  });

  afterEach(() => {
    cleanup(dataDir, coreDir);
  });

  it("当所有必需文件存在时返回ok", () => {
    const corePath = getTestCorePath();
    createTestFiles(dataDir, corePath);

    const result = validateCorePrerequisites(dataDir, corePath);

    expect(result.ok).toBe(true);
    expect(result.missing).toHaveLength(0);
  });

  it("当配置文件缺失时返回具体缺失项", () => {
    const corePath = getTestCorePath();
    createTestFiles(dataDir, corePath);
    fs.rmSync(path.join(dataDir, "config", "config.yml"));

    const result = validateCorePrerequisites(dataDir, corePath);

    expect(result.ok).toBe(false);
    expect(result.missing.length).toBeGreaterThanOrEqual(1);
    expect(result.missing.some((m) => m.includes("配置"))).toBe(true);
  });

  it("当Core可执行文件缺失时返回具体缺失项", () => {
    const corePath = getTestCorePath();
    createTestFiles(dataDir, corePath);
    fs.rmSync(corePath);

    const result = validateCorePrerequisites(dataDir, corePath);

    expect(result.ok).toBe(false);
    expect(result.missing.some((m) => m.includes("Core可执行文件"))).toBe(true);
  });

  it("当多个文件缺失时返回全部缺失项", () => {
    const corePath = getTestCorePath();
    createTestFiles(dataDir, corePath);
    fs.rmSync(path.join(dataDir, "config", "config.yml"));
    fs.rmSync(path.join(dataDir, "data", "sql.sql"));

    const result = validateCorePrerequisites(dataDir, corePath);

    expect(result.ok).toBe(false);
    expect(result.missing.length).toBe(2);
  });

  it("空目录下返回所有缺失项", () => {
    const corePath = getTestCorePath();
    fs.mkdirSync(dataDir, { recursive: true });

    const result = validateCorePrerequisites(dataDir, corePath);

    expect(result.ok).toBe(false);
    expect(result.missing.length).toBe(5);
  });
});
