package com.amitia.amitia_app.runtime.recovery

import com.amitia.amitia_app.runtime.install.ActiveRuntimeManager
import com.amitia.amitia_app.runtime.install.ActiveRuntimeResult

internal sealed interface InstalledRuntimeResult {
    data object Installed : InstalledRuntimeResult
    data object NoActiveRuntime : InstalledRuntimeResult
    data class Corrupted(val reason: String) : InstalledRuntimeResult
}

internal interface InstalledRuntimeSource {
    fun current(): InstalledRuntimeResult
}

internal class ActiveRuntimeBackedInstalledRuntimeSource(
    private val activeRuntimeManager: ActiveRuntimeManager?,
) : InstalledRuntimeSource {
    override fun current(): InstalledRuntimeResult {
        if (activeRuntimeManager == null) {
            return InstalledRuntimeResult.NoActiveRuntime
        }
        return when (val result = activeRuntimeManager.current()) {
            is ActiveRuntimeResult.Active -> InstalledRuntimeResult.Installed
            is ActiveRuntimeResult.NoActiveRuntime -> InstalledRuntimeResult.NoActiveRuntime
            is ActiveRuntimeResult.Failure -> InstalledRuntimeResult.Corrupted(result.message)
        }
    }
}
