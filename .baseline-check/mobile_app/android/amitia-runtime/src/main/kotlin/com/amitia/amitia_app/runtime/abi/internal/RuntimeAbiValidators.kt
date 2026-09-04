package com.amitia.amitia_app.runtime.abi.internal

internal object RuntimeAbiValidators {
    private val INVALID_ABIS = setOf("armeabi-v7a", "x86", "x86_64", "mips", "mips64", "riscv64")

    fun isForbiddenAbi(abi: String): Boolean {
        val normalized = abi.trim().lowercase(java.util.Locale.ROOT)
        return normalized in INVALID_ABIS
    }

    fun containsArm64(abis: List<String>): Boolean {
        return abis.any { it.trim().equals("arm64-v8a", ignoreCase = true) }
    }
}