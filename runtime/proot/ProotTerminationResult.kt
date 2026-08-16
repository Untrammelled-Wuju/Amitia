package com.amitia.amitia_app.runtime.proot

sealed interface ProotTerminationResult {
    data class ConfirmedExited(val exitCode: Int?) : ProotTerminationResult
    data object StillAlive : ProotTerminationResult
}
