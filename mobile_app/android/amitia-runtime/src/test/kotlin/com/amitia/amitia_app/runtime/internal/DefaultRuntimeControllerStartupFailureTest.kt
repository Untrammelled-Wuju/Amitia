package com.amitia.amitia_app.runtime.internal

import com.amitia.amitia_app.runtime.api.RuntimeErrorCode
import com.amitia.amitia_app.runtime.api.RuntimeOperationCallback
import com.amitia.amitia_app.runtime.api.RuntimeOperationResult
import com.amitia.amitia_app.runtime.api.RuntimeStartReason
import com.amitia.amitia_app.runtime.api.RuntimeStartRequest
import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.service.RuntimeServiceHost
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostEvent
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener
import com.amitia.amitia_app.runtime.service.RuntimeServiceResult
import com.amitia.amitia_app.runtime.proot.ProotExit
import com.amitia.amitia_app.runtime.proot.ProotSession
import com.amitia.amitia_app.runtime.proot.ProotStopResult
import com.amitia.amitia_app.runtime.proot.ProotTerminationResult
import com.amitia.amitia_app.runtime.startup.RuntimeStartupDetector
import com.amitia.amitia_app.runtime.startup.RuntimeStartupError
import com.amitia.amitia_app.runtime.startup.RuntimeStartupRequest
import com.amitia.amitia_app.runtime.startup.RuntimeStartupResult
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.atomic.AtomicInteger

internal class TeardownTrackingServiceHost : RuntimeServiceHost {
    private val listeners = CopyOnWriteArrayList<RuntimeServiceHostListener>()
    val teardownRequestCount = AtomicInteger(0)
    val ensureStartedGenerations = CopyOnWriteArrayList<Long>()
    @Volatile var session: ProotSession? = null

    override fun ensureStarted(generation: Long, profile: String): RuntimeServiceResult {
        ensureStartedGenerations.add(generation)
        return RuntimeServiceResult.Success
    }

    override fun requestStop(targetGeneration: Long): RuntimeServiceResult = RuntimeServiceResult.Success

    override fun requestTeardownAfterStartupFailure() {
        teardownRequestCount.incrementAndGet()
    }

    override fun addListener(listener: RuntimeServiceHostListener) {
        listeners.add(listener)
    }

    override fun removeListener(listener: RuntimeServiceHostListener) {
        listeners.remove(listener)
    }

    override fun currentSession(): ProotSession? = session
    override fun currentGeneration(): Long = 0L

    fun emit(event: RuntimeServiceHostEvent) {
        val snapshot = ArrayList(listeners)
        snapshot.forEach { it.onServiceHostEvent(event) }
    }
}

class DefaultRuntimeControllerStartupFailureTest {

    private fun createStoppedStateStore(): RuntimeStateStore {
        val store = RuntimeStateStore()
        store.update {
            com.amitia.amitia_app.runtime.api.RuntimeSnapshot(
                state = RuntimeState.STOPPED,
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

    private fun startAndCapture(controller: DefaultRuntimeController): RuntimeOperationResult {
        val resultRef = java.util.concurrent.atomic.AtomicReference<RuntimeOperationResult>()
        val latch = java.util.concurrent.CountDownLatch(1)
        val callback = object : RuntimeOperationCallback {
            override fun onCompleted(result: RuntimeOperationResult) {
                resultRef.set(result)
                latch.countDown()
            }
        }
        controller.start(RuntimeStartRequest(reason = RuntimeStartReason.USER_REQUEST), callback)
        latch.await(5, java.util.concurrent.TimeUnit.SECONDS)
        return resultRef.get()
    }


    private fun alwaysAliveSession(): ProotSession = object : ProotSession {
        override val sessionId: String = "startup-failure-session"
        override val exit: ProotExit? = null
        override fun isAlive(): Boolean = true
        override fun awaitExit(timeoutMillis: Long): Int? = null
        override fun activate() {}
        override fun requestStop() {}
        override fun stop(graceMillis: Long): ProotStopResult =
            ProotStopResult.AlreadyStopped(sessionId, null)
        override fun terminateAndConfirmExit(
            gracefulTimeoutMs: Long,
            forceTimeoutMs: Long,
        ): ProotTerminationResult = ProotTerminationResult.ConfirmedExited(null)
        override fun close() {}
    }

    @Test
    fun deadSessionReady_waitsBrieflyForExitCodeBeforeFailing() {
        val stateStore = createStoppedStateStore()
        val host = TeardownTrackingServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )

        assertTrue(startAndCapture(controller) is RuntimeOperationResult.Success)
        val generation = controller.snapshot().generation
        host.session = object : ProotSession {
            override val sessionId: String = "delayed-exit-session"
            override val exit: ProotExit? = null
            override fun isAlive(): Boolean = false
            override fun awaitExit(timeoutMillis: Long): Int? = if (timeoutMillis > 0L) 23 else null
            override fun activate() {}
            override fun requestStop() {}
            override fun stop(graceMillis: Long): ProotStopResult =
                ProotStopResult.AlreadyStopped(sessionId, 23)
            override fun terminateAndConfirmExit(
                gracefulTimeoutMs: Long,
                forceTimeoutMs: Long,
            ): ProotTerminationResult = ProotTerminationResult.ConfirmedExited(23)
            override fun close() {}
        }

        host.emit(RuntimeServiceHostEvent.SessionReady(generation, host.session!!.sessionId))

        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
        assertTrue(controller.snapshot().lastError?.message?.contains("exitCode=23") == true)
        assertEquals(1, host.teardownRequestCount.get())
    }

    @Test
    fun detectorFailure_isPublishedEvenWhenCleanupEventNeverArrives() {
        val stateStore = createStoppedStateStore()
        val host = TeardownTrackingServiceHost()
        val detector = object : RuntimeStartupDetector {
            override fun awaitStartup(request: RuntimeStartupRequest): RuntimeStartupResult =
                RuntimeStartupResult.Failed(
                    generation = request.generation,
                    error = RuntimeStartupError.BackendConnectionRefused,
                )

            override fun cancel() {}
        }
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            startupDetector = detector,
        )

        val result = startAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Success)
        val generation = controller.snapshot().generation
        host.session = alwaysAliveSession()
        host.emit(RuntimeServiceHostEvent.SessionReady(generation, host.session!!.sessionId))

        val deadline = System.nanoTime() + java.util.concurrent.TimeUnit.SECONDS.toNanos(2)
        while (controller.snapshot().state == RuntimeState.STARTING && System.nanoTime() < deadline) {
            Thread.sleep(10)
        }

        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
        assertEquals(RuntimeErrorCode.STARTUP_BACKEND_CONNECTION_REFUSED, controller.snapshot().lastError?.code)
        assertEquals(1, host.teardownRequestCount.get())
    }

