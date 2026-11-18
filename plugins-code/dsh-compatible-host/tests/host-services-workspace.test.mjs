import assert from "node:assert/strict";
import test from "node:test";
import { createAmitiaDshHostServices } from "../src/runtime/host-services.mjs";

function createHost(handlers = {}) {
  const calls = [];
  const host = {
    calls,
    async call(method, input) {
      calls.push({ method, input });
      const handler = handlers[method];
      if (typeof handler === "function") return handler(input, host);
      return { method, input };
    },
  };
  return host;
}

test("workspaceFiles 把目录、检索、状态、文本与字节读取映射到会话工作区操作", async () => {
  const host = createHost();
  const services = createAmitiaDshHostServices(host);
  await services.workspaceFiles.list({ sessionId: "session-1", path: "src" });
  await services.workspaceFiles.search({
    sessionId: "session-1",
    query: "index",
    excludedPaths: ["node_modules"],
    includeHidden: true,
    respectGitignore: true,
  });
  await services.workspaceFiles.search({ sessionId: "session-1", query: "index" });
  await services.workspaceFiles.stat({ sessionId: "session-1", path: "README.md" });
  await services.workspaceFiles.read({ sessionId: "session-1", path: "README.md", offset: 2, limit: 5 });
  await services.workspaceFiles.readBytes({ sessionId: "session-1", path: "logo.png", baseFile: "README.md", offset: 4, length: 8 });
  assert.deepEqual(host.calls.map((entry) => entry.method), [
    "host.workspace.read",
    "host.workspace.read",
    "host.workspace.read",
    "host.workspace.read",
    "host.workspace.read",
    "host.workspace.read",
  ]);
  assert.deepEqual(host.calls.map((entry) => entry.input.operation), [
    "file_list",
    "file_search",
    "file_search",
    "file_stat",
    "file_read_text",
    "file_read_bytes",
  ]);
  assert.deepEqual(host.calls[0].input.payload, { sessionId: "session-1", path: "src" });
  assert.deepEqual(host.calls[1].input.payload, {
    sessionId: "session-1",
    query: "index",
    excludedPaths: ["node_modules"],
    includeHidden: true,
    respectGitignore: true,
  });
  assert.deepEqual(host.calls[2].input.payload, {
    sessionId: "session-1",
    query: "index",
    excludedPaths: [],
    includeHidden: false,
    respectGitignore: false,
  });
  assert.deepEqual(host.calls[3].input.payload, { sessionId: "session-1", path: "README.md" });
  assert.deepEqual(host.calls[4].input.payload, { sessionId: "session-1", path: "README.md", offset: 2, limit: 5 });
  assert.deepEqual(host.calls[5].input.payload, {
    sessionId: "session-1",
    path: "logo.png",
    baseFile: "README.md",
    offset: 4,
    length: 8,
  });
});

test("workspaceFiles 原样透传宿主返回体", async () => {
  const response = { path: "", entries: [{ name: "src", type: "directory" }], truncated: false };
  const host = createHost({ "host.workspace.read": () => response });
  const services = createAmitiaDshHostServices(host);
  assert.equal(await services.workspaceFiles.list({ sessionId: "session-1", path: "" }), response);
});

test("workspaceFiles 原样抛出宿主错误与 cause", async () => {
  const cause = new Error("workspace mount missing");
  const thrown = Object.assign(new Error("host.workspace.read failed: workspace is not bound"), {
    code: "host_call_failed",
    response: { code: 409, msg: "workspace is not bound" },
    cause,
  });
  const host = createHost({
    "host.workspace.read": () => {
      throw thrown;
    },
  });
  const services = createAmitiaDshHostServices(host);
  await assert.rejects(() => services.workspaceFiles.read({ sessionId: "session-1", path: "a.md" }), (error) => {
    assert.equal(error, thrown);
    assert.equal(error.code, "host_call_failed");
    assert.deepEqual(error.response, { code: 409, msg: "workspace is not bound" });
    assert.equal(error.cause, cause);
    return true;
  });
});

test("workspaceFiles 缺少 sessionId 时明确失败且不调用宿主", async () => {
  const host = createHost();
  const services = createAmitiaDshHostServices(host);
  await assert.rejects(() => services.workspaceFiles.list({ path: "src" }), (error) => {
    assert.match(error.message, /DSH workspaceFiles\.list 需要 sessionId/);
    assert.match(error.message, /按 sessionId 解析工作区挂载/);
    return true;
  });
  assert.equal(host.calls.length, 0);
});

