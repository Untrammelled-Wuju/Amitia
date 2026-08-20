import type { UIProviderDefinition } from "./types";

function stringList(value: unknown): string[] {
  return Array.isArray(value) ? value.map((item) => String(item ?? "").trim().toLowerCase()).filter(Boolean) : [];
}

function messageMimeTypes(message: Record<string, any>): string[] {
  const values = new Set<string>();
  for (const value of [message.mimeType, message.mime_type, message.contentType, message.content_type]) {
    if (value) values.add(String(value).toLowerCase());
  }
  for (const attachment of Array.isArray(message.attachments) ? message.attachments : []) {
    for (const value of [attachment?.mimeType, attachment?.mime_type, attachment?.contentType, attachment?.type]) {
      if (value) values.add(String(value).toLowerCase());
    }
  }
  return [...values];
}

function mimeMatches(actual: string, expected: string): boolean {
  if (expected === "*/*" || expected === "*") return true;
  if (expected.endsWith("/*")) return actual.startsWith(expected.slice(0, -1));
  return actual === expected;
}

function scoreProvider(provider: UIProviderDefinition, message: Record<string, any>): number | null {
  if (!provider.enabled || provider.builtin || provider.capability !== "conversation.message_renderer") return null;
  const metadata = provider.metadata ?? {};
  const messageTypes = stringList(metadata.messageTypes);
  const roles = stringList(metadata.roles);
  const mimeTypes = stringList(metadata.mimeTypes);
  const extensionTypes = stringList(metadata.extensionTypes);
  const type = String(message.type ?? message.messageType ?? "text").toLowerCase();
  const role = String(message.role ?? "").toLowerCase();
  const extensionType = String(message.extensionType ?? message.extension_type ?? "").toLowerCase();
  const actualMimes = messageMimeTypes(message);

  if (messageTypes.length && !messageTypes.includes(type) && !messageTypes.includes("*")) return null;
  if (roles.length && !roles.includes(role) && !roles.includes("*")) return null;
  if (extensionTypes.length && !extensionTypes.includes(extensionType) && !extensionTypes.includes("*")) return null;
  if (mimeTypes.length && !actualMimes.some((actual) => mimeTypes.some((expected) => mimeMatches(actual, expected)))) return null;

  let specificity = 0;
  if (messageTypes.length) specificity += 8;
  if (roles.length) specificity += 4;
  if (mimeTypes.length) specificity += 4;
  if (extensionTypes.length) specificity += 8;
  return (provider.priority ?? 0) * 100 + specificity;
}

export function resolveMessageRenderer(
  providers: UIProviderDefinition[],
  resolved: UIProviderDefinition | null,
  message: Record<string, any>,
): UIProviderDefinition | null {
  const builtin = providers.find((provider) => provider.enabled && provider.builtin) ?? null;
  if (!resolved || !resolved.enabled) return builtin;
  if (resolved.builtin) return resolved;

  // Installation/enabling is not activation. Message selectors only refine the
  // profile-selected renderer; they must never let an arbitrary enabled plugin
  // take over a message surface without an explicit profile selection.
  return scoreProvider(resolved, message) !== null ? resolved : builtin;
}
