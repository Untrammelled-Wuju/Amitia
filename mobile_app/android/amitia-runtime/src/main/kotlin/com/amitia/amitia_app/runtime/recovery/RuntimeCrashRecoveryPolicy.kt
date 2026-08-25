package com.amitia.amitia_app.runtime.recovery

internal interface RuntimeCrashRecoveryPolicy {
    fun evaluate(request: RuntimeRecoveryRequest): RuntimeRecoveryDecision
    fun recordReady(generation: Long)

    /** Cancel bookkeeping associated with a scheduled job without resetting crash history. */
    fun cancelPending() {}

    /** Explicitly start a new crash-recovery budget (user stop/retry or fresh lifecycle). */
    fun resetBudget() {}
}

internal class DefaultRuntimeCrashRecoveryPolicy(
    private val installedRuntimeSource: InstalledRuntimeSource,
    private val maxAttempts: Int = DEFAULT_MAX_ATTEMPTS,
    private val recoveryWindowMillis: Long = DEFAULT_RECOVERY_WINDOW_MILLIS,
    private val stableReadyWindowMillis: Long = DEFAULT_STABLE_READY_WINDOW_MILLIS,
    private val clock: () -> Long = { System.currentTimeMillis() },
) : RuntimeCrashRecoveryPolicy {

    @Volatile private var attempts: Int = 0
    @Volatile private var firstAttemptTime: Long = 0L
    @Volatile private var lastReadyGeneration: Long = -1L
    @Volatile private var lastReadyTime: Long = 0L

    override fun evaluate(request: RuntimeRecoveryRequest): RuntimeRecoveryDecision {
        if (request.requestedStop) {
            return RuntimeRecoveryDecision.DoNotRecover
        }

        if (!isRecoverable(request.error)) {
            return RuntimeRecoveryDecision.DoNotRecover
        }

        when (val installed = installedRuntimeSource.current()) {
            is InstalledRuntimeResult.NoActiveRuntime ->
                return RuntimeRecoveryDecision.DoNotRecover
            is InstalledRuntimeResult.Corrupted ->
                return RuntimeRecoveryDecision.DoNotRecover
            is InstalledRuntimeResult.Installed -> { /* proceed */ }
        }

        val now = clock()

        if (lastReadyGeneration >= 0 && now - lastReadyTime >= stableReadyWindowMillis &&
            request.failedGeneration >= lastReadyGeneration) {
            attempts = 0
            firstAttemptTime = now
            lastReadyGeneration = -1L
            lastReadyTime = 0L
        }

        if (attempts == 0) {
            firstAttemptTime = now
        }

        if (attempts >= maxAttempts) {
            return RuntimeRecoveryDecision.Exhausted(attempts)
        }

        if (now - firstAttemptTime >= recoveryWindowMillis && attempts > 0) {
            attempts = 0
            firstAttemptTime = now
        }

        val delay = calculateBackoff(attempts)
        attempts++

        return RuntimeRecoveryDecision.RecoverAfter(delay)
    }

    override fun recordReady(generation: Long) {
        lastReadyGeneration = generation
        lastReadyTime = clock()
    }

    override fun cancelPending() {
        // Scheduling is owned by RuntimeRecoveryScheduler. Cancelling a pending
        // job must not erase the attempt budget, otherwise every crash becomes
        // attempt #1 and RECOVERY_EXHAUSTED can never be reached.
    }

    override fun resetBudget() {
        attempts = 0
        firstAttemptTime = 0L
        lastReadyGeneration = -1L
        lastReadyTime = 0L
    }

    private fun isRecoverable(error: com.amitia.amitia_app.runtime.api.RuntimeError): Boolean {
        return when (error.code) {
            com.amitia.amitia_app.runtime.api.RuntimeErrorCode.STARTUP_CANCELLED,
            com.amitia.amitia_app.runtime.api.RuntimeErrorCode.STARTUP_GENERATION_STALE ->
                false
            com.amitia.amitia_app.runtime.api.RuntimeErrorCode.UNSUPPORTED_ABI,
            com.amitia.amitia_app.runtime.api.RuntimeErrorCode.UNSUPPORTED_PLATFORM ->
                false
            com.amitia.amitia_app.runtime.api.RuntimeErrorCode.RUNTIME_NOT_INSTALLED ->
                false
            com.amitia.amitia_app.runtime.api.RuntimeErrorCode.PACKAGE_NOT_FOUND,
            com.amitia.amitia_app.runtime.api.RuntimeErrorCode.PACKAGE_INVALID ->
                false
            com.amitia.amitia_app.runtime.api.RuntimeErrorCode.MANIFEST_INVALID,
            com.amitia.amitia_app.runtime.api.RuntimeErrorCode.RUNTIME_CORRUPTED ->
                false
            com.amitia.amitia_app.runtime.api.RuntimeErrorCode.NO_ACTIVE_RUNTIME ->
                false
            com.amitia.amitia_app.runtime.api.RuntimeErrorCode.STARTUP_HEALTH_AUTH_FAILED ->
                false
            com.amitia.amitia_app.runtime.api.RuntimeErrorCode.PERMISSION_DENIED ->
                false
            else -> true
        }
    }

    private fun calculateBackoff(attemptNumber: Int): Long {
        return (1000L shl attemptNumber.coerceAtMost(4)).coerceAtMost(16000L)
    }

    internal fun snapshotState(): PolicyState {
        return PolicyState(
            attempts = attempts,
            firstAttemptTime = firstAttemptTime,
            lastReadyGeneration = lastReadyGeneration,
            lastReadyTime = lastReadyTime,
        )
    }

    internal data class PolicyState(
        val attempts: Int,
        val firstAttemptTime: Long,
        val lastReadyGeneration: Long,
        val lastReadyTime: Long,
    )

    companion object {
        const val DEFAULT_MAX_ATTEMPTS = 3
        const val DEFAULT_RECOVERY_WINDOW_MILLIS = 5L * 60L * 1000L
        const val DEFAULT_STABLE_READY_WINDOW_MILLIS = 60L * 1000L
    }
}
