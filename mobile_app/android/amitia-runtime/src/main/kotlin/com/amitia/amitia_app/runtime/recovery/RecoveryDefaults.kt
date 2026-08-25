package com.amitia.amitia_app.runtime.recovery

internal class NoOpRecoveryPolicy : RuntimeCrashRecoveryPolicy {
    override fun evaluate(request: RuntimeRecoveryRequest): RuntimeRecoveryDecision =
        RuntimeRecoveryDecision.DoNotRecover
    override fun recordReady(generation: Long) {}
    override fun cancelPending() {}
    override fun resetBudget() {}
}

internal class AlwaysInstalledRuntimeSource : InstalledRuntimeSource {
    override fun current(): InstalledRuntimeResult = InstalledRuntimeResult.Installed
}
