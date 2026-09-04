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

    private fun unixPath(file: File): String = file.absolutePath.replace('\\', '/')

    @Test
    fun controlRoot_pointsToNoBackupFilesDirSubdirectory() {
        val layout = createLayout()
        assertTrue(unixPath(layout.controlRoot).contains("amitia-runtime"))
    }

    @Test
    fun allControlRoots_areWithinControlRoot() {
        val layout = createLayout()
        val controlRoots = layout.allControlRoots()
        for (root in controlRoots) {
            assertTrue(
                "Control root ${root.absolutePath} must be within ${layout.controlRoot}",
                unixPath(root).startsWith(unixPath(layout.controlRoot))
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
                unixPath(root).startsWith(unixPath(layout.controlRoot))
            )
        }
    }

    @Test
    fun runtimeVersionRoot_returnsVersionedPath() {
        val layout = createLayout()
        val versionRoot = layout.runtimeVersionRoot("1.0.0")
        assertTrue(unixPath(versionRoot).contains("versions/1.0.0"))
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
        assertTrue(unixPath(receiptFile).contains("install-receipts/1.0.0.json"))
    }

    @Test
    fun activeRuntimeFile_returnsCorrectPath() {
        val layout = createLayout()
        val activeFile = layout.activeRuntimeFile
        assertTrue(unixPath(activeFile).contains("active-runtime.json"))
        assertTrue(unixPath(activeFile).startsWith(unixPath(layout.metadataRoot)))
    }

    @Test
    fun runtimeManifestFile_returnsCorrectPath() {
        val layout = createLayout()
        val manifestFile = layout.runtimeManifestFile
        assertTrue(unixPath(manifestFile).contains("runtime-manifest.json"))
    }

    @Test
    fun runtimeManifestShaFile_returnsCorrectPath() {
        val layout = createLayout()
        val shaFile = layout.runtimeManifestShaFile
        assertTrue(unixPath(shaFile).contains("runtime-manifest.json.sha256"))
    }

    @Test
    fun securityDir_returnsCorrectPath() {
        val layout = createLayout()
        val securityDir = layout.securityDir()
        assertTrue(unixPath(securityDir).contains("security"))
        assertTrue(unixPath(securityDir).startsWith(unixPath(layout.dataRoot)))
    }

    @Test
    fun localTokenFile_returnsCorrectPath() {
        val layout = createLayout()
        val tokenFile = layout.localTokenFile()
        assertTrue(unixPath(tokenFile).contains("security/local-token"))
    }

    @Test
    fun qdrantDataDir_returnsCorrectPath() {
        val layout = createLayout()
        val qdrantDir = layout.qdrantDataDir()
        assertTrue(unixPath(qdrantDir).contains("providers/qdrant"))
    }

    @Test
    fun nodeDataDir_returnsCorrectPath() {
        val layout = createLayout()
        val nodeDir = layout.nodeDataDir()
        assertTrue(unixPath(nodeDir).contains("/node"))
    }

    @Test
    fun nodeCacheDir_returnsCorrectPath() {
        val layout = createLayout()
        val nodeCacheDir = layout.nodeCacheDir()
        assertTrue(unixPath(nodeCacheDir).contains("cache/node"))
    }

    @Test
    fun homeRoot_returnsCorrectPath() {
        val layout = createLayout()
        assertTrue(unixPath(layout.homeRoot).contains("/home"))
        assertFalse(unixPath(layout.homeRoot).startsWith(unixPath(layout.controlRoot)))
        assertTrue(unixPath(layout.homeRoot).startsWith(unixPath(layout.dataRoot.parentFile!!)))
    }

    @Test
    fun homeRoot_notInsideVersionsRoot() {
        val layout = createLayout()
        assertFalse(
            "homeRoot must not be inside versions root",
            unixPath(layout.homeRoot).startsWith(unixPath(layout.versionsRoot))
        )
    }

    @Test
    fun versionsRoot_notInsideDataRoot() {
        val layout = createLayout()
        assertFalse(
            "versionsRoot must not be inside data root",
            unixPath(layout.versionsRoot).startsWith(unixPath(layout.dataRoot))
        )
    }

    @Test
    fun stagingAndVersions_sameParentDirectory() {
        val layout = createLayout()
        assertEquals(
            "staging and versions must share the same parent (runtime-control) for atomic rename",
            unixPath(layout.stagingRoot.parentFile!!),
            unixPath(layout.versionsRoot.parentFile!!)
        )
    }

    @Test
    fun runtimeVersionRoot_rejectsBackslashVersion() {
        val layout = createLayout()
        try {
            layout.runtimeVersionRoot("evil\\path")
            fail("Expected IllegalArgumentException")
        } catch (_: IllegalArgumentException) {
        }
    }

    @Test
    fun runtimeVersionRoot_rejectsColonVersion() {
        val layout = createLayout()
        try {
            layout.runtimeVersionRoot("C:\\windows")
            fail("Expected IllegalArgumentException")
        } catch (_: IllegalArgumentException) {
        }
    }

    @Test
    fun runtimeVersionRoot_rejectsNullBytes() {
        val layout = createLayout()
        try {
            layout.runtimeVersionRoot("1.0\u0000evil")
            fail("Expected IllegalArgumentException")
        } catch (_: IllegalArgumentException) {
        }
    }

    @Test
    fun homeDir_returnsCorrectPath() {
        val layout = createLayout()
        assertTrue(unixPath(layout.homeRoot).contains("home"))
    }

    @Test
    fun paths_areStableAcrossInstances() {
        val controlBase = tempFolder.newFolder("shared-noBackup")
        val dataBase = tempFolder.newFolder("shared-files")
        val layout1 = DefaultRuntimeHostLayout(controlBase, dataBase)
        val layout2 = DefaultRuntimeHostLayout(controlBase, dataBase)
        assertEquals(unixPath(layout1.controlRoot), unixPath(layout2.controlRoot))
        assertEquals(unixPath(layout1.runtimeVersionRoot("1.0.0")), unixPath(layout2.runtimeVersionRoot("1.0.0")))
        assertEquals(unixPath(layout1.homeRoot), unixPath(layout2.homeRoot))
        assertEquals(unixPath(layout1.dataRoot), unixPath(layout2.dataRoot))
    }

    private fun fail(message: String) {
        throw AssertionError(message)
    }
}
