package com.amitia.amitia_app.runtime.service

import com.amitia.amitia_app.runtime.AndroidRuntimeModule
import com.amitia.amitia_app.runtime.internal.DefaultRuntimeModule
import org.junit.After
import org.junit.Assert.assertSame
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment

@RunWith(RobolectricTestRunner::class)
class RuntimeServiceCompositionRootTest {

    @After
    fun tearDown() {
        AndroidRuntimeModule.resetCacheForTest()
    }

    @Test
    fun create_returnsSameControllerInstance() {
        val context = RuntimeEnvironment.getApplication()
        val module1 = AndroidRuntimeModule.create(context) as DefaultRuntimeModule
        val module2 = AndroidRuntimeModule.create(context) as DefaultRuntimeModule
        assertSame(module1.controller, module2.controller)
    }

    @Test
    fun create_returnsSameStateStoreInstance() {
        val context = RuntimeEnvironment.getApplication()
        val module1 = AndroidRuntimeModule.create(context) as DefaultRuntimeModule
        val module2 = AndroidRuntimeModule.create(context) as DefaultRuntimeModule
        assertSame(module1.stateStore(), module2.stateStore())
    }

    @Test
    fun create_returnsSameServiceHostInstance() {
        val context = RuntimeEnvironment.getApplication()
        val module1 = AndroidRuntimeModule.create(context) as DefaultRuntimeModule
        val module2 = AndroidRuntimeModule.create(context) as DefaultRuntimeModule
        assertSame(module1.serviceHost(), module2.serviceHost())
    }
}
