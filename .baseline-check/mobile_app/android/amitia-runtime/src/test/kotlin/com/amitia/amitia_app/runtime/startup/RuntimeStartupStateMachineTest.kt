package com.amitia.amitia_app.runtime.startup

import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.internal.RuntimeStateMachine
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Test

class RuntimeStartupStateMachineTest {

    @Test
    fun startupReadyTarget_fromStarting_returnsReady() {
        val target = RuntimeStateMachine.startupReadyTarget(RuntimeState.STARTING)
        assertEquals(RuntimeState.READY, target)
    }

    @Test
    fun startupReadyTarget_fromStopped_returnsNull() {
        val target = RuntimeStateMachine.startupReadyTarget(RuntimeState.STOPPED)
        assertNull(target)
    }

    @Test
    fun startupReadyTarget_fromReady_returnsNull() {
        val target = RuntimeStateMachine.startupReadyTarget(RuntimeState.READY)
        assertNull(target)
    }

    @Test
    fun startupFailureTarget_fromStarting_returnsFailed() {
        val target = RuntimeStateMachine.startupFailureTarget(RuntimeState.STARTING)
        assertEquals(RuntimeState.FAILED, target)
    }

    @Test
    fun startupFailureTarget_fromStopping_returnsFailed() {
        val target = RuntimeStateMachine.startupFailureTarget(RuntimeState.STOPPING)
        assertEquals(RuntimeState.FAILED, target)
    }

    @Test
    fun startupFailureTarget_fromStopped_returnsNull() {
        val target = RuntimeStateMachine.startupFailureTarget(RuntimeState.STOPPED)
        assertNull(target)
    }

    @Test
    fun requireTransitionTo_validTransition_returnsNull() {
        val error = RuntimeStateMachine.requireTransitionTo(RuntimeState.STARTING, RuntimeState.READY)
        assertNull(error)
    }

    @Test
    fun requireTransitionTo_invalidTransition_returnsError() {
        val error = RuntimeStateMachine.requireTransitionTo(RuntimeState.STOPPED, RuntimeState.READY)
        assertNotNull(error)
        assertEquals(RuntimeState.STOPPED, error?.from)
        assertEquals(RuntimeState.READY, error?.to)
    }

    @Test
    fun startupReady_thenFailure_isValidFlow() {
        val readyTarget = RuntimeStateMachine.startupReadyTarget(RuntimeState.STARTING)
        assertNotNull(readyTarget)
        val canTransition = RuntimeStateMachine.canTransition(RuntimeState.STARTING, readyTarget!!)
        assertEquals(true, canTransition)
    }
}
