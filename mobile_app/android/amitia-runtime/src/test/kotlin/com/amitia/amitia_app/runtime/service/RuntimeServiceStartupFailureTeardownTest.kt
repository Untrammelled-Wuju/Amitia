package com.amitia.amitia_app.runtime.service

import android.content.Intent
import com.amitia.amitia_app.runtime.AndroidRuntimeModule
import com.amitia.amitia_app.runtime.install.ActiveProgramRootResult
import com.amitia.amitia_app.runtime.install.ActiveRuntimeManager
import com.amitia.amitia_app.runtime.install.internal.DefaultRuntimeHostLayout
import com.amitia.amitia_app.runtime.proot.ProotComponent
import com.amitia.amitia_app.runtime.proot.ProotEnvironment
import com.amitia.amitia_app.runtime.proot.ProotLaunchRequest
import com.amitia.amitia_app.runtime.proot.ProotObserver
import com.amitia.amitia_app.runtime.proot.ProotSession
import com.amitia.amitia_app.runtime.proot.ProotStopResult
import com.amitia.amitia_app.runtime.proot.internal.ProotEnvironmentAssembler
import org.junit.After
import org.junit.Assert.assertEquals
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
class RuntimeServiceStartupFailureTeardownTest {

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
    fun invalidGeneration_failsClosedWithTeardown() {
        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 0L)
        }, 0, 1)

        val failedEvents = events.filterIsInstance<RuntimeServiceHostEvent.LaunchFailed>()
        assertEquals(1, failedEvents.size())
        assertEquals(RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR, failedEvents[0].cause)
    }

    @Test
    fun noProotComponent_preLaunchFailureTeardown() {
        setAndroidRuntimeModule(
            prootComponent = null,
            prootRootfsPath = "/rootfs",
            assembler = TestProotEnvironmentAssembler(),
            activeRuntimeManager = ValidActiveRuntimeManager(),
        )

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 1L)
        }, 0, 1)

        val failedEvents = events.filterIsInstance<RuntimeServiceHostEvent.LaunchFailed>()
        assertEquals(1, failedEvents.size())
        assertEquals(1L, failedEvents[0].generation)
        assertEquals(RuntimeServiceTerminationCause.PROOT_COMPONENT_MISSING, failedEvents[0].cause)
        assertNull(service.currentProotSession())
    }

    @Test
    fun noRootfs_preLaunchFailureTeardown() {
        val component = TestProotComponent()
        setAndroidRuntimeModule(
            prootComponent = component,
            prootRootfsPath = null,
            assembler = TestProotEnvironmentAssembler(),
            activeRuntimeManager = ValidActiveRuntimeManager(),
        )

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 2L)
        }, 0, 1)

        val failedEvents = events.filterIsInstance<RuntimeServiceHostEvent.LaunchFailed>()
        assertEquals(1, failedEvents.size())
        assertEquals(2L, failedEvents[0].generation)
        assertEquals(RuntimeServiceTerminationCause.ROOTFS_MISSING, failedEvents[0].cause)
        assertNull(service.currentProotSession())
        assertEquals(0, component.launchCount)
    }

    @Test
    fun noActiveRuntime_preLaunchFailureTeardown() {
        val component = TestProotComponent()
        val assembler = TestProotEnvironmentAssembler()
        setAndroidRuntimeModule(
            prootComponent = component,
            prootRootfsPath = "/rootfs",
            assembler = assembler,
            activeRuntimeManager = NoActiveRuntimeManager(),
        )

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 3L)
        }, 0, 1)

        val failedEvents = events.filterIsInstance<RuntimeServiceHostEvent.LaunchFailed>()
        assertEquals(1, failedEvents.size())
        assertEquals(3L, failedEvents[0].generation)
        assertEquals(RuntimeServiceTerminationCause.NO_ACTIVE_RUNTIME, failedEvents[0].cause)
        assertNull(service.currentProotSession())
        assertEquals(0, component.launchCount)
    }

    @Test
    fun environmentBuildFailure_preLaunchFailureTeardown() {
        val component = TestProotComponent()
        val assembler = FailingEnvironmentAssembler()
        setAndroidRuntimeModule(
            prootComponent = component,
            prootRootfsPath = "/rootfs",
            assembler = assembler,
            activeRuntimeManager = ValidActiveRuntimeManager(),
        )

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 4L)
        }, 0, 1)

        val failedEvents = events.filterIsInstance<RuntimeServiceHostEvent.LaunchFailed>()
        assertEquals(1, failedEvents.size())
        assertEquals(4L, failedEvents[0].generation)
        assertEquals(RuntimeServiceTerminationCause.ENVIRONMENT_BUILD_FAILED, failedEvents[0].cause)
        assertNull(service.currentProotSession())
        assertEquals(0, component.launchCount)
    }

    @Test
    fun assemblerMissing_preLaunchFailureTeardown() {
        val component = TestProotComponent()
        setAndroidRuntimeModule(
            prootComponent = component,
            prootRootfsPath = "/rootfs",
            assembler = null,
            activeRuntimeManager = ValidActiveRuntimeManager(),
        )

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 6L)
        }, 0, 1)

        val failedEvents = events.filterIsInstance<RuntimeServiceHostEvent.LaunchFailed>()
        assertEquals(1, failedEvents.size())
        assertEquals(6L, failedEvents[0].generation)
        assertEquals(RuntimeServiceTerminationCause.ASSEMBLER_MISSING, failedEvents[0].cause)
        assertNull(service.currentProotSession())
        assertEquals(0, component.launchCount)
    }

    @Test
    fun launchException_prootLaunchFailureTeardown() {
        val component = ThrowingProotComponent()
        val assembler = TestProotEnvironmentAssembler()
        setAndroidRuntimeModule(
            prootComponent = component,
            prootRootfsPath = "/rootfs",
            assembler = assembler,
            activeRuntimeManager = ValidActiveRuntimeManager(),
        )

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 7L)
        }, 0, 1)

        val failedEvents = events.filterIsInstance<RuntimeServiceHostEvent.LaunchFailed>()
        assertEquals(1, failedEvents.size())
        assertEquals(7L, failedEvents[0].generation)
        assertEquals(RuntimeServiceTerminationCause.ENVIRONMENT_BUILD_FAILED, failedEvents[0].cause)
        assertNull(service.currentProotSession())
        assertEquals(1, component.launchCount)
    }

    @Test
    fun successfulLaunch_doesNotTeardown() {
        val component = TestProotComponent()
        val assembler = TestProotEnvironmentAssembler()
        setAndroidRuntimeModule(
            prootComponent = component,
            prootRootfsPath = "/rootfs",
            assembler = assembler,
            activeRuntimeManager = ValidActiveRuntimeManager(),
        )

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 5L)
        }, 0, 1)

        assertEquals(0, events.count { it is RuntimeServiceHostEvent.LaunchFailed })
        assertEquals(1, component.launchCount)
        assertNotNull(service.currentProotSession())
    }

    private fun setAndroidRuntimeModule(
        prootComponent: ProotComponent?,
        prootRootfsPath: String?,
        assembler: ProotEnvironmentAssembler?,
        activeRuntimeManager: ActiveRuntimeManager?,
    ) {
        TestAndroidRuntimeModuleOverride(
            prootComponent = prootComponent,
            prootRootfsPath = prootRootfsPath,
            assembler = assembler,
            activeRuntimeManager = activeRuntimeManager,
        ).apply()
    }
}

