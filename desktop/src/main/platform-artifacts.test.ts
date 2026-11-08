import { afterEach, beforeEach, describe, expect, it } from "vitest";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import {
  coreBinaryName,
  corePathCandidates,
  databaseArchiveNames,
  databaseSourceCandidates,
  firstExisting,
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

  it("缺少平台目录时不回退到其他可执行文件", () => {
    fs.mkdirSync(path.join(sandbox, "core"), { recursive: true });
    fs.writeFileSync(path.join(sandbox, "core", "AmitiaCore.exe"), "legacy", "utf8");
    expect(resolveCorePath(sandbox, "win32", "x64")).toBe(path.join(sandbox, "core", "windows-x64", "AmitiaCore.exe"));
  });

  it("核心候选路径只包含当前平台目录", () => {
    const candidates = corePathCandidates(sandbox, "darwin", "arm64");
    expect(candidates).toEqual([path.join(sandbox, "core", "macos-arm64", "AmitiaCore")]);
  });

  it("按平台选择数据库归档名", () => {
    expect(databaseArchiveNames("qdrant", "darwin", "arm64")).toEqual(
      ["qdrant.zip"],
    );
    expect(databaseArchiveNames("surrealdb", "win32", "x64")).toEqual(
      ["surreal.zip"],
    );
  });

  it("数据库资源优先查找平台目录", () => {
    const targetDir = path.join(sandbox, "databases", "macos-arm64");
    fs.mkdirSync(targetDir, { recursive: true });
    fs.writeFileSync(path.join(targetDir, "qdrant.zip"), "darwin", "utf8");
    const candidates = databaseSourceCandidates(sandbox, "qdrant", "darwin", "arm64");
    expect(firstExisting(candidates)).toBe(path.join(targetDir, "qdrant.zip"));
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

  it.each([
    ["qdrant", "qdrant-macos-arm64.zip"],
    ["surrealdb", "surreal-macos-arm64.zip"],
  ] as const)("从当前平台的仓库目录读取 %s 归档", (kind, archive) => {
    const target = path.join(sandbox, "databases", "macos-arm64", kind);
    fs.mkdirSync(target, { recursive: true });
    const source = path.join(target, archive);
    fs.writeFileSync(source, "current-platform");
    expect(firstExisting(databaseSourceCandidates(sandbox, kind, "darwin", "arm64"))).toBe(source);
    expect(firstExisting(databaseSourceCandidates(sandbox, kind, "darwin", "x64"))).toBeNull();
  });
});
