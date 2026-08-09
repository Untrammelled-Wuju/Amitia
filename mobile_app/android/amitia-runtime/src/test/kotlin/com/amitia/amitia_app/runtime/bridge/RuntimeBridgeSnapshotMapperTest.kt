package com.amitia.amitia_app.runtime.bridge

import com.amitia.amitia_app.runtime.api.RuntimeComponentSnapshot
import com.amitia.amitia_app.runtime.api.RuntimeComponentState
import com.amitia.amitia_app.runtime.api.RuntimeError
import com.amitia.amitia_app.runtime.api.RuntimeErrorCode
import com.amitia.amitia_app.runtime.api.RuntimeProgress
import com.amitia.amitia_app.runtime.api.RuntimeSnapshot
import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.manifest.RuntimeManifest
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestInstallation
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestPayload
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestComponent
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestPaths
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestTarget
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestVerification
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class RuntimeBridgeSnapshotMapperTest {

    @Test
    fun maps_unknown_state_to_unavailable() {
        val snapshot = createSnapshot(RuntimeState.UNKNOWN)
        val mapped = RuntimeBridgeSnapshotMapper.toBridgeSnapshot(
            snapshot = snapshot,
            manifest = null,
            runtimeInstalled = false,
            runtimeAvailable = false,
        )
        assertEquals("UNAVAILABLE", mapped["state"])
        assertEquals(1, mapped["schemaVersion"])
    }

    @Test
    fun maps_ready_state() {
        val snapshot = createSnapshot(RuntimeState.READY)
        val mapped = RuntimeBridgeSnapshotMapper.toBridgeSnapshot(
            snapshot = snapshot,
            manifest = null,
            runtimeInstalled = true,
            runtimeAvailable = true,
        )
        assertEquals("READY", mapped["state"])
    }

    @Test
    fun maps_installing_state() {
        val snapshot = createSnapshot(RuntimeState.INSTALLING)
        val mapped = RuntimeBridgeSnapshotMapper.toBridgeSnapshot(
            snapshot = snapshot,
            manifest = null,
            runtimeInstalled = false,
            runtimeAvailable = false,
        )
        assertEquals("INSTALLING", mapped["state"])
    }

    @Test
    fun maps_failed_state() {
        val snapshot = createSnapshot(RuntimeState.FAILED)
        val mapped = RuntimeBridgeSnapshotMapper.toBridgeSnapshot(
            snapshot = snapshot,
            manifest = null,
            runtimeInstalled = true,
            runtimeAvailable = false,
        )
        assertEquals("FAILED", mapped["state"])
    }

    @Test
    fun maps_corrupted_state_to_failed() {
        val snapshot = createSnapshot(RuntimeState.CORRUPTED)
        val mapped = RuntimeBridgeSnapshotMapper.toBridgeSnapshot(
            snapshot = snapshot,
            manifest = null,
            runtimeInstalled = true,
            runtimeAvailable = false,
        )
        assertEquals("FAILED", mapped["state"])
    }

    @Test
    fun maps_generation() {
        val snapshot = createSnapshot(RuntimeState.INSTALLED, generation = 42)
        val mapped = RuntimeBridgeSnapshotMapper.toBridgeSnapshot(
            snapshot = snapshot,
            manifest = null,
            runtimeInstalled = false,
            runtimeAvailable = false,
        )
        assertEquals(42L, mapped["generation"])
    }

    @Test
    fun maps_runtime_installed_flag() {
        val snapshot = createSnapshot(RuntimeState.INSTALLED)
        val mapped = RuntimeBridgeSnapshotMapper.toBridgeSnapshot(
            snapshot = snapshot,
            manifest = null,
            runtimeInstalled = true,
            runtimeAvailable = false,
        )
        assertEquals(true, mapped["runtimeInstalled"])
    }

    @Test
    fun maps_runtime_available_flag() {
        val snapshot = createSnapshot(RuntimeState.READY)
        val mapped = RuntimeBridgeSnapshotMapper.toBridgeSnapshot(
            snapshot = snapshot,
            manifest = null,
            runtimeInstalled = true,
            runtimeAvailable = true,
        )
        assertEquals(true, mapped["runtimeAvailable"])
    }

    @Test
    fun maps_last_error() {
        val error = RuntimeError(
            code = RuntimeErrorCode.START_FAILED,
            message = "test error message",
            recoverable = true,
        )
        val snapshot = createSnapshot(RuntimeState.FAILED, error = error)
        val mapped = RuntimeBridgeSnapshotMapper.toBridgeSnapshot(
            snapshot = snapshot,
            manifest = null,
            runtimeInstalled = true,
            runtimeAvailable = false,
        )
        val errorMap = mapped["error"] as? Map<*, *>
        assertNotNull(errorMap)
        assertEquals("START_FAILED", errorMap!!["code"])
        assertEquals("test error message", errorMap["message"])
        assertEquals(true, errorMap["retryable"])
    }

    @Test
    fun maps_null_error() {
        val snapshot = createSnapshot(RuntimeState.READY)
        val mapped = RuntimeBridgeSnapshotMapper.toBridgeSnapshot(
            snapshot = snapshot,
            manifest = null,
            runtimeInstalled = true,
            runtimeAvailable = true,
        )
        assertNull(mapped["error"])
    }

    @Test
    fun maps_manifest_without_host_paths() {
        val manifest = createManifest()
        val snapshot = createSnapshot(RuntimeState.INSTALLED)
        val mapped = RuntimeBridgeSnapshotMapper.toBridgeSnapshot(
            snapshot = snapshot,
            manifest = manifest,
            runtimeInstalled = true,
            runtimeAvailable = true,
        )
        val manifestMap = mapped["manifest"] as? Map<*, *>
        assertNotNull(manifestMap)
        assertEquals(1, manifestMap!!["schemaVersion"])
        assertEquals("1.0.0", manifestMap["runtimeVersion"])
        assertEquals("test-package", manifestMap["packageId"])
        assertEquals("android", manifestMap["targetPlatform"])
        assertEquals("arm64-v8a", manifestMap["targetArch"])
        assertFalse(manifestMap.keys.any { it.toString().contains("path", ignoreCase = true) })
        assertFalse(manifestMap.keys.any { it.toString().contains("host", ignoreCase = true) })
    }

    @Test
    fun manifest_does_not_contain_local_token() {
        val manifest = createManifest()
        val snapshot = createSnapshot(RuntimeState.INSTALLED)
        val mapped = RuntimeBridgeSnapshotMapper.toBridgeSnapshot(
            snapshot = snapshot,
            manifest = manifest,
            runtimeInstalled = true,
            runtimeAvailable = true,
        )
        val manifestMap = mapped["manifest"] as? Map<*, *>
        assertNotNull(manifestMap)
        val serialized = manifestMap.toString()
        assertFalse(serialized.contains("local-token"))
        assertFalse(serialized.contains("X-Amitia-Local-Token"))
    }

    private fun createSnapshot(
        state: RuntimeState,
        generation: Long = 1L,
        error: RuntimeError? = null,
    ): RuntimeSnapshot {
        return RuntimeSnapshot(
            state = state,
            runtimeVersion = "1.0.0",
            activeOperationId = null,
            activeOperationType = null,
            progress = RuntimeProgress.none(),
            components = emptyList(),
            lastError = error,
            generation = generation,
            updatedAtEpochMillis = 1000L,
        )
    }

    private fun createManifest(): RuntimeManifest {
        return RuntimeManifest(
            schemaVersion = 1,
            runtimeVersion = "1.0.0",
            sourceCommit = "a".repeat(40),
            packageId = "test-package",
            packageSha256 = "a".repeat(64),
            target = RuntimeManifestTarget(
                hostPlatform = "android",
                hostAbi = "arm64-v8a",
                runtimeKind = "proot",
                guestPlatform = "linux",
                guestArchitecture = "arm64",
                distribution = "ubuntu",
                distributionRelease = "22.04",
            ),
            installation = RuntimeManifestInstallation(
                activeVersion = "1.0.0",
                rootfsId = "rootfs",
                runtimeRootId = "runtime-root",
                runtimeRootTreeSha256 = "b".repeat(64),
            ),
            payloads = listOf(
                RuntimeManifestPayload(
                    id = "rootfs",
                    role = "rootfs",
                    sha256 = "b".repeat(64),
                    size = 1024L,
                )
            ),
            components = listOf(
                RuntimeManifestComponent(
                    id = "runtime.package",
                    version = "1.0.0",
                    architecture = "arm64",
                    root = "/data/local/tmp/component",
                    entry = "main",
                    sha256 = "c".repeat(64),
                    treeSha256 = "d".repeat(64),
                    source = "package",
                )
            ),
            paths = RuntimeManifestPaths(
                rootfsHostPath = "/data/local/tmp/rootfs",
                runtimeRootHostPath = "/data/local/tmp/runtime",
                configHostPath = "/data/local/tmp/config",
                dataHostPath = "/data/local/tmp/data",
                cacheHostPath = "/data/local/tmp/cache",
                logHostPath = "/data/local/tmp/log",
                runHostPath = "/data/local/tmp/run",
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
    }
}
