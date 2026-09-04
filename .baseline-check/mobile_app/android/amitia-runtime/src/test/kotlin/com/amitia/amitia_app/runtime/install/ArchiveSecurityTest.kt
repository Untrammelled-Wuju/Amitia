package com.amitia.amitia_app.runtime.install

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File
import java.util.zip.ZipEntry
import java.util.zip.ZipOutputStream

class ArchiveSecurityTest {

    @get:Rule
    val tempFolder = TemporaryFolder()

    @Test
    fun extractZipContainer_rejectsAbsolutePath() {
        val extractor = DefaultSafeArchiveExtractor()
        val zipFile = tempFolder.newFolder("abs-attack").let { folder ->
            val zip = File(folder, "abs.zip")
            ZipOutputStream(zip.outputStream()).use { zos ->
                val entry = ZipEntry("/etc/passwd")
                zos.putNextEntry(entry)
                zos.write("root:x:0:0".toByteArray())
                zos.closeEntry()
            }
            zip
        }
        val targetDir = tempFolder.newFolder("extract-abs")

        val result = extractor.extractZipContainer(
            zipFile = zipFile,
            targetDir = targetDir,
            allowedEntries = setOf("/etc/passwd")
        )

        assertTrue(result is SafeExtractResult.Failure)
        assertEquals(RuntimeInstallErrorCode.ARCHIVE_PATH_INVALID, (result as SafeExtractResult.Failure).code)
    }

    @Test
    fun extractZipContainer_rejectsWindowsDrivePath() {
        val extractor = DefaultSafeArchiveExtractor()
        val zipFile = tempFolder.newFolder("win-attack").let { folder ->
            val zip = File(folder, "win.zip")
            ZipOutputStream(zip.outputStream()).use { zos ->
                val entry = ZipEntry("C:\\Windows\\System32\\evil.dll")
                zos.putNextEntry(entry)
                zos.write("evil".toByteArray())
                zos.closeEntry()
            }
            zip
        }
        val targetDir = tempFolder.newFolder("extract-win")

        val result = extractor.extractZipContainer(
            zipFile = zipFile,
            targetDir = targetDir,
            allowedEntries = setOf("C:\\Windows\\System32\\evil.dll")
        )

        assertTrue(result is SafeExtractResult.Failure)
        assertEquals(RuntimeInstallErrorCode.ARCHIVE_PATH_INVALID, (result as SafeExtractResult.Failure).code)
    }

    @Test
    fun extractZipContainer_rejectsDoubleDotTraversal() {
        val extractor = DefaultSafeArchiveExtractor()
        val zipFile = tempFolder.newFolder("dotdot-attack").let { folder ->
            val zip = File(folder, "dotdot.zip")
            ZipOutputStream(zip.outputStream()).use { zos ->
                val entry = ZipEntry("foo/../../../etc/shadow")
                zos.putNextEntry(entry)
                zos.write("secret".toByteArray())
                zos.closeEntry()
            }
            zip
        }
        val targetDir = tempFolder.newFolder("extract-dotdot")

        val result = extractor.extractZipContainer(
            zipFile = zipFile,
            targetDir = targetDir,
            allowedEntries = setOf("foo/../../../etc/shadow")
        )

        assertTrue(result is SafeExtractResult.Failure)
    }

    @Test
    fun extractZipContainer_rejectsEncodedTraversal() {
        val extractor = DefaultSafeArchiveExtractor()
        val zipFile = tempFolder.newFolder("encoded-attack").let { folder ->
            val zip = File(folder, "encoded.zip")
            ZipOutputStream(zip.outputStream()).use { zos ->
                val entry = ZipEntry("....//....//etc/passwd")
                zos.putNextEntry(entry)
                zos.write("root".toByteArray())
                zos.closeEntry()
            }
            zip
        }
        val targetDir = tempFolder.newFolder("extract-encoded")

        val result = extractor.extractZipContainer(
            zipFile = zipFile,
            targetDir = targetDir,
            allowedEntries = setOf("....//....//etc/passwd")
        )

        assertTrue(result is SafeExtractResult.Failure)
    }

