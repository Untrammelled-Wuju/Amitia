package com.amitia.amitia_app.runtime.install

import com.amitia.amitia_app.runtime.install.internal.DefaultActiveRuntimeManager
import com.amitia.amitia_app.runtime.install.internal.DefaultRuntimeHostLayout
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger

class RuntimeHostLayoutAndActiveManagerTest {

    @get:Rule
    val tempFolder = TemporaryFolder()

    private lateinit var layout: DefaultRuntimeHostLayout

    @Before
    fun setUp() {
        layout = DefaultRuntimeHostLayout(
            controlBaseDir = tempFolder.newFolder("control"),
            dataBaseDir = tempFolder.newFolder("data"),
        )
    }

    private fun unixPath(file: File): String = file.absolutePath.replace('\\', '/')

    @Test
    fun hostLayout_isDeterministic_samePathsForSameInput() {
        val control1 = tempFolder.newFolder("ctrl1")
        val data1 = tempFolder.newFolder("data1")
        val layout1 = DefaultRuntimeHostLayout(control1, data1)
        val layout2 = DefaultRuntimeHostLayout(control1, data1)

        assertEquals(unixPath(layout1.controlRoot), unixPath(layout2.controlRoot))
        assertEquals(unixPath(layout1.rootfsRoot), unixPath(layout2.rootfsRoot))
        assertEquals(unixPath(layout1.versionsRoot), unixPath(layout2.versionsRoot))
        assertEquals(unixPath(layout1.stagingRoot), unixPath(layout2.stagingRoot))
        assertEquals(unixPath(layout1.metadataRoot), unixPath(layout2.metadataRoot))
        assertEquals(unixPath(layout1.configRoot), unixPath(layout2.configRoot))
        assertEquals(unixPath(layout1.dataRoot), unixPath(layout2.dataRoot))
        assertEquals(unixPath(layout1.cacheRoot), unixPath(layout2.cacheRoot))
        assertEquals(unixPath(layout1.logRoot), unixPath(layout2.logRoot))
        assertEquals(unixPath(layout1.runRoot), unixPath(layout2.runRoot))
        assertEquals(unixPath(layout1.homeRoot), unixPath(layout2.homeRoot))
        assertEquals(
            unixPath(layout1.runtimeVersionRoot("1.0.0")),
            unixPath(layout2.runtimeVersionRoot("1.0.0"))
        )
    }

    @Test
    fun allControlRoots_withinControlRoot() {
        val controlRoots = layout.allControlRoots()
        for (root in controlRoots) {
            assertTrue(
                "Control path ${root.absolutePath} must be inside controlRoot",
                unixPath(root).startsWith(unixPath(layout.controlRoot))
            )
        }
    }

    @Test
    fun allDataRoots_notInsideControlRoot() {
        val dataRoots = layout.allDataRoots()
        for (root in dataRoots) {
            assertFalse(
                "Data path ${root.absolutePath} must NOT be inside controlRoot",
                unixPath(root).startsWith(unixPath(layout.controlRoot))
            )
        }
    }

    @Test
    fun versionsRoot_notInsideDataRoot() {
        assertFalse(
            "versionsRoot must not be inside dataRoot",
            unixPath(layout.versionsRoot).startsWith(unixPath(layout.dataRoot))
        )
    }

    @Test
    fun dataRoot_notInsideVersionsRoot() {
        assertFalse(
            "dataRoot must not be inside versionsRoot",
            unixPath(layout.dataRoot).startsWith(unixPath(layout.versionsRoot))
        )
    }

    @Test
    fun stagingAndVersions_shareSameParent() {
        assertEquals(
            "staging and versions must share same parent for atomic rename",
            unixPath(layout.stagingRoot.parentFile!!),
            unixPath(layout.versionsRoot.parentFile!!)
        )
    }

    @Test
    fun homeRoot_inPersistentDataNotInVersions() {
        assertFalse(
            "homeRoot must not be inside versionsRoot",
            unixPath(layout.homeRoot).startsWith(unixPath(layout.versionsRoot))
        )
        assertTrue(
            "homeRoot should be under data directory",
            unixPath(layout.homeRoot).contains("/amitia/")
        )
    }

