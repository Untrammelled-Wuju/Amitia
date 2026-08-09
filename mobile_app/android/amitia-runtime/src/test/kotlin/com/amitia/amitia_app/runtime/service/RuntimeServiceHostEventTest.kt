package com.amitia.amitia_app.runtime.service

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class RuntimeServiceHostEventTest {

    @Test
    fun runtimeServiceHostEvent_expectedStopped_isSingletonObject() {
        val a = RuntimeServiceHostEvent.ExpectedStopped
        val b = RuntimeServiceHostEvent.ExpectedStopped
        assertTrue(a === b)
    }

    @Test
    fun runtimeServiceHostEvent_foregroundStarted_isSingletonObject() {
        val a = RuntimeServiceHostEvent.ForegroundStarted
        val b = RuntimeServiceHostEvent.ForegroundStarted
        assertTrue(a === b)
    }

    @Test
    fun runtimeServiceHostEvent_unexpectedTermination_carriesCause() {
        val event = RuntimeServiceHostEvent.UnexpectedTermination(
            cause = RuntimeServiceTerminationCause.FOREGROUND_FAILED
        )
        assertEquals(RuntimeServiceTerminationCause.FOREGROUND_FAILED, event.cause)
    }

    @Test
    fun runtimeServiceTerminationCause_containsExpectedValues() {
        val causes = RuntimeServiceTerminationCause.entries
        assertTrue(causes.contains(RuntimeServiceTerminationCause.FOREGROUND_FAILED))
        assertTrue(causes.contains(RuntimeServiceTerminationCause.NOTIFICATION_FAILED))
        assertTrue(causes.contains(RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR))
    }

    @Test
    fun runtimeServiceHostListener_forwardsEvent() {
        val received = mutableListOf<RuntimeServiceHostEvent>()
        val listener = RuntimeServiceHostListener { event -> received.add(event) }
        listener.onServiceHostEvent(RuntimeServiceHostEvent.ExpectedStopped)
        assertEquals(1, received.size)
        assertEquals(RuntimeServiceHostEvent.ExpectedStopped, received[0])
    }
}
