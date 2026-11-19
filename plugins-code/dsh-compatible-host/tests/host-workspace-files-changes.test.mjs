import assert from "node:assert/strict";
import test from "node:test";
import { createAmitiaDshHostServices } from "../src/runtime/host-services.mjs";

function parentOf(path) {
  const index = path.lastIndexOf("/");
  if (index <= 0) return "/";
  return path.slice(0, index);
}

function baseOf(path) {
  return path.slice(path.lastIndexOf("/") + 1);
}

function createWatchHost(tree) {
  const calls = [];
  return {
    calls,
    async call(method, input = {}) {
      calls.push({ method, input });
      if (method !== "host.workspace.read") return {};
      const operation = input.operation;
      const payload = input.payload ?? {};
      if (operation === "file_list") {
        const target = payload.path ?? "";
        if (target !== "" && target !== "/" && tree[target]?.type !== "directory") {
          throw new Error(`workspace path ${JSON.stringify(target)} is not a directory`);
        }
        const entries = Object.keys(tree)
          .filter((path) => parentOf(path) === target && path !== target)
          .map((path) => ({
            name: baseOf(path),
            type: tree[path].type,
            ...(tree[path].size === undefined ? {} : { size: tree[path].size }),
          }));
        return { path: target, entries, truncated: false };
      }
      if (operation === "file_stat") {
        const node = tree[payload.path];
        if (!node || node.type !== "file") throw new Error(`workspace file ${JSON.stringify(payload.path)} is missing`);
        return { absolutePath: payload.path, version: node.version, bytes: node.size };
      }
      return {};
    },
  };
}

async function take(iterator, count) {
  const frames = [];
  while (frames.length < count) {
    const next = await iterator.next();
    if (next.done) break;
    frames.push(next.value);
  }
  return frames;
}

const TREE = () => ({
  "/w": { type: "directory" },
  "/w/a.txt": { type: "file", size: 1, version: "1" },
  "/w/b.txt": { type: "file", size: 2, version: "2" },
});

test("workspaceFiles.changes 先发 ready，再推送文件版本变化", async () => {
  const tree = TREE();
  const host = createWatchHost(tree);
  const services = createAmitiaDshHostServices(host);
  const controller = new AbortController();
  const iterator = services.workspaceFiles.changes({ sessionId: "s1", path: "/w/a.txt", intervalMs: 100 }, controller.signal)[Symbol.asyncIterator]();
  assert.deepEqual(await iterator.next(), { value: { kind: "ready" }, done: false });
  tree["/w/a.txt"] = { type: "file", size: 4, version: "2" };
  const frames = await take(iterator, 1);
  assert.deepEqual(frames[0], { kind: "change", change: { absolutePath: "/w/a.txt", version: "2" } });
  controller.abort();
  assert.deepEqual(await iterator.next(), { value: undefined, done: true });
});

test("workspaceFiles.changes 目标消失时推送 absent 帧", async () => {
  const tree = TREE();
  const host = createWatchHost(tree);
  const services = createAmitiaDshHostServices(host);
  const controller = new AbortController();
  const iterator = services.workspaceFiles.changes({ sessionId: "s1", path: "/w/a.txt", intervalMs: 100 }, controller.signal)[Symbol.asyncIterator]();
  await iterator.next();
  delete tree["/w/a.txt"];
  const frames = await take(iterator, 1);
  assert.deepEqual(frames[0], { kind: "change", change: { absolutePath: "/w/a.txt", absent: true } });
  controller.abort();
  await iterator.next();
});

test("workspaceFiles.changes 目录模式推送增删子项", async () => {
  const tree = TREE();
  const host = createWatchHost(tree);
  const services = createAmitiaDshHostServices(host);
  const controller = new AbortController();
  const iterator = services.workspaceFiles.changes({ sessionId: "s1", path: "/w", intervalMs: 100 }, controller.signal)[Symbol.asyncIterator]();
  await iterator.next();
  tree["/w/c.txt"] = { type: "file", size: 3, version: "1" };
  const added = await take(iterator, 1);
  assert.deepEqual(added[0], { kind: "change", change: { absolutePath: "/w/c.txt", version: "1" } });
  delete tree["/w/b.txt"];
  const removed = await take(iterator, 1);
  assert.deepEqual(removed[0], { kind: "change", change: { absolutePath: "/w/b.txt", absent: true } });
  controller.abort();
  await iterator.next();
});

test("workspaceFiles.changes 目录模式能发现同尺寸内容变化", async () => {
  const tree = TREE();
  const host = createWatchHost(tree);
  const services = createAmitiaDshHostServices(host);
  const controller = new AbortController();
  const iterator = services.workspaceFiles.changes({ sessionId: "s1", path: "/w", intervalMs: 100 }, controller.signal)[Symbol.asyncIterator]();
  await iterator.next();
  tree["/w/a.txt"] = { type: "file", size: 1, version: "9" };
  const frames = await take(iterator, 1);
  assert.deepEqual(frames[0], { kind: "change", change: { absolutePath: "/w/a.txt", version: "9" } });
  controller.abort();
  await iterator.next();
});

test("workspaceFiles.changes 拒绝缺失 path、缺失 sessionId 与不存在的目标", async () => {
  const host = createWatchHost(TREE());
  const services = createAmitiaDshHostServices(host);
  await assert.rejects(async () => {
    for await (const frame of services.workspaceFiles.changes({ sessionId: "s1" })) void frame;
  }, /需要 path/);
  await assert.rejects(async () => {
    for await (const frame of services.workspaceFiles.changes({ path: "/w/a.txt" })) void frame;
  }, /需要 sessionId/);
  await assert.rejects(async () => {
    for await (const frame of services.workspaceFiles.changes({ sessionId: "s1", path: "/w/missing.txt" })) void frame;
  }, /目标不存在/);
});

test("workspaceFiles.changes 原样透传宿主列表错误", async () => {
  const host = {
    async call() {
      throw new Error("workspace mount unavailable");
    },
  };
  const services = createAmitiaDshHostServices(host);
  await assert.rejects(async () => {
    for await (const frame of services.workspaceFiles.changes({ sessionId: "s1", path: "/w/a.txt" })) void frame;
  }, /workspace mount unavailable/);
});
