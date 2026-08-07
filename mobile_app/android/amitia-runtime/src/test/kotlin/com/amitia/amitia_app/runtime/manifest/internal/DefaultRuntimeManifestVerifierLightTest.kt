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

class DefaultRuntimeManifestVerifierLightTest {

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
        components = listOf(
            RuntimeManifestComponent("runtime.backend", "1.0.0", "arm64", "backend", "amitia-server", "d".repeat(64), null, "package"),
            RuntimeManifestComponent("runtime.proot", null, null, "", null, null, null, "android-native"),
        ),
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
        tempRoot = File.createTempFile("verifier_test", "").apply { delete(); mkdirs() }
        rootfsDir = File(tempRoot, "rootfs").apply { mkdirs() }
        runtimeRoot = File(tempRoot, "runtime_root").apply { mkdirs() }
        File(runtimeRoot, "backend").apply { mkdirs() }.apply {
            File(this, "amitia-server").writeText("binary", Charsets.UTF_8)
        }
    }

    private fun manifestWithExistingPaths(): RuntimeManifest {
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
                    sha256 = "wrong_sha_early_test",
                    treeSha256 = null,
                    source = "package",
                ),
                RuntimeManifestComponent("runtime.proot", null, null, "", null, null, null, "android-native"),
            )
        )
    }

    @Test
    fun lightVerify_allFilesExist_success() {
        val m = manifestWithExistingPaths()
        val result = verifier.verify(m, RuntimeManifestVerifyMode.LIGHT)
        assertTrue("LIGHT verify should succeed: $result", result is RuntimeManifestResult.Success)
    }

    @Test
    fun lightVerify_rootfsMissing_fails() {
        val m = sampleManifest().copy(
            paths = sampleManifest().paths.copy(
                rootfsHostPath = File(tempRoot, "nonexistent").absolutePath,
                runtimeRootHostPath = runtimeRoot.absolutePath,
            )
        )
        val result = verifier.verify(m, RuntimeManifestVerifyMode.LIGHT)
        assertEquals(
            RuntimeManifestErrorCode.ROOTFS_MISSING,
            (result as RuntimeManifestResult.Failure).error.code
        )
    }

    @Test
    fun lightVerify_runtimeRootMissing_fails() {
        val m = sampleManifest().copy(
            paths = sampleManifest().paths.copy(
                rootfsHostPath = rootfsDir.absolutePath,
                runtimeRootHostPath = File(tempRoot, "nonexistent").absolutePath,
            )
        )
        val result = verifier.verify(m, RuntimeManifestVerifyMode.LIGHT)
        assertEquals(
            RuntimeManifestErrorCode.RUNTIME_ROOT_MISSING,
            (result as RuntimeManifestResult.Failure).error.code
        )
    }

    @Test
    fun lightVerify_backendEntryMissing_fails() {
        File(runtimeRoot, "backend/amitia-server").delete()
        val m = manifestWithExistingPaths()
        val result = verifier.verify(m, RuntimeManifestVerifyMode.LIGHT)
        assertEquals(
            RuntimeManifestErrorCode.MANIFEST_COMPONENT_MISSING,
            (result as RuntimeManifestResult.Failure).error.code
        )
    }

    @Test
    fun lightVerify_backendShaMismatch_stillSucceeds() {
        val m = manifestWithExistingPaths()
        val result = verifier.verify(m, RuntimeManifestVerifyMode.LIGHT)
        assertTrue("LIGHT should not check backend SHA", result is RuntimeManifestResult.Success)
    }

    @Test
    fun lightVerify_verificationFlagsNotAllTrue_fails() {
        val m = manifestWithExistingPaths().copy(
            verification = RuntimeManifestVerification(false, true, true, true, true, true)
        )
        val result = verifier.verify(m, RuntimeManifestVerifyMode.LIGHT)
        assertEquals(
            RuntimeManifestErrorCode.MANIFEST_FIELD_INVALID,
            (result as RuntimeManifestResult.Failure).error.code
        )
    }
}
