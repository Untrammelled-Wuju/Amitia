package com.amitia.amitia_app.runtime.service

import com.amitia.amitia_app.runtime.proot.ProotSession

interface RuntimeServiceHost {
    fun ensureStarted(generation: Long, profile: String): RuntimeServiceResult
    fun requestStop(targetGeneration: Long): RuntimeServiceResult

    /**
     * Legacy cleanup entry-point kept for test doubles and alternate hosts.
     * Production callers should use the detailed overload so the original
     * startup failure survives service teardown.
     */
    fun requestTeardownAfterStartupFailure()

    fun requestTeardownAfterStartupFailure(
        generation: Long,
        cause: RuntimeServiceTerminationCause,
        message: String,
        phase: String,
    ): RuntimeServiceResult {
        requestTeardownAfterStartupFailure()
        return RuntimeServiceResult.Success
    }

    fun addListener(listener: RuntimeServiceHostListener)
    fun removeListener(listener: RuntimeServiceHostListener)
    fun currentSession(): ProotSession?
    fun currentGeneration(): Long
    fun lifecycleSnapshot(): RuntimeServiceLifecycleSnapshot? = null
}
