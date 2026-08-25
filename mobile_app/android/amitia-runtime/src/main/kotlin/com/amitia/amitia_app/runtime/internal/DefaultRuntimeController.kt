package com.amitia.amitia_app.runtime.internal

import com.amitia.amitia_app.runtime.api.RuntimeController
import com.amitia.amitia_app.runtime.api.RuntimeErrorCode
import com.amitia.amitia_app.runtime.api.RuntimeError
import com.amitia.amitia_app.runtime.api.RuntimeInstallRequest
import com.amitia.amitia_app.runtime.api.RuntimeListener
import com.amitia.amitia_app.runtime.api.RuntimeOperationCallback
import com.amitia.amitia_app.runtime.api.RuntimeOperationHandle
import com.amitia.amitia_app.runtime.api.RuntimeOperationResult
import com.amitia.amitia_app.runtime.api.RuntimeOperationType
import com.amitia.amitia_app.runtime.api.RuntimeRepairRequest
import com.amitia.amitia_app.runtime.api.RuntimeSnapshot
import com.amitia.amitia_app.runtime.api.RuntimeStartReason
import com.amitia.amitia_app.runtime.api.RuntimeStartRequest
import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.api.RuntimeStopRequest
import com.amitia.amitia_app.runtime.api.RuntimeSubscription
import com.amitia.amitia_app.runtime.api.RuntimeVerifyRequest
import com.amitia.amitia_app.runtime.abi.RuntimeAbiGate
import com.amitia.amitia_app.runtime.abi.RuntimeAbiStatus
import com.amitia.amitia_app.runtime.connection.BackendEndpointPolicy
import com.amitia.amitia_app.runtime.connection.embeddedAndroidBackendPolicy
import com.amitia.amitia_app.runtime.install.InstalledRuntimeVerifier
import com.amitia.amitia_app.runtime.install.RuntimeHostLayout
import com.amitia.amitia_app.runtime.install.RuntimeInstallResult
import com.amitia.amitia_app.runtime.install.RuntimeInstaller
import com.amitia.amitia_app.runtime.proot.ProotSession
import com.amitia.amitia_app.runtime.recovery.AlwaysInstalledRuntimeSource
import com.amitia.amitia_app.runtime.recovery.ExecutorRuntimeRecoveryScheduler
import com.amitia.amitia_app.runtime.recovery.InstalledRuntimeSource
import com.amitia.amitia_app.runtime.recovery.NoOpRecoveryPolicy
import com.amitia.amitia_app.runtime.recovery.RuntimeCrashRecoveryPolicy
import com.amitia.amitia_app.runtime.recovery.RuntimeRecoveryDecision
import com.amitia.amitia_app.runtime.recovery.RuntimeRecoveryRequest
import com.amitia.amitia_app.runtime.recovery.RuntimeRecoveryScheduler
import com.amitia.amitia_app.runtime.service.RuntimeServiceHost
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostEvent
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener
import com.amitia.amitia_app.runtime.service.RuntimeServiceLifecycleSnapshot
import com.amitia.amitia_app.runtime.service.RuntimeProcessPhase
import com.amitia.amitia_app.runtime.service.RuntimeServicePhase
import com.amitia.amitia_app.runtime.service.RuntimeServiceResult
import com.amitia.amitia_app.runtime.service.RuntimeServiceTerminationCause
import com.amitia.amitia_app.runtime.startup.DefaultRuntimeStartupDetector
import com.amitia.amitia_app.runtime.startup.RuntimeStartupDetector
import com.amitia.amitia_app.runtime.startup.RuntimeStartupError
import com.amitia.amitia_app.runtime.startup.RuntimeStartupRequest
import com.amitia.amitia_app.runtime.startup.RuntimeStartupResult
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicLong
import java.util.concurrent.atomic.AtomicReference

