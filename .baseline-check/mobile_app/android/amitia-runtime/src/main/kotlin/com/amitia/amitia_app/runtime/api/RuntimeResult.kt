package com.amitia.amitia_app.runtime.api

sealed class RuntimeOperationResult {
    abstract val operationId: String
    abstract val type: RuntimeOperationType
    abstract val snapshot: RuntimeSnapshot

    data class Success(
        override val operationId: String,
        override val type: RuntimeOperationType,
        override val snapshot: RuntimeSnapshot
    ) : RuntimeOperationResult()

    data class Failure(
        override val operationId: String,
        override val type: RuntimeOperationType,
        val error: RuntimeError,
        override val snapshot: RuntimeSnapshot
    ) : RuntimeOperationResult()

    data class Cancelled(
        override val operationId: String,
        override val type: RuntimeOperationType,
        override val snapshot: RuntimeSnapshot
    ) : RuntimeOperationResult()
}
