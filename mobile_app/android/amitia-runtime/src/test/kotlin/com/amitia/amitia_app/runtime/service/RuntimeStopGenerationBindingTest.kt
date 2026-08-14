package com.amitia.amitia_app.runtime.service

import android.content.Intent
import com.amitia.amitia_app.runtime.service.internal.RuntimeForegroundNotification
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.Robolectric
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import java.util.concurrent.CopyOnWriteArrayList

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [26])
class RuntimeStopGenerationBindingTest {

    private lateinit var service: RuntimeService
    private val events = CopyOnWriteArrayList<RuntimeServiceHostEvent>()

    @Before
    fun setUp() {
        service = Robolectric.setupService(RuntimeService::class.java)
        val binder = service.onBind(null) as RuntimeServiceBinder
        binder.endpoint.addListener(RuntimeServiceHostListener { events.add(it) })
    }

    @After
    fun tearDown() {
        service.onDestroy()
        events.clear()
        RuntimeService.clearInstanceRef()
    }

    @Test
    fun stopWithMatchingGeneration_completesSuccessfully() {
        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 100L)
        }, 0, 1)

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_STOP_HOST
            putExtra(RuntimeServiceContract.EXTRA_TARGET_GENERATION, 100L)
        }, 0, 2)

        val expectedStopped = events.filterIsInstance<RuntimeServiceHostEvent.ExpectedStopped>()
        assertEquals(1, expectedStopped.size)
        assertEquals(100L, expectedStopped[0].generation)
    }

    @Test
    fun stopWithMismatchedGeneration_doesNotStopCurrentSession() {
        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 200L)
        }, 0, 1)

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_STOP_HOST
            putExtra(RuntimeServiceContract.EXTRA_TARGET_GENERATION, 100L)
        }, 0, 2)

        val expectedStopped = events.filterIsInstance<RuntimeServiceHostEvent.ExpectedStopped>()
        assertEquals(0, expectedStopped.size)
    }

    @Test
    fun stopWithMissingGeneration_failsClosed() {
        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 300L)
        }, 0, 1)

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_STOP_HOST
        }, 0, 2)

        val expectedStopped = events.filterIsInstance<RuntimeServiceHostEvent.ExpectedStopped>()
        assertEquals(0, expectedStopped.size)
    }

    @Test
    fun stopWithInvalidGenerationZero_failsClosed() {
        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 400L)
        }, 0, 1)

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_STOP_HOST
            putExtra(RuntimeServiceContract.EXTRA_TARGET_GENERATION, 0L)
        }, 0, 2)

        val expectedStopped = events.filterIsInstance<RuntimeServiceHostEvent.ExpectedStopped>()
        assertEquals(0, expectedStopped.size)
    }

    @Test
    fun stopWithInvalidGenerationNegative_failsClosed() {
        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 500L)
        }, 0, 1)

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_STOP_HOST
            putExtra(RuntimeServiceContract.EXTRA_TARGET_GENERATION, -1L)
        }, 0, 2)

        val expectedStopped = events.filterIsInstance<RuntimeServiceHostEvent.ExpectedStopped>()
        assertEquals(0, expectedStopped.size)
    }

    @Test
    fun stopWithNoActiveSession_failsClosed() {
        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_STOP_HOST
            putExtra(RuntimeServiceContract.EXTRA_TARGET_GENERATION, 600L)
        }, 0, 1)

        val expectedStopped = events.filterIsInstance<RuntimeServiceHostEvent.ExpectedStopped>()
        assertEquals(0, expectedStopped.size)
    }

    @Test
    fun lateStopForOldGeneration_doesNotStopNewSession() {
        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 700L)
        }, 0, 1)

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_STOP_HOST
            putExtra(RuntimeServiceContract.EXTRA_TARGET_GENERATION, 700L)
        }, 0, 2)

        val expectedStopped = events.filterIsInstance<RuntimeServiceHostEvent.ExpectedStopped>()
        assertEquals(1, expectedStopped.size)
        assertEquals(700L, expectedStopped[0].generation)

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_START_HOST
            putExtra(RuntimeServiceContract.EXTRA_RUNTIME_GENERATION, 800L)
        }, 0, 3)

        service.onStartCommand(Intent().apply {
            action = RuntimeServiceContract.ACTION_STOP_HOST
            putExtra(RuntimeServiceContract.EXTRA_TARGET_GENERATION, 700L)
        }, 0, 4)

        val expectedStoppedAfter = events.filterIsInstance<RuntimeServiceHostEvent.ExpectedStopped>()
        assertEquals(1, expectedStoppedAfter.size)
        assertEquals(700L, expectedStoppedAfter[0].generation)
    }
}
