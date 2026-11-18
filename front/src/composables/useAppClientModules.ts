import { shallowRef, onMounted, type Component } from "vue";
import { apiClient } from "@/composables/useApi";
import { pageModuleComponent, type PageModuleDescriptor } from "@/ui-runtime/pageModuleRuntime";
import { useAppStore } from "@/stores/app";

const APP_CLIENT_MODULES_ENABLED = false;

const APP_SLOT_PREFIXES = ["ui.portal.", "app.shell.", "provider.app."];

interface AppContributionSummary {
  contribution_id: string;
  extension_id: string;
  slot?: { slot_id?: string };
  entry?: { type?: string; path?: string; content_hash?: string };
  sandbox?: { type?: string };
  integrity?: { generation?: number };
  actions?: Array<{ action_id?: string }>;
}

interface AppModuleTicket {
  extensionId: string;
  contributionId: string;
  entryPath: string;
  entryHash?: string;
  generation?: number;
  accessToken: string;
}

export interface AppClientModule {
  extensionId: string;
  contributionId: string;
  component: Component;
  actions: Record<string, (input?: unknown) => Promise<unknown>>;
}

function buildActionContext(): Record<string, unknown> {
  const appStore = useAppStore();
  const characterId = String(appStore.currentCharacter?.id ?? "").trim();
  return characterId ? { characterId } : {};
}

function buildActions(contributionId: string, actionIds: string[]): Record<string, (input?: unknown) => Promise<unknown>> {
  const actions: Record<string, (input?: unknown) => Promise<unknown>> = {};
  for (const actionId of actionIds) {
    actions[actionId] = async (input?: unknown) => {
      const response = await apiClient.post<Record<string, unknown>>(
        `/api/extension/action/${encodeURIComponent(contributionId)}/${encodeURIComponent(actionId)}`,
        { context: buildActionContext(), input },
      );
      const data = response.data as Record<string, unknown> | undefined;
      if (data?.ok === false) {
        const failure = data.error as Record<string, unknown> | string | undefined;
        throw new Error(typeof failure === "string" ? failure : String((failure as Record<string, unknown> | undefined)?.message ?? JSON.stringify(failure)));
      }
      const result = data?.result as Record<string, unknown> | undefined;
      return result?.structured ?? result ?? data;
    };
  }
  return actions;
}

export function useAppClientModules(options: { enabled?: boolean; contributionIds?: readonly string[] } = {}) {
  const modules = shallowRef<AppClientModule[]>([]);
  const error = shallowRef<Error | null>(null);

  onMounted(async () => {
    if (!(options.enabled ?? APP_CLIENT_MODULES_ENABLED)) return;
    const listed = await apiClient.get<{ contributions?: AppContributionSummary[] }>(
      "/api/extensions/ui/contributions",
    );
    const contributions = listed.data?.contributions ?? [];
    const targets = contributions.filter((item) => {
      const slotId = String(item.slot?.slot_id ?? "");
      const selected = options.contributionIds
        ? options.contributionIds.includes(item.contribution_id)
        : APP_SLOT_PREFIXES.some((prefix) => slotId.startsWith(prefix));
      return item.sandbox?.type === "client_module"
        && item.entry?.type === "client_module"
        && selected;
    });
    const loaded: AppClientModule[] = [];
    for (const item of targets) {
      try {
        const ticketResponse = await apiClient.post<AppModuleTicket>(
          "/api/extensions/ui/app-module-ticket",
          { extensionId: item.extension_id, contributionId: item.contribution_id },
        );
        const ticket = ticketResponse.data;
        const descriptor: PageModuleDescriptor = {
          extensionId: ticket.extensionId,
          pageId: ticket.contributionId,
          generation: Number(ticket.generation ?? item.integrity?.generation ?? 0),
          entryPath: ticket.entryPath,
          entryHash: ticket.entryHash,
          moduleToken: ticket.accessToken,
        };
        const actionIds = (item.actions ?? [])
          .map((action) => String(action.action_id ?? "").trim())
          .filter((actionId) => actionId !== "");
        loaded.push({
          extensionId: item.extension_id,
          contributionId: item.contribution_id,
          component: pageModuleComponent(descriptor),
          actions: buildActions(item.contribution_id, actionIds),
        });
      } catch (cause) {
        const failure = cause instanceof Error ? cause : new Error(String(cause));
        error.value = failure;
        console.error(`[app-client-module] ${item.extension_id}/${item.contribution_id} 加载失败: ${failure.stack ?? failure.message}`);
      }
    }
    const hostCommandOwner = loaded[0];
    if (hostCommandOwner) {
      const command = hostCommandOwner.actions.command;
      (globalThis as unknown as { __dshHostCommand?: (input?: unknown) => Promise<unknown> }).__dshHostCommand = (input?: unknown) => command(input);
    }
    modules.value = loaded;
  });

  return { modules, error };
}
