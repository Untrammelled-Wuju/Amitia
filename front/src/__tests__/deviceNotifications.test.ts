import { beforeEach, expect, it } from "vitest";
import {
  legacyReplyPreferences,
  notificationRegistrationPayload,
} from "../runtime/device-notifications";

beforeEach(() => localStorage.clear());

it("builds a complete canonical device registration payload", () => {
  const payload = notificationRegistrationPayload("device-1", "desktop");
  expect(payload).toMatchObject({
    deviceId: "device-1",
    platform: "desktop",
    preferredProvider: "desktop-sse",
  });
  expect(payload.locale).toBeTruthy();
  expect(payload.timezone).toBeTruthy();
  expect(payload).not.toHaveProperty("systemNotificationsEnabled");
});

it("preserves legacy reply notification and sound choices for one-time migration", () => {
  localStorage.setItem("amitia.reply-notifications.desktop.v1", "false");
  localStorage.setItem("amitia.reply-sound.desktop.v1", "true");
  expect(legacyReplyPreferences()).toEqual({
    messagePushEnabled: false,
    soundEnabled: true,
  });
});
