// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { computed, ref, watch } from "vue";
import { apiClient } from "../ui-index";
import { useDeviceOwnedConversation } from "./useDeviceOwnedConversation";
import type { OwnedExecutionScope } from "@/runtime/device-owned-chat";

export interface WorldBookEntry {
  id: string;
  scope: "global" | "role";
  matchType: string;
  matchPattern: string;
  matchScope: string;
  injectContent: string;
  priority: number;
  hitCount: number;
  enabled: boolean;
  characterId: string;
  ownerId: string;
  roleId: string;
  revision: number;
  createdAt: string;
  updatedAt: string;
  secondaryKeys?: string[];
  caseSensitive?: boolean;
  selective?: boolean;
  constant?: boolean;
  insertionOrder?: number;
  position?: string;
  extensions?: Record<string, unknown>;
  characterBookEntryName?: string;
  characterBookEntryId?: unknown;
  sourceCardHash?: string;
  sourceEntryIndex?: number;
  executionScope?: OwnedExecutionScope;
}

export interface WorldBookListResponse {
  items: WorldBookEntry[];
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;
}

export interface MatchResult {
  entry: WorldBookEntry;
  matchScope: string;
  hitText: string;
}

export interface TestMatchResponse {
  matches: MatchResult[];
}

const matchTypeLabels: Record<string, string> = {
  regex: "正则匹配",
  exact: "精确匹配",
  keyword: "关键词匹配",
};

const scopeLabels: Record<string, string> = {
  full_context: "全部上下文",
  user_message: "仅用户消息",
  assistant_reply: "仅AI回复",
};

function authorityStamp(scope: OwnedExecutionScope) {
  return JSON.stringify([
    scope.spaceId,
    scope.initiatorDeviceId,
    scope.targetDeviceId,
    scope.coreId,
    scope.providerEpoch,
    scope.targetProviderEpoch,
    scope.coordinated,
    scope.modeRevision,
    scope.permissionRevision,
    scope.targetPermissionRevision,
    scope.roleOwnerId,
    scope.resourceOwnerId,
    scope.roleId,
    scope.roleRevision,
  ]);
}

