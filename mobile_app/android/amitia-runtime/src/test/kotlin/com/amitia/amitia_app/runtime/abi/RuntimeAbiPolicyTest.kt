package com.amitia.amitia_app.runtime.abi

import org.junit.Assert.assertEquals
import org.junit.Assert.assertSame
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class RuntimeAbiPolicyTest {

    @Test
    fun `amitiaAndroid returns arm64-only policy`() {
        val policy = RuntimeAbiPolicy.amitiaAndroid()
        assertEquals(setOf("arm64-v8a"), policy.allowedAbis)
        assertEquals("arm64-v8a", policy.required64BitAbi)
        assertTrue(policy.requires64BitProcess)
    }

    @Test
    fun `amitiaAndroid rejects forbidden abi`() {
        assertThrows(IllegalArgumentException::class.java) {
            RuntimeAbiPolicy(
                allowedAbis = setOf("arm64-v8a", "x86_64"),
                required64BitAbi = "arm64-v8a",
                requires64BitProcess = true
            )
        }
    }

    @Test
    fun `amitiaAndroid is stable singleton`() {
        assertSame(RuntimeAbiPolicy.amitiaAndroid(), RuntimeAbiPolicy.amitiaAndroid())
    }

    @Test
    fun `allowedAbis returns defensive copy`() {
        val policy = RuntimeAbiPolicy.amitiaAndroid()
        val abis = policy.allowedAbis
        assertEquals(1, abis.size)
    }

    @Test
    fun `throws when required64BitAbi not in allowedAbis`() {
        assertThrows(IllegalArgumentException::class.java) {
            RuntimeAbiPolicy(
                allowedAbis = setOf("arm64-v8a"),
                required64BitAbi = "x86_64",
                requires64BitProcess = true
            )
        }
    }
}
