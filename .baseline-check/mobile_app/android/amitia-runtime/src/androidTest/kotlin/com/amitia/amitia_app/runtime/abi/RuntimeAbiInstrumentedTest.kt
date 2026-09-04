package com.amitia.amitia_app.runtime.abi

import android.os.Build
import androidx.test.ext.junit.runners.AndroidJUnit4
import com.amitia.amitia_app.runtime.abi.internal.BuildAndroidAbiProvider
import com.amitia.amitia_app.runtime.abi.internal.DefaultRuntimeAbiGate
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class RuntimeAbiInstrumentedTest {

    @Test
    fun device_supports_arm64_v8a() {
        assertTrue(
            "Device must support arm64-v8a",
            Build.SUPPORTED_ABIS.contains("arm64-v8a")
        )
    }

    @Test
    fun supported_abis_contains_arm64() {
        val abis = Build.SUPPORTED_ABIS.toList()
        assertTrue(
            "Supported ABIs must include arm64-v8a, got: $abis",
            abis.any { it == "arm64-v8a" }
        )
    }

    @Test
    fun gate_evaluates_supported_on_device() {
        val gate = DefaultRuntimeAbiGate(provider = BuildAndroidAbiProvider())
        val result = gate.evaluate()
        assertTrue(
            "Gate should report Supported on arm64 device, got: $result",
            result is RuntimeAbiStatus.Supported
        )
    }

    @Test
    fun snapshot_from_device_contains_arm64() {
        val provider = BuildAndroidAbiProvider()
        val snapshot = provider.snapshot()
        assertTrue(
            "Snapshot supportedAbis must contain arm64-v8a, got: ${snapshot.supportedAbis}",
            snapshot.supportedAbis.contains("arm64-v8a")
        )
    }

    @Test
    fun snapshot_64_bit_list_contains_arm64() {
        val provider = BuildAndroidAbiProvider()
        val snapshot = provider.snapshot()
        assertTrue(
            "Snapshot supported64BitAbis must contain arm64-v8a, got: ${snapshot.supported64BitAbis}",
            snapshot.supported64BitAbis.contains("arm64-v8a")
        )
    }

    @Test
    fun supported_abi_field_is_arm64_v8a() {
        val gate = DefaultRuntimeAbiGate(provider = BuildAndroidAbiProvider())
        val result = gate.evaluate() as RuntimeAbiStatus.Supported
        assertEquals("arm64-v8a", result.abi)
    }
}
