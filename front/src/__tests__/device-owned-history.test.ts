import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), transition: vi.fn(), stream: vi.fn() }));
vi.mock("../composables/useApi", () => ({ apiClient: { get: mocks.get, post: mocks.post } }));
vi.mock("../runtime/runtime-adapter", () => ({ getDeploymentConfig: async () => ({ mode: "cloud", serverURL: "https://core" }), getNativeProviderTransition: mocks.transition }));
vi.mock("../runtime/device-owned-chat", () => ({ streamOwnedChat: mocks.stream }));

import { useDeviceOwnedConversation } from "../composables/useDeviceOwnedConversation";
import { conversationReference, parseConversationReference } from "../runtime/device-owned-conversation-reference";

const scope = { coreId: "core", providerEpoch: 1, modeRevision: 1, permissionRevision: 1, targetPermissionRevision: 1, resourceOwnerId: "a", roleId: "role", roleRevision: 1 };
const response = (nextCursors: Record<string, string>, historical?: Record<string, string>, revision = 1) => ({ data: { executionScope: { ...scope, roleRevision: revision }, snapshot: { ownerId: "a", resources: [], nextCursors }, ...(historical ? { historicalSnapshot: { ownerId: "old-device", resources: [], nextCursors: historical } } : {}) } });

describe("owned conversation history", () => {
  it("passes normalized search through every page without filtering message matches by title", async () => {
    const owned = useDeviceOwnedConversation();
    mocks.get.mockResolvedValueOnce({ data: { executionScope: scope, snapshot: { ownerId: "a", resources: [{ kind: "conversation", body: { id: "first", title: "ordinary" } }], nextCursors: { conversation: "search-page" } } } });
    mocks.get.mockResolvedValueOnce({ data: { executionScope: scope, snapshot: { ownerId: "a", resources: [{ kind: "conversation", body: { id: "second", title: "ordinary" } }] } } });
    const rows = await owned.conversations("role", " NEEDLE ");
    expect(rows).toHaveLength(2);
    expect(mocks.get.mock.calls[0][1].params).toEqual({ characterId: "role", keyword: "needle" });
    expect(mocks.get.mock.calls[1][1].params).toEqual({ characterId: "role", keyword: "needle", resourceKind: "conversation", cursor: "search-page" });
    mocks.get.mockResolvedValue(response({}));
    await owned.conversations("role", "different");
    expect(mocks.get.mock.calls[2][1].params).toEqual({ characterId: "role", keyword: "different" });
  });
	it("shows permanent owner rejection without copying private content or another owner's failures", () => {
		const owned = useDeviceOwnedConversation();
		const rows = owned.messages({ executionScope: scope, snapshot: { ownerId: "a", resources: [] }, deliveryFailures: [
			{ ownerId: "a", requestId: "rejected", conversationId: "chat", errorCode: "mesh.owned_resource_version", failedAt: "2026-10-04T00:00:00Z" },
			{ ownerId: "other", requestId: "hidden", errorCode: "mesh.owned_resource_version", failedAt: "2026-10-04T00:00:00Z" },
		] } as any);
		expect(rows).toHaveLength(1);
		expect(rows[0].role).toBe("system");
		expect(rows[0].content).toContain("有 1 项保存请求");
		expect(rows[0].sourceRevision).toBeNull();
	});
  beforeEach(() => { mocks.get.mockReset(); mocks.post.mockReset(); mocks.transition.mockReset(); mocks.stream.mockReset(); useDeviceOwnedConversation().stopLocal(""); });

  it("edits the displayed summary revision at its actual owner resource", async () => {
    const owned = useDeviceOwnedConversation();
    owned.enabled.value = true;
    const resource = { kind: "summary", id: "continuation/hash/summary", ownerId: "a", roleId: "role", revision: 7, body: { content: { summary: "shown" } } };
    mocks.get.mockResolvedValue({ data: { executionScope: scope, snapshot: { ownerId: "a", resources: [resource] } } });
    const displayed = await owned.conversationSummary("meshconv1:a:chat", "role");
    expect(displayed?.summaryText).toBe("shown");
    mocks.get.mockResolvedValue({ data: { executionScope: scope, snapshot: { ownerId: "a", resources: [{ ...resource, revision: 9 }] } } });
    await owned.query("meshconv1:a:chat", "role");
    mocks.post.mockResolvedValue({ data: { ownerId: "a", versions: { "summary/continuation/hash/summary": 8 } } });
    await owned.editConversationSummary("meshconv1:a:chat", "edited", false, displayed?.summaryViewId);
    expect(mocks.post.mock.calls[0][1]).toMatchObject({ id: resource.id, expectedRevision: 7, expectedExecutionScope: scope });
    await expect(owned.editConversationSummary("meshconv1:a:chat", "again")).rejects.toThrow("请先加载");
  });

  it("keeps a historical summary readable without allowing writes", async () => {
    const owned = useDeviceOwnedConversation();
    mocks.get.mockResolvedValue({ data: { executionScope: scope, snapshot: { ownerId: "a", resources: [] }, historicalSnapshot: { ownerId: "old", resources: [{ kind: "summary", id: "chat/summary", ownerId: "old", revision: 1, body: { content: { summary: "history" } } }] } } });
    expect((await owned.conversationSummary("chat", "role"))?.editable).toBe(false);
    await expect(owned.editConversationSummary("chat", undefined, true)).rejects.toThrow("历史摘要");
    expect(mocks.post).not.toHaveBeenCalled();
  });

  it("generates through Core with the prepared owner version and requires its acknowledgement", async () => {
    const owned = useDeviceOwnedConversation();
    owned.enabled.value = true;
    mocks.get.mockResolvedValueOnce({ data: { executionScope: scope, conversationId: "continuation/hash", snapshot: { ownerId: "a", resources: [] } } });
    mocks.get.mockResolvedValueOnce({ data: { executionScope: scope, resource: null } });
    const viewId = await owned.prepareSummaryGeneration("meshconv1:a:chat", "role");
    mocks.post.mockImplementation(async (_path, request) => ({ data: { saved: true, summaryText: "generated", sourceResourceId: "continuation/hash/summary", sourceOwnerId: "a", sourceRevision: 1, executionScope: { ...scope, requestId: request.requestId }, acknowledgement: { ownerId: "a", requestId: `${request.requestId}|summary-result`, versions: { "summary/continuation/hash/summary": 1 } } } }));
    expect((await owned.generateConversationSummary("meshconv1:a:chat", viewId)).summaryText).toBe("generated");
    expect(mocks.post.mock.calls[0][0]).toBe("/api/device-mesh/v1/business/conversations/chat/summary/generate");
    expect(mocks.post.mock.calls[0][1]).toMatchObject({ expectedRevision: 0, expectedExecutionScope: scope, conversationOrigin: { ownerId: "a", id: "chat" } });
    await expect(owned.generateConversationSummary("meshconv1:a:chat", viewId)).rejects.toThrow("生成页面已变化");
  });

  it("discards a prepared summary on provider change before sending", async () => {
    const owned = useDeviceOwnedConversation();
    owned.enabled.value = true;
    mocks.get.mockResolvedValueOnce({ data: { executionScope: scope, snapshot: { ownerId: "a", resources: [] } } });
    mocks.get.mockResolvedValueOnce({ data: { executionScope: scope, resource: null } });
    const viewId = await owned.prepareSummaryGeneration("chat", "role");
    owned.stopLocal("provider changed");
    await expect(owned.generateConversationSummary("chat", viewId)).rejects.toThrow("生成页面已变化");
    expect(mocks.post).not.toHaveBeenCalled();
  });

  it("rejects an old edit dialog after another role reloads the same summary", async () => {
    const owned = useDeviceOwnedConversation();
    mocks.get.mockResolvedValue({ data: { executionScope: scope, snapshot: { ownerId: "a", resources: [{ kind: "summary", id: "chat/summary", ownerId: "a", revision: 1, body: { content: { summary: "old" } } }] } } });
    const oldView = await owned.conversationSummary("chat", "role");
    mocks.get.mockResolvedValue({ data: { executionScope: { ...scope, roleId: "other-role" }, snapshot: { ownerId: "a", resources: [{ kind: "summary", id: "chat/summary", ownerId: "a", revision: 2, body: { content: { summary: "new" } } }] } } });
    await owned.conversationSummary("chat", "other-role");
    await expect(owned.editConversationSummary("chat", "old dialog", false, oldView?.summaryViewId)).rejects.toThrow("摘要视图已变化");
    expect(mocks.post).not.toHaveBeenCalled();
  });

  it("keeps same-id owners distinct and coalesces only an explicit continuation", async () => {
    const owned = useDeviceOwnedConversation();
    mocks.get.mockResolvedValue({ data: { executionScope: { ...scope, targetDeviceId: "device", resourceOwnerId: "core" }, historicalConversations: [{ id: "same", ownerId: "device", title: "old" }], snapshot: { ownerId: "core", resources: [
      { kind: "conversation", id: "same", body: { id: "same", title: "independent" } },
      { kind: "conversation", id: "continuation/hash", body: { id: "continuation/hash", title: "continued", conversationOrigin: { ownerId: "device", id: "same" } } },
    ] } } });
    const rows = await owned.conversations("role");
    expect(rows).toHaveLength(2);
    expect(rows.find((row) => row.id === "meshconv1:device:same")?.title).toBe("continued");
    expect(rows.find((row) => row.id === "meshconv1:core:same")?.title).toBe("independent");
  });

  it("routes opaque references to their source and keeps public identity after owner ACK", async () => {
    const owned = useDeviceOwnedConversation();
    owned.enabled.value = true;
    mocks.get.mockResolvedValue(response({}));
    await owned.query("meshconv1:a:same", "role");
    expect(mocks.get).toHaveBeenCalledWith("/api/device-mesh/v1/business/conversations/same", { params: { characterId: "role", conversationOwnerId: "a" } });
    mocks.stream.mockImplementation(async (request, _signal, emit) => {
      expect(request.conversationId).toBe("same");
      expect(request.conversationOrigin).toEqual({ ownerId: "a", id: "same" });
      expect(request.context.conversationId).toBe("same");
      emit({ type: "started", conversationId: "continuation/hash", executionScope: scope });
      return { conversationId: "continuation/hash", conversationOrigin: { ownerId: "a", id: "same" }, executionScope: scope };
    });
    const seen: any[] = [];
    const result = await owned.send({ requestId: "request", message: "hello", conversationId: "meshconv1:a:same", context: { previousCoreId: "old-core", conversationId: "meshconv1:a:same", messages: [] } }, (event) => seen.push(event));
    expect(seen[0].conversationId).toBe("meshconv1:a:same");
    expect(result.conversationId).toBe("meshconv1:a:same");
    const origin = { ownerId: "设备:a", id: "对话/id:1" };
    expect(parseConversationReference(conversationReference(origin))).toEqual(origin);
    expect(() => parseConversationReference("meshconv1:a:")).toThrow();
  });

  it("keeps index rebuild on the displayed owner and authority", async () => {
    const owned = useDeviceOwnedConversation();
    owned.enabled.value = true;
    owned.coreId.value = "core";
    const result = { executionScope: scope, status: { ownerId: "a", roleId: "role", layers: [] } };
    mocks.get.mockResolvedValue({ data: result });
    await owned.projections("role");
    mocks.post.mockResolvedValue({ data: result });
    await owned.projections("role", scope as any);
    expect(mocks.post).toHaveBeenCalledWith("/api/device-mesh/v1/business/projections/rebuild", { characterId: "role", expectedExecutionScope: scope });
    mocks.get.mockResolvedValue({ data: { ...result, status: { ...result.status, ownerId: "another-device" } } });
    await expect(owned.projections("role")).rejects.toThrow("索引状态已丢弃");
  });

  it("reads all owner pages and keeps same message IDs at different owners", async () => {
    const owned = useDeviceOwnedConversation();
    const first = response({ message: "next" }, {});
    first.data.snapshot.resources = [{ kind: "message", id: "same", ownerId: "a", revision: 1, body: { id: "same", role: "user", content: "current", createdAt: "2026-10-04T00:00:00Z" } }] as any;
    (first.data as any).historicalSnapshot.legacyMessages = [{ id: "same", role: "user", content: "old", createdAt: "2026-10-03T00:00:00Z" }];
    const second = response({});
    second.data.snapshot.resources = [{ kind: "message", id: "second", ownerId: "a", revision: 1, body: { id: "second", role: "assistant", content: "later", createdAt: "2026-10-04T00:00:01Z" } }] as any;
    mocks.get.mockResolvedValueOnce(first).mockResolvedValueOnce(second);
    const rows = await owned.allMessages("meshconv1:a:chat", "role");
    expect(rows.map((row) => row.content)).toEqual(["old", "current", "later"]);
    expect(rows.every((row) => row.conversationId === "meshconv1:a:chat")).toBe(true);
    expect(mocks.get.mock.calls.at(-1)?.[1].params.cursor).toBe("next");
  });

  it("rejects a repeated owner history cursor", async () => {
    mocks.get.mockResolvedValue(response({ message: "repeated" }));
    await expect(useDeviceOwnedConversation().allMessages("chat", "role")).rejects.toThrow("历史分页结果无效");
    expect(mocks.get).toHaveBeenCalledTimes(2);
  });

  it("submits the displayed resource revision even after the cache is refreshed", async () => {
    const owned = useDeviceOwnedConversation();
    owned.enabled.value = true;
    owned.coreId.value = "core";
    mocks.get.mockResolvedValue({ data: { executionScope: scope, snapshot: { ownerId: "a", resources: [{ kind: "message", id: "message", ownerId: "a", roleId: "role", revision: 9, deleted: false, body: {} }] } } });
    await owned.query("chat", "role");
    mocks.post.mockResolvedValue({ data: { ownerId: "a", versions: { "message/message": 8 } } });
    await owned.edit("message", "message", { content: "edit" }, { characterId: "role", expectedRevision: 7, expectedExecutionScope: scope as any, expectedOwnerId: "a" });
    expect(mocks.post.mock.calls[0]?.[1].expectedRevision).toBe(7);
  });

  it("retains source revisions for owned messages and leaves legacy messages read-only", () => {
    const owned = useDeviceOwnedConversation();
    const messages = owned.messages({ executionScope: scope, historicalSnapshot: { ownerId: "old-device", resources: [], legacyMessages: [{ id: "legacy", role: "user", content: "old" }] }, snapshot: { ownerId: "a", resources: [{ kind: "message", id: "current", revision: 7, body: { id: "current", role: "assistant", content: "current" } }] } } as any);
    expect(messages[0].sourceRevision).toBeNull();
    expect(messages[0].ownerId).toBe("old-device");
    expect(messages[1].sourceRevision).toBe(7);
  });

  it("passes the original device memory cursor after the owned store is exhausted", async () => {
    const owned = useDeviceOwnedConversation();
    owned.coreId.value = "core";
    mocks.get.mockResolvedValue(response({}));
    await owned.data("memory", "role", "", "", { legacyCursor: "device-original-page" });
    expect(mocks.get).toHaveBeenCalledWith("/api/device-mesh/v1/business/data", { params: { kind: "memory", characterId: "role", conversationId: "", cursor: "", legacyCursor: "device-original-page" } });
  });

  it("passes both historical memory cursors without changing the Core role", async () => {
    const owned = useDeviceOwnedConversation();
    owned.coreId.value = "core";
    mocks.get.mockResolvedValue(response({}));
    await owned.data("memory", "role", "", "", { historicalRoleId: "device-old-role", historicalCursor: "owned-page", historicalLegacyCursor: "legacy-page" });
    expect(mocks.get).toHaveBeenCalledWith("/api/device-mesh/v1/business/data", { params: { kind: "memory", characterId: "role", conversationId: "", cursor: "", historicalRoleId: "device-old-role", historicalCursor: "owned-page", historicalLegacyCursor: "legacy-page" } });
  });

  it("preserves the authority seen by the page when submitting an edit", async () => {
    const owned = useDeviceOwnedConversation();
    owned.enabled.value = true;
    owned.coreId.value = "core";
    const resource = { kind: "memory", id: "same-id", ownerId: "a", roleId: "role", revision: 1, deleted: false, body: {} };
    mocks.get.mockResolvedValue({ data: { executionScope: scope, snapshot: { ownerId: "a", resources: [resource] } } });
    await owned.data("memory", "role");
    const displayed = { ...scope, coreId: "previous-core" } as any;
    mocks.post.mockRejectedValue(new Error("数据来源已变化"));
    await expect(owned.edit("memory", "same-id", { allowContextUse: false }, { characterId: "role", expectedExecutionScope: displayed, expectedOwnerId: "a" })).rejects.toThrow("数据来源已变化");
    expect(mocks.post.mock.calls[0]?.[1].expectedExecutionScope.coreId).toBe("previous-core");
  });
  it("pages each owner independently and does not restart exhausted sources", async () => {
    const owned = useDeviceOwnedConversation();
    mocks.get.mockResolvedValueOnce(response({ message: "core-1", legacyMessage: "local-1" }, { message: "old-1", legacyMessage: "old-local-1" }));
    await owned.query("chat", "role");
    expect(owned.hasMore("chat", "role")).toBe(true);
    mocks.get.mockResolvedValueOnce(response({ legacyMessage: "local-2" }));
    await owned.query("chat", "role", true);
    expect(mocks.get.mock.calls.at(-1)?.[1].params).toEqual({ characterId: "role", resourceKind: "message", cursor: "core-1", legacyCursor: "local-1", historicalCursor: "old-1", historicalLegacyCursor: "old-local-1" });
    mocks.get.mockResolvedValueOnce(response({ message: "unrequested-first-page" }));
    await owned.query("chat", "role", true);
    expect(mocks.get.mock.calls.at(-1)?.[1].params).toEqual({ characterId: "role", resourceKind: "message", legacyCursor: "local-2" });
    expect(owned.hasMore("chat", "role")).toBe(false);
  });
  it("rejects a changed role revision between pages", async () => {
    const owned = useDeviceOwnedConversation();
    mocks.get.mockResolvedValueOnce(response({ message: "cursor" }));
    await owned.query("chat", "role");
    mocks.get.mockResolvedValueOnce(response({}, undefined, 2));
    await expect(owned.query("chat", "role", true)).rejects.toThrow("角色已变化");
  });
  it("loads conversation lists beyond the first page", async () => {
    const owned = useDeviceOwnedConversation();
    const first = response({ conversation: "page-two" });
    first.data.snapshot.resources = [{ kind: "conversation", id: "first", body: { id: "first" } }] as any;
    const second = response({});
    second.data.snapshot.resources = [{ kind: "conversation", id: "second", body: { id: "second" } }] as any;
    mocks.get.mockResolvedValueOnce(first).mockResolvedValueOnce(second);
    expect((await owned.conversations("role")).map((row) => row.id)).toEqual(["meshconv1:a:first", "meshconv1:a:second"]);
    expect(mocks.get.mock.calls.at(-1)?.[1].params.cursor).toBe("page-two");
  });
  it("rejects repeated list cursors instead of looping", async () => {
    const owned = useDeviceOwnedConversation();
    mocks.get.mockResolvedValue(response({ conversation: "repeated" }));
    await expect(owned.conversations("role")).rejects.toThrow("分页结果无效");
    expect(mocks.get).toHaveBeenCalledTimes(2);
  });
  it("continues historical-device lists after current Core pages are exhausted", async () => {
    const owned = useDeviceOwnedConversation();
    mocks.get.mockResolvedValueOnce({ data: { ...response({}).data, historicalConversations: [{ id: "old-first", ownerId: "old-device" }], nextHistoricalListCursor: "device-page-two" } });
    mocks.get.mockResolvedValueOnce({ data: { ...response({}).data, historicalConversations: [{ id: "old-second", ownerId: "old-device" }] } });
    expect((await owned.conversations("role")).map((row) => row.id)).toEqual(["meshconv1:old-device:old-first", "meshconv1:old-device:old-second"]);
    expect(mocks.get.mock.calls.at(-1)?.[1].params).toEqual({ characterId: "role", resourceKind: "conversation", historicalListCursor: "device-page-two" });
  });
  it("shares list loading across the sidebar and conversation picker", async () => {
    const owned = useDeviceOwnedConversation();
    let finish!: (value: any) => void;
    mocks.get.mockReturnValueOnce(new Promise((resolve) => { finish = resolve; }));
    const sidebar = owned.conversations("role");
    const picker = owned.conversations("role");
    finish(response({}));
    expect(await sidebar).toEqual([]);
    expect(await picker).toEqual([]);
    expect(mocks.get).toHaveBeenCalledTimes(1);
  });
  it("drops an older load that finishes after a newer load", async () => {
    const owned = useDeviceOwnedConversation();
    let finish!: (value: any) => void;
    mocks.get.mockReturnValueOnce(new Promise((resolve) => { finish = resolve; }));
    const earlier = owned.query("chat", "role");
    mocks.get.mockResolvedValueOnce(response({ message: "new-page" }));
    await owned.query("chat", "role");
    finish(response({ message: "stale-page" }));
    await expect(earlier).rejects.toThrow("加载请求已变化");
    mocks.get.mockResolvedValueOnce(response({}));
    await owned.query("chat", "role", true);
    expect(mocks.get.mock.calls.at(-1)?.[1].params.cursor).toBe("new-page");
  });
  it("blocks local fallback while the successor is awaiting approval", async () => {
    const owned = useDeviceOwnedConversation();
    mocks.transition.mockResolvedValue({ providerChangePending: true, coreId: "b", successorCoreId: "c" });
    await expect(owned.refresh()).rejects.toThrow("等待新服务批准");
    expect(owned.enabled.value).toBe(true);
    expect(owned.roles.value).toEqual([]);
    expect(mocks.get).not.toHaveBeenCalled();
  });
	 it("restores a Core switch from native state even without previous browser history", async () => {
		const owned = useDeviceOwnedConversation();
		owned.coreId.value = "";
		owned.enabled.value = false;
		owned.policy.value = null;
		mocks.transition.mockResolvedValue({ coreId: "core-c", previousCoreId: "core-b", providerChangeId: "native-cold-start-change" });
		mocks.get.mockImplementation(async (path) => ({ data: path.endsWith("/coordination/me") ? { coreId: "core-c", coordinationAvailable: true, policy: { coordinated: true, providerEpoch: 1, modeRevision: 1, permissionRevision: 1 } } : { roles: [], roleOwnerId: "core-c" } }));
		await owned.refresh();
		expect(owned.notice.value).toContain("已从「core-b」切换为「core-c」");
		owned.stopLocal("已查看提示");
		await owned.refresh();
		expect(owned.notice.value).toBe("已查看提示");
	});
});
