import type { UIContributionSummary } from "@/stores/extensionUI";

export type PluginSurfaceKind =
  | "host_runtime"
  | "schema"
  | "native"
  | "none";

export function resolvePluginSurface(
  contribution: Pick<
    UIContributionSummary,
    "kind" | "sandbox" | "runtimeId"
  >,
): PluginSurfaceKind {
  if (contribution.runtimeId?.trim().startsWith("host.")) {
    return "host_runtime";
  }
  if (contribution.sandbox === "schema_renderer") return "schema";
  if (contribution.sandbox === "host_native") return "native";
  if (
    ["schema_page", "settings_section", "panel", "card"].includes(
      contribution.kind,
    )
  ) {
    return "schema";
  }
  if (contribution.sandbox === "client_module") return "none";
  if (
    [
      "action",
      "menu_item",
      "toolbar_item",
      "status_item",
      "message_action",
      "composer_action",
      "desktop_command",
      "badge",
    ].includes(contribution.kind)
  ) {
    return "native";
  }
  return "none";
}
