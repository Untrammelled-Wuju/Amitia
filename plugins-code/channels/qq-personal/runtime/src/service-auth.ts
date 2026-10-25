import { createHash, timingSafeEqual } from "node:crypto"

export interface ServiceIdentity {
  extensionId: string
  moduleId: string
  token: string
}

function digest(value: string): Buffer {
  return createHash("sha256").update(value).digest()
}

function equalSecret(actual: string, expected: string): boolean {
  if (!actual || !expected) return false
  return timingSafeEqual(digest(actual), digest(expected))
}

function headerValue(headers: Record<string, unknown>, name: string): string {
  const value = headers[name.toLowerCase()]
  if (Array.isArray(value)) return String(value[0] || "").trim()
  return String(value || "").trim()
}

export function authorizeServiceRequest(
  headers: Record<string, unknown>,
  identity: ServiceIdentity,
): boolean {
  const authorization = headerValue(headers, "authorization")
  const match = authorization.match(/^Bearer\s+(.+)$/i)
  if (!match) return false
  if (headerValue(headers, "x-amitia-extension-id") !== identity.extensionId) return false
  if (headerValue(headers, "x-amitia-module-id") !== identity.moduleId) return false
  return equalSecret(match[1].trim(), identity.token)
}
