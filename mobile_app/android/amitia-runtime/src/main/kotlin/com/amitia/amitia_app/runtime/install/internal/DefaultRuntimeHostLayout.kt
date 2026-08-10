package com.amitia.amitia_app.runtime.install.internal

import com.amitia.amitia_app.runtime.install.RuntimeHostLayout
import java.io.File

internal class DefaultRuntimeHostLayout(
    private val controlBaseDir: File,
    private val dataBaseDir: File,
) : RuntimeHostLayout {

    override val controlRoot: File = File(controlBaseDir, RuntimeHostLayout.CONTROL_DIR_NAME)

    override val rootfsRoot: File = File(controlRoot, RuntimeHostLayout.DIR_ROOTFS)
    override val versionsRoot: File = File(controlRoot, RuntimeHostLayout.DIR_VERSIONS)
    override val stagingRoot: File = File(controlRoot, RuntimeHostLayout.DIR_STAGING)
    override val metadataRoot: File = File(controlRoot, RuntimeHostLayout.DIR_METADATA)
    override val transactionsRoot: File = File(controlRoot, RuntimeHostLayout.DIR_TRANSACTIONS)
    override val locksRoot: File = File(controlRoot, RuntimeHostLayout.DIR_LOCKS)

    private val dataDir = File(dataBaseDir, RuntimeHostLayout.DATA_DIR_NAME)
    override val configRoot: File = File(dataDir, RuntimeHostLayout.DIR_CONFIG)
    override val dataRoot: File = File(dataDir, RuntimeHostLayout.DIR_DATA)
    override val cacheRoot: File = File(dataDir, RuntimeHostLayout.DIR_CACHE)
    override val logRoot: File = File(dataDir, RuntimeHostLayout.DIR_LOGS)
    override val runRoot: File = File(dataDir, RuntimeHostLayout.DIR_RUN)
    override val homeRoot: File = File(dataDir, RuntimeHostLayout.DIR_HOME)

    private val installReceiptsRoot: File = File(metadataRoot, RuntimeHostLayout.DIR_INSTALL_RECEIPTS)

    init {
        require(controlBaseDir.absolutePath.isNotBlank()) {
            "controlBaseDir path must not be blank"
        }
        require(dataBaseDir.absolutePath.isNotBlank()) {
            "dataBaseDir path must not be blank"
        }
    }

    override fun runtimeVersionRoot(version: String): File {
        validateVersion(version)
        return File(versionsRoot, version)
    }

    override fun installReceiptFile(version: String): File {
        validateVersion(version)
        return File(installReceiptsRoot, "$version.json")
    }

    override val activeRuntimeFile: File
        get() = File(metadataRoot, RuntimeHostLayout.FILE_ACTIVE_RUNTIME)

    override val runtimeManifestFile: File
        get() = File(metadataRoot, RuntimeHostLayout.FILE_RUNTIME_MANIFEST)

    override val runtimeManifestShaFile: File
        get() = File(metadataRoot, RuntimeHostLayout.FILE_RUNTIME_MANIFEST_SHA)

    fun securityDir(): File = File(dataRoot, RuntimeHostLayout.SECURITY_DIR)

    fun localTokenFile(): File = File(securityDir(), RuntimeHostLayout.LOCAL_TOKEN_FILE)

    fun qdrantDataDir(): File = File(dataRoot, RuntimeHostLayout.QDRANT_DIR)

    fun nodeDataDir(): File = File(dataRoot, RuntimeHostLayout.NODE_DIR)

    fun nodeCacheDir(): File = File(cacheRoot, RuntimeHostLayout.NODE_DIR)

    fun extensionsDir(): File = File(dataRoot, RuntimeHostLayout.EXTENSIONS_DIR)

    fun tasksDir(): File = File(dataRoot, RuntimeHostLayout.TASKS_DIR)

    fun workspacesDir(): File = File(dataRoot, RuntimeHostLayout.WORKSPACES_DIR)

    fun allControlRoots(): List<File> = listOf(
        rootfsRoot,
        versionsRoot,
        stagingRoot,
        metadataRoot,
        transactionsRoot,
        locksRoot,
    )

    fun allDataRoots(): List<File> = listOf(
        configRoot,
        dataRoot,
        cacheRoot,
        logRoot,
        runRoot,
        homeRoot,
    )

    private fun validateVersion(version: String) {
        require(version.isNotBlank()) { "version must not be blank" }
        require(!version.contains("..")) { "version must not contain path traversal: $version" }
        require(!version.startsWith("/")) { "version must not start with /: $version" }
        require(!version.contains("\\")) { "version must not contain backslash: $version" }
        require(!version.contains(":")) { "version must not contain colon: $version" }
        require(version.matches(VALID_VERSION_PATTERN)) {
            "version contains invalid characters: $version"
        }
    }

    private val VALID_VERSION_PATTERN = Regex("^[a-zA-Z0-9._+\\-]+$")

    companion object {
        fun fromContext(
            noBackupFilesDir: File,
            filesDir: File,
        ): DefaultRuntimeHostLayout {
            return DefaultRuntimeHostLayout(
                controlBaseDir = noBackupFilesDir,
                dataBaseDir = filesDir,
            )
        }
    }
}
