package com.amitia.amitia_app.runtime.service

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertSame
import org.junit.Test
import com.amitia.amitia_app.runtime.service.internal.DefaultRuntimeServiceEndpoint

class RuntimeServiceLifecycleSnapshotTest {

    @Test
    fun t1_updateLifecycleSnapshot_thenLifecycleSnapshotReturnsIt() {
        val endpoint = DefaultRuntimeServiceEndpoint { null }
        val snapshot = RuntimeServiceLifecycleSnapshot(
            generation = 1L,
            sessionId = "session-1",
            servicePhase = RuntimeServicePhase.FOREGROUND,
            processPhase = RuntimeProcessPhase.STARTED,
            startupPhase = RuntimeStartupPhase.NOT_STARTED,
            terminalState = null,
            latestStartId = 1,
            stopRequested = false,
        )

        endpoint.updateLifecycleSnapshot(snapshot)

        val result = endpoint.lifecycleSnapshot()
        assertNotNull(result)
        assertEquals(1L, result?.generation)
        assertEquals("session-1", result?.sessionId)
        assertEquals(RuntimeServicePhase.FOREGROUND, result?.servicePhase)
        assertEquals(RuntimeProcessPhase.STARTED, result?.processPhase)
        assertEquals(RuntimeStartupPhase.NOT_STARTED, result?.startupPhase)
        assertNull(result?.terminalState)
        assertEquals(1, result?.latestStartId)
        assertEquals(false, result?.stopRequested)
    }

    @Test
    fun t1_updateLifecycleSnapshot_overwritesPrevious() {
        val endpoint = DefaultRuntimeServiceEndpoint { null }
        val snapshot1 = RuntimeServiceLifecycleSnapshot(
            generation = 1L,
            sessionId = "session-1",
            servicePhase = RuntimeServicePhase.CREATED,
            processPhase = RuntimeProcessPhase.CREATED,
            startupPhase = RuntimeStartupPhase.NOT_STARTED,
            terminalState = null,
            latestStartId = 1,
            stopRequested = false,
        )
        val snapshot2 = RuntimeServiceLifecycleSnapshot(
            generation = 2L,
            sessionId = "session-2",
            servicePhase = RuntimeServicePhase.FOREGROUND,
            processPhase = RuntimeProcessPhase.READY,
            startupPhase = RuntimeStartupPhase.READY,
            terminalState = null,
            latestStartId = 2,
            stopRequested = false,
        )

        endpoint.updateLifecycleSnapshot(snapshot1)
        endpoint.updateLifecycleSnapshot(snapshot2)

        val result = endpoint.lifecycleSnapshot()
        assertSame(snapshot2, result)
    }

    @Test
    fun t1_lifecycleSnapshot_initialIsNull() {
        val endpoint = DefaultRuntimeServiceEndpoint { null }
        assertNull(endpoint.lifecycleSnapshot())
    }

    @Test
    fun t1_snapshotAndLifecycleSnapshot_areIndependent() {
        val endpoint = DefaultRuntimeServiceEndpoint { null }
        val lifecycleSnapshot = RuntimeServiceLifecycleSnapshot(
            generation = 1L,
            sessionId = "session-1",
            servicePhase = RuntimeServicePhase.FOREGROUND,
            processPhase = RuntimeProcessPhase.STARTED,
            startupPhase = RuntimeStartupPhase.NOT_STARTED,
            terminalState = null,
            latestStartId = 1,
            stopRequested = false,
        )

        endpoint.updateLifecycleSnapshot(lifecycleSnapshot)

        val hostSnapshot = endpoint.snapshot()
        assertEquals(true, hostSnapshot.created)
        assertEquals(false, hostSnapshot.foreground)
        assertEquals(0, hostSnapshot.boundClients)
    }

    @Test
    fun t4_terminalState_mapping() {
        val endpoint = DefaultRuntimeServiceEndpoint { null }
        val snapshotExpectedStopped = RuntimeServiceLifecycleSnapshot(
            generation = 1L,
            sessionId = "session-1",
            servicePhase = RuntimeServicePhase.DESTROYED,
            processPhase = RuntimeProcessPhase.EXITED,
            startupPhase = RuntimeStartupPhase.FAILED,
            terminalState = RuntimeTerminalState.EXPECTED_STOPPED,
            latestStartId = 1,
            stopRequested = true,
        )

        endpoint.updateLifecycleSnapshot(snapshotExpectedStopped)
        val result = endpoint.lifecycleSnapshot()
        assertEquals(RuntimeTerminalState.EXPECTED_STOPPED, result?.terminalState)

        val snapshotUnexpected = RuntimeServiceLifecycleSnapshot(
            generation = 2L,
            sessionId = "session-2",
            servicePhase = RuntimeServicePhase.DESTROYED,
            processPhase = RuntimeProcessPhase.EXITED,
            startupPhase = RuntimeStartupPhase.FAILED,
            terminalState = RuntimeTerminalState.UNEXPECTED_TERMINATION,
            latestStartId = 2,
            stopRequested = false,
        )

        endpoint.updateLifecycleSnapshot(snapshotUnexpected)
        val result2 = endpoint.lifecycleSnapshot()
        assertEquals(RuntimeTerminalState.UNEXPECTED_TERMINATION, result2?.terminalState)
    }
}
