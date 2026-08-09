package com.amitia.amitia_app.runtime.startup

internal sealed interface RuntimeStartupResult {
    val generation: Long

    data class Ready(
        override val generation: Long
    ) : RuntimeStartupResult

    data class Failed(
        override val generation: Long,
        val error: RuntimeStartupError
    ) : RuntimeStartupResult

    data class Cancelled(
        override val generation: Long
    ) : RuntimeStartupResult
}
