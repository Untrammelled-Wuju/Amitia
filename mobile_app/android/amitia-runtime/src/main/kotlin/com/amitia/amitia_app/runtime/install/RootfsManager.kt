package com.amitia.amitia_app.runtime.install

import java.io.File
import java.util.UUID

internal data class RootfsInfo(
    val rootfsId: String,
    val payloadSha256: String,
    val installedPath: String,
)

internal sealed interface RootfsPrepareResult {
    data class Reused(val info: RootfsInfo) : RootfsPrepareResult
    data class NewlyInstalled(val info: RootfsInfo) : RootfsPrepareResult
    data class Replaced(val info: RootfsInfo) : RootfsPrepareResult
    data class Conflict(
        val existingRootfsId: String,
        val newRootfsId: String,
    ) : RootfsPrepareResult
    data class Failure(
        val code: RuntimeInstallErrorCode,
        val message: String,
    ) : RootfsPrepareResult
}

internal interface RootfsManager {
    fun prepareRootfs(
        rootfsPayloadFile: File,
        expectedRootfsId: String,
        expectedPayloadSha256: String,
        allowReplace: Boolean = false,
    ): RootfsPrepareResult
    fun getInstalledRootfs(): RootfsInfo?
}

internal class DefaultRootfsManager(
    private val controlRoot: File,
    private val extractor: SafeArchiveExtractor,
) : RootfsManager {

    private val rootfsRoot: File = File(controlRoot, RuntimeHostLayout.DIR_ROOTFS)
    private val metadataRoot: File = File(controlRoot, RuntimeHostLayout.DIR_METADATA)
    private var pendingReplacement: PendingReplacement? = null
    private val rootfsMarkerFile: File
        get() = File(metadataRoot, "installed-rootfs.json")

    override fun prepareRootfs(
        rootfsPayloadFile: File,
        expectedRootfsId: String,
        expectedPayloadSha256: String,
        allowReplace: Boolean,
    ): RootfsPrepareResult {
        val installed = getInstalledRootfs()

        if (installed != null) {
            if (installed.rootfsId == expectedRootfsId && installed.payloadSha256 == expectedPayloadSha256) {
                return RootfsPrepareResult.Reused(installed)
            }
            if (!allowReplace) return RootfsPrepareResult.Conflict(installed.rootfsId, expectedRootfsId)
            return replaceRootfs(rootfsPayloadFile, expectedRootfsId, expectedPayloadSha256)
        }

        val rootfsDir = File(rootfsRoot.toPath().toString())
        if (rootfsDir.exists()) {
            rootfsDir.deleteRecursively()
        }
        rootfsDir.mkdirs()

        val extractResult = extractor.extractTarXz(
            tarXzFile = rootfsPayloadFile,
            targetDir = rootfsDir,
            rootBoundary = rootfsDir.absolutePath,
        )

        when (extractResult) {
            is SafeExtractResult.Success -> {
                val info = RootfsInfo(
                    rootfsId = expectedRootfsId,
                    payloadSha256 = expectedPayloadSha256,
                    installedPath = rootfsDir.absolutePath,
                )
                saveRootfsMarker(info)
                return RootfsPrepareResult.NewlyInstalled(info)
            }
            is SafeExtractResult.Failure -> {
                return RootfsPrepareResult.Failure(extractResult.code, extractResult.message)
            }
        }
    }

    fun completePreparation(success: Boolean) {
        val replacement = pendingReplacement ?: return
        if (success) {
            replacement.backupDir.deleteRecursively()
        } else {
            rootfsRoot.deleteRecursively()
            if (!replacement.backupDir.renameTo(rootfsRoot)) {
                replacement.backupDir.copyRecursively(rootfsRoot, overwrite = true)
                replacement.backupDir.deleteRecursively()
            }
            rootfsMarkerFile.writeBytes(replacement.previousMarker)
        }
        pendingReplacement = null
    }

    private fun replaceRootfs(
        payload: File,
        rootfsId: String,
        payloadSha256: String,
    ): RootfsPrepareResult {
        val nonce = UUID.randomUUID().toString()
        val stageDir = File(controlRoot, ".rootfs-stage-$nonce")
        val backupDir = File(controlRoot, ".rootfs-backup-$nonce")
        val previousMarker = rootfsMarkerFile.readBytes()
        stageDir.mkdirs()
        val extraction = extractor.extractTarXz(payload, stageDir, stageDir.absolutePath)
        if (extraction is SafeExtractResult.Failure) {
            stageDir.deleteRecursively()
            return RootfsPrepareResult.Failure(extraction.code, extraction.message)
        }
        try {
            if (!rootfsRoot.renameTo(backupDir)) {
                throw IllegalStateException("failed to back up installed rootfs")
            }
            if (!stageDir.renameTo(rootfsRoot)) {
                throw IllegalStateException("failed to publish replacement rootfs")
            }
            val info = RootfsInfo(rootfsId, payloadSha256, rootfsRoot.absolutePath)
            saveRootfsMarker(info)
            pendingReplacement = PendingReplacement(backupDir, previousMarker)
            return RootfsPrepareResult.Replaced(info)
        } catch (error: Exception) {
            if (backupDir.exists()) {
                rootfsRoot.deleteRecursively()
                if (!backupDir.renameTo(rootfsRoot)) {
                    backupDir.copyRecursively(rootfsRoot, overwrite = true)
                    backupDir.deleteRecursively()
                }
                rootfsMarkerFile.writeBytes(previousMarker)
            }
            return RootfsPrepareResult.Failure(RuntimeInstallErrorCode.ROOTFS_CONFLICT, error.message ?: "rootfs replacement failed")
        } finally {
            stageDir.deleteRecursively()
        }
    }

    override fun getInstalledRootfs(): RootfsInfo? {
        if (!rootfsMarkerFile.exists()) return null
        return try {
            val json = rootfsMarkerFile.readText(Charsets.UTF_8)
            val rootfsId = extractJsonString(json, "rootfsId")
            val payloadSha = extractJsonString(json, "payloadSha256")
            val path = extractJsonString(json, "installedPath")
            RootfsInfo(rootfsId, payloadSha, path)
        } catch (_: Exception) {
            null
        }
    }

    private fun saveRootfsMarker(info: RootfsInfo) {
        metadataRoot.mkdirs()
        val content = buildString {
            appendLine("{")
            appendLine("  \"rootfsId\": \"${info.rootfsId}\",")
            appendLine("  \"payloadSha256\": \"${info.payloadSha256}\",")
            appendLine("  \"installedPath\": \"${info.installedPath}\"")
            appendLine("}")
        }
        rootfsMarkerFile.writeText(content, Charsets.UTF_8)
    }

    private fun extractJsonString(json: String, key: String): String {
        val pattern = "\"$key\"\\s*:\\s*\"([^\"]+)\"".toRegex()
        val match = pattern.find(json) ?: throw IllegalArgumentException("missing key: $key")
        return match.groupValues[1]
    }

    private data class PendingReplacement(val backupDir: File, val previousMarker: ByteArray)
}
