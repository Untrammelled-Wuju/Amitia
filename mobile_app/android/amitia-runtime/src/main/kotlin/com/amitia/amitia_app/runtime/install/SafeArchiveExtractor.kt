package com.amitia.amitia_app.runtime.install

import java.io.File
import java.util.zip.ZipEntry
import java.util.zip.ZipFile

internal data class ExtractionResult(
    val entriesExtracted: Int,
    val totalBytes: Long,
)

internal sealed interface SafeExtractResult {
    data class Success(val result: ExtractionResult) : SafeExtractResult
    data class Failure(
        val code: RuntimeInstallErrorCode,
        val message: String,
    ) : SafeExtractResult
}

internal interface SafeArchiveExtractor {
    fun extractZipContainer(
        zipFile: File,
        targetDir: File,
        allowedEntries: Set<String>,
    ): SafeExtractResult

    fun extractTarXz(
        tarXzFile: File,
        targetDir: File,
        rootBoundary: String?,
    ): SafeExtractResult
}

internal class DefaultSafeArchiveExtractor(
    private val maxEntries: Long = 100_000L,
    private val maxTotalSize: Long = 2L * 1024L * 1024L * 1024L,
    private val maxSingleFileSize: Long = 500L * 1024L * 1024L,
) : SafeArchiveExtractor {

    override fun extractZipContainer(
        zipFile: File,
        targetDir: File,
        allowedEntries: Set<String>,
    ): SafeExtractResult {
        if (!zipFile.exists()) {
            return SafeExtractResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_NOT_FOUND,
                "zip file not found: ${zipFile.absolutePath}"
            )
        }

        val zip = try {
            ZipFile(zipFile)
        } catch (e: Exception) {
            return SafeExtractResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_READ_FAILED,
                "failed to open zip: ${e.message}"
            )
        }

        try {
            val entries = zip.entries()
            val seenPaths = HashSet<String>()
            var totalExtracted = 0
            var totalBytes = 0L

            while (entries.hasMoreElements()) {
                val entry = entries.nextElement()
                val entryName = entry.name

                if (!allowedEntries.contains(entryName)) {
                    continue
                }

                if (!isPathSafe(entryName)) {
                    return SafeExtractResult.Failure(
                        RuntimeInstallErrorCode.ARCHIVE_PATH_INVALID,
                        "unsafe path in zip: $entryName"
                    )
                }

                if (seenPaths.contains(entryName)) {
                    return SafeExtractResult.Failure(
                        RuntimeInstallErrorCode.ARCHIVE_ENTRY_DUPLICATE,
                        "duplicate entry in zip: $entryName"
                    )
                }
                seenPaths.add(entryName)

                if (++totalExtracted > maxEntries) {
                    return SafeExtractResult.Failure(
                        RuntimeInstallErrorCode.ARCHIVE_TOO_LARGE,
                        "too many entries in archive"
                    )
                }

                if (entry.size > maxSingleFileSize) {
                    return SafeExtractResult.Failure(
                        RuntimeInstallErrorCode.ARCHIVE_TOO_LARGE,
                        "single entry too large: $entryName"
                    )
                }

                totalBytes += entry.size
                if (totalBytes > maxTotalSize) {
                    return SafeExtractResult.Failure(
                        RuntimeInstallErrorCode.ARCHIVE_TOO_LARGE,
                        "total extraction size exceeds limit"
                    )
                }

                val targetFile = File(targetDir, entryName)
                if (!targetFile.absolutePath.startsWith(targetDir.absolutePath)) {
                    return SafeExtractResult.Failure(
                        RuntimeInstallErrorCode.ARCHIVE_PATH_INVALID,
                        "path traversal detected: $entryName"
                    )
                }

                if (entry.isDirectory) {
                    targetFile.mkdirs()
                } else {
                    targetFile.parentFile?.mkdirs()
                    zip.getInputStream(entry).use { input ->
                        targetFile.outputStream().use { output ->
                            input.copyTo(output)
                        }
                    }
                }
            }

            return SafeExtractResult.Success(
                ExtractionResult(totalExtracted, totalBytes)
            )
        } catch (e: Exception) {
            return SafeExtractResult.Failure(
                RuntimeInstallErrorCode.ARCHIVE_INVALID,
                "extraction failed: ${e.message}"
            )
        } finally {
            try {
                zip.close()
            } catch (_: Exception) {
            }
        }
    }

    override fun extractTarXz(
        tarXzFile: File,
        targetDir: File,
        rootBoundary: String?,
    ): SafeExtractResult {
        targetDir.mkdirs()
        var totalExtracted = 0
        var totalBytes = 0L

        try {
            val processBuilder = ProcessBuilder(
                "tar",
                "-xJf",
                tarXzFile.absolutePath,
                "-C",
                targetDir.absolutePath,
                "--no-same-owner",
                "--no-same-permissions",
            )
            processBuilder.redirectErrorStream(true)
            val process = processBuilder.start()

            val exitCode = try {
                process.waitFor()
            } catch (e: InterruptedException) {
                process.destroy()
                return SafeExtractResult.Failure(
                    RuntimeInstallErrorCode.RUNTIME_EXTRACT_FAILED,
                    "extraction interrupted"
                )
            }

            if (exitCode != 0) {
                return SafeExtractResult.Failure(
                    RuntimeInstallErrorCode.RUNTIME_EXTRACT_FAILED,
                    "tar extraction failed with exit code: $exitCode"
                )
            }

            val entries = targetDir.walkTopDown().filter { it.isFile }.toList()
            totalExtracted = entries.size
            totalBytes = entries.sumOf { it.length() }

            if (rootBoundary != null) {
                val boundary = File(rootBoundary)
                for (entry in entries) {
                    if (!entry.absolutePath.startsWith(boundary.absolutePath)) {
                        return SafeExtractResult.Failure(
                            RuntimeInstallErrorCode.ARCHIVE_PATH_INVALID,
                            "entry outside root boundary: ${entry.absolutePath}"
                        )
                    }
                }
            }

            for (entry in entries) {
                if (entry.absolutePath != entry.canonicalPath) {
                    val resolvedCanonical = entry.canonicalFile.parentFile ?: continue
                    val targetCanonical = targetDir.canonicalFile
                    if (!resolvedCanonical.absolutePath.startsWith(targetCanonical.absolutePath)) {
                        return SafeExtractResult.Failure(
                            RuntimeInstallErrorCode.ARCHIVE_PATH_INVALID,
                            "symlink escapes root boundary"
                        )
                    }
                }
            }

            return SafeExtractResult.Success(
                ExtractionResult(totalExtracted, totalBytes)
            )
        } catch (e: Exception) {
            return SafeExtractResult.Failure(
                RuntimeInstallErrorCode.RUNTIME_EXTRACT_FAILED,
                "tar extraction error: ${e.message}"
            )
        }
    }

    private fun isPathSafe(path: String): Boolean {
        if (path.isEmpty()) return false
        if (path.contains("..")) return false
        if (path.startsWith("/")) return false
        if (path.startsWith("C:", ignoreCase = true)) return false
        if (path.contains(":")) return false
        return true
    }
}
