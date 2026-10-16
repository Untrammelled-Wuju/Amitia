export type RemoteBrowserFailure = { terminal: boolean; message: string };

const finalCodes = new Set(['capability_revoked', 'access_denied', 'invalid_request', 'unsupported_operation']);
const finalStatuses = new Set([400, 401, 403, 404, 410, 501]);

export function classifyRemoteBrowserFailure(cause: unknown): RemoteBrowserFailure {
  const error = cause as { response?: { status?: number; data?: { error?: unknown; errorCode?: unknown; retryable?: unknown } }; message?: string } | null;
  const status = error?.response?.status;
  const payload = error?.response?.data;
  const code = typeof payload?.errorCode === 'string' ? payload.errorCode : '';
  const terminal = (typeof status === 'number' && finalStatuses.has(status)) || finalCodes.has(code) || (payload?.retryable === false && code !== '');
  const message = typeof payload?.error === 'string' && payload.error ? payload.error : error?.message || '远程浏览器不可用';
  return { terminal, message };
}
