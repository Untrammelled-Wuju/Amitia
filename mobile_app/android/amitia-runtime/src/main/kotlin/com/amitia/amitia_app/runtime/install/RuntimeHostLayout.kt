package com.amitia.amitia_app.runtime.install

import java.io.File

interface RuntimeHostLayout {
    val controlRoot: File

    val rootfsRoot: File
    val versionsRoot: File
    val stagingRoot: File
    val metadataRoot: File
    val transactionsRoot: File
    val locksRoot: File
    val packagesRoot: File

    val configRoot: File
    val dataRoot: File
    val cacheRoot: File
    val logRoot: File
    val runRoot: File
    val homeRoot: File

    fun runtimeVersionRoot(
        version: String,
    ): File

    fun installReceiptFile(
        version: String,
    ): File

    val activeRuntimeFile: File
    val runtimeManifestFile: File
    val runtimeManifestShaFile: File

    companion object {
        const val CONTROL_DIR_NAME = "amitia-runtime"
        const val DATA_DIR_NAME = "amitia"

        const val DIR_ROOTFS = "rootfs"
        const val DIR_VERSIONS = "versions"
        const val DIR_STAGING = "staging"
        const val DIR_METADATA = "metadata"
        const val DIR_TRANSACTIONS = "transactions"
        const val DIR_LOCKS = "locks"
        const val DIR_PACKAGES = "packages"

        const val DIR_CONFIG = "config"
        const val DIR_DATA = "data"
        const val DIR_CACHE = "cache"
        const val DIR_LOGS = "logs"
        const val DIR_RUN = "run"
        const val DIR_HOME = "home"

        const val DIR_INSTALL_RECEIPTS = "install-receipts"

        const val FILE_INSTALL_LOCK = "install.lock"
        const val FILE_ACTIVE_RUNTIME = "active-runtime.json"
        const val FILE_RUNTIME_MANIFEST = "runtime-manifest.json"
        const val FILE_RUNTIME_MANIFEST_SHA = "runtime-manifest.json.sha256"

        const val SECURITY_DIR = "security"
        const val LOCAL_TOKEN_FILE = "local-token"

        const val QDRANT_DIR = "providers/qdrant"
        const val NODE_DIR = "node"
        const val EXTENSIONS_DIR = "extensions"
        const val TASKS_DIR = "tasks"
        const val WORKSPACES_DIR = "workspaces"
    }
}
