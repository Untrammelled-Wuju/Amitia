const SESSION_KEY = "ai-companion-session-id"
const USER_KEY = "ai-companion-user-id"

function createId(prefix: string): string {
  const id = typeof crypto !== "undefined" && typeof crypto.randomUUID === "function"
    ? crypto.randomUUID()
    : `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`
  return `${prefix}-${id}`
}

function getStoredId(key: string, prefix: string): string {
  if (typeof window === "undefined") return createId(prefix)
  const existing = localStorage.getItem(key)
  if (existing) return existing
  const next = createId(prefix)
  localStorage.setItem(key, next)
  return next
}

export function createRequestEnvelope() {
  return {
    requestId: createId("req"),
    sessionId: getStoredId(SESSION_KEY, "sess"),
    userId: getStoredId(USER_KEY, "user"),
  }
}
