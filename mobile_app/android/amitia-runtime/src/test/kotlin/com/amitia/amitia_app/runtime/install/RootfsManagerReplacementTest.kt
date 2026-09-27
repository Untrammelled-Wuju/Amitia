package com.amitia.amitia_app.runtime.install

import java.io.File
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder

class RootfsManagerReplacementTest {

    @get:Rule val folder = TemporaryFolder()

    @Test
    fun replacementCanRollBackAndCommit() {
        val control = folder.newFolder("control")
        val extractor = object : SafeArchiveExtractor {
            override fun extractZipContainer(
                zipFile: File,
                targetDir: File,
                allowedEntries: Set<String>,
            ): SafeExtractResult = SafeExtractResult.Success(ExtractionResult(0, 0))

            override fun extractTarXz(
                tarXzFile: File,
                targetDir: File,
                rootBoundary: String?,
            ): SafeExtractResult {
                targetDir.mkdirs()
                File(targetDir, "version").writeText(tarXzFile.name)
                return SafeExtractResult.Success(ExtractionResult(1, tarXzFile.length()))
            }
        }
        val manager = DefaultRootfsManager(control, extractor)
        val first = folder.newFile("first")
        val second = folder.newFile("second")
        val installed = manager.prepareRootfs(first, "old", "old-hash", false)
        assertTrue(installed is RootfsPrepareResult.NewlyInstalled)

        val blocked = manager.prepareRootfs(second, "new", "new-hash", false)
        assertTrue(blocked is RootfsPrepareResult.Conflict)

        val replaced = manager.prepareRootfs(second, "new", "new-hash", true)
        assertTrue(replaced is RootfsPrepareResult.Replaced)
        assertEquals("second", File(control, "rootfs/version").readText())
        manager.completePreparation(false)
        assertEquals("first", File(control, "rootfs/version").readText())
        assertEquals("old", manager.getInstalledRootfs()?.rootfsId)

        manager.prepareRootfs(second, "new", "new-hash", true)
        manager.completePreparation(true)
        assertEquals("second", File(control, "rootfs/version").readText())
        assertEquals("new", manager.getInstalledRootfs()?.rootfsId)
    }
}