    @Test
    fun versionPathTraversal_rejected() {
        val traversals = listOf(
            "../evil",
            "..\\evil",
            "../../etc/passwd",
            "foo/../bar",
            "./../../escape",
        )
        for (version in traversals) {
            try {
                layout.runtimeVersionRoot(version)
                throw AssertionError("Should have rejected traversal: $version")
            } catch (_: IllegalArgumentException) {
            }
        }
    }

    @Test
    fun absoluteVersionPath_rejected() {
        val absoluteVersions = listOf(
            "/data/user/0/com.amitia.amitia_app/files/runtime",
            "/etc/passwd",
            "\\windows\\system32",
        )
        for (version in absoluteVersions) {
            try {
                layout.runtimeVersionRoot(version)
                throw AssertionError("Should have rejected absolute path: $version")
            } catch (_: IllegalArgumentException) {
            }
        }
    }

    @Test
    fun emptyVersion_rejected() {
        try {
            layout.runtimeVersionRoot("")
            throw AssertionError("Should have rejected empty version")
        } catch (_: IllegalArgumentException) {
        }
    }

    @Test
    fun noExternalStorage_inLayoutPaths() {
        val externalPrefixes = listOf("/sdcard", "/storage/emulated")
        val allRoots = layout.allControlRoots() + layout.allDataRoots() + listOf(layout.activeRuntimeFile)
        for (file in allRoots) {
            val path = unixPath(file)
            for (prefix in externalPrefixes) {
                assertFalse(
                    "Runtime host path must not be in external storage: $path",
                    path.startsWith(prefix)
                )
            }
        }
    }

    @Test
    fun activate_whenNoMarker_returnsNoActiveRuntime() {
        val manager = DefaultActiveRuntimeManager(layout)
        val result = manager.current()
        assertTrue("Should return NoActiveRuntime", result is ActiveRuntimeResult.NoActiveRuntime)
    }

    @Test
    fun activate_validMetadata_returnsActiveRuntime() {
        val versionDir = layout.runtimeVersionRoot("1.0.0")
        versionDir.mkdirs()

        val manager = DefaultActiveRuntimeManager(layout)
        val result = manager.activate("1.0.0")

        assertTrue("Should return Active", result is ActiveRuntimeResult.Active)
        assertEquals("1.0.0", (result as ActiveRuntimeResult.Active).info.version)
    }

    @Test
    fun activate_metadataMissingVersionDir_returnsFailure() {
        val manager = DefaultActiveRuntimeManager(layout)
        val result = manager.activate("nonexistent")

        assertTrue("Should return Failure", result is ActiveRuntimeResult.Failure)
        assertEquals(
            RuntimeInstallErrorCode.ACTIVE_RUNTIME_MISSING,
            (result as ActiveRuntimeResult.Failure).code
        )
    }

    @Test
    fun activeMetadata_doesNotContainAbsolutePath() {
        val versionDir = layout.runtimeVersionRoot("1.0.0")
        versionDir.mkdirs()

        val manager = DefaultActiveRuntimeManager(layout)
        manager.activate("1.0.0")

        val content = layout.activeRuntimeFile.readText()
        assertFalse(
            "Active metadata must not contain control root absolute path",
            content.contains(layout.controlRoot.absolutePath)
        )
        assertFalse(
            "Active metadata must not contain /data/user/",
            content.contains("/data/user/")
        )
        assertFalse(
            "Active metadata must not contain 'READY'",
            content.contains("READY")
        )
        assertFalse(
            "Active metadata must not contain 'PID'",
            content.contains("PID")
        )
    }

    @Test
    fun activeMetadata_containsOnlySchemaAndVersion() {
        val versionDir = layout.runtimeVersionRoot("1.0.0")
        versionDir.mkdirs()

        val manager = DefaultActiveRuntimeManager(layout)
        manager.activate("1.0.0")

        val content = layout.activeRuntimeFile.readText()
        assertTrue(content.contains("\"schemaVersion\""))
        assertTrue(content.contains("\"runtimeVersion\""))
        assertTrue(content.contains("1.0.0"))
    }

