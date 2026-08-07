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
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestVerifyMode
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.io.File

class DefaultRuntimeManifestVerifierFullTest {

    private val verifier = DefaultRuntimeManifestVerifier()
    private lateinit var tempRoot: File
    private lateinit var runtimeRoot: File
    private lateinit var rootfsDir: File

    private fun sampleManifest(): RuntimeManifest = RuntimeManifest(
        schemaVersion = RuntimeManifest.SCHEMA_VERSION,
        runtimeVersion = "1.0.0",
        sourceCommit = "abc1234567890abc1234567890abc1234567890",
        packageId = "202608070001",
        packageSha256 = "a".repeat(64),
        target = RuntimeManifestTarget("android", "arm64-v8a", "proot", "linux", "arm64", "ubuntu", "24.0.4"),
        installation = RuntimeManifestInstallation("1.0.0", "r1", "rr1", "b".repeat(64)),
        payloads = listOf(RuntimeManifestPayload("p1", "rootfs", "c".repeat(64), 1024L)),
        components = emptyList(),
        paths = RuntimeManifestPaths(
            rootfsHostPath = "",
            runtimeRootHostPath = "",
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
        verification = RuntimeManifestVerification(true, true, true, true, true, true),
    )

    @Before
    fun setup() {
        tempRoot = File.createTempFile("verifier_full_test", "").apply { delete(); mkdirs() }
        rootfsDir = File(tempRoot, "rootfs").apply { mkdirs() }
        runtimeRoot = File(tempRoot, "runtime_root").apply { mkdirs() }
    }

    private fun manifestWithComponents(): RuntimeManifest {
        val backendFile = File(runtimeRoot, "backend/amitia-server").apply {
            parentFile.mkdirs()
            writeText("binary", Charsets.UTF_8)
        }
        val actualSha = InstalledFileHasher.sha256(backendFile)
        return sampleManifest().copy(
            paths = sampleManifest().paths.copy(
                rootfsHostPath = rootfsDir.absolutePath,
                runtimeRootHostPath = runtimeRoot.absolutePath,
            ),
            components = listOf(
                RuntimeManifestComponent(
                    id = "runtime.backend",
                    version = "1.0.0",
                    architecture = "arm64",
                    root = "backend",
                    entry = "amitia-server",
                    sha256 = actualSha,
                    treeSha256 = null,
                    source = "package",
                ),
                RuntimeManifestComponent("runtime.proot", null, null, "", null, null, null, "android-native"),
            ),
        )
    }

    @Test
    fun fullVerify_allMatch_success() {
        val m = manifestWithComponents()
        val result = verifier.verify(m, RuntimeManifestVerifyMode.FULL)
        assertTrue("FULL verify should succeed: $result", result is RuntimeManifestResult.Success)
    }

    @Test
    fun fullVerify_backendShaMismatch_fails() {
        val m = manifestWithComponents().copy(
            components = listOf(
                RuntimeManifestComponent(
                    id = "runtime.backend",
                    version = "1.0.0",
                    architecture = "arm64",
                    root = "backend",
                    entry = "amitia-server",
                    sha256 = "0".repeat(64),
                    treeSha256 = null,
                    source = "package",
                ),
                RuntimeManifestComponent("runtime.proot", null, null, "", null, null, null, "android-native"),
            )
        )
        val result = verifier.verify(m, RuntimeManifestVerifyMode.FULL)
        assertEquals(
            RuntimeManifestErrorCode.MANIFEST_COMPONENT_HASH_MISMATCH,
            (result as RuntimeManifestResult.Failure).error.code
        )
    }

    @Test
    fun lightVerify_backendShaMismatch_passes() {
        val m = manifestWithComponents().copy(
            components = listOf(
                RuntimeManifestComponent(
                    id = "runtime.backend",
                    version = "1.0.0",
                    architecture = "arm64",
                    root = "backend",
                    entry = "amitia-server",
                    sha256 = "0".repeat(64),
                    treeSha256 = null,
                    source = "package",
                ),
                RuntimeManifestComponent("runtime.proot", null, null, "", null, null, null, "android-native"),
            )
        )
        val result = verifier.verify(m, RuntimeManifestVerifyMode.LIGHT)
        assertTrue("LIGHT should not check SHA", result is RuntimeManifestResult.Success)
    }

    @Test
    fun fullVerify_rootfsMissing_fails() {
        val m = manifestWithComponents().copy(
            paths = manifestWithComponents().paths.copy(rootfsHostPath = File(tempRoot, "x").absolutePath)
        )
        val result = verifier.verify(m, RuntimeManifestVerifyMode.FULL)
        assertEquals(
            RuntimeManifestErrorCode.ROOTFS_MISSING,
            (result as RuntimeManifestResult.Failure).error.code
        )
    }
}
