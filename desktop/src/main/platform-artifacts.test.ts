import { afterEach, beforeEach, describe, expect, it } from "vitest";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import {
  coreBinaryName,
  corePathCandidates,
  databaseArchiveNames,
  databaseSourceCandidates,
  resolveCorePath,
  runtimeTarget,
} from "./platform-artifacts";

describe("platform-artifacts", () => {
  const sandbox = path.join(os.tmpdir(), "amitia-platform-artifacts");

  beforeEach(() => {
    fs.rmSync(sandbox, { recursive: true, force: true });
    fs.mkdirSync(sandbox, { recursive: true });
  });

  afterEach(() => {
    fs.rmSync(sandbox, { recursive: true, force: true });
  });

  it("按平台返回核心文件名", () => {
    expect(coreBinaryName("win32")).toBe("AmitiaCore.exe");
    expect(coreBinaryName("darwin")).toBe("AmitiaCore");
    expect(coreBinaryName("linux")).toBe("AmitiaCore");
  });

  it("按平台与架构生成运行目标", () => {
    expect(runtimeTarget("darwin", "arm64")).toBe("macos-arm64");
    expect(runtimeTarget("win32", "x64")).toBe("windows-x64");
    expect(runtimeTarget("linux", "arm64")).toBe("linux-arm64");
  });

  it("优先使用平台目录下的核心", () => {
    const targetDir = path.join(sandbox, "core", "macos-arm64");
    fs.mkdirSync(targetDir, { recursive: true });
    fs.writeFileSync(path.join(targetDir, "AmitiaCore"), "darwin", "utf8");
    fs.writeFileSync(path.join(sandbox, "core", "AmitiaCore.exe"), "windows", "utf8");
    expect(resolveCorePath(sandbox, "darwin", "arm64")).toBe(path.join(targetDir, "AmitiaCore"));
  });

  it("缺少平台目录时回退到旧版可执行文件", () => {
    fs.mkdirSync(path.join(sandbox, "core"), { recursive: true });
    fs.writeFileSync(path.join(sandbox, "core", "AmitiaCore.exe"), "legacy", "utf8");
    expect(resolveCorePath(sandbox, "win32", "x64")).toBe(path.join(sandbox, "core", "AmitiaCore.exe"));
  });

  it("核心候选路径包含平台目录与旧版路径", () => {
    const candidates = corePathCandidates(sandbox, "darwin", "arm64");
    expect(candidates[0]).toBe(path.join(sandbox, "core", "macos-arm64", "AmitiaCore"));
    expect(candidates).toContain(path.join(sandbox, "core", "AmitiaCore.exe"));
  });

  it("按平台选择数据库归档名", () => {
    expect(databaseArchiveNames("qdrant", "darwin", "arm64")).toEqual(
      expect.arrayContaining(["qdrant-darwin-arm64.zip"]),
    );
    expect(databaseArchiveNames("surrealdb", "win32", "x64")).toEqual(
      expect.arrayContaining(["surreal.exe.zip"]),
    );
  });

  it("数据库资源优先查找平台目录", () => {
    const targetDir = path.join(sandbox, "databases", "macos-arm64");
    fs.mkdirSync(targetDir, { recursive: true });
    fs.writeFileSync(path.join(targetDir, "qdrant.zip"), "darwin", "utf8");
    const candidates = databaseSourceCandidates(sandbox, "qdrant", "darwin", "arm64");
    expect(candidates[0]).toBe(path.join(targetDir, "qdrant.zip"));
  });

  it("数据库资源支持额外根目录回退", () => {
    const extraRoot = path.join(sandbox, "backend");
    fs.mkdirSync(path.join(extraRoot, "surrealdb"), { recursive: true });
    fs.writeFileSync(path.join(extraRoot, "surrealdb", "surreal.zip"), "repo", "utf8");
    const candidates = databaseSourceCandidates(
      path.join(sandbox, "resources"),
      "surrealdb",
      "darwin",
      "arm64",
      [extraRoot],
    );
    expect(candidates).toContain(path.join(extraRoot, "surrealdb", "surreal.zip"));
  });
});
