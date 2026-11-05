import { describe, expect, it } from 'vitest';
import { isSuccessfulResponseCode } from '@/composables/request';

describe('API response envelope success codes', () => {
  it.each([200, 201, 202, 204, 299])('accepts %i as success', (code) => {
    expect(isSuccessfulResponseCode(code)).toBe(true);
  });

  it.each([0, 199, 300, 400, 500])('rejects %i as failure', (code) => {
    expect(isSuccessfulResponseCode(code)).toBe(false);
  });
});
