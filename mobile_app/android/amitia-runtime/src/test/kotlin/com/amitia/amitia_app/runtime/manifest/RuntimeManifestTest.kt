package com.amitia.amitia_app.runtime.manifest

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test

class RuntimeManifestTest {

    private fun sampleManifest(): RuntimeManifest = RuntimeManifest(
        schemaVersion = RuntimeManifest.SCHEMA_VERSION,
        runtimeVersion = "1.0.0",
        sourceCommit = "a".repeat(40),
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
            RuntimeManifestPayload(
                id = "202608070001-rootfs",
                role = "rootfs",
                sha256 = "c".repeat(64),
                size = 1024L,
            ),
        ),
        components = listOf(
            RuntimeManifestComponent(
                id = "runtime.backend",
                version = "1.0.0",
                architecture = "arm64",
                root = "backend",
                entry = "amitia-server",
                sha256 = "d".repeat(64),
                treeSha256 = null,
                source = "package",
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
    fun schemaVersion_fixedOne() {
        assertEquals(1, RuntimeManifest.SCHEMA_VERSION)
    }

    @Test
    fun runtimeVersion_stored() {
        assertEquals("1.0.0", sampleManifest().runtimeVersion)
    }

    @Test
    fun sourceCommit_40_hex_chars() {
        val m = sampleManifest().copy(sourceCommit = "a".repeat(40))
        assertEquals(40, m.sourceCommit.length)
    }

    @Test
    fun packageSha256_64_hex_chars() {
        val m = sampleManifest()
        assertEquals(64, m.packageSha256.length)
    }

    @Test
    fun target_fields_present() {
        val t = sampleManifest().target
        assertEquals("android", t.hostPlatform)
        assertEquals("arm64-v8a", t.hostAbi)
        assertEquals("proot", t.runtimeKind)
        assertEquals("linux", t.guestPlatform)
        assertEquals("arm64", t.guestArchitecture)
        assertEquals("ubuntu", t.distribution)
    }

    @Test
    fun installation_activeVersion_equals_runtimeVersion() {
        val m = sampleManifest()
        assertEquals(m.runtimeVersion, m.installation.activeVersion)
    }

    @Test
    fun payloads_sortedStable() {
        val m = sampleManifest().copy(
            payloads = listOf(
                RuntimeManifestPayload("p2", "runtime", "e".repeat(64), 2048L),
                RuntimeManifestPayload("p1", "rootfs", "f".repeat(64), 1024L),
            )
        )
        assertEquals(2, m.payloads.size)
        assertEquals("p2", m.payloads[0].id)
        assertEquals("p1", m.payloads[1].id)
    }

    @Test
    fun components_sortedStable() {
        val m = sampleManifest().copy(
            components = listOf(
                RuntimeManifestComponent("runtime.qdrant", "1.19.0", "arm64", "qdrant", "bin/qdrant", null, null, "package"),
                RuntimeManifestComponent("runtime.backend", "1.0.0", "arm64", "backend", "amitia-server", null, null, "package"),
            )
        )
        assertEquals(2, m.components.size)
    }

    @Test
    fun paths_guestRoots_areAbsolute() {
        val p = sampleManifest().paths
        assertTrue(p.guestRuntimeRoot.startsWith("/"))
        assertTrue(p.guestConfigRoot.startsWith("/"))
        assertTrue(p.guestDataRoot.startsWith("/"))
        assertTrue(p.guestCacheRoot.startsWith("/"))
        assertTrue(p.guestLogRoot.startsWith("/"))
        assertTrue(p.guestRunRoot.startsWith("/"))
    }

    @Test
    fun verification_allVerified_trueWhenAllTrue() {
        val v = RuntimeManifestVerification(
            packageVerified = true,
            rootfsVerified = true,
            runtimeRootVerified = true,
            componentsVerified = true,
            guestLayoutVerified = true,
            mountContractVerified = true,
        )
        assertTrue(v.allVerified())
    }

    @Test
    fun noTimestamp_fields() {
        val javaFields = RuntimeManifest::class.java.declaredFields
        for (f in javaFields) {
            val name = f.name.lowercase()
            assertTrue(
                "field '$f' must not contain timestamp semantic",
                !name.contains("installedat") && !name.contains("createdat") &&
                    !name.contains("updatedat") && !name.contains("generatedat") &&
                    !name.contains("generatedtime") && !name.contains("lastmodified")
            )
        }
    }

    @Test
    fun noPid_fields() {
        val javaFields = RuntimeManifest::class.java.declaredFields
        for (f in javaFields) {
            val name = f.name.lowercase()
            assertTrue(
                "field '$f' must not contain PID semantic",
                !name.contains("pid") && !name.contains("backendpid") &&
                    !name.contains("qdrantpid") && !name.contains("prootpid")
            )
        }
    }

    @Test
    fun noCredential_fields() {
        val javaFields = listOf(
            RuntimeManifest::class.java,
            RuntimeManifestTarget::class.java,
            RuntimeManifestInstallation::class.java,
            RuntimeManifestPaths::class.java,
            RuntimeManifestPayload::class.java,
            RuntimeManifestComponent::class.java,
            RuntimeManifestVerification::class.java,
        )
        for (clazz in javaFields) {
            for (f in clazz.declaredFields) {
                val name = f.name.lowercase()
                assertTrue(
                    "field '${clazz.simpleName}.${f.name}' must not contain credential semantic",
                    !name.contains("token") && !name.contains("password") &&
                        !name.contains("secret") && !name.contains("authorization") &&
                        !name.contains("apikey")
                )
            }
        }
    }

    @Test
    fun noBackendEndpoint_fields() {
        val javaFields = listOf(
            RuntimeManifest::class.java,
            RuntimeManifestTarget::class.java,
            RuntimeManifestInstallation::class.java,
            RuntimeManifestPaths::class.java,
            RuntimeManifestPayload::class.java,
            RuntimeManifestComponent::class.java,
            RuntimeManifestVerification::class.java,
        )
        val forbiddenFields = setOf("backendurl", "backendhost", "backendport", "endpointurl", "serverurl")
        for (clazz in javaFields) {
            for (f in clazz.declaredFields) {
                val name = f.name.lowercase()
                assertTrue(
                    "field '${clazz.simpleName}.${f.name}' must not contain backend endpoint semantic",
                    name !in forbiddenFields
                )
            }
        }
    }

    @Test
    fun deterministicJson() {
        val m = sampleManifest()
        val json1 = com.amitia.amitia_app.runtime.manifest.internal.RuntimeManifestJson.serialize(m)
        val json2 = com.amitia.amitia_app.runtime.manifest.internal.RuntimeManifestJson.serialize(m)
        assertEquals(json1, json2)
    }

    @Test
    fun invalidSchema_throws() {
        try {
            sampleManifest().copy(schemaVersion = 99)
            fail("should throw on invalid schema")
        } catch (_: IllegalArgumentException) {
        }
    }

    @Test
    fun invalidSourceCommit_throws() {
        try {
            sampleManifest().copy(sourceCommit = "xxxx")
            fail("should throw on invalid commit")
        } catch (_: IllegalArgumentException) {
        }
    }
}
