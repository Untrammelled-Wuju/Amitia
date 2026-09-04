package com.amitia.amitia_app.runtime.abi

import android.os.Build
import androidx.test.ext.junit.runners.AndroidJUnit4
import com.amitia.amitia_app.runtime.abi.internal.BuildAndroidAbiProvider
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Assume.assumeTrue
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class RuntimeProcessBitnessInstrumentedTest {

    @Test
    fun process_is_64_bit() {
        assumeTrue(
            "Requires API 23+ for Process.is64Bit()",
            Build.VERSION.SDK_INT >= Build.VERSION_CODES.M
        )
        assertTrue(
            "Process must be 64-bit",
            android.os.Process.is64Bit()
        )
    }

    @Test
    fun supported_64_bit_abis_not_empty() {
        assumeTrue(
            "Requires API 21+ for SUPPORTED_64_BIT_ABIS",
            Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP
        )
        assertTrue(
            "SUPPORTED_64_BIT_ABIS must not be empty",
            Build.SUPPORTED_64_BIT_ABIS.isNotEmpty()
        )
    }

    @Test
    fun snapshot_process_bitness_matches_runtime() {
        val provider = BuildAndroidAbiProvider()
        val snapshot = provider.snapshot()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            val expected = android.os.Process.is64Bit()
            assertTrue(
                "Snapshot processIs64Bit must match Process.is64Bit()",
                snapshot.processIs64Bit == expected
            )
        } else {
            assertNotNull(snapshot)
        }
    }

    @Test
    fun snapshot_64_bit_list_matches_build() {
        assumeTrue(
            "Requires API 21+ for SUPPORTED_64_BIT_ABIS",
            Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP
        )
        val provider = BuildAndroidAbiProvider()
        val snapshot = provider.snapshot()
        val device64 = Build.SUPPORTED_64_BIT_ABIS.toList()
        assertTrue(
            "Snapshot supported64BitAbis must match Build.SUPPORTED_64_BIT_ABIS. Got: ${snapshot.supported64BitAbis} vs device: $device64",
            snapshot.supported64BitAbis == device64
        )
    }

    @Test
    fun os_architecture_not_null() {
        val provider = BuildAndroidAbiProvider()
        val snapshot = provider.snapshot()
        assertTrue(
            "Snapshot osArchitecture should not be null or blank",
            !snapshot.osArchitecture.isNullOrBlank()
        )
    }
}
