package com.amitia.amitia_app.runtime.internal

import com.amitia.amitia_app.runtime.api.RuntimeOperationCallback
import com.amitia.amitia_app.runtime.api.RuntimeOperationResult
import com.amitia.amitia_app.runtime.api.RuntimeSnapshot
import com.amitia.amitia_app.runtime.api.RuntimeStartReason
import com.amitia.amitia_app.runtime.api.RuntimeStartRequest
import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.api.RuntimeStopReason
import com.amitia.amitia_app.runtime.api.RuntimeStopRequest
import com.amitia.amitia_app.runtime.proot.ProotSession
import com.amitia.amitia_app.runtime.recovery.InstalledRuntimeSource
import com.amitia.amitia_app.runtime.recovery.RuntimeCrashRecoveryPolicy
import com.amitia.amitia_app.runtime.recovery.RuntimeRecoveryDecision
import com.amitia.amitia_app.runtime.recovery.RuntimeRecoveryJob
import com.amitia.amitia_app.runtime.recovery.RuntimeRecoveryRequest
import com.amitia.amitia_app.runtime.recovery.RuntimeRecoveryScheduler
import com.amitia.amitia_app.runtime.service.RuntimeServiceHost
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostEvent
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener
import com.amitia.amitia_app.runtime.service.RuntimeServiceResult
import com.amitia.amitia_app.runtime.service.RuntimeServiceTerminationCause
import com.amitia.amitia_app.runtime.startup.RuntimeStartupDetector
import com.amitia.amitia_app.runtime.startup.RuntimeStartupError
import com.amitia.amitia_app.runtime.startup.RuntimeStartupRequest
import com.amitia.amitia_app.runtime.startup.RuntimeStartupResult
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicReference

internal class ControllerTestServiceHost : RuntimeServiceHost {
    private val listeners = CopyOnWriteArrayList<RuntimeServiceHostListener>()
    val events = CopyOnWriteArrayList<RuntimeServiceHostEvent>()
    val stopGenerations = CopyOnWriteArrayList<Long>()
    val ensureStartedGenerations = CopyOnWriteArrayList<Long>()
    var ensureStartResult: RuntimeServiceResult = RuntimeServiceResult.Success
    var currentSessionRef: ProotSession? = null

    override fun ensureStarted(generation: Long, profile: String): RuntimeServiceResult {
        ensureStartedGenerations.add(generation)
        return ensureStartResult
    }

    override fun requestStop(targetGeneration: Long): RuntimeServiceResult {
        stopGenerations.add(targetGeneration)
        return RuntimeServiceResult.Success
    }

    override fun requestTeardownAfterStartupFailure() {}

    override fun addListener(listener: RuntimeServiceHostListener) {
        listeners.add(listener)
    }

    override fun removeListener(listener: RuntimeServiceHostListener) {
        listeners.remove(listener)
    }

    override fun currentSession(): ProotSession? = currentSessionRef

    override fun currentGeneration(): Long = ensureStartedGenerations.lastOrNull() ?: 0L

    fun emit(event: RuntimeServiceHostEvent) {
        val snapshot = ArrayList(listeners)
        snapshot.forEach { it.onServiceHostEvent(event) }
    }
}

internal class ControllerTestStartupDetector : RuntimeStartupDetector {
    var result: RuntimeStartupResult = RuntimeStartupResult.Ready(1L, 100L, 1)
    var cancelCount = 0

    override fun awaitStartup(request: RuntimeStartupRequest): RuntimeStartupResult {
        return when (val r = result) {
            is RuntimeStartupResult.Ready -> RuntimeStartupResult.Ready(request.generation, r.elapsedMs, r.probeCount)
            is RuntimeStartupResult.Failed -> RuntimeStartupResult.Failed(request.generation, r.error, r.elapsedMs)
            is RuntimeStartupResult.Cancelled -> RuntimeStartupResult.Cancelled(request.generation)
        }
    }

    override fun cancel() {
        cancelCount++
    }
}

internal class ControllerTestRecoveryPolicy : RuntimeCrashRecoveryPolicy {
    var decision: RuntimeRecoveryDecision = RuntimeRecoveryDecision.DoNotRecover
    var evaluateCount = 0
    var recordReadyCount = 0

    override fun evaluate(request: RuntimeRecoveryRequest): RuntimeRecoveryDecision {
        evaluateCount++
        return decision
    }

    override fun recordReady(generation: Long) {
        recordReadyCount++
    }

    override fun cancelPending() {}
}

internal class ControllerTestRecoveryScheduler : RuntimeRecoveryScheduler {
    val scheduledJobs = CopyOnWriteArrayList<() -> Unit>()

