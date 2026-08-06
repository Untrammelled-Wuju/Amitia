package com.amitia.amitia_app.runtime.api

interface RuntimeController {
    fun snapshot(): RuntimeSnapshot
    fun subscribe(listener: RuntimeListener): RuntimeSubscription
    fun install(
        request: RuntimeInstallRequest,
        callback: RuntimeOperationCallback
    ): RuntimeOperationHandle
    fun verify(
        request: RuntimeVerifyRequest,
        callback: RuntimeOperationCallback
    ): RuntimeOperationHandle
    fun start(
        request: RuntimeStartRequest,
        callback: RuntimeOperationCallback
    ): RuntimeOperationHandle
    fun stop(
        request: RuntimeStopRequest,
        callback: RuntimeOperationCallback
    ): RuntimeOperationHandle
    fun repair(
        request: RuntimeRepairRequest,
        callback: RuntimeOperationCallback
    ): RuntimeOperationHandle
}
