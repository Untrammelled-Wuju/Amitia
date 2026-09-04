package com.amitia.amitia_app.runtime.proot

sealed class ProotAvailability {
    data class Available(val artifact: ProotArtifact, val absoluteBinaryPath: String) : ProotAvailability()
    data class Unavailable(val errorCode: ProotErrorCode, val messageKey: String) : ProotAvailability()
    data class Invalid(val errorCode: ProotErrorCode, val messageKey: String) : ProotAvailability()
    object Closed : ProotAvailability()
}