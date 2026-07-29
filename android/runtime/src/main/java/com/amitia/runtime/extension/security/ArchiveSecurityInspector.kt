package com.amitia.runtime.extension.security

import java.io.InputStream
import java.util.zip.ZipEntry
import java.util.zip.ZipInputStream

class ArchiveSecurityInspector(
    private val policy: ArchivePolicy = ArchivePolicy.default(),
    private val pathValidator: SafePathValidator = SafePathValidator(
        maxPathLength = policy.maxPathLength,
        maxDirectoryDepth = policy.maxDirectoryDepth
    ),
    private val entryValidator: EntryValidator = EntryValidator(policy)
) {

    data class SecureEntry(
        val path: String,
        val data: ByteArray,
        val compressedSize: Long,
        val uncompressedSize: Long
    )

    data class InspectionSummary(
        val totalCompressed: Long,
        val totalUncompressed: Long,
        val compressionRatio: Double,
        val entryCount: Int,
        val warnings: List<String>
    )

    fun inspectAndExtract(stream: InputStream): Pair<Map<String, ByteArray>, InspectionSummary> {
        val entries = mutableMapOf<String, ByteArray>()
        val seenPaths = mutableSetOf<String>()
        val warnings = mutableListOf<String>()
        var totalCompressed = 0L
        var totalUncompressed = 0L
        var entryCount = 0

        ZipInputStream(stream).use { zis ->
            var entry: ZipEntry? = zis.nextEntry
            while (entry != null) {
                entryCount++

                if (entryCount > policy.maxEntryCount) {
                    throw EntryCountExceededException(
                        "entry count exceeds limit ${policy.maxEntryCount}"
                    )
                }

                val name = entry.name

                if (!isUtf8Valid(name) || name.contains(0.toChar())) {
                    throw NonUtf8PathException("non-UTF8 or null byte in path: $name")
                }

                val normalized = pathValidator.normalizeArchivePath(name)
                pathValidator.validatePath(normalized)

                val canonicalKey = normalized.lowercase()
                if (seenPaths.contains(canonicalKey)) {
                    throw DuplicatePathException("duplicate entry detected: $normalized")
                }
                seenPaths.add(canonicalKey)

                if (!AllowedRootDirs.isAllowed(normalized)) {
                    throw InvalidStructureException("unknown root entry: $normalized")
                }

                if (!entry.isDirectory) {
                    val uncompressedSize = if (entry.size > 0) entry.size else 0L
                    val compressedSize = if (entry.compressedSize > 0) entry.compressedSize else 0L

                    if (uncompressedSize > 0 && uncompressedSize > policy.maxSingleEntryBytes) {
                        throw SizeLimitExceededException(
                            "entry exceeds max size ${policy.maxSingleEntryBytes}: $normalized (size=$uncompressedSize)"
                        )
                    }

                    if (compressedSize > 0 && uncompressedSize > 0) {
                        val ratio = uncompressedSize.toDouble() / compressedSize.toDouble()
                        if (ratio > policy.maxCompressionRatio) {
                            throw CompressionRatioExceededException(
                                "compression ratio $ratio exceeds limit ${policy.maxCompressionRatio}: $normalized"
                            )
                        }
                    }

                    val limitedSize = if (uncompressedSize > 0 && uncompressedSize <= policy.maxSingleEntryBytes) {
                        uncompressedSize
                    } else {
                        policy.maxSingleEntryBytes
                    }

                    val data = readLimitedBytes(zis, limitedSize + 1)
                    if (data.size.toLong() > policy.maxSingleEntryBytes) {
                        throw SizeLimitExceededException(
                            "entry exceeds max size ${policy.maxSingleEntryBytes}: $normalized (actual=${data.size})"
                        )
                    }

                    totalCompressed += compressedSize
                    totalUncompressed += data.size

                    if (totalUncompressed > policy.maxTotalUncompressedBytes) {
                        throw SizeLimitExceededException(
                            "total uncompressed size exceeds limit ${policy.maxTotalUncompressedBytes}"
                        )
                    }

                    val validationResult = entryValidator.validate(normalized, data)
                    if (!validationResult.passed) {
                        throw ForbiddenFileTypeException(
                            validationResult.errors.joinToString("; ")
                        )
                    }
                    warnings.addAll(validationResult.warnings)

                    entries[normalized] = data
                }

                zis.closeEntry()
                entry = zis.nextEntry
            }
        }

        val compressionRatio = if (totalCompressed > 0) {
            totalUncompressed.toDouble() / totalCompressed.toDouble()
        } else 0.0

        val summary = InspectionSummary(
            totalCompressed = totalCompressed,
            totalUncompressed = totalUncompressed,
            compressionRatio = compressionRatio,
            entryCount = entryCount,
            warnings = warnings
        )

        return Pair(entries, summary)
    }

    private fun readLimitedBytes(stream: InputStream, maxBytes: Long): ByteArray {
        val buffer = ByteArray(8192)
        val output = java.io.ByteArrayOutputStream()
        var totalRead = 0L

        while (totalRead < maxBytes) {
            val toRead = minOf(buffer.size.toLong(), maxBytes - totalRead).toInt()
            val read = stream.read(buffer, 0, toRead)
            if (read == -1) break
            output.write(buffer, 0, read)
            totalRead += read
        }

        return output.toByteArray()
    }

    private fun isUtf8Valid(input: String): Boolean {
        return try {
            val bytes = input.toByteArray(Charsets.UTF_8)
            val redecoded = String(bytes, Charsets.UTF_8)
            redecoded == input
        } catch (e: Exception) {
            false
        }
    }
}
