package com.amitia.amitia_app.runtime.manifest.internal

import com.amitia.amitia_app.runtime.manifest.RuntimeManifestComponent
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestErrorCode
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestInstallation
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestPaths
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestPayload
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestTarget
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestVerification
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Test

class DefaultRuntimeManifestValidatorTest {

    private val validator = DefaultRuntimeManifestValidator()

    private fun sampleTarget(): RuntimeManifestTarget = RuntimeManifestTarget(
        hostPlatform = "android",
        hostAbi = "arm64-v8a",
            runtimeKind = "embedded-proot",
        guestPlatform = "linux",
        guestArchitecture = "arm64",
        distribution = "ubuntu",
        distributionRelease = "24.0.4",
    )

    private fun samplePaths(): RuntimeManifestPaths = RuntimeManifestPaths(
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
    )

    private fun sampleVerification(): RuntimeManifestVerification = RuntimeManifestVerification(
        packageVerified = true,
        rootfsVerified = true,
        runtimeRootVerified = true,
        componentsVerified = true,
        guestLayoutVerified = true,
        mountContractVerified = true,
    )

    @Test
    fun validSchema_ok() {
        assertNull(validator.validateSchema(1))
    }

    @Test
    fun unknownSchema_fails() {
        val err = validator.validateSchema(99)
        assertNotNull(err)
        assertEquals(RuntimeManifestErrorCode.UNSUPPORTED_MANIFEST_SCHEMA, err!!.code)
    }

    @Test
    fun validVersion_ok() {
        assertNull(validator.validateRuntimeVersion("1.0.0"))
    }

    @Test
    fun blankVersion_fails() {
        val err = validator.validateRuntimeVersion("")
        assertNotNull(err)
        assertEquals(RuntimeManifestErrorCode.MANIFEST_FIELD_INVALID, err!!.code)
    }

    @Test
    fun validCommit_ok() {
        assertNull(validator.validateSourceCommit("a".repeat(40)))
    }

    @Test
    fun invalidCommit_fails() {
        val err = validator.validateSourceCommit("zzz")
        assertNotNull(err)
        assertEquals(RuntimeManifestErrorCode.MANIFEST_FIELD_INVALID, err!!.code)
    }

    @Test
    fun validPackageSha_ok() {
        assertNull(validator.validatePackageSha256("a".repeat(64)))
    }

    @Test
    fun invalidPackageSha_fails() {
        val err = validator.validatePackageSha256("short")
        assertNotNull(err)
        assertEquals(RuntimeManifestErrorCode.MANIFEST_FIELD_INVALID, err!!.code)
    }

    @Test
    fun validTarget_android_ok() {
        assertNull(validator.validateTarget(sampleTarget()))
    }

    @Test
    fun androidTarget_wrongHostPlatform_fails() {
        val err = validator.validateTarget(sampleTarget().copy(hostPlatform = "windows"))
        assertNotNull(err)
        assertEquals(RuntimeManifestErrorCode.MANIFEST_TARGET_MISMATCH, err!!.code)
    }

    @Test
    fun androidTarget_wrongGuestPlatform_fails() {
        val err = validator.validateTarget(sampleTarget().copy(guestPlatform = "freebsd"))
        assertNotNull(err)
        assertEquals(RuntimeManifestErrorCode.MANIFEST_TARGET_MISMATCH, err!!.code)
    }

    @Test
    fun validInstallation_activeVersionMatches_ok() {
        val inst = RuntimeManifestInstallation("1.0.0", "r1", "rr1", "b".repeat(64))
        assertNull(validator.validateInstallation(inst, "1.0.0"))
    }

    @Test
    fun installation_activeVersionMismatch_fails() {
        val inst = RuntimeManifestInstallation("1.0.0", "r1", "rr1", "b".repeat(64))
        val err = validator.validateInstallation(inst, "2.0.0")
        assertNotNull(err)
        assertEquals(RuntimeManifestErrorCode.MANIFEST_ACTIVE_VERSION_MISMATCH, err!!.code)
    }

    @Test
    fun validComponents_ok() {
        val comps = listOf(
            RuntimeManifestComponent("runtime.backend", "1.0.0", "arm64", "backend", "amitia-server", null, null, "package"),
        )
        assertNull(validator.validateComponents(comps))
    }

    @Test
    fun duplicateComponents_fails() {
        val comps = listOf(
            RuntimeManifestComponent("runtime.backend", "1.0.0", "arm64", "backend", "amitia-server", null, null, "package"),
            RuntimeManifestComponent("runtime.backend", "1.0.0", "arm64", "backend", "amitia-server", null, null, "package"),
        )
        val err = validator.validateComponents(comps)
        assertNotNull(err)
        assertEquals(RuntimeManifestErrorCode.MANIFEST_COMPONENT_DUPLICATE, err!!.code)
    }

    @Test
    fun validPayloads_ok() {
        val payloads = listOf(
            RuntimeManifestPayload("p1", "rootfs", "c".repeat(64), 1024L),
        )
        assertNull(validator.validatePayloads(payloads))
    }

    @Test
    fun duplicatePayloads_fails() {
        val payloads = listOf(
            RuntimeManifestPayload("p1", "rootfs", "c".repeat(64), 1024L),
            RuntimeManifestPayload("p1", "runtime", "d".repeat(64), 2048L),
        )
        val err = validator.validatePayloads(payloads)
        assertNotNull(err)
        assertEquals(RuntimeManifestErrorCode.MANIFEST_PAYLOAD_DUPLICATE, err!!.code)
    }

    @Test
    fun validPaths_ok() {
        assertNull(validator.validatePaths(samplePaths()))
    }

    @Test
    fun hostPathOverlap_fails() {
        val paths = samplePaths().copy(
            configHostPath = "/data/.__runtime/data/config"
        )
        val err = validator.validatePaths(paths)
        assertNotNull(err)
        assertEquals(RuntimeManifestErrorCode.MANIFEST_PATH_OVERLAP, err!!.code)
    }

    @Test
    fun guestPathNotAbsolute_fails() {
        val paths = samplePaths().copy(
            guestRuntimeRoot = "relative/path"
        )
        val err = validator.validatePaths(paths)
        assertNotNull(err)
        assertEquals(RuntimeManifestErrorCode.MANIFEST_PATH_INVALID, err!!.code)
    }

    @Test
    fun allVerificationTrue_ok() {
        assertNull(validator.validateVerification(sampleVerification()))
    }

    @Test
    fun verificationNotAllTrue_fails() {
        val err = validator.validateVerification(
            sampleVerification().copy(componentsVerified = false)
        )
        assertNotNull(err)
        assertEquals(RuntimeManifestErrorCode.MANIFEST_FIELD_INVALID, err!!.code)
    }
}
