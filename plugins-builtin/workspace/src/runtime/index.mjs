let activeHost = null;

function asObject(value) {
  return value && typeof value === "object" && !Array.isArray(value) ? value : {};
}

function requireString(value, name) {
  const result = String(value ?? "").trim();
  if (!result) throw new Error(`${name} is required`);
  return result;
}

function parentWorkspaceUri(uri, rootUri) {
  const current = requireString(uri, "uri").replace(/\/+$/, "");
  const root = requireString(rootUri, "workspaceUri").replace(/\/+$/, "");
  if (current === root) return root + "/";
  const slash = current.lastIndexOf("/");
  const parent = slash >= 0 ? current.slice(0, slash) : root;
  return parent.length >= root.length ? parent + "/" : root + "/";
}

async function readWorkspace(host, operation, payload) {
  return host.call("host.workspace.read", { operation, payload: payload ?? {} });
}

async function writeWorkspace(host, operation, payload) {
  return host.call("host.workspace.write", { operation, payload: payload ?? {} });
}

async function command(host, inputValue) {
  const input = asObject(inputValue);
  const payload = asObject(input.payload);
  switch (String(input.action || "mounts")) {
    case "mounts":
      return readWorkspace(host, "list_mounts", {});
    case "browse":
      return readWorkspace(host, "browse", {
        uri: requireString(payload.uri, "uri"),
        maxBytes: Number(payload.maxBytes || 1048576),
      });
    case "browse_parent":
      return readWorkspace(host, "browse", {
        uri: parentWorkspaceUri(payload.currentUri, payload.workspaceUri),
        maxBytes: Number(payload.maxBytes || 1048576),
      });
    case "preview":
      return readWorkspace(host, "read_file", {
        uri: requireString(payload.uri, "uri"),
        maxBytes: Number(payload.maxBytes || 1048576),
      });
    case "git_overview": {
      const workspaceUri = requireString(payload.workspaceUri, "workspaceUri");
      const [status, branches, remotes, log] = await Promise.all([
        readWorkspace(host, "git_status", { workspaceUri, includeIgnored: false }),
        readWorkspace(host, "git_branches", { workspaceUri }),
        readWorkspace(host, "git_remotes", { workspaceUri }),
        readWorkspace(host, "git_log", { workspaceUri, limit: Number(payload.limit || 50) }),
      ]);
      return { status, branches, remotes, log };
    }
    case "git_diff":
      return readWorkspace(host, "git_diff", {
        workspaceUri: requireString(payload.workspaceUri, "workspaceUri"),
        mode: String(payload.mode || "worktree"),
        paths: Array.isArray(payload.paths) ? payload.paths : [],
        maxBytes: Number(payload.maxBytes || 2097152),
      });
    case "git_add":
      return writeWorkspace(host, "git_add", {
        workspaceUri: requireString(payload.workspaceUri, "workspaceUri"),
        paths: Array.isArray(payload.paths) ? payload.paths : [],
        all: payload.all === true,
        force: payload.force === true,
      });
    case "git_restore":
      return writeWorkspace(host, "git_restore", {
        workspaceUri: requireString(payload.workspaceUri, "workspaceUri"),
        paths: Array.isArray(payload.paths) ? payload.paths : [],
        staged: payload.staged === true,
        worktree: payload.worktree !== false,
      });
    case "git_commit":
      return writeWorkspace(host, "git_commit", {
        workspaceUri: requireString(payload.workspaceUri, "workspaceUri"),
        message: requireString(payload.message, "message"),
      });
    case "git_checkout":
      return writeWorkspace(host, "git_checkout", {
        workspaceUri: requireString(payload.workspaceUri, "workspaceUri"),
        branch: requireString(payload.branch, "branch"),
        create: payload.create === true,
        fromRef: String(payload.fromRef || ""),
        detach: payload.detach === true,
        force: payload.force === true,
      });
    case "git_fetch":
      return writeWorkspace(host, "git_fetch", {
        workspaceUri: requireString(payload.workspaceUri, "workspaceUri"),
        remote: String(payload.remote || ""),
      });
    case "git_pull":
      return writeWorkspace(host, "git_pull", {
        workspaceUri: requireString(payload.workspaceUri, "workspaceUri"),
        remote: String(payload.remote || ""),
        branch: String(payload.branch || ""),
      });
    case "git_push":
      return writeWorkspace(host, "git_push", {
        workspaceUri: requireString(payload.workspaceUri, "workspaceUri"),
        remote: requireString(payload.remote || "origin", "remote"),
        localRef: requireString(payload.localRef, "localRef"),
        remoteRef: requireString(payload.remoteRef, "remoteRef"),
        setUpstream: payload.setUpstream === true,
      });
    case "isolated_info":
      return readWorkspace(host, "isolated_info", {
        workspaceUri: requireString(payload.workspaceUri, "workspaceUri"),
      });
    case "create_isolated":
      return writeWorkspace(host, "create_isolated", {
        name: requireString(payload.name, "name"),
        mode: String(payload.mode || "snapshot"),
        sourceWorkspaceUri: String(payload.sourceWorkspaceUri || ""),
        gitRemote: payload.mode === "git_clone" ? {
          url: requireString(payload.remoteUrl, "remoteUrl"),
          ref: String(payload.ref || ""),
        } : undefined,
        ref: String(payload.ref || ""),
        readOnly: payload.readOnly === true,
        lifetime: String(payload.lifetime || ""),
      });
    case "delete_isolated":
      return writeWorkspace(host, "delete_isolated", {
        workspaceUri: requireString(payload.workspaceUri, "workspaceUri"),
        force: payload.force === true,
      });
    case "write_file":
      return writeWorkspace(host, "write_file", {
        uri: requireString(payload.uri, "uri"),
        content: String(payload.content ?? ""),
        overwrite: payload.overwrite !== false,
        atomic: payload.atomic !== false,
      });
    case "mkdir":
      return writeWorkspace(host, "mkdir", { uri: requireString(payload.uri, "uri") });
    case "rename":
      return writeWorkspace(host, "rename", {
        uri: requireString(payload.uri, "uri"),
        newName: requireString(payload.newName, "newName"),
      });
    case "move":
      return writeWorkspace(host, "move", {
        sourceUri: requireString(payload.sourceUri, "sourceUri"),
        destinationDirUri: requireString(payload.destinationDirUri, "destinationDirUri"),
      });
    case "copy":
      return writeWorkspace(host, "copy", {
        sourceUri: requireString(payload.sourceUri, "sourceUri"),
        destinationDirUri: requireString(payload.destinationDirUri, "destinationDirUri"),
      });
    case "delete":
      return writeWorkspace(host, "delete", {
        uri: requireString(payload.uri, "uri"),
        recursive: payload.recursive === true,
      });
    default:
      throw new Error(`未知工作区操作: ${String(input.action)}`);
  }
}

const workspaceExtension = {
  async activate(context) {
    activeHost = context.host;
    context.handlers.bindTool("command", (input) => command(activeHost, input));
    context.log.info("workspace plugin activated");
  },
  async deactivate() {
    activeHost = null;
  },
};

if (typeof module !== "undefined" && module.exports) {
  module.exports = workspaceExtension;
}

if (typeof globalThis.defineExtension === "function") {
  globalThis.defineExtension(workspaceExtension);
}
