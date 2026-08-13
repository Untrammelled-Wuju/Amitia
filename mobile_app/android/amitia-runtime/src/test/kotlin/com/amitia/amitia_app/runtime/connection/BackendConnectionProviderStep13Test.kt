package com.amitia.amitia_app.runtime.connection

import com.amitia.amitia_app.runtime.api.RuntimeComponentSnapshot
import com.amitia.amitia_app.runtime.api.RuntimeComponentState
import com.amitia.amitia_app.runtime.api.RuntimeSnapshot
import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.connection.internal.DefaultBackendConnectionProvider
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File
import java.nio.file.Files

class BackendConnectionProviderStep13Test {
    @Test
    fun `DEGRADED state returns unavailable_READY only`() {
        val snapshot = snapshotWithComponents(
            listOf(RuntimeComponentSnapshot("backend", RuntimeComponentState.READY, true, null, null, 0L)),
            RuntimeState.DEGRADED,
            1L,
        )
        val provider = providerFor(snapshot, validTokenFile())
        val result = provider.current()
        assertTrue(result is BackendConnectionAvailability.Unavailable)
    }

    @Test
    fun `STARTING state returns unavailable`() {
        val snapshot = snapshotWithState(RuntimeState.STARTING, 1L)
        val provider = providerFor(snapshot, validTokenFile())
        val result = provider.current()
        assertTrue(result is BackendConnectionAvailability.Unavailable)
    }

    @Test
    fun `STOPPED state returns unavailable`() {
        val snapshot = snapshotWithState(RuntimeState.STOPPED, 1L)
        val provider = providerFor(snapshot, validTokenFile())
        val result = provider.current()
        assertTrue(result is BackendConnectionAvailability.Unavailable)
    }

    @Test
    fun `STOPPING state returns unavailable`() {
        val snapshot = snapshotWithState(RuntimeState.STOPPING, 1L)
        val provider = providerFor(snapshot, validTokenFile())
        val result = provider.current()
        assertTrue(result is BackendConnectionAvailability.Unavailable)
    }

    @Test
    fun `FAILED state returns unavailable`() {
        val snapshot = snapshotWithState(RuntimeState.FAILED, 1L)
        val provider = providerFor(snapshot, validTokenFile())
        val result = provider.current()
        assertTrue(result is BackendConnectionAvailability.Unavailable)
    }

    @Test
    fun `generation zero returns unavailable`() {
        val snapshot = snapshotWithComponents(
            listOf(RuntimeComponentSnapshot("backend", RuntimeComponentState.READY, true, null, null, 0L)),
            RuntimeState.READY,
            0L,
        )
        val provider = providerFor(snapshot, validTokenFile())
        val result = provider.current()
        assertTrue(result is BackendConnectionAvailability.Unavailable)
    }

    @Test
    fun `generation equals snapshot generation exact`() {
        val snapshot = snapshotWithComponents(
            listOf(RuntimeComponentSnapshot("backend", RuntimeComponentState.READY, true, null, null, 0L)),
            RuntimeState.READY,
            42L,
        )
        val provider = providerFor(snapshot, validTokenFile())
        val result = provider.current()
        assertTrue(result is BackendConnectionAvailability.Available)
        val descriptor = (result as BackendConnectionAvailability.Available).descriptor
        assertEquals(42L, descriptor.generation)
    }

    @Test
    fun `credential missing returns unavailable not fallback`() {
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
    fun `invalid credential returns unavailable`() {
        val snapshot = snapshotWithComponents(
            listOf(RuntimeComponentSnapshot("backend", RuntimeComponentState.READY, true, null, null, 0L)),
            RuntimeState.READY,
            1L,
        )
        val tmp = Files.createTempDirectory("amitia-bad-cred").toFile()
        val secDir = File(tmp, "security")
        secDir.mkdirs()
        File(secDir, "local-token").writeText("short")
        val provider = providerFor(snapshot, tmp.absolutePath)
        val result = provider.current()
        assertTrue(result is BackendConnectionAvailability.Unavailable)
    }

    @Test
    fun `restart changes generation`() {
        val readySnap = snapshotWithComponents(
            listOf(RuntimeComponentSnapshot("backend", RuntimeComponentState.READY, true, null, null, 0L)),
            RuntimeState.READY,
            7L,
        )
        val tokenDir = validTokenFile()
        val provider = providerFor(readySnap, tokenDir)

        val first = provider.current() as BackendConnectionAvailability.Available
        assertEquals(7L, first.descriptor.generation)

        val restartingSnap = snapshotWithState(RuntimeState.STOPPING, 7L)
        val providerRestart = providerFor(restartingSnap, tokenDir)
        assertTrue(providerRestart.current() is BackendConnectionAvailability.Unavailable)

        val newReadySnap = snapshotWithComponents(
            listOf(RuntimeComponentSnapshot("backend", RuntimeComponentState.READY, true, null, null, 0L)),
            RuntimeState.READY,
            8L,
        )
        val providerNew = providerFor(newReadySnap, tokenDir)
        val second = providerNew.current() as BackendConnectionAvailability.Available
        assertEquals(8L, second.descriptor.generation)
        assertFalse(second.descriptor.generation == first.descriptor.generation)
    }

    @Test
    fun `endpoint comes from policy only`() {
        val snapshot = snapshotWithComponents(
            listOf(RuntimeComponentSnapshot("backend", RuntimeComponentState.READY, true, null, null, 0L)),
            RuntimeState.READY,
            1L,
        )
        val provider = providerFor(snapshot, validTokenFile())
        val result = provider.current() as BackendConnectionAvailability.Available
        assertEquals("127.0.0.1", result.descriptor.host)
        assertEquals(18899, result.descriptor.port)
        assertEquals("http", result.descriptor.httpScheme)
        assertEquals("ws", result.descriptor.webSocketScheme)
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
