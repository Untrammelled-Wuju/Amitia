package com.amitia.amitia_app.runtime.service

internal interface RuntimeServiceEndpoint {
    fun snapshot(): RuntimeServiceSnapshot
    fun addListener(listener: RuntimeServiceHostListener)
    fun removeListener(listener: RuntimeServiceHostListener)
}
