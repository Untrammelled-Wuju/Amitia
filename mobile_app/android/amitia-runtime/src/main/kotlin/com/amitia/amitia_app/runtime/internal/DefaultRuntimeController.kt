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
import com.amitia.amitia_app.runtime.api.RuntimeStartRequest
import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.api.RuntimeStopRequest
import com.amitia.amitia_app.runtime.api.RuntimeSubscription
import com.amitia.amitia_app.runtime.api.RuntimeVerifyRequest
import com.amitia.amitia_app.runtime.abi.RuntimeAbiGate
import com.amitia.amitia_app.runtime.connection.BackendEndpointPolicy
import com.amitia.amitia_app.runtime.connection.embeddedAndroidBackendPolicy
import com.amitia.amitia_app.runtime.install.ActiveRuntimeManager
import com.amitia.amitia_app.runtime.install.RuntimeInstaller
import com.amitia.amitia_app.runtime.proot.ProotComponent
import com.amitia.amitia_app.runtime.proot.ProotErrorCode
import com.amitia.amitia_app.runtime.proot.ProotSession
import com.amitia.amitia_app.runtime.proot.ProotStopResult
import com.amitia.amitia_app.runtime.recovery.DefaultRuntimeCrashRecoveryPolicy
import com.amitia.amitia_app.runtime.recovery.ExecutorRuntimeRecoveryScheduler
import com.amitia.amitia_app.runtime.recovery.InstalledRuntimeResult
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
import java.util.concurrent.atomic.AtomicReference

