package com.amitia.amitia_app.runtime.abi

data class RuntimeAbiSnapshot(
    val supportedAbis: List<String>,
    val supported64BitAbis: List<String>,
    val supported32BitAbis: List<String>,
    val processIs64Bit: Boolean?,
    val osArchitecture: String?
) {
    init {
        require(supportedAbis.none { it.isBlank() }) { "supportedAbis must not contain blank" }
        require(supported64BitAbis.none { it.isBlank() }) { "supported64BitAbis must not contain blank" }
        require(supported32BitAbis.none { it.isBlank() }) { "supported32BitAbis must not contain blank" }
    }
}