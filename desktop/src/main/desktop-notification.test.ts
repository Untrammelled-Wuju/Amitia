import { expect, it, vi } from "vitest";

const notificationMock = vi.hoisted(() => ({
  supported: true,
  instances: [] as Array<{ options: Record<string, unknown>; show: ReturnType<typeof vi.fn> }>,
}));
const appMock = vi.hoisted(() => ({
  getPath: vi.fn((name: string) => `/tmp/${name}`),
  setPath: vi.fn(),
  setName: vi.fn(),
  setAppUserModelId: vi.fn(),
  setDesktopName: vi.fn(),
}));

vi.mock("electron", () => ({
  app: appMock,
  Notification: class {
    static isSupported() {
      return notificationMock.supported;
    }
    show = vi.fn();
    constructor(options: Record<string, unknown>) {
      notificationMock.instances.push({ options, show: this.show });
    }
  },
}));

import {
  configureDesktopNotificationHost,
  createDesktopNotification,
  desktopNotificationMetadata,
  desktopNotificationOptions,
} from "./desktop-notification";

const payload = {
  requestId: "notification-1",
  type: "message.completed",
  title: "林澈",
  body: "消息内容",
  conversationId: "conversation-1",
  groupTitle: "林澈",
  sound: true,
};

it("maps macOS and Windows notification identity and grouping", () => {
  expect(desktopNotificationOptions(payload, "darwin")).toMatchObject({
    id: "notification-1",
    groupId: "conversation-1",
    silent: false,
  });
  expect(desktopNotificationOptions(payload, "win32")).toMatchObject({
    id: "notification-1",
    groupId: "conversation-1",
    groupTitle: "林澈",
  });
});

it("maps Linux notification urgency without unsupported platform fields", () => {
  expect(desktopNotificationOptions({ ...payload, type: "call.incoming" }, "linux")).toMatchObject({
    urgency: "critical",
  });
  expect(desktopNotificationOptions({ ...payload, type: "run.update" }, "linux")).toMatchObject({
    urgency: "low",
  });
  expect(desktopNotificationOptions(payload, "linux")).not.toHaveProperty("id");
  expect(desktopNotificationOptions(payload, "linux")).not.toHaveProperty("groupId");
});

it("reports host OS metadata and fails closed when notifications are unsupported", () => {
  expect(desktopNotificationMetadata("darwin").osVersion).toMatch(/^macOS /);
  expect(desktopNotificationMetadata("win32").osVersion).toMatch(/^Windows /);
  expect(desktopNotificationMetadata("linux").osVersion.length).toBeGreaterThan(0);
  notificationMock.supported = false;
  expect(createDesktopNotification(payload, "linux")).toBeNull();
});

it("configures Windows and Linux notification host identity without changing user data paths", () => {
  configureDesktopNotificationHost("win32");
  configureDesktopNotificationHost("linux");
  expect(appMock.setPath).toHaveBeenCalledWith("userData", "/tmp/userData");
  expect(appMock.setPath).toHaveBeenCalledWith("sessionData", "/tmp/sessionData");
  expect(appMock.setAppUserModelId).toHaveBeenCalledWith("cn.amitia.desktop");
  expect(appMock.setDesktopName).toHaveBeenCalledWith("amitia.desktop");
});
