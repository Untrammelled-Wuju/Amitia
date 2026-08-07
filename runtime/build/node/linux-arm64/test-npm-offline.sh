#!/bin/bash
set -e

mkdir -p /tmp/test-home /tmp/npm-cache /tmp/npm-prefix /tmp/offline-test
cd /tmp/offline-test

echo '{"name": "offline-test", "version": "1.0.0"}' > package.json

/amitia/node/bin/node /amitia/node/lib/node_modules/npm/bin/npm-cli.js install /testpkg \
  --offline \
  --ignore-scripts \
  --no-audit \
  --no-fund

echo "[npm 离线安装完成]"

/amitia/node/bin/node -e "const m = require('/tmp/offline-test/node_modules/test-local-package'); console.log('[PASS] npm 离线安装验证:', m.greet('Amitia'));"

rm -rf /tmp/offline-test
