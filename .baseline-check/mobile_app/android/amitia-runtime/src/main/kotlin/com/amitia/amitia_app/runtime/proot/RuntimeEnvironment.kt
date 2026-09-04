package com.amitia.amitia_app.runtime.proot

internal data class RuntimeEnvironment(
    val hostProcess: Map<String, String>,
    val guestRuntime: Map<String, String>,
) {
    init {
        validateKeys(hostProcess)
        validateKeys(guestRuntime)
        validateValues(hostProcess)
        validateValues(guestRuntime)
    }

    companion object {
        private val KEY_PATTERN = Regex("^[A-Za-z_][A-Za-z0-9_]*$")

        fun validateKeys(env: Map<String, String>) {
            for (key in env.keys) {
                require(key.isNotEmpty()) { "environment key must not be empty" }
                require(!key.contains("=")) { "environment key must not contain =" }
                require(KEY_PATTERN.matches(key)) { "environment key invalid: $key" }
            }
        }

        fun validateValues(env: Map<String, String>) {
            for ((_, value) in env) {
                require(!value.contains("\u0000")) { "environment value must not contain NUL" }
            }
        }
    }
}