    @Test
    fun activeMetadata_unsupportedSchema_failsFast() {
        layout.metadataRoot.mkdirs()
        layout.activeRuntimeFile.writeText(
            """
            {
              "schemaVersion": 999,
              "runtimeVersion": "1.0.0"
            }
            """.trimIndent()
        )

        val manager = DefaultActiveRuntimeManager(layout)
        val result = manager.current()

        assertTrue("Should fail with invalid metadata", result is ActiveRuntimeResult.Failure)
        assertEquals(
            RuntimeInstallErrorCode.ACTIVE_RUNTIME_METADATA_INVALID,
            (result as ActiveRuntimeResult.Failure).code
        )
    }

    @Test
    fun activation_cannotActivateWithInvalidVersion() {
        val manager = DefaultActiveRuntimeManager(layout)
        val result = manager.activate("../evil")

        assertTrue(result is ActiveRuntimeResult.Failure)
        assertEquals(
            RuntimeInstallErrorCode.RUNTIME_VERSION_INVALID,
            (result as ActiveRuntimeResult.Failure).code
        )
    }

    @Test
    fun concurrent_activations_doNotCorruptMetadata() {
        val version1Dir = layout.runtimeVersionRoot("1.0.0")
        version1Dir.mkdirs()
        val version2Dir = layout.runtimeVersionRoot("2.0.0")
        version2Dir.mkdirs()

        val threadCount = 8
        val iterations = 10
        val latch = CountDownLatch(threadCount)
        val executor = Executors.newFixedThreadPool(threadCount)
        val failures = AtomicInteger(0)

        repeat(threadCount) { threadIndex ->
            executor.execute {
                try {
                    repeat(iterations) {
                        val manager = DefaultActiveRuntimeManager(layout)
                        val version = if (threadIndex % 2 == 0) "1.0.0" else "2.0.0"
                        manager.activate(version)
                    }
                } catch (_: Exception) {
                    failures.incrementAndGet()
                } finally {
                    latch.countDown()
                }
            }
        }

        latch.await(15, TimeUnit.SECONDS)
        executor.shutdown()

        assertEquals("Concurrent activations should not throw exceptions", 0, failures.get())

        val content = layout.activeRuntimeFile.readText()
        assertTrue(
            "Active metadata must remain valid JSON after concurrent activations",
            content.contains("\"runtimeVersion\"")
        )
        assertTrue(
            "Active metadata must reference a valid version",
            content.contains("1.0.0") || content.contains("2.0.0")
        )
    }

    @Test
    fun repeated_activation_isIdempotent() {
        val versionDir = layout.runtimeVersionRoot("1.0.0")
        versionDir.mkdirs()

        val manager = DefaultActiveRuntimeManager(layout)
        manager.activate("1.0.0")
        val content1 = layout.activeRuntimeFile.readText()

        manager.activate("1.0.0")
        val content2 = layout.activeRuntimeFile.readText()

        assertEquals(content1, content2)
    }

    @Test
    fun atomic_activation_interruptedWrite_preservesOldMetadata() {
        val versionDir1 = layout.runtimeVersionRoot("1.0.0")
        versionDir1.mkdirs()
        val versionDir2 = layout.runtimeVersionRoot("2.0.0")
        versionDir2.mkdirs()

        val manager = DefaultActiveRuntimeManager(layout)
        manager.activate("1.0.0")

        val originalContent = layout.activeRuntimeFile.readText()
        assertTrue(originalContent.contains("1.0.0"))

        manager.activate("2.0.0")

        val newContent = layout.activeRuntimeFile.readText()
        assertTrue("Must have new version", newContent.contains("2.0.0"))
        assertFalse("Must not contain old version", newContent.contains("1.0.0"))
    }

