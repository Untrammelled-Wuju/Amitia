import type { MenuItemConstructorOptions } from "electron";
import type { DesktopActionBridge } from "./action-bridge";
import type { DesktopSnapshot, ResolvedContribution } from "./types";

const TRAY_GROUP_ORDER = ["quick_actions", "extensions", "status"] as const;

export class TrayHost {
  private readonly actionBridge: DesktopActionBridge;
  private readonly setExtensionItems: (
    items: MenuItemConstructorOptions[],
  ) => Promise<void>;

  constructor(
    actionBridge: DesktopActionBridge,
    setExtensionItems: (
      items: MenuItemConstructorOptions[],
    ) => Promise<void>,
  ) {
    this.actionBridge = actionBridge;
    this.setExtensionItems = setExtensionItems;
  }

  applySnapshot(snapshot: DesktopSnapshot): void {
    void this.setExtensionItems(this.buildTemplate(snapshot));
  }

  cleanup(): void {
    void this.setExtensionItems([]);
  }

  private buildTemplate(
    snapshot: DesktopSnapshot,
  ): MenuItemConstructorOptions[] {
    const items: MenuItemConstructorOptions[] = [];
    for (const group of TRAY_GROUP_ORDER) {
      const contribs = snapshot.trayTree[`tray.${group}`] ?? [];
      const built = this.buildExtensionItems(contribs);
      if (built.length > 0) {
        items.push(...built);
      }
    }
    return items;
  }

  private buildExtensionItems(
    contribs: ResolvedContribution[],
  ): MenuItemConstructorOptions[] {
    const registered = contribs.filter((c) => c.status === "registered");
    if (registered.length === 0) return [];
    const sorted = [...registered].sort(
      (a, b) => a.definition.order.priority - b.definition.order.priority,
    );
    const items: MenuItemConstructorOptions[] = [];
    let lastGroup: string | undefined;
    for (const contrib of sorted) {
      const group = contrib.definition.order.group;
      if (group && lastGroup !== undefined && group !== lastGroup) {
        items.push({ type: "separator" });
      }
      lastGroup = group;
      items.push(this.buildExtensionItem(contrib));
    }
    return items;
  }

  private buildExtensionItem(
    contrib: ResolvedContribution,
  ): MenuItemConstructorOptions {
    const def = contrib.definition;
    const isStatus =
      def.desktopType === "app.tray.item" && def.target === "status";
    return {
      id: `ext:${def.contributionId}`,
      label: contrib.effectiveLabel,
      enabled: isStatus ? false : this.evaluateEnabled(contrib),
      click: isStatus
        ? undefined
        : () => {
            void this.actionBridge.invokeAction({
              contributionId: def.contributionId,
              extensionId: def.extensionId,
              input: def.action.input,
            });
          },
    };
  }

  private evaluateEnabled(contrib: ResolvedContribution): boolean {
    const rule = contrib.definition.enabledRule;
    if (rule?.platform && !rule.platform.includes(process.platform)) {
      return false;
    }
    return true;
  }
}
