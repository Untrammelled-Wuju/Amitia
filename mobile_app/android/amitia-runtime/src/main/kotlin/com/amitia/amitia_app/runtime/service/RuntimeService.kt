package com.amitia.amitia_app.runtime.service

import android.app.Service
import android.content.Context
import android.content.Intent
import android.os.IBinder
import androidx.core.app.ServiceCompat
import androidx.core.content.ContextCompat
import com.amitia.amitia_app.runtime.AndroidRuntimeModule
import com.amitia.amitia_app.runtime.service.internal.DefaultRuntimeServiceEndpoint
import com.amitia.amitia_app.runtime.service.internal.RuntimeForegroundNotification
import com.amitia.amitia_app.runtime.service.internal.RuntimeForegroundNotificationResult
import com.amitia.amitia_app.runtime.proot.ProotEvent
import com.amitia.amitia_app.runtime.proot.ProotObserver
import com.amitia.amitia_app.runtime.proot.ProotSession
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicReference
import java.util.concurrent.locks.ReentrantLock
import kotlin.concurrent.withLock

class RuntimeService : Service() {
    private val lock = ReentrantLock()
    private val destroyed = AtomicBoolean(false)
    private val serviceState = AtomicReference(ServiceHostState.CREATED)
    private val notificationManager by lazy { RuntimeForegroundNotification(this) }

    private val endpoint by lazy { DefaultRuntimeServiceEndpoint { this@RuntimeService } }
    private val binder by lazy { RuntimeServiceBinder(endpoint) }

    private enum class TerminalEventKind {
        EXPECTED_STOPPED,
        UNEXPECTED_TERMINATION,
    }

    private data class ServiceSessionContext(
        val generation: Long,
        val session: ProotSession,
        var stopRequested: Boolean = false,
        var terminalEvent: TerminalEventKind? = null,
    )

    private val currentSessionContextRef: AtomicReference<ServiceSessionContext?> = AtomicReference(null)
    private val currentSessionRef: AtomicReference<ProotSession?> = AtomicReference(null)
    private val currentSessionIdRef: AtomicReference<String?> = AtomicReference(null)
    private val currentGenerationRef: AtomicReference<Long> = AtomicReference(0L)

    init {
        instanceRef.set(this)
    }

    override fun onCreate() {
        super.onCreate()
        instanceRef.set(this)
        lock.withLock {
            destroyed.set(false)
            serviceState.set(ServiceHostState.CREATED)
            currentSessionContextRef.set(null)
            currentSessionRef.set(null)
            currentSessionIdRef.set(null)
            currentGenerationRef.set(0L)
        }
    }

    override fun onBind(intent: Intent?): IBinder {
        return binder
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent == null) {
            stopSelfSafely()
            return START_NOT_STICKY
        }

