package com.amitia.amitia_app.runtime.recovery

import com.amitia.amitia_app.runtime.api.RuntimeErrorCode
import com.amitia.amitia_app.runtime.api.RuntimeOperationResult
import com.amitia.amitia_app.runtime.api.RuntimeStartReason
import com.amitia.amitia_app.runtime.api.RuntimeStartRequest
import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.api.RuntimeStopReason
import com.amitia.amitia_app.runtime.api.RuntimeStopRequest
import com.amitia.amitia_app.runtime.internal.DefaultRuntimeController
import com.amitia.amitia_app.runtime.internal.RuntimeStateStore
import com.amitia.amitia_app.runtime.proot.ProotSession
import com.amitia.amitia_app.runtime.service.RuntimeServiceHost
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostEvent
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener
import com.amitia.amitia_app.runtime.service.RuntimeServiceResult
import com.amitia.amitia_app.runtime.service.RuntimeServiceTerminationCause
import com.amitia.amitia_app.runtime.startup.RuntimeStartupDetector
import com.amitia.amitia_app.runtime.startup.RuntimeStartupRequest
import com.amitia.amitia_app.runtime.startup.RuntimeStartupResult
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicReference

internal class FakeRecoveryTestHost : RuntimeServiceHost {
    private val listeners = CopyOnWriteArrayList<RuntimeServiceHostListener>()
    val ensureStartedCalls = AtomicInteger(0)
    val startReasons = CopyOnWriteArrayList<RuntimeStartReason>()
    var sessionOverride: ProotSession? = null

    override fun ensureStarted(generation: Long): RuntimeServiceResult {
        ensureStartedCalls.incrementAndGet()
        return RuntimeServiceResult.Success
    }
    override fun requestStop(targetGeneration: Long): RuntimeServiceResult = RuntimeServiceResult.Success
    override fun requestTeardownAfterStartupFailure() {}
    override fun addListener(listener: RuntimeServiceHostListener) { listeners.add(listener) }
    override fun removeListener(listener: RuntimeServiceHostListener) { listeners.remove(listener) }
    override fun currentSession(): ProotSession? = sessionOverride
    override fun currentGeneration(): Long = 0L
    fun emit(event: RuntimeServiceHostEvent) {
        listeners.forEach { it.onServiceHostEvent(event) }
    }
}

internal class RecordingRecoveryPolicy(
    private val decisions: List<RuntimeRecoveryDecision>,
) : RuntimeCrashRecoveryPolicy {
    val evaluateCalls = AtomicInteger(0)
    val recordReadyCalls = CopyOnWriteArrayList<Long>()
    val cancelPendingCalls = AtomicInteger(0)
    private val decisionIndex = AtomicInteger(0)

    override fun evaluate(request: RuntimeRecoveryRequest): RuntimeRecoveryDecision {
        evaluateCalls.incrementAndGet()
        val idx = decisionIndex.getAndIncrement()
        return if (idx < decisions.size) decisions[idx] else RuntimeRecoveryDecision.Exhausted(idx)
    }
    override fun recordReady(generation: Long) {
        recordReadyCalls.add(generation)
    }
    override fun cancelPending() {
        cancelPendingCalls.incrementAndGet()
    }
}

internal class ControllableScheduler : RuntimeRecoveryScheduler {
    val scheduledJobs = CopyOnWriteArrayList<() -> Unit>()
    val cancelledJobs = AtomicInteger(0)
    var delayOfLastSchedule: Long = -1L

    override fun schedule(delayMillis: Long, action: () -> Unit): RuntimeRecoveryJob {
        delayOfLastSchedule = delayMillis
        scheduledJobs.add(action)
        return object : RuntimeRecoveryJob {
            val cancelled = AtomicBoolean(false)
            override fun cancel() {
                cancelled.set(true)
                scheduledJobs.remove(action)
                cancelledJobs.incrementAndGet()
            }
            override val isCancelled: Boolean get() = cancelled.get()
        }
    }

    fun drainAll() {
        val jobs = ArrayList(scheduledJobs)
        scheduledJobs.clear()
        jobs.forEach { it() }
    }

    fun drainSingle(): Boolean {
        if (scheduledJobs.isEmpty()) return false
        val job = scheduledJobs.removeAt(0)
        job()
        return true
    }
}

