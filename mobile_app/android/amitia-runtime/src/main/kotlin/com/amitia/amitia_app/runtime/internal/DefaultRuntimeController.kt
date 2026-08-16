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
import com.amitia.amitia_app.runtime.service.RuntimeServiceResult
import com.amitia.amitia_app.runtime.service.RuntimeServiceTerminationCause
import com.amitia.amitia_app.runtime.startup.DefaultRuntimeStartupDetector
import com.amitia.amitia_app.runtime.startup.RuntimeStartupDetector
import com.amitia.amitia_app.runtime.startup.RuntimeStartupError
import com.amitia.amitia_app.runtime.startup.RuntimeStartupRequest
import com.amitia.amitia_app.runtime.startup.RuntimeStartupResult
import java.util.concurrent.atomic.AtomicBoolean
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
    private val startupDetectionThread = AtomicReference<Thread?>(null)
    private val pendingRecoveryJob = AtomicReference<com.amitia.amitia_app.runtime.recovery.RuntimeRecoveryJob?>(null)
    private val lastFailedGeneration = AtomicLong(-1L)
    private val lastFailedAttemptId = AtomicReference<String?>(null)

    init {
        serviceHost.addListener(serviceHostListener)
        runBootstrap()
    }

    private fun runBootstrap() {
        val bootstrapper = bootstrapper ?: return
        if (stateStore.isInitialized()) return

        when (val result = bootstrapper.bootstrap()) {
            is RuntimeBootstrapResult.NotInstalled -> {
                stateStore.initialize(RuntimeState.NOT_INSTALLED)
            }
            is RuntimeBootstrapResult.InstalledStopped -> {
                stateStore.initialize(RuntimeState.STOPPED, result.runtimeVersion)
            }
            is RuntimeBootstrapResult.Failed -> {
                stateStore.initialize(RuntimeState.FAILED)
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

    private fun onServiceHostEvent(event: RuntimeServiceHostEvent) {
        when (event) {
            is RuntimeServiceHostEvent.ForegroundStarted -> {
            }
            is RuntimeServiceHostEvent.SessionReady -> {
                onSessionReady(event.generation)
            }
            is RuntimeServiceHostEvent.ExpectedStopped -> {
                cancelPendingRecovery()
                cancelStartupDetector()
                val current = stateStore.snapshot()
                if (current.generation != event.generation) return
                val target = RuntimeStateMachine.expectedStopTarget(current.state)
                if (target != null && target != current.state) {
                    stateStore.update { it.copy(state = target) }
                }
                clearExpectedStop(event.generation)
            }
            is RuntimeServiceHostEvent.UnexpectedTermination -> {
                cancelStartupDetector()
                if (!isCurrentGeneration(event.generation)) return
                val current = stateStore.snapshot()
                if (!isRecoveryEligible(current, event.cause)) return
                val target = RuntimeStateMachine.unexpectedTerminationTarget(current.state)
                if (target != null && target != current.state) {
                    val error = mapTerminationCauseToError(event.cause)
                    stateStore.update { it.copy(state = target, lastError = error) }
                    clearExpectedStop(event.generation)
                    if (target == RuntimeState.FAILED) {
                        lastFailedGeneration.set(stateStore.snapshot().generation)
                        evaluateRecovery(error, requestedStop = false)
                    }
                }
            }
            is RuntimeServiceHostEvent.StartupFailed -> {
                cancelStartupDetector()
                if (!isCurrentGeneration(event.generation)) return
                val current = stateStore.snapshot()
                if (!isStartupFailedRecoveryEligible(current, event.cause)) return
                val target = RuntimeStateMachine.unexpectedTerminationTarget(current.state)
                if (target != null && target != current.state) {
                    val error = mapTerminationCauseToError(event.cause, event.message)
                    stateStore.update { it.copy(state = target, lastError = error) }
                    if (target == RuntimeState.FAILED) {
                        lastFailedGeneration.set(stateStore.snapshot().generation)
                        evaluateRecovery(error, requestedStop = false)
                    }
                }
            }
        }
    }

    private fun onSessionReady(generation: Long) {
        if (!isCurrentGeneration(generation)) return

        val current = stateStore.snapshot()
        if (current.state != RuntimeState.STARTING) return
        if (current.generation != generation) return

        val attemptId = idGenerator.nextOperationId()
        currentStartAttemptId.set(attemptId)

        val session = serviceHost.currentSession()
        if (session == null) {
            val target = RuntimeStateMachine.startupFailureTarget(current.state)
            if (target != null) {
                stateStore.update {
                    it.copy(
                        state = target,
                        lastError = RuntimeError(
                            code = RuntimeErrorCode.START_FAILED,
                            message = "startup session is not available after SessionReady event",
                            recoverable = true
                        )
                    )
                }
            }
            return
        }
        if (!session.isAlive()) {
            val target = RuntimeStateMachine.startupFailureTarget(current.state)
            if (target != null) {
                stateStore.update {
                    it.copy(
                        state = target,
                        lastError = RuntimeError(
                            code = RuntimeErrorCode.START_FAILED,
                            message = "startup session exited before detection could start",
                            recoverable = true
                        )
                    )
                }
            }
            return
        }

        activeDetectorSession.set(session)
        startStartupDetection(session, generation, attemptId)
    }

    private fun startStartupDetection(session: ProotSession, generation: Long, attemptId: String) {
        cancelStartupDetector()
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
        } catch (_: IllegalThreadStateException) {
            startupDetectionThread.set(null)
            activeDetectorSession.set(null)
        } catch (_: Throwable) {
            startupDetectionThread.set(null)
            activeDetectorSession.set(null)
        }
    }

    private fun runStartupDetection(session: ProotSession, generation: Long, attemptId: String) {
        val request = RuntimeStartupRequest(
            generation = generation,
            session = session,
            endpoint = endpointPolicy,
            startAttemptId = attemptId
        )
        val result = startupDetector.awaitStartup(request)
        onStartupDetectionCompleted(result, generation, attemptId)
    }

    private fun onStartupDetectionCompleted(result: RuntimeStartupResult, expectedGeneration: Long, expectedAttemptId: String) {
        startupDetectionThread.set(null)
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
                        recordRecoveryReady(expectedGeneration)
                    }
                }
            }
            is RuntimeStartupResult.Failed -> {
                cancelStartupDetector()
                serviceHost.requestTeardownAfterStartupFailure()
            }
            is RuntimeStartupResult.Cancelled -> {
            }
        }
    }

    private fun mapStartupErrorToRuntimeError(error: RuntimeStartupError): RuntimeError {
        return when (error) {
            is RuntimeStartupError.Cancelled -> RuntimeError(
                code = RuntimeErrorCode.OPERATION_CANCELLED,
                message = "startup detection was cancelled",
                recoverable = true
            )
            is RuntimeStartupError.ProotNotRunning,
            is RuntimeStartupError.ProotExited -> RuntimeError(
                code = RuntimeErrorCode.RUNTIME_EXECUTION_NOT_AVAILABLE,
                message = when (error) {
                    is RuntimeStartupError.ProotExited -> "outer proot process exited with code: ${error.exitCode ?: "unknown"}"
                    else -> "outer proot process is not running"
                },
                recoverable = true
            )
            is RuntimeStartupError.Timeout -> RuntimeError(
                code = RuntimeErrorCode.TIMEOUT,
                message = "startup timed out after ${error.elapsedMs}ms (${error.probeCount} probes)",
                recoverable = true
            )
            is RuntimeStartupError.InvalidEndpoint -> RuntimeError(
                code = RuntimeErrorCode.INVALID_REQUEST,
                message = "invalid backend endpoint",
                recoverable = true
            )
            is RuntimeStartupError.BackendConnectionRefused,
            is RuntimeStartupError.BackendLivenessFailed,
            is RuntimeStartupError.BackendReadinessFailed,
            is RuntimeStartupError.HealthAuthFailed,
            is RuntimeStartupError.HealthEndpointMissing,
            is RuntimeStartupError.InvalidResponse -> RuntimeError(
                code = RuntimeErrorCode.START_FAILED,
                message = when (error) {
                    is RuntimeStartupError.BackendConnectionRefused -> "backend connection refused"
                    is RuntimeStartupError.BackendLivenessFailed -> "backend liveness check failed"
                    is RuntimeStartupError.BackendReadinessFailed -> "backend readiness check failed"
                    is RuntimeStartupError.HealthAuthFailed -> "backend readiness probe returned auth error"
                    is RuntimeStartupError.HealthEndpointMissing -> "backend readiness endpoint is missing"
                    is RuntimeStartupError.InvalidResponse -> "invalid readiness response: ${error.reason}"
                    else -> "startup detection failed"
                },
                recoverable = true
            )
            else -> RuntimeError(
                code = RuntimeErrorCode.START_FAILED,
                message = "startup detection failed",
                recoverable = true
            )
        }
    }

    private fun cancelStartupDetector() {
        currentStartAttemptId.set(null)
        activeDetectorSession.set(null)
        try {
            startupDetector.cancel()
        } catch (_: Throwable) {
        }
        val thread = startupDetectionThread.getAndSet(null)
        if (thread != null && thread.isAlive) {
            try {
                thread.interrupt()
            } catch (_: Throwable) {
            }
        }
    }

    private fun evaluateRecovery(error: RuntimeError, requestedStop: Boolean) {
        val current = stateStore.snapshot()
        if (current.state != RuntimeState.FAILED) return
        cancelPendingRecovery()
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
        cancelPendingRecovery()
        val failedGen = lastFailedGeneration.get()
        if (failedGen <= 0) return
        try {
            val job = recoveryScheduler.schedule(delayMillis) {
                executeRecoveryStart(failedGen)
            }
            pendingRecoveryJob.set(job)
        } catch (_: Throwable) {
        }
    }

    private fun executeRecoveryStart(failedGeneration: Long) {
        val current = stateStore.snapshot()
        if (current.state != RuntimeState.FAILED) return
        if (current.generation != failedGeneration) return
        cancelPendingRecoveryStartPrerequisites(failedGeneration)
    }

    private fun cancelPendingRecoveryStartPrerequisites(failedGeneration: Long): Boolean {
        val liveSession = serviceHost.currentSession()
        if (liveSession != null && liveSession.isAlive()) return false
        val snapshot = serviceHost.currentGeneration()
        if (snapshot != failedGeneration) return false
        cancelPendingRecovery()
        start(
            RuntimeStartRequest(reason = RuntimeStartReason.RECOVERY),
            object : RuntimeOperationCallback {
                override fun onCompleted(result: RuntimeOperationResult) {}
            }
        )
        return true
    }

    private fun cancelPendingRecovery() {
        val job = pendingRecoveryJob.getAndSet(null)
        if (job != null) {
            try {
                job.cancel()
            } catch (_: Throwable) {
            }
        }
        try {
            recoveryPolicy.cancelPending()
        } catch (_: Throwable) {
        }
    }

    private fun recordRecoveryReady(generation: Long) {
        cancelPendingRecovery()
        try {
            recoveryPolicy.recordReady(generation)
        } catch (_: Throwable) {
        }
    }

    private fun mapTerminationCauseToError(cause: RuntimeServiceTerminationCause, detail: String? = null): RuntimeError {
        return when (cause) {
            RuntimeServiceTerminationCause.FOREGROUND_FAILED -> RuntimeError(
                code = RuntimeErrorCode.START_FAILED,
                message = "foreground service start failed",
                recoverable = true
            )
            RuntimeServiceTerminationCause.NOTIFICATION_FAILED -> RuntimeError(
                code = RuntimeErrorCode.START_FAILED,
                message = "notification creation failed",
                recoverable = true
            )
            RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR -> RuntimeError(
                code = RuntimeErrorCode.RUNTIME_EXECUTION_NOT_AVAILABLE,
                message = "runtime service unexpectedly terminated",
                recoverable = true
            )
            RuntimeServiceTerminationCause.SESSION_EXITED -> RuntimeError(
                code = RuntimeErrorCode.RUNTIME_EXECUTION_NOT_AVAILABLE,
                message = "outer proot session exited unexpectedly",
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
                message = "exit watcher failed to observe process termination",
                recoverable = true
            )
            RuntimeServiceTerminationCause.STOP_RESULT_FAILED -> RuntimeError(
                code = RuntimeErrorCode.STOP_SERVICE_TEARDOWN_FAILED,
                message = "stopSelfResult returned false, service did not stop as expected",
                recoverable = true
            )
        }
    }

    override fun snapshot(): RuntimeSnapshot = stateStore.snapshot()

    fun lifecycleSnapshot(): com.amitia.amitia_app.runtime.service.RuntimeServiceLifecycleSnapshot? {
        return serviceHost.let { host ->
            (host as? com.amitia.amitia_app.runtime.service.internal.AndroidRuntimeServiceHost)?.let {
                com.amitia.amitia_app.runtime.service.internal.RuntimeServiceLifecycleSnapshot(
                    generation = stateStore.snapshot().generation,
                    sessionId = null,
                    servicePhase = com.amitia.amitia_app.runtime.service.internal.ServicePhase.CREATED,
                    processPhase = com.amitia.amitia_app.runtime.service.internal.ProcessPhase.CREATED,
                    startupPhase = com.amitia.amitia_app.runtime.service.internal.StartupPhase.NOT_STARTED,
                    terminalState = null,
                    latestStartId = 0,
                    stopRequested = false,
                )
            }
        }
    }

    override fun subscribe(listener: RuntimeListener): RuntimeSubscription = stateStore.subscribe(listener)

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

    override fun start(
        request: RuntimeStartRequest,
        callback: RuntimeOperationCallback
    ): RuntimeOperationHandle {
        val operationId = idGenerator.nextOperationId()
        val handle = CompletedOperationHandle(operationId, RuntimeOperationType.START)

        cancelStartupDetector()

        val current = stateStore.snapshot()
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

        cancelPendingRecovery()

        val newSnapshot = stateStore.transitionToStarting()
        val allocatedGeneration = newSnapshot.generation

        expectedStopRef.set(null)

        val startResult = serviceHost.ensureStarted(allocatedGeneration, request.profile)
        if (startResult is RuntimeServiceResult.Failure) {
            val error = RuntimeError(
                code = RuntimeErrorCode.START_FAILED,
                message = "failed to ensure service is started: ${startResult.error.message}",
                recoverable = true
            )
            val target = RuntimeStateMachine.startupFailureTarget(RuntimeState.STARTING)
            if (target != null) {
                stateStore.update { it.copy(state = target, lastError = error) }
            }
            callback.onCompleted(
                RuntimeOperationResult.Failure(
                    operationId = operationId,
                    type = RuntimeOperationType.START,
                    error = error,
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
            cancelPendingRecovery()
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
        try {
            val installResult = installer.install(
                com.amitia.amitia_app.runtime.install.RuntimeInstallRequest(
                    packageFile = java.io.File(request.packageUri),
                    expectedRuntimeVersion = request.expectedVersion
                )
            )
            when (installResult) {
                is RuntimeInstallResult.Success,
                is RuntimeInstallResult.AlreadyInstalled -> {
                    val bootstrapResult = bootstrapper?.bootstrap()
                    if (bootstrapResult !is RuntimeBootstrapResult.InstalledStopped) {
                        callback.onCompleted(
                            RuntimeOperationResult.Failure(
                                operationId = operationId,
                                type = RuntimeOperationType.INSTALL,
                                error = RuntimeError(
                                    code = RuntimeErrorCode.INSTALL_FAILED,
                                    message = "install succeeded but runtime authority not consistent",
                                    recoverable = true
                                ),
                                snapshot = stateStore.snapshot()
                            )
                        )
                        return handle
                    }
                    stateStore.update {
                        it.copy(
                            state = RuntimeState.STOPPED,
                            runtimeVersion = bootstrapResult.runtimeVersion
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
                    callback.onCompleted(
                        RuntimeOperationResult.Failure(
                            operationId = operationId,
                            type = RuntimeOperationType.INSTALL,
                            error = RuntimeError(
                                code = RuntimeErrorCode.INSTALL_FAILED,
                                message = installResult.message,
                                recoverable = true
                            ),
                            snapshot = stateStore.snapshot()
                        )
                    )
                }
            }
        } catch (e: Exception) {
            callback.onCompleted(
                RuntimeOperationResult.Failure(
                    operationId = operationId,
                    type = RuntimeOperationType.INSTALL,
                    error = RuntimeError(
                        code = RuntimeErrorCode.INSTALL_FAILED,
                        message = "install failed: ${e.message}",
                        recoverable = true
                    ),
                    snapshot = stateStore.snapshot()
                )
            )
        }
        return handle
    }

    private fun executeVerify(
        installer: RuntimeInstaller,
        request: RuntimeVerifyRequest,
        callback: RuntimeOperationCallback
    ): RuntimeOperationHandle {
        val operationId = idGenerator.nextOperationId()
        val handle = CompletedOperationHandle(operationId, RuntimeOperationType.VERIFY)

        val verifier = installedVerifier
        if (verifier == null) {
            callback.onCompleted(
                RuntimeOperationResult.Failure(
                    operationId = operationId,
                    type = RuntimeOperationType.VERIFY,
                    error = RuntimeError(
                        code = RuntimeErrorCode.NOT_IMPLEMENTED,
                        message = "installed runtime verifier not available",
                        recoverable = false
                    ),
                    snapshot = stateStore.snapshot()
                )
            )
            return handle
        }

        val bootstrap = bootstrapper?.bootstrap()
        val runtimeVersion = when (bootstrap) {
            is RuntimeBootstrapResult.InstalledStopped -> bootstrap.runtimeVersion
            else -> stateStore.snapshot().runtimeVersion
        }

        if (runtimeVersion == null) {
            callback.onCompleted(
                RuntimeOperationResult.Failure(
                    operationId = operationId,
                    type = RuntimeOperationType.VERIFY,
                    error = RuntimeError(
                        code = RuntimeErrorCode.INVALID_STATE,
                        message = "no runtime version available to verify",
                        recoverable = true
                    ),
                    snapshot = stateStore.snapshot()
                )
            )
            return handle
        }

        val layout = hostLayout
        if (layout == null) {
            callback.onCompleted(
                RuntimeOperationResult.Failure(
                    operationId = operationId,
                    type = RuntimeOperationType.VERIFY,
                    error = RuntimeError(
                        code = RuntimeErrorCode.NOT_IMPLEMENTED,
                        message = "host layout not available for verify",
                        recoverable = false
                    ),
                    snapshot = stateStore.snapshot()
                )
            )
            return handle
        }

        val versionDir = layout.runtimeVersionRoot(runtimeVersion)
        val verifyResult = verifier.verify(versionDir)

        when (verifyResult) {
            is com.amitia.amitia_app.runtime.install.InstalledRuntimeVerificationResult.Success -> {
                callback.onCompleted(
                    RuntimeOperationResult.Success(
                        operationId = operationId,
                        type = RuntimeOperationType.VERIFY,
                        snapshot = stateStore.snapshot()
                    )
                )
            }
            is com.amitia.amitia_app.runtime.install.InstalledRuntimeVerificationResult.Failure -> {
                stateStore.update { it.copy(state = RuntimeState.FAILED) }
                callback.onCompleted(
                    RuntimeOperationResult.Failure(
                        operationId = operationId,
                        type = RuntimeOperationType.VERIFY,
                        error = RuntimeError(
                            code = RuntimeErrorCode.RUNTIME_VERIFY_FAILED,
                            message = verifyResult.message,
                            recoverable = true
                        ),
                        snapshot = stateStore.snapshot()
                    )
                )
            }
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
        try {
            val packageFile = request.packageUri?.let { java.io.File(it) }
            if (packageFile != null) {
                val installResult = installer.install(
                    com.amitia.amitia_app.runtime.install.RuntimeInstallRequest(
                        packageFile = packageFile,
                        expectedRuntimeVersion = null
                    )
                )
                when (installResult) {
                    is RuntimeInstallResult.Success,
                    is RuntimeInstallResult.AlreadyInstalled -> {
                        val bootstrapResult = bootstrapper?.bootstrap()
                        if (bootstrapResult !is RuntimeBootstrapResult.InstalledStopped) {
                            callback.onCompleted(
                                RuntimeOperationResult.Failure(
                                    operationId = operationId,
                                    type = RuntimeOperationType.REPAIR,
                                    error = RuntimeError(
                                        code = RuntimeErrorCode.REPAIR_FAILED,
                                        message = "repair succeeded but runtime authority not consistent",
                                        recoverable = true
                                    ),
                                    snapshot = stateStore.snapshot()
                                )
                            )
                            return handle
                        }
                        stateStore.update {
                            it.copy(
                                state = RuntimeState.STOPPED,
                                runtimeVersion = bootstrapResult.runtimeVersion
                            )
                        }
                        callback.onCompleted(
                            RuntimeOperationResult.Success(
                                operationId = operationId,
                                type = RuntimeOperationType.REPAIR,
                                snapshot = stateStore.snapshot()
                            )
                        )
                    }
                    is RuntimeInstallResult.Failure -> {
                        callback.onCompleted(
                            RuntimeOperationResult.Failure(
                                operationId = operationId,
                                type = RuntimeOperationType.REPAIR,
                                error = RuntimeError(
                                    code = RuntimeErrorCode.REPAIR_FAILED,
                                    message = installResult.message,
                                    recoverable = true
                                ),
                                snapshot = stateStore.snapshot()
                            )
                        )
                    }
                }
            } else {
                callback.onCompleted(
                    RuntimeOperationResult.Failure(
                        operationId = operationId,
                        type = RuntimeOperationType.REPAIR,
                        error = RuntimeError(
                            code = RuntimeErrorCode.REPAIR_FAILED,
                            message = "repair requires a package Uri",
                            recoverable = false
                        ),
                        snapshot = stateStore.snapshot()
                    )
                )
            }
        } catch (e: Exception) {
            callback.onCompleted(
                RuntimeOperationResult.Failure(
                    operationId = operationId,
                    type = RuntimeOperationType.REPAIR,
                    error = RuntimeError(
                        code = RuntimeErrorCode.REPAIR_FAILED,
                        message = "repair failed: ${e.message}",
                        recoverable = true
                    ),
                    snapshot = stateStore.snapshot()
                )
            )
        }
        return handle
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

private typealias AtomicLong = java.util.concurrent.atomic.AtomicLong
