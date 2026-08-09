package com.amitia.amitia_app.runtime.recovery

import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.internal.DefaultRuntimeController
import com.amitia.amitia_app.runtime.internal.RuntimeStateStore
import com.amitia.amitia_app.runtime.service.RuntimeServiceHost
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostEvent
import com.amitia.amitia_app.runtime.service.RuntimeServiceTerminationCause
import com.amitia.amitia_app.runtime.startup.RuntimeStartupDetector
import com.amitia.amitia_app.runtime.startup.RuntimeStartupRequest
import com.amitia.amitia_app.runtime.startup.RuntimeStartupResult
import org.junit.Assert.assertEquals
import org.junit.Test
import java.util.concurrent.atomic.AtomicInteger

internal class FakeRecoveryScheduler : RuntimeRecoveryScheduler {
    val scheduledJobs = mutableListOf<ScheduleRequest>()
    var nextJobId = AtomicInteger(0)

    data class ScheduleRequest(
        val delayMillis: Long,
        val action: () -> Unit,
        val id: Int,
    )

    override fun schedule(delayMillis: Long, action: () -> Unit): RuntimeRecoveryJob {
        val id = nextJobId.incrementAndGet()
        scheduledJobs.add(ScheduleRequest(delayMillis, action, id))
        return object : RuntimeRecoveryJob {
            override fun cancel() { }
            override val isCancelled: Boolean = false
        }
    }
}

internal class FakeStartupDetector : RuntimeStartupDetector {
    override fun awaitStartup(request: RuntimeStartupRequest): RuntimeStartupResult {
        return RuntimeStartupResult.Cancelled(request.generation)
    }
    override fun cancel() { }
}

internal class FakeServiceHostWithCrashes : RuntimeServiceHost {
    private val listeners = mutableListOf<com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener>()

    override fun ensureStarted() = com.amitia.amitia_app.runtime.service.RuntimeServiceResult.Success
    override fun requestStop() = com.amitia.amitia_app.runtime.service.RuntimeServiceResult.Success
    override fun addListener(listener: com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener) {
        listeners.add(listener)
    }
    override fun removeListener(listener: com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener) {
        listeners.remove(listener)
    }
    fun emit(event: RuntimeServiceHostEvent) {
        val snapshot = ArrayList(listeners)
        snapshot.forEach { it.onServiceHostEvent(event) }
    }
}

internal class FakeProotComponentNoSession : com.amitia.amitia_app.runtime.proot.ProotComponent {
    override fun availability() = com.amitia.amitia_app.runtime.proot.ProotAvailability.Unavailable(
        com.amitia.amitia_app.runtime.proot.ProotErrorCode.BINARY_NOT_FOUND, "test"
    )
    override fun launch(request: com.amitia.amitia_app.runtime.proot.ProotLaunchRequest, observer: com.amitia.amitia_app.runtime.proot.ProotObserver) =
        FakeProotSession()
    override fun launchProbe(request: com.amitia.amitia_app.runtime.proot.ProotLaunchRequest, observer: com.amitia.amitia_app.runtime.proot.ProotObserver) =
        FakeProotSession()
    override fun currentSession(): com.amitia.amitia_app.runtime.proot.ProotSession? = null
    override fun stop() = com.amitia.amitia_app.runtime.proot.ProotStopResult.AlreadyStopped("none", null)
    override fun close() {}
}

internal class FakeProotSession(
    override val sessionId: String = "fake-session",
) : com.amitia.amitia_app.runtime.proot.ProotSession {
    override fun isAlive(): Boolean = false
    override fun awaitExit(timeoutMillis: Long): Int? = 0
    override fun stop(graceMillis: Long) = com.amitia.amitia_app.runtime.proot.ProotStopResult.AlreadyStopped(sessionId, 0)
    override fun close() {}
}

class RuntimeCrashRecoveryControllerTest {

    private fun createStateStoreWith(targetState: RuntimeState): RuntimeStateStore {
        val store = RuntimeStateStore()
        val path = when (targetState) {
            RuntimeState.FAILED -> listOf(
                RuntimeState.INSTALLED, RuntimeState.STARTING, RuntimeState.FAILED
            )
            RuntimeState.STARTING -> listOf(
                RuntimeState.INSTALLED, RuntimeState.STARTING
            )
            RuntimeState.STOPPING -> listOf(
                RuntimeState.INSTALLED, RuntimeState.STARTING, RuntimeState.STOPPING
            )
            RuntimeState.STOPPED -> listOf(
                RuntimeState.INSTALLED, RuntimeState.STARTING, RuntimeState.STOPPING, RuntimeState.STOPPED
            )
            else -> listOf()
        }
        for (s in path) {
            store.update { it.copy(state = s) }
        }
        return store
    }

    private fun createController(
        stateStore: RuntimeStateStore,
        host: FakeServiceHostWithCrashes,
        policy: RuntimeCrashRecoveryPolicy,
        scheduler: FakeRecoveryScheduler,
    ): DefaultRuntimeController {
        return DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            startupDetector = FakeStartupDetector(),
            recoveryPolicy = policy,
            recoveryScheduler = scheduler,
        )
    }

    @Test
    fun unexpectedTermination_evaluatesRecovery() {
        val stateStore = createStateStoreWith(RuntimeState.STARTING)
        val host = FakeServiceHostWithCrashes()
        val scheduler = FakeRecoveryScheduler()
        val policy = object : RuntimeCrashRecoveryPolicy {
            override fun evaluate(request: RuntimeRecoveryRequest): RuntimeRecoveryDecision {
                return RuntimeRecoveryDecision.RecoverAfter(2000L)
            }
            override fun recordReady(generation: Long) { }
            override fun cancelPending() { }
        }
        val controller = createController(stateStore, host, policy, scheduler)
        host.emit(RuntimeServiceHostEvent.UnexpectedTermination(
            RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR
        ))
        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
        assertEquals(1, scheduler.scheduledJobs.size)
        assertEquals(2000L, scheduler.scheduledJobs[0].delayMillis)
    }

    @Test
    fun expectedStopped_noRecoveryScheduled() {
        val stateStore = createStateStoreWith(RuntimeState.STOPPING)
        val host = FakeServiceHostWithCrashes()
        val scheduler = FakeRecoveryScheduler()
        val policy = object : RuntimeCrashRecoveryPolicy {
            override fun evaluate(request: RuntimeRecoveryRequest): RuntimeRecoveryDecision {
                return RuntimeRecoveryDecision.RecoverAfter(1000L)
            }
            override fun recordReady(generation: Long) { }
            override fun cancelPending() { }
        }
        val controller = createController(stateStore, host, policy, scheduler)
        host.emit(RuntimeServiceHostEvent.ExpectedStopped)
        assertEquals(RuntimeState.STOPPED, controller.snapshot().state)
        assertEquals(0, scheduler.scheduledJobs.size)
    }

    @Test
    fun doNotRecovery_noJobScheduled() {
        val stateStore = createStateStoreWith(RuntimeState.STARTING)
        val host = FakeServiceHostWithCrashes()
        val scheduler = FakeRecoveryScheduler()
        val policy = object : RuntimeCrashRecoveryPolicy {
            override fun evaluate(request: RuntimeRecoveryRequest): RuntimeRecoveryDecision {
                return RuntimeRecoveryDecision.DoNotRecover
            }
            override fun recordReady(generation: Long) { }
            override fun cancelPending() { }
        }
        val controller = createController(stateStore, host, policy, scheduler)
        host.emit(RuntimeServiceHostEvent.UnexpectedTermination(
            RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR
        ))
        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
        assertEquals(0, scheduler.scheduledJobs.size)
    }
}
