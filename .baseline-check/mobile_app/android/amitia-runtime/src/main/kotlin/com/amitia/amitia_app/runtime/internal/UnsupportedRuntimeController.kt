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
import java.util.concurrent.atomic.AtomicBoolean

internal class UnsupportedRuntimeController(
    private val idGenerator: RuntimeIdGenerator = UuidRuntimeIdGenerator
) : RuntimeController {

    private val closed = AtomicBoolean(false)

    override fun snapshot(): RuntimeSnapshot = RuntimeSnapshot.initial()

    override fun subscribe(listener: RuntimeListener): RuntimeSubscription {
        listener.onRuntimeSnapshotChanged(RuntimeSnapshot.initial())
        return NoOpSubscription
    }

    override fun install(
        request: RuntimeInstallRequest,
        callback: RuntimeOperationCallback
    ): RuntimeOperationHandle = executeUnsupported(RuntimeOperationType.INSTALL, callback)

    override fun verify(
        request: RuntimeVerifyRequest,
        callback: RuntimeOperationCallback
    ): RuntimeOperationHandle = executeUnsupported(RuntimeOperationType.VERIFY, callback)

    override fun start(
        request: RuntimeStartRequest,
        callback: RuntimeOperationCallback
    ): RuntimeOperationHandle = executeUnsupported(RuntimeOperationType.START, callback)

    override fun stop(
        request: RuntimeStopRequest,
        callback: RuntimeOperationCallback
    ): RuntimeOperationHandle = executeUnsupported(RuntimeOperationType.STOP, callback)

    override fun repair(
        request: RuntimeRepairRequest,
        callback: RuntimeOperationCallback
    ): RuntimeOperationHandle = executeUnsupported(RuntimeOperationType.REPAIR, callback)

    private fun executeUnsupported(
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
        val result = RuntimeOperationResult.Failure(
            operationId = operationId,
            type = type,
            error = error,
            snapshot = RuntimeSnapshot.initial()
        )
        callback.onCompleted(result)
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

    private object NoOpSubscription : RuntimeSubscription {
        override fun cancel() {}
        override fun isCancelled(): Boolean = true
    }
}
