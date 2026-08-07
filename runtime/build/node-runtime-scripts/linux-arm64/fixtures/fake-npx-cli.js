const fs = require('node:fs');
const path = require('node:path');

const logDir = process.env.AMITIA_FAKE_NODE_DIR || '/tmp/fake-node-logs';
fs.mkdirSync(logDir, { recursive: true });
fs.writeFileSync(path.join(logDir, 'npx-args'), process.argv.slice(2).join('\n'));
fs.writeFileSync(path.join(logDir, 'npx-env'), JSON.stringify(process.env, null, 2));

const arg = process.argv[2];
if (arg === '--version') {
  process.stdout.write('11.17.0\n');
  process.exit(0);
}

process.stdout.write('mock-npx-ok\n');
process.exit(0);
