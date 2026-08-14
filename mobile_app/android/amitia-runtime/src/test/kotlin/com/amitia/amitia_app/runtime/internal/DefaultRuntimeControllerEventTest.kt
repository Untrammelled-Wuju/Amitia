package com.amitia.amitia_app.runtime.internal

import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.recovery.InstalledRuntimeSource
import com.amitia.amitia_app.runtime.recovery.RuntimeCrashRecoveryPolicy
import com.amitia.amitia_app.runtime.recovery.RuntimeRecoveryDecision
import com.amitia.amitia_app.runtime.recovery.RuntimeRecoveryRequest
import com.amitia.amitia_app.runtime.recovery.RuntimeRecoveryScheduler
import com.amitia.amitia_app.runtime.recovery.RuntimeRecoveryJob
import com.amitia.amitia_app.runtime.service.RuntimeServiceHost
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostEvent
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener
import com.amitia.amitia_app.runtime.service.RuntimeServiceResult
import com.amitia.amitia_app.runtime.service.RuntimeServiceTerminationCause
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

internal class FakeRuntimeServiceHost : RuntimeServiceHost {
    private val listeners = mutableListOf<RuntimeServiceHostListener>()
    override fun ensureStarted(generation: Long) = RuntimeServiceResult.Success
    override fun requestStop(targetGeneration: Long) = RuntimeServiceResult.Success
    override fun requestTeardownAfterStartupFailure() {}
    override fun addListener(listener: RuntimeServiceHostListener) { listeners.add(listener) }
    override fun removeListener(listener: RuntimeServiceHostListener) { listeners.remove(listener) }
    override fun currentSession(): com.amitia.amitia_app.runtime.proot.ProotSession? = null
    override fun currentGeneration(): Long = 0L
    fun emit(event: RuntimeServiceHostEvent) {
        val snapshot = ArrayList(listeners)
        snapshot.forEach { it.onServiceHostEvent(event) }
    }
    fun listenersSize(): Int = listeners.size
}

internal val noRecoveryPolicy = object : RuntimeCrashRecoveryPolicy {
    override fun evaluate(request: RuntimeRecoveryRequest): RuntimeRecoveryDecision =
        RuntimeRecoveryDecision.DoNotRecover
    override fun recordReady(generation: Long) {}
    override fun cancelPending() {}
}

internal val alwaysRecoverPolicy = object : RuntimeCrashRecoveryPolicy {
    override fun evaluate(request: RuntimeRecoveryRequest): RuntimeRecoveryDecision =
        RuntimeRecoveryDecision.RecoverAfter(50L)
    override fun recordReady(generation: Long) {}
    override fun cancelPending() {}
}

internal val immediateScheduler = object : RuntimeRecoveryScheduler {
    override fun schedule(delayMillis: Long, action: () -> Unit): RuntimeRecoveryJob {
        action()
        return object : RuntimeRecoveryJob {
            override fun cancel() {}
            override val isCancelled: Boolean = false
        }
    }
}

internal val manualScheduler = object : RuntimeRecoveryScheduler {
    private val pending = mutableListOf<() -> Unit>()
    override fun schedule(delayMillis: Long, action: () -> Unit): RuntimeRecoveryJob {
        pending.add(action)
        return object : RuntimeRecoveryJob {
            override fun cancel() { pending.remove(action) }
            override val isCancelled: Boolean = !pending.contains(action)
        }
    }
    fun drain() {
        val jobs = ArrayList(pending)
        pending.clear()
        jobs.forEach { it() }
    }
}

internal val installedSource = object : InstalledRuntimeSource {
    override fun current() = com.amitia.amitia_app.runtime.recovery.InstalledRuntimeResult.Installed
}

internal fun defaultTestController(
    stateStore: RuntimeStateStore,
    host: FakeRuntimeServiceHost,
    policy: RuntimeCrashRecoveryPolicy = noRecoveryPolicy,
    scheduler: RuntimeRecoveryScheduler = immediateScheduler,
): DefaultRuntimeController {
    return DefaultRuntimeController(
        stateStore = stateStore,
        serviceHost = host,
        recoveryPolicy = policy,
        recoveryScheduler = scheduler,
        installedRuntimeSource = installedSource,
    )
}

class DefaultRuntimeControllerEventTest {

    private fun stateStoreWith(targetState: RuntimeState): RuntimeStateStore {
        val store = RuntimeStateStore()
        val path = when (targetState) {
            RuntimeState.READY -> listOf(RuntimeState.INSTALLED, RuntimeState.STARTING, RuntimeState.READY)
            RuntimeState.STARTING -> listOf(RuntimeState.INSTALLED, RuntimeState.STARTING)
            RuntimeState.STOPPING -> listOf(RuntimeState.INSTALLED, RuntimeState.STARTING, RuntimeState.STOPPING)
            else -> listOf()
        }
        for (s in path) {
            store.update { it.copy(state = s) }
        }
        return store
    }

    @Test
    fun controller_receivesUnexpectedTermination_fromReadyGoesToFailed() {
        val stateStore = stateStoreWith(RuntimeState.READY)
        val host = FakeRuntimeServiceHost()
        val controller = defaultTestController(stateStore = stateStore, host = host)
        host.emit(
            RuntimeServiceHostEvent.UnexpectedTermination(
                generation = controller.snapshot().generation,
                cause = RuntimeServiceTerminationCause.FOREGROUND_FAILED
            )
        )
        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
    }

    @Test
    fun controller_doesNotAutoRestart_fromStartingGoesToFailed() {
        val stateStore = stateStoreWith(RuntimeState.STARTING)
        val host = FakeRuntimeServiceHost()
        val controller = defaultTestController(stateStore = stateStore, host = host)
        host.emit(
            RuntimeServiceHostEvent.UnexpectedTermination(
                generation = controller.snapshot().generation,
                cause = RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR
            )
        )
        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
    }

    @Test
    fun expectedStop_fromStoppingGoesToStopped() {
        val stateStore = stateStoreWith(RuntimeState.STOPPING)
        val host = FakeRuntimeServiceHost()
        val controller = defaultTestController(stateStore = stateStore, host = host)
        host.emit(RuntimeServiceHostEvent.ExpectedStopped(generation = controller.snapshot().generation))
        assertEquals(RuntimeState.STOPPED, controller.snapshot().state)
    }

    @Test
    fun unexpectedTermination_setsLastError() {
        val stateStore = stateStoreWith(RuntimeState.READY)
        val host = FakeRuntimeServiceHost()
        val controller = defaultTestController(stateStore = stateStore, host = host)
        host.emit(
            RuntimeServiceHostEvent.UnexpectedTermination(
                generation = controller.snapshot().generation,
                cause = RuntimeServiceTerminationCause.NOTIFICATION_FAILED
            )
        )
        assertTrue(controller.snapshot().lastError != null)
    }

    @Test
    fun addListener_registersMultipleControllers() {
        val host = FakeRuntimeServiceHost()
        val stateStore1 = RuntimeStateStore()
        val stateStore2 = RuntimeStateStore()
        defaultTestController(stateStore = stateStore1, host = host)
        defaultTestController(stateStore = stateStore2, host = host)
        assertEquals(2, host.listenersSize())
    }
}
