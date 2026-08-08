package com.amitia.amitia_app.runtime.install.internal

import com.amitia.amitia_app.runtime.install.RuntimeHostLayout
import com.amitia.amitia_app.runtime.install.RuntimeInstallPaths
import java.io.File

internal class DefaultRuntimeInstallPaths(
    private val baseDir: File,
) : RuntimeInstallPaths {

    private val controlRoot = File(baseDir, "runtime-control")
    private val rootfsRoot = File(controlRoot, RuntimeHostLayout.DIR_ROOTFS)
    private val versionsRoot = File(controlRoot, RuntimeHostLayout.DIR_VERSIONS)
    private val stagingRoot = File(controlRoot, RuntimeHostLayout.DIR_STAGING)
    private val metadataRoot = File(controlRoot, RuntimeHostLayout.DIR_METADATA)
    private val transactionsRoot = File(controlRoot, RuntimeHostLayout.DIR_TRANSACTIONS)
    private val locksRoot = File(controlRoot, RuntimeHostLayout.DIR_LOCKS)
    private val receiptsRoot = File(metadataRoot, "install-receipts")

    init {
        require(baseDir.absolutePath.isNotBlank()) { "baseDir must not be blank" }
    }

    override fun controlRoot(): String = controlRoot.absolutePath
    override fun rootfsRoot(): String = rootfsRoot.absolutePath
    override fun versionsRoot(): String = versionsRoot.absolutePath
    override fun stagingRoot(): String = stagingRoot.absolutePath
    override fun metadataRoot(): String = metadataRoot.absolutePath
    override fun transactionsRoot(): String = transactionsRoot.absolutePath
    override fun locksRoot(): String = locksRoot.absolutePath
    override fun receiptsRoot(): String = receiptsRoot.absolutePath

    override fun versionDir(version: String): String = File(versionsRoot, version).absolutePath
    override fun stagingDir(transactionId: String): String = File(stagingRoot, transactionId).absolutePath
    override fun installLockFile(): String = File(locksRoot, RuntimeHostLayout.FILE_INSTALL_LOCK).absolutePath
    override fun receiptFile(version: String): String = File(receiptsRoot, "$version.json").absolutePath
    override fun activeRuntimeFile(): String = File(metadataRoot, RuntimeHostLayout.FILE_ACTIVE_RUNTIME).absolutePath

    companion object {
        fun fromContext(filesDir: File): DefaultRuntimeInstallPaths {
            return DefaultRuntimeInstallPaths(File(filesDir, "amitia-runtime"))
        }
    }
}
