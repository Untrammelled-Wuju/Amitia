package com.amitia.amitia_app.runtime.abi.internal

import com.amitia.amitia_app.runtime.abi.AndroidAbiProvider
import com.amitia.amitia_app.runtime.abi.RuntimeAbiSnapshot
import com.amitia.amitia_app.runtime.abi.RuntimeAbiStatus
import com.amitia.amitia_app.runtime.abi.UnsupportedReason
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class DefaultRuntimeAbiGateTest {

    private fun snapshot(
        abis: List<String> = listOf("arm64-v8a"),
        abis64: List<String> = listOf("arm64-v8a"),
        abis32: List<String> = emptyList(),
        is64: Boolean? = true
    ) = RuntimeAbiSnapshot(
        supportedAbis = abis,
        supported64BitAbis = abis64,
        supported32BitAbis = abis32,
        processIs64Bit = is64,
        osArchitecture = null
    )

    private fun fakeGate(s: RuntimeAbiSnapshot): DefaultRuntimeAbiGate {
        return DefaultRuntimeAbiGate(provider = FakeProvider(s))
    }

    @Test
    fun `supported when arm64-v8a present and 64 bit process`() {
        val gate = fakeGate(snapshot())
        val result = gate.evaluate()
        assertTrue(result is RuntimeAbiStatus.Supported)
        assertEquals("arm64-v8a", (result as RuntimeAbiStatus.Supported).abi)
    }

    @Test
    fun `unsupported when supportedAbis empty`() {
        val gate = fakeGate(snapshot(abis = emptyList()))
        val result = gate.evaluate()
        assertTrue(result is RuntimeAbiStatus.Unsupported)
        assertEquals(UnsupportedReason.SUPPORTED_ABIS_EMPTY, (result as RuntimeAbiStatus.Unsupported).reason)
    }

    @Test
    fun `unsupported when arm64 missing from supported`() {
        val gate = fakeGate(snapshot(abis = listOf("x86_64")))
        val result = gate.evaluate()
        assertTrue(result is RuntimeAbiStatus.Unsupported)
        assertEquals(UnsupportedReason.ARM64_ABI_MISSING, (result as RuntimeAbiStatus.Unsupported).reason)
    }

    @Test
    fun `unsupported when arm64 missing from 64-bit list`() {
        val gate = fakeGate(snapshot(abis64 = emptyList()))
        val result = gate.evaluate()
        assertTrue(result is RuntimeAbiStatus.Unsupported)
        assertEquals(UnsupportedReason.ARM64_64_BIT_ABI_MISSING, (result as RuntimeAbiStatus.Unsupported).reason)
    }

    @Test
    fun `unsupported when process is 32 bit`() {
        val gate = fakeGate(snapshot(is64 = false))
        val result = gate.evaluate()
        assertTrue(result is RuntimeAbiStatus.Unsupported)
        assertEquals(UnsupportedReason.PROCESS_IS_32_BIT, (result as RuntimeAbiStatus.Unsupported).reason)
    }

    @Test
    fun `supported when is64 is null unknown`() {
        val gate = fakeGate(snapshot(is64 = null))
        val result = gate.evaluate()
        assertTrue(result is RuntimeAbiStatus.Supported)
    }

    @Test
    fun `isSupported returns true for supported status`() {
        val gate = fakeGate(snapshot())
        assertTrue(gate.isSupported())
    }

    @Test
    fun `isSupported returns false for unsupported status`() {
        val gate = fakeGate(snapshot(abis = emptyList()))
        assertFalse(gate.isSupported())
    }

    @Test
    fun `supported result is cached`() {
        val gate = fakeGate(snapshot())
        val r1 = gate.evaluate()
        val r2 = gate.evaluate()
        assertEquals(r1, r2)
    }

    private class FakeProvider(private val snapshotValue: RuntimeAbiSnapshot) : AndroidAbiProvider {
        override fun snapshot(): RuntimeAbiSnapshot = snapshotValue
    }
}
