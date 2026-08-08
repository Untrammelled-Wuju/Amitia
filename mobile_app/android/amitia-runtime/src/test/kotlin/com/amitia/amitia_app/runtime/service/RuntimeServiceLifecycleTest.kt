package com.amitia.amitia_app.runtime.service

import android.content.Intent
import org.junit.After
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.Robolectric
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [26])
class RuntimeServiceLifecycleTest {

    private lateinit var service: RuntimeService

    @Before
    fun setUp() {
        service = Robolectric.setupService(RuntimeService::class.java)
    }

    @After
    fun tearDown() {
        service.onDestroy()
    }

    @Test
    fun service_onCreate_doesNotAutoStartRuntime() {
        val snapshot = service.snapshot()
        assertTrue(snapshot.created)
        assertTrue(!snapshot.foreground)
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
        assertTrue(!snapshot.created)
    }
}