    override fun schedule(delayMillis: Long, action: () -> Unit): RuntimeRecoveryJob {
        scheduledJobs.add(action)
        return object : RuntimeRecoveryJob {
            override fun cancel() { scheduledJobs.remove(action) }
            override val isCancelled: Boolean = !scheduledJobs.contains(action)
        }
    }

    fun drain() {
        val jobs = ArrayList(scheduledJobs)
        scheduledJobs.clear()
        jobs.forEach { it() }
    }
}

internal class ControllerTestInstalledRuntimeSource : InstalledRuntimeSource {
    override fun current() = com.amitia.amitia_app.runtime.recovery.InstalledRuntimeResult.Installed
}

class DefaultRuntimeControllerTest {

    private fun createStateStore(initialState: RuntimeState): RuntimeStateStore {
        val store = RuntimeStateStore()
        store.update {
            RuntimeSnapshot(
                state = initialState,
                runtimeVersion = null,
                activeOperationId = null,
                activeOperationType = null,
                progress = it.progress,
                components = it.components,
                lastError = null,
                generation = it.generation,
                updatedAtEpochMillis = it.updatedAtEpochMillis
            )
        }
        return store
    }

    private fun createStoppedStateStore(): RuntimeStateStore = createStateStore(RuntimeState.STOPPED)

    private fun createReadyStateStore(generation: Long): RuntimeStateStore {
        val store = RuntimeStateStore()
        store.update { it.copy(state = RuntimeState.INSTALLED) }
        store.update { it.copy(state = RuntimeState.STARTING) }
        store.update { it.copy(state = RuntimeState.READY, generation = generation) }
        return store
    }

