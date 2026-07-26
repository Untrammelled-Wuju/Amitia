package com.amitia.runtime.bridge

import com.amitia.runtime.api.RuntimeFacade
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class RuntimeBridgeStub @Inject constructor(
    private val delegate: RuntimeFacade
) : RuntimeBridge, RuntimeFacade by delegate {

    override fun isAvailable(): Boolean = false

    override fun notifyBackendReady(endpoint: String, authToken: String) {
    }

    override fun notifyBackendStopped() {
    }

    override fun notifyLowMemory() {
    }

    override fun notifyNetworkChanged(available: Boolean) {
    }

    override fun requestForegroundNotification(contentText: String) {
    }

    override fun cancelForegroundNotification() {
    }

    override fun reportDiagnostic(tag: String, message: String) {
    }
}
