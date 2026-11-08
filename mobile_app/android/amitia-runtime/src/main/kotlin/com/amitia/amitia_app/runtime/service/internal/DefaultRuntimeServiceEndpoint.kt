package com.amitia.amitia_app.runtime.service.internal

import com.amitia.amitia_app.runtime.service.RuntimeService
import com.amitia.amitia_app.runtime.service.RuntimeServiceEndpoint
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostEvent
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener
import com.amitia.amitia_app.runtime.service.RuntimeServiceLifecycleSnapshot
import com.amitia.amitia_app.runtime.service.RuntimeProcessPhase
import com.amitia.amitia_app.runtime.service.RuntimeServiceResult
import com.amitia.amitia_app.runtime.service.RuntimeServiceSnapshot
import com.amitia.amitia_app.runtime.service.RuntimeServiceTerminationCause
import com.amitia.amitia_app.runtime.service.RuntimeTerminalState
import com.amitia.amitia_app.runtime.service.ServiceTeardownResult
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.atomic.AtomicReference

internal class DefaultRuntimeServiceEndpoint(
    private val serviceProvider: () -> RuntimeService?
) : RuntimeServiceEndpoint {

    private val listeners = CopyOnWriteArrayList<RuntimeServiceHostListener>()
    private val lifecycleSnapshotRef = AtomicReference<RuntimeServiceLifecycleSnapshot?>(null)

    override fun snapshot(): RuntimeServiceSnapshot {
        val service = serviceProvider()
        return service?.snapshot() ?: RuntimeServiceSnapshot(
            created = false, foreground = false, boundClients = 0
        )
    }

    override fun currentSnapshot(): RuntimeServiceSnapshot = snapshot()

    override fun requestStop(targetGeneration: Long): RuntimeServiceResult {
        val service = serviceProvider() ?: return RuntimeServiceResult.Failure(
            com.amitia.amitia_app.runtime.service.RuntimeServiceError(
                code = com.amitia.amitia_app.runtime.service.RuntimeServiceErrorCode.SERVICE_STOP_FAILED,
                message = "bound runtime service is unavailable",
            )
        )
        return service.requestStopFromEndpoint(targetGeneration)
    }

    override fun addListener(listener: RuntimeServiceHostListener) {
        if (listeners.addIfAbsent(listener)) {
            replayLifecycleSnapshot(listener)
        }
    }

    override fun removeListener(listener: RuntimeServiceHostListener) {
        listeners.remove(listener)
    }

    override fun lifecycleSnapshot(): RuntimeServiceLifecycleSnapshot? = lifecycleSnapshotRef.get()

    override fun updateLifecycleSnapshot(snapshot: RuntimeServiceLifecycleSnapshot) {
        lifecycleSnapshotRef.set(snapshot)
    }

    override fun notifySnapshotUpdated() {
        val snapshot = lifecycleSnapshotRef.get() ?: return
        val event = RuntimeServiceHostEvent.SnapshotUpdated(snapshot)
        val listenerSnapshot = ArrayList(listeners)
        for (listener in listenerSnapshot) {
            try {
                listener.onServiceHostEvent(event)
            } catch (originalError49: Throwable) {
              originalError49.printStackTrace()

            }
        }
    }

    fun notify(event: RuntimeServiceHostEvent) {
        val snapshot = ArrayList(listeners)
        for (listener in snapshot) {
            try {
                listener.onServiceHostEvent(event)
            } catch (originalError59: Throwable) {
              originalError59.printStackTrace()

            }
        }
    }

    private fun replayLifecycleSnapshot(listener: RuntimeServiceHostListener) {
        val snapshot = lifecycleSnapshotRef.get() ?: return
        val event = when (snapshot.terminalState) {
            RuntimeTerminalState.EXPECTED_STOPPED -> RuntimeServiceHostEvent.ExpectedStopped(
                generation = snapshot.generation,
                result = snapshot.teardownResult ?: ServiceTeardownResult.SupersededByNewStart,
            )
            RuntimeTerminalState.UNEXPECTED_TERMINATION -> RuntimeServiceHostEvent.UnexpectedTermination(
                generation = snapshot.generation,
                cause = snapshot.terminationCause ?: RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR,
                message = snapshot.terminationMessage,
            )
            RuntimeTerminalState.STARTUP_FAILURE_CLEANUP -> RuntimeServiceHostEvent.StartupFailed(
                generation = snapshot.generation,
                cause = snapshot.terminationCause ?: RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR,
                message = snapshot.startupFailureMessage ?: snapshot.terminationMessage ?: "runtime startup failed",
                sessionId = snapshot.sessionId,
                launchStartId = snapshot.latestStartId,
                phase = snapshot.startupFailurePhase ?: "startup_failure",
            )
            null -> if (snapshot.processPhase == RuntimeProcessPhase.STARTED) {
                RuntimeServiceHostEvent.SessionReady(
                    generation = snapshot.generation,
                    sessionId = snapshot.sessionId ?: "",
                )
            } else {
                null
            }
        }
        if (event == null) return
        try {
            listener.onServiceHostEvent(event)
        } catch (originalError93: Throwable) {
            originalError93.printStackTrace()
        }
    }
}