internal class FakeReadyStartupDetector(
    private val result: RuntimeStartupResult,
) : RuntimeStartupDetector {
    override fun awaitStartup(request: RuntimeStartupRequest): RuntimeStartupResult = result
    override fun cancel() {}
}

class RuntimeCrashRecoveryIntegrationTest {

    private fun stateStoreAt(targetState: RuntimeState): RuntimeStateStore {
        val store = RuntimeStateStore()
        store.update { it.copy(state = RuntimeState.INSTALLED, generation = 1L) }
        when (targetState) {
            RuntimeState.STARTING -> store.update { it.copy(state = RuntimeState.STARTING, generation = 2L) }
            RuntimeState.STOPPING -> {
                store.update { it.copy(state = RuntimeState.STARTING, generation = 2L) }
                store.update { it.copy(state = RuntimeState.STOPPING, generation = 3L) }
            }
            RuntimeState.STOPPED -> {
                store.update { it.copy(state = RuntimeState.STARTING, generation = 2L) }
                store.update { it.copy(state = RuntimeState.STOPPING, generation = 3L) }
                store.update { it.copy(state = RuntimeState.STOPPED, generation = 4L) }
            }
            RuntimeState.READY -> {
                store.update { it.copy(state = RuntimeState.STARTING, generation = 2L) }
                store.update { it.copy(state = RuntimeState.READY, generation = 3L) }
            }
            RuntimeState.FAILED -> {
                store.update { it.copy(state = RuntimeState.STARTING, generation = 2L) }
                store.update { it.copy(state = RuntimeState.FAILED, generation = 3L) }
            }
            else -> {}
        }
        return store
    }

    private fun makeController(
        state: RuntimeState,
        policy: RuntimeCrashRecoveryPolicy,
        scheduler: RuntimeRecoveryScheduler,
        detector: RuntimeStartupDetector? = null,
    ): Pair<DefaultRuntimeController, FakeRecoveryTestHost> {
        val store = stateStoreAt(state)
        val host = FakeRecoveryTestHost()
        val controller = DefaultRuntimeController(
            stateStore = store,
            serviceHost = host,
            recoveryPolicy = policy,
            recoveryScheduler = scheduler,
            installedRuntimeSource = FixedInstalledSource(),
            startupDetector = detector ?: FakeReadyStartupDetector(
                RuntimeStartupResult.Ready(generation = store.snapshot().generation, elapsedMs = 1L, probeCount = 1)
            ),
        )
        return Pair(controller, host)
    }

    @Test
    fun unexpectedExit_fromReady_evaluatesRecovery() {
        val policy = RecordingRecoveryPolicy(listOf(RuntimeRecoveryDecision.DoNotRecover))
        val scheduler = ControllableScheduler()
        val (controller, host) = makeController(RuntimeState.READY, policy, scheduler)

        val genBefore = controller.snapshot().generation
        host.emit(RuntimeServiceHostEvent.UnexpectedTermination(
            generation = controller.snapshot().generation,
            cause = RuntimeServiceTerminationCause.SESSION_EXITED
        ))

        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
        assertEquals(1, policy.evaluateCalls.get())
        assertEquals(0, scheduler.scheduledJobs.size)
        assertTrue(controller.snapshot().generation > genBefore)
    }

    @Test
    fun unexpectedExit_withRecoverAfter_schedulesRecoveryJob() {
        val policy = RecordingRecoveryPolicy(listOf(RuntimeRecoveryDecision.RecoverAfter(1000L)))
        val scheduler = ControllableScheduler()
        val (controller, host) = makeController(RuntimeState.READY, policy, scheduler)

        host.emit(RuntimeServiceHostEvent.UnexpectedTermination(
            generation = controller.snapshot().generation,
            cause = RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR
        ))

        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
        assertEquals(1, policy.evaluateCalls.get())
        assertEquals(1, scheduler.scheduledJobs.size)
        assertEquals(1000L, scheduler.delayOfLastSchedule)
    }

