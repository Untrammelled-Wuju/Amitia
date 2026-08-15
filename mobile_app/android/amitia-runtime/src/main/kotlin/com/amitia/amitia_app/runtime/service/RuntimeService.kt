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
import java.util.concurrent.atomic.AtomicLong
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
        STARTUP_FAILURE_CLEANUP,
    }

    private enum class ServiceTeardownReason {
        EXPECTED_STOP,
        UNEXPECTED_TERMINATION,
        STARTUP_FAILURE,
    }

    private data class StartupFailureCleanupContext(
        val generation: Long,
        val startId: Int,
        val cause: RuntimeServiceTerminationCause,
        val message: String,
        var processConfirmedDead: Boolean = false,
    )

    private data class ServiceSessionContext(
        val generation: Long,
        val session: ProotSession,
        val startId: Int,
        var stopRequested: Boolean = false,
        var terminalEvent: TerminalEventKind? = null,
    )

    private val currentSessionContextRef: AtomicReference<ServiceSessionContext?> = AtomicReference(null)
    private val currentSessionRef: AtomicReference<ProotSession?> = AtomicReference(null)
    private val currentSessionIdRef: AtomicReference<String?> = AtomicReference(null)
    private val currentGenerationRef: AtomicReference<Long> = AtomicReference(0L)
    private val currentIntent: AtomicReference<Intent?> = AtomicReference(null)
    private val currentStartIdRef = AtomicReference(0)
    private val startupFailureCleanupContextRef = AtomicReference<StartupFailureCleanupContext?>(null)
    private val serviceTeardownCompleted = AtomicBoolean(false)

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
            startupFailureCleanupContextRef.set(null)
            serviceTeardownCompleted.set(false)
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

        currentIntent.set(intent)
        currentStartIdRef.set(startId)

        when (intent.action) {
            RuntimeServiceContract.ACTION_START_HOST -> {
                val generation = intent.getLongExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 0L)
                val profile = intent.getStringExtra(RuntimeServiceContract.EXTRA_RUNTIME_PROFILE) ?: "local"
                handleStartHost(generation, profile, startId)
            }
            RuntimeServiceContract.ACTION_STOP_HOST -> handleStopHost()
            RuntimeServiceContract.ACTION_TEARDOWN_AFTER_STARTUP_FAILURE -> {
                val generation = currentGenerationRef.get()
                teardownAfterStartupFailure(generation, startId, RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR, "startup failure cleanup requested by controller")
            }
            else -> {
                stopSelfSafely()
                return START_NOT_STICKY
            }
        }

        return START_NOT_STICKY
    }

    private fun handleStartHost(generation: Long, profile: String, startId: Int) {
        if (generation <= 0L) {
            teardownAfterStartupFailure(generation, startId, RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR, "invalid generation: $generation")
            return
        }

        if (!serviceState.compareAndSet(ServiceHostState.CREATED, ServiceHostState.FOREGROUND)) {
            teardownAfterStartupFailure(generation, startId, RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR, "service state is not CREATED, current state: ${serviceState.get()}")
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
                    startProotSessionLocked(generation, profile, startId)
                } catch (e: Exception) {
                    teardownAfterStartupFailure(generation, startId, RuntimeServiceTerminationCause.FOREGROUND_FAILED, "foreground start failed: ${e.message}")
                }
            }
            is RuntimeForegroundNotificationResult.Failure -> {
                teardownAfterStartupFailure(generation, startId, RuntimeServiceTerminationCause.NOTIFICATION_FAILED, "notification creation failed")
            }
        }
    }

    private fun startProotSessionLocked(generation: Long, profile: String, startId: Int) {
        currentGenerationRef.set(generation)

        val component = AndroidRuntimeModule.prootComponent
        if (component == null) {
            teardownAfterStartupFailure(generation, startId, RuntimeServiceTerminationCause.PROOT_COMPONENT_MISSING, "proot component is not available")
            return
        }

        val rootfsPath = AndroidRuntimeModule.prootRootfsPath
        if (rootfsPath == null) {
            teardownAfterStartupFailure(generation, startId, RuntimeServiceTerminationCause.ROOTFS_MISSING, "rootfs path is not available")
            return
        }

        val assembler = AndroidRuntimeModule.prootEnvironmentAssembler
        if (assembler == null) {
            teardownAfterStartupFailure(generation, startId, RuntimeServiceTerminationCause.ASSEMBLER_MISSING, "launch assembler is not available")
            return
        }

        val activeProgramSource = resolveActiveProgramSource(generation, startId)
        if (activeProgramSource == null) {
            return
        }

        val request = try {
            val spec = assembler.assembleBackendLaunch(activeProgramSource, profile)
            assembler.toProotLaunchRequest(spec, "")
        } catch (e: com.amitia.amitia_app.runtime.proot.internal.ProotEnvironmentException) {
            teardownAfterStartupFailure(generation, startId, RuntimeServiceTerminationCause.ENVIRONMENT_BUILD_FAILED, "environment build failed: ${e.message}")
            return
        } catch (e: java.lang.IllegalArgumentException) {
            teardownAfterStartupFailure(generation, startId, RuntimeServiceTerminationCause.MOUNT_CONTRACT_INVALID, "mount contract invalid: ${e.message}")
            return
        } catch (e: java.lang.IllegalStateException) {
            teardownAfterStartupFailure(generation, startId, RuntimeServiceTerminationCause.MOUNT_CONTRACT_INVALID, "mount contract invalid: ${e.message}")
            return
        }

        val observer = ProotObserver { event -> onProotEvent(event, generation) }
        val session = try {
            component.launch(request, observer, generation)
        } catch (e: Exception) {
            teardownAfterStartupFailure(generation, startId, RuntimeServiceTerminationCause.ENVIRONMENT_BUILD_FAILED, "proot launch exception: ${e.message}")
            return
        }
        currentSessionRef.set(session)
        currentSessionIdRef.set(session.sessionId)
        currentSessionContextRef.set(
            ServiceSessionContext(
                generation = generation,
                session = session,
                startId = startId,
            )
        )
    }

    private fun resolveActiveProgramSource(generation: Long, startId: Int): java.io.File? {
        val manager = AndroidRuntimeModule.activeRuntimeManager
        if (manager == null) {
            teardownAfterStartupFailure(generation, startId, RuntimeServiceTerminationCause.NO_ACTIVE_RUNTIME, "active runtime manager is not available")
            return null
        }
        return when (val result = manager.resolveActiveProgramRoot()) {
            is com.amitia.amitia_app.runtime.install.ActiveProgramRootResult.Ready ->
                result.root.hostDirectory
            is com.amitia.amitia_app.runtime.install.ActiveProgramRootResult.NoActiveRuntime -> {
                teardownAfterStartupFailure(generation, startId, RuntimeServiceTerminationCause.NO_ACTIVE_RUNTIME, "no active runtime")
                null
            }
            is com.amitia.amitia_app.runtime.install.ActiveProgramRootResult.Failure -> {
                teardownAfterStartupFailure(generation, startId, RuntimeServiceTerminationCause.ACTIVE_PROGRAM_ROOT_INVALID, "active program root failure: ${result.message}")
                null
            }
        }
    }

    private fun teardownAfterStartupFailure(
        generation: Long,
        startId: Int,
        cause: RuntimeServiceTerminationCause,
        message: String,
    ) {
        val cleanupContext = StartupFailureCleanupContext(
            generation = generation,
            startId = startId,
            cause = cause,
            message = message,
        )
        if (!startupFailureCleanupContextRef.compareAndSet(null, cleanupContext)) {
            return
        }

        val sessionContext = currentSessionContextRef.get()
        if (sessionContext == null) {
            performStartupFailureCleanup(cleanupContext)
            return
        }

        val session = currentSessionRef.get()
        if (session != null && session.isAlive()) {
            session.requestStop()
            session.stop(GRACEFUL_SHUTDOWN_TIMEOUT_MS)
        }
    }

    private fun onProotEvent(event: ProotEvent, generation: Long) {
        val currentGen = currentGenerationRef.get()
        if (currentGen != generation) return
        val currentSid = currentSessionIdRef.get()
        if (currentSid != null && event.sessionId != currentSid) return
        when (event) {
            is ProotEvent.Started -> {
                if (startupFailureCleanupContextRef.get() != null) return
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
                handleSessionExited(exit)
            }
            else -> {}
        }
    }

    private fun handleSessionExited(exit: com.amitia.amitia_app.runtime.proot.ProotExit) {
        lock.withLock {
            if (destroyed.get()) return
            if (serviceState.get() == ServiceHostState.DESTROYED) return
            val sessionContext = currentSessionContextRef.get()
            if (sessionContext == null || sessionContext.generation != exit.generation) return

            val cleanupContext = startupFailureCleanupContextRef.get()
            val isStartupFailureCleanup = cleanupContext != null && cleanupContext.generation == exit.generation

            if (isStartupFailureCleanup) {
                cleanupContext!!.processConfirmedDead = true
                sessionContext.terminalEvent = TerminalEventKind.STARTUP_FAILURE_CLEANUP
                performStartupFailureCleanup(cleanupContext)
                return
            }

            if (sessionContext.terminalEvent != null) return

            sessionContext.terminalEvent = when {
                exit.stopRequested -> TerminalEventKind.EXPECTED_STOPPED
                else -> TerminalEventKind.UNEXPECTED_TERMINATION
            }

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
                teardownServiceForTerminalSession(
                    generation = exit.generation,
                    sessionId = exit.sessionId,
                    startId = sessionContext.startId,
                    reason = ServiceTeardownReason.EXPECTED_STOP,
                )
                endpoint.notify(RuntimeServiceHostEvent.ExpectedStopped(generation = exit.generation))
            } else {
                serviceState.set(ServiceHostState.DESTROYED)
                teardownServiceForTerminalSession(
                    generation = exit.generation,
                    sessionId = exit.sessionId,
                    startId = sessionContext.startId,
                    reason = ServiceTeardownReason.UNEXPECTED_TERMINATION,
                )
                endpoint.notify(
                    RuntimeServiceHostEvent.UnexpectedTermination(
                        generation = exit.generation,
                        cause = RuntimeServiceTerminationCause.SESSION_EXITED
                    )
                )
            }
            clearSessionState()
        }
    }

    private fun teardownServiceForTerminalSession(
        generation: Long,
        sessionId: String?,
        startId: Int,
        reason: ServiceTeardownReason,
    ) {
        if (!serviceTeardownCompleted.compareAndSet(false, true)) return
        try {
            stopForeground(STOP_FOREGROUND_REMOVE)
        } catch (_: Exception) {
        }
        try {
            stopSelfResult(startId)
        } catch (_: Exception) {
            try {
                stopSelf()
            } catch (_: Exception) {
            }
        }
    }

    private fun performStartupFailureCleanup(cleanupContext: StartupFailureCleanupContext) {
        serviceState.set(ServiceHostState.CREATED)
        clearSessionState()
        startupFailureCleanupContextRef.set(null)
        serviceTeardownCompleted.set(true)
        try {
            stopForeground(STOP_FOREGROUND_REMOVE)
        } catch (_: Exception) {
        }
        try {
            stopSelfResult(cleanupContext.startId)
        } catch (_: Exception) {
            try {
                stopSelf()
            } catch (_: Exception) {
            }
        }
        endpoint.notify(
            RuntimeServiceHostEvent.LaunchFailed(
                generation = cleanupContext.generation,
                cause = cleanupContext.cause,
                message = cleanupContext.message,
            )
        )
    }

    private fun clearSessionState() {
        currentSessionContextRef.set(null)
        currentSessionRef.set(null)
        currentSessionIdRef.set(null)
        currentGenerationRef.set(0L)
    }

    private fun handleStopHost() {
        val intent = currentIntent.get() ?: return
        val targetGeneration = intent.getLongExtra(RuntimeServiceContract.EXTRA_TARGET_GENERATION, Long.MIN_VALUE)
        if (targetGeneration == Long.MIN_VALUE || targetGeneration <= 0L) {
            return
        }
        val sessionContext = currentSessionContextRef.get()
        if (sessionContext == null) {
            return
        }
        if (sessionContext.generation != targetGeneration) {
            return
        }
        sessionContext.stopRequested = true
        val session = currentSessionRef.get()
        if (session != null && session.isAlive()) {
            session.requestStop()
            session.stop(GRACEFUL_SHUTDOWN_TIMEOUT_MS)
        }
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
        var unexpectedTerminationGeneration: Long? = null
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
                    unexpectedTerminationGeneration = sessionContext.generation
                }
            }
            clearSessionState()
            startupFailureCleanupContextRef.set(null)
        }
        if (expectedStoppedGeneration != null) {
            endpoint.notify(RuntimeServiceHostEvent.ExpectedStopped(generation = expectedStoppedGeneration!!))
        }
        if (unexpectedTerminationCause != null && unexpectedTerminationGeneration != null) {
            endpoint.notify(
                RuntimeServiceHostEvent.UnexpectedTermination(
                    generation = unexpectedTerminationGeneration!!,
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

        const val GRACEFUL_SHUTDOWN_TIMEOUT_MS = 5000L
        const val FORCE_SHUTDOWN_TIMEOUT_MS = 3000L

        fun currentSession(context: Context): ProotSession? {
            return instanceRef.get()?.currentProotSession()
        }

        fun currentGeneration(context: Context): Long {
            return instanceRef.get()?.currentGenerationRef?.get() ?: 0L
        }

        private const val WORKING_DIRECTORY = com.amitia.amitia_app.runtime.proot.GuestLayout.BACKEND_DIR
        private const val GUEST_SERVER_COMMAND = com.amitia.amitia_app.runtime.proot.GuestLayout.BACKEND_SERVER

        internal fun startHost(context: Context, generation: Long, profile: String): RuntimeServiceResult {
            return try {
                val intent = Intent(context, RuntimeService::class.java).apply {
                    action = RuntimeServiceContract.ACTION_START_HOST
                    putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, generation)
                    putExtra(RuntimeServiceContract.EXTRA_RUNTIME_PROFILE, profile)
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

        internal fun stopHost(context: Context, targetGeneration: Long): RuntimeServiceResult {
            return try {
                val intent = Intent(context, RuntimeService::class.java).apply {
                    action = RuntimeServiceContract.ACTION_STOP_HOST
                    putExtra(RuntimeServiceContract.EXTRA_TARGET_GENERATION, targetGeneration)
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
