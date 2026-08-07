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

if [ -z "${AMITIA_DATA_ROOT}" ]; then
  amitia_die 20 "env_AMITIA_DATA_ROOT_missing"
fi
if [ -z "${AMITIA_CACHE_ROOT}" ]; then
  amitia_die 20 "env_AMITIA_CACHE_ROOT_missing"
fi
if [ -z "${AMITIA_TEMP_ROOT}" ]; then
  amitia_die 20 "env_AMITIA_TEMP_ROOT_missing"
fi

require_absolute_guest_path "${AMITIA_DATA_ROOT}" "AMITIA_DATA_ROOT"
require_absolute_guest_path "${AMITIA_CACHE_ROOT}" "AMITIA_CACHE_ROOT"
require_absolute_guest_path "${AMITIA_TEMP_ROOT}" "AMITIA_TEMP_ROOT"

if [ ! -e "${AMITIA_DATA_ROOT}" ]; then
  amitia_die 22 "data_root_missing"
fi
if [ ! -d "${AMITIA_DATA_ROOT}" ]; then
  amitia_die 22 "data_root_not_dir"
fi
if [ ! -e "${AMITIA_CACHE_ROOT}" ]; then
  amitia_die 22 "cache_root_missing"
fi
if [ ! -d "${AMITIA_CACHE_ROOT}" ]; then
  amitia_die 22 "cache_root_not_dir"
fi
if [ ! -e "${AMITIA_TEMP_ROOT}" ]; then
  amitia_die 22 "temp_root_missing"
fi
if [ ! -d "${AMITIA_TEMP_ROOT}" ]; then
  amitia_die 22 "temp_root_not_dir"
fi

AMITIA_NODE_HOME="${AMITIA_DATA_ROOT}/node/home"
AMITIA_NODE_PREFIX="${AMITIA_DATA_ROOT}/node/prefix"
AMITIA_NPM_CACHE="${AMITIA_CACHE_ROOT}/node/npm"
AMITIA_NODE_TMP="${AMITIA_TEMP_ROOT}/node"

amitia_mkdir_safe() {
  _dir="$1"
  if [ -e "${_dir}" ]; then
    if [ ! -d "${_dir}" ]; then
      amitia_die 23 "path_conflict_file"
    fi
    return 0
  fi
  mkdir -p "${_dir}"
  chmod 0700 "${_dir}"
}

umask 077
amitia_mkdir_safe "${AMITIA_DATA_ROOT}/node"
amitia_mkdir_safe "${AMITIA_NODE_HOME}"
amitia_mkdir_safe "${AMITIA_NODE_PREFIX}"
amitia_mkdir_safe "${AMITIA_CACHE_ROOT}/node"
amitia_mkdir_safe "${AMITIA_NPM_CACHE}"
amitia_mkdir_safe "${AMITIA_NODE_TMP}"

printf 'AMITIA_NODE_HOME=%s\n' "${AMITIA_NODE_HOME}"
printf 'AMITIA_NODE_PREFIX=%s\n' "${AMITIA_NODE_PREFIX}"
printf 'AMITIA_NPM_CACHE=%s\n' "${AMITIA_NPM_CACHE}"
printf 'AMITIA_NODE_TMP=%s\n' "${AMITIA_NODE_TMP}"
exit 0
