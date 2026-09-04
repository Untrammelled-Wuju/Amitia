package com.amitia.amitia_app.runtime.internal

import com.amitia.amitia_app.runtime.api.RuntimeState
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class RuntimeStateMachineFailureTest {

    @Test
    fun unexpectedTerminationTarget_installing_returnsFailed() {
        assertEquals(RuntimeState.FAILED, RuntimeStateMachine.unexpectedTerminationTarget(RuntimeState.INSTALLING))
    }

    @Test
    fun unexpectedTerminationTarget_starting_returnsFailed() {
        assertEquals(RuntimeState.FAILED, RuntimeStateMachine.unexpectedTerminationTarget(RuntimeState.STARTING))
    }

    @Test
    fun unexpectedTerminationTarget_ready_returnsFailed() {
        assertEquals(RuntimeState.FAILED, RuntimeStateMachine.unexpectedTerminationTarget(RuntimeState.READY))
    }

    @Test
    fun unexpectedTerminationTarget_stopping_returnsStopped() {
        assertEquals(RuntimeState.STOPPED, RuntimeStateMachine.unexpectedTerminationTarget(RuntimeState.STOPPING))
    }

    @Test
    fun unexpectedTerminationTarget_stopped_returnsNull() {
        assertNull(RuntimeStateMachine.unexpectedTerminationTarget(RuntimeState.STOPPED))
    }

    @Test
    fun unexpectedTerminationTarget_failed_returnsNull() {
        assertNull(RuntimeStateMachine.unexpectedTerminationTarget(RuntimeState.FAILED))
    }

    @Test
    fun expectedStopTarget_stopping_returnsStopped() {
        assertEquals(RuntimeState.STOPPED, RuntimeStateMachine.expectedStopTarget(RuntimeState.STOPPING))
    }

    @Test
    fun expectedStopTarget_stopped_returnsNull() {
        assertNull(RuntimeStateMachine.expectedStopTarget(RuntimeState.STOPPED))
    }

    @Test
    fun expectedStopTarget_otherStates_returnNull() {
        assertNull(RuntimeStateMachine.expectedStopTarget(RuntimeState.READY))
        assertNull(RuntimeStateMachine.expectedStopTarget(RuntimeState.STARTING))
        assertNull(RuntimeStateMachine.expectedStopTarget(RuntimeState.INSTALLING))
    }
}
