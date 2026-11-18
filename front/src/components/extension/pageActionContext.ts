export function extensionPageActionContext(context: Record<string, unknown>): Record<string, unknown> {
  const { sessionId: _sessionId, ...actionContext } = context;
  return actionContext;
}

export function extensionPageActionResult(response: Record<string, unknown> | undefined): unknown {
  const result = response?.result && typeof response.result === "object"
    ? response.result as Record<string, unknown>
    : undefined;
  if (result?.error) {
    const error = result.error;
    const message = typeof error === "string"
      ? error
      : String((error as Record<string, unknown> | undefined)?.message ?? JSON.stringify(error));
    throw new Error(message);
  }
  return result?.structured ?? result ?? response;
}
