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

    private enum class ServiceTeardownReason {
        EXPECTED_STOP,
        UNEXPECTED_TERMINATION,
        STARTUP_FAILURE,
    }

    private enum class ServicePhase {
        CREATED,
        FOREGROUND,
        DESTROYED,
        UNOBSERVABLE,
    }

    private enum class ProcessPhase {
        CREATED,
        STARTED,
        READY,
        EXITING,
        EXITED,
        UNKNOWN,
    }

    private enum class StartupPhase {
        NOT_STARTED,
        DETECTING,
        READY,
        FAILED,
    }

    private data class StartupFailureCleanupContext(
        val generation: Long,
        val sessionId: String?,
        val launchStartId: Int,
        val originalFailure: RuntimeServiceTerminationCause,
        val phase: String,
        var processConfirmedDead: Boolean = false,
        var noProcessCreated: Boolean = false,
        var cleanupPhase: StartupFailureCleanupPhase = StartupFailureCleanupPhase.FAILURE_DETECTED,
    )

    private data class ServiceSessionContext(
        val generation: Long,
        val session: ProotSession,
        val launchStartId: Int,
        var latestStartId: Int,
        var stopStartId: Int? = null,
        var stopRequested: Boolean = false,
        var terminalEvent: TerminalEventKind? = null,
        var teardownState: TeardownState = TeardownState.NOT_STARTED,
        var servicePhase: ServicePhase = ServicePhase.CREATED,
        var processPhase: ProcessPhase = ProcessPhase.CREATED,
        var startupPhase: StartupPhase = StartupPhase.NOT_STARTED,
    )

    private enum class TerminalEventKind {
        EXPECTED_STOPPED,
        UNEXPECTED_TERMINATION,
        STARTUP_FAILURE_CLEANUP,
    }

    private fun toRuntimeTerminalState(kind: TerminalEventKind?): RuntimeTerminalState? = when (kind) {
        TerminalEventKind.EXPECTED_STOPPED -> RuntimeTerminalState.EXPECTED_STOPPED
        TerminalEventKind.UNEXPECTED_TERMINATION -> RuntimeTerminalState.UNEXPECTED_TERMINATION
        TerminalEventKind.STARTUP_FAILURE_CLEANUP -> RuntimeTerminalState.STARTUP_FAILURE_CLEANUP
        null -> null
    }

    private enum class StartupFailureCleanupPhase {
        FAILURE_DETECTED,
        STOP_REQUESTED,
        WAITING_FOR_PROCESS_EXIT,
        PROCESS_EXIT_CONFIRMED,
        SERVICE_TEARDOWN,
        COMPLETE,
    }

    private enum class TeardownState {
        NOT_STARTED,
        IN_PROGRESS,
        COMPLETE,
        SUPERSEDED,
        FAILED,
    }

    private fun toRuntimeServicePhase(phase: ServicePhase): RuntimeServicePhase = when (phase) {
        ServicePhase.CREATED -> RuntimeServicePhase.CREATED
        ServicePhase.FOREGROUND -> RuntimeServicePhase.FOREGROUND
        ServicePhase.DESTROYED -> RuntimeServicePhase.DESTROYED
        ServicePhase.UNOBSERVABLE -> RuntimeServicePhase.UNOBSERVABLE
    }

    private fun toRuntimeProcessPhase(phase: ProcessPhase): RuntimeProcessPhase = when (phase) {
        ProcessPhase.CREATED -> RuntimeProcessPhase.CREATED
        ProcessPhase.STARTED -> RuntimeProcessPhase.STARTED
        ProcessPhase.READY -> RuntimeProcessPhase.READY
        ProcessPhase.EXITING -> RuntimeProcessPhase.EXITING
        ProcessPhase.EXITED -> RuntimeProcessPhase.EXITED
        ProcessPhase.UNKNOWN -> RuntimeProcessPhase.UNKNOWN
    }

    private fun toRuntimeStartupPhase(phase: StartupPhase): RuntimeStartupPhase = when (phase) {
        StartupPhase.NOT_STARTED -> RuntimeStartupPhase.NOT_STARTED
        StartupPhase.DETECTING -> RuntimeStartupPhase.DETECTING
        StartupPhase.READY -> RuntimeStartupPhase.READY
        StartupPhase.FAILED -> RuntimeStartupPhase.FAILED
    }

    private val currentSessionContextRef: AtomicReference<ServiceSessionContext?> = AtomicReference(null)
    private val currentSessionRef: AtomicReference<ProotSession?> = AtomicReference(null)
    private val currentSessionIdRef: AtomicReference<String?> = AtomicReference(null)
    private val currentGenerationRef: AtomicReference<Long> = AtomicReference(0L)
    private val currentIntent: AtomicReference<Intent?> = AtomicReference(null)
    private val currentStartIdRef = AtomicReference(0)
    private val startupFailureCleanupContextRef = AtomicReference<StartupFailureCleanupContext?>(null)
    private val lifecycleSnapshotRef = AtomicReference<RuntimeServiceLifecycleSnapshot>(
        RuntimeServiceLifecycleSnapshot(
            generation = 0L,
            sessionId = null,
            servicePhase = RuntimeServicePhase.CREATED,
            processPhase = RuntimeProcessPhase.CREATED,
            startupPhase = RuntimeStartupPhase.NOT_STARTED,
            terminalState = null,
            latestStartId = 0,
            stopRequested = false,
        )
    )
    private val latestStartIdRef = AtomicReference(0)
    private val stopRequestedRef = AtomicBoolean(false)

    init {
        instanceRef.set(this)
    }

    private fun updateLifecycleSnapshot() {
        val ctx = currentSessionContextRef.get()
        val sessionId = currentSessionIdRef.get()
        val generation = currentGenerationRef.get()
        val servicePhase = when (serviceState.get()) {
            ServiceHostState.CREATED -> if (ctx != null) toRuntimeServicePhase(ctx.servicePhase) else RuntimeServicePhase.CREATED
            ServiceHostState.FOREGROUND -> RuntimeServicePhase.FOREGROUND
            ServiceHostState.DESTROYED -> RuntimeServicePhase.DESTROYED
        }
        val processPhase = if (ctx != null) toRuntimeProcessPhase(ctx.processPhase) else RuntimeProcessPhase.CREATED
        val terminalState = toRuntimeTerminalState(ctx?.terminalEvent)
        val startupPhase = ctx?.startupPhase?.let { toRuntimeStartupPhase(it) } ?: RuntimeStartupPhase.NOT_STARTED
        val snapshot = RuntimeServiceLifecycleSnapshot(
            generation = generation,
            sessionId = sessionId,
            servicePhase = servicePhase,
            processPhase = processPhase,
            startupPhase = startupPhase,
            terminalState = terminalState,
            latestStartId = latestStartIdRef.get(),
            stopRequested = stopRequestedRef.get(),
        )
        lifecycleSnapshotRef.set(snapshot)
        endpoint.updateLifecycleSnapshot(snapshot)
    }

    private fun currentLifecycleSnapshot(): RuntimeServiceLifecycleSnapshot = lifecycleSnapshotRef.get()

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
            latestStartIdRef.set(0)
            stopRequestedRef.set(false)
            updateLifecycleSnapshot()
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
            RuntimeServiceContract.ACTION_STOP_HOST -> {
                val targetGeneration = intent.getLongExtra(RuntimeServiceContract.EXTRA_TARGET_GENERATION, Long.MIN_VALUE)
                handleStopHost(targetGeneration, startId)
            }
            RuntimeServiceContract.ACTION_TEARDOWN_AFTER_STARTUP_FAILURE -> {
                val generation = currentGenerationRef.get()
                teardownAfterStartupFailure(
                    generation = generation,
                    sessionId = currentSessionIdRef.get(),
                    launchStartId = currentStartIdRef.get(),
                    cause = RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR,
                    phase = "controller_requested_cleanup"
                )
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
            teardownAfterStartupFailure(
                generation = generation,
                sessionId = currentSessionIdRef.get(),
                launchStartId = startId,
                cause = RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR,
                phase = "invalid_generation",
                noProcessCreated = true
            )
            return
        }

        if (!serviceState.compareAndSet(ServiceHostState.CREATED, ServiceHostState.FOREGROUND)) {
            teardownAfterStartupFailure(
                generation = generation,
                sessionId = currentSessionIdRef.get(),
                launchStartId = startId,
                cause = RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR,
                phase = "state_not_created",
                noProcessCreated = true
            )
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
                    teardownAfterStartupFailure(
                        generation = generation,
                        sessionId = currentSessionIdRef.get(),
                        launchStartId = startId,
                        cause = RuntimeServiceTerminationCause.FOREGROUND_FAILED,
                        phase = "foreground_start_exception"
                    )
                }
            }
            is RuntimeForegroundNotificationResult.Failure -> {
                teardownAfterStartupFailure(
                    generation = generation,
                    sessionId = currentSessionIdRef.get(),
                    launchStartId = startId,
                    cause = RuntimeServiceTerminationCause.NOTIFICATION_FAILED,
                    phase = "notification_creation"
                )
            }
        }
    }

    private fun startProotSessionLocked(generation: Long, profile: String, startId: Int) {
        currentGenerationRef.set(generation)

        val component = AndroidRuntimeModule.prootComponent
        if (component == null) {
            teardownAfterStartupFailure(
                generation = generation,
                sessionId = currentSessionIdRef.get(),
                launchStartId = startId,
                cause = RuntimeServiceTerminationCause.PROOT_COMPONENT_MISSING,
                phase = "proot_component_check"
            )
            return
        }

        val rootfsPath = AndroidRuntimeModule.prootRootfsPath
        if (rootfsPath == null) {
            teardownAfterStartupFailure(
                generation = generation,
                sessionId = currentSessionIdRef.get(),
                launchStartId = startId,
                cause = RuntimeServiceTerminationCause.ROOTFS_MISSING,
                phase = "rootfs_path_check"
            )
            return
        }

        val assembler = AndroidRuntimeModule.prootEnvironmentAssembler
        if (assembler == null) {
            teardownAfterStartupFailure(
                generation = generation,
                sessionId = currentSessionIdRef.get(),
                launchStartId = startId,
                cause = RuntimeServiceTerminationCause.ASSEMBLER_MISSING,
                phase = "assembler_check"
            )
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
            teardownAfterStartupFailure(
                generation = generation,
                sessionId = currentSessionIdRef.get(),
                launchStartId = startId,
                cause = RuntimeServiceTerminationCause.ENVIRONMENT_BUILD_FAILED,
                phase = "environment_build"
            )
            return
        } catch (e: java.lang.IllegalArgumentException) {
            teardownAfterStartupFailure(
                generation = generation,
                sessionId = currentSessionIdRef.get(),
                launchStartId = startId,
                cause = RuntimeServiceTerminationCause.MOUNT_CONTRACT_INVALID,
                phase = "mount_contract"
            )
            return
        } catch (e: java.lang.IllegalStateException) {
            teardownAfterStartupFailure(
                generation = generation,
                sessionId = currentSessionIdRef.get(),
                launchStartId = startId,
                cause = RuntimeServiceTerminationCause.MOUNT_CONTRACT_INVALID,
                phase = "mount_contract"
            )
            return
        }

        val observer = ProotObserver { event -> onProotEvent(event, generation) }
        val session = try {
            component.launch(request, observer, generation)
        } catch (e: Exception) {
            teardownAfterStartupFailure(
                generation = generation,
                sessionId = currentSessionIdRef.get(),
                launchStartId = startId,
                cause = RuntimeServiceTerminationCause.ENVIRONMENT_BUILD_FAILED,
                phase = "proot_launch_exception"
            )
            return
        }
        if (!session.isAlive()) {
            teardownAfterStartupFailure(
                generation = generation,
                sessionId = currentSessionIdRef.get(),
                launchStartId = startId,
                cause = RuntimeServiceTerminationCause.ENVIRONMENT_BUILD_FAILED,
                phase = "proot_launch_failed_session"
            )
            return
        }
            currentSessionRef.set(session)
            currentSessionIdRef.set(session.sessionId)
            currentSessionContextRef.set(
                ServiceSessionContext(
                    generation = generation,
                    session = session,
                    launchStartId = startId,
                    latestStartId = startId,
                    servicePhase = ServicePhase.FOREGROUND,
                    processPhase = ProcessPhase.STARTED,
                )
            )
            latestStartIdRef.set(startId)
            updateLifecycleSnapshot()
            session.activate()
            session.markStarted()
    }

    private fun resolveActiveProgramSource(generation: Long, startId: Int): java.io.File? {
        val manager = AndroidRuntimeModule.activeRuntimeManager
        if (manager == null) {
            teardownAfterStartupFailure(
                generation = generation,
                sessionId = currentSessionIdRef.get(),
                launchStartId = startId,
                cause = RuntimeServiceTerminationCause.NO_ACTIVE_RUNTIME,
                phase = "active_runtime_manager_check"
            )
            return null
        }
        return when (val result = manager.resolveActiveProgramRoot()) {
            is com.amitia.amitia_app.runtime.install.ActiveProgramRootResult.Ready ->
                result.root.hostDirectory
            is com.amitia.amitia_app.runtime.install.ActiveProgramRootResult.NoActiveRuntime -> {
                teardownAfterStartupFailure(
                    generation = generation,
                    sessionId = currentSessionIdRef.get(),
                    launchStartId = startId,
                    cause = RuntimeServiceTerminationCause.NO_ACTIVE_RUNTIME,
                    phase = "no_active_runtime"
                )
                null
            }
            is com.amitia.amitia_app.runtime.install.ActiveProgramRootResult.Failure -> {
                teardownAfterStartupFailure(
                    generation = generation,
                    sessionId = currentSessionIdRef.get(),
                    launchStartId = startId,
                    cause = RuntimeServiceTerminationCause.ACTIVE_PROGRAM_ROOT_INVALID,
                    phase = "active_program_root_failure"
                )
                null
            }
        }
    }

    private fun teardownAfterStartupFailure(
        generation: Long,
        sessionId: String?,
        launchStartId: Int,
        cause: RuntimeServiceTerminationCause,
        phase: String,
        noProcessCreated: Boolean = false,
    ) {
        val cleanupContext = StartupFailureCleanupContext(
            generation = generation,
            sessionId = sessionId,
            launchStartId = launchStartId,
            originalFailure = cause,
            phase = phase,
            noProcessCreated = noProcessCreated,
            cleanupPhase = StartupFailureCleanupPhase.FAILURE_DETECTED,
        )
        if (!startupFailureCleanupContextRef.compareAndSet(null, cleanupContext)) {
            return
        }

        cleanupContext.cleanupPhase = StartupFailureCleanupPhase.STOP_REQUESTED

        if (noProcessCreated) {
            cleanupContext.processConfirmedDead = true
            cleanupContext.cleanupPhase = StartupFailureCleanupPhase.PROCESS_EXIT_CONFIRMED
            performStartupFailureCleanup(cleanupContext)
            return
        }

        val sessionContext = currentSessionContextRef.get()
        if (sessionContext == null) {
            cleanupContext.processConfirmedDead = true
            cleanupContext.cleanupPhase = StartupFailureCleanupPhase.PROCESS_EXIT_CONFIRMED
            performStartupFailureCleanup(cleanupContext)
            return
        }

        cleanupContext.cleanupPhase = StartupFailureCleanupPhase.WAITING_FOR_PROCESS_EXIT

        val session = currentSessionRef.get()
        if (session != null) {
            when (val result = session.terminateAndConfirmExit(
                gracefulTimeoutMs = GRACEFUL_SHUTDOWN_TIMEOUT_MS,
                forceTimeoutMs = FORCE_SHUTDOWN_TIMEOUT_MS
            )) {
                is ProotTerminationResult.ConfirmedExited -> {
                    cleanupContext.processConfirmedDead = true
                    cleanupContext.cleanupPhase = StartupFailureCleanupPhase.PROCESS_EXIT_CONFIRMED
                    performStartupFailureCleanup(cleanupContext)
                }
                is ProotTerminationResult.StillAlive -> {
                    cleanupContext.processConfirmedDead = false
                }
            }
        } else {
            cleanupContext.processConfirmedDead = true
            cleanupContext.cleanupPhase = StartupFailureCleanupPhase.PROCESS_EXIT_CONFIRMED
            performStartupFailureCleanup(cleanupContext)
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
            is ProotEvent.ExitWatcherFailed -> {
                if (exitWatcherFailureAlreadyHandled()) return
                handleExitWatcherFailed(event)
            }
            else -> {}
        }
    }

    private fun exitWatcherFailureAlreadyHandled(): Boolean {
        val ctx = currentSessionContextRef.get() ?: return false
        return ctx.terminalEvent != null
    }

    private fun handleExitWatcherFailed(event: ProotEvent.ExitWatcherFailed) {
        lock.withLock {
            if (destroyed.get()) return
            val ctx = currentSessionContextRef.get() ?: return
            if (ctx.terminalEvent != null) return

            ctx.servicePhase = ServicePhase.UNOBSERVABLE
            ctx.processPhase = ProcessPhase.UNKNOWN
            stopRequestedRef.set(false)
            updateLifecycleSnapshot()

            val cleanupContext = startupFailureCleanupContextRef.get()
            if (cleanupContext != null && cleanupContext.generation == event.generation) {
                confirmProcessDeathForStartupFailure(cleanupContext)
                return
            }

            val session = currentSessionRef.get()
            if (session != null) {
                when (val result = session.terminateAndConfirmExit(
                    gracefulTimeoutMs = GRACEFUL_SHUTDOWN_TIMEOUT_MS,
                    forceTimeoutMs = FORCE_SHUTDOWN_TIMEOUT_MS
                )) {
                    is ProotTerminationResult.ConfirmedExited -> {
                        ctx.processPhase = ProcessPhase.EXITED
                        ctx.teardownState = TeardownState.IN_PROGRESS
                        publishWatcherFailureTerminalEvent(ctx, event, result.exitCode)
                    }
                    is ProotTerminationResult.StillAlive -> {
                        ctx.teardownState = TeardownState.IN_PROGRESS
                        updateLifecycleSnapshot()
                    }
                }
            } else {
                ctx.teardownState = TeardownState.IN_PROGRESS
                updateLifecycleSnapshot()
            }
        }
    }

    private fun publishWatcherFailureTerminalEvent(
        ctx: ServiceSessionContext,
        event: ProotEvent.ExitWatcherFailed,
        exitCode: Int?,
    ) {
        val terminalKind = if (ctx.stopRequested) {
            TerminalEventKind.EXPECTED_STOPPED
        } else {
            TerminalEventKind.UNEXPECTED_TERMINATION
        }
        ctx.terminalEvent = terminalKind

        val teardownReason = if (terminalKind == TerminalEventKind.EXPECTED_STOPPED) {
            ServiceTeardownReason.EXPECTED_STOP
        } else {
            ServiceTeardownReason.UNEXPECTED_TERMINATION
        }
        val stopId = ctx.stopStartId ?: ctx.launchStartId
        val teardownResult = performServiceTeardown(ctx, stopId, teardownReason)
        updateLifecycleSnapshot()

        publishTerminalEventForExpectedOrUnexpected(
            ctx = ctx,
            teardownResult = teardownResult,
            unexpectedCause = RuntimeServiceTerminationCause.EXIT_WATCHER_FAILED,
        )
    }

    private fun confirmProcessDeathForStartupFailure(cleanupContext: StartupFailureCleanupContext) {
        val session = currentSessionRef.get()
        if (session == null) {
            cleanupContext.processConfirmedDead = true
            cleanupContext.cleanupPhase = StartupFailureCleanupPhase.PROCESS_EXIT_CONFIRMED
            performStartupFailureCleanup(cleanupContext)
            return
        }

        when (val result = session.terminateAndConfirmExit(
            gracefulTimeoutMs = GRACEFUL_SHUTDOWN_TIMEOUT_MS,
            forceTimeoutMs = FORCE_SHUTDOWN_TIMEOUT_MS
        )) {
            is ProotTerminationResult.ConfirmedExited -> {
                cleanupContext.processConfirmedDead = true
                cleanupContext.cleanupPhase = StartupFailureCleanupPhase.PROCESS_EXIT_CONFIRMED
                performStartupFailureCleanup(cleanupContext)
            }
            is ProotTerminationResult.StillAlive -> {
                cleanupContext.processConfirmedDead = false
                cleanupContext.cleanupPhase = StartupFailureCleanupPhase.WAITING_FOR_PROCESS_EXIT
            }
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
                cleanupContext.cleanupPhase = StartupFailureCleanupPhase.PROCESS_EXIT_CONFIRMED
                sessionContext.terminalEvent = TerminalEventKind.STARTUP_FAILURE_CLEANUP
                sessionContext.processPhase = ProcessPhase.EXITED
                performStartupFailureCleanup(cleanupContext)
                return
            }

            if (sessionContext.terminalEvent != null) return

            sessionContext.processPhase = ProcessPhase.EXITED

            val terminalKind = if (sessionContext.stopRequested) {
                TerminalEventKind.EXPECTED_STOPPED
            } else {
                TerminalEventKind.UNEXPECTED_TERMINATION
            }
            sessionContext.terminalEvent = terminalKind

            val teardownReason = if (terminalKind == TerminalEventKind.EXPECTED_STOPPED) {
                ServiceTeardownReason.EXPECTED_STOP
            } else {
                ServiceTeardownReason.UNEXPECTED_TERMINATION
            }
            val stopId = sessionContext.stopStartId ?: sessionContext.launchStartId
            val teardownResult = performServiceTeardown(sessionContext, stopId, teardownReason)
            updateLifecycleSnapshot()

            publishTerminalEventForExpectedOrUnexpected(
                ctx = sessionContext,
                teardownResult = teardownResult,
                unexpectedCause = RuntimeServiceTerminationCause.SESSION_EXITED,
            )
        }
    }

    private fun publishTerminalEventForExpectedOrUnexpected(
        ctx: ServiceSessionContext,
        teardownResult: ServiceTeardownResult,
        unexpectedCause: RuntimeServiceTerminationCause,
    ) {
        when {
            ctx.terminalEvent == TerminalEventKind.EXPECTED_STOPPED &&
                teardownResult is ServiceTeardownResult.FullyStopped -> {
                serviceState.set(ServiceHostState.DESTROYED)
                ctx.servicePhase = ServicePhase.DESTROYED
                updateLifecycleSnapshot()
                endpoint.notify(
                    RuntimeServiceHostEvent.ExpectedStopped(
                        generation = ctx.generation,
                        result = teardownResult,
                    )
                )
            }
            ctx.terminalEvent == TerminalEventKind.EXPECTED_STOPPED &&
                teardownResult !is ServiceTeardownResult.FullyStopped -> {
                serviceState.set(ServiceHostState.DESTROYED)
                ctx.servicePhase = ServicePhase.DESTROYED
                updateLifecycleSnapshot()
                endpoint.notify(
                    RuntimeServiceHostEvent.UnexpectedTermination(
                        generation = ctx.generation,
                        cause = RuntimeServiceTerminationCause.STOP_RESULT_FAILED,
                    )
                )
            }
            ctx.terminalEvent == TerminalEventKind.UNEXPECTED_TERMINATION -> {
                serviceState.set(ServiceHostState.DESTROYED)
                ctx.servicePhase = ServicePhase.DESTROYED
                updateLifecycleSnapshot()
                endpoint.notify(
                    RuntimeServiceHostEvent.UnexpectedTermination(
                        generation = ctx.generation,
                        cause = unexpectedCause,
                    )
                )
            }
        }
        clearSessionState()
    }

    private fun performServiceTeardown(context: ServiceSessionContext, startId: Int, reason: ServiceTeardownReason): ServiceTeardownResult {
        if (context.teardownState == TeardownState.COMPLETE) {
            return ServiceTeardownResult.SupersededByNewStart
        }
        if (latestStartIdRef.get() > startId) {
            context.teardownState = TeardownState.SUPERSEDED
            return ServiceTeardownResult.SupersededByNewStart
        }
        context.teardownState = TeardownState.IN_PROGRESS
        try {
            stopForeground(STOP_FOREGROUND_REMOVE)
        } catch (_: Exception) {
        }
        val stopResult = try {
            stopSelfResult(startId)
        } catch (_: Exception) {
            false
        }
        return if (stopResult) {
            context.teardownState = TeardownState.COMPLETE
            ServiceTeardownResult.FullyStopped(startId)
        } else {
            if (latestStartIdRef.get() > startId) {
                context.teardownState = TeardownState.SUPERSEDED
                ServiceTeardownResult.SupersededByNewStart
            } else {
                context.teardownState = TeardownState.FAILED
                ServiceTeardownResult.Failed
            }
        }
    }

    private fun performStartupFailureCleanup(cleanupContext: StartupFailureCleanupContext) {
        if (!cleanupContext.processConfirmedDead && !cleanupContext.noProcessCreated) {
            return
        }
        cleanupContext.cleanupPhase = StartupFailureCleanupPhase.SERVICE_TEARDOWN
        serviceState.set(ServiceHostState.CREATED)
        clearSessionState()
        startupFailureCleanupContextRef.set(null)
        cleanupContext.cleanupPhase = StartupFailureCleanupPhase.COMPLETE
        try {
            stopForeground(STOP_FOREGROUND_REMOVE)
        } catch (_: Exception) {
        }
        tryStopSelf(cleanupContext.launchStartId)
        updateLifecycleSnapshot()
        endpoint.notify(
            RuntimeServiceHostEvent.StartupFailed(
                generation = cleanupContext.generation,
                cause = cleanupContext.originalFailure,
                message = cleanupContext.phase,
                sessionId = cleanupContext.sessionId,
                launchStartId = cleanupContext.launchStartId,
                phase = cleanupContext.phase,
            )
        )
    }

    private fun tryStopSelf(startId: Int): Boolean {
        return try {
            stopSelfResult(startId)
        } catch (_: Exception) {
            false
        }
    }

    private fun clearSessionState() {
        currentSessionContextRef.set(null)
        currentSessionRef.set(null)
        currentSessionIdRef.set(null)
        currentGenerationRef.set(0L)
        latestStartIdRef.set(0)
        stopRequestedRef.set(false)
        updateLifecycleSnapshot()
    }

    private fun handleStopHost(targetGeneration: Long, stopStartId: Int) {
        if (targetGeneration == Long.MIN_VALUE || targetGeneration <= 0L) {
            return
        }
        lock.withLock {
            val sessionContext = currentSessionContextRef.get() ?: return
            if (sessionContext.generation != targetGeneration) {
                return
            }
            val currentStopId = sessionContext.stopStartId
            if (currentStopId != null && stopStartId < currentStopId) {
                return
            }
            sessionContext.stopStartId = stopStartId
            sessionContext.stopRequested = true
            sessionContext.latestStartId = stopStartId
            latestStartIdRef.set(stopStartId)
            stopRequestedRef.set(true)
            sessionContext.processPhase = ProcessPhase.EXITING
            val session = currentSessionRef.get()
            if (session != null && session.isAlive()) {
                session.requestStop()
                session.stop(GRACEFUL_SHUTDOWN_TIMEOUT_MS)
            }
            updateLifecycleSnapshot()
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
        var unexpectedTerminationGeneration: Long? = null
        var unexpectedTerminationCause: RuntimeServiceTerminationCause? = null
        lock.withLock {
            destroyed.set(true)
            serviceState.set(ServiceHostState.DESTROYED)
            val sessionContext = currentSessionContextRef.get()
            if (sessionContext != null && sessionContext.terminalEvent == null) {
                val session = currentSessionRef.get()
                val processConfirmedDead = if (session != null) {
                    when (val result = session.terminateAndConfirmExit(
                        gracefulTimeoutMs = GRACEFUL_SHUTDOWN_TIMEOUT_MS,
                        forceTimeoutMs = FORCE_SHUTDOWN_TIMEOUT_MS
                    )) {
                        is ProotTerminationResult.ConfirmedExited -> {
                            sessionContext.processPhase = ProcessPhase.EXITED
                            true
                        }
                        is ProotTerminationResult.StillAlive -> {
                            sessionContext.processPhase = ProcessPhase.UNKNOWN
                            false
                        }
                    }
                } else {
                    true
                }

                sessionContext.terminalEvent = TerminalEventKind.UNEXPECTED_TERMINATION
                sessionContext.servicePhase = ServicePhase.DESTROYED
                sessionContext.teardownState = TeardownState.COMPLETE
                unexpectedTerminationCause = RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR
                unexpectedTerminationGeneration = sessionContext.generation
                try {
                    stopForeground(STOP_FOREGROUND_REMOVE)
                } catch (_: Exception) {
                }

                if (processConfirmedDead) {
                    clearSessionState()
                }
            } else {
                clearSessionState()
            }
            startupFailureCleanupContextRef.set(null)
            updateLifecycleSnapshot()
        }
        if (unexpectedTerminationGeneration != null && unexpectedTerminationCause != null) {
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
