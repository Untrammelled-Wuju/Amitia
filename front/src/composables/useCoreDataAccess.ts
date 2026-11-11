import { onMounted, onUnmounted, ref } from "vue";
import { getDeploymentConfig } from "../runtime/runtime-adapter";
import { apiClient } from "./useApi";
import { originalErrorText } from "../runtime/original-error";

export type CoreDataAccessState = {
  read: boolean;
  write: boolean;
  reason: string;
  coreId: string;
};

export function parseCoreDataAccess(status: any): CoreDataAccessState {
  const policy = status?.policy;
  const access = status?.coreDataAccess;
  if (
    typeof status?.coreId !== "string"
    || !status.coreId
    || status?.coordinationAvailable !== true
    || !policy
    || !Number.isSafeInteger(policy.permissionRevision)
    || policy.permissionRevision < 1
    || typeof access?.read !== "boolean"
    || typeof access?.write !== "boolean"
  ) {
    throw new Error("统筹中心未返回有效的数据访问权限");
  }
  const reason = String(access.reason || "");
  if (access.read !== true || (access.write !== true && !reason.trim())) {
    throw new Error("统筹中心数据访问权限状态不一致");
  }
  return {
    read: access.read,
    write: access.write,
    reason,
    coreId: status.coreId,
  };
}

export function useCoreDataAccess() {
  const access = ref<CoreDataAccessState>({
    read: true,
    write: false,
    reason: "正在确认统筹中心数据权限",
    coreId: "",
  });
  const checking = ref(true);
  const error = ref("");
  let generation = 0;
  let refreshNumber = 0;

  async function refresh() {
    const current = generation;
    const request = ++refreshNumber;
    checking.value = true;
    error.value = "";
    access.value = { read: true, write: false, reason: "正在确认统筹中心数据权限", coreId: "" };
    try {
      const deployment = await getDeploymentConfig();
      let next: CoreDataAccessState;
      if (deployment.mode !== "cloud") {
        next = { read: true, write: true, reason: "", coreId: "" };
      } else {
        const response = await apiClient.get<any>(
          "/api/device-mesh/v1/coordination/me",
        );
        next = parseCoreDataAccess(
          response?.data?.data ?? response?.data ?? response,
        );
      }
      if (current !== generation || request !== refreshNumber) throw new Error("统筹中心权限检查已更新，请重试");
      access.value = next;
      return next;
    } catch (cause) {
      if (current === generation && request === refreshNumber) {
        const message = originalErrorText(cause);
        access.value = {
          read: true,
          write: false,
          reason: message,
          coreId: "",
        };
        error.value = message;
      }
      throw cause;
    } finally {
      if (current === generation && request === refreshNumber) checking.value = false;
    }
  }

  function invalidate() {
    generation++;
    access.value = {
      read: true,
      write: false,
      reason: "正在确认统筹中心数据权限",
      coreId: "",
    };
    observeRefresh();
  }

  function observeRefresh() {
    const current = generation;
    const promise = refresh();
    const request = refreshNumber;
    void promise.catch((cause) => {
      if (current === generation && request === refreshNumber) error.value = originalErrorText(cause);
    });
  }

  async function requireWrite() {
    const current = await refresh();
    if (!current.write) {
      throw new Error(current.reason || "当前设备只能查看统筹中心数据");
    }
    return current;
  }

  onMounted(() => {
    observeRefresh();
    window.addEventListener("amitia:runtime-connection-changed", invalidate);
    window.addEventListener("amitia:execution-scope-changed", invalidate);
    window.addEventListener("focus", invalidate);
    document.addEventListener("visibilitychange", invalidate);
  });
  onUnmounted(() => {
    generation++;
    window.removeEventListener("amitia:runtime-connection-changed", invalidate);
    window.removeEventListener("amitia:execution-scope-changed", invalidate);
    window.removeEventListener("focus", invalidate);
    document.removeEventListener("visibilitychange", invalidate);
  });

  return { access, checking, error, refresh, requireWrite };
}
