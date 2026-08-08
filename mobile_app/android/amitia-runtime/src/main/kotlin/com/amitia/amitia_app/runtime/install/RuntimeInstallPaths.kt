package com.amitia.amitia_app.runtime.install

internal interface RuntimeInstallPaths {
    fun controlRoot(): String
    fun rootfsRoot(): String
    fun versionsRoot(): String
    fun stagingRoot(): String
    fun metadataRoot(): String
    fun transactionsRoot(): String
    fun locksRoot(): String
    fun receiptsRoot(): String

    fun versionDir(version: String): String
    fun stagingDir(transactionId: String): String
    fun installLockFile(): String
    fun receiptFile(version: String): String
    fun activeRuntimeFile(): String

    companion object {
        const val DIR_ROOTFS = "rootfs"
        const val DIR_VERSIONS = "versions"
        const val DIR_STAGING = "staging"
        const val DIR_METADATA = "metadata"
        const val DIR_TRANSACTIONS = "transactions"
        const val DIR_LOCKS = "locks"
        const val DIR_RECEIPTS = "install-receipts"
        const val FILE_INSTALL_LOCK = "install.lock"
        const val FILE_ACTIVE_RUNTIME = "active-runtime.json"
    }
}
