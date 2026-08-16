package com.amitia.amitia_app.runtime.service

import android.content.Intent
import com.amitia.amitia_app.runtime.AndroidRuntimeModule
import com.amitia.amitia_app.runtime.install.ActiveProgramRoot
import com.amitia.amitia_app.runtime.install.ActiveProgramRootResult
import com.amitia.amitia_app.runtime.install.ActiveRuntimeManager
import com.amitia.amitia_app.runtime.install.ActiveRuntimeResult
import com.amitia.amitia_app.runtime.install.internal.DefaultRuntimeHostLayout
import com.amitia.amitia_app.runtime.proot.MountRole
import com.amitia.amitia_app.runtime.proot.ProotComponent
import com.amitia.amitia_app.runtime.proot.ProotEnvironment
import com.amitia.amitia_app.runtime.proot.ProotLaunchRequest
import com.amitia.amitia_app.runtime.proot.ProotLaunchSpec
import com.amitia.amitia_app.runtime.proot.ProotObserver
import com.amitia.amitia_app.runtime.proot.ProotSession
import com.amitia.amitia_app.runtime.proot.ProotStopResult
import com.amitia.amitia_app.runtime.proot.ProotTerminationResult
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironment
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentRequest
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentResult
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentBuilder
import com.amitia.amitia_app.runtime.proot.internal.ProotEnvironmentAssembler
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.Robolectric
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import java.util.concurrent.CopyOnWriteArrayList
import java.io.File

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [26])
class RuntimeServiceLifecycleTest {

    private lateinit var service: RuntimeService
    private val events = CopyOnWriteArrayList<RuntimeServiceHostEvent>()

    @Before
    fun setUp() {
        AndroidRuntimeModule.resetCacheForTest()
        service = Robolectric.setupService(RuntimeService::class.java)
        val binder = service.onBind(null) as RuntimeServiceBinder
        binder.endpoint.addListener(RuntimeServiceHostListener { events.add(it) })
    }

    @After
    fun tearDown() {
        AndroidRuntimeModule.resetCacheForTest()
        service.onDestroy()
        events.clear()
        RuntimeService.clearInstanceRef()
    }

