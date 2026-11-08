package com.amitia.amitia_app.runtime.service

internal interface RuntimeServiceEndpoint {
    fun snapshot(): RuntimeServiceSnapshot
    fun currentSnapshot(): RuntimeServiceSnapshot
    fun requestStop(targetGeneration: Long): RuntimeServiceResult
    fun addListener(listener: RuntimeServiceHostListener)
    fun removeListener(listener: RuntimeServiceHostListener)
    fun lifecycleSnapshot(): RuntimeServiceLifecycleSnapshot?
    fun updateLifecycleSnapshot(snapshot: RuntimeServiceLifecycleSnapshot)
    fun notifySnapshotUpdated()
}
