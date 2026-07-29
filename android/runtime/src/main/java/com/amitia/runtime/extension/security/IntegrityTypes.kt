package com.amitia.runtime.extension.security

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement

@Serializable
data class FileEntry(
    val path: String,
    val size: Long,
    val hash: String,
    @SerialName("modified") val modified: String? = null,
    @SerialName("isDir") val isDir: Boolean = false
)

@Serializable
data class IntegrityFilesDoc(
    val algorithm: String = "sha256",
    val files: Map<String, FileEntry> = emptyMap(),
    @SerialName("generatedAt") val generatedAt: String? = null
)

@Serializable
data class IntegrityTreeDoc(
    val algorithm: String = "sha256",
    @SerialName("treeHash") val treeHash: String,
    @SerialName("generatedAt") val generatedAt: String? = null
)

@Serializable
data class SignatureDoc(
    val algorithm: String,
    @SerialName("keyId") val keyId: String,
    val signature: ByteArray,
    @SerialName("signedAt") val signedAt: String? = null,
    @SerialName("publisherId") val publisherId: String? = null
) {
    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (other !is SignatureDoc) return false
        return algorithm == other.algorithm &&
            keyId == other.keyId &&
            signature.contentEquals(other.signature) &&
            signedAt == other.signedAt &&
            publisherId == other.publisherId
    }

    override fun hashCode(): Int {
        var result = algorithm.hashCode()
        result = 31 * result + keyId.hashCode()
        result = 31 * result + signature.contentHashCode()
        result = 31 * result + (signedAt?.hashCode() ?: 0)
        result = 31 * result + (publisherId?.hashCode() ?: 0)
        return result
    }
}

@Serializable
data class SignaturePayloadDoc(
    @SerialName("extensionId") val extensionId: String,
    val version: String,
    @SerialName("manifestVersion") val manifestVersion: Int,
    @SerialName("manifestHash") val manifestHash: String,
    @SerialName("contentTreeHash") val contentTreeHash: String,
    @SerialName("packageHash") val packageHash: String,
    @SerialName("publisherId") val publisherId: String,
    @SerialName("keyId") val keyId: String,
    @SerialName("createdAt") val createdAt: String,
    @SerialName("compatibilityHash") val compatibilityHash: String? = null,
    val channel: String? = null
)

@Serializable
data class SignatureDocument(
    val format: String,
    val algorithm: String,
    @SerialName("publisherId") val publisherId: String,
    @SerialName("keyId") val keyId: String,
    @SerialName("payloadHash") val payloadHash: String,
    val signature: String,
    @SerialName("createdAt") val createdAt: String,
    val channel: String? = null
)

@Serializable
data class ArchiveEntryInfo(
    val path: String,
    val normalizedPath: String,
    val compressedSize: Long,
    val uncompressedSize: Long,
    val crc32: Long
)

data class ArchiveInspectionResult(
    val totalCompressed: Long,
    val totalUncompressed: Long,
    val compressionRatio: Double,
    val entryCount: Int,
    val entries: List<ArchiveEntryInfo>,
    val pathCollisions: List<SafePathValidator.PathCollision>,
    val errors: List<String>,
    val warnings: List<String>,
    val passed: Boolean
)