    @Test
    fun unexpectedExit_withExhausted_setsRecoveryExhaustedError() {
        val policy = RecordingRecoveryPolicy(listOf(RuntimeRecoveryDecision.Exhausted(3)))
        val scheduler = ControllableScheduler()
        val (controller, host) = makeController(RuntimeState.READY, policy, scheduler)

        host.emit(RuntimeServiceHostEvent.UnexpectedTermination(
            generation = controller.snapshot().generation,
            cause = RuntimeServiceTerminationCause.FOREGROUND_FAILED
        ))

        assertEquals(1, policy.evaluateCalls.get())
        assertEquals(0, scheduler.scheduledJobs.size)
        val lastErr = controller.snapshot().lastError
        assertNotNull(lastErr)
        assertEquals(RuntimeErrorCode.RECOVERY_EXHAUSTED, lastErr!!.code)
        assertTrue(!lastErr.recoverable)
    }

    @Test
    fun expectedStop_fromStopping_doesNotEvaluateRecovery() {
        val policy = RecordingRecoveryPolicy(listOf(RuntimeRecoveryDecision.RecoverAfter(100L)))
        val scheduler = ControllableScheduler()
        val (controller, host) = makeController(RuntimeState.STOPPING, policy, scheduler)

        host.emit(RuntimeServiceHostEvent.ExpectedStopped(generation = controller.snapshot().generation))

        assertEquals(RuntimeState.STOPPED, controller.snapshot().state)
        assertEquals(0, policy.evaluateCalls.get())
        assertEquals(0, scheduler.scheduledJobs.size)
        assertNull(controller.snapshot().lastError)
    }

    @Test
    fun stop_fromFailed_cancelsPendingRecoveryAndTransitionsToStopping() {
        val policy = RecordingRecoveryPolicy(listOf(
            RuntimeRecoveryDecision.RecoverAfter(5000L),
            RuntimeRecoveryDecision.RecoverAfter(5000L),
        ))
        val scheduler = ControllableScheduler()
        val (controller, host) = makeController(RuntimeState.READY, policy, scheduler)

        host.emit(RuntimeServiceHostEvent.UnexpectedTermination(
            generation = controller.snapshot().generation,
            cause = RuntimeServiceTerminationCause.SESSION_EXITED
        ))
        assertEquals(1, scheduler.scheduledJobs.size)

        val latch = CountDownLatch(1)
        controller.stop(RuntimeStopRequest(RuntimeStopReason.USER_REQUEST, force = false)) { latch.countDown() }
        latch.await(2, TimeUnit.SECONDS)

        assertTrue(policy.cancelPendingCalls.get() >= 1)
        assertEquals(0, scheduler.scheduledJobs.size)
        assertEquals(1, scheduler.cancelledJobs.get())
        assertEquals(RuntimeState.STOPPING, controller.snapshot().state)
    }

    @Test
    fun manualStart_fromStoppedAfterRestore_clearsPendingAndStarts() {
        val policy = RecordingRecoveryPolicy(listOf(
            RuntimeRecoveryDecision.RecoverAfter(5000L),
        ))
        val scheduler = ControllableScheduler()
        val (controller, host) = makeController(RuntimeState.READY, policy, scheduler)

        host.emit(RuntimeServiceHostEvent.UnexpectedTermination(
            generation = controller.snapshot().generation,
            cause = RuntimeServiceTerminationCause.SESSION_EXITED
        ))
        assertEquals(1, scheduler.scheduledJobs.size)

        val latch = CountDownLatch(1)
        controller.start(RuntimeStartRequest(RuntimeStartReason.RECOVERY)) { latch.countDown() }
        latch.await(2, TimeUnit.SECONDS)

        assertEquals(0, scheduler.scheduledJobs.size)
        assertEquals(RuntimeState.STARTING, controller.snapshot().state)
    }

    @Test
    fun unexpectedExit_fromStarting_evaluatesRecovery() {
        val policy = RecordingRecoveryPolicy(listOf(RuntimeRecoveryDecision.DoNotRecover))
        val scheduler = ControllableScheduler()
        val (controller, host) = makeController(RuntimeState.STARTING, policy, scheduler)

        host.emit(RuntimeServiceHostEvent.UnexpectedTermination(
            generation = controller.snapshot().generation,
            cause = RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR
        ))

        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
        assertEquals(1, policy.evaluateCalls.get())
    }