    @Test
    fun l001_freshStart_sessionRegisteredBeforeStarted() {
        val component = LifecycleProotComponent()
        setAndroidRuntimeModule(
            prootComponent = component,
            prootRootfsPath = "/rootfs",
            assembler = LifecycleProotEnvironmentAssembler(),
            activeRuntimeManager = LifecycleValidActiveRuntimeManager(),
        )

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 1L)
        }, 0, 1)

        assertNotNull(service.currentProotSession())
        assertEquals(1, component.launchCount)
        assertEquals(0, events.count { it is RuntimeServiceHostEvent.StartupFailed })
    }

    @Test
    fun l002_processLaunchImmediatelyExits_failsClosed() {
        val component = ImmediatelyDeadProotComponent()
        setAndroidRuntimeModule(
            prootComponent = component,
            prootRootfsPath = "/rootfs",
            assembler = LifecycleProotEnvironmentAssembler(),
            activeRuntimeManager = LifecycleValidActiveRuntimeManager(),
        )

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 1L)
        }, 0, 1)

        val failedEvents = events.filterIsInstance<RuntimeServiceHostEvent.StartupFailed>()
        assertEquals(1, failedEvents.size())
        assertEquals(1L, failedEvents[0].generation)
        assertNull(service.currentProotSession())
    }

    @Test
    fun l003_listenerLateBinding_replaysLastEvent() {
        setAndroidRuntimeModule(
            prootComponent = LifecycleProotComponent(),
            prootRootfsPath = "/rootfs",
            assembler = LifecycleProotEnvironmentAssembler(),
            activeRuntimeManager = LifecycleValidActiveRuntimeManager(),
        )

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 1L)
        }, 0, 1)

        assertTrue(events.any { it is RuntimeServiceHostEvent.SessionReady })

        val lateEvents = CopyOnWriteArrayList<RuntimeServiceHostEvent>()
        val binder = service.onBind(null) as RuntimeServiceBinder
        binder.endpoint.addListener(RuntimeServiceHostListener { lateEvents.add(it) })

        val lateReadyEvents = lateEvents.filterIsInstance<RuntimeServiceHostEvent.SessionReady>()
        assertEquals(1, lateReadyEvents.size())
    }

    @Test
    fun l004_normalStop_expectedStoppedAndTeardown() {
        val component = LifecycleProotComponent()
        setAndroidRuntimeModule(
            prootComponent = component,
            prootRootfsPath = "/rootfs",
            assembler = LifecycleProotEnvironmentAssembler(),
            activeRuntimeManager = LifecycleValidActiveRuntimeManager(),
        )

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 1L)
        }, 0, 1)

        assertTrue(events.any { it is RuntimeServiceHostEvent.SessionReady })

        component.session?.requestStop()
        val stopResult = component.session?.stop(100L)
        assertNotNull(stopResult)

        val terminalEvents = events.filter {
            it is RuntimeServiceHostEvent.ExpectedStopped || it is RuntimeServiceHostEvent.UnexpectedTermination
        }
        assertEquals(1, terminalEvents.size)
        assertEquals(1L, terminalEvents[0].generation)
    }

    @Test
    fun l005_nTeardownWithN1Start_oldTeardownDoesNotAffectNewSession() {
        val component = LifecycleProotComponent()
        setAndroidRuntimeModule(
            prootComponent = component,
            prootRootfsPath = "/rootfs",
            assembler = LifecycleProotEnvironmentAssembler(),
            activeRuntimeManager = LifecycleValidActiveRuntimeManager(),
        )

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 1L)
        }, 0, 1)

        val session1 = service.currentProotSession()
        assertNotNull(session1)

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_STOP_HOST
            putExtra(RuntimeServiceContract.EXTRA_TARGET_GENERATION, 1L)
        }, 0, 2)

        val terminalEvents = events.filter {
            it is RuntimeServiceHostEvent.ExpectedStopped || it is RuntimeServiceHostEvent.UnexpectedTermination
        }
        assertTrue(terminalEvents.size <= 1)
    }

    @Test
    fun l006_watcherFatalFailure_processStillAlive_failClosed() {
        val component = WatcherFailureProotComponent()
        setAndroidRuntimeModule(
            prootComponent = component,
            prootRootfsPath = "/rootfs",
            assembler = LifecycleProotEnvironmentAssembler(),
            activeRuntimeManager = LifecycleValidActiveRuntimeManager(),
        )

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 1L)
        }, 0, 1)

        component.simulateWatcherFailure()

        val unexpectedTerminations = events.filterIsInstance<RuntimeServiceHostEvent.UnexpectedTermination>()
        assertEquals(1, unexpectedTerminations.size())
        assertEquals(RuntimeServiceTerminationCause.EXIT_WATCHER_FAILED, unexpectedTerminations[0].cause)
    }

    @Test
    fun l007_watcherFailureThenProcessDeath_singleTerminalEvent() {
        val component = WatcherFailureProotComponent()
        setAndroidRuntimeModule(
            prootComponent = component,
            prootRootfsPath = "/rootfs",
            assembler = LifecycleProotEnvironmentAssembler(),
            activeRuntimeManager = LifecycleValidActiveRuntimeManager(),
        )

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 1L)
        }, 0, 1)

        component.simulateWatcherFailure()
        component.simulateExited()

        val unexpectedTerminations = events.filterIsInstance<RuntimeServiceHostEvent.UnexpectedTermination>()
        assertEquals(1, unexpectedTerminations.size())
    }

    @Test
    fun l009_readyCrash_unexpectedTermination() {
        val component = LifecycleProotComponent()
        setAndroidRuntimeModule(
            prootComponent = component,
            prootRootfsPath = "/rootfs",
            assembler = LifecycleProotEnvironmentAssembler(),
            activeRuntimeManager = LifecycleValidActiveRuntimeManager(),
        )

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 1L)
        }, 0, 1)

        assertTrue(events.any { it is RuntimeServiceHostEvent.SessionReady })

        component.session?.let { component.simulateUnexpectedExit() }

        val unexpectedTerminations = events.filterIsInstance<RuntimeServiceHostEvent.UnexpectedTermination>()
        assertEquals(1, unexpectedTerminations.size())
    }

    @Test
    fun l011_staleExited_doesNotAffectNewGeneration() {
        val component = LifecycleProotComponent()
        setAndroidRuntimeModule(
            prootComponent = component,
            prootRootfsPath = "/rootfs",
            assembler = LifecycleProotEnvironmentAssembler(),
            activeRuntimeManager = LifecycleValidActiveRuntimeManager(),
        )

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 1L)
        }, 0, 1)

        assertNotNull(service.currentProotSession())

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 2L)
        }, 0, 2)

        val unexpectedTerminations = events.filterIsInstance<RuntimeServiceHostEvent.UnexpectedTermination>()
        assertEquals(0, unexpectedTerminations.size())
    }

    @Test
    fun l012_duplicateStop_singleTerminalEvent() {
        val component = LifecycleProotComponent()
        setAndroidRuntimeModule(
            prootComponent = component,
            prootRootfsPath = "/rootfs",
            assembler = LifecycleProotEnvironmentAssembler(),
            activeRuntimeManager = LifecycleValidActiveRuntimeManager(),
        )

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 1L)
        }, 0, 1)

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_STOP_HOST
            putExtra(RuntimeServiceContract.EXTRA_TARGET_GENERATION, 1L)
        }, 0, 2)

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_STOP_HOST
            putExtra(RuntimeServiceContract.EXTRA_TARGET_GENERATION, 1L)
        }, 0, 3)

        val terminalEvents = events.filter {
            it is RuntimeServiceHostEvent.ExpectedStopped || it is RuntimeServiceHostEvent.UnexpectedTermination
        }
        assertEquals(1, terminalEvents.size)
    }

    @Test
    fun l014_serviceReconnect_snapshotRestoresState() {
        setAndroidRuntimeModule(
            prootComponent = LifecycleProotComponent(),
            prootRootfsPath = "/rootfs",
            assembler = LifecycleProotEnvironmentAssembler(),
            activeRuntimeManager = LifecycleValidActiveRuntimeManager(),
        )

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 1L)
        }, 0, 1)

        val snapshotBefore = service.snapshot()
        assertTrue(snapshotBefore.created)

        RuntimeService.clearInstanceRef()
        AndroidRuntimeModule.resetCacheForTest()

        service.onCreate()
        val snapshotAfter = service.snapshot()
        assertTrue(snapshotAfter.created)
    }

    @Test
    fun l015_readOnlyMount_programReadOnlyDataWritable() {
        val tempFolder = org.junit.rules.TemporaryFolder()
        tempFolder.create()
        val layout = DefaultRuntimeHostLayout(
            controlBaseDir = tempFolder.newFolder("control"),
            dataBaseDir = tempFolder.newFolder("data"),
        )
        val programSource = File(layout.runtimeVersionRoot("1.0.0"), "program")
        programSource.mkdirs()

        val mountContract = com.amitia.amitia_app.runtime.proot.MountContract.build(
            hostLayout = layout,
            activeProgramSource = programSource,
        )

        assertFalse(mountContract.programMount.writable)
        assertTrue(mountContract.dataMount.writable)
        assertTrue(mountContract.cacheMount.writable)
        assertTrue(mountContract.logsMount.writable)
        assertEquals(MountRole.PROGRAM, mountContract.programMount.role)
    }

    @Test
    fun service_onCreate_doesNotAutoStartRuntime() {
        val snapshot = service.snapshot()
        assertTrue(snapshot.created)
        assertFalse(snapshot.foreground)
    }

    @Test
    fun service_onBind_returnsBinder() {
        val intent = Intent()
        val binder = service.onBind(intent)
        assertNotNull(binder)
        assertTrue(binder is RuntimeServiceBinder)
    }

    @Test
    fun service_canBeStopped() {
        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_STOP_HOST
        }, 0, 1)
        assertTrue(true)
    }

    @Test
    fun service_onDestroy_notifiesEndpoint() {
        service.onDestroy()
        val snapshot = service.snapshot()
        assertFalse(snapshot.created)
    }

    private fun setAndroidRuntimeModule(
        prootComponent: ProotComponent?,
        prootRootfsPath: String?,
        assembler: ProotEnvironmentAssembler?,
        activeRuntimeManager: ActiveRuntimeManager?,
    ) {
        LifecycleTestAndroidRuntimeModuleOverride(
            prootComponent = prootComponent,
            prootRootfsPath = prootRootfsPath,
            assembler = assembler,
            activeRuntimeManager = activeRuntimeManager,
        ).apply()
    }
}

