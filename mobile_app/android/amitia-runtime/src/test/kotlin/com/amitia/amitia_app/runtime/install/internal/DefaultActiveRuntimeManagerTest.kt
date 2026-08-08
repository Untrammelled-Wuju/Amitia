package com.amitia.amitia_app.runtime.install.internal

import com.amitia.amitia_app.runtime.install.ActiveRuntimeResult
import com.amitia.amitia_app.runtime.install.PathValidator
import com.amitia.amitia_app.runtime.install.RuntimeHostLayout
import com.amitia.amitia_app.runtime.install.RuntimeInstallErrorCode
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

class DefaultActiveRuntimeManagerTest {

    @get:Rule
    val tempFolder = TemporaryFolder()

    private lateinit var layout: FakeRuntimeHostLayout

    @Before
    fun setUp() {
        layout = FakeRuntimeHostLayout(
            tempFolder.newFolder("control"),
            tempFolder.newFolder("data"),
        )
    }

    @Test
    fun current_returnsNoActiveRuntime_whenNoMarker() {
        val manager = DefaultActiveRuntimeManager(layout)
        val result = manager.current()
        assertTrue(result is ActiveRuntimeResult.NoActiveRuntime)
    }

    @Test
    fun current_returnsActiveRuntime_whenValidMarker() {
        val versionDir = File(layout.versionsRoot, "1.0.0")
        versionDir.mkdirs()
        File(versionDir, "backend").mkdirs()
        writeActiveMarker("1.0.0")

        val manager = DefaultActiveRuntimeManager(layout)
        val result = manager.current()

        assertTrue(result is ActiveRuntimeResult.Active)
        assertEquals("1.0.0", (result as ActiveRuntimeResult.Active).info.version)
    }

    @Test
    fun current_returnsFailure_whenMarkerCorrupted() {
        layout.activeRuntimeFile.parentFile!!.mkdirs()
        layout.activeRuntimeFile.writeText("not json at all")

        val manager = DefaultActiveRuntimeManager(layout)
        val result = manager.current()

        assertTrue(result is ActiveRuntimeResult.Failure)
        assertEquals(
            RuntimeInstallErrorCode.ACTIVE_RUNTIME_METADATA_INVALID,
            (result as ActiveRuntimeResult.Failure).code
        )
    }

    @Test
    fun current_returnsFailure_whenVersionDirectoryMissing() {
        writeActiveMarker("2.0.0")

        val manager = DefaultActiveRuntimeManager(layout)
        val result = manager.current()

        assertTrue(result is ActiveRuntimeResult.Failure)
        assertEquals(
            RuntimeInstallErrorCode.ACTIVE_RUNTIME_MISSING,
            (result as ActiveRuntimeResult.Failure).code
        )
    }

    @Test
    fun current_doesNotScanVersions() {
        val version1 = File(layout.versionsRoot, "1.0.0")
        version1.mkdirs()
        val version2 = File(layout.versionsRoot, "2.0.0")
        version2.mkdirs()

        val manager = DefaultActiveRuntimeManager(layout)
        val result = manager.current()

        assertTrue(result is ActiveRuntimeResult.NoActiveRuntime)
    }

    @Test
    fun activate_succeeds_whenVersionExists() {
        val versionDir = File(layout.versionsRoot, "1.0.0")
        versionDir.mkdirs()

        val manager = DefaultActiveRuntimeManager(layout)
        val result = manager.activate("1.0.0")

        assertTrue(result is ActiveRuntimeResult.Active)
        assertTrue(layout.activeRuntimeFile.exists())

        val markerContent = layout.activeRuntimeFile.readText()
        assertTrue(markerContent.contains("1.0.0"))
    }

    @Test
    fun activate_fails_whenVersionMissing() {
        val manager = DefaultActiveRuntimeManager(layout)
        val result = manager.activate("1.0.0")

        assertTrue(result is ActiveRuntimeResult.Failure)
        assertEquals(
            RuntimeInstallErrorCode.ACTIVE_RUNTIME_MISSING,
            (result as ActiveRuntimeResult.Failure).code
        )
    }

