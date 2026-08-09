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
import com.amitia.amitia_app.runtime.api.RuntimeStopRequest
import com.amitia.amitia_app.runtime.api.RuntimeSubscription
import com.amitia.amitia_app.runtime.api.RuntimeVerifyRequest
import com.amitia.amitia_app.runtime.abi.RuntimeAbiGate
import com.amitia.amitia_app.runtime.install.ActiveRuntimeManager
import com.amitia.amitia_app.runtime.install.RuntimeInstaller
import com.amitia.amitia_app.runtime.service.RuntimeServiceHost
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostEvent
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener
import com.amitia.amitia_app.runtime.service.RuntimeServiceTerminationCause
import java.util.concurrent.atomic.AtomicBoolean

internal class DefaultRuntimeController(
    private val stateStore: RuntimeStateStore,
    private val serviceHost: RuntimeServiceHost,
    private val installer: RuntimeInstaller? = null,
    private val abiGate: RuntimeAbiGate? = null,
    private val activeRuntime: ActiveRuntimeManager? = null,
    private val idGenerator: RuntimeIdGenerator = UuidRuntimeIdGenerator,
    private val clock: RuntimeClock = SystemRuntimeClock
) : RuntimeController {

    private val expectedStopRequested = AtomicBoolean(false)
    private val serviceHostListener = RuntimeServiceHostListener { event ->
        onServiceHostEvent(event)
    }

    init {
        serviceHost.addListener(serviceHostListener)
    }

    private fun onServiceHostEvent(event: RuntimeServiceHostEvent) {
        when (event) {
            is RuntimeServiceHostEvent.ForegroundStarted -> {
            }
            is RuntimeServiceHostEvent.ExpectedStopped -> {
                val current = stateStore.snapshot()
                val target = RuntimeStateMachine.expectedStopTarget(current.state)
                if (target != null && target != current.state) {
                    stateStore.update { it.copy(state = target) }
                }
            }
            is RuntimeServiceHostEvent.UnexpectedTermination -> {
                val current = stateStore.snapshot()
                val target = RuntimeStateMachine.unexpectedTerminationTarget(current.state)
                if (target != null && target != current.state) {
                    val error = mapTerminationCauseToError(event.cause)
                    stateStore.update { it.copy(state = target, lastError = error) }
                }
            }
        }
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

        val startResult = serviceHost.ensureStarted()
        if (startResult is com.amitia.amitia_app.runtime.service.RuntimeServiceResult.Failure) {
            val error = RuntimeError(
                code = RuntimeErrorCode.START_FAILED,
                message = "failed to ensure service is started: ${startResult.error.message}",
                recoverable = true
            )
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

        val error = RuntimeError(
            code = RuntimeErrorCode.RUNTIME_EXECUTION_NOT_AVAILABLE,
            message = "runtime execution layer is not available",
            recoverable = true
        )
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

    override fun stop(
        request: RuntimeStopRequest,
        callback: RuntimeOperationCallback
    ): RuntimeOperationHandle {
        expectedStopRequested.set(true)
        val operationId = idGenerator.nextOperationId()
        val handle = CompletedOperationHandle(operationId, RuntimeOperationType.STOP)

        val stopResult = serviceHost.requestStop()
        if (stopResult is com.amitia.amitia_app.runtime.service.RuntimeServiceResult.Failure) {
            val error = RuntimeError(
                code = RuntimeErrorCode.STOP_FAILED,
                message = "failed to request service stop: ${stopResult.error.message}",
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

        callback.onCompleted(
            RuntimeOperationResult.Success(
                operationId = operationId,
                type = RuntimeOperationType.STOP,
                snapshot = stateStore.snapshot()
            )
        )
        return handle
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
