package com.amitia.amitia_app.runtime.service

import com.amitia.amitia_app.runtime.proot.ProotSession

interface RuntimeServiceHost {
    fun ensureStarted(generation: Long, profile: String): RuntimeServiceResult
    fun requestStop(targetGeneration: Long): RuntimeServiceResult
    fun requestTeardownAfterStartupFailure()
    fun addListener(listener: RuntimeServiceHostListener)
    fun removeListener(listener: RuntimeServiceHostListener)
    fun currentSession(): ProotSession?
    fun currentGeneration(): Long
}
