package com.amitia.amitia_app.runtime.connection

import com.amitia.amitia_app.runtime.api.RuntimeComponentSnapshot
import com.amitia.amitia_app.runtime.api.RuntimeComponentState
import com.amitia.amitia_app.runtime.api.RuntimeSnapshot
import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.connection.internal.DefaultBackendConnectionProvider
import com.amitia.amitia_app.runtime.connection.internal.RuntimeCredentialResolver
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File
import java.nio.file.Files

class DefaultBackendConnectionProviderTest {
    @Test
    fun `returns unavailable when runtime state is not ready`() {
        val snapshot = snapshotWithState(RuntimeState.INSTALLED, 1L)
        val provider = providerFor(snapshot, validTokenFile())
        val result = provider.current()
        assertTrue(result is BackendConnectionAvailability.Unavailable)
    }

    @Test
    fun `returns unavailable when required component not ready`() {
        val snapshot = snapshotWithComponents(
            listOf(RuntimeComponentSnapshot("backend", RuntimeComponentState.STARTING, true, null, null, 0L)),
            RuntimeState.READY,
            1L,
        )
        val provider = providerFor(snapshot, validTokenFile())
        val result = provider.current()
        assertTrue(result is BackendConnectionAvailability.Unavailable)
    }

    @Test
    fun `returns available when runtime ready and credential valid`() {
        val snapshot = snapshotWithComponents(
            listOf(RuntimeComponentSnapshot("backend", RuntimeComponentState.READY, true, null, null, 0L)),
            RuntimeState.READY,
            1L,
        )
        val tokenDir = validTokenFile()
        val provider = providerFor(snapshot, tokenDir)
        val result = provider.current()
        assertTrue(result is BackendConnectionAvailability.Available)
        val descriptor = (result as BackendConnectionAvailability.Available).descriptor
        assertEquals("127.0.0.1", descriptor.host)
        assertEquals(18899, descriptor.port)
        assertEquals(1L, descriptor.generation)
    }

    @Test
    fun `returns unavailable when credential missing`() {
        val snapshot = snapshotWithComponents(
            listOf(RuntimeComponentSnapshot("backend", RuntimeComponentState.READY, true, null, null, 0L)),
            RuntimeState.READY,
            1L,
        )
        val tmp = Files.createTempDirectory("amitia-no-token").toFile()
        val provider = providerFor(snapshot, tmp.absolutePath)
        val result = provider.current()
        assertTrue(result is BackendConnectionAvailability.Unavailable)
    }

    @Test
    fun `repeated query same instance keeps same generation`() {
        val snapshot = snapshotWithComponents(
            listOf(RuntimeComponentSnapshot("backend", RuntimeComponentState.READY, true, null, null, 0L)),
            RuntimeState.READY,
            5L,
        )
        val provider = providerFor(snapshot, validTokenFile())
        val first = provider.current() as BackendConnectionAvailability.Available
        val second = provider.current() as BackendConnectionAvailability.Available
        assertEquals(first.descriptor.generation, second.descriptor.generation)
    }

    private fun providerFor(snapshot: RuntimeSnapshot, dataRoot: String): DefaultBackendConnectionProvider {
        return DefaultBackendConnectionProvider(
            snapshotProvider = { snapshot },
            dataRootProvider = { dataRoot },
        )
    }

    private fun snapshotWithState(state: RuntimeState, generation: Long): RuntimeSnapshot {
        return RuntimeSnapshot(
            state = state,
            runtimeVersion = null,
            activeOperationId = null,
            activeOperationType = null,
            com.amitia.amitia_app.runtime.api.RuntimeProgress.none(),
            components = emptyList(),
            lastError = null,
            generation = generation,
            updatedAtEpochMillis = 0L,
        )
    }

    private fun snapshotWithComponents(
        components: List<RuntimeComponentSnapshot>,
        state: RuntimeState,
        generation: Long,
    ): RuntimeSnapshot {
        return RuntimeSnapshot(
            state = state,
            runtimeVersion = null,
            activeOperationId = null,
            activeOperationType = null,
            com.amitia.amitia_app.runtime.api.RuntimeProgress.none(),
            components = components,
            lastError = null,
            generation = generation,
            updatedAtEpochMillis = 0L,
        )
    }

    private fun validTokenFile(): String {
        val tmp = Files.createTempDirectory("amitia-token-valid").toFile()
        val securityDir = File(tmp, "security")
        securityDir.mkdirs()
        val tokenFile = File(securityDir, "local-token")
        tokenFile.writeText("valid_token_${"a".repeat(32)}_suffix")
        return tmp.absolutePath
    }
}
