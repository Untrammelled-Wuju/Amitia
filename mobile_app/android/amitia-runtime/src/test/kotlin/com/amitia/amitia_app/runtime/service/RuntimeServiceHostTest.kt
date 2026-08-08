package com.amitia.amitia_app.runtime.service

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class RuntimeServiceHostTest {

    @Test
    fun runtimeServiceHost_interfaceHasThreeMethods() {
        val methods = RuntimeServiceHost::class.java.declaredMethods
        val names = methods.map { it.name }.toSet()
        assertTrue(names.contains("ensureStarted"))
        assertTrue(names.contains("requestStop"))
        assertTrue(names.contains("isServiceRunning"))
        assertEquals(3, methods.size)
    }

    @Test
    fun runtimeServiceHost_mutatingMethodsReturnRuntimeServiceResult() {
        val methods = RuntimeServiceHost::class.java.declaredMethods
        val names = methods.map { it.name to it.returnType }.toMap()
        assertEquals(RuntimeServiceResult::class.java, names["ensureStarted"])
        assertEquals(RuntimeServiceResult::class.java, names["requestStop"])
    }
}
