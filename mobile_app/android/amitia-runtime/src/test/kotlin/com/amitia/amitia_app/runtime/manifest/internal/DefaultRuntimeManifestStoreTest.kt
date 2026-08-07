package com.amitia.amitia_app.runtime.manifest.internal

import com.amitia.amitia_app.runtime.manifest.RuntimeManifest
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestComponent
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestErrorCode
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestInstallation
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestPaths
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestPayload
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestResult
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestTarget
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestVerification
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

class DefaultRuntimeManifestStoreTest {

    private fun sampleManifest(): RuntimeManifest = RuntimeManifest(
        schemaVersion = RuntimeManifest.SCHEMA_VERSION,
        runtimeVersion = "1.0.0",
        sourceCommit = "abc1234567890abc1234567890abc1234567890",
        packageId = "202608070001",
        packageSha256 = "a".repeat(64),
        target = RuntimeManifestTarget("android", "arm64-v8a", "proot", "linux", "arm64", "ubuntu", "24.0.4"),
        installation = RuntimeManifestInstallation("1.0.0", "r1", "rr1", "b".repeat(64)),
        payloads = listOf(RuntimeManifestPayload("p1", "rootfs", "c".repeat(64), 1024L)),
        components = listOf(RuntimeManifestComponent("runtime.backend", "1.0.0", "arm64", "backend", "amitia-server", "d".repeat(64), null, "package")),
        paths = RuntimeManifestPaths(
            "/data/.__runtime/rootfs", "/data/.__runtime/1.0.0",
            "/data/.__runtime/config", "/data/.__runtime/data",
            "/data/.__runtime/cache", "/data/.__runtime/logs", "/data/.__runtime/run",
            "/opt/amitia", "/etc/amitia", "/var/lib/amitia",
            "/var/cache/amitia", "/var/log/amitia", "/run/amitia"
        ),
        verification = RuntimeManifestVerification(true, true, true, true, true, true),
    )

    private fun createTempMetadataRoot(): File {
        val tmp = File.createTempFile("manifest_test", "")
        tmp.delete()
        tmp.mkdirs()
        return tmp
    }

    @Test
    fun firstWrite_succeeds() {
        val root = createTempMetadataRoot()
        try {
            val store = DefaultRuntimeManifestStore(root.absolutePath)
            val result = store.write(sampleManifest())
            assertTrue(result is RuntimeManifestResult.Success)
            assertTrue(File(root, "runtime-manifest.json").exists())
            assertTrue(File(root, "runtime-manifest.json.sha256").exists())
        } finally {
            root.deleteRecursively()
        }
    }

    @Test
    fun firstRead_noManifest_returnsNotFound() {
        val root = createTempMetadataRoot()
        try {
            val store = DefaultRuntimeManifestStore(root.absolutePath)
            val result = store.read()
            assertTrue(result is RuntimeManifestResult.Failure)
            assertEquals(RuntimeManifestErrorCode.MANIFEST_NOT_FOUND, (result as RuntimeManifestResult.Failure).error.code)
        } finally {
            root.deleteRecursively()
        }
    }

    @Test
    fun writeThenRead_roundTrip() {
        val root = createTempMetadataRoot()
        try {
            val store = DefaultRuntimeManifestStore(root.absolutePath)
            store.write(sampleManifest())
            val result = store.read()
            assertTrue(result is RuntimeManifestResult.Success)
            assertEquals("1.0.0", (result as RuntimeManifestResult.Success).manifest.runtimeVersion)
        } finally {
            root.deleteRecursively()
        }
    }

    @Test
    fun shaFile_validatesContent() {
        val root = createTempMetadataRoot()
        try {
            val store = DefaultRuntimeManifestStore(root.absolutePath)
            store.write(sampleManifest())

            val result = store.read()
            assertTrue(result is RuntimeManifestResult.Success)
        } finally {
            root.deleteRecursively()
        }
    }

    @Test
    fun corruptedSha_failsReading() {
        val root = createTempMetadataRoot()
        try {
            val store = DefaultRuntimeManifestStore(root.absolutePath)
            store.write(sampleManifest())

            File(root, "runtime-manifest.json.sha256").writeText("deadbeef  runtime-manifest.json\n", Charsets.UTF_8)

            val result = store.read()
            assertTrue(result is RuntimeManifestResult.Failure)
            assertEquals(RuntimeManifestErrorCode.MANIFEST_HASH_MISMATCH, (result as RuntimeManifestResult.Failure).error.code)
        } finally {
            root.deleteRecursively()
        }
    }

    @Test
    fun corruptedJson_failsReading() {
        val root = createTempMetadataRoot()
        try {
            val store = DefaultRuntimeManifestStore(root.absolutePath)
            store.write(sampleManifest())

            File(root, "runtime-manifest.json").writeText("{invalid json", Charsets.UTF_8)

            val result = store.read()
            assertTrue(result is RuntimeManifestResult.Failure)
        } finally {
            root.deleteRecursively()
        }
    }

    @Test
    fun delete_removesFiles() {
        val root = createTempMetadataRoot()
        try {
            val store = DefaultRuntimeManifestStore(root.absolutePath)
            store.write(sampleManifest())
            assertTrue(File(root, "runtime-manifest.json").exists())

            store.delete()

            assertFalse(File(root, "runtime-manifest.json").exists())
        } finally {
            root.deleteRecursively()
        }
    }

    @Test
    fun overwrite_preservesAtmosphere() {
        val root = createTempMetadataRoot()
        try {
            val store = DefaultRuntimeManifestStore(root.absolutePath)
            store.write(sampleManifest())

            val m2 = sampleManifest().copy(runtimeVersion = "1.0.1")
            val writeResult = store.write(m2)
            assertTrue(writeResult is RuntimeManifestResult.Success)

            val readResult = store.read()
            assertTrue(readResult is RuntimeManifestResult.Success)
            assertEquals("1.0.1", (readResult as RuntimeManifestResult.Success).manifest.runtimeVersion)
        } finally {
            root.deleteRecursively()
        }
    }
}
