package com.amitia.amitia_app.runtime.service.internal

import android.content.Context
import android.content.Intent
import com.amitia.amitia_app.runtime.proot.ProotSession
import com.amitia.amitia_app.runtime.service.RuntimeService
import com.amitia.amitia_app.runtime.service.RuntimeServiceContract
import com.amitia.amitia_app.runtime.service.RuntimeServiceHost
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener
import com.amitia.amitia_app.runtime.service.RuntimeServiceResult
import java.util.concurrent.CopyOnWriteArrayList

internal class AndroidRuntimeServiceHost(
    private val appContext: Context
) : RuntimeServiceHost {

    private val context get() = appContext.applicationContext
    private val listeners = CopyOnWriteArrayList<RuntimeServiceHostListener>()
    private val serviceConnection = RuntimeServiceConnection(
        onConnected = { endpoint ->
            endpoint.addListener(internalEndpointListener)
        },
        onDisconnected = { }
    )
    private var bound = false

    private val internalEndpointListener = RuntimeServiceHostListener { event ->
        val snapshot = ArrayList(listeners)
        for (listener in snapshot) {
            try {
                listener.onServiceHostEvent(event)
            } catch (_: Throwable) {
            }
        }
    }

    override fun ensureStarted(generation: Long, profile: String): RuntimeServiceResult {
        val result = RuntimeService.startHost(context, generation, profile)
        if (result is RuntimeServiceResult.Success && !bound) {
            bound = RuntimeServiceConnection.bind(context, serviceConnection)
        }
        return result
    }

    override fun requestStop(targetGeneration: Long): RuntimeServiceResult {
        return RuntimeService.stopHost(context, targetGeneration)
    }

    override fun requestTeardownAfterStartupFailure() {
        try {
            val intent = Intent(context, RuntimeService::class.java).apply {
                action = RuntimeServiceContract.ACTION_TEARDOWN_AFTER_STARTUP_FAILURE
            }
            context.startService(intent)
        } catch (_: Throwable) {
        }
    }

    override fun addListener(listener: RuntimeServiceHostListener) {
        listeners.add(listener)
    }

    override fun removeListener(listener: RuntimeServiceHostListener) {
        listeners.remove(listener)
    }

    override fun currentSession(): ProotSession? {
        return RuntimeService.currentSession(context)
    }

    override fun currentGeneration(): Long {
        return RuntimeService.currentGeneration(context)
    }
}