test("workspaceFiles.changes 明确失败并指明缺少变更订阅接口", () => {
  const host = createHost();
  const services = createAmitiaDshHostServices(host);
  assert.throws(() => services.workspaceFiles.changes(), (error) => {
    assert.equal(error.code, "DSH_HOST_OPERATION_UNAVAILABLE");
    assert.match(error.message, /DSH workspaceFiles\.changes 不可用/);
    assert.match(error.message, /期望能力 工作区目录与文件变更帧/);
    assert.match(error.message, /宿主缺少接口 host\.workspace\.watch/);
    assert.equal(error.details.operation, "workspaceFiles.changes");
    assert.equal(error.missingHostInterface.startsWith("host.workspace.watch"), true);
    return true;
  });
  assert.equal(host.calls.length, 0);
});

test("fileReferences 复用会话工作区读取并保持宿主返回体", async () => {
  const statResponse = { absolutePath: "/tmp/project/README.md", version: "1:8" };
  const host = createHost({ "host.workspace.read": () => statResponse });
  const services = createAmitiaDshHostServices(host);
  assert.equal(await services.fileReferences.stat({ sessionId: "session-1", path: "README.md" }), statResponse);
  await services.fileReferences.search({ sessionId: "session-1", query: "readme" });
  assert.deepEqual(host.calls.map((entry) => entry.input.operation), ["file_stat", "file_search"]);
  await assert.rejects(() => services.fileReferences.list({}), (error) => {
    assert.match(error.message, /DSH fileReferences\.list 需要 sessionId/);
    return true;
  });
});

test("sessionFileReferences 用位置参数注入会话作用域", async () => {
  const host = createHost();
  const services = createAmitiaDshHostServices(host);
  await services.sessionFileReferences.list("session-9", { path: "docs" });
  await services.sessionFileReferences.search("session-9", { query: "spec" });
  await services.sessionFileReferences.stat("session-9", { path: "docs/a.md" });
  await services.sessionFileReferences.read("session-9", { path: "docs/a.md" });
  assert.deepEqual(host.calls.map((entry) => entry.input), [
    { operation: "file_list", payload: { sessionId: "session-9", path: "docs" } },
    {
      operation: "file_search",
      payload: { sessionId: "session-9", query: "spec", excludedPaths: [], includeHidden: false, respectGitignore: false },
    },
    { operation: "file_stat", payload: { sessionId: "session-9", path: "docs/a.md" } },
    { operation: "file_read_text", payload: { sessionId: "session-9", path: "docs/a.md" } },
  ]);
  await assert.rejects(() => services.sessionFileReferences.list("", {}), (error) => {
    assert.match(error.message, /DSH sessionFileReferences\.list 需要 sessionId/);
    return true;
  });
});

test("fs 读取链路按目标会话解析工作区并透传宿主返回", async () => {
  const readResponse = { path: "a.txt", content: "hello" };
  const host = createHost({ "host.workspace.read": () => readResponse });
  const services = createAmitiaDshHostServices(host);
  const target = services.fs.resolve("a.txt", { sessionId: "session-1" });
  assert.deepEqual(target, { path: "a.txt", sessionId: "session-1" });
  assert.equal(services.fs.processPath(target), "a.txt");
  assert.equal(await services.fs.readText(target), readResponse);
  await services.fs.stat(target);
  await services.fs.lstat("a.txt", { sessionId: "session-1" });
  await services.fs.readBytes(target, undefined, 16);
  await services.fs.readByteRange(target, { offset: 0, length: 4 });
  await services.fs.listDir(target);
  assert.deepEqual(host.calls.map((entry) => entry.input), [
    { operation: "file_read_text", payload: { sessionId: "session-1", path: "a.txt" } },
    { operation: "file_stat", payload: { sessionId: "session-1", path: "a.txt" } },
    { operation: "file_stat", payload: { sessionId: "session-1", path: "a.txt" } },
    { operation: "file_read_bytes", payload: { sessionId: "session-1", path: "a.txt", length: 16 } },
    { operation: "file_read_bytes", payload: { sessionId: "session-1", path: "a.txt", offset: 0, length: 4 } },
    { operation: "file_list", payload: { sessionId: "session-1", path: "a.txt" } },
  ]);
});