internal class LifecycleProotComponent : ProotComponent {
    var launchCount = 0
        private set
    var session: LifecycleProotSession? = null
        private set

    override fun availability() = com.amitia.amitia_app.runtime.proot.ProotAvailability.Unavailable(
        com.amitia.amitia_app.runtime.proot.ProotErrorCode.NOT_PACKAGED, "test"
    )

    override fun launch(request: ProotLaunchRequest, observer: ProotObserver, generation: Long): ProotSession {
        launchCount++
        val s = LifecycleProotSession(generation, observer)
        session = s
        return s
    }

    override fun launchProbe(request: ProotLaunchRequest, observer: ProotObserver, generation: Long): ProotSession {
        return LifecycleProotSession(generation, observer)
    }

    override fun currentSession(): ProotSession? = session

    override fun stop(): ProotStopResult {
        session?.requestStop()
        return ProotStopResult.Graceful(session?.sessionId ?: "none", 0)
    }

    override fun close() {
        session = null
    }

    fun simulateUnexpectedExit() {
        session?.simulateExit()
    }
}

internal class LifecycleProotSession(
    private val gen: Long,
    private val observer: ProotObserver,
) : ProotSession {
    override val sessionId: String = "lifecycle-session-$gen"
    private val alive = java.util.concurrent.atomic.AtomicBoolean(true)
    private val stopReq = java.util.concurrent.atomic.AtomicBoolean(false)
    private val exitRef = java.util.concurrent.atomic.AtomicReference<com.amitia.amitia_app.runtime.proot.ProotExit?>(null)
    private val started = java.util.concurrent.atomic.AtomicBoolean(false)

    override fun isAlive(): Boolean = alive.get()

    override fun awaitExit(timeoutMillis: Long): Int? = exitRef.get()?.exitCode

    override fun activate() {
        if (started.compareAndSet(false, true)) {
            observer.onEvent(com.amitia.amitia_app.runtime.proot.ProotEvent.Started(sessionId, System.currentTimeMillis()))
        }
    }

    override fun stop(graceMillis: Long): ProotStopResult {
        stopReq.set(true)
        val result = ProotStopResult.Graceful(sessionId, 0)
        val exit = com.amitia.amitia_app.runtime.proot.ProotExit(
            generation = gen,
            sessionId = sessionId,
            exitCode = 0,
            stopRequested = true,
        )
        exitRef.set(exit)
        alive.set(false)
        observer.onEvent(com.amitia.amitia_app.runtime.proot.ProotEvent.Exited(exit))
        return result
    }

    override fun close() {
        alive.set(false)
    }

    override fun requestStop() {
        stopReq.set(true)
    }

    override val exit: com.amitia.amitia_app.runtime.proot.ProotExit? get() = exitRef.get()

    override fun terminateAndConfirmExit(gracefulTimeoutMs: Long, forceTimeoutMs: Long): ProotTerminationResult {
        return ProotTerminationResult.ConfirmedExited(exit?.exitCode)
    }

    fun simulateExit() {
        alive.set(false)
        val exit = com.amitia.amitia_app.runtime.proot.ProotExit(
            generation = gen,
            sessionId = sessionId,
            exitCode = 1,
            stopRequested = stopReq.get(),
        )
        exitRef.set(exit)
        observer.onEvent(com.amitia.amitia_app.runtime.proot.ProotEvent.Exited(exit))
    }
}

