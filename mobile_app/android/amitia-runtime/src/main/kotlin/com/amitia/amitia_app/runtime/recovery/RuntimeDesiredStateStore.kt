package com.amitia.amitia_app.runtime.recovery

import android.content.Context
import java.util.UUID

/**
 * Single persistent source of truth for the user's embedded-runtime intent and
 * crash-recovery bookkeeping. This store deliberately lives in the runtime
 * library so RuntimeService, boot/package receivers and recovery jobs cannot
 * drift into independent SharedPreferences contracts.
 */
data class RuntimeDesiredStateSnapshot(
    val desiredRunning: Boolean = false,
    val profile: String = "local",
    val lastRequestedAt: Long = 0L,
    val lastStartedAt: Long = 0L,
    val lastReadyAt: Long = 0L,
    val lastStoppedAt: Long = 0L,
    val lastFailureAt: Long = 0L,
    val lastFailureCode: String? = null,
    val lastFailureGeneration: Long = 0L,
    val recoveryAttempt: Int = 0,
    val recoveryFirstAttemptAt: Long = 0L,
    val nextRecoveryAt: Long = 0L,
    val recoveryReason: String? = null,
    val recoveryToken: String? = null,
    val recoveryExhausted: Boolean = false,
    val lastReadyGeneration: Long = -1L,
    val bootGeneration: Long = 0L,
)

data class RuntimeRecoveryPolicyState(
    val attempts: Int = 0,
    val firstAttemptTime: Long = 0L,
    val lastReadyGeneration: Long = -1L,
    val lastReadyTime: Long = 0L,
    val crashFingerprint: String? = null,
    val crashFingerprintCount: Int = 0,
    val crashFingerprintWindowStartedAt: Long = 0L,
)

interface RuntimeDesiredStateStore {
    fun snapshot(): RuntimeDesiredStateSnapshot
    fun requestStart(profile: String)
    fun markStarted(profile: String)
    fun markReady(generation: Long)
    fun requestStop()
    fun recordFailure(code: String, generation: Long)
    fun scheduleRecovery(failedGeneration: Long, delayMillis: Long, reason: String): RuntimeDesiredStateSnapshot
    fun clearScheduledRecovery(token: String? = null)
    fun markRecoveryExhausted(attempts: Int)
    fun incrementBootGeneration(): Long
    fun loadRecoveryPolicyState(): RuntimeRecoveryPolicyState
    fun saveRecoveryPolicyState(state: RuntimeRecoveryPolicyState)
    fun resetRecoveryPolicyState()
}

internal object NoOpRuntimeDesiredStateStore : RuntimeDesiredStateStore {
    override fun snapshot() = RuntimeDesiredStateSnapshot()
    override fun requestStart(profile: String) = Unit
    override fun markStarted(profile: String) = Unit
    override fun markReady(generation: Long) = Unit
    override fun requestStop() = Unit
    override fun recordFailure(code: String, generation: Long) = Unit
    override fun scheduleRecovery(failedGeneration: Long, delayMillis: Long, reason: String) = RuntimeDesiredStateSnapshot()
    override fun clearScheduledRecovery(token: String?) = Unit
    override fun markRecoveryExhausted(attempts: Int) = Unit
    override fun incrementBootGeneration(): Long = 0L
    override fun loadRecoveryPolicyState() = RuntimeRecoveryPolicyState()
    override fun saveRecoveryPolicyState(state: RuntimeRecoveryPolicyState) = Unit
    override fun resetRecoveryPolicyState() = Unit
}

class AndroidRuntimeDesiredStateStore(context: Context) : RuntimeDesiredStateStore {
    private val prefs = context.applicationContext.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
    private val lock = Any()

    override fun snapshot(): RuntimeDesiredStateSnapshot = synchronized(lock) {
        RuntimeDesiredStateSnapshot(
            desiredRunning = prefs.getBoolean(KEY_DESIRED_RUNNING, false),
            profile = prefs.getString(KEY_PROFILE, "local")?.trim().takeUnless { it.isNullOrEmpty() } ?: "local",
            lastRequestedAt = prefs.getLong(KEY_LAST_REQUESTED_AT, 0L),
            lastStartedAt = prefs.getLong(KEY_LAST_STARTED_AT, 0L),
            lastReadyAt = prefs.getLong(KEY_LAST_READY_AT, 0L),
            lastStoppedAt = prefs.getLong(KEY_LAST_STOPPED_AT, 0L),
            lastFailureAt = prefs.getLong(KEY_LAST_FAILURE_AT, 0L),
            lastFailureCode = prefs.getString(KEY_LAST_FAILURE_CODE, null),
            lastFailureGeneration = prefs.getLong(KEY_LAST_FAILURE_GENERATION, 0L),
            recoveryAttempt = prefs.getInt(KEY_RECOVERY_ATTEMPT, 0),
            recoveryFirstAttemptAt = prefs.getLong(KEY_RECOVERY_FIRST_ATTEMPT_AT, 0L),
            nextRecoveryAt = prefs.getLong(KEY_NEXT_RECOVERY_AT, 0L),
            recoveryReason = prefs.getString(KEY_RECOVERY_REASON, null),
            recoveryToken = prefs.getString(KEY_RECOVERY_TOKEN, null),
            recoveryExhausted = prefs.getBoolean(KEY_RECOVERY_EXHAUSTED, false),
            lastReadyGeneration = prefs.getLong(KEY_LAST_READY_GENERATION, -1L),
            bootGeneration = prefs.getLong(KEY_BOOT_GENERATION, 0L),
        )
    }

