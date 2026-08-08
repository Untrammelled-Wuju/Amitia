package com.amitia.amitia_app.runtime.internal

import com.amitia.amitia_app.runtime.api.RuntimeState

class IllegalRuntimeTransitionException(
    val from: RuntimeState,
    val to: RuntimeState
) : IllegalStateException("${from}->${to}")

internal object RuntimeStateMachine {

    private val transitions: Map<RuntimeState, Set<RuntimeState>> = buildMap {
        put(RuntimeState.UNKNOWN, setOf(
            RuntimeState.NOT_INSTALLED,
            RuntimeState.INSTALLED,
            RuntimeState.STOPPED,
            RuntimeState.CORRUPTED,
            RuntimeState.FAILED
        ))
        put(RuntimeState.NOT_INSTALLED, setOf(
            RuntimeState.INSTALLING
        ))
        put(RuntimeState.INSTALLING, setOf(
            RuntimeState.INSTALLED,
            RuntimeState.FAILED,
            RuntimeState.NOT_INSTALLED
        ))
        put(RuntimeState.INSTALLED, setOf(
            RuntimeState.VERIFYING,
            RuntimeState.STARTING,
            RuntimeState.REPAIRING,
            RuntimeState.NOT_INSTALLED
        ))
        put(RuntimeState.VERIFYING, setOf(
            RuntimeState.INSTALLED,
            RuntimeState.CORRUPTED,
            RuntimeState.FAILED
        ))
        put(RuntimeState.STARTING, setOf(
            RuntimeState.READY,
            RuntimeState.DEGRADED,
            RuntimeState.FAILED,
            RuntimeState.STOPPING
        ))
        put(RuntimeState.READY, setOf(
            RuntimeState.READY,
            RuntimeState.DEGRADED,
            RuntimeState.STOPPING,
            RuntimeState.FAILED
        ))
        put(RuntimeState.DEGRADED, setOf(
            RuntimeState.READY,
            RuntimeState.STOPPING,
            RuntimeState.FAILED,
            RuntimeState.REPAIRING
        ))
        put(RuntimeState.STOPPING, setOf(
            RuntimeState.STOPPED,
            RuntimeState.FAILED
        ))
        put(RuntimeState.STOPPED, setOf(
            RuntimeState.VERIFYING,
            RuntimeState.STARTING,
            RuntimeState.REPAIRING,
            RuntimeState.NOT_INSTALLED
        ))
        put(RuntimeState.CORRUPTED, setOf(
            RuntimeState.REPAIRING,
            RuntimeState.NOT_INSTALLED,
            RuntimeState.FAILED
        ))
        put(RuntimeState.REPAIRING, setOf(
            RuntimeState.INSTALLED,
            RuntimeState.CORRUPTED,
            RuntimeState.FAILED,
            RuntimeState.NOT_INSTALLED
        ))
        put(RuntimeState.FAILED, setOf(
            RuntimeState.VERIFYING,
            RuntimeState.REPAIRING,
            RuntimeState.STARTING,
            RuntimeState.STOPPING,
            RuntimeState.NOT_INSTALLED
        ))
    }

    fun canTransition(from: RuntimeState, to: RuntimeState): Boolean {
        if (from == to) return true
        return transitions[from]?.contains(to) == true
    }

    fun requireTransition(from: RuntimeState, to: RuntimeState) {
        if (!canTransition(from, to)) {
            throw IllegalRuntimeTransitionException(from, to)
        }
    }

    fun unexpectedTerminationTarget(current: RuntimeState): RuntimeState? = when (current) {
        RuntimeState.INSTALLING -> RuntimeState.FAILED
        RuntimeState.STARTING -> RuntimeState.FAILED
        RuntimeState.READY -> RuntimeState.FAILED
        RuntimeState.STOPPING -> RuntimeState.STOPPED
        RuntimeState.STOPPED -> null
        RuntimeState.FAILED -> null
        else -> null
    }

    fun expectedStopTarget(current: RuntimeState): RuntimeState? = when (current) {
        RuntimeState.STOPPING -> RuntimeState.STOPPED
        RuntimeState.STOPPED -> null
        else -> null
    }
}
