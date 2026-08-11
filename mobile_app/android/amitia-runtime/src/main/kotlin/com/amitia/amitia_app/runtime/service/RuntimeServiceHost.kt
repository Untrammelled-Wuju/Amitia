package com.amitia.amitia_app.runtime.service

import com.amitia.amitia_app.runtime.proot.ProotSession

internal interface RuntimeServiceHost {
    fun ensureStarted(): RuntimeServiceResult
    fun requestStop(): RuntimeServiceResult
    fun addListener(listener: RuntimeServiceHostListener)
    fun removeListener(listener: RuntimeServiceHostListener)
    fun currentSession(): ProotSession?
    fun currentGeneration(): Long
}
