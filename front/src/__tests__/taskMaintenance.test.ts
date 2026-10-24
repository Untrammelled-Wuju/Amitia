import { beforeEach, describe, expect, it, vi } from 'vitest';

const api = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn(), post: vi.fn() }));
vi.mock('../composables/useApi', () => ({ useApi: () => api }));
import { fetchConfigApi, fetchStatusApi, saveConfigApi } from '../views/long-running/api';

const config = () => ({ source: 'task-runtime' as const, runtimeProfile: 'cloud-core', maxTasks: 4, timeout: { disabled: false, seconds: 180 }, revision: 1, timeoutRevision: 2, applied: true });

describe('task maintenance authority', () => {
  beforeEach(() => vi.resetAllMocks());
  it('reads real runtime state and pins the cloud configuration scope', async () => {
    api.get.mockResolvedValue({ source: 'task-runtime', tasks: [], activeCount: 0 });
    const context = JSON.stringify(['https://core.example', 'core-id', 1, 2, 3]);
    await fetchStatusApi(context);
    expect(api.get).toHaveBeenCalledWith('/api/runtime/tasks/status', undefined, { headers: { 'X-Amitia-Expected-Core-ID': 'core-id', 'X-Amitia-Expected-Configuration-Policy': '1:2:3' } });
    api.get.mockResolvedValue({ running: false, tasks: [] });
    await expect(fetchStatusApi('local')).rejects.toThrow();
  });
  it('rejects missing executor acknowledgement and detects partial saves', async () => {
    api.get.mockResolvedValue({ ...config(), applied: false });
    await expect(fetchConfigApi('local')).rejects.toThrow('未确认');
    api.put.mockResolvedValue(config());
    await expect(saveConfigApi(config(), 'local')).rejects.toThrow('不一致');
    api.put.mockResolvedValue({ ...config(), revision: 2, timeoutRevision: 3 });
    await expect(saveConfigApi(config(), 'local')).resolves.toMatchObject({ revision: 2, applied: true });
    expect(api.put.mock.lastCall?.[1]).toEqual({ maxTasks: 4, timeout: { disabled: false, seconds: 180 }, expectedRevision: 1, expectedTimeoutRevision: 2 });
  });
});
