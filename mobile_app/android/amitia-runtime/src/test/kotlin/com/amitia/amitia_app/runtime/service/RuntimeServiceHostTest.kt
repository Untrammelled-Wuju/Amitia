package com.amitia.amitia_app.runtime.service

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class RuntimeServiceHostTest {

    @Test
    fun runtimeServiceHost_interfaceHasExpectedMethods() {
        val methods = RuntimeServiceHost::class.java.declaredMethods
        val names = methods.map { it.name }.toSet()
        assertTrue(names.contains("ensureStarted"))
        assertTrue(names.contains("requestStop"))
        assertTrue(names.contains("addListener"))
        assertTrue(names.contains("removeListener"))
        assertTrue(names.contains("currentSession"))
        assertTrue(names.contains("currentGeneration"))
        assertEquals(6, methods.size)
    }

    @Test
    fun runtimeServiceHost_mutatingMethodsReturnRuntimeServiceResult() {
        val methods = RuntimeServiceHost::class.java.declaredMethods
        val names = methods.map { it.name to it.returnType }.toMap()
        assertEquals(RuntimeServiceResult::class.java, names["ensureStarted"])
        assertEquals(RuntimeServiceResult::class.java, names["requestStop"])
    }

    @Test
    fun runtimeServiceHost_listenerMethodsAcceptHostListener() {
        val methods = RuntimeServiceHost::class.java.declaredMethods
        val addListener = methods.first { it.name == "addListener" }
        val removeListener = methods.first { it.name == "removeListener" }
        assertEquals(1, addListener.parameterTypes.size)
        assertEquals(RuntimeServiceHostListener::class.java, addListener.parameterTypes[0])
        assertEquals(1, removeListener.parameterTypes.size)
        assertEquals(RuntimeServiceHostListener::class.java, removeListener.parameterTypes[0])
    }
}