    override fun requestStart(profile: String) = synchronized(lock) {
        val normalized = profile.trim().ifEmpty { "local" }
        prefs.edit()
            .putBoolean(KEY_DESIRED_RUNNING, true)
            .putString(KEY_PROFILE, normalized)
            .putLong(KEY_LAST_REQUESTED_AT, System.currentTimeMillis())
            .putBoolean(KEY_RECOVERY_EXHAUSTED, false)
            .apply()
    }

    override fun markStarted(profile: String) = synchronized(lock) {
        prefs.edit()
            .putBoolean(KEY_DESIRED_RUNNING, true)
            .putString(KEY_PROFILE, profile.trim().ifEmpty { "local" })
            .putLong(KEY_LAST_STARTED_AT, System.currentTimeMillis())
            // A persisted recovery token means "start has not yet been accepted".
            // Once RuntimeService accepted the generation, Android must not replay
            // the same JobScheduler request after a service/job lifecycle stop.
            .remove(KEY_NEXT_RECOVERY_AT)
            .remove(KEY_RECOVERY_REASON)
            .remove(KEY_RECOVERY_TOKEN)
            .putBoolean(KEY_RECOVERY_EXHAUSTED, false)
            .apply()
    }

    override fun markReady(generation: Long) = synchronized(lock) {
        prefs.edit()
            .putLong(KEY_LAST_READY_AT, System.currentTimeMillis())
            .putLong(KEY_LAST_READY_GENERATION, generation)
            .remove(KEY_NEXT_RECOVERY_AT)
            .remove(KEY_RECOVERY_REASON)
            .remove(KEY_RECOVERY_TOKEN)
            .putBoolean(KEY_RECOVERY_EXHAUSTED, false)
            .apply()
    }

    override fun requestStop() = synchronized(lock) {
        prefs.edit()
            .putBoolean(KEY_DESIRED_RUNNING, false)
            .putLong(KEY_LAST_STOPPED_AT, System.currentTimeMillis())
            .remove(KEY_NEXT_RECOVERY_AT)
            .remove(KEY_RECOVERY_REASON)
            .remove(KEY_RECOVERY_TOKEN)
            .putInt(KEY_RECOVERY_ATTEMPT, 0)
            .putLong(KEY_RECOVERY_FIRST_ATTEMPT_AT, 0L)
            .remove(KEY_CRASH_FINGERPRINT)
            .putInt(KEY_CRASH_FINGERPRINT_COUNT, 0)
            .putLong(KEY_CRASH_FINGERPRINT_WINDOW_STARTED_AT, 0L)
            .putBoolean(KEY_RECOVERY_EXHAUSTED, false)
            .apply()
    }

    override fun recordFailure(code: String, generation: Long) = synchronized(lock) {
        prefs.edit()
            .putLong(KEY_LAST_FAILURE_AT, System.currentTimeMillis())
            .putString(KEY_LAST_FAILURE_CODE, code.trim())
            .putLong(KEY_LAST_FAILURE_GENERATION, generation)
            .apply()
    }

    override fun scheduleRecovery(failedGeneration: Long, delayMillis: Long, reason: String): RuntimeDesiredStateSnapshot = synchronized(lock) {
        val token = UUID.randomUUID().toString()
        val nextAt = System.currentTimeMillis() + delayMillis.coerceAtLeast(0L)
        prefs.edit()
            .putLong(KEY_LAST_FAILURE_GENERATION, failedGeneration)
            .putLong(KEY_NEXT_RECOVERY_AT, nextAt)
            .putString(KEY_RECOVERY_REASON, reason.trim().ifEmpty { "runtime_failure" })
            .putString(KEY_RECOVERY_TOKEN, token)
            .putBoolean(KEY_RECOVERY_EXHAUSTED, false)
            .apply()
        snapshot()
    }

    override fun clearScheduledRecovery(token: String?) = synchronized(lock) {
        val current = prefs.getString(KEY_RECOVERY_TOKEN, null)
        if (token != null && current != null && token != current) return@synchronized
        prefs.edit()
            .remove(KEY_NEXT_RECOVERY_AT)
            .remove(KEY_RECOVERY_REASON)
            .remove(KEY_RECOVERY_TOKEN)
            .apply()
    }

