package com.amitia.amitia_app.runtime

import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.internal.IllegalRuntimeTransitionException
import com.amitia.amitia_app.runtime.internal.RuntimeStateMachine
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test

class RuntimeStateMachineTest {

    @Test
    fun unknown_to_notInstalled_allowed() {
        assertTrue(RuntimeStateMachine.canTransition(RuntimeState.UNKNOWN, RuntimeState.NOT_INSTALLED))
        RuntimeStateMachine.requireTransition(RuntimeState.UNKNOWN, RuntimeState.NOT_INSTALLED)
    }

    @Test
    fun unknown_to_installed_allowed() {
        assertTrue(RuntimeStateMachine.canTransition(RuntimeState.UNKNOWN, RuntimeState.INSTALLED))
        RuntimeStateMachine.requireTransition(RuntimeState.UNKNOWN, RuntimeState.INSTALLED)
    }

    @Test
    fun notInstalled_to_installing_allowed() {
        assertTrue(RuntimeStateMachine.canTransition(RuntimeState.NOT_INSTALLED, RuntimeState.INSTALLING))
        RuntimeStateMachine.requireTransition(RuntimeState.NOT_INSTALLED, RuntimeState.INSTALLING)
    }

    @Test
    fun installing_to_installed_allowed() {
        assertTrue(RuntimeStateMachine.canTransition(RuntimeState.INSTALLING, RuntimeState.INSTALLED))
        RuntimeStateMachine.requireTransition(RuntimeState.INSTALLING, RuntimeState.INSTALLED)
    }

    @Test
    fun installing_to_failed_allowed() {
        assertTrue(RuntimeStateMachine.canTransition(RuntimeState.INSTALLING, RuntimeState.FAILED))
        RuntimeStateMachine.requireTransition(RuntimeState.INSTALLING, RuntimeState.FAILED)
    }

    @Test
    fun installed_to_verifying_allowed() {
        assertTrue(RuntimeStateMachine.canTransition(RuntimeState.INSTALLED, RuntimeState.VERIFYING))
        RuntimeStateMachine.requireTransition(RuntimeState.INSTALLED, RuntimeState.VERIFYING)
    }

    @Test
    fun installed_to_starting_allowed() {
        assertTrue(RuntimeStateMachine.canTransition(RuntimeState.INSTALLED, RuntimeState.STARTING))
        RuntimeStateMachine.requireTransition(RuntimeState.INSTALLED, RuntimeState.STARTING)
    }

    @Test
    fun verifying_to_corrupted_allowed() {
        assertTrue(RuntimeStateMachine.canTransition(RuntimeState.VERIFYING, RuntimeState.CORRUPTED))
        RuntimeStateMachine.requireTransition(RuntimeState.VERIFYING, RuntimeState.CORRUPTED)
    }

    @Test
    fun starting_to_ready_allowed() {
        assertTrue(RuntimeStateMachine.canTransition(RuntimeState.STARTING, RuntimeState.READY))
        RuntimeStateMachine.requireTransition(RuntimeState.STARTING, RuntimeState.READY)
    }

    @Test
    fun starting_to_degraded_allowed() {
        assertTrue(RuntimeStateMachine.canTransition(RuntimeState.STARTING, RuntimeState.DEGRADED))
        RuntimeStateMachine.requireTransition(RuntimeState.STARTING, RuntimeState.DEGRADED)
    }

    @Test
    fun ready_to_stopping_allowed() {
        assertTrue(RuntimeStateMachine.canTransition(RuntimeState.READY, RuntimeState.STOPPING))
        RuntimeStateMachine.requireTransition(RuntimeState.READY, RuntimeState.STOPPING)
    }

    @Test
    fun degraded_to_ready_allowed() {
        assertTrue(RuntimeStateMachine.canTransition(RuntimeState.DEGRADED, RuntimeState.READY))
        RuntimeStateMachine.requireTransition(RuntimeState.DEGRADED, RuntimeState.READY)
    }

    @Test
    fun stopping_to_stopped_allowed() {
        assertTrue(RuntimeStateMachine.canTransition(RuntimeState.STOPPING, RuntimeState.STOPPED))
        RuntimeStateMachine.requireTransition(RuntimeState.STOPPING, RuntimeState.STOPPED)
    }

    @Test
    fun corrupted_to_repairing_allowed() {
        assertTrue(RuntimeStateMachine.canTransition(RuntimeState.CORRUPTED, RuntimeState.REPAIRING))
        RuntimeStateMachine.requireTransition(RuntimeState.CORRUPTED, RuntimeState.REPAIRING)
    }

    @Test
    fun repairing_to_installed_allowed() {
        assertTrue(RuntimeStateMachine.canTransition(RuntimeState.REPAIRING, RuntimeState.INSTALLED))
        RuntimeStateMachine.requireTransition(RuntimeState.REPAIRING, RuntimeState.INSTALLED)
    }

    @Test
    fun failed_to_repairing_allowed() {
        assertTrue(RuntimeStateMachine.canTransition(RuntimeState.FAILED, RuntimeState.REPAIRING))
        RuntimeStateMachine.requireTransition(RuntimeState.FAILED, RuntimeState.REPAIRING)
    }

    @Test
    fun sameStateTransition_isIdempotent() {
        assertTrue(RuntimeStateMachine.canTransition(RuntimeState.READY, RuntimeState.READY))
        RuntimeStateMachine.requireTransition(RuntimeState.READY, RuntimeState.READY)
    }

    @Test
    fun notInstalled_to_ready_forbidden() {
        assertFalse(RuntimeStateMachine.canTransition(RuntimeState.NOT_INSTALLED, RuntimeState.READY))
        try {
            RuntimeStateMachine.requireTransition(RuntimeState.NOT_INSTALLED, RuntimeState.READY)
            fail("expected exception")
        } catch (_: IllegalRuntimeTransitionException) {
        }
    }

    @Test
    fun ready_to_installing_forbidden() {
        assertFalse(RuntimeStateMachine.canTransition(RuntimeState.READY, RuntimeState.INSTALLING))
        try {
            RuntimeStateMachine.requireTransition(RuntimeState.READY, RuntimeState.INSTALLING)
            fail("expected exception")
        } catch (_: IllegalRuntimeTransitionException) {
        }
    }

    @Test
    fun stopped_to_ready_forbidden() {
        assertFalse(RuntimeStateMachine.canTransition(RuntimeState.STOPPED, RuntimeState.READY))
        try {
            RuntimeStateMachine.requireTransition(RuntimeState.STOPPED, RuntimeState.READY)
            fail("expected exception")
        } catch (_: IllegalRuntimeTransitionException) {
        }
    }

    @Test
    fun exception_containsOnly_stateNames() {
        try {
            RuntimeStateMachine.requireTransition(RuntimeState.NOT_INSTALLED, RuntimeState.READY)
            fail("expected exception")
        } catch (e: IllegalRuntimeTransitionException) {
            val msg = e.message ?: ""
            assertTrue(msg.contains("NOT_INSTALLED"))
            assertTrue(msg.contains("READY"))
            assertFalse(msg.contains("/"))
            assertFalse(msg.contains(" "))
        }
    }
}
