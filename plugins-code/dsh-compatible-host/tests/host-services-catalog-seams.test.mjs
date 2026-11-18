import assert from "node:assert/strict";
import test from "node:test";
import { createAmitiaDshHostServices } from "../src/runtime/host-services.mjs";

function createHost() {
  const calls = [];
  return {
    calls,
    async call(method, input) {
      calls.push({ method, input });
      return { method, input };
    },
  };
}

function assertSeamFailure(run, { method, capability, missing }) {
  assert.throws(run, (error) => {
    assert.equal(error.code, "DSH_HOST_OPERATION_UNAVAILABLE");
    assert.match(error.message, new RegExp(`DSH ${method} 不可用`));
    assert.match(error.message, new RegExp(capability));
    assert.match(error.message, new RegExp(missing.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
    assert.equal(error.details.operation, method);
    assert.equal(error.details.capability, capability);
    assert.equal(error.missingHostInterface.startsWith(missing), true);
    return true;
  }, method);
}

test("directoryPicker 的浏览与建目录走宿主目录接口，选择保持缺口", async () => {
  const host = createHost();
  const services = createAmitiaDshHostServices(host);
  const listing = await services.directoryPicker.list("/tmp");
  assert.equal(listing.method, "host.directory.list");
  assert.deepEqual(listing.input, { path: "/tmp", limit: 200 });
  const created = await services.directoryPicker.createDirectory("/tmp", "proj");
  assert.equal(created.method, "host.directory.create");
  assert.deepEqual(created.input, { path: "/tmp", name: "proj" });
  assertSeamFailure(() => services.directoryPicker.pick(), {
    method: "directoryPicker.pick",
    capability: "交互式选择宿主目录",
    missing: "host.directory.pick",
  });
  assert.equal(host.calls.length, 2);
});

test("directoryPickerController 任意方法都明确失败并带上实际方法名", () => {
  const host = createHost();
  const services = createAmitiaDshHostServices(host);
  assertSeamFailure(() => services.directoryPickerController.select(), {
    method: "directoryPickerController.select",
    capability: "directoryPickerController",
    missing: "host.directory.pick",
  });
  assertSeamFailure(() => services.directoryPickerController.confirm("id"), {
    method: "directoryPickerController.confirm",
    capability: "directoryPickerController",
    missing: "host.directory.pick",
  });
  assert.equal(host.calls.length, 0);
});

test("workspaceController 任意方法都明确失败并指明缺少控制器语义", () => {
  const host = createHost();
  const services = createAmitiaDshHostServices(host);
  assertSeamFailure(() => services.workspaceController.open(), {
    method: "workspaceController.open",
    capability: "workspaceController",
    missing: "storageDomain / sessionPersistence",
  });
  assertSeamFailure(() => services.workspaceController.switchTo("workspace-1"), {
    method: "workspaceController.switchTo",
    capability: "workspaceController",
    missing: "storageDomain / sessionPersistence",
  });
  assert.equal(host.calls.length, 0);
});

test("sessionSkillCatalog 从 host.skill.list 拉取技能目录", async () => {
  const calls = [];
  const host = {
    calls,
    async call(method, input) {
      calls.push({ method, input });
      return { items: [{ skillId: "s1", name: "demo", enabled: true }] };
    },
  };
  const services = createAmitiaDshHostServices(host);
  const catalog = await services.sessionSkillCatalog.refresh();
  assert.deepEqual(catalog.items, [{ skillId: "s1", name: "demo", description: "", enabled: true }]);
  assert.deepEqual(services.sessionSkillCatalog.list(), catalog.items);
  assert.equal(services.sessionSkillCatalog.snapshot().state, "ready");
  assert.equal(calls[0].method, "host.skill.list");
});

test("skills 本地注册与读取真实生效", () => {
  const host = createHost();
  const services = createAmitiaDshHostServices(host);
  const skill = { name: "amitia-demo", description: "demo skill" };
  const dispose = services.skills.register(skill);
  assert.equal(services.skills.get("amitia-demo"), skill);
  dispose();
  assertSeamFailure(() => services.skills.get("amitia-demo"), {
    method: "skills.get",
    capability: "Amitia 技能目录（SKILL.md 分层清单）",
    missing: "host.skill.list",
  });
  assert.equal(host.calls.length, 0);
});

test("skills 拒绝重复注册并回收 provider 注册", () => {
  const host = createHost();
  const services = createAmitiaDshHostServices(host);
  const dispose = services.skills.register({ name: "amitia-demo" });
  assert.throws(() => services.skills.register({ name: "amitia-demo" }), /DSH 技能 amitia-demo 已经注册/);
  assert.throws(() => services.skills.register({}), /DSH skills\.register 需要技能 name/);
  assert.throws(() => services.skills.registerProvider("nope"), /DSH skills\.registerProvider 需要 provider 工厂函数/);
  let created = 0;
  const disposeProvider = services.skills.registerProvider(() => {
    created += 1;
    return { name: "provider" };
  });
  assert.equal(created, 0);
  dispose();
  disposeProvider();
  disposeProvider();
  assertSeamFailure(() => services.skills.get("amitia-demo"), {
    method: "skills.get",
    capability: "Amitia 技能目录（SKILL.md 分层清单）",
    missing: "host.skill.list",
  });
  assert.equal(host.calls.length, 0);
});

test("skills 的 list 与 snapshot 合并宿主目录与本地注册", async () => {
  const calls = [];
  const host = {
    calls,
    async call(method, input) {
      calls.push({ method, input });
      if (method === "host.skill.list") {
        return { items: [{ skillId: "hosted-1", name: "hosted-skill", description: "宿主技能", enabled: true }] };
      }
      return { method, input };
    },
  };
  const services = createAmitiaDshHostServices(host);
  services.skills.register({ name: "amitia-demo" });
  await services.skills.refresh();
  assert.deepEqual(services.skills.list().map((skill) => skill.name), ["amitia-demo", "hosted-skill"]);
  const snapshot = services.skills.snapshot();
  assert.equal(snapshot.state, "ready");
  assert.equal(snapshot.items.length, 2);
  assert.equal(services.skills.get("hosted-skill").skillId, "hosted-1");
  assert.equal(calls.some((entry) => entry.method === "host.skill.list"), true);
});