export function useWorldBook() {
  const owned = useDeviceOwnedConversation();
  const loadedContext = ref("");
  const rules = ref<WorldBookEntry[]>([]);
  const loading = ref(false);
  const total = ref(0);
  const page = ref(1);
  const totalPages = ref(1);
  let currentScope: OwnedExecutionScope | null = null;
  let currentRole = "";

  watch(() => [owned.enabled.value, owned.coreId.value, owned.roleOwnerId.value], () => {
    rules.value = [];
    total.value = 0;
    loadedContext.value = "";
    currentScope = null;
  });

  const canConfigure = computed(() => owned.enabled.value && owned.roles.value.length > 0);
  const contextKey = computed(() => loadedContext.value);

  async function resolveAccess(expected = "") {
    if (!await owned.refresh()) throw new Error(owned.notice.value || "设备世界书服务尚未就绪");
    const role = owned.selectInitialRole();
    if (!role) throw new Error("当前没有可用的世界书角色");
    currentRole = role;
    if (expected && expected !== loadedContext.value) throw new Error("世界书来源已变化，请重新加载");
    return role;
  }

  function resourceRule(resource: any, scope: OwnedExecutionScope): WorldBookEntry {
    const body = resource?.body || {};
    const ruleScope = resource.roleId === "@global-worldbook" || body.scope === "global" ? "global" : "role";
    return {
      ...body,
      id: resource.id,
      scope: ruleScope,
      characterId: ruleScope === "role" ? resource.roleId : "",
      ownerId: resource.ownerId,
      roleId: resource.roleId,
      revision: resource.revision,
      enabled: body.enabled !== false,
      executionScope: scope,
    };
  }

  function filterRules(items: WorldBookEntry[], filters: { matchType?: string; keyword?: string }) {
    const keyword = String(filters.keyword || "").trim().toLowerCase();
    return items.filter((rule) => {
      if (filters.matchType && rule.matchType !== filters.matchType) return false;
      if (!keyword) return true;
      return [rule.matchPattern, rule.injectContent, rule.matchType, rule.matchScope]
        .some((value) => String(value || "").toLowerCase().includes(keyword));
    });
  }

  async function fetchRules(filters: {
    matchType?: string;
    keyword?: string;
    page?: number;
    pageSize?: number;
  } = {}) {
    loading.value = true;
    try {
      const role = await resolveAccess();
      const result = await owned.data("worldbook", role);
      const scope = result.executionScope;
      if (!scope || scope.coreId !== owned.coreId.value || scope.roleId !== role || result.snapshot?.ownerId !== scope.resourceOwnerId) {
        throw new Error("世界书所属设备、Core 或角色无效");
      }
      const loaded = (result.snapshot.resources || [])
        .filter((resource: any) => resource.kind === "worldbook")
        .map((resource: any) => resourceRule(resource, scope));
      const filtered = filterRules(loaded, filters);
      const pageSize = Math.max(1, Math.min(100, filters.pageSize || 20));
      const pageNo = Math.max(1, filters.page || 1);
      const pages = Math.max(1, Math.ceil(filtered.length / pageSize));
      currentScope = scope;
      currentRole = role;
      loadedContext.value = authorityStamp(scope);
      rules.value = filtered.slice((pageNo - 1) * pageSize, pageNo * pageSize);
      total.value = filtered.length;
      page.value = Math.min(pageNo, pages);
      totalPages.value = pages;
    } catch (error) {
      rules.value = [];
      total.value = 0;
      loadedContext.value = "";
      currentScope = null;
      throw error;
    } finally {
      loading.value = false;
    }
  }

  async function requireScope(expected = loadedContext.value) {
    const role = await resolveAccess(expected);
    if (!currentScope || !currentRole || currentRole !== role || !loadedContext.value) {
      await fetchRules();
    }
    if (!currentScope || currentScope.roleId !== role) throw new Error("世界书数据归属尚未加载");
    return currentScope;
  }

  async function manage(action: "create" | "update" | "delete", rule: Partial<WorldBookEntry>, expected: string) {
    const scope = await requireScope(expected);
    const requestId = crypto.randomUUID();
    const payloadRule = {
      id: rule.id || "",
      scope: rule.scope || "role",
      characterId: rule.scope === "global" ? "" : scope.roleId,
      matchType: rule.matchType || "keyword",
      matchPattern: rule.matchPattern || "",
      matchScope: rule.matchScope || "full_context",
      injectContent: rule.injectContent || "",
      priority: Number(rule.priority || 0),
      hitCount: Number(rule.hitCount || 0),
      enabled: rule.enabled !== false,
      secondaryKeys: rule.secondaryKeys || [],
      caseSensitive: rule.caseSensitive,
      selective: rule.selective,
      constant: rule.constant,
      insertionOrder: rule.insertionOrder || 0,
      position: rule.position,
      extensions: rule.extensions || {},
      characterBookEntryName: rule.characterBookEntryName || "",
      characterBookEntryId: rule.characterBookEntryId,
      sourceCardHash: rule.sourceCardHash || "",
      sourceEntryIndex: rule.sourceEntryIndex || 0,
      createdAt: rule.createdAt || "",
      updatedAt: rule.updatedAt || "",
    };
    const result = await apiClient.post<any>("/api/device-mesh/v1/business/worldbook/manage", {
      requestId,
      characterId: scope.roleId,
      targetDeviceId: scope.targetDeviceId,
      expectedExecutionScope: scope,
      action,
      rule: payloadRule,
      expectedRevision: rule.revision || 0,
    });
    const data = result?.data ?? result;
    if (!data?.saved || data.executionScope?.requestId !== requestId ||
        authorityStamp(data.executionScope) !== authorityStamp({ ...scope, requestId: data.executionScope.requestId } as OwnedExecutionScope) ||
        data.acknowledgement?.ownerId !== scope.resourceOwnerId ||
        data.acknowledgement?.requestId !== requestId ||
        data.acknowledgement?.versions?.[`checkpoint/worldbook/${requestId}`] !== 1) {
      throw new Error("世界书尚未获得数据所有者保存确认");
    }
    if (action !== "delete" && (!data.rule || data.rule.ownerId !== scope.resourceOwnerId || data.rule.revision < 1)) {
      throw new Error("世界书保存回执无效");
    }
    await fetchRules();
  }

  async function createRule(data: Partial<WorldBookEntry>, expected = loadedContext.value) {
    await manage("create", data, expected);
  }

  async function createRules(items: Partial<WorldBookEntry>[], expected = loadedContext.value) {
    for (const item of items) {
      const scope = item.scope || (String(item.characterId || "").trim() ? "role" : "global");
      await createRule({ ...item, scope }, expected);
    }
  }

  async function updateRule(id: string, data: Partial<WorldBookEntry>, expected = loadedContext.value) {
    const existing = rules.value.find((rule) => rule.id === id);
    if (!existing) throw new Error("世界书规则已变化，请重新加载");
    await manage("update", { ...existing, ...data, id }, expected);
  }

  async function deleteRule(id: string, expected = loadedContext.value) {
    const existing = rules.value.find((rule) => rule.id === id);
    if (!existing) throw new Error("世界书规则已变化，请重新加载");
    await manage("delete", existing, expected);
  }

  async function testMatch(text: string): Promise<TestMatchResponse | null> {
    const scope = await requireScope();
    const result = await apiClient.post<any>("/api/device-mesh/v1/business/worldbook/match", {
      requestId: crypto.randomUUID(),
      characterId: scope.roleId,
      targetDeviceId: scope.targetDeviceId,
      expectedExecutionScope: scope,
      text,
    });
    const data = result?.data ?? result;
    if (!Array.isArray(data?.matches)) throw new Error("世界书匹配结果无效");
    const byID = new Map([...rules.value].map((rule) => [rule.id, rule]));
    return {
      matches: data.matches.map((match: any) => ({
        entry: byID.get(match.ruleId) || { id: match.ruleId, matchPattern: "", ownerId: scope.resourceOwnerId, roleId: scope.roleId, revision: match.revision },
        matchScope: match.matchScope,
        hitText: match.hitText,
      })),
    };
  }

  async function deleteAll() {
    for (const rule of [...rules.value]) await deleteRule(rule.id, loadedContext.value);
  }

  function matchTypeLabel(type: string) {
    return matchTypeLabels[type] || type;
  }

  function scopeLabel(scope: string) {
    return scopeLabels[scope] || scope;
  }

  return {
    access: {
      canConfigure,
      contextKey,
      requireAccess: async (expected = loadedContext.value) => {
        await requireScope(expected);
        return loadedContext.value;
      },
    },
    loadedContext,
    rules,
    loading,
    total,
    page,
    totalPages,
    fetchRules,
    createRule,
    createRules,
    updateRule,
    deleteRule,
    testMatch,
    deleteAll,
    matchTypeLabel,
    scopeLabel,
  };
}
