package com.amitia.amitia_app.runtime.manifest.internal

import com.amitia.amitia_app.runtime.manifest.RuntimeManifest
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestComponent
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestInstallation
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestPaths
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestPayload
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestTarget
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestVerification
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test

class RuntimeManifestJsonTest {

    private fun sampleManifest(): RuntimeManifest = RuntimeManifest(
        schemaVersion = RuntimeManifest.SCHEMA_VERSION,
        runtimeVersion = "1.0.0",
        sourceCommit = "abc1234567890abc1234567890abc1234567890",
        packageId = "202608070001",
        packageSha256 = "a".repeat(64),
        target = RuntimeManifestTarget(
            hostPlatform = "android",
            hostAbi = "arm64-v8a",
            runtimeKind = "proot",
            guestPlatform = "linux",
            guestArchitecture = "arm64",
            distribution = "ubuntu",
            distributionRelease = "24.0.4",
        ),
        installation = RuntimeManifestInstallation(
            activeVersion = "1.0.0",
            rootfsId = "202608070001-rootfs",
            runtimeRootId = "202608070001-runtime",
            runtimeRootTreeSha256 = "b".repeat(64),
        ),
        payloads = listOf(
            RuntimeManifestPayload("202608070001-rootfs", "rootfs", "c".repeat(64), 1024L),
            RuntimeManifestPayload("202608070001-runtime", "runtime", "d".repeat(64), 2048L),
        ),
        components = listOf(
            RuntimeManifestComponent(
                id = "runtime.backend",
                version = "1.0.0",
                architecture = "arm64",
                root = "backend",
                entry = "amitia-server",
                sha256 = "e".repeat(64),
                treeSha256 = null,
                source = "package",
            ),
            RuntimeManifestComponent(
                id = "runtime.proot",
                version = null,
                architecture = null,
                root = "",
                entry = null,
                sha256 = null,
                treeSha256 = null,
                source = "android-native",
            ),
        ),
        paths = RuntimeManifestPaths(
            rootfsHostPath = "/data/.__runtime/rootfs",
            runtimeRootHostPath = "/data/.__runtime/1.0.0",
            configHostPath = "/data/.__runtime/config",
            dataHostPath = "/data/.__runtime/data",
            cacheHostPath = "/data/.__runtime/cache",
            logHostPath = "/data/.__runtime/logs",
            runHostPath = "/data/.__runtime/run",
            guestRuntimeRoot = "/opt/amitia",
            guestConfigRoot = "/etc/amitia",
            guestDataRoot = "/var/lib/amitia",
            guestCacheRoot = "/var/cache/amitia",
            guestLogRoot = "/var/log/amitia",
            guestRunRoot = "/run/amitia",
        ),
        verification = RuntimeManifestVerification(
            packageVerified = true,
            rootfsVerified = true,
            runtimeRootVerified = true,
            componentsVerified = true,
            guestLayoutVerified = true,
            mountContractVerified = true,
        ),
    )

    @Test
    fun serializeContainsAllRequiredFields() {
        val json = RuntimeManifestJson.serialize(sampleManifest())
        assertTrue(json.contains("\"schemaVersion\""))
        assertTrue(json.contains("\"runtimeVersion\""))
        assertTrue(json.contains("\"sourceCommit\""))
        assertTrue(json.contains("\"packageId\""))
        assertTrue(json.contains("\"packageSha256\""))
        assertTrue(json.contains("\"target\""))
        assertTrue(json.contains("\"installation\""))
        assertTrue(json.contains("\"payloads\""))
        assertTrue(json.contains("\"components\""))
        assertTrue(json.contains("\"paths\""))
        assertTrue(json.contains("\"verification\""))
    }

    @Test
    fun roundTrip_preservesFields() {
        val json = RuntimeManifestJson.serialize(sampleManifest())
        val restored = RuntimeManifestJson.deserialize(json)
        assertEquals(sampleManifest().runtimeVersion, restored.runtimeVersion)
        assertEquals(sampleManifest().sourceCommit, restored.sourceCommit)
        assertEquals(sampleManifest().packageSha256, restored.packageSha256)
        assertEquals(sampleManifest().target.hostPlatform, restored.target.hostPlatform)
        assertEquals(sampleManifest().installation.activeVersion, restored.installation.activeVersion)
        assertEquals(sampleManifest().payloads.size, restored.payloads.size)
        assertEquals(sampleManifest().payloads[0].id, restored.payloads[0].id)
        assertEquals(sampleManifest().components.size, restored.components.size)
        assertEquals(sampleManifest().components[0].id, restored.components[0].id)
        assertEquals(sampleManifest().paths.rootfsHostPath, restored.paths.rootfsHostPath)
        assertEquals(sampleManifest().verification.allVerified(), restored.verification.allVerified())
    }

    @Test
    fun serializeTwoTimes_sameJson() {
        val json1 = RuntimeManifestJson.serialize(sampleManifest())
        val json2 = RuntimeManifestJson.serialize(sampleManifest())
        assertEquals(json1, json2)
    }

    @Test
    fun json_keysSorted() {
        val json = RuntimeManifestJson.serialize(sampleManifest())
        val schemaIdx = json.indexOf("\"schemaVersion\"")
        val runtimeIdx = json.indexOf("\"runtimeVersion\"")
        val sourceIdx = json.indexOf("\"sourceCommit\"")
        assertTrue(schemaIdx < runtimeIdx)
        assertTrue(runtimeIdx < sourceIdx)
    }

    @Test
    fun deserializeWithWhitespace_succeeds() {
        val json = RuntimeManifestJson.serialize(sampleManifest())
        val restored = RuntimeManifestJson.deserialize("  \n$json\n  ")
        assertEquals(sampleManifest().runtimeVersion, restored.runtimeVersion)
    }

    @Test(expected = Exception::class)
    fun deserializeMalformedJson_fails() {
        RuntimeManifestJson.deserialize("{invalid json")
    }

    @Test(expected = Exception::class)
    fun deserializeMissingSchemaVersion_fails() {
        val json = """{"runtimeVersion":"1.0.0"}"""
        RuntimeManifestJson.deserialize(json)
    }

    @Test
    fun serialize_containsNoExplicitNull() {
        val json = RuntimeManifestJson.serialize(sampleManifest())
        assertTrue(!json.contains("\"version\":null"))
        assertNotNull(json)
    }
}