internal class ImmediatelyDeadProotComponent : ProotComponent {
    var launchCount = 0
        private set

    override fun availability() = com.amitia.amitia_app.runtime.proot.ProotAvailability.Unavailable(
        com.amitia.amitia_app.runtime.proot.ProotErrorCode.NOT_PACKAGED, "test"
    )

    override fun launch(request: ProotLaunchRequest, observer: ProotObserver, generation: Long): ProotSession {
        launchCount++
        return LifecycleDeadProotSession(generation)
    }

    override fun launchProbe(request: ProotLaunchRequest, observer: ProotObserver, generation: Long): ProotSession {
        return LifecycleDeadProotSession(generation)
    }

    override fun currentSession(): ProotSession? = null

    override fun stop(): ProotStopResult = ProotStopResult.AlreadyStopped("dead", null)

    override fun close() {}
}

internal class LifecycleDeadProotSession(private val gen: Long) : ProotSession {
    override val sessionId: String = "dead-session-$gen"
    override fun isAlive(): Boolean = false
    override fun awaitExit(timeoutMillis: Long): Int? = 1
    override fun stop(graceMillis: Long) = ProotStopResult.AlreadyStopped(sessionId, 1)
    override fun close() {}
    override fun requestStop() {}
    override val exit: com.amitia.amitia_app.runtime.proot.ProotExit = com.amitia.amitia_app.runtime.proot.ProotExit(
        generation = gen,
        sessionId = sessionId,
        exitCode = 1,
        stopRequested = false,
    )

