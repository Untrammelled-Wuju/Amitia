package com.amitia.amitia_app.runtime.recovery

import com.amitia.amitia_app.runtime.api.RuntimeError
import com.amitia.amitia_app.runtime.api.RuntimeState

internal data class RuntimeRecoveryRequest(
    val failedGeneration: Long,
    val currentState: RuntimeState,
    val error: RuntimeError,
    val requestedStop: Boolean,
)
