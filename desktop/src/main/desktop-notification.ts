import { app, Notification, type NotificationConstructorOptions } from "electron";
import { hostname, release, type } from "node:os";

export interface DesktopNotificationPayload {
  requestId: string;
  type?: string;
  title: string;
  body: string;
  conversationId?: string;
  groupTitle?: string;
  sound: boolean;
}

function desktopOSVersion(platform: NodeJS.Platform): string {
  if (platform === "darwin") return `macOS ${release()}`;
  if (platform === "win32") return `Windows ${release()}`;
  if (platform === "linux") return `${type()} ${release()}`;
  return `${platform} ${release()}`;
}

export function desktopNotificationMetadata(platform: NodeJS.Platform = process.platform) {
  return {
    deviceName: hostname(),
    osVersion: desktopOSVersion(platform),
  };
}

export function configureDesktopNotificationHost(platform: NodeJS.Platform = process.platform): void {
  let userData = "";
  let sessionData = "";
  try {
    userData = app.getPath("userData");
    sessionData = app.getPath("sessionData");
  } catch (error) {
  console.error("[original-error]", error);
}
  app.setName("Amitia");
  try {
    if (userData) app.setPath("userData", userData);
    if (sessionData) app.setPath("sessionData", sessionData);
  } catch (error) {
  console.error("[original-error]", error);
}
  if (platform === "win32") {
    app.setAppUserModelId("cn.amitia.desktop");
  } else if (platform === "linux") {
    try {
      app.setDesktopName("amitia.desktop");
    } catch (error) {
  console.error("[original-error]", error);
}
  }
}

export function desktopNotificationOptions(
  payload: DesktopNotificationPayload,
  platform: NodeJS.Platform = process.platform,
): NotificationConstructorOptions {
  const options: NotificationConstructorOptions = {
    title: payload.title,
    body: payload.body,
    silent: !payload.sound,
  };
  if (platform === "darwin" || platform === "win32") {
    options.id = payload.requestId;
    if (payload.conversationId) options.groupId = payload.conversationId;
    if (platform === "win32" && payload.groupTitle) options.groupTitle = payload.groupTitle;
    if (platform === "win32" && (payload.type?.startsWith("call.") || payload.type?.startsWith("run."))) {
      options.timeoutType = "never";
    }
  }
  if (platform === "linux") {
    options.urgency = payload.type?.startsWith("call.") ? "critical" : payload.type?.startsWith("run.") ? "low" : "normal";
  }
  return options;
}

export function createDesktopNotification(
  payload: DesktopNotificationPayload,
  platform: NodeJS.Platform = process.platform,
): Notification | null {
  if (!Notification.isSupported()) return null;
  try {
    return new Notification(desktopNotificationOptions(payload, platform));
  } catch (error) {
  console.error("[original-error]", error);
return null;
  }
}
