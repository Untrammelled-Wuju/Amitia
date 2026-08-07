#!/bin/sh
CDPATH=
SCRIPT_PATH="${0}"
case "${SCRIPT_PATH}" in
  /*) ;;
  *) SCRIPT_PATH="${PWD}/${SCRIPT_PATH}" ;;
esac
SCRIPT_DIR=$(cd "$(dirname "${SCRIPT_PATH}")" && pwd)
RUNTIME_ROOT=$(cd "${SCRIPT_DIR}/../.." && pwd)
AMITIA_NODE_LIB="${SCRIPT_DIR}/lib/amitia-node-common"
if [ ! -f "${AMITIA_NODE_LIB}.sh" ]; then
  printf 'AMITIA_NODE_ERROR code=50 key=common_lib_not_found\n' >&2
  exit 50
fi
. "${AMITIA_NODE_LIB}.sh"

require_executable_file "${AMITIA_NODE_BIN}" "node"
PROBE_JS="${SCRIPT_DIR}/probe-node-runtime.mjs"
require_regular_file "${PROBE_JS}" "probe_js"

if [ -z "${AMITIA_NODE_HOME}" ] || [ -z "${AMITIA_NODE_TMP}" ] || [ -z "${AMITIA_NPM_CACHE}" ] || [ -z "${AMITIA_NODE_PREFIX}" ]; then
  amitia_die 20 "env_isolated_missing"
fi

env -i \
  HOME="${AMITIA_NODE_HOME}" \
  TMPDIR="${AMITIA_NODE_TMP}" \
  LANG=C.UTF-8 \
  LC_ALL=C.UTF-8 \
  TZ=Etc/UTC \
  PATH="${RUNTIME_ROOT}/node/bin:${AMITIA_NODE_PREFIX}/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin" \
  npm_config_cache="${AMITIA_NPM_CACHE}" \
  npm_config_prefix="${AMITIA_NODE_PREFIX}" \
  npm_config_update_notifier=false \
  npm_config_audit=false \
  npm_config_fund=false \
  npm_config_color=false \
  npm_config_progress=false \
  NO_UPDATE_NOTIFIER=1 \
  NODE_DISABLE_COLORS=1 \
  "${AMITIA_NODE_BIN}" "${PROBE_JS}"
exit $?
