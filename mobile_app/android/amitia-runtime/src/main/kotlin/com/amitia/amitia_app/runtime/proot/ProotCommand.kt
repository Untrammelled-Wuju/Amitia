package com.amitia.amitia_app.runtime.proot

data class ProotCommand(val binaryPath: String, val arguments: List<String>, val environment: Map<String, String>) {
    override fun toString(): String = "ProotCommand(binaryPath=$binaryPath, arguments=$arguments, environmentKeys=${environment.keys})"
}
