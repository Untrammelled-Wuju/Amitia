package com.amitia.amitia_app.runtime.api

fun interface RuntimeListener {
    fun onRuntimeSnapshotChanged(snapshot: RuntimeSnapshot)
}

interface RuntimeSubscription {
    fun cancel()
    fun isCancelled(): Boolean
}

fun interface RuntimeOperationCallback {
    fun onCompleted(result: RuntimeOperationResult)
}