    override fun markRecoveryExhausted(attempts: Int) = synchronized(lock) {
        prefs.edit()
            .putBoolean(KEY_RECOVERY_EXHAUSTED, true)
            .putInt(KEY_RECOVERY_ATTEMPT, attempts.coerceAtLeast(0))
            .remove(KEY_NEXT_RECOVERY_AT)
            .remove(KEY_RECOVERY_REASON)
            .remove(KEY_RECOVERY_TOKEN)
            .apply()
    }

    override fun incrementBootGeneration(): Long = synchronized(lock) {
        val current = prefs.getLong(KEY_BOOT_GENERATION, 0L)
        val next = if (current == Long.MAX_VALUE) 1L else current + 1L
        prefs.edit().putLong(KEY_BOOT_GENERATION, next).commit()
        next
    }

    override fun loadRecoveryPolicyState(): RuntimeRecoveryPolicyState = synchronized(lock) {
        RuntimeRecoveryPolicyState(
            attempts = prefs.getInt(KEY_RECOVERY_ATTEMPT, 0),
            firstAttemptTime = prefs.getLong(KEY_RECOVERY_FIRST_ATTEMPT_AT, 0L),
            lastReadyGeneration = prefs.getLong(KEY_LAST_READY_GENERATION, -1L),
            lastReadyTime = prefs.getLong(KEY_LAST_READY_AT, 0L),
            crashFingerprint = prefs.getString(KEY_CRASH_FINGERPRINT, null),
            crashFingerprintCount = prefs.getInt(KEY_CRASH_FINGERPRINT_COUNT, 0),
            crashFingerprintWindowStartedAt = prefs.getLong(KEY_CRASH_FINGERPRINT_WINDOW_STARTED_AT, 0L),
        )
    }

    override fun saveRecoveryPolicyState(state: RuntimeRecoveryPolicyState) = synchronized(lock) {
        prefs.edit()
            .putInt(KEY_RECOVERY_ATTEMPT, state.attempts.coerceAtLeast(0))
            .putLong(KEY_RECOVERY_FIRST_ATTEMPT_AT, state.firstAttemptTime.coerceAtLeast(0L))
            .putLong(KEY_LAST_READY_GENERATION, state.lastReadyGeneration)
            .putLong(KEY_LAST_READY_AT, state.lastReadyTime.coerceAtLeast(0L))
            .putString(KEY_CRASH_FINGERPRINT, state.crashFingerprint)
            .putInt(KEY_CRASH_FINGERPRINT_COUNT, state.crashFingerprintCount.coerceAtLeast(0))
            .putLong(KEY_CRASH_FINGERPRINT_WINDOW_STARTED_AT, state.crashFingerprintWindowStartedAt.coerceAtLeast(0L))
            .apply()
    }

    override fun resetRecoveryPolicyState() = synchronized(lock) {
        prefs.edit()
            .putInt(KEY_RECOVERY_ATTEMPT, 0)
            .putLong(KEY_RECOVERY_FIRST_ATTEMPT_AT, 0L)
            .putLong(KEY_LAST_READY_GENERATION, -1L)
            .putLong(KEY_LAST_READY_AT, 0L)
            .remove(KEY_CRASH_FINGERPRINT)
            .putInt(KEY_CRASH_FINGERPRINT_COUNT, 0)
            .putLong(KEY_CRASH_FINGERPRINT_WINDOW_STARTED_AT, 0L)
            .putBoolean(KEY_RECOVERY_EXHAUSTED, false)
            .apply()
    }

    companion object {
        const val PREFS_NAME = "amitia_runtime_desired_state"
        const val KEY_DESIRED_RUNNING = "desired_running"
        const val KEY_PROFILE = "profile"
        private const val KEY_LAST_REQUESTED_AT = "last_requested_at"
        private const val KEY_LAST_STARTED_AT = "last_started_at"
        private const val KEY_LAST_READY_AT = "last_ready_at"
        private const val KEY_LAST_STOPPED_AT = "last_stopped_at"
        private const val KEY_LAST_FAILURE_AT = "last_failure_at"
        private const val KEY_LAST_FAILURE_CODE = "last_failure_code"
        private const val KEY_LAST_FAILURE_GENERATION = "last_failure_generation"
        private const val KEY_RECOVERY_ATTEMPT = "recovery_attempt"
        private const val KEY_RECOVERY_FIRST_ATTEMPT_AT = "recovery_first_attempt_at"
        private const val KEY_NEXT_RECOVERY_AT = "next_recovery_at"
        private const val KEY_RECOVERY_REASON = "recovery_reason"
        private const val KEY_RECOVERY_TOKEN = "recovery_token"
        private const val KEY_RECOVERY_EXHAUSTED = "recovery_exhausted"
        private const val KEY_LAST_READY_GENERATION = "last_ready_generation"
        private const val KEY_CRASH_FINGERPRINT = "crash_fingerprint"
        private const val KEY_CRASH_FINGERPRINT_COUNT = "crash_fingerprint_count"
        private const val KEY_CRASH_FINGERPRINT_WINDOW_STARTED_AT = "crash_fingerprint_window_started_at"
        private const val KEY_BOOT_GENERATION = "boot_generation"
    }
}
