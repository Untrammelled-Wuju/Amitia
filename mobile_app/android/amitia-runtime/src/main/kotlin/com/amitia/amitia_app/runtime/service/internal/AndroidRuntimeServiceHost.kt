package com.amitia.amitia_app.runtime.service.internal

import android.content.Context
import android.content.Intent
import com.amitia.amitia_app.runtime.proot.ProotSession
import com.amitia.amitia_app.runtime.service.RuntimeService
import com.amitia.amitia_app.runtime.service.RuntimeServiceContract
import com.amitia.amitia_app.runtime.service.RuntimeServiceHost
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener
import com.amitia.amitia_app.runtime.service.RuntimeServiceLifecycleSnapshot
import com.amitia.amitia_app.runtime.service.RuntimeProcessPhase
import com.amitia.amitia_app.runtime.service.RuntimeServicePhase
import com.amitia.amitia_app.runtime.service.RuntimeTerminalState
import com.amitia.amitia_app.runtime.service.RuntimeServiceResult
import com.amitia.amitia_app.runtime.service.ServiceTeardownResult
import com.amitia.amitia_app.runtime.service.RuntimeServiceTerminationCause
import java.util.concurrent.CopyOnWriteArrayList

internal class AndroidRuntimeServiceHost(
    private val appContext: Context
) : RuntimeServiceHost {

    private val context get() = appContext.applicationContext
    private val listeners = CopyOnWriteArrayList<RuntimeServiceHostListener>()
    private val serviceConnection = RuntimeServiceConnection(
        onConnected = { endpoint ->
            endpoint.addListener(internalEndpointListener)
            val snapshot = endpoint.lifecycleSnapshot()
            if (snapshot != null) {
                reconcileLifecycleSnapshot(snapshot)
            }
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

    override fun lifecycleSnapshot(): RuntimeServiceLifecycleSnapshot? {
        return serviceConnection.endpoint()?.lifecycleSnapshot()
    }

    private fun reconcileLifecycleSnapshot(snapshot: RuntimeServiceLifecycleSnapshot) {
        if (snapshot.generation <= 0) {
            return
        }
        if (snapshot.processPhase == RuntimeProcessPhase.STARTED && snapshot.terminalState == null) {
            return
        }
        if (snapshot.terminalState != null) {
            val event = when (snapshot.terminalState) {
                RuntimeTerminalState.EXPECTED_STOPPED -> RuntimeServiceHostEvent.ExpectedStopped(
                    generation = snapshot.generation,
                    result = ServiceTeardownResult.SupersededByNewStart
                )
                RuntimeTerminalState.UNEXPECTED_TERMINATION -> RuntimeServiceHostEvent.UnexpectedTermination(
                    generation = snapshot.generation,
                    cause = RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR
                )
                RuntimeTerminalState.STARTUP_FAILURE_CLEANUP -> RuntimeServiceHostEvent.StartupFailed(
                    generation = snapshot.generation,
                    cause = RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR,
                    message = "startup failure cleanup",
                    sessionId = snapshot.sessionId,
                    launchStartId = snapshot.latestStartId,
                    phase = "startup_failure"
                )
            }
            val listenerSnapshot = ArrayList(listeners)
            for (listener in listenerSnapshot) {
                try {
                    listener.onServiceHostEvent(event)
                } catch (_: Throwable) {
                }
            }
        }
    }
}