test("fs.writeText 与 deleteFile 经 conversation_workspace 拼出工作区 URI", async () => {
  const host = createHost({
    "host.workspace.read": (input) => {
      assert.equal(input.operation, "conversation_workspace");
      return { conversationId: "session-1", rootUri: "local:///tmp/project", readOnly: false };
    },
    "host.workspace.write": (input) => ({ operation: input.operation, applied: true }),
  });
  const services = createAmitiaDshHostServices(host);
  const target = services.fs.resolve("docs/a.md", { sessionId: "session-1" });
  assert.deepEqual(await services.fs.writeText(target, "hello", { overwrite: true }), { operation: "write_file", applied: true });
  assert.deepEqual(await services.fs.deleteFile(target), { operation: "delete", applied: true });
  assert.deepEqual(host.calls.map((entry) => [entry.method, entry.input]), [
    ["host.workspace.read", { operation: "conversation_workspace", payload: { conversationId: "session-1" } }],
    [
      "host.workspace.write",
      {
        operation: "write_file",
        payload: { uri: "local:///tmp/project/docs/a.md", content: "hello", overwrite: true, atomic: true },
      },
    ],
    ["host.workspace.read", { operation: "conversation_workspace", payload: { conversationId: "session-1" } }],
    ["host.workspace.write", { operation: "delete", payload: { uri: "local:///tmp/project/docs/a.md", recursive: false } }],
  ]);
});

test("fs 写入缺少会话作用域或 rootUri 时明确失败", async () => {
  const host = createHost({ "host.workspace.read": () => ({ conversationId: "session-1", rootUri: "" }) });
  const services = createAmitiaDshHostServices(host);
  const target = services.fs.resolve("a.txt");
  await assert.rejects(() => services.fs.readText(target), (error) => {
    assert.match(error.message, /DSH fs\.readText 需要 sessionId/);
    return true;
  });
  const scoped = services.fs.resolve("a.txt", { sessionId: "session-1" });
  await assert.rejects(() => services.fs.writeText(scoped, "x"), (error) => {
    assert.match(error.message, /Amitia host\.workspace\.read conversation_workspace 未返回 rootUri/);
    return true;
  });
});

test("fs.editText 与 patchText 都走宿主读写，watch 轮询检测变更", async () => {
  const stats = [
    { size: 1, modifiedAt: "t1" },
    { size: 2, modifiedAt: "t2" },
  ];
  const calls = [];
  const host = {
    calls,
    async call(method, input) {
      calls.push({ method, input });
      if (method === "host.workspace.read" && input?.operation === "file_stat") {
        return stats.shift() ?? { size: 2, modifiedAt: "t2" };
      }
      if (method === "host.workspace.read" && input?.operation === "file_read_text") {
        return { content: "hello world\nsecond line\n" };
      }
      if (method === "host.workspace.read") {
        return { conversationId: "session-1", rootUri: "local:///tmp/project", readOnly: false };
      }
      if (method === "host.workspace.write") {
        return { version: "v2" };
      }
      return { method, input };
    },
  };
  const services = createAmitiaDshHostServices(host);
  const patched = await services.fs.patchText(
    { path: "/tmp/project/a.txt", sessionId: "session-1" },
    [{ lineNumber: 2, oldLines: ["second line"], newLines: ["pátched line"], isEndOfFile: false }],
  );
  assert.equal(patched.after, "hello world\npátched line\n");
  assert.equal(patched.version, "v2");
  const outcome = await services.fs.editText(
    { path: "/tmp/project/a.txt", sessionId: "session-1" },
    { oldString: "world", newString: "DSH", replaceAll: false },
  );
  assert.equal(outcome.before, "hello world\nsecond line\n");
  assert.equal(outcome.after, "hello DSH\nsecond line\n");
  assert.equal(outcome.version, "v2");
  const changes = [];
  const handle = services.fs.watch({ path: "/tmp/project/a.txt", sessionId: "session-1" }, (change) => {
    changes.push(change);
  }, { intervalMs: 5 });
  await new Promise((resolve) => setTimeout(resolve, 80));
  handle.dispose();
  assert.equal(changes.length >= 1, true);
  assert.equal(handle.path, "/tmp/project/a.txt");
});

