import assert from "node:assert/strict";

let extension = null;
globalThis.defineExtension = (value) => {
  extension = value;
};
await import(`../src/runtime/index.mjs?runtime-test=${Date.now()}`);
assert.ok(extension);

const calls = [];
let command = null;
await extension.activate({
  host: {
    async call(method, input) {
      calls.push({ method, input });
      switch (input.operation) {
        case "list_mounts":
          return [{ id: "workspace-a", name: "A", rootUri: "amitia://workspace/@workspace-a/" }];
        case "browse":
          return { path: input.payload.uri, entries: [{ name: "src", type: "directory" }] };
        case "git_status":
          return { branch: "develop", clean: true, entries: [] };
        case "git_branches":
          return { branches: [{ name: "develop", current: true }] };
        case "git_remotes":
          return [{ name: "origin", fetchUrl: "https://example.com/repo.git" }];
        case "git_log":
          return { entries: [{ hash: "abc123", subject: "initial" }] };
        default:
          return { ok: true, operation: input.operation };
      }
    },
  },
  log: { info() {}, warn() {} },
  handlers: {
    bindTool(name, handler) {
      if (name === "command") command = handler;
    },
  },
});

assert.ok(command);
assert.equal((await command({ action: "mounts" }))[0].id, "workspace-a");
assert.equal((await command({
  action: "browse_parent",
  payload: {
    currentUri: "amitia://workspace/@workspace-a/src/core",
    workspaceUri: "amitia://workspace/@workspace-a/",
  },
})).path, "amitia://workspace/@workspace-a/src/");
const overview = await command({
  action: "git_overview",
  payload: { workspaceUri: "amitia://workspace/@workspace-a/" },
});
assert.equal(overview.status.branch, "develop");
assert.equal(overview.branches.branches[0].name, "develop");
await command({
  action: "git_commit",
  payload: { workspaceUri: "amitia://workspace/@workspace-a/", message: "test" },
});
assert.equal(calls.at(-1).method, "host.workspace.write");
assert.equal(calls.at(-1).input.operation, "git_commit");
await assert.rejects(command({ action: "unknown" }), /未知工作区操作/);
await extension.deactivate();
console.log("workspace-runtime=ok");
