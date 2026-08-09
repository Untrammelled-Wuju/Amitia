package com.amitia.amitia_app.runtime.recovery

internal sealed interface RuntimeRecoveryDecision {
    data object DoNotRecover : RuntimeRecoveryDecision
    data class RecoverAfter(val delayMillis: Long) : RuntimeRecoveryDecision
    data class Exhausted(val attempts: Int) : RuntimeRecoveryDecision
}