    @Test
    fun retryWhilePreviousRuntimeIsAlive_doesNotAllocateNewGeneration() {
        val stateStore = RuntimeStateStore().apply {
            update { it.copy(state = RuntimeState.FAILED, generation = 9L) }
        }
        val host = TeardownTrackingServiceHost().apply {
            session = alwaysAliveSession()
        }
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )

        val result = startAndCapture(controller)

        assertTrue(result is RuntimeOperationResult.Failure)
        assertEquals(
            RuntimeErrorCode.OPERATION_ALREADY_RUNNING,
            (result as RuntimeOperationResult.Failure).error.code,
        )
        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
        assertEquals(9L, controller.snapshot().generation)
        assertTrue(host.ensureStartedGenerations.isEmpty())
    }

    @Test
    fun duplicateStart_doesNotCancelActiveStartupDetector() {
        val stateStore = createStoppedStateStore()
        val host = TeardownTrackingServiceHost()
        val detectorEntered = java.util.concurrent.CountDownLatch(1)
        val allowReady = java.util.concurrent.CountDownLatch(1)
        val detector = object : RuntimeStartupDetector {
            override fun awaitStartup(request: RuntimeStartupRequest): RuntimeStartupResult {
                detectorEntered.countDown()
                allowReady.await(2, java.util.concurrent.TimeUnit.SECONDS)
                return RuntimeStartupResult.Ready(request.generation, 1L, 1)
            }

            override fun cancel() {}
        }
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            startupDetector = detector,
        )

        assertTrue(startAndCapture(controller) is RuntimeOperationResult.Success)
        val generation = controller.snapshot().generation
        host.session = alwaysAliveSession()
        host.emit(RuntimeServiceHostEvent.SessionReady(generation, host.session!!.sessionId))
        assertTrue(detectorEntered.await(1, java.util.concurrent.TimeUnit.SECONDS))

        val duplicate = startAndCapture(controller)
        assertTrue(duplicate is RuntimeOperationResult.Failure)
        assertEquals(
            RuntimeErrorCode.OPERATION_ALREADY_RUNNING,
            (duplicate as RuntimeOperationResult.Failure).error.code,
        )

        allowReady.countDown()
        val deadline = System.nanoTime() + java.util.concurrent.TimeUnit.SECONDS.toNanos(2)
        while (controller.snapshot().state == RuntimeState.STARTING && System.nanoTime() < deadline) {
            Thread.sleep(10)
        }
        assertEquals(RuntimeState.READY, controller.snapshot().state)
    }

    @Test
    fun retryWhileCancelledDetectorStillRunning_isRejectedUntilWorkerExits() {
        val stateStore = createStoppedStateStore()
        val host = TeardownTrackingServiceHost()
        val detectorEntered = java.util.concurrent.CountDownLatch(1)
        val releaseDetector = java.util.concurrent.CountDownLatch(1)
        val detector = object : RuntimeStartupDetector {
            override fun awaitStartup(request: RuntimeStartupRequest): RuntimeStartupResult {
                detectorEntered.countDown()
                while (true) {
                    try {
                        if (releaseDetector.await(50, java.util.concurrent.TimeUnit.MILLISECONDS)) break
                    } catch (_: InterruptedException) {
                        // Model a probe that does not immediately abort.
                    }
                }
                return RuntimeStartupResult.Cancelled(request.generation)
            }

            override fun cancel() {}
        }
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            startupDetector = detector,
        )

        assertTrue(startAndCapture(controller) is RuntimeOperationResult.Success)
        val generation = controller.snapshot().generation
        host.session = alwaysAliveSession()
        host.emit(RuntimeServiceHostEvent.SessionReady(generation, host.session!!.sessionId))
        assertTrue(detectorEntered.await(1, java.util.concurrent.TimeUnit.SECONDS))

        host.session = null
        host.emit(RuntimeServiceHostEvent.StartupFailed(
            generation = generation,
            cause = com.amitia.amitia_app.runtime.service.RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR,
            message = "forced terminal event",
            sessionId = null,
            launchStartId = 1,
            phase = "test",
        ))
        assertEquals(RuntimeState.FAILED, controller.snapshot().state)

        val retry = startAndCapture(controller)
        assertTrue(retry is RuntimeOperationResult.Failure)
        assertEquals(
            RuntimeErrorCode.OPERATION_ALREADY_RUNNING,
            (retry as RuntimeOperationResult.Failure).error.code,
        )
        assertEquals(generation, controller.snapshot().generation)

        releaseDetector.countDown()
        val deadline = System.nanoTime() + java.util.concurrent.TimeUnit.SECONDS.toNanos(1)
        while (System.nanoTime() < deadline) {
            Thread.sleep(10)
            val afterExit = startAndCapture(controller)
            if (afterExit is RuntimeOperationResult.Success) {
                assertTrue(controller.snapshot().generation > generation)
                return
            }
        }
        throw AssertionError("controller did not release detector ownership after worker exit")
    }

    @Test
    fun launchFailure_emittedByService_triggersControllerFailure() {
        val stateStore = createStoppedStateStore()
        val host = TeardownTrackingServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        val result = startAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Success)

        val currentGen = controller.snapshot().generation
        host.emit(RuntimeServiceHostEvent.StartupFailed(
            generation = currentGen,
            cause = com.amitia.amitia_app.runtime.service.RuntimeServiceTerminationCause.NO_ACTIVE_RUNTIME,
            message = "no active runtime",
            sessionId = null,
            launchStartId = 1,
            phase = "test"
        ))

        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
    }

    @Test
    fun launchFailure_afterFailure_canRecover() {
        val stateStore = createStoppedStateStore()
        val host = TeardownTrackingServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )

        val result1 = startAndCapture(controller)
        assertTrue(result1 is RuntimeOperationResult.Success)
        val gen1 = controller.snapshot().generation

        host.emit(RuntimeServiceHostEvent.StartupFailed(
            generation = gen1,
            cause = com.amitia.amitia_app.runtime.service.RuntimeServiceTerminationCause.NO_ACTIVE_RUNTIME,
            message = "no active runtime",
            sessionId = null,
            launchStartId = 1,
            phase = "test"
        ))
        assertEquals(RuntimeState.FAILED, controller.snapshot().state)

        val result2 = startAndCapture(controller)
        assertTrue(result2 is RuntimeOperationResult.Success)
        assertTrue(controller.snapshot().generation > gen1)
    }

    @Test
    fun launchFailure_doesNotRequestTeardownOnServiceHost() {
        val stateStore = createStoppedStateStore()
        val host = TeardownTrackingServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )

        startAndCapture(controller)
        assertEquals(0, host.teardownRequestCount.get())

        val currentGen = controller.snapshot().generation
        host.emit(RuntimeServiceHostEvent.StartupFailed(
            generation = currentGen,
            cause = com.amitia.amitia_app.runtime.service.RuntimeServiceTerminationCause.NO_ACTIVE_RUNTIME,
            message = "no active runtime",
            sessionId = null,
            launchStartId = 1,
            phase = "test"
        ))

        assertEquals(0, host.teardownRequestCount.get())
    }

    @Test
    fun launchFailure_staleGeneration_ignored() {
        val stateStore = createStoppedStateStore()
        val host = TeardownTrackingServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )

        startAndCapture(controller)
        val gen1 = controller.snapshot().generation

        startAndCapture(controller)
        val gen2 = controller.snapshot().generation

        host.emit(RuntimeServiceHostEvent.StartupFailed(
            generation = gen1,
            cause = com.amitia.amitia_app.runtime.service.RuntimeServiceTerminationCause.NO_ACTIVE_RUNTIME,
            message = "no active runtime",
            sessionId = null,
            launchStartId = 1,
            phase = "test"
        ))

        assertTrue(controller.snapshot().generation == gen2 || controller.snapshot().state == RuntimeState.STOPPED)
    }
}