    override fun terminateAndConfirmExit(gracefulTimeoutMs: Long, forceTimeoutMs: Long): ProotTerminationResult {
        return ProotTerminationResult.ConfirmedExited(exit?.exitCode)
    }
}

internal class WatcherFailureProotComponent : ProotComponent {
    var launchCount = 0
        private set
    var session: LifecycleWatcherFailureSession? = null
        private set

    override fun availability() = com.amitia.amitia_app.runtime.proot.ProotAvailability.Unavailable(
        com.amitia.amitia_app.runtime.proot.ProotErrorCode.NOT_PACKAGED, "test"
    )

    override fun launch(request: ProotLaunchRequest, observer: ProotObserver, generation: Long): ProotSession {
        launchCount++
        val s = LifecycleWatcherFailureSession(generation, observer)
        session = s
        return s
    }

    override fun launchProbe(request: ProotLaunchRequest, observer: ProotObserver, generation: Long): ProotSession {
        return LifecycleWatcherFailureSession(generation, observer)
    }

    override fun currentSession(): ProotSession? = session

    override fun stop(): ProotStopResult {
        session?.let { it.requestStop() }
        return ProotStopResult.Graceful(session?.sessionId ?: "none", 0)
    }

    override fun close() {
        session = null
    }

    fun simulateWatcherFailure() {
        session?.simulateWatcherFailure()
    }

    fun simulateExited() {
        session?.simulateExited()
    }
}