internal class DefaultRuntimeController(
    private val stateStore: RuntimeStateStore,
    private val serviceHost: RuntimeServiceHost,
    private val installer: RuntimeInstaller? = null,
    private val abiGate: RuntimeAbiGate? = null,
    private val bootstrapper: RuntimeBootstrapper? = null,
    private val installedVerifier: InstalledRuntimeVerifier? = null,
    private val hostLayout: RuntimeHostLayout? = null,
    private val idGenerator: RuntimeIdGenerator = UuidRuntimeIdGenerator,
    private val startupDetector: RuntimeStartupDetector = DefaultRuntimeStartupDetector(),
    private val endpointPolicy: BackendEndpointPolicy = embeddedAndroidBackendPolicy(),
    private val recoveryPolicy: RuntimeCrashRecoveryPolicy = NoOpRecoveryPolicy(),
    private val recoveryScheduler: RuntimeRecoveryScheduler = ExecutorRuntimeRecoveryScheduler(),
    private val installedRuntimeSource: InstalledRuntimeSource = AlwaysInstalledRuntimeSource()
) : RuntimeController {

    private data class ExpectedStopContext(
        val generation: Long,
    )

    private val expectedStopRef =
        AtomicReference<ExpectedStopContext?>(null)
    private val serviceHostListener = RuntimeServiceHostListener { event ->
        onServiceHostEvent(event)
    }

    private val currentStartAttemptId = AtomicReference<String?>(null)
    private val activeDetectorSession = AtomicReference<ProotSession?>(null)
    private val activeDetectorGeneration = AtomicLong(-1L)
    private val startupDetectionThread = AtomicReference<Thread?>(null)
    private val serviceSessionWatchdogThread = AtomicReference<Thread?>(null)
    private val pendingRecoveryJob = AtomicReference<com.amitia.amitia_app.runtime.recovery.RuntimeRecoveryJob?>(null)
    private val lastFailedGeneration = AtomicLong(-1L)
    private val verifiedRuntimeVersionRef = AtomicReference<String?>(null)

    private companion object {
        const val SERVICE_SESSION_TIMEOUT_MS = 30_000L
        const val SERVICE_SESSION_POLL_MS = 100L
    }

    init {
        serviceHost.addListener(serviceHostListener)
        runBootstrap()
    }

    private fun runBootstrap() {
        if (stateStore.isInitialized()) return

        when (val abiStatus = abiGate?.evaluate()) {
            is RuntimeAbiStatus.Unsupported -> {
                stateStore.initialize(
                    targetState = RuntimeState.FAILED,
                    lastError = RuntimeError(
                        code = RuntimeErrorCode.UNSUPPORTED_ABI,
                        message = "embedded runtime ABI is unsupported: ${abiStatus.reason.name}",
                        recoverable = false,
                    ),
                )
                return
            }
            is RuntimeAbiStatus.DetectionFailed -> {
                stateStore.initialize(
                    targetState = RuntimeState.FAILED,
                    lastError = RuntimeError(
                        code = RuntimeErrorCode.UNSUPPORTED_ABI,
                        message = "embedded runtime ABI detection failed: ${abiStatus.error.messageKey}",
                        recoverable = abiStatus.error.recoverable,
                    ),
                )
                return
            }
            is RuntimeAbiStatus.Supported, null -> Unit
        }

        val bootstrapper = bootstrapper
        if (bootstrapper == null) {
            stateStore.initialize(RuntimeState.NOT_INSTALLED)
            return
        }

        when (val result = bootstrapper.bootstrap()) {
            is RuntimeBootstrapResult.NotInstalled -> {
                stateStore.initialize(RuntimeState.NOT_INSTALLED)
            }
            is RuntimeBootstrapResult.InstalledStopped -> {
                stateStore.initialize(RuntimeState.STOPPED, result.runtimeVersion)
            }
            is RuntimeBootstrapResult.Failed -> {
                stateStore.initialize(
                    targetState = RuntimeState.CORRUPTED,
                    lastError = RuntimeError(
                        code = RuntimeErrorCode.RUNTIME_CORRUPTED,
                        message = result.message,
                        recoverable = true,
                        detailsSource = mapOf("bootstrapCode" to result.code.name),
                    ),
                )
            }
        }
    }

    private fun currentGeneration(): Long = stateStore.snapshot().generation

    private fun isCurrentGeneration(generation: Long): Boolean {
        return generation > 0 && generation == stateStore.snapshot().generation
    }

    private fun isRecoveryEligible(current: RuntimeSnapshot, cause: RuntimeServiceTerminationCause): Boolean {
        if (current.generation <= 0) return false
        val liveSession = serviceHost.currentSession()
        if (liveSession != null && liveSession.isAlive()) return false
        val serviceLifecycleSnapshot = serviceHost.lifecycleSnapshot()
        if (serviceLifecycleSnapshot != null) {
            if (serviceLifecycleSnapshot.processPhase == RuntimeProcessPhase.UNKNOWN) return false
            if (serviceLifecycleSnapshot.servicePhase == RuntimeServicePhase.UNOBSERVABLE) return false
        }
        return when (cause) {
            RuntimeServiceTerminationCause.EXIT_WATCHER_FAILED -> false
            else -> true
        }
    }

    private fun isStartupFailedRecoveryEligible(current: RuntimeSnapshot, cause: RuntimeServiceTerminationCause): Boolean {
        if (current.generation <= 0) return false
        val liveSession = serviceHost.currentSession()
        if (liveSession != null && liveSession.isAlive()) return false
        return true
    }

    private fun clearExpectedStop(generation: Long) {
        while (true) {
            val current = expectedStopRef.get() ?: return
            if (current.generation != generation) return
            if (expectedStopRef.compareAndSet(current, null)) return
        }
    }

    @Synchronized
    private fun onServiceHostEvent(event: RuntimeServiceHostEvent) {
        when (event) {
            is RuntimeServiceHostEvent.ForegroundStarted -> {
            }
            is RuntimeServiceHostEvent.SessionReady -> {
                onSessionReady(event.generation)
            }
            is RuntimeServiceHostEvent.ExpectedStopped -> {
                val current = stateStore.snapshot()
                if (current.generation != event.generation) return
                cancelPendingRecovery(resetBudget = true)
                cancelServiceSessionWatchdog()
                cancelStartupDetector()
                val target = RuntimeStateMachine.expectedStopTarget(current.state)
                if (target != null && target != current.state) {
                    stateStore.update { it.copy(state = target, activeProfile = null) }
                }
                clearExpectedStop(event.generation)
            }
            is RuntimeServiceHostEvent.UnexpectedTermination -> {
                if (!isCurrentGeneration(event.generation)) return
                cancelServiceSessionWatchdog()
                cancelStartupDetector()
                val current = stateStore.snapshot()
                if (!isRecoveryEligible(current, event.cause)) return
                val target = RuntimeStateMachine.unexpectedTerminationTarget(current.state)
                if (target != null && target != current.state) {
                    val error = mapTerminationCauseToError(event.cause, event.message)
                    stateStore.update { it.copy(state = target, lastError = error) }
                    clearExpectedStop(event.generation)
                    if (target == RuntimeState.FAILED) {
                        lastFailedGeneration.set(stateStore.snapshot().generation)
                        evaluateRecovery(error, requestedStop = false)
                    }
                }
            }
            is RuntimeServiceHostEvent.StartupFailed -> {
                if (!isCurrentGeneration(event.generation)) return
                cancelServiceSessionWatchdog()
                cancelStartupDetector()
                val current = stateStore.snapshot()
                if (!isStartupFailedRecoveryEligible(current, event.cause)) return
                val serviceError = mapTerminationCauseToError(event.cause, event.message)
                val target = RuntimeStateMachine.unexpectedTerminationTarget(current.state)
                if (target != null && target != current.state) {
                    stateStore.update { it.copy(state = target, lastError = serviceError) }
                    if (target == RuntimeState.FAILED) {
                        lastFailedGeneration.set(stateStore.snapshot().generation)
                        evaluateRecovery(serviceError, requestedStop = false)
                    }
                } else if (current.state == RuntimeState.FAILED) {
                    // Startup detection may have already published a precise error
                    // (for example STARTUP_BACKEND_CONNECTION_REFUSED) before the
                    // service emits its terminal cleanup event. Preserve that code
                    // instead of degrading it back to SERVICE_INTERNAL_ERROR, but
                    // retain any additional native diagnostic text.
                    val effectiveError = mergeStartupFailureDetail(current.lastError, serviceError)
                    if (current.lastError != effectiveError) {
                        stateStore.update { it.copy(lastError = effectiveError) }
                    }
                    lastFailedGeneration.set(current.generation)
                    evaluateRecovery(effectiveError, requestedStop = false)
                }
            }
            is RuntimeServiceHostEvent.SnapshotUpdated -> {
                // Snapshot updated, no action needed
            }
        }
    }

    @Synchronized
    private fun onSessionReady(generation: Long) {
        if (!isCurrentGeneration(generation)) return

        val current = stateStore.snapshot()
        if (current.state != RuntimeState.STARTING) return
        if (current.generation != generation) return

        // RuntimeServiceHost can report SessionReady both from the process-local
        // listener and from a post-bind lifecycle reconciliation. Only one
        // detector may own a runtime generation.
        if (!activeDetectorGeneration.compareAndSet(-1L, generation)) {
            return
        }
        cancelServiceSessionWatchdog()

        val session = serviceHost.currentSession()
        if (session == null) {
            activeDetectorGeneration.compareAndSet(generation, -1L)
            failSessionReadyPrecondition(
                generation = generation,
                message = "startup session is not available after SessionReady event",
                phase = "session_ready_missing",
            )
            return
        }
        if (!session.isAlive()) {
            activeDetectorGeneration.compareAndSet(generation, -1L)
            failSessionReadyPrecondition(
                generation = generation,
                message = "startup session exited before detection could start",
                phase = "session_ready_dead",
            )
            return
        }

        val attemptId = idGenerator.nextOperationId()
        currentStartAttemptId.set(attemptId)
        activeDetectorSession.set(session)
        startStartupDetection(session, generation, attemptId)
    }

    @Synchronized
    private fun failSessionReadyPrecondition(generation: Long, message: String, phase: String) {
        if (!isCurrentGeneration(generation)) return
        val current = stateStore.snapshot()
        val error = RuntimeError(
            code = RuntimeErrorCode.START_FAILED,
            message = message,
            recoverable = true,
        )
        val target = RuntimeStateMachine.startupFailureTarget(current.state)
        if (target != null) {
            stateStore.update { it.copy(state = target, lastError = error) }
            if (target == RuntimeState.FAILED) {
                lastFailedGeneration.set(generation)
            }
        }
        val teardownResult = serviceHost.requestTeardownAfterStartupFailure(
            generation = generation,
            cause = RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR,
            message = message,
            phase = phase,
        )
        if (teardownResult is RuntimeServiceResult.Failure) {
            // Do not schedule recovery while the old service may still own a
            // process. Normal recovery starts only after the terminal host
            // event confirms teardown. If teardown dispatch itself failed,
            // recover only when the host is demonstrably stopped.
            evaluateRecoveryIfHostStopped(generation, error)
        }
    }

    private fun startStartupDetection(session: ProotSession, generation: Long, attemptId: String) {
        currentStartAttemptId.set(attemptId)
        activeDetectorSession.set(session)
        val workerThread = Thread({
            runStartupDetection(session, generation, attemptId)
        }, "RuntimeStartupDetection").apply {
            isDaemon = true
        }
        startupDetectionThread.set(workerThread)
        try {
            workerThread.start()
        } catch (error: Throwable) {
            startupDetectionThread.set(null)
            activeDetectorSession.set(null)
            onStartupDetectionCompleted(
                RuntimeStartupResult.Failed(
                    generation = generation,
                    error = RuntimeStartupError.InternalError(
                        "failed to start startup detector: ${error.message ?: error.javaClass.simpleName}"
                    ),
                ),
                generation,
                attemptId,
            )
        }
    }

    private fun runStartupDetection(session: ProotSession, generation: Long, attemptId: String) {
        val request = RuntimeStartupRequest(
            generation = generation,
            session = session,
            endpoint = endpointPolicy,
            startAttemptId = attemptId
        )
        val result = try {
            startupDetector.awaitStartup(request)
        } catch (error: InterruptedException) {
            RuntimeStartupResult.Cancelled(generation)
        } catch (error: Throwable) {
            RuntimeStartupResult.Failed(
                generation = generation,
                error = RuntimeStartupError.InternalError(
                    "startup detector crashed: ${error.message ?: error.javaClass.simpleName}"
                ),
            )
        }
        onStartupDetectionCompleted(result, generation, attemptId)
    }

    @Synchronized
    private fun onStartupDetectionCompleted(result: RuntimeStartupResult, expectedGeneration: Long, expectedAttemptId: String) {
        startupDetectionThread.compareAndSet(Thread.currentThread(), null)
        if (currentStartAttemptId.get() != expectedAttemptId) return
        if (!isCurrentGeneration(expectedGeneration)) return

        when (result) {
            is RuntimeStartupResult.Ready -> {
                val current = stateStore.snapshot()
                if (current.state == RuntimeState.STARTING &&
                    current.generation == expectedGeneration
                ) {
                    val target = RuntimeStateMachine.startupReadyTarget(current.state)
                    if (target != null) {
                        stateStore.update { it.copy(state = target) }
                        activeDetectorGeneration.compareAndSet(expectedGeneration, -1L)
                        currentStartAttemptId.compareAndSet(expectedAttemptId, null)
                        activeDetectorSession.set(null)
                        recordRecoveryReady(expectedGeneration)
                    }
                }
            }
            is RuntimeStartupResult.Failed -> {
                val runtimeError = mapStartupErrorToRuntimeError(result.error)
                // The detector has reached a definitive startup failure. Publish it
                // immediately so a lost/delayed service cleanup event can never
                // leave the controller indefinitely in STARTING. Recovery is
                // evaluated only after the host confirms terminal cleanup.
                publishDetectedStartupFailure(expectedGeneration, runtimeError)
                cancelStartupDetector()
                val teardownResult = serviceHost.requestTeardownAfterStartupFailure(
                    generation = expectedGeneration,
                    cause = RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR,
                    message = runtimeError.message,
                    phase = "startup_detection",
                )
                if (teardownResult is RuntimeServiceResult.Failure) {
                    // The state is already FAILED. There may be no subsequent host
                    // event, so attempt recovery only if the host is demonstrably
                    // no longer running a live session.
                    evaluateRecoveryIfHostStopped(expectedGeneration, runtimeError)
                }
            }
            is RuntimeStartupResult.Cancelled -> {
            }
        }
    }

    private fun mapStartupErrorToRuntimeError(error: RuntimeStartupError): RuntimeError {
        return when (error) {
            is RuntimeStartupError.Cancelled -> RuntimeError(
                code = RuntimeErrorCode.STARTUP_CANCELLED,
                message = "startup detection was cancelled",
                recoverable = true
            )
            is RuntimeStartupError.GenerationStale -> RuntimeError(
                code = RuntimeErrorCode.STARTUP_GENERATION_STALE,
                message = "startup detection belongs to a stale runtime generation",
                recoverable = true
            )
            is RuntimeStartupError.ProotNotRunning -> RuntimeError(
                code = RuntimeErrorCode.STARTUP_PROOT_NOT_RUNNING,
                message = "outer proot process is not running",
                recoverable = true
            )
            is RuntimeStartupError.ProotExited -> RuntimeError(
                code = RuntimeErrorCode.STARTUP_PROOT_EXITED,
                message = "outer proot process exited with code ${error.exitCode ?: "unknown"} after ${error.elapsedMs}ms",
                recoverable = true
            )
            is RuntimeStartupError.BackendConnectionRefused -> RuntimeError(
                code = RuntimeErrorCode.STARTUP_BACKEND_CONNECTION_REFUSED,
                message = "backend connection was refused while runtime was starting",
                recoverable = true
            )
            is RuntimeStartupError.BackendLivenessFailed -> RuntimeError(
                code = RuntimeErrorCode.STARTUP_BACKEND_LIVENESS_FAILED,
                message = "backend liveness check failed during startup",
                recoverable = true
            )
            is RuntimeStartupError.BackendReadinessFailed -> RuntimeError(
                code = RuntimeErrorCode.STARTUP_BACKEND_READINESS_FAILED,
                message = "backend readiness check failed during startup",
                recoverable = true
            )
            is RuntimeStartupError.HealthAuthFailed -> RuntimeError(
                code = RuntimeErrorCode.STARTUP_HEALTH_AUTH_FAILED,
                message = "backend readiness probe returned an authentication error",
                recoverable = true
            )
            is RuntimeStartupError.HealthEndpointMissing -> RuntimeError(
                code = RuntimeErrorCode.STARTUP_HEALTH_ENDPOINT_MISSING,
                message = "backend readiness endpoint is missing",
                recoverable = true
            )
            is RuntimeStartupError.Timeout -> RuntimeError(
                code = RuntimeErrorCode.STARTUP_TIMEOUT,
                message = "startup timed out after ${error.elapsedMs}ms (${error.probeCount} probes; limit ${error.timeoutMs}ms)",
                recoverable = true
            )
            is RuntimeStartupError.InvalidEndpoint -> RuntimeError(
                code = RuntimeErrorCode.STARTUP_INVALID_ENDPOINT,
                message = "backend startup endpoint is invalid",
                recoverable = false
            )
            is RuntimeStartupError.InvalidResponse -> RuntimeError(
                code = RuntimeErrorCode.STARTUP_BACKEND_READINESS_FAILED,
                message = "invalid readiness response: ${error.reason}",
                recoverable = true
            )
            is RuntimeStartupError.InternalError -> RuntimeError(
                code = RuntimeErrorCode.INTERNAL_ERROR,
                message = error.message.ifBlank { "startup detector failed internally" },
                recoverable = true
            )
        }
    }

    private fun publishDetectedStartupFailure(generation: Long, error: RuntimeError) {
        if (!isCurrentGeneration(generation)) return
        val current = stateStore.snapshot()
        if (current.state != RuntimeState.STARTING || current.generation != generation) return
        val target = RuntimeStateMachine.startupFailureTarget(current.state) ?: return
        stateStore.update { it.copy(state = target, lastError = error) }
        if (target == RuntimeState.FAILED) {
            lastFailedGeneration.set(generation)
        }
    }

    private fun evaluateRecoveryIfHostStopped(generation: Long, error: RuntimeError) {
        if (!isCurrentGeneration(generation)) return
        val current = stateStore.snapshot()
        if (current.state != RuntimeState.FAILED) return
        val liveSession = try {
            serviceHost.currentSession()
        } catch (_: Throwable) {
            return
        }
        if (liveSession != null && liveSession.isAlive()) return
        evaluateRecovery(error, requestedStop = false)
    }

    private fun mergeStartupFailureDetail(primary: RuntimeError?, service: RuntimeError): RuntimeError {
        if (primary == null) return service
        val serviceMessage = service.message.trim()
        if (serviceMessage.isEmpty() || serviceMessage == primary.message.trim()) return primary
        val primaryMessage = primary.message.trim()
        val mergedMessage = if (serviceMessage.contains(primaryMessage)) {
            serviceMessage
        } else {
            "$primaryMessage; native: $serviceMessage"
        }
        return RuntimeError(
            code = primary.code,
            message = mergedMessage,
            recoverable = primary.recoverable,
            componentId = primary.componentId,
            detailsSource = primary.details,
        )
    }

    private fun cancelStartupDetector() {
        currentStartAttemptId.set(null)
        activeDetectorSession.set(null)
        activeDetectorGeneration.set(-1L)
        try {
            startupDetector.cancel()
        } catch (_: Throwable) {
        }
        val thread = startupDetectionThread.get()
        if (thread != null && thread !== Thread.currentThread() && thread.isAlive) {
            try {
                thread.interrupt()
            } catch (_: Throwable) {
            }
            // Keep the reference until the worker actually exits. A new
            // generation must not reuse the shared detector while the old
            // awaitStartup call still owns it.
        } else if (thread != null && !thread.isAlive) {
            startupDetectionThread.compareAndSet(thread, null)
        }
    }

    private fun startServiceSessionWatchdog(generation: Long) {
        cancelServiceSessionWatchdog()
        val thread = Thread({
            val deadlineNanos = System.nanoTime() + SERVICE_SESSION_TIMEOUT_MS * 1_000_000L
            try {
                while (!Thread.currentThread().isInterrupted) {
                    if (!isCurrentGeneration(generation)) return@Thread
                    val current = stateStore.snapshot()
                    if (current.state != RuntimeState.STARTING) return@Thread

                    val session = try {
                        serviceHost.currentSession()
                    } catch (_: Throwable) {
                        null
                    }
                    if (session != null && session.isAlive()) {
                        onSessionReady(generation)
                        return@Thread
                    }

                    if (System.nanoTime() >= deadlineNanos) {
                        serviceSessionWatchdogThread.compareAndSet(Thread.currentThread(), null)
                        failSessionReadyPrecondition(
                            generation = generation,
                            message = "runtime service did not publish a live PRoot session within ${SERVICE_SESSION_TIMEOUT_MS}ms",
                            phase = "service_session_timeout",
                        )
                        return@Thread
                    }
                    Thread.sleep(SERVICE_SESSION_POLL_MS)
                }
            } catch (_: InterruptedException) {
                Thread.currentThread().interrupt()
            } finally {
                serviceSessionWatchdogThread.compareAndSet(Thread.currentThread(), null)
            }
        }, "RuntimeServiceSessionWatchdog").apply {
            isDaemon = true
        }
        serviceSessionWatchdogThread.set(thread)
        try {
            thread.start()
        } catch (error: Throwable) {
            serviceSessionWatchdogThread.compareAndSet(thread, null)
            failSessionReadyPrecondition(
                generation = generation,
                message = "failed to start runtime service watchdog: ${error.message ?: error.javaClass.simpleName}",
                phase = "service_watchdog_start",
            )
        }
    }

    private fun cancelServiceSessionWatchdog() {
        val thread = serviceSessionWatchdogThread.getAndSet(null)
        if (thread != null && thread !== Thread.currentThread() && thread.isAlive) {
            try {
                thread.interrupt()
            } catch (_: Throwable) {
            }
        }
    }

    private fun evaluateRecovery(error: RuntimeError, requestedStop: Boolean) {
        val current = stateStore.snapshot()
        if (current.state != RuntimeState.FAILED) return
        cancelPendingRecovery(resetBudget = false)
        val currentExpectedStop = expectedStopRef.get()
        val isExpectedStop = currentExpectedStop != null && currentExpectedStop.generation == current.generation
        val request = RuntimeRecoveryRequest(
            failedGeneration = current.generation,
            currentState = current.state,
            error = error,
            requestedStop = requestedStop || isExpectedStop
        )
        when (val decision = recoveryPolicy.evaluate(request)) {
            is RuntimeRecoveryDecision.DoNotRecover -> {
            }
            is RuntimeRecoveryDecision.RecoverAfter -> {
                scheduleRecovery(decision.delayMillis)
            }
            is RuntimeRecoveryDecision.Exhausted -> {
                stateStore.update {
                    it.copy(
                        lastError = RuntimeError(
                            code = RuntimeErrorCode.RECOVERY_EXHAUSTED,
                            message = "recovery budget exhausted after ${decision.attempts} attempts",
                            recoverable = false
                        )
                    )
                }
            }
        }
    }

    private fun scheduleRecovery(delayMillis: Long) {
        cancelPendingRecovery(resetBudget = false)
        val failedGen = lastFailedGeneration.get()
        if (failedGen <= 0) return
        try {
            val jobRef = AtomicReference<com.amitia.amitia_app.runtime.recovery.RuntimeRecoveryJob?>(null)
            val job = recoveryScheduler.schedule(delayMillis) {
                val runningJob = jobRef.get()
                if (runningJob != null) {
                    pendingRecoveryJob.compareAndSet(runningJob, null)
                }
                executeRecoveryStart(failedGen)
            }
            jobRef.set(job)
            pendingRecoveryJob.set(job)
        } catch (_: Throwable) {
        }
    }

    @Synchronized
    private fun executeRecoveryStart(failedGeneration: Long) {
        val current = stateStore.snapshot()
        if (current.state != RuntimeState.FAILED) return
        if (current.generation != 0L && current.generation != failedGeneration) return
        cancelPendingRecoveryStartPrerequisites(failedGeneration)
    }

    private fun cancelPendingRecoveryStartPrerequisites(failedGeneration: Long): Boolean {
        val liveSession = serviceHost.currentSession()
        if (liveSession != null && liveSession.isAlive()) return false
        val serviceLifecycleSnapshot = serviceHost.lifecycleSnapshot()
        if (serviceLifecycleSnapshot != null) {
            if (serviceLifecycleSnapshot.processPhase == RuntimeProcessPhase.UNKNOWN) return false
            if (serviceLifecycleSnapshot.servicePhase == RuntimeServicePhase.UNOBSERVABLE) return false
        }
        val currentGen = serviceHost.currentGeneration()
        if (currentGen != 0L && currentGen != failedGeneration) return false
        cancelPendingRecovery(resetBudget = false)
        val recoveryProfile = stateStore.snapshot().activeProfile ?: "local"
        start(
            RuntimeStartRequest(reason = RuntimeStartReason.RECOVERY, profile = recoveryProfile),
            object : RuntimeOperationCallback {
                override fun onCompleted(result: RuntimeOperationResult) {}
            }
        )
        return true
    }

    private fun cancelPendingRecovery(resetBudget: Boolean = false) {
        val job = pendingRecoveryJob.getAndSet(null)
        if (job != null) {
            try {
                job.cancel()
            } catch (_: Throwable) {
            }
        }
        try {
            recoveryPolicy.cancelPending()
            if (resetBudget) {
                recoveryPolicy.resetBudget()
            }
        } catch (_: Throwable) {
        }
    }

    private fun recordRecoveryReady(generation: Long) {
        cancelPendingRecovery(resetBudget = false)
        try {
            recoveryPolicy.recordReady(generation)
        } catch (_: Throwable) {
        }
    }

    private fun mapTerminationCauseToError(cause: RuntimeServiceTerminationCause, detail: String? = null): RuntimeError {
        return when (cause) {
            RuntimeServiceTerminationCause.FOREGROUND_FAILED -> RuntimeError(
                code = RuntimeErrorCode.START_FAILED,
                message = detail?.takeIf { it.isNotBlank() } ?: "foreground service start failed",
                recoverable = true
            )
            RuntimeServiceTerminationCause.NOTIFICATION_FAILED -> RuntimeError(
                code = RuntimeErrorCode.START_FAILED,
                message = detail?.takeIf { it.isNotBlank() } ?: "notification creation failed",
                recoverable = true
            )
            RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR -> RuntimeError(
                code = RuntimeErrorCode.RUNTIME_EXECUTION_NOT_AVAILABLE,
                message = detail?.takeIf { it.isNotBlank() } ?: "runtime service unexpectedly terminated",
                recoverable = true
            )
            RuntimeServiceTerminationCause.SESSION_EXITED -> RuntimeError(
                code = RuntimeErrorCode.RUNTIME_EXECUTION_NOT_AVAILABLE,
                message = detail?.takeIf { it.isNotBlank() } ?: "outer proot session exited unexpectedly",
                recoverable = true
            )
            RuntimeServiceTerminationCause.PROOT_COMPONENT_MISSING -> RuntimeError(
                code = RuntimeErrorCode.START_FAILED,
                message = detail ?: "proot component is not available",
                recoverable = true
            )
            RuntimeServiceTerminationCause.ROOTFS_MISSING -> RuntimeError(
                code = RuntimeErrorCode.START_FAILED,
                message = detail ?: "rootfs path is not available",
                recoverable = true
            )
            RuntimeServiceTerminationCause.ASSEMBLER_MISSING -> RuntimeError(
                code = RuntimeErrorCode.START_FAILED,
                message = detail ?: "launch assembler is not available",
                recoverable = true
            )
            RuntimeServiceTerminationCause.NO_ACTIVE_RUNTIME -> RuntimeError(
                code = RuntimeErrorCode.START_FAILED,
                message = detail ?: "no active runtime",
                recoverable = true
            )
            RuntimeServiceTerminationCause.ACTIVE_PROGRAM_ROOT_MISSING -> RuntimeError(
                code = RuntimeErrorCode.START_FAILED,
                message = detail ?: "active program root is missing",
                recoverable = true
            )
            RuntimeServiceTerminationCause.ACTIVE_PROGRAM_ROOT_INVALID -> RuntimeError(
                code = RuntimeErrorCode.START_FAILED,
                message = detail ?: "active program root is invalid",
                recoverable = true
            )
            RuntimeServiceTerminationCause.ENVIRONMENT_BUILD_FAILED -> RuntimeError(
                code = RuntimeErrorCode.START_FAILED,
                message = detail ?: "environment build failed",
                recoverable = true
            )
            RuntimeServiceTerminationCause.MOUNT_CONTRACT_INVALID -> RuntimeError(
                code = RuntimeErrorCode.START_FAILED,
                message = detail ?: "mount contract is invalid",
                recoverable = true
            )
            RuntimeServiceTerminationCause.EXIT_WATCHER_FAILED -> RuntimeError(
                code = RuntimeErrorCode.RUNTIME_EXECUTION_NOT_AVAILABLE,
                message = detail?.takeIf { it.isNotBlank() } ?: "exit watcher failed to observe process termination",
                recoverable = true
            )
            RuntimeServiceTerminationCause.STOP_RESULT_FAILED -> RuntimeError(
                code = RuntimeErrorCode.STOP_SERVICE_TEARDOWN_FAILED,
                message = detail?.takeIf { it.isNotBlank() } ?: "stopSelfResult returned false, service did not stop as expected",
                recoverable = true
            )
        }
    }

    private fun runtimeAbiError(): RuntimeError? {
        return when (val status = abiGate?.evaluate()) {
            is RuntimeAbiStatus.Supported, null -> null
            is RuntimeAbiStatus.Unsupported -> RuntimeError(
                code = RuntimeErrorCode.UNSUPPORTED_ABI,
                message = "embedded runtime ABI is unsupported: ${status.reason.name}",
                recoverable = false,
            )
            is RuntimeAbiStatus.DetectionFailed -> RuntimeError(
                code = RuntimeErrorCode.UNSUPPORTED_ABI,
                message = "embedded runtime ABI detection failed: ${status.error.messageKey}",
                recoverable = status.error.recoverable,
            )
        }
    }

    private fun normalizeRuntimeProfile(profile: String): String? {
        val normalized = profile.trim().lowercase()
        if (normalized.isEmpty() || normalized.length > 32) return null
        if (!normalized.matches(Regex("[a-z0-9][a-z0-9-]*"))) return null
        return normalized
    }

    private fun verifyRuntimeBeforeStart(current: RuntimeSnapshot): RuntimeError? {
        val version = current.runtimeVersion ?: return RuntimeError(
            code = RuntimeErrorCode.RUNTIME_NOT_INSTALLED,
            message = "runtime version is unavailable; install or repair the embedded runtime before start",
            recoverable = true,
        )
        if (verifiedRuntimeVersionRef.get() == version) return null
        val verifier = installedVerifier ?: return null
        val layout = hostLayout ?: return null
        val versionDir = layout.runtimeVersionRoot(version)
        val result = try {
            verifier.verify(versionDir)
        } catch (error: Throwable) {
            val runtimeError = RuntimeError(
                code = RuntimeErrorCode.VERIFY_FAILED,
                message = "runtime verification failed before start: ${error.message ?: error.javaClass.simpleName}",
                recoverable = true,
            )
            transitionToCorruptedIfAllowed(runtimeError)
            return runtimeError
        }
        return when (result) {
            is com.amitia.amitia_app.runtime.install.InstalledRuntimeVerificationResult.Success -> {
                verifiedRuntimeVersionRef.set(version)
                null
            }
            is com.amitia.amitia_app.runtime.install.InstalledRuntimeVerificationResult.Failure -> {
                val runtimeError = RuntimeError(
                    code = RuntimeErrorCode.RUNTIME_CORRUPTED,
                    message = result.message,
                    recoverable = true,
                )
                transitionToCorruptedIfAllowed(runtimeError)
                runtimeError
            }
        }
    }

    private fun transitionToCorruptedIfAllowed(error: RuntimeError) {
        val snapshot = stateStore.snapshot()
        if (RuntimeStateMachine.canTransition(snapshot.state, RuntimeState.CORRUPTED)) {
            stateStore.update {
                it.copy(
                    state = RuntimeState.CORRUPTED,
                    lastError = error,
                    activeProfile = null,
                )
            }
        }
    }

    override fun snapshot(): RuntimeSnapshot = stateStore.snapshot()

    fun lifecycleSnapshot(): RuntimeServiceLifecycleSnapshot? =
        serviceHost.lifecycleSnapshot()

    override fun subscribe(listener: RuntimeListener): RuntimeSubscription = stateStore.subscribe(listener)

    @Synchronized
    override fun install(
        request: RuntimeInstallRequest,
        callback: RuntimeOperationCallback
    ): RuntimeOperationHandle {
        val impl = installer
        if (impl == null) {
            return completeNotImplemented(RuntimeOperationType.INSTALL, callback)
        }
        return executeInstall(impl, request, callback)
    }

    @Synchronized
    override fun verify(
        request: RuntimeVerifyRequest,
        callback: RuntimeOperationCallback
    ): RuntimeOperationHandle {
        val impl = installer
        if (impl == null) {
            return completeNotImplemented(RuntimeOperationType.VERIFY, callback)
        }
        return executeVerify(impl, request, callback)
    }

    private fun startupOwnershipError(): RuntimeError? {
        val detectorThread = startupDetectionThread.get()
        if (detectorThread != null && detectorThread.isAlive) {
            return RuntimeError(
                code = RuntimeErrorCode.OPERATION_ALREADY_RUNNING,
                message = "previous startup detector is still shutting down; wait before starting a new generation",
                recoverable = true,
            )
        } else if (detectorThread != null) {
            startupDetectionThread.compareAndSet(detectorThread, null)
        }

        val session = try {
            serviceHost.currentSession()
        } catch (error: Throwable) {
            return RuntimeError(
                code = RuntimeErrorCode.RUNTIME_EXECUTION_NOT_AVAILABLE,
                message = "failed to inspect existing runtime process before start: ${error.message ?: error.javaClass.simpleName}",
                recoverable = true,
            )
        }
        if (session != null && session.isAlive()) {
            return RuntimeError(
                code = RuntimeErrorCode.OPERATION_ALREADY_RUNNING,
                message = "previous runtime process is still active; wait for teardown before starting a new generation",
                recoverable = true,
            )
        }

        val lifecycle = try {
            serviceHost.lifecycleSnapshot()
        } catch (error: Throwable) {
            return RuntimeError(
                code = RuntimeErrorCode.RUNTIME_EXECUTION_NOT_AVAILABLE,
                message = "failed to inspect runtime service lifecycle before start: ${error.message ?: error.javaClass.simpleName}",
                recoverable = true,
            )
        }
        if (lifecycle != null && lifecycle.terminalState == null) {
            val ownsOrMayOwnProcess = lifecycle.processPhase == RuntimeProcessPhase.STARTED ||
                lifecycle.processPhase == RuntimeProcessPhase.READY ||
                lifecycle.processPhase == RuntimeProcessPhase.EXITING ||
                lifecycle.processPhase == RuntimeProcessPhase.UNKNOWN
            if (ownsOrMayOwnProcess) {
                return RuntimeError(
                    code = RuntimeErrorCode.OPERATION_ALREADY_RUNNING,
                    message = "previous runtime service still owns or may own a process (${lifecycle.processPhase.name.lowercase()}); wait for teardown",
                    recoverable = true,
                )
            }
        }
        return null
    }

    @Synchronized
    override fun start(
        request: RuntimeStartRequest,
        callback: RuntimeOperationCallback
    ): RuntimeOperationHandle {
        val operationId = idGenerator.nextOperationId()
        val handle = CompletedOperationHandle(operationId, RuntimeOperationType.START)
        val requestedProfile = normalizeRuntimeProfile(request.profile)
        if (requestedProfile == null) {
            callback.onCompleted(
                RuntimeOperationResult.Failure(
                    operationId = operationId,
                    type = RuntimeOperationType.START,
                    error = RuntimeError(
                        code = RuntimeErrorCode.INVALID_REQUEST,
                        message = "invalid runtime profile: ${request.profile}",
                        recoverable = false,
                    ),
                    snapshot = stateStore.snapshot(),
                )
            )
            return handle
        }

        val current = stateStore.snapshot()
        val abiError = runtimeAbiError()
        if (abiError != null) {
            callback.onCompleted(
                RuntimeOperationResult.Failure(
                    operationId = operationId,
                    type = RuntimeOperationType.START,
                    error = abiError,
                    snapshot = current,
                )
            )
            return handle
        }
        if (current.state == RuntimeState.READY || current.state == RuntimeState.DEGRADED) {
            if (current.activeProfile == requestedProfile) {
                callback.onCompleted(
                    RuntimeOperationResult.Success(
                        operationId = operationId,
                        type = RuntimeOperationType.START,
                        snapshot = current,
                    )
                )
            } else {
                callback.onCompleted(
                    RuntimeOperationResult.Failure(
                        operationId = operationId,
                        type = RuntimeOperationType.START,
                        error = RuntimeError(
                            code = RuntimeErrorCode.INVALID_STATE,
                            message = "runtime is already ready with profile ${current.activeProfile ?: "unknown"}; stop it before switching to $requestedProfile",
                            recoverable = true,
                        ),
                        snapshot = current,
                    )
                )
            }
            return handle
        }
        if (current.state == RuntimeState.STARTING) {
            callback.onCompleted(
                RuntimeOperationResult.Failure(
                    operationId = operationId,
                    type = RuntimeOperationType.START,
                    error = RuntimeError(
                        code = RuntimeErrorCode.OPERATION_ALREADY_RUNNING,
                        message = "runtime is already starting",
                        recoverable = true
                    ),
                    snapshot = stateStore.snapshot()
                )
            )
            return handle
        }

        val stateError = RuntimeStateMachine.requireTransitionTo(current.state, RuntimeState.STARTING)
        if (stateError != null) {
            callback.onCompleted(
                RuntimeOperationResult.Failure(
                    operationId = operationId,
                    type = RuntimeOperationType.START,
                    error = RuntimeError(
                        code = RuntimeErrorCode.INVALID_STATE,
                        message = "cannot start from state: ${current.state}",
                        recoverable = true
                    ),
                    snapshot = stateStore.snapshot()
                )
            )
            return handle
        }

        val verificationError = verifyRuntimeBeforeStart(current)
        if (verificationError != null) {
            callback.onCompleted(
                RuntimeOperationResult.Failure(
                    operationId = operationId,
                    type = RuntimeOperationType.START,
                    error = verificationError,
                    snapshot = stateStore.snapshot(),
                )
            )
            return handle
        }

        val ownershipError = startupOwnershipError()
        if (ownershipError != null) {
            callback.onCompleted(
                RuntimeOperationResult.Failure(
                    operationId = operationId,
                    type = RuntimeOperationType.START,
                    error = ownershipError,
                    snapshot = stateStore.snapshot(),
                )
            )
            return handle
        }

        // Only cancel ownership from a previous generation after this start
        // request has been accepted. A duplicate STARTING request must be a
        // read-only rejection; cancelling here earlier would kill the active
        // detector/watchdog and strand the runtime in STARTING.
        cancelServiceSessionWatchdog()
        cancelStartupDetector()
        cancelPendingRecovery(resetBudget = request.reason != RuntimeStartReason.RECOVERY)

        val newSnapshot = stateStore.transitionToStarting(requestedProfile)
        val allocatedGeneration = newSnapshot.generation

        expectedStopRef.set(null)
        startServiceSessionWatchdog(allocatedGeneration)

        val startResult = serviceHost.ensureStarted(allocatedGeneration, requestedProfile)
        if (startResult is RuntimeServiceResult.Failure) {
            cancelServiceSessionWatchdog()
            val serviceError = RuntimeError(
                code = RuntimeErrorCode.START_FAILED,
                message = "failed to ensure service is started: ${startResult.error.message}",
                recoverable = true
            )
            val afterHostAttempt = stateStore.snapshot()
            val effectiveError = if (afterHostAttempt.state == RuntimeState.STARTING) {
                val target = RuntimeStateMachine.startupFailureTarget(afterHostAttempt.state)
                if (target != null) {
                    stateStore.update { it.copy(state = target, lastError = serviceError) }
                }
                serviceError
            } else {
                // A process-level service event can arrive synchronously while
                // ensureStarted is still establishing the binding. Preserve the
                // richer terminal error instead of overwriting it.
                afterHostAttempt.lastError ?: serviceError
            }
            callback.onCompleted(
                RuntimeOperationResult.Failure(
                    operationId = operationId,
                    type = RuntimeOperationType.START,
                    error = effectiveError,
                    snapshot = stateStore.snapshot()
                )
            )
            return handle
        }

        callback.onCompleted(
            RuntimeOperationResult.Success(
                operationId = operationId,
                type = RuntimeOperationType.START,
                snapshot = stateStore.snapshot()
            )
        )
        return handle
    }

    @Synchronized
    override fun stop(
        request: RuntimeStopRequest,
        callback: RuntimeOperationCallback
    ): RuntimeOperationHandle {
        val operationId = idGenerator.nextOperationId()
        val handle = CompletedOperationHandle(operationId, RuntimeOperationType.STOP)

        try {
            val current = stateStore.snapshot()

            if (current.state == RuntimeState.STOPPED) {
                callback.onCompleted(
                    RuntimeOperationResult.Success(
                        operationId = operationId,
                        type = RuntimeOperationType.STOP,
                        snapshot = stateStore.snapshot()
                    )
                )
                return handle
            }

            if (current.state == RuntimeState.STOPPING) {
                callback.onCompleted(
                    RuntimeOperationResult.Success(
                        operationId = operationId,
                        type = RuntimeOperationType.STOP,
                        snapshot = stateStore.snapshot()
                    )
                )
                return handle
            }

            val stoppingTarget = RuntimeStateMachine.stoppingTarget(current.state)
            if (stoppingTarget == null) {
                callback.onCompleted(
                    RuntimeOperationResult.Failure(
                        operationId = operationId,
                        type = RuntimeOperationType.STOP,
                        error = RuntimeError(
                            code = RuntimeErrorCode.INVALID_STATE,
                            message = "cannot stop from state: ${current.state}",
                            recoverable = true
                        ),
                        snapshot = stateStore.snapshot()
                    )
                )
                return handle
            }

            val generationToStop = current.generation
            expectedStopRef.set(ExpectedStopContext(generation = generationToStop))
            cancelPendingRecovery(resetBudget = true)
            cancelServiceSessionWatchdog()
            cancelStartupDetector()

            stateStore.update { it.copy(state = RuntimeState.STOPPING) }

            val stopResult = serviceHost.requestStop(generationToStop)
            if (stopResult is RuntimeServiceResult.Failure) {
                clearExpectedStop(generationToStop)
                val error = RuntimeError(
                    code = RuntimeErrorCode.STOP_SERVICE_TEARDOWN_FAILED,
                    message = "failed to request service stop: ${stopResult.error.message}",
                    recoverable = true
                )
                val failTarget = RuntimeStateMachine.startupFailureTarget(RuntimeState.STOPPING)
                if (failTarget != null) {
                    stateStore.update { it.copy(state = failTarget, lastError = error) }
                }
                callback.onCompleted(
                    RuntimeOperationResult.Failure(
                        operationId = operationId,
                        type = RuntimeOperationType.STOP,
                        error = error,
                        snapshot = stateStore.snapshot()
                    )
                )
                return handle
            }

            callback.onCompleted(
                RuntimeOperationResult.Success(
                    operationId = operationId,
                    type = RuntimeOperationType.STOP,
                    snapshot = stateStore.snapshot()
                )
            )
            return handle
        } catch (e: Exception) {
            val error = RuntimeError(
                code = RuntimeErrorCode.INTERNAL_ERROR,
                message = "stop failed with exception: ${e.message}",
                recoverable = true
            )
            callback.onCompleted(
                RuntimeOperationResult.Failure(
                    operationId = operationId,
                    type = RuntimeOperationType.STOP,
                    error = error,
                    snapshot = stateStore.snapshot()
                )
            )
            return handle
        }
    }

    @Synchronized
    override fun repair(
        request: RuntimeRepairRequest,
        callback: RuntimeOperationCallback
    ): RuntimeOperationHandle {
        val impl = installer
        if (impl == null) {
            return completeNotImplemented(RuntimeOperationType.REPAIR, callback)
        }
        return executeRepair(impl, request, callback)
    }

    private fun completeNotImplemented(
        type: RuntimeOperationType,
        callback: RuntimeOperationCallback
    ): RuntimeOperationHandle {
        val operationId = idGenerator.nextOperationId()
        val handle = CompletedOperationHandle(operationId, type)
        val error = RuntimeError(
            code = RuntimeErrorCode.NOT_IMPLEMENTED,
            message = "operation not implemented: ${type.name}",
            recoverable = false
        )
        callback.onCompleted(
            RuntimeOperationResult.Failure(
                operationId = operationId,
                type = type,
                error = error,
                snapshot = stateStore.snapshot()
            )
        )
        return handle
    }

    private fun executeInstall(
        installer: RuntimeInstaller,
        request: RuntimeInstallRequest,
        callback: RuntimeOperationCallback
    ): RuntimeOperationHandle {
        val operationId = idGenerator.nextOperationId()
        val handle = CompletedOperationHandle(operationId, RuntimeOperationType.INSTALL)
        val before = stateStore.snapshot()
        if (before.state != RuntimeState.NOT_INSTALLED && before.state != RuntimeState.FAILED) {
            callback.onCompleted(
                RuntimeOperationResult.Failure(
                    operationId = operationId,
                    type = RuntimeOperationType.INSTALL,
                    error = RuntimeError(
                        code = RuntimeErrorCode.INVALID_STATE,
                        message = "cannot install runtime from state: ${before.state}",
                        recoverable = true,
                    ),
                    snapshot = before,
                )
            )
            return handle
        }

        val transitionError = RuntimeStateMachine.requireTransitionTo(before.state, RuntimeState.INSTALLING)
        if (transitionError != null) {
            callback.onCompleted(
                RuntimeOperationResult.Failure(
                    operationId = operationId,
                    type = RuntimeOperationType.INSTALL,
                    error = RuntimeError(
                        code = RuntimeErrorCode.INVALID_STATE,
                        message = "runtime cannot enter INSTALLING from ${before.state}",
                        recoverable = true,
                    ),
                    snapshot = before,
                )
            )
            return handle
        }

        cancelPendingRecovery(resetBudget = true)
        stateStore.update {
            it.copy(
                state = RuntimeState.INSTALLING,
                activeOperationId = operationId,
                activeOperationType = RuntimeOperationType.INSTALL,
                lastError = null,
                activeProfile = null,
            )
        }

        try {
            val installResult = installer.install(
                com.amitia.amitia_app.runtime.install.RuntimeInstallRequest(
                    packageFile = java.io.File(request.packageUri),
                    expectedRuntimeVersion = request.expectedVersion,
                    allowRepairExisting = request.allowRepairExisting,
                )
            )
            when (installResult) {
                is RuntimeInstallResult.Success,
                is RuntimeInstallResult.AlreadyInstalled -> {
                    val bootstrapResult = bootstrapper?.bootstrap()
                    if (bootstrapResult !is RuntimeBootstrapResult.InstalledStopped) {
                        val error = RuntimeError(
                            code = RuntimeErrorCode.INSTALL_FAILED,
                            message = when (bootstrapResult) {
                                is RuntimeBootstrapResult.Failed -> "install committed but runtime authority is inconsistent: ${bootstrapResult.message}"
                                is RuntimeBootstrapResult.NotInstalled -> "install committed but no active runtime was published"
                                null -> "install committed but runtime bootstrap authority is unavailable"
                                else -> "install committed but runtime authority is inconsistent"
                            },
                            recoverable = true,
                        )
                        finishInstallFailure(error)
                        callback.onCompleted(
                            RuntimeOperationResult.Failure(
                                operationId = operationId,
                                type = RuntimeOperationType.INSTALL,
                                error = error,
                                snapshot = stateStore.snapshot(),
                            )
                        )
                        return handle
                    }
                    verifiedRuntimeVersionRef.set(bootstrapResult.runtimeVersion)
                    stateStore.update {
                        it.copy(
                            state = RuntimeState.INSTALLED,
                            runtimeVersion = bootstrapResult.runtimeVersion,
                            activeOperationId = null,
                            activeOperationType = null,
                            lastError = null,
                            activeProfile = null,
                        )
                    }
                    callback.onCompleted(
                        RuntimeOperationResult.Success(
                            operationId = operationId,
                            type = RuntimeOperationType.INSTALL,
                            snapshot = stateStore.snapshot()
                        )
                    )
                }
                is RuntimeInstallResult.Failure -> {
                    val error = mapInstallFailure(installResult)
                    finishInstallFailure(error)
                    callback.onCompleted(
                        RuntimeOperationResult.Failure(
                            operationId = operationId,
                            type = RuntimeOperationType.INSTALL,
                            error = error,
                            snapshot = stateStore.snapshot()
                        )
                    )
                }
            }
        } catch (e: Exception) {
            val error = RuntimeError(
                code = RuntimeErrorCode.INSTALL_FAILED,
                message = "install failed: ${e.message ?: e.javaClass.simpleName}",
                recoverable = true
            )
            finishInstallFailure(error)
            callback.onCompleted(
                RuntimeOperationResult.Failure(
                    operationId = operationId,
                    type = RuntimeOperationType.INSTALL,
                    error = error,
                    snapshot = stateStore.snapshot()
                )
            )
        }
        return handle
    }

    private fun mapInstallFailure(failure: RuntimeInstallResult.Failure): RuntimeError {
        val code = when (failure.code) {
            com.amitia.amitia_app.runtime.install.RuntimeInstallErrorCode.UNSUPPORTED_ABI -> RuntimeErrorCode.UNSUPPORTED_ABI
            com.amitia.amitia_app.runtime.install.RuntimeInstallErrorCode.PACKAGE_NOT_FOUND -> RuntimeErrorCode.PACKAGE_NOT_FOUND
            com.amitia.amitia_app.runtime.install.RuntimeInstallErrorCode.PACKAGE_INVALID,
            com.amitia.amitia_app.runtime.install.RuntimeInstallErrorCode.ARCHIVE_INVALID,
            com.amitia.amitia_app.runtime.install.RuntimeInstallErrorCode.ARCHIVE_PATH_INVALID,
            com.amitia.amitia_app.runtime.install.RuntimeInstallErrorCode.ARCHIVE_ENTRY_DUPLICATE,
            com.amitia.amitia_app.runtime.install.RuntimeInstallErrorCode.ARCHIVE_ENTRY_UNSUPPORTED -> RuntimeErrorCode.PACKAGE_INVALID
            com.amitia.amitia_app.runtime.install.RuntimeInstallErrorCode.PACKAGE_HASH_MISMATCH -> RuntimeErrorCode.CHECKSUM_MISMATCH
            com.amitia.amitia_app.runtime.install.RuntimeInstallErrorCode.INSUFFICIENT_STORAGE -> RuntimeErrorCode.STORAGE_INSUFFICIENT
            else -> RuntimeErrorCode.INSTALL_FAILED
        }
        val recoverable = code != RuntimeErrorCode.UNSUPPORTED_ABI &&
            code != RuntimeErrorCode.PACKAGE_INVALID &&
            code != RuntimeErrorCode.CHECKSUM_MISMATCH
        return RuntimeError(
            code = code,
            message = failure.message,
            recoverable = recoverable,
            detailsSource = buildMap {
                put("installCode", failure.code.name)
                put("phase", failure.phase.name)
                failure.transactionId?.let { put("transactionId", it) }
            },
        )
    }

    private fun finishInstallFailure(error: RuntimeError) {
        verifiedRuntimeVersionRef.set(null)
        val bootstrap = try { bootstrapper?.bootstrap() } catch (_: Throwable) { null }
        val target = when (bootstrap) {
            is RuntimeBootstrapResult.InstalledStopped -> RuntimeState.INSTALLED
            is RuntimeBootstrapResult.NotInstalled -> RuntimeState.NOT_INSTALLED
            is RuntimeBootstrapResult.Failed -> RuntimeState.CORRUPTED
            null -> RuntimeState.FAILED
        }
        val version = (bootstrap as? RuntimeBootstrapResult.InstalledStopped)?.runtimeVersion
        stateStore.update {
            it.copy(
                state = target,
                runtimeVersion = version ?: if (target == RuntimeState.NOT_INSTALLED) null else it.runtimeVersion,
                activeOperationId = null,
                activeOperationType = null,
                lastError = error,
                activeProfile = null,
            )
        }
    }

    private fun executeVerify(
        installer: RuntimeInstaller,
        request: RuntimeVerifyRequest,
        callback: RuntimeOperationCallback
    ): RuntimeOperationHandle {
        val operationId = idGenerator.nextOperationId()
        val handle = CompletedOperationHandle(operationId, RuntimeOperationType.VERIFY)
        val verifier = installedVerifier
        val layout = hostLayout
        if (verifier == null || layout == null) {
            callback.onCompleted(
                RuntimeOperationResult.Failure(
                    operationId = operationId,
                    type = RuntimeOperationType.VERIFY,
                    error = RuntimeError(
                        code = RuntimeErrorCode.NOT_IMPLEMENTED,
                        message = "installed runtime verifier is not available",
                        recoverable = false,
                    ),
                    snapshot = stateStore.snapshot(),
                )
            )
            return handle
        }

        val before = stateStore.snapshot()
        val transitionError = RuntimeStateMachine.requireTransitionTo(before.state, RuntimeState.VERIFYING)
        if (transitionError != null) {
            callback.onCompleted(
                RuntimeOperationResult.Failure(
                    operationId = operationId,
                    type = RuntimeOperationType.VERIFY,
                    error = RuntimeError(
                        code = RuntimeErrorCode.INVALID_STATE,
                        message = "cannot verify runtime from state: ${before.state}",
                        recoverable = true,
                    ),
                    snapshot = before,
                )
            )
            return handle
        }

        stateStore.update {
            it.copy(
                state = RuntimeState.VERIFYING,
                activeOperationId = operationId,
                activeOperationType = RuntimeOperationType.VERIFY,
            )
        }

        val bootstrap = try { bootstrapper?.bootstrap() } catch (_: Throwable) { null }
        val runtimeVersion = (bootstrap as? RuntimeBootstrapResult.InstalledStopped)?.runtimeVersion
            ?: before.runtimeVersion
        if (runtimeVersion == null) {
            val error = RuntimeError(
                code = RuntimeErrorCode.RUNTIME_NOT_INSTALLED,
                message = "no runtime version is available to verify",
                recoverable = true,
            )
            stateStore.update {
                it.copy(
                    state = RuntimeState.FAILED,
                    activeOperationId = null,
                    activeOperationType = null,
                    lastError = error,
                    activeProfile = null,
                )
            }
            callback.onCompleted(RuntimeOperationResult.Failure(operationId, RuntimeOperationType.VERIFY, error, stateStore.snapshot()))
            return handle
        }

        try {
            when (val verifyResult = verifier.verify(layout.runtimeVersionRoot(runtimeVersion))) {
                is com.amitia.amitia_app.runtime.install.InstalledRuntimeVerificationResult.Success -> {
                    verifiedRuntimeVersionRef.set(runtimeVersion)
                    stateStore.update {
                        it.copy(
                            state = RuntimeState.INSTALLED,
                            runtimeVersion = runtimeVersion,
                            activeOperationId = null,
                            activeOperationType = null,
                            lastError = null,
                            activeProfile = null,
                        )
                    }
                    callback.onCompleted(RuntimeOperationResult.Success(operationId, RuntimeOperationType.VERIFY, stateStore.snapshot()))
                }
                is com.amitia.amitia_app.runtime.install.InstalledRuntimeVerificationResult.Failure -> {
                    verifiedRuntimeVersionRef.set(null)
                    val error = RuntimeError(
                        code = RuntimeErrorCode.RUNTIME_CORRUPTED,
                        message = verifyResult.message,
                        recoverable = true,
                    )
                    stateStore.update {
                        it.copy(
                            state = RuntimeState.CORRUPTED,
                            activeOperationId = null,
                            activeOperationType = null,
                            lastError = error,
                            activeProfile = null,
                        )
                    }
                    callback.onCompleted(RuntimeOperationResult.Failure(operationId, RuntimeOperationType.VERIFY, error, stateStore.snapshot()))
                }
            }
        } catch (e: Exception) {
            verifiedRuntimeVersionRef.set(null)
            val error = RuntimeError(
                code = RuntimeErrorCode.VERIFY_FAILED,
                message = "runtime verification failed: ${e.message ?: e.javaClass.simpleName}",
                recoverable = true,
            )
            stateStore.update {
                it.copy(
                    state = RuntimeState.FAILED,
                    activeOperationId = null,
                    activeOperationType = null,
                    lastError = error,
                    activeProfile = null,
                )
            }
            callback.onCompleted(RuntimeOperationResult.Failure(operationId, RuntimeOperationType.VERIFY, error, stateStore.snapshot()))
        }
        return handle
    }

    private fun executeRepair(
        installer: RuntimeInstaller,
        request: RuntimeRepairRequest,
        callback: RuntimeOperationCallback
    ): RuntimeOperationHandle {
        val operationId = idGenerator.nextOperationId()
        val handle = CompletedOperationHandle(operationId, RuntimeOperationType.REPAIR)
        val before = stateStore.snapshot()
        val packageUri = request.packageUri?.trim()?.takeIf { it.isNotEmpty() }
        if (packageUri == null) {
            val error = RuntimeError(
                code = RuntimeErrorCode.REPAIR_FAILED,
                message = "repair requires a runtime package Uri",
                recoverable = false,
            )
            callback.onCompleted(RuntimeOperationResult.Failure(operationId, RuntimeOperationType.REPAIR, error, before))
            return handle
        }

        if (before.state !in setOf(
                RuntimeState.INSTALLED,
                RuntimeState.STOPPED,
                RuntimeState.CORRUPTED,
                RuntimeState.FAILED,
            )
        ) {
            val error = RuntimeError(
                code = RuntimeErrorCode.INVALID_STATE,
                message = "cannot repair runtime from state: ${before.state}; stop the runtime first",
                recoverable = true,
            )
            callback.onCompleted(RuntimeOperationResult.Failure(operationId, RuntimeOperationType.REPAIR, error, before))
            return handle
        }

        val ownershipError = startupOwnershipError()
        if (ownershipError != null) {
            callback.onCompleted(RuntimeOperationResult.Failure(operationId, RuntimeOperationType.REPAIR, ownershipError, before))
            return handle
        }

        val transitionError = RuntimeStateMachine.requireTransitionTo(before.state, RuntimeState.REPAIRING)
        if (transitionError != null) {
            val error = RuntimeError(
                code = RuntimeErrorCode.INVALID_STATE,
                message = "runtime cannot enter REPAIRING from ${before.state}",
                recoverable = true,
            )
            callback.onCompleted(RuntimeOperationResult.Failure(operationId, RuntimeOperationType.REPAIR, error, before))
            return handle
        }

        cancelPendingRecovery(resetBudget = true)
        cancelServiceSessionWatchdog()
        cancelStartupDetector()
        verifiedRuntimeVersionRef.set(null)
        stateStore.update {
            it.copy(
                state = RuntimeState.REPAIRING,
                activeOperationId = operationId,
                activeOperationType = RuntimeOperationType.REPAIR,
                lastError = null,
                activeProfile = null,
            )
        }

        try {
            val installResult = installer.install(
                com.amitia.amitia_app.runtime.install.RuntimeInstallRequest(
                    packageFile = java.io.File(packageUri),
                    expectedRuntimeVersion = before.runtimeVersion,
                    allowRepairExisting = true,
                )
            )
            when (installResult) {
                is RuntimeInstallResult.Success,
                is RuntimeInstallResult.AlreadyInstalled -> {
                    val bootstrapResult = bootstrapper?.bootstrap()
                    if (bootstrapResult !is RuntimeBootstrapResult.InstalledStopped) {
                        val error = RuntimeError(
                            code = RuntimeErrorCode.REPAIR_FAILED,
                            message = when (bootstrapResult) {
                                is RuntimeBootstrapResult.Failed -> "repair committed but runtime authority is inconsistent: ${bootstrapResult.message}"
                                is RuntimeBootstrapResult.NotInstalled -> "repair committed but no active runtime was published"
                                null -> "repair committed but runtime bootstrap authority is unavailable"
                                else -> "repair committed but runtime authority is inconsistent"
                            },
                            recoverable = true,
                        )
                        finishRepairFailure(error, before.runtimeVersion)
                        callback.onCompleted(RuntimeOperationResult.Failure(operationId, RuntimeOperationType.REPAIR, error, stateStore.snapshot()))
                        return handle
                    }

                    val verifier = installedVerifier
                    val layout = hostLayout
                    if (verifier != null && layout != null) {
                        when (val verified = verifier.verify(layout.runtimeVersionRoot(bootstrapResult.runtimeVersion))) {
                            is com.amitia.amitia_app.runtime.install.InstalledRuntimeVerificationResult.Failure -> {
                                val error = RuntimeError(
                                    code = RuntimeErrorCode.REPAIR_FAILED,
                                    message = "repaired runtime failed verification: ${verified.message}",
                                    recoverable = true,
                                )
                                finishRepairFailure(error, bootstrapResult.runtimeVersion)
                                callback.onCompleted(RuntimeOperationResult.Failure(operationId, RuntimeOperationType.REPAIR, error, stateStore.snapshot()))
                                return handle
                            }
                            is com.amitia.amitia_app.runtime.install.InstalledRuntimeVerificationResult.Success -> Unit
                        }
                    }

                    verifiedRuntimeVersionRef.set(bootstrapResult.runtimeVersion)
                    stateStore.update {
                        it.copy(
                            state = RuntimeState.INSTALLED,
                            runtimeVersion = bootstrapResult.runtimeVersion,
                            activeOperationId = null,
                            activeOperationType = null,
                            lastError = null,
                            activeProfile = null,
                        )
                    }
                    callback.onCompleted(RuntimeOperationResult.Success(operationId, RuntimeOperationType.REPAIR, stateStore.snapshot()))
                }
                is RuntimeInstallResult.Failure -> {
                    val error = RuntimeError(
                        code = RuntimeErrorCode.REPAIR_FAILED,
                        message = installResult.message,
                        recoverable = true,
                        detailsSource = buildMap {
                            put("installCode", installResult.code.name)
                            put("phase", installResult.phase.name)
                            installResult.transactionId?.let { put("transactionId", it) }
                        },
                    )
                    finishRepairFailure(error, before.runtimeVersion)
                    callback.onCompleted(RuntimeOperationResult.Failure(operationId, RuntimeOperationType.REPAIR, error, stateStore.snapshot()))
                }
            }
        } catch (e: Exception) {
            val error = RuntimeError(
                code = RuntimeErrorCode.REPAIR_FAILED,
                message = "repair failed: ${e.message ?: e.javaClass.simpleName}",
                recoverable = true,
            )
            finishRepairFailure(error, before.runtimeVersion)
            callback.onCompleted(RuntimeOperationResult.Failure(operationId, RuntimeOperationType.REPAIR, error, stateStore.snapshot()))
        }
        return handle
    }

    private fun finishRepairFailure(error: RuntimeError, previousRuntimeVersion: String?) {
        verifiedRuntimeVersionRef.set(null)
        val bootstrap = try { bootstrapper?.bootstrap() } catch (_: Throwable) { null }
        val target = when (bootstrap) {
            is RuntimeBootstrapResult.InstalledStopped -> RuntimeState.INSTALLED
            is RuntimeBootstrapResult.NotInstalled -> RuntimeState.NOT_INSTALLED
            is RuntimeBootstrapResult.Failed -> RuntimeState.CORRUPTED
            null -> RuntimeState.FAILED
        }
        val version = (bootstrap as? RuntimeBootstrapResult.InstalledStopped)?.runtimeVersion ?: previousRuntimeVersion
        stateStore.update {
            it.copy(
                state = target,
                runtimeVersion = if (target == RuntimeState.NOT_INSTALLED) null else version,
                activeOperationId = null,
                activeOperationType = null,
                lastError = error,
                activeProfile = null,
            )
        }
    }

    private class CompletedOperationHandle(
        override val operationId: String,
        override val type: RuntimeOperationType
    ) : RuntimeOperationHandle {
        override fun cancel(): Boolean = false
        override fun isCancelled(): Boolean = false
        override fun isCompleted(): Boolean = true
    }
}
