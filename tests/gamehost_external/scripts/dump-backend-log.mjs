import { existsSync, readFileSync } from 'node:fs';

const log = process.env.GAMEHOST_BACKEND_LOG;
if (!log || !existsSync(log)) {
  process.stdout.write('GameHost backend log is unavailable.\n');
  process.exit(0);
}
const lines = readFileSync(log, 'utf8').split(/\r?\n/);
process.stdout.write(`${lines.slice(-500).join('\n')}\n`);
