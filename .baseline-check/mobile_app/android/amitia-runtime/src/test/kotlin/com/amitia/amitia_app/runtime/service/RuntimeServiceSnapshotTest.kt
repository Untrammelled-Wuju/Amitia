package com.amitia.amitia_app.runtime.service

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class RuntimeServiceSnapshotTest {

    @Test
    fun snapshot_containsOnlyHostState() {
        val snapshot = RuntimeServiceSnapshot(
            created = true,
            foreground = true,
            boundClients = 2
        )
        assertTrue(snapshot.created)
        assertTrue(snapshot.foreground)
        assertEquals(2, snapshot.boundClients)
    }

    @Test
    fun snapshot_doesNotExposeRuntimeState() {
        val snapshot = RuntimeServiceSnapshot(
            created = true,
            foreground = false,
            boundClients = 0
        )
        assertFalse(snapshot.foreground)
        assertTrue(snapshot.created)
    }

    @Test
    fun snapshot_defaultsToMinimal() {
        val snapshot = RuntimeServiceSnapshot(
            created = false,
            foreground = false,
            boundClients = 0
        )
        assertFalse(snapshot.created)
        assertFalse(snapshot.foreground)
        assertEquals(0, snapshot.boundClients)
    }
}
