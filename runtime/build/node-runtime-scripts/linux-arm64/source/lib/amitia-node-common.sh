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

AMITIA_NODE_BIN="${RUNTIME_ROOT}/node/bin/node"
AMITIA_NPM_CLI="${RUNTIME_ROOT}/node/lib/node_modules/npm/bin/npm-cli.js"
AMITIA_NPX_CLI="${RUNTIME_ROOT}/node/lib/node_modules/npm/bin/npx-cli.js"
AMITIA_PLUGIN_HOST="${RUNTIME_ROOT}/plugin-host/dist/index.js"
AMITIA_TASK_HOST="${RUNTIME_ROOT}/task-host/dist/index.js"

AMITIA_ERR_USAGE=2
AMITIA_ERR_LAYOUT=10
AMITIA_ERR_NODE_MISSING=11
AMITIA_ERR_NPM_MISSING=12
AMITIA_ERR_NPX_MISSING=13
AMITIA_ERR_PLUGIN_MISSING=14
AMITIA_ERR_TASK_MISSING=15
AMITIA_ERR_ENV_MISSING=20
AMITIA_ERR_ENV_INVALID=21
AMITIA_ERR_ROOT_MISSING=22
AMITIA_ERR_ROOT_CREATE=23
AMITIA_ERR_NODE_VERSION=30
AMITIA_ERR_NPM_VERSION=31
AMITIA_ERR_PLATFORM=32
AMITIA_ERR_ARCH=33
AMITIA_ERR_PLUGIN_START=40
AMITIA_ERR_TASK_START=41
AMITIA_ERR_INTERNAL=50

amitia_die() {
  printf 'AMITIA_NODE_ERROR code=%s key=%s\n' "$1" "$2" >&2
  exit "$1"
}

require_absolute_guest_path() {
  _val="$1"
  _name="$2"
  if [ -z "${_val}" ]; then
    amitia_die "${AMITIA_ERR_ENV_MISSING}" "env_${_name}_empty"
  fi
  case "${_val}" in
    /*) ;;
    *) amitia_die "${AMITIA_ERR_ENV_INVALID}" "env_${_name}_not_absolute" ;;
  esac
  case "${_val}" in
    *\\*) amitia_die "${AMITIA_ERR_ENV_INVALID}" "env_${_name}_contains_backslash" ;;
  esac
  case "${_val}" in
    *:*) amitia_die "${AMITIA_ERR_ENV_INVALID}" "env_${_name}_contains_drive" ;;
  esac
  case "${_val}" in
    */../*|*/..) amitia_die "${AMITIA_ERR_ENV_INVALID}" "env_${_name}_contains_dotdot" ;;
  esac
  case "${_val}" in
    *[[:space:]]) amitia_die "${AMITIA_ERR_ENV_INVALID}" "env_${_name}_trailing_space" ;;
  esac
}

require_regular_file() {
  _path="$1"
  _name="$2"
  if [ ! -e "${_path}" ]; then
    amitia_die "${AMITIA_ERR_LAYOUT}" "file_${_name}_missing"
  fi
  if [ ! -f "${_path}" ]; then
    amitia_die "${AMITIA_ERR_LAYOUT}" "file_${_name}_not_regular"
  fi
  if [ ! -s "${_path}" ]; then
    amitia_die "${AMITIA_ERR_LAYOUT}" "file_${_name}_empty"
  fi
}

require_executable_file() {
  require_regular_file "$1" "$2"
  if [ ! -x "$1" ]; then
    amitia_die "${AMITIA_ERR_LAYOUT}" "file_${_name}_not_executable"
  fi
}

require_env_var() {
  _nam="$2"
  _val=$(printenv "$1")
  if [ -z "${_val}" ]; then
    amitia_die "${AMITIA_ERR_ENV_MISSING}" "env_${_nam}_missing"
  fi
  require_absolute_guest_path "${_val}" "${_nam}"
}

amitia_build_isolated_env() {
  _node_home="${AMITIA_NODE_HOME}"
  _node_tmp="${AMITIA_NODE_TMP}"
  _npm_cache="${AMITIA_NPM_CACHE}"
  _npm_prefix="${AMITIA_NODE_PREFIX}"
  _runtime_root="${RUNTIME_ROOT}"

  printf 'HOME=%s\n' "${_node_home}"
  printf 'TMPDIR=%s\n' "${_node_tmp}"
  printf 'LANG=C.UTF-8\n'
  printf 'LC_ALL=C.UTF-8\n'
  printf 'TZ=Etc/UTC\n'
  printf 'PATH=%s/node/bin:%s/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n' "${_runtime_root}" "${_npm_prefix}"
  printf 'npm_config_cache=%s\n' "${_npm_cache}"
  printf 'npm_config_prefix=%s\n' "${_npm_prefix}"
  printf 'npm_config_update_notifier=false\n'
  printf 'npm_config_audit=false\n'
  printf 'npm_config_fund=false\n'
  printf 'npm_config_color=false\n'
  printf 'npm_config_progress=false\n'
  printf 'NO_UPDATE_NOTIFIER=1\n'
  printf 'NODE_DISABLE_COLORS=1\n'
}
