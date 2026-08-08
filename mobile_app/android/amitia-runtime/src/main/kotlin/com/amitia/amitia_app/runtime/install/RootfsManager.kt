package com.amitia.amitia_app.runtime.install

import java.io.File

internal data class RootfsInfo(
    val rootfsId: String,
    val payloadSha256: String,
    val installedPath: String,
)

internal sealed interface RootfsPrepareResult {
    data class Reused(val info: RootfsInfo) : RootfsPrepareResult
    data class NewlyInstalled(val info: RootfsInfo) : RootfsPrepareResult
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
    ): RootfsPrepareResult
    fun getInstalledRootfs(): RootfsInfo?
}

internal class DefaultRootfsManager(
    private val controlRoot: File,
    private val extractor: SafeArchiveExtractor,
) : RootfsManager {

    private val rootfsRoot: File = File(controlRoot, RuntimeHostLayout.DIR_ROOTFS)
    private val metadataRoot: File = File(controlRoot, RuntimeHostLayout.DIR_METADATA)
    private val rootfsMarkerFile: File
        get() = File(metadataRoot, "installed-rootfs.json")

    override fun prepareRootfs(
        rootfsPayloadFile: File,
        expectedRootfsId: String,
        expectedPayloadSha256: String,
    ): RootfsPrepareResult {
        val installed = getInstalledRootfs()

        if (installed != null) {
            if (installed.rootfsId == expectedRootfsId && installed.payloadSha256 == expectedPayloadSha256) {
                return RootfsPrepareResult.Reused(installed)
            }
            return RootfsPrepareResult.Conflict(installed.rootfsId, expectedRootfsId)
        }

        val rootfsDir = File(rootfsRoot.toPath().toString())
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
        try {
            val content = buildString {
                appendLine("{")
                appendLine("  \"rootfsId\": \"${info.rootfsId}\",")
                appendLine("  \"payloadSha256\": \"${info.payloadSha256}\",")
                appendLine("  \"installedPath\": \"${info.installedPath}\"")
                appendLine("}")
            }
            rootfsMarkerFile.writeText(content, Charsets.UTF_8)
        } catch (_: Exception) {
        }
    }

    private fun extractJsonString(json: String, key: String): String {
        val pattern = "\"$key\"\\s*:\\s*\"([^\"]+)\"".toRegex()
        val match = pattern.find(json) ?: throw IllegalArgumentException("missing key: $key")
        return match.groupValues[1]
    }
}
