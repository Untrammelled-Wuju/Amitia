package com.amitia.amitia_app.runtime.service.internal

import android.content.Context
import android.content.Intent
import com.amitia.amitia_app.runtime.proot.ProotSession
import com.amitia.amitia_app.runtime.service.RuntimeProcessPhase
import com.amitia.amitia_app.runtime.service.RuntimeService
import com.amitia.amitia_app.runtime.service.RuntimeServiceContract
import com.amitia.amitia_app.runtime.service.RuntimeServiceError
import com.amitia.amitia_app.runtime.service.RuntimeServiceErrorCode
import com.amitia.amitia_app.runtime.service.RuntimeServiceHost
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostEvent
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener
import com.amitia.amitia_app.runtime.service.RuntimeServiceLifecycleSnapshot
import com.amitia.amitia_app.runtime.service.RuntimeServiceResult
import com.amitia.amitia_app.runtime.service.RuntimeServiceTerminationCause
import com.amitia.amitia_app.runtime.service.RuntimeTerminalState
import com.amitia.amitia_app.runtime.service.ServiceTeardownResult
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicLong

internal class AndroidRuntimeServiceHost(
    private val appContext: Context
) : RuntimeServiceHost {

    private val context get() = appContext.applicationContext
    private val listeners = CopyOnWriteArrayList<RuntimeServiceHostListener>()
    private val bound = AtomicBoolean(false)
    private val terminalGeneration = AtomicLong(0L)
    private val serviceConnection = RuntimeServiceConnection(
        onConnected = { endpoint ->
            bound.set(true)
            endpoint.lifecycleSnapshot()?.let(::reconcileLifecycleSnapshot)
        },
        onDisconnected = {
            bound.set(false)
        }
    )

    private val internalEndpointListener = RuntimeServiceHostListener { event ->
        val terminal = event is RuntimeServiceHostEvent.ExpectedStopped ||
            event is RuntimeServiceHostEvent.UnexpectedTermination ||
            event is RuntimeServiceHostEvent.StartupFailed
        if (terminal) {
            val generation = when (event) {
                is RuntimeServiceHostEvent.ExpectedStopped -> event.generation
                is RuntimeServiceHostEvent.UnexpectedTermination -> event.generation
                is RuntimeServiceHostEvent.StartupFailed -> event.generation
                else -> 0L
            }
            terminalGeneration.accumulateAndGet(generation) { current, candidate -> maxOf(current, candidate) }
        }
        dispatch(event)
        if (terminal) releaseBinding()
    }

    init {
        RuntimeService.addProcessListener(internalEndpointListener)
    }

    override fun ensureStarted(generation: Long, profile: String): RuntimeServiceResult {
        val result = RuntimeService.startHost(context, generation, profile)
        if (result !is RuntimeServiceResult.Success) return result

        if (bound.get() && serviceConnection.endpoint() != null) {
            serviceConnection.endpoint()?.lifecycleSnapshot()?.let(::reconcileLifecycleSnapshot)
            return result
        }

        val bindAccepted = RuntimeServiceConnection.bind(context, serviceConnection)
        if (!bindAccepted) {
            bound.set(false)
            requestTeardownAfterStartupFailure(
                generation = generation,
                cause = RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR,
                message = "runtime service started but the app could not bind to its control endpoint",
                phase = "service_bind",
            )
            return RuntimeServiceResult.Failure(
                RuntimeServiceError(
                    code = RuntimeServiceErrorCode.SERVICE_BIND_FAILED,
                    message = "runtime service bind failed for generation $generation",
                )
            )
        }
        bound.set(true)
        if (terminalGeneration.get() >= generation) {
            releaseBinding()
            return RuntimeServiceResult.Failure(
                RuntimeServiceError(
                    code = RuntimeServiceErrorCode.SERVICE_START_FAILED,
                    message = "runtime generation $generation terminated while the service binding was being established",
                )
            )
        }
        return result
    }

    override fun requestStop(targetGeneration: Long): RuntimeServiceResult {
        return RuntimeService.stopHost(context, targetGeneration)
    }

    override fun requestTeardownAfterStartupFailure() {
        val generation = currentGeneration()
        requestTeardownAfterStartupFailure(
            generation = generation,
            cause = RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR,
            message = "runtime startup failed",
            phase = "controller_requested_cleanup",
        )
    }

    override fun requestTeardownAfterStartupFailure(
        generation: Long,
        cause: RuntimeServiceTerminationCause,
        message: String,
        phase: String,
    ): RuntimeServiceResult {
        return try {
            val intent = Intent(context, RuntimeService::class.java).apply {
                action = RuntimeServiceContract.ACTION_TEARDOWN_AFTER_STARTUP_FAILURE
                putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, generation)
                putExtra(RuntimeServiceContract.EXTRA_FAILURE_CAUSE, cause.name)
                putExtra(RuntimeServiceContract.EXTRA_FAILURE_MESSAGE, message)
                putExtra(RuntimeServiceContract.EXTRA_FAILURE_PHASE, phase)
            }
            context.startService(intent)
            RuntimeServiceResult.Success
        } catch (e: Exception) {
            RuntimeServiceResult.Failure(
                RuntimeServiceError(
                    code = RuntimeServiceErrorCode.SERVICE_STOP_FAILED,
                    message = "failed to request startup-failure teardown: ${e.message ?: e.javaClass.simpleName}",
                    cause = e,
                )
            )
        }
    }

    override fun addListener(listener: RuntimeServiceHostListener) {
        listeners.add(listener)
    }

    override fun removeListener(listener: RuntimeServiceHostListener) {
        listeners.remove(listener)
    }

    override fun currentSession(): ProotSession? = RuntimeService.currentSession(context)

    override fun currentGeneration(): Long = RuntimeService.currentGeneration(context)

    override fun lifecycleSnapshot(): RuntimeServiceLifecycleSnapshot? =
        serviceConnection.endpoint()?.lifecycleSnapshot()

    private fun reconcileLifecycleSnapshot(snapshot: RuntimeServiceLifecycleSnapshot) {
        if (snapshot.generation <= 0) return

        if (snapshot.processPhase == RuntimeProcessPhase.STARTED && snapshot.terminalState == null) {
            dispatch(
                RuntimeServiceHostEvent.SessionReady(
                    generation = snapshot.generation,
                    sessionId = snapshot.sessionId ?: "",
                )
            )
            return
        }

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
            null -> null
        }

        if (event != null) {
            dispatch(event)
            releaseBinding()
        }
    }

    private fun dispatch(event: RuntimeServiceHostEvent) {
        for (listener in ArrayList(listeners)) {
            try {
                listener.onServiceHostEvent(event)
            } catch (_: Throwable) {
            }
        }
    }

    private fun releaseBinding() {
        if (bound.compareAndSet(true, false)) {
            RuntimeServiceConnection.unbind(context, serviceConnection)
        } else {
            serviceConnection.clearEndpoint()
        }
    }
}
