export const DESKTOP_IPC_CHANNELS = {
  SNAPSHOT_APPLY: "desktop:snapshot:apply",
  SNAPSHOT_GET: "desktop:snapshot:get",
  ACTION_INVOKE: "desktop:action:invoke",
  OPERATION_UPDATE: "desktop:operation:update",
  RESOURCE_RESOLVE: "desktop:resource:resolve",
  CLEANUP: "desktop:cleanup",
  SHORCUT_TRIGGERED: "desktop:shortcut:triggered",
  MENU_ITEM_CLICKED: "desktop:menu:item:clicked",
  TRAY_ITEM_CLICKED: "desktop:tray:item:clicked",
} as const;
