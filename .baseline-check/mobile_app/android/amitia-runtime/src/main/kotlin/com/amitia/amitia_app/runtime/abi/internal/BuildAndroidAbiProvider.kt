package com.amitia.amitia_app.runtime.abi.internal

import android.os.Build
import com.amitia.amitia_app.runtime.abi.AndroidAbiProvider
import com.amitia.amitia_app.runtime.abi.RuntimeAbiSnapshot

internal class BuildAndroidAbiProvider(
    private val processBitnessProvider: () -> Boolean? = { defaultProcessIs64Bit() },
    private val osArchProvider: () -> String? = { System.getProperty("os.arch") }
) : AndroidAbiProvider {

    override fun snapshot(): RuntimeAbiSnapshot {
        val supported = runCatching { Build.SUPPORTED_ABIS?.toList() ?: emptyList() }.getOrDefault(emptyList())
        val supported64 = runCatching { Build.SUPPORTED_64_BIT_ABIS?.toList() ?: emptyList() }.getOrDefault(emptyList())
        val supported32 = runCatching { Build.SUPPORTED_32_BIT_ABIS?.toList() ?: emptyList() }.getOrDefault(emptyList())
        val processBitness = runCatching { processBitnessProvider() }.getOrNull()
        val osArch = runCatching { osArchProvider() }.getOrNull()

        return RuntimeAbiSnapshot(
            supportedAbis = RuntimeAbiNormalizer.normalizeList(supported),
            supported64BitAbis = RuntimeAbiNormalizer.normalizeList(supported64),
            supported32BitAbis = RuntimeAbiNormalizer.normalizeList(supported32),
            processIs64Bit = processBitness,
            osArchitecture = osArch
        )
    }

    companion object {
        fun defaultProcessIs64Bit(): Boolean? {
            return try {
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
                    android.os.Process.is64Bit()
                } else {
                    null
                }
            } catch (_: Throwable) {
                null
            }
        }
    }
}