        when (intent.action) {
            RuntimeServiceContract.ACTION_START_HOST -> {
                val generation = intent.getLongExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 0L)
                handleStartHost(generation)
            }
            RuntimeServiceContract.ACTION_STOP_HOST -> handleStopHost()
            else -> {
                stopSelfSafely()
                return START_NOT_STICKY
            }
        }

        return START_NOT_STICKY
    }

    private fun handleStartHost(generation: Long) {
        if (generation <= 0L) {
            serviceState.set(ServiceHostState.DESTROYED)
            endpoint.notify(
                RuntimeServiceHostEvent.UnexpectedTermination(
                    generation = generation,
                    cause = RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR
                )
            )
            stopSelfSafely()
            return
        }

        if (!serviceState.compareAndSet(ServiceHostState.CREATED, ServiceHostState.FOREGROUND)) {
            return
        }

        when (val notificationResult = notificationManager.createNotification()) {
            is RuntimeForegroundNotificationResult.Success -> {
                try {
                    ServiceCompat.startForeground(
                        this,
                        RuntimeServiceContract.NOTIFICATION_ID,
                        notificationResult.notification,
                        RuntimeServiceContract.FOREGROUND_SERVICE_TYPE
                    )
                    startProotSessionLocked(generation)
                } catch (e: Exception) {
                    serviceState.set(ServiceHostState.DESTROYED)
                    endpoint.notify(
                        RuntimeServiceHostEvent.UnexpectedTermination(
                            generation = generation,
                            cause = RuntimeServiceTerminationCause.FOREGROUND_FAILED
                        )
                    )
                    stopSelfSafely()
                }
            }
            is RuntimeForegroundNotificationResult.Failure -> {
                serviceState.set(ServiceHostState.DESTROYED)
                endpoint.notify(
                    RuntimeServiceHostEvent.UnexpectedTermination(
                        generation = generation,
                        cause = RuntimeServiceTerminationCause.NOTIFICATION_FAILED
                    )
                )
                stopSelfSafely()
            }
        }
    }

    private fun startProotSessionLocked(generation: Long) {
        currentGenerationRef.set(generation)

        val component = AndroidRuntimeModule.prootComponent
        if (component == null) {
            emitLaunchFailed(generation, RuntimeServiceTerminationCause.PROOT_COMPONENT_MISSING, "proot component is not available")
            return
        }

        val rootfsPath = AndroidRuntimeModule.prootRootfsPath
        if (rootfsPath == null) {
            emitLaunchFailed(generation, RuntimeServiceTerminationCause.ROOTFS_MISSING, "rootfs path is not available")
            return
        }

        val assembler = AndroidRuntimeModule.prootEnvironmentAssembler
        if (assembler == null) {
            emitLaunchFailed(generation, RuntimeServiceTerminationCause.ASSEMBLER_MISSING, "launch assembler is not available")
            return
        }

        val activeProgramSource = resolveActiveProgramSource(generation)
        if (activeProgramSource == null) {
            return
        }

        val request = try {
            val spec = assembler.assembleBackendLaunch(activeProgramSource)
            assembler.toProotLaunchRequest(spec, "")
        } catch (e: com.amitia.amitia_app.runtime.proot.internal.ProotEnvironmentException) {
            emitLaunchFailed(generation, RuntimeServiceTerminationCause.ENVIRONMENT_BUILD_FAILED, "environment build failed: ${e.message}")
            return
        } catch (e: java.lang.IllegalArgumentException) {
            emitLaunchFailed(generation, RuntimeServiceTerminationCause.MOUNT_CONTRACT_INVALID, "mount contract invalid: ${e.message}")
            return
        } catch (e: java.lang.IllegalStateException) {
            emitLaunchFailed(generation, RuntimeServiceTerminationCause.MOUNT_CONTRACT_INVALID, "mount contract invalid: ${e.message}")
            return
        }

        val observer = ProotObserver { event -> onProotEvent(event, generation) }
        val session = component.launch(request, observer, generation)
        currentSessionRef.set(session)
        currentSessionIdRef.set(session.sessionId)
        currentSessionContextRef.set(
            ServiceSessionContext(
                generation = generation,
                session = session,
            )
        )
    }

    private fun resolveActiveProgramSource(generation: Long): java.io.File? {
        val manager = AndroidRuntimeModule.activeRuntimeManager
        if (manager == null) {
            emitLaunchFailed(generation, RuntimeServiceTerminationCause.NO_ACTIVE_RUNTIME, "active runtime manager is not available")
            return null
        }
        return when (val result = manager.resolveActiveProgramRoot()) {
            is com.amitia.amitia_app.runtime.install.ActiveProgramRootResult.Ready ->
                result.root.hostDirectory
            is com.amitia.amitia_app.runtime.install.ActiveProgramRootResult.NoActiveRuntime -> {
                emitLaunchFailed(generation, RuntimeServiceTerminationCause.NO_ACTIVE_RUNTIME, "no active runtime")
                null
            }
            is com.amitia.amitia_app.runtime.install.ActiveProgramRootResult.Failure -> {
                emitLaunchFailed(generation, RuntimeServiceTerminationCause.ACTIVE_PROGRAM_ROOT_INVALID, "active program root failure: ${result.message}")
                null
            }
        }
    }

    private fun emitLaunchFailed(generation: Long, cause: RuntimeServiceTerminationCause, message: String) {
        endpoint.notify(
            RuntimeServiceHostEvent.LaunchFailed(
                generation = generation,
                cause = cause,
                message = message,
            )
        )
    }

    private fun onProotEvent(event: ProotEvent, generation: Long) {
        val currentGen = currentGenerationRef.get()
        if (currentGen != generation) return
        val currentSid = currentSessionIdRef.get()
        if (currentSid != null && event.sessionId != currentSid) return
        when (event) {
            is ProotEvent.Started -> {
                if (currentGenerationRef.get() == generation) {
                    endpoint.notify(
                        RuntimeServiceHostEvent.SessionReady(
                            generation = generation,
                            sessionId = event.sessionId
                        )
                    )
                }
            }
            is ProotEvent.Exited -> {
                val exit = event.exit
                if (exit.generation != currentGen) return
                if (currentSid != null && exit.sessionId != currentSid) return
                lock.withLock {
                    if (destroyed.get()) return
                    if (serviceState.get() == ServiceHostState.DESTROYED) return
                    val sessionContext = currentSessionContextRef.get()
                    if (sessionContext == null || sessionContext.generation != generation) return
                    if (sessionContext.terminalEvent != null) return
                    sessionContext.terminalEvent = if (exit.stopRequested) TerminalEventKind.EXPECTED_STOPPED else TerminalEventKind.UNEXPECTED_TERMINATION
                    endpoint.notify(
                        RuntimeServiceHostEvent.SessionExited(
                            generation = exit.generation,
                            sessionId = exit.sessionId,
                            exitCode = exit.exitCode,
                            forced = exit.stopRequested
                        )
                    )
                    if (serviceState.get() != ServiceHostState.FOREGROUND) return
                    if (exit.stopRequested) {
                        serviceState.set(ServiceHostState.DESTROYED)
                        endpoint.notify(RuntimeServiceHostEvent.ExpectedStopped(generation = exit.generation))
                    } else {
                        serviceState.set(ServiceHostState.DESTROYED)
                        endpoint.notify(
                            RuntimeServiceHostEvent.UnexpectedTermination(
                                generation = exit.generation,
                                cause = RuntimeServiceTerminationCause.SESSION_EXITED
                            )
                        )
                    }
                    currentSessionContextRef.set(null)
                    currentSessionRef.set(null)
                    currentSessionIdRef.set(null)
                    stopSelfSafely()
                }
            }
            else -> {}
        }
    }

    private fun handleStopHost() {
        val session = currentSessionRef.get()
        if (session != null && session.isAlive()) {
            session.requestStop()
            session.stop(10_000L)
        }
        val sessionContext = currentSessionContextRef.get()
        if (sessionContext != null) {
            sessionContext.stopRequested = true
        }
        stopForegroundSafely()
        stopSelfSafely()
    }

    private fun stopForegroundSafely() {
        try {
            stopForeground(STOP_FOREGROUND_REMOVE)
        } catch (_: Exception) {
        }
    }

    private fun stopSelfSafely() {
        try {
            stopSelf()
        } catch (_: Exception) {
        }
    }

    override fun onDestroy() {
        var expectedStoppedGeneration: Long? = null
        var unexpectedTerminationCause: RuntimeServiceTerminationCause? = null
        lock.withLock {
            destroyed.set(true)
            serviceState.set(ServiceHostState.DESTROYED)
            val sessionContext = currentSessionContextRef.get()
            if (sessionContext != null && sessionContext.terminalEvent == null) {
                if (sessionContext.stopRequested) {
                    sessionContext.terminalEvent = TerminalEventKind.EXPECTED_STOPPED
                    expectedStoppedGeneration = sessionContext.generation
                } else {
                    sessionContext.terminalEvent = TerminalEventKind.UNEXPECTED_TERMINATION
                    unexpectedTerminationCause = RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR
                }
            }
            currentSessionContextRef.set(null)
            currentSessionRef.set(null)
            currentSessionIdRef.set(null)
        }
        if (expectedStoppedGeneration != null) {
            endpoint.notify(RuntimeServiceHostEvent.ExpectedStopped(generation = expectedStoppedGeneration!!))
        }
        if (unexpectedTerminationCause != null) {
            endpoint.notify(
                RuntimeServiceHostEvent.UnexpectedTermination(
                    generation = currentGenerationRef.get(),
                    cause = unexpectedTerminationCause!!
                )
            )
        }
        instanceRef.compareAndSet(this, null)
        super.onDestroy()
    }

    override fun onTaskRemoved(rootIntent: Intent?) {
        super.onTaskRemoved(rootIntent)
    }

    internal fun snapshot(): RuntimeServiceSnapshot {
        return RuntimeServiceSnapshot(
            created = serviceState.get() != ServiceHostState.DESTROYED,
            foreground = serviceState.get() == ServiceHostState.FOREGROUND,
            boundClients = 0
        )
    }

    internal fun currentProotSession(): ProotSession? = currentSessionRef.get()

    private enum class ServiceHostState {
        CREATED,
        FOREGROUND,
        DESTROYED
    }

    internal companion object {
        private val instanceRef = AtomicReference<RuntimeService?>(null)

        fun currentSession(context: Context): ProotSession? {
            return instanceRef.get()?.currentProotSession()
        }

        fun currentGeneration(context: Context): Long {
            return instanceRef.get()?.currentGenerationRef?.get() ?: 0L
        }

        private const val WORKING_DIRECTORY = com.amitia.amitia_app.runtime.proot.GuestLayout.BACKEND_DIR
        private const val GUEST_SERVER_COMMAND = com.amitia.amitia_app.runtime.proot.GuestLayout.BACKEND_SERVER

        internal fun startHost(context: Context, generation: Long): RuntimeServiceResult {
            return try {
                val intent = Intent(context, RuntimeService::class.java).apply {
                    action = RuntimeServiceContract.ACTION_START_HOST
                    putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, generation)
                }
                ContextCompat.startForegroundService(context, intent)
                RuntimeServiceResult.Success
            } catch (e: Exception) {
                instanceRef.get()?.let { svc ->
                    synchronized(svc) {
                        svc.destroyed.set(true)
                        svc.serviceState.set(ServiceHostState.DESTROYED)
                        svc.currentSessionRef.set(null)
                        svc.currentSessionIdRef.set(null)
                    }
                }
                RuntimeServiceResult.Failure(
                    RuntimeServiceError(
                        code = RuntimeServiceErrorCode.SERVICE_START_FAILED,
                        message = "failed to start runtime service: ${e.message}",
                        cause = e
                    )
                )
            }
        }

        internal fun stopHost(context: Context): RuntimeServiceResult {
            val svc = instanceRef.get()
            if (svc != null) {
                svc.lock.withLock {
                    val sessionContext = svc.currentSessionContextRef.get()
                    if (sessionContext != null) {
                        sessionContext.stopRequested = true
                    }
                }
            }
            return try {
                val intent = Intent(context, RuntimeService::class.java).apply {
                    action = RuntimeServiceContract.ACTION_STOP_HOST
                }
                context.startService(intent)
                RuntimeServiceResult.Success
            } catch (e: Exception) {
                RuntimeServiceResult.Failure(
                    RuntimeServiceError(
                        code = RuntimeServiceErrorCode.SERVICE_STOP_FAILED,
                        message = "failed to stop runtime service: ${e.message}",
                        cause = e
                    )
                )
            }
        }

        fun clearInstanceRef() {
            instanceRef.set(null)
        }
    }
}

internal data class RuntimeServiceSnapshot(
    val created: Boolean,
    val foreground: Boolean,
    val boundClients: Int
)
