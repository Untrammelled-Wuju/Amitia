package com.amitia.amitia_app.runtime.service

import com.amitia.amitia_app.runtime.service.internal.DefaultRuntimeServiceEndpoint
import org.junit.Assert.assertEquals
import org.junit.Test

class DefaultRuntimeServiceEndpointTest {

    @Test
    fun endpoint_snapshot_returnsEmptyWhenServiceUnavailable() {
        val endpoint = DefaultRuntimeServiceEndpoint { null }
        val snapshot = endpoint.snapshot()
        assertEquals(false, snapshot.created)
        assertEquals(false, snapshot.foreground)
        assertEquals(0, snapshot.boundClients)
    }

    @Test
    fun endpoint_notifiesAllListeners() {
        val received1 = mutableListOf<RuntimeServiceHostEvent>()
        val received2 = mutableListOf<RuntimeServiceHostEvent>()
        val endpoint = DefaultRuntimeServiceEndpoint { null }
        val l1 = RuntimeServiceHostListener { received1.add(it) }
        val l2 = RuntimeServiceHostListener { received2.add(it) }

        endpoint.addListener(l1)
        endpoint.addListener(l2)
        val event = RuntimeServiceHostEvent.ExpectedStopped(generation = 1L)
        endpoint.notify(event)

        assertEquals(1, received1.size)
        assertEquals(1, received2.size)
        assertEquals(event, received1[0])
        assertEquals(event, received2[0])
    }

    @Test
    fun endpoint_removeListener_stopsReceivingEvents() {
        val received = mutableListOf<RuntimeServiceHostEvent>()
        val endpoint = DefaultRuntimeServiceEndpoint { null }
        val listener = RuntimeServiceHostListener { received.add(it) }
        endpoint.addListener(listener)
        endpoint.removeListener(listener)
        endpoint.notify(RuntimeServiceHostEvent.ExpectedStopped(generation = 1L))
        assertEquals(0, received.size)
    }

    @Test
    fun endpoint_listenerException_doesNotPreventOtherListeners() {
        val received = mutableListOf<RuntimeServiceHostEvent>()
        val endpoint = DefaultRuntimeServiceEndpoint { null }

        val badListener = RuntimeServiceHostListener { throw RuntimeException("test") }
        val goodListener = RuntimeServiceHostListener { received.add(it) }

        endpoint.addListener(badListener)
        endpoint.addListener(goodListener)
        endpoint.notify(RuntimeServiceHostEvent.ExpectedStopped(generation = 1L))
        assertEquals(1, received.size)
    }

    @Test
    fun endpoint_snapshot_delegatesToServiceProvider() {
        val endpoint = DefaultRuntimeServiceEndpoint { null }
        val snapshot = endpoint.snapshot()
        assertEquals(false, snapshot.created)
    }
}
