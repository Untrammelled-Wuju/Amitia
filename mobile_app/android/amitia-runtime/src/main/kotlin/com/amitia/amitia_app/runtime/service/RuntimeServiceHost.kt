package com.amitia.amitia_app.runtime.service

internal interface RuntimeServiceHost {
    fun ensureStarted(): RuntimeServiceResult
    fun requestStop(): RuntimeServiceResult
    fun addListener(listener: RuntimeServiceHostListener)
    fun removeListener(listener: RuntimeServiceHostListener)
}
