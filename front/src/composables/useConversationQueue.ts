import { onUnmounted, ref, watch, type Ref } from "vue";
import { ownedChatRuntimeRequest, ownedChatRuntimeScopedRequest } from "@/runtime/owned-chat-management";
import type { OwnedExecutionScope } from "@/runtime/device-owned-chat";

export interface QueuedMessage {
  requestId: string;
  message: string;
  position: number;
  status: string;
  error?: string;
  attachments?: Array<{ kind: string; name: string; mimeType: string }>;
  quote?: { ownerId: string; characterId: string; conversationId: string; messageId: string; expectedRevision: number };
}

export function useConversationQueue(conversationId: Ref<string>, characterId: Ref<string>, activeTurnId: Ref<string>) {
  const items = ref<QueuedMessage[]>([]);
  const scope = ref<OwnedExecutionScope | null>(null);
  const error = ref("");
  const busy = ref(false);
  let generation = 0;
  let disposed = false;
  let timer: ReturnType<typeof setTimeout> | undefined;

  function describeError(value: unknown) {
    if (value && typeof value === "object") {
      const detail = value as Record<string, unknown>;
      const seen = new WeakSet<object>();
      const raw = JSON.stringify(detail, (_key, entry) => {
        if (entry && typeof entry === "object") {
          if (seen.has(entry)) return "[循环引用]";
          seen.add(entry);
        }
        return entry;
      }, 2);
      return [value instanceof Error ? value.stack || value.message : "", raw].filter(Boolean).join("\n");
    }
    return String(value);
  }

  async function refresh() {
    const id = conversationId.value;
    const role = characterId.value;
    const current = ++generation;
    if (!id || !role) {
      items.value = [];
      scope.value = null;
      error.value = "";
      return;
    }
    try {
      const request = scope.value
        ? ownedChatRuntimeScopedRequest<{ items: QueuedMessage[]; executionScope: OwnedExecutionScope }>("queue/list", { conversationId: id }, scope.value)
        : ownedChatRuntimeRequest<{ items: QueuedMessage[]; executionScope: OwnedExecutionScope }>("queue/list", { conversationId: id }, role);
      const result = await request;
      if (disposed || current !== generation || conversationId.value !== id || characterId.value !== role) return;
      items.value = result.items;
      scope.value = result.executionScope;
      error.value = "";
    } catch (cause) {
      if (!disposed && current === generation && conversationId.value === id && characterId.value === role) error.value = describeError(cause);
    }
  }

  async function operate(operation: string, payload: Record<string, unknown>) {
    if (busy.value) throw new Error("正在处理排队消息，请稍后重试");
    const id = conversationId.value;
    const role = characterId.value;
    if (!id || !role) throw new Error("当前会话或角色不可用");
    busy.value = true;
    try {
      const body = { conversationId: id, ...payload };
      const result = scope.value
        ? await ownedChatRuntimeScopedRequest(operation, body, scope.value)
        : await ownedChatRuntimeRequest(operation, body, role);
      if (result?.executionScope) scope.value = result.executionScope;
      if (conversationId.value === id && characterId.value === role) await refresh();
    } catch (cause) {
      scope.value = null;
      throw cause;
    } finally {
      busy.value = false;
    }
  }

  function poll() {
    if (disposed) return;
    void refresh().finally(() => { if (!disposed) timer = setTimeout(poll, 2000); });
  }

  watch([conversationId, characterId], () => { items.value = []; scope.value = null; void refresh(); }, { immediate: true });
  timer = setTimeout(poll, 2000);
  onUnmounted(() => { disposed = true; generation++; if (timer) clearTimeout(timer); });

  return {
    items,
    error,
    busy,
    enqueue: (message: string, options: Record<string, unknown>) => operate("queue/enqueue", { message, ...options }),
    remove: (queuedRequestId: string) => operate("queue/delete", { queuedRequestId }),
    steer: (queuedRequestId: string) => operate("queue/steer", { queuedRequestId, turnId: activeTurnId.value }),
    reorder: (queueOrder: string[]) => operate("queue/reorder", { queueOrder }),
    refresh,
    reportError: (cause: unknown) => { error.value = describeError(cause); return error.value; },
  };
}
