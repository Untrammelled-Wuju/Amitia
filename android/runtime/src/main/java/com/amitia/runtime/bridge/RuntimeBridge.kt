package com.amitia.runtime.bridge

import com.amitia.runtime.api.RuntimeFacade

interface RuntimeBridge : RuntimeFacade {

    fun isAvailable(): Boolean

    fun notifyBackendReady(endpoint: String, authToken: String)

    fun notifyBackendStopped()

    fun notifyLowMemory()

    fun notifyNetworkChanged(available: Boolean)

    fun requestForegroundNotification(contentText: String)

    fun cancelForegroundNotification()

    fun reportDiagnostic(tag: String, message: String)
}
