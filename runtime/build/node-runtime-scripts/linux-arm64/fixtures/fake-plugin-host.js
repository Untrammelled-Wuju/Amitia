const fs = require('node:fs');
const path = require('node:path');

const logDir = process.env.AMITIA_FAKE_NODE_DIR || '/tmp/fake-node-logs';
fs.mkdirSync(logDir, { recursive: true });
fs.writeFileSync(path.join(logDir, 'plugin-host-args'), process.argv.slice(2).join('\n'));
fs.writeFileSync(path.join(logDir, 'plugin-host-env'), JSON.stringify(process.env, null, 2));
fs.writeFileSync(path.join(logDir, 'plugin-host-cwd'), process.cwd());

process.stdout.write('plugin-host-ok\n');
process.exit(0);
