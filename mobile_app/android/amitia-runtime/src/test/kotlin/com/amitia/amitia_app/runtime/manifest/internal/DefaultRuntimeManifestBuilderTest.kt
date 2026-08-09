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
import org.junit.Assert.assertTrue
import org.junit.Test

class DefaultRuntimeManifestBuilderTest {

    private fun sampleManifest(): RuntimeManifest = RuntimeManifest(
        schemaVersion = RuntimeManifest.SCHEMA_VERSION,
        runtimeVersion = "1.0.0",
        sourceCommit = "a".repeat(40),
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

    @Test
    fun build_withManifest_returnsSuccess() {
        val builder = DefaultRuntimeManifestBuilder().setManifest(sampleManifest())
        val result = builder.build()
        assertTrue(result is RuntimeManifestResult.Success)
    }

    @Test
    fun build_withoutManifest_fails() {
        val builder = DefaultRuntimeManifestBuilder()
        val result = builder.build()
        assertTrue(result is RuntimeManifestResult.Failure)
        assertEquals(RuntimeManifestErrorCode.MANIFEST_INVALID_JSON, (result as RuntimeManifestResult.Failure).error.code)
    }

    @Test
    fun build_sameInput_sameManifest() {
        val builder1 = DefaultRuntimeManifestBuilder().setManifest(sampleManifest())
        val builder2 = DefaultRuntimeManifestBuilder().setManifest(sampleManifest())
        val r1 = builder1.build() as RuntimeManifestResult.Success
        val r2 = builder2.build() as RuntimeManifestResult.Success
        assertEquals(r1.manifest, r2.manifest)
    }

    @Test
    fun builder_doesNotStartProcess() {
        val builder = DefaultRuntimeManifestBuilder().setManifest(sampleManifest())
        val result = builder.build()
        assertTrue(result is RuntimeManifestResult.Success)
    }
}
