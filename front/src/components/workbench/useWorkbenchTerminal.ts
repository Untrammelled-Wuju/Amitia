import { apiClient } from '@/composables/useApi';
import { parseTerminalLifecyclePage } from './terminalLifecycle';

export interface TerminalSnapshot {
  id: string;
  taskId?: string;
  workspaceUri?: string;
  mode: 'pty' | 'pipe' | 'conpty';
  running: boolean;
  exitCode: number;
  data: string;
  cursor: number;
  fromCursor?: number;
  truncated: boolean;
}

export interface TerminalSessionInfo {
  id: string;
  taskId?: string;
  workspaceUri?: string;
  mode: 'pty' | 'pipe' | 'conpty';
  running: boolean;
  exitCode: number;
  createdAt: string;
}

export interface TerminalTranscript {
  id: string;
  taskId?: string;
  workspaceUri?: string;
  mode: string;
  running: boolean;
  exitCode: number;
  data: string;
  baseCursor: number;
  endCursor: number;
  truncated: boolean;
  capturedAt: string;
  startedAt: string;
  outputBytes: number;
  outputDigest: string;
  digestScope: 'retained-bytes-only';
  lifecycle: TerminalLifecycleEvent[];
  lifecycleLatestSequence: number;
  lifecycleTruncated: boolean;
}

export interface TerminalLifecycleEvent {
  sequence: number;
  kind: 'session.started' | 'session.resized' | 'process.exited';
  at: string;
  exitCode?: number;
  rows?: number;
  cols?: number;
}

export interface TerminalLifecyclePage {
  sessionId: string;
  taskId?: string;
  workspaceUri?: string;
  latestSequence: number;
  truncated: boolean;
  events: TerminalLifecycleEvent[];
  scope: 'live-session-memory-only';
}

export interface TerminalTaskReceipt {
  sessionId: string;
  taskId: string;
  conversationId: string;
  workspaceUri: string;
  mode: string;
  startedAt: string;
  updatedAt: string;
  state: 'running' | 'exited' | 'closed' | 'revoked' | 'interrupted';
  exitCode?: number;
  latestSequence: number;
  evidenceKind: 'terminal-lifecycle-metadata-not-test-proof';
}

export interface TerminalTaskReceiptSummary {
  total: number;
  running: number;
  exited: number;
  interrupted: number;
  revoked: number;
  closed: number;
  latestState: 'none' | TerminalTaskReceipt['state'];
  latestSessionId?: string;
  lastExitCode?: number;
  verificationKind: 'lifecycle-only-not-test-proof';
}

export interface TerminalMount {
  id: string;
  name: string;
  rootUri: string;
  available: boolean;
  readOnly: boolean;
  kind: string;
}

export function encodeInput(value: string): string {
  const bytes = new TextEncoder().encode(value);
  let binary = '';
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary);
}

export function decodeOutput(encoded: string): Uint8Array {
  const binary = atob(encoded);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  return bytes;
}

export const terminalApi = {
  async taskReceiptDiagnostics(conversationId: string, taskId: string): Promise<{ receipts: TerminalTaskReceipt[]; summary: TerminalTaskReceiptSummary }> {
    if (!/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(taskId)) throw new Error('任务标识格式错误');
    const result = await apiClient.get('/api/workbench/terminal/task-receipts', { params: { conversationId, taskId } });
    if (result.data?.taskId !== taskId || result.data?.conversationId !== conversationId || !Array.isArray(result.data?.receipts) || result.data.receipts.length > 32) throw new Error('任务终端凭据响应无效');
    const receipts = result.data.receipts.filter((receipt: TerminalTaskReceipt) =>
      /^[a-f0-9]{40}$/.test(receipt.sessionId) && receipt.taskId === taskId && receipt.conversationId === conversationId &&
      ['running','exited','closed','revoked','interrupted'].includes(receipt.state) &&
      receipt.evidenceKind === 'terminal-lifecycle-metadata-not-test-proof');
    const summary = result.data.summary as TerminalTaskReceiptSummary | undefined;
    if (!summary || summary.verificationKind !== 'lifecycle-only-not-test-proof' ||
        summary.total !== receipts.length || !Number.isSafeInteger(summary.total) ||
        !['none','running','exited','closed','revoked','interrupted'].includes(summary.latestState) ||
        ['running','exited','interrupted','revoked','closed'].some(key => !Number.isSafeInteger(summary[key as keyof Pick<TerminalTaskReceiptSummary, 'running'|'exited'|'interrupted'|'revoked'|'closed'>])) ||
        summary.running + summary.exited + summary.interrupted + summary.revoked + summary.closed !== summary.total ||
        (receipts.length > 0 && (summary.latestState !== receipts[0].state || summary.latestSessionId !== receipts[0].sessionId))) {
      throw new Error('任务终端诊断摘要与实际记录不一致');
    }
    return { receipts, summary };
  },
  async taskReceipts(conversationId: string, taskId: string): Promise<TerminalTaskReceipt[]> {
    return (await terminalApi.taskReceiptDiagnostics(conversationId, taskId)).receipts;
  },
  async listSessions(conversationId: string): Promise<TerminalSessionInfo[]> {
    const result = await apiClient.get('/api/workbench/terminal/sessions', { params: { conversationId } });
    if (!Array.isArray(result.data)) return [];
    return result.data.filter((item: TerminalSessionInfo) =>
      /^[a-f0-9]{40}$/.test(item.id) && ['pty', 'pipe', 'conpty'].includes(item.mode));
  },
  async listMounts(): Promise<TerminalMount[]> {
    const result = await apiClient.get('/api/workspaces');
    return (Array.isArray(result.data) ? result.data : []).filter((mount: TerminalMount) => mount.kind === 'local' && mount.available && !mount.readOnly);
  },
  async open(conversationId: string, workspaceUri: string, taskId = ""): Promise<TerminalSnapshot> {
    const result = await apiClient.post('/api/workbench/terminal/open', { conversationId, workspaceUri, taskId, rows: 30, cols: 100 });
    return result.data as TerminalSnapshot;
  },
  async lifecycle(id: string, conversationId: string, after: number): Promise<TerminalLifecyclePage> {
    if (!Number.isSafeInteger(after) || after < 0) throw new Error('无效的终端事件游标');
    const result = await apiClient.get(`/api/workbench/terminal/${encodeURIComponent(id)}/lifecycle`, { params: { conversationId, after } });
    return parseTerminalLifecyclePage(result.data, id);
  },
  async exportTranscript(id: string, conversationId: string): Promise<TerminalTranscript> {
    const result = await apiClient.get(`/api/workbench/terminal/${encodeURIComponent(id)}/export`, { params: { conversationId } });
    return result.data as TerminalTranscript;
  },
  async read(id: string, conversationId: string, cursor: number): Promise<TerminalSnapshot> {
    const result = await apiClient.get(`/api/workbench/terminal/${encodeURIComponent(id)}`, { params: { conversationId, cursor } });
    return result.data as TerminalSnapshot;
  },
  async input(id: string, conversationId: string, value: string): Promise<void> {
    await apiClient.post(`/api/workbench/terminal/${encodeURIComponent(id)}/input`, { conversationId, data: encodeInput(value) });
  },
  async resize(id: string, conversationId: string, rows: number, cols: number): Promise<void> {
    await apiClient.post(`/api/workbench/terminal/${encodeURIComponent(id)}/resize`, { conversationId, rows, cols });
  },
  async close(id: string, conversationId: string): Promise<void> {
    await apiClient.delete(`/api/workbench/terminal/${encodeURIComponent(id)}`, { params: { conversationId } });
  },
};