    @Test
    fun successfulReady_recordsReadyToPolicyAndClearsRecoveryState() {
        val policy = RecordingRecoveryPolicy(emptyList())
        val scheduler = ControllableScheduler()
        val store = stateStoreAt(RuntimeState.STARTING)
        val host = FakeRecoveryTestHost()
        val session = object : ProotSession {
            override val sessionId = "test-alive"
            override fun isAlive() = true
            override fun awaitExit(timeoutMillis: Long): Int? = null
            override fun stop(graceMillis: Long) =
                com.amitia.amitia_app.runtime.proot.ProotStopResult.AlreadyStopped(sessionId, null)
            override fun close() {}
            override fun requestStop() {}
            override val exit: com.amitia.amitia_app.runtime.proot.ProotExit? = null
        }
        host.sessionOverride = session

        val controller = DefaultRuntimeController(
            stateStore = store,
            serviceHost = host,
            recoveryPolicy = policy,
            recoveryScheduler = scheduler,
            installedRuntimeSource = FixedInstalledSource(),
            startupDetector = FakeReadyStartupDetector(
                RuntimeStartupResult.Ready(generation = store.snapshot().generation, elapsedMs = 1L, probeCount = 1)
            ),
        )

        val genBefore = controller.snapshot().generation
        host.emit(RuntimeServiceHostEvent.SessionReady(generation = genBefore + 1, sessionId = "test-alive"))

        val deadline = System.currentTimeMillis() + 3000
        while (System.currentTimeMillis() < deadline && controller.snapshot().state != RuntimeState.READY) {
            Thread.sleep(20)
        }

        assertEquals(RuntimeState.READY, controller.snapshot().state)
        assertEquals(1, policy.recordReadyCalls.size)
    }

    @Test
    fun recoveryDrain_triggersStartWhenFailedAndSameGeneration() {
        val policy = RecordingRecoveryPolicy(listOf(RuntimeRecoveryDecision.RecoverAfter(50L)))
        val (controller, host) = runRecoveryDrain(policy)

        assertEquals(RuntimeState.STARTING, controller.snapshot().state)
        assertEquals(1, host.ensureStartedCalls.get())
    }

    @Test
    fun recoveryDrain_generationMismatch_doesNotStart() {
        val policy = RecordingRecoveryPolicy(listOf(
            RuntimeRecoveryDecision.RecoverAfter(50L),
            RuntimeRecoveryDecision.RecoverAfter(50L),
        ))
        val scheduler = ControllableScheduler()
        val (controller, host) = makeController(RuntimeState.READY, policy, scheduler)
        host.emit(RuntimeServiceHostEvent.UnexpectedTermination(
            generation = controller.snapshot().generation,
            cause = RuntimeServiceTerminationCause.SESSION_EXITED
        ))
        val failedGen = controller.snapshot().generation

        scheduler.drainAll()
        val deadline = System.currentTimeMillis() + 2000
        while (System.currentTimeMillis() < deadline && controller.snapshot().state == RuntimeState.FAILED) {
            Thread.sleep(20)
        }

        assertEquals(RuntimeState.STARTING, controller.snapshot().state)
        val newGen = controller.snapshot().generation
        assertTrue(newGen != failedGen)
        val ensureCallsBeforeSecondDrain = host.ensureStartedCalls.get()

        host.emit(RuntimeServiceHostEvent.UnexpectedTermination(
            generation = controller.snapshot().generation,
            cause = RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR
        ))
        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
        assertEquals(1, scheduler.scheduledJobs.size)

       scheduler.drainAll()
        val deadline2 = System.currentTimeMillis() + 2000
        while (System.currentTimeMillis() < deadline2 && controller.snapshot().state == RuntimeState.FAILED) {
            Thread.sleep(20)
        }
        assertEquals(RuntimeState.STARTING, controller.snapshot().state)
        assertEquals(ensureCallsBeforeSecondDrain + 1, host.ensureStartedCalls.get())
    }

    private fun runRecoveryDrain(
        policy: RecordingRecoveryPolicy,
    ): Pair<DefaultRuntimeController, FakeRecoveryTestHost> {
        val scheduler = ControllableScheduler()
        val (controller, host) = makeController(RuntimeState.READY, policy, scheduler)

        host.emit(RuntimeServiceHostEvent.UnexpectedTermination(
            generation = controller.snapshot().generation,
            cause = RuntimeServiceTerminationCause.SESSION_EXITED
        ))
        val failedGen = controller.snapshot().generation
        assertEquals(1, scheduler.scheduledJobs.size)

        val latch = CountDownLatch(1)
        controller.start(RuntimeStartRequest(RuntimeStartReason.USER_REQUEST)) { latch.countDown() }
        latch.await(2, TimeUnit.SECONDS)
        assertEquals(RuntimeState.STARTING, controller.snapshot().state)
        assertTrue(controller.snapshot().generation != failedGen)

        scheduler.drainAll()

        val deadline = System.currentTimeMillis() + 2000
        while (System.currentTimeMillis() < deadline) {
            if (controller.snapshot().state == RuntimeState.STARTING &&
                controller.snapshot().generation != failedGen) break
            Thread.sleep(20)
        }

        return Pair(controller, host)
    }

