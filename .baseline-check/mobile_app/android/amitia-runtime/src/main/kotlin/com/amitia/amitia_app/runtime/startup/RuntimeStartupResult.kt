package com.amitia.amitia_app.runtime.startup

internal sealed interface RuntimeStartupResult {
    val generation: Long

    data class Ready(
        override val generation: Long,
        val elapsedMs: Long,
        val probeCount: Int
    ) : RuntimeStartupResult

    data class Failed(
        override val generation: Long,
        val error: RuntimeStartupError,
        val elapsedMs: Long = 0L
    ) : RuntimeStartupResult

    data class Cancelled(
        override val generation: Long
    ) : RuntimeStartupResult
}