class ThrowingProotComponent : ProotComponent {
    var launchCount = 0
        private set

    override fun availability() = com.amitia.amitia_app.runtime.proot.ProotAvailability.Unavailable(
        com.amitia.amitia_app.runtime.proot.ProotErrorCode.NOT_PACKAGED, "test"
    )

    override fun launch(request: ProotLaunchRequest, observer: ProotObserver, generation: Long): ProotSession {
        launchCount++
        throw RuntimeException("simulated launch failure")
    }

    override fun launchProbe(request: ProotLaunchRequest, observer: ProotObserver, generation: Long): ProotSession {
        return TestProotSession()
    }

    override fun currentSession(): ProotSession? = null

    override fun stop(): ProotStopResult = ProotStopResult.AlreadyStopped("test", null)

    override fun close() {}
}

class NoActiveRuntimeManager : ActiveRuntimeManager {
    override fun current() = com.amitia.amitia_app.runtime.install.ActiveRuntimeResult.NoActiveRuntime
    override fun activate(version: String) = com.amitia.amitia_app.runtime.install.ActiveRuntimeResult.NoActiveRuntime
    override fun resolveActiveProgramRoot() = ActiveProgramRootResult.NoActiveRuntime
}

class ValidActiveRuntimeManager : ActiveRuntimeManager {
    override fun current() = com.amitia.amitia_app.runtime.install.ActiveRuntimeResult.Active(
        com.amitia.amitia_app.runtime.install.ActiveRuntimeInfo("1.0.0", 0L)
    )
    override fun activate(version: String) = com.amitia.amitia_app.runtime.install.ActiveRuntimeResult.Active(
        com.amitia.amitia_app.runtime.install.ActiveRuntimeInfo(version, 0L)
    )
    override fun resolveActiveProgramRoot() = ActiveProgramRootResult.Ready(
        com.amitia.amitia_app.runtime.install.ActiveProgramRoot(
            runtimeVersion = "1.0.0",
            hostDirectory = File("/data/versions/1.0.0"),
            manifestIdentity = "test",
        )
    )
}

class TestAndroidRuntimeModuleOverride(
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