    @Test
    fun repeatedCrash_withinBudget_evaluatesEachTimeUntilExhausted() {
        val decisions = listOf(
            RuntimeRecoveryDecision.RecoverAfter(10L),
            RuntimeRecoveryDecision.RecoverAfter(20L),
            RuntimeRecoveryDecision.Exhausted(2),
        )
        val policy = RecordingRecoveryPolicy(decisions)
        val scheduler = ControllableScheduler()
        val (controller, host) = makeController(RuntimeState.READY, policy, scheduler)

        host.emit(RuntimeServiceHostEvent.UnexpectedTermination(
            generation = controller.snapshot().generation,
            cause = RuntimeServiceTerminationCause.SESSION_EXITED
        ))
        assertEquals(1, policy.evaluateCalls.get())
        assertEquals(1, scheduler.scheduledJobs.size)

        scheduler.drainAll()
        val deadline1 = System.currentTimeMillis() + 3000
        while (System.currentTimeMillis() < deadline1 && controller.snapshot().state != RuntimeState.STARTING) {
            Thread.sleep(20)
        }
        assertEquals(RuntimeState.STARTING, controller.snapshot().state)

        host.emit(RuntimeServiceHostEvent.UnexpectedTermination(
            generation = controller.snapshot().generation,
            cause = RuntimeServiceTerminationCause.SESSION_EXITED
        ))
        assertEquals(2, policy.evaluateCalls.get())
        assertEquals(1, scheduler.scheduledJobs.size)

        scheduler.drainAll()
        val deadline2 = System.currentTimeMillis() + 3000
        while (System.currentTimeMillis() < deadline2 && controller.snapshot().state != RuntimeState.STARTING) {
            Thread.sleep(20)
        }

        host.emit(RuntimeServiceHostEvent.UnexpectedTermination(
            generation = controller.snapshot().generation,
            cause = RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR
        ))
        assertEquals(3, policy.evaluateCalls.get())
        assertEquals(0, scheduler.scheduledJobs.size)
        assertEquals(RuntimeErrorCode.RECOVERY_EXHAUSTED, controller.snapshot().lastError?.code)
    }

    @Test
    fun noRecoveryPolicy_default_doesNotScheduleRecovery() {
        val (controller, host) = makeController(
            RuntimeState.READY,
            NoOpRecoveryPolicy(),
            ControllableScheduler(),
        )

        host.emit(RuntimeServiceHostEvent.UnexpectedTermination(
            generation = controller.snapshot().generation,
            cause = RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR
        ))

        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
    }

    @Test
    fun crashThenManualStart_resetsRecoveryState() {
        val policy = RecordingRecoveryPolicy(listOf(
            RuntimeRecoveryDecision.RecoverAfter(5000L),
            RuntimeRecoveryDecision.RecoverAfter(5000L),
        ))
        val scheduler = ControllableScheduler()
        val (controller, host) = makeController(RuntimeState.READY, policy, scheduler)

        host.emit(RuntimeServiceHostEvent.UnexpectedTermination(
            generation = controller.snapshot().generation,
            cause = RuntimeServiceTerminationCause.SESSION_EXITED
        ))
        assertEquals(1, scheduler.scheduledJobs.size)

        val latch = CountDownLatch(1)
        controller.start(RuntimeStartRequest(RuntimeStartReason.USER_REQUEST)) { latch.countDown() }
        latch.await(2, TimeUnit.SECONDS)

        assertEquals(0, scheduler.scheduledJobs.size)
        assertEquals(RuntimeState.STARTING, controller.snapshot().state)
    }

    private class FixedInstalledSource : InstalledRuntimeSource {
        override fun current() = InstalledRuntimeResult.Installed
    }
}
