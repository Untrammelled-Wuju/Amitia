package com.amitia.amitia_app.runtime.api

interface RuntimeOperationHandle {
    val operationId: String
    val type: RuntimeOperationType
    fun cancel(): Boolean
    fun isCancelled(): Boolean
    fun isCompleted(): Boolean
}