    @Test
    fun activate_rejectsInvalidVersion() {
        val manager = DefaultActiveRuntimeManager(layout)

        val result1 = manager.activate("..")
        assertTrue(result1 is ActiveRuntimeResult.Failure)

        val result2 = manager.activate("/absolute")
        assertTrue(result2 is ActiveRuntimeResult.Failure)

        val result3 = manager.activate("")
        assertTrue(result3 is ActiveRuntimeResult.Failure)
    }

    @Test
    fun activate_doesNotStartRuntime() {
        val versionDir = File(layout.versionsRoot, "1.0.0")
        versionDir.mkdirs()

        val manager = DefaultActiveRuntimeManager(layout)
        val beforeActivation = layout.activeRuntimeFile.exists()

        assertFalse(beforeActivation)

        manager.activate("1.0.0")

        assertTrue(layout.activeRuntimeFile.exists())
    }

    @Test
    fun markerContent_isDeterministic() {
        val versionDir = File(layout.versionsRoot, "1.0.0")
        versionDir.mkdirs()

        val manager1 = DefaultActiveRuntimeManager(layout)
        manager1.activate("1.0.0")
        val content1 = layout.activeRuntimeFile.readText()

        layout.activeRuntimeFile.delete()

        val manager2 = DefaultActiveRuntimeManager(layout)
        manager2.activate("1.0.0")
        val content2 = layout.activeRuntimeFile.readText()

        assertEquals(content1, content2)
    }

    @Test
    fun markerDoesNotContainPath() {
        val versionDir = File(layout.versionsRoot, "1.0.0")
        versionDir.mkdirs()

        val manager = DefaultActiveRuntimeManager(layout)
        manager.activate("1.0.0")

        val content = layout.activeRuntimeFile.readText()
        assertFalse(content.contains(layout.controlRoot.absolutePath))
        assertFalse(content.contains("/data/user/"))
    }

    private fun writeActiveMarker(version: String) {
        val markerFile = layout.activeRuntimeFile
        markerFile.parentFile?.mkdirs()
        markerFile.writeText(
            """
            {
              "schemaVersion": 1,
              "runtimeVersion": "$version"
            }
            """.trimIndent()
        )
    }
}

internal class FakeRuntimeHostLayout(
    override val controlRoot: File,
    private val dataBaseDir: File,
) : RuntimeHostLayout {

    private val dataDir = File(dataBaseDir, "amitia")

    override val rootfsRoot: File = File(controlRoot, RuntimeHostLayout.DIR_ROOTFS)
    override val versionsRoot: File = File(controlRoot, RuntimeHostLayout.DIR_VERSIONS)
    override val stagingRoot: File = File(controlRoot, RuntimeHostLayout.DIR_STAGING)
    override val metadataRoot: File = File(controlRoot, RuntimeHostLayout.DIR_METADATA)
    override val transactionsRoot: File = File(controlRoot, RuntimeHostLayout.DIR_TRANSACTIONS)
    override val locksRoot: File = File(controlRoot, RuntimeHostLayout.DIR_LOCKS)

    override val configRoot: File = File(dataDir, RuntimeHostLayout.DIR_CONFIG)
    override val dataRoot: File = File(dataDir, RuntimeHostLayout.DIR_DATA)
    override val cacheRoot: File = File(dataDir, RuntimeHostLayout.DIR_CACHE)
    override val logRoot: File = File(dataDir, RuntimeHostLayout.DIR_LOGS)
    override val runRoot: File = File(dataDir, RuntimeHostLayout.DIR_RUN)

    override fun runtimeVersionRoot(version: String): File {
        if (!PathValidator.isValidRuntimeVersion(version)) {
            throw IllegalArgumentException("invalid version: $version")
        }
        return File(versionsRoot, version)
    }

    override fun installReceiptFile(version: String): File {
        return File(File(metadataRoot, RuntimeHostLayout.DIR_INSTALL_RECEIPTS), "$version.json")
    }

    override val activeRuntimeFile: File
        get() = File(metadataRoot, RuntimeHostLayout.FILE_ACTIVE_RUNTIME)

    override val runtimeManifestFile: File
        get() = File(metadataRoot, RuntimeHostLayout.FILE_RUNTIME_MANIFEST)

    override val runtimeManifestShaFile: File
        get() = File(metadataRoot, RuntimeHostLayout.FILE_RUNTIME_MANIFEST_SHA)
}