    @Test
    fun extractZipContainer_emptyPathRejected() {
        val extractor = DefaultSafeArchiveExtractor()
        val zipFile = tempFolder.newFolder("empty-path").let { folder ->
            val zip = File(folder, "empty.zip")
            ZipOutputStream(zip.outputStream()).use { zos ->
                val entry = ZipEntry("")
                zos.putNextEntry(entry)
                zos.write("empty".toByteArray())
                zos.closeEntry()
            }
            zip
        }
        val targetDir = tempFolder.newFolder("extract-empty")

        val result = extractor.extractZipContainer(
            zipFile = zipFile,
            targetDir = targetDir,
            allowedEntries = setOf("")
        )

        assertTrue(result is SafeExtractResult.Failure)
        assertEquals(RuntimeInstallErrorCode.ARCHIVE_PATH_INVALID, (result as SafeExtractResult.Failure).code)
    }

    @Test
    fun extractZipContainer_rejectsDriveLetterWithColon() {
        val extractor = DefaultSafeArchiveExtractor()
        val zipFile = tempFolder.newFolder("colon-attack").let { folder ->
            val zip = File(folder, "colon.zip")
            ZipOutputStream(zip.outputStream()).use { zos ->
                val entry = ZipEntry("D:/evil/file.txt")
                zos.putNextEntry(entry)
                zos.write("evil".toByteArray())
                zos.closeEntry()
            }
            zip
        }
        val targetDir = tempFolder.newFolder("extract-colon")

        val result = extractor.extractZipContainer(
            zipFile = zipFile,
            targetDir = targetDir,
            allowedEntries = setOf("D:/evil/file.txt")
        )

        assertTrue(result is SafeExtractResult.Failure)
    }

    @Test
    fun isPathSafe_absolutePath_returnsFalse() {
        assertTrue(!com.amitia.amitia_app.runtime.install.PathValidator.isAbsolutePathSafe("/sdcard/evil"))
    }

    @Test
    fun isPathSafe_storagePath_returnsFalse() {
        assertTrue(!com.amitia.amitia_app.runtime.install.PathValidator.isAbsolutePathSafe("/storage/emulated/0/evil"))
    }

    @Test
    fun isPathSafe_traversalPath_returnsFalse() {
        assertTrue(!com.amitia.amitia_app.runtime.install.PathValidator.isAbsolutePathSafe("../../etc/passwd"))
    }

    @Test
    fun isPathSafe_relativePath_returnsTrue() {
        assertTrue(com.amitia.amitia_app.runtime.install.PathValidator.isAbsolutePathSafe("runtime/backend/server"))
    }

    @Test
    fun extractZipContainer_targetStaysWithinBoundary() {
        val extractor = DefaultSafeArchiveExtractor()
        val zipFile = tempFolder.newFolder("boundary-test").let { folder ->
            val zip = File(folder, "boundary.zip")
            ZipOutputStream(zip.outputStream()).use { zos ->
                val entry = ZipEntry("valid/file.txt")
                zos.putNextEntry(entry)
                zos.write("safe content".toByteArray())
                zos.closeEntry()
            }
            zip
        }
        val targetDir = tempFolder.newFolder("extract-boundary")

        val result = extractor.extractZipContainer(
            zipFile = zipFile,
            targetDir = targetDir,
            allowedEntries = setOf("valid/file.txt")
        )

        assertTrue(result is SafeExtractResult.Success)

        val extractedFile = File(targetDir, "file.txt")
        assertTrue(extractedFile.exists())
        val canonicalPath = extractedFile.canonicalPath
        assertTrue(
            "Extracted file must be within target directory",
            canonicalPath.startsWith(targetDir.canonicalPath)
        )
    }
}
