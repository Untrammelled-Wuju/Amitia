import * as path from 'path';
import { PseudoHost } from '../harness';

const PLUGIN_PATH = path.resolve(__dirname, '../../../../typescript/dist/index.js');

async function runCrash(exitCode: number, delayMs: number): Promise<number> {
  const host = new PseudoHost(PLUGIN_PATH);
  await host.start();

  const crashPromise = host.callRPC('mock.fault', 'mock.fault.crash', { exitCode, delayMs }, 3000);
  await host.waitExit();

  await crashPromise.catch(() => undefined);

  const code = host.getExitCode();
  return code === null ? -1 : code;
}

describe('ProcessCrash', () => {
  it('CRASH-001: crash with exitCode 1 and no delay exits with code 1', async () => {
    const code = await runCrash(1, 0);
    expect(code).toBe(1);
  });

  it('CRASH-002: crash with exitCode 42 exits with code 42', async () => {
    const code = await runCrash(42, 0);
    expect(code).toBe(42);
  });

  it('CRASH-003: crash with exitCode 0 defaults to exitCode 1', async () => {
    const code = await runCrash(0, 0);
    expect(code).toBe(1);
  });

  it('CRASH-004: crash with delayMs 100 and exitCode 7 exits with code 7', async () => {
    const code = await runCrash(7, 100);
    expect(code).toBe(7);
  });

  it('CRASH-005: crash with exitCode 255 exits with code 255', async () => {
    const code = await runCrash(255, 0);
    expect(code).toBe(255);
  });

  it('CRASH-006: crash with delayMs 50 and exitCode 3 exits with code 3', async () => {
    const code = await runCrash(3, 50);
    expect(code).toBe(3);
  });
});