    private fun createController(
        stateStore: RuntimeStateStore,
        host: ControllerTestServiceHost,
        detector: ControllerTestStartupDetector = ControllerTestStartupDetector(),
        policy: ControllerTestRecoveryPolicy = ControllerTestRecoveryPolicy(),
        scheduler: ControllerTestRecoveryScheduler = ControllerTestRecoveryScheduler(),
    ): DefaultRuntimeController {
        return DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            startupDetector = detector,
            recoveryPolicy = policy,
            recoveryScheduler = scheduler,
            installedRuntimeSource = ControllerTestInstalledRuntimeSource(),
        )
    }

    private fun startAndCapture(controller: DefaultRuntimeController): RuntimeOperationResult {
        val resultRef = AtomicReference<RuntimeOperationResult>()
        val latch = CountDownLatch(1)
        val callback = object : RuntimeOperationCallback {
            override fun onCompleted(result: RuntimeOperationResult) {
                resultRef.set(result)
                latch.countDown()
            }
        }
        controller.start(RuntimeStartRequest(reason = RuntimeStartReason.USER_REQUEST), callback)
        latch.await(5, TimeUnit.SECONDS)
        return resultRef.get()
    }

    private fun stopAndCapture(controller: DefaultRuntimeController): RuntimeOperationResult {
        val resultRef = AtomicReference<RuntimeOperationResult>()
        val latch = CountDownLatch(1)
        val callback = object : RuntimeOperationCallback {
            override fun onCompleted(result: RuntimeOperationResult) {
                resultRef.set(result)
                latch.countDown()
            }
        }
        controller.stop(RuntimeStopRequest(reason = RuntimeStopReason.USER_REQUEST, force = false), callback)
        latch.await(5, TimeUnit.SECONDS)
        return resultRef.get()
    }

    @Test
    fun l001_freshStart_startingToReady_viaStartupDetection() {
        val stateStore = createStoppedStateStore()
        val host = ControllerTestServiceHost()
        val detector = ControllerTestStartupDetector()
        detector.result = RuntimeStartupResult.Ready(1L, 50L, 1)
        val controller = createController(stateStore, host, detector)

        val result = startAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Success)

        val currentGen = controller.snapshot().generation
        host.emit(RuntimeServiceHostEvent.SessionReady(generation = currentGen, sessionId = "test"))

        assertEquals(RuntimeState.READY, controller.snapshot().state)
    }

    @Test
    fun l006_watcherFatalFailure_failClosedNoRecovery() {
        val stateStore = createReadyStateStore(1L)
        val host = ControllerTestServiceHost()
        val policy = ControllerTestRecoveryPolicy()
        policy.decision = RuntimeRecoveryDecision.DoNotRecover
        val controller = createController(stateStore, host, policy = policy)

        host.emit(
            RuntimeServiceHostEvent.UnexpectedTermination(
                generation = 1L,
                cause = RuntimeServiceTerminationCause.EXIT_WATCHER_FAILED,
            )
        )

        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
        assertEquals(0, host.ensureStartedGenerations.size)
    }

    @Test
    fun l008_startingCrash_failedAndRecovery() {
        val stateStore = createStateStore(RuntimeState.STOPPED)
        val host = ControllerTestServiceHost()
        val policy = ControllerTestRecoveryPolicy()
        policy.decision = RuntimeRecoveryDecision.RecoverAfter(0L)
        val scheduler = ControllerTestRecoveryScheduler()
        val controller = createController(stateStore, host, policy = policy, scheduler = scheduler)

        val result = startAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Success)

        val currentGen = controller.snapshot().generation
        host.emit(
            RuntimeServiceHostEvent.UnexpectedTermination(
                generation = currentGen,
                cause = RuntimeServiceTerminationCause.SESSION_EXITED,
            )
        )

        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
        assertTrue(scheduler.scheduledJobs.isNotEmpty())
    }

    @Test
    fun l009_readyCrash_unexpectedTerminationTriggersRecovery() {
        val stateStore = createReadyStateStore(5L)
        val host = ControllerTestServiceHost()
        val policy = ControllerTestRecoveryPolicy()
        policy.decision = RuntimeRecoveryDecision.RecoverAfter(0L)
        val scheduler = ControllerTestRecoveryScheduler()
        val controller = createController(stateStore, host, policy = policy, scheduler = scheduler)

        host.emit(
            RuntimeServiceHostEvent.UnexpectedTermination(
                generation = 5L,
                cause = RuntimeServiceTerminationCause.SESSION_EXITED,
            )
        )

        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
        assertTrue(scheduler.scheduledJobs.isNotEmpty())
    }

    @Test
    fun l010_startupTimeout_triggersServiceTeardownAndFailure() {
        val stateStore = createStoppedStateStore()
        val host = ControllerTestServiceHost()
        val detector = ControllerTestStartupDetector()
        detector.result = RuntimeStartupResult.Failed(
            generation = 1L,
            error = RuntimeStartupError.Timeout(90_000L, 90_000L, 600),
        )
        val controller = createController(stateStore, host, detector)

        val result = startAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Success)

        val currentGen = controller.snapshot().generation
        host.emit(RuntimeServiceHostEvent.SessionReady(generation = currentGen, sessionId = "timeout-test"))

        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
    }

    @Test
    fun l011_staleEvent_ignoredByCurrentGeneration() {
        val stateStore = createReadyStateStore(10L)
        val host = ControllerTestServiceHost()
        val controller = createController(stateStore, host)

        host.emit(
            RuntimeServiceHostEvent.UnexpectedTermination(
                generation = 5L,
                cause = RuntimeServiceTerminationCause.SESSION_EXITED,
            )
        )

        assertEquals(RuntimeState.READY, controller.snapshot().state)
    }

    @Test
    fun l016_startFromStarting_returnsAlreadyRunning() {
        val stateStore = createStoppedStateStore()
        val host = ControllerTestServiceHost()
        val controller = createController(stateStore, host)

        val result1 = startAndCapture(controller)
        assertTrue(result1 is RuntimeOperationResult.Success)

        val result2 = startAndCapture(controller)
        assertTrue(result2 is RuntimeOperationResult.Failure)
    }

    @Test
    fun expectedStop_clearsExpectedStopAndTransitionsToStopped() {
        val stateStore = createStateStore(RuntimeState.STOPPED)
        val host = ControllerTestServiceHost()
        val controller = createController(stateStore, host)

        startAndCapture(controller)
        val gen = controller.snapshot().generation

        stopAndCapture(controller)
        assertTrue(controller.snapshot().state == RuntimeState.STOPPING)

        host.emit(RuntimeServiceHostEvent.ExpectedStopped(gen))
        assertEquals(RuntimeState.STOPPED, controller.snapshot().state)
    }

    @Test
    fun startupDetection_cancelsOnStop() {
        val stateStore = createStoppedStateStore()
        val host = ControllerTestServiceHost()
        val detector = ControllerTestStartupDetector()
        detector.result = RuntimeStartupResult.Cancelled(1L)
        val controller = createController(stateStore, host, detector)

        startAndCapture(controller)
        val gen = controller.snapshot().generation

        host.emit(RuntimeServiceHostEvent.SessionReady(generation = gen, sessionId = "cancel-test"))
        stopAndCapture(controller)

        assertTrue(detector.cancelCount > 0)
    }

    @Test
    fun readyRecordsRecoveryReady() {
        val stateStore = createStoppedStateStore()
        val host = ControllerTestServiceHost()
        val detector = ControllerTestStartupDetector()
        detector.result = RuntimeStartupResult.Ready(1L, 100L, 1)
        val policy = ControllerTestRecoveryPolicy()
        val controller = createController(stateStore, host, detector, policy)

        startAndCapture(controller)
        val gen = controller.snapshot().generation

        host.emit(RuntimeServiceHostEvent.SessionReady(generation = gen, sessionId = "ready-test"))

        assertEquals(1, policy.recordReadyCount)
    }
}
