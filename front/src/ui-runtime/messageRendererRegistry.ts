import type { UIProviderDefinition, UIProviderResolveContext } from "./types";
import { resolveHostEnvironment } from "@/composables/useHostEnvironment";
import { isProviderCompatible } from "./providerRuntime";
import { providerHasMessageSelectors } from "./providerCollection";

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

/** Selector-specific renderers from all enabled plugins compete; selector-less renderers remain profile-global. */
export function resolveMessageRenderer(
  providers: UIProviderDefinition[],
  resolved: UIProviderDefinition | null,
  message: Record<string, any>,
  context?: UIProviderResolveContext,
): UIProviderDefinition | null {
  const platform = resolveHostEnvironment().platform;
  const builtin = providers.find((provider) => provider.enabled && provider.builtin && isProviderCompatible(provider, context, platform)) ?? null;
  const candidates = providers
    .filter((provider) => !provider.builtin && providerHasMessageSelectors(provider) && isProviderCompatible(provider, context, platform))
    .map((provider) => ({ provider, score: scoreProvider(provider, message) }))
    .filter((item): item is { provider: UIProviderDefinition; score: number } => item.score !== null)
    .sort((a, b) => b.score - a.score || a.provider.providerId.localeCompare(b.provider.providerId));
  if (candidates.length > 0) return candidates[0].provider;

  if (resolved?.enabled && resolved.capability === "conversation.message_renderer" && isProviderCompatible(resolved, context, platform)) {
    if (resolved.builtin) return resolved;
    if (!providerHasMessageSelectors(resolved) && scoreProvider(resolved, message) !== null) return resolved;
  }
  return builtin;
}