internal class LifecycleWatcherFailureSession(
    private val gen: Long,
    private val observer: ProotObserver,
) : ProotSession {
    override val sessionId: String = "watcher-failure-session-$gen"
    private val alive = java.util.concurrent.atomic.AtomicBoolean(true)
    private val stopReq = java.util.concurrent.atomic.AtomicBoolean(false)
    private val exitRef = java.util.concurrent.atomic.AtomicReference<com.amitia.amitia_app.runtime.proot.ProotExit?>(null)
    private val started = java.util.concurrent.atomic.AtomicBoolean(false)

    override fun isAlive(): Boolean = alive.get()

    override fun awaitExit(timeoutMillis: Long): Int? = exitRef.get()?.exitCode

    override fun activate() {
        if (started.compareAndSet(false, true)) {
            observer.onEvent(com.amitia.amitia_app.runtime.proot.ProotEvent.Started(sessionId, System.currentTimeMillis()))
        }
    }

    override fun stop(graceMillis: Long): ProotStopResult {
        stopReq.set(true)
        alive.set(false)
        val exit = com.amitia.amitia_app.runtime.proot.ProotExit(
            generation = gen,
            sessionId = sessionId,
            exitCode = 0,
            stopRequested = true,
        )
        exitRef.set(exit)
        observer.onEvent(com.amitia.amitia_app.runtime.proot.ProotEvent.Exited(exit))
        return ProotStopResult.Graceful(sessionId, 0)
    }

    override fun close() {
        alive.set(false)
    }

    override fun requestStop() {
        stopReq.set(true)
    }

    override val exit: com.amitia.amitia_app.runtime.proot.ProotExit? get() = exitRef.get()

    override fun terminateAndConfirmExit(gracefulTimeoutMs: Long, forceTimeoutMs: Long): ProotTerminationResult {
        return ProotTerminationResult.ConfirmedExited(exit?.exitCode)
    }

    fun simulateWatcherFailure() {
        observer.onEvent(
            com.amitia.amitia_app.runtime.proot.ProotEvent.ExitWatcherFailed(
                sessionId = sessionId,
                generation = gen,
                message = "simulated watcher failure",
            )
        )
    }

    fun simulateExited() {
        alive.set(false)
        val exit = com.amitia.amitia_app.runtime.proot.ProotExit(
            generation = gen,
            sessionId = sessionId,
            exitCode = 1,
            stopRequested = stopReq.get(),
        )
        exitRef.set(exit)
        observer.onEvent(com.amitia.amitia_app.runtime.proot.ProotEvent.Exited(exit))
    }
}

internal class LifecycleProotEnvironmentAssembler : ProotEnvironmentAssembler(
    layout = DefaultRuntimeHostLayout(
        controlBaseDir = File("/data/control"),
        dataBaseDir = File("/data/data"),
    ),
    environmentBuilder = LifecycleTestRuntimeEnvironmentBuilder(),
) {
    override fun assembleBackendLaunch(activeProgramSource: File, runtimeProfile: String): ProotLaunchSpec {
        return ProotLaunchSpec(
            binaryPath = "",
            rootfsPath = "/rootfs",
            workingDirectory = "/opt/amitia",
            command = listOf("/opt/amitia/server"),
            bindMounts = emptyList(),
            environment = ProotEnvironment.EMPTY,
        )
    }
}

internal class LifecycleTestRuntimeEnvironmentBuilder : RuntimeEnvironmentBuilder {
    override fun build(request: RuntimeEnvironmentRequest): RuntimeEnvironmentResult {
        return RuntimeEnvironmentResult.Success(
            RuntimeEnvironment(
                hostProcess = emptyMap(),
                guestRuntime = emptyMap(),
            )
        )
    }
}

internal class LifecycleValidActiveRuntimeManager : ActiveRuntimeManager {
    override fun current() = ActiveRuntimeResult.Active(
        com.amitia.amitia_app.runtime.install.ActiveRuntimeInfo("1.0.0", 0L)
    )
    override fun activate(version: String) = ActiveRuntimeResult.Active(
        com.amitia.amitia_app.runtime.install.ActiveRuntimeInfo(version, 0L)
    )
    override fun resolveActiveProgramRoot() = ActiveProgramRootResult.Ready(
        ActiveProgramRoot(
            runtimeVersion = "1.0.0",
            hostDirectory = File("/data/versions/1.0.0"),
            manifestIdentity = "test",
        )
    )
}

internal class LifecycleTestAndroidRuntimeModuleOverride(
    private val prootComponent: ProotComponent?,
    private val prootRootfsPath: String?,
    private val assembler: ProotEnvironmentAssembler?,
    private val activeRuntimeManager: ActiveRuntimeManager?,
) {
    fun apply() {
        val clazz = AndroidRuntimeModule::class.java
        setField(clazz, "cachedProotComponent", prootComponent)
        setField(clazz, "cachedRootfsPath", prootRootfsPath)
        setField(clazz, "cachedProotEnvironmentAssembler", assembler)
        setField(clazz, "cachedActiveRuntimeManager", activeRuntimeManager)
    }

    private fun setField(clazz: Class<*>, fieldName: String, value: Any?) {
        val field = clazz.getDeclaredField(fieldName)
        field.isAccessible = true
        field.set(AndroidRuntimeModule, value)
    }
}
