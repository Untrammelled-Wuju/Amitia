package com.amitia.amitia_app.runtime.install

import org.apache.commons.compress.archivers.tar.TarArchiveEntry
import org.apache.commons.compress.archivers.tar.TarArchiveInputStream
import org.apache.commons.compress.compressors.xz.XZCompressorInputStream
import java.io.File
import java.io.FileInputStream
import java.io.FileOutputStream
import java.io.IOException
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

                val strippedName = stripContainerPrefix(entryName)
                val targetFile = File(targetDir, strippedName)
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
        if (!tarXzFile.isFile) {
            return SafeExtractResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_NOT_FOUND,
                "tar.xz file not found: ${tarXzFile.absolutePath}"
            )
        }
        if (!targetDir.exists() && !targetDir.mkdirs()) {
            return SafeExtractResult.Failure(
                RuntimeInstallErrorCode.RUNTIME_EXTRACT_FAILED,
                "cannot create target directory: ${targetDir.absolutePath}"
            )
        }

        var totalExtracted = 0
        var totalBytes = 0L

        try {
            FileInputStream(tarXzFile).use { fis ->
                XZCompressorInputStream(fis).use { xzIn ->
                    TarArchiveInputStream(xzIn).use { tarIn ->
                        var entry: TarArchiveEntry? = tarIn.nextEntry
                        while (entry != null) {
                            val entryName = entry.name

                            if (++totalExtracted > maxEntries) {
                                return SafeExtractResult.Failure(
                                    RuntimeInstallErrorCode.ARCHIVE_TOO_LARGE,
                                    "too many entries in archive"
                                )
                            }

                            val entrySize = entry.size
                            if (entrySize > maxSingleFileSize) {
                                return SafeExtractResult.Failure(
                                    RuntimeInstallErrorCode.ARCHIVE_TOO_LARGE,
                                    "single entry too large: $entryName"
                                )
                            }

                            totalBytes += entrySize
                            if (totalBytes > maxTotalSize) {
                                return SafeExtractResult.Failure(
                                    RuntimeInstallErrorCode.ARCHIVE_TOO_LARGE,
                                    "total extraction size exceeds limit"
                                )
                            }

                            validateAndExtractEntry(tarIn, entry, targetDir)

                            if (rootBoundary != null && !entry.isSymbolicLink) {
                                val extractedFile = File(targetDir, entryName).canonicalFile
                                val boundary = File(rootBoundary).canonicalFile
                                if (!extractedFile.canonicalPath.startsWith(boundary.canonicalPath)) {
                                    return SafeExtractResult.Failure(
                                        RuntimeInstallErrorCode.ARCHIVE_PATH_INVALID,
                                        "entry outside root boundary: $entryName"
                                    )
                                }
                            }

                            entry = tarIn.nextEntry
                        }
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

    private fun validateAndExtractEntry(
        tarIn: TarArchiveInputStream,
        entry: TarArchiveEntry,
        targetDir: File
    ) {
        val entryName = entry.name

        if (entryName.contains("..") || entryName.startsWith("/")) {
            throw IOException("path traversal detected: $entryName")
        }

        val targetFile = File(targetDir, entryName).canonicalFile
        if (!targetFile.canonicalPath.startsWith(targetDir.canonicalPath)) {
            throw IOException("entry escapes target directory: $entryName")
        }

        when {
            entry.isDirectory -> {
                if (!targetFile.exists()) {
                    check(targetFile.mkdirs()) { "cannot create directory: $targetFile" }
                }
            }
            entry.isSymbolicLink -> {
                val linkName = entry.linkName
                if (linkName.split('/').any { it == ".." }) {
                    throw IOException("symlink target invalid: $linkName")
                }
                createSymlink(targetFile, linkName)
            }
            entry.isFile -> {
                val parentDir = targetFile.parentFile
                if (parentDir != null && !parentDir.exists()) {
                    check(parentDir.mkdirs()) { "cannot create parent directory for: $targetFile" }
                }
                FileOutputStream(targetFile).use { fos ->
                    tarIn.copyTo(fos)
                }
                applyFilePermissions(targetFile, entry.mode)
            }
            else -> {
                throw IOException("unsupported entry type: $entryName")
            }
        }
    }

    private fun createSymlink(targetFile: File, linkName: String) {
        val targetPath = if (linkName.startsWith("./")) {
            linkName.substring(2)
        } else {
            linkName
        }
        targetFile.parentFile?.mkdirs()
        java.nio.file.Files.createSymbolicLink(
            targetFile.toPath(),
            java.nio.file.Paths.get(targetPath)
        )
    }

    private fun applyFilePermissions(file: File, mode: Int) {
        val readable = (mode and 292) != 0
        val writable = (mode and 146) != 0
        val executable = (mode and 73) != 0

        file.setReadable(readable, false)
        file.setWritable(writable, false)
        file.setExecutable(executable, false)
    }

    private fun isPathSafe(path: String): Boolean {
        if (path.isEmpty()) return false
        if (path.contains("..")) return false
        if (path.startsWith("/")) return false
        if (path.startsWith("C:", ignoreCase = true)) return false
        if (path.contains(":")) return false
        return true
    }

    private fun stripContainerPrefix(path: String): String {
        val firstSlash = path.indexOf('/')
        if (firstSlash < 0 || firstSlash == path.length - 1) return path
        return path.substring(firstSlash + 1)
    }
}
