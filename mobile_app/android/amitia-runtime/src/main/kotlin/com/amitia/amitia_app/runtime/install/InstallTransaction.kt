package com.amitia.amitia_app.runtime.install

import java.io.File
import java.util.UUID

internal enum class TransactionStage {
    CREATED,
    PACKAGE_VERIFIED,
    ROOTFS_PREPARED,
    RUNTIME_EXTRACTED,
    RUNTIME_VERIFIED,
    PUBLISHED,
    ACTIVATED,
    COMPLETED,
    FAILED,
}

internal data class TransactionJournal(
    val transactionId: String,
    val stage: TransactionStage,
    val runtimeVersion: String?,
    val packageSha256: String?,
    val stagingDir: String?,
    val targetVersionDir: String?,
    val createdAtEpochMillis: Long,
    val updatedAtEpochMillis: Long,
)

internal interface InstallTransaction {
    val transactionId: String
    fun stage(): TransactionStage
    fun updateStage(stage: TransactionStage)
    fun setRuntimeVersion(version: String)
    fun setPackageSha256(sha: String)
    fun setTargetVersionDir(dir: String)
    fun getJournal(): TransactionJournal
    fun rollback()
}

internal interface InstallTransactionFactory {
    fun create(runtimeInstallPaths: RuntimeInstallPaths): InstallTransaction
    fun recover(runtimeInstallPaths: RuntimeInstallPaths): List<InstallTransaction>
}

internal class DefaultInstallTransaction(
    private val journalFile: File,
    private val stagingDir: String,
    private val timeProvider: () -> Long = { System.currentTimeMillis() },
) : InstallTransaction {
    private val tid: String = UUID.randomUUID().toString()
    private var currentStage: TransactionStage = TransactionStage.CREATED
    private var runtimeVersion: String? = null
    private var packageSha256: String? = null
    private var targetVersionDir: String? = null
    private val createdAt: Long = timeProvider()
    private var updatedAt: Long = createdAt

    override val transactionId: String get() = tid

    init {
        persistJournal()
    }

    override fun stage(): TransactionStage = currentStage

    override fun updateStage(stage: TransactionStage) {
        currentStage = stage
        updatedAt = timeProvider()
        persistJournal()
    }

    override fun setRuntimeVersion(version: String) {
        runtimeVersion = version
        updatedAt = timeProvider()
        persistJournal()
    }

    override fun setPackageSha256(sha: String) {
        packageSha256 = sha
        updatedAt = timeProvider()
        persistJournal()
    }

    override fun setTargetVersionDir(dir: String) {
        targetVersionDir = dir
        updatedAt = timeProvider()
        persistJournal()
    }

    override fun getJournal(): TransactionJournal = TransactionJournal(
        transactionId = tid,
        stage = currentStage,
        runtimeVersion = runtimeVersion,
        packageSha256 = packageSha256,
        stagingDir = stagingDir,
        targetVersionDir = targetVersionDir,
        createdAtEpochMillis = createdAt,
        updatedAtEpochMillis = updatedAt,
    )

    override fun rollback() {
        val dir = File(stagingDir)
        if (dir.exists() && dir.isDirectory) {
            dir.deleteRecursively()
        }
        try {
            journalFile.delete()
        } catch (_: Exception) {
        }
    }

    private fun persistJournal() {
        try {
            journalFile.parentFile?.mkdirs()
            val tmpFile = File("${journalFile.absolutePath}.tmp")
            val content = buildString {
                appendLine("transactionId=$tid")
                appendLine("stage=${currentStage.name}")
                appendLine("runtimeVersion=${runtimeVersion ?: ""}")
                appendLine("packageSha256=${packageSha256 ?: ""}")
                appendLine("stagingDir=$stagingDir")
                appendLine("targetVersionDir=${targetVersionDir ?: ""}")
                appendLine("createdAtEpochMillis=$createdAt")
                appendLine("updatedAtEpochMillis=$updatedAt")
            }
            tmpFile.writeText(content, Charsets.UTF_8)
            if (!tmpFile.renameTo(journalFile)) {
                journalFile.delete()
                tmpFile.renameTo(journalFile)
            }
        } catch (_: Exception) {
        }
    }
}
