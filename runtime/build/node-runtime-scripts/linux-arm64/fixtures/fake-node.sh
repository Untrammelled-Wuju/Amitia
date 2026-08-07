#!/bin/sh
AMITIA_FAKE_NODE_DIR="${AMITIA_FAKE_NODE_DIR:-/tmp/fake-node-logs}"
mkdir -p "${AMITIA_FAKE_NODE_DIR}"
echo "$@" > "${AMITIA_FAKE_NODE_DIR}/last-args"
env > "${AMITIA_FAKE_NODE_DIR}/last-env"
echo "v24.19.0"
exit 0
