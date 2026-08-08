package com.amitia.amitia_app.runtime.install

import com.amitia.amitia_app.runtime.install.internal.DefaultRuntimeHostLayout
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

class RuntimeHostLayoutTest {

    @get:Rule
    val tempFolder = TemporaryFolder()

    private fun createLayout(): DefaultRuntimeHostLayout {
        val controlBase = tempFolder.newFolder("noBackupFiles")
        val dataBase = tempFolder.newFolder("files")
        return DefaultRuntimeHostLayout(controlBase, dataBase)
    }

    @Test
    fun controlRoot_pointsToNoBackupFilesDirSubdirectory() {
        val layout = createLayout()
        assertTrue(layout.controlRoot.absolutePath.contains("amitia-runtime"))
    }

    @Test
    fun allControlRoots_areWithinControlRoot() {
        val layout = createLayout()
        val controlRoots = layout.allControlRoots()
        for (root in controlRoots) {
            assertTrue(
                "Control root ${root.absolutePath} must be within ${layout.controlRoot}",
                root.absolutePath.startsWith(layout.controlRoot.absolutePath)
            )
        }
    }

    @Test
    fun allDataRoots_areNotWithinControlRoot() {
        val layout = createLayout()
        val dataRoots = layout.allDataRoots()
        for (root in dataRoots) {
            assertFalse(
                "Data root ${root.absolutePath} must not be within control root",
                root.absolutePath.startsWith(layout.controlRoot.absolutePath)
            )
        }
    }

    @Test
    fun runtimeVersionRoot_returnsVersionedPath() {
        val layout = createLayout()
        val versionRoot = layout.runtimeVersionRoot("1.0.0")
        assertTrue(versionRoot.absolutePath.contains("versions/1.0.0"))
    }

    @Test
    fun runtimeVersionRoot_rejectsInvalidVersion() {
        val layout = createLayout()
        try {
            layout.runtimeVersionRoot("../etc/passwd")
            fail("Expected IllegalArgumentException")
        } catch (_: IllegalArgumentException) {
        }
        try {
            layout.runtimeVersionRoot("")
            fail("Expected IllegalArgumentException")
        } catch (_: IllegalArgumentException) {
        }
        try {
            layout.runtimeVersionRoot("/absolute/path")
            fail("Expected IllegalArgumentException")
        } catch (_: IllegalArgumentException) {
        }
    }

    @Test
    fun installReceiptFile_returnsCorrectPath() {
        val layout = createLayout()
        val receiptFile = layout.installReceiptFile("1.0.0")
        assertTrue(receiptFile.absolutePath.contains("install-receipts/1.0.0.json"))
    }

    @Test
    fun activeRuntimeFile_returnsCorrectPath() {
        val layout = createLayout()
        val activeFile = layout.activeRuntimeFile
        assertTrue(activeFile.absolutePath.contains("active-runtime.json"))
        assertTrue(activeFile.absolutePath.startsWith(layout.metadataRoot.absolutePath))
    }

    @Test
    fun runtimeManifestFile_returnsCorrectPath() {
        val layout = createLayout()
        val manifestFile = layout.runtimeManifestFile
        assertTrue(manifestFile.absolutePath.contains("runtime-manifest.json"))
    }

    @Test
    fun runtimeManifestShaFile_returnsCorrectPath() {
        val layout = createLayout()
        val shaFile = layout.runtimeManifestShaFile
        assertTrue(shaFile.absolutePath.contains("runtime-manifest.json.sha256"))
    }

    @Test
    fun securityDir_returnsCorrectPath() {
        val layout = createLayout()
        val securityDir = layout.securityDir()
        assertTrue(securityDir.absolutePath.contains("security"))
        assertTrue(securityDir.absolutePath.startsWith(layout.dataRoot.absolutePath))
    }

    @Test
    fun localTokenFile_returnsCorrectPath() {
        val layout = createLayout()
        val tokenFile = layout.localTokenFile()
        assertTrue(tokenFile.absolutePath.contains("security/local-token"))
    }

    @Test
    fun qdrantDataDir_returnsCorrectPath() {
        val layout = createLayout()
        val qdrantDir = layout.qdrantDataDir()
        assertTrue(qdrantDir.absolutePath.contains("providers/qdrant"))
    }

    @Test
    fun nodeDataDir_returnsCorrectPath() {
        val layout = createLayout()
        val nodeDir = layout.nodeDataDir()
        assertTrue(nodeDir.absolutePath.contains("/node"))
    }

    @Test
    fun nodeCacheDir_returnsCorrectPath() {
        val layout = createLayout()
        val nodeCacheDir = layout.nodeCacheDir()
        assertTrue(nodeCacheDir.absolutePath.contains("cache/node"))
    }

    @Test
    fun paths_areStableAcrossInstances() {
        val layout1 = createLayout()
        val layout2 = createLayout()
        assertEquals(layout1.controlRoot.absolutePath, layout2.controlRoot.absolutePath)
        assertEquals(layout1.runtimeVersionRoot("1.0.0").absolutePath, layout2.runtimeVersionRoot("1.0.0").absolutePath)
    }

    private fun fail(message: String) {
        throw AssertionError(message)
    }
}
