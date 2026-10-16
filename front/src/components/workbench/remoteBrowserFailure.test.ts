import { describe, expect, it } from 'vitest';
import { classifyRemoteBrowserFailure } from './remoteBrowserFailure';

describe('remote Chromium observation failure handling', () => {
  it('terminates on revoked capabilities and access denial', () => {
    for (const status of [400, 401, 403, 404, 410, 501]) {
      expect(classifyRemoteBrowserFailure({ response: { status, data: { error: 'invalid' } } }).terminal).toBe(true);
    }
    expect(classifyRemoteBrowserFailure({ response: { status: 503, data: { errorCode: 'capability_revoked' } } }).terminal).toBe(true);
    expect(classifyRemoteBrowserFailure({ response: { status: 502, data: { errorCode: 'resource_exceeded', retryable: false } } }).terminal).toBe(true);
  });
  it('retries only transport/runtime failures', () => {
    expect(classifyRemoteBrowserFailure({ response: { status: 503, data: { errorCode: 'device_offline', retryable: true } } }).terminal).toBe(false);
    expect(classifyRemoteBrowserFailure(new Error('network down')).terminal).toBe(false);
  });
  it('keeps server-safe error messages', () => {
    expect(classifyRemoteBrowserFailure({ response: { status: 410, data: { error: '观察已撤销' } } }).message).toBe('观察已撤销');
  });
});
