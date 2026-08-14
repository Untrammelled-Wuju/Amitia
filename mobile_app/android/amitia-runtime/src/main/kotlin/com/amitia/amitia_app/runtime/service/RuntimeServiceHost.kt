package com.amitia.amitia_app.runtime.service

import com.amitia.amitia_app.runtime.proot.ProotSession

interface RuntimeServiceHost {
    fun ensureStarted(generation: Long): RuntimeServiceResult
    fun requestStop(targetGeneration: Long): RuntimeServiceResult
    fun addListener(listener: RuntimeServiceHostListener)
    fun removeListener(listener: RuntimeServiceHostListener)
    fun currentSession(): ProotSession?
    fun currentGeneration(): Long
}
