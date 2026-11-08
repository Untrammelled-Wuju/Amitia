import { onMounted, onUnmounted, ref } from "vue";
import { getDeploymentConfig } from "../runtime/runtime-adapter";
import { apiClient } from "./useApi";

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

  async function refresh() {
    const current = generation;
    checking.value = true;
    error.value = "";
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
      if (current !== generation) return access.value;
      access.value = next;
      return next;
    } catch (cause) {
      if (current === generation) {
        const message = cause instanceof Error ? cause.message : String(cause);
        access.value = {
          read: true,
          write: false,
          reason: message,
          coreId: "",
        };
        error.value = message;
      }
      return access.value;
    } finally {
      if (current === generation) checking.value = false;
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
    void refresh();
  }

  async function requireWrite() {
    const current = await refresh();
    if (!current.write) {
      throw new Error(current.reason || "当前设备只能查看统筹中心数据");
    }
    return current;
  }

  onMounted(() => {
    void refresh();
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