internal class DefaultRuntimeController(
    private val stateStore: RuntimeStateStore,
    private val serviceHost: RuntimeServiceHost,
    private val installer: RuntimeInstaller? = null,
    private val abiGate: RuntimeAbiGate? = null,
    private val activeRuntime: ActiveRuntimeManager? = null,
    private val idGenerator: RuntimeIdGenerator = UuidRuntimeIdGenerator,
    private val clock: RuntimeClock = SystemRuntimeClock,
    private val prootComponent: ProotComponent? = null,
    private val startupDetector: RuntimeStartupDetector? = DefaultRuntimeStartupDetector(),
    private val backendEndpoint: BackendEndpointPolicy = embeddedAndroidBackendPolicy(),
    private val recoveryPolicy: RuntimeCrashRecoveryPolicy = DefaultRuntimeCrashRecoveryPolicy(
        com.amitia.amitia_app.runtime.recovery.ActiveRuntimeBackedInstalledRuntimeSource(activeRuntime),
    ),
    private val recoveryScheduler: RuntimeRecoveryScheduler = ExecutorRuntimeRecoveryScheduler(),
) : RuntimeController {

    private val expectedStopRequested = AtomicReference(false)
    private val startupDetectors = AtomicReference<RuntimeStartupDetector?>(null)
    private val pendingRecoveryJob = AtomicReference<com.amitia.amitia_app.runtime.recovery.RuntimeRecoveryJob?>(null)
    private val serviceHostListener = RuntimeServiceHostListener { event ->
        onServiceHostEvent(event)
    }

    init {
        serviceHost.addListener(serviceHostListener)
    }

    private fun cancelPendingRecovery() {
        pendingRecoveryJob.getAndSet(null)?.cancel()
        recoveryPolicy.cancelPending()
    }

    private fun onServiceHostEvent(event: RuntimeServiceHostEvent) {
        when (event) {
            is RuntimeServiceHostEvent.ForegroundStarted -> {
            }
            is RuntimeServiceHostEvent.ExpectedStopped -> {
                cancelPendingRecovery()
                val current = stateStore.snapshot()
                val target = RuntimeStateMachine.expectedStopTarget(current.state)
                if (target != null && target != current.state) {
                    stateStore.update { it.copy(state = target) }
                }
            }
            is RuntimeServiceHostEvent.UnexpectedTermination -> {
                cancelStartupDetector()
                cancelPendingRecovery()
                val current = stateStore.snapshot()
                val target = RuntimeStateMachine.unexpectedTerminationTarget(current.state)
                if (target != null && target != current.state) {
                    val error = mapTerminationCauseToError(event.cause)
                    stateStore.update { it.copy(state = target, lastError = error) }
                    evaluateRecovery(error, requestedStop = false)
                }
            }
        }
    }

    private fun evaluateRecovery(error: RuntimeError, requestedStop: Boolean) {
        val current = stateStore.snapshot()
        val request = RuntimeRecoveryRequest(
            failedGeneration = current.generation,
            currentState = current.state,
            error = error,
            requestedStop = requestedStop,
        )
        val decision = recoveryPolicy.evaluate(request)
        when (decision) {
            is RuntimeRecoveryDecision.DoNotRecover -> { /* do nothing */ }
            is RuntimeRecoveryDecision.RecoverAfter -> {
                val failedGen = current.generation
                val job = recoveryScheduler.schedule(delayMillis = decision.delayMillis) {
                    if (stateStore.snapshot().generation != failedGen) return@schedule
                    if (pendingRecoveryJob.get()?.isCancelled != false) return@schedule
                    start(RuntimeStartRequest(reason = com.amitia.amitia_app.runtime.api.RuntimeStartReason.RECOVERY), object : RuntimeOperationCallback {
                        override fun onCompleted(result: RuntimeOperationResult) { }
                    })
                }
                pendingRecoveryJob.set(job)
            }
            is RuntimeRecoveryDecision.Exhausted -> {
                stateStore.update {
                    it.copy(lastError = RuntimeError(
                        code = RuntimeErrorCode.RECOVERY_EXHAUSTED,
                        message = "recovery budget exhausted after ${decision.attempts} attempts",
                        recoverable = false,
                    ))
                }
            }
        }
    }

    private fun cancelStartupDetector() {
        startupDetectors.getAndSet(null)?.cancel()
        startupDetector?.cancel()
    }

    private fun mapTerminationCauseToError(cause: RuntimeServiceTerminationCause): RuntimeError {
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
        }
    }

    override fun snapshot(): RuntimeSnapshot = stateStore.snapshot()

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
        stateStore.update { it.copy(state = RuntimeState.STARTING) }

        val startResult = serviceHost.ensureStarted()
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

        val proot = prootComponent
        if (proot == null || startupDetector == null) {
            val error = RuntimeError(
                code = RuntimeErrorCode.RUNTIME_EXECUTION_NOT_AVAILABLE,
                message = "runtime execution layer is not available",
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

        val session = proot.currentSession()
        if (session == null || !session.isAlive()) {
            val error = RuntimeError(
                code = RuntimeErrorCode.START_FAILED,
                message = "no active proot session available",
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

        val currentGeneration = stateStore.snapshot().generation
        val startupRequest = RuntimeStartupRequest(
            generation = currentGeneration,
            session = session,
            endpoint = backendEndpoint
        )

        cancelStartupDetector()
        val detector = startupDetector
        startupDetectors.set(detector)

        val startupThread = Thread {
            val result = detector.awaitStartup(startupRequest)
            handleStartupResult(result, operationId, callback)
        }.apply {
            isDaemon = true
            name = "runtime-startup-detector"
            start()
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

    private fun handleStartupResult(
        result: RuntimeStartupResult,
        operationId: String,
        callback: RuntimeOperationCallback
    ) {
        val current = stateStore.snapshot()
        if (result.generation != current.generation) {
            return
        }

        when (result) {
            is RuntimeStartupResult.Ready -> {
                val target = RuntimeStateMachine.startupReadyTarget(current.state)
                if (target != null && target != current.state) {
                    stateStore.update { it.copy(state = target) }
                    recoveryPolicy.recordReady(current.generation)
                }
            }
            is RuntimeStartupResult.Failed -> {
                canceledPendingRecovery()
                val target = RuntimeStateMachine.startupFailureTarget(current.state)
                if (target != null && target != current.state) {
                    val error = mapStartupErrorToRuntimeError(result.error)
                    stateStore.update { state ->
                        if (state.lastError != null) {
                            state.copy(state = target)
                        } else {
                            state.copy(state = target, lastError = error)
                        }
                    }
                    evaluateRecovery(error, requestedStop = false)
                }
            }
            is RuntimeStartupResult.Cancelled -> {
            }
        }
        startupDetectors.compareAndSet(startupDetector, null)
    }

    private fun canceledPendingRecovery() {
        pendingRecoveryJob.getAndSet(null)?.cancel()
    }

    private fun mapStartupErrorToRuntimeError(error: RuntimeStartupError): RuntimeError {
        return when (error) {
            RuntimeStartupError.Cancelled -> RuntimeError(
                code = RuntimeErrorCode.STARTUP_CANCELLED,
                message = "startup was cancelled",
                recoverable = true
            )
            RuntimeStartupError.GenerationStale -> RuntimeError(
                code = RuntimeErrorCode.STARTUP_GENERATION_STALE,
                message = "startup result from stale generation",
                recoverable = true
            )
            RuntimeStartupError.ProotNotRunning -> RuntimeError(
                code = RuntimeErrorCode.STARTUP_PROOT_NOT_RUNNING,
                message = "proot is not running",
                recoverable = true
            )
            is RuntimeStartupError.ProotExited -> RuntimeError(
                code = RuntimeErrorCode.STARTUP_PROOT_EXITED,
                message = "proot exited during startup with code ${error.exitCode}",
                recoverable = true
            )
            RuntimeStartupError.BackendConnectionRefused -> RuntimeError(
                code = RuntimeErrorCode.STARTUP_BACKEND_CONNECTION_REFUSED,
                message = "backend connection refused",
                recoverable = true
            )
            RuntimeStartupError.BackendLivenessFailed -> RuntimeError(
                code = RuntimeErrorCode.STARTUP_BACKEND_LIVENESS_FAILED,
                message = "backend liveness check failed",
                recoverable = true
            )
            RuntimeStartupError.BackendReadinessFailed -> RuntimeError(
                code = RuntimeErrorCode.STARTUP_BACKEND_READINESS_FAILED,
                message = "backend readiness check failed",
                recoverable = true
            )
            RuntimeStartupError.HealthAuthFailed -> RuntimeError(
                code = RuntimeErrorCode.STARTUP_HEALTH_AUTH_FAILED,
                message = "health endpoint authentication failed",
                recoverable = false
            )
            RuntimeStartupError.HealthEndpointMissing -> RuntimeError(
                code = RuntimeErrorCode.STARTUP_HEALTH_ENDPOINT_MISSING,
                message = "health endpoint not found",
                recoverable = true
            )
            RuntimeStartupError.Timeout -> RuntimeError(
                code = RuntimeErrorCode.STARTUP_TIMEOUT,
                message = "runtime startup timed out",
                recoverable = true
            )
            RuntimeStartupError.InvalidEndpoint -> RuntimeError(
                code = RuntimeErrorCode.STARTUP_INVALID_ENDPOINT,
                message = "invalid backend endpoint",
                recoverable = true
            )
            is RuntimeStartupError.InternalError -> RuntimeError(
                code = RuntimeErrorCode.INTERNAL_ERROR,
                message = "internal error: ${error.message}",
                recoverable = true
            )
        }
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
                    RuntimeOperationResult.Failure(
                        operationId = operationId,
                        type = RuntimeOperationType.STOP,
                        error = RuntimeError(
                            code = RuntimeErrorCode.STOP_ALREADY_IN_PROGRESS,
                            message = "stop is already in progress",
                            recoverable = false
                        ),
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

            expectedStopRequested.set(true)
            cancelPendingRecovery()
            cancelStartupDetector()

            val proot = prootComponent
            if (proot != null) {
                val session = proot.currentSession()
                if (session != null && session.isAlive()) {
                    val sessionResult = proot.stop()
                    if (sessionResult is ProotStopResult.Failed) {
                        val error = when (sessionResult.errorCode) {
                            ProotErrorCode.PROCESS_TIMEOUT -> RuntimeError(
                                code = RuntimeErrorCode.STOP_GRACEFUL_TIMEOUT,
                                message = "graceful stop timed out: ${sessionResult.message}",
                                recoverable = true
                            )
                            else -> RuntimeError(
                                code = RuntimeErrorCode.STOP_FORCE_FAILED,
                                message = "failed to stop proot session: ${sessionResult.message}",
                                recoverable = true
                            )
                        }
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
                }
            }

            stateStore.update { it.copy(state = RuntimeState.STOPPING) }

            val stopResult = serviceHost.requestStop()
            if (stopResult is RuntimeServiceResult.Failure) {
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
            val result = installer.install(
                com.amitia.amitia_app.runtime.install.RuntimeInstallRequest(
                    packageFile = java.io.File(request.packageUri),
                    expectedRuntimeVersion = request.expectedVersion
                )
            )
            callback.onCompleted(
                RuntimeOperationResult.Success(
                    operationId = operationId,
                    type = RuntimeOperationType.INSTALL,
                    snapshot = stateStore.snapshot()
                )
            )
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
        callback.onCompleted(
            RuntimeOperationResult.Success(
                operationId = operationId,
                type = RuntimeOperationType.VERIFY,
                snapshot = stateStore.snapshot()
            )
        )
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
                val result = installer.install(
                    com.amitia.amitia_app.runtime.install.RuntimeInstallRequest(
                        packageFile = packageFile,
                        expectedRuntimeVersion = null
                    )
                )
            }
            callback.onCompleted(
                RuntimeOperationResult.Success(
                    operationId = operationId,
                    type = RuntimeOperationType.REPAIR,
                    snapshot = stateStore.snapshot()
                )
            )
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
