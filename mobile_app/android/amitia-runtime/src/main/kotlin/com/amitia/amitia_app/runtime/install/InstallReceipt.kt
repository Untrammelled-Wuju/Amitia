package com.amitia.amitia_app.runtime.install

import java.io.File

internal data class RuntimeInstallReceipt(
    val schemaVersion: Int,
    val runtimeVersion: String,
    val packageId: String,
    val packageSha256: String,
    val rootfsId: String,
    val rootfsPayloadSha256: String,
    val runtimePayloadSha256: String,
    val runtimeRootTreeSha256: String,
    val componentSha256: Map<String, String> = emptyMap(),
) {
    companion object {
        const val SCHEMA_VERSION = 2
    }
}

internal sealed interface InstallReceiptResult {
    data class Success(val receipt: RuntimeInstallReceipt) : InstallReceiptResult
    data class Failure(
        val code: RuntimeInstallErrorCode,
        val message: String,
    ) : InstallReceiptResult
}

internal interface InstallReceiptStore {
    fun save(receipt: RuntimeInstallReceipt): InstallReceiptResult
    fun load(version: String): InstallReceiptResult
    fun exists(version: String): Boolean
}

internal class DefaultInstallReceiptStore(
    private val layout: RuntimeHostLayout,
) : InstallReceiptStore {

    override fun save(receipt: RuntimeInstallReceipt): InstallReceiptResult {
        val receiptFile = layout.installReceiptFile(receipt.runtimeVersion)
        return try {
            receiptFile.parentFile?.mkdirs()
            val tmpFile = File("${receiptFile.absolutePath}.tmp")
            val content = buildString {
                appendLine("{")
                appendLine("  \"schemaVersion\": ${receipt.schemaVersion},")
                appendLine("  \"runtimeVersion\": \"${receipt.runtimeVersion}\",")
                appendLine("  \"packageId\": \"${receipt.packageId}\",")
                appendLine("  \"packageSha256\": \"${receipt.packageSha256}\",")
                appendLine("  \"rootfsId\": \"${receipt.rootfsId}\",")
                appendLine("  \"rootfsPayloadSha256\": \"${receipt.rootfsPayloadSha256}\",")
                appendLine("  \"runtimePayloadSha256\": \"${receipt.runtimePayloadSha256}\",")
                appendLine("  \"runtimeRootTreeSha256\": \"${receipt.runtimeRootTreeSha256}\",")
                val componentEntries = receipt.componentSha256.entries
                    .sortedBy { it.key }
                    .joinToString(",\n") { (id, sha) -> "    \"$id\": \"$sha\"" }
                if (componentEntries.isEmpty()) {
                    appendLine("  \"componentSha256\": {}")
                } else {
                    appendLine("  \"componentSha256\": {")
                    appendLine(componentEntries)
                    appendLine("  }")
                }
                appendLine("}")
            }
            tmpFile.writeText(content, Charsets.UTF_8)
            if (!tmpFile.renameTo(receiptFile)) {
                receiptFile.delete()
                if (!tmpFile.renameTo(receiptFile)) {
                    return InstallReceiptResult.Failure(
                        RuntimeInstallErrorCode.INSTALL_RECEIPT_WRITE_FAILED,
                        "failed to write receipt atomically"
                    )
                }
            }
            InstallReceiptResult.Success(receipt)
        } catch (e: Exception) {
            InstallReceiptResult.Failure(
                RuntimeInstallErrorCode.INSTALL_RECEIPT_WRITE_FAILED,
                "failed to save receipt: ${e.message}"
            )
        }
    }

    override fun load(version: String): InstallReceiptResult {
        val receiptFile = layout.installReceiptFile(version)
        if (!receiptFile.exists()) {
            return InstallReceiptResult.Failure(
                RuntimeInstallErrorCode.INSTALL_RECEIPT_WRITE_FAILED,
                "receipt not found for version: $version"
            )
        }
        return try {
            InstallReceiptResult.Success(parseReceipt(receiptFile.readText(Charsets.UTF_8)))
        } catch (e: Exception) {
            InstallReceiptResult.Failure(
                RuntimeInstallErrorCode.INSTALL_RECEIPT_WRITE_FAILED,
                "failed to read receipt: ${e.message}"
            )
        }
    }

    override fun exists(version: String): Boolean {
        return layout.installReceiptFile(version).exists()
    }

    private fun parseReceipt(json: String): RuntimeInstallReceipt {
        val schemaVersion = extractJsonNumberOrNull(json, "schemaVersion")
        val runtimeVersion = extractJsonString(json, "runtimeVersion")
        val packageId = extractJsonString(json, "packageId")
        val packageSha256 = extractJsonString(json, "packageSha256")
        val rootfsId = extractJsonString(json, "rootfsId")
        val rootfsPayloadSha256 = extractJsonString(json, "rootfsPayloadSha256")
        val runtimePayloadSha256 = extractJsonString(json, "runtimePayloadSha256")
        val runtimeRootTreeSha256 = extractJsonString(json, "runtimeRootTreeSha256")

        return RuntimeInstallReceipt(
            schemaVersion = schemaVersion ?: RuntimeInstallReceipt.SCHEMA_VERSION,
            runtimeVersion = runtimeVersion,
            packageId = packageId,
            packageSha256 = packageSha256,
            rootfsId = rootfsId,
            rootfsPayloadSha256 = rootfsPayloadSha256,
            runtimePayloadSha256 = runtimePayloadSha256,
            runtimeRootTreeSha256 = runtimeRootTreeSha256,
            componentSha256 = parseComponentSha256(json),
        )
    }

    private fun parseComponentSha256(json: String): Map<String, String> {
        val objectMatch = Regex("\"componentSha256\"\\s*:\\s*\\{([^{}]*)\\}").find(json)
            ?: return emptyMap()
        val entryPattern = Regex("\"([^\"]+)\"\\s*:\\s*\"([a-fA-F0-9]{64})\"")
        return entryPattern.findAll(objectMatch.groupValues[1]).associate {
            it.groupValues[1] to it.groupValues[2].lowercase()
        }
    }

    private fun extractJsonNumberOrNull(json: String, key: String): Int? {
        val pattern = "\"$key\"\\s*:\\s*(\\d+)".toRegex()
        return pattern.find(json)?.groupValues?.get(1)?.toIntOrNull()
    }

    private fun extractJsonString(json: String, key: String): String {
        val pattern = "\"$key\"\\s*:\\s*\"([^\"]+)\"".toRegex()
        val match = pattern.find(json) ?: throw IllegalArgumentException("missing key: $key")
        return match.groupValues[1]
    }
}
