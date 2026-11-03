import { createIdentityExtension } from "./common.mjs";

function eventBody(input) {
  return {
    summary: input.title || "",
    description: input.description || "",
    location: input.location || "",
    start: { dateTime: input.start, timeZone: input.timeZone || "UTC" },
    end: { dateTime: input.end, timeZone: input.timeZone || "UTC" },
    attendees: Array.isArray(input.attendees)
      ? input.attendees.map((email) => ({ email }))
      : [],
  };
}

async function google(requestJSON, credential, input, operation) {
  const base = "https://www.googleapis.com/calendar/v3/calendars";
  const calendar = encodeURIComponent(input.calendarId || "primary");
  const headers = { Authorization: `Bearer ${credential}` };
  if (operation === "events.list") {
    const query = new URLSearchParams({
      timeMin: input.timeMin || new Date().toISOString(),
      timeMax: input.timeMax || new Date(Date.now() + 7 * 86400000).toISOString(),
      maxResults: String(Math.min(Number(input.limit || 50), 250)),
      singleEvents: "true",
      orderBy: "startTime",
      ...(input.query ? { q: input.query } : {}),
    });
    return requestJSON(`${base}/${calendar}/events?${query}`, { headers });
  }
  if (operation === "events.create") {
    return requestJSON(`${base}/${calendar}/events`, {
      method: "POST",
      headers,
      body: JSON.stringify(eventBody(input)),
    });
  }
  if (operation === "events.update") {
    return requestJSON(`${base}/${calendar}/events/${encodeURIComponent(input.eventId)}`, {
      method: "PATCH",
      headers,
      body: JSON.stringify(eventBody(input)),
    });
  }
  if (operation === "events.delete") {
    return requestJSON(`${base}/${calendar}/events/${encodeURIComponent(input.eventId)}`, {
      method: "DELETE",
      headers,
    });
  }
  throw new Error(`不支持的 Google Calendar 操作: ${operation}`);
}

async function microsoft(requestJSON, credential, input, operation) {
  const base = "https://graph.microsoft.com/v1.0/me/events";
  const headers = { Authorization: `Bearer ${credential}` };
  if (operation === "events.list") {
    const query = new URLSearchParams({
      $top: String(Math.min(Number(input.limit || 50), 100)),
      $orderby: "start/dateTime",
      ...(input.query ? { $search: `"${input.query}"` } : {}),
    });
    return requestJSON(`${base}?${query}`, { headers });
  }
  if (operation === "events.create") {
    return requestJSON(base, {
      method: "POST",
      headers,
      body: JSON.stringify({
        subject: input.title || "",
        body: { contentType: "Text", content: input.description || "" },
        start: { dateTime: input.start, timeZone: input.timeZone || "UTC" },
        end: { dateTime: input.end, timeZone: input.timeZone || "UTC" },
        location: { displayName: input.location || "" },
        attendees: Array.isArray(input.attendees)
          ? input.attendees.map((email) => ({ emailAddress: { address: email }, type: "required" }))
          : [],
      }),
    });
  }
  if (operation === "events.update") {
    return requestJSON(`${base}/${encodeURIComponent(input.eventId)}`, {
      method: "PATCH",
      headers,
      body: JSON.stringify({
        subject: input.title,
        body: input.description ? { contentType: "Text", content: input.description } : undefined,
        start: input.start ? { dateTime: input.start, timeZone: input.timeZone || "UTC" } : undefined,
        end: input.end ? { dateTime: input.end, timeZone: input.timeZone || "UTC" } : undefined,
      }),
    });
  }
  if (operation === "events.delete") {
    return requestJSON(`${base}/${encodeURIComponent(input.eventId)}`, { method: "DELETE", headers });
  }
  throw new Error(`不支持的 Microsoft Calendar 操作: ${operation}`);
}

const extension = createIdentityExtension({
  kind: "calendar",
  defaultConfig: {
    agentId: "",
    enabled: true,
    provider: { type: "google", calendarId: "primary" },
    credentialRef: "",
    defaults: { timeZone: "UTC" },
  },
  async execute({ input, config, credential, requestJSON }) {
    const operation = String(input.operation || "events.list");
    if (!credential) throw new Error("日历凭据未配置");
    if (config.provider.type === "google") return google(requestJSON, credential, input, operation);
    if (config.provider.type === "microsoft") return microsoft(requestJSON, credential, input, operation);
    throw new Error(`不支持的日历 Provider: ${config.provider.type}`);
  },
});

if (typeof module !== "undefined" && module.exports) module.exports = extension;
if (typeof globalThis.defineExtension === "function") globalThis.defineExtension(extension);
