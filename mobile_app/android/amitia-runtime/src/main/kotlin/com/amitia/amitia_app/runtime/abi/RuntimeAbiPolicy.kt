package com.amitia.amitia_app.runtime.abi

data class RuntimeAbiPolicy(
    val allowedAbis: Set<String>,
    val required64BitAbi: String,
    val requires64BitProcess: Boolean
) {
    companion object {
        fun amitiaAndroid(): RuntimeAbiPolicy {
            return RuntimeAbiPolicy(
                allowedAbis = setOf("arm64-v8a"),
                required64BitAbi = "arm64-v8a",
                requires64BitProcess = true
            )
        }
    }

    init {
        require(allowedAbis.isNotEmpty()) { "allowedAbis must not be empty" }
        require(required64BitAbi.isNotEmpty()) { "required64BitAbi must not be empty" }
        for (abi in allowedAbis) {
            require(abi.isNotBlank()) { "abi must not be blank" }
            require(!abi.equals("armeabi-v7a", ignoreCase = true)) { "armeabi-v7a is not supported" }
            require(!abi.equals("x86", ignoreCase = true)) { "x86 is not supported" }
            require(!abi.equals("x86_64", ignoreCase = true)) { "x86_64 is not supported" }
        }
    }
}