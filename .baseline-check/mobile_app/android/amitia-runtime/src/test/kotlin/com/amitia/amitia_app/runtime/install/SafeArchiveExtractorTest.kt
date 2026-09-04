package com.amitia.amitia_app.runtime.install

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File
import java.util.zip.ZipEntry
import java.util.zip.ZipOutputStream

class SafeArchiveExtractorTest {

    @get:Rule
    val tempFolder = TemporaryFolder()

    @Test
    fun extractZipContainer_allowsOnlySpecifiedEntries() {
        val extractor = DefaultSafeArchiveExtractor(maxEntries = 100, maxTotalSize = 10_000_000, maxSingleFileSize = 1_000_000)
        val zipFile = createTestZip(
            "test.zip",
            mapOf(
                "metadata/package-index.json" to "{\"runtimeVersion\":\"1.0.0\"}",
                "metadata/SHA256SUMS" to "sha256  payload/runtime/runtime.tar.xz",
                "payload/runtime/runtime.tar.xz" to "binary content",
                "payload/unknown/file.txt" to "unknown content",
            )
        )
        val targetDir = tempFolder.newFolder("extract-allowed-only")

        val result = extractor.extractZipContainer(
            zipFile = zipFile,
            targetDir = targetDir,
            allowedEntries = setOf("metadata/package-index.json", "metadata/SHA256SUMS")
        )

        assertTrue(result is SafeExtractResult.Success)
        assertTrue(File(targetDir, "package-index.json").exists())
        assertTrue(File(targetDir, "SHA256SUMS").exists())
        assertFalse(File(targetDir, "file.txt").exists())
    }

    @Test
    fun extractZipContainer_rejectsPathTraversal() {
        val extractor = DefaultSafeArchiveExtractor()
        val zipFile = tempFolder.newFolder("malicious").let { folder ->
            val zip = File(folder, "evil.zip")
            ZipOutputStream(zip.outputStream()).use { zos ->
                val entry = ZipEntry("../evil.txt")
                zos.putNextEntry(entry)
                zos.write("evil content".toByteArray())
                zos.closeEntry()
            }
            zip
        }
        val targetDir = tempFolder.newFolder("extract-traversal")

        val result = extractor.extractZipContainer(
            zipFile = zipFile,
            targetDir = targetDir,
            allowedEntries = setOf("../evil.txt")
        )

        assertTrue(result is SafeExtractResult.Failure)
        val failure = result as SafeExtractResult.Failure
        assertEquals(RuntimeInstallErrorCode.ARCHIVE_PATH_INVALID, failure.code)
    }

    @Test
    fun extractZipContainer_rejectsDuplicateEntries() {
        val extractor = DefaultSafeArchiveExtractor()
        val zipFile = createTestZip(
            "dup.zip",
            mapOf(
                "metadata/package-index.json" to "content 1",
                "metadata/SHA256SUMS" to "hash sums",
            )
        )
        val targetDir = tempFolder.newFolder("extract-dup")

        val result = extractor.extractZipContainer(
            zipFile = zipFile,
            targetDir = targetDir,
            allowedEntries = setOf("metadata/package-index.json", "metadata/SHA256SUMS", "metadata/package-index.json")
        )

        assertTrue(result is SafeExtractResult.Success)
    }

    @Test
    fun extractZipContainer_failsForMissingFile() {
        val extractor = DefaultSafeArchiveExtractor()
        val missingFile = File(tempFolder.root, "nonexistent.zip")
        val targetDir = tempFolder.newFolder("extract-missing")

        val result = extractor.extractZipContainer(
            zipFile = missingFile,
            targetDir = targetDir,
            allowedEntries = setOf("test")
        )

        assertTrue(result is SafeExtractResult.Failure)
        assertEquals(RuntimeInstallErrorCode.PACKAGE_NOT_FOUND, (result as SafeExtractResult.Failure).code)
    }

    @Test
    fun calculateDataSize_correctlyTracksTotal() {
        val extractor = DefaultSafeArchiveExtractor(maxEntries = 100, maxTotalSize = 100_000_000, maxSingleFileSize = 10_000_000)
        val content = "x".repeat(1000)
        val zipFile = createTestZip(
            "sized.zip",
            (1..10).associate { "file$it.txt" to content }
        )

        val targetDir = tempFolder.newFolder("extract-sized")
        val result = extractor.extractZipContainer(
            zipFile = zipFile,
            targetDir = targetDir,
            allowedEntries = (1..10).map { "file$it.txt" }.toSet()
        )

        assertTrue(result is SafeExtractResult.Success)
        val success = result as SafeExtractResult.Success
        assertEquals(10, success.result.entriesExtracted)
        assertEquals(10000L, success.result.totalBytes)
    }

    private fun createTestZip(name: String, entries: Map<String, String>): File {
        val zipFile = File(tempFolder.root, name)
        ZipOutputStream(zipFile.outputStream()).use { zos ->
            for ((path, content) in entries) {
                val entry = ZipEntry(path)
                zos.putNextEntry(entry)
                zos.write(content.toByteArray())
                zos.closeEntry()
            }
        }
        return zipFile
    }
}
