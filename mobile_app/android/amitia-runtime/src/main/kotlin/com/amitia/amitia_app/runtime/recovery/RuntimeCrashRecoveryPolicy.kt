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
    private val stateStore: RuntimeDesiredStateStore = NoOpRuntimeDesiredStateStore,
    private val maxAttempts: Int = DEFAULT_MAX_ATTEMPTS,
    private val recoveryWindowMillis: Long = DEFAULT_RECOVERY_WINDOW_MILLIS,
    private val stableReadyWindowMillis: Long = DEFAULT_STABLE_READY_WINDOW_MILLIS,
    private val clock: () -> Long = { System.currentTimeMillis() },
) : RuntimeCrashRecoveryPolicy {

    @Volatile private var attempts: Int = 0
    @Volatile private var firstAttemptTime: Long = 0L
    @Volatile private var lastReadyGeneration: Long = -1L
    @Volatile private var lastReadyTime: Long = 0L
    @Volatile private var crashFingerprint: String? = null
    @Volatile private var crashFingerprintCount: Int = 0
    @Volatile private var crashFingerprintWindowStartedAt: Long = 0L

    init {
        restorePersistentState()
    }

    @Synchronized
    override fun evaluate(request: RuntimeRecoveryRequest): RuntimeRecoveryDecision {
        if (request.requestedStop) return RuntimeRecoveryDecision.DoNotRecover
        if (!isRecoverable(request.error)) return RuntimeRecoveryDecision.DoNotRecover

        when (installedRuntimeSource.current()) {
            is InstalledRuntimeResult.NoActiveRuntime,
            is InstalledRuntimeResult.Corrupted -> return RuntimeRecoveryDecision.DoNotRecover
            is InstalledRuntimeResult.Installed -> Unit
        }

        val now = clock()
        val currentFingerprint = crashFingerprint(request)
        if (crashFingerprintWindowStartedAt <= 0L ||
            now - crashFingerprintWindowStartedAt >= recoveryWindowMillis ||
            currentFingerprint != crashFingerprint
        ) {
            crashFingerprint = currentFingerprint
            crashFingerprintCount = 1
            crashFingerprintWindowStartedAt = now
        } else {
            crashFingerprintCount++
        }
        if (crashFingerprintCount >= CRASH_FINGERPRINT_LIMIT) {
            persistState()
            return RuntimeRecoveryDecision.Exhausted(crashFingerprintCount)
        }

        if (lastReadyGeneration >= 0 && now - lastReadyTime >= stableReadyWindowMillis &&
            request.failedGeneration >= lastReadyGeneration
        ) {
            attempts = 0
            firstAttemptTime = now
            lastReadyGeneration = -1L
            lastReadyTime = 0L
        }
        if (attempts == 0) firstAttemptTime = now

        if (attempts >= maxAttempts) {
            persistState()
            return RuntimeRecoveryDecision.Exhausted(attempts)
        }

        // Do not reset the budget merely because the process was killed and
        // restarted. The window only rolls forward after it genuinely expires.
        if (firstAttemptTime > 0L && now - firstAttemptTime >= recoveryWindowMillis && attempts > 0) {
            attempts = 0
            firstAttemptTime = now
        }

        val delay = calculateBackoff(attempts)
        attempts++
        persistState()
        return RuntimeRecoveryDecision.RecoverAfter(delay)
    }

    @Synchronized
    override fun recordReady(generation: Long) {
        lastReadyGeneration = generation
        lastReadyTime = clock()
        persistState()
    }

    override fun cancelPending() {
        // Pending-job cancellation must not erase crash history.
    }

    @Synchronized
    override fun resetBudget() {
        attempts = 0
        firstAttemptTime = 0L
        lastReadyGeneration = -1L
        lastReadyTime = 0L
        crashFingerprint = null
        crashFingerprintCount = 0
        crashFingerprintWindowStartedAt = 0L
        runCatching { stateStore.resetRecoveryPolicyState() }
    }

    private fun restorePersistentState() {
        val persisted = runCatching { stateStore.loadRecoveryPolicyState() }.getOrNull() ?: return
        attempts = persisted.attempts.coerceAtLeast(0)
        firstAttemptTime = persisted.firstAttemptTime.coerceAtLeast(0L)
        lastReadyGeneration = persisted.lastReadyGeneration
        lastReadyTime = persisted.lastReadyTime.coerceAtLeast(0L)
        crashFingerprint = persisted.crashFingerprint
        crashFingerprintCount = persisted.crashFingerprintCount.coerceAtLeast(0)
        crashFingerprintWindowStartedAt = persisted.crashFingerprintWindowStartedAt.coerceAtLeast(0L)
    }

    private fun persistState() {
        runCatching {
            stateStore.saveRecoveryPolicyState(
                RuntimeRecoveryPolicyState(
                    attempts = attempts,
                    firstAttemptTime = firstAttemptTime,
                    lastReadyGeneration = lastReadyGeneration,
                    lastReadyTime = lastReadyTime,
                    crashFingerprint = crashFingerprint,
                    crashFingerprintCount = crashFingerprintCount,
                    crashFingerprintWindowStartedAt = crashFingerprintWindowStartedAt,
                )
            )
        }
    }


    private fun crashFingerprint(request: RuntimeRecoveryRequest): String {
        val phase = request.error.details["phase"]
            ?: request.error.details["startupPhase"]
            ?: request.error.details["bootstrapCode"]
            ?: ""
        return listOf(
            request.error.code.name,
            request.error.componentId.orEmpty(),
            phase,
        ).joinToString("|")
    }

    private fun isRecoverable(error: com.amitia.amitia_app.runtime.api.RuntimeError): Boolean = when (error.code) {
        com.amitia.amitia_app.runtime.api.RuntimeErrorCode.STARTUP_CANCELLED,
        com.amitia.amitia_app.runtime.api.RuntimeErrorCode.STARTUP_GENERATION_STALE,
        com.amitia.amitia_app.runtime.api.RuntimeErrorCode.UNSUPPORTED_ABI,
        com.amitia.amitia_app.runtime.api.RuntimeErrorCode.UNSUPPORTED_PLATFORM,
        com.amitia.amitia_app.runtime.api.RuntimeErrorCode.RUNTIME_NOT_INSTALLED,
        com.amitia.amitia_app.runtime.api.RuntimeErrorCode.PACKAGE_NOT_FOUND,
        com.amitia.amitia_app.runtime.api.RuntimeErrorCode.PACKAGE_INVALID,
        com.amitia.amitia_app.runtime.api.RuntimeErrorCode.MANIFEST_INVALID,
        com.amitia.amitia_app.runtime.api.RuntimeErrorCode.RUNTIME_CORRUPTED,
        com.amitia.amitia_app.runtime.api.RuntimeErrorCode.NO_ACTIVE_RUNTIME,
        com.amitia.amitia_app.runtime.api.RuntimeErrorCode.STARTUP_HEALTH_AUTH_FAILED,
        com.amitia.amitia_app.runtime.api.RuntimeErrorCode.PERMISSION_DENIED -> false
        else -> true
    }

    private fun calculateBackoff(attemptNumber: Int): Long =
        (1000L shl attemptNumber.coerceAtMost(4)).coerceAtMost(16000L)

    internal fun snapshotState(): PolicyState = PolicyState(
        attempts = attempts,
        firstAttemptTime = firstAttemptTime,
        lastReadyGeneration = lastReadyGeneration,
        lastReadyTime = lastReadyTime,
    )

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
        const val CRASH_FINGERPRINT_LIMIT = 3
    }
}