test("fs.moveFile 同目录改名走 rename，跨目录先 move 再 rename", async () => {
  const host = createHost({
    "host.workspace.read": () => ({ conversationId: "session-1", rootUri: "local:///tmp/project", readOnly: false }),
    "host.workspace.write": (input) => ({ operation: input.operation, applied: true }),
  });
  const services = createAmitiaDshHostServices(host);
  assert.deepEqual(
    await services.fs.moveFile(
      services.fs.resolve("docs/a.md", { sessionId: "session-1" }),
      services.fs.resolve("docs/b.md", { sessionId: "session-1" }),
      {},
    ),
    { operation: "rename", applied: true },
  );
  assert.deepEqual(
    await services.fs.moveFile(
      services.fs.resolve("docs/a.md", { sessionId: "session-1" }),
      services.fs.resolve("archive/2026/a.md", { sessionId: "session-1" }),
      {},
    ),
    { operation: "move", applied: true },
  );
  assert.deepEqual(
    await services.fs.moveFile(
      services.fs.resolve("docs/a.md", { sessionId: "session-1" }),
      services.fs.resolve("archive/b.md", { sessionId: "session-1" }),
      {},
    ),
    { operation: "rename", applied: true },
  );
  assert.deepEqual(host.calls.map((entry) => [entry.method, entry.input]), [
    ["host.workspace.read", { operation: "conversation_workspace", payload: { conversationId: "session-1" } }],
    ["host.workspace.read", { operation: "conversation_workspace", payload: { conversationId: "session-1" } }],
    ["host.workspace.write", { operation: "rename", payload: { uri: "local:///tmp/project/docs/a.md", newName: "b.md" } }],
    ["host.workspace.read", { operation: "conversation_workspace", payload: { conversationId: "session-1" } }],
    ["host.workspace.read", { operation: "conversation_workspace", payload: { conversationId: "session-1" } }],
    [
      "host.workspace.write",
      { operation: "move", payload: { sourceUri: "local:///tmp/project/docs/a.md", destinationDirUri: "local:///tmp/project/archive/2026" } },
    ],
    ["host.workspace.read", { operation: "conversation_workspace", payload: { conversationId: "session-1" } }],
    ["host.workspace.read", { operation: "conversation_workspace", payload: { conversationId: "session-1" } }],
    [
      "host.workspace.write",
      { operation: "move", payload: { sourceUri: "local:///tmp/project/docs/a.md", destinationDirUri: "local:///tmp/project/archive" } },
    ],
    ["host.workspace.write", { operation: "rename", payload: { uri: "local:///tmp/project/archive/b.md", newName: "b.md" } }],
  ]);
});

test("fs.moveFile 带 hunk 时明确失败并带上源与目标", async () => {
  const host = createHost({
    "host.workspace.read": () => ({ conversationId: "session-1", rootUri: "local:///tmp/project" }),
  });
  const services = createAmitiaDshHostServices(host);
  await assert.rejects(
    () => services.fs.moveFile(
      services.fs.resolve("a.md", { sessionId: "session-1" }),
      services.fs.resolve("b.md", { sessionId: "session-1" }),
      { hunks: [{ at: 0 }] },
    ),
    (error) => {
      assert.equal(error.code, "DSH_HOST_OPERATION_UNAVAILABLE");
      assert.match(error.message, /DSH fs\.moveFile 不可用/);
      assert.match(error.message, /宿主缺少接口 host\.workspace\.patch/);
      assert.deepEqual(error.details.request, { source: "a.md", destination: "b.md", hunks: 1 });
      return true;
    },
  );
  assert.equal(host.calls.length, 0);
});

test("fileReferences、sessionFileReferences 与 fs 原样抛出宿主错误", async () => {
  const cause = new Error("workspace runner offline");
  const thrown = Object.assign(new Error("host.workspace.read failed: file_list is out of scope"), {
    code: "host_scope_denied",
    response: { code: 403, msg: "file_list is out of scope" },
    cause,
  });
  const host = createHost({
    "host.workspace.read": () => {
      throw thrown;
    },
  });
  const services = createAmitiaDshHostServices(host);
  await assert.rejects(() => services.fileReferences.search({ sessionId: "session-1", query: "a" }), (error) => {
    assert.equal(error, thrown);
    assert.equal(error.code, "host_scope_denied");
    assert.deepEqual(error.response, { code: 403, msg: "file_list is out of scope" });
    assert.equal(error.cause, cause);
    return true;
  });
  await assert.rejects(() => services.sessionFileReferences.list("session-1"), (error) => {
    assert.equal(error, thrown);
    assert.equal(error.cause, cause);
    return true;
  });
  await assert.rejects(() => services.fs.readText({ sessionId: "session-1", path: "a.md" }), (error) => {
    assert.equal(error, thrown);
    assert.deepEqual(error.response, { code: 403, msg: "file_list is out of scope" });
    return true;
  });
  assert.equal(host.calls.length, 3);
});
