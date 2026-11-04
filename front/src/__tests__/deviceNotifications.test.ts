import { beforeEach, expect, it } from "vitest";
import { notificationRegistrationPayload } from "../runtime/device-notifications";

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
