import { readFileSync } from 'fs';
import { join } from 'path';
import {
  PERM_GAMEHOST_CONTROL,
  PERM_GAMEHOST_CHANNEL_USE,
  PERM_GAMEHOST_HOST_API_INVOKE,
  PERM_GAMEHOST_ARTIFACT_DEPLOY,
} from './permission';

describe('GameHost Permission Constants', () => {
  it('should have exactly four canonical GameHost permissions', () => {
    const perms = [PERM_GAMEHOST_CONTROL, PERM_GAMEHOST_CHANNEL_USE, PERM_GAMEHOST_HOST_API_INVOKE, PERM_GAMEHOST_ARTIFACT_DEPLOY];
    const unique = new Set(perms);
    expect(unique.size).toBe(4);
  });

  it('should use canonical permission ID values', () => {
    expect(PERM_GAMEHOST_CONTROL).toBe('gamehost.control');
    expect(PERM_GAMEHOST_CHANNEL_USE).toBe('gamehost.channel.use');
    expect(PERM_GAMEHOST_HOST_API_INVOKE).toBe('gamehost.host_api.invoke');
    expect(PERM_GAMEHOST_ARTIFACT_DEPLOY).toBe('gamehost.artifact.deploy');
  });

  it('should match the golden contract fixture', () => {
    const fixturePath = join(__dirname, '../../../conformance/fixtures/permission_contract.json');
    const raw = readFileSync(fixturePath, 'utf-8');
    const fixture = JSON.parse(raw);
    const actual = [PERM_GAMEHOST_CONTROL, PERM_GAMEHOST_CHANNEL_USE, PERM_GAMEHOST_HOST_API_INVOKE, PERM_GAMEHOST_ARTIFACT_DEPLOY];
    expect(actual).toEqual(fixture.gameHostPermissions);
  });
});
