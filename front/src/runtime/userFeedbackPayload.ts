// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { resolveUIHostDeviceId } from "@/ui-runtime/deviceIdentity";
import { getDeviceTimezone } from "@/utils/requestEnvelope";
import { collectClientErrors } from "./client-error-tracker";
import { isDesktopShell } from "./runtime-capabilities";
import { getDeploymentConfig } from "./runtime-adapter";

export interface UserFeedbackScreenshot {
  name: string;
  mediaType: string;
  dataBase64: string;
}

export interface UserFeedbackDraft {
  category: string;
  title: string;
  description: string;
  contact: string;
  includeLogs: boolean;
  screenshot: UserFeedbackScreenshot | null;
}

export interface UserFeedbackPayloadContext {
  appVersion: string;
  uploadUrl: string;
}

export async function buildUserFeedbackPayload(
  draft: UserFeedbackDraft,
  context: UserFeedbackPayloadContext,
) {
  let runtimeMode = "local";
  try {
    runtimeMode = (await getDeploymentConfig()).mode;
  } catch {
    runtimeMode = "local";
  }
  let deviceId = "";
  try {
    deviceId = await resolveUIHostDeviceId();
  } catch {
    deviceId = "";
  }
  return {
    category: draft.category,
    title: draft.title.trim(),
    description: draft.description.trim(),
    contact: draft.contact.trim(),
    includeLogs: draft.includeLogs,
    clientErrors: collectClientErrors(),
    platform: isDesktopShell() ? "desktop" : "web",
    clientType: isDesktopShell() ? "desktop" : "web",
    appVersion: context.appVersion,
    runtimeMode,
    deviceId,
    osInfo: typeof navigator !== "undefined" ? navigator.userAgent : "",
    locale: typeof navigator !== "undefined" ? navigator.language : "",
    timezone: getDeviceTimezone(),
    uploadUrl: context.uploadUrl.trim(),
    screenshot: draft.screenshot,
  };
}