    @Test
    fun versionSwitch_preservesPersistentDataRoots() {
        val configMarker = File(layout.configRoot, "marker.conf")
        configMarker.parentFile.mkdirs()
        configMarker.writeText("config-value")

        val dataMarker = File(layout.dataRoot, "business.db")
        dataMarker.parentFile.mkdirs()
        dataMarker.writeText("data-content")

        val cacheMarker = File(layout.cacheRoot, "npm-cache")
        cacheMarker.parentFile.mkdirs()
        cacheMarker.writeText("cache-content")

        val logMarker = File(layout.logRoot, "app.log")
        logMarker.parentFile.mkdirs()
        logMarker.writeText("log-entry")

        layout.runRoot.mkdirs()
        val runMarker = File(layout.runRoot, "pid")
        runMarker.writeText("12345")

        layout.homeRoot.mkdirs()
        val homeMarker = File(layout.homeRoot, ".profile")
        homeMarker.writeText("home-config")

        val version1Dir = layout.runtimeVersionRoot("1.0.0")
        version1Dir.mkdirs()
        val version2Dir = layout.runtimeVersionRoot("2.0.0")
        version2Dir.mkdirs()

        val manager = DefaultActiveRuntimeManager(layout)
        manager.activate("1.0.0")
        manager.activate("2.0.0")

        assertTrue("Config must persist after version switch", configMarker.exists())
        assertEquals("config-value", configMarker.readText())

        assertTrue("Data must persist after version switch", dataMarker.exists())
        assertEquals("data-content", dataMarker.readText())

        assertTrue("Cache must persist after version switch", cacheMarker.exists())
        assertEquals("cache-content", cacheMarker.readText())

        assertTrue("Logs must persist after version switch", logMarker.exists())
        assertEquals("log-entry", logMarker.readText())

        assertTrue("Run dir must persist after version switch", runMarker.exists())
        assertEquals("12345", runMarker.readText())

        assertTrue("Home must persist after version switch", homeMarker.exists())
        assertEquals("home-config", homeMarker.readText())
    }

    @Test
    fun versionSwitch_doesNotModifyVersionDirectories() {
        val version1Dir = layout.runtimeVersionRoot("1.0.0")
        version1Dir.mkdirs()
        File(version1Dir, "program.bin").writeText("v1-binary")

        val version2Dir = layout.runtimeVersionRoot("2.0.0")
        version2Dir.mkdirs()
        File(version2Dir, "program.bin").writeText("v2-binary")

        val manager = DefaultActiveRuntimeManager(layout)
        manager.activate("2.0.0")
        manager.activate("1.0.0")

        assertEquals("v1-binary", File(version1Dir, "program.bin").readText())
        assertEquals("v2-binary", File(version2Dir, "program.bin").readText())
    }

    @Test
    fun missingMetadata_noFallbackSingleVersion() {
        val versionDir = File(layout.versionsRoot, "1.0.0")
        versionDir.mkdirs()

        val manager = DefaultActiveRuntimeManager(layout)
        val result = manager.current()

        assertTrue(
            "With no active metadata, must NOT fallback to single installed version",
            result is ActiveRuntimeResult.NoActiveRuntime
        )
    }

    @Test
    fun missingMetadata_noFallbackInvalidMetadata() {
        layout.metadataRoot.mkdirs()
        layout.activeRuntimeFile.writeText("""{"runtimeVersion": "latest"}""")

        val versionDir = File(layout.versionsRoot, "1.0.0")
        versionDir.mkdirs()

        val manager = DefaultActiveRuntimeManager(layout)
        val result = manager.current()

        assertTrue(
            "With invalid metadata, must NOT fallback to installed versions",
            result is ActiveRuntimeResult.Failure
        )
    }

    @Test
    fun activeManager_doesNotScanVersionsForCurrent() {
        val v99 = File(layout.versionsRoot, "99.0.0")
        v99.mkdirs()

        val manager = DefaultActiveRuntimeManager(layout)
        val result = manager.current()

        assertTrue(
            "Must NOT scan versions directory to infer active version",
            result is ActiveRuntimeResult.NoActiveRuntime
        )
    }

    @Test
    fun compositionIdentity_sameLayoutUsedByInstallerAndManager() {
        val sharedCtrl = tempFolder.newFolder("shared-ctrl")
        val sharedData = tempFolder.newFolder("shared-data")
        val installerLayout = DefaultRuntimeHostLayout(
            controlBaseDir = sharedCtrl,
            dataBaseDir = sharedData,
        )
        val managerLayout = DefaultRuntimeHostLayout(
            controlBaseDir = sharedCtrl,
            dataBaseDir = sharedData,
        )

        assertEquals(
            unixPath(installerLayout.versionsRoot),
            unixPath(managerLayout.versionsRoot)
        )
        assertEquals(
            unixPath(installerLayout.activeRuntimeFile),
            unixPath(managerLayout.activeRuntimeFile)
        )
    }
}

