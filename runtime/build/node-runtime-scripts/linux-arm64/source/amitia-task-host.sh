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

if [ -z "${AMITIA_TASK_WORKSPACE}" ]; then
  amitia_die 20 "env_AMITIA_TASK_WORKSPACE_missing"
fi
require_absolute_guest_path "${AMITIA_TASK_WORKSPACE}" "AMITIA_TASK_WORKSPACE"

require_executable_file "${AMITIA_NODE_BIN}" "node"
require_regular_file "${AMITIA_TASK_HOST}" "task_host"

if [ ! -d "${AMITIA_TASK_WORKSPACE}" ]; then
  amitia_die 22 "task_workspace_not_dir"
fi

if [ -z "${AMITIA_NODE_HOME}" ] || [ -z "${AMITIA_NODE_TMP}" ] || [ -z "${AMITIA_NPM_CACHE}" ] || [ -z "${AMITIA_NODE_PREFIX}" ]; then
  amitia_die 20 "env_isolated_missing"
fi

_TASK_ENV="HOME=${AMITIA_NODE_HOME}"
_TASK_ENV="${_TASK_ENV} TMPDIR=${AMITIA_NODE_TMP}"
_TASK_ENV="${_TASK_ENV} LANG=C.UTF-8"
_TASK_ENV="${_TASK_ENV} LC_ALL=C.UTF-8"
_TASK_ENV="${_TASK_ENV} TZ=Etc/UTC"
_TASK_ENV="${_TASK_ENV} PATH=${RUNTIME_ROOT}/node/bin:${AMITIA_NODE_PREFIX}/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
_TASK_ENV="${_TASK_ENV} npm_config_cache=${AMITIA_NPM_CACHE}"
_TASK_ENV="${_TASK_ENV} npm_config_prefix=${AMITIA_NODE_PREFIX}"
_TASK_ENV="${_TASK_ENV} npm_config_update_notifier=false"
_TASK_ENV="${_TASK_ENV} npm_config_audit=false"
_TASK_ENV="${_TASK_ENV} npm_config_fund=false"
_TASK_ENV="${_TASK_ENV} npm_config_color=false"
_TASK_ENV="${_TASK_ENV} npm_config_progress=false"
_TASK_ENV="${_TASK_ENV} NO_UPDATE_NOTIFIER=1"
_TASK_ENV="${_TASK_ENV} NODE_DISABLE_COLORS=1"
_TASK_ENV="${_TASK_ENV} AMITIA_TASK_WORKSPACE=${AMITIA_TASK_WORKSPACE}"

if [ -n "${AMITIA_TASK_ID}" ]; then
  _TASK_ENV="${_TASK_ENV} AMITIA_TASK_ID=${AMITIA_TASK_ID}"
fi
if [ -n "${AMITIA_SESSION_ID}" ]; then
  _TASK_ENV="${_TASK_ENV} AMITIA_SESSION_ID=${AMITIA_SESSION_ID}"
fi
if [ -n "${AMITIA_LOG_LEVEL}" ]; then
  _TASK_ENV="${_TASK_ENV} AMITIA_LOG_LEVEL=${AMITIA_LOG_LEVEL}"
fi
if [ -n "${AMITIA_RUNTIME_TOKEN}" ]; then
  _TASK_ENV="${_TASK_ENV} AMITIA_RUNTIME_TOKEN=${AMITIA_RUNTIME_TOKEN}"
fi

cd "${AMITIA_TASK_WORKSPACE}" || amitia_die 41 "task_chdir_failed"

exec env -i ${_TASK_ENV} \
  "${AMITIA_NODE_BIN}" "${AMITIA_TASK_HOST}" "$@"
