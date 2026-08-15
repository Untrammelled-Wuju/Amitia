package com.amitia.amitia_app.runtime.proot

object GuestLayout {
    const val PROGRAM = "/opt/amitia"
    const val CONFIG = "/etc/amitia"
    const val DATA = "/var/lib/amitia"
    const val CACHE = "/var/cache/amitia"
    const val LOGS = "/var/log/amitia"
    const val RUN = "/run/amitia"
    const val HOME = "/home/amitia"
    const val TMP = "/tmp"

    const val BACKEND_DIR = "$PROGRAM/backend"
    const val BACKEND_SERVER = "$BACKEND_DIR/amitia-server"

    const val NODE_DIR = "$PROGRAM/node"
    const val NODE_BIN = "$NODE_DIR/bin"
    const val NODE_BIN_NODE = "$NODE_BIN/node"
    const val NPM_CLI = "$NODE_DIR/lib/node_modules/npm/bin/npm-cli.js"
    const val NPX_CLI = "$NODE_DIR/lib/node_modules/npm/bin/npx-cli.js"

    const val QDRANT_DIR = "$PROGRAM/qdrant"
    const val QDRANT_BIN = "$QDRANT_DIR/bin/qdrant"

    const val PLUGIN_HOST_DIR = "$PROGRAM/plugin-host"
    const val PLUGIN_HOST_ENTRY = "$PLUGIN_HOST_DIR/dist/index.js"

    const val TASK_HOST_DIR = "$PROGRAM/task-host"
    const val TASK_HOST_ENTRY = "$TASK_HOST_DIR/dist/index.js"

    const val SIDECAR_DIR = "$PROGRAM/sidecar"
    const val SIDECAR_LAUNCHER = "$SIDECAR_DIR/launcher.mjs"
    const val SIDECAR_BUNDLE = "$SIDECAR_DIR/bundle.mjs"

    const val QQ_SIDECAR_DIR = "$PROGRAM/qq-sidecar"
    const val QQ_SIDECAR_LAUNCHER = "$QQ_SIDECAR_DIR/launcher.mjs"
    const val QQ_SIDECAR_BUNDLE = "$QQ_SIDECAR_DIR/bundle.mjs"

    const val SCRIPTS_DIR = "$PROGRAM/scripts"
    const val SCRIPTS_NODE_DIR = "$SCRIPTS_DIR/node"

    const val MANIFEST_DIR = "$PROGRAM/manifest"

    const val LICENSES_DIR = "$PROGRAM/licenses"

    const val SECURITY_DIR = "$DATA/security"
    const val LOCAL_TOKEN = "$SECURITY_DIR/local-token"

    const val PROVIDERS_DIR = "$DATA/providers"
    const val QDRANT_STORAGE = "$PROVIDERS_DIR/qdrant/storage"

    const val CONFIG_PROVIDERS_DIR = "$CONFIG/providers"
    const val QDRANT_CONFIG = "$CONFIG_PROVIDERS_DIR/qdrant/config.yaml"

    const val NPM_CACHE = "$CACHE/npm"

    val PATH = "$NODE_BIN:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

    val PROGRAM_SUBDIRS = listOf(
        "backend", "node", "qdrant", "sidecar", "qq-sidecar",
        "plugin-host", "task-host", "scripts", "manifest", "licenses",
    )

    val ALL_ROOTS = listOf(
        PROGRAM, CONFIG, DATA, CACHE, LOGS, RUN, HOME, TMP,
    )
}
