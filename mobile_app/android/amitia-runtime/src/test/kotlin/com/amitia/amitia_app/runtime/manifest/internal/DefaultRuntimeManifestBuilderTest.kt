package com.amitia.amitia_app.runtime.manifest.internal

import com.amitia.amitia_app.runtime.abi.RuntimeAbiSnapshot
import com.amitia.amitia_app.runtime.abi.RuntimeAbiStatus
import com.amitia.amitia_app.runtime.install.RuntimeHostLayout
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
import java.io.File

class DefaultRuntimeManifestBuilderTest {

    private fun testLayout(): RuntimeHostLayout = object : RuntimeHostLayout {
        override val controlRoot = File("/data/.__runtime/control")
        override val rootfsRoot = File("/data/.__runtime/rootfs")
        override val versionsRoot = File("/data/.__runtime/versions")
        override val stagingRoot = File("/data/.__runtime/staging")
        override val metadataRoot = File("/data/.__runtime/metadata")
        override val transactionsRoot = File("/data/.__runtime/transactions")
        override val locksRoot = File("/data/.__runtime/locks")
        override val packagesRoot = File("/data/.__runtime/packages")
        override val configRoot = File("/data/.__runtime/config")
        override val dataRoot = File("/data/.__runtime/data")
        override val cacheRoot = File("/data/.__runtime/cache")
        override val logRoot = File("/data/.__runtime/logs")
        override val runRoot = File("/data/.__runtime/run")
        override val homeRoot = File("/data/.__runtime/home")

        override fun runtimeVersionRoot(version: String): File =
            File(versionsRoot, version)

        override fun installReceiptFile(version: String): File =
            File("/data/.__runtime/install-receipts/$version.json")

        override val activeRuntimeFile = File("/data/.__runtime/active-runtime.json")
        override val runtimeManifestFile = File("/data/.__runtime/runtime-manifest.json")
        override val runtimeManifestShaFile = File("/data/.__runtime/runtime-manifest.json.sha256")
    }

    private fun testAbiStatus(): RuntimeAbiStatus.Supported =
        RuntimeAbiStatus.Supported(
            abi = "arm64-v8a",
            processIs64Bit = true,
            snapshot = RuntimeAbiSnapshot(emptyMap())
        )

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
            "/data/.__runtime/rootfs", "/data/.__runtime/versions/1.0.0",
            "/data/.__runtime/config", "/data/.__runtime/data",
            "/data/.__runtime/cache", "/data/.__runtime/logs", "/data/.__runtime/run",
            "/opt/amitia", "/etc/amitia", "/var/lib/amitia",
            "/var/cache/amitia", "/var/log/amitia", "/run/amitia"
        ),
        verification = RuntimeManifestVerification(true, true, true, true, true, true),
    )

    @Test
    fun build_withManifest_returnsSuccess() {
        val builder = DefaultRuntimeManifestBuilder(testLayout(), testAbiStatus()).setManifest(sampleManifest())
        val result = builder.build()
        assertTrue(result is RuntimeManifestResult.Success)
    }

    @Test
    fun build_withoutManifest_fails() {
        val builder = DefaultRuntimeManifestBuilder(testLayout(), testAbiStatus())
        val result = builder.build()
        assertTrue(result is RuntimeManifestResult.Failure)
        assertEquals(RuntimeManifestErrorCode.MANIFEST_INVALID_JSON, (result as RuntimeManifestResult.Failure).error.code)
    }

    @Test
    fun build_sameInput_sameManifest() {
        val builder1 = DefaultRuntimeManifestBuilder(testLayout(), testAbiStatus()).setManifest(sampleManifest())
        val builder2 = DefaultRuntimeManifestBuilder(testLayout(), testAbiStatus()).setManifest(sampleManifest())
        val r1 = builder1.build() as RuntimeManifestResult.Success
        val r2 = builder2.build() as RuntimeManifestResult.Success
        assertEquals(r1.manifest, r2.manifest)
    }

    @Test
    fun builder_doesNotStartProcess() {
        val builder = DefaultRuntimeManifestBuilder(testLayout(), testAbiStatus()).setManifest(sampleManifest())
        val result = builder.build()
        assertTrue(result is RuntimeManifestResult.Success)
    }

    @Test
    fun buildFromInstalledTree_guestArchitectureIsArm64() {
        val builder = DefaultRuntimeManifestBuilder(testLayout(), testAbiStatus())
        val result = builder.buildFromInstalledTree(
            runtimeVersion = "1.2.3",
            sourceCommit = "abc1234567890def",
            packageId = "amitia-runtime-1.2.3",
            packageSha256 = "a".repeat(64),
            rootfsId = "rootfs-1",
            runtimeRootTreeSha256 = "b".repeat(64),
            payloads = emptyList(),
            components = emptyList(),
        )
        val manifest = (result as RuntimeManifestResult.Success).manifest
        assertEquals("arm64", manifest.target.guestArchitecture)
        assertEquals("arm64-v8a", manifest.target.hostAbi)
    }

    @Test
    fun buildFromInstalledTree_runtimeRootHostPathIsVersionDirectory() {
        val builder = DefaultRuntimeManifestBuilder(testLayout(), testAbiStatus())
        val result = builder.buildFromInstalledTree(
            runtimeVersion = "1.2.3",
            sourceCommit = "abc1234567890def",
            packageId = "amitia-runtime-1.2.3",
            packageSha256 = "a".repeat(64),
            rootfsId = "rootfs-1",
            runtimeRootTreeSha256 = "b".repeat(64),
            payloads = emptyList(),
            components = emptyList(),
        )
        val manifest = (result as RuntimeManifestResult.Success).manifest
        assertEquals("/data/.__runtime/versions/1.2.3", manifest.paths.runtimeRootHostPath)
    }
}
